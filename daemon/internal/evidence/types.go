// Package evidence implements the support diagnostics + redacted evidence bundle (Roadmap 71): a
// permission-gated, tenant-scoped, redacted-by-default bundle that collects resource summaries +
// links (never raw secrets or unbounded logs) for support/incident triage. Bundle generation and
// access are audited; redaction failure fails closed.
package evidence

import "time"

// ScopeKind selects what an evidence bundle is about.
type ScopeKind string

const (
	ScopeRun         ScopeKind = "run"
	ScopeWorkflow    ScopeKind = "workflow"
	ScopeThread      ScopeKind = "thread"
	ScopeConnector   ScopeKind = "connector"
	ScopeProvider    ScopeKind = "provider"
	ScopeRoutine     ScopeKind = "routine"
	ScopeQuotaDenial ScopeKind = "quota_denial"
	ScopeTimeWindow  ScopeKind = "time_window"
)

// Scope is the bundle target: a kind + a ref, or a time window.
type Scope struct {
	Kind        ScopeKind  `json:"kind"`
	Ref         string     `json:"ref,omitempty"`
	WindowStart *time.Time `json:"windowStart,omitempty"`
	WindowEnd   *time.Time `json:"windowEnd,omitempty"`
}

// RedactionStatus is the bundle's redaction outcome.
type RedactionStatus string

const (
	RedactionRedacted     RedactionStatus = "redacted"
	RedactionFailedClosed RedactionStatus = "failed_closed"
)

// Section is one collected evidence section: redacted resource summaries + links, never raw
// secrets or full logs.
type Section struct {
	Kind         string            `json:"kind"`
	ResourceRefs []string          `json:"resourceRefs,omitempty"`
	Summary      map[string]string `json:"summary,omitempty"`
	Links        []string          `json:"links,omitempty"`
}

// Bundle is a generated, redacted evidence bundle.
type Bundle struct {
	BundleID           string          `json:"bundleId"`
	TenantID           string          `json:"tenantId"`
	Actor              string          `json:"actor"`
	Scope              Scope           `json:"scope"`
	Sections           []Section       `json:"sections"`
	RedactionStatus    RedactionStatus `json:"redactionStatus"`
	CreatedAt          time.Time       `json:"createdAt"`
	RetentionExpiresAt time.Time       `json:"retentionExpiresAt"`
}

// AccessEvent is an audit record for bundle generation or access.
type AccessEvent struct {
	BundleID   string    `json:"bundleId"`
	TenantID   string    `json:"tenantId"`
	Actor      string    `json:"actor"`
	Action     string    `json:"action"` // generated | accessed
	OccurredAt time.Time `json:"occurredAt"`
}
