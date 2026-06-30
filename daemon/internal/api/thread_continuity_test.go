package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestChatQueryRouteContinuityFieldsRemainAdditive(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	authManager := auth.NewManager()
	identityManager := identity.NewManager(sqliteStore)
	authHeader, tenantID := issueContinuityAuthHeader(t, authManager, identityManager)

	// Anchor to wall-clock: continuity windows use time.Now(), so a frozen past date ages
	// the seeded turns out of the window once real time advances.
	now := time.Now().UTC()
	seedAPIContinuityThread(t, sqliteStore, tenantID, now)
	if _, err := sqliteStore.SaveContinuityTurn(context.Background(), threads.ContinuityTurn{
		ContinuityTurnID:       "turn_prior",
		TenantID:               tenantID,
		ThreadID:               "thr_continuity",
		SessionSegmentID:       "seg_continuity",
		AcceptanceSequence:     1,
		Role:                   threads.ContinuityRoleUser,
		SourceKind:             threads.SourceKindChat,
		SafeContent:            "prior context",
		ContentRedactionStatus: threads.RedactionStatusRedacted,
		RecordedAt:             now,
		RetentionExpiresAt:     now.Add(90 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveContinuityTurn returned error: %v", err)
	}

	var captured []llm.Message
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&testLLMProvider{
		name: "echo",
		completeFn: func(ctx context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
			captured = append([]llm.Message(nil), request.Messages...)
			return llm.ProviderResponse{
				Output:       "chat reply",
				FinishReason: "stop",
				Usage:        llm.Usage{InputTokens: 2, OutputTokens: 2, TotalTokens: 4},
			}, nil
		},
		streamFn: func(ctx context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{}, nil
		},
	})
	testCfg := config.Config{BindAddr: "127.0.0.1:19191", DataDir: "~/.dope", LogLevel: "info", Version: "test", LLM: config.LLMConfig{DefaultTimeoutMs: 30000}}
	eventBus := events.NewBus()
	providerManager, chatService := newProviderManagerAndChatServiceForTests(testCfg, dispatcher, eventBus, sqliteStore, nil)
	server := NewServer(Dependencies{
		Config:    testCfg,
		Logger:    telemetry.New("error").Slog(),
		EventBus:  eventBus,
		Auth:      authManager,
		Identity:  identityManager,
		Router:    router.NewSessionRouter(),
		Runtime:   runtime.NewManager(),
		LLM:       dispatcher,
		Chat:      chatService,
		Providers: providerManager,
		Store:     sqliteStore,
	})

	legacyRec := httptest.NewRecorder()
	legacyReq := httptest.NewRequest(http.MethodPost, "/v1/chat/query", strings.NewReader(`{"provider":"echo","model":"echo-v1","query":"hello"}`))
	legacyReq.Header.Set("Authorization", authHeader)
	server.Handler().ServeHTTP(legacyRec, legacyReq)
	if legacyRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for legacy chat query, got %d body=%s", legacyRec.Code, legacyRec.Body.String())
	}
	legacyResponse := decodeStrictResponse[ChatQueryResponse](t, legacyRec.Body.Bytes())
	if legacyResponse.ThreadID != "" || legacyResponse.ContinuityApplied != nil || legacyResponse.ContinuityIncludedCount != nil {
		t.Fatalf("expected continuity fields to be omitted without threadId, got %+v", legacyResponse)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/query", strings.NewReader(`{"provider":"echo","model":"echo-v1","query":"follow up","threadId":"thr_continuity","continuity":{"mode":"auto"}}`))
	req.Header.Set("Authorization", authHeader)
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for continuity chat query, got %d body=%s", rec.Code, rec.Body.String())
	}
	response := decodeStrictResponse[ChatQueryResponse](t, rec.Body.Bytes())
	if response.ThreadID != "thr_continuity" || response.SessionSegmentID != "seg_continuity" {
		t.Fatalf("unexpected continuity identifiers: %+v", response)
	}
	if response.ContinuityApplied == nil || !*response.ContinuityApplied || response.ContinuityStatus != string(threads.ContinuityStatusApplied) {
		t.Fatalf("unexpected continuity status: %+v", response)
	}
	if response.RequestTurnID == "" || response.ResponseTurnID == "" || response.ContinuityPreviewID == "" {
		t.Fatalf("expected turn and preview IDs, got %+v", response)
	}
	if response.ContinuityIncludedCount == nil || *response.ContinuityIncludedCount != 1 {
		t.Fatalf("expected one included prior turn, got %+v", response.ContinuityIncludedCount)
	}
	if !messagesContain(captured, "prior context") {
		t.Fatalf("expected dispatched messages to include prior context, got %+v", captured)
	}
}

