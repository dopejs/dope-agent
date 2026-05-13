package store

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/profiles"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
)

func TestAgentProfileStoreCreateUpdateRetireAndFallback(t *testing.T) {
	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	ctx := context.Background()
	actor := identity.TenantContext{TenantID: "ten_profiles", PrincipalID: "prn_admin", Permissions: []identity.Permission{identity.PermissionProfilesManage, identity.PermissionProfilesInspect}}

	created, err := sqliteStore.CreateAgentProfile(ctx, actor, profiles.MutationInput{DisplayName: "Support", Persona: profiles.Persona{Tone: "direct"}, Activate: true})
	if err != nil {
		t.Fatalf("CreateAgentProfile returned error: %v", err)
	}
	if created.Profile.Status != profiles.StatusActive || created.Version.VersionNumber != 1 {
		t.Fatalf("unexpected created profile: %+v", created)
	}

	updated, err := sqliteStore.UpdateAgentProfile(ctx, actor, created.Profile.ProfileID, profiles.MutationInput{DisplayName: "Support Updated", Persona: profiles.Persona{Tone: "calm"}})
	if err != nil {
		t.Fatalf("UpdateAgentProfile returned error: %v", err)
	}
	if updated.Version.VersionNumber != 2 || updated.Version.SourceVersionID != created.Version.ProfileVersionID {
		t.Fatalf("unexpected version lineage: %+v", updated.Version)
	}

	selection, err := sqliteStore.ActivateAgentProfile(ctx, actor, created.Profile.ProfileID, profiles.ActivationInput{ProfileVersionID: updated.Version.ProfileVersionID})
	if err != nil {
		t.Fatalf("ActivateAgentProfile returned error: %v", err)
	}
	if selection.ProfileVersionID != updated.Version.ProfileVersionID {
		t.Fatalf("unexpected selection: %+v", selection)
	}

	retired, err := sqliteStore.RetireAgentProfile(ctx, actor, created.Profile.ProfileID, profiles.StatusArchived, profiles.RetirementInput{ReasonCode: "test_archive"})
	if err != nil {
		t.Fatalf("RetireAgentProfile returned error: %v", err)
	}
	if retired.Profile.Status != profiles.StatusArchived || retired.Selection.SelectionReason != profiles.SelectionSystemFallback {
		t.Fatalf("expected archived profile and fallback selection, got %+v", retired)
	}

	active, activeSelection, _, err := sqliteStore.ActiveAgentProfileSelection(ctx, actor.TenantID)
	if err != nil {
		t.Fatalf("ActiveAgentProfileSelection returned error: %v", err)
	}
	if active.ProfileID == created.Profile.ProfileID || activeSelection.SelectionReason != profiles.SelectionSystemFallback {
		t.Fatalf("fallback did not replace retired default: %+v %+v", active, activeSelection)
	}
}

