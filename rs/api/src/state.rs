//! Application state shared by every route family.
//!
//! Port of Go's api.Dependencies struct (daemon/internal/api/server.go).
//! Every manager is an Option<Arc<T>> so the foundation compiles and serves
//! health/version/system-info even before app wiring populates the managers;
//! route families read their manager lazily and return 503/500 when the
//! manager they need is absent. store/config/event_bus are required
//! because every route family and the middleware layer need them.

use std::sync::Arc;

use parking_lot::Mutex;

use dope_activation::Service as ActivationService;
use dope_billing::Manager as BillingManager;
use dope_calendar::Manager as CalendarManager;
use dope_capabilities::Supervisor as CapabilitiesSupervisor;
use dope_computeruse::Manager as ComputerUseManager;
use dope_config::Config;
use dope_connectors::Supervisor as ConnectorsSupervisor;
use dope_delivery::Manager as DeliveryManager;
use dope_evaluation::Manager as EvaluationManager;
use dope_events::Bus;
use dope_identity::auth::Manager as AuthManager;
use dope_identity::{Manager as IdentityManager, Store as IdentityStore};
use dope_integrations::Manager as IntegrationsManager;
use dope_livevalidation::Manager as LiveValidationManager;
use dope_llm::Dispatcher;
use dope_mail::Manager as MailManager;
use dope_mcp::Manager as McpManager;
use dope_policy::Engine;
use dope_providers::Manager as ProvidersManager;
use dope_router::SessionRouter;
use dope_runtime::Manager as RuntimeManager;
use dope_sandbox::Manager as SandboxManager;
use dope_secrets::Manager as SecretsManager;
use dope_setupwizard::Service as SetupWizardService;
use dope_skills::Registry;
use dope_store::SQLiteStore;

/// Read-only view of the tenant-backfill migration gate the API needs.
/// Port of Go's api.MigrationStatus interface; the app layer supplies an
/// implementation wrapping the migration runner.
pub trait MigrationStatus: Send + Sync {
    fn in_progress(&self) -> bool;
    fn pending_steps(&self) -> Vec<String>;
}

