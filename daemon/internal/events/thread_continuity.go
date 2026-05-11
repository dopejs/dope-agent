package events

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

const (
	ThreadContinuityTurnRecordedName    = "thread.continuity_turn_recorded"
	ThreadContinuityPreviewRecordedName = "thread.continuity_preview_recorded"
)

func ThreadContinuityTurnRecordedEvent(turn threads.ContinuityTurn, outcome string) Event {
	if outcome == "" {
		outcome = "recorded"
	}
	occurredAt := turn.RecordedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		TenantID:   turn.TenantID,
		Category:   "thread",
		Name:       ThreadContinuityTurnRecordedName,
		OccurredAt: occurredAt.UTC(),
		Scope: Scope{
			SessionID: turn.SessionSegmentID,
		},
		Resource: Resource{
			Kind: "thread_continuity_turn",
			ID:   turn.ContinuityTurnID,
		},
		Payload: map[string]any{
			"tenantId":           turn.TenantID,
			"threadId":           turn.ThreadID,
			"sessionSegmentId":   turn.SessionSegmentID,
			"continuityTurnId":   turn.ContinuityTurnID,
			"dispatchId":         turn.DispatchID,
			"action":             "turn_recorded",
			"outcome":            outcome,
			"reasonCode":         "included_recent",
			"redactionStatus":    string(turn.ContentRedactionStatus),
			"acceptanceSequence": turn.AcceptanceSequence,
		},
	}
}

func ThreadContinuityPreviewRecordedEvent(preview threads.ContinuityPreview) Event {
	occurredAt := preview.AssemblyCompletedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		TenantID:   preview.TenantID,
		Category:   "thread",
		Name:       ThreadContinuityPreviewRecordedName,
		OccurredAt: occurredAt.UTC(),
		Scope: Scope{
			SessionID: preview.SessionSegmentID,
		},
		Resource: Resource{
			Kind: "thread_continuity_preview",
			ID:   preview.ContinuityPreviewID,
		},
		Payload: map[string]any{
			"tenantId":            preview.TenantID,
			"threadId":            preview.ThreadID,
			"sessionSegmentId":    preview.SessionSegmentID,
			"continuityPreviewId": preview.ContinuityPreviewID,
			"dispatchId":          preview.DispatchID,
			"action":              "preview_recorded",
			"outcome":             string(preview.Status),
			"reasonCode":          preview.FailureClass,
			"redactionStatus":     string(preview.RedactionStatus),
		},
	}
}
