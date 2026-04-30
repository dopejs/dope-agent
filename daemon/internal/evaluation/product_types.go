package evaluation

import "time"

type ProductResourceKind string

const (
	ProductResourceDiscoveryPolicy      ProductResourceKind = "discovery_policy"
	ProductResourceDiscoveryRun         ProductResourceKind = "discovery_run"
	ProductResourceDiscoveredCandidate  ProductResourceKind = "discovered_candidate"
	ProductResourceCandidateEvidence    ProductResourceKind = "candidate_evidence"
	ProductResourceSuppression          ProductResourceKind = "suppression"
	ProductResourceProductFixture       ProductResourceKind = "product_fixture"
	ProductResourceFixtureRevision      ProductResourceKind = "fixture_revision"
	ProductResourceCampaign             ProductResourceKind = "campaign"
	ProductResourceCampaignItem         ProductResourceKind = "campaign_item"
	ProductResourceAttemptGroup         ProductResourceKind = "campaign_attempt_group"
	ProductResourceDashboardProjection  ProductResourceKind = "dashboard_projection"
	ProductResourceToolCallInspection   ProductResourceKind = "tool_call_inspection"
	ProductResourceRetentionApplication ProductResourceKind = "retention_application"
)

type ProductLifecycleStatus string

const (
	ProductStatusQueued     ProductLifecycleStatus = "queued"
	ProductStatusRunning    ProductLifecycleStatus = "running"
	ProductStatusCompleted  ProductLifecycleStatus = "completed"
	ProductStatusPartial    ProductLifecycleStatus = "partial"
	ProductStatusFailed     ProductLifecycleStatus = "failed"
	ProductStatusCancelled  ProductLifecycleStatus = "cancelled"
	ProductStatusDraft      ProductLifecycleStatus = "draft"
	ProductStatusInReview   ProductLifecycleStatus = "in_review"
	ProductStatusApproved   ProductLifecycleStatus = "approved"
	ProductStatusRejected   ProductLifecycleStatus = "rejected"
	ProductStatusPublished  ProductLifecycleStatus = "published"
	ProductStatusArchived   ProductLifecycleStatus = "archived"
	ProductStatusDeleted    ProductLifecycleStatus = "deleted"
	ProductStatusExpired    ProductLifecycleStatus = "expired"
	ProductStatusSuppressed ProductLifecycleStatus = "suppressed"
)

type RetentionState string

const (
	RetentionStateActive    RetentionState = "active"
	RetentionStateExpired   RetentionState = "expired"
	RetentionStateDeleted   RetentionState = "deleted"
	RetentionStateTombstone RetentionState = "tombstone"
)

type SuppressionState string

const (
	SuppressionStateNone       SuppressionState = "none"
	SuppressionStateSuppressed SuppressionState = "suppressed"
	SuppressionStateExpired    SuppressionState = "expired"
	SuppressionStateRevoked    SuppressionState = "revoked"
)

type RedactionStatus string

const (
	RedactionStatusClean    RedactionStatus = "clean"
	RedactionStatusRedacted RedactionStatus = "redacted"
	RedactionStatusFailed   RedactionStatus = "failed"
)

type ScoreBand string

const (
	ScoreBandHigh   ScoreBand = "high"
	ScoreBandMedium ScoreBand = "medium"
	ScoreBandLow    ScoreBand = "low"
)

type DiscoveryPolicy struct {
	PolicyID             string       `json:"policyId"`
	TenantID             string       `json:"tenantId"`
	Enabled              bool         `json:"enabled"`
	SourceKinds          []SourceKind `json:"sourceKinds"`
	WindowStart          time.Time    `json:"windowStart"`
	WindowEnd            time.Time    `json:"windowEnd"`
	MaxInspectedRecords  int          `json:"maxInspectedRecords"`
	MaxEmittedCandidates int          `json:"maxEmittedCandidates"`
	CostBudget           int          `json:"costBudget"`
	SensitiveFieldRules  []string     `json:"sensitiveFieldRules,omitempty"`
	RetentionPolicyRef   string       `json:"retentionPolicyRef,omitempty"`
	CreatedBy            string       `json:"createdBy,omitempty"`
	CreatedAt            time.Time    `json:"createdAt"`
	UpdatedAt            time.Time    `json:"updatedAt"`
}

