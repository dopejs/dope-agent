package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

func seedHealthyMailIntegration(t *testing.T, manager *integrations.Manager, sqliteStore *store.SQLiteStore, integrationID string, canonicalDefault bool) integrations.Resource {
	t.Helper()

	resource, err := manager.Create(integrations.CreateInput{
		IntegrationID:    integrationID,
		DomainKind:       "mail",
		DisplayName:      integrationID,
		EnvironmentScope: "test",
		CanonicalDefault: canonicalDefault,
		AccountBinding: integrations.AccountBinding{
			AccountKey:   "alice@example.com",
			AccountLabel: "Alice Mailbox",
		},
		BackendBinding: integrations.BackendBinding{
			BackendKind:           integrations.BackendKindFakeLocal,
			SupportsProbeRead:     true,
			SupportsProbeMutation: true,
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	resource, err = manager.UpdateReadiness(resource.IntegrationID, integrations.UpdateReadinessInput{
		ReadinessStatus:  integrations.ReadinessStatusHealthy,
		AuthState:        integrations.AuthStateAuthorized,
		HealthState:      integrations.HealthStateHealthy,
		SecretResolution: "resolved",
	})
	if err != nil {
		t.Fatalf("UpdateReadiness returned error: %v", err)
	}
	if sqliteStore != nil {
		if err := sqliteStore.UpsertIntegration(context.Background(), resource); err != nil {
			t.Fatalf("UpsertIntegration returned error: %v", err)
		}
	}
	return resource
}

func newMailServerForTest(t *testing.T) (*store.SQLiteStore, *events.Bus, *integrations.Manager, *mail.Manager, *Server) {
	t.Helper()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	eventBus := events.NewBus()
	integrationManager := integrations.NewManager("test")
	mailManager := mail.NewManager("test")
	server := NewServer(Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			BindAddr:    "127.0.0.1:19191",
			DataDir:     t.TempDir(),
			LogLevel:    "info",
			Version:     "test",
		},
		Logger:       telemetry.New("error").Slog(),
		EventBus:     eventBus,
		Integrations: integrationManager,
		Mail:         mailManager,
		Store:        sqliteStore,
	})
	t.Cleanup(func() {
		eventBus.Close()
		_ = sqliteStore.Close()
	})
	return sqliteStore, eventBus, integrationManager, mailManager, server
}

func TestMailRoutesSupportSelectionFallbackAndInspection(t *testing.T) {
	t.Parallel()

	sqliteStore, _, integrationManager, _, server := newMailServerForTest(t)
	seedHealthyMailIntegration(t, integrationManager, sqliteStore, "mail-a", true)
	seedHealthyMailIntegration(t, integrationManager, sqliteStore, "mail-b", false)

	accountRec := httptest.NewRecorder()
	accountReq := httptest.NewRequest(http.MethodGet, "/v1/mail/accounts", nil)
	server.Handler().ServeHTTP(accountRec, accountReq)
	if accountRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for accounts, got %d body=%s", accountRec.Code, accountRec.Body.String())
	}
	accounts := decodeStrictResponse[MailAccountListResponse](t, accountRec.Body.Bytes())
	if len(accounts.Items) != 2 {
		t.Fatalf("expected both projected mail accounts, got %+v", accounts.Items)
	}

	explicitRec := httptest.NewRecorder()
	explicitReq := httptest.NewRequest(http.MethodGet, "/v1/mail/threads?integrationId=mail-b", nil)
	server.Handler().ServeHTTP(explicitRec, explicitReq)
	if explicitRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for explicit thread list, got %d body=%s", explicitRec.Code, explicitRec.Body.String())
	}
	explicit := decodeStrictResponse[MailThreadListResponse](t, explicitRec.Body.Bytes())
	if explicit.Account.IntegrationID != "mail-b" || explicit.Operation.SelectionMode != "explicit" {
		t.Fatalf("expected explicit mail selection, got account=%+v operation=%+v", explicit.Account, explicit.Operation)
	}
	if len(explicit.Items) == 0 || explicit.Items[0].ThreadID != "thread_seed" {
		t.Fatalf("expected seeded thread snapshot, got %+v", explicit.Items)
	}

	defaultRec := httptest.NewRecorder()
	defaultReq := httptest.NewRequest(http.MethodGet, "/v1/mail/threads", nil)
	server.Handler().ServeHTTP(defaultRec, defaultReq)
	if defaultRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for default thread list, got %d body=%s", defaultRec.Code, defaultRec.Body.String())
	}
	defaults := decodeStrictResponse[MailThreadListResponse](t, defaultRec.Body.Bytes())
	if defaults.Account.IntegrationID != "mail-a" || defaults.Operation.SelectionMode != "canonical_default" {
		t.Fatalf("expected canonical default fallback, got account=%+v operation=%+v", defaults.Account, defaults.Operation)
	}

	messageRec := httptest.NewRecorder()
	messageReq := httptest.NewRequest(http.MethodGet, "/v1/mail/messages/msg_seed?integrationId=mail-a", nil)
	server.Handler().ServeHTTP(messageRec, messageReq)
	if messageRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for message detail, got %d body=%s", messageRec.Code, messageRec.Body.String())
	}
	message := decodeStrictResponse[MailMessageResponse](t, messageRec.Body.Bytes())
	if message.Operation.OperationClass != mail.OperationClassGetMessage || message.Message.MessageID != "msg_seed" {
		t.Fatalf("expected seeded message detail, got %+v", message)
	}

	draftRec := httptest.NewRecorder()
	draftReq := httptest.NewRequest(http.MethodGet, "/v1/mail/drafts?integrationId=mail-a", nil)
	server.Handler().ServeHTTP(draftRec, draftReq)
	if draftRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for draft list, got %d body=%s", draftRec.Code, draftRec.Body.String())
	}
	drafts := decodeStrictResponse[MailDraftListResponse](t, draftRec.Body.Bytes())
	if len(drafts.Items) == 0 || drafts.Items[0].DraftID != "draft_seed" {
		t.Fatalf("expected seeded draft snapshot, got %+v", drafts.Items)
	}

	persistedOps, err := sqliteStore.ListMailOperations(context.Background(), "test", store.MailOperationFilter{IntegrationID: "mail-b"})
	if err != nil {
		t.Fatalf("ListMailOperations returned error: %v", err)
	}
	if len(persistedOps) == 0 {
		t.Fatal("expected persisted explicit mail read operations")
	}
}

