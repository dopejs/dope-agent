package events

import (
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestThreadContinuityEventsAreMetadataOnly(t *testing.T) {
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	turnEvent := ThreadContinuityTurnRecordedEvent(threads.ContinuityTurn{
		ContinuityTurnID:       "turn_1",
		TenantID:               "ten_1",
		ThreadID:               "thr_1",
		SessionSegmentID:       "seg_1",
		AcceptanceSequence:     7,
		Role:                   threads.ContinuityRoleUser,
		SourceKind:             threads.SourceKindChat,
		ContentRedactionStatus: threads.RedactionStatusRedacted,
		RecordedAt:             now,
	}, "")
	if turnEvent.Name != ThreadContinuityTurnRecordedName || turnEvent.Payload["acceptanceSequence"] != int64(7) {
		t.Fatalf("unexpected turn event: %#v", turnEvent)
	}
	if _, leaked := turnEvent.Payload["safeContent"]; leaked {
		t.Fatalf("turn event leaked content: %#v", turnEvent.Payload)
	}

	previewEvent := ThreadContinuityPreviewRecordedEvent(threads.ContinuityPreview{
		ContinuityPreviewID: "contprev_1",
		TenantID:            "ten_1",
		ThreadID:            "thr_1",
		SessionSegmentID:    "seg_1",
		Status:              threads.ContinuityStatusApplied,
		AssemblyCompletedAt: now,
		RedactionStatus:     threads.RedactionStatusRedacted,
	})
	if previewEvent.Name != ThreadContinuityPreviewRecordedName || previewEvent.Payload["outcome"] != "applied" {
		t.Fatalf("unexpected preview event: %#v", previewEvent)
	}
	if _, leaked := previewEvent.Payload["items"]; leaked {
		t.Fatalf("preview event leaked item detail: %#v", previewEvent.Payload)
	}
}