type DiscoveryRun struct {
	DiscoveryRunID       string                 `json:"discoveryRunId"`
	TenantID             string                 `json:"tenantId"`
	PolicyID             string                 `json:"policyId,omitempty"`
	Status               ProductLifecycleStatus `json:"status"`
	Cursor               string                 `json:"cursor,omitempty"`
	SourceKinds          []SourceKind           `json:"sourceKinds"`
	WindowStart          time.Time              `json:"windowStart"`
	WindowEnd            time.Time              `json:"windowEnd"`
	MaxInspectedRecords  int                    `json:"maxInspectedRecords"`
	MaxEmittedCandidates int                    `json:"maxEmittedCandidates"`
	CostBudget           int                    `json:"costBudget"`
	InspectedRecords     int                    `json:"inspectedRecords"`
	EmittedCandidates    int                    `json:"emittedCandidates"`
	PartialReason        string                 `json:"partialReason,omitempty"`
	StartedBy            string                 `json:"startedBy,omitempty"`
	StartedAt            time.Time              `json:"startedAt"`
	CompletedAt          *time.Time             `json:"completedAt,omitempty"`
	UpdatedAt            time.Time              `json:"updatedAt"`
	IdempotencyKey       string                 `json:"idempotencyKey,omitempty"`
}

type DiscoveredCandidate struct {
	DiscoveredCandidateID string           `json:"discoveredCandidateId"`
	TenantID              string           `json:"tenantId"`
	DiscoveryRunID        string           `json:"discoveryRunId"`
	SourceKind            SourceKind       `json:"sourceKind"`
	SourceID              string           `json:"sourceId"`
	SourceRefs            []SourceRef      `json:"sourceRefs,omitempty"`
	Score                 float64          `json:"score"`
	ScoreBand             ScoreBand        `json:"scoreBand"`
	ExplanationFields     map[string]any   `json:"explanationFields,omitempty"`
	RedactionStatus       RedactionStatus  `json:"redactionStatus"`
	EvidenceRef           string           `json:"evidenceRef,omitempty"`
	ReadinessStatus       ReadinessStatus  `json:"readinessStatus"`
	SuppressionState      SuppressionState `json:"suppressionState"`
	RetentionState        RetentionState   `json:"retentionState"`
	CreatedAt             time.Time        `json:"createdAt"`
	UpdatedAt             time.Time        `json:"updatedAt"`
	ExpiresAt             *time.Time       `json:"expiresAt,omitempty"`
}

type CandidateEvidence struct {
	EvidenceID              string         `json:"evidenceId"`
	TenantID                string         `json:"tenantId"`
	DiscoveredCandidateID   string         `json:"discoveredCandidateId"`
	SourceRefs              []SourceRef    `json:"sourceRefs,omitempty"`
	Summary                 string         `json:"summary,omitempty"`
	RedactedPayload         map[string]any `json:"redactedPayload,omitempty"`
	RedactionRulesApplied   []string       `json:"redactionRulesApplied,omitempty"`
	SensitiveFieldsExcluded []string       `json:"sensitiveFieldsExcluded,omitempty"`
	MaterializationAllowed  bool           `json:"materializationAllowed"`
	RetentionState          RetentionState `json:"retentionState"`
	CreatedAt               time.Time      `json:"createdAt"`
	ExpiresAt               *time.Time     `json:"expiresAt,omitempty"`
}

type SuppressionRecord struct {
	SuppressionID   string              `json:"suppressionId"`
	TenantID        string              `json:"tenantId"`
	TargetKind      ProductResourceKind `json:"targetKind"`
	TargetID        string              `json:"targetId,omitempty"`
	TargetSourceRef string              `json:"targetSourceRef,omitempty"`
	ReasonCode      string              `json:"reasonCode"`
	Reason          string              `json:"reason,omitempty"`
	CreatedBy       string              `json:"createdBy,omitempty"`
	CreatedAt       time.Time           `json:"createdAt"`
	ExpiresAt       *time.Time          `json:"expiresAt,omitempty"`
	Active          bool                `json:"active"`
}

type ProductManagedFixture struct {
	FixtureID         string                 `json:"fixtureId"`
	TenantID          string                 `json:"tenantId"`
	DisplayName       string                 `json:"displayName"`
	DomainClass       FixtureDomainClass     `json:"domainClass"`
	SourceKind        string                 `json:"sourceKind"`
	SourceRefs        []SourceRef            `json:"sourceRefs,omitempty"`
	SourceCandidateID string                 `json:"sourceCandidateId,omitempty"`
	CurrentRevisionID string                 `json:"currentRevisionId,omitempty"`
	ReviewState       ProductLifecycleStatus `json:"reviewState"`
	SuppressionState  SuppressionState       `json:"suppressionState"`
	RetentionState    RetentionState         `json:"retentionState"`
	CreatedBy         string                 `json:"createdBy,omitempty"`
	CreatedAt         time.Time              `json:"createdAt"`
	UpdatedAt         time.Time              `json:"updatedAt"`
}

type FixtureRevision struct {
	RevisionID         string          `json:"revisionId"`
	FixtureID          string          `json:"fixtureId"`
	TenantID           string          `json:"tenantId"`
	RevisionNumber     int             `json:"revisionNumber"`
	ContentSummary     string          `json:"contentSummary,omitempty"`
	FixturePayload     map[string]any  `json:"fixturePayload,omitempty"`
	ChangeSummary      string          `json:"changeSummary,omitempty"`
	SourceEvidenceRefs []string        `json:"sourceEvidenceRefs,omitempty"`
	RedactionStatus    RedactionStatus `json:"redactionStatus"`
	CreatedBy          string          `json:"createdBy,omitempty"`
	CreatedAt          time.Time       `json:"createdAt"`
}

