package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
)

func TestBotAPITransportValidatesCredentialAndSendsReply(t *testing.T) {
	t.Parallel()

	var sendPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bot123:token/getMe":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"result": map[string]any{
					"id":         42,
					"username":   "dope_test_bot",
					"first_name": "Dope Test",
				},
			})
		case "/bot123:token/sendMessage":
			if err := json.NewDecoder(r.Body).Decode(&sendPayload); err != nil {
				t.Fatalf("decode sendMessage payload: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"result": map[string]any{
					"message_id": 77,
				},
			})
		default:
			t.Fatalf("unexpected Telegram Bot API path %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	transport, err := NewBotAPITransport(BotAPITransportConfig{
		ConnectorID: "telegram-main",
		BotToken:    "123:token",
		BaseURL:     server.URL,
		HTTPClient:  server.Client(),
	})
	if err != nil {
		t.Fatalf("NewBotAPITransport returned error: %v", err)
	}
	binding, err := transport.ValidateCredential(context.Background())
	if err != nil {
		t.Fatalf("ValidateCredential returned error: %v", err)
	}
	if binding.ConnectorAccountID != "telegram_bot_42" || binding.ProviderAccountLabel != "dope_test_bot" || binding.PermissionState != PermissionValid {
		t.Fatalf("unexpected binding: %+v", binding)
	}

	sent, err := transport.SendReply(context.Background(), imtypes.OutboundReply{
		ConnectorID:              "telegram-main",
		ChannelID:                "chat_1",
		Content:                  "hello",
		ReplyToExternalMessageID: "message_1",
	})
	if err != nil {
		t.Fatalf("SendReply returned error: %v", err)
	}
	if sent.ExternalMessageID != "77" {
		t.Fatalf("expected Telegram message id 77, got %+v", sent)
	}
	if sendPayload["chat_id"] != "chat_1" || sendPayload["text"] != "hello" || sendPayload["reply_to_message_id"] != "message_1" {
		t.Fatalf("unexpected sendMessage payload: %+v", sendPayload)
	}
}

func TestBotAPITransportClassifiesProviderErrors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "Unauthorized"})
	}))
	t.Cleanup(server.Close)

	transport, err := NewBotAPITransport(BotAPITransportConfig{
		ConnectorID: "telegram-main",
		BotToken:    "bad:token",
		BaseURL:     server.URL,
		HTTPClient:  server.Client(),
	})
	if err != nil {
		t.Fatalf("NewBotAPITransport returned error: %v", err)
	}
	_, err = transport.ValidateCredential(context.Background())
	if err == nil {
		t.Fatal("expected validation error")
	}
	if got := DiagnosticReasonForError(err); got != "auth_missing" {
		t.Fatalf("expected auth_missing classification, got %s", got)
	}
}
