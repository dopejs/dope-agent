package events

import "time"

type ConnectorSlackSetupValidatedInput struct {
	TenantID           string
	ConnectorID        string
	WorkspaceBindingID string
	TerminalState      string
	OAuthState         string
	RoutePolicyState   string
	DeliveryEligible   bool
	ReasonCode         string
	SlackCondition     string
	RedactionStatus    string
	ValidatedAt        time.Time
}

type ConnectorSlackRouteOutcomeRecordedInput struct {
	TenantID        string
	ConnectorID     string
	WorkspaceID     string
	ConversationID  string
	MessageID       string
	EventID         string
	Outcome         string
	ReasonCode      string
	Surface         string
	RedactionStatus string
}

func ConnectorSlackSetupValidated(input ConnectorSlackSetupValidatedInput) Event {
	return Event{
		Category: "connector",
		Name:     "connector.slack_setup_validated",
		Scope:    Scope{ConnectorID: input.ConnectorID},
		Resource: Resource{Kind: "slack_hosted_setup", ID: input.ConnectorID},
		Payload: map[string]any{
			"tenantId":           input.TenantID,
			"connectorId":        input.ConnectorID,
			"workspaceBindingId": input.WorkspaceBindingID,
			"terminalState":      input.TerminalState,
			"oauthState":         input.OAuthState,
			"routePolicyState":   input.RoutePolicyState,
			"deliveryEligible":   input.DeliveryEligible,
			"reasonCode":         input.ReasonCode,
			"slackCondition":     input.SlackCondition,
			"redactionStatus":    input.RedactionStatus,
			"validatedAt":        input.ValidatedAt,
		},
	}
}

func ConnectorSlackRouteOutcomeRecorded(input ConnectorSlackRouteOutcomeRecordedInput) Event {
	return Event{
		Category: "connector",
		Name:     "connector.route_outcome_recorded",
		Scope:    Scope{ConnectorID: input.ConnectorID},
		Resource: Resource{Kind: "connector_route_outcome", ID: input.MessageID},
		Payload: map[string]any{
			"tenantId":        input.TenantID,
			"connectorId":     input.ConnectorID,
			"workspaceId":     input.WorkspaceID,
			"conversationId":  input.ConversationID,
			"messageId":       input.MessageID,
			"eventId":         input.EventID,
			"outcome":         input.Outcome,
			"reasonCode":      input.ReasonCode,
			"surface":         input.Surface,
			"redactionStatus": input.RedactionStatus,
		},
	}
}
