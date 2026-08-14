//! dope-app — the daemon application assembly (port of
//! daemon/internal/app/app.go). Builds every manager, populates the
//! dope-api AppState, and serves the axum router until shutdown.

use std::path::Path;
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::Duration;

use dope_activation::{Dependencies as ActivationDependencies, Service as ActivationService};
use dope_api::AppState;
use dope_audit::Emitter as AuditEmitter;
use dope_billing::Manager as BillingManager;
use dope_calendar::Manager as CalendarManager;
use dope_capabilities::Supervisor as CapabilitiesSupervisor;
use dope_chat::{ChatStore, Service as ChatService};
use dope_checkpoints::Manager as CheckpointsManager;
use dope_computeruse::{Dependencies as ComputerUseDependencies, Manager as ComputerUseManager};
use dope_config::{Config, Environment};
use dope_connectors::Supervisor as ConnectorsSupervisor;
use dope_delivery::{Manager as DeliveryManager, TestSinkAdapter};
use dope_evaluation::{Dependencies as EvaluationDependencies, Manager as EvaluationManager};
use dope_events::Bus;
use dope_execprofile::Manager as ExecProfileManager;
use dope_identity::auth::Manager as AuthManager;
use dope_identity::{Manager as IdentityManager, Store as IdentityStore};
use dope_integrations::Manager as IntegrationsManager;
use dope_livevalidation::{
    Dependencies as LiveValidationDependencies, Manager as LiveValidationManager,
};
use dope_llm::Dispatcher;
use dope_mail::Manager as MailManager;
use dope_mcp::Manager as McpManager;
use dope_policy::Engine as PolicyEngine;
use dope_reminders::{Dependencies as RemindersDependencies, Manager as RemindersManager};
use dope_router::SessionRouter;
use dope_runtime::Manager as RuntimeManager;
use dope_sandbox::Manager as SandboxManager;
use dope_scheduler::{Dependencies as SchedulerDependencies, Scheduler};
use dope_secrets::Manager as SecretsManager;
use dope_setupwizard::{
    ServiceDependencies as SetupWizardDependencies, new_service as new_setup_wizard,
};
use dope_skills::Registry as SkillsRegistry;
use dope_store::{BillingRepositoryHandle, ComputerUseStoreHandle, SQLiteStore, SecretStoreHandle};
use dope_triage::Manager as TriageManager;
use dope_webhook::Manager as WebhookManager;

mod adapters;
mod error;

pub use error::AppError;

/// Port of Go `environmentScope` for `config.Environment`.
pub fn environment_scope(environment: Environment) -> &'static str {
    match environment {
        Environment::Test => "test",
        Environment::Prod => "prod",
    }
}

/// The daemon application (port of Go `app.App`). The manager instances
/// live inside [`dope_api::AppState`]; the fields retained here are the ones
/// with explicit lifecycle (`start`/`close`) in Go `Run`/`Close`.
pub struct App {
    pub config: Config,
    pub state: AppState,
    event_bus: Arc<Bus>,
    sandboxes: Option<Arc<SandboxManager>>,
    scheduler: Option<Arc<Scheduler>>,
    reminders: Option<Arc<RemindersManager>>,
    closed: AtomicBool,
}