func TestMailMutationRoutesPreserveSendTruthAndBlockUnsafeSend(t *testing.T) {
	t.Parallel()

	sqliteStore, _, integrationManager, _, server := newMailServerForTest(t)
	seedHealthyMailIntegration(t, integrationManager, sqliteStore, "mail-a", true)

	createDraftRec := httptest.NewRecorder()
	createDraftReq := httptest.NewRequest(http.MethodPost, "/v1/mail/drafts", strings.NewReader(`{
		"integrationId":"mail-a",
		"composeMode":"new_message",
		"to":["carol@example.com"],
		"subject":"Phase 30 draft",
		"body":"Draft body"
	}`))
	createDraftReq.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(createDraftRec, createDraftReq)
	if createDraftRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for draft create, got %d body=%s", createDraftRec.Code, createDraftRec.Body.String())
	}
	createdDraft := decodeStrictResponse[MailDraftResponse](t, createDraftRec.Body.Bytes())
	if createdDraft.Operation.ResultMode != mail.ResultModeDraftOnly || createdDraft.Draft.ComposeMode != mail.ComposeModeNewMessage {
		t.Fatalf("expected draft-only create result, got %+v", createdDraft)
	}

	updateDraftRec := httptest.NewRecorder()
	updateDraftReq := httptest.NewRequest(http.MethodPost, "/v1/mail/drafts/"+createdDraft.Draft.DraftID+"/update", strings.NewReader(`{
		"subject":"Phase 30 updated",
		"body":"Updated body"
	}`))
	updateDraftReq.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(updateDraftRec, updateDraftReq)
	if updateDraftRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for draft update, got %d body=%s", updateDraftRec.Code, updateDraftRec.Body.String())
	}
	updatedDraft := decodeStrictResponse[MailDraftResponse](t, updateDraftRec.Body.Bytes())
	if updatedDraft.Draft.DraftID != createdDraft.Draft.DraftID || updatedDraft.Draft.DraftStatus != mail.DraftStatusUpdated {
		t.Fatalf("expected stable updated draft identity, got %+v", updatedDraft.Draft)
	}

	sendDirectRec := httptest.NewRecorder()
	sendDirectReq := httptest.NewRequest(http.MethodPost, "/v1/mail/messages/send", strings.NewReader(`{
		"integrationId":"mail-a",
		"to":["dave@example.com"],
		"subject":"Phase 30 direct send",
		"body":"Sent body"
	}`))
	sendDirectReq.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(sendDirectRec, sendDirectReq)
	if sendDirectRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for direct send, got %d body=%s", sendDirectRec.Code, sendDirectRec.Body.String())
	}
	direct := decodeStrictResponse[MailMessageResponse](t, sendDirectRec.Body.Bytes())
	if direct.Operation.ResultMode != mail.ResultModeSent || direct.Operation.SendPath != mail.SendPathDirect {
		t.Fatalf("expected sent direct path truth, got %+v", direct.Operation)
	}

	sendDraftRec := httptest.NewRecorder()
	sendDraftReq := httptest.NewRequest(http.MethodPost, "/v1/mail/drafts/"+createdDraft.Draft.DraftID+"/send", strings.NewReader(`{"integrationId":"mail-a"}`))
	sendDraftReq.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(sendDraftRec, sendDraftReq)
	if sendDraftRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for send draft, got %d body=%s", sendDraftRec.Code, sendDraftRec.Body.String())
	}
	sentDraft := decodeStrictResponse[MailMessageResponse](t, sendDraftRec.Body.Bytes())
	if sentDraft.Operation.ResultMode != mail.ResultModeSent || sentDraft.Operation.SendPath != mail.SendPathDraft {
		t.Fatalf("expected send-draft truth, got %+v", sentDraft.Operation)
	}

	replyRec := httptest.NewRecorder()
	replyReq := httptest.NewRequest(http.MethodPost, "/v1/mail/messages/msg_seed/reply", strings.NewReader(`{
		"integrationId":"mail-a",
		"resultMode":"draft",
		"body":"Reply later"
	}`))
	replyReq.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(replyRec, replyReq)
	if replyRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for reply draft, got %d body=%s", replyRec.Code, replyRec.Body.String())
	}
	replyDraft := decodeStrictResponse[MailDraftResponse](t, replyRec.Body.Bytes())
	if replyDraft.Operation.ResultMode != mail.ResultModeDraftOnly || replyDraft.Draft.SourceMessageID != "msg_seed" {
		t.Fatalf("expected reply draft linkage, got %+v", replyDraft)
	}

	forwardRec := httptest.NewRecorder()
	forwardReq := httptest.NewRequest(http.MethodPost, "/v1/mail/messages/msg_seed/forward", strings.NewReader(`{
		"integrationId":"mail-a",
		"resultMode":"send",
		"to":["erin@example.com"],
		"body":"FYI"
	}`))
	forwardReq.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(forwardRec, forwardReq)
	if forwardRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for forward send, got %d body=%s", forwardRec.Code, forwardRec.Body.String())
	}
	forwarded := decodeStrictResponse[MailMessageResponse](t, forwardRec.Body.Bytes())
	if forwarded.Operation.ResultMode != mail.ResultModeSent || forwarded.Message.ForwardedFromMessageID != "msg_seed" {
		t.Fatalf("expected forward linkage, got %+v", forwarded)
	}

	blockedAttachmentRec := httptest.NewRecorder()
	blockedAttachmentReq := httptest.NewRequest(http.MethodPost, "/v1/mail/messages/send", strings.NewReader(`{
		"integrationId":"mail-a",
		"to":["frank@example.com"],
		"subject":"Blocked attachment",
		"attachmentRefs":[{"attachmentRefId":"missing_contract_attachment"}]
	}`))
	blockedAttachmentReq.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(blockedAttachmentRec, blockedAttachmentReq)
	if blockedAttachmentRec.Code != http.StatusBadRequest || !strings.Contains(blockedAttachmentRec.Body.String(), mail.ErrMailAttachmentUnresolved.Error()) {
		t.Fatalf("expected attachment-blocked send, got %d body=%s", blockedAttachmentRec.Code, blockedAttachmentRec.Body.String())
	}

	backgroundBlockedRec := httptest.NewRecorder()
	backgroundBlockedReq := httptest.NewRequest(http.MethodPost, "/v1/mail/messages/send", strings.NewReader(`{
		"integrationId":"mail-a",
		"to":["gina@example.com"],
		"subject":"Workflow blocked",
		"source":{"workflowId":"wf_1"}
	}`))
	backgroundBlockedReq.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(backgroundBlockedRec, backgroundBlockedReq)
	if backgroundBlockedRec.Code != http.StatusBadRequest || !strings.Contains(backgroundBlockedRec.Body.String(), mail.ErrMailBackgroundSendBlocked.Error()) {
		t.Fatalf("expected workflow send gating, got %d body=%s", backgroundBlockedRec.Code, backgroundBlockedRec.Body.String())
	}

	blockedOps, err := sqliteStore.ListMailOperations(context.Background(), "test", store.MailOperationFilter{Status: string(mail.OperationStatusBlocked)})
	if err != nil {
		t.Fatalf("ListMailOperations(blocked) returned error: %v", err)
	}
	if len(blockedOps) < 2 {
		t.Fatalf("expected blocked mail operations to persist, got %+v", blockedOps)
	}
}
