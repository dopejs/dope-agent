package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	_ "modernc.org/sqlite"
)

func TestSQLiteStorePersistsTenantIdentityRecordsAcrossRestart(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dataDir := t.TempDir()
	now := time.Now().UTC().Truncate(0)
	disabledAt := now.Add(2 * time.Minute)
	removedAt := now.Add(3 * time.Minute)
	acceptedAt := now.Add(4 * time.Minute)
	expiresAt := now.Add(24 * time.Hour)
	decidedAt := now.Add(5 * time.Minute)
	revokedAt := now.Add(6 * time.Minute)

	store, err := NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	tenant := identity.Tenant{
		TenantID:                "ten_personal",
		TenantKind:              identity.TenantKindPersonal,
		DisplayName:             "Personal tenant",
		Status:                  identity.StatusActive,
		CreatedAt:               now,
		UpdatedAt:               now,
		CreatedByPrincipalID:    "prn_local",
		DefaultOwnerPrincipalID: "prn_local",
	}
	principal := identity.Principal{
		PrincipalID:     "prn_local",
		PrincipalKind:   identity.PrincipalKindLocalOperator,
		DisplayName:     "Local operator",
		Status:          identity.StatusDisabled,
		DefaultTenantID: tenant.TenantID,
		CreatedAt:       now,
		UpdatedAt:       now,
		DisabledAt:      &disabledAt,
		RemovedAt:       &removedAt,
	}
	membership := identity.Membership{
		MembershipID: "mem_owner",
		TenantID:     tenant.TenantID,
		PrincipalID:  principal.PrincipalID,
		Role:         identity.RoleOwner,
		Status:       identity.StatusRemoved,
		InvitationID: "inv_local",
		CreatedAt:    now,
		UpdatedAt:    now,
		AcceptedAt:   &acceptedAt,
		RemovedAt:    &removedAt,
	}
	invitation := identity.TenantInvitation{
		InvitationID:         "inv_local",
		TenantID:             tenant.TenantID,
		InvitedPrincipalID:   principal.PrincipalID,
		InvitedByPrincipalID: principal.PrincipalID,
		Role:                 identity.RoleAdmin,
		Status:               identity.StatusAccepted,
		CreatedAt:            now,
		UpdatedAt:            now,
		ExpiresAt:            &expiresAt,
		DecidedAt:            &decidedAt,
	}
	grant := identity.TokenTenantGrant{
		GrantID:              "grant_local",
		TokenID:              "tok_local",
		TenantID:             tenant.TenantID,
		IsDefault:            true,
		Status:               identity.StatusRevoked,
		CreatedAt:            now,
		UpdatedAt:            now,
		RevokedAt:            &revokedAt,
		GrantedByPrincipalID: principal.PrincipalID,
	}
	auditEvent := identity.TenantAuditEvent{
		AuditEventID:      "audit_bootstrap",
		EventKind:         "tenant.bootstrap_completed",
		TenantID:          tenant.TenantID,
		PrincipalID:       principal.PrincipalID,
		TargetPrincipalID: principal.PrincipalID,
		TokenID:           grant.TokenID,
		Outcome:           identity.AuditOutcomeSucceeded,
		ReasonCode:        "local_bootstrap",
		CreatedAt:         now,
		Document: map[string]any{
			"source": "test",
		},
	}
	if err := store.UpsertTenant(ctx, tenant); err != nil {
		t.Fatalf("UpsertTenant returned error: %v", err)
	}
	if err := store.UpsertPrincipal(ctx, principal); err != nil {
		t.Fatalf("UpsertPrincipal returned error: %v", err)
	}
	if err := store.UpsertMembership(ctx, membership); err != nil {
		t.Fatalf("UpsertMembership returned error: %v", err)
	}
	if err := store.UpsertTenantInvitation(ctx, invitation); err != nil {
		t.Fatalf("UpsertTenantInvitation returned error: %v", err)
	}
	if err := store.UpsertTokenTenantGrant(ctx, grant); err != nil {
		t.Fatalf("UpsertTokenTenantGrant returned error: %v", err)
	}
	if _, err := store.AppendTenantAuditEvent(ctx, auditEvent); err != nil {
		t.Fatalf("AppendTenantAuditEvent returned error: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	store, err = NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore reopen returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	persistedTenant, ok, err := store.GetTenant(ctx, tenant.TenantID)
	if err != nil {
		t.Fatalf("GetTenant returned error: %v", err)
	}
	if !ok || !sameTenant(persistedTenant, tenant) {
		t.Fatalf("expected persisted tenant %+v, got ok=%v %+v", tenant, ok, persistedTenant)
	}
	persistedPrincipal, ok, err := store.GetPrincipal(ctx, principal.PrincipalID)
	if err != nil {
		t.Fatalf("GetPrincipal returned error: %v", err)
	}
	if !ok || !samePrincipal(persistedPrincipal, principal) {
		t.Fatalf("expected persisted principal %+v, got ok=%v %+v", principal, ok, persistedPrincipal)
	}
	memberships, err := store.ListMemberships(ctx, identity.MembershipFilter{TenantID: tenant.TenantID, Status: identity.StatusRemoved})
	if err != nil {
		t.Fatalf("ListMemberships returned error: %v", err)
	}
	if len(memberships) != 1 || !sameMembership(memberships[0], membership) {
		t.Fatalf("expected persisted membership %+v, got %+v", membership, memberships)
	}
	invitations, err := store.ListTenantInvitations(ctx, identity.InvitationFilter{TenantID: tenant.TenantID, PrincipalID: principal.PrincipalID})
	if err != nil {
		t.Fatalf("ListTenantInvitations returned error: %v", err)
	}
	if len(invitations) != 1 || !sameInvitation(invitations[0], invitation) {
		t.Fatalf("expected persisted invitation %+v, got %+v", invitation, invitations)
	}
	grants, err := store.ListTokenTenantGrants(ctx, grant.TokenID)
	if err != nil {
		t.Fatalf("ListTokenTenantGrants returned error: %v", err)
	}
	if len(grants) != 1 || !sameGrant(grants[0], grant) {
		t.Fatalf("expected persisted grant %+v, got %+v", grant, grants)
	}
	auditEvents, err := store.ListTenantAuditEvents(ctx, identity.AuditEventFilter{TenantID: tenant.TenantID, TokenID: grant.TokenID})
	if err != nil {
		t.Fatalf("ListTenantAuditEvents returned error: %v", err)
	}
	if len(auditEvents) != 1 || auditEvents[0].AuditEventID != auditEvent.AuditEventID || auditEvents[0].Document["source"] != "test" {
		t.Fatalf("expected persisted audit event %+v, got %+v", auditEvent, auditEvents)
	}
}

func TestSQLiteStorePersistsOrganizationMembershipsAndInvitationsAcrossRestart(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dataDir := t.TempDir()
	now := time.Now().UTC().Truncate(0)
	expiresAt := now.Add(24 * time.Hour)

	store, err := NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	tenant := identity.Tenant{
		TenantID:                "ten_org",
		TenantKind:              identity.TenantKindOrganization,
		DisplayName:             "Organization",
		Status:                  identity.StatusActive,
		CreatedAt:               now,
		UpdatedAt:               now,
		CreatedByPrincipalID:    "prn_owner",
		DefaultOwnerPrincipalID: "prn_owner",
	}
	owner := identity.Principal{PrincipalID: "prn_owner", PrincipalKind: identity.PrincipalKindUser, DisplayName: "Owner", Status: identity.StatusActive, DefaultTenantID: tenant.TenantID, CreatedAt: now, UpdatedAt: now}
	invited := identity.Principal{PrincipalID: "prn_invited", PrincipalKind: identity.PrincipalKindUser, DisplayName: "Invited", Status: identity.StatusActive, DefaultTenantID: tenant.TenantID, CreatedAt: now, UpdatedAt: now}
	ownerMembership := identity.Membership{MembershipID: "mem_org_owner", TenantID: tenant.TenantID, PrincipalID: owner.PrincipalID, Role: identity.RoleOwner, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now, AcceptedAt: &now}
	invitation := identity.TenantInvitation{InvitationID: "inv_org", TenantID: tenant.TenantID, InvitedPrincipalID: invited.PrincipalID, InvitedByPrincipalID: owner.PrincipalID, Role: identity.RoleOperator, Status: identity.StatusInvited, CreatedAt: now, UpdatedAt: now, ExpiresAt: &expiresAt}
	for _, principal := range []identity.Principal{owner, invited} {
		if err := store.UpsertPrincipal(ctx, principal); err != nil {
			t.Fatalf("UpsertPrincipal returned error: %v", err)
		}
	}
	if err := store.UpsertTenant(ctx, tenant); err != nil {
		t.Fatalf("UpsertTenant returned error: %v", err)
	}
	if err := store.UpsertMembership(ctx, ownerMembership); err != nil {
		t.Fatalf("UpsertMembership returned error: %v", err)
	}
	if err := store.UpsertTenantInvitation(ctx, invitation); err != nil {
		t.Fatalf("UpsertTenantInvitation returned error: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	store, err = NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore reopen returned error: %v", err)
	}
	defer func() { _ = store.Close() }()
	memberships, err := store.ListMemberships(ctx, identity.MembershipFilter{TenantID: tenant.TenantID, Status: identity.StatusActive})
	if err != nil {
		t.Fatalf("ListMemberships returned error: %v", err)
	}
	if len(memberships) != 1 || !sameMembership(memberships[0], ownerMembership) {
		t.Fatalf("expected persisted org membership %+v, got %+v", ownerMembership, memberships)
	}
	invitations, err := store.ListTenantInvitations(ctx, identity.InvitationFilter{TenantID: tenant.TenantID, PrincipalID: invited.PrincipalID, Status: identity.StatusInvited})
	if err != nil {
		t.Fatalf("ListTenantInvitations returned error: %v", err)
	}
	if len(invitations) != 1 || !sameInvitation(invitations[0], invitation) {
		t.Fatalf("expected persisted org invitation %+v, got %+v", invitation, invitations)
	}
}

func TestSQLiteStorePersistsAccessTokenLifecycleFieldsAcrossRestart(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dataDir := t.TempDir()
	now := time.Now().UTC().Truncate(0)
	lastUsedAt := now.Add(time.Minute)
	expiresAt := now.Add(time.Hour)
	revokedAt := now.Add(2 * time.Hour)

	store, err := NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	token := auth.AccessToken{
		TokenID:            "tok_lifecycle",
		PrincipalID:        "prn_local",
		Label:              "local",
		Mode:               auth.PairingModeToken,
		TokenHash:          "hash",
		TokenPreview:       "dope_preview",
		Status:             "revoked",
		DefaultTenantID:    "ten_personal",
		CreatedAt:          now,
		UpdatedAt:          now,
		LastUsedAt:         &lastUsedAt,
		ExpiresAt:          &expiresAt,
		RevokedAt:          &revokedAt,
		RotatedFromTokenID: "tok_old",
		RotatedToTokenID:   "tok_new",
	}
	if err := store.UpsertAccessToken(ctx, token); err != nil {
		t.Fatalf("UpsertAccessToken returned error: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	store, err = NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore reopen returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	tokens, err := store.ListAccessTokens(ctx)
	if err != nil {
		t.Fatalf("ListAccessTokens returned error: %v", err)
	}
	if len(tokens) != 1 || !sameAccessToken(tokens[0], token) {
		t.Fatalf("expected persisted token %+v, got %+v", token, tokens)
	}
}

func TestSQLiteStoreUpgradesLegacyAuthTokensWithActiveStatus(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, defaultDatabaseFile)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	for _, stmt := range schemaMigrations[0].Statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("db.Exec legacy baseline statement returned error: %v", err)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`
		INSERT INTO auth_tokens (token_id, label, mode, token_hash, token_preview, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "tok_legacy", "legacy", string(auth.PairingModeLocal), "hash", "dope_preview", now, now); err != nil {
		t.Fatalf("db.Exec insert legacy token returned error: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	store, err := NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	tokens, err := store.ListAccessTokens(context.Background())
	if err != nil {
		t.Fatalf("ListAccessTokens returned error: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected one legacy token, got %+v", tokens)
	}
	if tokens[0].Status != "active" {
		t.Fatalf("expected migrated legacy token to be active, got %+v", tokens[0])
	}
}

func samePrincipal(a, b identity.Principal) bool {
	return a.PrincipalID == b.PrincipalID &&
		a.PrincipalKind == b.PrincipalKind &&
		a.DisplayName == b.DisplayName &&
		a.Status == b.Status &&
		a.DefaultTenantID == b.DefaultTenantID &&
		a.CreatedAt.Equal(b.CreatedAt) &&
		a.UpdatedAt.Equal(b.UpdatedAt) &&
		sameOptionalTime(a.DisabledAt, b.DisabledAt) &&
		sameOptionalTime(a.RemovedAt, b.RemovedAt)
}

func sameTenant(a, b identity.Tenant) bool {
	return a.TenantID == b.TenantID &&
		a.TenantKind == b.TenantKind &&
		a.DisplayName == b.DisplayName &&
		a.Status == b.Status &&
		a.CreatedAt.Equal(b.CreatedAt) &&
		a.UpdatedAt.Equal(b.UpdatedAt) &&
		a.CreatedByPrincipalID == b.CreatedByPrincipalID &&
		a.DefaultOwnerPrincipalID == b.DefaultOwnerPrincipalID
}

func sameMembership(a, b identity.Membership) bool {
	return a.MembershipID == b.MembershipID &&
		a.TenantID == b.TenantID &&
		a.PrincipalID == b.PrincipalID &&
		a.Role == b.Role &&
		a.Status == b.Status &&
		a.InvitationID == b.InvitationID &&
		a.CreatedAt.Equal(b.CreatedAt) &&
		a.UpdatedAt.Equal(b.UpdatedAt) &&
		sameOptionalTime(a.AcceptedAt, b.AcceptedAt) &&
		sameOptionalTime(a.RemovedAt, b.RemovedAt)
}

func sameInvitation(a, b identity.TenantInvitation) bool {
	return a.InvitationID == b.InvitationID &&
		a.TenantID == b.TenantID &&
		a.InvitedPrincipalID == b.InvitedPrincipalID &&
		a.InvitedByPrincipalID == b.InvitedByPrincipalID &&
		a.Role == b.Role &&
		a.Status == b.Status &&
		a.CreatedAt.Equal(b.CreatedAt) &&
		a.UpdatedAt.Equal(b.UpdatedAt) &&
		sameOptionalTime(a.ExpiresAt, b.ExpiresAt) &&
		sameOptionalTime(a.DecidedAt, b.DecidedAt)
}

func sameGrant(a, b identity.TokenTenantGrant) bool {
	return a.GrantID == b.GrantID &&
		a.TokenID == b.TokenID &&
		a.TenantID == b.TenantID &&
		a.IsDefault == b.IsDefault &&
		a.Status == b.Status &&
		a.CreatedAt.Equal(b.CreatedAt) &&
		a.UpdatedAt.Equal(b.UpdatedAt) &&
		sameOptionalTime(a.RevokedAt, b.RevokedAt) &&
		a.GrantedByPrincipalID == b.GrantedByPrincipalID
}

func sameAccessToken(a, b auth.AccessToken) bool {
	return a.TokenID == b.TokenID &&
		a.PrincipalID == b.PrincipalID &&
		a.Label == b.Label &&
		a.Mode == b.Mode &&
		a.TokenHash == b.TokenHash &&
		a.TokenPreview == b.TokenPreview &&
		a.Status == b.Status &&
		a.DefaultTenantID == b.DefaultTenantID &&
		a.CreatedAt.Equal(b.CreatedAt) &&
		a.UpdatedAt.Equal(b.UpdatedAt) &&
		sameOptionalTime(a.LastUsedAt, b.LastUsedAt) &&
		sameOptionalTime(a.ExpiresAt, b.ExpiresAt) &&
		sameOptionalTime(a.RevokedAt, b.RevokedAt) &&
		a.RotatedFromTokenID == b.RotatedFromTokenID &&
		a.RotatedToTokenID == b.RotatedToTokenID
}

func sameOptionalTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}
