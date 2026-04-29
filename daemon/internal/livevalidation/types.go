package livevalidation

import "time"

type AttemptStatus string

const (
	AttemptStatusQueued               AttemptStatus = "queued"
	AttemptStatusAwaitingApproval     AttemptStatus = "awaiting_approval"
	AttemptStatusRunning              AttemptStatus = "running"
	AttemptStatusCompleted            AttemptStatus = "completed"
	AttemptStatusBlocked              AttemptStatus = "blocked"
	AttemptStatusAborted              AttemptStatus = "aborted"
	AttemptStatusFailed               AttemptStatus = "failed"
	AttemptStatusOperatorActionNeeded AttemptStatus = "operator_action_needed"
)

type ApprovalMode string

const (
	ApprovalModeScopeLevel ApprovalMode = "scope_level"
	ApprovalModePerAction  ApprovalMode = "per_action"
	ApprovalModeMixed      ApprovalMode = "mixed"
)

type ApprovalTarget string

const (
	ApprovalTargetScope  ApprovalTarget = "scope"
	ApprovalTargetAction ApprovalTarget = "action"
)

type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"
	ApprovalStatusApproved ApprovalStatus = "approved"
	ApprovalStatusDenied   ApprovalStatus = "denied"
	ApprovalStatusExpired  ApprovalStatus = "expired"
)

type KillSwitchScope string

const (
	KillSwitchScopeTenant KillSwitchScope = "tenant"
	KillSwitchScopeGlobal KillSwitchScope = "global"
)

type GateDecision struct {
	Allowed    bool      `json:"allowed"`
	ReasonCode string    `json:"reasonCode,omitempty"`
	Reference  string    `json:"reference,omitempty"`
	CheckedAt  time.Time `json:"checkedAt"`
}

type LedgerSummary map[LedgerOutcome]int

type Attempt struct {
	ValidationID       string          `json:"validationId"`
	TenantID           string          `json:"tenantId"`
	CandidateID        string          `json:"candidateId"`
	SourceAttemptID    string          `json:"sourceAttemptId,omitempty"`
	RequestedBy        string          `json:"requestedBy"`
	EnvironmentScope   string          `json:"environmentScope"`
	RequestedScope     SideEffectScope `json:"requestedScope"`
	Status             AttemptStatus   `json:"status"`
	PermissionDecision GateDecision    `json:"permissionDecision"`
	QuotaDecision      GateDecision    `json:"quotaDecision"`
	KillSwitchDecision GateDecision    `json:"killSwitchDecision"`
	ApprovalSummary    ApprovalSummary `json:"approvalSummary"`
	LedgerSummary      LedgerSummary   `json:"ledgerSummary"`
	ComparisonID       string          `json:"comparisonId,omitempty"`
	CreatedAt          time.Time       `json:"createdAt"`
	StartedAt          *time.Time      `json:"startedAt,omitempty"`
	CompletedAt        *time.Time      `json:"completedAt,omitempty"`
	UpdatedAt          time.Time       `json:"updatedAt"`
}

type SideEffectScope struct {
	ScopeID             string       `json:"scopeId"`
	ValidationID        string       `json:"validationId"`
	IncludedToolClasses []ToolClass  `json:"includedToolClasses,omitempty"`
	ExcludedToolClasses []ToolClass  `json:"excludedToolClasses,omitempty"`
	IncludedActions     []string     `json:"includedActions,omitempty"`
	ExcludedActions     []string     `json:"excludedActions,omitempty"`
	ApprovalMode        ApprovalMode `json:"approvalMode"`
	DeclaredBy          string       `json:"declaredBy"`
	DeclaredAt          time.Time    `json:"declaredAt"`
}

type ApprovalSummary struct {
	Required int `json:"required"`
	Approved int `json:"approved"`
	Denied   int `json:"denied"`
	Expired  int `json:"expired"`
	Pending  int `json:"pending"`
}

type FreshApproval struct {
	ApprovalID    string         `json:"approvalId"`
	ValidationID  string         `json:"validationId"`
	TenantID      string         `json:"tenantId"`
	Target        ApprovalTarget `json:"approvalTarget"`
	ToolClass     ToolClass      `json:"toolClass"`
	SafetyClass   SafetyClass    `json:"safetyClass"`
	ActionRef     string         `json:"actionRef,omitempty"`
	ApprovedScope string         `json:"approvedScope,omitempty"`
	Status        ApprovalStatus `json:"status"`
	RequestedBy   string         `json:"requestedBy"`
	ResolvedBy    string         `json:"resolvedBy,omitempty"`
	RequestedAt   time.Time      `json:"requestedAt"`
	ResolvedAt    *time.Time     `json:"resolvedAt,omitempty"`
}

type SideEffectLedgerEntry struct {
	LedgerEntryID    string        `json:"ledgerEntryId"`
	ValidationID     string        `json:"validationId"`
	TenantID         string        `json:"tenantId"`
	CandidateID      string        `json:"candidateId"`
	SourceRef        string        `json:"sourceRef"`
	ToolClass        ToolClass     `json:"toolClass"`
	SafetyClass      SafetyClass   `json:"safetyClass"`
	ActionRef        string        `json:"actionRef"`
	ApprovalID       string        `json:"approvalId,omitempty"`
	CorrelationKey   string        `json:"correlationKey,omitempty"`
	DownstreamRef    string        `json:"downstreamRef,omitempty"`
	Outcome          LedgerOutcome `json:"outcome"`
	ReasonCode       string        `json:"reasonCode,omitempty"`
	AttemptedAt      *time.Time    `json:"attemptedAt,omitempty"`
	CompletedAt      *time.Time    `json:"completedAt,omitempty"`
	UpdatedAt        time.Time     `json:"updatedAt"`
	EvidenceRefs     []string      `json:"evidenceRefs,omitempty"`
	RetryCount       int           `json:"retryCount"`
	AmbiguousCommit  bool          `json:"ambiguousCommit"`
	ReconciliationID string        `json:"reconciliationId,omitempty"`
}

