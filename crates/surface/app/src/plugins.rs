//! Builtin plugin definitions: every non-kernel subsystem of the daemon,
//! expressed as a [`BuiltinPlugin`] with a declared dependency edge set and a
//! build function over the shared [`Assembly`].
//!
//! The kernel (store, event bus, session router, runtime, checkpoints,
//! policy, auth, identity, secrets, audit — the trust boundary) is built in
//! `App::with_profile` before any plugin runs; it is deliberately *not* a
//! plugin and cannot be disabled. Everything else assembles here in declared
//! order. Disabling a plugin leaves its `AppState` field `None`, which the
//! API layer already answers with not-wired errors; dependents are
//! transitively disabled by `dope_plugin::resolve`.
//!
//! The channel plugins (`channel-*`) build nothing at assembly time — their
//! runtimes are constructed in `App::serve` — but their enablement gates the
//! runtime construction and their `requires` edges keep them honest about
//! the managers the message loop dereferences (runtime, chat).

use std::path::Path;
use std::sync::Arc;
use std::time::Duration;

use dope_activation::{
    BillingProjectorAdapter as ActivationBillingProjectorAdapter,
    ChatRunnerAdapter as ActivationChatRunnerAdapter, Service as ActivationService,
    SqliteActivationStore,
};
use dope_api::AppState;
use dope_billing::Manager as BillingManager;
use dope_calendar::Manager as CalendarManager;
use dope_capabilities::Supervisor as CapabilitiesSupervisor;
use dope_chat::{ChatStore, Service as ChatService};
use dope_computeruse::{
    Dependencies as ComputerUseDependencies, Manager as ComputerUseManager, SqliteArtifactRecorder,
};
use dope_config::Config;
use dope_connectors::Supervisor as ConnectorsSupervisor;
use dope_delivery::{ConnectorAdapter, Manager as DeliveryManager, TestSinkAdapter};
use dope_evaluation::{Dependencies as EvaluationDependencies, Manager as EvaluationManager};
use dope_events::Bus;
use dope_evidence::{Manager as EvidenceManager, RoutineCollector};
use dope_execprofile::{Manager as ExecProfileManager, SandboxHealthChecker};
use dope_integrations::Manager as IntegrationsManager;
use dope_livevalidation::{
    Dependencies as LiveValidationDependencies, Manager as LiveValidationManager,
};
use dope_llm::Dispatcher;
use dope_mail::Manager as MailManager;
use dope_mcp::Manager as McpManager;
use dope_memory::Manager as MemoryManager;
use dope_plugin::{PluginDescriptor, SeamMap};
use dope_policy::Engine as PolicyEngine;
use dope_reminders::{Dependencies as RemindersDependencies, Manager as RemindersManager};
use dope_sandbox::Manager as SandboxManager;
use dope_scheduler::{Dependencies as SchedulerDependencies, Scheduler};
use dope_secrets::Manager as SecretsManager;
use dope_setupwizard::{
    ServiceDependencies as SetupWizardDependencies, new_service as new_setup_wizard,
};
use dope_skills::Registry as SkillsRegistry;
use dope_store::{
    BillingRepositoryHandle, ComputerUseStoreHandle, EvaluationStoreHandle,
    LiveValidationStoreHandle, SQLiteStore, SecretStoreHandle, SetupWizardStoreHandle,
};
use dope_triage::Manager as TriageManager;
use dope_webhook::Manager as WebhookManager;

use crate::adapters;
use crate::AppError;

// ---------------------------------------------------------------------------
// Seams shared between the kernel and plugins during assembly
// ---------------------------------------------------------------------------

/// Managed-provider registry, provided by `llm`, consumed by `providers`.
#[derive(Clone)]
pub(crate) struct ManagedRegistrySeam(pub Arc<dyn dope_providers::ManagedRegistry>);

/// Secret metadata store handle (kernel-provided; `sandbox` builds its own
/// secret-manager instance over it because `set_secret_manager` takes
/// ownership).
#[derive(Clone)]
pub(crate) struct SecretStoreSeam(pub Arc<SecretStoreHandle>);

