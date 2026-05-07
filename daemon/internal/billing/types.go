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

type QuotaStatus string

const (
	QuotaStatusAvailable     QuotaStatus = "available"
	QuotaStatusNearLimit     QuotaStatus = "near_limit"
	QuotaStatusExhausted     QuotaStatus = "exhausted"
	QuotaStatusUnlimited     QuotaStatus = "unlimited"
	QuotaStatusNotMeasurable QuotaStatus = "not_measurable"
	QuotaStatusRestricted    QuotaStatus = "restricted"
	QuotaStatusUnavailable   QuotaStatus = "unavailable"
)

type NearLimitReason string

const (
	NearLimitReasonNone                     NearLimitReason = ""
	NearLimitReasonPercentThreshold         NearLimitReason = "percent_threshold"
	NearLimitReasonBelowOneTypicalOperation NearLimitReason = "below_one_typical_operation"
)

type RecoveryAction string

const (
	RecoveryActionWait                       RecoveryAction = "wait"
	RecoveryActionReduceScope                RecoveryAction = "reduce_scope"
	RecoveryActionRequestOverride            RecoveryAction = "request_override"
	RecoveryActionContactSupport             RecoveryAction = "contact_support"
	RecoveryActionOperatorResolutionRequired RecoveryAction = "operator_resolution_required"
	RecoveryActionRetryLater                 RecoveryAction = "retry_later"
)

type DenialClassification string

const (
	DenialClassificationQuotaExhaustion       DenialClassification = "quota_exhaustion"
	DenialClassificationAbuseRestriction      DenialClassification = "abuse_restriction"
	DenialClassificationQuotaStateUnavailable DenialClassification = "quota_state_unavailable"
	DenialClassificationUnauthorized          DenialClassification = "unauthorized"
	DenialClassificationOperatorActionNeeded  DenialClassification = "operator_action_needed"
)

type AbuseRestrictionStatus string

const (
	AbuseRestrictionStatusActive  AbuseRestrictionStatus = "active"
	AbuseRestrictionStatusExpired AbuseRestrictionStatus = "expired"
)

type PlanSummary struct {
	PlanKey           string          `json:"planKey"`
	EnforcementMode   EnforcementMode `json:"enforcementMode"`
	Status            PlanStatus      `json:"status,omitempty"`
	EffectiveAt       time.Time       `json:"effectiveAt"`
	BasePlanLabel     string          `json:"basePlanLabel,omitempty"`
	CheckoutAvailable bool            `json:"checkoutAvailable"`
}

type UsagePeriodSummary struct {
	PeriodStart      time.Time `json:"periodStart"`
	PeriodEnd        time.Time `json:"periodEnd"`
	PeriodAnchor     string    `json:"periodAnchor"`
	ConsumedAmount   int64     `json:"consumedAmount"`
	ReservedAmount   int64     `json:"reservedAmount"`
	AdjustedAmount   int64     `json:"adjustedAmount"`
	CarryoverApplied int64     `json:"carryoverApplied"`
	RemainingAmount  int64     `json:"remainingAmount"`
	OverLimit        bool      `json:"overLimit"`
}

