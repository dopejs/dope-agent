package threads

import (
	"errors"
	"strings"
	"time"
)

type ConversationShape string

const (
	ConversationShapeDirectMessage ConversationShape = "direct_message"
	ConversationShapeGroup         ConversationShape = "group"
	ConversationShapeRoom          ConversationShape = "room"
	ConversationShapeWeb           ConversationShape = "web"
	ConversationShapeUnknown       ConversationShape = "unknown"
	ConversationShapeUnsupported   ConversationShape = "unsupported"
)

type ShapeEvidenceStatus string

const (
	ShapeEvidenceStatusProven      ShapeEvidenceStatus = "proven"
	ShapeEvidenceStatusPartial     ShapeEvidenceStatus = "partial"
	ShapeEvidenceStatusUnsupported ShapeEvidenceStatus = "unsupported"
	ShapeEvidenceStatusFailed      ShapeEvidenceStatus = "failed"
)

type MentionStatus string

const (
	MentionStatusQualified   MentionStatus = "qualified"
	MentionStatusMissing     MentionStatus = "missing"
	MentionStatusAmbiguous   MentionStatus = "ambiguous"
	MentionStatusUnsupported MentionStatus = "unsupported"
	MentionStatusFailed      MentionStatus = "failed"
)

type AllowlistStatus string

const (
	AllowlistStatusEligible     AllowlistStatus = "eligible"
	AllowlistStatusNotAllowlist AllowlistStatus = "not_allowlisted"
	AllowlistStatusUnsupported  AllowlistStatus = "unsupported"
	AllowlistStatusFailed       AllowlistStatus = "failed"
)

type ParticipationDecisionValue string

const (
	ParticipationDecisionAccepted    ParticipationDecisionValue = "accepted"
	ParticipationDecisionIgnored     ParticipationDecisionValue = "ignored"
	ParticipationDecisionBlocked     ParticipationDecisionValue = "blocked"
	ParticipationDecisionDenied      ParticipationDecisionValue = "denied"
	ParticipationDecisionDuplicate   ParticipationDecisionValue = "duplicate"
	ParticipationDecisionUnsupported ParticipationDecisionValue = "unsupported"
	ParticipationDecisionFailed      ParticipationDecisionValue = "failed"
)

type ResetEventStatus string

const (
	ResetEventStatusSucceeded    ResetEventStatus = "succeeded"
	ResetEventStatusDenied       ResetEventStatus = "denied"
	ResetEventStatusFailedClosed ResetEventStatus = "failed_closed"
	ResetEventStatusUnsupported  ResetEventStatus = "unsupported"
)

const (
	GroupRoomReasonAcceptedQualifyingMention    = "accepted_qualifying_mention"
	GroupRoomReasonMissingQualifyingMention     = "missing_qualifying_mention"
	GroupRoomReasonNotAllowlisted               = "not_allowlisted"
	GroupRoomReasonPermissionDenied             = "permission_denied"
	GroupRoomReasonDuplicateSourceEvent         = "duplicate_source_event"
	GroupRoomReasonUnsupportedConversationShape = "unsupported_conversation_shape"
	GroupRoomReasonRedactionFailed              = "redaction_failed"
	GroupRoomReasonIncompleteSourceIdentity     = "incomplete_source_identity"
	GroupRoomReasonConnectorDisabled            = "connector_disabled"
	GroupRoomReasonConnectorFailed              = "connector_failed"
	GroupRoomReasonScopedResetSucceeded         = "scoped_reset_succeeded"
)

var ErrInvalidConversationShape = errors.New("invalid conversation shape")

