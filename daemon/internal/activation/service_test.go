package activation

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

var (
	_ StateStore         = (*fakeStateStore)(nil)
	_ IdentityRepository = (*fakeIdentityRepository)(nil)
	_ BillingProjector   = (*fakeBillingProjector)(nil)
	_ ChatRunner         = (*fakeChatRunner)(nil)
	_ AuditSink          = (*fakeAuditSink)(nil)
)

func TestNewServiceDefaultsBoundaryConfiguration(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	svc := NewService(Dependencies{
		StateStore:       &fakeStateStore{},
		Identity:         &fakeIdentityRepository{},
		Billing:          &fakeBillingProjector{},
		Chat:             &fakeChatRunner{},
		Audit:            &fakeAuditSink{},
		Now:              func() time.Time { return now },
		EnvironmentScope: "prod",
		Hosted:           true,
	})

	if svc == nil {
		t.Fatal("NewService returned nil")
	}
	if got := svc.now(); !got.Equal(now) {
		t.Fatalf("expected injected now %s, got %s", now, got)
	}
	if svc.environmentScope != "prod" || !svc.hosted {
		t.Fatalf("unexpected environment boundary: env=%q hosted=%v", svc.environmentScope, svc.hosted)
	}
}

func TestServiceActivateCreatesAndReusesOnePersonalTenant(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	repo := newMemoryIdentityRepository()
	repo.principals["prn_hosted"] = identity.Principal{
		PrincipalID:   "prn_hosted",
		PrincipalKind: identity.PrincipalKindUser,
		DisplayName:   "Hosted User",
		Status:        identity.StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	stateStore := newMemoryStateStore()
	auditSink := &recordingAuditSink{}
	svc := NewService(Dependencies{
		StateStore:       stateStore,
		Identity:         repo,
		Audit:            auditSink,
		Now:              func() time.Time { return now },
		EnvironmentScope: "test",
		Hosted:           true,
	})
	input := ActivateInput{
		Token: identity.TokenAuthority{
			TokenID:     "tok_hosted",
			PrincipalID: "prn_hosted",
			Status:      identity.StatusActive,
		},
		Source: "signup",
	}

	first, err := svc.Activate(ctx, input)
	if err != nil {
		t.Fatalf("Activate first returned error: %v", err)
	}
	second, err := svc.Activate(ctx, input)
	if err != nil {
		t.Fatalf("Activate second returned error: %v", err)
	}

	if first.TenantID == "" || first.TenantID != second.TenantID {
		t.Fatalf("expected stable personal tenant, first=%q second=%q", first.TenantID, second.TenantID)
	}
	if first.ActivationID != second.ActivationID || second.Status != StatusActive {
		t.Fatalf("expected stable active activation, first=%#v second=%#v", first, second)
	}
	personalTenants := 0
	for _, tenant := range repo.tenants {
		if tenant.TenantKind == identity.TenantKindPersonal && tenant.DefaultOwnerPrincipalID == "prn_hosted" {
			personalTenants++
		}
	}
	if personalTenants != 1 {
		t.Fatalf("expected one personal tenant, got %d", personalTenants)
	}
	if len(stateStore.statesByKey) != 1 {
		t.Fatalf("expected one activation state by principal tenant, got %d", len(stateStore.statesByKey))
	}
	if !auditSink.hasEvent("tenant.activation_started", ReasonCode("")) || !auditSink.hasEvent("tenant.activation_completed", ReasonCode("")) {
		t.Fatalf("expected activation started and completed audit events, got %#v", auditSink.events)
	}
}

func TestServiceActivateConcurrentAttemptsConverge(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	repo := newMemoryIdentityRepository()
	repo.principals["prn_concurrent"] = identity.Principal{
		PrincipalID:   "prn_concurrent",
		PrincipalKind: identity.PrincipalKindUser,
		DisplayName:   "Concurrent User",
		Status:        identity.StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	stateStore := newMemoryStateStore()
	auditSink := &recordingAuditSink{}
	svc := NewService(Dependencies{
		StateStore:       stateStore,
		Identity:         repo,
		Audit:            auditSink,
		Now:              func() time.Time { return now },
		EnvironmentScope: "test",
		Hosted:           true,
	})

	const attempts = 12
	results := make(chan State, attempts)
	errs := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state, err := svc.Activate(ctx, ActivateInput{Token: identity.TokenAuthority{TokenID: "tok_concurrent", PrincipalID: "prn_concurrent", Status: identity.StatusActive}})
			if err != nil {
				errs <- err
				return
			}
			results <- state
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatalf("Activate returned concurrent error: %v", err)
	}

	var tenantID string
	for state := range results {
		if tenantID == "" {
			tenantID = state.TenantID
		}
		if state.TenantID != tenantID || state.Status != StatusActive {
			t.Fatalf("concurrent activation diverged: first tenant=%q state=%#v", tenantID, state)
		}
	}
	if len(stateStore.statesByKey) != 1 {
		t.Fatalf("expected one activation state after concurrent attempts, got %d", len(stateStore.statesByKey))
	}
}

func TestServiceActivateDeniesDisabledPrincipalWithStableReason(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	repo := newMemoryIdentityRepository()
	repo.principals["prn_disabled"] = identity.Principal{
		PrincipalID:   "prn_disabled",
		PrincipalKind: identity.PrincipalKindUser,
		DisplayName:   "Disabled User",
		Status:        identity.StatusDisabled,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	stateStore := newMemoryStateStore()
	auditSink := &recordingAuditSink{}
	svc := NewService(Dependencies{
		StateStore:       stateStore,
		Identity:         repo,
		Audit:            auditSink,
		Now:              func() time.Time { return now },
		EnvironmentScope: "test",
		Hosted:           true,
	})

	_, err := svc.Activate(ctx, ActivateInput{Token: identity.TokenAuthority{TokenID: "tok_disabled", PrincipalID: "prn_disabled", Status: identity.StatusActive}})
	if got := ReasonCodeFromError(err); got != ReasonPrincipalDisabled {
		t.Fatalf("expected reason %q, got %q err=%v", ReasonPrincipalDisabled, got, err)
	}
	if len(stateStore.statesByKey) != 0 {
		t.Fatalf("disabled activation should not persist completion state: %#v", stateStore.statesByKey)
	}
	if !auditSink.hasEvent("tenant.activation_denied", ReasonPrincipalDisabled) {
		t.Fatalf("expected activation denied audit event, got %#v", auditSink.events)
	}
}

func TestServiceActivateDeniesRevokedTokenAndTenantAccessWithStableReasons(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	repo := newMemoryIdentityRepository()
	repo.principals["prn_revoked"] = identity.Principal{
		PrincipalID:     "prn_revoked",
		PrincipalKind:   identity.PrincipalKindUser,
		DisplayName:     "Revoked User",
		Status:          identity.StatusActive,
		DefaultTenantID: "ten_personal",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	repo.tenants["ten_personal"] = identity.Tenant{
		TenantID:                "ten_personal",
		TenantKind:              identity.TenantKindPersonal,
		DisplayName:             "Personal tenant",
		Status:                  identity.StatusActive,
		CreatedAt:               now,
		UpdatedAt:               now,
		DefaultOwnerPrincipalID: "prn_revoked",
	}
	repo.memberships["mem_revoked"] = identity.Membership{
		MembershipID: "mem_revoked",
		TenantID:     "ten_personal",
		PrincipalID:  "prn_revoked",
		Role:         identity.RoleOwner,
		Status:       identity.StatusRemoved,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	stateStore := newMemoryStateStore()
	svc := NewService(Dependencies{
		StateStore:       stateStore,
		Identity:         repo,
		Audit:            &fakeAuditSink{},
		Now:              func() time.Time { return now },
		EnvironmentScope: "test",
		Hosted:           true,
	})

	_, err := svc.Activate(ctx, ActivateInput{Token: identity.TokenAuthority{TokenID: "tok_revoked", PrincipalID: "prn_revoked", Status: identity.StatusRevoked}})
	if got := ReasonCodeFromError(err); got != ReasonPrincipalDenied {
		t.Fatalf("expected revoked token reason %q, got %q err=%v", ReasonPrincipalDenied, got, err)
	}
	_, err = svc.Activate(ctx, ActivateInput{Token: identity.TokenAuthority{TokenID: "tok_active", PrincipalID: "prn_revoked", Status: identity.StatusActive}})
	if got := ReasonCodeFromError(err); got != ReasonTenantAccessRevoked {
		t.Fatalf("expected revoked tenant access reason %q, got %q err=%v", ReasonTenantAccessRevoked, got, err)
	}
	if len(stateStore.statesByKey) != 0 {
		t.Fatalf("revoked activation should not persist completion state: %#v", stateStore.statesByKey)
	}
}

type fakeStateStore struct{}

func (*fakeStateStore) UpsertActivationState(context.Context, State) error {
	return nil
}

func (*fakeStateStore) GetActivationState(context.Context, string) (State, bool, error) {
	return State{}, false, nil
}

func (*fakeStateStore) GetActivationStateForPrincipalTenant(context.Context, string, string) (State, bool, error) {
	return State{}, false, nil
}

type fakeIdentityRepository struct{}

func (*fakeIdentityRepository) GetPrincipal(context.Context, string) (identity.Principal, bool, error) {
	return identity.Principal{}, false, nil
}

func (*fakeIdentityRepository) ListPrincipals(context.Context, identity.PrincipalFilter) ([]identity.Principal, error) {
	return nil, nil
}

func (*fakeIdentityRepository) UpsertPrincipal(context.Context, identity.Principal) error {
	return nil
}

func (*fakeIdentityRepository) GetTenant(context.Context, string) (identity.Tenant, bool, error) {
	return identity.Tenant{}, false, nil
}

func (*fakeIdentityRepository) ListTenants(context.Context, identity.TenantFilter) ([]identity.Tenant, error) {
	return nil, nil
}

func (*fakeIdentityRepository) UpsertTenant(context.Context, identity.Tenant) error {
	return nil
}

func (*fakeIdentityRepository) ListMemberships(context.Context, identity.MembershipFilter) ([]identity.Membership, error) {
	return nil, nil
}

func (*fakeIdentityRepository) UpsertMembership(context.Context, identity.Membership) error {
	return nil
}

func (*fakeIdentityRepository) ListTokenTenantGrants(context.Context, string) ([]identity.TokenTenantGrant, error) {
	return nil, nil
}

func (*fakeIdentityRepository) UpsertTokenTenantGrant(context.Context, identity.TokenTenantGrant) error {
	return nil
}

type fakeBillingProjector struct{}

func (*fakeBillingProjector) UsageSummary(context.Context, string, bool) (billing.UsageSummary, error) {
	return billing.UsageSummary{}, nil
}

type fakeChatRunner struct{}

func (*fakeChatRunner) RunActivationTestChat(context.Context, TestChatInput) (TestChatResult, error) {
	return TestChatResult{}, nil
}

type fakeAuditSink struct{}

func (*fakeAuditSink) AppendTenantAuditEvent(context.Context, identity.TenantAuditEvent) (identity.TenantAuditEvent, error) {
	return identity.TenantAuditEvent{}, nil
}

type recordingAuditSink struct {
	events []identity.TenantAuditEvent
}

func (s *recordingAuditSink) AppendTenantAuditEvent(_ context.Context, event identity.TenantAuditEvent) (identity.TenantAuditEvent, error) {
	s.events = append(s.events, event)
	return event, nil
}

func (s *recordingAuditSink) hasEvent(kind string, reason ReasonCode) bool {
	for _, event := range s.events {
		if event.EventKind != kind {
			continue
		}
		if reason != "" && event.ReasonCode != string(reason) {
			continue
		}
		return true
	}
	return false
}

type memoryStateStore struct {
	mu          sync.Mutex
	statesByID  map[string]State
	statesByKey map[string]State
}

func newMemoryStateStore() *memoryStateStore {
	return &memoryStateStore{
		statesByID:  map[string]State{},
		statesByKey: map[string]State{},
	}
}

func (s *memoryStateStore) UpsertActivationState(_ context.Context, state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statesByID[state.ActivationID] = state
	s.statesByKey[state.PrincipalID+"|"+state.TenantID] = state
	return nil
}

func (s *memoryStateStore) GetActivationState(_ context.Context, activationID string) (State, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.statesByID[activationID]
	return state, ok, nil
}

func (s *memoryStateStore) GetActivationStateForPrincipalTenant(_ context.Context, principalID, tenantID string) (State, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.statesByKey[principalID+"|"+tenantID]
	return state, ok, nil
}

type memoryIdentityRepository struct {
	mu          sync.Mutex
	principals  map[string]identity.Principal
	tenants     map[string]identity.Tenant
	memberships map[string]identity.Membership
	grants      map[string]identity.TokenTenantGrant
}

func newMemoryIdentityRepository() *memoryIdentityRepository {
	return &memoryIdentityRepository{
		principals:  map[string]identity.Principal{},
		tenants:     map[string]identity.Tenant{},
		memberships: map[string]identity.Membership{},
		grants:      map[string]identity.TokenTenantGrant{},
	}
}

func (r *memoryIdentityRepository) GetPrincipal(_ context.Context, principalID string) (identity.Principal, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	principal, ok := r.principals[principalID]
	return principal, ok, nil
}

func (r *memoryIdentityRepository) ListPrincipals(_ context.Context, filter identity.PrincipalFilter) ([]identity.Principal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := []identity.Principal{}
	for _, principal := range r.principals {
		if filter.Status != "" && principal.Status != filter.Status {
			continue
		}
		items = append(items, principal)
	}
	return items, nil
}

func (r *memoryIdentityRepository) UpsertPrincipal(_ context.Context, principal identity.Principal) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.principals[principal.PrincipalID] = principal
	return nil
}

func (r *memoryIdentityRepository) GetTenant(_ context.Context, tenantID string) (identity.Tenant, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tenant, ok := r.tenants[tenantID]
	return tenant, ok, nil
}

func (r *memoryIdentityRepository) ListTenants(_ context.Context, filter identity.TenantFilter) ([]identity.Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := []identity.Tenant{}
	for _, tenant := range r.tenants {
		if filter.TenantKind != "" && tenant.TenantKind != filter.TenantKind {
			continue
		}
		if filter.Status != "" && tenant.Status != filter.Status {
			continue
		}
		items = append(items, tenant)
	}
	return items, nil
}

func (r *memoryIdentityRepository) UpsertTenant(_ context.Context, tenant identity.Tenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tenants[tenant.TenantID] = tenant
	return nil
}

func (r *memoryIdentityRepository) ListMemberships(_ context.Context, filter identity.MembershipFilter) ([]identity.Membership, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := []identity.Membership{}
	for _, membership := range r.memberships {
		if filter.TenantID != "" && membership.TenantID != filter.TenantID {
			continue
		}
		if filter.Status != "" && membership.Status != filter.Status {
			continue
		}
		items = append(items, membership)
	}
	return items, nil
}

func (r *memoryIdentityRepository) UpsertMembership(_ context.Context, membership identity.Membership) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.memberships[membership.MembershipID] = membership
	return nil
}

func (r *memoryIdentityRepository) ListTokenTenantGrants(_ context.Context, tokenID string) ([]identity.TokenTenantGrant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := []identity.TokenTenantGrant{}
	for _, grant := range r.grants {
		if grant.TokenID == tokenID {
			items = append(items, grant)
		}
	}
	return items, nil
}

func (r *memoryIdentityRepository) UpsertTokenTenantGrant(_ context.Context, grant identity.TokenTenantGrant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.grants[grant.GrantID] = grant
	return nil
}
