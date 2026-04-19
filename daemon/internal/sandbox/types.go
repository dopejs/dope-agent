package sandbox

import "time"

type Source string

const (
	SourceBuiltin Source = "builtin"
)

const (
	ProfileIDSubprocessDefault     = "subprocess_default"
	ProfileIDManagedProviderClaude = "managed_provider_claude"
	ProfileIDManagedProviderCodex  = "managed_provider_codex"
)

type BackendKind string

const (
	BackendKindSubprocess BackendKind = "subprocess"
	BackendKindDocker     BackendKind = "docker"
	BackendKindSSH        BackendKind = "ssh"
	BackendKindRemote     BackendKind = "remote"
)

type FilesystemMode string

const (
	FilesystemModeNone   FilesystemMode = "none"
	FilesystemModeScoped FilesystemMode = "scoped"
	FilesystemModeFull   FilesystemMode = "full"
)

type NetworkMode string

const (
	NetworkModeDeny      NetworkMode = "deny"
	NetworkModeAllowList NetworkMode = "allow_list"
	NetworkModeFull      NetworkMode = "full"
)

type EnvironmentMode string

const (
	EnvironmentModeClean       EnvironmentMode = "clean"
	EnvironmentModeInheritSafe EnvironmentMode = "inherit_safe"
	EnvironmentModeInheritAll  EnvironmentMode = "inherit_all"
)

type ApprovalMode string

const (
	ApprovalModeAllow ApprovalMode = "allow"
	ApprovalModeAsk   ApprovalMode = "ask"
	ApprovalModeDeny  ApprovalMode = "deny"
)

type DecisionResolution string

const (
	DecisionResolutionAllow DecisionResolution = "allow"
	DecisionResolutionAsk   DecisionResolution = "ask"
	DecisionResolutionDeny  DecisionResolution = "deny"
)

type DecisionApprovalStatus string

const (
	DecisionApprovalStatusNotApplicable DecisionApprovalStatus = "not_applicable"
	DecisionApprovalStatusPending       DecisionApprovalStatus = "pending"
	DecisionApprovalStatusApproved      DecisionApprovalStatus = "approved"
	DecisionApprovalStatusRejected      DecisionApprovalStatus = "rejected"
)

type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
	ExecutionStatusDenied    ExecutionStatus = "denied"
)

type ErrorClass string

const (
	ErrorClassNone             ErrorClass = ""
	ErrorClassPolicyDenied     ErrorClass = "policy_denied"
	ErrorClassApprovalRequired ErrorClass = "approval_required"
	ErrorClassApprovalRejected ErrorClass = "approval_rejected"
	ErrorClassInvalidProfile   ErrorClass = "invalid_profile"
	ErrorClassBackendMissing   ErrorClass = "backend_unavailable"
	ErrorClassLaunchFailed     ErrorClass = "launch_failed"
	ErrorClassProcessFailed    ErrorClass = "process_failed"
	ErrorClassProviderFailed   ErrorClass = "provider_error"
	ErrorClassProviderAuth     ErrorClass = "provider_auth_failed"
	ErrorClassTimeout          ErrorClass = "timeout"
	ErrorClassCancelled        ErrorClass = "cancelled"
	ErrorClassIOCaptureFailed  ErrorClass = "io_capture_failed"
)

type ManagedProviderActionKind string

const (
	ManagedProviderActionAuthStatus      ManagedProviderActionKind = "auth_status"
	ManagedProviderActionLogout          ManagedProviderActionKind = "logout"
	ManagedProviderActionPromptExecution ManagedProviderActionKind = "prompt_execution"
)

type ManagedProviderOperationStatus string

const (
	ManagedProviderOperationStatusPending              ManagedProviderOperationStatus = "pending"
	ManagedProviderOperationStatusDenied               ManagedProviderOperationStatus = "denied"
	ManagedProviderOperationStatusLocalStateInspection ManagedProviderOperationStatus = "local_state_inspection"
	ManagedProviderOperationStatusRunning              ManagedProviderOperationStatus = "running"
	ManagedProviderOperationStatusCompleted            ManagedProviderOperationStatus = "completed"
	ManagedProviderOperationStatusFailed               ManagedProviderOperationStatus = "failed"
	ManagedProviderOperationStatusCancelled            ManagedProviderOperationStatus = "cancelled"
)

type LocalStateAccessMode string

const (
	LocalStateAccessModeRead  LocalStateAccessMode = "read"
	LocalStateAccessModeWrite LocalStateAccessMode = "write"
)

