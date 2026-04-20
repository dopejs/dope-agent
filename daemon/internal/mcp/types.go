package mcp

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
)

type Source string

const (
	SourceAPI     Source = "api"
	SourceConfig  Source = "config"
	SourceBuiltin Source = "builtin"
)

type TransportKind string

const (
	TransportKindStdio          TransportKind = "stdio"
	TransportKindStreamableHTTP TransportKind = "streamable-http"
)

type OriginKind string

const (
	OriginKindManual  OriginKind = "manual"
	OriginKindCatalog OriginKind = "catalog"
)

type InstallMethod string

const (
	InstallMethodAPI    InstallMethod = "api"
	InstallMethodScript InstallMethod = "script"
)

type AvailabilityStatus string

const (
	AvailabilityStatusReady       AvailabilityStatus = "ready"
	AvailabilityStatusBlocked     AvailabilityStatus = "blocked"
	AvailabilityStatusUnavailable AvailabilityStatus = "unavailable"
	AvailabilityStatusUnsupported AvailabilityStatus = "unsupported"
)

type LifecycleStatus string

const (
	LifecycleStatusDisabled    LifecycleStatus = "disabled"
	LifecycleStatusStopped     LifecycleStatus = "stopped"
	LifecycleStatusStarting    LifecycleStatus = "starting"
	LifecycleStatusHealthy     LifecycleStatus = "healthy"
	LifecycleStatusDegraded    LifecycleStatus = "degraded"
	LifecycleStatusBackingOff  LifecycleStatus = "backing_off"
	LifecycleStatusFailed      LifecycleStatus = "failed"
	LifecycleStatusStopping    LifecycleStatus = "stopping"
	LifecycleStatusDenied      LifecycleStatus = "denied"
	LifecycleStatusUnsupported LifecycleStatus = "unsupported"
)

type DiscoveryStatus string

const (
	DiscoveryStatusDiscovered  DiscoveryStatus = "discovered"
	DiscoveryStatusStale       DiscoveryStatus = "stale"
	DiscoveryStatusUnavailable DiscoveryStatus = "unavailable"
)

type ExposureMode string

const (
	ExposureModeBlocked          ExposureMode = "blocked"
	ExposureModeAllow            ExposureMode = "allow"
	ExposureModeApprovalRequired ExposureMode = "approval_required"
)

type Server struct {
	ServerID         string        `json:"serverId"`
	DisplayName      string        `json:"displayName"`
	Source           Source        `json:"source"`
	OriginKind       OriginKind    `json:"originKind,omitempty"`
	CatalogEntryID   string        `json:"catalogEntryId,omitempty"`
	InstallMethod    InstallMethod `json:"installMethod,omitempty"`
	EnvironmentScope string        `json:"environmentScope,omitempty"`
	Enabled          bool          `json:"enabled"`
	SandboxProfileID string        `json:"sandboxProfileId"`
	DeclarationID    string        `json:"declarationId"`
	Declaration      Declaration   `json:"declaration"`
	TransportKind    TransportKind `json:"transportKind"`
	Command          string        `json:"command"`
	Args             []string      `json:"args"`
	Endpoint         string        `json:"endpoint,omitempty"`
	WorkingDir       string        `json:"workingDir,omitempty"`
	SecretRefs       []string      `json:"secretRefs,omitempty"`
	AutoRestart      bool          `json:"autoRestart"`
	OperatorModified bool          `json:"operatorModified,omitempty"`
	CreatedAt        time.Time     `json:"createdAt"`
	UpdatedAt        time.Time     `json:"updatedAt"`
}

type ServerState struct {
	ServerID           string          `json:"serverId"`
	Status             LifecycleStatus `json:"status"`
	HealthReason       string          `json:"healthReason,omitempty"`
	FailureCount       int             `json:"failureCount"`
	RestartCount       int             `json:"restartCount"`
	LastStartedAt      *time.Time      `json:"lastStartedAt,omitempty"`
	LastStoppedAt      *time.Time      `json:"lastStoppedAt,omitempty"`
	LastHeartbeatAt    *time.Time      `json:"lastHeartbeatAt,omitempty"`
	NextRestartAt      *time.Time      `json:"nextRestartAt,omitempty"`
	LastExecutionID    string          `json:"lastExecutionId,omitempty"`
	LastPolicyRecordID string          `json:"lastPolicyRecordId,omitempty"`
	UpdatedAt          time.Time       `json:"updatedAt"`
}

type Tool struct {
	ServerID          string          `json:"serverId"`
	ToolName          string          `json:"toolName"`
	Title             string          `json:"title,omitempty"`
	Description       string          `json:"description,omitempty"`
	SchemaFingerprint string          `json:"schemaFingerprint,omitempty"`
	DiscoveryStatus   DiscoveryStatus `json:"discoveryStatus"`
	LastDiscoveredAt  *time.Time      `json:"lastDiscoveredAt,omitempty"`
	UpdatedAt         time.Time       `json:"updatedAt"`
}

