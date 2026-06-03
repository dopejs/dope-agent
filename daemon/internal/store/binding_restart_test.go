package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/bindings"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

// B16/FR-027: bindings, workspaces, capability visibility, and runtime evidence survive a
// daemon restart (reopening the same data directory).
func TestBindingState_SurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	actor := identity.TenantContext{TenantID: "ten_restart", PrincipalID: "prn_admin"}

	s1, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	profile := seedActiveProfile(t, s1, "ten_restart")
	ws, err := s1.EnsureDefaultWorkspace(ctx, "ten_restart")
	if err != nil {
		t.Fatalf("ensure default: %v", err)
	}
	rule, _, err := s1.CreateBindingRule(ctx, actor, bindings.CreateBindingRequest{ScopeKind: bindings.ScopeChannel, ScopeRef: "discord:persist", SelectedProfileID: profile.ProfileID, SelectedWorkspaceID: ws.WorkspaceID})
	if err != nil {
		t.Fatalf("create binding: %v", err)
	}
	if _, _, err := s1.SetCapabilityVisibility(ctx, actor, bindings.SetVisibilityRequest{ScopeKind: bindings.VisibilityScopeWorkspace, ScopeRef: ws.WorkspaceID, CapabilityID: "tool.shell", Visibility: bindings.VisibilityHidden}); err != nil {
		t.Fatalf("set visibility: %v", err)
	}
	ev := bindings.BuildRuntimeBindingEvidence(bindings.EffectiveBindingSelection{Outcome: bindings.OutcomeResolved, BindingScope: bindings.RuntimeScopeChannel, SelectedProfileID: profile.ProfileID, SelectedWorkspaceID: ws.WorkspaceID}, bindings.RuntimeBindingEvidenceInput{TenantID: "ten_restart", ResourceKind: "thread", ResourceID: "thr_p", OccurredAt: time.Now().UTC()})
	if _, err := s1.RecordRuntimeBindingEvidence(ctx, ev); err != nil {
		t.Fatalf("record evidence: %v", err)
	}
	_ = s1.Close()

	s2, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("open 2: %v", err)
	}
	defer s2.Close()

	got, found, err := s2.GetBindingRule(ctx, "ten_restart", rule.BindingID)
	if err != nil || !found {
		t.Fatalf("binding lost after restart: %v found=%v", err, found)
	}
	if got.SelectedProfileID != profile.ProfileID {
		t.Fatalf("binding selection lost: %+v", got)
	}
	vis, err := s2.ListCapabilityVisibility(ctx, "ten_restart", bindings.VisibilityScopeWorkspace, ws.WorkspaceID)
	if err != nil || len(vis) != 1 {
		t.Fatalf("visibility lost: %v len=%d", err, len(vis))
	}
	_, evFound, err := s2.LatestRuntimeBindingEvidence(ctx, "ten_restart", "thread", "thr_p")
	if err != nil || !evFound {
		t.Fatalf("runtime evidence lost: %v found=%v", err, evFound)
	}
}
