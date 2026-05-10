package events

import "time"

type ConnectorMatrixSetupValidatedInput struct {
	TenantID            string
	ConnectorID         string
	HomeserverBindingID string
	TerminalState       string
	BotCredentialState  string
	HomeserverState     string
	RoutePolicyState    string
	DeliveryEligible    bool
	ReasonCode          string
	MatrixCondition     string
	RedactionStatus     string
	ValidatedAt         time.Time
}

type ConnectorMatrixRouteOutcomeRecordedInput struct {
	TenantID        string
	ConnectorID     string
	HomeserverID    string
	ConversationID  string
	MatrixEventID   string
	SyncBatchID     string
	TransactionID   string
	Outcome         string
	ReasonCode      string
	Surface         string
	RedactionStatus string
}

type ConnectorMatrixSmokeEvidenceRecordedInput struct {
	TenantID            string
	ConnectorID         string
	SmokeEvidenceID     string
	HomeserverBindingID string
	Status              string
	AuthorizationMode   string
	Owner               string
	Reason              string
	RedactionStatus     string
	ValidatedAt         time.Time
	RetentionExpiresAt  time.Time
}

func ConnectorMatrixSetupValidated(input ConnectorMatrixSetupValidatedInput) Event {
	return Event{
		Category: "connector",
		Name:     ConnectorEventMatrixSetupValidated,
		Scope:    Scope{ConnectorID: input.ConnectorID},
		Resource: Resource{Kind: "matrix_hosted_setup", ID: input.ConnectorID},
		Payload: map[string]any{
			"tenantId":            input.TenantID,
			"connectorId":         input.ConnectorID,
			"homeserverBindingId": input.HomeserverBindingID,
			"terminalState":       input.TerminalState,
			"botCredentialState":  input.BotCredentialState,
			"homeserverState":     input.HomeserverState,
			"routePolicyState":    input.RoutePolicyState,
			"deliveryEligible":    input.DeliveryEligible,
			"reasonCode":          input.ReasonCode,
			"matrixCondition":     input.MatrixCondition,
			"redactionStatus":     input.RedactionStatus,
			"validatedAt":         input.ValidatedAt,
		},
	}
}

func ConnectorMatrixRouteOutcomeRecorded(input ConnectorMatrixRouteOutcomeRecordedInput) Event {
	return Event{
		Category: "connector",
		Name:     ConnectorEventRouteOutcomeRecorded,
		Scope:    Scope{ConnectorID: input.ConnectorID},
		Resource: Resource{Kind: "connector_route_outcome", ID: input.MatrixEventID},
		Payload: map[string]any{
			"tenantId":        input.TenantID,
			"connectorId":     input.ConnectorID,
			"homeserverId":    input.HomeserverID,
			"conversationId":  input.ConversationID,
			"matrixEventId":   input.MatrixEventID,
			"syncBatchId":     input.SyncBatchID,
			"transactionId":   input.TransactionID,
			"outcome":         input.Outcome,
			"reasonCode":      input.ReasonCode,
			"surface":         input.Surface,
			"redactionStatus": input.RedactionStatus,
		},
	}
}

func ConnectorMatrixSmokeEvidenceRecorded(input ConnectorMatrixSmokeEvidenceRecordedInput) Event {
	return Event{
		TenantID: input.TenantID,
		Category: "connector",
		Name:     ConnectorEventMatrixSmokeEvidenceRecorded,
		Scope:    Scope{ConnectorID: input.ConnectorID},
		Resource: Resource{Kind: "matrix_smoke_evidence", ID: input.SmokeEvidenceID},
		Payload: map[string]any{
			"tenantId":            input.TenantID,
			"connectorId":         input.ConnectorID,
			"smokeEvidenceId":     input.SmokeEvidenceID,
			"homeserverBindingId": input.HomeserverBindingID,
			"status":              input.Status,
			"authorizationMode":   input.AuthorizationMode,
			"owner":               input.Owner,
			"reason":              input.Reason,
			"redactionStatus":     input.RedactionStatus,
			"validatedAt":         input.ValidatedAt,
			"retentionExpiresAt":  input.RetentionExpiresAt,
		},
	}
}
