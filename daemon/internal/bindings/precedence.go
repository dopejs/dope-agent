package bindings

import "strings"

// ResolutionInput carries the candidate bindings and tenant defaults needed to resolve
// an effective binding selection at work-start. Availability oracles let the resolver
// fail closed (FR-031) without depending on the store.
type ResolutionInput struct {
	// ChannelBinding is the active channel-scoped binding for the originating channel,
	// if one exists. Nil means no channel binding.
	ChannelBinding *BindingRule
	// AccountBinding is the active integration-account default binding, if one exists.
	AccountBinding *BindingRule

	TenantDefaultProfileID        string
	TenantDefaultProfileVersionID string
	TenantDefaultWorkspaceID      string

	// ProfileAvailable reports whether a profile id is currently selectable (active,
	// not archived/disabled/removed). Required.
	ProfileAvailable func(profileID string) bool
	// WorkspaceAvailable reports whether a workspace id is currently selectable.
	// Required.
	WorkspaceAvailable func(workspaceID string) bool
}

// ResolveSelection applies the fixed precedence order — channel binding, then
// integration-account default, then tenant default (FR-006) — and fails closed when an
// explicit binding selects an invalid profile or workspace (FR-031). It never silently
// substitutes a different profile, workspace, or lower-precedence binding.
func ResolveSelection(input ResolutionInput) EffectiveBindingSelection {
	profileAvailable := input.ProfileAvailable
	if profileAvailable == nil {
		profileAvailable = func(string) bool { return false }
	}
	workspaceAvailable := input.WorkspaceAvailable
	if workspaceAvailable == nil {
		workspaceAvailable = func(string) bool { return false }
	}

	channel := activeBinding(input.ChannelBinding)
	account := activeBinding(input.AccountBinding)

	sel := EffectiveBindingSelection{}

	// --- Determine the profile source by precedence. ---
	var profileID, profileVersionID string
	explicit := false
	switch {
	case channel != nil && strings.TrimSpace(channel.SelectedProfileID) != "":
		profileID = strings.TrimSpace(channel.SelectedProfileID)
		profileVersionID = strings.TrimSpace(channel.SelectedProfileVersionID)
		sel.BindingScope = RuntimeScopeChannel
		sel.BindingID = channel.BindingID
		explicit = true
	case account != nil && strings.TrimSpace(account.SelectedProfileID) != "":
		profileID = strings.TrimSpace(account.SelectedProfileID)
		profileVersionID = strings.TrimSpace(account.SelectedProfileVersionID)
		sel.BindingScope = RuntimeScopeIntegrationAccount
		sel.BindingID = account.BindingID
		explicit = true
	default:
		profileID = strings.TrimSpace(input.TenantDefaultProfileID)
		profileVersionID = strings.TrimSpace(input.TenantDefaultProfileVersionID)
		if sel.BindingScope == "" {
			sel.BindingScope = RuntimeScopeTenantDefault
		}
	}

	// --- Determine the workspace source. Only channel bindings carry a workspace;
	// integration-account defaults supply profile only (FR-005). ---
	var workspaceID string
	if channel != nil && strings.TrimSpace(channel.SelectedWorkspaceID) != "" {
		workspaceID = strings.TrimSpace(channel.SelectedWorkspaceID)
		// A channel binding that only set a workspace still makes channel the scope.
		if sel.BindingScope == RuntimeScopeTenantDefault {
			sel.BindingScope = RuntimeScopeChannel
			sel.BindingID = channel.BindingID
		}
		explicit = true
	} else {
		workspaceID = strings.TrimSpace(input.TenantDefaultWorkspaceID)
	}

	sel.SelectedProfileID = profileID
	sel.SelectedProfileVersionID = profileVersionID
	sel.SelectedWorkspaceID = workspaceID

	// --- Fail closed on invalid explicit selections (FR-031). ---
	if profileID == "" || !profileAvailable(profileID) {
		sel.Outcome = OutcomeRepairRequired
		sel.RepairStatus = RepairInvalid
		sel.RepairReason = "selected_profile_unavailable"
		return sel
	}
	if workspaceID == "" || !workspaceAvailable(workspaceID) {
		sel.Outcome = OutcomeRepairRequired
		sel.RepairStatus = RepairInvalid
		sel.RepairReason = "selected_workspace_unavailable"
		return sel
	}

	if explicit {
		sel.Outcome = OutcomeResolved
	} else {
		sel.Outcome = OutcomeDefault
	}
	return sel
}

// activeBinding returns the binding only if it is non-nil and active; disabled bindings
// are treated as absent for resolution.
func activeBinding(b *BindingRule) *BindingRule {
	if b == nil {
		return nil
	}
	if b.Status == BindingDisabled {
		return nil
	}
	return b
}
