package opsreadiness

import "time"

const (
	HostedHostClassStableTestHost  = "stable_test_host"
	HostedHostClassVPS             = "vps"
	HostedHostClassDeveloperLaptop = "developer_laptop"
	HostedHostClassUnsupported     = "unsupported"

	HostedLiveConnectorsDisabled = "disabled"
	HostedLiveConnectorsLive     = "live"

	HostedSupervisorModeRepoForeground = "repo_foreground"

	HostedRunStatusProvisioning = "provisioning"
	HostedRunStatusRunning      = "running"
	HostedRunStatusStopped      = "stopped"
	HostedRunStatusFailed       = "failed"
	HostedRunStatusCompleted    = "completed"
	HostedRunStatusExpired      = "expired"

	HostedEventStart          = "start"
	HostedEventStop           = "stop"
	HostedEventRestart        = "restart"
	HostedEventStatus         = "status"
	HostedEventHealthCheck    = "health_check"
	HostedEventCrashDetected  = "crash_detected"
	HostedEventRebootRecovery = "reboot_recovery"
	HostedEventManualStop     = "manual_stop"
	HostedEventFailedRestart  = "failed_restart"
	HostedEventRepeatedCrash  = "repeated_crash"

	HostedResultPassed               = "passed"
	HostedResultFailed               = "failed"
	HostedResultBlocked              = "blocked"
	HostedResultUnsupported          = "unsupported"
	HostedResultOperatorActionNeeded = "operator_action_needed"

	HostedRedactionPassed = "passed"
	HostedRedactionFailed = "failed"

	FailureOwnerDaemon                 = "daemon"
	FailureOwnerHost                   = "host"
	FailureOwnerNetwork                = "network"
	FailureOwnerProvider               = "provider"
	FailureOwnerCredential             = "credential"
	FailureOwnerQuota                  = "quota"
	FailureOwnerOperatorAction         = "operator_action"
	FailureOwnerUnsupportedObservation = "unsupported_observation"
	FailureOwnerUnknown                = "unknown"

	HostedUpgradePhasePreflight  = "preflight"
	HostedUpgradePhasePostflight = "postflight"

	HostedRollbackInPlace                   = "in_place_rollback"
	HostedRollbackRestoreFromBackupRequired = "restore_from_backup_required"
	HostedRollbackNoRollbackNeeded          = "no_rollback_needed"
	HostedRollbackBlocked                   = "blocked"
)

var RequiredHostedEvidenceTypes = []string{
	"deployment_manifest",
	"configuration_profile",
	"health_checks",
	"logs",
	"soak_report",
	"backup_evidence",
	"restore_evidence",
	"upgrade_preflight",
	"upgrade_postflight",
	"rollback_decision",
	"integration_diagnostics",
	"resource_observations",
	"redaction_check",
	"retention_metadata",
}

type HostedOperationalProfile struct {
	ProfileID          string
	ProfileName        string
	Environment        string
	HostClass          string
	DataDirectory      string
	LogDirectory       string
	ArtifactDirectory  string
	BackupDirectory    string
	ReportDirectory    string
	TemporaryDirectory string
	LiveConnectorMode  string
	RetentionDays      int
}

type HostedRun struct {
	RunID              string
	ProfileID          string
	CommitOrVersion    string
	Host               string
	Operator           string
	StartedAt          time.Time
	CompletedAt        time.Time
	SupervisorMode     string
	Status             string
	ArtifactRoot       string
	RetentionExpiresAt time.Time
}

type HostedDeploymentManifest struct {
	ManifestID           string    `json:"manifestId"`
	RunID                string    `json:"runId"`
	ProfileID            string    `json:"profileId"`
	CommitOrVersion      string    `json:"commitOrVersion"`
	Branch               string    `json:"branch"`
	Host                 string    `json:"host"`
	Operator             string    `json:"operator"`
	StartedAt            time.Time `json:"startedAt"`
	ConfigurationProfile string    `json:"configurationProfile"`
	DataDirectory        string    `json:"dataDirectory"`
	ArtifactDirectory    string    `json:"artifactDirectory"`
	SupervisorMode       string    `json:"supervisorMode"`
	DaemonAddress        string    `json:"daemonAddress"`
	LiveConnectorMode    string    `json:"liveConnectorMode"`
	RedactionStatus      string    `json:"redactionStatus"`
	RetentionExpiresAt   time.Time `json:"retentionExpiresAt"`
}

