package threads

import (
	"testing"
	"time"
)

func TestEvaluateParticipationRequiresAllowlistAndMention(t *testing.T) {
	t.Parallel()
	accepted := EvaluateParticipation(ParticipationEvaluationInput{
		Shape:             ConversationShapeRoom,
		AllowlistEligible: true,
		QualifyingMention: true,
		PermissionAllowed: true,
		RedactionAllowed:  true,
		SafeSummary:       "safe room mention",
	})
	if accepted.Decision != ParticipationDecisionAccepted || !accepted.CreatedAssistantWork {
		t.Fatalf("accepted decision = %#v", accepted)
	}
	missingMention := EvaluateParticipation(ParticipationEvaluationInput{
		Shape:             ConversationShapeRoom,
		AllowlistEligible: true,
		PermissionAllowed: true,
		RedactionAllowed:  true,
	})
	if missingMention.Decision != ParticipationDecisionIgnored || missingMention.ReasonCode != GroupRoomReasonMissingQualifyingMention || missingMention.CreatedAssistantWork {
		t.Fatalf("missing mention decision = %#v", missingMention)
	}
	notAllowlisted := EvaluateParticipation(ParticipationEvaluationInput{
		Shape:             ConversationShapeGroup,
		QualifyingMention: true,
		PermissionAllowed: true,
		RedactionAllowed:  true,
	})
	if notAllowlisted.Decision != ParticipationDecisionBlocked || notAllowlisted.ReasonCode != GroupRoomReasonNotAllowlisted || notAllowlisted.CreatedAssistantWork {
		t.Fatalf("not allowlisted decision = %#v", notAllowlisted)
	}
}

func TestResolveConversationShapePreservesStableRoomIdentity(t *testing.T) {
	t.Parallel()
	first := ResolveConversationShape(ConversationShapeResolutionInput{
		TenantID:                  "ten_1",
		ThreadID:                  "thr_room_1",
		SessionSegmentID:          "seg_1",
		SourceKind:                SourceKindChannel,
		ConnectorID:               "slack-main",
		ConnectorKind:             "slack",
		SourceAccountID:           "workspace_redacted",
		SourceConversationID:      "channel_a",
		SourceConversationSummary: "Slack / #support",
		ClaimedShape:              ConversationShapeRoom,
	})
	second := ResolveConversationShape(ConversationShapeResolutionInput{
		TenantID:                  "ten_1",
		ThreadID:                  "thr_room_2",
		SessionSegmentID:          "seg_2",
		SourceKind:                SourceKindChannel,
		ConnectorID:               "slack-main",
		ConnectorKind:             "slack",
		SourceAccountID:           "workspace_redacted",
		SourceConversationID:      "channel_b",
		SourceConversationSummary: "Slack / #support",
		ClaimedShape:              ConversationShapeRoom,
	})
	if first.Shape != ConversationShapeRoom || first.ShapeEvidenceStatus != ShapeEvidenceStatusProven {
		t.Fatalf("first shape evidence = %#v", first)
	}
	if first.SourceConversationID == second.SourceConversationID {
		t.Fatalf("room identity collapsed: %#v %#v", first, second)
	}
	unsupported := ResolveConversationShape(ConversationShapeResolutionInput{SourceKind: SourceKindChannel})
	if unsupported.Shape != ConversationShapeUnsupported || unsupported.ShapeEvidenceStatus != ShapeEvidenceStatusUnsupported {
		t.Fatalf("unsupported shape evidence = %#v", unsupported)
	}
}

func TestUnknownShapeDoesNotCreateParticipation(t *testing.T) {
	t.Parallel()
	decision := EvaluateParticipation(ParticipationEvaluationInput{
		Shape:             ConversationShapeUnknown,
		AllowlistEligible: true,
		QualifyingMention: true,
		PermissionAllowed: true,
		RedactionAllowed:  true,
	})
	if decision.Decision != ParticipationDecisionUnsupported || decision.CreatedAssistantWork {
		t.Fatalf("unknown shape decision = %#v", decision)
	}
}

func TestBuildScopedResetEventCapturesConversationShapeAndSourceScope(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	action := LifecycleAction{
		ThreadID:                "thr_reset",
		TenantID:                "ten_1",
		ActionKind:              LifecycleActionReset,
		ActorPrincipalID:        "prn_1",
		PriorSessionSegmentID:   "seg_old",
		ResultingSessionSegment: "seg_new",
		ReasonCode:              "operator_reset",
		RequestedAt:             now,
		CompletedAt:             now,
		Status:                  "succeeded",
		AuditEventID:            "audit_reset",
		RedactionStatus:         RedactionStatusRedacted,
	}
	for _, shape := range []ConversationShape{ConversationShapeDirectMessage, ConversationShapeGroup, ConversationShapeRoom, ConversationShapeWeb} {
		event := BuildScopedResetEvent(action, ConversationShapeEvidence{
			Shape:                     shape,
			ShapeEvidenceStatus:       ShapeEvidenceStatusProven,
			SourceConversationID:      "source_" + string(shape),
			RedactionStatus:           RedactionStatusRedacted,
			SourceConversationSummary: "safe summary",
		})
		if event.ConversationShape != shape || event.SourceConversationID == "" || event.Status != ResetEventStatusSucceeded {
			t.Fatalf("reset event for %s = %+v", shape, event)
		}
		if event.PermissionGate != "connectors.manage" || event.PriorSessionSegmentID != "seg_old" || event.ResultingSessionSegmentID != "seg_new" {
			t.Fatalf("reset event missing scope or permission evidence: %+v", event)
		}
	}
}

func TestBuildScopedResetEventFailsClosedForUnsupportedSourceShape(t *testing.T) {
	t.Parallel()

	event := BuildScopedResetEvent(LifecycleAction{
		ThreadID:                "thr_reset",
		TenantID:                "ten_1",
		ActionKind:              LifecycleActionReset,
		ResultingSessionSegment: "seg_new",
		AuditEventID:            "audit_reset",
	}, ConversationShapeEvidence{Shape: ConversationShapeUnsupported, ShapeEvidenceStatus: ShapeEvidenceStatusUnsupported})
	if event.Status != ResetEventStatusUnsupported || event.ReasonCode != GroupRoomReasonUnsupportedConversationShape {
		t.Fatalf("unsupported reset event = %+v", event)
	}
}
