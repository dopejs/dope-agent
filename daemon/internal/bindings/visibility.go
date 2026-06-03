package bindings

import (
	"errors"
	"strings"
)

// ErrCapabilityNotExecutable is returned when a hidden or disabled capability is requested
// for execution under the active binding (FR-016).
var ErrCapabilityNotExecutable = errors.New("capability is not executable under the active binding")

// EnforceExecutable returns ErrCapabilityNotExecutable when the decision forbids execution.
func EnforceExecutable(decision CapabilityDecision) error {
	if decision.Executable {
		return nil
	}
	return ErrCapabilityNotExecutable
}

// EffectiveVisibility is the resolved visibility of a capability after combining all
// applicable scopes. It is distinct from Visibility (the per-scope user input) because
// "default_enabled" collapses to visible+offered and a higher-level prohibition
// surfaces as "blocked".
type EffectiveVisibility string

const (
	EffectiveVisible  EffectiveVisibility = "visible"
	EffectiveHidden   EffectiveVisibility = "hidden"
	EffectiveDisabled EffectiveVisibility = "disabled"
	// EffectiveBlocked means a higher-level tenant/connector limit prohibits the
	// capability regardless of profile/workspace policy.
	EffectiveBlocked EffectiveVisibility = "blocked"
)

// ScopeVisibility is one scope's contribution to the effective decision.
type ScopeVisibility struct {
	Scope      string     // safe scope label, e.g. "tenant", "connector", "profile", "workspace"
	Visibility Visibility // the policy value at this scope; empty means "no opinion"
	// Blocked marks a hard higher-level prohibition (tenant/connector limit).
	Blocked bool
}

// VisibilityInput resolves one capability's effective visibility from the strictest of
// the higher-level tenant/connector limits and the profile/workspace policies.
type VisibilityInput struct {
	CapabilityID string
	// Limits are higher-level tenant/connector constraints (enforced, not user-edited
	// in this phase). A Blocked limit wins over everything (FR-017).
	Limits []ScopeVisibility
	// ProfilePolicy and WorkspacePolicy are the user-editable scopes (FR-018). Empty
	// Visibility means the scope set no policy for this capability.
	ProfilePolicy   Visibility
	WorkspacePolicy Visibility
}

// strictnessRank orders visibility values; higher is stricter and wins.
func strictnessRank(v Visibility) int {
	switch v {
	case VisibilityDisabled:
		return 3
	case VisibilityHidden:
		return 2
	case VisibilityVisible, VisibilityDefaultEnabled:
		return 1
	default:
		return 0
	}
}

// ResolveCapabilityVisibility computes the effective decision for a single capability.
// Strictest wins across tenant/connector limits and profile/workspace policy.
// default_enabled never overrides a hidden/disabled/blocked outcome (FR-019).
func ResolveCapabilityVisibility(input VisibilityInput) CapabilityDecision {
	decision := CapabilityDecision{CapabilityID: strings.TrimSpace(input.CapabilityID)}

	// Higher-level hard prohibition wins unconditionally.
	for _, limit := range input.Limits {
		if limit.Blocked {
			decision.Effective = EffectiveBlocked
			decision.Offered = false
			decision.Executable = false
			decision.Reason = "blocked_by_higher_policy"
			decision.Scope = safeScopeLabel(limit.Scope)
			return decision
		}
	}

	// Collect every scope's opinion with a stable scope label for the winning reason.
	type opinion struct {
		scope string
		vis   Visibility
	}
	opinions := make([]opinion, 0, len(input.Limits)+2)
	for _, limit := range input.Limits {
		if limit.Visibility != "" {
			opinions = append(opinions, opinion{scope: safeScopeLabel(limit.Scope), vis: limit.Visibility})
		}
	}
	if input.ProfilePolicy != "" {
		opinions = append(opinions, opinion{scope: "profile", vis: input.ProfilePolicy})
	}
	if input.WorkspacePolicy != "" {
		opinions = append(opinions, opinion{scope: "workspace", vis: input.WorkspacePolicy})
	}

	// Default when no scope expresses an opinion: capability is visible but not
	// offered-by-default (it is allowable, not promoted).
	if len(opinions) == 0 {
		decision.Effective = EffectiveVisible
		decision.Offered = true
		decision.Executable = true
		decision.Reason = "no_policy_default_visible"
		decision.Scope = "default"
		return decision
	}

	// Strictest wins; record which scope produced it.
	winner := opinions[0]
	sawDefaultEnabled := false
	for _, o := range opinions {
		if o.vis == VisibilityDefaultEnabled {
			sawDefaultEnabled = true
		}
		if strictnessRank(o.vis) > strictnessRank(winner.vis) {
			winner = o
		}
	}

	switch winner.vis {
	case VisibilityDisabled:
		decision.Effective = EffectiveDisabled
		decision.Offered = false
		decision.Executable = false
		decision.Reason = "disabled_by_policy"
	case VisibilityHidden:
		decision.Effective = EffectiveHidden
		decision.Offered = false
		decision.Executable = false
		decision.Reason = "hidden_by_policy"
	default: // visible or default_enabled
		decision.Effective = EffectiveVisible
		decision.Offered = true
		decision.Executable = true
		// default_enabled only takes effect when nothing stricter applies (FR-019).
		decision.DefaultEnabled = sawDefaultEnabled
		if sawDefaultEnabled {
			decision.Reason = "default_enabled"
		} else {
			decision.Reason = "visible_by_policy"
		}
	}
	decision.Scope = winner.scope
	return decision
}

// ResolveVisibilitySet resolves a batch of capabilities, preserving input order.
func ResolveVisibilitySet(inputs []VisibilityInput) []CapabilityDecision {
	decisions := make([]CapabilityDecision, 0, len(inputs))
	for _, in := range inputs {
		decisions = append(decisions, ResolveCapabilityVisibility(in))
	}
	return decisions
}

func safeScopeLabel(scope string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return "unknown"
	}
	return scope
}
