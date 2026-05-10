package matrix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
)

func TestClientTransportSendsMatrixTextReplyWithBearerToken(t *testing.T) {
	t.Parallel()

	var sawPath string
	var sawAuthorization string
	var sawBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.EscapedPath()
		sawAuthorization = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&sawBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"event_id":"$reply1"}`))
	}))
	t.Cleanup(server.Close)

	transport, err := NewClientTransport(ClientTransportConfig{
		ConnectorID:    "matrix-main",
		HomeserverURL:  server.URL,
		BotAccessToken: "matrix-token-do-not-leak",
	})
	if err != nil {
		t.Fatalf("NewClientTransport returned error: %v", err)
	}
	sent, err := transport.SendReply(context.Background(), imtypes.OutboundReply{
		ConnectorID:              "matrix-main",
		ChannelID:                "!room:example.org",
		Content:                  "hello",
		ReplyToExternalMessageID: "$event1",
	})
	if err != nil {
		t.Fatalf("SendReply returned error: %v", err)
	}
	if sent.ExternalMessageID != "$reply1" {
		t.Fatalf("unexpected sent reply: %+v", sent)
	}
	if !strings.Contains(sawPath, "/_matrix/client/v3/rooms/%21room:example.org/send/m.room.message/dope_event1") {
		t.Fatalf("unexpected Matrix send path: %s", sawPath)
	}
	if sawAuthorization != "Bearer matrix-token-do-not-leak" {
		t.Fatalf("unexpected authorization header: %q", sawAuthorization)
	}
	if sawBody["msgtype"] != "m.text" || sawBody["body"] != "hello" {
		t.Fatalf("unexpected Matrix message body: %+v", sawBody)
	}
}

func TestClientTransportStartConsumesSyncTextEvents(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_matrix/client/v3/sync" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer matrix-token-do-not-leak" {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"next_batch":"batch_2",
			"rooms":{"join":{"!room:example.org":{"timeline":{"events":[{
				"type":"m.room.message",
				"event_id":"$event1",
				"sender":"@alice:example.org",
				"origin_server_ts":1778407200000,
				"content":{"msgtype":"m.text","body":"@bot:example.org hello"}
			}]}}}}
		}`))
	}))
	t.Cleanup(server.Close)

	transport, err := NewClientTransport(ClientTransportConfig{
		ConnectorID:    "matrix-main",
		HomeserverURL:  server.URL,
		BotAccessToken: "matrix-token-do-not-leak",
		MaxSyncCycles:  1,
	})
	if err != nil {
		t.Fatalf("NewClientTransport returned error: %v", err)
	}
	var events []InboundEvent
	if err := transport.Start(context.Background(), func(_ context.Context, event InboundEvent) {
		events = append(events, event)
	}); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one Matrix sync event, got %+v", events)
	}
	event := events[0]
	if event.ConnectorID != "matrix-main" || event.HomeserverID != "example.org" || event.ConversationID != "!room:example.org" || event.MatrixEventID != "$event1" || event.SyncBatchID != "batch_2" {
		t.Fatalf("unexpected normalized sync event: %+v", event)
	}
	if event.MessageKind != MessageUnencryptedText || event.ConversationType != ConversationRoom || event.Text != "@bot:example.org hello" {
		t.Fatalf("unexpected Matrix message classification: %+v", event)
	}
	if event.ReceivedAt != time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC) {
		t.Fatalf("unexpected received time: %s", event.ReceivedAt)
	}
}

