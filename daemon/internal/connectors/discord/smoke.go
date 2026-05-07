package discord

import (
	"strings"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

type SmokeStatus string

const (
	SmokePassed  SmokeStatus = "passed"
	SmokeFailed  SmokeStatus = "failed"
	SmokeSkipped SmokeStatus = "skipped"
)

type CredentialMode string

const (
	CredentialModeSafeLive    CredentialMode = "safe_live"
	CredentialModeFake        CredentialMode = "fake"
	CredentialModeUnavailable CredentialMode = "unavailable"
)

type SmokeInput struct {
	SmokeEvidenceID         string
	TenantID                string
	ConnectorID             string
	SafeCredentialsApproved bool
	Passed                  bool
	Owner                   string
	Reason                  string
	RemainingRisk           string
	ValidatedAt             time.Time
	SafeEvidence            map[string]string
}

type SmokeEvidence struct {
	SmokeEvidenceID    string                         `json:"smokeEvidenceId"`
	TenantID           string                         `json:"tenantId"`
	ConnectorID        string                         `json:"connectorId"`
	Status             SmokeStatus                    `json:"status"`
	CredentialMode     CredentialMode                 `json:"credentialMode"`
	Owner              string                         `json:"owner"`
	Reason             string                         `json:"reason"`
	RemainingRisk      string                         `json:"remainingRisk"`
	ValidatedAt        time.Time                      `json:"validatedAt"`
	RetentionExpiresAt time.Time                      `json:"retentionExpiresAt"`
	RedactionStatus    baseconnectors.RedactionStatus `json:"redactionStatus"`
	SafeEvidence       map[string]string              `json:"safeEvidence,omitempty"`
}

func BuildSmokeEvidence(input SmokeInput) SmokeEvidence {
	now := input.ValidatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	id := strings.TrimSpace(input.SmokeEvidenceID)
	if id == "" {
		id = "discord_smoke_" + strings.TrimSpace(input.ConnectorID)
	}
	owner := strings.TrimSpace(input.Owner)
	if owner == "" {
		owner = "operator"
	}
	evidence := SmokeEvidence{
		SmokeEvidenceID:    id,
		TenantID:           strings.TrimSpace(input.TenantID),
		ConnectorID:        strings.TrimSpace(input.ConnectorID),
		Status:             SmokeSkipped,
		CredentialMode:     CredentialModeUnavailable,
		Owner:              owner,
		Reason:             firstNonEmpty(strings.TrimSpace(input.Reason), "safe_credentials_unavailable"),
		RemainingRisk:      firstNonEmpty(strings.TrimSpace(input.RemainingRisk), "Live Discord hosted smoke was not run."),
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    baseconnectors.RedactionStatusRedacted,
		SafeEvidence:       input.SafeEvidence,
	}
	if !input.SafeCredentialsApproved {
		return evidence
	}
	evidence.CredentialMode = CredentialModeSafeLive
	if input.Passed {
		evidence.Status = SmokePassed
		evidence.Reason = "healthy"
		evidence.RemainingRisk = ""
		return evidence
	}
	evidence.Status = SmokeFailed
	evidence.Reason = firstNonEmpty(strings.TrimSpace(input.Reason), string(baseconnectors.DiagnosticUnknownConnectorFailure))
	return evidence
}
