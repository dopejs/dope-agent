package activation

import "time"

type Status string

const (
	StatusNotStarted           Status = "not_started"
	StatusInProgress           Status = "in_progress"
	StatusBlocked              Status = "blocked"
	StatusActive               Status = "active"
	StatusFirstActionCompleted Status = "first_action_completed"
)

const (
	StepResolvePersonalTenant = "resolve_personal_tenant"
	StepTenantResolved        = "tenant_resolved"
	StepQuotaBaseline         = "quota_baseline"
	StepQuotaBaselineReady    = "quota_baseline_ready"
	StepTestChat              = "test_chat"
	StepTestChatCompleted     = "test_chat_completed"
	StepCompleted             = "completed"
)

type ReasonCode string

const (
	ReasonPrincipalDisabled        ReasonCode = "activation_denied:principal_disabled"
	ReasonPrincipalDenied          ReasonCode = "activation_denied:principal_denied"
	ReasonTenantAccessRevoked      ReasonCode = "activation_denied:tenant_access_revoked"
	ReasonQuotaBaselineUnavailable ReasonCode = "activation_blocked:quota_baseline_unavailable"
	ReasonEnvironmentUnavailable   ReasonCode = "activation_blocked:environment_unavailable"
	ReasonTestChatUnavailable      ReasonCode = "activation_blocked:test_chat_unavailable"
	ReasonTenantResolutionFailed   ReasonCode = "activation_failed:tenant_resolution"
	ReasonTestChatFailed           ReasonCode = "activation_failed:test_chat"
	ReasonAuditWriteFailed         ReasonCode = "activation_failed:audit_write"
	ReasonPersistenceFailed        ReasonCode = "activation_failed:persistence"
	ReasonUnexpectedFailed         ReasonCode = "activation_failed:unexpected"
)

type ReadinessKind string

const (
	ReadinessKindTenantAccess  ReadinessKind = "tenant_access"
	ReadinessKindEnvironment   ReadinessKind = "environment"
	ReadinessKindQuotaBaseline ReadinessKind = "quota_baseline"
	ReadinessKindTestChat      ReadinessKind = "test_chat"
)

type ReadinessStatus string

const (
	ReadinessStatusReady                ReadinessStatus = "ready"
	ReadinessStatusBlocked              ReadinessStatus = "blocked"
	ReadinessStatusDegraded             ReadinessStatus = "degraded"
	ReadinessStatusMissingConfiguration ReadinessStatus = "missing_configuration"
	ReadinessStatusOptional             ReadinessStatus = "optional"
)

type RemediationOwner string

const (
	RemediationOwnerProductUser  RemediationOwner = "product_user"
	RemediationOwnerOperator     RemediationOwner = "operator"
	RemediationOwnerTenantAdmin  RemediationOwner = "tenant_admin"
	RemediationOwnerSystem       RemediationOwner = "system"
	RemediationOwnerNoneRequired RemediationOwner = "none_required"
)

type ReadinessItem struct {
	ItemID                string           `json:"itemId"`
	ItemKind              ReadinessKind    `json:"itemKind"`
	Status                ReadinessStatus  `json:"status"`
	ReasonCode            ReasonCode       `json:"reasonCode,omitempty"`
	DisplayName           string           `json:"displayName"`
	RequiredForActivation bool             `json:"requiredForActivation"`
	Retryable             bool             `json:"retryable"`
	RemediationOwner      RemediationOwner `json:"remediationOwner"`
	UpdatedAt             time.Time        `json:"updatedAt"`
}

type QuotaBaselineStatus string

const (
	QuotaBaselineStatusAvailable   QuotaBaselineStatus = "available"
	QuotaBaselineStatusUnavailable QuotaBaselineStatus = "unavailable"
)

