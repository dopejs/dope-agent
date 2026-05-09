package events

import "testing"

func TestConnectorDeliveryEventsAcceptTelegramConnectorEvidence(t *testing.T) {
	t.Parallel()

	failed := ConnectorForegroundReplyFailed(ConnectorForegroundReplyFailedInput{
		TenantID:             "ten_telegram",
		ConnectorID:          "telegram-main",
		MessageDeliveryID:    "delivery_reply_1",
		ReasonCode:           "reply_failed",
		RetrySafety:          "retryable",
		BackgroundDeliveryID: "delivery_background_1",
		SeparationStatus:     "separate_truths",
	})
	if failed.Name != "connector.foreground_reply_failed" || failed.Payload["connectorId"] != "telegram-main" {
		t.Fatalf("unexpected foreground reply failed event: %+v", failed)
	}
	if failed.Payload["redactionStatus"] != "redacted" || failed.Payload["separationStatus"] != "separate_truths" {
		t.Fatalf("unexpected foreground reply failed payload: %+v", failed.Payload)
	}

	separation := ConnectorDeliverySeparationRecorded(ConnectorDeliverySeparationInput{
		TenantID:                 "ten_telegram",
		ConnectorID:              "telegram-main",
		BoundaryID:               "boundary_1",
		ForegroundReplyOutcomeID: "foreground_reply_1",
		BackgroundDeliveryID:     "delivery_background_1",
		TransportKind:            "telegram",
		SeparationStatus:         "separate_truths",
	})
	if separation.Name != "connector.delivery_separation_recorded" || separation.Payload["transportKind"] != "telegram" {
		t.Fatalf("unexpected delivery separation event: %+v", separation)
	}
}

func TestConnectorDeliveryEventsAcceptSlackConnectorEvidence(t *testing.T) {
	t.Parallel()

	failed := ConnectorForegroundReplyFailed(ConnectorForegroundReplyFailedInput{
		TenantID:             "ten_slack",
		ConnectorID:          "slack-main",
		MessageDeliveryID:    "delivery_slack_reply_1",
		ReasonCode:           "reply_failed",
		RetrySafety:          "retryable",
		BackgroundDeliveryID: "delivery_slack_background_1",
		SeparationStatus:     "separate_truths",
	})
	if failed.Name != "connector.foreground_reply_failed" || failed.Payload["connectorId"] != "slack-main" {
		t.Fatalf("unexpected Slack foreground reply failed event: %+v", failed)
	}
	if failed.Payload["redactionStatus"] != "redacted" || failed.Payload["backgroundDeliveryId"] != "delivery_slack_background_1" {
		t.Fatalf("unexpected Slack foreground reply failed payload: %+v", failed.Payload)
	}

	separation := ConnectorDeliverySeparationRecorded(ConnectorDeliverySeparationInput{
		TenantID:                 "ten_slack",
		ConnectorID:              "slack-main",
		BoundaryID:               "boundary_slack_1",
		ForegroundReplyOutcomeID: "foreground_slack_reply_1",
		BackgroundDeliveryID:     "delivery_slack_background_1",
		TransportKind:            "slack",
		SeparationStatus:         "separate_truths",
	})
	if separation.Name != "connector.delivery_separation_recorded" || separation.Payload["transportKind"] != "slack" {
		t.Fatalf("unexpected Slack delivery separation event: %+v", separation)
	}
}
