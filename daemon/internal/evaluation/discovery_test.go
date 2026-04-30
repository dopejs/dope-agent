package evaluation

import (
	"errors"
	"testing"
	"time"
)

func TestBuildDiscoveryRunFromPolicyValidatesBoundsAndIdempotency(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	policy := DiscoveryPolicy{
		PolicyID:             "policy_1",
		TenantID:             "ten_eval",
		Enabled:              true,
		SourceKinds:          []SourceKind{SourceKindRun},
		WindowStart:          now.Add(-time.Hour),
		WindowEnd:            now,
		MaxInspectedRecords:  10,
		MaxEmittedCandidates: 2,
		CostBudget:           5,
	}

	run, err := BuildDiscoveryRunFromPolicy(policy, StartDiscoveryRunInput{
		StartedBy:      "prn_eval",
		Cursor:         "cursor_1",
		IdempotencyKey: "idem_1",
	}, now)
	if err != nil {
		t.Fatalf("BuildDiscoveryRunFromPolicy: %v", err)
	}
	if run.TenantID != policy.TenantID || run.PolicyID != policy.PolicyID || run.Status != ProductStatusQueued {
		t.Fatalf("unexpected run identity/status: %+v", run)
	}
	if run.Cursor != "cursor_1" || run.IdempotencyKey != "idem_1" {
		t.Fatalf("cursor/idempotency not preserved: %+v", run)
	}
	if DiscoveryIdempotencyScope(run) != "ten_eval:idem_1" {
		t.Fatalf("unexpected idempotency scope: %s", DiscoveryIdempotencyScope(run))
	}
}

func TestBuildDiscoveryRunFromPolicyRejectsInvalidBounds(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	_, err := BuildDiscoveryRunFromPolicy(DiscoveryPolicy{
		PolicyID:             "policy_bad",
		TenantID:             "ten_eval",
		Enabled:              true,
		WindowStart:          now,
		WindowEnd:            now.Add(-time.Hour),
		MaxInspectedRecords:  10,
		MaxEmittedCandidates: 2,
		CostBudget:           5,
	}, StartDiscoveryRunInput{}, now)
	if !errors.Is(err, ErrEvaluationProductInvalidBounds) {
		t.Fatalf("err=%v, want ErrEvaluationProductInvalidBounds", err)
	}
}

func TestApplyDiscoveryRunProgressMarksPartialAtBounds(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	run := DiscoveryRun{
		DiscoveryRunID:       "run_1",
		TenantID:             "ten_eval",
		Status:               ProductStatusRunning,
		MaxInspectedRecords:  10,
		MaxEmittedCandidates: 2,
		UpdatedAt:            now,
	}

	updated := ApplyDiscoveryRunProgress(run, DiscoveryProgress{InspectedRecords: 10, EmittedCandidates: 1, Cursor: "next"}, now.Add(time.Minute))
	if updated.Status != ProductStatusPartial || updated.PartialReason != DiscoveryPartialReasonMaxInspectedRecords {
		t.Fatalf("expected max-inspected partial, got %+v", updated)
	}
	if updated.Cursor != "next" || updated.UpdatedAt != now.Add(time.Minute) {
		t.Fatalf("progress did not update cursor/time: %+v", updated)
	}
}
