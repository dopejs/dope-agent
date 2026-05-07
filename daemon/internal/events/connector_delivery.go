package events

type ConnectorForegroundReplyFailedInput struct {
	TenantID             string
	ConnectorID          string
	MessageDeliveryID    string
	ReasonCode           string
	RetrySafety          string
	BackgroundDeliveryID string
	SeparationStatus     string
}

func ConnectorForegroundReplyFailed(input ConnectorForegroundReplyFailedInput) Event {
	return Event{
		TenantID: input.TenantID,
		Category: "connector",
		Name:     "connector.foreground_reply_failed",
		Scope:    Scope{ConnectorID: input.ConnectorID},
		Resource: Resource{Kind: "connector_foreground_reply", ID: input.MessageDeliveryID},
		Payload: map[string]any{
			"tenantId":             input.TenantID,
			"connectorId":          input.ConnectorID,
			"messageDeliveryId":    input.MessageDeliveryID,
			"status":               "failed",
			"reasonCode":           input.ReasonCode,
			"retrySafety":          input.RetrySafety,
			"backgroundDeliveryId": input.BackgroundDeliveryID,
			"separationStatus":     input.SeparationStatus,
			"redactionStatus":      "redacted",
		},
	}
}

type ConnectorDeliverySeparationInput struct {
	TenantID                 string
	ConnectorID              string
	BoundaryID               string
	ForegroundReplyOutcomeID string
	BackgroundDeliveryID     string
	TransportKind            string
	SeparationStatus         string
}

func ConnectorDeliverySeparationRecorded(input ConnectorDeliverySeparationInput) Event {
	return Event{
		TenantID: input.TenantID,
		Category: "connector",
		Name:     "connector.delivery_separation_recorded",
		Scope:    Scope{ConnectorID: input.ConnectorID},
		Resource: Resource{Kind: "connector_delivery_boundary", ID: input.BoundaryID},
		Payload: map[string]any{
			"tenantId":                 input.TenantID,
			"connectorId":              input.ConnectorID,
			"foregroundReplyOutcomeId": input.ForegroundReplyOutcomeID,
			"backgroundDeliveryId":     input.BackgroundDeliveryID,
			"transportKind":            input.TransportKind,
			"separationStatus":         input.SeparationStatus,
			"redactionStatus":          "redacted",
		},
	}
}
