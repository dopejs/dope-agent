//! dope-app — the daemon application assembly. The trust-boundary kernel
//! (store, event bus, identity, auth, policy, secrets, audit) is built
//! directly; every other subsystem assembles as a builtin plugin
//! (`plugins::BUILTINS`) whose enablement is resolved from the per-data-dir
//! profile `<data_dir>/plugins.json`. The resolved [`dope_plugin::AssemblyReport`]
//! lands on `AppState.plugins` for `/v1/plugins` introspection.

use std::path::Path;
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::Duration;

use dope_api::AppState;
use dope_audit::Emitter as AuditEmitter;
use dope_checkpoints::Manager as CheckpointsManager;
use dope_config::{Config, Environment};
use dope_discord::{GatewayTransport as DiscordGatewayTransport, new_runtime as new_discord_runtime};
use dope_events::Bus;
use dope_identity::auth::Manager as AuthManager;
use dope_identity::{Manager as IdentityManager, Store as IdentityStore};
use dope_im::MessageLoop;
use dope_matrix::{
    ClientTransportConfig as MatrixClientTransportConfig, new_client_transport as new_matrix_transport,
    new_runtime as new_matrix_runtime,
};
use dope_policy::Engine as PolicyEngine;
use dope_router::SessionRouter;
use dope_runtime::Manager as RuntimeManager;
use dope_secrets::Manager as SecretsManager;
use dope_slack::{
    WebApiTransport as SlackWebApiTransport, WebApiTransportConfig as SlackWebApiTransportConfig,
    new_runtime as new_slack_runtime,
};
use dope_store::{SQLiteStore, SecretStoreHandle};
use dope_telegram::{
    BotApiTransport as TelegramBotApiTransport, BotApiTransportConfig as TelegramBotApiTransportConfig,
    Runtime as TelegramRuntime,
};

mod adapters;
mod error;
mod plugins;
mod restore;

pub use error::AppError;

/// Port of Go `environmentScope` for `config.Environment`.
pub fn environment_scope(environment: Environment) -> &'static str {
    match environment {
        Environment::Test => "test",
        Environment::Prod => "prod",
    }
}

/// The daemon application. The manager instances live inside
/// [`dope_api::AppState`]; lifecycle-bearing managers (scheduler, reminders,
/// sandboxes) are read back from the state in `serve`/`close`.
pub struct App {
    pub config: Config,
    pub state: AppState,
    event_bus: Arc<Bus>,
    /// Dedicated SQLite connection shared by the connector message loops and
    /// the four channel-connector runtimes (the runtimes take
    /// Arc<SQLiteStore>, which cannot be derived from the Mutex-wrapped
    /// primary handle).
    connector_store: Arc<SQLiteStore>,
    /// Constructed connector runtimes, filled by App::serve and closed by
    /// App::close. None until serve runs (or when all connectors are disabled).
    connector_runtimes: parking_lot::Mutex<Option<ConnectorRuntimes>>,
    closed: AtomicBool,
    /// Test-only visibility into the seam adapters wired by the builtin
    /// plugins: each manager keeps its hooks behind private fields, so the
    /// wiring tests assert through these captured clones instead.
    #[cfg(test)]
    wiring: AppWiring,
}

/// The seam adapters wired into the managers by the builtin plugins,
/// captured for the wiring tests. Every field is Some when the corresponding
/// adapter is wired (activation store/billing/chat, computeruse artifact
/// recorder, execprofile sandbox health checker, evidence routine collector,
/// delivery connector adapter, mcp execution starter + secret resolver).
#[cfg(test)]
#[derive(Default)]
pub struct AppWiring {
    pub activation_store: Option<Arc<dope_activation::SqliteActivationStore>>,
    pub activation_billing: Option<Arc<dope_activation::BillingProjectorAdapter>>,
    pub activation_chat: Option<Arc<dope_activation::ChatRunnerAdapter>>,
    pub computeruse_recorder: Option<Arc<dope_computeruse::SqliteArtifactRecorder>>,
    pub execprofile_health: Option<Arc<dope_execprofile::SandboxHealthChecker>>,
    pub evidence_collector: Option<Arc<dope_evidence::RoutineCollector>>,
    pub delivery_connector: Option<Arc<dope_delivery::ConnectorAdapter>>,
    pub mcp_starter: Option<Arc<adapters::McpExecutionStarter>>,
    pub mcp_secret_resolver: Option<Arc<adapters::McpSecretResolver>>,
}

/// The four channel-connector runtimes (Go app.App fields discordRuntime,
/// telegramRuntime, slackRuntime, matrixRuntime). Each is None when the
/// connector is disabled in config or its channel plugin is disabled in the
/// profile. The telegram/matrix runtimes drive a blocking transport loop
/// from a dedicated thread (start runs the loop on the calling thread), so
/// they are stored as Arc to share with the thread; discord/slack start
/// non-blocking and stay inline.
struct ConnectorRuntimes {
    discord: Option<dope_discord::Runtime>,
    telegram: Option<Arc<TelegramRuntime>>,
    slack: Option<dope_slack::Runtime>,
    matrix: Option<Arc<dope_matrix::Runtime>>,
    telegram_thread: Option<std::thread::JoinHandle<()>>,
}

