package integrations

import "time"

const DiagnosticDefaultRetention = 90 * 24 * time.Hour
const DiagnosticStaleAfter = 15 * time.Minute

type DiagnosticStatus string

const (
	DiagnosticStatusUnknown     DiagnosticStatus = "unknown"
	DiagnosticStatusHealthy     DiagnosticStatus = "healthy"
	DiagnosticStatusDegraded    DiagnosticStatus = "degraded"
	DiagnosticStatusBlocked     DiagnosticStatus = "blocked"
	DiagnosticStatusUnsupported DiagnosticStatus = "unsupported"
)

type DiagnosticReasonCode string

const (
	ReasonHealthy                   DiagnosticReasonCode = "healthy"
	ReasonAppAuthorizationMissing   DiagnosticReasonCode = "app_authorization_missing"
	ReasonBotAuthorizationMissing   DiagnosticReasonCode = "bot_authorization_missing"
	ReasonUserAuthorizationMissing  DiagnosticReasonCode = "user_authorization_missing"
	ReasonTenantApprovalPending     DiagnosticReasonCode = "tenant_approval_pending"
	ReasonScopeMissing              DiagnosticReasonCode = "scope_missing"
	ReasonTokenMissing              DiagnosticReasonCode = "token_missing"
	ReasonTokenExpired              DiagnosticReasonCode = "token_expired"
	ReasonTokenRevoked              DiagnosticReasonCode = "token_revoked"
	ReasonRefreshCredentialsMissing DiagnosticReasonCode = "refresh_credentials_missing"
	ReasonTokenRefreshFailed        DiagnosticReasonCode = "token_refresh_failed"
	ReasonTenantMismatch            DiagnosticReasonCode = "tenant_mismatch"
	ReasonRateLimited               DiagnosticReasonCode = "rate_limited"
	ReasonProviderUnavailable       DiagnosticReasonCode = "provider_unavailable"
	ReasonTransientProviderFailure  DiagnosticReasonCode = "transient_provider_failure"
	ReasonNetworkFailed             DiagnosticReasonCode = "network_failed"
	ReasonAmbiguousDownstreamCommit DiagnosticReasonCode = "ambiguous_downstream_commit"
	ReasonUnsafeToRetry             DiagnosticReasonCode = "unsafe_to_retry"
	ReasonOperatorActionNeeded      DiagnosticReasonCode = "operator_action_needed"
	ReasonLimitedDiagnostic         DiagnosticReasonCode = "limited_diagnostic"
	ReasonUnsupportedDiagnostic     DiagnosticReasonCode = "unsupported_diagnostic"
	ReasonRedactionFailedClosed     DiagnosticReasonCode = "redaction_failed_closed"
	ReasonUnknownProviderError      DiagnosticReasonCode = "unknown_provider_error"
)

type RetrySafety string

const (
	RetrySafetyNoActionNeeded       RetrySafety = "no_action_needed"
	RetrySafetyRetryable            RetrySafety = "retryable"
	RetrySafetyBlocked              RetrySafety = "blocked"
	RetrySafetyUnsafeToRetry        RetrySafety = "unsafe_to_retry"
	RetrySafetyOperatorActionNeeded RetrySafety = "operator_action_needed"
)

type RemediationOwner string

const (
	RemediationOwnerProductUser  RemediationOwner = "product_user"
	RemediationOwnerTenantAdmin  RemediationOwner = "tenant_admin"
	RemediationOwnerOperator     RemediationOwner = "operator"
	RemediationOwnerProvider     RemediationOwner = "provider"
	RemediationOwnerNoneRequired RemediationOwner = "none_required"
)

type FreshnessState string

const (
	FreshnessStateFresh FreshnessState = "fresh"
	FreshnessStateStale FreshnessState = "stale"
)

type RedactionStatus string

const (
	RedactionStatusRedacted     RedactionStatus = "redacted"
	RedactionStatusSuppressed   RedactionStatus = "suppressed"
	RedactionStatusFailedClosed RedactionStatus = "failed_closed"
)

type DiagnosticRunStatus string

const (
	DiagnosticRunQueued    DiagnosticRunStatus = "queued"
	DiagnosticRunRunning   DiagnosticRunStatus = "running"
	DiagnosticRunCompleted DiagnosticRunStatus = "completed"
	DiagnosticRunFailed    DiagnosticRunStatus = "failed"
	DiagnosticRunBlocked   DiagnosticRunStatus = "blocked"
)

