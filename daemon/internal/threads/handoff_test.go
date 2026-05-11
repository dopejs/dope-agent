package threads

import (
	"errors"
	"testing"
	"time"
)

func TestValidateHandoffRequiresSeparateDestinationThreadAndPermission(t *testing.T) {
	t.Parallel()
	link := HandoffLink{
		SourceThreadID:      "thr_source",
		DestinationThreadID: "thr_destination",
	}
	if err := ValidateHandoff(HandoffValidationInput{
		Link:                         link,
		HasMutationPermission:        true,
		SourceEligible:               true,
		DestinationEligible:          true,
		SourcePermissionAllowed:      true,
		DestinationPermissionAllowed: true,
	}); err != nil {
		t.Fatalf("ValidateHandoff valid: %v", err)
	}
	link.DestinationThreadID = "thr_source"
	if err := ValidateHandoff(HandoffValidationInput{
		Link:                         link,
		HasMutationPermission:        true,
		SourceEligible:               true,
		DestinationEligible:          true,
		SourcePermissionAllowed:      true,
		DestinationPermissionAllowed: true,
	}); !errors.Is(err, ErrHandoffSameThread) {
		t.Fatalf("expected same thread error, got %v", err)
	}
	link.DestinationThreadID = "thr_destination"
	if err := ValidateHandoff(HandoffValidationInput{
		Link:                         link,
		SourceEligible:               true,
		DestinationEligible:          true,
		SourcePermissionAllowed:      true,
		DestinationPermissionAllowed: true,
	}); !errors.Is(err, ErrHandoffPermissionDenied) {
		t.Fatalf("expected permission error, got %v", err)
	}
}

func TestBuildHandoffSourceReferencesExcludesResetBoundaryAndUnsafeTurns(t *testing.T) {
	t.Parallel()
	link := HandoffLink{
		HandoffLinkID:               "handoff_1",
		TenantID:                    "ten_1",
		SourceThreadID:              "thr_source",
		SourceSessionSegmentID:      "seg_current",
		DestinationThreadID:         "thr_dest",
		DestinationSessionSegmentID: "seg_dest",
	}
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	refs := BuildHandoffSourceReferences(link, []ContinuityTurn{
		{ContinuityTurnID: "turn_1", SessionSegmentID: "seg_current", SafeContent: "eligible", ContentRedactionStatus: RedactionStatusRedacted, RetentionExpiresAt: now.Add(time.Hour)},
		{ContinuityTurnID: "turn_old", SessionSegmentID: "seg_old", SafeContent: "old", ContentRedactionStatus: RedactionStatusRedacted, RetentionExpiresAt: now.Add(time.Hour)},
		{ContinuityTurnID: "turn_unsafe", SessionSegmentID: "seg_current", SafeContent: "unsafe", ContentRedactionStatus: RedactionStatusSuppressed, RetentionExpiresAt: now.Add(time.Hour)},
		{ContinuityTurnID: "turn_expired", SessionSegmentID: "seg_current", SafeContent: "expired", ContentRedactionStatus: RedactionStatusRedacted, RetentionExpiresAt: now.Add(-time.Hour)},
	}, now)
	if refs[0].Decision != HandoffReferenceDecisionReferenced || refs[0].EligibilityStatus != HandoffReferenceEligible {
		t.Fatalf("eligible ref = %#v", refs[0])
	}
	if refs[1].Decision != HandoffReferenceDecisionExcluded || refs[1].EligibilityStatus != HandoffReferenceResetBoundary {
		t.Fatalf("reset-boundary ref = %#v", refs[1])
	}
	if refs[2].Decision != HandoffReferenceDecisionExcluded || refs[2].EligibilityStatus != HandoffReferenceRedactionFailed {
		t.Fatalf("unsafe ref = %#v", refs[2])
	}
	if refs[3].Decision != HandoffReferenceDecisionExcluded || refs[3].EligibilityStatus != HandoffReferenceRetentionExpired {
		t.Fatalf("expired ref = %#v", refs[3])
	}
}

func TestBuildHandoffSourceReferencesExcludesCrossRoomTurns(t *testing.T) {
	t.Parallel()
	link := HandoffLink{
		HandoffLinkID:          "handoff_1",
		TenantID:               "ten_1",
		SourceThreadID:         "thr_source",
		SourceSessionSegmentID: "seg_current",
		DestinationThreadID:    "thr_dest",
	}
	refs := BuildHandoffSourceReferences(link, []ContinuityTurn{
		{ContinuityTurnID: "turn_cross_room", ThreadID: "thr_other_room", SessionSegmentID: "seg_current", SafeContent: "other room", ContentRedactionStatus: RedactionStatusRedacted},
	}, time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC))
	if len(refs) != 1 || refs[0].Decision != HandoffReferenceDecisionExcluded || refs[0].EligibilityStatus != HandoffReferenceIncompleteEvidence {
		t.Fatalf("cross-room ref = %#v", refs)
	}
}