impl App {
    /// Builds the full application under the profile at
    /// `<data_dir>/plugins.json` (default profile when the file is absent; a
    /// malformed profile fails the boot loudly).
    pub fn new(cfg: Config) -> Result<Self, AppError> {
        plugins::ensure_data_dir(&cfg.data_dir);
        let profile = dope_plugin::PluginProfile::load(&cfg.data_dir)
            .map_err(|err| AppError::PluginProfile(err.to_string()))?;
        Self::with_profile(cfg, profile)
    }

    /// Builds the application under an explicit plugin profile: kernel
    /// first, then every enabled builtin plugin in declared order, then the
    /// persisted-state restore.
    pub fn with_profile(
        cfg: Config,
        profile: dope_plugin::PluginProfile,
    ) -> Result<Self, AppError> {
        let data_dir = cfg.data_dir.clone();
        let env_scope = environment_scope(cfg.environment);
        let hosted = cfg.environment == Environment::Prod;

        // --- kernel: SQLite store (migrations run inside SQLiteStore::new).
        // The primary handle is shared by the API state and the
        // parking_lot::Mutex-based managers; sandbox/mcp/chat require a
        // std::sync::Mutex handle (their concrete constructor types), so a
        // second connection to the same WAL database is opened for them.
        let store = Arc::new(parking_lot::Mutex::new(
            SQLiteStore::new(&data_dir).map_err(AppError::Store)?,
        ));
        let secondary = Arc::new(std::sync::Mutex::new(
            SQLiteStore::new(&data_dir).map_err(AppError::Store)?,
        ));
        // The connector message loops + runtimes take a plain Arc<SQLiteStore>
        // (the runtimes are single-threaded, non-Send store owners), so a
        // dedicated connection to the same WAL database is opened for them.
        #[allow(clippy::arc_with_non_send_sync)] // port-mandated Arc<SQLiteStore>
        let connector_store = Arc::new(SQLiteStore::new(&data_dir).map_err(AppError::Store)?);

        // --- kernel: event bus + core managers ---
        let event_bus = Arc::new(Bus::new());
        let session_router = Arc::new(SessionRouter::new());
        let runtime = Arc::new(RuntimeManager::new());
        let checkpoints = Arc::new(CheckpointsManager::new(store.clone(), runtime.clone()));
        let policy_engine = Arc::new(PolicyEngine::new());
        let auth_manager = Arc::new(AuthManager::new());
        let identity_manager: Arc<IdentityManager<dyn IdentityStore + Send + Sync>> = {
            let erased: Arc<dyn IdentityStore + Send + Sync> =
                Arc::new(adapters::IdentityStoreHandle(store.clone()));
            Arc::new(IdentityManager::new(erased))
        };

        // --- kernel: secrets (tenant secret lifecycle + local value backend).
        // Trust-boundary decision: identity, auth, policy, secrets and audit
        // stay in the kernel and are not disableable plugins.
        let secret_backend = Arc::new(
            dope_secrets::LocalBackend::new(Path::new(&data_dir).join("tenant-secret-values"))
                .map_err(|err| AppError::Secrets(err.to_string()))?,
        );
        let secret_store = Arc::new(SecretStoreHandle::new(
            SQLiteStore::new(&data_dir).map_err(AppError::Store)?,
        ));
        let secret_manager = Arc::new(SecretsManager::new(
            secret_store.clone(),
            secret_backend.clone(),
        ));

        // --- kernel: audit emitter ---
        let audit_emitter = Arc::new(AuditEmitter::new(event_bus.clone()));

        // --- kernel state population ---
        let mut state = AppState::new(cfg.clone(), event_bus.clone(), store.clone());
        state.policy = Some(policy_engine);
        state.auth = Some(auth_manager);
        state.identity = Some(identity_manager);
        state.router = Some(session_router);
        state.runtime = Some(runtime.clone());
        state.secrets = Some(secret_manager);
        state.checkpoints = Some(checkpoints);
        state.audit_emitter = Some(audit_emitter);
        state.tenant_migration_status = Some(Arc::new(adapters::NoMigrationInProgress));

        // --- kernel seams consumed by plugins ---
        let mut seams = dope_plugin::SeamMap::new();
        seams.put(plugins::SecretStoreSeam(secret_store));
        seams.put(plugins::SecretBackendSeam(secret_backend));
        seams.put(plugins::WorkflowLauncherSeam(Arc::new(
            adapters::WorkflowLauncherImpl::new(runtime),
        )));

        // --- plugin assembly ---
        let report = dope_plugin::resolve(&plugins::descriptors(), &profile);
        for warning in &report.warnings {
            eprintln!("[dope] plugin profile: {warning}");
        }
        let mut asm = plugins::Assembly {
            cfg: cfg.clone(),
            env_scope,
            hosted,
            store,
            secondary,
            event_bus: event_bus.clone(),
            state,
            seams,
            #[cfg(test)]
            wiring: AppWiring::default(),
        };
        for plugin in plugins::BUILTINS {
            if report.enabled(plugin.descriptor.id) {
                (plugin.build)(&mut asm)?;
            }
        }
        asm.state.plugins = Some(Arc::new(report));

        // Restore persisted in-memory state from SQLite (Go
        // recoverPersistedStateWithSecrets). Idempotent: every Restore
        // replaces the in-memory registry wholesale; disabled plugins are
        // skipped by restore's per-manager Some guards.
        restore::recover_persisted_state(&asm.state, &event_bus, cfg.environment)?;

        Ok(App {
            config: cfg,
            state: asm.state,
            event_bus,
            connector_store,
            connector_runtimes: parking_lot::Mutex::new(None),
            closed: AtomicBool::new(false),
            #[cfg(test)]
            wiring: asm.wiring,
        })
    }

