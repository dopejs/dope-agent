package delivery

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
)

type TargetKind string

const (
	TargetKindConnectorRoute TargetKind = "connector_route"
	TargetKindTestSink       TargetKind = "test_sink"
)

type TargetStatus string

const (
	TargetStatusActive        TargetStatus = "active"
	TargetStatusDisabled      TargetStatus = "disabled"
	TargetStatusUnreachable   TargetStatus = "unreachable"
	TargetStatusMisconfigured TargetStatus = "misconfigured"
)

type PreferenceScopeKind string

const (
	PreferenceScopeUserDefault         PreferenceScopeKind = "user_default"
	PreferenceScopeIntegrationOverride PreferenceScopeKind = "integration_override"
)

type ResultClass string

const (
	ResultClassRoutineSuccess ResultClass = "routine_success"
	ResultClassUrgent         ResultClass = "urgent"
	ResultClassFailure        ResultClass = "failure"
)

type DeliveryMode string

const (
	DeliveryModeImmediate  DeliveryMode = "immediate"
	DeliveryModeDigest     DeliveryMode = "digest"
	DeliveryModeSuppressed DeliveryMode = "suppressed"
)

type OutcomeStatus string

const (
	OutcomeStatusPending     OutcomeStatus = "pending"
	OutcomeStatusQueued      OutcomeStatus = "queued"
	OutcomeStatusDispatching OutcomeStatus = "dispatching"
	OutcomeStatusDelivered   OutcomeStatus = "delivered"
	OutcomeStatusSuppressed  OutcomeStatus = "suppressed"
	OutcomeStatusFailed      OutcomeStatus = "failed"
)

type AttemptStatus string

const (
	AttemptStatusRunning          AttemptStatus = "running"
	AttemptStatusDelivered        AttemptStatus = "delivered"
	AttemptStatusRetryableFailure AttemptStatus = "retryable_failure"
	AttemptStatusTerminalFailure  AttemptStatus = "terminal_failure"
)

type SummaryWindowStatus string

const (
	SummaryWindowStatusOpen        SummaryWindowStatus = "open"
	SummaryWindowStatusReady       SummaryWindowStatus = "ready"
	SummaryWindowStatusDispatching SummaryWindowStatus = "dispatching"
	SummaryWindowStatusDelivered   SummaryWindowStatus = "delivered"
	SummaryWindowStatusFailed      SummaryWindowStatus = "failed"
	SummaryWindowStatusCancelled   SummaryWindowStatus = "cancelled"
)

type ConnectorBinding struct {
	ConnectorID string `json:"connectorId,omitempty"`
	ChannelID   string `json:"channelId,omitempty"`
	PeerID      string `json:"peerId,omitempty"`
	ThreadID    string `json:"threadId,omitempty"`
}

type SummaryPolicy struct {
	RoutineSuccessMode DeliveryMode `json:"routineSuccessMode,omitempty"`
	WindowMinutes      int          `json:"windowMinutes,omitempty"`
}

type SuppressionPolicy struct {
	SuppressRoutineSuccess bool `json:"suppressRoutineSuccess,omitempty"`
	SuppressUrgent         bool `json:"suppressUrgent,omitempty"`
	SuppressFailure        bool `json:"suppressFailure,omitempty"`
}

type DeliveryTarget struct {
	TargetID          string            `json:"targetId"`
	DisplayName       string            `json:"displayName"`
	EnvironmentScope  string            `json:"environmentScope"`
	TargetKind        TargetKind        `json:"targetKind"`
	Status            TargetStatus      `json:"status"`
	ConnectorBinding  *ConnectorBinding `json:"connectorBinding,omitempty"`
	AddressSummary    string            `json:"addressSummary,omitempty"`
	SupportsImmediate bool              `json:"supportsImmediate"`
	SupportsDigest    bool              `json:"supportsDigest"`
	LastValidatedAt   *time.Time        `json:"lastValidatedAt,omitempty"`
	CreatedAt         time.Time         `json:"createdAt"`
	UpdatedAt         time.Time         `json:"updatedAt"`
}

type DeliveryPreference struct {
	PreferenceID            string                 `json:"preferenceId"`
	EnvironmentScope        string                 `json:"environmentScope"`
	ScopeKind               PreferenceScopeKind    `json:"scopeKind"`
	IntegrationID           string                 `json:"integrationId,omitempty"`
	PreferredTargetsByClass map[ResultClass]string `json:"preferredTargetsByClass"`
	SummaryPolicy           SummaryPolicy          `json:"summaryPolicy,omitempty"`
	SuppressionPolicy       SuppressionPolicy      `json:"suppressionPolicy,omitempty"`
	Active                  bool                   `json:"active"`
	CreatedAt               time.Time              `json:"createdAt"`
	UpdatedAt               time.Time              `json:"updatedAt"`
}

