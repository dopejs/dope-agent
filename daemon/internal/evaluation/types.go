package evaluation

import "time"

type CandidateKind string

const (
	CandidateKindCuratedWork CandidateKind = "curated_work"
	CandidateKindFixture     CandidateKind = "fixture"
)

type SourceKind string

const (
	SourceKindRun         SourceKind = "run"
	SourceKindWorkflow    SourceKind = "workflow"
	SourceKindSchedule    SourceKind = "schedule"
	SourceKindIntegration SourceKind = "integration"
	SourceKindComputerUse SourceKind = "computer_use"
	SourceKindFixture     SourceKind = "fixture"
)

type ReadinessStatus string

const (
	ReadinessFullyReplayable     ReadinessStatus = "fully_replayable"
	ReadinessPartiallyReplayable ReadinessStatus = "partially_replayable"
	ReadinessBlocked             ReadinessStatus = "blocked"
	ReadinessUnreplayable        ReadinessStatus = "unreplayable"
)

type ReplayMode string

const (
	ReplayModeNonLive        ReplayMode = "non_live"
	ReplayModeLiveValidation ReplayMode = "live_validation"
)

type ReplayAttemptStatus string

const (
	ReplayAttemptStatusQueued       ReplayAttemptStatus = "queued"
	ReplayAttemptStatusRunning      ReplayAttemptStatus = "running"
	ReplayAttemptStatusCompleted    ReplayAttemptStatus = "completed"
	ReplayAttemptStatusBlocked      ReplayAttemptStatus = "blocked"
	ReplayAttemptStatusUnreplayable ReplayAttemptStatus = "unreplayable"
	ReplayAttemptStatusFailed       ReplayAttemptStatus = "failed"
	ReplayAttemptStatusCancelled    ReplayAttemptStatus = "cancelled"
)

type ApprovalHandling string

const (
	ApprovalBlocked               ApprovalHandling = "blocked"
	ApprovalEvidenceOnly          ApprovalHandling = "evidence_only"
	ApprovalFreshApprovalRequired ApprovalHandling = "fresh_approval_required"
)

type SideEffectHandling string

const (
	SideEffectBlocked      SideEffectHandling = "blocked"
	SideEffectEvidenceOnly SideEffectHandling = "evidence_only"
	SideEffectLive         SideEffectHandling = "live"
)

type ComparisonTerminalStatus string

const (
	ComparisonMatched      ComparisonTerminalStatus = "matched"
	ComparisonDrifted      ComparisonTerminalStatus = "drifted"
	ComparisonBlocked      ComparisonTerminalStatus = "blocked"
	ComparisonUnreplayable ComparisonTerminalStatus = "unreplayable"
)

type DriftPlane string

const (
	DriftPlaneRuntime     DriftPlane = "runtime"
	DriftPlanePolicy      DriftPlane = "policy"
	DriftPlaneIntegration DriftPlane = "integration"
	DriftPlaneDelivery    DriftPlane = "delivery"
	DriftPlaneEvidence    DriftPlane = "evidence"
	DriftPlaneUnknown     DriftPlane = "unknown"
	DriftPlaneMixed       DriftPlane = "mixed"
)

type FixtureDomainClass string

const (
	FixtureDomainSchedule    FixtureDomainClass = "schedule"
	FixtureDomainIntegration FixtureDomainClass = "integration"
	FixtureDomainComputerUse FixtureDomainClass = "computer_use"
)

type SourceRef struct {
	Kind  SourceKind `json:"kind"`
	ID    string     `json:"id"`
	Route string     `json:"route,omitempty"`
}

type SafetyScope struct {
	Mode        ReplayMode `json:"mode"`
	Description string     `json:"description,omitempty"`
}

type PlaneSummaries struct {
	Runtime     string `json:"runtime,omitempty"`
	Policy      string `json:"policy,omitempty"`
	Integration string `json:"integration,omitempty"`
	Delivery    string `json:"delivery,omitempty"`
	Evidence    string `json:"evidence,omitempty"`
}

type ReplayCandidate struct {
	CandidateID          string          `json:"candidateId"`
	CandidateKind        CandidateKind   `json:"candidateKind"`
	DisplayName          string          `json:"displayName"`
	Description          string          `json:"description,omitempty"`
	SourceKind           SourceKind      `json:"sourceKind"`
	SourceID             string          `json:"sourceId"`
	SourceRefs           []SourceRef     `json:"sourceRefs"`
	EnvironmentScope     string          `json:"environmentScope"`
	ReadinessStatus      ReadinessStatus `json:"readinessStatus"`
	ReadinessReasons     []string        `json:"readinessReasons"`
	Limitations          []string        `json:"limitations"`
	DefaultReplayMode    ReplayMode      `json:"defaultReplayMode"`
	FixtureID            string          `json:"fixtureId,omitempty"`
	LatestAttemptID      string          `json:"latestAttemptId,omitempty"`
	LatestComparisonID   string          `json:"latestComparisonId,omitempty"`
	ExpectedComparison   PlaneSummaries  `json:"expectedComparisonSummary,omitempty"`
	CapturedEvidenceRefs []SourceRef     `json:"capturedEvidenceRefs,omitempty"`
	CreatedAt            time.Time       `json:"createdAt"`
	UpdatedAt            time.Time       `json:"updatedAt"`
}