type ToolExposureRule struct {
	ServerID       string       `json:"serverId"`
	ToolName       string       `json:"toolName"`
	RuntimeSurface string       `json:"runtimeSurface"`
	ExposureMode   ExposureMode `json:"exposureMode"`
	Active         bool         `json:"active"`
	Reason         string       `json:"reason,omitempty"`
	UpdatedAt      time.Time    `json:"updatedAt"`
}

type SecretSummary struct {
	ConsumerID       string `json:"consumerId"`
	SecretRef        string `json:"secretRef"`
	EnvironmentScope string `json:"environmentScope"`
	DefaultRuleID    string `json:"defaultRuleId,omitempty"`
	Resolution       string `json:"resolution"`
	DeliveryKind     string `json:"deliveryKind,omitempty"`
	RedactionRule    string `json:"redactionRule,omitempty"`
}

type Declaration struct {
	ExecutionMode               sandbox.ExecutionMode `json:"executionMode"`
	AllowedBackendKinds         []sandbox.BackendKind `json:"allowedBackendKinds,omitempty"`
	ReadRoots                   []string              `json:"readRoots,omitempty"`
	WriteRoots                  []string              `json:"writeRoots,omitempty"`
	NetworkMode                 sandbox.NetworkMode   `json:"networkMode,omitempty"`
	AllowedHosts                []string              `json:"allowedHosts,omitempty"`
	AllowedPorts                []int                 `json:"allowedPorts,omitempty"`
	AllowLoopback               bool                  `json:"allowLoopback,omitempty"`
	ApprovalMode                sandbox.ApprovalMode  `json:"approvalMode,omitempty"`
	RequiredEnforcementStrength string                `json:"requiredEnforcementStrength,omitempty"`
	Active                      bool                  `json:"active"`
}

type ServerResource struct {
	Server
	State                  ServerState        `json:"state"`
	SecretSummary          []SecretSummary    `json:"secretSummary,omitempty"`
	ToolCount              int                `json:"toolCount"`
	Tools                  []ToolResource     `json:"tools,omitempty"`
	TransportConfigSummary string             `json:"transportConfigSummary,omitempty"`
	AvailabilityStatus     AvailabilityStatus `json:"availabilityStatus,omitempty"`
	AvailabilityReason     string             `json:"availabilityReason,omitempty"`
}

type ToolResource struct {
	Tool
	Exposure              []ToolExposureRule `json:"exposure,omitempty"`
	EffectiveAvailability string             `json:"effectiveAvailability"`
	ApprovalRequired      bool               `json:"approvalRequired"`
	UnavailableReason     string             `json:"unavailableReason,omitempty"`
}

type CreateServerInput struct {
	ServerID         string        `json:"serverId"`
	DisplayName      string        `json:"displayName"`
	OriginKind       OriginKind    `json:"originKind,omitempty"`
	CatalogEntryID   string        `json:"catalogEntryId,omitempty"`
	InstallMethod    InstallMethod `json:"installMethod,omitempty"`
	EnvironmentScope string        `json:"environmentScope,omitempty"`
	Enabled          bool          `json:"enabled"`
	SandboxProfileID string        `json:"sandboxProfileId"`
	DeclarationID    string        `json:"declarationId"`
	Declaration      *Declaration  `json:"declaration,omitempty"`
	TransportKind    TransportKind `json:"transportKind"`
	Command          string        `json:"command"`
	Args             []string      `json:"args"`
	Endpoint         string        `json:"endpoint,omitempty"`
	WorkingDir       string        `json:"workingDir"`
	SecretRefs       []string      `json:"secretRefs"`
	AutoRestart      bool          `json:"autoRestart"`
	OperatorModified bool          `json:"operatorModified,omitempty"`
}

type UpdateServerInput struct {
	DisplayName      *string        `json:"displayName,omitempty"`
	Enabled          *bool          `json:"enabled,omitempty"`
	SandboxProfileID *string        `json:"sandboxProfileId,omitempty"`
	DeclarationID    *string        `json:"declarationId,omitempty"`
	Declaration      *Declaration   `json:"declaration,omitempty"`
	TransportKind    *TransportKind `json:"transportKind,omitempty"`
	Command          *string        `json:"command,omitempty"`
	Args             []string       `json:"args,omitempty"`
	Endpoint         *string        `json:"endpoint,omitempty"`
	WorkingDir       *string        `json:"workingDir,omitempty"`
	SecretRefs       []string       `json:"secretRefs,omitempty"`
	AutoRestart      *bool          `json:"autoRestart,omitempty"`
}

type UpdateExposureInput struct {
	RuntimeSurface string       `json:"runtimeSurface"`
	ExposureMode   ExposureMode `json:"exposureMode"`
	Active         bool         `json:"active"`
	Reason         string       `json:"reason,omitempty"`
}

