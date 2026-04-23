package orchestration

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
)

type WorkflowStatus string

const (
	WorkflowStatusPlanning       WorkflowStatus = "planning"
	WorkflowStatusPlanningFailed WorkflowStatus = "planning_failed"
	WorkflowStatusPlanned        WorkflowStatus = "planned"
	WorkflowStatusRunning        WorkflowStatus = "running"
	WorkflowStatusBlocked        WorkflowStatus = "blocked"
	WorkflowStatusCompleted      WorkflowStatus = "completed"
	WorkflowStatusPartialFailed  WorkflowStatus = "partial_failed"
	WorkflowStatusFailed         WorkflowStatus = "failed"
	WorkflowStatusCancelled      WorkflowStatus = "cancelled"
	WorkflowStatusInterrupted    WorkflowStatus = "interrupted"
)

type StepStatus string

const (
	StepStatusPlanned           StepStatus = "planned"
	StepStatusWaitingDependency StepStatus = "waiting_dependency"
	StepStatusReady             StepStatus = "ready"
	StepStatusBlocked           StepStatus = "blocked"
	StepStatusRunning           StepStatus = "running"
	StepStatusCompleted         StepStatus = "completed"
	StepStatusFailed            StepStatus = "failed"
	StepStatusCancelled         StepStatus = "cancelled"
	StepStatusInterrupted       StepStatus = "interrupted"
	StepStatusSkipped           StepStatus = "skipped"
)

type DependencyType string

const (
	DependencyTypeSuccess    DependencyType = "success"
	DependencyTypeFailure    DependencyType = "failure"
	DependencyTypeCompletion DependencyType = "completion"
)

type HandoffStatus string

const (
	HandoffStatusPending   HandoffStatus = "pending"
	HandoffStatusAvailable HandoffStatus = "available"
	HandoffStatusConsumed  HandoffStatus = "consumed"
	HandoffStatusInvalid   HandoffStatus = "invalid"
)

type BlockedReason string

const (
	BlockedReasonApprovalDenied      BlockedReason = "approval_denied"
	BlockedReasonPolicyBlocked       BlockedReason = "policy_blocked"
	BlockedReasonConsumerUnavailable BlockedReason = "consumer_unavailable"
)

type CreateWorkflowInput struct {
	Goal           string           `json:"goal,omitempty"`
	CalendarAction *calendar.Action `json:"calendarAction,omitempty"`
	MailAction     *mail.Action     `json:"mailAction,omitempty"`
}

type Workflow struct {
	WorkflowID             string         `json:"workflowId"`
	RunID                  string         `json:"runId"`
	ScheduleID             string         `json:"scheduleId,omitempty"`
	ScheduleAttemptID      string         `json:"scheduleAttemptId,omitempty"`
	ReminderID             string         `json:"reminderId,omitempty"`
	ReminderOccurrenceID   string         `json:"reminderOccurrenceId,omitempty"`
	EnvironmentScope       string         `json:"environmentScope,omitempty"`
	Goal                   string         `json:"goal"`
	Status                 WorkflowStatus `json:"status"`
	PlanSummary            string         `json:"planSummary,omitempty"`
	FailureSummary         string         `json:"failureSummary,omitempty"`
	LatestDeliveryID       string         `json:"latestDeliveryId,omitempty"`
	LatestDeliveryStatus   string         `json:"latestDeliveryStatus,omitempty"`
	LatestDeliveryTargetID string         `json:"latestDeliveryTargetId,omitempty"`
	CreatedAt              time.Time      `json:"createdAt"`
	UpdatedAt              time.Time      `json:"updatedAt"`
	StartedAt              *time.Time     `json:"startedAt,omitempty"`
	CompletedAt            *time.Time     `json:"completedAt,omitempty"`
	InterruptedAt          *time.Time     `json:"interruptedAt,omitempty"`
	Steps                  []WorkflowStep `json:"steps,omitempty"`
	Dependencies           []Dependency   `json:"dependencies,omitempty"`
	Handoffs               []Handoff      `json:"handoffs,omitempty"`
}

