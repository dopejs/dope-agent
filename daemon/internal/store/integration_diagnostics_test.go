package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestIntegrationDiagnosticStorePersistsLatestStateAndRetention(t *testing.T) {
	t.Parallel()

	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC()
	run := integrations.DiagnosticRun{
		DiagnosticRunID:     "diag_run_store_1",
		TenantID:            "ten_diag",
		IntegrationID:       "integration_diag",
		RequestedBy:         "operator",
		Trigger:             "operator_inspection",
		Status:              integrations.DiagnosticRunCompleted,
		StartedAt:           now,
		CheckedCapabilities: []string{"calendar.read"},
		ResultIDs:           []string{"diag_result_store_1"},
		RedactionStatus:     integrations.RedactionStatusRedacted,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
		IdempotencyKey:      "client_key_1",
	}
	if err := s.SaveIntegrationDiagnosticRun(context.Background(), run); err != nil {
		t.Fatalf("SaveIntegrationDiagnosticRun: %v", err)
	}
	result := integrations.DiagnosticResult{
		DiagnosticResultID: "diag_result_store_1",
		TenantID:           "ten_diag",
		IntegrationID:      "integration_diag",
		DomainKind:         "calendar",
		ProviderKind:       "feishu_lark",
		Capability:         "calendar.read",
		Status:             integrations.DiagnosticStatusHealthy,
		ReasonCode:         integrations.ReasonHealthy,
		RemediationOwner:   integrations.RemediationOwnerNoneRequired,
		RetrySafety:        integrations.RetrySafetyNoActionNeeded,
		CheckedAt:          now,
		StaleAfter:         now.Add(integrations.DiagnosticStaleAfter),
		FreshnessState:     integrations.DiagnosticFreshness(now, now.Add(integrations.DiagnosticStaleAfter)),
		RunID:              run.DiagnosticRunID,
		RedactionStatus:    integrations.RedactionStatusRedacted,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
	}
	if err := s.SaveIntegrationDiagnosticResult(context.Background(), result); err != nil {
		t.Fatalf("SaveIntegrationDiagnosticResult: %v", err)
	}
	got, err := s.LatestIntegrationDiagnosticResults(context.Background(), integrations.DiagnosticResultFilter{
		TenantID:      "ten_diag",
		IntegrationID: "integration_diag",
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("LatestIntegrationDiagnosticResults: %v", err)
	}
	if len(got) != 1 || got[0].DiagnosticResultID != result.DiagnosticResultID || got[0].FreshnessState != integrations.FreshnessStateFresh {
		t.Fatalf("unexpected diagnostic results: %+v", got)
	}
}

func TestIntegrationDiagnosticStoreMarksStaleAndHidesExpiredResults(t *testing.T) {
	t.Parallel()

	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	oldCheckedAt := time.Now().UTC().Add(-20 * time.Minute)
	expiredCheckedAt := time.Now().UTC().Add(-2 * time.Hour)
	staleResult := integrations.DiagnosticResult{
		DiagnosticResultID: "diag_result_stale",
		TenantID:           "ten_diag",
		IntegrationID:      "integration_diag",
		DomainKind:         "calendar",
		ProviderKind:       "feishu_lark",
		Capability:         "calendar.read",
		Status:             integrations.DiagnosticStatusBlocked,
		ReasonCode:         integrations.ReasonScopeMissing,
		RemediationOwner:   integrations.RemediationOwnerTenantAdmin,
		RetrySafety:        integrations.RetrySafetyBlocked,
		CheckedAt:          oldCheckedAt,
		StaleAfter:         oldCheckedAt.Add(integrations.DiagnosticStaleAfter),
		FreshnessState:     integrations.FreshnessStateFresh,
		RedactionStatus:    integrations.RedactionStatusRedacted,
		RetentionExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	expiredResult := staleResult
	expiredResult.DiagnosticResultID = "diag_result_expired"
	expiredResult.CheckedAt = expiredCheckedAt
	expiredResult.StaleAfter = expiredCheckedAt.Add(integrations.DiagnosticStaleAfter)
	expiredResult.RetentionExpiresAt = time.Now().UTC().Add(-time.Minute)

	if err := s.SaveIntegrationDiagnosticResult(context.Background(), staleResult); err != nil {
		t.Fatalf("SaveIntegrationDiagnosticResult stale: %v", err)
	}
	if err := s.SaveIntegrationDiagnosticResult(context.Background(), expiredResult); err != nil {
		t.Fatalf("SaveIntegrationDiagnosticResult expired: %v", err)
	}

	visible, err := s.LatestIntegrationDiagnosticResults(context.Background(), integrations.DiagnosticResultFilter{
		TenantID:       "ten_diag",
		IntegrationID:  "integration_diag",
		IncludeExpired: false,
	})
	if err != nil {
		t.Fatalf("LatestIntegrationDiagnosticResults visible: %v", err)
	}
	if len(visible) != 1 || visible[0].DiagnosticResultID != staleResult.DiagnosticResultID {
		t.Fatalf("expected only retained stale result, got %+v", visible)
	}
	if visible[0].FreshnessState != integrations.FreshnessStateStale {
		t.Fatalf("expected stale freshness, got %+v", visible[0])
	}

	all, err := s.LatestIntegrationDiagnosticResults(context.Background(), integrations.DiagnosticResultFilter{
		TenantID:       "ten_diag",
		IntegrationID:  "integration_diag",
		IncludeExpired: true,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("LatestIntegrationDiagnosticResults include expired: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected retained and expired results when requested, got %+v", all)
	}
}

func TestIntegrationDiagnosticStoreListsAndGetsRunsByTenant(t *testing.T) {
	t.Parallel()

	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC()
	for _, run := range []integrations.DiagnosticRun{
		{DiagnosticRunID: "diag_run_a", TenantID: "ten_a", IntegrationID: "integration_a", RequestedBy: "operator", Trigger: "operator_inspection", Status: integrations.DiagnosticRunCompleted, StartedAt: now, RedactionStatus: integrations.RedactionStatusRedacted, RetentionExpiresAt: now.Add(time.Hour)},
		{DiagnosticRunID: "diag_run_b", TenantID: "ten_b", IntegrationID: "integration_b", RequestedBy: "operator", Trigger: "operator_inspection", Status: integrations.DiagnosticRunCompleted, StartedAt: now, RedactionStatus: integrations.RedactionStatusRedacted, RetentionExpiresAt: now.Add(time.Hour)},
	} {
		if err := s.SaveIntegrationDiagnosticRun(context.Background(), run); err != nil {
			t.Fatalf("SaveIntegrationDiagnosticRun %s: %v", run.DiagnosticRunID, err)
		}
	}

	items, err := s.ListIntegrationDiagnosticRuns(context.Background(), integrations.DiagnosticRunFilter{TenantID: "ten_a"})
	if err != nil {
		t.Fatalf("ListIntegrationDiagnosticRuns: %v", err)
	}
	if len(items) != 1 || items[0].DiagnosticRunID != "diag_run_a" {
		t.Fatalf("unexpected tenant scoped runs: %+v", items)
	}
	if _, ok, err := s.GetIntegrationDiagnosticRun(context.Background(), "ten_a", "diag_run_b", false); err != nil || ok {
		t.Fatalf("tenant A must not read tenant B run ok=%v err=%v", ok, err)
	}
}

func TestDiagnosticRetentionRecordsTrackExpiredEvidence(t *testing.T) {
	t.Parallel()

	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	createdAt := time.Now().UTC().Add(-91 * 24 * time.Hour)
	record := integrations.NewDiagnosticRetentionRecord("ten_diag", "diagnostic_run", "diag_run_expired", createdAt)
	if err := s.SaveDiagnosticRetentionRecord(context.Background(), record); err != nil {
		t.Fatalf("SaveDiagnosticRetentionRecord: %v", err)
	}
	expired, err := s.ExpiredDiagnosticRetentionRecords(context.Background(), "ten_diag", time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("ExpiredDiagnosticRetentionRecords: %v", err)
	}
	if len(expired) != 1 || expired[0].TargetID != "diag_run_expired" || expired[0].RetentionState != integrations.DiagnosticRetentionActive {
		t.Fatalf("unexpected expired retention records: %+v", expired)
	}
	applied, err := s.ApplyExpiredDiagnosticRetentionRecords(context.Background(), "ten_diag", time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("ApplyExpiredDiagnosticRetentionRecords: %v", err)
	}
	if len(applied) != 1 || applied[0].RetentionState != integrations.DiagnosticRetentionExpired || applied[0].AppliedAt == nil {
		t.Fatalf("expected expired retention record to be applied, got %+v", applied)
	}
	expired, err = s.ExpiredDiagnosticRetentionRecords(context.Background(), "ten_diag", time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("ExpiredDiagnosticRetentionRecords after apply: %v", err)
	}
	if len(expired) != 0 {
		t.Fatalf("expected applied retention records to be hidden from active expiry query, got %+v", expired)
	}
}
