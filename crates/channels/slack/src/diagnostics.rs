//! Slack diagnostics (port of diagnostics.go): failure classification into
//! diagnostic reason codes, and the diagnostic-state builder with unsafe
//! evidence redaction (Go's shared ClassifyDiagnostic is not yet ported into
//! kura-connectors, so the classification is implemented here on top of the
//! kura-connectors types).

use std::collections::HashMap;

use chrono::{DateTime, Duration, Utc};

use kura_connectors::{
    ConnectorDiagnosticState, DiagnosticReasonCode, FreshnessState, LifecycleState,
    RedactionStatus, RemediationOwner, RetrySafety,
};

use crate::error::SlackError;
use crate::transport_webapi::WebApiError;
use crate::util::is_unset_time;

/// Go `DiagnosticReasonForError`: classifies a transport/provider failure
/// into a diagnostic reason, preferring the classified Web API error class and
/// falling back to message-keyword heuristics.
#[must_use]
pub fn diagnostic_reason_for_error(
    err: &(dyn std::error::Error + 'static),
) -> DiagnosticReasonCode {
    if let Some(web_api) = err.downcast_ref::<WebApiError>() {
        return web_api.error_class();
    }
    if let Some(slack) = err.downcast_ref::<SlackError>() {
        if let SlackError::WebApi(web_api) = slack {
            return web_api.error_class();
        }
        return diagnostic_reason_for_message(&slack.to_string());
    }
    diagnostic_reason_for_message(&err.to_string())
}

/// Go `DiagnosticReasonForError`'s message-keyword fallback (also used to
/// classify plain string errors surfaced by the message loop).
#[must_use]
pub fn diagnostic_reason_for_message(message: &str) -> DiagnosticReasonCode {
    let lowered = message.to_lowercase();
    for keyword in ["oauth", "token", "revoked", "invalid_auth"] {
        if lowered.contains(keyword) {
            return DiagnosticReasonCode::AuthMissing;
        }
    }
    for keyword in ["scope", "permission", "not_in_channel", "approval"] {
        if lowered.contains(keyword) {
            return DiagnosticReasonCode::PermissionMissing;
        }
    }
    for keyword in ["rate", "429"] {
        if lowered.contains(keyword) {
            return DiagnosticReasonCode::RateLimited;
        }
    }
    for keyword in ["network", "event delivery", "timeout"] {
        if lowered.contains(keyword) {
            return DiagnosticReasonCode::NetworkFailed;
        }
    }
    if lowered.contains("unsupported") {
        return DiagnosticReasonCode::UnsupportedCapability;
    }
    for keyword in ["unavailable", "5xx"] {
        if lowered.contains(keyword) {
            return DiagnosticReasonCode::ProviderUnavailable;
        }
    }
    DiagnosticReasonCode::UnknownConnectorFailure
}

/// Go `BuildDiagnosticState`: builds a connector diagnostic state, suppressing
/// evidence and stamping a redaction failure when the evidence contains unsafe
/// values.
pub fn build_diagnostic_state(
    tenant_id: &str,
    connector_id: &str,
    workspace_binding_id: &str,
    reason: DiagnosticReasonCode,
    evidence: &HashMap<String, String>,
    now: DateTime<Utc>,
) -> Result<ConnectorDiagnosticState, SlackError> {
    let connector_id = connector_id.trim();
    if connector_id.is_empty() {
        return Err(SlackError::DiagnosticConnectorRequired);
    }
    let now = if is_unset_time(&now) { Utc::now() } else { now };
    let redaction_reliable = !contains_unsafe_evidence(evidence);
    let (redaction_status, safe, redaction_failure_id) = if redaction_reliable {
        (
            RedactionStatus::Redacted,
            safe_evidence(evidence),
            String::new(),
        )
    } else {
        (
            RedactionStatus::Suppressed,
            HashMap::new(),
            format!("redaction_failed_{connector_id}"),
        )
    };
    Ok(ConnectorDiagnosticState {
        diagnostic_state_id: format!("diag_{connector_id}_{}", reason.as_str()),
        tenant_id: tenant_id.trim().to_string(),
        connector_id: connector_id.to_string(),
        connector_account_id: workspace_binding_id.trim().to_string(),
        status: status_for_diagnostic(reason),
        reason_code: reason,
        remediation_owner: remediation_for_diagnostic(reason),
        user_visible_severity: severity_for_diagnostic(reason).to_string(),
        retry_safety: retry_safety_for_diagnostic(reason),
        evidence_timestamp: now,
        freshness_state: FreshnessState::Fresh,
        redaction_status,
        retention_expires_at: now + Duration::days(90),
        safe_evidence: safe,
        redaction_failure_id,
    })
}

/// Go `containsUnsafeEvidence`: detects secret-like keys/values in evidence.
#[must_use]
pub fn contains_unsafe_evidence(evidence: &HashMap<String, String>) -> bool {
    for (key, value) in evidence {
        let lower_key = key.to_lowercase();
        let lower_value = value.to_lowercase();
        if lower_key.contains("token")
            || lower_key.contains("secret")
            || lower_key.contains("authorization")
            || lower_value.contains("xoxb-")
            || lower_value.contains("secret")
            || lower_value.contains("signing secret")
            || lower_value.contains("authorization")
            || lower_value.contains("bot token")
        {
            return true;
        }
    }
    false
}

/// Go `safeEvidence`: a copy of the evidence when safe, else an empty map
/// (Go returns nil, which serializes as absent).
#[must_use]
pub fn safe_evidence(evidence: &HashMap<String, String>) -> HashMap<String, String> {
    if contains_unsafe_evidence(evidence) {
        return HashMap::new();
    }
    evidence.clone()
}

/// Go `statusForDiagnostic`.
fn status_for_diagnostic(reason: DiagnosticReasonCode) -> LifecycleState {
    match reason {
        DiagnosticReasonCode::AuthMissing => LifecycleState::Failed,
        DiagnosticReasonCode::PermissionMissing => LifecycleState::PermissionBlocked,
        DiagnosticReasonCode::RateLimited => LifecycleState::RateLimited,
        DiagnosticReasonCode::ProviderUnavailable | DiagnosticReasonCode::NetworkFailed => {
            LifecycleState::Degraded
        }
        DiagnosticReasonCode::UnsupportedCapability => LifecycleState::UnsupportedCapability,
        DiagnosticReasonCode::BlockedRoute | DiagnosticReasonCode::DuplicateInbound => {
            LifecycleState::Degraded
        }
        DiagnosticReasonCode::ReplyFailed | DiagnosticReasonCode::UnknownConnectorFailure => {
            LifecycleState::Failed
        }
    }
}

/// Go `remediationForDiagnostic`.
fn remediation_for_diagnostic(reason: DiagnosticReasonCode) -> RemediationOwner {
    match reason {
        DiagnosticReasonCode::AuthMissing => RemediationOwner::User,
        DiagnosticReasonCode::PermissionMissing | DiagnosticReasonCode::BlockedRoute => {
            RemediationOwner::Admin
        }
        DiagnosticReasonCode::RateLimited | DiagnosticReasonCode::ProviderUnavailable => {
            RemediationOwner::Provider
        }
        DiagnosticReasonCode::NetworkFailed
        | DiagnosticReasonCode::ReplyFailed
        | DiagnosticReasonCode::UnknownConnectorFailure => RemediationOwner::Operator,
        DiagnosticReasonCode::UnsupportedCapability | DiagnosticReasonCode::DuplicateInbound => {
            RemediationOwner::NoneRequired
        }
    }
}

/// Go `retrySafetyForDiagnostic`.
fn retry_safety_for_diagnostic(reason: DiagnosticReasonCode) -> RetrySafety {
    match reason {
        DiagnosticReasonCode::RateLimited => RetrySafety::RetryAfter,
        DiagnosticReasonCode::ProviderUnavailable | DiagnosticReasonCode::NetworkFailed => {
            RetrySafety::Retryable
        }
        DiagnosticReasonCode::AuthMissing
        | DiagnosticReasonCode::PermissionMissing
        | DiagnosticReasonCode::BlockedRoute => RetrySafety::Blocked,
        DiagnosticReasonCode::DuplicateInbound | DiagnosticReasonCode::UnsupportedCapability => {
            RetrySafety::NoActionNeeded
        }
        DiagnosticReasonCode::ReplyFailed | DiagnosticReasonCode::UnknownConnectorFailure => {
            RetrySafety::Unsafe
        }
    }
}

/// Go `severityForDiagnostic`.
fn severity_for_diagnostic(reason: DiagnosticReasonCode) -> &'static str {
    match reason {
        DiagnosticReasonCode::DuplicateInbound | DiagnosticReasonCode::UnsupportedCapability => {
            "info"
        }
        DiagnosticReasonCode::BlockedRoute | DiagnosticReasonCode::RateLimited => "warning",
        DiagnosticReasonCode::AuthMissing
        | DiagnosticReasonCode::PermissionMissing
        | DiagnosticReasonCode::ProviderUnavailable
        | DiagnosticReasonCode::NetworkFailed
        | DiagnosticReasonCode::ReplyFailed
        | DiagnosticReasonCode::UnknownConnectorFailure => "error",
    }
}
