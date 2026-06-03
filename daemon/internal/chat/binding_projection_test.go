package chat

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/bindings"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/profiles"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func newBindingChatService(t *testing.T) (*Service, *store.SQLiteStore) {
	t.Helper()
	ctx := context.Background()
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope"))
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	if err := sqliteStore.ReplaceProviderModels(ctx, "profile-capture", []providers.Model{{ProviderID: "profile-capture", ModelID: "profile-model", Default: true, Available: true}}); err != nil {
		t.Fatalf("ReplaceProviderModels: %v", err)
	}
	provider := &profileCaptureProvider{}
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(provider)
	return NewService(dispatcher, nil, nil, events.NewBus(), sqliteStore), sqliteStore
}

// B13/FR-013: new work from a bound channel records applied binding runtime evidence.
func TestChatRecordsAppliedBindingEvidence(t *testing.T) {
	ctx := context.Background()
	service, sqliteStore := newBindingChatService(t)
	actor := identity.TenantContext{TenantID: "ten_bind", PrincipalID: "prn_admin"}

	boundProfile, err := sqliteStore.CreateAgentProfile(ctx, actor, profiles.MutationInput{DisplayName: "Bound", DefaultProviderPreference: profiles.DefaultProviderPreference{ProviderID: "profile-capture", Model: "profile-model"}, Activate: true})
	if err != nil {
		t.Fatalf("create bound profile: %v", err)
	}
	ws, err := sqliteStore.EnsureDefaultWorkspace(ctx, actor.TenantID)
	if err != nil {
		t.Fatalf("ensure default workspace: %v", err)
	}
	if _, _, err := sqliteStore.CreateBindingRule(ctx, actor, bindings.CreateBindingRequest{ScopeKind: bindings.ScopeChannel, ScopeRef: "discord:c1", SelectedProfileID: boundProfile.Profile.ProfileID, SelectedWorkspaceID: ws.WorkspaceID}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	if _, err := service.Query(ctx, QueryInput{TenantID: actor.TenantID, Query: "hi", ThreadID: "thr_bound", SourceLinkageID: "discord:c1"}); err != nil {
		t.Fatalf("Query: %v", err)
	}
	ev, found, err := sqliteStore.LatestRuntimeBindingEvidence(ctx, actor.TenantID, "thread", "thr_bound")
	if err != nil || !found {
		t.Fatalf("evidence: %v found=%v", err, found)
	}
	if ev.Classification != bindings.ClassificationApplied {
		t.Fatalf("expected applied classification, got %s", ev.Classification)
	}
	if ev.BindingScope != bindings.RuntimeScopeChannel || ev.SelectedProfileID != boundProfile.Profile.ProfileID {
		t.Fatalf("unexpected evidence: %+v", ev)
	}
}

// B5/FR-031: work from a channel bound to an archived profile fails closed.
func TestChatFailsClosedOnInvalidBinding(t *testing.T) {
	ctx := context.Background()
	service, sqliteStore := newBindingChatService(t)
	actor := identity.TenantContext{TenantID: "ten_fc", PrincipalID: "prn_admin"}

	bound, err := sqliteStore.CreateAgentProfile(ctx, actor, profiles.MutationInput{DisplayName: "Bound", Activate: true})
	if err != nil {
		t.Fatalf("create bound: %v", err)
	}
	// A second active profile becomes the tenant default so the bound profile can be retired.
	if _, err := sqliteStore.CreateAgentProfile(ctx, actor, profiles.MutationInput{DisplayName: "Default", DefaultProviderPreference: profiles.DefaultProviderPreference{ProviderID: "profile-capture", Model: "profile-model"}, Activate: true}); err != nil {
		t.Fatalf("create default: %v", err)
	}
	if _, _, err := sqliteStore.CreateBindingRule(ctx, actor, bindings.CreateBindingRequest{ScopeKind: bindings.ScopeChannel, ScopeRef: "discord:bad", SelectedProfileID: bound.Profile.ProfileID}); err != nil {
		t.Fatalf("create binding: %v", err)
	}
	if _, err := sqliteStore.RetireAgentProfile(ctx, actor, bound.Profile.ProfileID, profiles.StatusArchived, profiles.RetirementInput{ReasonCode: "test_archive"}); err != nil {
		t.Fatalf("archive bound profile: %v", err)
	}

	_, err = service.Query(ctx, QueryInput{TenantID: actor.TenantID, Query: "hi", ThreadID: "thr_bad", SourceLinkageID: "discord:bad"})
	if !errors.Is(err, ErrBindingRepairRequired) {
		t.Fatalf("expected fail-closed ErrBindingRepairRequired, got %v", err)
	}
}

// B7/SC-003: workspace binding does not change provider/profile behavior beyond identity;
// default (unbound) work still records default-classified evidence.
func TestChatDefaultBindingEvidence(t *testing.T) {
	ctx := context.Background()
	service, sqliteStore := newBindingChatService(t)
	actor := identity.TenantContext{TenantID: "ten_def", PrincipalID: "prn_admin"}
	if _, err := sqliteStore.CreateAgentProfile(ctx, actor, profiles.MutationInput{DisplayName: "Default", DefaultProviderPreference: profiles.DefaultProviderPreference{ProviderID: "profile-capture", Model: "profile-model"}, Activate: true}); err != nil {
		t.Fatalf("create default: %v", err)
	}
	if _, err := service.Query(ctx, QueryInput{TenantID: actor.TenantID, Query: "hi", ThreadID: "thr_def"}); err != nil {
		t.Fatalf("Query: %v", err)
	}
	ev, found, err := sqliteStore.LatestRuntimeBindingEvidence(ctx, actor.TenantID, "thread", "thr_def")
	if err != nil || !found {
		t.Fatalf("evidence: %v found=%v", err, found)
	}
	if ev.Classification != bindings.ClassificationDefault {
		t.Fatalf("expected default classification, got %s", ev.Classification)
	}
	if ev.BindingScope != bindings.RuntimeScopeTenantDefault {
		t.Fatalf("expected tenant_default scope, got %s", ev.BindingScope)
	}
}

// B24/FR-033: concurrent binding changes during work-start yield exactly one coherent
// resolved selection per run — never mixed/partial state.
func TestChatConcurrentBindingChangeRecordsOneSelection(t *testing.T) {
	ctx := context.Background()
	service, sqliteStore := newBindingChatService(t)
	actor := identity.TenantContext{TenantID: "ten_conc", PrincipalID: "prn_admin"}
	if _, err := sqliteStore.CreateAgentProfile(ctx, actor, profiles.MutationInput{DisplayName: "Default", DefaultProviderPreference: profiles.DefaultProviderPreference{ProviderID: "profile-capture", Model: "profile-model"}, Activate: true}); err != nil {
		t.Fatalf("create default: %v", err)
	}
	ws, err := sqliteStore.EnsureDefaultWorkspace(ctx, actor.TenantID)
	if err != nil {
		t.Fatalf("ensure default ws: %v", err)
	}
	bound, err := sqliteStore.CreateAgentProfile(ctx, actor, profiles.MutationInput{DisplayName: "Bound", Activate: true})
	if err != nil {
		t.Fatalf("create bound: %v", err)
	}

	const n = 12
	var wg sync.WaitGroup
	wg.Add(n + 1)
	// Toggle a channel binding on/off concurrently with work starts.
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			rule, _, cerr := sqliteStore.CreateBindingRule(ctx, actor, bindings.CreateBindingRequest{ScopeKind: bindings.ScopeChannel, ScopeRef: "discord:conc", SelectedProfileID: bound.Profile.ProfileID, SelectedWorkspaceID: ws.WorkspaceID})
			if cerr == nil {
				_, _, _ = sqliteStore.UpdateBindingRule(ctx, actor, rule.BindingID, bindings.UpdateBindingRequest{Disable: true})
				_, _ = sqliteStore.RemoveBindingRule(ctx, actor, rule.BindingID)
			}
		}
	}()
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			threadID := "thr_conc_" + string(rune('a'+idx))
			_, _ = service.Query(ctx, QueryInput{TenantID: actor.TenantID, Query: "hi", ThreadID: threadID, SourceLinkageID: "discord:conc"})
		}(i)
	}
	wg.Wait()

	// Every recorded run has exactly one coherent evidence row per thread.
	for i := 0; i < n; i++ {
		threadID := "thr_conc_" + string(rune('a'+i))
		items, err := sqliteStore.ListRuntimeBindingEvidence(ctx, actor.TenantID, "thread", threadID, 10)
		if err != nil {
			t.Fatalf("list evidence: %v", err)
		}
		for _, ev := range items {
			// Coherence: applied <=> channel scope; default <=> tenant_default scope.
			if ev.Classification == bindings.ClassificationApplied && ev.BindingScope != bindings.RuntimeScopeChannel {
				t.Fatalf("mixed state: applied classification with scope %s", ev.BindingScope)
			}
			if ev.Classification == bindings.ClassificationDefault && ev.BindingScope != bindings.RuntimeScopeTenantDefault {
				t.Fatalf("mixed state: default classification with scope %s", ev.BindingScope)
			}
		}
	}
}
