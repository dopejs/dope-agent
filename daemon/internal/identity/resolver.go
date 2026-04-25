package identity

import (
	"context"
	"time"
)

const (
	TenantSourceDefault        = "default"
	TenantSourceExplicitHeader = "explicit_header"
)

type ResolverStore interface {
	GetPrincipal(ctx context.Context, principalID string) (Principal, bool, error)
	GetTenant(ctx context.Context, tenantID string) (Tenant, bool, error)
	ListMemberships(ctx context.Context, filter MembershipFilter) ([]Membership, error)
	ListTokenTenantGrants(ctx context.Context, tokenID string) ([]TokenTenantGrant, error)
}

type TokenAuthority struct {
	TokenID         string
	PrincipalID     string
	DefaultTenantID string
	Status          LifecycleStatus
	ExpiresAt       *time.Time
}

type Resolver struct {
	store ResolverStore
	now   func() time.Time
}

func NewResolver(store ResolverStore) *Resolver {
	return &Resolver{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (r *Resolver) Resolve(ctx context.Context, token TokenAuthority, selectedTenantID string) (TenantContext, error) {
	if r == nil || r.store == nil {
		return TenantContext{}, ErrTenantAccessDenied
	}
	now := r.now().UTC()
	if token.TokenID == "" || token.PrincipalID == "" || token.Status != StatusActive {
		return TenantContext{}, ErrTenantAccessDenied
	}
	if token.ExpiresAt != nil && !token.ExpiresAt.After(now) {
		return TenantContext{}, ErrTenantAccessDenied
	}
	principal, ok, err := r.store.GetPrincipal(ctx, token.PrincipalID)
	if err != nil {
		return TenantContext{}, err
	}
	if !ok || principal.Status != StatusActive {
		return TenantContext{}, ErrTenantAccessDenied
	}
	tenantID := selectedTenantID
	source := TenantSourceExplicitHeader
	if tenantID == "" {
		tenantID = token.DefaultTenantID
		if tenantID == "" {
			tenantID = principal.DefaultTenantID
		}
		source = TenantSourceDefault
	}
	if tenantID == "" {
		return TenantContext{}, ErrTenantAccessDenied
	}
	tenant, ok, err := r.store.GetTenant(ctx, tenantID)
	if err != nil {
		return TenantContext{}, err
	}
	if !ok || tenant.Status != StatusActive {
		return TenantContext{}, ErrTenantAccessDenied
	}
	if !tokenHasTenantGrant(ctx, r.store, token.TokenID, tenantID) {
		return TenantContext{}, ErrTenantAccessDenied
	}
	memberships, err := r.store.ListMemberships(ctx, MembershipFilter{TenantID: tenantID, Status: StatusActive, Limit: 500})
	if err != nil {
		return TenantContext{}, err
	}
	for _, membership := range memberships {
		if membership.PrincipalID != principal.PrincipalID || membership.Status != StatusActive {
			continue
		}
		perms := PermissionsForRole(membership.Role, membership.Status)
		return TenantContext{
			PrincipalID:  principal.PrincipalID,
			TokenID:      token.TokenID,
			TenantID:     tenantID,
			TenantSource: source,
			MembershipID: membership.MembershipID,
			Role:         membership.Role,
			Permissions:  perms,
			ResolvedAt:   now,
		}, nil
	}
	return TenantContext{}, ErrTenantAccessDenied
}

func tokenHasTenantGrant(ctx context.Context, store ResolverStore, tokenID, tenantID string) bool {
	grants, err := store.ListTokenTenantGrants(ctx, tokenID)
	if err != nil {
		return false
	}
	for _, grant := range grants {
		if grant.TenantID == tenantID && grant.Status == StatusActive {
			return true
		}
	}
	return false
}
