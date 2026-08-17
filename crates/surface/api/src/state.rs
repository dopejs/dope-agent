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
use dope_catalog::Manager as CatalogManager;
use dope_chat::Service as ChatService;
use dope_checkpoints::Manager as CheckpointsManager;
use dope_evidence::Manager as EvidenceManager;
use dope_execprofile::Manager as ExecProfileManager;
use dope_reminders::Manager as RemindersManager;
use dope_routine::Manager as RoutineManager;
use dope_scheduler::Scheduler;
use dope_memory::Manager as MemoryManager;
use dope_triage::Manager as TriageManager;
use dope_webhook::Manager as WebhookManager;

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
    pub chat: Option<Arc<ChatService>>,
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
    pub reminders: Option<Arc<RemindersManager>>,
    /// Go Dependencies.Triage.
    ///
    pub triage: Option<Arc<TriageManager>>,
    /// Memory plane manager (Roadmap 78, spec 058).
    pub memory: Option<Arc<MemoryManager>>,
    /// Go Dependencies.Routines.
    ///
    pub routines: Option<Arc<RoutineManager>>,
    /// Go Dependencies.Webhooks.
    ///
    pub webhooks: Option<Arc<WebhookManager>>,
    /// Go Dependencies.Catalog.
    ///
    pub catalog: Option<Arc<CatalogManager>>,
    /// Go Dependencies.ExecProfiles.
    ///
    pub exec_profiles: Option<Arc<ExecProfileManager>>,
    /// Go Dependencies.Evidence.
    ///
    pub evidence: Option<Arc<EvidenceManager>>,
    /// Go Dependencies.Connectors.
    pub connectors: Option<Arc<ConnectorsSupervisor>>,
    /// Go Dependencies.Capabilities.
    pub capabilities: Option<Arc<CapabilitiesSupervisor>>,
    /// Go Dependencies.ComputerUse.
    pub computer_use: Option<Arc<ComputerUseManager>>,
    /// Go Dependencies.Scheduler.
    ///
    pub scheduler: Option<Arc<Scheduler>>,
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
    pub checkpoints: Option<Arc<CheckpointsManager>>,
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
            memory: None,
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
