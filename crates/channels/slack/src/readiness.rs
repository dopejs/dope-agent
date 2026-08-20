//! Slack hosted-setup readiness (port of readiness.go): terminal/OAuth/route
//! policy states, the workspace binding, the hosted setup projection, and the
//! setup evaluation state machine.

use std::collections::HashMap;

use chrono::{DateTime, Duration, Utc};
use serde::{Deserialize, Serialize};

use kura_connectors::{DiagnosticReasonCode, LifecycleState, RedactionStatus};

use crate::destinations::{RoutePolicy, has_ready_route_policy, normalize_route_policy};
use crate::util::{first_non_empty, is_unset_time};

/// Hosted-setup terminal state (Go `TerminalState`).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum TerminalState {
    Ready,
    Degraded,
    Unavailable,
    Cancelled,
    #[default]
    ActionRequired,
}

impl TerminalState {
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            TerminalState::Ready => "ready",
            TerminalState::Degraded => "degraded",
            TerminalState::Unavailable => "unavailable",
            TerminalState::Cancelled => "cancelled",
            TerminalState::ActionRequired => "action-required",
        }
    }
}

impl std::fmt::Display for TerminalState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

/// OAuth grant lifecycle state (Go `OAuthState`). The `GrantMissing`
/// default matches Go's empty-string zero value, which the setup evaluator
/// maps to grant_missing.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum OAuthState {
    NotStarted,
    Started,
    CallbackReceived,
    GrantValid,
    #[default]
    GrantMissing,
    ScopeMissing,
    ApprovalRequired,
    Revoked,
    RedactionSuppressed,
}

impl OAuthState {
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            OAuthState::NotStarted => "not_started",
            OAuthState::Started => "started",
            OAuthState::CallbackReceived => "callback_received",
            OAuthState::GrantValid => "grant_valid",
            OAuthState::GrantMissing => "grant_missing",
            OAuthState::ScopeMissing => "scope_missing",
            OAuthState::ApprovalRequired => "approval_required",
            OAuthState::Revoked => "revoked",
            OAuthState::RedactionSuppressed => "redaction_suppressed",
        }
    }
}

impl std::fmt::Display for OAuthState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Route-policy readiness state (Go `RoutePolicyState`).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum RoutePolicyState {
    #[default]
    None,
    Partial,
    Valid,
    Stale,
}

impl RoutePolicyState {
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            RoutePolicyState::None => "none",
            RoutePolicyState::Partial => "partial",
            RoutePolicyState::Valid => "valid",
            RoutePolicyState::Stale => "stale",
        }
    }
}

impl std::fmt::Display for RoutePolicyState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Slack workspace binding (Go `WorkspaceBinding`): the verified workspace
/// identity + OAuth/scope grant states.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct WorkspaceBinding {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub connector_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub workspace_binding_id: String,
    pub workspace_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub workspace_label: String,
    pub installation_id: String,
    pub oauth_grant_state: String,
    pub required_scope_state: String,
    pub validated_at: DateTime<Utc>,
    pub redaction_status: RedactionStatus,
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub safe_evidence: HashMap<String, String>,
}

/// Input to [evaluate_hosted_setup] (Go `HostedSetupInput`; carries no JSON
/// tags).
#[derive(Debug, Clone, Default, PartialEq)]
pub struct HostedSetupInput {
    pub tenant_id: String,
    pub connector_id: String,
    pub display_name: String,
    pub workspace_binding: WorkspaceBinding,
    pub expected_workspace_id: String,
    pub oauth_state: OAuthState,
    pub route_policy: RoutePolicy,
    pub provider_available: bool,
    pub network_available: bool,
    pub cancelled: bool,
    pub redaction_reliable: bool,
    pub redaction_suppressed: bool,
    pub started_at: DateTime<Utc>,
    pub setup_timeout: Duration,
    pub validated_at: DateTime<Utc>,
}

/// Hosted-setup projection (Go `HostedSetup`).
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HostedSetup {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    pub connector_id: String,
    pub connector_kind: String,
    pub display_name: String,
    pub status: LifecycleState,
    pub terminal_state: TerminalState,
    pub oauth_state: OAuthState,
    pub route_policy_state: RoutePolicyState,
    pub delivery_eligible: bool,
    pub workspace_binding_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub workspace_binding: Option<WorkspaceBinding>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub route_policy: Option<RoutePolicy>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub created_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub updated_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub validated_at: Option<DateTime<Utc>>,
    pub redaction_status: RedactionStatus,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub retention_expires_at: Option<DateTime<Utc>>,
}

