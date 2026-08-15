//! Persisted-state recovery (port of Go recoverPersistedStateWithSecrets) and
//! connector-config projection helpers for the daemon app assembly.

use std::sync::Arc;

use dope_api::AppState;
use dope_config::Environment;
use dope_events::Bus;
use dope_store::calendar::CalendarOperationFilter;
use dope_store::mail::MailOperationFilter;

use crate::environment_scope;
use crate::error::AppError;

/// Builds the telegram allowments from the connector config allow-lists
/// (port of Go telegramAllowmentsFromConfig).
pub(crate) fn telegram_allowments_from_config(
    cfg: &dope_config::TelegramConnectorConfig,
) -> Vec<dope_telegram::AllowmentValidation> {
    let mut items = Vec::new();
    let mut push = |scope_type: dope_telegram::ScopeType,
                    scope_id: &str,
                    group_gate: dope_telegram::GroupGate| {
        let scope_id = scope_id.trim();
        if scope_id.is_empty() {
            return;
        }
        items.push(dope_telegram::AllowmentValidation {
            scope_type,
            scope_id: scope_id.to_string(),
            enabled: true,
            group_gate: Some(group_gate),
            validation_state: dope_telegram::AllowmentValidationState::Valid,
            ..Default::default()
        });
    };
    for id in &cfg.allowed_user_ids {
        push(
            dope_telegram::ScopeType::User,
            id,
            dope_telegram::GroupGate::NotApplicable,
        );
    }
    for id in &cfg.allowed_direct_chat_ids {
        push(
            dope_telegram::ScopeType::DirectChat,
            id,
            dope_telegram::GroupGate::NotApplicable,
        );
    }
    for id in &cfg.allowed_group_ids {
        push(
            dope_telegram::ScopeType::Group,
            id,
            dope_telegram::GroupGate::MentionOrCommandRequired,
        );
    }
    items
}

/// Error mapper for store loads: `load_err("x")` returns a
/// closure converting a store String error into `AppError::Restore`.
fn load_err(what: &str) -> impl FnOnce(String) -> AppError + '_ {
    let what = what.to_string();
    move |err: String| AppError::Restore(format!("{what}: {err}"))
}

