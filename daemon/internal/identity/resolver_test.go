package identity

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"
)

func TestResolverDefaultExplicitAndDeniedTenants(t *testing.T) {
	store := newMemoryStore()
	now := time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC)
	principal := Principal{PrincipalID: "prn_1", Status: StatusActive, DefaultTenantID: "ten_default"}
	store.principals[principal.PrincipalID] = principal
	for _, tenantID := range []string{"ten_default", "ten_other"} {
		store.tenants[tenantID] = Tenant{TenantID: tenantID, Status: StatusActive}
		store.memberships["mem_"+tenantID] = Membership{MembershipID: "mem_" + tenantID, TenantID: tenantID, PrincipalID: principal.PrincipalID, Role: RoleOwner, Status: StatusActive}
		store.grants["grant_"+tenantID] = TokenTenantGrant{GrantID: "grant_" + tenantID, TokenID: "tok_1", TenantID: tenantID, Status: StatusActive}
	}
	resolver := NewResolver(store)
	resolver.now = func() time.Time { return now }
	token := TokenAuthority{TokenID: "tok_1", PrincipalID: principal.PrincipalID, DefaultTenantID: "ten_default", Status: StatusActive}

	defaultCtx, err := resolver.Resolve(context.Background(), token, "")
	if err != nil {
		t.Fatalf("Resolve default returned error: %v", err)
	}
	if defaultCtx.TenantID != "ten_default" || defaultCtx.TenantSource != TenantSourceDefault {
		t.Fatalf("unexpected default context: %+v", defaultCtx)
	}

	explicitCtx, err := resolver.Resolve(context.Background(), token, "ten_other")
	if err != nil {
		t.Fatalf("Resolve explicit returned error: %v", err)
	}
	if explicitCtx.TenantID != "ten_other" || explicitCtx.TenantSource != TenantSourceExplicitHeader {
		t.Fatalf("unexpected explicit context: %+v", explicitCtx)
	}

	_, err = resolver.Resolve(context.Background(), token, "ten_missing")
	if !errors.Is(err, ErrTenantAccessDenied) {
		t.Fatalf("expected ErrTenantAccessDenied, got %v", err)
	}
}

func TestResolverDeniesLifecycleAndGrantFailures(t *testing.T) {
	store := newMemoryStore()
	store.principals["prn_1"] = Principal{PrincipalID: "prn_1", Status: StatusDisabled, DefaultTenantID: "ten_1"}
	store.tenants["ten_1"] = Tenant{TenantID: "ten_1", Status: StatusActive}
	store.memberships["mem_1"] = Membership{MembershipID: "mem_1", TenantID: "ten_1", PrincipalID: "prn_1", Role: RoleOwner, Status: StatusActive}
	store.grants["grant_1"] = TokenTenantGrant{GrantID: "grant_1", TokenID: "tok_1", TenantID: "ten_1", Status: StatusActive}
	resolver := NewResolver(store)

	_, err := resolver.Resolve(context.Background(), TokenAuthority{TokenID: "tok_1", PrincipalID: "prn_1", Status: StatusActive}, "ten_1")
	if !errors.Is(err, ErrTenantAccessDenied) {
		t.Fatalf("expected disabled principal denial, got %v", err)
	}

	store.principals["prn_1"] = Principal{PrincipalID: "prn_1", Status: StatusActive, DefaultTenantID: "ten_1"}
	store.memberships["mem_1"] = Membership{MembershipID: "mem_1", TenantID: "ten_1", PrincipalID: "prn_1", Role: RoleOwner, Status: StatusRemoved}
	_, err = resolver.Resolve(context.Background(), TokenAuthority{TokenID: "tok_1", PrincipalID: "prn_1", Status: StatusActive}, "ten_1")
	if !errors.Is(err, ErrTenantAccessDenied) {
		t.Fatalf("expected removed membership denial, got %v", err)
	}
}

