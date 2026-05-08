package events

import "time"

type ConnectorTelegramSetupValidatedInput struct {
	TenantID        string
	ConnectorID     string
	TerminalState   string
	HostedReady     bool
	CredentialState string
	AllowmentState  string
	ReasonCode      string
	RedactionStatus string
	ValidatedAt     time.Time
}

func ConnectorTelegramSetupValidated(input ConnectorTelegramSetupValidatedInput) Event {
	return Event{
		Category: "connector",
		Name:     "connector.telegram_setup_validated",
		Scope:    Scope{ConnectorID: input.ConnectorID},
		Resource: Resource{Kind: "telegram_hosted_setup", ID: input.ConnectorID},
		Payload: map[string]any{
			"tenantId":        input.TenantID,
			"connectorId":     input.ConnectorID,
			"terminalState":   input.TerminalState,
			"hostedReady":     input.HostedReady,
			"credentialState": input.CredentialState,
			"allowmentState":  input.AllowmentState,
			"reasonCode":      input.ReasonCode,
			"redactionStatus": input.RedactionStatus,
			"validatedAt":     input.ValidatedAt,
		},
	}
}