type KillSwitch struct {
	KillSwitchID string          `json:"killSwitchId"`
	Scope        KillSwitchScope `json:"scope"`
	TenantID     string          `json:"tenantId,omitempty"`
	Enabled      bool            `json:"enabled"`
	Reason       string          `json:"reason"`
	ChangedBy    string          `json:"changedBy"`
	ChangedAt    time.Time       `json:"changedAt"`
	ExpiresAt    *time.Time      `json:"expiresAt,omitempty"`
}

type AmbiguousCommitCause string

const (
	AmbiguousCauseTimeout             AmbiguousCommitCause = "timeout"
	AmbiguousCauseConnectionLoss      AmbiguousCommitCause = "connection_loss"
	AmbiguousCauseUnknownResponse     AmbiguousCommitCause = "unknown_provider_response"
	AmbiguousCauseDaemonRestart       AmbiguousCommitCause = "daemon_restart"
	AmbiguousCauseConflictingEvidence AmbiguousCommitCause = "conflicting_evidence"
	AmbiguousCauseOther               AmbiguousCommitCause = "other"
)

type AmbiguousCommit struct {
	AmbiguousCommitID     string               `json:"ambiguousCommitId"`
	LedgerEntryID         string               `json:"ledgerEntryId"`
	ValidationID          string               `json:"validationId"`
	TenantID              string               `json:"tenantId"`
	Cause                 AmbiguousCommitCause `json:"cause"`
	LastKnownRequestRef   string               `json:"lastKnownRequestRef,omitempty"`
	AutomaticRetryStopped bool                 `json:"automaticRetryStopped"`
	CreatedAt             time.Time            `json:"createdAt"`
	UpdatedAt             time.Time            `json:"updatedAt"`
}

type ReconciliationResolutionValue string

const (
	ResolutionConfirmedCommitted    ReconciliationResolutionValue = "confirmed_committed"
	ResolutionConfirmedNotCommitted ReconciliationResolutionValue = "confirmed_not_committed"
	ResolutionCompensated           ReconciliationResolutionValue = "compensated"
	ResolutionAcceptedManualState   ReconciliationResolutionValue = "accepted_manual_state"
	ResolutionUnsupportedUnresolved ReconciliationResolutionValue = "unsupported_unresolved"
)

type ReconciliationResolution struct {
	ReconciliationID  string                        `json:"reconciliationId"`
	AmbiguousCommitID string                        `json:"ambiguousCommitId"`
	TenantID          string                        `json:"tenantId"`
	ResolvedBy        string                        `json:"resolvedBy"`
	Resolution        ReconciliationResolutionValue `json:"resolution"`
	Reason            string                        `json:"reason"`
	EvidenceRefs      []string                      `json:"evidenceRefs,omitempty"`
	ResolvedAt        time.Time                     `json:"resolvedAt"`
}

type ComparisonStatus string

const (
	ComparisonStatusMatched              ComparisonStatus = "matched"
	ComparisonStatusDrifted              ComparisonStatus = "drifted"
	ComparisonStatusBlocked              ComparisonStatus = "blocked"
	ComparisonStatusUnsupported          ComparisonStatus = "unsupported"
	ComparisonStatusOperatorActionNeeded ComparisonStatus = "operator_action_needed"
)

type Comparison struct {
	ComparisonID       string           `json:"comparisonId"`
	ValidationID       string           `json:"validationId"`
	CandidateID        string           `json:"candidateId"`
	BaselineRef        string           `json:"baselineRef"`
	TerminalStatus     ComparisonStatus `json:"terminalStatus"`
	LedgerSummary      LedgerSummary    `json:"ledgerSummary"`
	UnsupportedClasses []ToolClass      `json:"unsupportedClasses,omitempty"`
	Denials            []string         `json:"denials,omitempty"`
	AmbiguousCommits   []string         `json:"ambiguousCommits,omitempty"`
	DriftFindings      []string         `json:"driftFindings,omitempty"`
	GeneratedAt        time.Time        `json:"generatedAt"`
}

type RetentionAppliesTo string

const (
	RetentionAppliesAttempts       RetentionAppliesTo = "attempts"
	RetentionAppliesLedgerEntries  RetentionAppliesTo = "ledger_entries"
	RetentionAppliesReconciliation RetentionAppliesTo = "reconciliation_decisions"
	RetentionAppliesComparisons    RetentionAppliesTo = "comparisons"
	RetentionAppliesAll            RetentionAppliesTo = "all"
)

type RetentionMode string

const (
	RetentionModeIndefinite RetentionMode = "indefinite"
	RetentionModeExplicit   RetentionMode = "explicit"
)

type RetentionPolicy struct {
	PolicyID             string             `json:"policyId"`
	TenantID             string             `json:"tenantId,omitempty"`
	AppliesTo            RetentionAppliesTo `json:"appliesTo"`
	Mode                 RetentionMode      `json:"mode"`
	RetentionPeriod      string             `json:"retentionPeriod,omitempty"`
	CreatedByPrincipalID string             `json:"createdByPrincipalId"`
	Reason               string             `json:"reason,omitempty"`
	CreatedAt            time.Time          `json:"createdAt"`
	ExpiresAt            *time.Time         `json:"expiresAt,omitempty"`
}