type QuotaOverrideSummary struct {
	BaseLimit      int64      `json:"baseLimit"`
	EffectiveLimit int64      `json:"effectiveLimit"`
	Reason         string     `json:"reason,omitempty"`
	EffectiveAt    time.Time  `json:"effectiveAt"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
}

type AbuseRestrictionSummary struct {
	RestrictionID         string                 `json:"restrictionId,omitempty"`
	Status                AbuseRestrictionStatus `json:"status"`
	AffectedCategory      Category               `json:"affectedCategory,omitempty"`
	RecoveryAction        RecoveryAction         `json:"recoveryAction"`
	VisibleReasonCode     string                 `json:"visibleReasonCode,omitempty"`
	SourceAuditRef        string                 `json:"sourceAuditRef,omitempty"`
	SupportContactAllowed bool                   `json:"supportContactAllowed"`
	StartedAt             time.Time              `json:"startedAt,omitempty"`
	ExpiresAt             *time.Time             `json:"expiresAt,omitempty"`
}

type AbuseRestrictionRecord struct {
	RestrictionID         string                 `json:"restrictionId"`
	TenantID              string                 `json:"tenantId"`
	Status                AbuseRestrictionStatus `json:"status"`
	AffectedCategory      Category               `json:"affectedCategory"`
	RecoveryAction        RecoveryAction         `json:"recoveryAction"`
	VisibleReasonCode     string                 `json:"visibleReasonCode"`
	SourceAuditRef        string                 `json:"sourceAuditRef,omitempty"`
	SupportContactAllowed bool                   `json:"supportContactAllowed"`
	StartedAt             time.Time              `json:"startedAt"`
	ExpiresAt             *time.Time             `json:"expiresAt,omitempty"`
	Document              map[string]any         `json:"document,omitempty"`
}

func (record AbuseRestrictionRecord) Summary() AbuseRestrictionSummary {
	return AbuseRestrictionSummary{
		RestrictionID:         record.RestrictionID,
		Status:                record.Status,
		AffectedCategory:      record.AffectedCategory,
		RecoveryAction:        record.RecoveryAction,
		VisibleReasonCode:     record.VisibleReasonCode,
		SourceAuditRef:        record.SourceAuditRef,
		SupportContactAllowed: record.SupportContactAllowed,
		StartedAt:             record.StartedAt,
		ExpiresAt:             record.ExpiresAt,
	}
}

type QuotaStatusItem struct {
	Category               Category                 `json:"category"`
	Unit                   Unit                     `json:"unit"`
	Status                 QuotaStatus              `json:"status"`
	CurrentPeriod          UsagePeriodSummary       `json:"currentPeriod"`
	PreviousPeriod         *UsagePeriodSummary      `json:"previousPeriod,omitempty"`
	Limit                  int64                    `json:"limit"`
	RemainingAmount        int64                    `json:"remainingAmount"`
	NearLimit              bool                     `json:"nearLimit"`
	NearLimitReason        NearLimitReason          `json:"nearLimitReason,omitempty"`
	TypicalOperationAmount int64                    `json:"typicalOperationAmount"`
	BaseLimit              int64                    `json:"baseLimit"`
	EffectiveLimit         int64                    `json:"effectiveLimit"`
	Override               *QuotaOverrideSummary    `json:"override,omitempty"`
	Restriction            *AbuseRestrictionSummary `json:"restriction,omitempty"`
	RecoveryActions        []RecoveryAction         `json:"recoveryActions"`
}

type QuotaSection struct {
	SectionKey string            `json:"sectionKey"`
	Label      string            `json:"label"`
	Items      []QuotaStatusItem `json:"items"`
}

type TenantQuotaDashboard struct {
	TenantID    string         `json:"tenantId"`
	Plan        PlanSummary    `json:"plan"`
	Sections    []QuotaSection `json:"sections"`
	GeneratedAt time.Time      `json:"generatedAt"`
	Permission  map[string]any `json:"permission,omitempty"`
}

type QuotaDenialDetail struct {
	DenialID          string                   `json:"denialId"`
	TenantID          string                   `json:"tenantId"`
	OperationRef      string                   `json:"operationRef"`
	OperationKey      string                   `json:"operationKey"`
	GuardedEntryPoint string                   `json:"guardedEntryPoint"`
	Category          Category                 `json:"category,omitempty"`
	ReasonCode        string                   `json:"reasonCode"`
	Classification    DenialClassification     `json:"classification"`
	RequestedAmount   int64                    `json:"requestedAmount"`
	RemainingAmount   int64                    `json:"remainingAmount"`
	RecoveryActions   []RecoveryAction         `json:"recoveryActions"`
	Restriction       *AbuseRestrictionSummary `json:"restriction,omitempty"`
	CreatedAt         time.Time                `json:"createdAt"`
}

type BillingEvidenceRedaction struct {
	Path        string `json:"path"`
	Reason      string `json:"reason"`
	Replacement string `json:"replacement"`
}

type BillingEvidenceExport struct {
	SchemaVersion          string                     `json:"schemaVersion"`
	ExportID               string                     `json:"exportId"`
	TenantID               string                     `json:"tenantId"`
	GeneratedAt            time.Time                  `json:"generatedAt"`
	GeneratedByPrincipalID string                     `json:"generatedByPrincipalId"`
	Denial                 QuotaDenialDetail          `json:"denial"`
	UsageSnapshot          []QuotaStatusItem          `json:"usageSnapshot"`
	EffectiveLimitState    map[string]any             `json:"effectiveLimitState"`
	AuditRefs              []string                   `json:"auditRefs"`
	Redactions             []BillingEvidenceRedaction `json:"redactions"`
}
