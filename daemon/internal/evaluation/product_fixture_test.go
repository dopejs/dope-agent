package evaluation

import (
	"errors"
	"testing"
	"time"
)

func TestProductFixtureLifecycleCreateReviewSuppressRetention(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	candidate := fixtureCandidate(now)
	evidence := fixtureEvidence(now)

	fixture, revision, err := CreateProductFixtureFromCandidate(ProductFixtureInput{
		TenantID:        "ten_eval",
		DisplayName:     "Schedule Product Fixture",
		DomainClass:     FixtureDomainSchedule,
		SourceCandidate: candidate,
		SourceEvidence:  evidence,
		FixturePayload:  map[string]any{"goal": "schedule follow-up"},
		ChangeSummary:   "initial product fixture",
		CreatedBy:       "prn_eval",
	}, now)
	if err != nil {
		t.Fatalf("CreateProductFixtureFromCandidate: %v", err)
	}
	if fixture.ReviewState != ProductStatusDraft || fixture.CurrentRevisionID != revision.RevisionID {
		t.Fatalf("unexpected fixture/revision state: fixture=%+v revision=%+v", fixture, revision)
	}
	if revision.RevisionNumber != 1 || revision.SourceEvidenceRefs[0] != evidence.EvidenceID {
		t.Fatalf("revision lost provenance: %+v", revision)
	}

	revised, secondRevision, err := CreateProductFixtureRevision(fixture, FixtureRevisionInput{
		FixturePayload:     map[string]any{"goal": "schedule revised follow-up"},
		ContentSummary:     "revised schedule fixture",
		ChangeSummary:      "tighten expectation",
		SourceEvidenceRefs: []string{evidence.EvidenceID},
		RedactionStatus:    RedactionStatusRedacted,
		CreatedBy:          "prn_eval",
	}, 2, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("CreateProductFixtureRevision: %v", err)
	}
	if secondRevision.RevisionNumber != 2 || revised.CurrentRevisionID != secondRevision.RevisionID || revised.ReviewState != ProductStatusDraft {
		t.Fatalf("revision update failed: fixture=%+v revision=%+v", revised, secondRevision)
	}

	approved, err := ReviewProductFixture(revised, secondRevision.RevisionID, FixtureReviewApproved, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("ReviewProductFixture: %v", err)
	}
	if err := ProductFixtureSelectable(approved); err != nil {
		t.Fatalf("approved fixture should be selectable: %v", err)
	}

	suppressed, err := SuppressProductFixture(approved, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("SuppressProductFixture: %v", err)
	}
	if err := ProductFixtureSelectable(suppressed); !errors.Is(err, ErrEvaluationProductFixtureNotSelectable) {
		t.Fatalf("suppressed selectable err=%v, want not selectable", err)
	}

	deleted, err := ApplyProductFixtureRetention(approved, RetentionStateDeleted, now.Add(4*time.Minute))
	if err != nil {
		t.Fatalf("ApplyProductFixtureRetention: %v", err)
	}
	if deleted.ReviewState != ProductStatusDeleted || deleted.RetentionState != RetentionStateDeleted {
		t.Fatalf("deleted fixture state not applied: %+v", deleted)
	}
}

func TestProductFixtureCreationRejectsSuppressedExpiredOrFailedEvidence(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	candidate := fixtureCandidate(now)
	candidate.SuppressionState = SuppressionStateSuppressed
	_, _, err := CreateProductFixtureFromCandidate(ProductFixtureInput{
		TenantID:        "ten_eval",
		DisplayName:     "Suppressed",
		DomainClass:     FixtureDomainSchedule,
		SourceCandidate: candidate,
		SourceEvidence:  fixtureEvidence(now),
		FixturePayload:  map[string]any{},
	}, now)
	if !errors.Is(err, ErrEvaluationProductFixtureNotSelectable) {
		t.Fatalf("err=%v, want not selectable", err)
	}

	candidate = fixtureCandidate(now)
	evidence := fixtureEvidence(now)
	evidence.MaterializationAllowed = false
	_, _, err = CreateProductFixtureFromCandidate(ProductFixtureInput{
		TenantID:        "ten_eval",
		DisplayName:     "Failed Redaction",
		DomainClass:     FixtureDomainSchedule,
		SourceCandidate: candidate,
		SourceEvidence:  evidence,
		FixturePayload:  map[string]any{},
	}, now)
	if !errors.Is(err, ErrEvaluationProductFixtureNotSelectable) {
		t.Fatalf("err=%v, want not selectable", err)
	}
}

func fixtureCandidate(now time.Time) DiscoveredCandidate {
	return DiscoveredCandidate{
		DiscoveredCandidateID: "candidate_1",
		TenantID:              "ten_eval",
		DiscoveryRunID:        "discovery_run_1",
		SourceKind:            SourceKindRun,
		SourceID:              "run_1",
		SourceRefs:            []SourceRef{{Kind: SourceKindRun, ID: "run_1", Route: "/v1/runs/run_1"}},
		Score:                 0.9,
		ScoreBand:             ScoreBandHigh,
		RedactionStatus:       RedactionStatusRedacted,
		ReadinessStatus:       ReadinessFullyReplayable,
		SuppressionState:      SuppressionStateNone,
		RetentionState:        RetentionStateActive,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
}

func fixtureEvidence(now time.Time) CandidateEvidence {
	return CandidateEvidence{
		EvidenceID:             "evidence_1",
		TenantID:               "ten_eval",
		DiscoveredCandidateID:  "candidate_1",
		RedactedPayload:        map[string]any{"goal": "safe"},
		MaterializationAllowed: true,
		RetentionState:         RetentionStateActive,
		CreatedAt:              now,
	}
}
