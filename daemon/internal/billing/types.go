package billing

import "time"

type Category string

const (
	CategoryRunLaunches              Category = "run_launches"
	CategoryWorkflowLaunches         Category = "workflow_launches"
	CategoryRuntimeToolCalls         Category = "runtime_tool_calls"
	CategoryLiveValidationAttempts   Category = "live_validation_attempts"
	CategoryIntegrationOperations    Category = "integration_operations"
	CategoryArtifactStorageBytes     Category = "artifact_storage_bytes"
	CategoryReplayEvaluationAttempts Category = "replay_evaluation_attempts"
)

type Unit string

const (
	UnitCount    Unit = "count"
	UnitBytes    Unit = "bytes"
	UnitAttempts Unit = "attempts"
)

type PeriodKind string

const (
	PeriodNone    PeriodKind = "none"
	PeriodDaily   PeriodKind = "daily"
	PeriodMonthly PeriodKind = "monthly"
)

type EnforcementMode string

const (
	EnforcementModeEnforced      EnforcementMode = "enforced"
	EnforcementModeUnlimited     EnforcementMode = "unlimited"
	EnforcementModeNotMeasurable EnforcementMode = "not_measurable"
)

type PlanStatus string

const (
	PlanStatusActive     PlanStatus = "active"
	PlanStatusScheduled  PlanStatus = "scheduled"
	PlanStatusDisabled   PlanStatus = "disabled"
	PlanStatusSuperseded PlanStatus = "superseded"
)

type ReservationStatus string

const (
	ReservationStatusReserved             ReservationStatus = "reserved"
	ReservationStatusCommitted            ReservationStatus = "committed"
	ReservationStatusReleased             ReservationStatus = "released"
	ReservationStatusRefunded             ReservationStatus = "refunded"
	ReservationStatusDenied               ReservationStatus = "denied"
	ReservationStatusOperatorActionNeeded ReservationStatus = "operator_action_needed"
)

type UsageEventKind string

const (
	UsageEventReservation      UsageEventKind = "reservation"
	UsageEventCommit           UsageEventKind = "commit"
	UsageEventRefund           UsageEventKind = "refund"
	UsageEventRelease          UsageEventKind = "release"
	UsageEventDenial           UsageEventKind = "denial"
	UsageEventManualAdjustment UsageEventKind = "manual_adjustment"
	UsageEventPeriodReset      UsageEventKind = "period_reset"
	UsageEventRecoveryDecision UsageEventKind = "recovery_decision"
	UsageEventPlanChanged      UsageEventKind = "plan_changed"
	UsageEventQuotaOverride    UsageEventKind = "quota_override_changed"
	UsageEventOverLimitCommit  UsageEventKind = "over_limit_commit"
	UsageEventRetentionPolicy  UsageEventKind = "retention_policy_changed"
)

type TenantPlan struct {
	PlanID                string          `json:"planId"`
	TenantID              string          `json:"tenantId"`
	PlanKey               string          `json:"planKey"`
	Status                PlanStatus      `json:"status"`
	EnforcementMode       EnforcementMode `json:"enforcementMode"`
	EffectiveAt           time.Time       `json:"effectiveAt"`
	SupersededAt          *time.Time      `json:"supersededAt,omitempty"`
	AssignedByPrincipalID string          `json:"assignedByPrincipalId,omitempty"`
	AssignmentReason      string          `json:"assignmentReason,omitempty"`
	Document              map[string]any  `json:"document,omitempty"`
}