type HostedSupervisorEvent struct {
	EventID         string    `json:"eventId"`
	RunID           string    `json:"runId"`
	EventType       string    `json:"eventType"`
	RequestedBy     string    `json:"requestedBy"`
	StartedAt       time.Time `json:"startedAt"`
	CompletedAt     time.Time `json:"completedAt"`
	DaemonHealth    string    `json:"daemonHealth"`
	RecoverySeconds int       `json:"recoverySeconds,omitempty"`
	Result          string    `json:"result"`
	FailureOwner    string    `json:"failureOwner,omitempty"`
	EvidencePath    string    `json:"evidencePath"`
}

type HostedEvidenceLink struct {
	EvidenceType       string    `json:"evidenceType"`
	Path               string    `json:"path"`
	RunID              string    `json:"runId"`
	ProfileID          string    `json:"profileId"`
	CommitOrVersion    string    `json:"commitOrVersion"`
	Status             string    `json:"status"`
	GeneratedAt        time.Time `json:"generatedAt"`
	RetentionExpiresAt time.Time `json:"retentionExpiresAt"`
	RedactionStatus    string    `json:"redactionStatus"`
	UnsupportedFields  []string  `json:"unsupportedFields,omitempty"`
	BlockingFindings   []string  `json:"blockingFindings,omitempty"`
}

type HostedReleaseEvidenceIndex struct {
	ReleaseIndexID            string
	RunID                     string
	ProfileID                 string
	CommitOrVersion           string
	GeneratedAt               time.Time
	ReviewTarget              string
	RetentionExpiresAt        time.Time
	Decision                  string
	ReviewElapsed             time.Duration
	AuthorizedRetentionPolicy string
	EvidenceLinks             []HostedEvidenceLink
}

type HostedBackupEvidence struct {
	BackupID              string
	RunID                 string
	SourceProfileID       string
	SourceCommitOrVersion string
	ArtifactPath          string
	Checksum              string
	TenantSummary         []TenantStateSummary
	IncludedMaterial      []string
	ExcludedMaterial      []string
	CompatibilityNotes    []string
	RedactionStatus       string
	GeneratedAt           time.Time
}

type HostedRestoreRehearsalResult struct {
	RestoreResultID             string
	RunID                       string
	BackupID                    string
	TargetProfileID             string
	TargetDataDirectory         string
	TargetIsAlternate           bool
	TenantCount                 int
	TenantStates                []TenantStateSummary
	TenantStateResult           string
	MigrationStateResult        string
	CredentialRemediationResult string
	QuotaStateResult            string
	DaemonHealthResult          string
	CrossTenantLeakage          bool
	RawCredentialScanResult     string
	Result                      string
	GeneratedAt                 time.Time
}

type HostedRollbackDecisionRecord struct {
	RollbackDecisionID      string
	RunID                   string
	Trigger                 string
	Decision                string
	Rationale               string
	RequiredBackupID        string
	SupportingEvidenceLinks []string
	Operator                string
	DecidedAt               time.Time
}

type HostedUpgradeEvidence struct {
	UpgradeEvidenceID          string
	RunID                      string
	Phase                      string
	DeploymentIdentity         string
	ProfileIdentity            string
	DataLocation               string
	ArtifactLocation           string
	RequiredBackupState        string
	DaemonHealth               string
	ConfigurationReadiness     string
	TenantDataVerification     string
	MigrationState             string
	CredentialRemediationState string
	QuotaState                 string
	OperationalDiagnostics     string
	RollbackGuidance           string
	FailureOwner               string
	BlockingFindings           []string
	GeneratedAt                time.Time
}

type HostedObservation struct {
	Value       string `json:"value,omitempty"`
	Unsupported bool   `json:"unsupported,omitempty"`
}

type HostedObservationReport struct {
	ObservationReportID        string
	RunID                      string
	SampleWindow               string
	DaemonHealth               string
	DatabaseSize               HostedObservation
	LogSize                    HostedObservation
	Memory                     HostedObservation
	Goroutines                 HostedObservation
	FileDescriptors            HostedObservation
	QueueOrBacklog             HostedObservation
	ConnectorHealth            HostedObservation
	MCPHealth                  HostedObservation
	IntegrationDiagnosticState HostedObservation
	UnsupportedFields          []string
	MonotonicResourceGrowth    bool
	FailureOwner               string
	BlockingFindings           []string
	GeneratedAt                time.Time
}
