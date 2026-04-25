package identity

import (
	"context"
	"errors"
	"testing"
)

func TestBootstrapLocalCreatesPersonalTenantAndTokenGrant(t *testing.T) {
	store := newManagerMemoryStore()
	manager := NewManager(store)

	principal, tenant, err := manager.BootstrapLocal(context.Background(), []string{"tok_1"})
	if err != nil {
		t.Fatalf("BootstrapLocal returned error: %v", err)
	}
	if principal.Status != StatusActive || tenant.TenantKind != TenantKindPersonal {
		t.Fatalf("unexpected bootstrap result principal=%+v tenant=%+v", principal, tenant)
	}
	grants, err := store.ListTokenTenantGrants(context.Background(), "tok_1")
	if err != nil {
		t.Fatalf("ListTokenTenantGrants returned error: %v", err)
	}
	if len(grants) != 1 || grants[0].TenantID != tenant.TenantID || !grants[0].IsDefault {
		t.Fatalf("unexpected token grants: %+v", grants)
	}
}

func TestManagerOrganizationMembershipLifecycle(t *testing.T) {
	store := newManagerMemoryStore()
	manager := NewManager(store)
	ctx := context.Background()
	principal, tenant, err := manager.BootstrapLocal(ctx, []string{"tok_owner"})
	if err != nil {
		t.Fatalf("BootstrapLocal returned error: %v", err)
	}
	actor := TenantContext{PrincipalID: principal.PrincipalID, TokenID: "tok_owner", TenantID: tenant.TenantID, Role: RoleOwner, Permissions: PermissionsForRole(RoleOwner, StatusActive)}
	invited := Principal{PrincipalID: "prn_invited", PrincipalKind: PrincipalKindUser, DisplayName: "Invited", Status: StatusActive, DefaultTenantID: tenant.TenantID}
	store.principals[invited.PrincipalID] = invited

	org, ownerMembership, err := manager.CreateOrganizationTenant(ctx, actor, "Acme")
	if err != nil {
		t.Fatalf("CreateOrganizationTenant returned error: %v", err)
	}
	if org.TenantKind != TenantKindOrganization || ownerMembership.Role != RoleOwner || ownerMembership.Status != StatusActive {
		t.Fatalf("unexpected organization result org=%+v membership=%+v", org, ownerMembership)
	}
	orgActor := TenantContext{PrincipalID: principal.PrincipalID, TokenID: "tok_owner", TenantID: org.TenantID, Role: RoleOwner, Permissions: PermissionsForRole(RoleOwner, StatusActive)}
	invitation, err := manager.CreateInvitation(ctx, orgActor, CreateInvitationInput{TenantID: org.TenantID, InvitedPrincipalID: invited.PrincipalID, Role: RoleAdmin})
	if err != nil {
		t.Fatalf("CreateInvitation returned error: %v", err)
	}
	membership, err := manager.AcceptInvitation(ctx, invited.PrincipalID, invitation.InvitationID)
	if err != nil {
		t.Fatalf("AcceptInvitation returned error: %v", err)
	}
	if membership.Role != RoleAdmin || membership.Status != StatusActive {
		t.Fatalf("unexpected accepted membership: %+v", membership)
	}
	updated, err := manager.UpdateMembershipRole(ctx, orgActor, org.TenantID, membership.MembershipID, RoleViewer)
	if err != nil {
		t.Fatalf("UpdateMembershipRole returned error: %v", err)
	}
	if updated.Role != RoleViewer {
		t.Fatalf("expected viewer role, got %+v", updated)
	}
	removed, err := manager.RemoveMembership(ctx, orgActor, org.TenantID, membership.MembershipID)
	if err != nil {
		t.Fatalf("RemoveMembership returned error: %v", err)
	}
	if removed.Status != StatusRemoved {
		t.Fatalf("expected removed membership, got %+v", removed)
	}

	if _, err := manager.RemoveMembership(ctx, orgActor, org.TenantID, ownerMembership.MembershipID); !errors.Is(err, ErrOwnerInvariant) {
		t.Fatalf("expected last-owner protection, got %v", err)
	}
}

func TestManagerFailsClosedWhenMembershipAuditFails(t *testing.T) {
	store := newManagerMemoryStore()
	manager := NewManager(store)
	ctx := context.Background()
	principal, tenant, err := manager.BootstrapLocal(ctx, []string{"tok_owner"})
	if err != nil {
		t.Fatalf("BootstrapLocal returned error: %v", err)
	}
	store.auditErr = errors.New("audit store down")
	actor := TenantContext{PrincipalID: principal.PrincipalID, TokenID: "tok_owner", TenantID: tenant.TenantID, Role: RoleOwner, Permissions: PermissionsForRole(RoleOwner, StatusActive)}
	_, _, err = manager.CreateOrganizationTenant(ctx, actor, "Denied Org")
	if !errors.Is(err, ErrAuditWriteFailed) {
		t.Fatalf("expected ErrAuditWriteFailed, got %v", err)
	}
}