type ManagedProviderRequirementDeclaration struct {
	ProviderID            string                    `json:"providerId"`
	ActionKind            ManagedProviderActionKind `json:"actionKind"`
	ProfileID             string                    `json:"profileId"`
	BackendKind           BackendKind               `json:"backendKind"`
	ReadRoots             []string                  `json:"readRoots"`
	WriteRoots            []string                  `json:"writeRoots"`
	NetworkMode           NetworkMode               `json:"networkMode"`
	AllowedHosts          []string                  `json:"allowedHosts"`
	AllowedPorts          []int                     `json:"allowedPorts"`
	ApprovalMode          ApprovalMode              `json:"approvalMode"`
	SensitiveStateClasses []string                  `json:"sensitiveStateClasses"`
	EnforcementStrength   string                    `json:"enforcementStrength"`
	Active                bool                      `json:"active"`
}

type SensitiveLocalStateAccessSummary struct {
	ProviderID    string                    `json:"providerId"`
	ActionKind    ManagedProviderActionKind `json:"actionKind"`
	StateClass    string                    `json:"stateClass"`
	AccessMode    LocalStateAccessMode      `json:"accessMode"`
	PathSummary   string                    `json:"pathSummary"`
	Declared      bool                      `json:"declared"`
	Sensitive     bool                      `json:"sensitive"`
	RedactionRule string                    `json:"redactionRule"`
}

type ManagedProviderOperation struct {
	OperationID               string                             `json:"operationId"`
	ProviderID                string                             `json:"providerId"`
	ActionKind                ManagedProviderActionKind          `json:"actionKind"`
	RequestedBy               string                             `json:"requestedBy"`
	RequirementProfileID      string                             `json:"requirementProfileId"`
	Decision                  DecisionResolution                 `json:"decision"`
	ApprovalStatus            DecisionApprovalStatus             `json:"approvalStatus"`
	FailureClass              string                             `json:"failureClass,omitempty"`
	EnforcementStrength       string                             `json:"enforcementStrength"`
	SensitiveStateClasses     []string                           `json:"sensitiveStateClasses,omitempty"`
	ExecutionID               string                             `json:"executionId,omitempty"`
	StartedAt                 time.Time                          `json:"startedAt"`
	CompletedAt               *time.Time                         `json:"completedAt,omitempty"`
	Status                    ManagedProviderOperationStatus     `json:"status"`
	LocalStateAccessSummaries []SensitiveLocalStateAccessSummary `json:"localStateAccessSummaries,omitempty"`
}

type FilesystemPolicy struct {
	Mode               FilesystemMode `json:"mode"`
	ReadRoots          []string       `json:"readRoots"`
	WriteRoots         []string       `json:"writeRoots"`
	TempRoots          []string       `json:"tempRoots"`
	AllowDataDir       bool           `json:"allowDataDir"`
	AllowUserAgentsDir bool           `json:"allowUserAgentsDir"`
	AllowHomeRead      bool           `json:"allowHomeRead"`
	AllowHomeWrite     bool           `json:"allowHomeWrite"`
}

type NetworkPolicy struct {
	Mode            NetworkMode `json:"mode"`
	AllowedHosts    []string    `json:"allowedHosts"`
	AllowedPorts    []int       `json:"allowedPorts"`
	AllowLoopback   bool        `json:"allowLoopback"`
	EnforcementMode string      `json:"enforcementMode"`
}

type EnvironmentPolicy struct {
	Mode         EnvironmentMode   `json:"mode"`
	AllowedVars  []string          `json:"allowedVars"`
	InjectedVars map[string]string `json:"injectedVars"`
	RedactedVars []string          `json:"redactedVars"`
}

type ApprovalPolicy struct {
	Mode                          ApprovalMode `json:"mode"`
	RequiredForCommands           []string     `json:"requiredForCommands"`
	RequiredForWritesOutsideRoots bool         `json:"requiredForWritesOutsideRoots"`
	RequiredForNetwork            bool         `json:"requiredForNetwork"`
	RequiredForUnknownBackends    bool         `json:"requiredForUnknownBackends"`
}

type ProcessPolicy struct {
	TimeoutMs        int  `json:"timeoutMs"`
	MaxTimeoutMs     int  `json:"maxTimeoutMs"`
	KillGraceMs      int  `json:"killGraceMs"`
	CaptureStdout    bool `json:"captureStdout"`
	CaptureStderr    bool `json:"captureStderr"`
	MaxOutputBytes   int  `json:"maxOutputBytes"`
	AllowStreaming   bool `json:"allowStreaming"`
	RestartOnFailure bool `json:"restartOnFailure"`
}

type Profile struct {
	ProfileID        string            `json:"profileId"`
	Title            string            `json:"title"`
	Description      string            `json:"description"`
	BackendKind      BackendKind       `json:"backendKind"`
	DefaultWorkDir   string            `json:"defaultWorkDir"`
	FilesystemPolicy FilesystemPolicy  `json:"filesystemPolicy"`
	NetworkPolicy    NetworkPolicy     `json:"networkPolicy"`
	EnvPolicy        EnvironmentPolicy `json:"envPolicy"`
	ApprovalPolicy   ApprovalPolicy    `json:"approvalPolicy"`
	ProcessPolicy    ProcessPolicy     `json:"processPolicy"`
	DefaultTimeoutMs int               `json:"defaultTimeoutMs"`
	MaxTimeoutMs     int               `json:"maxTimeoutMs"`
	Restartable      bool              `json:"restartable"`
	Source           Source            `json:"source"`
	Active           bool              `json:"active"`
}

