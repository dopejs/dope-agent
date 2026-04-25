package identity

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

type Store interface {
	ResolverStore
	AuditStore
	UpsertTenant(ctx context.Context, tenant Tenant) error
	UpsertPrincipal(ctx context.Context, principal Principal) error
	UpsertMembership(ctx context.Context, membership Membership) error
	UpsertTenantInvitation(ctx context.Context, invitation TenantInvitation) error
	UpsertTokenTenantGrant(ctx context.Context, grant TokenTenantGrant) error
	ListTenants(ctx context.Context, filter TenantFilter) ([]Tenant, error)
	ListPrincipals(ctx context.Context, filter PrincipalFilter) ([]Principal, error)
	ListTenantInvitations(ctx context.Context, filter InvitationFilter) ([]TenantInvitation, error)
	ListTokenAuthorities(ctx context.Context) ([]TokenAuthority, error)
}

type CreateInvitationInput struct {
	TenantID           string
	InvitedPrincipalID string
	Role               Role
	ExpiresAt          *time.Time
}

type Manager struct {
	store    Store
	auditor  *Auditor
	resolver *Resolver
	now      func() time.Time
}

func (m *Manager) CreateOrganizationTenant(ctx context.Context, actor TenantContext, displayName string) (Tenant, Membership, error) {
	if err := RequirePermission(actor, PermissionTenantManage); err != nil {
		return Tenant{}, Membership{}, err
	}
	now := m.now().UTC()
	tenant := Tenant{
		TenantID:                "ten_" + randomID(8),
		TenantKind:              TenantKindOrganization,
		DisplayName:             strings.TrimSpace(displayName),
		Status:                  StatusActive,
		CreatedAt:               now,
		UpdatedAt:               now,
		CreatedByPrincipalID:    actor.PrincipalID,
		DefaultOwnerPrincipalID: actor.PrincipalID,
	}
	if tenant.DisplayName == "" {
		return Tenant{}, Membership{}, ErrTenantInvalid
	}
	membership := Membership{
		MembershipID: "mem_" + randomID(8),
		TenantID:     tenant.TenantID,
		PrincipalID:  actor.PrincipalID,
		Role:         RoleOwner,
		Status:       StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
		AcceptedAt:   &now,
	}
	if _, err := m.auditor.Require(ctx, TenantAuditEvent{AuditEventID: "audit_" + randomID(8), EventKind: "tenant.organization_created", TenantID: tenant.TenantID, PrincipalID: actor.PrincipalID, Outcome: AuditOutcomeSucceeded, ReasonCode: "organization_created", CreatedAt: now}); err != nil {
		return Tenant{}, Membership{}, err
	}
	if err := m.store.UpsertTenant(ctx, tenant); err != nil {
		return Tenant{}, Membership{}, err
	}
	if err := m.store.UpsertMembership(ctx, membership); err != nil {
		return Tenant{}, Membership{}, err
	}
	if actor.TokenID != "" {
		if err := m.store.UpsertTokenTenantGrant(ctx, TokenTenantGrant{
			GrantID:              "grant_" + randomID(8),
			TokenID:              actor.TokenID,
			TenantID:             tenant.TenantID,
			Status:               StatusActive,
			CreatedAt:            now,
			UpdatedAt:            now,
			GrantedByPrincipalID: actor.PrincipalID,
		}); err != nil {
			return Tenant{}, Membership{}, err
		}
	}
	return tenant, membership, nil
}

