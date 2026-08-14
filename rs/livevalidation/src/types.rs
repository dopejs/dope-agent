//! Core domain types (port of `types.go`). JSON field names match the Go
//! tags (camelCase) so persisted and wire representations stay compatible.

use std::collections::BTreeMap;

use chrono::DateTime;
use chrono::Utc;
use serde::Deserialize;
use serde::Serialize;

use crate::ledger::LedgerOutcome;
use crate::matrix::SafetyClass;
use crate::matrix::ToolClass;

define_string_enum!(
    /// Lifecycle status of a live-validation attempt.
    AttemptStatus {
        QUEUED => "queued",
        AWAITING_APPROVAL => "awaiting_approval",
        RUNNING => "running",
        COMPLETED => "completed",
        BLOCKED => "blocked",
        ABORTED => "aborted",
        FAILED => "failed",
        OPERATOR_ACTION_NEEDED => "operator_action_needed"
    }
);

define_string_enum!(
    /// How fresh approvals are required for a scope.
    ApprovalMode {
        SCOPE_LEVEL => "scope_level",
        PER_ACTION => "per_action",
        MIXED => "mixed"
    }
);

define_string_enum!(
    /// Whether an approval covers a whole scope or one action.
    ApprovalTarget {
        SCOPE => "scope",
        ACTION => "action"
    }
);

define_string_enum!(
    /// Resolution state of a fresh approval.
    ApprovalStatus {
        PENDING => "pending",
        APPROVED => "approved",
        DENIED => "denied",
        EXPIRED => "expired"
    }
);

define_string_enum!(
    /// Kill-switch reach: one tenant or the whole deployment.
    KillSwitchScope {
        TENANT => "tenant",
        GLOBAL => "global"
    }
);

define_string_enum!(
    /// Why a side-effect outcome is ambiguous.
    AmbiguousCommitCause {
        TIMEOUT => "timeout",
        CONNECTION_LOSS => "connection_loss",
        UNKNOWN_RESPONSE => "unknown_provider_response",
        DAEMON_RESTART => "daemon_restart",
        CONFLICTING_EVIDENCE => "conflicting_evidence",
        OTHER => "other"
    }
);

define_string_enum!(
    /// Operator resolution for an ambiguous commit.
    ReconciliationResolutionValue {
        CONFIRMED_COMMITTED => "confirmed_committed",
        CONFIRMED_NOT_COMMITTED => "confirmed_not_committed",
        COMPENSATED => "compensated",
        ACCEPTED_MANUAL_STATE => "accepted_manual_state",
        UNSUPPORTED_UNRESOLVED => "unsupported_unresolved"
    }
);

define_string_enum!(
    /// Terminal verdict of an outcome comparison.
    ComparisonStatus {
        MATCHED => "matched",
        DRIFTED => "drifted",
        BLOCKED => "blocked",
        UNSUPPORTED => "unsupported",
        OPERATOR_ACTION_NEEDED => "operator_action_needed"
    }
);

define_string_enum!(
    /// Which evidence family a retention policy covers.
    RetentionAppliesTo {
        ATTEMPTS => "attempts",
        LEDGER_ENTRIES => "ledger_entries",
        RECONCILIATION => "reconciliation_decisions",
        COMPARISONS => "comparisons",
        ALL => "all"
    }
);

define_string_enum!(
    /// Retention duration mode.
    RetentionMode {
        INDEFINITE => "indefinite",
        EXPLICIT => "explicit"
    }
);

define_string_enum!(
    /// Simulated downstream outcomes used by the fake executor tests.
    FakeOutcome {
        COMPLETED => "completed",
        FAILED => "failed",
        TIMEOUT_AFTER_SUBMIT => "timeout_after_submit",
        DUPLICATE_RETRY => "duplicate_retry",
        SUBMIT_UNKNOWN => "submit_unknown"
    }
);

/// Outcome of a single gate check (permission, quota, kill switch).
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GateDecision {
    pub allowed: bool,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reference: String,
    pub checked_at: DateTime<Utc>,
}

/// Per-outcome counts over the side-effect ledger (Go
/// `map[LedgerOutcome]int`).
pub type LedgerSummary = BTreeMap<LedgerOutcome, i64>;

