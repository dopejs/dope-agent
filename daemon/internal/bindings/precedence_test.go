package bindings

import "testing"

func allAvailable(string) bool  { return true }
func noneAvailable(string) bool { return false }

func baseInput() ResolutionInput {
	return ResolutionInput{
		TenantDefaultProfileID:        "prof_default",
		TenantDefaultProfileVersionID: "profv_default",
		TenantDefaultWorkspaceID:      "ws_default",
		ProfileAvailable:              allAvailable,
		WorkspaceAvailable:            allAvailable,
	}
}

func TestResolveSelection_NoBinding_UsesTenantDefault(t *testing.T) {
	sel := ResolveSelection(baseInput())
	if sel.Outcome != OutcomeDefault {
		t.Fatalf("expected default outcome, got %s", sel.Outcome)
	}
	if sel.BindingScope != RuntimeScopeTenantDefault {
		t.Fatalf("expected tenant_default scope, got %s", sel.BindingScope)
	}
	if sel.SelectedProfileID != "prof_default" || sel.SelectedWorkspaceID != "ws_default" {
		t.Fatalf("unexpected selection: %+v", sel)
	}
}

func TestResolveSelection_ChannelBindingWins(t *testing.T) {
	in := baseInput()
	in.ChannelBinding = &BindingRule{
		BindingID:           "bnd_chan",
		ScopeKind:           ScopeChannel,
		Status:              BindingActive,
		SelectedProfileID:   "prof_chan",
		SelectedWorkspaceID: "ws_chan",
	}
	in.AccountBinding = &BindingRule{
		BindingID:         "bnd_acct",
		ScopeKind:         ScopeIntegrationAccount,
		Status:            BindingActive,
		SelectedProfileID: "prof_acct",
	}
	sel := ResolveSelection(in)
	if sel.Outcome != OutcomeResolved {
		t.Fatalf("expected resolved, got %s", sel.Outcome)
	}
	if sel.BindingScope != RuntimeScopeChannel || sel.BindingID != "bnd_chan" {
		t.Fatalf("expected channel scope, got %s/%s", sel.BindingScope, sel.BindingID)
	}
	if sel.SelectedProfileID != "prof_chan" || sel.SelectedWorkspaceID != "ws_chan" {
		t.Fatalf("channel binding should win: %+v", sel)
	}
}

func TestResolveSelection_AccountDefaultWhenNoChannel(t *testing.T) {
	in := baseInput()
	in.AccountBinding = &BindingRule{
		BindingID:         "bnd_acct",
		ScopeKind:         ScopeIntegrationAccount,
		Status:            BindingActive,
		SelectedProfileID: "prof_acct",
	}
	sel := ResolveSelection(in)
	if sel.Outcome != OutcomeResolved {
		t.Fatalf("expected resolved, got %s", sel.Outcome)
	}
	if sel.BindingScope != RuntimeScopeIntegrationAccount {
		t.Fatalf("expected account scope, got %s", sel.BindingScope)
	}
	if sel.SelectedProfileID != "prof_acct" {
		t.Fatalf("account profile expected, got %s", sel.SelectedProfileID)
	}
	// Account binding supplies no workspace; tenant default workspace applies.
	if sel.SelectedWorkspaceID != "ws_default" {
		t.Fatalf("expected tenant default workspace, got %s", sel.SelectedWorkspaceID)
	}
}

// B4: an explicit channel binding stays stable when the account default changes.
func TestResolveSelection_ChannelStableWhenAccountChanges(t *testing.T) {
	in := baseInput()
	in.ChannelBinding = &BindingRule{
		BindingID:         "bnd_chan",
		ScopeKind:         ScopeChannel,
		Status:            BindingActive,
		SelectedProfileID: "prof_chan",
	}
	in.AccountBinding = &BindingRule{
		BindingID:         "bnd_acct_v2",
		ScopeKind:         ScopeIntegrationAccount,
		Status:            BindingActive,
		SelectedProfileID: "prof_acct_changed",
	}
	sel := ResolveSelection(in)
	if sel.SelectedProfileID != "prof_chan" {
		t.Fatalf("channel binding must be stable, got %s", sel.SelectedProfileID)
	}
}

// B5/FR-031: invalid selected profile fails closed, no silent substitution.
func TestResolveSelection_InvalidProfileFailsClosed(t *testing.T) {
	in := baseInput()
	in.ChannelBinding = &BindingRule{
		BindingID:         "bnd_chan",
		ScopeKind:         ScopeChannel,
		Status:            BindingActive,
		SelectedProfileID: "prof_archived",
	}
	in.ProfileAvailable = func(id string) bool { return id != "prof_archived" }
	sel := ResolveSelection(in)
	if sel.Outcome != OutcomeRepairRequired {
		t.Fatalf("expected repair_required, got %s", sel.Outcome)
	}
	if sel.RepairReason != "selected_profile_unavailable" {
		t.Fatalf("unexpected repair reason: %s", sel.RepairReason)
	}
	// Must NOT have substituted the tenant default.
	if sel.SelectedProfileID == "prof_default" {
		t.Fatalf("must not silently substitute tenant default")
	}
}

func TestResolveSelection_InvalidWorkspaceFailsClosed(t *testing.T) {
	in := baseInput()
	in.ChannelBinding = &BindingRule{
		BindingID:           "bnd_chan",
		ScopeKind:           ScopeChannel,
		Status:              BindingActive,
		SelectedProfileID:   "prof_chan",
		SelectedWorkspaceID: "ws_gone",
	}
	in.WorkspaceAvailable = func(id string) bool { return id != "ws_gone" }
	sel := ResolveSelection(in)
	if sel.Outcome != OutcomeRepairRequired || sel.RepairReason != "selected_workspace_unavailable" {
		t.Fatalf("expected workspace repair_required, got %+v", sel)
	}
}

func TestResolveSelection_DisabledBindingIgnored(t *testing.T) {
	in := baseInput()
	in.ChannelBinding = &BindingRule{
		BindingID:         "bnd_chan",
		ScopeKind:         ScopeChannel,
		Status:            BindingDisabled,
		SelectedProfileID: "prof_chan",
	}
	sel := ResolveSelection(in)
	if sel.Outcome != OutcomeDefault || sel.SelectedProfileID != "prof_default" {
		t.Fatalf("disabled binding must be ignored, got %+v", sel)
	}
}

func TestResolveSelection_NilOraclesFailClosed(t *testing.T) {
	sel := ResolveSelection(ResolutionInput{TenantDefaultProfileID: "p", TenantDefaultWorkspaceID: "w"})
	if sel.Outcome != OutcomeRepairRequired {
		t.Fatalf("missing oracles must fail closed, got %s", sel.Outcome)
	}
}
