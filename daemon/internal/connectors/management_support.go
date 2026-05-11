package connectors

import "time"

func BuildSupportEvidenceBundle(input ProjectionInput, connector Connector, principalID string, now time.Time) SupportEvidenceBundle {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	projection := BuildConnectorProjection(connector, latestDiagnostic(input.Diagnostics[connector.ConnectorID]), now)
	return SupportEvidenceBundle{
		TenantID:               input.TenantID,
		ConnectorID:            connector.ConnectorID,
		GeneratedByPrincipalID: principalID,
		GeneratedAt:            now,
		CurrentState:           projection.EnablementState,
		Redactions:             []string{"message_body", "raw_provider_payload", "credentials", "authorization_grants"},
		RetentionExpiresAt:     now.Add(90 * 24 * time.Hour),
		RedactionStatus:        RedactionStatusRedacted,
		SafeEvidence: map[string]string{
			"connectorKind": connector.Kind,
			"displayName":   connector.DisplayName,
		},
	}
}
