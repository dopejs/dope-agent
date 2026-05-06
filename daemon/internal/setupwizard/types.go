package setupwizard

import (
	"errors"
	"time"
)

const (
	TargetOpenAICompatible = "provider.openai_compatible"
	TargetFeishuLark       = "integration.feishu_lark"
)

type TargetKind string

const (
	TargetKindProvider    TargetKind = "provider"
	TargetKindIntegration TargetKind = "integration"
	TargetKindChannel     TargetKind = "channel"
	TargetKindConnector   TargetKind = "connector"
)

type SetupStyle string

const (
	SetupStyleSubmittedSecret SetupStyle = "submitted_secret"
	SetupStyleOAuth           SetupStyle = "oauth"
	SetupStyleUnsupported     SetupStyle = "unsupported"
)

type SupportStatus string

const (
	SupportStatusSupported      SupportStatus = "supported"
	SupportStatusUnsupported    SupportStatus = "unsupported"
	SupportStatusActionRequired SupportStatus = "action_required"
)

type SetupState string

const (
	StateNotStarted     SetupState = "not_started"
	StateInProgress     SetupState = "in_progress"
	StateReady          SetupState = "ready"
	StateDegraded       SetupState = "degraded"
	StateUnavailable    SetupState = "unavailable"
	StateCancelled      SetupState = "cancelled"
	StateActionRequired SetupState = "action_required"
	StateDisabled       SetupState = "disabled"
)

type SafeUseMode string

const (
	SafeUseNormal  SafeUseMode = "normal"
	SafeUseLimited SafeUseMode = "limited_safe"
	SafeUseBlocked SafeUseMode = "blocked"
)

type RemediationOwner string

const (
	OwnerProductUser  RemediationOwner = "product_user"
	OwnerTenantAdmin  RemediationOwner = "tenant_admin"
	OwnerOperator     RemediationOwner = "operator"
	OwnerProvider     RemediationOwner = "provider"
	OwnerNoneRequired RemediationOwner = "none_required"
)

type RedactionStatus string

const (
	RedactionRedacted     RedactionStatus = "redacted"
	RedactionSuppressed   RedactionStatus = "suppressed"
	RedactionFailedClosed RedactionStatus = "failed_closed"
)

type RetrySafety string

const (
	RetryNoActionNeeded RetrySafety = "no_action_needed"
	RetryRetryable      RetrySafety = "retryable"
	RetryBlocked        RetrySafety = "blocked"
	RetryUnsafe         RetrySafety = "unsafe_to_retry"
)

type OAuthResult string

const (
	OAuthResultCompleted      OAuthResult = "completed"
	OAuthResultDenied         OAuthResult = "denied"
	OAuthResultAbandoned      OAuthResult = "abandoned"
	OAuthResultExpired        OAuthResult = "expired"
	OAuthResultReplay         OAuthResult = "replay"
	OAuthResultTenantMismatch OAuthResult = "tenant_mismatch"
	OAuthResultProviderError  OAuthResult = "provider_error"
)

type SetupOperation string

const (
	OperationStart           SetupOperation = "start"
	OperationSubmitSecret    SetupOperation = "submit_secret"
	OperationOAuthStart      SetupOperation = "oauth_start"
	OperationOAuthCallback   SetupOperation = "oauth_callback"
	OperationDiagnosticProbe SetupOperation = "diagnostic_probe"
	OperationRetry           SetupOperation = "retry"
	OperationReplace         SetupOperation = "replace"
	OperationCancel          SetupOperation = "cancel"
	OperationDisable         SetupOperation = "disable"
)

const (
	ReasonHealthy                 = "healthy"
	ReasonCredentialMissing       = "credential_missing"
	ReasonScopeMissing            = "scope_missing"
	ReasonTenantApprovalPending   = "tenant_approval_pending"
	ReasonTokenMissing            = "token_missing"
	ReasonTokenExpired            = "token_expired"
	ReasonTokenRevoked            = "token_revoked"
	ReasonOAuthDenied             = "oauth_denied"
	ReasonOAuthAbandoned          = "oauth_abandoned"
	ReasonOAuthExpired            = "oauth_expired"
	ReasonOAuthReplay             = "oauth_replay"
	ReasonTenantMismatch          = "tenant_mismatch"
	ReasonProviderUnavailable     = "provider_unavailable"
	ReasonNetworkFailed           = "network_failed"
	ReasonRateLimited             = "rate_limited"
	ReasonUnsupportedTarget       = "unsupported_target"
	ReasonRedactionFailedClosed   = "redaction_failed_closed"
	ReasonUserCancelled           = "user_cancelled"
	ReasonDisabledByUser          = "disabled_by_user"
	ReasonSetupPersistenceFailure = "setup_failed:persistence"
)