func TestAgentProfileStoreRetainsHistoryRollbackAndOverlays(t *testing.T) {
	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	ctx := context.Background()
	actor := identity.TenantContext{TenantID: "ten_profile_history", PrincipalID: "prn_admin", Permissions: []identity.Permission{identity.PermissionProfilesManage, identity.PermissionProfilesInspect}}

	created, err := sqliteStore.CreateAgentProfile(ctx, actor, profiles.MutationInput{
		DisplayName: "History Agent",
		Persona:     profiles.Persona{Tone: "direct"},
		Activate:    true,
		OverlayReferences: []profiles.OverlayReferenceInput{{
			ReferenceKind: "prompt",
			ReferenceURI:  "prompt://profiles/history",
			Scope:         "profile",
		}},
	})
	if err != nil {
		t.Fatalf("CreateAgentProfile returned error: %v", err)
	}
	updated, err := sqliteStore.UpdateAgentProfile(ctx, actor, created.Profile.ProfileID, profiles.MutationInput{
		DisplayName: "History Agent Updated",
		Persona:     profiles.Persona{Tone: "calm"},
		OverlayReferences: []profiles.OverlayReferenceInput{{
			ReferenceKind: "config",
			ReferenceURI:  "config://profiles/history",
			Scope:         "profile",
		}},
	})
	if err != nil {
		t.Fatalf("UpdateAgentProfile returned error: %v", err)
	}
	rolledBack, err := sqliteStore.RollbackAgentProfile(ctx, actor, created.Profile.ProfileID, profiles.RollbackInput{SourceProfileVersionID: created.Version.ProfileVersionID})
	if err != nil {
		t.Fatalf("RollbackAgentProfile returned error: %v", err)
	}
	if rolledBack.Version.ChangeKind != profiles.ChangeRolledBack || rolledBack.Version.SourceVersionID != created.Version.ProfileVersionID {
		t.Fatalf("unexpected rollback version: %+v", rolledBack.Version)
	}
	if rolledBack.Selection.ProfileVersionID != rolledBack.Version.ProfileVersionID || rolledBack.Selection.SelectionReason != profiles.SelectionRollbackActivated {
		t.Fatalf("rollback should update tenant-default selection to the new rollback version, got %+v", rolledBack.Selection)
	}

	versions, err := sqliteStore.ListAgentProfileVersions(ctx, actor.TenantID, created.Profile.ProfileID, 10)
	if err != nil {
		t.Fatalf("ListAgentProfileVersions returned error: %v", err)
	}
	if len(versions) != 3 || versions[2].ProfileVersionID != created.Version.ProfileVersionID || versions[1].ProfileVersionID != updated.Version.ProfileVersionID {
		t.Fatalf("expected immutable retained history, got %+v", versions)
	}
	if versions[0].ChangeKind != profiles.ChangeRolledBack || versions[0].SourceVersionID != created.Version.ProfileVersionID {
		t.Fatalf("rollback version should be persisted as rolled_back without rewrite ambiguity, got %+v", versions[0])
	}
	detail, found, err := sqliteStore.GetAgentProfileDetail(ctx, actor.TenantID, created.Profile.ProfileID)
	if err != nil || !found {
		t.Fatalf("GetAgentProfileDetail found=%v err=%v", found, err)
	}
	if len(detail.OverlayReferences) != 1 || detail.OverlayReferences[0].ReferenceKind != "prompt" || detail.OverlayReferences[0].ProfileVersionID != rolledBack.Version.ProfileVersionID {
		t.Fatalf("rollback should restore source overlay references on new version, got %+v", detail.OverlayReferences)
	}
}

func TestAgentProfileStoreRetireDoesNotHardDeleteAndDisableNonDefault(t *testing.T) {
	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	ctx := context.Background()
	actor := identity.TenantContext{TenantID: "ten_profile_retire", PrincipalID: "prn_admin", Permissions: []identity.Permission{identity.PermissionProfilesManage, identity.PermissionProfilesInspect}}

	created, err := sqliteStore.CreateAgentProfile(ctx, actor, profiles.MutationInput{DisplayName: "Disposable Agent"})
	if err != nil {
		t.Fatalf("CreateAgentProfile returned error: %v", err)
	}
	disabled, err := sqliteStore.RetireAgentProfile(ctx, actor, created.Profile.ProfileID, profiles.StatusDisabled, profiles.RetirementInput{ReasonCode: "test_disable"})
	if err != nil {
		t.Fatalf("RetireAgentProfile returned error: %v", err)
	}
	if disabled.Profile.Status != profiles.StatusDisabled || disabled.Profile.DisabledAt == nil {
		t.Fatalf("expected disabled profile with timestamp, got %+v", disabled.Profile)
	}
	detail, found, err := sqliteStore.GetAgentProfileDetail(ctx, actor.TenantID, created.Profile.ProfileID)
	if err != nil || !found {
		t.Fatalf("retired profile should remain inspectable, found=%v err=%v", found, err)
	}
	if detail.Profile.Status != profiles.StatusDisabled {
		t.Fatalf("unexpected retired profile detail: %+v", detail.Profile)
	}
}