impl App {
    /// Builds the full application: config, store + migrations, every
    /// manager, and the populated API state. Port of Go `app.New`.
    pub fn new(cfg: Config) -> Result<Self, AppError> {
        let data_dir = cfg.data_dir.clone();
        let env_scope = environment_scope(cfg.environment);
        let hosted = cfg.environment == Environment::Prod;

        // --- SQLite store (migrations run inside SQLiteStore::new).
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

        // --- event bus ---
        let event_bus = Arc::new(Bus::new());

        // --- core managers ---
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

        // --- secrets (tenant secret lifecycle + local value backend) ---
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

        // --- LLM dispatcher (echo fallback + managed CLI providers) ---
        let llm = Arc::new(Dispatcher::new());
        if cfg.llm.default_timeout_ms > 0 {
            llm.set_default_timeout(Duration::from_millis(cfg.llm.default_timeout_ms as u64));
        }
        if cfg.llm.default_max_retries > 0 {
            llm.set_default_retries(cfg.llm.default_max_retries);
        }
        if !cfg.llm.default_model.trim().is_empty() {
            llm.set_default_model(&cfg.llm.default_model);
        }
        let managed_registry: Arc<dyn dope_providers::ManagedRegistry> =
            Arc::new(dope_managedproviders::Registry::new(&cfg, None));
        for bridge in managed_registry.list() {
            llm.register_provider(bridge.provider());
        }
        // Deterministic in-process fallback so the daemon always has a
        // default provider (Go registers echo in dispatcher.go).
        llm.register_provider(Arc::new(dope_llm::EchoProvider::new()));
        let default_provider = if cfg.llm.default_provider.trim().is_empty() {
            "echo".to_string()
        } else {
            cfg.llm.default_provider.clone()
        };
        let _ = llm.set_default_provider(&default_provider);

        // --- skills registry ---
        let skills = Arc::new(
            SkillsRegistry::new(&data_dir).map_err(|err| AppError::Skills(err.to_string()))?,
        );

        // --- sandbox + MCP (share the std::sync::Mutex store handle) ---
        let sandboxes = Arc::new(SandboxManager::new(
            cfg.clone(),
            Some(secondary.clone()),
            (*event_bus).clone(),
            PolicyEngine::new(),
        ));
        // TODO(app wiring): sandbox persistence + secret resolution are
        // wired; the sandbox secret manager is a second instance sharing the
        // same store/backend because set_secret_manager takes ownership.
        sandboxes.set_secret_manager(SecretsManager::new(
            secret_store.clone(),
            secret_backend.clone(),
        ));
        let mcp = Arc::new(McpManager::new(
            cfg.clone(),
            Some(secondary.clone()),
            Some((*event_bus).clone()),
            None, // TODO(app wiring): AttachedExecutionStarter (sandbox execution plane)
            Some(PolicyEngine::new()),
            None, // TODO(app wiring): concrete MCP transports are deferred
        ));
        // TODO(app wiring): mcp.set_secret_manager needs a dope_secrets::Manager
        // -> mcp::SecretResolver adapter; falls back to mcp-secrets.json.

        // --- domain managers ---
        let integrations = Arc::new(IntegrationsManager::new(env_scope));
        let calendar = Arc::new(CalendarManager::new(env_scope));
        let mail = Arc::new(MailManager::new(env_scope));
        let providers = Arc::new(dope_providers::new_manager(
            cfg.llm.clone(),
            Some(llm.clone()),
            vec![managed_registry],
        ));
        let connectors = Arc::new(ConnectorsSupervisor::new());
        let capabilities = Arc::new(CapabilitiesSupervisor::new());
        let chat = Arc::new(ChatService::new_service(
            llm.clone(),
            Some(providers.clone()),
            Some(skills.clone()),
            Some((*event_bus).clone()),
            Some(secondary.clone() as Arc<dyn ChatStore>),
        ));

        // --- billing (store-backed repository handle) ---
        let billing_repo = Arc::new(BillingRepositoryHandle::new(
            SQLiteStore::new(&data_dir).map_err(AppError::Store)?,
        ));
        let billing = Arc::new(BillingManager::new(billing_repo));

        // --- activation (store/billing/chat seams not yet implemented in
        // dope-store; the service fails closed per call) ---
        // TODO(app wiring): activation StateStore/IdentityRepository/
        // BillingProjector/ChatRunner/AuditSink impls are missing.
        let activation = Arc::new(ActivationService::new(ActivationDependencies {
            environment_scope: env_scope.to_string(),
            hosted,
            ..ActivationDependencies::default()
        }));

        // --- computer use (store-backed handle) ---
        let computeruse_store = Arc::new(ComputerUseStoreHandle::new(
            SQLiteStore::new(&data_dir).map_err(AppError::Store)?,
        ));
        let computer_use = Arc::new(ComputerUseManager::new(ComputerUseDependencies {
            environment_scope: env_scope.to_string(),
            runtime: Some(runtime.clone()),
            policy: Some(policy_engine.clone()),
            store: computeruse_store,
            driver: None,
            // TODO(app wiring): ArtifactRecorder over dope-artifacts not yet ported
            artifacts: None,
        }));

        // --- delivery (test sink adapter; connector adapter is deferred to
        // the wave-7 channels port) ---
        let delivery = Arc::new(DeliveryManager::new(
            env_scope,
            (*event_bus).clone(),
            store.clone(),
            vec![Arc::new(TestSinkAdapter::new())],
        ));

        // --- workflow launcher shared by scheduler/reminders/webhooks ---
        let workflow_launcher = Arc::new(adapters::WorkflowLauncherImpl::new(runtime.clone()));

        // --- scheduler ---
        let scheduler = Arc::new(Scheduler::new(SchedulerDependencies {
            environment: cfg.environment,
            runtime: runtime.clone(),
            event_bus: Some((*event_bus).clone()),
            store: store.clone(),
            workflow_launcher: Some(workflow_launcher.clone()),
            clock: None,
            tick_interval: Duration::ZERO,
        }));

        // --- reminders ---
        let reminders = Arc::new(RemindersManager::new(RemindersDependencies {
            environment_scope: env_scope.to_string(),
            store: store.clone(),
            event_bus: Some((*event_bus).clone()),
            delivery: Some((*delivery).clone()),
            workflow_launcher: Some(workflow_launcher.clone()),
            clock: None,
            tick_interval: Duration::ZERO,
        }));

        // --- routines (compiled onto the scheduler through the adapter) ---
        let mut routine_manager = dope_routine::Manager::new(
            env_scope,
            Box::new(adapters::RoutineSchedulerAdapter::new(scheduler.clone())),
        );
        routine_manager.with_store(store.clone());
        let routines = Arc::new(routine_manager);

        // --- triage ---
        let mut triage_manager = TriageManager::new(env_scope);
        triage_manager.with_store(store.clone());
        let triage = Arc::new(triage_manager);

        // --- webhooks (firer launches scheduled workflows) ---
        let mut webhook_manager = WebhookManager::new(
            env_scope,
            Some(Box::new(adapters::WebhookFirerImpl::new(workflow_launcher))),
            None,
        );
        webhook_manager.with_store(store.clone());
        let webhooks = Arc::new(webhook_manager);

        // --- catalog / exec profiles / evidence (default hooks; the Go
        // sandbox-health / evidence-collector projections are deferred) ---
        // TODO(app wiring): execprofile HealthChecker + evidence Collector
        // adapters are not ported; default all-pass hooks apply.
        let mut catalog_manager = dope_catalog::Manager::new(env_scope, None, None);
        catalog_manager.with_store(store.clone());
        let catalog = Arc::new(catalog_manager);
        let mut exec_profile_manager = ExecProfileManager::new(env_scope, None, None, None);
        exec_profile_manager.with_store(store.clone());
        let exec_profiles = Arc::new(exec_profile_manager);
        let mut evidence_manager = dope_evidence::Manager::new(env_scope, None, None);
        evidence_manager.with_store(store.clone());
        let evidence = Arc::new(evidence_manager);

        // --- evaluation (store handle missing in dope-store -> None) ---
        // TODO(app wiring): dope_evaluation::Store impl over SQLiteStore is
        // not present in dope-store; the manager runs without persistence.
        let evaluation = Arc::new(EvaluationManager::new(EvaluationDependencies {
            environment_scope: env_scope.to_string(),
            store: None,
            fixtures_dir: String::new(),
            runtime_recorder: None,
            billing: Some(billing.clone()),
            hosted_billing: hosted,
            clock: None,
        }));

        // --- live validation (async Store not implementable outside the
        // crate -> None) ---
        // TODO(app wiring): dope_livevalidation::Store cannot be implemented
        // outside the crate (LedgerOutcome is not re-exported); see api tests.
        let live_validation = Arc::new(LiveValidationManager::new(LiveValidationDependencies {
            environment_scope: env_scope.to_string(),
            store: None,
            enabled: true,
            billing: Some(billing.clone()),
            hosted_billing: hosted,
            clock: None,
            ledger_event_sink: None,
            candidate_tool_class_resolver: None,
        }));

        // --- setup wizard (MemoryStore default; no dope-store impl) ---
        // TODO(app wiring): dope_setupwizard::Store impl over SQLiteStore is
        // missing; the in-memory store is used.
        let setup_wizard = Arc::new(new_setup_wizard(SetupWizardDependencies {
            secrets: Some(secret_manager.clone()),
            ..SetupWizardDependencies::default()
        }));

        // --- audit emitter ---
        let audit_emitter = Arc::new(AuditEmitter::new(event_bus.clone()));

        // --- API state ---
        let mut state = AppState::new(cfg.clone(), event_bus.clone(), store.clone());
        state.policy = Some(policy_engine.clone());
        state.auth = Some(auth_manager.clone());
        state.identity = Some(identity_manager.clone());
        state.router = Some(session_router.clone());
        state.runtime = Some(runtime.clone());
        state.llm = Some(llm.clone());
        state.chat = Some(chat.clone());
        state.providers = Some(providers.clone());
        state.skills = Some(skills.clone());
        state.sandboxes = Some(sandboxes.clone());
        state.secrets = Some(secret_manager.clone());
        state.mcp = Some(mcp.clone());
        state.integrations = Some(integrations.clone());
        state.calendar = Some(calendar.clone());
        state.mail = Some(mail.clone());
        state.reminders = Some(reminders.clone());
        state.triage = Some(triage.clone());
        state.routines = Some(routines.clone());
        state.webhooks = Some(webhooks.clone());
        state.catalog = Some(catalog.clone());
        state.exec_profiles = Some(exec_profiles.clone());
        state.evidence = Some(evidence.clone());
        state.connectors = Some(connectors.clone());
        state.capabilities = Some(capabilities.clone());
        state.computer_use = Some(computer_use.clone());
        state.scheduler = Some(scheduler.clone());
        state.delivery = Some(delivery.clone());
        state.billing = Some(billing.clone());
        state.activation = Some(activation.clone());
        state.setup_wizard = Some(setup_wizard.clone());
        state.checkpoints = Some(checkpoints.clone());
        state.evaluation = Some(evaluation.clone());
        state.live_validation = Some(live_validation.clone());
        state.audit_emitter = Some(audit_emitter.clone());
        state.tenant_migration_status = Some(Arc::new(adapters::NoMigrationInProgress));

        Ok(App {
            config: cfg,
            state,
            event_bus,
            sandboxes: Some(sandboxes),
            scheduler: Some(scheduler),
            reminders: Some(reminders),
            closed: AtomicBool::new(false),
        })
    }

    /// Loads the effective config (env + config.json) and builds the app.
    /// Port of Go `app.New()` (config.Load inside).
    pub fn from_env() -> Result<Self, AppError> {
        Self::new(dope_config::load()?)
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
        if let Some(scheduler) = &self.scheduler {
            if let Err(err) = scheduler.start() {
                eprintln!("[dope] scheduler start failed: {err}");
            }
        }
        if let Some(reminders) = &self.reminders {
            if let Err(err) = reminders.start() {
                eprintln!("[dope] reminders start failed: {err}");
            }
        }
    }

    /// Stops background loops and closes the bus (port of Go `App.Close`).
    /// Idempotent.
    pub fn close(&self) {
        if self.closed.swap(true, Ordering::SeqCst) {
            return;
        }
        if let Some(scheduler) = &self.scheduler {
            let _ = scheduler.close();
        }
        if let Some(reminders) = &self.reminders {
            reminders.close();
        }
        if let Some(sandboxes) = &self.sandboxes {
            let _ = sandboxes.close();
        }
        self.event_bus.close();
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
            55
        );
        app2.close();
    }
}