/// Secret value backend (kernel-provided, same consumer as the store seam).
#[derive(Clone)]
pub(crate) struct SecretBackendSeam(pub Arc<dope_secrets::LocalBackend>);

/// Workflow launcher over the runtime manager (kernel-provided; consumed by
/// `scheduler` and `webhooks`).
#[derive(Clone)]
pub(crate) struct WorkflowLauncherSeam(pub Arc<adapters::WorkflowLauncherImpl>);

// ---------------------------------------------------------------------------
// Assembly context
// ---------------------------------------------------------------------------

/// Mutable assembly context threaded through the plugin build functions.
/// Kernel-built handles live as named fields; cross-plugin intermediates go
/// through [`SeamMap`]; managers land on `state` (the same `Option` fields
/// the API reads).
pub(crate) struct Assembly {
    pub cfg: Config,
    pub env_scope: &'static str,
    pub hosted: bool,
    pub store: Arc<parking_lot::Mutex<SQLiteStore>>,
    /// std::sync::Mutex handle required by the sandbox/mcp/chat constructors.
    pub secondary: Arc<std::sync::Mutex<SQLiteStore>>,
    pub event_bus: Arc<Bus>,
    pub state: AppState,
    pub seams: SeamMap,
    #[cfg(test)]
    pub wiring: crate::AppWiring,
}

impl Assembly {
    /// Opens an additional connection to the shared WAL database (the
    /// pattern the store-backed handles use).
    fn open_store(&self) -> Result<SQLiteStore, AppError> {
        SQLiteStore::new(&self.cfg.data_dir).map_err(AppError::Store)
    }
}

/// One builtin plugin: static descriptor + build function.
pub(crate) struct BuiltinPlugin {
    pub descriptor: PluginDescriptor,
    pub build: fn(&mut Assembly) -> Result<(), AppError>,
}

