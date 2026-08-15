//! Port of daemon/internal/delivery: the delivery domain types (targets, preferences,
//! outcomes, attempts, summary windows) plus the manager, dispatcher, digest, linkage, adapter
//! seam, and live-validation rows. The manager logic lives in [`manager`], [`dispatcher`],
//! [`digest`], and [`linkage`]; the channel-connector adapter ([`connector_adapter`]) ports the
//! Go ConnectorAdapter (connector reply senders, connector message/boundary persistence, and
//! the matrix/telegram/slack hosted-setup delivery-eligibility gating). The adapter seam
//! ([`DeliveryAdapter`]) and the channel/thread store hooks ([`ChannelDeliveryHooks`]) are
//! ported as traits with documented no-op defaults where the store domain is still deferred.
//!
//! context.Context is replaced by synchronous Rust: the Go manager's background goroutines for
//! retries and summary-window emission become detached std threads (see
//! [`Manager::configure_for_testing`] for the test knobs).

use std::collections::HashMap;

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

macro_rules! string_enum {
    ($name:ident { $first:ident => $first_s:literal $(, $v:ident => $s:literal)* $(,)? }) => {
        #[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
        #[serde(rename_all = "snake_case")]
        pub enum $name {
            #[default]
            $first,
            $($v),*
        }
        impl $name {
            #[must_use]
            pub fn as_str(self) -> &'static str {
                match self {
                    $name::$first => $first_s,
                    $( $name::$v => $s ),*
                }
            }
        }
        impl std::fmt::Display for $name {
            fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                f.write_str(self.as_str())
            }
        }
    };
}

string_enum!(TargetKind {
    ConnectorRoute => "connector_route",
    TestSink => "test_sink",
});

string_enum!(TargetStatus {
    Active => "active",
    Disabled => "disabled",
    Unreachable => "unreachable",
    Misconfigured => "misconfigured",
});

string_enum!(PreferenceScopeKind {
    UserDefault => "user_default",
    IntegrationOverride => "integration_override",
});

string_enum!(ResultClass {
    RoutineSuccess => "routine_success",
    Urgent => "urgent",
    Failure => "failure",
});

string_enum!(DeliveryMode {
    Immediate => "immediate",
    Digest => "digest",
    Suppressed => "suppressed",
});

string_enum!(OutcomeStatus {
    Pending => "pending",
    Queued => "queued",
    Dispatching => "dispatching",
    Delivered => "delivered",
    Suppressed => "suppressed",
    Failed => "failed",
});

string_enum!(AttemptStatus {
    Running => "running",
    Delivered => "delivered",
    RetryableFailure => "retryable_failure",
    TerminalFailure => "terminal_failure",
});

