package store_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Roadmap 35 (T065a) — per-tenant uniqueness verification (SC-006).
//
// Phase 3 introduces two persistent UNIQUE constraints under
// `(tenant_id, ...)` via schema migration v33 (T027):
//   - uq_memberships_tenant_principal
//   - uq_token_grants_tenant_token
//
// The remaining Roadmap 35 per-tenant UNIQUE constraints (on
// schedules, integrations, calendar_accounts, mail_accounts,
// mcp_tool_exposure_rules, evaluation_regression_fixtures) are added
// via shadow swap in T077a (Phase 4). They are exercised by the
// uniqueness verification that T077a installs alongside the swap.
//
// This file verifies the Phase 3 set: tenants A and B can each insert
// the same `(principal_id)` and `(token_id)` natural keys, and within
// a single tenant a duplicate insert MUST fail with a SQLite UNIQUE
// violation.

func TestUniqueness_MembershipsPerTenant(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	now := time.Now().UTC()

	// Create the tenants and the principal first so the FKs hold.
	for _, tenantID := range []string{"ten_aaaa1111", "ten_bbbb2222"} {
		if err := s.UpsertTenant(ctx, identity.Tenant{
			TenantID:    tenantID,
			TenantKind:  identity.TenantKindPersonal,
			DisplayName: "tenant " + tenantID,
			Status:      identity.StatusActive,
			CreatedAt:   now,
			UpdatedAt:   now,
		}); err != nil {
			t.Fatalf("UpsertTenant %s: %v", tenantID, err)
		}
	}
	for _, principalID := range []string{"prn_p1", "prn_p2"} {
		if err := s.UpsertPrincipal(ctx, identity.Principal{
			PrincipalID:     principalID,
			PrincipalKind:   identity.PrincipalKindLocalOperator,
			DisplayName:     "p " + principalID,
			Status:          identity.StatusActive,
			DefaultTenantID: "ten_aaaa1111",
			CreatedAt:       now,
			UpdatedAt:       now,
		}); err != nil {
			t.Fatalf("UpsertPrincipal %s: %v", principalID, err)
		}
	}

	upsert := func(tenantID, principalID, membershipID string) error {
		return s.UpsertMembership(ctx, identity.Membership{
			MembershipID: membershipID,
			TenantID:     tenantID,
			PrincipalID:  principalID,
			Role:         identity.RoleOwner,
			Status:       identity.StatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
			AcceptedAt:   &now,
		})
	}

	// Same (principal) under different tenants MUST succeed.
	if err := upsert("ten_aaaa1111", "prn_p1", "mem_1"); err != nil {
		t.Fatalf("tenant A insert: %v", err)
	}
	if err := upsert("ten_bbbb2222", "prn_p1", "mem_2"); err != nil {
		t.Fatalf("tenant B insert with same principal MUST succeed (per-tenant scope), got %v", err)
	}

	// Duplicate within same tenant + principal MUST violate the
	// UNIQUE constraint introduced by T027.
	err = upsert("ten_aaaa1111", "prn_p1", "mem_3")
	if err == nil {
		t.Fatalf("duplicate (tenant_id, principal_id) MUST fail with UNIQUE violation, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unique") {
		t.Fatalf("expected UNIQUE violation, got %v", err)
	}
}

func TestUniqueness_TokenGrantsPerTenant(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	now := time.Now().UTC()

	for _, tenantID := range []string{"ten_aaaa1111", "ten_bbbb2222"} {
		if err := s.UpsertTenant(ctx, identity.Tenant{
			TenantID:    tenantID,
			TenantKind:  identity.TenantKindPersonal,
			DisplayName: "tenant",
			Status:      identity.StatusActive,
			CreatedAt:   now,
			UpdatedAt:   now,
		}); err != nil {
			t.Fatalf("UpsertTenant: %v", err)
		}
	}

	upsert := func(tenantID, tokenID, grantID string) error {
		return s.UpsertTokenTenantGrant(ctx, identity.TokenTenantGrant{
			GrantID:   grantID,
			TokenID:   tokenID,
			TenantID:  tenantID,
			IsDefault: false,
			Status:    identity.StatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	if err := upsert("ten_aaaa1111", "tok_xyz", "grt_1"); err != nil {
		t.Fatalf("tenant A grant: %v", err)
	}
	if err := upsert("ten_bbbb2222", "tok_xyz", "grt_2"); err != nil {
		t.Fatalf("tenant B grant for same token MUST succeed, got %v", err)
	}

	err = upsert("ten_aaaa1111", "tok_xyz", "grt_3")
	if err == nil {
		t.Fatalf("duplicate (tenant_id, token_id) MUST fail with UNIQUE violation, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unique") {
		t.Fatalf("expected UNIQUE violation, got %v", err)
	}
}
