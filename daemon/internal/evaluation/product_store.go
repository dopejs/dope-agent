package evaluation

import "context"

type ProductListFilter struct {
	TenantID string
	Cursor   string
	Limit    int
}

type DiscoveryPolicyFilter struct {
	ProductListFilter
	Enabled *bool
}

type DiscoveryRunFilter struct {
	ProductListFilter
	Status     ProductLifecycleStatus
	SourceKind SourceKind
}

type DiscoveredCandidateFilter struct {
	ProductListFilter
	DiscoveryRunID   string
	SourceKind       SourceKind
	ReadinessStatus  ReadinessStatus
	SuppressionState SuppressionState
	ScoreBand        ScoreBand
}

type RetentionApplicationFilter struct {
	ProductListFilter
	ResourceKinds []ProductResourceKind
	DryRun        bool
}

type ProductStore interface {
	UpsertDiscoveryPolicy(context.Context, DiscoveryPolicy) error
	ListDiscoveryPolicies(context.Context, DiscoveryPolicyFilter) ([]DiscoveryPolicy, error)
	SaveDiscoveryRun(context.Context, DiscoveryRun) error
	ListDiscoveryRuns(context.Context, DiscoveryRunFilter) ([]DiscoveryRun, error)
	SaveDiscoveredCandidate(context.Context, DiscoveredCandidate, CandidateEvidence) error
	ListDiscoveredCandidates(context.Context, DiscoveredCandidateFilter) ([]DiscoveredCandidate, error)
	CreateSuppression(context.Context, SuppressionRecord) error
	ApplyRetention(context.Context, RetentionApplicationFilter) error
}