/// Port of Go recoverPersistedStateWithSecrets: restores in-memory manager
/// state (sessions, checkpoints, connectors, capabilities, policy, auth,
/// identity, providers, sandbox, mcp, integrations, calendar, mail) from the
/// SQLite store and re-publishes persisted events on the bus. Idempotent:
/// every restore replaces the in-memory registry wholesale, and the store
/// DAOs are read-only. The reminders manager reads from the store directly
/// (no in-memory registry), so it needs no restore (Go `_ = reminderManager`).
pub(crate) fn recover_persisted_state(
    state: &AppState,
    event_bus: &Arc<Bus>,
    environment: Environment,
) -> Result<(), AppError> {
    let store = &state.store;

    // Sessions (Go ListSessions -> sessionRouter.RestoreSessions).
    let sessions = store
        .lock()
        .list_sessions()
        .map_err(load_err("load persisted sessions"))?;
    if let Some(router) = &state.router {
        router.restore_sessions(sessions);
    }

    // Checkpoints (Go checkpointManager.Restore).
    if let Some(checkpoints) = &state.checkpoints {
        checkpoints
            .restore()
            .map_err(|err| AppError::Restore(format!("restore runtime checkpoints: {err}")))?;
    }

    // Connectors (Go ListConnectors -> connectorSupervisor.Restore).
    let connectors = store
        .lock()
        .list_connectors()
        .map_err(load_err("load persisted connectors"))?;
    if let Some(supervisor) = &state.connectors {
        supervisor.restore(connectors);
    }

    // Capabilities (Go ListCapabilities -> capabilitySupervisor.Restore).
    let capabilities = store
        .lock()
        .list_capabilities()
        .map_err(load_err("load persisted capabilities"))?;
    if let Some(supervisor) = &state.capabilities {
        supervisor.restore(capabilities);
    }

    // Policy (Go ListApprovals/ListDecisions -> policyEngine.Restore).
    let approvals = store
        .lock()
        .list_approvals()
        .map_err(load_err("load persisted approvals"))?;
    let decisions = store
        .lock()
        .list_decisions()
        .map_err(load_err("load persisted decisions"))?;
    if let Some(policy) = &state.policy {
        policy.restore(approvals, decisions);
    }

    // Auth + local identity bootstrap (Go authManager.Restore +
    // identityManager.BootstrapLocal + SeedDefaultTenantCache).
    let pairings = store
        .lock()
        .list_pairings()
        .map_err(load_err("load persisted pairings"))?;
    let tokens = store
        .lock()
        .list_access_tokens()
        .map_err(load_err("load persisted access tokens"))?;
    if let Some(auth) = &state.auth {
        auth.restore(pairings, tokens.clone());
    }
    if let Some(identity) = &state.identity {
        let local_token_ids: Vec<String> = tokens
            .iter()
            .filter(|token| {
                token.mode == dope_identity::auth::PairingMode::Local
                    && token.status == dope_identity::auth::TokenStatus::Active
            })
            .map(|token| token.token_id.clone())
            .collect();
        // BootstrapLocal is idempotent (returns the existing principal when
        // present); failures are logged, not fatal, matching the store-level
        // tenant fallback used by the connector runtimes.
        if let Err(err) = identity.bootstrap_local(&local_token_ids) {
            eprintln!("[dope] local identity bootstrap failed: {err}");
        }
        let _ = store.lock().seed_default_tenant_cache();
    }

    // Providers (Go RestoreManagedAuthStates/Models/Preferences).
    let auth_states = store
        .lock()
        .list_provider_auth_states()
        .map_err(load_err("load persisted provider auth states"))?;
    let models = store
        .lock()
        .list_provider_models()
        .map_err(load_err("load persisted provider models"))?;
    let preferences = store
        .lock()
        .list_provider_preferences()
        .map_err(load_err("load persisted provider preferences"))?;
    if let Some(providers) = &state.providers {
        providers.restore_managed_auth_states(auth_states);
        providers.restore_provider_models(models);
        providers.restore_provider_preferences(preferences);
    }

    // Sandbox + MCP (Go sandboxManager.Restore / mcpManager.Restore).
    if let Some(sandboxes) = &state.sandboxes {
        sandboxes
            .restore()
            .map_err(|err| AppError::Restore(format!("restore sandbox executions: {err}")))?;
    }
    if let Some(mcp) = &state.mcp {
        mcp.restore()
            .map_err(|err| AppError::Restore(format!("restore mcp state: {err}")))?;
    }

    // Integrations / calendar / mail (Go ListIntegrations/Calendar/Mail ->
    // manager.Restore), scoped by environment.
    let env = environment_scope(environment);
    if let Some(integrations) = &state.integrations {
        let items = store
            .lock()
            .list_integrations(env)
            .map_err(load_err("load persisted integrations"))?;
        integrations.restore(items);
    }
    if let Some(calendar) = &state.calendar {
        let accounts = store
            .lock()
            .list_calendar_accounts(env)
            .map_err(load_err("load persisted calendar accounts"))?;
        let operations = store
            .lock()
            .list_calendar_operations(env, &CalendarOperationFilter::default())
            .map_err(load_err("load persisted calendar operations"))?;
        let artifacts = store
            .lock()
            .list_calendar_artifacts(env, "")
            .map_err(load_err("load persisted calendar artifacts"))?;
        calendar.restore(accounts, operations, artifacts);
    }
    if let Some(mail) = &state.mail {
        let accounts = store
            .lock()
            .list_mail_accounts(env)
            .map_err(load_err("load persisted mail accounts"))?;
        let operations = store
            .lock()
            .list_mail_operations(env, &MailOperationFilter::default())
            .map_err(load_err("load persisted mail operations"))?;
        let artifacts = store
            .lock()
            .list_mail_artifacts(env, "")
            .map_err(load_err("load persisted mail artifacts"))?;
        mail.restore(accounts, operations, artifacts);
    }

    // Re-publish persisted events on the live bus (Go ListEvents -> Publish).
    let events = store
        .lock()
        .list_events(&dope_events::Filter::default())
        .map_err(load_err("load persisted events"))?;
    for event in events {
        event_bus.publish(event);
    }
    Ok(())
}