type DiagnosticResult struct {
	DiagnosticResultID   string               `json:"diagnosticResultId"`
	TenantID             string               `json:"tenantId"`
	IntegrationID        string               `json:"integrationId"`
	IntegrationAccountID string               `json:"integrationAccountId,omitempty"`
	DomainKind           string               `json:"domainKind"`
	ProviderKind         string               `json:"providerKind"`
	Capability           string               `json:"capability"`
	Status               DiagnosticStatus     `json:"status"`
	ReasonCode           DiagnosticReasonCode `json:"reasonCode"`
	RemediationOwner     RemediationOwner     `json:"remediationOwner"`
	RemediationHint      string               `json:"remediationHint,omitempty"`
	RetrySafety          RetrySafety          `json:"retrySafety"`
	CheckedAt            time.Time            `json:"checkedAt"`
	StaleAfter           time.Time            `json:"staleAfter"`
	FreshnessState       FreshnessState       `json:"freshnessState"`
	RunID                string               `json:"runId,omitempty"`
	RedactionStatus      RedactionStatus      `json:"redactionStatus"`
	EvidenceSummary      string               `json:"evidenceSummary,omitempty"`
	RetentionExpiresAt   time.Time            `json:"retentionExpiresAt"`
	SmokeReportID        string               `json:"smokeReportId,omitempty"`
	ArtifactRefs         []string             `json:"artifactRefs,omitempty"`
	CreatedAt            time.Time            `json:"createdAt,omitempty"`
	UpdatedAt            time.Time            `json:"updatedAt,omitempty"`
}

type DiagnosticRun struct {
	DiagnosticRunID      string               `json:"diagnosticRunId"`
	TenantID             string               `json:"tenantId"`
	IntegrationID        string               `json:"integrationId"`
	IntegrationAccountID string               `json:"integrationAccountId,omitempty"`
	DomainKind           string               `json:"domainKind,omitempty"`
	ProviderKind         string               `json:"providerKind,omitempty"`
	RequestedBy          string               `json:"requestedBy"`
	Trigger              string               `json:"trigger"`
	Status               DiagnosticRunStatus  `json:"status"`
	StartedAt            time.Time            `json:"startedAt"`
	CompletedAt          *time.Time           `json:"completedAt,omitempty"`
	CheckedCapabilities  []string             `json:"checkedCapabilities"`
	ResultIDs            []string             `json:"resultIds"`
	FailureReasonCode    DiagnosticReasonCode `json:"failureReasonCode,omitempty"`
	RedactionStatus      RedactionStatus      `json:"redactionStatus"`
	RetentionExpiresAt   time.Time            `json:"retentionExpiresAt"`
	IdempotencyKey       string               `json:"idempotencyKey,omitempty"`
}

type DiagnosticReasonCodeDefinition struct {
	ReasonCode              DiagnosticReasonCode `json:"reasonCode"`
	Category                string               `json:"category"`
	DefaultSeverity         string               `json:"defaultSeverity,omitempty"`
	DefaultRetrySafety      RetrySafety          `json:"defaultRetrySafety"`
	DefaultRemediationOwner RemediationOwner     `json:"defaultRemediationOwner"`
	UserMessageKey          string               `json:"userMessageKey"`
	OperatorMessageKey      string               `json:"operatorMessageKey"`
	SupportedDomains        []string             `json:"supportedDomains,omitempty"`
}

type ProviderErrorClassification struct {
	ClassificationID     string               `json:"classificationId"`
	TenantID             string               `json:"tenantId"`
	ProviderKind         string               `json:"providerKind"`
	DomainKind           string               `json:"domainKind"`
	IntegrationID        string               `json:"integrationId,omitempty"`
	OperationClass       string               `json:"operationClass,omitempty"`
	ProviderErrorClass   string               `json:"providerErrorClass,omitempty"`
	ProviderStatusCode   string               `json:"providerStatusCode,omitempty"`
	RedactedProviderCode string               `json:"redactedProviderCode,omitempty"`
	ReasonCode           DiagnosticReasonCode `json:"reasonCode"`
	RetrySafety          RetrySafety          `json:"retrySafety"`
	RemediationOwner     RemediationOwner     `json:"remediationOwner"`
	EvidenceConfidence   string               `json:"evidenceConfidence,omitempty"`
	Ambiguous            bool                 `json:"ambiguous,omitempty"`
	RedactionStatus      RedactionStatus      `json:"redactionStatus"`
	CreatedAt            time.Time            `json:"createdAt"`
}

type DiagnosticFailureProjection struct {
	ReasonCode       DiagnosticReasonCode `json:"reasonCode"`
	RemediationOwner RemediationOwner     `json:"remediationOwner"`
	RemediationHint  string               `json:"remediationHint"`
	RetrySafety      RetrySafety          `json:"retrySafety"`
	FreshnessState   FreshnessState       `json:"freshnessState"`
	CheckedAt        time.Time            `json:"checkedAt"`
	RedactionStatus  RedactionStatus      `json:"redactionStatus"`
}

