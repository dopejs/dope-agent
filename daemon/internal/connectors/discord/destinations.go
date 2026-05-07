package discord

import (
	"strings"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

type DestinationType string

const (
	DestinationGuild         DestinationType = "guild"
	DestinationChannel       DestinationType = "channel"
	DestinationDirectMessage DestinationType = "direct_message"
)

type DestinationValidationState string

const (
	DestinationValid                 DestinationValidationState = "valid"
	DestinationInvalid               DestinationValidationState = "invalid"
	DestinationMissingPermission     DestinationValidationState = "missing_permission"
	DestinationMessageContentMissing DestinationValidationState = "message_content_missing"
	DestinationBotNotMember          DestinationValidationState = "bot_not_member"
	DestinationNotFound              DestinationValidationState = "not_found"
	DestinationDMRestricted          DestinationValidationState = "dm_restricted"
	DestinationStale                 DestinationValidationState = "stale"
)

type DestinationValidation struct {
	TenantID        string                         `json:"tenantId,omitempty"`
	ConnectorID     string                         `json:"connectorId,omitempty"`
	DestinationID   string                         `json:"destinationId"`
	DestinationType DestinationType                `json:"destinationType"`
	ProviderLabel   string                         `json:"providerLabel,omitempty"`
	Selected        bool                           `json:"selected"`
	ValidationState DestinationValidationState     `json:"validationState"`
	ReasonCode      string                         `json:"reasonCode,omitempty"`
	ValidatedAt     time.Time                      `json:"validatedAt"`
	RedactionStatus baseconnectors.RedactionStatus `json:"redactionStatus"`
	SafeEvidence    map[string]string              `json:"safeEvidence,omitempty"`
}

func hasExplicitHostedDestination(destinations []DestinationValidation) bool {
	for _, destination := range destinations {
		if !destination.Selected {
			continue
		}
		switch destination.DestinationType {
		case DestinationGuild, DestinationChannel:
			if strings.TrimSpace(destination.DestinationID) != "" {
				return true
			}
		}
	}
	return false
}

func selectedDestinationsValid(destinations []DestinationValidation) bool {
	if len(destinations) == 0 {
		return false
	}
	for _, destination := range destinations {
		if !destination.Selected {
			continue
		}
		if destination.ValidationState != DestinationValid {
			return false
		}
	}
	return true
}