/// A live-validation attempt and the gate evidence recorded while starting
/// it.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Attempt {
    pub validation_id: String,
    pub tenant_id: String,
    pub candidate_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub source_attempt_id: String,
    pub requested_by: String,
    pub environment_scope: String,
    pub requested_scope: SideEffectScope,
    pub status: AttemptStatus,
    pub permission_decision: GateDecision,
    pub quota_decision: GateDecision,
    pub kill_switch_decision: GateDecision,
    pub approval_summary: ApprovalSummary,
    #[serde(default)]
    pub ledger_summary: LedgerSummary,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub comparison_id: String,
    pub created_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub started_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub completed_at: Option<DateTime<Utc>>,
    pub updated_at: DateTime<Utc>,
}

/// Declared blast radius of a validation attempt.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SideEffectScope {
    pub scope_id: String,
    pub validation_id: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub included_tool_classes: Vec<ToolClass>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub excluded_tool_classes: Vec<ToolClass>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub included_actions: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub excluded_actions: Vec<String>,
    pub approval_mode: ApprovalMode,
    pub declared_by: String,
    pub declared_at: DateTime<Utc>,
}

/// Rollup of fresh-approval state across all requirements.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ApprovalSummary {
    pub required: i64,
    pub approved: i64,
    pub denied: i64,
    pub expired: i64,
    pub pending: i64,
}

/// A fresh (per-attempt) approval grant or denial.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct FreshApproval {
    pub approval_id: String,
    pub validation_id: String,
    pub tenant_id: String,
    pub approval_target: ApprovalTarget,
    pub tool_class: ToolClass,
    pub safety_class: SafetyClass,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub action_ref: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub approved_scope: String,
    pub status: ApprovalStatus,
    pub requested_by: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub resolved_by: String,
    pub requested_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub resolved_at: Option<DateTime<Utc>>,
}

/// One recorded side effect (or skipped/denied action) of an attempt.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SideEffectLedgerEntry {
    pub ledger_entry_id: String,
    pub validation_id: String,
    pub tenant_id: String,
    pub candidate_id: String,
    pub source_ref: String,
    pub tool_class: ToolClass,
    pub safety_class: SafetyClass,
    pub action_ref: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub approval_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub correlation_key: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub downstream_ref: String,
    pub outcome: LedgerOutcome,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub attempted_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub completed_at: Option<DateTime<Utc>>,
    pub updated_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub evidence_refs: Vec<String>,
    pub retry_count: i64,
    pub ambiguous_commit: bool,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reconciliation_id: String,
}

/// A live-validation kill switch.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct KillSwitch {
    pub kill_switch_id: String,
    pub scope: KillSwitchScope,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    pub enabled: bool,
    pub reason: String,
    pub changed_by: String,
    pub changed_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub expires_at: Option<DateTime<Utc>>,
}

/// A side effect whose commit state is unknown; automatic retry is stopped.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AmbiguousCommit {
    pub ambiguous_commit_id: String,
    pub ledger_entry_id: String,
    pub validation_id: String,
    pub tenant_id: String,
    pub cause: AmbiguousCommitCause,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub last_known_request_ref: String,
    pub automatic_retry_stopped: bool,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

/// Operator decision resolving an ambiguous commit.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ReconciliationResolution {
    pub reconciliation_id: String,
    pub ambiguous_commit_id: String,
    pub tenant_id: String,
    pub resolved_by: String,
    pub resolution: ReconciliationResolutionValue,
    pub reason: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub evidence_refs: Vec<String>,
    pub resolved_at: DateTime<Utc>,
}

/// Terminal comparison of a live attempt against its baseline.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Comparison {
    pub comparison_id: String,
    pub validation_id: String,
    pub candidate_id: String,
    pub baseline_ref: String,
    pub terminal_status: ComparisonStatus,
    #[serde(default)]
    pub ledger_summary: LedgerSummary,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub unsupported_classes: Vec<ToolClass>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub denials: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub ambiguous_commits: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub drift_findings: Vec<String>,
    pub generated_at: DateTime<Utc>,
}

/// Evidence retention policy.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RetentionPolicy {
    pub policy_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    pub applies_to: RetentionAppliesTo,
    pub mode: RetentionMode,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub retention_period: String,
    pub created_by_principal_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reason: String,
    pub created_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub expires_at: Option<DateTime<Utc>>,
}