type DiagnosticResultFilter struct {
	TenantID       string
	IntegrationID  string
	ProviderKind   string
	DomainKind     string
	Status         DiagnosticStatus
	ReasonCode     DiagnosticReasonCode
	Cursor         string
	Limit          int
	IncludeExpired bool
}

type DiagnosticRunFilter struct {
	TenantID       string
	IntegrationID  string
	ProviderKind   string
	DomainKind     string
	Status         DiagnosticRunStatus
	ReasonCode     DiagnosticReasonCode
	Cursor         string
	Limit          int
	IncludeExpired bool
}

type DiagnosticInspectionInput struct {
	Resource     Resource
	Capability   string
	RunID        string
	CheckedAt    time.Time
	EvidenceText string
	ForceGeneric bool
}

type DiagnosticRunInput struct {
	Resource     Resource
	RequestedBy  string
	ClientKey    string
	Capabilities []string
	Trigger      string
	StartedAt    time.Time
}

func DiagnosticFreshness(now, staleAfter time.Time) FreshnessState {
	if staleAfter.IsZero() || now.After(staleAfter) {
		return FreshnessStateStale
	}
	return FreshnessStateFresh
}

func DiagnosticRetentionExpiry(now time.Time) time.Time {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return now.UTC().Add(DiagnosticDefaultRetention)
}

