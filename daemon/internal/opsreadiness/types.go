package opsreadiness

import "time"

const (
	StatusPass = "pass"
	StatusFail = "fail"
	StatusSkip = "skip"

	TopologyTenantScopedSingleNode = "tenant_scoped_single_node"
	EnvironmentTest                = "test"

	ClassificationRecovered            = "recovered"
	ClassificationInterrupted          = "interrupted"
	ClassificationRetried              = "retried"
	ClassificationRetryExhausted       = "retry_exhausted"
	ClassificationOperatorActionNeeded = "operator_action_needed"

	ResultShip                  = "ship"
	ResultNoShip                = "no_ship"
	ResultShipWithRecordedSkips = "ship_with_recorded_skips"
)

var RequiredWorkloadAreas = []string{
	"runtime",
	"scheduler",
	"integrations",
	"delivery",
	"approvals",
	"quotas",
	"tenant_switching",
	"evaluation",
}

var RequiredFaultTypes = []string{
	"transient_5xx",
	"rate_limit",
	"auth_expiry",
	"provider_unavailable",
	"slow_response",
	"malformed_response",
}

var RequiredResourceCategories = []string{
	"logs",
	"stored_data_size",
	"active_work_or_queue_backlog",
	"memory",
}

type RunbookEvidence struct {
	Name               string
	Steps              []string
	Elapsed            time.Duration
	MaxElapsed         time.Duration
	HealthChecks       []string
	Diagnostics        []string
	FailureModes       []string
	RollbackOrCleanup  []string
	OutOfScope         []string
	TestEnvironment    bool
	ProductionOptIn    bool
	UsedProductionData bool
}

type MigrationVerificationReport struct {
	SourceVersion            string
	TargetVersion            string
	PreflightChecks          []string
	PostflightChecks         []string
	MigrationProgress        string
	TenantIntegritySummary   string
	QuotaAccountingSummary   string
	CredentialRemediation    string
	RollbackPath             string
	Result                   string
	OperatorDiagnostics      []string
	PersistedStateReversible bool
}

type RollbackDecision struct {
	InPlaceSafe              bool
	PersistedStateReversible bool
	BackupVerified           bool
	SelectedPath             string
	Reason                   string
}

type TenantStateSummary struct {
	TenantID             string   `json:"tenantId"`
	CredentialRefs       []string `json:"credentialRefs"`
	QuotaState           string   `json:"quotaState"`
	WorkState            string   `json:"workState"`
	ReconnectRequired    bool     `json:"reconnectRequired"`
	OperatorActionNeeded bool     `json:"operatorActionNeeded"`
}

type BackupArtifact struct {
	ArtifactID         string               `json:"artifactId"`
	CreatedAt          time.Time            `json:"createdAt"`
	SourceVersion      string               `json:"sourceVersion"`
	SourceEnvironment  string               `json:"sourceEnvironment"`
	IncludedMaterial   []string             `json:"includedMaterial"`
	ExcludedMaterial   []string             `json:"excludedMaterial"`
	TenantCount        int                  `json:"tenantCount"`
	TenantStateSummary []TenantStateSummary `json:"tenantStateSummary"`
	IntegrityChecks    []string             `json:"integrityChecks"`
	CompatibilityNotes []string             `json:"compatibilityNotes"`
}

type RestoreVerificationResult struct {
	BackupArtifactID             string
	RestoreEnvironment           string
	TenantRecordChecksPassed     int
	TenantRecordChecksTotal      int
	CrossTenantLeakageObserved   bool
	RawCredentialMaterialFound   bool
	SecretReferenceChecks        []string
	QuotaStateChecks             []string
	WorkStateChecks              []string
	CredentialRemediationStates  []string
	InvalidBackupFailedClearly   bool
	PartialRestoreReportedPassed bool
	Result                       string
}

type WorkloadCoverage map[string]bool

type RestartEvent struct {
	RestartID      string
	UnfinishedWork string
	Classification string
	RecoveryTime   time.Duration
}

type FaultDrillResult struct {
	FaultType                     string
	Domain                        string
	ObservedClassification        string
	RetryExhausted                bool
	OperatorActionNeeded          bool
	ContainsRawCredentialMaterial bool
}

type ResourceObservation struct {
	Category           string
	Available          bool
	MonotonicGrowth    bool
	QueueBacklogAge    time.Duration
	OperatorVisibility string
}

type SoakReport struct {
	ReportID                 string
	BranchOrVersion          string
	Environment              string
	DataDirectory            string
	BaselineTopology         string
	StartedAt                time.Time
	CompletedAt              time.Time
	Duration                 time.Duration
	TemporaryShorterDuration bool
	TemporaryDurationReason  string
	FollowUpFullRerun        bool
	TenantSetSummary         []TenantStateSummary
	WorkloadCoverage         WorkloadCoverage
	RestartEvents            []RestartEvent
	FaultDrillResults        []FaultDrillResult
	ResourceObservations     []ResourceObservation
	CrossTenantLeakage       bool
	UnclassifiedFailures     []string
	RetryExhaustionSummary   []FaultDrillResult
	FinalResult              string
}

type RealAccountSmokeStatus struct {
	Domain                        string
	SafeCredentialsAvailable      bool
	Enabled                       bool
	Result                        string
	SkipReason                    string
	FakeBackendCoveragePassing    bool
	ContainsRawCredentialMaterial bool
}

type ReleaseReadinessEvidence struct {
	InstallRunbookPassed          bool
	UpgradeRunbookPassed          bool
	BackupArtifactPassed          bool
	RestoreVerificationPassed     bool
	MigrationVerificationPassed   bool
	RollbackGuidancePresent       bool
	SoakReportPassed              bool
	ResourceGrowthChecksPassed    bool
	CredentialRedactionPassed     bool
	FakeBackendCoveragePassed     bool
	Roadmap40RerunGatePresent     bool
	Roadmap41RerunGatePresent     bool
	Roadmap42DiagnosticsPresent   bool
	Roadmap42SmokeEvidencePresent bool
	ReviewElapsed                 time.Duration
	RealAccountSmoke              []RealAccountSmokeStatus
	DiagnosticSmokeReports        []SmokeMatrixReport
	Decision                      string
}
