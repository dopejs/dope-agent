package reminders

import (
	"context"
	"errors"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
)

var (
	ErrReminderNotFound            = errors.New("reminder not found")
	ErrReminderOccurrenceNotFound  = errors.New("reminder occurrence not found")
	ErrReminderInvalidTrigger      = errors.New("reminder trigger is invalid")
	ErrReminderInvalidState        = errors.New("reminder occurrence is not actionable")
	ErrReminderSnoozeRequired      = errors.New("snoozedUntil is required")
	ErrReminderWorkflowConfig      = errors.New("workflowLaunchConfig is required for launch_workflow reminders")
	ErrReminderUnsupportedBehavior = errors.New("reminder behavior is unsupported")
)

type BehaviorMode string

const (
	BehaviorModeNotifyOnly     BehaviorMode = "notify_only"
	BehaviorModeLaunchWorkflow BehaviorMode = "launch_workflow"
)

type State string

const (
	StatePending      State = "pending"
	StateDue          State = "due"
	StateAcknowledged State = "acknowledged"
	StateSnoozed      State = "snoozed"
	StateCompleted    State = "completed"
	StateDismissed    State = "dismissed"
	StateCancelled    State = "cancelled"
	StateOverdue      State = "overdue"
	StateMissed       State = "missed"
)

type ActionKind string

const (
	ActionKindCreated             ActionKind = "created"
	ActionKindDue                 ActionKind = "due"
	ActionKindAcknowledged        ActionKind = "acknowledged"
	ActionKindSnoozed             ActionKind = "snoozed"
	ActionKindCompleted           ActionKind = "completed"
	ActionKindDismissed           ActionKind = "dismissed"
	ActionKindCancelled           ActionKind = "cancelled"
	ActionKindRescheduled         ActionKind = "rescheduled"
	ActionKindOverdue             ActionKind = "overdue"
	ActionKindMissed              ActionKind = "missed"
	ActionKindWorkflowStarted     ActionKind = "workflow_started"
	ActionKindWorkflowStartFailed ActionKind = "workflow_start_failed"
	ActionKindDeliveryLinked      ActionKind = "delivery_linked"
)

type ActorKind string

const (
	ActorKindSystem   ActorKind = "system"
	ActorKindUser     ActorKind = "user"
	ActorKindReminder ActorKind = "reminder"
)

type FollowUpLinkKind string

const (
	FollowUpLinkKindCalendarOperation FollowUpLinkKind = "calendar_operation"
	FollowUpLinkKindMailOperation     FollowUpLinkKind = "mail_operation"
	FollowUpLinkKindRun               FollowUpLinkKind = "run"
	FollowUpLinkKindWorkflow          FollowUpLinkKind = "workflow"
)

type Reminder struct {
	ReminderID           string                `json:"reminderId"`
	EnvironmentScope     string                `json:"environmentScope"`
	TenantID             string                `json:"tenantId,omitempty"`
	Title                string                `json:"title"`
	Details              string                `json:"details,omitempty"`
	BehaviorMode         BehaviorMode          `json:"behaviorMode"`
	Trigger              scheduler.Trigger     `json:"trigger"`
	CurrentState         State                 `json:"currentState"`
	NextDueAt            *time.Time            `json:"nextDueAt,omitempty"`
	ActiveOccurrenceID   string                `json:"activeOccurrenceId,omitempty"`
	WorkflowLaunchConfig *WorkflowLaunchConfig `json:"workflowLaunchConfig,omitempty"`
	FollowUpLink         *FollowUpLink         `json:"followUpLink,omitempty"`
	CreatedAt            time.Time             `json:"createdAt"`
	UpdatedAt            time.Time             `json:"updatedAt"`
	CancelledAt          *time.Time            `json:"cancelledAt,omitempty"`
}

type WorkflowLaunchConfig struct {
	SessionID      string           `json:"sessionId,omitempty"`
	Entrypoint     string           `json:"entrypoint"`
	RunGoal        string           `json:"runGoal,omitempty"`
	WorkflowGoal   string           `json:"workflowGoal,omitempty"`
	CalendarAction *calendar.Action `json:"calendarAction,omitempty"`
	MailAction     *mail.Action     `json:"mailAction,omitempty"`
}