func (m *Manager) CreateInvitation(ctx context.Context, actor TenantContext, input CreateInvitationInput) (TenantInvitation, error) {
	if err := RequirePermission(actor, PermissionTenantManage); err != nil {
		return TenantInvitation{}, err
	}
	if input.TenantID == "" || input.InvitedPrincipalID == "" || input.Role == "" {
		return TenantInvitation{}, ErrInvitationInvalid
	}
	if input.TenantID != actor.TenantID {
		return TenantInvitation{}, ErrTenantAccessDenied
	}
	if _, ok, err := m.store.GetPrincipal(ctx, input.InvitedPrincipalID); err != nil {
		return TenantInvitation{}, err
	} else if !ok {
		return TenantInvitation{}, ErrPrincipalInvalid
	}
	now := m.now().UTC()
	invitation := TenantInvitation{
		InvitationID:         "inv_" + randomID(8),
		TenantID:             input.TenantID,
		InvitedPrincipalID:   input.InvitedPrincipalID,
		InvitedByPrincipalID: actor.PrincipalID,
		Role:                 input.Role,
		Status:               StatusInvited,
		CreatedAt:            now,
		UpdatedAt:            now,
		ExpiresAt:            input.ExpiresAt,
	}
	if _, err := m.auditor.Require(ctx, TenantAuditEvent{AuditEventID: "audit_" + randomID(8), EventKind: "tenant.invitation_created", TenantID: invitation.TenantID, PrincipalID: actor.PrincipalID, TargetPrincipalID: invitation.InvitedPrincipalID, Outcome: AuditOutcomeSucceeded, ReasonCode: "invitation_created", CreatedAt: now}); err != nil {
		return TenantInvitation{}, err
	}
	if err := m.store.UpsertTenantInvitation(ctx, invitation); err != nil {
		return TenantInvitation{}, err
	}
	return invitation, nil
}

func (m *Manager) AcceptInvitation(ctx context.Context, principalID, invitationID string) (Membership, error) {
	invitation, err := m.findInvitation(ctx, invitationID)
	if err != nil {
		return Membership{}, err
	}
	now := m.now().UTC()
	if invitation.InvitedPrincipalID != principalID || invitation.Status != StatusInvited || (invitation.ExpiresAt != nil && !invitation.ExpiresAt.After(now)) {
		return Membership{}, ErrInvitationInvalid
	}
	principal, ok, err := m.store.GetPrincipal(ctx, principalID)
	if err != nil {
		return Membership{}, err
	}
	if !ok || principal.Status != StatusActive {
		return Membership{}, ErrPrincipalInvalid
	}
	invitation.Status = StatusAccepted
	invitation.UpdatedAt = now
	invitation.DecidedAt = &now
	membership := Membership{
		MembershipID: "mem_" + randomID(8),
		TenantID:     invitation.TenantID,
		PrincipalID:  principalID,
		Role:         invitation.Role,
		Status:       StatusActive,
		InvitationID: invitation.InvitationID,
		CreatedAt:    now,
		UpdatedAt:    now,
		AcceptedAt:   &now,
	}
	if _, err := m.auditor.Require(ctx, TenantAuditEvent{AuditEventID: "audit_" + randomID(8), EventKind: "tenant.invitation_accepted", TenantID: invitation.TenantID, PrincipalID: principalID, Outcome: AuditOutcomeSucceeded, ReasonCode: "invitation_accepted", CreatedAt: now}); err != nil {
		return Membership{}, err
	}
	if err := m.store.UpsertTenantInvitation(ctx, invitation); err != nil {
		return Membership{}, err
	}
	if err := m.store.UpsertMembership(ctx, membership); err != nil {
		return Membership{}, err
	}
	return membership, nil
}

func (m *Manager) DecideInvitation(ctx context.Context, principalID, invitationID string, status LifecycleStatus) (TenantInvitation, error) {
	if status != StatusRejected && status != StatusRevoked && status != StatusExpired {
		return TenantInvitation{}, ErrInvitationInvalid
	}
	invitation, err := m.findInvitation(ctx, invitationID)
	if err != nil {
		return TenantInvitation{}, err
	}
	if status == StatusRejected && invitation.InvitedPrincipalID != principalID {
		return TenantInvitation{}, ErrInvitationInvalid
	}
	now := m.now().UTC()
	invitation.Status = status
	invitation.UpdatedAt = now
	invitation.DecidedAt = &now
	if _, err := m.auditor.Require(ctx, TenantAuditEvent{AuditEventID: "audit_" + randomID(8), EventKind: "tenant.invitation_" + string(status), TenantID: invitation.TenantID, PrincipalID: principalID, TargetPrincipalID: invitation.InvitedPrincipalID, Outcome: AuditOutcomeSucceeded, ReasonCode: "invitation_" + string(status), CreatedAt: now}); err != nil {
		return TenantInvitation{}, err
	}
	if err := m.store.UpsertTenantInvitation(ctx, invitation); err != nil {
		return TenantInvitation{}, err
	}
	return invitation, nil
}

