package threads

import (
	"errors"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/profiles"
)

type HandoffStatus string

const (
	HandoffStatusSucceeded   HandoffStatus = "succeeded"
	HandoffStatusDenied      HandoffStatus = "denied"
	HandoffStatusFailed      HandoffStatus = "failed_closed"
	HandoffStatusUnsupported HandoffStatus = "unsupported"
	HandoffStatusExpired     HandoffStatus = "expired"
)

type HandoffSourceReferenceStatus string

const (
	HandoffSourceReferenceAvailable HandoffSourceReferenceStatus = "available"
	HandoffSourceReferenceConsumed  HandoffSourceReferenceStatus = "consumed"
	HandoffSourceReferenceBlocked   HandoffSourceReferenceStatus = "blocked"
	HandoffSourceReferenceExpired   HandoffSourceReferenceStatus = "expired"
	HandoffSourceReferenceNone      HandoffSourceReferenceStatus = "none"
)

type HandoffSourceReferenceEligibility string

const (
	HandoffReferenceEligible           HandoffSourceReferenceEligibility = "eligible"
	HandoffReferencePermissionDenied   HandoffSourceReferenceEligibility = "permission_denied"
	HandoffReferenceRedactionFailed    HandoffSourceReferenceEligibility = "redaction_failed"
	HandoffReferenceRetentionExpired   HandoffSourceReferenceEligibility = "retention_expired"
	HandoffReferenceResetBoundary      HandoffSourceReferenceEligibility = "reset_boundary"
	HandoffReferenceIncompleteEvidence HandoffSourceReferenceEligibility = "incomplete_evidence"
	HandoffReferenceUnsupported        HandoffSourceReferenceEligibility = "unsupported"
)

type HandoffSourceReferenceDecision string

const (
	HandoffReferenceDecisionReferenced HandoffSourceReferenceDecision = "referenced"
	HandoffReferenceDecisionExcluded   HandoffSourceReferenceDecision = "excluded"
	HandoffReferenceDecisionConsumed   HandoffSourceReferenceDecision = "consumed"
)

var (
	ErrHandoffSameThread       = errors.New("handoff source and destination threads must be different")
	ErrHandoffPermissionDenied = errors.New("handoff requires connectors.manage and source/destination permission")
	ErrHandoffNotEligible      = errors.New("handoff source or destination is not eligible")
)

type HandoffLink struct {
	HandoffLinkID                string                       `json:"handoffLinkId,omitempty"`
	TenantID                     string                       `json:"tenantId,omitempty"`
	SourceThreadID               string                       `json:"sourceThreadId"`
	SourceSessionSegmentID       string                       `json:"sourceSessionSegmentId,omitempty"`
	DestinationThreadID          string                       `json:"destinationThreadId"`
	DestinationSessionSegmentID  string                       `json:"destinationSessionSegmentId,omitempty"`
	SourceConversationShape      ConversationShape            `json:"sourceConversationShape"`
	DestinationConversationShape ConversationShape            `json:"destinationConversationShape"`
	SourceKind                   SourceKind                   `json:"sourceKind,omitempty"`
	DestinationKind              SourceKind                   `json:"destinationKind,omitempty"`
	SourceConnectorID            string                       `json:"sourceConnectorId,omitempty"`
	DestinationConnectorID       string                       `json:"destinationConnectorId,omitempty"`
	SourceConversationID         string                       `json:"sourceConversationId,omitempty"`
	DestinationConversationID    string                       `json:"destinationConversationId,omitempty"`
	ActorPrincipalID             string                       `json:"actorPrincipalId,omitempty"`
	PermissionGate               string                       `json:"permissionGate"`
	Status                       HandoffStatus                `json:"status"`
	ReasonCode                   string                       `json:"reasonCode,omitempty"`
	FirstDestinationResponseID   string                       `json:"firstDestinationResponseId,omitempty"`
	SourceReferenceStatus        HandoffSourceReferenceStatus `json:"sourceReferenceStatus"`
	ActiveProfileProjection      *profiles.RuntimeProjection  `json:"activeProfileProjection,omitempty"`
	CreatedAt                    time.Time                    `json:"createdAt,omitempty"`
	ConsumedAt                   time.Time                    `json:"consumedAt,omitempty"`
	RetentionExpiresAt           time.Time                    `json:"retentionExpiresAt,omitempty"`
	RedactionStatus              RedactionStatus              `json:"redactionStatus"`
}

