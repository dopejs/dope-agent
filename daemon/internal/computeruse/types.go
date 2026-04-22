package computeruse

import "time"

type SessionStatus string

const (
	SessionStatusStarting    SessionStatus = "starting"
	SessionStatusActive      SessionStatus = "active"
	SessionStatusBlocked     SessionStatus = "blocked"
	SessionStatusClosing     SessionStatus = "closing"
	SessionStatusClosed      SessionStatus = "closed"
	SessionStatusFailed      SessionStatus = "failed"
	SessionStatusInterrupted SessionStatus = "interrupted"
)

type ActionStatus string

const (
	ActionStatusRequested       ActionStatus = "requested"
	ActionStatusWaitingApproval ActionStatus = "waiting_approval"
	ActionStatusRunning         ActionStatus = "running"
	ActionStatusCompleted       ActionStatus = "completed"
	ActionStatusDenied          ActionStatus = "denied"
	ActionStatusFailed          ActionStatus = "failed"
	ActionStatusInterrupted     ActionStatus = "interrupted"
)

type ActionKind string

const (
	ActionKindNavigate     ActionKind = "navigate"
	ActionKindBack         ActionKind = "back"
	ActionKindForward      ActionKind = "forward"
	ActionKindWait         ActionKind = "wait"
	ActionKindScreenshot   ActionKind = "screenshot"
	ActionKindSnapshot     ActionKind = "snapshot"
	ActionKindClick        ActionKind = "click"
	ActionKindInput        ActionKind = "input"
	ActionKindSelect       ActionKind = "select"
	ActionKindDownload     ActionKind = "download"
	ActionKindCloseSession ActionKind = "close_session"
)

type PageTarget string

const (
	PageTargetActivePage PageTarget = "active_page"
	PageTargetNewTab     PageTarget = "new_tab"
	PageTargetNewWindow  PageTarget = "new_window"
)

type RiskLevel string

const (
	RiskLevelLow  RiskLevel = "low"
	RiskLevelHigh RiskLevel = "high"
)

type MatchResult string

const (
	MatchResultMatched    MatchResult = "matched"
	MatchResultMismatched MatchResult = "mismatched"
)

type ArtifactKind string

const (
	ArtifactKindScreenshot   ArtifactKind = "screenshot"
	ArtifactKindPageSnapshot ArtifactKind = "page_snapshot"
	ArtifactKindDownload     ArtifactKind = "download"
)

type ArtifactStatus string

const (
	ArtifactStatusCapturing     ArtifactStatus = "capturing"
	ArtifactStatusAvailable     ArtifactStatus = "available"
	ArtifactStatusCaptureFailed ArtifactStatus = "capture_failed"
)

type FailureClass string

const (
	FailureClassPolicyDenied        FailureClass = "policy_denial"
	FailureClassUnavailableConsumer FailureClass = "unavailable_consumer"
	FailureClassNavigationFailure   FailureClass = "navigation_failure"
	FailureClassTargetMismatch      FailureClass = "target_mismatch"
	FailureClassUnsupportedAction   FailureClass = "unsupported_action"
	FailureClassInterrupted         FailureClass = "interrupted"
)