type AccessRequest struct {
	ReadRoots     []string    `json:"readRoots"`
	WriteRoots    []string    `json:"writeRoots"`
	NetworkMode   NetworkMode `json:"networkMode,omitempty"`
	AllowedHosts  []string    `json:"allowedHosts"`
	AllowedPorts  []int       `json:"allowedPorts"`
	AllowLoopback bool        `json:"allowLoopback,omitempty"`
}

type ExecutionRequest struct {
	ProfileID    string            `json:"profileId,omitempty"`
	Command      string            `json:"command"`
	Args         []string          `json:"args"`
	Cwd          string            `json:"cwd,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	Stdin        string            `json:"stdin,omitempty"`
	TimeoutMs    int               `json:"timeoutMs,omitempty"`
	RequestedBy  string            `json:"requestedBy,omitempty"`
	ResourceKind string            `json:"resourceKind,omitempty"`
	ResourceID   string            `json:"resourceId,omitempty"`
	Scope        string            `json:"scope,omitempty"`
	ApprovalID   string            `json:"approvalId,omitempty"`
	Reason       string            `json:"reason,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Access       AccessRequest     `json:"access"`
}

type Decision struct {
	DecisionID           string                 `json:"decisionId"`
	ExecutionID          string                 `json:"executionId,omitempty"`
	Resolution           DecisionResolution     `json:"resolution"`
	MatchedRules         []string               `json:"matchedRules"`
	ApprovalRequired     bool                   `json:"approvalRequired"`
	ApprovalStatus       DecisionApprovalStatus `json:"approvalStatus"`
	EffectiveProfileID   string                 `json:"effectiveProfileId"`
	EffectiveBackendKind BackendKind            `json:"effectiveBackendKind"`
	Explanation          string                 `json:"explanation"`
}

type Result struct {
	ExecutionID     string          `json:"executionId,omitempty"`
	Status          ExecutionStatus `json:"status"`
	StartedAt       *time.Time      `json:"startedAt,omitempty"`
	CompletedAt     *time.Time      `json:"completedAt,omitempty"`
	ExitCode        *int            `json:"exitCode,omitempty"`
	Signal          string          `json:"signal,omitempty"`
	Stdout          string          `json:"stdout,omitempty"`
	Stderr          string          `json:"stderr,omitempty"`
	OutputTruncated bool            `json:"outputTruncated"`
	ErrorClass      ErrorClass      `json:"errorClass,omitempty"`
	ErrorCode       string          `json:"errorCode,omitempty"`
	Error           string          `json:"error,omitempty"`
	Partial         bool            `json:"partial"`
	BackendMetadata map[string]any  `json:"backendMetadata,omitempty"`
}

type ExecutionFinalization struct {
	Status     ExecutionStatus `json:"status,omitempty"`
	ErrorClass ErrorClass      `json:"errorClass,omitempty"`
	ErrorCode  string          `json:"errorCode,omitempty"`
	Error      string          `json:"error,omitempty"`
}

type Execution struct {
	ExecutionID   string            `json:"executionId"`
	ProfileID     string            `json:"profileId"`
	BackendKind   BackendKind       `json:"backendKind"`
	Command       string            `json:"command"`
	Args          []string          `json:"args"`
	Cwd           string            `json:"cwd"`
	EnvKeys       []string          `json:"envKeys"`
	StdinProvided bool              `json:"stdinProvided"`
	TimeoutMs     int               `json:"timeoutMs"`
	RequestedBy   string            `json:"requestedBy,omitempty"`
	ResourceKind  string            `json:"resourceKind,omitempty"`
	ResourceID    string            `json:"resourceId,omitempty"`
	Scope         string            `json:"scope,omitempty"`
	ApprovalID    string            `json:"approvalId,omitempty"`
	Reason        string            `json:"reason,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Access        AccessRequest     `json:"access"`
	Status        ExecutionStatus   `json:"status"`
	Decision      Decision          `json:"decision"`
	Result        Result            `json:"result"`
	RequestedAt   time.Time         `json:"requestedAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
	StartedAt     *time.Time        `json:"startedAt,omitempty"`
	CompletedAt   *time.Time        `json:"completedAt,omitempty"`
}

func IsTerminal(status ExecutionStatus) bool {
	switch status {
	case ExecutionStatusCompleted, ExecutionStatusFailed, ExecutionStatusCancelled, ExecutionStatusDenied:
		return true
	default:
		return false
	}
}