var (
	ErrTenantRequired       = errors.New("tenant context is required")
	ErrPermissionDenied     = errors.New("setup permission denied")
	ErrTargetRequired       = errors.New("setup target is required")
	ErrUnsupportedTarget    = errors.New("setup target is unsupported")
	ErrSessionRequired      = errors.New("setup session id is required")
	ErrSessionNotFound      = errors.New("setup session not found")
	ErrSecretRefRequired    = errors.New("secret ref is required")
	ErrSecretValueRequired  = errors.New("secret value is required")
	ErrOAuthStateRequired   = errors.New("oauth state is required")
	ErrOAuthStateMismatch   = errors.New("oauth state does not match setup session")
	ErrUnsafeEvidence       = errors.New("setup evidence contains forbidden credential material")
	ErrDiagnosticLinkNeeded = errors.New("ready or degraded setup requires diagnostic linkage")
)

type SetupTarget struct {
	TargetID                string        `json:"targetId"`
	TenantID                string        `json:"tenantId,omitempty"`
	TargetKind              TargetKind    `json:"targetKind"`
	SetupStyle              SetupStyle    `json:"setupStyle"`
	DisplayName             string        `json:"displayName"`
	ProofTarget             bool          `json:"proofTarget"`
	SupportStatus           SupportStatus `json:"supportStatus"`
	RequiredPermissions     []string      `json:"requiredPermissions,omitempty"`
	LimitedSafeCapabilities []string      `json:"limitedSafeCapabilities,omitempty"`
	CurrentSessionID        string        `json:"currentSessionId,omitempty"`
	CurrentState            SetupState    `json:"currentState,omitempty"`
	DiagnosticResultID      string        `json:"diagnosticResultId,omitempty"`
}

type SetupSession struct {
	SetupSessionID        string            `json:"setupSessionId"`
	TenantID              string            `json:"tenantId"`
	ActorPrincipalID      string            `json:"actorPrincipalId,omitempty"`
	TargetID              string            `json:"targetId"`
	TargetKind            TargetKind        `json:"targetKind"`
	SetupStyle            SetupStyle        `json:"setupStyle"`
	State                 SetupState        `json:"state"`
	ReasonCode            string            `json:"reasonCode,omitempty"`
	Retryable             bool              `json:"retryable"`
	RemediationOwner      RemediationOwner  `json:"remediationOwner"`
	SafeUseMode           SafeUseMode       `json:"safeUseMode"`
	AllowedCapabilities   []string          `json:"allowedCapabilities"`
	CurrentAttemptID      string            `json:"currentAttemptId,omitempty"`
	DiagnosticResultID    string            `json:"diagnosticResultId,omitempty"`
	DiagnosticRunID       string            `json:"diagnosticRunId,omitempty"`
	DiagnosticStage       string            `json:"diagnosticStage,omitempty"`
	DiagnosticSourceKind  string            `json:"diagnosticSourceKind,omitempty"`
	DiagnosticSourceID    string            `json:"diagnosticSourceId,omitempty"`
	DiagnosticAllowedUse  []string          `json:"diagnosticAllowedCapabilities,omitempty"`
	RedactionStatus       RedactionStatus   `json:"redactionStatus"`
	ResourceRefs          []ResourceRef     `json:"resourceRefs,omitempty"`
	RedactedEvidence      map[string]string `json:"redactedEvidence,omitempty"`
	OAuthStateRef         string            `json:"oauthStateRef,omitempty"`
	CreatedAt             time.Time         `json:"createdAt"`
	UpdatedAt             time.Time         `json:"updatedAt"`
	LastTransitionAt      time.Time         `json:"lastTransitionAt"`
	LastTransitionAuditID string            `json:"lastTransitionAuditEventId,omitempty"`
	OperatorRemediation   string            `json:"operatorRemediation,omitempty"`
	UserRemediation       string            `json:"userRemediation,omitempty"`
	UnsupportedReasonCode string            `json:"unsupportedReasonCode,omitempty"`
}