type LifecycleAction string

const (
	LifecycleActionStart   LifecycleAction = "start"
	LifecycleActionStop    LifecycleAction = "stop"
	LifecycleActionRestart LifecycleAction = "restart"
	LifecycleActionCancel  LifecycleAction = "cancel"
)

type LifecycleResponse struct {
	Action        LifecycleAction `json:"action"`
	Server        ServerResource  `json:"server"`
	Idempotent    bool            `json:"idempotent"`
	ExecutionID   string          `json:"executionId,omitempty"`
	FailureClass  string          `json:"failureClass,omitempty"`
	Blocked       bool            `json:"blocked"`
	BlockedReason string          `json:"blockedReason,omitempty"`
	PreflightMs   int64           `json:"preflightMs"`
}

type AuthorizeToolInput struct {
	RuntimeSurface string `json:"runtimeSurface"`
	ApprovalID     string `json:"approvalId,omitempty"`
	RequestedBy    string `json:"requestedBy,omitempty"`
}

type ToolAuthorizationStatus string

const (
	ToolAuthorizationStatusAllowed  ToolAuthorizationStatus = "allowed"
	ToolAuthorizationStatusPending  ToolAuthorizationStatus = "pending"
	ToolAuthorizationStatusRejected ToolAuthorizationStatus = "rejected"
	ToolAuthorizationStatusBlocked  ToolAuthorizationStatus = "blocked"
)

type ToolAuthorizationResponse struct {
	Status    ToolAuthorizationStatus       `json:"status"`
	Tool      ToolResource                  `json:"tool"`
	SessionID string                        `json:"sessionId,omitempty"`
	Message   string                        `json:"message,omitempty"`
	Approval  *policy.Approval              `json:"approval,omitempty"`
	Decision  *policy.Decision              `json:"decision,omitempty"`
	Sandbox   *sandbox.ConsumerContractView `json:"sandbox,omitempty"`
}

type CatalogInstallSupport struct {
	ScriptSupported bool     `json:"scriptSupported"`
	ScriptArgs      []string `json:"scriptArgs,omitempty"`
}

type CatalogPrerequisite struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
}

type CatalogSecretRequirement struct {
	SecretRef   string `json:"secretRef"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
}

type CatalogEntry struct {
	ID                     string                     `json:"id"`
	DisplayName            string                     `json:"displayName"`
	Description            string                     `json:"description"`
	TransportKind          TransportKind              `json:"transportKind"`
	SourceKind             string                     `json:"sourceKind"`
	Tags                   []string                   `json:"tags,omitempty"`
	ImmediateUse           bool                       `json:"immediateUse"`
	Prerequisites          []CatalogPrerequisite      `json:"prerequisites,omitempty"`
	SecretRequirements     []CatalogSecretRequirement `json:"secretRequirements,omitempty"`
	EnvironmentEligibility []string                   `json:"environmentEligibility,omitempty"`
	AvailabilityStatus     AvailabilityStatus         `json:"availabilityStatus"`
	AvailabilityReason     string                     `json:"availabilityReason,omitempty"`
	InstallSupport         CatalogInstallSupport      `json:"installSupport"`
	DefaultInstallSpec     CreateServerInput          `json:"defaultInstallSpec"`
}

type CatalogInstallInput struct {
	ServerID         string   `json:"serverId,omitempty"`
	DisplayName      string   `json:"displayName,omitempty"`
	Enabled          *bool    `json:"enabled,omitempty"`
	SandboxProfileID string   `json:"sandboxProfileId,omitempty"`
	Command          string   `json:"command,omitempty"`
	Args             []string `json:"args,omitempty"`
	Endpoint         string   `json:"endpoint,omitempty"`
	WorkingDir       string   `json:"workingDir,omitempty"`
	SecretRefs       []string `json:"secretRefs,omitempty"`
}

type CatalogInstallResult struct {
	InstallID          string             `json:"installId"`
	Status             string             `json:"status"`
	CatalogEntryID     string             `json:"catalogEntryId"`
	ServerID           string             `json:"serverId,omitempty"`
	AvailabilityStatus AvailabilityStatus `json:"availabilityStatus"`
	AvailabilityReason string             `json:"availabilityReason,omitempty"`
	AuditEventIDs      []string           `json:"auditEventIds,omitempty"`
	Server             *ServerResource    `json:"server,omitempty"`
}

type ToolInvocationResult struct {
	SessionID    string `json:"sessionId,omitempty"`
	Output       any    `json:"output,omitempty"`
	FailureClass string `json:"failureClass,omitempty"`
	Error        string `json:"error,omitempty"`
}

func IsTerminalStatus(status LifecycleStatus) bool {
	switch status {
	case LifecycleStatusDisabled, LifecycleStatusStopped, LifecycleStatusFailed, LifecycleStatusDenied, LifecycleStatusUnsupported:
		return true
	default:
		return false
	}
}