/// Go `EvaluateHostedSetup`: evaluates a hosted-setup input into its
/// terminal state machine outcome.
#[must_use]
pub fn evaluate_hosted_setup(input: HostedSetupInput) -> HostedSetup {
    let now = if is_unset_time(&input.validated_at) {
        Utc::now()
    } else {
        input.validated_at
    };
    let policy = normalize_route_policy(input.route_policy, now);
    let binding = normalize_workspace_binding(
        &input.tenant_id,
        &input.connector_id,
        input.workspace_binding,
        now,
    );
    let mut setup = HostedSetup {
        tenant_id: input.tenant_id.trim().to_string(),
        connector_id: input.connector_id.trim().to_string(),
        connector_kind: "slack".to_string(),
        display_name: input.display_name.trim().to_string(),
        status: LifecycleState::Degraded,
        terminal_state: TerminalState::ActionRequired,
        oauth_state: input.oauth_state,
        route_policy_state: RoutePolicyState::None,
        workspace_binding_id: binding.workspace_binding_id.clone(),
        workspace_binding: Some(binding.clone()),
        route_policy: Some(policy.clone()),
        delivery_eligible: false,
        reason_code: String::new(),
        created_at: Some(now),
        updated_at: Some(now),
        validated_at: Some(now),
        redaction_status: RedactionStatus::Redacted,
        retention_expires_at: Some(now + Duration::days(90)),
    };
    // Go: an empty OAuthState defaults to grant_missing; the typed port's
    // default variant is already GrantMissing, so this is a no-op guard.
    if setup.oauth_state == OAuthState::default() {
        setup.oauth_state = OAuthState::GrantMissing;
    }
    let timeout = if input.setup_timeout <= Duration::zero() {
        Duration::minutes(5)
    } else {
        input.setup_timeout
    };
    if input.redaction_suppressed || setup.oauth_state == OAuthState::RedactionSuppressed {
        setup.status = LifecycleState::Failed;
        setup.terminal_state = TerminalState::ActionRequired;
        setup.oauth_state = OAuthState::RedactionSuppressed;
        setup.redaction_status = RedactionStatus::Suppressed;
        setup.reason_code = DiagnosticReasonCode::UnknownConnectorFailure
            .as_str()
            .to_string();
        return setup;
    }
    if input.cancelled {
        setup.status = LifecycleState::Disabled;
        setup.terminal_state = TerminalState::Cancelled;
        setup.reason_code = "user_cancelled".to_string();
        return setup;
    }
    if !is_unset_time(&input.started_at) && now.signed_duration_since(input.started_at) > timeout {
        setup.status = LifecycleState::Failed;
        setup.terminal_state = TerminalState::Unavailable;
        setup.reason_code = "setup_timeout".to_string();
        return setup;
    }
    if setup.oauth_state != OAuthState::GrantValid {
        setup.reason_code = reason_for_oauth_state(setup.oauth_state);
        return setup;
    }
    if !input.provider_available {
        setup.status = LifecycleState::Failed;
        setup.terminal_state = TerminalState::Unavailable;
        setup.reason_code = DiagnosticReasonCode::ProviderUnavailable
            .as_str()
            .to_string();
        return setup;
    }
    if !input.network_available {
        setup.status = LifecycleState::Failed;
        setup.terminal_state = TerminalState::Unavailable;
        setup.reason_code = DiagnosticReasonCode::NetworkFailed.as_str().to_string();
        return setup;
    }
    if !input.expected_workspace_id.trim().is_empty()
        && !binding.workspace_id.trim().is_empty()
        && input.expected_workspace_id.trim() != binding.workspace_id.trim()
    {
        setup.reason_code = "workspace_mismatch".to_string();
        return setup;
    }
    if !workspace_binding_ready(&binding) {
        setup.reason_code = reason_for_oauth_state(setup.oauth_state);
        return setup;
    }
    if has_ready_route_policy(&policy) {
        setup.status = LifecycleState::Healthy;
        setup.terminal_state = TerminalState::Ready;
        setup.route_policy_state = RoutePolicyState::Valid;
        setup.delivery_eligible = true;
        setup.reason_code = "healthy".to_string();
        return setup;
    }
    setup.route_policy_state = RoutePolicyState::None;
    setup.reason_code = DiagnosticReasonCode::BlockedRoute.as_str().to_string();
    setup
}

/// Go `normalizeWorkspaceBinding`.
fn normalize_workspace_binding(
    tenant_id: &str,
    connector_id: &str,
    mut binding: WorkspaceBinding,
    now: DateTime<Utc>,
) -> WorkspaceBinding {
    binding.tenant_id = first_non_empty(&[&binding.tenant_id, tenant_id]);
    binding.connector_id = first_non_empty(&[&binding.connector_id, connector_id]);
    if binding.workspace_binding_id.is_empty() && !binding.connector_id.is_empty() {
        binding.workspace_binding_id = format!("slack_workspace_{}", binding.connector_id);
    }
    binding.oauth_grant_state = first_non_empty(&[&binding.oauth_grant_state, "missing"]);
    binding.required_scope_state = first_non_empty(&[&binding.required_scope_state, "unknown"]);
    if is_unset_time(&binding.validated_at) {
        binding.validated_at = now;
    }
    if binding.redaction_status == RedactionStatus::default() {
        binding.redaction_status = RedactionStatus::Redacted;
    }
    binding
}

/// Go `workspaceBindingReady`.
#[must_use]
pub fn workspace_binding_ready(binding: &WorkspaceBinding) -> bool {
    !binding.workspace_id.trim().is_empty()
        && !binding.installation_id.trim().is_empty()
        && binding.oauth_grant_state.trim() == "valid"
        && binding.required_scope_state.trim() == "valid"
}

/// Go `reasonForOAuthState`.
#[must_use]
pub fn reason_for_oauth_state(state: OAuthState) -> String {
    match state {
        OAuthState::GrantValid => DiagnosticReasonCode::BlockedRoute.as_str().to_string(),
        OAuthState::ScopeMissing | OAuthState::ApprovalRequired => {
            DiagnosticReasonCode::PermissionMissing.as_str().to_string()
        }
        OAuthState::Revoked
        | OAuthState::GrantMissing
        | OAuthState::NotStarted
        | OAuthState::Started
        | OAuthState::CallbackReceived => DiagnosticReasonCode::AuthMissing.as_str().to_string(),
        OAuthState::RedactionSuppressed => DiagnosticReasonCode::UnknownConnectorFailure
            .as_str()
            .to_string(),
    }
}