    /// Loads the effective config (env + config.json) and builds the app.
    pub fn from_env() -> Result<Self, AppError> {
        Self::new(dope_config::load()?)
    }

    /// True when `id` resolved enabled in the plugin assembly (states built
    /// without a report — tests — behave as all-enabled).
    fn plugin_enabled(&self, id: &str) -> bool {
        self.state
            .plugins
            .as_ref()
            .is_none_or(|report| report.enabled(id))
    }

    /// The axum router over the populated state (port of Go
    /// `api.NewServer(...).Handler()`).
    #[allow(clippy::needless_pass_by_value)]
    #[must_use]
    pub fn router(&self) -> axum::Router {
        dope_api::routes::router(self.state.clone())
    }

    /// Starts background loops (scheduler tick, reminders tick) best-effort,
    /// binds the HTTP listener, serves until a shutdown signal, then closes
    /// the application. Port of Go `App.Run`.
    pub async fn serve(self: Arc<Self>) -> Result<(), AppError> {
        let bind_addr = self.config.bind_addr.clone();

        self.start_background_loops();

        // Construct + start the enabled channel-connector runtimes before
        // serving (Go Run: discord -> telegram -> slack -> matrix).
        let mut runtimes = self.build_connector_runtimes().await?;
        self.start_connector_runtimes(&mut runtimes)?;
        *self.connector_runtimes.lock() = Some(runtimes);

        // Publish system.started (best effort; the store/bus carry it).
        let _ = self.publish_system_event(
            "system.started",
            serde_json::json!({ "service": "dope", "version": self.config.version }),
        );

        let app = self.router();
        let listener = tokio::net::TcpListener::bind(&bind_addr)
            .await
            .map_err(|source| AppError::Bind {
                addr: bind_addr.clone(),
                source,
            })?;
        eprintln!("[dope] listening on http://{bind_addr}");

        axum::serve(listener, app)
            .with_graceful_shutdown(shutdown_signal())
            .await
            .map_err(AppError::Serve)?;

        let _ = self.publish_system_event(
            "system.stopped",
            serde_json::json!({ "service": "dope", "reason": "shutdown" }),
        );
        self.close();
        Ok(())
    }