func DefaultDiagnosticReasonCodeCatalog() []DiagnosticReasonCodeDefinition {
	return []DiagnosticReasonCodeDefinition{
		{ReasonCode: ReasonHealthy, Category: "healthy", DefaultRetrySafety: RetrySafetyNoActionNeeded, DefaultRemediationOwner: RemediationOwnerNoneRequired, UserMessageKey: "integration.diagnostic.healthy", OperatorMessageKey: "integration.diagnostic.healthy"},
		{ReasonCode: ReasonAppAuthorizationMissing, Category: "authorization", DefaultRetrySafety: RetrySafetyBlocked, DefaultRemediationOwner: RemediationOwnerOperator, UserMessageKey: "integration.diagnostic.app_authorization_missing", OperatorMessageKey: "integration.diagnostic.app_authorization_missing"},
		{ReasonCode: ReasonBotAuthorizationMissing, Category: "authorization", DefaultRetrySafety: RetrySafetyBlocked, DefaultRemediationOwner: RemediationOwnerOperator, UserMessageKey: "integration.diagnostic.bot_authorization_missing", OperatorMessageKey: "integration.diagnostic.bot_authorization_missing"},
		{ReasonCode: ReasonUserAuthorizationMissing, Category: "authorization", DefaultRetrySafety: RetrySafetyBlocked, DefaultRemediationOwner: RemediationOwnerProductUser, UserMessageKey: "integration.diagnostic.user_authorization_missing", OperatorMessageKey: "integration.diagnostic.user_authorization_missing"},
		{ReasonCode: ReasonTenantApprovalPending, Category: "tenant_approval", DefaultRetrySafety: RetrySafetyBlocked, DefaultRemediationOwner: RemediationOwnerTenantAdmin, UserMessageKey: "integration.diagnostic.tenant_approval_pending", OperatorMessageKey: "integration.diagnostic.tenant_approval_pending"},
		{ReasonCode: ReasonScopeMissing, Category: "scope", DefaultRetrySafety: RetrySafetyBlocked, DefaultRemediationOwner: RemediationOwnerTenantAdmin, UserMessageKey: "integration.diagnostic.scope_missing", OperatorMessageKey: "integration.diagnostic.scope_missing"},
		{ReasonCode: ReasonTokenMissing, Category: "token", DefaultRetrySafety: RetrySafetyBlocked, DefaultRemediationOwner: RemediationOwnerProductUser, UserMessageKey: "integration.diagnostic.token_missing", OperatorMessageKey: "integration.diagnostic.token_missing"},
		{ReasonCode: ReasonTokenExpired, Category: "token", DefaultRetrySafety: RetrySafetyBlocked, DefaultRemediationOwner: RemediationOwnerProductUser, UserMessageKey: "integration.diagnostic.token_expired", OperatorMessageKey: "integration.diagnostic.token_expired"},
		{ReasonCode: ReasonTokenRevoked, Category: "token", DefaultRetrySafety: RetrySafetyBlocked, DefaultRemediationOwner: RemediationOwnerProductUser, UserMessageKey: "integration.diagnostic.token_revoked", OperatorMessageKey: "integration.diagnostic.token_revoked"},
		{ReasonCode: ReasonRefreshCredentialsMissing, Category: "token", DefaultRetrySafety: RetrySafetyBlocked, DefaultRemediationOwner: RemediationOwnerOperator, UserMessageKey: "integration.diagnostic.refresh_credentials_missing", OperatorMessageKey: "integration.diagnostic.refresh_credentials_missing"},
		{ReasonCode: ReasonTokenRefreshFailed, Category: "token", DefaultRetrySafety: RetrySafetyBlocked, DefaultRemediationOwner: RemediationOwnerOperator, UserMessageKey: "integration.diagnostic.token_refresh_failed", OperatorMessageKey: "integration.diagnostic.token_refresh_failed"},
		{ReasonCode: ReasonTenantMismatch, Category: "tenant_mismatch", DefaultRetrySafety: RetrySafetyBlocked, DefaultRemediationOwner: RemediationOwnerOperator, UserMessageKey: "integration.diagnostic.tenant_mismatch", OperatorMessageKey: "integration.diagnostic.tenant_mismatch"},
		{ReasonCode: ReasonRateLimited, Category: "quota", DefaultRetrySafety: RetrySafetyRetryable, DefaultRemediationOwner: RemediationOwnerProvider, UserMessageKey: "integration.diagnostic.rate_limited", OperatorMessageKey: "integration.diagnostic.rate_limited"},
		{ReasonCode: ReasonProviderUnavailable, Category: "provider", DefaultRetrySafety: RetrySafetyRetryable, DefaultRemediationOwner: RemediationOwnerProvider, UserMessageKey: "integration.diagnostic.provider_unavailable", OperatorMessageKey: "integration.diagnostic.provider_unavailable"},
		{ReasonCode: ReasonTransientProviderFailure, Category: "provider", DefaultRetrySafety: RetrySafetyRetryable, DefaultRemediationOwner: RemediationOwnerProvider, UserMessageKey: "integration.diagnostic.transient_provider_failure", OperatorMessageKey: "integration.diagnostic.transient_provider_failure"},
		{ReasonCode: ReasonNetworkFailed, Category: "network", DefaultRetrySafety: RetrySafetyRetryable, DefaultRemediationOwner: RemediationOwnerOperator, UserMessageKey: "integration.diagnostic.network_failed", OperatorMessageKey: "integration.diagnostic.network_failed"},
		{ReasonCode: ReasonAmbiguousDownstreamCommit, Category: "retry_safety", DefaultRetrySafety: RetrySafetyUnsafeToRetry, DefaultRemediationOwner: RemediationOwnerOperator, UserMessageKey: "integration.diagnostic.ambiguous_downstream_commit", OperatorMessageKey: "integration.diagnostic.ambiguous_downstream_commit"},
		{ReasonCode: ReasonUnsafeToRetry, Category: "retry_safety", DefaultRetrySafety: RetrySafetyUnsafeToRetry, DefaultRemediationOwner: RemediationOwnerOperator, UserMessageKey: "integration.diagnostic.unsafe_to_retry", OperatorMessageKey: "integration.diagnostic.unsafe_to_retry"},
		{ReasonCode: ReasonOperatorActionNeeded, Category: "retry_safety", DefaultRetrySafety: RetrySafetyOperatorActionNeeded, DefaultRemediationOwner: RemediationOwnerOperator, UserMessageKey: "integration.diagnostic.operator_action_needed", OperatorMessageKey: "integration.diagnostic.operator_action_needed"},
		{ReasonCode: ReasonLimitedDiagnostic, Category: "unsupported", DefaultRetrySafety: RetrySafetyNoActionNeeded, DefaultRemediationOwner: RemediationOwnerOperator, UserMessageKey: "integration.diagnostic.limited", OperatorMessageKey: "integration.diagnostic.limited"},
		{ReasonCode: ReasonUnsupportedDiagnostic, Category: "unsupported", DefaultRetrySafety: RetrySafetyNoActionNeeded, DefaultRemediationOwner: RemediationOwnerOperator, UserMessageKey: "integration.diagnostic.unsupported", OperatorMessageKey: "integration.diagnostic.unsupported"},
		{ReasonCode: ReasonRedactionFailedClosed, Category: "redaction", DefaultRetrySafety: RetrySafetyBlocked, DefaultRemediationOwner: RemediationOwnerOperator, UserMessageKey: "integration.diagnostic.redaction_failed_closed", OperatorMessageKey: "integration.diagnostic.redaction_failed_closed"},
		{ReasonCode: ReasonUnknownProviderError, Category: "unknown", DefaultRetrySafety: RetrySafetyOperatorActionNeeded, DefaultRemediationOwner: RemediationOwnerOperator, UserMessageKey: "integration.diagnostic.unknown_provider_error", OperatorMessageKey: "integration.diagnostic.unknown_provider_error"},
	}
}