type FollowUpLink struct {
	LinkKind           FollowUpLinkKind `json:"linkKind"`
	SourceID           string           `json:"sourceId"`
	EnvironmentScope   string           `json:"environmentScope,omitempty"`
	SourceSummary      string           `json:"sourceSummary,omitempty"`
	SourceDisplayState string           `json:"sourceDisplayState,omitempty"`
	Stale              bool             `json:"stale,omitempty"`
	LastCheckedAt      *time.Time       `json:"lastCheckedAt,omitempty"`
}

type Occurrence struct {
	OccurrenceID           string     `json:"occurrenceId"`
	ReminderID             string     `json:"reminderId"`
	EnvironmentScope       string     `json:"environmentScope"`
	State                  State      `json:"state"`
	ScheduledFor           time.Time  `json:"scheduledFor"`
	BecameDueAt            *time.Time `json:"becameDueAt,omitempty"`
	AcknowledgedAt         *time.Time `json:"acknowledgedAt,omitempty"`
	SnoozedUntil           *time.Time `json:"snoozedUntil,omitempty"`
	CompletedAt            *time.Time `json:"completedAt,omitempty"`
	DismissedAt            *time.Time `json:"dismissedAt,omitempty"`
	CancelledAt            *time.Time `json:"cancelledAt,omitempty"`
	OverdueAt              *time.Time `json:"overdueAt,omitempty"`
	MissedAt               *time.Time `json:"missedAt,omitempty"`
	RunID                  string     `json:"runId,omitempty"`
	WorkflowID             string     `json:"workflowId,omitempty"`
	LatestDeliveryID       string     `json:"latestDeliveryId,omitempty"`
	LatestDeliveryStatus   string     `json:"latestDeliveryStatus,omitempty"`
	LatestDeliveryTargetID string     `json:"latestDeliveryTargetId,omitempty"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}

type ActionRecord struct {
	ActionID      string     `json:"actionId"`
	ReminderID    string     `json:"reminderId"`
	OccurrenceID  string     `json:"occurrenceId,omitempty"`
	ActionKind    ActionKind `json:"actionKind"`
	ActorKind     ActorKind  `json:"actorKind"`
	PreviousState State      `json:"previousState,omitempty"`
	NewState      State      `json:"newState,omitempty"`
	Reason        string     `json:"reason,omitempty"`
	RunID         string     `json:"runId,omitempty"`
	WorkflowID    string     `json:"workflowId,omitempty"`
	DeliveryID    string     `json:"deliveryId,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

type CreateInput struct {
	Title                string
	Details              string
	BehaviorMode         BehaviorMode
	Trigger              scheduler.Trigger
	WorkflowLaunchConfig *WorkflowLaunchConfig
	FollowUpLink         *FollowUpLink
}

type TransitionInput struct {
	OccurrenceID string
	Reason       string
	ActorKind    ActorKind
	SnoozedUntil *time.Time
	Trigger      *scheduler.Trigger
}

type OccurrenceFilter struct {
	ReminderID      string
	State           State
	ScheduledBefore *time.Time
	ScheduledAfter  *time.Time
	RunID           string
	WorkflowID      string
	DeliveryID      string
}

type WorkflowLaunchResult struct {
	RunID      string
	WorkflowID string
}

type WorkflowLauncher interface {
	LaunchReminderWorkflow(ctx context.Context, cfg WorkflowLaunchConfig, reminderID, occurrenceID string) (WorkflowLaunchResult, error)
}

func IsUnresolvedState(state State) bool {
	switch state {
	case StateDue, StateSnoozed, StateOverdue:
		return true
	default:
		return false
	}
}

func IsTerminalState(state State) bool {
	switch state {
	case StateAcknowledged, StateSnoozed, StateCompleted, StateDismissed, StateCancelled, StateMissed:
		return true
	default:
		return false
	}
}
