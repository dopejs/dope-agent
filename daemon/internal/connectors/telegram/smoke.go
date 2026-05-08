package telegram

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
	SmokeEvidenceID string
	TenantID        string
	ConnectorID     string
	SafeCredential  bool
	FakeSafePass    bool
	Passed          bool
	Owner           string
	Reason          string
	RemainingRisk   string
	ValidatedAt     time.Time
	SafeEvidence    map[string]string
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
		id = "telegram_smoke_" + strings.TrimSpace(input.ConnectorID)
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
		RemainingRisk:      firstNonEmpty(strings.TrimSpace(input.RemainingRisk), "Live Telegram hosted smoke was not run."),
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    baseconnectors.RedactionStatusRedacted,
		SafeEvidence:       safeEvidence(input.SafeEvidence),
	}
	if containsUnsafeEvidence(input.SafeEvidence) {
		evidence.RedactionStatus = baseconnectors.RedactionStatusSuppressed
	}
	if input.FakeSafePass {
		evidence.CredentialMode = CredentialModeFake
	} else if input.SafeCredential {
		evidence.CredentialMode = CredentialModeSafeLive
	} else {
		return evidence
	}
	if input.Passed {
		evidence.Status = SmokePassed
		evidence.Reason = "healthy"
		return evidence
	}
	evidence.Status = SmokeFailed
	evidence.Reason = firstNonEmpty(strings.TrimSpace(input.Reason), string(baseconnectors.DiagnosticUnknownConnectorFailure))
	return evidence
}