/// The builtin plugin set in build order (dependencies before dependents,
/// matching the pre-pluginization `App::new` construction order).
pub(crate) const BUILTINS: &[BuiltinPlugin] = &[
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "llm",
            summary: "LLM dispatcher with managed CLI providers and the echo fallback",
            provides: &["llm.dispatcher", "llm.managed-registry"],
            requires: &[],
        },
        build: build_llm,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "skills",
            summary: "Skill registry over <data_dir>/skills",
            provides: &["skills.registry"],
            requires: &[],
        },
        build: build_skills,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "sandbox",
            summary: "Sandboxed execution plane",
            provides: &["sandbox.manager"],
            requires: &[],
        },
        build: build_sandbox,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "mcp",
            summary: "MCP server registry and attached executions",
            provides: &["mcp.manager"],
            requires: &["sandbox"],
        },
        build: build_mcp,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "integrations",
            summary: "Integration account registry (adapter RPC plane)",
            provides: &["integrations.manager"],
            requires: &[],
        },
        build: build_integrations,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "calendar",
            summary: "Calendar accounts, events and scheduling intents",
            provides: &["calendar.manager"],
            requires: &[],
        },
        build: build_calendar,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "mail",
            summary: "Mail accounts, messages and drafts",
            provides: &["mail.manager"],
            requires: &[],
        },
        build: build_mail,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "providers",
            summary: "Provider registry (managed auth, models, checks)",
            provides: &["providers.manager"],
            requires: &["llm"],
        },
        build: build_providers,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "connectors",
            summary: "Channel connector supervisor",
            provides: &["connectors.supervisor"],
            requires: &[],
        },
        build: build_connectors,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "capabilities",
            summary: "Supervised capability process registry",
            provides: &["capabilities.supervisor"],
            requires: &[],
        },
        build: build_capabilities,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "chat",
            summary: "Chat query service over the LLM dispatcher",
            provides: &["chat.service"],
            requires: &["llm"],
        },
        build: build_chat,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "billing",
            summary: "Billing plans, usage ledgers and quota reservations",
            provides: &["billing.manager"],
            requires: &[],
        },
        build: build_billing,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "activation",
            summary: "Activation/onboarding state machine",
            provides: &["activation.service"],
            requires: &["billing"],
        },
        build: build_activation,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "computer-use",
            summary: "Computer-use sessions with artifact recording",
            provides: &["computeruse.manager"],
            requires: &[],
        },
        build: build_computer_use,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "delivery",
            summary: "Outbound delivery with connector and test sinks",
            provides: &["delivery.manager"],
            requires: &[],
        },
        build: build_delivery,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "scheduler",
            summary: "Scheduled workflow launches",
            provides: &["scheduler"],
            requires: &[],
        },
        build: build_scheduler,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "reminders",
            summary: "Reminders with delivery escalation",
            provides: &["reminders.manager"],
            requires: &[],
        },
        build: build_reminders,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "routines",
            summary: "Routines compiled onto the scheduler",
            provides: &["routines.manager"],
            requires: &["scheduler"],
        },
        build: build_routines,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "memory",
            summary: "Layered memory plane (L0-L3) with LLM consolidation",
            provides: &["memory.manager"],
            requires: &["llm"],
        },
        build: build_memory,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "triage",
            summary: "Inbound triage queue",
            provides: &["triage.manager"],
            requires: &[],
        },
        build: build_triage,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "webhooks",
            summary: "Webhook ingress with quota-gated workflow launches",
            provides: &["webhooks.manager"],
            // The quota gate is billing-backed and fail-closed (Roadmap 75);
            // running webhooks without billing would drop that enforcement.
            requires: &["billing"],
        },
        build: build_webhooks,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "catalog",
            summary: "Install catalog (skills, MCP servers, capabilities)",
            provides: &["catalog.manager"],
            requires: &["sandbox"],
        },
        build: build_catalog,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "exec-profiles",
            summary: "Execution profiles with sandbox-backed health checks",
            provides: &["execprofile.manager"],
            requires: &["sandbox"],
        },
        build: build_exec_profiles,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "evidence",
            summary: "Support evidence bundles",
            provides: &["evidence.manager"],
            requires: &["routines"],
        },
        build: build_evidence,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "evaluation",
            summary: "Evaluation harness with billing enforcement",
            provides: &["evaluation.manager"],
            requires: &["billing"],
        },
        build: build_evaluation,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "live-validation",
            summary: "Live validation ledger with billing enforcement",
            provides: &["livevalidation.manager"],
            requires: &["billing"],
        },
        build: build_live_validation,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "setup-wizard",
            summary: "First-run setup wizard",
            provides: &["setupwizard.service"],
            requires: &[],
        },
        build: build_setup_wizard,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "channel-discord",
            summary: "Discord channel runtime (built at serve time)",
            provides: &["channel.discord"],
            requires: &["connectors", "chat"],
        },
        build: build_channel_noop,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "channel-telegram",
            summary: "Telegram channel runtime (built at serve time)",
            provides: &["channel.telegram"],
            requires: &["connectors", "chat"],
        },
        build: build_channel_noop,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "channel-slack",
            summary: "Slack channel runtime (built at serve time)",
            provides: &["channel.slack"],
            requires: &["connectors", "chat"],
        },
        build: build_channel_noop,
    },
    BuiltinPlugin {
        descriptor: PluginDescriptor {
            id: "channel-matrix",
            summary: "Matrix channel runtime (built at serve time)",
            provides: &["channel.matrix"],
            requires: &["connectors", "chat"],
        },
        build: build_channel_noop,
    },
];

/// The descriptor list in build order, for `dope_plugin::resolve`.
pub(crate) fn descriptors() -> Vec<PluginDescriptor> {
    BUILTINS.iter().map(|plugin| plugin.descriptor).collect()
}

// ---------------------------------------------------------------------------
// Build functions (each a verbatim port of its pre-pluginization App::new
// block; comments carried over where they record decisions)
// ---------------------------------------------------------------------------