func (m *Manager) UpdateMembershipRole(ctx context.Context, actor TenantContext, tenantID, membershipID string, role Role) (Membership, error) {
	if err := RequirePermission(actor, PermissionTenantManage); err != nil {
		return Membership{}, err
	}
	membership, err := m.findMembership(ctx, tenantID, membershipID)
	if err != nil {
		return Membership{}, err
	}
	if membership.Role == RoleOwner && role != RoleOwner {
		if err := m.ensureAnotherActiveOwner(ctx, tenantID, membershipID); err != nil {
			return Membership{}, err
		}
	}
	now := m.now().UTC()
	membership.Role = role
	membership.UpdatedAt = now
	if _, err := m.auditor.Require(ctx, TenantAuditEvent{AuditEventID: "audit_" + randomID(8), EventKind: "tenant.membership_role_updated", TenantID: tenantID, PrincipalID: actor.PrincipalID, TargetPrincipalID: membership.PrincipalID, Outcome: AuditOutcomeSucceeded, ReasonCode: "membership_role_updated", CreatedAt: now}); err != nil {
		return Membership{}, err
	}
	if err := m.store.UpsertMembership(ctx, membership); err != nil {
		return Membership{}, err
	}
	return membership, nil
}

func (m *Manager) RemoveMembership(ctx context.Context, actor TenantContext, tenantID, membershipID string) (Membership, error) {
	if err := RequirePermission(actor, PermissionTenantManage); err != nil {
		return Membership{}, err
	}
	membership, err := m.findMembership(ctx, tenantID, membershipID)
	if err != nil {
		return Membership{}, err
	}
	if membership.Role == RoleOwner {
		if err := m.ensureAnotherActiveOwner(ctx, tenantID, membershipID); err != nil {
			return Membership{}, err
		}
	}
	now := m.now().UTC()
	membership.Status = StatusRemoved
	membership.UpdatedAt = now
	membership.RemovedAt = &now
	if _, err := m.auditor.Require(ctx, TenantAuditEvent{AuditEventID: "audit_" + randomID(8), EventKind: "tenant.membership_removed", TenantID: tenantID, PrincipalID: actor.PrincipalID, TargetPrincipalID: membership.PrincipalID, Outcome: AuditOutcomeSucceeded, ReasonCode: "membership_removed", CreatedAt: now}); err != nil {
		return Membership{}, err
	}
	if err := m.store.UpsertMembership(ctx, membership); err != nil {
		return Membership{}, err
	}
	return membership, nil
}

func (m *Manager) ReplaceTokenTenantGrants(ctx context.Context, actor TenantContext, tokenID string, tenantIDs []string, defaultTenantID string) ([]TokenTenantGrant, error) {
	if err := RequirePermission(actor, PermissionTenantManage); err != nil {
		return nil, err
	}
	targetPrincipalID, err := m.tokenPrincipalID(ctx, actor, tokenID)
	if err != nil {
		return nil, err
	}
	seen, err := m.validateTokenTenantGrantSet(ctx, targetPrincipalID, tenantIDs, defaultTenantID)
	if err != nil {
		return nil, err
	}
	if defaultTenantID == "" {
		return nil, ErrTokenGrantInvalid
	}
	now := m.now().UTC()
	if _, err := m.auditor.Require(ctx, TenantAuditEvent{AuditEventID: "audit_" + randomID(8), EventKind: "tenant.token_grants_changed", TenantID: defaultTenantID, PrincipalID: actor.PrincipalID, TargetPrincipalID: targetPrincipalID, TokenID: tokenID, Outcome: AuditOutcomeSucceeded, ReasonCode: "token_grants_changed", CreatedAt: now}); err != nil {
		return nil, err
	}
	existing, err := m.store.ListTokenTenantGrants(ctx, tokenID)
	if err != nil {
		return nil, err
	}
	for _, grant := range existing {
		if _, keep := seen[grant.TenantID]; keep && grant.Status == StatusActive {
			continue
		}
		if grant.Status == StatusActive {
			grant.Status = StatusRevoked
			grant.UpdatedAt = now
			grant.RevokedAt = &now
			if err := m.store.UpsertTokenTenantGrant(ctx, grant); err != nil {
				return nil, err
			}
		}
	}
	result := make([]TokenTenantGrant, 0, len(seen))
	for tenantID := range seen {
		var grant TokenTenantGrant
		for _, existingGrant := range existing {
			if existingGrant.TenantID == tenantID {
				grant = existingGrant
				break
			}
		}
		if grant.GrantID == "" || grant.Status != StatusActive {
			grant = TokenTenantGrant{GrantID: "grant_" + randomID(8), TokenID: tokenID, TenantID: tenantID, Status: StatusActive, CreatedAt: now}
		}
		grant.IsDefault = tenantID == defaultTenantID
		grant.UpdatedAt = now
		grant.RevokedAt = nil
		grant.GrantedByPrincipalID = actor.PrincipalID
		if err := m.store.UpsertTokenTenantGrant(ctx, grant); err != nil {
			return nil, err
		}
		result = append(result, grant)
	}
	return result, nil
}

