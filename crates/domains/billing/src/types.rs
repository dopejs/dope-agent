//! Domain types for the billing/quota subsystem (port of `types.go`).
//!
//! Serde field names match the Go JSON tags (camelCase); `omitempty` tags map
//! to `skip_serializing_if` helpers.

use chrono::DateTime;
use chrono::NaiveDate;
use chrono::Utc;
use serde::Deserialize;
use serde::Serialize;

/// Go's zero `time.Time` (`0001-01-01T00:00:00Z`), used as the serde default
/// for `time.Time` fields and for `IsZero` checks in admin flows.
#[must_use]
pub fn go_zero_time() -> DateTime<Utc> {
    DateTime::from_naive_utc_and_offset(
        NaiveDate::from_ymd_opt(1, 1, 1)
            .and_then(|date| date.and_hms_opt(0, 0, 0))
            .unwrap_or(DateTime::UNIX_EPOCH.naive_utc()),
        Utc,
    )
}

pub(crate) fn is_false(value: &bool) -> bool {
    !*value
}

pub(crate) fn is_zero_i64(value: &i64) -> bool {
    *value == 0
}

define_string_enum!(
    /// Metered resource category.
    Category {
        RUN_LAUNCHES => "run_launches",
        WORKFLOW_LAUNCHES => "workflow_launches",
        RUNTIME_TOOL_CALLS => "runtime_tool_calls",
        LIVE_VALIDATION_ATTEMPTS => "live_validation_attempts",
        INTEGRATION_OPERATIONS => "integration_operations",
        ARTIFACT_STORAGE_BYTES => "artifact_storage_bytes",
        REPLAY_EVALUATION_ATTEMPTS => "replay_evaluation_attempts"
    }
);

define_string_enum!(
    /// Unit a category is metered in.
    Unit {
        COUNT => "count",
        BYTES => "bytes",
        ATTEMPTS => "attempts"
    }
);

define_string_enum!(
    /// Quota period cadence.
    PeriodKind {
        NONE => "none",
        DAILY => "daily",
        MONTHLY => "monthly"
    }
);

define_string_enum!(
    /// Plan enforcement behavior.
    EnforcementMode {
        ENFORCED => "enforced",
        UNLIMITED => "unlimited",
        NOT_MEASURABLE => "not_measurable"
    }
);

define_string_enum!(
    /// Tenant plan lifecycle status.
    PlanStatus {
        ACTIVE => "active",
        SCHEDULED => "scheduled",
        DISABLED => "disabled",
        SUPERSEDED => "superseded"
    }
);

define_string_enum!(
    /// Usage reservation lifecycle status.
    ReservationStatus {
        RESERVED => "reserved",
        COMMITTED => "committed",
        RELEASED => "released",
        REFUNDED => "refunded",
        DENIED => "denied",
        OPERATOR_ACTION_NEEDED => "operator_action_needed"
    }
);

define_string_enum!(
    /// Usage event kinds appended to the billing evidence log.
    UsageEventKind {
        RESERVATION => "reservation",
        COMMIT => "commit",
        REFUND => "refund",
        RELEASE => "release",
        DENIAL => "denial",
        MANUAL_ADJUSTMENT => "manual_adjustment",
        PERIOD_RESET => "period_reset",
        RECOVERY_DECISION => "recovery_decision",
        PLAN_CHANGED => "plan_changed",
        QUOTA_OVERRIDE => "quota_override_changed",
        OVER_LIMIT_COMMIT => "over_limit_commit",
        RETENTION_POLICY => "retention_policy_changed"
    }
);

define_string_enum!(
    /// Projected quota status for dashboards and denial classification.
    QuotaStatus {
        AVAILABLE => "available",
        NEAR_LIMIT => "near_limit",
        EXHAUSTED => "exhausted",
        UNLIMITED => "unlimited",
        NOT_MEASURABLE => "not_measurable",
        RESTRICTED => "restricted",
        UNAVAILABLE => "unavailable"
    }
);

define_string_enum!(
    /// Why a quota is considered near its limit.
    NearLimitReason {
        NONE => "",
        PERCENT_THRESHOLD => "percent_threshold",
        BELOW_ONE_TYPICAL_OPERATION => "below_one_typical_operation"
    }
);

define_string_enum!(
    /// Operator/user actions suggested for a quota state.
    RecoveryAction {
        WAIT => "wait",
        REDUCE_SCOPE => "reduce_scope",
        REQUEST_OVERRIDE => "request_override",
        CONTACT_SUPPORT => "contact_support",
        OPERATOR_RESOLUTION_REQUIRED => "operator_resolution_required",
        RETRY_LATER => "retry_later"
    }
);