    /// Starts the background loops (scheduler, reminders) best-effort;
    /// failures are logged, not fatal (the sync ports record lifecycle only).
    fn start_background_loops(&self) {
        if let Some(scheduler) = &self.state.scheduler {
            if let Err(err) = scheduler.start() {
                eprintln!("[dope] scheduler start failed: {err}");
            }
        }
        if let Some(reminders) = &self.state.reminders {
            if let Err(err) = reminders.start() {
                eprintln!("[dope] reminders start failed: {err}");
            }
        }
        // Memory tick (spec 058 phase 2 W2/W5): every 60s run idle-triggered
        // consolidation and the retention sweep. Blocking work (store + LLM
        // consolidation) runs on the blocking pool.
        if self.state.memory.is_some() {
            let state = self.state.clone();
            tokio::spawn(async move {
                let mut ticker = tokio::time::interval(Duration::from_secs(60));
                ticker.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Skip);
                loop {
                    ticker.tick().await;
                    let tick_state = state.clone();
                    let _ = tokio::task::spawn_blocking(move || {
                        dope_api::routes::memory::memory_tick(&tick_state);
                    })
                    .await;
                }
            });
        }
    }

    /// Stops background loops and closes the bus (port of Go `App.Close`).
    /// Idempotent.
    pub fn close(&self) {
        if self.closed.swap(true, Ordering::SeqCst) {
            return;
        }
        // Connector runtimes close first (Go Close order: discord -> telegram
        // -> slack -> matrix); the telegram poll thread is joined once the
        // transport close drops the channel sender and unblocks its loop.
        let mut first_err: Option<String> = None;
        if let Some(runtimes) = self.connector_runtimes.lock().as_mut() {
            if let Some(discord) = &runtimes.discord {
                if let Err(err) = discord.close() {
                    first_err.get_or_insert_with(|| format!("discord: {err}"));
                }
            }
            if let Some(telegram) = &runtimes.telegram {
                if let Err(err) = telegram.close() {
                    first_err.get_or_insert_with(|| format!("telegram: {err}"));
                }
                if let Some(handle) = runtimes.telegram_thread.take() {
                    let _ = handle.join();
                }
            }
            if let Some(slack) = &runtimes.slack {
                if let Err(err) = slack.close() {
                    first_err.get_or_insert_with(|| format!("slack: {err}"));
                }
            }
            if let Some(matrix) = &runtimes.matrix {
                // The matrix client sync loop has no cancellation seam (the
                // transport close is a no-op), so its thread is detached and
                // ends with the process; only the close is reported here.
                if let Err(err) = matrix.close() {
                    first_err.get_or_insert_with(|| format!("matrix: {err}"));
                }
                // Leak a clone so the detached matrix thread never observes a
                // freed runtime after the App is dropped.
                std::mem::forget(matrix.clone());
            }
        }
        if let Some(err) = first_err {
            eprintln!("[dope] connector close error: {err}");
        }
        if let Some(scheduler) = &self.state.scheduler {
            let _ = scheduler.close();
        }
        if let Some(reminders) = &self.state.reminders {
            reminders.close();
        }
        if let Some(sandboxes) = &self.state.sandboxes {
            let _ = sandboxes.close();
        }
        self.event_bus.close();
    }

    // ---------------------------------------------------------------------
    // Channel-connector runtimes
    // ---------------------------------------------------------------------

    /// Builds one connector message loop over the shared managers (port of Go
    /// im.NewMessageLoop). dope-im::MessageLoop and dope_router::SessionRouter
    /// are not Clone, so each connector runtime receives its own loop + router
    /// over the same runtime/checkpoints/bus/store/chat deps — matching Go's
    /// per-runtime im.NewMessageLoop(sessionRouter, ...) construction.
    fn connector_message_loop(&self) -> MessageLoop {
        let runtime = self.state.runtime.clone().expect("runtime wired in kernel");
        let chat = self.state.chat.clone().expect("chat plugin enabled (channel requires)");
        MessageLoop::new(
            SessionRouter::new(),
            runtime.clone(),
            Some(CheckpointsManager::new(self.state.store.clone(), runtime)),
            Some((*self.event_bus).clone()),
            self.connector_store.clone(),
            (*chat).clone(),
        )
    }

    /// Resolves a connector secret value through the secret manager (Go
    /// slackBotTokenProvider/matrixBotAccessTokenProvider resolve via
    /// secrets.Resolve). None when no tenant or secret is available; the
    /// caller falls back to the raw config token.
    async fn resolve_connector_secret(&self, secret_ref: &str) -> Option<String> {
        let tenant_id = self
            .state
            .store
            .lock()
            .resolve_default_personal_tenant_id()
            .ok()?;
        let manager = self.state.secrets.clone()?;
        let resolved = manager
            .resolve(dope_secrets::ResolveInput {
                tenant_id,
                secret_ref: secret_ref.trim().to_string(),
            })
            .await
            .ok()?;
        Some(resolved.value)
    }

    /// Constructs the four connector runtimes for the enabled connectors in
    /// config (Go app.New connector block). A connector builds only when its
    /// config flag AND its channel plugin are enabled, so no network or
    /// credential is touched unless both agree.
    async fn build_connector_runtimes(&self) -> Result<ConnectorRuntimes, AppError> {
        let connector_cfg = &self.config.connectors;
        let supervisor = self
            .state
            .connectors
            .clone()
            .expect("connectors supervisor plugin enabled (channel requires)");
        let store = Some(self.connector_store.clone());
        let bus = Some((*self.event_bus).clone());

        // --- discord ---
        let discord_enabled =
            connector_cfg.discord.enabled && self.plugin_enabled("channel-discord");
        let discord_cfg = dope_discord::Config {
            enabled: discord_enabled,
            connector_id: connector_cfg.discord.connector_id.clone(),
            display_name: connector_cfg.discord.display_name.clone(),
            delivery_mode: connector_cfg.discord.delivery_mode.clone(),
            bot_token: connector_cfg.discord.bot_token.clone(),
            require_mention: connector_cfg.discord.require_mention,
            respond_in_dm: connector_cfg.discord.respond_in_dm,
            allowed_guild_ids: connector_cfg.discord.allowed_guild_ids.clone(),
            allowed_channel_ids: connector_cfg.discord.allowed_channel_ids.clone(),
            tenant_id: String::new(),
        };
        let discord_transport: Option<Box<dyn dope_discord::Transport>> = if discord_enabled {
            Some(Box::new(
                DiscordGatewayTransport::new(discord_cfg.clone()).map_err(|err| {
                    AppError::ConnectorRuntime(format!("discord transport: {err}"))
                })?,
            ))
        } else {
            None
        };
        let discord = new_discord_runtime(
            discord_cfg,
            Some(dope_telemetry::Logger::new(&self.config.log_level)),
            supervisor.clone(),
            // The discord runtime takes Arc<MessageLoop>; the loop is !Send
            // (single-threaded store), matching the runtime's design.
            #[allow(clippy::arc_with_non_send_sync)]
            Arc::new(self.connector_message_loop()),
            store.clone(),
            bus.clone(),
            discord_transport,
        )
        .map_err(|err| AppError::ConnectorRuntime(format!("discord runtime: {err}")))?;

        // --- telegram ---
        let telegram_enabled =
            connector_cfg.telegram.enabled && self.plugin_enabled("channel-telegram");
        let telegram_cfg = dope_telegram::Config {
            enabled: telegram_enabled,
            connector_id: connector_cfg.telegram.connector_id.clone(),
            display_name: connector_cfg.telegram.display_name.clone(),
            bot_token: connector_cfg.telegram.bot_token.clone(),
            bot_username: connector_cfg.telegram.bot_username.clone(),
            tenant_id: String::new(),
            allowments: restore::telegram_allowments_from_config(&connector_cfg.telegram),
        };
        let telegram_transport: Option<Arc<dyn dope_telegram::Transport>> = if telegram_enabled {
            Some(Arc::new(
                TelegramBotApiTransport::new(TelegramBotApiTransportConfig {
                    connector_id: connector_cfg.telegram.connector_id.clone(),
                    bot_token: connector_cfg.telegram.bot_token.clone(),
                    bot_username: connector_cfg.telegram.bot_username.clone(),
                    base_url: connector_cfg.telegram.bot_api_base_url.clone(),
                    ..Default::default()
                })
                .map_err(|err| {
                    AppError::ConnectorRuntime(format!("telegram transport: {err}"))
                })?,
            ))
        } else {
            None
        };
        let telegram = TelegramRuntime::new(
            telegram_cfg,
            supervisor.clone(),
            Some(self.connector_message_loop()),
            store.clone(),
            bus.clone(),
            telegram_transport,
        )
        .map_err(|err| AppError::ConnectorRuntime(format!("telegram runtime: {err}")))?
        .map(Arc::new);

        // --- slack ---
        let slack_enabled = connector_cfg.slack.enabled && self.plugin_enabled("channel-slack");
        let slack_cfg = dope_slack::Config {
            enabled: slack_enabled,
            connector_id: connector_cfg.slack.connector_id.clone(),
            display_name: connector_cfg.slack.display_name.clone(),
            workspace_binding_id: connector_cfg.slack.workspace_binding_id.clone(),
            workspace_id: connector_cfg.slack.workspace_id.clone(),
            bot_user_id: connector_cfg.slack.bot_user_id.clone(),
            allowed_channel_ids: connector_cfg.slack.allowed_channel_ids.clone(),
            allowed_dm_user_ids: connector_cfg.slack.allowed_dm_user_ids.clone(),
            allowed_dm_user_groups: connector_cfg.slack.allowed_dm_user_groups.clone(),
            tenant_id: String::new(),
        };
        let slack_token = if connector_cfg.slack.bot_token_secret_ref.trim().is_empty() {
            String::new()
        } else {
            self.resolve_connector_secret(&connector_cfg.slack.bot_token_secret_ref)
                .await
                .unwrap_or_default()
        };
        let slack_transport: Option<Arc<dyn dope_slack::Transport>> = if slack_enabled {
            Some(Arc::new(SlackWebApiTransport::new(
                SlackWebApiTransportConfig {
                    connector_id: connector_cfg.slack.connector_id.clone(),
                    base_url: connector_cfg.slack.api_base_url.clone(),
                    bot_token: slack_token,
                    ..Default::default()
                },
            )))
        } else {
            None
        };
        let slack = new_slack_runtime(
            slack_cfg,
            supervisor.clone(),
            self.connector_message_loop(),
            store.clone(),
            bus.clone(),
            slack_transport,
        )
        .map_err(|err| AppError::ConnectorRuntime(format!("slack runtime: {err}")))?;

        // --- matrix ---
        let matrix_enabled = connector_cfg.matrix.enabled && self.plugin_enabled("channel-matrix");
        let matrix_cfg = dope_matrix::types::Config {
            enabled: matrix_enabled,
            connector_id: connector_cfg.matrix.connector_id.clone(),
            display_name: connector_cfg.matrix.display_name.clone(),
            homeserver_url: connector_cfg.matrix.homeserver_url.clone(),
            homeserver_id: connector_cfg.matrix.homeserver_id.clone(),
            bot_user_id: connector_cfg.matrix.bot_user_id.clone(),
            selected_room_ids: connector_cfg.matrix.selected_room_ids.clone(),
            allowed_direct_user_ids: connector_cfg.matrix.allowed_direct_user_ids.clone(),
            configured_commands: connector_cfg.matrix.configured_commands.clone(),
        };
        let matrix_transport: Option<Box<dyn dope_matrix::Transport>> = if matrix_enabled {
            Some(Box::new(
                new_matrix_transport(MatrixClientTransportConfig {
                    connector_id: connector_cfg.matrix.connector_id.clone(),
                    homeserver_url: connector_cfg.matrix.homeserver_url.clone(),
                    bot_access_token: connector_cfg.matrix.bot_access_token.clone(),
                    selected_room_ids: connector_cfg.matrix.selected_room_ids.clone(),
                    allowed_direct_user_ids: connector_cfg.matrix.allowed_direct_user_ids.clone(),
                    ..Default::default()
                })
                .map_err(|err| {
                    AppError::ConnectorRuntime(format!("matrix transport: {err}"))
                })?,
            ))
        } else {
            None
        };
        let matrix = new_matrix_runtime(
            matrix_cfg,
            supervisor.clone(),
            self.connector_message_loop(),
            store,
            bus,
            matrix_transport,
        )
        .map_err(|err| AppError::ConnectorRuntime(format!("matrix runtime: {err}")))?
        .map(Arc::new);

        Ok(ConnectorRuntimes {
            discord,
            telegram,
            slack,
            matrix,
            telegram_thread: None,
        })
    }

    /// Starts the constructed connector runtimes (Go App.Run connector
    /// block). Discord/slack start non-blocking on the calling thread; the
    /// telegram transport loop blocks until close, so it runs on a dedicated
    /// thread that is joined in App::close.
    fn start_connector_runtimes(
        &self,
        runtimes: &mut ConnectorRuntimes,
    ) -> Result<(), AppError> {
        if let Some(discord) = &runtimes.discord {
            discord
                .start()
                .map_err(|err| AppError::ConnectorStart(format!("discord: {err}")))?;
        }
        if let Some(telegram) = &runtimes.telegram {
            // SAFETY: dope_telegram::Runtime is !Send (its inner store
            // connection is single-threaded), so the thread receives a raw
            // pointer instead of moving the runtime. The runtime is kept alive
            // by the App's Arc for the whole process, and every runtime method
            // accesses inner state through the parking_lot::Mutex, so
            // concurrent start() (this thread) and close() (the app thread) are
            // serialized. close() closes the transport, dropping the poll
            // thread's channel sender, which unblocks start() so the thread
            // returns and can be joined before the App is dropped.
            let raw = Arc::as_ptr(telegram) as usize;
            let handle = std::thread::Builder::new()
                .name("telegram-connector".to_string())
                .spawn(move || {
                    // SAFETY: raw is the address of the App-owned runtime,
                    // which stays valid until this thread is joined in close().
                    let rt = unsafe { &*(raw as *const TelegramRuntime) };
                    let _ = rt.start();
                })
                .map_err(|err| AppError::ConnectorStart(format!("telegram thread: {err}")))?;
            runtimes.telegram_thread = Some(handle);
        }
        if let Some(slack) = &runtimes.slack {
            slack
                .start()
                .map_err(|err| AppError::ConnectorStart(format!("slack: {err}")))?;
        }
        if let Some(matrix) = &runtimes.matrix {
            // SAFETY: same serialized-inner rationale as the telegram thread.
            // The matrix client sync loop has no cancellation seam (the
            // transport close is a no-op), so the thread is detached and runs
            // until the process exits; close() leaks a clone of the App's Arc
            // so the runtime is never freed under the running thread.
            let raw = Arc::as_ptr(matrix) as usize;
            let tenant_id = self
                .state
                .store
                .lock()
                .resolve_default_personal_tenant_id()
                .unwrap_or_default();
            std::thread::Builder::new()
                .name("matrix-connector".to_string())
                .spawn(move || {
                    // SAFETY: raw is the address of the App-owned runtime;
                    // close() leaks a clone so it stays valid for the process.
                    let rt = unsafe { &*(raw as *const dope_matrix::Runtime) };
                    let _ = rt.start(&tenant_id);
                })
                .map_err(|err| AppError::ConnectorStart(format!("matrix thread: {err}")))?;
        }
        Ok(())
    }

    /// Persists a system event on the store and publishes it on the bus
    /// (port of Go `publishSystemEvent`).
    fn publish_system_event(&self, name: &str, payload: serde_json::Value) -> Result<(), AppError> {
        let mut event = dope_events::Event::default();
        event.environment_scope = environment_scope(self.config.environment).to_string();
        event.category = "system".to_string();
        event.name = name.to_string();
        event.resource = dope_events::Resource {
            kind: "system".to_string(),
            id: "dope".to_string(),
        };
        if let Some(object) = payload.as_object() {
            event.payload = object.clone();
        }
        let persisted = self
            .state
            .store
            .lock()
            .append_event(&event)
            .map_err(AppError::SystemEvent)?;
        self.event_bus.publish(persisted);
        Ok(())
    }
}