type HandoffSourceReference struct {
	HandoffSourceReferenceID    string                            `json:"handoffSourceReferenceId,omitempty"`
	HandoffLinkID               string                            `json:"handoffLinkId"`
	TenantID                    string                            `json:"tenantId,omitempty"`
	SourceThreadID              string                            `json:"sourceThreadId"`
	SourceSessionSegmentID      string                            `json:"sourceSessionSegmentId,omitempty"`
	DestinationThreadID         string                            `json:"destinationThreadId"`
	DestinationSessionSegmentID string                            `json:"destinationSessionSegmentId,omitempty"`
	ContinuityTurnID            string                            `json:"continuityTurnId,omitempty"`
	ArtifactExcerptRef          string                            `json:"artifactExcerptRef,omitempty"`
	EligibilityStatus           HandoffSourceReferenceEligibility `json:"eligibilityStatus"`
	Decision                    HandoffSourceReferenceDecision    `json:"decision"`
	SafeSummary                 string                            `json:"safeSummary,omitempty"`
	RedactionStatus             RedactionStatus                   `json:"redactionStatus"`
	CreatedAt                   time.Time                         `json:"createdAt,omitempty"`
	ConsumedAt                  time.Time                         `json:"consumedAt,omitempty"`
	RetentionExpiresAt          time.Time                         `json:"retentionExpiresAt,omitempty"`
}

type HandoffValidationInput struct {
	Link                         HandoffLink
	HasMutationPermission        bool
	SourceEligible               bool
	DestinationEligible          bool
	SourcePermissionAllowed      bool
	DestinationPermissionAllowed bool
}

func ValidateHandoff(input HandoffValidationInput) error {
	if input.Link.SourceThreadID == "" || input.Link.DestinationThreadID == "" || input.Link.SourceThreadID == input.Link.DestinationThreadID {
		return ErrHandoffSameThread
	}
	if !input.HasMutationPermission || !input.SourcePermissionAllowed || !input.DestinationPermissionAllowed {
		return ErrHandoffPermissionDenied
	}
	if !input.SourceEligible || !input.DestinationEligible {
		return ErrHandoffNotEligible
	}
	return nil
}

func BuildHandoffSourceReferences(link HandoffLink, turns []ContinuityTurn, now time.Time) []HandoffSourceReference {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	refs := make([]HandoffSourceReference, 0, len(turns))
	for _, turn := range turns {
		eligibility := HandoffReferenceEligible
		decision := HandoffReferenceDecisionReferenced
		if turn.ThreadID != "" && turn.ThreadID != link.SourceThreadID {
			eligibility = HandoffReferenceIncompleteEvidence
			decision = HandoffReferenceDecisionExcluded
		}
		if turn.SessionSegmentID != link.SourceSessionSegmentID {
			eligibility = HandoffReferenceResetBoundary
			decision = HandoffReferenceDecisionExcluded
		}
		if turn.ContentRedactionStatus == RedactionStatusRedactionFailed || turn.ContentRedactionStatus == RedactionStatusSuppressed {
			eligibility = HandoffReferenceRedactionFailed
			decision = HandoffReferenceDecisionExcluded
		}
		if !turn.RetentionExpiresAt.IsZero() && !turn.RetentionExpiresAt.After(now) {
			eligibility = HandoffReferenceRetentionExpired
			decision = HandoffReferenceDecisionExcluded
		}
		refs = append(refs, HandoffSourceReference{
			HandoffLinkID:               link.HandoffLinkID,
			TenantID:                    link.TenantID,
			SourceThreadID:              link.SourceThreadID,
			SourceSessionSegmentID:      link.SourceSessionSegmentID,
			DestinationThreadID:         link.DestinationThreadID,
			DestinationSessionSegmentID: link.DestinationSessionSegmentID,
			ContinuityTurnID:            turn.ContinuityTurnID,
			EligibilityStatus:           eligibility,
			Decision:                    decision,
			SafeSummary:                 SafeGroupRoomEvidenceSummary(turn.SafeContent).Text,
			RedactionStatus:             RedactionStatusRedacted,
			CreatedAt:                   now,
			RetentionExpiresAt:          turn.RetentionExpiresAt,
		})
	}
	return refs
}
