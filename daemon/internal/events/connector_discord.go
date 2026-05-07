package events

import "time"

type ConnectorDiscordSetupValidatedInput struct {
	TenantID        string
	ConnectorID     string
	ReadinessState  string
	HostedReady     bool
	CredentialState string
	ReasonCode      string
	RedactionStatus string
	ValidatedAt     time.Time
}

func ConnectorDiscordSetupValidated(input ConnectorDiscordSetupValidatedInput) Event {
	return Event{
		Category: "connector",
		Name:     "connector.discord_setup_validated",
		Scope:    Scope{ConnectorID: input.ConnectorID},
		Resource: Resource{Kind: "discord_hosted_setup", ID: input.ConnectorID},
		Payload: map[string]any{
			"tenantId":        input.TenantID,
			"connectorId":     input.ConnectorID,
			"readinessState":  input.ReadinessState,
			"hostedReady":     input.HostedReady,
			"credentialState": input.CredentialState,
			"reasonCode":      input.ReasonCode,
			"redactionStatus": input.RedactionStatus,
			"validatedAt":     input.ValidatedAt,
		},
	}
}
