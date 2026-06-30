package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/bindings"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/profiles"
)

// FR-006/FR-033: ResolveBindingSelection applies channel -> integration-account -> tenant
// default precedence within a single snapshot. This proves the account tier is actually wired
// (previously ResolveAccountBinding had no runtime caller) and that an explicit channel
// binding wins over an account default.
func TestResolveBindingSelectionPrecedence(t *testing.T) {
	ctx := context.Background()
	st, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer st.Close()
	actor := identity.TenantContext{TenantID: "ten_prec", PrincipalID: "prn_admin"}

	mk := func(name string) string {
		res, err := st.CreateAgentProfile(ctx, actor, profiles.MutationInput{DisplayName: name, Activate: true})
		if err != nil {
			t.Fatalf("CreateAgentProfile %s: %v", name, err)
		}
		return res.Profile.ProfileID
	}
	tenantDefault := mk("Default")
	accountProfile := mk("AccountProfile")
	channelProfile := mk("ChannelProfile")

	// Seed an integration so the account binding's scope_ref validates.
	now := time.Now().UTC()
	if err := st.UpsertIntegration(ctx, integrations.Resource{
		IntegrationID: "acct_1", TenantID: actor.TenantID, DomainKind: "calendar",
		EnvironmentScope: "test", ReadinessStatus: integrations.ReadinessStatusHealthy,
		CreatedAt: now, UpdatedAt: now, LastTransitionAt: now,
	}); err != nil {
		t.Fatalf("UpsertIntegration: %v", err)
	}

	resolve := func(channelRef, accountRef string) bindings.EffectiveBindingSelection {
		sel, err := st.ResolveBindingSelection(ctx, BindingResolutionParams{
			TenantID: actor.TenantID, ChannelScopeRef: channelRef, AccountScopeRef: accountRef,
			TenantDefaultProfileID: tenantDefault,
		})
		if err != nil {
			t.Fatalf("ResolveBindingSelection: %v", err)
		}
		return sel
	}

	// No bindings -> tenant default.
	if sel := resolve("", ""); sel.SelectedProfileID != tenantDefault || sel.BindingScope != bindings.RuntimeScopeTenantDefault {
		t.Fatalf("expected tenant default, got profile=%s scope=%s", sel.SelectedProfileID, sel.BindingScope)
	}

	// Account default applies when no channel binding exists.
	if _, _, err := st.CreateBindingRule(ctx, actor, bindings.CreateBindingRequest{ScopeKind: bindings.ScopeIntegrationAccount, ScopeRef: "acct_1", SelectedProfileID: accountProfile}); err != nil {
		t.Fatalf("create account binding: %v", err)
	}
	if sel := resolve("", "acct_1"); sel.SelectedProfileID != accountProfile || sel.BindingScope != bindings.RuntimeScopeIntegrationAccount {
		t.Fatalf("expected account tier, got profile=%s scope=%s", sel.SelectedProfileID, sel.BindingScope)
	}

	// Channel binding wins over the account default.
	if _, _, err := st.CreateBindingRule(ctx, actor, bindings.CreateBindingRequest{ScopeKind: bindings.ScopeChannel, ScopeRef: "chan_1", SelectedProfileID: channelProfile}); err != nil {
		t.Fatalf("create channel binding: %v", err)
	}
	if sel := resolve("chan_1", "acct_1"); sel.SelectedProfileID != channelProfile || sel.BindingScope != bindings.RuntimeScopeChannel {
		t.Fatalf("expected channel tier to win, got profile=%s scope=%s", sel.SelectedProfileID, sel.BindingScope)
	}

	// Account default still applies for a different channel with no channel binding.
	if sel := resolve("chan_other", "acct_1"); sel.SelectedProfileID != accountProfile || sel.BindingScope != bindings.RuntimeScopeIntegrationAccount {
		t.Fatalf("expected account tier for unbound channel, got profile=%s scope=%s", sel.SelectedProfileID, sel.BindingScope)
	}
}