/// Shared application state, mirroring Go's api.Dependencies field set.
///
/// logger is intentionally skipped (the Go field is *slog.Logger; the Rust
/// surface has no equivalent yet — telemetry is out of scope for this wave).
/// reminders is a placeholder because the dope-reminders crate has not been
/// ported yet (see MISSING-MANAGERS note below).
#[derive(Clone)]
pub struct AppState {
    /// Go Dependencies.Config.
    pub config: Config,
    /// Go Dependencies.EventBus.
    pub event_bus: Arc<Bus>,
    /// Go Dependencies.Policy.
    pub policy: Option<Arc<Engine>>,
    /// Go Dependencies.Auth (pairing + access-token lifecycle).
    pub auth: Option<Arc<AuthManager>>,
    /// Go Dependencies.Identity (tenant/principal resolution). The store is
    /// erased behind the object-safe Store trait so the manager can be shared.
    pub identity: Option<Arc<IdentityManager<dyn IdentityStore + Send + Sync>>>,
    /// Go Dependencies.Router.
    pub router: Option<Arc<SessionRouter>>,
    /// Go Dependencies.Runtime.
    pub runtime: Option<Arc<RuntimeManager>>,
    /// Go Dependencies.LLM.
    pub llm: Option<Arc<Dispatcher>>,
    /// Go Dependencies.Chat.
    ///
    /// TODO: PLACEHOLDER — `dope_chat::Service` holds `Arc<dyn ChatStore>`
    /// (no `Send + Sync` supertraits), so it cannot live in axum `State`
    /// (requires `Send + Sync`). Real type returns once `dyn ChatStore` is
    /// `+ Send + Sync` or the service is wrapped.
    pub chat: Option<Arc<()>>,
    /// Go Dependencies.Providers.
    pub providers: Option<Arc<ProvidersManager>>,
    /// Go Dependencies.Skills.
    pub skills: Option<Arc<Registry>>,
    /// Go Dependencies.Sandboxes.
    pub sandboxes: Option<Arc<SandboxManager>>,
    /// Go Dependencies.Secrets.
    pub secrets: Option<Arc<SecretsManager>>,
    /// Go Dependencies.MCP.
    pub mcp: Option<Arc<McpManager>>,
    /// Go Dependencies.Integrations.
    pub integrations: Option<Arc<IntegrationsManager>>,
    /// Go Dependencies.Calendar.
    pub calendar: Option<Arc<CalendarManager>>,
    /// Go Dependencies.Mail.
    pub mail: Option<Arc<MailManager>>,
    /// Go Dependencies.Reminders.
    ///
    /// TODO: MISSING MANAGER — the dope-reminders crate does not exist in the
    /// rs/ workspace (no rs/reminders/ directory despite MIGRATION.md wave 6
    /// listing reminders(1326) as done; dope-reminders is declared in the
    /// workspace but resolves to nothing). Reminders DTOs in types.rs use
    /// serde_json::Value placeholders until the crate lands.
    pub reminders: Option<Arc<()>>,
    /// Go Dependencies.Triage.
    ///
    /// TODO: PLACEHOLDER — `dope_triage::Manager<'a>` holds `Option<&SQLiteStore>`
    /// (a `!Sync` reference), so it cannot live in axum `State`. Real type
    /// returns once the manager owns its store handle thread-safely.
    pub triage: Option<Arc<()>>,
    /// Go Dependencies.Routines.
    ///
    /// TODO: PLACEHOLDER — `dope_routine::Manager<'a>` holds `Box<dyn Scheduler>`
    /// (no `Send + Sync` supertraits), so it cannot live in axum `State`.
    pub routines: Option<Arc<()>>,
    /// Go Dependencies.Webhooks.
    ///
    /// TODO: PLACEHOLDER — `dope_webhook::Manager<'a>` holds `Box<dyn Firer>` /
    /// `Box<dyn QuotaGate>` (no `Send + Sync` supertraits), so it cannot live
    /// in axum `State`.
    pub webhooks: Option<Arc<()>>,
    /// Go Dependencies.Catalog.
    ///
    /// TODO: PLACEHOLDER — `dope_catalog::Manager<'a>` holds `Box<dyn
    /// RequirementChecker>` / `Box<dyn PermissionGate>` (no `Send + Sync`
    /// supertraits), so it cannot live in axum `State`.
    pub catalog: Option<Arc<()>>,
    /// Go Dependencies.ExecProfiles.
    ///
    /// TODO: PLACEHOLDER — `dope_execprofile::Manager<'a>` holds `Box<dyn
    /// HealthChecker>` / `Box<dyn RequirementChecker>` / `Box<dyn
    /// PermissionGate>` (no `Send + Sync` supertraits), so it cannot live in
    /// axum `State`.
    pub exec_profiles: Option<Arc<()>>,
    /// Go Dependencies.Evidence.
    ///
    /// TODO: PLACEHOLDER — `dope_evidence::Manager<'a>` holds `Box<dyn
    /// Collector>` / `Box<dyn PermissionGate>` (no `Send + Sync` supertraits),
    /// so it cannot live in axum `State`.
    pub evidence: Option<Arc<()>>,
    /// Go Dependencies.Connectors.
    pub connectors: Option<Arc<ConnectorsSupervisor>>,
    /// Go Dependencies.Capabilities.
    pub capabilities: Option<Arc<CapabilitiesSupervisor>>,
    /// Go Dependencies.ComputerUse.
    pub computer_use: Option<Arc<ComputerUseManager>>,
    /// Go Dependencies.Scheduler.
    ///
    /// TODO: PLACEHOLDER — `dope_scheduler::Scheduler` holds `Box<dyn Clock>`
    /// and `Option<Arc<dyn WorkflowLauncher>>` (no `Send + Sync` supertraits),
    /// so it cannot live in axum `State`. Real type returns once those trait
    /// objects are `+ Send + Sync`.
    pub scheduler: Option<Arc<()>>,
    /// Go Dependencies.Delivery.
    pub delivery: Option<Arc<DeliveryManager>>,
    /// Go Dependencies.Billing.
    pub billing: Option<Arc<BillingManager>>,
    /// Go Dependencies.Activation.
    pub activation: Option<Arc<ActivationService>>,
    /// Go Dependencies.SetupWizard.
    pub setup_wizard: Option<Arc<SetupWizardService>>,
    /// Go Dependencies.Store.
    ///
    /// Wrapped in a mutex because rusqlite `Connection` is `!Sync`; the Go
    /// daemon shares the store across goroutines behind its own lock, so the
    /// mutex is the Rust equivalent.
    pub store: Arc<Mutex<SQLiteStore>>,
    /// Go Dependencies.Checkpoints.
    ///
    /// TODO: PLACEHOLDER — `dope_checkpoints::Manager` holds `Arc<SQLiteStore>`
    /// (rusqlite `Connection` is `!Sync`), so it cannot live in axum `State`
    /// while it owns its own store handle; the API-layer store is mutex-wrapped.
    pub checkpoints: Option<Arc<()>>,
    /// Go Dependencies.Evaluation.
    pub evaluation: Option<Arc<EvaluationManager>>,
    /// Go Dependencies.LiveValidation.
    pub live_validation: Option<Arc<LiveValidationManager>>,
    /// Go Dependencies.AuditEmitter (emits audit.cross_tenant_access_denied).
    pub audit_emitter: Option<Arc<dope_audit::Emitter>>,
    /// Go Dependencies.TenantMigrationStatus. None behaves as if all
    /// backfills are complete.
    pub tenant_migration_status: Option<Arc<dyn MigrationStatus>>,
}

impl AppState {
    /// Builds a state with only the required core (config, event_bus,
    /// store) populated; every manager is None.
    #[must_use]
    pub fn new(config: Config, event_bus: Arc<Bus>, store: Arc<Mutex<SQLiteStore>>) -> Self {
        Self {
            config,
            event_bus,
            policy: None,
            auth: None,
            identity: None,
            router: None,
            runtime: None,
            llm: None,
            chat: None,
            providers: None,
            skills: None,
            sandboxes: None,
            secrets: None,
            mcp: None,
            integrations: None,
            calendar: None,
            mail: None,
            reminders: None,
            triage: None,
            routines: None,
            webhooks: None,
            catalog: None,
            exec_profiles: None,
            evidence: None,
            connectors: None,
            capabilities: None,
            computer_use: None,
            scheduler: None,
            delivery: None,
            billing: None,
            activation: None,
            setup_wizard: None,
            store,
            checkpoints: None,
            evaluation: None,
            live_validation: None,
            audit_emitter: None,
            tenant_migration_status: None,
        }
    }
}

// NOTE: no Default impl. Config has no Default (environment is required) and
// SQLiteStore::new needs a real data dir, so a Default would have to hide an
// unwrap. Tests build state explicitly through AppState::new with a temp-dir
// store (see routes.rs tests).
