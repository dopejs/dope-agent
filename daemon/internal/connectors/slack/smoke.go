package slack

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

type AuthorizationMode string

const (
	AuthorizationModeSafeLive    AuthorizationMode = "safe_live"
	AuthorizationModeFakeOAuth   AuthorizationMode = "fake_oauth"
	AuthorizationModeUnavailable AuthorizationMode = "unavailable"
)

type SmokeInput struct {
	SmokeEvidenceID    string
	TenantID           string
	ConnectorID        string
	WorkspaceBindingID string
	SafeLiveApproved   bool
	FakeOAuth          bool
	Passed             bool
	Owner              string
	Reason             string
	RemainingRisk      string
	ValidatedAt        time.Time
	SafeEvidence       map[string]string
}

type SmokeEvidence struct {
	SmokeEvidenceID    string                         `json:"smokeEvidenceId"`
	TenantID           string                         `json:"tenantId"`
	ConnectorID        string                         `json:"connectorId"`
	WorkspaceBindingID string                         `json:"workspaceBindingId"`
	Status             SmokeStatus                    `json:"status"`
	AuthorizationMode  AuthorizationMode              `json:"authorizationMode"`
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
		id = "slack_smoke_" + strings.TrimSpace(input.ConnectorID)
	}
	evidence := SmokeEvidence{
		SmokeEvidenceID:    id,
		TenantID:           strings.TrimSpace(input.TenantID),
		ConnectorID:        strings.TrimSpace(input.ConnectorID),
		WorkspaceBindingID: strings.TrimSpace(input.WorkspaceBindingID),
		Status:             SmokeSkipped,
		AuthorizationMode:  AuthorizationModeUnavailable,
		Owner:              firstNonEmpty(input.Owner, "operator"),
		Reason:             firstNonEmpty(input.Reason, "safe_slack_authorization_unavailable"),
		RemainingRisk:      firstNonEmpty(input.RemainingRisk, "Live Slack hosted smoke was not run."),
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    baseconnectors.RedactionStatusRedacted,
		SafeEvidence:       safeEvidence(input.SafeEvidence),
	}
	if containsUnsafeEvidence(input.SafeEvidence) {
		evidence.RedactionStatus = baseconnectors.RedactionStatusSuppressed
	}
	if input.FakeOAuth {
		evidence.AuthorizationMode = AuthorizationModeFakeOAuth
	}
	if !input.SafeLiveApproved && !input.FakeOAuth {
		return evidence
	}
	if input.SafeLiveApproved {
		evidence.AuthorizationMode = AuthorizationModeSafeLive
	}
	if input.Passed {
		evidence.Status = SmokePassed
		evidence.Reason = "healthy"
		evidence.RemainingRisk = ""
		return evidence
	}
	evidence.Status = SmokeFailed
	evidence.Reason = firstNonEmpty(input.Reason, string(baseconnectors.DiagnosticUnknownConnectorFailure))
	return evidence
}
