package integrations

import "time"

type ReadinessStatus string

const (
	ReadinessStatusNotConfigured ReadinessStatus = "not_configured"
	ReadinessStatusAuthPending   ReadinessStatus = "auth_pending"
	ReadinessStatusHealthy       ReadinessStatus = "healthy"
	ReadinessStatusDegraded      ReadinessStatus = "degraded"
	ReadinessStatusUnavailable   ReadinessStatus = "unavailable"
)

type AuthState string

const (
	AuthStateNotStarted    AuthState = "not_started"
	AuthStatePending       AuthState = "pending"
	AuthStateAuthorized    AuthState = "authorized"
	AuthStateExpired       AuthState = "expired"
	AuthStateRevoked       AuthState = "revoked"
	AuthStateNotApplicable AuthState = "not_applicable"
)

type HealthState string

const (
	HealthStateUnknown     HealthState = "unknown"
	HealthStateHealthy     HealthState = "healthy"
	HealthStateDegraded    HealthState = "degraded"
	HealthStateUnavailable HealthState = "unavailable"
)

type BackendKind string

const (
	BackendKindMCP             BackendKind = "mcp"
	BackendKindManagedProvider BackendKind = "managed_provider"
	BackendKindNative          BackendKind = "native"
	BackendKindFakeLocal       BackendKind = "fake_local"
)

type ProbeKind string

const (
	ProbeKindInspect ProbeKind = "inspect"
	ProbeKindMutate  ProbeKind = "mutate"
)

type AccountBinding struct {
	AccountKey        string `json:"accountKey,omitempty"`
	AccountLabel      string `json:"accountLabel,omitempty"`
	AccountType       string `json:"accountType,omitempty"`
	TenantLabel       string `json:"tenantLabel,omitempty"`
	ExternalAccountID string `json:"externalAccountId,omitempty"`
	KnownAfterAuth    bool   `json:"knownAfterAuth,omitempty"`
}

type BackendBinding struct {
	BackendKind             BackendKind `json:"backendKind"`
	BackendRefID            string      `json:"backendRefId,omitempty"`
	BackendDisplayName      string      `json:"backendDisplayName,omitempty"`
	SourceKind              string      `json:"sourceKind,omitempty"`
	SupportsInteractiveAuth bool        `json:"supportsInteractiveAuth,omitempty"`
	SupportsProbeRead       bool        `json:"supportsProbeRead,omitempty"`
	SupportsProbeMutation   bool        `json:"supportsProbeMutation,omitempty"`
}

type Provenance struct {
	SecretResolution      string `json:"secretResolution,omitempty"`
	SecretMaterialPresent bool   `json:"secretMaterialPresent,omitempty"`
	EnvironmentScope      string `json:"environmentScope,omitempty"`
	BackedBy              string `json:"backedBy,omitempty"`
}

type Resource struct {
	IntegrationID          string          `json:"integrationId"`
	DomainKind             string          `json:"domainKind"`
	DisplayName            string          `json:"displayName"`
	EnvironmentScope       string          `json:"environmentScope"`
	ReadinessStatus        ReadinessStatus `json:"readinessStatus"`
	AuthState              AuthState       `json:"authState,omitempty"`
	HealthState            HealthState     `json:"healthState,omitempty"`
	ReadinessReason        string          `json:"readinessReason,omitempty"`
	RequiredOperatorAction string          `json:"requiredOperatorAction,omitempty"`
	CanonicalDefault       bool            `json:"canonicalDefault"`
	AccountBinding         AccountBinding  `json:"accountBinding,omitempty"`
	BackendBinding         BackendBinding  `json:"backendBinding"`
	Provenance             Provenance      `json:"provenance,omitempty"`
	CreatedAt              time.Time       `json:"createdAt"`
	UpdatedAt              time.Time       `json:"updatedAt"`
	LastReadyAt            *time.Time      `json:"lastReadyAt,omitempty"`
	LastTransitionAt       time.Time       `json:"lastTransitionAt"`
}

type BindingSummary struct {
	IntegrationID         string          `json:"integrationId"`
	DomainKind            string          `json:"domainKind"`
	DisplayName           string          `json:"displayName"`
	AccountKey            string          `json:"accountKey,omitempty"`
	CanonicalDefault      bool            `json:"canonicalDefault"`
	ReadinessAtInvocation ReadinessStatus `json:"readinessAtInvocation"`
	BackendKind           BackendKind     `json:"backendKind"`
	SecretResolution      string          `json:"secretResolution,omitempty"`
	EnvironmentScope      string          `json:"environmentScope"`
	CapturedAt            time.Time       `json:"capturedAt"`
}

type CreateInput struct {
	IntegrationID    string
	DomainKind       string
	DisplayName      string
	AccountBinding   AccountBinding
	BackendBinding   BackendBinding
	CanonicalDefault bool
	EnvironmentScope string
}

type UpdateReadinessInput struct {
	ReadinessStatus        ReadinessStatus `json:"readinessStatus"`
	AuthState              AuthState       `json:"authState,omitempty"`
	HealthState            HealthState     `json:"healthState,omitempty"`
	ReadinessReason        string          `json:"reason,omitempty"`
	RequiredOperatorAction string          `json:"requiredOperatorAction,omitempty"`
	AccountBinding         AccountBinding  `json:"accountBinding,omitempty"`
	SecretResolution       string          `json:"secretResolution,omitempty"`
}

type ProbeResult struct {
	ProbeKind     ProbeKind      `json:"probeKind"`
	Status        string         `json:"status"`
	ResultSummary map[string]any `json:"resultSummary,omitempty"`
	FailureClass  string         `json:"failureClass,omitempty"`
}
