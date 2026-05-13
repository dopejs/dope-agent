package store

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/profiles"
)

func TestAgentProfileProjectionPersistsActiveSelectionAndRuntimeEvidence(t *testing.T) {
	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	ctx := context.Background()
	actor := identity.TenantContext{TenantID: "ten_profile_projection", PrincipalID: "prn_admin", Permissions: []identity.Permission{identity.PermissionProfilesManage, identity.PermissionProfilesInspect}}

	result, err := sqliteStore.CreateAgentProfile(ctx, actor, profiles.MutationInput{
		DisplayName: "Projection Agent",
		Persona:     profiles.Persona{Tone: "direct", SafeSummary: "direct profile"},
		Activate:    true,
	})
	if err != nil {
		t.Fatalf("CreateAgentProfile returned error: %v", err)
	}
	active, selection, found, err := sqliteStore.ActiveAgentProfileSelection(ctx, actor.TenantID)
	if err != nil || !found {
		t.Fatalf("ActiveAgentProfileSelection found=%v err=%v", found, err)
	}
	if active.ProfileID != result.Profile.ProfileID || selection.SelectionReason != profiles.SelectionDefaultSeeded {
		t.Fatalf("unexpected active selection: %+v %+v", active, selection)
	}

	projection := profiles.BuildRuntimeProjection(active, selection, profiles.RuntimeProjectionInput{
		ResourceKind: profiles.RuntimeResourceThread,
		ResourceID:   "thr_profile_projection",
		ThreadID:     "thr_profile_projection",
		SessionID:    "seg_profile_projection",
	})
	recorded, err := sqliteStore.RecordRuntimeProfileProjection(ctx, projection)
	if err != nil {
		t.Fatalf("RecordRuntimeProfileProjection returned error: %v", err)
	}
	items, err := sqliteStore.ListRuntimeProfileProjections(ctx, actor.TenantID, string(profiles.RuntimeResourceThread), "thr_profile_projection", "", 5)
	if err != nil {
		t.Fatalf("ListRuntimeProfileProjections returned error: %v", err)
	}
	if len(items) != 1 || items[0].RuntimeProfileProjectionID != recorded.RuntimeProfileProjectionID {
		t.Fatalf("unexpected projections: %+v", items)
	}
	if items[0].ConfigurationScope != "explicit_profile_configuration" || items[0].DeferredBindingClassification == "" {
		t.Fatalf("expected non-memory classification evidence, got %+v", items[0])
	}
}