/// Waits for SIGINT (ctrl-c) or SIGTERM.
async fn shutdown_signal() {
    let ctrl_c = async {
        let _ = tokio::signal::ctrl_c().await;
    };
    #[cfg(unix)]
    let terminate = async {
        let mut sigterm = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
            .expect("install SIGTERM handler");
        sigterm.recv().await;
    };
    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();
    tokio::select! {
        _ = ctrl_c => {}
        _ = terminate => {}
    }
}
#[cfg(test)]
mod tests {
    use super::*;

    use axum::body::to_bytes;
    use axum::http::{Request, StatusCode};
    use tower::ServiceExt;

    /// A test config pointing at a fresh temp data dir.
    fn test_config() -> Config {
        let dir = std::env::temp_dir().join(format!("dope-app-smoke-{}", uuid::Uuid::now_v7()));
        std::fs::create_dir_all(&dir).expect("create temp data dir");
        Config {
            environment: Environment::Test,
            bind_addr: "127.0.0.1:0".to_string(),
            data_dir: dir.to_string_lossy().into_owned(),
            log_level: "info".to_string(),
            version: "dev".to_string(),
            llm: Default::default(),
            connectors: Default::default(),
        }
    }

    /// End-to-end wiring proof: build the full App against a temp-dir store,
    /// serve the router in-process, and hit the introspection routes.
    #[tokio::test]
    async fn healthz_returns_ok_with_full_wiring() {
        let config = test_config();
        let app = App::new(config).expect("build app");

        // The store migrated to head schema and all managers are populated.
        let schema_version = app
            .state
            .store
            .lock()
            .schema_version()
            .expect("schema version");
        assert_eq!(schema_version, dope_store::CURRENT_SCHEMA_VERSION);
        assert!(app.state.policy.is_some());
        assert!(app.state.identity.is_some());
        assert!(app.state.chat.is_some());
        assert!(app.state.scheduler.is_some());
        assert!(app.state.evaluation.is_some());
        assert!(app.state.live_validation.is_some());

        // The default profile resolves every builtin plugin enabled.
        let report = app.state.plugins.as_ref().expect("assembly report");
        assert!(report.plugins.iter().all(|p| p.enabled), "all builtins enabled");
        assert!(report.warnings.is_empty());

        let router = app.router();
        let response = router
            .clone()
            .oneshot(
                Request::builder()
                    .uri("/healthz")
                    .body(axum::body::Body::empty())
                    .expect("request"),
            )
            .await
            .expect("oneshot");
        assert_eq!(response.status(), StatusCode::OK);
        let bytes = to_bytes(response.into_body(), usize::MAX)
            .await
            .expect("body");
        let json: serde_json::Value = serde_json::from_slice(&bytes).expect("json body");
        assert_eq!(json, serde_json::json!({ "ok": true, "service": "dope" }));

        // /version and /v1/system/info also route through the populated state.
        let version_response = router
            .clone()
            .oneshot(
                Request::builder()
                    .uri("/version")
                    .body(axum::body::Body::empty())
                    .expect("request"),
            )
            .await
            .expect("oneshot");
        assert_eq!(version_response.status(), StatusCode::OK);
        let version_bytes = to_bytes(version_response.into_body(), usize::MAX)
            .await
            .expect("body");
        let version_json: serde_json::Value =
            serde_json::from_slice(&version_bytes).expect("json body");
        assert_eq!(version_json, serde_json::json!({ "version": "dev" }));

        let info_response = router
            .oneshot(
                Request::builder()
                    .uri("/v1/system/info")
                    .body(axum::body::Body::empty())
                    .expect("request"),
            )
            .await
            .expect("oneshot");
        assert_eq!(info_response.status(), StatusCode::OK);

        // Close is idempotent and stops the background loops.
        app.close();
        app.close();
    }