func TestResetContinuityRequiresManageAndExcludesPreResetTurns(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()

	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.UpsertThread(context.Background(), threads.Thread{
		ThreadID:                "thr_reset",
		TenantID:                "ten_threads",
		LifecycleState:          threads.LifecycleStateActive,
		CurrentSessionSegmentID: "seg_old",
		SourceKind:              threads.SourceKindChat,
		LastActivityAt:          now,
		CreatedAt:               now,
		UpdatedAt:               now,
		RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
		RedactionStatus:         threads.RedactionStatusRedacted,
	}); err != nil {
		t.Fatalf("UpsertThread returned error: %v", err)
	}
	if err := sqliteStore.UpsertThreadSessionSegment(context.Background(), threads.SessionSegment{SessionSegmentID: "seg_old", ThreadID: "thr_reset", TenantID: "ten_threads", Generation: 1, State: "active", StartedAt: now, LastActiveAt: now}); err != nil {
		t.Fatalf("UpsertThreadSessionSegment returned error: %v", err)
	}
	if _, err := sqliteStore.SaveContinuityTurn(context.Background(), threads.ContinuityTurn{
		ContinuityTurnID:       "turn_pre_reset",
		TenantID:               "ten_threads",
		ThreadID:               "thr_reset",
		SessionSegmentID:       "seg_old",
		AcceptanceSequence:     1,
		Role:                   threads.ContinuityRoleUser,
		SourceKind:             threads.SourceKindChat,
		SafeContent:            "pre reset context",
		ContentRedactionStatus: threads.RedactionStatusRedacted,
		RecordedAt:             now,
		RetentionExpiresAt:     now.Add(90 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveContinuityTurn returned error: %v", err)
	}

	deniedReq := httptest.NewRequest(http.MethodPost, "/v1/threads/thr_reset/reset", strings.NewReader(`{"reasonCode":"test"}`))
	deniedReq = deniedReq.WithContext(withTenantContext(deniedReq.Context(), threadTenantContext(identity.PermissionCredentialsInspect)))
	deniedRec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, events.NewBus(), deniedRec, deniedReq)
	if deniedRec.Code != http.StatusForbidden {
		t.Fatalf("expected reset denial without connectors.manage, got %d body=%s", deniedRec.Code, deniedRec.Body.String())
	}

	resetReq := httptest.NewRequest(http.MethodPost, "/v1/threads/thr_reset/reset", strings.NewReader(`{"reasonCode":"test"}`))
	resetReq = resetReq.WithContext(withTenantContext(resetReq.Context(), threadTenantContext(identity.PermissionConnectorsManage)))
	resetRec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, events.NewBus(), resetRec, resetReq)
	if resetRec.Code != http.StatusOK {
		t.Fatalf("expected reset success, got %d body=%s", resetRec.Code, resetRec.Body.String())
	}
	resetResponse := decodeStrictResponse[threadLifecycleActionResponse](t, resetRec.Body.Bytes())

	var captured []llm.Message
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&testLLMProvider{
		name: "echo",
		completeFn: func(ctx context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
			captured = append([]llm.Message(nil), request.Messages...)
			return llm.ProviderResponse{
				Output:       "reply",
				FinishReason: "stop",
				Usage:        llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
			}, nil
		},
		streamFn: func(ctx context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{}, nil
		},
	})
	eventBus := events.NewBus()
	providerManager, chatService := newProviderManagerAndChatServiceForTests(config.Config{LLM: config.LLMConfig{DefaultTimeoutMs: 30000}}, dispatcher, eventBus, sqliteStore, nil)
	_ = providerManager
	chatReq := httptest.NewRequest(http.MethodPost, "/v1/chat/query", strings.NewReader(`{"provider":"echo","model":"echo-v1","query":"follow up","threadId":"thr_reset"}`))
	chatReq = chatReq.WithContext(withTenantContext(chatReq.Context(), threadTenantContext(identity.PermissionCredentialsInspect)))
	chatRec := httptest.NewRecorder()
	handleChatQuery(chatService, chatRec, chatReq)
	if chatRec.Code != http.StatusOK {
		t.Fatalf("expected chat success, got %d body=%s", chatRec.Code, chatRec.Body.String())
	}
	chatResponse := decodeStrictResponse[ChatQueryResponse](t, chatRec.Body.Bytes())
	if chatResponse.SessionSegmentID != resetResponse.CurrentSessionSegmentID || chatResponse.ContinuityApplied == nil || *chatResponse.ContinuityApplied {
		t.Fatalf("unexpected post-reset continuity response: %+v reset=%+v", chatResponse, resetResponse)
	}
	if chatResponse.ContinuityExcludedCount == nil || *chatResponse.ContinuityExcludedCount != 1 || messagesContain(captured, "pre reset context") {
		t.Fatalf("expected pre-reset turn excluded from dispatch, response=%+v messages=%+v", chatResponse, captured)
	}
	detail, found, err := sqliteStore.GetContinuityPreviewDetail(context.Background(), "ten_threads", "thr_reset", chatResponse.ContinuityPreviewID)
	if err != nil || !found {
		t.Fatalf("GetContinuityPreviewDetail found=%v err=%v", found, err)
	}
	if len(detail.Items) != 1 || detail.Items[0].ReasonCode != threads.ContinuityReasonResetBoundary {
		t.Fatalf("expected reset-boundary evidence, got %+v", detail.Items)
	}
}

