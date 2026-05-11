package events

import "time"

type ConnectorManagementEventInput struct {
	TenantID        string
	ConnectorID     string
	EvidenceID      string
	Action          string
	Outcome         string
	ReasonCode      string
	RedactionStatus string
	OccurredAt      time.Time
}

func ConnectorManagementSupportEvidenceGenerated(input ConnectorManagementEventInput) Event {
	return connectorManagementEvent(ConnectorEventSupportEvidenceGenerated, "channel_support_evidence", input)
}

func ConnectorManagementRedactionFailed(input ConnectorManagementEventInput) Event {
	return connectorManagementEvent(ConnectorEventManagementRedactionFailed, "channel_management_redaction_failure", input)
}

func ConnectorManagementRetentionApplied(input ConnectorManagementEventInput) Event {
	return connectorManagementEvent(ConnectorEventManagementRetentionApplied, "channel_management_retention", input)
}

func connectorManagementEvent(name, resourceKind string, input ConnectorManagementEventInput) Event {
	occurredAt := input.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		TenantID:   input.TenantID,
		Category:   "connector",
		Name:       name,
		OccurredAt: occurredAt,
		Scope:      Scope{ConnectorID: input.ConnectorID},
		Resource:   Resource{Kind: resourceKind, ID: input.EvidenceID},
		Payload: map[string]any{
			"tenantId":        input.TenantID,
			"connectorId":     input.ConnectorID,
			"evidenceId":      input.EvidenceID,
			"action":          firstNonEmptyConnectorManagement(input.Action, name),
			"outcome":         firstNonEmptyConnectorManagement(input.Outcome, "succeeded"),
			"reasonCode":      input.ReasonCode,
			"redactionStatus": input.RedactionStatus,
		},
	}
}

func firstNonEmptyConnectorManagement(items ...string) string {
	for _, item := range items {
		if item != "" {
			return item
		}
	}
	return ""
}
