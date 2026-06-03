package bindings

import (
	"errors"
	"testing"
)

func validBindingInput() BindingMutationInput {
	return BindingMutationInput{
		ScopeKind:               ScopeChannel,
		ScopeRef:                "discord:chan_123",
		SelectedProfileID:       "prof_1",
		SelectedWorkspaceID:     "ws_1",
		ScopeRefAvailable:       true,
		ScopeConnectorSupported: true,
		ProfileSelectable:       true,
		WorkspaceSelectable:     true,
	}
}

func TestValidateBindingMutation_OK(t *testing.T) {
	if err := ValidateBindingMutation(validBindingInput()); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestValidateBindingMutation_Reasons(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*BindingMutationInput)
		reason string
	}{
		{"cross tenant", func(b *BindingMutationInput) { b.CrossTenant = true }, "cross_tenant_reference_denied"},
		{"bad scope kind", func(b *BindingMutationInput) { b.ScopeKind = "nope" }, "binding_scope_kind_invalid"},
		{"empty ref", func(b *BindingMutationInput) { b.ScopeRef = "" }, "binding_scope_ref_malformed"},
		{"channel unavailable", func(b *BindingMutationInput) { b.ScopeRefAvailable = false }, "channel_unavailable"},
		{"unsupported connector", func(b *BindingMutationInput) { b.ScopeConnectorSupported = false }, "connector_binding_unsupported"},
		{"profile gone", func(b *BindingMutationInput) { b.ProfileSelectable = false }, "selected_profile_unavailable"},
		{"workspace gone", func(b *BindingMutationInput) { b.WorkspaceSelectable = false }, "selected_workspace_unavailable"},
		{"selects nothing", func(b *BindingMutationInput) { b.SelectedProfileID = ""; b.SelectedWorkspaceID = "" }, "binding_selects_nothing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := validBindingInput()
			tc.mutate(&in)
			err := ValidateBindingMutation(in)
			if !errors.Is(err, ErrInvalidBinding) {
				t.Fatalf("expected ErrInvalidBinding, got %v", err)
			}
			if got := ValidationReasonCode(err); got != tc.reason {
				t.Fatalf("expected reason %q, got %q", tc.reason, got)
			}
		})
	}
}

func TestValidateBindingMutation_AccountWorkspaceRejected(t *testing.T) {
	in := validBindingInput()
	in.ScopeKind = ScopeIntegrationAccount
	in.ScopeRef = "integration_acct_1"
	in.SelectedWorkspaceID = "ws_1"
	if got := ValidationReasonCode(ValidateBindingMutation(in)); got != "account_binding_workspace_not_allowed" {
		t.Fatalf("expected account workspace rejection, got %q", got)
	}
}

func TestValidateWorkspaceMutation(t *testing.T) {
	if err := ValidateWorkspaceMutation(WorkspaceMutationInput{DisplayName: "Personal"}); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
	if got := ValidationReasonCode(ValidateWorkspaceMutation(WorkspaceMutationInput{DisplayName: ""})); got != "workspace_display_name_required" {
		t.Fatalf("expected name required, got %q", got)
	}
	if got := ValidationReasonCode(ValidateWorkspaceMutation(WorkspaceMutationInput{DisplayName: "token=abc"})); got != "unsafe_workspace_content" {
		t.Fatalf("expected unsafe rejection, got %q", got)
	}
}

func TestValidateCapabilityVisibilityMutation(t *testing.T) {
	ok := CapabilityVisibilityMutationInput{ScopeKind: VisibilityScopeWorkspace, ScopeRef: "ws_1", CapabilityID: "cap.x", Visibility: VisibilityHidden}
	if err := ValidateCapabilityVisibilityMutation(ok); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
	bad := ok
	bad.ScopeKind = "channel"
	if got := ValidationReasonCode(ValidateCapabilityVisibilityMutation(bad)); got != "visibility_scope_not_editable" {
		t.Fatalf("expected scope not editable, got %q", got)
	}
	badVis := ok
	badVis.Visibility = "nope"
	if got := ValidationReasonCode(ValidateCapabilityVisibilityMutation(badVis)); got != "visibility_value_invalid" {
		t.Fatalf("expected visibility invalid, got %q", got)
	}
}

func TestRepairStatusForReferences(t *testing.T) {
	if got := RepairStatusForReferences(BindingActive, true, true, true, true); got != RepairHealthy {
		t.Fatalf("expected healthy, got %s", got)
	}
	if got := RepairStatusForReferences(BindingDisabled, true, true, true, true); got != RepairDisabled {
		t.Fatalf("expected disabled, got %s", got)
	}
	if got := RepairStatusForReferences(BindingActive, true, true, true, false); got != RepairUnsupported {
		t.Fatalf("expected unsupported, got %s", got)
	}
	if got := RepairStatusForReferences(BindingActive, true, true, false, true); got != RepairStale {
		t.Fatalf("expected stale, got %s", got)
	}
	if got := RepairStatusForReferences(BindingActive, false, true, true, true); got != RepairNeedsRepair {
		t.Fatalf("expected needs_repair, got %s", got)
	}
}