func TestThreadContinuityPreviewDetailRoute(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()

	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.UpsertThread(context.Background(), threads.Thread{
		ThreadID:                "thr_preview",
		TenantID:                "ten_threads",
		LifecycleState:          threads.LifecycleStateActive,
		CurrentSessionSegmentID: "seg_preview",
		SourceKind:              threads.SourceKindChat,
		LastActivityAt:          now,
		CreatedAt:               now,
		UpdatedAt:               now,
		RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
		RedactionStatus:         threads.RedactionStatusRedacted,
	}); err != nil {
		t.Fatalf("UpsertThread returned error: %v", err)
	}
	included := threads.ContinuityTurn{ContinuityTurnID: "turn_included", TenantID: "ten_threads", ThreadID: "thr_preview", SessionSegmentID: "seg_preview", AcceptanceSequence: 1, Role: threads.ContinuityRoleUser, SourceKind: threads.SourceKindChat, SafeContent: "included context", ContentRedactionStatus: threads.RedactionStatusRedacted, RecordedAt: now, RetentionExpiresAt: now.Add(90 * 24 * time.Hour)}
	excluded := included
	excluded.ContinuityTurnID = "turn_excluded"
	excluded.AcceptanceSequence = 2
	excluded.SafeContent = "excluded context"
	preview, err := sqliteStore.SaveContinuityPreview(context.Background(), threads.ContinuityPreview{
		ContinuityPreviewID: "contprev_preview",
		TenantID:            "ten_threads",
		ThreadID:            "thr_preview",
		SessionSegmentID:    "seg_preview",
		IncludedCount:       1,
		ExcludedCount:       1,
		ContinuityApplied:   true,
		Status:              threads.ContinuityStatusApplied,
		AssemblyStartedAt:   now,
		AssemblyCompletedAt: now.Add(time.Millisecond),
		RedactionStatus:     threads.RedactionStatusRedacted,
	}, []threads.ContinuityPreviewItem{
		threads.PreviewItemForTurn(included, threads.ContinuityDecisionIncluded, threads.ContinuityReasonIncludedRecent, 0),
		threads.PreviewItemForTurn(excluded, threads.ContinuityDecisionExcluded, threads.ContinuityReasonOverLimit, 1),
	})
	if err != nil {
		t.Fatalf("SaveContinuityPreview returned error: %v", err)
	}

	deniedReq := httptest.NewRequest(http.MethodGet, "/v1/threads/thr_preview/continuity-previews/"+preview.ContinuityPreviewID, nil)
	deniedReq = deniedReq.WithContext(withTenantContext(deniedReq.Context(), threadTenantContext(identity.PermissionConnectorsManage)))
	deniedRec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, events.NewBus(), deniedRec, deniedReq)
	if deniedRec.Code != http.StatusForbidden {
		t.Fatalf("expected credentials.inspect denial, got %d body=%s", deniedRec.Code, deniedRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/threads/thr_preview/continuity-previews/"+preview.ContinuityPreviewID, nil)
	req = req.WithContext(withTenantContext(req.Context(), threadTenantContext(identity.PermissionCredentialsInspect)))
	rec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, events.NewBus(), rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected preview detail success, got %d body=%s", rec.Code, rec.Body.String())
	}
	response := decodeStrictResponse[threads.ContinuityPreviewDetail](t, rec.Body.Bytes())
	if response.Preview.ContinuityPreviewID != preview.ContinuityPreviewID || len(response.Items) != 2 {
		t.Fatalf("unexpected preview detail response: %+v", response)
	}
	if response.Items[0].Decision != threads.ContinuityDecisionIncluded || response.Items[1].ReasonCode != threads.ContinuityReasonOverLimit {
		t.Fatalf("unexpected preview item evidence: %+v", response.Items)
	}
}