type WorkflowStep struct {
	WorkflowStepID             string                        `json:"workflowStepId"`
	WorkflowID                 string                        `json:"workflowId"`
	Title                      string                        `json:"title"`
	Position                   int                           `json:"position"`
	ConsumerKind               string                        `json:"consumerKind"`
	ConsumerID                 string                        `json:"consumerId"`
	ToolName                   string                        `json:"toolName"`
	Input                      any                           `json:"input,omitempty"`
	Status                     StepStatus                    `json:"status"`
	SelectionRationale         string                        `json:"selectionRationale,omitempty"`
	ApprovalModeExpected       string                        `json:"approvalModeExpected,omitempty"`
	DependencyIDs              []string                      `json:"dependencyIds,omitempty"`
	RuntimeStepID              string                        `json:"runtimeStepId,omitempty"`
	ActiveToolCallID           string                        `json:"activeToolCallId,omitempty"`
	AttemptCount               int                           `json:"attemptCount"`
	MaxAttempts                int                           `json:"maxAttempts"`
	LastFailureClass           string                        `json:"lastFailureClass,omitempty"`
	BlockedReason              string                        `json:"blockedReason,omitempty"`
	SideEffectsVisible         bool                          `json:"sideEffectsVisible,omitempty"`
	OutputSummary              string                        `json:"outputSummary,omitempty"`
	ComputerUseSessionID       string                        `json:"computerUseSessionId,omitempty"`
	ComputerUseActionIDs       []string                      `json:"computerUseActionIds,omitempty"`
	ComputerUseArtifacts       []computeruse.Artifact        `json:"computerUseArtifacts,omitempty"`
	IntegrationBindings        []integrations.BindingSummary `json:"integrationBindings,omitempty"`
	CalendarOperationSummaries []calendar.OperationSummary   `json:"calendarOperationSummaries,omitempty"`
	MailOperationSummaries     []mail.OperationSummary       `json:"mailOperationSummaries,omitempty"`
	CreatedAt                  time.Time                     `json:"createdAt"`
	UpdatedAt                  time.Time                     `json:"updatedAt"`
}

type Dependency struct {
	DependencyID       string         `json:"dependencyId"`
	WorkflowID         string         `json:"workflowId"`
	FromWorkflowStepID string         `json:"fromWorkflowStepId"`
	ToWorkflowStepID   string         `json:"toWorkflowStepId"`
	DependencyType     DependencyType `json:"dependencyType"`
	Reason             string         `json:"reason,omitempty"`
}

type Handoff struct {
	HandoffID          string        `json:"handoffId"`
	WorkflowID         string        `json:"workflowId"`
	FromWorkflowStepID string        `json:"fromWorkflowStepId"`
	ToWorkflowStepID   string        `json:"toWorkflowStepId"`
	Status             HandoffStatus `json:"status"`
	PayloadSummary     string        `json:"payloadSummary,omitempty"`
	SourcePath         string        `json:"sourcePath,omitempty"`
	ConsumedAt         *time.Time    `json:"consumedAt,omitempty"`
	InvalidReason      string        `json:"invalidReason,omitempty"`
}

func IsTerminalWorkflowStatus(status WorkflowStatus) bool {
	switch status {
	case WorkflowStatusPlanningFailed, WorkflowStatusCompleted, WorkflowStatusPartialFailed, WorkflowStatusFailed, WorkflowStatusCancelled, WorkflowStatusInterrupted:
		return true
	default:
		return false
	}
}

func IsTerminalStepStatus(status StepStatus) bool {
	switch status {
	case StepStatusCompleted, StepStatusFailed, StepStatusCancelled, StepStatusInterrupted, StepStatusSkipped, StepStatusBlocked:
		return true
	default:
		return false
	}
}