fn build_llm(asm: &mut Assembly) -> Result<(), AppError> {
    let llm = Arc::new(Dispatcher::new());
    if asm.cfg.llm.default_timeout_ms > 0 {
        llm.set_default_timeout(Duration::from_millis(asm.cfg.llm.default_timeout_ms as u64));
    }
    if asm.cfg.llm.default_max_retries > 0 {
        llm.set_default_retries(asm.cfg.llm.default_max_retries);
    }
    if !asm.cfg.llm.default_model.trim().is_empty() {
        llm.set_default_model(&asm.cfg.llm.default_model);
    }
    let managed_registry: Arc<dyn dope_providers::ManagedRegistry> =
        Arc::new(dope_managedproviders::Registry::new(&asm.cfg, None));
    for bridge in managed_registry.list() {
        llm.register_provider(bridge.provider());
    }
    // Deterministic in-process fallback so the daemon always has a default
    // provider (Go registers echo in dispatcher.go).
    llm.register_provider(Arc::new(dope_llm::EchoProvider::new()));
    let default_provider = if asm.cfg.llm.default_provider.trim().is_empty() {
        "echo".to_string()
    } else {
        asm.cfg.llm.default_provider.clone()
    };
    let _ = llm.set_default_provider(&default_provider);

    asm.seams.put(ManagedRegistrySeam(managed_registry));
    asm.state.llm = Some(llm);
    Ok(())
}

fn build_skills(asm: &mut Assembly) -> Result<(), AppError> {
    let skills = Arc::new(
        SkillsRegistry::new(&asm.cfg.data_dir).map_err(|err| AppError::Skills(err.to_string()))?,
    );
    asm.state.skills = Some(skills);
    Ok(())
}

fn build_sandbox(asm: &mut Assembly) -> Result<(), AppError> {
    let sandboxes = Arc::new(SandboxManager::new(
        asm.cfg.clone(),
        Some(asm.secondary.clone()),
        (*asm.event_bus).clone(),
        PolicyEngine::new(),
    ));
    // The sandbox secret manager is a second instance sharing the same
    // store/backend because set_secret_manager takes ownership.
    let secret_store = asm.seams.get::<SecretStoreSeam>().expect("kernel secret store").0;
    let secret_backend = asm.seams.get::<SecretBackendSeam>().expect("kernel secret backend").0;
    sandboxes.set_secret_manager(SecretsManager::new(secret_store, secret_backend));
    asm.state.sandboxes = Some(sandboxes);
    Ok(())
}

fn build_mcp(asm: &mut Assembly) -> Result<(), AppError> {
    let sandboxes = asm.state.sandboxes.clone().expect("sandbox plugin built");
    let secret_manager = asm.state.secrets.clone().expect("kernel secrets");
    let mcp_starter = Arc::new(adapters::McpExecutionStarter::new(sandboxes));
    let mcp_secret_resolver =
        Arc::new(adapters::McpSecretResolver::new(asm.store.clone(), secret_manager));
    let mcp = Arc::new(McpManager::new(
        asm.cfg.clone(),
        Some(asm.secondary.clone()),
        Some((*asm.event_bus).clone()),
        Some(mcp_starter.clone()),
        Some(PolicyEngine::new()),
        None, // concrete MCP transports attach lazily (restore path)
    ));
    mcp.set_secret_manager(mcp_secret_resolver.clone());
    asm.state.mcp = Some(mcp);
    #[cfg(test)]
    {
        asm.wiring.mcp_starter = Some(mcp_starter);
        asm.wiring.mcp_secret_resolver = Some(mcp_secret_resolver);
    }
    Ok(())
}

fn build_integrations(asm: &mut Assembly) -> Result<(), AppError> {
    asm.state.integrations = Some(Arc::new(IntegrationsManager::new(asm.env_scope)));
    Ok(())
}

fn build_calendar(asm: &mut Assembly) -> Result<(), AppError> {
    asm.state.calendar = Some(Arc::new(CalendarManager::new(asm.env_scope)));
    Ok(())
}