type ReplayCampaign struct {
	CampaignID     string                 `json:"campaignId"`
	TenantID       string                 `json:"tenantId"`
	DisplayName    string                 `json:"displayName"`
	Status         ProductLifecycleStatus `json:"status"`
	ScopeSummary   string                 `json:"scopeSummary,omitempty"`
	StartedBy      string                 `json:"startedBy,omitempty"`
	CreatedAt      time.Time              `json:"createdAt"`
	StartedAt      *time.Time             `json:"startedAt,omitempty"`
	CompletedAt    *time.Time             `json:"completedAt,omitempty"`
	PublishedAt    *time.Time             `json:"publishedAt,omitempty"`
	RetentionState RetentionState         `json:"retentionState"`
	IdempotencyKey string                 `json:"idempotencyKey,omitempty"`
}

type CampaignItem struct {
	CampaignItemID       string              `json:"campaignItemId"`
	CampaignID           string              `json:"campaignId"`
	TenantID             string              `json:"tenantId"`
	SourceType           ProductResourceKind `json:"sourceType"`
	SourceID             string              `json:"sourceId"`
	SourceSnapshot       map[string]any      `json:"sourceSnapshot,omitempty"`
	SelectionReason      string              `json:"selectionReason,omitempty"`
	SuppressionCheckedAt time.Time           `json:"suppressionCheckedAt"`
	CreatedAt            time.Time           `json:"createdAt"`
}

type CampaignAttemptGroup struct {
	AttemptGroupID            string                 `json:"attemptGroupId"`
	CampaignID                string                 `json:"campaignId"`
	CampaignItemID            string                 `json:"campaignItemId"`
	TenantID                  string                 `json:"tenantId"`
	ReplayAttemptIDs          []string               `json:"replayAttemptIds,omitempty"`
	ComparisonIDs             []string               `json:"comparisonIds,omitempty"`
	LiveValidationIDs         []string               `json:"liveValidationIds,omitempty"`
	Status                    ProductLifecycleStatus `json:"status"`
	DriftCount                int                    `json:"driftCount"`
	FailureCount              int                    `json:"failureCount"`
	UnsupportedCount          int                    `json:"unsupportedCount"`
	OperatorActionNeededCount int                    `json:"operatorActionNeededCount"`
	Summary                   string                 `json:"summary,omitempty"`
	CreatedAt                 time.Time              `json:"createdAt"`
	UpdatedAt                 time.Time              `json:"updatedAt"`
}

type DashboardProjection struct {
	ProjectionID                string         `json:"projectionId"`
	TenantID                    string         `json:"tenantId"`
	WindowStart                 time.Time      `json:"windowStart"`
	WindowEnd                   time.Time      `json:"windowEnd"`
	CampaignStatusCounts        map[string]int `json:"campaignStatusCounts,omitempty"`
	DriftSummary                map[string]int `json:"driftSummary,omitempty"`
	FailureSummary              map[string]int `json:"failureSummary,omitempty"`
	UnsupportedSummary          map[string]int `json:"unsupportedSummary,omitempty"`
	OperatorActionNeededSummary map[string]int `json:"operatorActionNeededSummary,omitempty"`
	LiveValidationSummary       map[string]int `json:"liveValidationSummary,omitempty"`
	CandidateSummary            map[string]int `json:"candidateSummary,omitempty"`
	FixtureSummary              map[string]int `json:"fixtureSummary,omitempty"`
	GeneratedAt                 time.Time      `json:"generatedAt"`
	Cursor                      string         `json:"cursor,omitempty"`
	RetentionState              RetentionState `json:"retentionState,omitempty"`
}

type ToolCallInspection struct {
	InspectionID             string          `json:"inspectionId"`
	TenantID                 string          `json:"tenantId"`
	CampaignID               string          `json:"campaignId"`
	CampaignItemID           string          `json:"campaignItemId"`
	ToolCallRef              string          `json:"toolCallRef"`
	OriginalEvidenceRef      string          `json:"originalEvidenceRef,omitempty"`
	NonLiveReplayEvidenceRef string          `json:"nonLiveReplayEvidenceRef,omitempty"`
	LiveValidationLedgerRefs []string        `json:"liveValidationLedgerRefs,omitempty"`
	Classification           string          `json:"classification"`
	DiffSummary              string          `json:"diffSummary,omitempty"`
	RedactionStatus          RedactionStatus `json:"redactionStatus"`
	RetentionState           RetentionState  `json:"retentionState,omitempty"`
	CreatedAt                time.Time       `json:"createdAt"`
	UpdatedAt                time.Time       `json:"updatedAt"`
}

type ProductPage struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit"`
}
