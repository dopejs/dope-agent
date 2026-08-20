//! Port of daemon/internal/reminders/types.go: reminder domain types (behavior
//! modes, occurrence states, action/actor kinds, follow-up link kinds), the reminder
//! ledger documents, the workflow-launch seam, and the package error set.
//!
//! JSON shape matches the Go tags exactly (camelCase; optional fields omitted when
//! empty/absent), so reminder documents round-trip byte-identically with Go rows.

use chrono::{DateTime, Utc};
use kura_calendar::Action as CalendarAction;
use kura_integrations::DiagnosticFailureProjection;
use kura_mail::Action as MailAction;
use kura_scheduler::Trigger;
use serde::{Deserialize, Serialize};

macro_rules! string_enum {
    ($(#[$meta:meta])* $name:ident { $first:ident => $first_s:literal $(, $v:ident => $s:literal)* $(,)? }) => {
        $(#[$meta])*
        #[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, Serialize, Deserialize)]
        pub enum $name {
            #[default]
            #[serde(rename = $first_s)]
            $first,
            $(#[serde(rename = $s)] $v),*
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

string_enum!(
    /// Go BehaviorMode.
    BehaviorMode {
        NotifyOnly => "notify_only",
        LaunchWorkflow => "launch_workflow",
    }
);

string_enum!(
    /// Go State — a reminder occurrence lifecycle state.
    State {
        Pending => "pending",
        Due => "due",
        Acknowledged => "acknowledged",
        Snoozed => "snoozed",
        Completed => "completed",
        Dismissed => "dismissed",
        Cancelled => "cancelled",
        Overdue => "overdue",
        Missed => "missed",
    }
);

string_enum!(
    /// Go ActionKind.
    ActionKind {
        Created => "created",
        Due => "due",
        Acknowledged => "acknowledged",
        Snoozed => "snoozed",
        Completed => "completed",
        Dismissed => "dismissed",
        Cancelled => "cancelled",
        Rescheduled => "rescheduled",
        Overdue => "overdue",
        Missed => "missed",
        WorkflowStarted => "workflow_started",
        WorkflowStartFailed => "workflow_start_failed",
        DeliveryLinked => "delivery_linked",
    }
);

string_enum!(
    /// Go ActorKind. User is the default so that a zero-valued TransitionInput maps to
    /// the Go fallback of nonEmptyActor(input.ActorKind, ActorKindUser).
    ActorKind {
        User => "user",
        System => "system",
        Reminder => "reminder",
    }
);

string_enum!(
    /// Go FollowUpLinkKind.
    FollowUpLinkKind {
        CalendarOperation => "calendar_operation",
        MailOperation => "mail_operation",
        Run => "run",
        Workflow => "workflow",
    }
);

/// Go ErrReminder* package error set.
#[derive(Debug, thiserror::Error, Clone, PartialEq, Eq)]
pub enum ReminderError {
    #[error("reminder not found")]
    ReminderNotFound,
    #[error("reminder occurrence not found")]
    OccurrenceNotFound,
    #[error("reminder trigger is invalid")]
    InvalidTrigger,
    #[error("reminder occurrence is not actionable")]
    InvalidState,
    #[error("snoozedUntil is required")]
    SnoozeRequired,
    #[error("workflowLaunchConfig is required for launch_workflow reminders")]
    WorkflowConfigRequired,
    #[error("reminder behavior is unsupported")]
    UnsupportedBehavior,
    #[error("title is required")]
    TitleRequired,
    #[error("store: {0}")]
    Store(String),
    #[error("encode: {0}")]
    Encode(String),
    #[error("decode: {0}")]
    Decode(String),
    #[error("{0}")]
    Other(String),
}

/// Go Reminder.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Reminder {
    pub reminder_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub environment_scope: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    pub title: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub details: String,
    pub behavior_mode: BehaviorMode,
    pub trigger: Trigger,
    pub current_state: State,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub next_due_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub active_occurrence_id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub workflow_launch_config: Option<WorkflowLaunchConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub follow_up_link: Option<FollowUpLink>,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cancelled_at: Option<DateTime<Utc>>,
}

impl Default for Reminder {
    fn default() -> Self {
        Self {
            reminder_id: String::new(),
            environment_scope: String::new(),
            tenant_id: String::new(),
            title: String::new(),
            details: String::new(),
            behavior_mode: BehaviorMode::NotifyOnly,
            trigger: Trigger::default(),
            current_state: State::Pending,
            next_due_at: None,
            active_occurrence_id: String::new(),
            workflow_launch_config: None,
            follow_up_link: None,
            created_at: DateTime::<Utc>::default(),
            updated_at: DateTime::<Utc>::default(),
            cancelled_at: None,
        }
    }
}

/// Go WorkflowLaunchConfig.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct WorkflowLaunchConfig {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub session_id: String,
    pub entrypoint: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub run_goal: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub workflow_goal: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub calendar_action: Option<CalendarAction>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mail_action: Option<MailAction>,
}

/// Go FollowUpLink.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct FollowUpLink {
    pub link_kind: FollowUpLinkKind,
    pub source_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub environment_scope: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub source_summary: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub source_display_state: String,
    #[serde(default, skip_serializing_if = "std::ops::Not::not")]
    pub stale: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_checked_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub diagnostic_failure: Option<DiagnosticFailureProjection>,
}

/// Go Occurrence.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Occurrence {
    pub occurrence_id: String,
    pub reminder_id: String,
    pub environment_scope: String,
    pub state: State,
    pub scheduled_for: DateTime<Utc>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub became_due_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub acknowledged_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub snoozed_until: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub completed_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub dismissed_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cancelled_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub overdue_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub missed_at: Option<DateTime<Utc>>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub run_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub workflow_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub latest_delivery_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub latest_delivery_status: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub latest_delivery_target_id: String,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

/// Go ActionRecord. previous_state/new_state are raw strings exactly like Go's State
/// (a string type): the created/rolled-over transitions record an empty previous state
/// (omitted from the document), matching the Go JSON.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ActionRecord {
    pub action_id: String,
    pub reminder_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub occurrence_id: String,
    pub action_kind: ActionKind,
    pub actor_kind: ActorKind,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub previous_state: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub new_state: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reason: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub run_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub workflow_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub delivery_id: String,
    pub created_at: DateTime<Utc>,
}

/// Go CreateInput.
#[derive(Debug, Clone, Default)]
pub struct CreateInput {
    pub title: String,
    pub details: String,
    pub behavior_mode: BehaviorMode,
    pub trigger: Trigger,
    pub workflow_launch_config: Option<WorkflowLaunchConfig>,
    pub follow_up_link: Option<FollowUpLink>,
}

/// Go TransitionInput.
#[derive(Debug, Clone, Default)]
pub struct TransitionInput {
    pub occurrence_id: String,
    pub reason: String,
    pub actor_kind: ActorKind,
    pub snoozed_until: Option<DateTime<Utc>>,
    pub trigger: Option<Trigger>,
}

/// Go OccurrenceFilter. state is None when no state filter applies (Go's zero-value
/// empty string).
#[derive(Debug, Clone, Default)]
pub struct OccurrenceFilter {
    pub reminder_id: String,
    pub state: Option<State>,
    pub scheduled_before: Option<DateTime<Utc>>,
    pub scheduled_after: Option<DateTime<Utc>>,
    pub run_id: String,
    pub workflow_id: String,
    pub delivery_id: String,
}

/// Go WorkflowLaunchResult.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct WorkflowLaunchResult {
    pub run_id: String,
    pub workflow_id: String,
}

/// Go WorkflowLauncher interface: launches the reminder's workflow for one occurrence.
///
/// Note on asynchrony: the Go seam is a plain interface method invoked synchronously by
/// the tick loop, and this port keeps that shape (like the kura-scheduler / kura-delivery
/// sync ports). An async variant can be layered on top later without changing the ledger
/// types.
pub trait WorkflowLauncher: Send + Sync {
    fn launch_reminder_workflow(
        &self,
        cfg: &WorkflowLaunchConfig,
        reminder_id: &str,
        occurrence_id: &str,
    ) -> Result<WorkflowLaunchResult, String>;
}

/// Go IsUnresolvedState.
#[must_use]
pub fn is_unresolved_state(state: State) -> bool {
    matches!(state, State::Due | State::Snoozed | State::Overdue)
}

/// Go IsTerminalState.
#[must_use]
pub fn is_terminal_state(state: State) -> bool {
    matches!(
        state,
        State::Acknowledged
            | State::Snoozed
            | State::Completed
            | State::Dismissed
            | State::Cancelled
            | State::Missed
    )
}
