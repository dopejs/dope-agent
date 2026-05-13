package store

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/profiles"
)

func TestAgentProfileSelectionAndProjectionRecoverAfterRestart(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	actor := identity.TenantContext{TenantID: "ten_profile_restart", PrincipalID: "prn_admin", Permissions: []identity.Permission{identity.PermissionProfilesManage, identity.PermissionProfilesInspect}}

	first, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	result, err := first.CreateAgentProfile(ctx, actor, profiles.MutationInput{DisplayName: "Restart Agent", Activate: true})
	if err != nil {
		t.Fatalf("CreateAgentProfile returned error: %v", err)
	}
	_, selection, found, err := first.ActiveAgentProfileSelection(ctx, actor.TenantID)
	if err != nil || !found {
		t.Fatalf("ActiveAgentProfileSelection found=%v err=%v", found, err)
	}
	if _, err := first.RecordRuntimeProfileProjection(ctx, profiles.BuildRuntimeProjection(result.Profile, selection, profiles.RuntimeProjectionInput{ResourceKind: profiles.RuntimeResourceRun, ResourceID: "run_restart", RunID: "run_restart"})); err != nil {
		t.Fatalf("RecordRuntimeProfileProjection returned error: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	restarted, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteStore after restart returned error: %v", err)
	}
	defer restarted.Close()
	active, recoveredSelection, found, err := restarted.ActiveAgentProfileSelection(ctx, actor.TenantID)
	if err != nil || !found {
		t.Fatalf("ActiveAgentProfileSelection after restart found=%v err=%v", found, err)
	}
	if active.ProfileID != result.Profile.ProfileID || recoveredSelection.SelectionID != selection.SelectionID {
		t.Fatalf("unexpected recovered selection: %+v %+v", active, recoveredSelection)
	}
	projections, err := restarted.ListRuntimeProfileProjections(ctx, actor.TenantID, string(profiles.RuntimeResourceRun), "run_restart", "", 1)
	if err != nil {
		t.Fatalf("ListRuntimeProfileProjections returned error: %v", err)
	}
	if len(projections) != 1 || projections[0].ProfileID != result.Profile.ProfileID {
		t.Fatalf("unexpected recovered projection: %+v", projections)
	}
}