type ConversationShapeEvidence struct {
	ConversationShapeID       string              `json:"conversationShapeId,omitempty"`
	TenantID                  string              `json:"tenantId,omitempty"`
	ThreadID                  string              `json:"threadId,omitempty"`
	SessionSegmentID          string              `json:"sessionSegmentId,omitempty"`
	Shape                     ConversationShape   `json:"shape"`
	SourceKind                SourceKind          `json:"sourceKind,omitempty"`
	ConnectorID               string              `json:"connectorId,omitempty"`
	ConnectorKind             string              `json:"connectorKind,omitempty"`
	SourceAccountID           string              `json:"sourceAccountId,omitempty"`
	SourceConversationID      string              `json:"sourceConversationId,omitempty"`
	SourceConversationSummary string              `json:"sourceConversationSummary,omitempty"`
	ParticipantSummary        string              `json:"participantSummary,omitempty"`
	ShapeEvidenceStatus       ShapeEvidenceStatus `json:"shapeEvidenceStatus"`
	RecordedAt                time.Time           `json:"recordedAt,omitempty"`
	UpdatedAt                 time.Time           `json:"updatedAt,omitempty"`
	RetentionExpiresAt        time.Time           `json:"retentionExpiresAt,omitempty"`
	RedactionStatus           RedactionStatus     `json:"redactionStatus"`
}

type ParticipationPolicy struct {
	ParticipationPolicyID     string            `json:"participationPolicyId,omitempty"`
	TenantID                  string            `json:"tenantId,omitempty"`
	ConnectorID               string            `json:"connectorId,omitempty"`
	ConnectorKind             string            `json:"connectorKind,omitempty"`
	SourceAccountID           string            `json:"sourceAccountId,omitempty"`
	SourceConversationID      string            `json:"sourceConversationId,omitempty"`
	Shape                     ConversationShape `json:"shape"`
	AllowlistEligible         bool              `json:"allowlistEligible"`
	QualifyingMentionRequired bool              `json:"qualifyingMentionRequired"`
	PolicyStatus              string            `json:"policyStatus"`
	ConfiguredByPrincipalID   string            `json:"configuredByPrincipalId,omitempty"`
	ConfiguredAt              time.Time         `json:"configuredAt,omitempty"`
	UpdatedAt                 time.Time         `json:"updatedAt,omitempty"`
	RetentionExpiresAt        time.Time         `json:"retentionExpiresAt,omitempty"`
	RedactionStatus           RedactionStatus   `json:"redactionStatus"`
}

type ParticipationDecision struct {
	ParticipationDecisionID string                     `json:"participationDecisionId,omitempty"`
	TenantID                string                     `json:"tenantId,omitempty"`
	ThreadID                string                     `json:"threadId,omitempty"`
	SessionSegmentID        string                     `json:"sessionSegmentId,omitempty"`
	ConnectorID             string                     `json:"connectorId,omitempty"`
	ConnectorKind           string                     `json:"connectorKind,omitempty"`
	SourceAccountID         string                     `json:"sourceAccountId,omitempty"`
	SourceConversationID    string                     `json:"sourceConversationId,omitempty"`
	SourceMessageID         string                     `json:"sourceMessageId,omitempty"`
	ConversationShape       ConversationShape          `json:"conversationShape"`
	PolicyID                string                     `json:"policyId,omitempty"`
	MentionStatus           MentionStatus              `json:"mentionStatus"`
	AllowlistStatus         AllowlistStatus            `json:"allowlistStatus"`
	Decision                ParticipationDecisionValue `json:"decision"`
	ReasonCode              string                     `json:"reasonCode"`
	CreatedAssistantWork    bool                       `json:"createdAssistantWork"`
	OccurredAt              time.Time                  `json:"occurredAt,omitempty"`
	RetentionExpiresAt      time.Time                  `json:"retentionExpiresAt,omitempty"`
	RedactionStatus         RedactionStatus            `json:"redactionStatus"`
	SafeSummary             string                     `json:"safeSummary,omitempty"`
}

