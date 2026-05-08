package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/setupwizard"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestTelegramSetupWizardPersistsValidCredentialEvenWhenAllowmentMissing(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bot123:token/getMe" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"result": map[string]any{
				"id":       42,
				"username": "dope_test_bot",
			},
		})
	}))
	t.Cleanup(server.Close)

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	integration := newTelegramSetupWizardIntegration(sqliteStore, config.TelegramConnectorConfig{
		ConnectorID:   "telegram-main",
		DisplayName:   "Telegram Main",
		BotAPIBaseURL: server.URL,
	})
	session := setupwizard.SetupSession{
		SetupSessionID: "setup_telegram",
		TenantID:       "ten_telegram",
		TargetID:       setupwizard.TargetTelegramConnector,
		CreatedAt:      time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC),
	}
	input := setupwizard.SubmitSecretInput{SessionID: session.SetupSessionID, SecretRef: "TELEGRAM_BOT_TOKEN", Value: "123:token"}

	probe, err := integration.ProbeSubmittedSecret(context.Background(), session, input)
	if err != nil {
		t.Fatalf("ProbeSubmittedSecret returned error: %v", err)
	}
	if probe.State != setupwizard.StateActionRequired || probe.ReasonCode != setupwizard.ReasonTelegramAllowmentMissing {
		t.Fatalf("expected action-required allowment diagnostic, got %+v", probe)
	}
	session.State = probe.State
	session.ReasonCode = probe.ReasonCode
	if err := integration.RecordSubmittedSecretSetup(context.Background(), session, input); err != nil {
		t.Fatalf("RecordSubmittedSecretSetup returned error: %v", err)
	}
	stored, ok, err := sqliteStore.GetTelegramHostedSetup(context.Background(), "ten_telegram", "telegram-main")
	if err != nil || !ok {
		t.Fatalf("GetTelegramHostedSetup ok=%v err=%v", ok, err)
	}
	if stored.CredentialState != "valid" || stored.TerminalState != "action-required" || stored.AccountBinding == nil {
		t.Fatalf("expected valid credential with action-required allowment state, got %+v", stored)
	}
}
