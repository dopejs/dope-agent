package livevalidation

import "context"

type AttemptFilter struct {
	TenantID         string
	EnvironmentScope string
	CandidateID      string
	Status           AttemptStatus
	Limit            int
}

type LedgerFilter struct {
	TenantID     string
	ValidationID string
	CandidateID  string
	ToolClass    ToolClass
	Outcome      LedgerOutcome
	Limit        int
}

type KillSwitchFilter struct {
	TenantID string
	Scope    KillSwitchScope
	Enabled  *bool
	Limit    int
}

type ComparisonFilter struct {
	TenantID       string
	ValidationID   string
	CandidateID    string
	TerminalStatus ComparisonStatus
	Limit          int
}

type Store interface {
	UpsertLiveValidationAttempt(ctx context.Context, item Attempt) error
	GetLiveValidationAttempt(ctx context.Context, tenantID, validationID string) (Attempt, bool, error)
	ListLiveValidationAttempts(ctx context.Context, filter AttemptFilter) ([]Attempt, error)
	UpsertLiveValidationScope(ctx context.Context, item SideEffectScope, tenantID string) error
	UpsertLiveValidationApproval(ctx context.Context, item FreshApproval) error
	AppendLiveValidationLedgerEntry(ctx context.Context, item SideEffectLedgerEntry) error
	UpdateLiveValidationLedgerEntryOutcome(ctx context.Context, ledgerEntryID string, outcome LedgerOutcome, reasonCode string) error
	ListLiveValidationLedgerEntries(ctx context.Context, filter LedgerFilter) ([]SideEffectLedgerEntry, error)
	UpsertLiveValidationKillSwitch(ctx context.Context, item KillSwitch) error
	ListLiveValidationKillSwitches(ctx context.Context, filter KillSwitchFilter) ([]KillSwitch, error)
	UpsertLiveValidationSupportMatrixSnapshot(ctx context.Context, tenantID, snapshotID string, rows []MatrixRow) error
	SaveLiveValidationAmbiguousCommit(ctx context.Context, item AmbiguousCommit) error
	SaveLiveValidationReconciliationResolution(ctx context.Context, item ReconciliationResolution) error
	SaveLiveValidationComparison(ctx context.Context, item Comparison) error
	ListLiveValidationComparisons(ctx context.Context, filter ComparisonFilter) ([]Comparison, error)
	SaveLiveValidationRetentionPolicy(ctx context.Context, item RetentionPolicy) error
}
