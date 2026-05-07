package events

type ConnectorInboundDuplicateInput struct {
	TenantID                string
	ConnectorID             string
	ConnectorAccountID      string
	ChannelOrConversationID string
	ProviderMessageID       string
	EquivalentRuleID        string
	ExistingDeliveryID      string
}

func ConnectorInboundDuplicateDetected(input ConnectorInboundDuplicateInput) Event {
	return Event{
		TenantID: input.TenantID,
		Category: "connector",
		Name:     "connector.inbound_duplicate_detected",
		Scope:    Scope{ConnectorID: input.ConnectorID},
		Resource: Resource{Kind: "connector_message", ID: input.ExistingDeliveryID},
		Payload: map[string]any{
			"tenantId":                input.TenantID,
			"connectorId":             input.ConnectorID,
			"connectorAccountId":      input.ConnectorAccountID,
			"channelOrConversationId": input.ChannelOrConversationID,
			"providerMessageId":       input.ProviderMessageID,
			"equivalentRuleId":        input.EquivalentRuleID,
			"existingDeliveryId":      input.ExistingDeliveryID,
			"redactionStatus":         "redacted",
		},
	}
}

type ConnectorRouteOutcomeInput struct {
	TenantID                string
	ConnectorID             string
	Outcome                 string
	ReasonCode              string
	Surface                 string
	MessageDeliveryID       string
	ConnectorAccountID      string
	ChannelOrConversationID string
	ProviderMessageID       string
	EquivalentRuleID        string
}

func ConnectorRouteOutcomeRecorded(input ConnectorRouteOutcomeInput) Event {
	resourceID := input.MessageDeliveryID
	if resourceID == "" {
		resourceID = input.ConnectorID
	}
	return Event{
		TenantID: input.TenantID,
		Category: "connector",
		Name:     "connector.route_outcome_recorded",
		Scope:    Scope{ConnectorID: input.ConnectorID},
		Resource: Resource{Kind: "connector_route_outcome", ID: resourceID},
		Payload: map[string]any{
			"tenantId":                input.TenantID,
			"connectorId":             input.ConnectorID,
			"outcome":                 input.Outcome,
			"reasonCode":              input.ReasonCode,
			"surface":                 input.Surface,
			"messageDeliveryId":       input.MessageDeliveryID,
			"connectorAccountId":      input.ConnectorAccountID,
			"channelOrConversationId": input.ChannelOrConversationID,
			"providerMessageId":       input.ProviderMessageID,
			"equivalentRuleId":        input.EquivalentRuleID,
			"redactionStatus":         "redacted",
		},
	}
}