func TestAgentProfileStoreSeedsDefaultProfileWithLegacyMappingEvidence(t *testing.T) {
	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	ctx := context.Background()

	profile, err := sqliteStore.EnsureDefaultAgentProfile(ctx, "ten_profile_default_seed")
	if err != nil {
		t.Fatalf("EnsureDefaultAgentProfile returned error: %v", err)
	}
	if profile.Status != profiles.StatusActive || profile.ActiveVersionID == "" {
		t.Fatalf("unexpected default profile: %+v", profile)
	}
	if len(profile.LegacyMappingEvidence) != 2 {
		t.Fatalf("expected provider and prompt/config legacy mapping evidence, got %+v", profile.LegacyMappingEvidence)
	}
	for _, evidence := range profile.LegacyMappingEvidence {
		if evidence.MappingState != profiles.OverlayPartial || evidence.RedactionStatus != profiles.RedactionRedacted || evidence.ReasonCode == "" {
			t.Fatalf("unexpected legacy mapping evidence: %+v", evidence)
		}
	}
	detail, found, err := sqliteStore.GetAgentProfileDetail(ctx, "ten_profile_default_seed", profile.ProfileID)
	if err != nil || !found {
		t.Fatalf("GetAgentProfileDetail found=%v err=%v", found, err)
	}
	if len(detail.OverlayReferences) != 2 {
		t.Fatalf("expected explicit legacy overlay references, got %+v", detail.OverlayReferences)
	}
}

func TestAgentProfileStoreValidatesProviderAvailability(t *testing.T) {
	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	ctx := context.Background()
	actor := identity.TenantContext{TenantID: "ten_profile_provider", PrincipalID: "prn_admin", Permissions: []identity.Permission{identity.PermissionProfilesManage, identity.PermissionProfilesInspect}}
	if err := sqliteStore.ReplaceProviderModels(ctx, "codex_managed", []providers.Model{
		{ProviderID: "codex_managed", ModelID: "gpt-5.4", Default: true, Available: true, ReasoningLevels: []string{"medium", "high"}},
		{ProviderID: "codex_managed", ModelID: "gpt-disabled", Available: false},
	}); err != nil {
		t.Fatalf("ReplaceProviderModels returned error: %v", err)
	}
	created, err := sqliteStore.CreateAgentProfile(ctx, actor, profiles.MutationInput{
		DisplayName: "Provider Agent",
		DefaultProviderPreference: profiles.DefaultProviderPreference{
			ProviderID:      "codex_managed",
			Model:           "gpt-5.4",
			ReasoningLevel:  "high",
			ValidationState: profiles.OverlayValid,
		},
	})
	if err != nil {
		t.Fatalf("expected available provider/model to be accepted, got %v", err)
	}
	if _, err := sqliteStore.UpdateAgentProfile(ctx, actor, created.Profile.ProfileID, profiles.MutationInput{DisplayName: "Provider Agent Updated"}); err != nil {
		t.Fatalf("UpdateAgentProfile returned error: %v", err)
	}
	if _, err := sqliteStore.CreateAgentProfile(ctx, actor, profiles.MutationInput{
		DisplayName: "Unavailable Provider Agent",
		DefaultProviderPreference: profiles.DefaultProviderPreference{
			ProviderID:      "codex_managed",
			Model:           "gpt-disabled",
			ValidationState: profiles.OverlayValid,
		},
	}); err == nil {
		t.Fatal("expected unavailable provider model to be rejected")
	}
	if _, err := sqliteStore.CreateAgentProfile(ctx, actor, profiles.MutationInput{
		DisplayName: "Unsupported Reasoning Agent",
		DefaultProviderPreference: profiles.DefaultProviderPreference{
			ProviderID:      "codex_managed",
			Model:           "gpt-5.4",
			ReasoningLevel:  "xhigh",
			ValidationState: profiles.OverlayValid,
		},
	}); err == nil {
		t.Fatal("expected unsupported reasoning level to be rejected")
	}
	if err := sqliteStore.ReplaceProviderModels(ctx, "codex_managed", []providers.Model{
		{ProviderID: "codex_managed", ModelID: "gpt-5.4", Default: true, Available: false, ReasoningLevels: []string{"medium", "high"}},
	}); err != nil {
		t.Fatalf("ReplaceProviderModels returned error: %v", err)
	}
	if _, err := sqliteStore.RollbackAgentProfile(ctx, actor, created.Profile.ProfileID, profiles.RollbackInput{SourceProfileVersionID: created.Version.ProfileVersionID}); err == nil {
		t.Fatal("expected rollback to currently unavailable provider model to fail closed")
	}
}

func TestAgentProfileSchemaMigrationCreatesTables(t *testing.T) {
	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	for _, table := range []string{"agent_profiles", "agent_profile_versions", "agent_profile_active_selections", "agent_profile_overlay_references", "agent_profile_runtime_projections"} {
		var count int
		if err := sqliteStore.db.QueryRow(`SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
			t.Fatalf("query %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("expected table %s to exist", table)
		}
	}
}
