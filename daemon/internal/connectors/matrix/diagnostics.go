package matrix

import (
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

type MatrixCondition string

const (
	MatrixConditionBotAuthInvalid           MatrixCondition = "bot_auth_invalid"
	MatrixConditionBotAuthRevoked           MatrixCondition = "bot_auth_revoked"
	MatrixConditionRoomPermissionMissing    MatrixCondition = "room_permission_missing"
	MatrixConditionOwnershipMismatch        MatrixCondition = "ownership_mismatch"
	MatrixConditionHomeserverUnsupported    MatrixCondition = "homeserver_unsupported"
	MatrixConditionHomeserverUnreachable    MatrixCondition = "homeserver_unreachable"
	MatrixConditionFederationFailed         MatrixCondition = "federation_failed"
	MatrixConditionRateLimited              MatrixCondition = "rate_limited"
	MatrixConditionProviderUnavailable      MatrixCondition = "provider_unavailable"
	MatrixConditionNetworkFailed            MatrixCondition = "network_failed"
	MatrixConditionBlockedRoute             MatrixCondition = "blocked_route"
	MatrixConditionDuplicateEvent           MatrixCondition = "duplicate_event"
	MatrixConditionEncryptedRoomUnsupported MatrixCondition = "encrypted_room_unsupported"
	MatrixConditionUndecryptableEvent       MatrixCondition = "undecryptable_event"
	MatrixConditionUnsupportedSurface       MatrixCondition = "unsupported_surface"
	MatrixConditionReplyFailed              MatrixCondition = "reply_failed"
	MatrixConditionUnknown                  MatrixCondition = "unknown"
)

type DiagnosticInput struct {
	TenantID          string
	ConnectorID       string
	EvidenceTimestamp time.Time
	Now               time.Time
	RedactionReliable bool
	SafeEvidence      map[string]string
}

type DiagnosticState struct {
	baseconnectors.ConnectorDiagnosticState
	MatrixCondition MatrixCondition `json:"matrixCondition"`
}

func MapCondition(condition MatrixCondition, input DiagnosticInput) DiagnosticState {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	evidenceAt := input.EvidenceTimestamp
	if evidenceAt.IsZero() {
		evidenceAt = now
	}
	redactionReliable := input.RedactionReliable
	if !redactionReliable {
		input.SafeEvidence = nil
	}
	base, _ := baseconnectors.ClassifyDiagnostic(baseconnectors.DiagnosticInput{
		TenantID:          input.TenantID,
		ConnectorID:       input.ConnectorID,
		ReasonCode:        reasonForCondition(condition),
		EvidenceTimestamp: evidenceAt,
		RedactionReliable: redactionReliable,
		SafeEvidence:      input.SafeEvidence,
	})
	base.FreshnessState = baseconnectors.FreshnessAt(evidenceAt, now)
	return DiagnosticState{ConnectorDiagnosticState: base, MatrixCondition: condition}
}

func reasonForCondition(condition MatrixCondition) baseconnectors.DiagnosticReasonCode {
	switch condition {
	case MatrixConditionBotAuthInvalid, MatrixConditionBotAuthRevoked:
		return baseconnectors.DiagnosticAuthMissing
	case MatrixConditionRoomPermissionMissing, MatrixConditionOwnershipMismatch:
		return baseconnectors.DiagnosticPermissionMissing
	case MatrixConditionHomeserverUnsupported, MatrixConditionEncryptedRoomUnsupported, MatrixConditionUndecryptableEvent, MatrixConditionUnsupportedSurface:
		return baseconnectors.DiagnosticUnsupportedCapability
	case MatrixConditionRateLimited:
		return baseconnectors.DiagnosticRateLimited
	case MatrixConditionProviderUnavailable:
		return baseconnectors.DiagnosticProviderUnavailable
	case MatrixConditionHomeserverUnreachable, MatrixConditionFederationFailed, MatrixConditionNetworkFailed:
		return baseconnectors.DiagnosticNetworkFailed
	case MatrixConditionBlockedRoute:
		return baseconnectors.DiagnosticBlockedRoute
	case MatrixConditionDuplicateEvent:
		return baseconnectors.DiagnosticDuplicateInbound
	case MatrixConditionReplyFailed:
		return baseconnectors.DiagnosticReplyFailed
	default:
		return baseconnectors.DiagnosticUnknownConnectorFailure
	}
}