fn build_mail(asm: &mut Assembly) -> Result<(), AppError> {
    asm.state.mail = Some(Arc::new(MailManager::new(asm.env_scope)));
    Ok(())
}

fn build_providers(asm: &mut Assembly) -> Result<(), AppError> {
    let llm = asm.state.llm.clone().expect("llm plugin built");
    let managed_registry = asm.seams.get::<ManagedRegistrySeam>().expect("llm registry seam").0;
    asm.state.providers = Some(Arc::new(dope_providers::new_manager(
        asm.cfg.llm.clone(),
        Some(llm),
        vec![managed_registry],
    )));
    Ok(())
}

fn build_connectors(asm: &mut Assembly) -> Result<(), AppError> {
    asm.state.connectors = Some(Arc::new(ConnectorsSupervisor::new()));
    Ok(())
}

fn build_capabilities(asm: &mut Assembly) -> Result<(), AppError> {
    asm.state.capabilities = Some(Arc::new(CapabilitiesSupervisor::new()));
    Ok(())
}

fn build_chat(asm: &mut Assembly) -> Result<(), AppError> {
    let llm = asm.state.llm.clone().expect("llm plugin built");
    let chat = Arc::new(ChatService::new_service(
        llm,
        asm.state.providers.clone(),
        asm.state.skills.clone(),
        Some((*asm.event_bus).clone()),
        Some(asm.secondary.clone() as Arc<dyn ChatStore>),
    ));
    asm.state.chat = Some(chat);
    Ok(())
}

fn build_billing(asm: &mut Assembly) -> Result<(), AppError> {
    let billing_repo = Arc::new(BillingRepositoryHandle::new(asm.open_store()?));
    asm.state.billing = Some(Arc::new(BillingManager::new(billing_repo)));
    Ok(())
}

fn build_activation(asm: &mut Assembly) -> Result<(), AppError> {
    let billing = asm.state.billing.clone().expect("billing plugin built");
    let activation_store = Arc::new(
        SqliteActivationStore::new(asm.open_store()?).map_err(AppError::Store)?,
    );
    let activation_billing = Arc::new(ActivationBillingProjectorAdapter::new(billing));
    let activation_chat = Arc::new(ActivationChatRunnerAdapter::new(asm.state.chat.clone()));
    asm.state.activation = Some(Arc::new(ActivationService::with_sqlite(
        activation_store.clone(),
        Some(activation_billing.clone()),
        Some(activation_chat.clone()),
        asm.env_scope,
        asm.hosted,
    )));
    #[cfg(test)]
    {
        asm.wiring.activation_store = Some(activation_store);
        asm.wiring.activation_billing = Some(activation_billing);
        asm.wiring.activation_chat = Some(activation_chat);
    }
    Ok(())
}

fn build_computer_use(asm: &mut Assembly) -> Result<(), AppError> {
    let computeruse_store = Arc::new(ComputerUseStoreHandle::new(asm.open_store()?));
    let computeruse_recorder = Arc::new(SqliteArtifactRecorder::new(
        computeruse_store.clone() as Arc<dyn dope_computeruse::Store>,
        &asm.cfg.data_dir,
        asm.env_scope,
    ));
    asm.state.computer_use = Some(Arc::new(ComputerUseManager::new(ComputerUseDependencies {
        environment_scope: asm.env_scope.to_string(),
        runtime: asm.state.runtime.clone(),
        policy: asm.state.policy.clone(),
        store: computeruse_store,
        driver: None,
        artifacts: Some(computeruse_recorder.clone()),
    })));
    #[cfg(test)]
    {
        asm.wiring.computeruse_recorder = Some(computeruse_recorder);
    }
    Ok(())
}

fn build_delivery(asm: &mut Assembly) -> Result<(), AppError> {
    let delivery_connector = Arc::new(ConnectorAdapter::new(asm.store.clone()));
    asm.state.delivery = Some(Arc::new(DeliveryManager::new(
        asm.env_scope,
        (*asm.event_bus).clone(),
        asm.store.clone(),
        vec![Arc::new(TestSinkAdapter::new()), delivery_connector.clone()],
    )));
    #[cfg(test)]
    {
        asm.wiring.delivery_connector = Some(delivery_connector);
    }
    Ok(())
}