type SetupAttempt struct {
	AttemptID          string            `json:"attemptId"`
	SetupSessionID     string            `json:"setupSessionId"`
	TenantID           string            `json:"tenantId"`
	ActorPrincipalID   string            `json:"actorPrincipalId,omitempty"`
	Operation          SetupOperation    `json:"operation"`
	FromState          SetupState        `json:"fromState,omitempty"`
	ToState            SetupState        `json:"toState"`
	ReasonCode         string            `json:"reasonCode,omitempty"`
	RedactedEvidence   map[string]string `json:"redactedEvidence,omitempty"`
	ResourceRefs       []ResourceRef     `json:"resourceRefs,omitempty"`
	RedactionStatus    RedactionStatus   `json:"redactionStatus"`
	DiagnosticResultID string            `json:"diagnosticResultId,omitempty"`
	CreatedAt          time.Time         `json:"createdAt"`
}

type ResourceRef struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Route string `json:"route,omitempty"`
}

type SetupDiagnostic struct {
	SetupSessionID       string           `json:"setupSessionId"`
	TargetID             string           `json:"targetId"`
	DiagnosticResultID   string           `json:"diagnosticResultId"`
	DiagnosticRunID      string           `json:"diagnosticRunId,omitempty"`
	DiagnosticStage      string           `json:"diagnosticStage,omitempty"`
	DiagnosticSourceKind string           `json:"diagnosticSourceKind,omitempty"`
	DiagnosticSourceID   string           `json:"diagnosticSourceId,omitempty"`
	Status               SetupState       `json:"status"`
	ReasonCode           string           `json:"reasonCode"`
	RetrySafety          RetrySafety      `json:"retrySafety"`
	RemediationOwner     RemediationOwner `json:"remediationOwner"`
	AllowedCapabilities  []string         `json:"allowedCapabilities,omitempty"`
	CheckedAt            time.Time        `json:"checkedAt"`
	StaleAfter           time.Time        `json:"staleAfter"`
	RedactionStatus      RedactionStatus  `json:"redactionStatus"`
}

type DiagnosticSource struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type SetupDiagnosticProbeResult struct {
	State               SetupState
	ReasonCode          string
	RetrySafety         RetrySafety
	RemediationOwner    RemediationOwner
	AllowedCapabilities []string
	DiagnosticResultID  string
	DiagnosticRunID     string
	DiagnosticStage     string
	DiagnosticSource    DiagnosticSource
}

type SetupAuditRecord struct {
	EventKind          string           `json:"eventKind"`
	TenantID           string           `json:"tenantId"`
	PrincipalID        string           `json:"principalId,omitempty"`
	SetupSessionID     string           `json:"setupSessionId"`
	TargetID           string           `json:"targetId"`
	TargetKind         TargetKind       `json:"targetKind"`
	SetupStyle         SetupStyle       `json:"setupStyle"`
	Operation          SetupOperation   `json:"operation"`
	FromState          SetupState       `json:"fromState,omitempty"`
	ToState            SetupState       `json:"toState"`
	ReasonCode         string           `json:"reasonCode,omitempty"`
	Retryable          bool             `json:"retryable"`
	RemediationOwner   RemediationOwner `json:"remediationOwner"`
	SafeUseMode        SafeUseMode      `json:"safeUseMode"`
	DiagnosticResultID string           `json:"diagnosticResultId,omitempty"`
	ResourceRefs       []ResourceRef    `json:"resourceRefs,omitempty"`
	RedactionStatus    RedactionStatus  `json:"redactionStatus"`
	Outcome            string           `json:"outcome"`
	CreatedAt          time.Time        `json:"createdAt"`
}

type DependentUseDecision struct {
	TenantID            string      `json:"tenantId"`
	TargetID            string      `json:"targetId"`
	SetupState          SetupState  `json:"setupState"`
	SafeUseMode         SafeUseMode `json:"safeUseMode"`
	AllowedCapabilities []string    `json:"allowedCapabilities,omitempty"`
	ReasonCode          string      `json:"reasonCode,omitempty"`
	CheckedAt           time.Time   `json:"checkedAt"`
}
