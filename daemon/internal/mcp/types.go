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
	TransportKindStdio TransportKind = "stdio"
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
	Enabled          bool          `json:"enabled"`
	SandboxProfileID string        `json:"sandboxProfileId"`
	DeclarationID    string        `json:"declarationId"`
	Declaration      Declaration   `json:"declaration"`
	TransportKind    TransportKind `json:"transportKind"`
	Command          string        `json:"command"`
	Args             []string      `json:"args"`
	WorkingDir       string        `json:"workingDir,omitempty"`
	SecretRefs       []string      `json:"secretRefs,omitempty"`
	AutoRestart      bool          `json:"autoRestart"`
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
	State         ServerState     `json:"state"`
	SecretSummary []SecretSummary `json:"secretSummary,omitempty"`
	ToolCount     int             `json:"toolCount"`
	Tools         []ToolResource  `json:"tools,omitempty"`
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
	Enabled          bool          `json:"enabled"`
	SandboxProfileID string        `json:"sandboxProfileId"`
	DeclarationID    string        `json:"declarationId"`
	Declaration      *Declaration  `json:"declaration,omitempty"`
	TransportKind    TransportKind `json:"transportKind"`
	Command          string        `json:"command"`
	Args             []string      `json:"args"`
	WorkingDir       string        `json:"workingDir"`
	SecretRefs       []string      `json:"secretRefs"`
	AutoRestart      bool          `json:"autoRestart"`
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
	Status   ToolAuthorizationStatus       `json:"status"`
	Tool     ToolResource                  `json:"tool"`
	Message  string                        `json:"message,omitempty"`
	Approval *policy.Approval              `json:"approval,omitempty"`
	Decision *policy.Decision              `json:"decision,omitempty"`
	Sandbox  *sandbox.ConsumerContractView `json:"sandbox,omitempty"`
}

func IsTerminalStatus(status LifecycleStatus) bool {
	switch status {
	case LifecycleStatusDisabled, LifecycleStatusStopped, LifecycleStatusFailed, LifecycleStatusDenied, LifecycleStatusUnsupported:
		return true
	default:
		return false
	}
}