type ResetEvent struct {
	ResetEventID              string            `json:"resetEventId,omitempty"`
	TenantID                  string            `json:"tenantId,omitempty"`
	ThreadID                  string            `json:"threadId,omitempty"`
	ConversationShape         ConversationShape `json:"conversationShape"`
	SourceConversationID      string            `json:"sourceConversationId,omitempty"`
	ActorPrincipalID          string            `json:"actorPrincipalId,omitempty"`
	PermissionGate            string            `json:"permissionGate"`
	PriorSessionSegmentID     string            `json:"priorSessionSegmentId,omitempty"`
	ResultingSessionSegmentID string            `json:"resultingSessionSegmentId,omitempty"`
	Status                    ResetEventStatus  `json:"status"`
	ReasonCode                string            `json:"reasonCode"`
	RequestedAt               time.Time         `json:"requestedAt,omitempty"`
	CompletedAt               time.Time         `json:"completedAt,omitempty"`
	AuditEventID              string            `json:"auditEventId,omitempty"`
	RetentionExpiresAt        time.Time         `json:"retentionExpiresAt,omitempty"`
	RedactionStatus           RedactionStatus   `json:"redactionStatus"`
}

type ParticipationEvaluationInput struct {
	Shape             ConversationShape
	AllowlistEligible bool
	QualifyingMention bool
	PermissionAllowed bool
	Duplicate         bool
	Unsupported       bool
	RedactionAllowed  bool
	OccurredAt        time.Time
	SafeSummary       string
}

type ConversationShapeResolutionInput struct {
	TenantID                  string
	ThreadID                  string
	SessionSegmentID          string
	SourceKind                SourceKind
	ConnectorID               string
	ConnectorKind             string
	SourceAccountID           string
	SourceConversationID      string
	SourceConversationSummary string
	ClaimedShape              ConversationShape
	Now                       time.Time
}

func NormalizeConversationShape(shape ConversationShape) (ConversationShape, error) {
	switch ConversationShape(strings.TrimSpace(string(shape))) {
	case ConversationShapeDirectMessage:
		return ConversationShapeDirectMessage, nil
	case ConversationShapeGroup:
		return ConversationShapeGroup, nil
	case ConversationShapeRoom:
		return ConversationShapeRoom, nil
	case ConversationShapeWeb:
		return ConversationShapeWeb, nil
	case ConversationShapeUnknown:
		return ConversationShapeUnknown, nil
	case ConversationShapeUnsupported:
		return ConversationShapeUnsupported, nil
	default:
		return "", ErrInvalidConversationShape
	}
}

func ResolveConversationShape(input ConversationShapeResolutionInput) ConversationShapeEvidence {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	shape := input.ClaimedShape
	status := ShapeEvidenceStatusProven
	switch {
	case shape == "":
		switch input.SourceKind {
		case SourceKindShell:
			shape = ConversationShapeWeb
		case SourceKindLegacy:
			shape = ConversationShapeUnknown
			status = ShapeEvidenceStatusPartial
		default:
			shape = ConversationShapeUnsupported
			status = ShapeEvidenceStatusUnsupported
		}
	case shape == ConversationShapeUnknown:
		status = ShapeEvidenceStatusPartial
	case shape == ConversationShapeUnsupported:
		status = ShapeEvidenceStatusUnsupported
	}
	return ConversationShapeEvidence{
		TenantID:                  input.TenantID,
		ThreadID:                  input.ThreadID,
		SessionSegmentID:          input.SessionSegmentID,
		Shape:                     shape,
		SourceKind:                input.SourceKind,
		ConnectorID:               input.ConnectorID,
		ConnectorKind:             input.ConnectorKind,
		SourceAccountID:           input.SourceAccountID,
		SourceConversationID:      strings.TrimSpace(input.SourceConversationID),
		SourceConversationSummary: SafeGroupRoomEvidenceSummary(input.SourceConversationSummary).Text,
		ShapeEvidenceStatus:       status,
		RecordedAt:                now,
		UpdatedAt:                 now,
		RedactionStatus:           RedactionStatusRedacted,
	}
}

func DefaultParticipationPolicy(shape ConversationShape) ParticipationPolicy {
	return ParticipationPolicy{
		Shape:                     shape,
		QualifyingMentionRequired: true,
		PolicyStatus:              "enabled",
		RedactionStatus:           RedactionStatusRedacted,
	}
}