type QuotaProjection struct {
	Category  string         `json:"category,omitempty"`
	Unit      string         `json:"unit,omitempty"`
	Limit     *int64         `json:"limit,omitempty"`
	Used      *int64         `json:"used,omitempty"`
	Remaining *int64         `json:"remaining,omitempty"`
	Period    string         `json:"period,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type QuotaBaseline struct {
	TenantID         string              `json:"tenantId"`
	PlanKey          string              `json:"planKey"`
	EnforcementMode  string              `json:"enforcementMode"`
	Status           QuotaBaselineStatus `json:"status"`
	Quotas           []QuotaProjection   `json:"quotas"`
	ProjectedAt      time.Time           `json:"projectedAt,omitempty"`
	ProjectionSource string              `json:"projectionSource,omitempty"`
	ReasonCode       ReasonCode          `json:"reasonCode,omitempty"`
}

const FirstActionTestChat = "test_chat"

type FirstAction struct {
	ActionID        string   `json:"actionId"`
	ActionKind      string   `json:"actionKind"`
	DisplayName     string   `json:"displayName,omitempty"`
	Recommended     bool     `json:"recommended"`
	Available       bool     `json:"available"`
	BlockingItemIDs []string `json:"blockingItemIds"`
	InvokeRoute     string   `json:"invokeRoute"`
	ResultRoute     string   `json:"resultRoute"`
}

func DefaultTestChatFirstAction(available bool, blockingItemIDs []string) FirstAction {
	if blockingItemIDs == nil {
		blockingItemIDs = []string{}
	}
	return FirstAction{
		ActionID:        FirstActionTestChat,
		ActionKind:      FirstActionTestChat,
		DisplayName:     "Test chat",
		Recommended:     true,
		Available:       available,
		BlockingItemIDs: blockingItemIDs,
		InvokeRoute:     "/v1/activation/test-chat",
		ResultRoute:     "/v1/activation",
	}
}

type TestChatStatus string

const (
	TestChatStatusCompleted TestChatStatus = "completed"
	TestChatStatusFailed    TestChatStatus = "failed"
	TestChatStatusCancelled TestChatStatus = "cancelled"
)

type TestChatMetadata struct {
	ActivationID string         `json:"activationId"`
	TenantID     string         `json:"tenantId"`
	DispatchID   string         `json:"dispatchId,omitempty"`
	Status       TestChatStatus `json:"status"`
	Provider     string         `json:"provider,omitempty"`
	Model        string         `json:"model,omitempty"`
	Usage        map[string]any `json:"usage,omitempty"`
	FinishReason string         `json:"finishReason,omitempty"`
	ReasonCode   ReasonCode     `json:"reasonCode,omitempty"`
	CompletedAt  *time.Time     `json:"completedAt,omitempty"`
}

type FailureStage string

const (
	FailureStageTenantResolution FailureStage = "tenant_resolution"
	FailureStageEligibility      FailureStage = "eligibility"
	FailureStageQuotaBaseline    FailureStage = "quota_baseline"
	FailureStageAuthorization    FailureStage = "authorization"
	FailureStageTestChat         FailureStage = "test_chat"
	FailureStageAudit            FailureStage = "audit"
	FailureStagePersistence      FailureStage = "persistence"
	FailureStageUnexpected       FailureStage = "unexpected"
)

type FailureReason struct {
	ReasonCode       ReasonCode       `json:"reasonCode"`
	Stage            FailureStage     `json:"stage"`
	Retryable        bool             `json:"retryable"`
	RemediationOwner RemediationOwner `json:"remediationOwner"`
	Message          string           `json:"message,omitempty"`
}

type AuditMetadata struct {
	ActivationID      string            `json:"activationId"`
	TenantID          string            `json:"tenantId,omitempty"`
	PrincipalID       string            `json:"principalId"`
	TokenID           string            `json:"tokenId,omitempty"`
	Stage             FailureStage      `json:"stage,omitempty"`
	FromStatus        Status            `json:"fromStatus,omitempty"`
	ToStatus          Status            `json:"toStatus,omitempty"`
	ReasonCode        ReasonCode        `json:"reasonCode,omitempty"`
	Retryable         bool              `json:"retryable"`
	RemediationOwner  RemediationOwner  `json:"remediationOwner,omitempty"`
	TestChat          *TestChatMetadata `json:"testChat,omitempty"`
	TransitionedAt    time.Time         `json:"transitionedAt"`
	EnvironmentScope  string            `json:"environmentScope,omitempty"`
	CompletedStepIDs  []string          `json:"completedStepIds,omitempty"`
	ReadinessItemIDs  []string          `json:"readinessItemIds,omitempty"`
	QuotaBaselineStat string            `json:"quotaBaselineStatus,omitempty"`
}

type State struct {
	ActivationID             string            `json:"activationId"`
	PrincipalID              string            `json:"principalId"`
	TenantID                 string            `json:"tenantId"`
	EnvironmentScope         string            `json:"environmentScope"`
	Status                   Status            `json:"status"`
	CurrentStepID            string            `json:"currentStepId"`
	CompletedStepIDs         []string          `json:"completedStepIds"`
	BlockingReasonCodes      []ReasonCode      `json:"blockingReasonCodes"`
	ReadinessItems           []ReadinessItem   `json:"readinessItems"`
	QuotaBaseline            *QuotaBaseline    `json:"quotaBaseline,omitempty"`
	FirstAction              FirstAction       `json:"firstAction"`
	TestChat                 *TestChatMetadata `json:"testChat,omitempty"`
	FailureReason            *FailureReason    `json:"failureReason,omitempty"`
	CreatedAt                time.Time         `json:"createdAt"`
	UpdatedAt                time.Time         `json:"updatedAt"`
	FirstActionCompletedAt   *time.Time        `json:"firstActionCompletedAt,omitempty"`
	LastEvaluatedAt          time.Time         `json:"lastEvaluatedAt"`
	LastTransitionAuditEvent string            `json:"lastTransitionAuditEventId,omitempty"`
	Metadata                 map[string]any    `json:"metadata,omitempty"`
}

type Diagnostic struct {
	ActivationID        string            `json:"activationId"`
	TenantID            string            `json:"tenantId,omitempty"`
	PrincipalID         string            `json:"principalId"`
	Status              Status            `json:"status"`
	Stage               FailureStage      `json:"stage"`
	ReasonCode          ReasonCode        `json:"reasonCode"`
	Retryable           bool              `json:"retryable"`
	RemediationOwner    RemediationOwner  `json:"remediationOwner"`
	LastTransitionAt    time.Time         `json:"lastTransitionAt"`
	ReadinessItemIDs    []string          `json:"readinessItemIds,omitempty"`
	QuotaBaselineStatus string            `json:"quotaBaselineStatus,omitempty"`
	TestChat            *TestChatMetadata `json:"testChat,omitempty"`
}
