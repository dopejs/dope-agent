package bindings

import "time"

// ScopeKind identifies what a binding rule applies to.
type ScopeKind string

const (
	ScopeChannel            ScopeKind = "channel"
	ScopeIntegrationAccount ScopeKind = "integration_account"
)

// WorkspaceStatus is the lifecycle state of a workspace record.
type WorkspaceStatus string

const (
	WorkspaceActive   WorkspaceStatus = "active"
	WorkspaceArchived WorkspaceStatus = "archived"
	WorkspaceDisabled WorkspaceStatus = "disabled"
)

// BindingStatus is the lifecycle state of a binding rule.
type BindingStatus string

const (
	BindingActive   BindingStatus = "active"
	BindingDisabled BindingStatus = "disabled"
)

// RepairStatus is the safe, user-facing health of a workspace or binding rule.
type RepairStatus string

const (
	RepairHealthy     RepairStatus = "healthy"
	RepairDisabled    RepairStatus = "disabled"
	RepairInvalid     RepairStatus = "invalid"
	RepairStale       RepairStatus = "stale"
	RepairUnsupported RepairStatus = "unsupported"
	RepairNeedsRepair RepairStatus = "needs_repair"
)

// ValidationStatus marks whether stored state passed validation.
type ValidationStatus string

const (
	ValidationValid   ValidationStatus = "valid"
	ValidationInvalid ValidationStatus = "invalid"
)

// RedactionStatus mirrors the profile domain redaction vocabulary.
type RedactionStatus string

const (
	RedactionRedacted    RedactionStatus = "redacted"
	RedactionNotRequired RedactionStatus = "not_required"
	RedactionSuppressed  RedactionStatus = "suppressed"
	RedactionFailed      RedactionStatus = "redaction_failed"
)

// Visibility is the per-capability, per-scope policy value an authorized user sets.
type Visibility string

const (
	VisibilityVisible        Visibility = "visible"
	VisibilityHidden         Visibility = "hidden"
	VisibilityDisabled       Visibility = "disabled"
	VisibilityDefaultEnabled Visibility = "default_enabled"
)

// VisibilityScopeKind is the scope a capability visibility policy is attached to.
// In this phase only profile and workspace scopes are user-editable; tenant and
// connector limits are enforced as higher-level constraints (see visibility.go).
type VisibilityScopeKind string

const (
	VisibilityScopeProfile   VisibilityScopeKind = "profile"
	VisibilityScopeWorkspace VisibilityScopeKind = "workspace"
)

// ResolutionOutcome is the result of resolving an effective binding selection at
// work-start.
type ResolutionOutcome string

const (
	// OutcomeResolved means an explicit binding produced a valid selection.
	OutcomeResolved ResolutionOutcome = "resolved"
	// OutcomeDefault means no explicit binding applied; tenant defaults were used.
	OutcomeDefault ResolutionOutcome = "default"
	// OutcomeRepairRequired means the selected profile/workspace is invalid and the
	// system must fail closed (FR-031): new work is blocked, no silent substitution.
	OutcomeRepairRequired ResolutionOutcome = "repair_required"
)

// Classification labels runtime binding evidence so legacy/default behavior is never
// presented as explicit user-configured binding (FR-026). It supersedes the planted
// profile-projection marker "roadmap_58_deferred_binding_unapplied".
type Classification string

const (
	ClassificationApplied Classification = "applied_binding"
	ClassificationDefault Classification = "default_binding"
	ClassificationLegacy  Classification = "legacy_default"
)

// BindingRuntimeScope identifies which binding precedence level influenced a run.
type BindingRuntimeScope string

const (
	RuntimeScopeChannel            BindingRuntimeScope = "channel"
	RuntimeScopeIntegrationAccount BindingRuntimeScope = "integration_account"
	RuntimeScopeTenantDefault      BindingRuntimeScope = "tenant_default"
)

// Workspace is a tenant-scoped persisted product record used for binding identity,
// safe display, status, audit, and repair. It grants no storage or filesystem access
// by itself (FR-002, FR-020).
type Workspace struct {
	WorkspaceID      string
	TenantID         string
	DisplayName      string
	Status           WorkspaceStatus
	IsDefault        bool
	OwnerPrincipalID string
	RepairStatus     RepairStatus
	RedactionStatus  RedactionStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ArchivedAt       *time.Time
}

// BindingRule connects a binding scope (channel or integration account) to a selected
// profile and/or workspace.
type BindingRule struct {
	BindingID                 string
	TenantID                  string
	ScopeKind                 ScopeKind
	ScopeRef                  string
	SelectedProfileID         string
	SelectedProfileVersionID  string
	SelectedWorkspaceID       string
	Status                    BindingStatus
	RepairStatus              RepairStatus
	ValidationStatus          ValidationStatus
	ActorPrincipalID          string
	AuditEventID              string
	PreviousSelectionSummary  string
	ResultingSelectionSummary string
	RedactionStatus           RedactionStatus
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
	DisabledAt                *time.Time
}

// CapabilityVisibilityPolicy is tenant-owned policy describing a capability's
// visibility for a profile- or workspace-scoped binding.
type CapabilityVisibilityPolicy struct {
	PolicyID         string
	TenantID         string
	ScopeKind        VisibilityScopeKind
	ScopeRef         string
	CapabilityID     string
	Visibility       Visibility
	ActorPrincipalID string
	ValidationStatus ValidationStatus
	RedactionStatus  RedactionStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// EffectiveBindingSelection is the resolved profile, workspace, binding scope, and
// capability visibility set that applies when new work starts. It is materialized into
// RuntimeBindingEvidence; it is not persisted as its own table.
type EffectiveBindingSelection struct {
	Outcome                  ResolutionOutcome
	BindingScope             BindingRuntimeScope
	BindingID                string
	SelectedProfileID        string
	SelectedProfileVersionID string
	SelectedWorkspaceID      string
	// RepairStatus and RepairReason are set when Outcome == OutcomeRepairRequired.
	RepairStatus RepairStatus
	RepairReason string
	// CapabilityVisibility summarizes the effective per-capability decisions.
	CapabilityVisibility []CapabilityDecision
}

// CapabilityDecision is the resolved, safe-to-surface visibility decision for one
// capability under the active binding (FR-013, FR-014, SC-012).
type CapabilityDecision struct {
	CapabilityID   string
	Effective      EffectiveVisibility
	DefaultEnabled bool
	Offered        bool
	Executable     bool
	// Reason is a safe machine-readable reason code explaining the decision.
	Reason string
	// Scope is the scope that produced the strictest (winning) constraint.
	Scope string
}

// RuntimeBindingEvidence is durable evidence attached to run/thread/session/workflow/
// handoff/channel inspection showing which binding selections influenced execution.
type RuntimeBindingEvidence struct {
	ProjectionID             string
	TenantID                 string
	ResourceKind             string
	ResourceID               string
	SelectedProfileID        string
	SelectedProfileVersionID string
	SelectedWorkspaceID      string
	BindingScope             BindingRuntimeScope
	BindingID                string
	Classification           Classification
	SelectionReason          string
	CapabilityVisibility     []CapabilityDecision
	OccurredAt               time.Time
	RedactionStatus          RedactionStatus
}

// DeferredBindingClassificationMarker is the placeholder value the profile runtime
// projection recorded before Roadmap 58. When an explicit binding influences a run,
// evidence records ClassificationApplied instead of this marker (FR-026, B22).
const DeferredBindingClassificationMarker = "roadmap_58_deferred_binding_unapplied"
