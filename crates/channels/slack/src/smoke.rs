//! Slack smoke evidence (port of smoke.go): structured smoke runs and the
//! authorization-mode/redaction state machine.

use std::collections::HashMap;

use chrono::{DateTime, Duration, Utc};
use serde::{Deserialize, Serialize};

use dope_connectors::{DiagnosticReasonCode, RedactionStatus};

use crate::diagnostics::{contains_unsafe_evidence, safe_evidence};
use crate::util::{first_non_empty, is_unset_time};

/// Smoke evidence status (Go `SmokeStatus`).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum SmokeStatus {
    Passed,
    Failed,
    #[default]
    Skipped,
}

impl SmokeStatus {
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            SmokeStatus::Passed => "passed",
            SmokeStatus::Failed => "failed",
            SmokeStatus::Skipped => "skipped",
        }
    }
}

impl std::fmt::Display for SmokeStatus {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Authorization mode used for a smoke run (Go `AuthorizationMode`).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AuthorizationMode {
    SafeLive,
    // serde snake_case would mangle this to fake_o_auth; the Go wire value
    // is fake_oauth.
    #[serde(rename = "fake_oauth")]
    FakeOAuth,
    #[default]
    Unavailable,
}

impl AuthorizationMode {
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            AuthorizationMode::SafeLive => "safe_live",
            AuthorizationMode::FakeOAuth => "fake_oauth",
            AuthorizationMode::Unavailable => "unavailable",
        }
    }
}

impl std::fmt::Display for AuthorizationMode {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Input to [build_smoke_evidence] (Go `SmokeInput`).
#[derive(Debug, Clone, Default, PartialEq)]
pub struct SmokeInput {
    pub smoke_evidence_id: String,
    pub tenant_id: String,
    pub connector_id: String,
    pub workspace_binding_id: String,
    pub safe_live_approved: bool,
    pub fake_oauth: bool,
    pub passed: bool,
    pub owner: String,
    pub reason: String,
    pub remaining_risk: String,
    pub validated_at: DateTime<Utc>,
    pub safe_evidence: HashMap<String, String>,
}

/// Smoke evidence record (Go `SmokeEvidence`).
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SmokeEvidence {
    pub smoke_evidence_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    pub connector_id: String,
    pub workspace_binding_id: String,
    pub status: SmokeStatus,
    pub authorization_mode: AuthorizationMode,
    pub owner: String,
    pub reason: String,
    pub remaining_risk: String,
    pub validated_at: DateTime<Utc>,
    pub retention_expires_at: DateTime<Utc>,
    pub redaction_status: RedactionStatus,
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub safe_evidence: HashMap<String, String>,
}

/// Go `BuildSmokeEvidence`.
#[must_use]
pub fn build_smoke_evidence(input: SmokeInput) -> SmokeEvidence {
    let now = if is_unset_time(&input.validated_at) {
        Utc::now()
    } else {
        input.validated_at
    };
    let id = input.smoke_evidence_id.trim();
    let id = if id.is_empty() {
        format!("slack_smoke_{}", input.connector_id.trim())
    } else {
        id.to_string()
    };
    let mut evidence = SmokeEvidence {
        smoke_evidence_id: id,
        tenant_id: input.tenant_id.trim().to_string(),
        connector_id: input.connector_id.trim().to_string(),
        workspace_binding_id: input.workspace_binding_id.trim().to_string(),
        status: SmokeStatus::Skipped,
        authorization_mode: AuthorizationMode::Unavailable,
        owner: first_non_empty(&[&input.owner, "operator"]),
        reason: first_non_empty(&[&input.reason, "safe_slack_authorization_unavailable"]),
        remaining_risk: first_non_empty(&[
            &input.remaining_risk,
            "Live Slack hosted smoke was not run.",
        ]),
        validated_at: now,
        retention_expires_at: now + Duration::days(90),
        redaction_status: RedactionStatus::Redacted,
        safe_evidence: safe_evidence(&input.safe_evidence),
    };
    if contains_unsafe_evidence(&input.safe_evidence) {
        evidence.redaction_status = RedactionStatus::Suppressed;
    }
    if input.fake_oauth {
        evidence.authorization_mode = AuthorizationMode::FakeOAuth;
    }
    if !input.safe_live_approved && !input.fake_oauth {
        return evidence;
    }
    if input.safe_live_approved {
        evidence.authorization_mode = AuthorizationMode::SafeLive;
    }
    if input.passed {
        evidence.status = SmokeStatus::Passed;
        evidence.reason = "healthy".to_string();
        evidence.remaining_risk = String::new();
        return evidence;
    }
    evidence.status = SmokeStatus::Failed;
    evidence.reason = first_non_empty(&[
        &input.reason,
        DiagnosticReasonCode::UnknownConnectorFailure.as_str(),
    ]);
    evidence
}
