package evaluation

import (
	"testing"
	"time"
)

func TestNewSuppressionRecordDefaultsAndRequiresTarget(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	record, err := NewSuppressionRecord(CreateSuppressionInput{
		TenantID:   "ten_eval",
		TargetKind: ProductResourceDiscoveredCandidate,
		TargetID:   "candidate_1",
		CreatedBy:  "prn_eval",
	}, now)
	if err != nil {
		t.Fatalf("NewSuppressionRecord: %v", err)
	}
	if record.SuppressionID != "suppression_discovered_candidate_candidate_1" || record.ReasonCode != "operator_hidden" || !record.Active {
		t.Fatalf("unexpected suppression defaults: %+v", record)
	}

	if _, err := NewSuppressionRecord(CreateSuppressionInput{TenantID: "ten_eval", TargetKind: ProductResourceDiscoveredCandidate}, now); err == nil {
		t.Fatal("expected missing target error")
	}
}

func TestSuppressionMatchesTargetAndSourceFamilies(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	candidate := DiscoveredCandidate{
		DiscoveredCandidateID: "candidate_1",
		TenantID:              "ten_eval",
		SourceKind:            SourceKindRun,
		SourceID:              "run_1",
	}
	records := []SuppressionRecord{
		{TenantID: "ten_eval", TargetKind: ProductResourceDiscoveredCandidate, TargetID: "candidate_2", Active: true, CreatedAt: now},
		{TenantID: "ten_eval", TargetKind: ProductResourceDiscoveredCandidate, TargetID: "candidate_1", Active: true, CreatedAt: now},
	}
	if !SuppressionApplies(candidate, records, now) {
		t.Fatal("candidate target suppression did not apply")
	}

	sourceFamily := []SuppressionRecord{
		{TenantID: "ten_eval", TargetKind: ProductResourceDiscoveryRun, TargetSourceRef: "run:run_1", Active: true, CreatedAt: now},
	}
	if !SuppressionApplies(candidate, sourceFamily, now) {
		t.Fatal("source family suppression did not apply")
	}
}

func TestSuppressionIgnoresInactiveExpiredAndCrossTenantRecords(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	expiredAt := now.Add(-time.Minute)
	candidate := DiscoveredCandidate{
		DiscoveredCandidateID: "candidate_1",
		TenantID:              "ten_eval",
		SourceKind:            SourceKindRun,
		SourceID:              "run_1",
	}
	records := []SuppressionRecord{
		{TenantID: "ten_eval", TargetKind: ProductResourceDiscoveredCandidate, TargetID: "candidate_1", Active: false, CreatedAt: now},
		{TenantID: "ten_eval", TargetKind: ProductResourceDiscoveredCandidate, TargetID: "candidate_1", Active: true, ExpiresAt: &expiredAt, CreatedAt: now},
		{TenantID: "ten_other", TargetKind: ProductResourceDiscoveredCandidate, TargetID: "candidate_1", Active: true, CreatedAt: now},
	}
	if SuppressionApplies(candidate, records, now) {
		t.Fatal("inactive, expired, or cross-tenant suppression applied")
	}
}

func TestSuppressionLookupRevocationAndCandidateFiltering(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	candidates := []DiscoveredCandidate{
		{DiscoveredCandidateID: "candidate_1", TenantID: "ten_eval", SourceKind: SourceKindRun, SourceID: "run_1"},
		{DiscoveredCandidateID: "candidate_2", TenantID: "ten_eval", SourceKind: SourceKindWorkflow, SourceID: "workflow_1"},
	}
	record, err := NewSuppressionRecord(CreateSuppressionInput{
		SuppressionID:   "suppression_1",
		TenantID:        "ten_eval",
		TargetKind:      ProductResourceDiscoveredCandidate,
		TargetSourceRef: "run:run_1",
		ReasonCode:      "operator_hidden",
	}, now)
	if err != nil {
		t.Fatalf("NewSuppressionRecord: %v", err)
	}

	if _, ok := FindActiveSuppression([]SuppressionRecord{record}, "ten_eval", "suppression_1", now); !ok {
		t.Fatal("active suppression was not found")
	}
	filtered := FilterSuppressedCandidates(candidates, []SuppressionRecord{record}, now)
	if len(filtered) != 1 || filtered[0].DiscoveredCandidateID != "candidate_2" {
		t.Fatalf("filtered candidates=%+v, want candidate_2 only", filtered)
	}

	revoked := RevokeSuppressionRecord(record, now.Add(time.Minute))
	if _, ok := FindActiveSuppression([]SuppressionRecord{revoked}, "ten_eval", "suppression_1", now.Add(2*time.Minute)); ok {
		t.Fatal("revoked suppression should not be active")
	}
	filtered = FilterSuppressedCandidates(candidates, []SuppressionRecord{revoked}, now.Add(2*time.Minute))
	if len(filtered) != 2 {
		t.Fatalf("revoked suppression filtered candidates: %+v", filtered)
	}
}
