package slack

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestWebAPITransportSendsThreadedReply(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat.postMessage" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer xoxb-redacted" {
			t.Fatalf("unexpected authorization header %q", got)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["channel"] != "C123" || body["text"] != "hello" || body["thread_ts"] != "171.0001" {
			t.Fatalf("unexpected Slack post body: %+v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "ts": "171.0002"})
	}))
	t.Cleanup(server.Close)

	transport := NewWebAPITransport(WebAPITransportConfig{
		ConnectorID: "slack-main",
		BaseURL:     server.URL,
		BotToken:    "xoxb-redacted",
	})
	sent, err := transport.SendReply(context.Background(), imtypes.OutboundReply{
		ConnectorID:              "slack-main",
		ChannelID:                "C123",
		Content:                  "hello",
		ReplyToExternalMessageID: "171.0001",
	})
	if err != nil {
		t.Fatalf("SendReply returned error: %v", err)
	}
	if sent.ExternalMessageID != "171.0002" {
		t.Fatalf("expected Slack ts as external message id, got %+v", sent)
	}
}

func TestWebAPITransportClassifiesProviderErrors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "missing_scope"})
	}))
	t.Cleanup(server.Close)

	transport := NewWebAPITransport(WebAPITransportConfig{
		ConnectorID: "slack-main",
		BaseURL:     server.URL,
		BotToken:    "xoxb-redacted",
	})
	_, err := transport.SendReply(context.Background(), imtypes.OutboundReply{ConnectorID: "slack-main", ChannelID: "C123", Content: "hello"})
	if err == nil {
		t.Fatal("expected Slack API error")
	}
	if reason := DiagnosticReasonForError(err); reason != "permission_missing" {
		t.Fatalf("expected permission_missing diagnostic, got %s from %v", reason, err)
	}
}

func TestWebAPITransportClassifiesNetworkFailure(t *testing.T) {
	t.Parallel()

	transport := NewWebAPITransport(WebAPITransportConfig{
		ConnectorID: "slack-main",
		BaseURL:     "https://slack.invalid",
		BotToken:    "xoxb-redacted",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("connection reset by peer")
		})},
	})
	_, err := transport.SendReply(context.Background(), imtypes.OutboundReply{ConnectorID: "slack-main", ChannelID: "C123", Content: "hello"})
	if err == nil {
		t.Fatal("expected Slack network error")
	}
	if reason := DiagnosticReasonForError(err); reason != "network_failed" {
		t.Fatalf("expected network_failed diagnostic, got %s from %v", reason, err)
	}
}

func TestWebAPITransportValidatesInstallation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth.test" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":      true,
			"team_id": "T123",
			"team":    "Test Workspace",
			"bot_id":  "B123",
			"user_id": "Ubot",
		})
	}))
	t.Cleanup(server.Close)

	transport := NewWebAPITransport(WebAPITransportConfig{ConnectorID: "slack-main", BaseURL: server.URL, BotToken: "xoxb-redacted"})
	binding, err := transport.ValidateInstallation(context.Background(), WorkspaceBinding{TenantID: "ten_slack", ConnectorID: "slack-main"})
	if err != nil {
		t.Fatalf("ValidateInstallation returned error: %v", err)
	}
	if binding.WorkspaceID != "T123" || binding.WorkspaceLabel != "Test Workspace" || binding.InstallationID != "B123" {
		t.Fatalf("unexpected binding: %+v", binding)
	}
	if binding.OAuthGrantState != "valid" || binding.RequiredScopeState != "valid" {
		t.Fatalf("expected valid grant/scope states, got %+v", binding)
	}
}
