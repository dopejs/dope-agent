// Package routine implements the routine builder (Roadmap 66): explicit, user-defined proactive
// routines that compile to the existing schedule + workflow + delivery planes. Routines are
// explicit configuration — no autonomous planning and no memory. Routine edits create new
// versions and never rewrite prior execution evidence (the underlying schedule attempts).
package routine

import "time"

// TriggerKind selects when a routine fires.
type TriggerKind string

const (
	TriggerKindCron TriggerKind = "cron"
	TriggerKindOnce TriggerKind = "once"
)

// State is the routine lifecycle state.
type State string

const (
	StateActive    State = "active"
	StatePaused    State = "paused"
	StateCancelled State = "cancelled"
)

// Trigger is the explicit firing schedule for a routine.
type Trigger struct {
	Kind     TriggerKind `json:"kind"`
	CronExpr string      `json:"cronExpr,omitempty"`
	Timezone string      `json:"timezone,omitempty"`
	FireAt   *time.Time  `json:"fireAt,omitempty"`
}

// Workflow is the explicit work a routine runs each fire.
type Workflow struct {
	Entrypoint string `json:"entrypoint,omitempty"` // defaults to "operator"
	Goal       string `json:"goal"`
}

// Definition is the explicit routine configuration a user composes/approves.
type Definition struct {
	Name                 string   `json:"name"`
	Trigger              Trigger  `json:"trigger"`
	Workflow             Workflow `json:"workflow"`
	ApprovalExpectation  string   `json:"approvalExpectation,omitempty"`  // e.g. "ask" | "allow"
	DeliveryPreferenceID string   `json:"deliveryPreferenceId,omitempty"` // delivery summary preference
	MaxRetries           int      `json:"maxRetries,omitempty"`
}

// Version is a snapshot of a routine definition plus the schedule it compiled to. Prior versions
// keep their schedule id so their execution evidence remains inspectable after edits (FR-003).
type Version struct {
	Version    int        `json:"version"`
	Definition Definition `json:"definition"`
	ScheduleID string     `json:"scheduleId,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// Routine is a product-level proactive routine.
type Routine struct {
	RoutineID         string     `json:"routineId"`
	EnvironmentScope  string     `json:"environmentScope"`
	Name              string     `json:"name"`
	State             State      `json:"state"`
	CurrentVersion    int        `json:"currentVersion"`
	CurrentScheduleID string     `json:"currentScheduleId,omitempty"`
	Definition        Definition `json:"definition"`
	Versions          []Version  `json:"versions"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

// Preview is the compiled, pre-activation projection of a routine definition: what schedule and
// workflow it compiles to, and the approval/delivery/quota expectations to confirm (FR-004).
type Preview struct {
	ScheduleKind         string `json:"scheduleKind"` // one_time | recurring
	TriggerSummary       string `json:"triggerSummary"`
	WorkflowSummary      string `json:"workflowSummary"`
	ApprovalExpectation  string `json:"approvalExpectation"`
	DeliveryPreferenceID string `json:"deliveryPreferenceId,omitempty"`
	RetrySummary         string `json:"retrySummary"`
}
