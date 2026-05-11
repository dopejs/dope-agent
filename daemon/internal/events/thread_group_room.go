package events

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

const (
	ThreadConversationShapeRecordedName     = "thread.conversation_shape_recorded"
	ThreadParticipationDecisionRecordedName = "thread.participation_decision_recorded"
	ThreadResetScopedName                   = "thread.reset_scoped"
	ThreadHandoffLinkedName                 = "thread.handoff_linked"
)

func ThreadConversationShapeEvent(evidence threads.ConversationShapeEvidence) Event {
	occurredAt := evidence.RecordedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		TenantID:   evidence.TenantID,
		Category:   "thread",
		Name:       ThreadConversationShapeRecordedName,
		OccurredAt: occurredAt.UTC(),
		Scope: Scope{
			SessionID:   evidence.SessionSegmentID,
			ConnectorID: evidence.ConnectorID,
		},
		Resource: Resource{
			Kind: "thread_conversation_shape",
			ID:   evidence.ConversationShapeID,
		},
		Payload: map[string]any{
			"tenantId":            evidence.TenantID,
			"threadId":            evidence.ThreadID,
			"sessionSegmentId":    evidence.SessionSegmentID,
			"conversationShapeId": evidence.ConversationShapeID,
			"shape":               string(evidence.Shape),
			"shapeEvidenceStatus": string(evidence.ShapeEvidenceStatus),
			"redactionStatus":     string(evidence.RedactionStatus),
		},
	}
}

func ThreadParticipationDecisionEvent(decision threads.ParticipationDecision) Event {
	occurredAt := decision.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		TenantID:   decision.TenantID,
		Category:   "thread",
		Name:       ThreadParticipationDecisionRecordedName,
		OccurredAt: occurredAt.UTC(),
		Scope: Scope{
			SessionID:   decision.SessionSegmentID,
			ConnectorID: decision.ConnectorID,
		},
		Resource: Resource{
			Kind: "thread_participation_decision",
			ID:   decision.ParticipationDecisionID,
		},
		Payload: map[string]any{
			"tenantId":                decision.TenantID,
			"threadId":                decision.ThreadID,
			"sessionSegmentId":        decision.SessionSegmentID,
			"participationDecisionId": decision.ParticipationDecisionID,
			"conversationShape":       string(decision.ConversationShape),
			"decision":                string(decision.Decision),
			"reasonCode":              decision.ReasonCode,
			"createdAssistantWork":    decision.CreatedAssistantWork,
			"redactionStatus":         string(decision.RedactionStatus),
		},
	}
}

func ThreadScopedResetEvent(reset threads.LifecycleAction, shape threads.ConversationShape) Event {
	occurredAt := reset.CompletedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		TenantID:   reset.TenantID,
		Category:   "thread",
		Name:       ThreadResetScopedName,
		OccurredAt: occurredAt.UTC(),
		Scope: Scope{
			SessionID: reset.ResultingSessionSegment,
		},
		Resource: Resource{
			Kind: "thread_reset_scoped",
			ID:   reset.LifecycleActionID,
		},
		Payload: map[string]any{
			"tenantId":          reset.TenantID,
			"threadId":          reset.ThreadID,
			"sessionSegmentId":  reset.ResultingSessionSegment,
			"conversationShape": string(shape),
			"lifecycleActionId": reset.LifecycleActionID,
			"status":            reset.Status,
			"permissionGate":    "connectors.manage",
			"redactionStatus":   string(reset.RedactionStatus),
		},
	}
}

func ThreadScopedResetEvidenceEvent(reset threads.ResetEvent) Event {
	occurredAt := reset.CompletedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		TenantID:   reset.TenantID,
		Category:   "thread",
		Name:       ThreadResetScopedName,
		OccurredAt: occurredAt.UTC(),
		Scope: Scope{
			SessionID: reset.ResultingSessionSegmentID,
		},
		Resource: Resource{
			Kind: "thread_reset_scoped",
			ID:   reset.ResetEventID,
		},
		Payload: map[string]any{
			"tenantId":          reset.TenantID,
			"threadId":          reset.ThreadID,
			"sessionSegmentId":  reset.ResultingSessionSegmentID,
			"conversationShape": string(reset.ConversationShape),
			"resetEventId":      reset.ResetEventID,
			"status":            string(reset.Status),
			"reasonCode":        reset.ReasonCode,
			"permissionGate":    reset.PermissionGate,
			"redactionStatus":   string(reset.RedactionStatus),
		},
	}
}

func ThreadHandoffLinkedEvent(link threads.HandoffLink) Event {
	occurredAt := link.CreatedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		TenantID:   link.TenantID,
		Category:   "thread",
		Name:       ThreadHandoffLinkedName,
		OccurredAt: occurredAt.UTC(),
		Scope: Scope{
			SessionID:   link.DestinationSessionSegmentID,
			ConnectorID: link.DestinationConnectorID,
		},
		Resource: Resource{
			Kind: "thread_handoff_link",
			ID:   link.HandoffLinkID,
		},
		Payload: map[string]any{
			"tenantId":                     link.TenantID,
			"handoffLinkId":                link.HandoffLinkID,
			"sourceThreadId":               link.SourceThreadID,
			"destinationThreadId":          link.DestinationThreadID,
			"sourceConversationShape":      string(link.SourceConversationShape),
			"destinationConversationShape": string(link.DestinationConversationShape),
			"status":                       string(link.Status),
			"reasonCode":                   link.ReasonCode,
			"permissionGate":               link.PermissionGate,
			"sourceReferenceStatus":        string(link.SourceReferenceStatus),
			"redactionStatus":              string(link.RedactionStatus),
		},
	}
}
