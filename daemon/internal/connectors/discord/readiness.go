package discord

import (
	"strings"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

type ReadinessState string

const (
	ReadinessHostedReady         ReadinessState = "hosted_ready"
	ReadinessDegradedNeedsRepair ReadinessState = "degraded_needs_repair"
	ReadinessFailed              ReadinessState = "failed"
	ReadinessDisabled            ReadinessState = "disabled"
)

type CredentialState string

const (
	CredentialMissing             CredentialState = "missing"
	CredentialSubmitted           CredentialState = "submitted"
	CredentialValid               CredentialState = "valid"
	CredentialInvalid             CredentialState = "invalid"
	CredentialRevoked             CredentialState = "revoked"
	CredentialRedactionSuppressed CredentialState = "redaction_suppressed"
)

type HostedSetupInput struct {
	TenantID       string
	ConnectorID    string
	DisplayName    string
	Credential     CredentialState
	RespondInDM    bool
	RequireMention bool
	DeliveryMode   string
	Destinations   []DestinationValidation
	ValidatedAt    time.Time
}

type HostedSetup struct {
	TenantID           string                         `json:"tenantId,omitempty"`
	ConnectorID        string                         `json:"connectorId"`
	ConnectorKind      string                         `json:"connectorKind"`
	DisplayName        string                         `json:"displayName"`
	Status             baseconnectors.LifecycleState  `json:"status"`
	ReadinessState     ReadinessState                 `json:"readinessState"`
	HostedReady        bool                           `json:"hostedReady"`
	CredentialState    CredentialState                `json:"credentialState"`
	RespondInDM        bool                           `json:"respondInDM"`
	RequireMention     bool                           `json:"requireMention"`
	DeliveryMode       string                         `json:"deliveryMode"`
	ReasonCode         string                         `json:"reasonCode,omitempty"`
	Destinations       []DestinationValidation        `json:"destinations,omitempty"`
	CreatedAt          time.Time                      `json:"createdAt,omitempty"`
	UpdatedAt          time.Time                      `json:"updatedAt,omitempty"`
	ValidatedAt        time.Time                      `json:"validatedAt,omitempty"`
	RedactionStatus    baseconnectors.RedactionStatus `json:"redactionStatus"`
	RetentionExpiresAt time.Time                      `json:"retentionExpiresAt,omitempty"`
}

func EvaluateHostedSetup(input HostedSetupInput) HostedSetup {
	now := input.ValidatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	mode := strings.TrimSpace(input.DeliveryMode)
	if mode == "" {
		mode = "gateway"
	}
	setup := HostedSetup{
		TenantID:           strings.TrimSpace(input.TenantID),
		ConnectorID:        strings.TrimSpace(input.ConnectorID),
		ConnectorKind:      "discord",
		DisplayName:        strings.TrimSpace(input.DisplayName),
		Status:             baseconnectors.LifecycleStateHealthy,
		ReadinessState:     ReadinessHostedReady,
		HostedReady:        true,
		CredentialState:    input.Credential,
		RespondInDM:        input.RespondInDM,
		RequireMention:     input.RequireMention,
		DeliveryMode:       mode,
		Destinations:       normalizeDestinationEvidence(input.TenantID, input.ConnectorID, input.Destinations, now),
		CreatedAt:          now,
		UpdatedAt:          now,
		ValidatedAt:        now,
		RedactionStatus:    baseconnectors.RedactionStatusRedacted,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
	}
	switch input.Credential {
	case CredentialValid:
		if !hasExplicitHostedDestination(setup.Destinations) {
			setup.Status = baseconnectors.LifecycleStateDegraded
			setup.ReadinessState = ReadinessDegradedNeedsRepair
			setup.HostedReady = false
			setup.ReasonCode = "missing_explicit_destination"
		} else if !selectedDestinationsValid(setup.Destinations) {
			setup.Status = baseconnectors.LifecycleStateDegraded
			setup.ReadinessState = ReadinessDegradedNeedsRepair
			setup.HostedReady = false
			setup.ReasonCode = "destination_validation_failed"
		} else {
			setup.ReasonCode = "healthy"
		}
	case CredentialMissing, "":
		setup.Status = baseconnectors.LifecycleStateFailed
		setup.ReadinessState = ReadinessFailed
		setup.HostedReady = false
		setup.CredentialState = CredentialMissing
		setup.ReasonCode = string(baseconnectors.DiagnosticAuthMissing)
	case CredentialInvalid, CredentialRevoked:
		setup.Status = baseconnectors.LifecycleStateFailed
		setup.ReadinessState = ReadinessFailed
		setup.HostedReady = false
		setup.ReasonCode = string(baseconnectors.DiagnosticAuthMissing)
	case CredentialRedactionSuppressed:
		setup.Status = baseconnectors.LifecycleStateFailed
		setup.ReadinessState = ReadinessFailed
		setup.HostedReady = false
		setup.RedactionStatus = baseconnectors.RedactionStatusSuppressed
		setup.ReasonCode = string(baseconnectors.DiagnosticUnknownConnectorFailure)
	default:
		setup.Status = baseconnectors.LifecycleStateFailed
		setup.ReadinessState = ReadinessFailed
		setup.HostedReady = false
		setup.ReasonCode = string(baseconnectors.DiagnosticUnknownConnectorFailure)
	}
	return setup
}

func normalizeDestinationEvidence(tenantID, connectorID string, destinations []DestinationValidation, now time.Time) []DestinationValidation {
	items := make([]DestinationValidation, 0, len(destinations))
	for _, destination := range destinations {
		if destination.ValidatedAt.IsZero() {
			destination.ValidatedAt = now
		}
		if destination.RedactionStatus == "" {
			destination.RedactionStatus = baseconnectors.RedactionStatusRedacted
		}
		if strings.TrimSpace(destination.TenantID) == "" {
			destination.TenantID = strings.TrimSpace(tenantID)
		}
		if strings.TrimSpace(destination.ConnectorID) == "" {
			destination.ConnectorID = strings.TrimSpace(connectorID)
		}
		if destination.ValidationState == "" {
			destination.ValidationState = DestinationInvalid
		}
		if destination.ReasonCode == "" {
			if destination.ValidationState == DestinationValid {
				destination.ReasonCode = "healthy"
			} else {
				destination.ReasonCode = string(baseconnectors.DiagnosticBlockedRoute)
			}
		}
		items = append(items, destination)
	}
	return items
}