func TestManagerReplacesTokenGrantsWithoutWidening(t *testing.T) {
	store := newManagerMemoryStore()
	manager := NewManager(store)
	ctx := context.Background()
	principal, personal, err := manager.BootstrapLocal(ctx, []string{"tok_1"})
	if err != nil {
		t.Fatalf("BootstrapLocal returned error: %v", err)
	}
	actor := TenantContext{PrincipalID: principal.PrincipalID, TokenID: "tok_1", TenantID: personal.TenantID, Role: RoleOwner, Permissions: PermissionsForRole(RoleOwner, StatusActive)}
	org, _, err := manager.CreateOrganizationTenant(ctx, actor, "Acme")
	if err != nil {
		t.Fatalf("CreateOrganizationTenant returned error: %v", err)
	}
	grants, err := manager.ReplaceTokenTenantGrants(ctx, actor, "tok_1", []string{org.TenantID}, org.TenantID)
	if err != nil {
		t.Fatalf("ReplaceTokenTenantGrants returned error: %v", err)
	}
	if len(grants) != 1 || grants[0].TenantID != org.TenantID || !grants[0].IsDefault {
		t.Fatalf("unexpected replacement grants: %+v", grants)
	}
	oldGrants, err := store.ListTokenTenantGrants(ctx, "tok_1")
	if err != nil {
		t.Fatalf("ListTokenTenantGrants returned error: %v", err)
	}
	for _, grant := range oldGrants {
		if grant.TenantID == personal.TenantID && grant.Status == StatusActive {
			t.Fatalf("expected old personal grant to be revoked, got %+v", oldGrants)
		}
	}
	if _, err := manager.ReplaceTokenTenantGrants(ctx, actor, "tok_1", []string{"ten_missing"}, "ten_missing"); !errors.Is(err, ErrTokenGrantInvalid) {
		t.Fatalf("expected invalid grant denial, got %v", err)
	}
}

type managerMemoryStore struct {
	*memoryStore
	invitations map[string]TenantInvitation
	audits      []TenantAuditEvent
}

func newManagerMemoryStore() *managerMemoryStore {
	return &managerMemoryStore{
		memoryStore: newMemoryStore(),
		invitations: make(map[string]TenantInvitation),
	}
}

func (s *managerMemoryStore) UpsertTenant(_ context.Context, tenant Tenant) error {
	s.tenants[tenant.TenantID] = tenant
	return nil
}

func (s *managerMemoryStore) UpsertPrincipal(_ context.Context, principal Principal) error {
	s.principals[principal.PrincipalID] = principal
	return nil
}

func (s *managerMemoryStore) UpsertMembership(_ context.Context, membership Membership) error {
	s.memberships[membership.MembershipID] = membership
	return nil
}

func (s *managerMemoryStore) UpsertTenantInvitation(_ context.Context, invitation TenantInvitation) error {
	s.invitations[invitation.InvitationID] = invitation
	return nil
}

func (s *managerMemoryStore) UpsertTokenTenantGrant(_ context.Context, grant TokenTenantGrant) error {
	s.grants[grant.GrantID] = grant
	return nil
}

func (s *managerMemoryStore) ListTenants(_ context.Context, _ TenantFilter) ([]Tenant, error) {
	items := make([]Tenant, 0, len(s.tenants))
	for _, item := range s.tenants {
		items = append(items, item)
	}
	return items, nil
}

func (s *managerMemoryStore) ListPrincipals(_ context.Context, _ PrincipalFilter) ([]Principal, error) {
	items := make([]Principal, 0, len(s.principals))
	for _, item := range s.principals {
		items = append(items, item)
	}
	return items, nil
}

func (s *managerMemoryStore) ListTenantInvitations(_ context.Context, _ InvitationFilter) ([]TenantInvitation, error) {
	items := make([]TenantInvitation, 0, len(s.invitations))
	for _, item := range s.invitations {
		items = append(items, item)
	}
	return items, nil
}

func (s *managerMemoryStore) ListTokenAuthorities(_ context.Context) ([]TokenAuthority, error) {
	return nil, nil
}

func (s *managerMemoryStore) AppendTenantAuditEvent(_ context.Context, event TenantAuditEvent) (TenantAuditEvent, error) {
	if s.auditErr != nil {
		return TenantAuditEvent{}, s.auditErr
	}
	s.audits = append(s.audits, event)
	return event, nil
}
