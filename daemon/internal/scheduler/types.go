package scheduler

import "time"

type ScheduleKind string

const (
	ScheduleKindOneTime   ScheduleKind = "one_time"
	ScheduleKindRecurring ScheduleKind = "recurring"
)

type ScheduleStatus string

const (
	ScheduleStatusScheduled      ScheduleStatus = "scheduled"
	ScheduleStatusActive         ScheduleStatus = "active"
	ScheduleStatusPaused         ScheduleStatus = "paused"
	ScheduleStatusCancelled      ScheduleStatus = "cancelled"
	ScheduleStatusCompleted      ScheduleStatus = "completed"
	ScheduleStatusDispatchFailed ScheduleStatus = "dispatch_failed"
)

type TriggerKind string

const (
	TriggerKindOnce TriggerKind = "once"
	TriggerKindCron TriggerKind = "cron"
)

type TargetKind string

const (
	TargetKindRun      TargetKind = "run"
	TargetKindWorkflow TargetKind = "workflow"
)

type TriggerSource string

const (
	TriggerSourceNormal  TriggerSource = "normal"
	TriggerSourceCatchUp TriggerSource = "catch_up"
	TriggerSourceRetry   TriggerSource = "retry"
)

type DispatchStatus string

const (
	DispatchStatusPending          DispatchStatus = "pending"
	DispatchStatusDispatching      DispatchStatus = "dispatching"
	DispatchStatusDispatched       DispatchStatus = "dispatched"
	DispatchStatusFailed           DispatchStatus = "failed"
	DispatchStatusMissed           DispatchStatus = "missed"
	DispatchStatusSkippedPaused    DispatchStatus = "skipped_paused"
	DispatchStatusSkippedOverlap   DispatchStatus = "skipped_overlap"
	DispatchStatusSkippedCancelled DispatchStatus = "skipped_cancelled"
	DispatchStatusExhausted        DispatchStatus = "exhausted"
)

type DownstreamStatus string

const (
	DownstreamStatusNone        DownstreamStatus = "none"
	DownstreamStatusRunning     DownstreamStatus = "running"
	DownstreamStatusCompleted   DownstreamStatus = "completed"
	DownstreamStatusFailed      DownstreamStatus = "failed"
	DownstreamStatusCancelled   DownstreamStatus = "cancelled"
	DownstreamStatusInterrupted DownstreamStatus = "interrupted"
)

type RetryBackoffKind string

const (
	RetryBackoffFixed       RetryBackoffKind = "fixed"
	RetryBackoffExponential RetryBackoffKind = "exponential"
)

type Schedule struct {
	ScheduleID       string            `json:"scheduleId"`
	EnvironmentScope string            `json:"environmentScope,omitempty"`
	Kind             ScheduleKind      `json:"kind"`
	Status           ScheduleStatus    `json:"status"`
	TargetRefID      string            `json:"targetRefId"`
	Trigger          Trigger           `json:"trigger"`
	Target           Target            `json:"target"`
	RetryPolicy      RetryPolicy       `json:"retryPolicy"`
	NextDueAt        *time.Time        `json:"nextDueAt,omitempty"`
	LastAttemptAt    *time.Time        `json:"lastAttemptAt,omitempty"`
	LastOutcome      string            `json:"lastOutcome,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
	PausedAt         *time.Time        `json:"pausedAt,omitempty"`
	CancelledAt      *time.Time        `json:"cancelledAt,omitempty"`
	CompletedAt      *time.Time        `json:"completedAt,omitempty"`
	Attempts         []DispatchAttempt `json:"attempts,omitempty"`
}

type Trigger struct {
	Kind      TriggerKind `json:"kind"`
	FireAt    *time.Time  `json:"fireAt,omitempty"`
	CronExpr  string      `json:"cronExpr,omitempty"`
	Timezone  string      `json:"timezone,omitempty"`
	NextDueAt *time.Time  `json:"nextDueAt,omitempty"`
}

type Target struct {
	Kind      TargetKind      `json:"kind"`
	Revision  int             `json:"revision"`
	Active    bool            `json:"active"`
	Run       *RunTarget      `json:"run,omitempty"`
	Workflow  *WorkflowTarget `json:"workflow,omitempty"`
	Summary   string          `json:"summary,omitempty"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type RunTarget struct {
	SessionID  string `json:"sessionId,omitempty"`
	Entrypoint string `json:"entrypoint"`
	Goal       string `json:"goal,omitempty"`
}

type WorkflowTarget struct {
	SessionID    string `json:"sessionId,omitempty"`
	Entrypoint   string `json:"entrypoint"`
	RunGoal      string `json:"runGoal,omitempty"`
	WorkflowGoal string `json:"workflowGoal,omitempty"`
}

type RetryPolicy struct {
	MaxRetries       int              `json:"maxRetries"`
	BackoffKind      RetryBackoffKind `json:"backoffKind"`
	BaseDelaySeconds int              `json:"baseDelaySeconds"`
	MaxDelaySeconds  int              `json:"maxDelaySeconds"`
}

type DispatchAttempt struct {
	AttemptID              string           `json:"scheduleAttemptId"`
	ScheduleID             string           `json:"scheduleId"`
	DueAt                  time.Time        `json:"dueAt"`
	TriggerSource          TriggerSource    `json:"triggerSource"`
	DispatchStatus         DispatchStatus   `json:"dispatchStatus"`
	FailureClass           string           `json:"failureClass,omitempty"`
	FailureReason          string           `json:"failureReason,omitempty"`
	RetryCount             int              `json:"retryCount"`
	RetryBudget            int              `json:"retryBudget"`
	NextRetryAt            *time.Time       `json:"nextRetryAt,omitempty"`
	ResolvedTargetRevision int              `json:"resolvedTargetRevision,omitempty"`
	RunID                  string           `json:"runId,omitempty"`
	WorkflowID             string           `json:"workflowId,omitempty"`
	DownstreamStatus       DownstreamStatus `json:"downstreamStatus"`
	SkippedReason          string           `json:"skippedReason,omitempty"`
	MissedCount            int              `json:"missedCount,omitempty"`
	CreatedAt              time.Time        `json:"createdAt"`
	UpdatedAt              time.Time        `json:"updatedAt"`
}

func IsTerminalScheduleStatus(status ScheduleStatus) bool {
	switch status {
	case ScheduleStatusCancelled, ScheduleStatusCompleted, ScheduleStatusDispatchFailed:
		return true
	default:
		return false
	}
}

func IsTerminalDispatchStatus(status DispatchStatus) bool {
	switch status {
	case DispatchStatusDispatched, DispatchStatusMissed, DispatchStatusSkippedPaused, DispatchStatusSkippedOverlap, DispatchStatusSkippedCancelled, DispatchStatusExhausted:
		return true
	default:
		return false
	}
}

func IsActiveDownstreamStatus(status DownstreamStatus) bool {
	return status == DownstreamStatusRunning
}
