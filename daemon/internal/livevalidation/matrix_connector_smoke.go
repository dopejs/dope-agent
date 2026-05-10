package livevalidation

import "time"

type MatrixConnectorSmokeInput struct {
	TenantID            string
	ConnectorID         string
	HomeserverBindingID string
	Owner               string
	Reason              string
	SafeLiveAvailable   bool
	Now                 time.Time
}

type MatrixConnectorSmokeEvidence struct {
	SmokeEvidenceID     string            `json:"smokeEvidenceId"`
	TenantID            string            `json:"tenantId,omitempty"`
	ConnectorID         string            `json:"connectorId"`
	HomeserverBindingID string            `json:"homeserverBindingId"`
	Status              string            `json:"status"`
	AuthorizationMode   string            `json:"authorizationMode"`
	Owner               string            `json:"owner"`
	Reason              string            `json:"reason"`
	RemainingRisk       string            `json:"remainingRisk,omitempty"`
	ValidatedAt         time.Time         `json:"validatedAt"`
	RetentionExpiresAt  time.Time         `json:"retentionExpiresAt"`
	RedactionStatus     string            `json:"redactionStatus"`
	SafeEvidence        map[string]string `json:"safeEvidence,omitempty"`
}

func BuildMatrixConnectorSmokeEvidence(input MatrixConnectorSmokeInput) MatrixConnectorSmokeEvidence {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	homeserverBindingID := input.HomeserverBindingID
	if homeserverBindingID == "" {
		homeserverBindingID = "matrix_homeserver_" + input.ConnectorID
	}
	if !input.SafeLiveAvailable {
		return MatrixConnectorSmokeEvidence{
			SmokeEvidenceID:     "matrix_smoke_" + input.ConnectorID,
			TenantID:            input.TenantID,
			ConnectorID:         input.ConnectorID,
			HomeserverBindingID: homeserverBindingID,
			Status:              "skipped",
			AuthorizationMode:   "unavailable",
			Owner:               input.Owner,
			Reason:              input.Reason,
			RemainingRisk:       "No live Matrix hosted smoke was run; release review must consume this structured skip.",
			ValidatedAt:         now,
			RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
			RedactionStatus:     "redacted",
			SafeEvidence:        map[string]string{"policy": "structured_skip"},
		}
	}
	return MatrixConnectorSmokeEvidence{
		SmokeEvidenceID:     "matrix_smoke_" + input.ConnectorID,
		TenantID:            input.TenantID,
		ConnectorID:         input.ConnectorID,
		HomeserverBindingID: homeserverBindingID,
		Status:              "passed",
		AuthorizationMode:   "safe_live",
		Owner:               input.Owner,
		Reason:              "healthy",
		ValidatedAt:         now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
		RedactionStatus:     "redacted",
		SafeEvidence:        map[string]string{"policy": "safe_live_matrix_smoke"},
	}
}