func EvaluateParticipation(input ParticipationEvaluationInput) ParticipationDecision {
	decision := ParticipationDecision{
		ConversationShape: input.Shape,
		MentionStatus:     MentionStatusQualified,
		AllowlistStatus:   AllowlistStatusEligible,
		Decision:          ParticipationDecisionAccepted,
		ReasonCode:        GroupRoomReasonAcceptedQualifyingMention,
		CreatedAssistantWork: input.Shape == ConversationShapeGroup ||
			input.Shape == ConversationShapeRoom,
		OccurredAt:      input.OccurredAt,
		RedactionStatus: RedactionStatusRedacted,
		SafeSummary:     SafeGroupRoomEvidenceSummary(input.SafeSummary).Text,
	}
	if input.OccurredAt.IsZero() {
		decision.OccurredAt = time.Now().UTC()
	}
	switch {
	case input.Unsupported || (input.Shape != ConversationShapeGroup && input.Shape != ConversationShapeRoom):
		decision.MentionStatus = MentionStatusUnsupported
		decision.AllowlistStatus = AllowlistStatusUnsupported
		decision.Decision = ParticipationDecisionUnsupported
		decision.ReasonCode = GroupRoomReasonUnsupportedConversationShape
		decision.CreatedAssistantWork = false
	case !input.RedactionAllowed:
		decision.Decision = ParticipationDecisionFailed
		decision.ReasonCode = GroupRoomReasonRedactionFailed
		decision.RedactionStatus = RedactionStatusSuppressed
		decision.SafeSummary = "suppressed"
		decision.CreatedAssistantWork = false
	case input.Duplicate:
		decision.Decision = ParticipationDecisionDuplicate
		decision.ReasonCode = GroupRoomReasonDuplicateSourceEvent
		decision.CreatedAssistantWork = false
	case !input.PermissionAllowed:
		decision.Decision = ParticipationDecisionDenied
		decision.ReasonCode = GroupRoomReasonPermissionDenied
		decision.CreatedAssistantWork = false
	case !input.AllowlistEligible:
		decision.AllowlistStatus = AllowlistStatusNotAllowlist
		decision.Decision = ParticipationDecisionBlocked
		decision.ReasonCode = GroupRoomReasonNotAllowlisted
		decision.CreatedAssistantWork = false
	case !input.QualifyingMention:
		decision.MentionStatus = MentionStatusMissing
		decision.Decision = ParticipationDecisionIgnored
		decision.ReasonCode = GroupRoomReasonMissingQualifyingMention
		decision.CreatedAssistantWork = false
	}
	return decision
}

func BuildScopedResetEvent(action LifecycleAction, shape ConversationShapeEvidence) ResetEvent {
	status := ResetEventStatusSucceeded
	reason := action.ReasonCode
	if strings.TrimSpace(reason) == "" {
		reason = GroupRoomReasonScopedResetSucceeded
	}
	conversationShape := shape.Shape
	if conversationShape == "" {
		conversationShape = ConversationShapeUnknown
		status = ResetEventStatusUnsupported
		reason = GroupRoomReasonUnsupportedConversationShape
	}
	if conversationShape == ConversationShapeUnknown || conversationShape == ConversationShapeUnsupported || shape.ShapeEvidenceStatus == ShapeEvidenceStatusUnsupported {
		status = ResetEventStatusUnsupported
		reason = GroupRoomReasonUnsupportedConversationShape
	}
	return ResetEvent{
		TenantID:                  action.TenantID,
		ThreadID:                  action.ThreadID,
		ConversationShape:         conversationShape,
		SourceConversationID:      strings.TrimSpace(shape.SourceConversationID),
		ActorPrincipalID:          action.ActorPrincipalID,
		PermissionGate:            "connectors.manage",
		PriorSessionSegmentID:     action.PriorSessionSegmentID,
		ResultingSessionSegmentID: action.ResultingSessionSegment,
		Status:                    status,
		ReasonCode:                reason,
		RequestedAt:               action.RequestedAt,
		CompletedAt:               action.CompletedAt,
		AuditEventID:              action.AuditEventID,
		RedactionStatus:           RedactionStatusRedacted,
	}
}