func TestChatQueryStreamContinuityParity(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()

	now := time.Now().UTC()
	if err := sqliteStore.UpsertThread(context.Background(), threads.Thread{ThreadID: "thr_stream", TenantID: "ten_threads", LifecycleState: threads.LifecycleStateActive, CurrentSessionSegmentID: "seg_stream", SourceKind: threads.SourceKindChat, LastActivityAt: now, CreatedAt: now, UpdatedAt: now, RetentionExpiresAt: now.Add(90 * 24 * time.Hour), RedactionStatus: threads.RedactionStatusRedacted}); err != nil {
		t.Fatalf("UpsertThread returned error: %v", err)
	}
	if err := sqliteStore.UpsertThreadSessionSegment(context.Background(), threads.SessionSegment{SessionSegmentID: "seg_stream", ThreadID: "thr_stream", TenantID: "ten_threads", Generation: 1, State: "active", StartedAt: now, LastActiveAt: now}); err != nil {
		t.Fatalf("UpsertThreadSessionSegment returned error: %v", err)
	}
	if _, err := sqliteStore.SaveContinuityTurn(context.Background(), threads.ContinuityTurn{ContinuityTurnID: "turn_stream_prior", TenantID: "ten_threads", ThreadID: "thr_stream", SessionSegmentID: "seg_stream", AcceptanceSequence: 1, Role: threads.ContinuityRoleUser, SourceKind: threads.SourceKindChat, SafeContent: "stream prior", ContentRedactionStatus: threads.RedactionStatusRedacted, RecordedAt: now, RetentionExpiresAt: now.Add(90 * 24 * time.Hour)}); err != nil {
		t.Fatalf("SaveContinuityTurn returned error: %v", err)
	}

	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&testLLMProvider{
		name: "echo",
		completeFn: func(ctx context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{}, nil
		},
		streamFn: func(ctx context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
			if err := emit(llm.StreamChunk{Delta: "reply", Output: "reply"}); err != nil {
				return llm.ProviderResponse{}, err
			}
			return llm.ProviderResponse{Output: "reply", FinishReason: "stop", Usage: llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}}, nil
		},
	})
	_, chatService := newProviderManagerAndChatServiceForTests(config.Config{LLM: config.LLMConfig{DefaultTimeoutMs: 30000}}, dispatcher, events.NewBus(), sqliteStore, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/query/stream", strings.NewReader(`{"provider":"echo","model":"echo-v1","query":"follow up","threadId":"thr_stream"}`))
	req = req.WithContext(withTenantContext(req.Context(), threadTenantContext(identity.PermissionCredentialsInspect)))
	rec := httptest.NewRecorder()
	handleChatQueryStream(chatService, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected stream success, got %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"threadId":"thr_stream"`) || !strings.Contains(body, `"continuityStatus":"applied"`) {
		t.Fatalf("expected stream continuity metadata, got %s", body)
	}
	startedIndex := strings.Index(body, "event: chat.query.started")
	deltaIndex := strings.Index(body, "event: chat.query.delta")
	if startedIndex < 0 || deltaIndex < 0 || startedIndex > deltaIndex {
		t.Fatalf("expected started event before delta, got %s", body)
	}
	startedBlock := body[startedIndex:deltaIndex]
	if !strings.Contains(startedBlock, `"continuityPreviewId":"contprev_`) {
		t.Fatalf("expected started event to include continuityPreviewId, got %s", startedBlock)
	}
}