fn build_scheduler(asm: &mut Assembly) -> Result<(), AppError> {
    let runtime = asm.state.runtime.clone().expect("kernel runtime");
    let workflow_launcher = asm.seams.get::<WorkflowLauncherSeam>().expect("kernel launcher").0;
    asm.state.scheduler = Some(Arc::new(Scheduler::new(SchedulerDependencies {
        environment: asm.cfg.environment,
        runtime,
        event_bus: Some((*asm.event_bus).clone()),
        store: asm.store.clone(),
        workflow_launcher: Some(workflow_launcher),
        clock: None,
        tick_interval: Duration::ZERO,
    })));
    Ok(())
}

fn build_reminders(asm: &mut Assembly) -> Result<(), AppError> {
    let workflow_launcher = asm.seams.get::<WorkflowLauncherSeam>().expect("kernel launcher").0;
    asm.state.reminders = Some(Arc::new(RemindersManager::new(RemindersDependencies {
        environment_scope: asm.env_scope.to_string(),
        store: asm.store.clone(),
        event_bus: Some((*asm.event_bus).clone()),
        delivery: asm.state.delivery.as_ref().map(|delivery| (**delivery).clone()),
        workflow_launcher: Some(workflow_launcher),
        clock: None,
        tick_interval: Duration::ZERO,
    })));
    Ok(())
}

fn build_routines(asm: &mut Assembly) -> Result<(), AppError> {
    let scheduler = asm.state.scheduler.clone().expect("scheduler plugin built");
    let mut routine_manager = dope_routine::Manager::new(
        asm.env_scope,
        Box::new(adapters::RoutineSchedulerAdapter::new(scheduler)),
    );
    routine_manager.with_store(asm.store.clone());
    asm.state.routines = Some(Arc::new(routine_manager));
    Ok(())
}

fn build_memory(asm: &mut Assembly) -> Result<(), AppError> {
    let llm = asm.state.llm.clone().expect("llm plugin built");
    asm.state.memory = Some(Arc::new(MemoryManager::new(
        asm.env_scope,
        None,
        Some(Arc::new(adapters::LlmConsolidator::new(llm))),
        None,
    )));
    Ok(())
}

fn build_triage(asm: &mut Assembly) -> Result<(), AppError> {
    let mut triage_manager = TriageManager::new(asm.env_scope);
    triage_manager.with_store(asm.store.clone());
    asm.state.triage = Some(Arc::new(triage_manager));
    Ok(())
}

fn build_webhooks(asm: &mut Assembly) -> Result<(), AppError> {
    let billing = asm.state.billing.clone().expect("billing plugin built");
    let workflow_launcher = asm.seams.get::<WorkflowLauncherSeam>().expect("kernel launcher").0;
    let mut webhook_manager = WebhookManager::new(
        asm.env_scope,
        Some(Box::new(adapters::WebhookFirerImpl::new(workflow_launcher))),
        Some(Box::new(adapters::WebhookQuotaGateImpl::new(
            billing,
            asm.store.clone(),
            asm.event_bus.clone(),
            asm.env_scope,
        ))),
    );
    webhook_manager.with_store(asm.store.clone());
    asm.state.webhooks = Some(Arc::new(webhook_manager));
    Ok(())
}

fn build_catalog(asm: &mut Assembly) -> Result<(), AppError> {
    let sandboxes = asm.state.sandboxes.clone().expect("sandbox plugin built");
    let mut catalog_manager = dope_catalog::Manager::new(
        asm.env_scope,
        Some(Box::new(adapters::CatalogSandboxRequirementChecker::new(sandboxes))),
        Some(Box::new(adapters::CatalogTenantPermissionGate::new(asm.store.clone()))),
    );
    catalog_manager.with_store(asm.store.clone());
    asm.state.catalog = Some(Arc::new(catalog_manager));
    Ok(())
}