type DeliveryAttempt struct {
	AttemptID                  string        `json:"attemptId"`
	DeliveryID                 string        `json:"deliveryId"`
	AttemptNumber              int           `json:"attemptNumber"`
	TargetID                   string        `json:"targetId"`
	TransportKind              string        `json:"transportKind"`
	Status                     AttemptStatus `json:"status"`
	FailureClass               string        `json:"failureClass,omitempty"`
	FailureReason              string        `json:"failureReason,omitempty"`
	NextRetryAt                *time.Time    `json:"nextRetryAt,omitempty"`
	ConnectorMessageDeliveryID string        `json:"connectorMessageDeliveryId,omitempty"`
	TransportReceiptSummary    string        `json:"transportReceiptSummary,omitempty"`
	StartedAt                  time.Time     `json:"startedAt"`
	CompletedAt                *time.Time    `json:"completedAt,omitempty"`
}

type DeliveryOutcome struct {
	DeliveryID                 string                                    `json:"deliveryId"`
	EnvironmentScope           string                                    `json:"environmentScope"`
	SourceKind                 string                                    `json:"sourceKind"`
	SourceID                   string                                    `json:"sourceId"`
	RunID                      string                                    `json:"runId,omitempty"`
	WorkflowID                 string                                    `json:"workflowId,omitempty"`
	ScheduleID                 string                                    `json:"scheduleId,omitempty"`
	ScheduleAttemptID          string                                    `json:"scheduleAttemptId,omitempty"`
	IntegrationID              string                                    `json:"integrationId,omitempty"`
	ResultClass                ResultClass                               `json:"resultClass"`
	Mode                       DeliveryMode                              `json:"mode"`
	Status                     OutcomeStatus                             `json:"status"`
	ChosenTargetID             string                                    `json:"chosenTargetId,omitempty"`
	PreferenceID               string                                    `json:"preferenceId,omitempty"`
	SummaryWindowID            string                                    `json:"summaryWindowId,omitempty"`
	PayloadPreview             string                                    `json:"payloadPreview,omitempty"`
	SuppressionReason          string                                    `json:"suppressionReason,omitempty"`
	CalendarOperationIDs       []string                                  `json:"calendarOperationIds,omitempty"`
	CalendarOperationSummaries []calendar.OperationSummary               `json:"calendarOperationSummaries,omitempty"`
	MailOperationIDs           []string                                  `json:"mailOperationIds,omitempty"`
	MailOperationSummaries     []mail.OperationSummary                   `json:"mailOperationSummaries,omitempty"`
	Attempts                   []DeliveryAttempt                         `json:"attempts,omitempty"`
	DiagnosticFailure          *integrations.DiagnosticFailureProjection `json:"diagnosticFailure,omitempty"`
	CreatedAt                  time.Time                                 `json:"createdAt"`
	UpdatedAt                  time.Time                                 `json:"updatedAt"`
	FinalizedAt                *time.Time                                `json:"finalizedAt,omitempty"`
}

type SummaryWindow struct {
	SummaryWindowID   string              `json:"summaryWindowId"`
	EnvironmentScope  string              `json:"environmentScope"`
	TargetID          string              `json:"targetId"`
	PreferenceID      string              `json:"preferenceId"`
	Status            SummaryWindowStatus `json:"status"`
	WindowStartedAt   time.Time           `json:"windowStartedAt"`
	WindowEndsAt      time.Time           `json:"windowEndsAt"`
	ResultCount       int                 `json:"resultCount"`
	EmittedDeliveryID string              `json:"emittedDeliveryId,omitempty"`
	CreatedAt         time.Time           `json:"createdAt"`
	UpdatedAt         time.Time           `json:"updatedAt"`
}

type OutcomeInput struct {
	SourceKind        string
	SourceID          string
	RunID             string
	WorkflowID        string
	ScheduleID        string
	ScheduleAttemptID string
	IntegrationID     string
	ResultClass       ResultClass
	PayloadPreview    string
}

type OutcomeFilter struct {
	SourceKind    string
	SourceID      string
	RunID         string
	WorkflowID    string
	ScheduleID    string
	IntegrationID string
	Status        OutcomeStatus
	TargetID      string
}

type SendResult struct {
	TransportKind              string
	ReceiptSummary             string
	ConnectorMessageDeliveryID string
}