define_string_enum!(
    /// Classification of a quota denial for support/evidence flows.
    DenialClassification {
        QUOTA_EXHAUSTION => "quota_exhaustion",
        ABUSE_RESTRICTION => "abuse_restriction",
        QUOTA_STATE_UNAVAILABLE => "quota_state_unavailable",
        UNAUTHORIZED => "unauthorized",
        OPERATOR_ACTION_NEEDED => "operator_action_needed"
    }
);

define_string_enum!(
    /// Abuse restriction lifecycle status.
    AbuseRestrictionStatus {
        ACTIVE => "active",
        EXPIRED => "expired"
    }
);

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TenantPlan {
    pub plan_id: String,
    pub tenant_id: String,
    pub plan_key: String,
    pub status: PlanStatus,
    pub enforcement_mode: EnforcementMode,
    #[serde(default = "go_zero_time")]
    pub effective_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub superseded_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub assigned_by_principal_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub assignment_reason: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub document: Option<serde_json::Map<String, serde_json::Value>>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct QuotaDefinition {
    pub quota_definition_id: String,
    pub category: Category,
    pub unit: Unit,
    pub period_kind: PeriodKind,
    pub period_anchor: String,
    pub default_limit: i64,
    pub carryover_enabled: bool,
    #[serde(default, skip_serializing_if = "is_zero_i64")]
    pub carryover_max: i64,
    pub reservation_rule: String,
    pub commit_rule: String,
    pub refund_rule: String,
    pub denial_reason_code: String,
    pub active: bool,
    #[serde(default = "go_zero_time")]
    pub created_at: DateTime<Utc>,
    #[serde(default = "go_zero_time")]
    pub updated_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub document: Option<serde_json::Map<String, serde_json::Value>>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct QuotaOverride {
    pub quota_override_id: String,
    pub tenant_id: String,
    pub category: Category,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub limit: Option<i64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub carryover_enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub carryover_max: Option<i64>,
    #[serde(default = "go_zero_time")]
    pub effective_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub expires_at: Option<DateTime<Utc>>,
    pub reason: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub created_by_principal_id: String,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct QuotaPeriod {
    pub quota_period_id: String,
    pub tenant_id: String,
    pub category: Category,
    pub period_kind: PeriodKind,
    #[serde(default = "go_zero_time")]
    pub period_start: DateTime<Utc>,
    #[serde(default = "go_zero_time")]
    pub period_end: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub carryover_from_period_id: String,
    pub status: String,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct UsageCounter {
    pub usage_counter_id: String,
    pub tenant_id: String,
    pub category: Category,
    pub quota_period_id: String,
    pub committed_amount: i64,
    pub reserved_amount: i64,
    pub adjusted_amount: i64,
    pub carryover_amount: i64,
    #[serde(default = "go_zero_time")]
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct UsageReservation {
    pub reservation_id: String,
    pub tenant_id: String,
    pub category: Category,
    pub quota_period_id: String,
    pub operation_key: String,
    pub amount_reserved: i64,
    pub amount_committed: i64,
    pub amount_refunded: i64,
    pub status: ReservationStatus,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reservation_point: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub commit_point: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub refund_point: String,
    #[serde(default = "go_zero_time")]
    pub created_at: DateTime<Utc>,
    #[serde(default = "go_zero_time")]
    pub updated_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub expires_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub recovery_reason: String,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct UsageEvent {
    pub usage_event_id: String,
    pub tenant_id: String,
    pub category: Category,
    pub quota_period_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub operation_key: String,
    pub event_kind: UsageEventKind,
    pub amount: i64,
    pub reason_code: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reason: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub actor_principal_id: String,
    pub outcome: String,
    #[serde(default = "go_zero_time")]
    pub created_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub document: Option<serde_json::Map<String, serde_json::Value>>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct QuotaDenial {
    pub denial_id: String,
    pub tenant_id: String,
    #[serde(default, skip_serializing_if = "Category::is_empty")]
    pub category: Category,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub quota_period_id: String,
    pub operation_key: String,
    pub reason_code: String,
    pub requested_amount: i64,
    pub remaining_amount: i64,
    pub guarded_entry_point: String,
    #[serde(default = "go_zero_time")]
    pub created_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ManualAdjustment {
    pub adjustment_id: String,
    pub tenant_id: String,
    pub category: Category,
    pub quota_period_id: String,
    pub amount_delta: i64,
    pub reason: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub created_by_principal_id: String,
    #[serde(default = "go_zero_time")]
    pub created_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AuditRetentionPolicy {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    pub retention_mode: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub retention_period: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub created_by_principal_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reason: String,
    #[serde(default = "go_zero_time")]
    pub created_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub expires_at: Option<DateTime<Utc>>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PlanSummary {
    pub plan_key: String,
    pub enforcement_mode: EnforcementMode,
    #[serde(default, skip_serializing_if = "PlanStatus::is_empty")]
    pub status: PlanStatus,
    #[serde(default = "go_zero_time")]
    pub effective_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub base_plan_label: String,
    pub checkout_available: bool,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct UsagePeriodSummary {
    #[serde(default = "go_zero_time")]
    pub period_start: DateTime<Utc>,
    #[serde(default = "go_zero_time")]
    pub period_end: DateTime<Utc>,
    pub period_anchor: String,
    pub consumed_amount: i64,
    pub reserved_amount: i64,
    pub adjusted_amount: i64,
    pub carryover_applied: i64,
    pub remaining_amount: i64,
    pub over_limit: bool,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct QuotaOverrideSummary {
    pub base_limit: i64,
    pub effective_limit: i64,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reason: String,
    #[serde(default = "go_zero_time")]
    pub effective_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub expires_at: Option<DateTime<Utc>>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AbuseRestrictionSummary {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub restriction_id: String,
    pub status: AbuseRestrictionStatus,
    #[serde(default, skip_serializing_if = "Category::is_empty")]
    pub affected_category: Category,
    pub recovery_action: RecoveryAction,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub visible_reason_code: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub source_audit_ref: String,
    pub support_contact_allowed: bool,
    #[serde(default = "go_zero_time")]
    pub started_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub expires_at: Option<DateTime<Utc>>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AbuseRestrictionRecord {
    pub restriction_id: String,
    pub tenant_id: String,
    pub status: AbuseRestrictionStatus,
    pub affected_category: Category,
    pub recovery_action: RecoveryAction,
    pub visible_reason_code: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub source_audit_ref: String,
    pub support_contact_allowed: bool,
    #[serde(default = "go_zero_time")]
    pub started_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub expires_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub document: Option<serde_json::Map<String, serde_json::Value>>,
}

impl AbuseRestrictionRecord {
    #[must_use]
    pub fn summary(&self) -> AbuseRestrictionSummary {
        AbuseRestrictionSummary {
            restriction_id: self.restriction_id.clone(),
            status: self.status.clone(),
            affected_category: self.affected_category.clone(),
            recovery_action: self.recovery_action.clone(),
            visible_reason_code: self.visible_reason_code.clone(),
            source_audit_ref: self.source_audit_ref.clone(),
            support_contact_allowed: self.support_contact_allowed,
            started_at: self.started_at,
            expires_at: self.expires_at,
        }
    }
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct QuotaStatusItem {
    pub category: Category,
    pub unit: Unit,
    pub status: QuotaStatus,
    pub current_period: UsagePeriodSummary,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub previous_period: Option<UsagePeriodSummary>,
    pub limit: i64,
    pub remaining_amount: i64,
    pub near_limit: bool,
    #[serde(default, skip_serializing_if = "NearLimitReason::is_empty")]
    pub near_limit_reason: NearLimitReason,
    pub typical_operation_amount: i64,
    pub base_limit: i64,
    pub effective_limit: i64,
    #[serde(default, rename = "override", skip_serializing_if = "Option::is_none")]
    pub override_: Option<QuotaOverrideSummary>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub restriction: Option<AbuseRestrictionSummary>,
    pub recovery_actions: Vec<RecoveryAction>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct QuotaSection {
    pub section_key: String,
    pub label: String,
    pub items: Vec<QuotaStatusItem>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TenantQuotaDashboard {
    pub tenant_id: String,
    pub plan: PlanSummary,
    pub sections: Vec<QuotaSection>,
    #[serde(default = "go_zero_time")]
    pub generated_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub permission: Option<serde_json::Map<String, serde_json::Value>>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct QuotaDenialDetail {
    pub denial_id: String,
    pub tenant_id: String,
    pub operation_ref: String,
    pub operation_key: String,
    pub guarded_entry_point: String,
    #[serde(default, skip_serializing_if = "Category::is_empty")]
    pub category: Category,
    pub reason_code: String,
    pub classification: DenialClassification,
    pub requested_amount: i64,
    pub remaining_amount: i64,
    pub recovery_actions: Vec<RecoveryAction>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub restriction: Option<AbuseRestrictionSummary>,
    #[serde(default = "go_zero_time")]
    pub created_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct BillingEvidenceRedaction {
    pub path: String,
    pub reason: String,
    pub replacement: String,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct BillingEvidenceExport {
    pub schema_version: String,
    pub export_id: String,
    pub tenant_id: String,
    #[serde(default = "go_zero_time")]
    pub generated_at: DateTime<Utc>,
    pub generated_by_principal_id: String,
    pub denial: QuotaDenialDetail,
    pub usage_snapshot: Vec<QuotaStatusItem>,
    pub effective_limit_state: serde_json::Map<String, serde_json::Value>,
    pub audit_refs: Vec<String>,
    pub redactions: Vec<BillingEvidenceRedaction>,
}