    /// The app can be built twice against the same data dir (restart path);
    /// migrations are idempotent.
    #[tokio::test]
    async fn rebuild_on_existing_store_is_idempotent() {
        let config = test_config();
        let app = App::new(config.clone()).expect("first build");
        app.close();
        let app2 = App::new(config).expect("second build");
        assert_eq!(
            app2.state
                .store
                .lock()
                .schema_version()
                .expect("schema version"),
            dope_store::CURRENT_SCHEMA_VERSION
        );
        app2.close();
    }

    /// Disabling a leaf plugin leaves its manager unwired (the API answers
    /// not-wired) while the rest of the assembly is unaffected, and the
    /// report records the decision.
    #[tokio::test]
    async fn disabled_leaf_plugin_leaves_manager_unwired() {
        let config = test_config();
        let profile = dope_plugin::PluginProfile {
            disabled: vec!["triage".to_string()],
            ..Default::default()
        };
        let app = App::with_profile(config, profile).expect("build app");
        assert!(app.state.triage.is_none(), "triage unwired");
        assert!(app.state.chat.is_some(), "unrelated plugins unaffected");
        let report = app.state.plugins.as_ref().expect("report");
        assert!(!report.enabled("triage"));
        let triage = report.plugins.iter().find(|p| p.id == "triage").expect("entry");
        assert_eq!(triage.reason.as_deref(), Some("disabled by profile"));
        app.close();
    }