type ReplayAttempt struct {
	AttemptID          string              `json:"attemptId"`
	CandidateID        string              `json:"candidateId"`
	SourceRefs         []SourceRef         `json:"sourceRefs"`
	EnvironmentScope   string              `json:"environmentScope"`
	Mode               ReplayMode          `json:"mode"`
	Status             ReplayAttemptStatus `json:"status"`
	SafetyScope        SafetyScope         `json:"safetyScope"`
	ApprovalHandling   ApprovalHandling    `json:"approvalHandling"`
	SideEffectHandling SideEffectHandling  `json:"sideEffectHandling"`
	LaunchedBy         string              `json:"launchedBy,omitempty"`
	ChangeWindowLabel  string              `json:"changeWindowLabel,omitempty"`
	BaselineAttemptID  string              `json:"baselineAttemptId,omitempty"`
	ResultRunID        string              `json:"resultRunId,omitempty"`
	ResultWorkflowID   string              `json:"resultWorkflowId,omitempty"`
	EvidenceRefs       []SourceRef         `json:"evidenceRefs"`
	BlockedReasons     []string            `json:"blockedReasons"`
	RuntimeSummary     string              `json:"runtimeSummary,omitempty"`
	PolicySummary      string              `json:"policySummary,omitempty"`
	IntegrationSummary string              `json:"integrationSummary,omitempty"`
	DeliverySummary    string              `json:"deliverySummary,omitempty"`
	EvidenceSummary    string              `json:"evidenceSummary,omitempty"`
	StartedAt          time.Time           `json:"startedAt,omitempty"`
	CompletedAt        time.Time           `json:"completedAt,omitempty"`
	CreatedAt          time.Time           `json:"createdAt"`
	UpdatedAt          time.Time           `json:"updatedAt"`
}

type ComparisonResult struct {
	ComparisonID       string                   `json:"comparisonId"`
	CandidateID        string                   `json:"candidateId"`
	BaselineRef        string                   `json:"baselineRef"`
	AttemptID          string                   `json:"attemptId"`
	EnvironmentScope   string                   `json:"environmentScope"`
	TerminalStatus     ComparisonTerminalStatus `json:"terminalStatus"`
	RuntimeSummary     string                   `json:"runtimeSummary"`
	PolicySummary      string                   `json:"policySummary"`
	IntegrationSummary string                   `json:"integrationSummary"`
	DeliverySummary    string                   `json:"deliverySummary"`
	EvidenceSummary    string                   `json:"evidenceSummary"`
	Confidence         string                   `json:"confidence"`
	Limitations        []string                 `json:"limitations"`
	DriftFindings      []DriftFinding           `json:"driftFindings"`
	ChangeWindowLabel  string                   `json:"changeWindowLabel,omitempty"`
	GeneratedAt        time.Time                `json:"generatedAt"`
}

type DriftFinding struct {
	FindingID         string      `json:"findingId"`
	ComparisonID      string      `json:"comparisonId"`
	Plane             DriftPlane  `json:"plane"`
	Severity          string      `json:"severity"`
	Summary           string      `json:"summary"`
	BaselineValue     string      `json:"baselineValue,omitempty"`
	ReplayValue       string      `json:"replayValue,omitempty"`
	EvidenceRefs      []SourceRef `json:"evidenceRefs,omitempty"`
	RecommendedAction string      `json:"recommendedAction,omitempty"`
	CreatedAt         time.Time   `json:"createdAt"`
}

type RegressionFixture struct {
	FixtureID                 string             `json:"fixtureId"`
	DisplayName               string             `json:"displayName"`
	DomainClass               FixtureDomainClass `json:"domainClass"`
	ManifestPath              string             `json:"manifestPath"`
	SourceRefs                []SourceRef        `json:"sourceRefs"`
	CapturedEvidenceRefs      []SourceRef        `json:"capturedEvidenceRefs"`
	Assumptions               []string           `json:"assumptions"`
	Limitations               []string           `json:"limitations"`
	ExpectedReplayMode        ReplayMode         `json:"expectedReplayMode"`
	ExpectedComparisonSummary PlaneSummaries     `json:"expectedComparisonSummary"`
	CandidateID               string             `json:"candidateId,omitempty"`
	EnvironmentScope          string             `json:"environmentScope"`
	CreatedAt                 time.Time          `json:"createdAt"`
	UpdatedAt                 time.Time          `json:"updatedAt"`
}

type CandidateFilter struct {
	EnvironmentScope string
	CandidateKind    CandidateKind
	SourceKind       SourceKind
	ReadinessStatus  ReadinessStatus
	Limit            int
}

type AttemptFilter struct {
	EnvironmentScope string
	CandidateID      string
	Status           ReplayAttemptStatus
	Limit            int
}

type ComparisonFilter struct {
	EnvironmentScope string
	CandidateID      string
	AttemptID        string
	TerminalStatus   ComparisonTerminalStatus
	Limit            int
}

type FixtureFilter struct {
	EnvironmentScope string
	DomainClass      FixtureDomainClass
	Limit            int
}

type CreateReplayAttemptInput struct {
	Mode              ReplayMode  `json:"mode,omitempty"`
	ChangeWindowLabel string      `json:"changeWindowLabel,omitempty"`
	BaselineAttemptID string      `json:"baselineAttemptId,omitempty"`
	SafetyScope       SafetyScope `json:"safetyScope,omitempty"`
	LaunchedBy        string      `json:"launchedBy,omitempty"`
}

type CreateComparisonInput struct {
	BaselineAttemptID string `json:"baselineAttemptId,omitempty"`
	BaselineRef       string `json:"baselineRef,omitempty"`
	ChangeWindowLabel string `json:"changeWindowLabel,omitempty"`
}