func (m *Manager) ValidateTokenTenantGrants(ctx context.Context, actor TenantContext, tokenID string, tenantIDs []string, defaultTenantID string) error {
	if err := RequirePermission(actor, PermissionTenantManage); err != nil {
		return err
	}
	targetPrincipalID, err := m.tokenPrincipalID(ctx, actor, tokenID)
	if err != nil {
		return err
	}
	_, err = m.validateTokenTenantGrantSet(ctx, targetPrincipalID, tenantIDs, defaultTenantID)
	return err
}

func NewManager(store Store) *Manager {
	return &Manager{
		store:    store,
		auditor:  NewAuditor(store),
		resolver: NewResolver(store),
		now:      func() time.Time { return time.Now().UTC() },
	}
}

func (m *Manager) Resolve(ctx context.Context, token TokenAuthority, tenantID string) (TenantContext, error) {
	return m.resolver.Resolve(ctx, token, tenantID)
}

func (m *Manager) BootstrapLocal(ctx context.Context, tokenIDs []string) (Principal, Tenant, error) {
	now := m.now().UTC()
	principals, err := m.store.ListPrincipals(ctx, PrincipalFilter{Limit: 1})
	if err != nil {
		return Principal{}, Tenant{}, err
	}
	if len(principals) > 0 {
		tenant, ok, err := m.store.GetTenant(ctx, principals[0].DefaultTenantID)
		if err != nil {
			return Principal{}, Tenant{}, err
		}
		if ok {
			return principals[0], tenant, m.ensureTokenGrants(ctx, principals[0], tenant, tokenIDs)
		}
	}
	principal := Principal{
		PrincipalID:     "prn_" + randomID(8),
		PrincipalKind:   PrincipalKindLocalOperator,
		DisplayName:     "Local operator",
		Status:          StatusActive,
		DefaultTenantID: "ten_" + randomID(8),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	tenant := Tenant{
		TenantID:                principal.DefaultTenantID,
		TenantKind:              TenantKindPersonal,
		DisplayName:             "Personal tenant",
		Status:                  StatusActive,
		CreatedAt:               now,
		UpdatedAt:               now,
		CreatedByPrincipalID:    principal.PrincipalID,
		DefaultOwnerPrincipalID: principal.PrincipalID,
	}
	membership := Membership{
		MembershipID: "mem_" + randomID(8),
		TenantID:     tenant.TenantID,
		PrincipalID:  principal.PrincipalID,
		Role:         RoleOwner,
		Status:       StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
		AcceptedAt:   &now,
	}
	if err := m.store.UpsertPrincipal(ctx, principal); err != nil {
		return Principal{}, Tenant{}, err
	}
	if err := m.store.UpsertTenant(ctx, tenant); err != nil {
		return Principal{}, Tenant{}, err
	}
	if err := m.store.UpsertMembership(ctx, membership); err != nil {
		return Principal{}, Tenant{}, err
	}
	if _, err := m.auditor.Require(ctx, TenantAuditEvent{
		AuditEventID: "audit_" + randomID(8),
		EventKind:    "tenant.bootstrap_completed",
		TenantID:     tenant.TenantID,
		PrincipalID:  principal.PrincipalID,
		Outcome:      AuditOutcomeSucceeded,
		ReasonCode:   "local_bootstrap",
		CreatedAt:    now,
	}); err != nil {
		return Principal{}, Tenant{}, err
	}
	return principal, tenant, m.ensureTokenGrants(ctx, principal, tenant, tokenIDs)
}

func (m *Manager) ensureTokenGrants(ctx context.Context, principal Principal, tenant Tenant, tokenIDs []string) error {
	now := m.now().UTC()
	for _, tokenID := range tokenIDs {
		tokenID = strings.TrimSpace(tokenID)
		if tokenID == "" {
			continue
		}
		grants, err := m.store.ListTokenTenantGrants(ctx, tokenID)
		if err != nil {
			return err
		}
		hasGrant := false
		for _, grant := range grants {
			if grant.TenantID == tenant.TenantID && grant.Status == StatusActive {
				hasGrant = true
				break
			}
		}
		if hasGrant {
			continue
		}
		if err := m.store.UpsertTokenTenantGrant(ctx, TokenTenantGrant{
			GrantID:              "grant_" + randomID(8),
			TokenID:              tokenID,
			TenantID:             tenant.TenantID,
			IsDefault:            true,
			Status:               StatusActive,
			CreatedAt:            now,
			UpdatedAt:            now,
			GrantedByPrincipalID: principal.PrincipalID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) tokenPrincipalID(ctx context.Context, actor TenantContext, tokenID string) (string, error) {
	if tokenID == "" {
		return "", ErrTokenGrantInvalid
	}
	if tokenID == actor.TokenID {
		return actor.PrincipalID, nil
	}
	tokens, err := m.store.ListTokenAuthorities(ctx)
	if err != nil {
		return "", err
	}
	for _, token := range tokens {
		if token.TokenID == tokenID && token.PrincipalID != "" {
			return token.PrincipalID, nil
		}
	}
	return "", ErrTokenGrantInvalid
}

func (m *Manager) validateTokenTenantGrantSet(ctx context.Context, principalID string, tenantIDs []string, defaultTenantID string) (map[string]struct{}, error) {
	if principalID == "" || len(tenantIDs) == 0 || defaultTenantID == "" {
		return nil, ErrTokenGrantInvalid
	}
	allowed := map[string]struct{}{}
	memberships, err := m.store.ListMemberships(ctx, MembershipFilter{Status: StatusActive, Limit: 1000})
	if err != nil {
		return nil, err
	}
	for _, membership := range memberships {
		if membership.PrincipalID == principalID && membership.Status == StatusActive {
			allowed[membership.TenantID] = struct{}{}
		}
	}
	seen := map[string]struct{}{}
	for _, tenantID := range tenantIDs {
		tenantID = strings.TrimSpace(tenantID)
		if tenantID == "" {
			return nil, ErrTokenGrantInvalid
		}
		if _, ok := allowed[tenantID]; !ok {
			return nil, ErrTokenGrantInvalid
		}
		seen[tenantID] = struct{}{}
	}
	if _, ok := seen[defaultTenantID]; !ok {
		return nil, ErrTokenGrantInvalid
	}
	return seen, nil
}

func (m *Manager) findInvitation(ctx context.Context, invitationID string) (TenantInvitation, error) {
	invitations, err := m.store.ListTenantInvitations(ctx, InvitationFilter{Limit: 1000})
	if err != nil {
		return TenantInvitation{}, err
	}
	for _, invitation := range invitations {
		if invitation.InvitationID == invitationID {
			return invitation, nil
		}
	}
	return TenantInvitation{}, ErrInvitationInvalid
}

func (m *Manager) findMembership(ctx context.Context, tenantID, membershipID string) (Membership, error) {
	memberships, err := m.store.ListMemberships(ctx, MembershipFilter{TenantID: tenantID, Limit: 1000})
	if err != nil {
		return Membership{}, err
	}
	for _, membership := range memberships {
		if membership.MembershipID == membershipID {
			return membership, nil
		}
	}
	return Membership{}, ErrMembershipInvalid
}

func (m *Manager) ensureAnotherActiveOwner(ctx context.Context, tenantID, membershipID string) error {
	memberships, err := m.store.ListMemberships(ctx, MembershipFilter{TenantID: tenantID, Status: StatusActive, Role: RoleOwner, Limit: 1000})
	if err != nil {
		return err
	}
	for _, membership := range memberships {
		if membership.MembershipID != membershipID && membership.Status == StatusActive && membership.Role == RoleOwner {
			return nil
		}
	}
	return ErrOwnerInvariant
}

func randomID(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(buf)
}