func issueContinuityAuthHeader(t *testing.T, authManager *auth.Manager, identityManager *identity.Manager) (string, string) {
	t.Helper()
	pairing, code, err := authManager.StartPairing(auth.StartPairingInput{Mode: auth.PairingModeLocal, Label: "continuity-test"})
	if err != nil {
		t.Fatalf("StartPairing returned error: %v", err)
	}
	_, token, secret, err := authManager.CompletePairing(pairing.PairingID, auth.CompletePairingInput{Code: code})
	if err != nil {
		t.Fatalf("CompletePairing returned error: %v", err)
	}
	_, tenant, err := identityManager.BootstrapLocal(context.Background(), []string{token.TokenID})
	if err != nil {
		t.Fatalf("BootstrapLocal returned error: %v", err)
	}
	return "Bearer " + secret, tenant.TenantID
}

func seedAPIContinuityThread(t *testing.T, sqliteStore *store.SQLiteStore, tenantID string, now time.Time) {
	t.Helper()
	if err := sqliteStore.UpsertThread(context.Background(), threads.Thread{
		ThreadID:                "thr_continuity",
		TenantID:                tenantID,
		LifecycleState:          threads.LifecycleStateActive,
		CurrentSessionSegmentID: "seg_continuity",
		SourceKind:              threads.SourceKindChat,
		LastActivityAt:          now,
		CreatedAt:               now,
		UpdatedAt:               now,
		RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
		RedactionStatus:         threads.RedactionStatusRedacted,
	}); err != nil {
		t.Fatalf("UpsertThread returned error: %v", err)
	}
	if err := sqliteStore.UpsertThreadSessionSegment(context.Background(), threads.SessionSegment{
		SessionSegmentID: "seg_continuity",
		ThreadID:         "thr_continuity",
		TenantID:         tenantID,
		Generation:       1,
		State:            "active",
		StartedAt:        now,
		LastActiveAt:     now,
	}); err != nil {
		t.Fatalf("UpsertThreadSessionSegment returned error: %v", err)
	}
}

func messagesContain(messages []llm.Message, content string) bool {
	for _, message := range messages {
		if message.Content == content {
			return true
		}
	}
	return false
}
