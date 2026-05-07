package events

type ConnectorDiagnosticStateChangedInput struct {
	TenantID          string
	ConnectorID       string
	DiagnosticStateID string
	PreviousStatus    string
	Status            string
	ReasonCode        string
	RemediationOwner  string
	RetrySafety       string
	FreshnessState    string
	RedactionStatus   string
}

func ConnectorDiagnosticStateChanged(input ConnectorDiagnosticStateChangedInput) Event {
	return Event{
		TenantID: input.TenantID,
		Category: "connector",
		Name:     "connector.diagnostic_state_changed",
		Scope:    Scope{ConnectorID: input.ConnectorID},
		Resource: Resource{Kind: "connector_diagnostic_state", ID: input.DiagnosticStateID},
		Payload: map[string]any{
			"tenantId":          input.TenantID,
			"diagnosticStateId": input.DiagnosticStateID,
			"connectorId":       input.ConnectorID,
			"previousStatus":    input.PreviousStatus,
			"status":            input.Status,
			"reasonCode":        input.ReasonCode,
			"remediationOwner":  input.RemediationOwner,
			"retrySafety":       input.RetrySafety,
			"freshnessState":    input.FreshnessState,
			"redactionStatus":   input.RedactionStatus,
		},
	}
}

type ConnectorDiagnosticRedactionFailedInput struct {
	TenantID           string
	ConnectorID        string
	RedactionFailureID string
	TargetKind         string
	TargetID           string
	ReasonCode         string
	RetentionExpiresAt string
}

func ConnectorDiagnosticRedactionFailed(input ConnectorDiagnosticRedactionFailedInput) Event {
	return Event{
		TenantID: input.TenantID,
		Category: "connector",
		Name:     "connector.diagnostic_redaction_failed",
		Scope:    Scope{ConnectorID: input.ConnectorID},
		Resource: Resource{Kind: "connector_diagnostic_redaction_failure", ID: input.RedactionFailureID},
		Payload: map[string]any{
			"tenantId":           input.TenantID,
			"connectorId":        input.ConnectorID,
			"targetKind":         input.TargetKind,
			"targetId":           input.TargetID,
			"reasonCode":         input.ReasonCode,
			"redactionStatus":    "suppressed",
			"retentionExpiresAt": input.RetentionExpiresAt,
		},
	}
}