type PageSummary struct {
	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`
}

type TrustedPageScope struct {
	ScopeID              string    `json:"scopeId"`
	ComputerUseSessionID string    `json:"computerUseSessionId,omitempty"`
	Origin               string    `json:"origin,omitempty"`
	PageURL              string    `json:"pageUrl,omitempty"`
	Title                string    `json:"title,omitempty"`
	ScopeRevision        int       `json:"scopeRevision"`
	DerivedFromActionID  string    `json:"derivedFromActionId,omitempty"`
	CreatedAt            time.Time `json:"createdAt"`
}

type TargetMatchContext struct {
	MatchStrategy         string      `json:"matchStrategy,omitempty"`
	ExpectedPageURL       string      `json:"expectedPageUrl,omitempty"`
	ExpectedSelector      string      `json:"expectedSelector,omitempty"`
	ExpectedText          string      `json:"expectedText,omitempty"`
	TrustedScopeRevision  int         `json:"trustedScopeRevision,omitempty"`
	ObservedPageURL       string      `json:"observedPageUrl,omitempty"`
	ObservedSelectorState string      `json:"observedSelectorState,omitempty"`
	MatchResult           MatchResult `json:"matchResult,omitempty"`
	EvaluatedAt           time.Time   `json:"evaluatedAt,omitempty"`
}

type Artifact struct {
	ArtifactID           string         `json:"artifactId"`
	EnvironmentScope     string         `json:"environmentScope,omitempty"`
	ComputerUseSessionID string         `json:"computerUseSessionId,omitempty"`
	ComputerUseActionID  string         `json:"computerUseActionId,omitempty"`
	RunID                string         `json:"runId"`
	Kind                 ArtifactKind   `json:"kind"`
	Status               ArtifactStatus `json:"status"`
	MIMEType             string         `json:"mimeType,omitempty"`
	FileName             string         `json:"fileName,omitempty"`
	ByteSize             int64          `json:"byteSize,omitempty"`
	StorageKey           string         `json:"storageKey,omitempty"`
	SHA256               string         `json:"sha256,omitempty"`
	CreatedAt            time.Time      `json:"createdAt"`
	AvailableAt          *time.Time     `json:"availableAt,omitempty"`
	CaptureFailureReason string         `json:"captureFailureReason,omitempty"`
}

type Action struct {
	ComputerUseActionID  string              `json:"computerUseActionId"`
	EnvironmentScope     string              `json:"environmentScope,omitempty"`
	ComputerUseSessionID string              `json:"computerUseSessionId"`
	RunID                string              `json:"runId"`
	StepID               string              `json:"stepId,omitempty"`
	ToolCallID           string              `json:"toolCallId,omitempty"`
	WorkflowID           string              `json:"workflowId,omitempty"`
	WorkflowStepID       string              `json:"workflowStepId,omitempty"`
	ActionKind           ActionKind          `json:"actionKind"`
	Status               ActionStatus        `json:"status"`
	RiskLevel            RiskLevel           `json:"riskLevel"`
	ApprovalID           string              `json:"approvalId,omitempty"`
	TargetMatchContext   *TargetMatchContext `json:"targetMatchContext,omitempty"`
	PageBefore           *PageSummary        `json:"pageBefore,omitempty"`
	PageAfter            *PageSummary        `json:"pageAfter,omitempty"`
	FailureClass         string              `json:"failureClass,omitempty"`
	FailureReason        string              `json:"failureReason,omitempty"`
	RequestedAt          time.Time           `json:"requestedAt"`
	UpdatedAt            time.Time           `json:"updatedAt"`
	CompletedAt          *time.Time          `json:"completedAt,omitempty"`
	Input                map[string]any      `json:"input,omitempty"`
	Artifacts            []Artifact          `json:"artifacts,omitempty"`
}

type Session struct {
	ComputerUseSessionID string            `json:"computerUseSessionId"`
	EnvironmentScope     string            `json:"environmentScope,omitempty"`
	RunID                string            `json:"runId"`
	WorkflowID           string            `json:"workflowId,omitempty"`
	WorkflowStepID       string            `json:"workflowStepId,omitempty"`
	Status               SessionStatus     `json:"status"`
	DriverKind           string            `json:"driverKind"`
	TrustedPageScope     *TrustedPageScope `json:"trustedPageScope,omitempty"`
	CurrentPage          *PageSummary      `json:"currentPage,omitempty"`
	LastActionID         string            `json:"lastActionId,omitempty"`
	StartedAt            time.Time         `json:"startedAt"`
	UpdatedAt            time.Time         `json:"updatedAt"`
	ClosedAt             *time.Time        `json:"closedAt,omitempty"`
	InterruptedAt        *time.Time        `json:"interruptedAt,omitempty"`
	Actions              []Action          `json:"actions,omitempty"`
}

type CreateSessionInput struct {
	WorkflowID     string `json:"workflowId,omitempty"`
	WorkflowStepID string `json:"workflowStepId,omitempty"`
	DriverKind     string `json:"driverKind,omitempty"`
	InitialURL     string `json:"initialUrl,omitempty"`
}

type CreateActionInput struct {
	ActionKind         ActionKind          `json:"actionKind"`
	URL                string              `json:"url,omitempty"`
	Value              string              `json:"value,omitempty"`
	SelectedValue      string              `json:"selectedValue,omitempty"`
	WaitMs             int                 `json:"waitMs,omitempty"`
	PageTarget         PageTarget          `json:"pageTarget,omitempty"`
	TargetMatchContext *TargetMatchContext `json:"targetMatchContext,omitempty"`
	Rationale          string              `json:"rationale,omitempty"`
}

type ActionRequestResult struct {
	Action   Action `json:"action"`
	Pending  bool   `json:"pending"`
	Approved bool   `json:"approved"`
}

type ArtifactCaptureRequest struct {
	RunID                string
	ComputerUseSessionID string
	ComputerUseActionID  string
	Kind                 ArtifactKind
	MIMEType             string
	FileName             string
	Content              []byte
}