string_enum!(SummaryWindowStatus {
    Open => "open",
    Ready => "ready",
    Dispatching => "dispatching",
    Delivered => "delivered",
    Failed => "failed",
    Cancelled => "cancelled",
});

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ConnectorBinding {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub connector_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub channel_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub peer_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub thread_id: String,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SummaryPolicy {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub routine_success_mode: Option<DeliveryMode>,
    #[serde(default, skip_serializing_if = "is_zero_i64")]
    pub window_minutes: i64,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SuppressionPolicy {
    #[serde(default, skip_serializing_if = "std::ops::Not::not")]
    pub suppress_routine_success: bool,
    #[serde(default, skip_serializing_if = "std::ops::Not::not")]
    pub suppress_urgent: bool,
    #[serde(default, skip_serializing_if = "std::ops::Not::not")]
    pub suppress_failure: bool,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DeliveryTarget {
    pub target_id: String,
    pub display_name: String,
    pub environment_scope: String,
    pub target_kind: TargetKind,
    pub status: TargetStatus,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub connector_binding: Option<ConnectorBinding>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub address_summary: String,
    #[serde(default, skip_serializing_if = "std::ops::Not::not")]
    pub supports_immediate: bool,
    #[serde(default, skip_serializing_if = "std::ops::Not::not")]
    pub supports_digest: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_validated_at: Option<DateTime<Utc>>,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DeliveryPreference {
    pub preference_id: String,
    pub environment_scope: String,
    pub scope_kind: PreferenceScopeKind,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub integration_id: String,
    pub preferred_targets_by_class: HashMap<ResultClass, String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub summary_policy: Option<SummaryPolicy>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub suppression_policy: Option<SuppressionPolicy>,
    #[serde(default, skip_serializing_if = "std::ops::Not::not")]
    pub active: bool,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DeliveryAttempt {
    pub attempt_id: String,
    pub delivery_id: String,
    pub attempt_number: i64,
    pub target_id: String,
    pub transport_kind: String,
    pub status: AttemptStatus,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub failure_class: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub failure_reason: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub next_retry_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub connector_message_delivery_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub transport_receipt_summary: String,
    pub started_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub completed_at: Option<DateTime<Utc>>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DeliveryOutcome {
    pub delivery_id: String,
    pub environment_scope: String,
    pub source_kind: String,
    pub source_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub run_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub workflow_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub schedule_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub schedule_attempt_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub integration_id: String,
    pub result_class: ResultClass,
    pub mode: DeliveryMode,
    pub status: OutcomeStatus,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub chosen_target_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub preference_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub summary_window_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub payload_preview: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub suppression_reason: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub calendar_operation_ids: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub calendar_operation_summaries: Vec<dope_calendar::OperationSummary>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub mail_operation_ids: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub mail_operation_summaries: Vec<dope_mail::OperationSummary>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub attempts: Vec<DeliveryAttempt>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub diagnostic_failure: Option<dope_integrations::DiagnosticFailureProjection>,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub finalized_at: Option<DateTime<Utc>>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SummaryWindow {
    pub summary_window_id: String,
    pub environment_scope: String,
    pub target_id: String,
    pub preference_id: String,
    pub status: SummaryWindowStatus,
    pub window_started_at: DateTime<Utc>,
    pub window_ends_at: DateTime<Utc>,
    pub result_count: i64,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub emitted_delivery_id: String,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Default)]
pub struct OutcomeInput {
    pub source_kind: String,
    pub source_id: String,
    pub run_id: String,
    pub workflow_id: String,
    pub schedule_id: String,
    pub schedule_attempt_id: String,
    pub integration_id: String,
    pub result_class: ResultClass,
    pub payload_preview: String,
}

#[derive(Debug, Clone, Default)]
pub struct OutcomeFilter {
    pub source_kind: String,
    pub source_id: String,
    pub run_id: String,
    pub workflow_id: String,
    pub schedule_id: String,
    pub integration_id: String,
    /// Outcome status filter. Go's zero value is the empty string (no filter); the
    /// [`OutcomeStatus`] enum has no empty variant, so `None` maps to "no status filter"
    /// and `Some(status)` filters on that status.
    pub status: Option<OutcomeStatus>,
    pub target_id: String,
}

#[derive(Debug, Clone, Default)]
pub struct SendResult {
    pub transport_kind: String,
    pub receipt_summary: String,
    pub connector_message_delivery_id: String,
    pub connector_delivery_boundary_id: String,
    pub separation_status: String,
}

#[must_use]
fn is_zero_i64(v: &i64) -> bool {
    *v == 0
}

pub mod adapters;
pub mod connector_adapter;
pub mod digest;
pub mod dispatcher;
pub mod linkage;
pub mod live_validation;
pub mod manager;
pub mod test_sink;

pub use adapters::{ChannelDeliveryHooks, DeliveryAdapter};
pub use connector_adapter::{ConnectorAdapter, ConnectorReplySender};
pub use linkage::{LatestSummary, latest_summary_from_outcome};
pub use live_validation::live_validation_matrix_rows;
pub use manager::{DeliveryError, Manager};
pub use test_sink::{TestSinkAdapter, TestSinkMessage};
