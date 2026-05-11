package threads

import (
	"testing"
	"time"
)

func TestContinuityPolicySelectsDefaultWindowByAcceptanceSequence(t *testing.T) {
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	turns := make([]ContinuityTurn, 0, 14)
	for i := 1; i <= 14; i++ {
		turns = append(turns, ContinuityTurn{
			ContinuityTurnID:       "turn",
			TenantID:               "ten_1",
			ThreadID:               "thr_1",
			SessionSegmentID:       "seg_1",
			AcceptanceSequence:     int64(i),
			Role:                   ContinuityRoleUser,
			SourceKind:             SourceKindChat,
			SafeContent:            "safe",
			ContentRedactionStatus: RedactionStatusRedacted,
			RecordedAt:             now.Add(time.Duration(i) * time.Minute),
			RetentionExpiresAt:     now.Add(90 * 24 * time.Hour),
		})
	}
	included, excluded := EligibleContinuityTurns(turns, DefaultContinuityPolicy(), now)
	if len(included) != DefaultContinuityMaxPriorTurns {
		t.Fatalf("included=%d want=%d", len(included), DefaultContinuityMaxPriorTurns)
	}
	if included[0].AcceptanceSequence != 3 || included[len(included)-1].AcceptanceSequence != 14 {
		t.Fatalf("unexpected sequence window: first=%d last=%d", included[0].AcceptanceSequence, included[len(included)-1].AcceptanceSequence)
	}
	if len(excluded) != 2 || excluded[0].ReasonCode != ContinuityReasonOverLimit {
		t.Fatalf("unexpected exclusions: %+v", excluded)
	}
}

func TestContinuityPolicyExcludesAgeRetentionAndUnsafeRedaction(t *testing.T) {
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	turns := []ContinuityTurn{
		continuityTestTurn("old", 1, now.AddDate(0, 0, -31), now.Add(90*24*time.Hour), RedactionStatusRedacted),
		continuityTestTurn("expired", 2, now, now.Add(-time.Hour), RedactionStatusRedacted),
		continuityTestTurn("unsafe", 3, now, now.Add(90*24*time.Hour), RedactionStatusRedactionFailed),
		continuityTestTurn("ok", 4, now, now.Add(90*24*time.Hour), RedactionStatusRedacted),
	}
	included, excluded := EligibleContinuityTurns(turns, DefaultContinuityPolicy(), now)
	if len(included) != 1 || included[0].ContinuityTurnID != "ok" {
		t.Fatalf("included=%+v", included)
	}
	reasons := map[ContinuityReason]bool{}
	for _, item := range excluded {
		reasons[item.ReasonCode] = true
	}
	for _, reason := range []ContinuityReason{ContinuityReasonTooOld, ContinuityReasonRetentionExpired, ContinuityReasonRedactionFailed} {
		if !reasons[reason] {
			t.Fatalf("missing exclusion reason %s in %+v", reason, excluded)
		}
	}
}

func TestResetBoundaryPreviewItemsExcludePreResetTurns(t *testing.T) {
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	items := ResetBoundaryPreviewItems([]ContinuityTurn{
		continuityTestTurn("pre_reset_1", 1, now, now.Add(90*24*time.Hour), RedactionStatusRedacted),
		continuityTestTurn("pre_reset_2", 2, now, now.Add(90*24*time.Hour), RedactionStatusRedacted),
	}, 3)
	if len(items) != 2 {
		t.Fatalf("items=%d want 2", len(items))
	}
	if items[0].Decision != ContinuityDecisionExcluded || items[0].ReasonCode != ContinuityReasonResetBoundary || items[0].ItemOrder != 3 {
		t.Fatalf("unexpected first reset-boundary item: %+v", items[0])
	}
	if items[1].ContinuityTurnID != "pre_reset_2" || items[1].ItemOrder != 4 {
		t.Fatalf("unexpected second reset-boundary item: %+v", items[1])
	}
}

func continuityTestTurn(id string, seq int64, recordedAt, retentionExpiresAt time.Time, status RedactionStatus) ContinuityTurn {
	return ContinuityTurn{
		ContinuityTurnID:       id,
		TenantID:               "ten_1",
		ThreadID:               "thr_1",
		SessionSegmentID:       "seg_1",
		AcceptanceSequence:     seq,
		Role:                   ContinuityRoleUser,
		SourceKind:             SourceKindChat,
		SafeContent:            id,
		ContentRedactionStatus: status,
		RecordedAt:             recordedAt,
		RetentionExpiresAt:     retentionExpiresAt,
	}
}
