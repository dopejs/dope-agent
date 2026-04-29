package tenancy_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/store/tenancy"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestLiveValidationTenantHelpersRejectCrossTenantWrites(t *testing.T) {
	t.Parallel()

	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	accessor := tenancy.NewEvaluation(s, nil)
	ctxA := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_a", PrincipalID: "prn_a"})
	ctxB := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_b", PrincipalID: "prn_b"})
	now := time.Now().UTC()
	attempt := livevalidation.Attempt{
		ValidationID:     "lv_cross_tenant",
		CandidateID:      "candidate_a",
		RequestedBy:      "prn_a",
		EnvironmentScope: "test",
		Status:           livevalidation.AttemptStatusQueued,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := accessor.UpsertLiveValidationAttemptForTenant(ctxA, attempt); err != nil {
		t.Fatalf("UpsertLiveValidationAttemptForTenant A: %v", err)
	}
	attempt.CandidateID = "candidate_b"
	if err := accessor.UpsertLiveValidationAttemptForTenant(ctxB, attempt); !errors.Is(err, tenancy.ErrCrossTenantWrite) {
		t.Fatalf("cross-tenant attempt write err=%v, want ErrCrossTenantWrite", err)
	}
	got, ok, err := s.GetLiveValidationAttempt(context.Background(), "ten_a", attempt.ValidationID)
	if err != nil {
		t.Fatalf("GetLiveValidationAttempt: %v", err)
	}
	if !ok || got.CandidateID != "candidate_a" || got.TenantID != "ten_a" {
		t.Fatalf("cross-tenant write mutated tenant A row: ok=%v item=%+v", ok, got)
	}
}

func TestLiveValidationTenantHelpersBindRelatedRows(t *testing.T) {
	t.Parallel()

	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	accessor := tenancy.NewEvaluation(s, nil)
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_live", PrincipalID: "prn_live"})
	now := time.Now().UTC()
	attempt := livevalidation.Attempt{
		ValidationID:     "lv_tenant_bind",
		CandidateID:      "candidate_1",
		RequestedBy:      "prn_live",
		EnvironmentScope: "test",
		Status:           livevalidation.AttemptStatusQueued,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := accessor.UpsertLiveValidationAttemptForTenant(ctx, attempt); err != nil {
		t.Fatalf("UpsertLiveValidationAttemptForTenant: %v", err)
	}
	if err := accessor.AppendLiveValidationLedgerEntryForTenant(ctx, livevalidation.SideEffectLedgerEntry{
		LedgerEntryID: "lv_ledger_bind",
		ValidationID:  attempt.ValidationID,
		CandidateID:   attempt.CandidateID,
		SourceRef:     "tool_call_1",
		ToolClass:     livevalidation.ToolClassDaemonInspectionRead,
		SafetyClass:   livevalidation.SafetyClassReadOnly,
		ActionRef:     "action_1",
		Outcome:       livevalidation.LedgerOutcomeAttempted,
		AttemptedAt:   &now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("AppendLiveValidationLedgerEntryForTenant: %v", err)
	}
	tenantID, ok, err := s.LookupRowTenant(context.Background(), "live_validation_ledger_entries", "ledger_entry_id", "lv_ledger_bind")
	if err != nil {
		t.Fatalf("LookupRowTenant: %v", err)
	}
	if !ok || tenantID != "ten_live" {
		t.Fatalf("ledger row tenant=%q ok=%v, want ten_live", tenantID, ok)
	}
}