func TestClientTransportStartClassifiesAllowedDirectSender(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_matrix/client/v3/sync" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"next_batch":"batch_direct",
			"rooms":{"join":{"!dm:example.org":{"timeline":{"events":[{
				"type":"m.room.message",
				"event_id":"$direct1",
				"sender":"@alice:example.org",
				"origin_server_ts":1778407200000,
				"content":{"msgtype":"m.text","body":"hello from dm"}
			}]}}}}
		}`))
	}))
	t.Cleanup(server.Close)

	transport, err := NewClientTransport(ClientTransportConfig{
		ConnectorID:          "matrix-main",
		HomeserverURL:        server.URL,
		BotAccessToken:       "matrix-token-do-not-leak",
		AllowedDirectUserIDs: []string{"@alice:example.org"},
		SelectedRoomIDs:      []string{"!room:example.org"},
		MaxSyncCycles:        1,
	})
	if err != nil {
		t.Fatalf("NewClientTransport returned error: %v", err)
	}
	var events []InboundEvent
	if err := transport.Start(context.Background(), func(_ context.Context, event InboundEvent) {
		events = append(events, event)
	}); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one Matrix sync event, got %+v", events)
	}
	if events[0].ConversationType != ConversationDirectMessage || events[0].ConversationID != "!dm:example.org" || events[0].SenderID != "@alice:example.org" {
		t.Fatalf("expected allowed direct sender classification, got %+v", events[0])
	}
}

func TestClientTransportValidatesBotIdentityAndRoomMembership(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.EscapedPath() {
		case "/_matrix/client/v3/account/whoami":
			_, _ = w.Write([]byte(`{"user_id":"@bot:example.org","device_id":"DEVICE1"}`))
		case "/_matrix/client/v3/rooms/%21room:example.org/state/m.room.member/@bot:example.org":
			_, _ = w.Write([]byte(`{"membership":"join"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.EscapedPath())
		}
	}))
	t.Cleanup(server.Close)

	transport, err := NewClientTransport(ClientTransportConfig{
		ConnectorID:    "matrix-main",
		HomeserverURL:  server.URL,
		BotAccessToken: "matrix-token-do-not-leak",
	})
	if err != nil {
		t.Fatalf("NewClientTransport returned error: %v", err)
	}
	binding, err := transport.ValidateHomeserverBinding(context.Background(), HomeserverBinding{
		TenantID:            "ten_matrix",
		ConnectorID:         "matrix-main",
		HomeserverBindingID: "matrix_hs_1",
		HomeserverURL:       server.URL,
		BotUserID:           "@bot:example.org",
	})
	if err != nil {
		t.Fatalf("ValidateHomeserverBinding returned error: %v", err)
	}
	if binding.AuthorizationState != AuthorizationValid || binding.CapabilityState != HomeserverCapabilityValid || binding.BotDeviceID != "DEVICE1" {
		t.Fatalf("unexpected Matrix homeserver binding: %+v", binding)
	}
	policy, err := transport.ValidateRoutePolicy(context.Background(), RoutePolicy{
		TenantID:            "ten_matrix",
		ConnectorID:         "matrix-main",
		HomeserverBindingID: "matrix_hs_1",
		SelectedRooms: []ConversationRoute{{
			ConversationID:   "!room:example.org",
			ConversationType: ConversationRoom,
		}},
		ValidationState: RoutePolicyValid,
	})
	if err != nil {
		t.Fatalf("ValidateRoutePolicy returned error: %v", err)
	}
	if !HasReadyRoutePolicy(policy) {
		t.Fatalf("expected ready Matrix route policy, got %+v", policy)
	}
}

func TestClientTransportRequiresAccessToken(t *testing.T) {
	t.Parallel()

	transport, err := NewClientTransport(ClientTransportConfig{ConnectorID: "matrix-main", HomeserverURL: "https://matrix.example.org"})
	if err != nil {
		t.Fatalf("NewClientTransport returned error: %v", err)
	}
	if _, err := transport.SendReply(context.Background(), imtypes.OutboundReply{ChannelID: "!room:example.org", Content: "hello"}); err == nil {
		t.Fatal("expected SendReply to require a Matrix bot access token")
	}
}

func TestExecuteSafeLiveSmokeValidatesCredentialRouteAndSendPath(t *testing.T) {
	t.Parallel()

	var sentBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.EscapedPath() {
		case "/_matrix/client/v3/account/whoami":
			_, _ = w.Write([]byte(`{"user_id":"@bot:example.org","device_id":"DEVICE1"}`))
		case "/_matrix/client/v3/rooms/%21room:example.org/state/m.room.member/@bot:example.org":
			_, _ = w.Write([]byte(`{"membership":"join"}`))
		default:
			if r.Method != http.MethodPut {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			}
			if err := json.NewDecoder(r.Body).Decode(&sentBody); err != nil {
				t.Fatalf("decode send body: %v", err)
			}
			_, _ = w.Write([]byte(`{"event_id":"$smoke_reply"}`))
		}
	}))
	t.Cleanup(server.Close)

	transport, err := NewClientTransport(ClientTransportConfig{
		ConnectorID:    "matrix-main",
		HomeserverURL:  server.URL,
		BotAccessToken: "matrix-token-do-not-leak",
	})
	if err != nil {
		t.Fatalf("NewClientTransport returned error: %v", err)
	}
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	evidence, err := ExecuteSafeLiveSmoke(context.Background(), SafeLiveSmokeInput{
		TenantID:    "ten_matrix",
		ConnectorID: "matrix-main",
		Owner:       "operator",
		Now:         now,
		Transport:   transport,
		Binding: HomeserverBinding{
			TenantID:            "ten_matrix",
			ConnectorID:         "matrix-main",
			HomeserverBindingID: "matrix_hs_1",
			HomeserverURL:       server.URL,
			BotUserID:           "@bot:example.org",
		},
		RoutePolicy: RoutePolicy{
			TenantID:            "ten_matrix",
			ConnectorID:         "matrix-main",
			HomeserverBindingID: "matrix_hs_1",
			SelectedRooms: []ConversationRoute{{
				ConversationID:   "!room:example.org",
				ConversationType: ConversationRoom,
			}},
			ValidationState: RoutePolicyValid,
		},
		SmokeRoomID: "!room:example.org",
	})
	if err != nil {
		t.Fatalf("ExecuteSafeLiveSmoke returned error: %v", err)
	}
	if evidence.Status != SmokePassed || evidence.AuthorizationMode != SmokeAuthorizationSafeLive || evidence.SafeEvidence["eventId"] != "$smoke_reply" {
		t.Fatalf("unexpected safe-live Matrix smoke evidence: %+v", evidence)
	}
	if sentBody["msgtype"] != "m.text" || sentBody["body"] == "" {
		t.Fatalf("expected Matrix smoke to send an m.text message, got %+v", sentBody)
	}
}
