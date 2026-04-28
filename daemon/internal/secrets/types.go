package secrets

import (
	"errors"
	"time"
)

type SecretStatus string

const (
	SecretStatusActive             SecretStatus = "active"
	SecretStatusDisabled           SecretStatus = "disabled"
	SecretStatusPendingRemediation SecretStatus = "pending_remediation"
)

type SecretVersionStatus string

const (
	SecretVersionStatusActive             SecretVersionStatus = "active"
	SecretVersionStatusSuperseded         SecretVersionStatus = "superseded"
	SecretVersionStatusDisabled           SecretVersionStatus = "disabled"
	SecretVersionStatusPendingRemediation SecretVersionStatus = "pending_remediation"
)

type ResolutionStatus string

const (
	ResolutionStatusResolved      ResolutionStatus = "resolved"
	ResolutionStatusUnavailable   ResolutionStatus = "unavailable"
	ResolutionStatusDenied        ResolutionStatus = "denied"
	ResolutionStatusNotApplicable ResolutionStatus = "not_applicable"
)

type AuditAction string

const (
	AuditActionSecretCreate     AuditAction = "secret.create"
	AuditActionSecretUpdate     AuditAction = "secret.update_metadata"
	AuditActionSecretRotate     AuditAction = "secret.rotate"
	AuditActionSecretDisable    AuditAction = "secret.disable"
	AuditActionSecretUse        AuditAction = "secret.use"
	AuditActionCredentialDenied AuditAction = "credential.denied"
)

type ResourceKind string

const (
	ResourceKindTenantSecret       ResourceKind = "tenant_secret"
	ResourceKindSecretVersion      ResourceKind = "tenant_secret_version"
	ResourceKindIntegration        ResourceKind = "integration"
	ResourceKindProviderAuthState  ResourceKind = "provider_auth_state"
	ResourceKindConnector          ResourceKind = "connector"
	ResourceKindMCPServer          ResourceKind = "mcp_server"
	ResourceKindMCPTool            ResourceKind = "mcp_tool"
	ResourceKindSandboxPolicy      ResourceKind = "sandbox_policy"
	ResourceKindDisabledCredential ResourceKind = "disabled_credential_resource"
)

var (
	ErrTenantRequired        = errors.New("tenant id is required")
	ErrSecretRefRequired     = errors.New("secret ref is required")
	ErrSecretValueRequired   = errors.New("secret value is required")
	ErrSecretNotFound        = errors.New("tenant secret not found")
	ErrSecretDisabled        = errors.New("tenant secret is disabled")
	ErrSecretVersionNotFound = errors.New("tenant secret version not found")
	ErrCrossTenantSecret     = errors.New("cross-tenant credential access denied")
)

type TenantSecret struct {
	SecretID          string         `json:"secretId"`
	TenantID          string         `json:"tenantId"`
	SecretRef         string         `json:"secretRef"`
	DisplayName       string         `json:"displayName,omitempty"`
	Status            SecretStatus   `json:"status"`
	ActiveVersionID   string         `json:"activeVersionId,omitempty"`
	DisabledReason    string         `json:"disabledReason,omitempty"`
	RemediationReason string         `json:"remediationReason,omitempty"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
	RotatedAt         *time.Time     `json:"rotatedAt,omitempty"`
	DisabledAt        *time.Time     `json:"disabledAt,omitempty"`
	Document          map[string]any `json:"document,omitempty"`
}

type SecretVersion struct {
	SecretVersionID string              `json:"secretVersionId"`
	SecretID        string              `json:"secretId"`
	TenantID        string              `json:"tenantId"`
	SecretRef       string              `json:"secretRef"`
	VersionNumber   int                 `json:"versionNumber"`
	Status          SecretVersionStatus `json:"status"`
	ValueBackendRef string              `json:"valueBackendRef,omitempty"`
	CreatedAt       time.Time           `json:"createdAt"`
	ActivatedAt     *time.Time          `json:"activatedAt,omitempty"`
	SupersededAt    *time.Time          `json:"supersededAt,omitempty"`
}

type ResolvedSecret struct {
	TenantID        string           `json:"tenantId"`
	SecretID        string           `json:"secretId"`
	SecretRef       string           `json:"secretRef"`
	SecretVersionID string           `json:"secretVersionId"`
	Resolution      ResolutionStatus `json:"resolution"`
	Value           string           `json:"-"`
	ResolvedAt      time.Time        `json:"resolvedAt"`
}

type DisabledResource struct {
	TenantID          string       `json:"tenantId"`
	ResourceKind      ResourceKind `json:"resourceKind"`
	ResourceID        string       `json:"resourceId"`
	Status            SecretStatus `json:"status"`
	DisabledReason    string       `json:"disabledReason,omitempty"`
	RemediationReason string       `json:"remediationReason,omitempty"`
	SecretRefs        []string     `json:"secretRefs,omitempty"`
	UpdatedAt         time.Time    `json:"updatedAt"`
}

type BridgedCredentialResource struct {
	TenantID       string       `json:"tenantId"`
	ResourceKind   ResourceKind `json:"resourceKind"`
	ResourceID     string       `json:"resourceId"`
	Status         string       `json:"status,omitempty"`
	DisabledReason string       `json:"disabledReason,omitempty"`
	SecretRefs     []string     `json:"secretRefs,omitempty"`
	UpdatedAt      time.Time    `json:"updatedAt"`
}

type LegacyCredentialResourceBridgeInput struct {
	TenantID           string
	ActiveSecretRefs   []string
	DisabledSecretRefs []string
}

type LegacyCredentialResourceBridgeResult struct {
	Bridged  []BridgedCredentialResource `json:"bridged,omitempty"`
	Disabled []BridgedCredentialResource `json:"disabled,omitempty"`
}

type AuditRecord struct {
	TenantID        string       `json:"tenantId"`
	PrincipalID     string       `json:"principalId,omitempty"`
	ResourceKind    ResourceKind `json:"resourceKind"`
	ResourceID      string       `json:"resourceId,omitempty"`
	Action          AuditAction  `json:"action"`
	Outcome         string       `json:"outcome"`
	ReasonCode      string       `json:"reasonCode,omitempty"`
	SecretRef       string       `json:"secretRef,omitempty"`
	SecretVersionID string       `json:"secretVersionId,omitempty"`
	SecretRefCount  int          `json:"secretRefCount,omitempty"`
	CreatedAt       time.Time    `json:"createdAt"`
}

type CreateInput struct {
	TenantID    string
	SecretRef   string
	DisplayName string
	Value       string
	Document    map[string]any
}

type UpdateMetadataInput struct {
	TenantID    string
	SecretRef   string
	DisplayName *string
	Document    map[string]any
}

type RotateInput struct {
	TenantID  string
	SecretRef string
	Value     string
}

type DisableInput struct {
	TenantID       string
	SecretRef      string
	DisabledReason string
}

type CreateDisabledMetadataInput struct {
	TenantID          string
	SecretRef         string
	DisplayName       string
	DisabledReason    string
	RemediationReason string
	Document          map[string]any
}

type ResolveInput struct {
	TenantID  string
	SecretRef string
}