    /// Disabling a dependency transitively disables its dependents — the
    /// fail-closed gates (webhook quota, billing enforcement) can never run
    /// half-wired.
    #[tokio::test]
    async fn disabling_billing_transitively_disables_dependents() {
        let config = test_config();
        let profile = dope_plugin::PluginProfile {
            disabled: vec!["billing".to_string()],
            ..Default::default()
        };
        let app = App::with_profile(config, profile).expect("build app");
        assert!(app.state.billing.is_none());
        assert!(app.state.activation.is_none(), "activation requires billing");
        assert!(app.state.webhooks.is_none(), "webhooks require billing");
        assert!(app.state.evaluation.is_none(), "evaluation requires billing");
        assert!(app.state.live_validation.is_none(), "live-validation requires billing");
        let report = app.state.plugins.as_ref().expect("report");
        let webhooks = report.plugins.iter().find(|p| p.id == "webhooks").expect("entry");
        assert_eq!(
            webhooks.reason.as_deref(),
            Some("requires disabled plugin `billing`")
        );
        app.close();
    }

    /// A profile written to <data_dir>/plugins.json is picked up by App::new.
    #[tokio::test]
    async fn profile_file_in_data_dir_is_loaded() {
        let config = test_config();
        std::fs::write(
            std::path::Path::new(&config.data_dir).join(dope_plugin::PROFILE_FILE_NAME),
            serde_json::json!({ "disabled": ["memory"] }).to_string(),
        )
        .expect("write profile");
        let app = App::new(config).expect("build app");
        assert!(app.state.memory.is_none(), "memory disabled via plugins.json");
        app.close();
    }