fn build_exec_profiles(asm: &mut Assembly) -> Result<(), AppError> {
    let sandboxes = asm.state.sandboxes.clone().expect("sandbox plugin built");
    #[cfg(test)]
    {
        asm.wiring.execprofile_health =
            Some(Arc::new(SandboxHealthChecker::new(Some(sandboxes.clone()))));
    }
    let mut exec_profile_manager = ExecProfileManager::new(
        asm.env_scope,
        Some(Box::new(SandboxHealthChecker::new(Some(sandboxes.clone())))),
        Some(Box::new(adapters::ExecProfileSandboxRequirementChecker::new(sandboxes))),
        Some(Box::new(adapters::ExecProfileTenantPermissionGate::new(asm.store.clone()))),
    );
    exec_profile_manager.with_store(asm.store.clone());
    asm.state.exec_profiles = Some(Arc::new(exec_profile_manager));
    Ok(())
}

fn build_evidence(asm: &mut Assembly) -> Result<(), AppError> {
    let routines = asm.state.routines.clone().expect("routines plugin built");
    #[cfg(test)]
    {
        asm.wiring.evidence_collector =
            Some(Arc::new(RoutineCollector::new(Some(routines.clone()))));
    }
    let mut evidence_manager = EvidenceManager::new(
        asm.env_scope,
        Some(Box::new(RoutineCollector::new(Some(routines)))),
        Some(Box::new(adapters::EvidenceSupportPermissionGate::new(asm.store.clone()))),
    );
    evidence_manager.with_store(asm.store.clone());
    asm.state.evidence = Some(Arc::new(evidence_manager));
    Ok(())
}

fn build_evaluation(asm: &mut Assembly) -> Result<(), AppError> {
    let billing = asm.state.billing.clone().expect("billing plugin built");
    let evaluation_store = Arc::new(EvaluationStoreHandle::new(asm.open_store()?));
    asm.state.evaluation = Some(Arc::new(EvaluationManager::new(EvaluationDependencies {
        environment_scope: asm.env_scope.to_string(),
        store: Some(evaluation_store),
        fixtures_dir: String::new(),
        runtime_recorder: None,
        billing: Some(billing),
        hosted_billing: asm.hosted,
        clock: None,
    })));
    Ok(())
}

fn build_live_validation(asm: &mut Assembly) -> Result<(), AppError> {
    let billing = asm.state.billing.clone().expect("billing plugin built");
    let live_validation_store = Arc::new(LiveValidationStoreHandle::new(asm.open_store()?));
    asm.state.live_validation = Some(Arc::new(LiveValidationManager::new(
        LiveValidationDependencies {
            environment_scope: asm.env_scope.to_string(),
            store: Some(live_validation_store),
            enabled: true,
            billing: Some(billing),
            hosted_billing: asm.hosted,
            clock: None,
            ledger_event_sink: None,
            candidate_tool_class_resolver: None,
        },
    )));
    Ok(())
}

fn build_setup_wizard(asm: &mut Assembly) -> Result<(), AppError> {
    let secrets = asm.state.secrets.clone().expect("kernel secrets");
    let setup_wizard_store = Arc::new(SetupWizardStoreHandle::new(asm.open_store()?));
    asm.state.setup_wizard = Some(Arc::new(new_setup_wizard(SetupWizardDependencies {
        store: Some(setup_wizard_store),
        secrets: Some(secrets),
        ..SetupWizardDependencies::default()
    })));
    Ok(())
}

/// Channel plugins assemble nothing here; their runtimes are constructed in
/// `App::serve` behind both the connector config flag and the plugin's
/// resolved enablement.
#[allow(clippy::unnecessary_wraps)]
fn build_channel_noop(_asm: &mut Assembly) -> Result<(), AppError> {
    Ok(())
}

/// Ensures the data dir exists before the profile is read (the store creates
/// it later anyway; profile load must not fail on a fresh install).
pub(crate) fn ensure_data_dir(data_dir: &str) {
    let _ = std::fs::create_dir_all(Path::new(data_dir));
}