func TestResolverUsesBoundedStoreLookupsForLargeMembershipSet(t *testing.T) {
	store := newMemoryStore()
	store.principals["prn_1"] = Principal{PrincipalID: "prn_1", Status: StatusActive, DefaultTenantID: "ten_199"}
	for idx := 0; idx < 250; idx++ {
		tenantID := "ten_" + strconv.Itoa(idx)
		store.tenants[tenantID] = Tenant{TenantID: tenantID, TenantKind: TenantKindOrganization, Status: StatusActive}
		store.memberships["mem_"+tenantID] = Membership{MembershipID: "mem_" + tenantID, TenantID: tenantID, PrincipalID: "prn_1", Role: RoleViewer, Status: StatusActive}
		store.grants["grant_"+tenantID] = TokenTenantGrant{GrantID: "grant_" + tenantID, TokenID: "tok_1", TenantID: tenantID, Status: StatusActive}
	}
	counting := &countingResolverStore{memoryStore: store}
	resolver := NewResolver(counting)

	tenantContext, err := resolver.Resolve(context.Background(), TokenAuthority{TokenID: "tok_1", PrincipalID: "prn_1", DefaultTenantID: "ten_199", Status: StatusActive}, "")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if tenantContext.TenantID != "ten_199" {
		t.Fatalf("expected default tenant ten_199, got %+v", tenantContext)
	}
	if counting.getPrincipalCalls != 1 || counting.getTenantCalls != 1 || counting.listMembershipCalls != 1 || counting.listGrantCalls != 1 {
		t.Fatalf("expected bounded resolver store calls, got principals=%d tenants=%d memberships=%d grants=%d", counting.getPrincipalCalls, counting.getTenantCalls, counting.listMembershipCalls, counting.listGrantCalls)
	}
}

type memoryStore struct {
	tenants     map[string]Tenant
	principals  map[string]Principal
	memberships map[string]Membership
	grants      map[string]TokenTenantGrant
	auditErr    error
}

type countingResolverStore struct {
	*memoryStore
	getPrincipalCalls   int
	getTenantCalls      int
	listMembershipCalls int
	listGrantCalls      int
}

func (s *countingResolverStore) GetPrincipal(ctx context.Context, principalID string) (Principal, bool, error) {
	s.getPrincipalCalls++
	return s.memoryStore.GetPrincipal(ctx, principalID)
}

func (s *countingResolverStore) GetTenant(ctx context.Context, tenantID string) (Tenant, bool, error) {
	s.getTenantCalls++
	return s.memoryStore.GetTenant(ctx, tenantID)
}

func (s *countingResolverStore) ListMemberships(ctx context.Context, filter MembershipFilter) ([]Membership, error) {
	s.listMembershipCalls++
	return s.memoryStore.ListMemberships(ctx, filter)
}

func (s *countingResolverStore) ListTokenTenantGrants(ctx context.Context, tokenID string) ([]TokenTenantGrant, error) {
	s.listGrantCalls++
	return s.memoryStore.ListTokenTenantGrants(ctx, tokenID)
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		tenants:     make(map[string]Tenant),
		principals:  make(map[string]Principal),
		memberships: make(map[string]Membership),
		grants:      make(map[string]TokenTenantGrant),
	}
}

func (s *memoryStore) GetPrincipal(_ context.Context, principalID string) (Principal, bool, error) {
	item, ok := s.principals[principalID]
	return item, ok, nil
}

func (s *memoryStore) GetTenant(_ context.Context, tenantID string) (Tenant, bool, error) {
	item, ok := s.tenants[tenantID]
	return item, ok, nil
}

func (s *memoryStore) ListMemberships(_ context.Context, filter MembershipFilter) ([]Membership, error) {
	items := make([]Membership, 0)
	for _, item := range s.memberships {
		if filter.TenantID != "" && item.TenantID != filter.TenantID {
			continue
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *memoryStore) ListTokenTenantGrants(_ context.Context, tokenID string) ([]TokenTenantGrant, error) {
	items := make([]TokenTenantGrant, 0)
	for _, item := range s.grants {
		if item.TokenID == tokenID {
			items = append(items, item)
		}
	}
	return items, nil
}