    /// Connector runtimes are skipped when every connector is disabled (no
    /// network, no credentials), and are constructed when a connector is
    /// enabled with a token. Restore is idempotent: rebuilding the app against
    /// the same data dir runs recoverPersistedState again without error.
    #[tokio::test]
    async fn connector_runtimes_constructed_or_skipped_and_restore_idempotent() {
        // All connectors disabled: the four runtimes are None (skipped).
        let config = test_config();
        let app = App::new(config.clone()).expect("build app");
        let runtimes = app
            .build_connector_runtimes()
            .await
            .expect("build connector runtimes");
        assert!(runtimes.discord.is_none(), "discord disabled -> skipped");
        assert!(runtimes.telegram.is_none(), "telegram disabled -> skipped");
        assert!(runtimes.slack.is_none(), "slack disabled -> skipped");
        assert!(runtimes.matrix.is_none(), "matrix disabled -> skipped");
        // Starting the (empty) runtime set is a no-op.
        let mut runtimes = runtimes;
        app.start_connector_runtimes(&mut runtimes)
            .expect("start no runtimes");
        app.close();

        // Discord enabled with a token: the runtime is constructed. Nothing is
        // started, so no network is touched.
        let mut enabled = config.clone();
        enabled.connectors.discord.enabled = true;
        enabled.connectors.discord.bot_token = "test-token".to_string();
        enabled.connectors.discord.connector_id = "discord-test".to_string();
        enabled.connectors.discord.display_name = "Discord Test".to_string();
        let app2 = App::new(enabled.clone()).expect("build app with discord enabled");
        let runtimes2 = app2
            .build_connector_runtimes()
            .await
            .expect("build runtimes with discord enabled");
        assert!(
            runtimes2.discord.is_some(),
            "discord enabled -> runtime constructed"
        );
        assert!(
            app2.state
                .connectors
                .as_ref()
                .expect("connectors supervisor")
                .list()
                .is_empty(),
            "no connector registered until start"
        );
        app2.close();

        // Restore idempotence: rebuild against the same data dir (the store now
        // holds whatever bootstrap wrote) and recover again.
        let app3 = App::new(enabled).expect("rebuild app on existing store");
        app3.close();
    }

    /// Disabling a channel plugin gates the runtime even when the connector
    /// config flag is on: profile wins, no network or credential is touched.
    #[tokio::test]
    async fn disabled_channel_plugin_gates_connector_runtime() {
        let mut config = test_config();
        config.connectors.discord.enabled = true;
        config.connectors.discord.bot_token = "test-token".to_string();
        config.connectors.discord.connector_id = "discord-test".to_string();
        config.connectors.discord.display_name = "Discord Test".to_string();
        let profile = dope_plugin::PluginProfile {
            disabled: vec!["channel-discord".to_string()],
            ..Default::default()
        };
        let app = App::with_profile(config, profile).expect("build app");
        let runtimes = app
            .build_connector_runtimes()
            .await
            .expect("build connector runtimes");
        assert!(
            runtimes.discord.is_none(),
            "channel-discord disabled -> runtime skipped despite config flag"
        );
        app.close();
    }

    /// Every seam adapter is wired into its manager by the builtin plugins
    /// (activation store/billing/chat, computeruse artifact recorder,
    /// execprofile sandbox health checker, evidence routine collector,
    /// delivery connector adapter, mcp execution starter + secret resolver).
    #[test]
    fn wired_seam_adapters_are_non_none() {
        let config = test_config();
        let app = App::new(config).expect("build app");
        let wiring = &app.wiring;
        assert!(
            wiring.activation_store.is_some(),
            "activation StateStore/IdentityRepository/AuditSink (SqliteActivationStore)"
        );
        assert!(wiring.activation_billing.is_some(), "activation BillingProjector");
        assert!(wiring.activation_chat.is_some(), "activation ChatRunner");
        assert!(wiring.computeruse_recorder.is_some(), "computeruse ArtifactRecorder");
        assert!(wiring.execprofile_health.is_some(), "execprofile HealthChecker");
        assert!(wiring.evidence_collector.is_some(), "evidence Collector");
        assert!(wiring.delivery_connector.is_some(), "delivery ConnectorAdapter");
        assert!(wiring.mcp_starter.is_some(), "mcp AttachedExecutionStarter");
        assert!(wiring.mcp_secret_resolver.is_some(), "mcp SecretResolver");
        app.close();
    }

}
