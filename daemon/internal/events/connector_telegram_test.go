package events

import (
	"testing"
	"time"
)

func TestConnectorTelegramSetupValidatedBuildsRedactedEvent(t *testing.T) {
	t.Parallel()

	validatedAt := time.Date(2026, 5, 8, 10, 1, 0, 0, time.UTC)
	event := ConnectorTelegramSetupValidated(ConnectorTelegramSetupValidatedInput{
		TenantID:        "ten_telegram",
		ConnectorID:     "telegram-main",
		TerminalState:   "ready",
		HostedReady:     true,
		CredentialState: "valid",
		AllowmentState:  "valid",
		ReasonCode:      "healthy",
		RedactionStatus: "redacted",
		ValidatedAt:     validatedAt,
	})

	if event.Category != "connector" || event.Name != "connector.telegram_setup_validated" {
		t.Fatalf("unexpected Telegram setup event name: %+v", event)
	}
	if event.Scope.ConnectorID != "telegram-main" || event.Resource.Kind != "telegram_hosted_setup" {
		t.Fatalf("unexpected Telegram setup event resource: %+v", event)
	}
	if event.Payload["tenantId"] != "ten_telegram" || event.Payload["terminalState"] != "ready" || event.Payload["redactionStatus"] != "redacted" {
		t.Fatalf("unexpected Telegram setup event payload: %+v", event.Payload)
	}
}