type QuotaDefinition struct {
	QuotaDefinitionID string         `json:"quotaDefinitionId"`
	Category          Category       `json:"category"`
	Unit              Unit           `json:"unit"`
	PeriodKind        PeriodKind     `json:"periodKind"`
	PeriodAnchor      string         `json:"periodAnchor"`
	DefaultLimit      int64          `json:"defaultLimit"`
	CarryoverEnabled  bool           `json:"carryoverEnabled"`
	CarryoverMax      int64          `json:"carryoverMax,omitempty"`
	ReservationRule   string         `json:"reservationRule"`
	CommitRule        string         `json:"commitRule"`
	RefundRule        string         `json:"refundRule"`
	DenialReasonCode  string         `json:"denialReasonCode"`
	Active            bool           `json:"active"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
	Document          map[string]any `json:"document,omitempty"`
}

type QuotaOverride struct {
	QuotaOverrideID      string     `json:"quotaOverrideId"`
	TenantID             string     `json:"tenantId"`
	Category             Category   `json:"category"`
	Limit                *int64     `json:"limit,omitempty"`
	CarryoverEnabled     *bool      `json:"carryoverEnabled,omitempty"`
	CarryoverMax         *int64     `json:"carryoverMax,omitempty"`
	EffectiveAt          time.Time  `json:"effectiveAt"`
	ExpiresAt            *time.Time `json:"expiresAt,omitempty"`
	Reason               string     `json:"reason"`
	CreatedByPrincipalID string     `json:"createdByPrincipalId,omitempty"`
}

type QuotaPeriod struct {
	QuotaPeriodID         string     `json:"quotaPeriodId"`
	TenantID              string     `json:"tenantId"`
	Category              Category   `json:"category"`
	PeriodKind            PeriodKind `json:"periodKind"`
	PeriodStart           time.Time  `json:"periodStart"`
	PeriodEnd             time.Time  `json:"periodEnd"`
	CarryoverFromPeriodID string     `json:"carryoverFromPeriodId,omitempty"`
	Status                string     `json:"status"`
}

type UsageCounter struct {
	UsageCounterID  string    `json:"usageCounterId"`
	TenantID        string    `json:"tenantId"`
	Category        Category  `json:"category"`
	QuotaPeriodID   string    `json:"quotaPeriodId"`
	CommittedAmount int64     `json:"committedAmount"`
	ReservedAmount  int64     `json:"reservedAmount"`
	AdjustedAmount  int64     `json:"adjustedAmount"`
	CarryoverAmount int64     `json:"carryoverAmount"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type UsageReservation struct {
	ReservationID    string            `json:"reservationId"`
	TenantID         string            `json:"tenantId"`
	Category         Category          `json:"category"`
	QuotaPeriodID    string            `json:"quotaPeriodId"`
	OperationKey     string            `json:"operationKey"`
	AmountReserved   int64             `json:"amountReserved"`
	AmountCommitted  int64             `json:"amountCommitted"`
	AmountRefunded   int64             `json:"amountRefunded"`
	Status           ReservationStatus `json:"status"`
	ReservationPoint string            `json:"reservationPoint,omitempty"`
	CommitPoint      string            `json:"commitPoint,omitempty"`
	RefundPoint      string            `json:"refundPoint,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
	ExpiresAt        *time.Time        `json:"expiresAt,omitempty"`
	RecoveryReason   string            `json:"recoveryReason,omitempty"`
}

type UsageEvent struct {
	UsageEventID     string         `json:"usageEventId"`
	TenantID         string         `json:"tenantId"`
	Category         Category       `json:"category"`
	QuotaPeriodID    string         `json:"quotaPeriodId"`
	OperationKey     string         `json:"operationKey,omitempty"`
	EventKind        UsageEventKind `json:"eventKind"`
	Amount           int64          `json:"amount"`
	ReasonCode       string         `json:"reasonCode"`
	Reason           string         `json:"reason,omitempty"`
	ActorPrincipalID string         `json:"actorPrincipalId,omitempty"`
	Outcome          string         `json:"outcome"`
	CreatedAt        time.Time      `json:"createdAt"`
	Document         map[string]any `json:"document,omitempty"`
}

type QuotaDenial struct {
	DenialID          string    `json:"denialId"`
	TenantID          string    `json:"tenantId"`
	Category          Category  `json:"category,omitempty"`
	QuotaPeriodID     string    `json:"quotaPeriodId,omitempty"`
	OperationKey      string    `json:"operationKey"`
	ReasonCode        string    `json:"reasonCode"`
	RequestedAmount   int64     `json:"requestedAmount"`
	RemainingAmount   int64     `json:"remainingAmount"`
	GuardedEntryPoint string    `json:"guardedEntryPoint"`
	CreatedAt         time.Time `json:"createdAt"`
}

type ManualAdjustment struct {
	AdjustmentID         string    `json:"adjustmentId"`
	TenantID             string    `json:"tenantId"`
	Category             Category  `json:"category"`
	QuotaPeriodID        string    `json:"quotaPeriodId"`
	AmountDelta          int64     `json:"amountDelta"`
	Reason               string    `json:"reason"`
	CreatedByPrincipalID string    `json:"createdByPrincipalId,omitempty"`
	CreatedAt            time.Time `json:"createdAt"`
}

type AuditRetentionPolicy struct {
	TenantID             string     `json:"tenantId,omitempty"`
	RetentionMode        string     `json:"retentionMode"`
	RetentionPeriod      string     `json:"retentionPeriod,omitempty"`
	CreatedByPrincipalID string     `json:"createdByPrincipalId,omitempty"`
	Reason               string     `json:"reason,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
	ExpiresAt            *time.Time `json:"expiresAt,omitempty"`
}
