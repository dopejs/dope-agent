package store

import (
	"context"
	"errors"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/bindings"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/profiles"
)

func seedActiveProfile(t *testing.T, s *SQLiteStore, tenantID string) profiles.AgentProfile {
	t.Helper()
	p, err := s.EnsureDefaultAgentProfile(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("EnsureDefaultAgentProfile: %v", err)
	}
	return p
}

// B1/B18: binding CRUD + validation rejects an unavailable profile reference.
func TestCreateBindingRule_LifecycleAndValidation(t *testing.T) {
	s := newBindingTestStore(t)
	ctx := context.Background()
	actor := identity.TenantContext{TenantID: "ten_b", PrincipalID: "prn_admin"}
	profile := seedActiveProfile(t, s, "ten_b")
	ws, err := s.EnsureDefaultWorkspace(ctx, "ten_b")
	if err != nil {
		t.Fatalf("EnsureDefaultWorkspace: %v", err)
	}

	rule, _, err := s.CreateBindingRule(ctx, actor, bindings.CreateBindingRequest{
		ScopeKind:           bindings.ScopeChannel,
		ScopeRef:            "discord:chan_1",
		SelectedProfileID:   profile.ProfileID,
		SelectedWorkspaceID: ws.WorkspaceID,
	})
	if err != nil {
		t.Fatalf("CreateBindingRule: %v", err)
	}
	if rule.Status != bindings.BindingActive || rule.RepairStatus != bindings.RepairHealthy {
		t.Fatalf("unexpected rule: %+v", rule)
	}

	// Invalid profile reference is rejected with a safe reason.
	_, _, err = s.CreateBindingRule(ctx, actor, bindings.CreateBindingRequest{
		ScopeKind:         bindings.ScopeChannel,
		ScopeRef:          "discord:chan_2",
		SelectedProfileID: "prof_missing",
	})
	if !errors.Is(err, bindings.ErrInvalidBinding) || bindings.ValidationReasonCode(err) != "selected_profile_unavailable" {
		t.Fatalf("expected selected_profile_unavailable, got %v", err)
	}

	// Duplicate active binding at same scope is rejected.
	_, _, err = s.CreateBindingRule(ctx, actor, bindings.CreateBindingRequest{ScopeKind: bindings.ScopeChannel, ScopeRef: "discord:chan_1", SelectedProfileID: profile.ProfileID})
	if !errors.Is(err, bindings.ErrInvalidBinding) {
		t.Fatalf("expected duplicate rejection, got %v", err)
	}

	got, found, err := s.GetBindingRule(ctx, "ten_b", rule.BindingID)
	if err != nil || !found {
		t.Fatalf("GetBindingRule: %v found=%v", err, found)
	}
	if got.SelectedProfileID != profile.ProfileID {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

// B3: precedence resolution against the store (channel binding resolves for new work).
func TestResolveChannelBinding(t *testing.T) {
	s := newBindingTestStore(t)
	ctx := context.Background()
	actor := identity.TenantContext{TenantID: "ten_r", PrincipalID: "prn_admin"}
	profile := seedActiveProfile(t, s, "ten_r")
	rule, _, err := s.CreateBindingRule(ctx, actor, bindings.CreateBindingRequest{ScopeKind: bindings.ScopeChannel, ScopeRef: "tg:chan_9", SelectedProfileID: profile.ProfileID})
	if err != nil {
		t.Fatalf("CreateBindingRule: %v", err)
	}
	resolved, err := s.ResolveChannelBinding(ctx, "ten_r", "tg:chan_9")
	if err != nil || resolved == nil {
		t.Fatalf("ResolveChannelBinding: %v rule=%v", err, resolved)
	}
	if resolved.BindingID != rule.BindingID {
		t.Fatalf("resolved wrong binding")
	}
	// Disabling makes it absent for resolution.
	if _, _, err := s.UpdateBindingRule(ctx, actor, rule.BindingID, bindings.UpdateBindingRequest{Disable: true}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	resolved, err = s.ResolveChannelBinding(ctx, "ten_r", "tg:chan_9")
	if err != nil {
		t.Fatalf("resolve after disable: %v", err)
	}
	if resolved != nil {
		t.Fatalf("disabled binding must not resolve")
	}
}

// B17/FR-022: a binding referencing a retired workspace is marked needing repair.
func TestRepairStatus_StaleWorkspace(t *testing.T) {
	s := newBindingTestStore(t)
	ctx := context.Background()
	actor := identity.TenantContext{TenantID: "ten_rep", PrincipalID: "prn_admin"}
	profile := seedActiveProfile(t, s, "ten_rep")
	ws, _, err := s.CreateWorkspace(ctx, actor, "Temp")
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	rule, _, err := s.CreateBindingRule(ctx, actor, bindings.CreateBindingRequest{ScopeKind: bindings.ScopeChannel, ScopeRef: "slack:chan_1", SelectedProfileID: profile.ProfileID, SelectedWorkspaceID: ws.WorkspaceID})
	if err != nil {
		t.Fatalf("CreateBindingRule: %v", err)
	}
	if _, _, err := s.UpdateWorkspaceStatus(ctx, actor, ws.WorkspaceID, bindings.WorkspaceDisabled); err != nil {
		t.Fatalf("disable workspace: %v", err)
	}
	got, found, err := s.GetBindingRule(ctx, "ten_rep", rule.BindingID)
	if err != nil || !found {
		t.Fatalf("GetBindingRule: %v", err)
	}
	if got.RepairStatus != bindings.RepairNeedsRepair {
		t.Fatalf("expected needs_repair, got %s", got.RepairStatus)
	}
}

// B8/B9/B10: capability visibility resolution through the store, strictest-wins.
func TestCapabilityVisibility_StoreResolution(t *testing.T) {
	s := newBindingTestStore(t)
	ctx := context.Background()
	actor := identity.TenantContext{TenantID: "ten_cap", PrincipalID: "prn_admin"}
	ws, err := s.EnsureDefaultWorkspace(ctx, "ten_cap")
	if err != nil {
		t.Fatalf("ensure default: %v", err)
	}
	profile := seedActiveProfile(t, s, "ten_cap")
	if _, _, err := s.SetCapabilityVisibility(ctx, actor, bindings.SetVisibilityRequest{ScopeKind: bindings.VisibilityScopeWorkspace, ScopeRef: ws.WorkspaceID, CapabilityID: "tool.shell", Visibility: bindings.VisibilityHidden}); err != nil {
		t.Fatalf("SetCapabilityVisibility: %v", err)
	}
	// Upsert keeps a single policy row.
	if _, _, err := s.SetCapabilityVisibility(ctx, actor, bindings.SetVisibilityRequest{ScopeKind: bindings.VisibilityScopeWorkspace, ScopeRef: ws.WorkspaceID, CapabilityID: "tool.shell", Visibility: bindings.VisibilityDisabled}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	list, err := s.ListCapabilityVisibility(ctx, "ten_cap", bindings.VisibilityScopeWorkspace, ws.WorkspaceID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Visibility != bindings.VisibilityDisabled {
		t.Fatalf("expected single disabled policy, got %+v", list)
	}
	decision, err := s.EffectiveCapabilityVisibility(ctx, "ten_cap", profile.ProfileID, ws.WorkspaceID, "tool.shell", nil)
	if err != nil {
		t.Fatalf("EffectiveCapabilityVisibility: %v", err)
	}
	if decision.Executable || decision.Offered || decision.Effective != bindings.EffectiveDisabled {
		t.Fatalf("disabled capability must not be offered/executable: %+v", decision)
	}
}

// B2: tenant isolation — a binding created in one tenant is invisible to another.
func TestBindingTenantIsolation(t *testing.T) {
	s := newBindingTestStore(t)
	ctx := context.Background()
	actor := identity.TenantContext{TenantID: "ten_one", PrincipalID: "prn_admin"}
	profile := seedActiveProfile(t, s, "ten_one")
	rule, _, err := s.CreateBindingRule(ctx, actor, bindings.CreateBindingRequest{ScopeKind: bindings.ScopeChannel, ScopeRef: "discord:x", SelectedProfileID: profile.ProfileID})
	if err != nil {
		t.Fatalf("CreateBindingRule: %v", err)
	}
	_, found, err := s.GetBindingRule(ctx, "ten_two", rule.BindingID)
	if err != nil {
		t.Fatalf("cross-tenant get: %v", err)
	}
	if found {
		t.Fatalf("binding must not be visible cross-tenant")
	}
	items, err := s.ListBindingRules(ctx, "ten_two", 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no bindings for other tenant, got %d", len(items))
	}
}

// B12/FR-011: when the audit write cannot be recorded, the binding mutation fails closed
// and leaves no binding state behind.
func TestCreateBindingRule_AuditFailureRollsBack(t *testing.T) {
	s := newBindingTestStore(t)
	ctx := context.Background()
	actor := identity.TenantContext{TenantID: "ten_audit", PrincipalID: "prn_admin"}
	profile := seedActiveProfile(t, s, "ten_audit")
	// Force the in-transaction audit insert to fail by removing the audit table.
	if _, err := s.db.ExecContext(ctx, `DROP TABLE binding_audit_events`); err != nil {
		t.Fatalf("drop audit table: %v", err)
	}
	_, _, err := s.CreateBindingRule(ctx, actor, bindings.CreateBindingRequest{ScopeKind: bindings.ScopeChannel, ScopeRef: "discord:audit", SelectedProfileID: profile.ProfileID})
	if err == nil {
		t.Fatalf("expected create to fail when audit cannot be written")
	}
	items, listErr := s.ListBindingRules(ctx, "ten_audit", 50)
	if listErr != nil {
		t.Fatalf("list: %v", listErr)
	}
	if len(items) != 0 {
		t.Fatalf("binding must not persist when audit write fails, got %d", len(items))
	}
}
