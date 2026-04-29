package identity

import (
	"errors"
	"time"
)

type TenantKind string

const (
	TenantKindPersonal     TenantKind = "personal"
	TenantKindOrganization TenantKind = "organization"
)

type LifecycleStatus string

const (
	StatusInvited  LifecycleStatus = "invited"
	StatusPending  LifecycleStatus = "pending"
	StatusActive   LifecycleStatus = "active"
	StatusDisabled LifecycleStatus = "disabled"
	StatusRemoved  LifecycleStatus = "removed"
	StatusRejected LifecycleStatus = "rejected"
	StatusRevoked  LifecycleStatus = "revoked"
	StatusExpired  LifecycleStatus = "expired"
	StatusAccepted LifecycleStatus = "accepted"
	StatusRotated  LifecycleStatus = "rotated"
)

type PrincipalKind string

const (
	PrincipalKindLocalOperator PrincipalKind = "local_operator"
	PrincipalKindUser          PrincipalKind = "user"
	PrincipalKindServiceClient PrincipalKind = "service_client"
)

type Role string

const (
	RoleOwner    Role = "owner"
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

type Permission string

const (
	PermissionTenantManage          Permission = "tenant.manage"
	PermissionSecretsManage         Permission = "secrets.manage"
	PermissionCredentialsInspect    Permission = "credentials.inspect"
	PermissionIntegrationsManage    Permission = "integrations.manage"
	PermissionConnectorsManage      Permission = "connectors.manage"
	PermissionMCPManage             Permission = "mcp.manage"
	PermissionRunsExecute           Permission = "runs.execute"
	PermissionApprovalsResolve      Permission = "approvals.resolve"
	PermissionLiveValidationExecute Permission = "live_validation.execute"
	PermissionEvaluationManage      Permission = "evaluation.manage"
	PermissionBillingView           Permission = "billing.view"
	PermissionBillingManage         Permission = "billing.manage"
	PermissionReadOnlyInspect       Permission = "read_only.inspect"
)

var AllSensitivePermissions = []Permission{
	PermissionTenantManage,
	PermissionSecretsManage,
	PermissionCredentialsInspect,
	PermissionIntegrationsManage,
	PermissionConnectorsManage,
	PermissionMCPManage,
	PermissionRunsExecute,
	PermissionApprovalsResolve,
	PermissionLiveValidationExecute,
	PermissionEvaluationManage,
	PermissionBillingView,
	PermissionBillingManage,
}

var (
	ErrTenantAccessDenied = errors.New("tenant access denied")
	ErrPermissionDenied   = errors.New("tenant permission denied")
	ErrAuditWriteFailed   = errors.New("tenant audit write failed")
	ErrOwnerInvariant     = errors.New("organization tenant requires at least one active owner")
	ErrInvitationInvalid  = errors.New("tenant invitation is invalid")
	ErrPrincipalInvalid   = errors.New("principal is invalid")
	ErrTokenGrantInvalid  = errors.New("token tenant grant is invalid")
	ErrTenantInvalid      = errors.New("tenant is invalid")
	ErrMembershipInvalid  = errors.New("membership is invalid")
	ErrUnsupportedRole    = errors.New("tenant role is invalid")
	ErrUnsupportedStatus  = errors.New("tenant lifecycle status is invalid")
	ErrUnsupportedTenant  = errors.New("tenant kind is invalid")
)

type Denial struct {
	Error     string `json:"error"`
	ErrorCode string `json:"errorCode"`
	RequestID string `json:"requestId,omitempty"`
}

func StableDenial() Denial {
	return Denial{Error: "tenant access denied", ErrorCode: "tenant_access_denied"}
}

type Tenant struct {
	TenantID                   string          `json:"tenantId"`
	TenantKind                 TenantKind      `json:"tenantKind"`
	DisplayName                string          `json:"displayName"`
	Status                     LifecycleStatus `json:"status"`
	CreatedAt                  time.Time       `json:"createdAt"`
	UpdatedAt                  time.Time       `json:"updatedAt"`
	CreatedByPrincipalID       string          `json:"createdByPrincipalId,omitempty"`
	DefaultOwnerPrincipalID    string          `json:"defaultOwnerPrincipalId,omitempty"`
	CallerMembershipRole       Role            `json:"callerMembershipRole,omitempty"`
	CallerMembershipStatus     LifecycleStatus `json:"callerMembershipStatus,omitempty"`
	CallerPermissions          []Permission    `json:"callerPermissions,omitempty"`
	DefaultForCurrentToken     bool            `json:"defaultForCurrentToken,omitempty"`
	DefaultForCurrentPrincipal bool            `json:"defaultForCurrentPrincipal,omitempty"`
}

type Principal struct {
	PrincipalID     string          `json:"principalId"`
	PrincipalKind   PrincipalKind   `json:"principalKind"`
	DisplayName     string          `json:"displayName"`
	Status          LifecycleStatus `json:"status"`
	DefaultTenantID string          `json:"defaultTenantId"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
	DisabledAt      *time.Time      `json:"disabledAt,omitempty"`
	RemovedAt       *time.Time      `json:"removedAt,omitempty"`
}

type Membership struct {
	MembershipID string          `json:"membershipId"`
	TenantID     string          `json:"tenantId"`
	PrincipalID  string          `json:"principalId"`
	Role         Role            `json:"role"`
	Status       LifecycleStatus `json:"status"`
	InvitationID string          `json:"invitationId,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
	AcceptedAt   *time.Time      `json:"acceptedAt,omitempty"`
	RemovedAt    *time.Time      `json:"removedAt,omitempty"`
}

type TenantInvitation struct {
	InvitationID         string          `json:"invitationId"`
	TenantID             string          `json:"tenantId"`
	InvitedPrincipalID   string          `json:"invitedPrincipalId"`
	InvitedByPrincipalID string          `json:"invitedByPrincipalId"`
	Role                 Role            `json:"role"`
	Status               LifecycleStatus `json:"status"`
	CreatedAt            time.Time       `json:"createdAt"`
	UpdatedAt            time.Time       `json:"updatedAt"`
	ExpiresAt            *time.Time      `json:"expiresAt,omitempty"`
	DecidedAt            *time.Time      `json:"decidedAt,omitempty"`
}

type TokenTenantGrant struct {
	GrantID              string          `json:"grantId"`
	TokenID              string          `json:"tokenId"`
	TenantID             string          `json:"tenantId"`
	IsDefault            bool            `json:"isDefault"`
	Status               LifecycleStatus `json:"status"`
	CreatedAt            time.Time       `json:"createdAt"`
	UpdatedAt            time.Time       `json:"updatedAt"`
	RevokedAt            *time.Time      `json:"revokedAt,omitempty"`
	GrantedByPrincipalID string          `json:"grantedByPrincipalId,omitempty"`
}

type TenantContext struct {
	PrincipalID  string       `json:"principalId"`
	TokenID      string       `json:"tokenId"`
	TenantID     string       `json:"tenantId"`
	TenantSource string       `json:"tenantSource"`
	MembershipID string       `json:"membershipId,omitempty"`
	Role         Role         `json:"role,omitempty"`
	Permissions  []Permission `json:"permissions"`
	ResolvedAt   time.Time    `json:"resolvedAt"`
}

type PermissionEvaluation struct {
	Permission Permission `json:"permission"`
	Allowed    bool       `json:"allowed"`
	ReasonCode string     `json:"reasonCode,omitempty"`
}

type TenantAuditEvent struct {
	AuditEventID      string         `json:"auditEventId"`
	EventKind         string         `json:"eventKind"`
	TenantID          string         `json:"tenantId,omitempty"`
	PrincipalID       string         `json:"principalId,omitempty"`
	TargetPrincipalID string         `json:"targetPrincipalId,omitempty"`
	TokenID           string         `json:"tokenId,omitempty"`
	Outcome           string         `json:"outcome"`
	ReasonCode        string         `json:"reasonCode"`
	CreatedAt         time.Time      `json:"createdAt"`
	Document          map[string]any `json:"document,omitempty"`
}

type TenantFilter struct {
	TenantKind TenantKind
	Status     LifecycleStatus
	Limit      int
}

type PrincipalFilter struct {
	TenantID string
	Status   LifecycleStatus
	Limit    int
}

type MembershipFilter struct {
	TenantID string
	Status   LifecycleStatus
	Role     Role
	Limit    int
}

type InvitationFilter struct {
	TenantID    string
	PrincipalID string
	Status      LifecycleStatus
	Limit       int
}

type AuditEventFilter struct {
	TenantID    string
	PrincipalID string
	TokenID     string
	EventKind   string
	Outcome     string
	Limit       int
}
