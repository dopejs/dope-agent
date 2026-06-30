package feishulark_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterprovider"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterrpc"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/providers/feishulark"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
)

// mailMux answers the primary-mailbox probe (the Manager projects the account before each op)
// and delegates the rest to op.
func mailMux(op http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/open-apis/mail/v1/user_mailboxes/primary" {
			writeFeishu(w, map[string]any{"mailbox_address": "agent@example.com", "name": "Agent"})
			return
		}
		op(w, r)
	}
}

func wiredMailManager(t *testing.T, h http.HandlerFunc) (*mail.Manager, []integrations.Resource) {
	t.Helper()
	srv := httptest.NewServer(h)
	provider := feishulark.NewMailProvider(feishulark.NewClient(srv.URL, srv.Client()))

	adapterReader, daemonWriter := io.Pipe()
	daemonReader, adapterWriter := io.Pipe()
	go func() {
		_ = adapterprovider.Serve(adapterReader, adapterWriter, provider)
		_ = adapterWriter.Close()
	}()

	resolver := adapterrpc.ScopedResolver(func(ctx context.Context, integrationID string) (json.RawMessage, error) {
		return json.Marshal(map[string]any{"accessToken": "scoped-token"})
	})
	client := adapterrpc.NewClient(daemonWriter, daemonReader).WithCredentials(resolver)

	m := mail.NewManager("test")
	m.RegisterBackend(integrations.BackendKindAdapterRPC, mail.NewAdapterBackend(client, 2*time.Second).WithProviderKind(string(integrations.BackendKindFeishuLark)))

	t.Cleanup(func() {
		_ = daemonWriter.Close()
		_ = adapterWriter.Close()
		srv.Close()
	})
	resource := integrations.Resource{
		IntegrationID:    "int-mail-1",
		DomainKind:       "mail",
		EnvironmentScope: "test",
		ReadinessStatus:  integrations.ReadinessStatusHealthy,
		CanonicalDefault: true,
		BackendBinding:   integrations.BackendBinding{BackendKind: integrations.BackendKindAdapterRPC},
	}
	return m, []integrations.Resource{resource}
}

func mailSel() mail.Selection { return mail.Selection{IntegrationID: "int-mail-1"} }

// US1 (FR-001/FR-002, SC-001): read closure maps real provider responses onto mail resources.
func TestMailReadClosure(t *testing.T) {
	m, resources := wiredMailManager(t, mailMux(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/threads") {
			writeFeishu(w, map[string]any{"items": []map[string]any{{
				"thread_id": "th-1", "subject": "Hello", "participants": []string{"a@example.com"},
				"latest_message_time": 1741397400, "message_count": 2,
			}}})
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	account, threads, op, _, err := m.ListThreads(resources, mail.ListThreadsInput{Selection: mailSel(), Limit: 10})
	if err != nil {
		t.Fatalf("ListThreads: %v", err)
	}
	if account.MailboxAddress != "agent@example.com" {
		t.Fatalf("mailbox not projected: %+v", account)
	}
	if op.Status != mail.OperationStatusCompleted || len(threads) != 1 || threads[0].ThreadID != "th-1" {
		t.Fatalf("threads not mapped: op=%q threads=%+v", op.Status, threads)
	}
}

// US2 (FR-004/FR-005, SC-002): direct send is a distinct operation preserving message identity.
func TestMailSendClosure(t *testing.T) {
	m, resources := wiredMailManager(t, mailMux(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages/send") {
			writeFeishu(w, map[string]any{"message": map[string]any{
				"message_id": "msg-1", "thread_id": "th-9", "subject": "Hi", "from": "agent@example.com",
				"to": []string{"b@example.com"}, "sent_time": 1741397400,
			}})
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	account, msg, op, _, err := m.SendMessage(resources, mail.SendMessageInput{
		Selection: mailSel(), To: []string{"b@example.com"}, Subject: "Hi", Body: "hello",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if msg.MessageID != "msg-1" || msg.Direction != mail.DirectionOutbound {
		t.Fatalf("message not mapped: %+v", msg)
	}
	if op.OperationClass != mail.OperationClassSendMessage || op.Status != mail.OperationStatusCompleted {
		t.Fatalf("send op wrong: %+v", op)
	}
	_ = account
}

// US2 (FR-007, SC-004): an ambiguous send acknowledgement is recorded as ambiguous-commit.
func TestMailAmbiguousSend(t *testing.T) {
	m, resources := wiredMailManager(t, mailMux(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages/send") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	_, _, op, _, err := m.SendMessage(resources, mail.SendMessageInput{
		Selection: mailSel(), To: []string{"b@example.com"}, Subject: "Hi", Body: "hello",
	})
	if err == nil {
		t.Fatal("expected ambiguous send error")
	}
	if op.Status != mail.OperationStatusFailed || op.FailureClass != "ambiguous_commit" {
		t.Fatalf("op = status:%q class:%q, want failed/ambiguous_commit", op.Status, op.FailureClass)
	}
}

// US3 (FR-006, SC-003): provider auth/scope failures map to stable diagnostics reasons.
func TestMailDiagnosticsMapStableReasons(t *testing.T) {
	m, resources := wiredMailManager(t, mailMux(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/threads") {
			writeFeishuCode(w, 99991669, "scope not granted secret-detail")
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	_, _, op, _, err := m.ListThreads(resources, mail.ListThreadsInput{Selection: mailSel(), Limit: 5})
	if err == nil {
		t.Fatal("expected scope failure")
	}
	if op.DiagnosticFailure == nil || op.DiagnosticFailure.ReasonCode != integrations.ReasonScopeMissing {
		t.Fatalf("diagnostic = %+v, want scope_missing", op.DiagnosticFailure)
	}
	if strings.Contains(op.FailureReason, "secret-detail") {
		t.Fatalf("raw provider message leaked: %q", op.FailureReason)
	}
}
