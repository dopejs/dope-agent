package chat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/setupwizard"
	"github.com/dopejs/dope-agent/daemon/internal/skills"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

type testProvider struct {
	name string
}

func (p *testProvider) Name() string { return p.name }

func (p *testProvider) Complete(_ context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
	return llm.ProviderResponse{
		Output:       "reply:" + request.Model,
		FinishReason: "stop",
		Usage:        llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
	}, nil
}

func (p *testProvider) Stream(_ context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
	if emit != nil {
		if err := emit(llm.StreamChunk{Delta: "reply:", Output: "reply:"}); err != nil {
			return llm.ProviderResponse{}, err
		}
		if err := emit(llm.StreamChunk{Delta: request.Model, Output: "reply:" + request.Model, FinishReason: "stop", Usage: &llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}}); err != nil {
			return llm.ProviderResponse{}, err
		}
	}
	return llm.ProviderResponse{
		Output:       "reply:" + request.Model,
		FinishReason: "stop",
		Usage:        llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
	}, nil
}

func TestQueryReturnsSelectedSkillContractsAndEvents(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&testProvider{name: "chat-test"})
	registry := newChatSkillRegistry(t)
	eventBus := events.NewBus()
	service := NewService(dispatcher, nil, registry, eventBus, sqliteStore)

	result, err := service.Query(context.Background(), QueryInput{
		Provider: "chat-test",
		Model:    "model-a",
		Skills:   []string{"shared"},
		Query:    "hello",
		Scope:    events.Scope{RunID: "run_1"},
	})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(result.SkillContracts) != 1 {
		t.Fatalf("expected one selected skill contract, got %+v", result.SkillContracts)
	}
	declaration, ok := result.SkillContracts[0]["declaration"].(map[string]any)
	if !ok {
		t.Fatalf("expected declaration payload, got %+v", result.SkillContracts[0])
	}
	if declaration["consumerKind"] != "skill" || declaration["consumerId"] != "shared" || declaration["operationKind"] != "skill_selection" {
		t.Fatalf("expected shared skill declaration vocabulary, got %+v", declaration)
	}

	llmEvents := eventBus.List(events.Filter{Category: "llm"})
	if len(llmEvents) != 2 {
		t.Fatalf("expected requested and completed llm events, got %+v", llmEvents)
	}
	if _, ok := llmEvents[0].Payload["skillContracts"]; !ok {
		t.Fatalf("expected requested llm event to include skill contracts, got %+v", llmEvents[0].Payload)
	}
}

func TestStreamEmitsSelectedSkillContractsOnChunks(t *testing.T) {
	t.Parallel()

	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&testProvider{name: "chat-stream"})
	registry := newChatSkillRegistry(t)
	service := NewService(dispatcher, nil, registry, events.NewBus(), nil)

	chunks := []StreamChunk{}
	result, err := service.Stream(context.Background(), QueryInput{
		Provider: "chat-stream",
		Model:    "model-b",
		Skills:   []string{"shared"},
		Query:    "stream hello",
	}, func(chunk StreamChunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected two stream chunks, got %+v", chunks)
	}
	for _, chunk := range chunks {
		if len(chunk.SkillContracts) != 1 {
			t.Fatalf("expected selected skill contracts on each chunk, got %+v", chunk)
		}
	}
	if len(result.SkillContracts) != 1 {
		t.Fatalf("expected query result skill contracts, got %+v", result.SkillContracts)
	}
}

func TestQueryAssemblesBoundedCurrentSegmentContinuity(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		seed            []threads.ContinuityTurn
		wantStatus      threads.ContinuityStatus
		wantApplied     bool
		wantIncluded    int
		wantExcluded    int
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:         "empty",
			wantStatus:   threads.ContinuityStatusEmpty,
			wantIncluded: 0,
		},
		{
			name: "within-limit",
			seed: []threads.ContinuityTurn{
				chatContinuityTurn("turn_1", "seg_1", 1, threads.ContinuityRoleUser, "prior user", now),
				chatContinuityTurn("turn_2", "seg_1", 2, threads.ContinuityRoleAssistant, "prior assistant", now.Add(time.Minute)),
			},
			wantStatus:   threads.ContinuityStatusApplied,
			wantApplied:  true,
			wantIncluded: 2,
			wantContains: []string{"prior user", "prior assistant"},
		},
		{
			name:         "over-limit",
			seed:         chatContinuityTurns("seg_1", 14, now),
			wantStatus:   threads.ContinuityStatusApplied,
			wantApplied:  true,
			wantIncluded: threads.DefaultContinuityMaxPriorTurns,
			wantExcluded: 2,
			wantContains: []string{"prior-03", "prior-14"},
			wantNotContains: []string{
				"prior-01",
				"prior-02",
			},
		},
		{
			name: "age-limited",
			seed: []threads.ContinuityTurn{
				chatContinuityTurn("turn_old", "seg_1", 1, threads.ContinuityRoleUser, "too old", now.AddDate(0, 0, -31)),
				chatContinuityTurn("turn_new", "seg_1", 2, threads.ContinuityRoleUser, "fresh", now),
			},
			wantStatus:      threads.ContinuityStatusApplied,
			wantApplied:     true,
			wantIncluded:    1,
			wantExcluded:    1,
			wantContains:    []string{"fresh"},
			wantNotContains: []string{"too old"},
		},
		{
			name: "current-segment-only",
			seed: []threads.ContinuityTurn{
				chatContinuityTurn("turn_other", "seg_old", 1, threads.ContinuityRoleUser, "old segment", now),
				chatContinuityTurn("turn_current", "seg_1", 2, threads.ContinuityRoleUser, "current segment", now),
			},
			wantStatus:      threads.ContinuityStatusApplied,
			wantApplied:     true,
			wantIncluded:    1,
			wantContains:    []string{"current segment"},
			wantNotContains: []string{"old segment"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope"))
			if err != nil {
				t.Fatalf("NewSQLiteStore returned error: %v", err)
			}
			defer sqliteStore.Close()
			seedChatContinuityThread(t, ctx, sqliteStore, now)
			for _, turn := range tt.seed {
				if _, err := sqliteStore.SaveContinuityTurn(ctx, turn); err != nil {
					t.Fatalf("SaveContinuityTurn returned error: %v", err)
				}
			}

			provider := &capturingProvider{name: "continuity-test"}
			dispatcher := llm.NewDispatcher()
			dispatcher.RegisterProvider(provider)
			service := NewService(dispatcher, nil, nil, events.NewBus(), sqliteStore)

			result, err := service.Query(ctx, QueryInput{
				TenantID: "ten_1",
				ThreadID: "thr_1",
				Provider: "continuity-test",
				Model:    "model-a",
				Query:    "follow up",
			})
			if err != nil {
				t.Fatalf("Query returned error: %v", err)
			}
			if result.ContinuityStatus != tt.wantStatus || result.ContinuityApplied != tt.wantApplied {
				t.Fatalf("continuity status=%s applied=%v", result.ContinuityStatus, result.ContinuityApplied)
			}
			if result.ContinuityIncludedCount != tt.wantIncluded || result.ContinuityExcludedCount != tt.wantExcluded {
				t.Fatalf("included=%d excluded=%d", result.ContinuityIncludedCount, result.ContinuityExcludedCount)
			}
			if result.RequestTurnID == "" || result.ResponseTurnID == "" || result.ContinuityPreviewID == "" {
				t.Fatalf("expected persisted continuity IDs, got %+v", result)
			}
			for _, content := range tt.wantContains {
				if !provider.sawMessage(content) {
					t.Fatalf("expected provider messages to include %q in %+v", content, provider.requests)
				}
			}
			for _, content := range tt.wantNotContains {
				if provider.sawMessage(content) {
					t.Fatalf("expected provider messages to exclude %q in %+v", content, provider.requests)
				}
			}
		})
	}
}

func TestQueryRecordsResetBoundaryExclusions(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	seedChatContinuityThread(t, ctx, sqliteStore, now)
	if err := sqliteStore.UpsertThread(ctx, threads.Thread{
		ThreadID:                "thr_1",
		TenantID:                "ten_1",
		LifecycleState:          threads.LifecycleStateReset,
		CurrentSessionSegmentID: "seg_1",
		SourceKind:              threads.SourceKindChat,
		LastActivityAt:          now,
		CreatedAt:               now,
		UpdatedAt:               now,
		RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
		RedactionStatus:         threads.RedactionStatusRedacted,
	}); err != nil {
		t.Fatalf("UpsertThread reset returned error: %v", err)
	}
	if _, err := sqliteStore.SaveContinuityTurn(ctx, chatContinuityTurn("turn_pre_reset", "seg_old", 1, threads.ContinuityRoleUser, "pre reset context", now)); err != nil {
		t.Fatalf("SaveContinuityTurn pre-reset returned error: %v", err)
	}

	provider := &capturingProvider{name: "continuity-reset"}
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(provider)
	service := NewService(dispatcher, nil, nil, events.NewBus(), sqliteStore)
	result, err := service.Query(ctx, QueryInput{
		TenantID: "ten_1",
		ThreadID: "thr_1",
		Provider: "continuity-reset",
		Model:    "model-a",
		Query:    "follow up after reset",
	})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if result.ContinuityApplied || result.ContinuityIncludedCount != 0 || result.ContinuityExcludedCount != 1 {
		t.Fatalf("unexpected reset continuity result: %+v", result)
	}
	if provider.sawMessage("pre reset context") {
		t.Fatalf("pre-reset context was dispatched: %+v", provider.requests)
	}
	detail, found, err := sqliteStore.GetContinuityPreviewDetail(ctx, "ten_1", "thr_1", result.ContinuityPreviewID)
	if err != nil || !found {
		t.Fatalf("GetContinuityPreviewDetail found=%v err=%v", found, err)
	}
	if len(detail.Items) != 1 || detail.Items[0].ReasonCode != threads.ContinuityReasonResetBoundary {
		t.Fatalf("expected reset boundary preview evidence, got %+v", detail.Items)
	}
}

func TestQueryInjectsSafeArtifactExcerptsAndPreviewEvidence(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	seedChatContinuityThread(t, ctx, sqliteStore, now)
	turn := chatContinuityTurn("turn_with_artifact", "seg_1", 1, threads.ContinuityRoleUser, "prior user", now)
	turn.ArtifactExcerptRefs = []threads.RuntimeArtifactExcerpt{{
		ArtifactExcerptID:  "artex_1",
		TenantID:           "ten_1",
		ThreadID:           "thr_1",
		SessionSegmentID:   "seg_1",
		ContinuityTurnID:   "turn_with_artifact",
		ResourceKind:       "run",
		ResourceID:         "run_1",
		ExcerptText:        "visible artifact excerpt",
		CreatedAt:          now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    threads.RedactionStatusRedacted,
	}}
	if _, err := sqliteStore.SaveContinuityTurn(ctx, turn); err != nil {
		t.Fatalf("SaveContinuityTurn returned error: %v", err)
	}

	provider := &capturingProvider{name: "continuity-artifact"}
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(provider)
	service := NewService(dispatcher, nil, nil, events.NewBus(), sqliteStore)
	result, err := service.Query(ctx, QueryInput{
		TenantID: "ten_1",
		ThreadID: "thr_1",
		Provider: "continuity-artifact",
		Model:    "model-a",
		Query:    "follow up",
	})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if !provider.sawMessage("visible artifact excerpt") {
		t.Fatalf("expected provider messages to include artifact excerpt, got %+v", provider.requests)
	}
	detail, found, err := sqliteStore.GetContinuityPreviewDetail(ctx, "ten_1", "thr_1", result.ContinuityPreviewID)
	if err != nil || !found {
		t.Fatalf("GetContinuityPreviewDetail found=%v err=%v", found, err)
	}
	foundArtifact := false
	for _, item := range detail.Items {
		if item.ItemKind == threads.ContinuityItemArtifactExcerpt && item.Decision == threads.ContinuityDecisionIncluded && item.SafeSummary == "visible artifact excerpt" {
			foundArtifact = true
		}
	}
	if !foundArtifact {
		t.Fatalf("expected included artifact excerpt preview item, got %+v", detail.Items)
	}
	if detail.Preview.IncludedCount != 2 {
		t.Fatalf("expected preview included count to include turn and artifact items, got %+v", detail.Preview)
	}
}

func TestQuerySuppressesUnsafeContinuityContent(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	seedChatContinuityThread(t, ctx, sqliteStore, now)

	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&capturingProvider{name: "continuity-redaction"})
	service := NewService(dispatcher, nil, nil, events.NewBus(), sqliteStore)
	result, err := service.Query(ctx, QueryInput{
		TenantID: "ten_1",
		ThreadID: "thr_1",
		Provider: "continuity-redaction",
		Model:    "model-a",
		Query:    "api_key=sk-secretsecretsecret",
	})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	turns, err := sqliteStore.ListContinuityTurns(ctx, store.ContinuityLookupQuery{TenantID: "ten_1", ThreadID: "thr_1", SessionSegmentID: "seg_1", Now: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("ListContinuityTurns returned error: %v", err)
	}
	for _, turn := range turns {
		if turn.ContinuityTurnID == result.RequestTurnID {
			if turn.SafeContent != "suppressed" || turn.ContentRedactionStatus != threads.RedactionStatusSuppressed {
				t.Fatalf("expected unsafe request suppressed, got %+v", turn)
			}
			return
		}
	}
	t.Fatalf("request turn %s not found in %+v", result.RequestTurnID, turns)
}

func TestQueryDeduplicatesConnectorSourceEventRequestAndResponseTurns(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	seedChatContinuityThread(t, ctx, sqliteStore, now)

	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&capturingProvider{name: "continuity-dedupe"})
	service := NewService(dispatcher, nil, nil, events.NewBus(), sqliteStore)
	input := QueryInput{
		TenantID:        "ten_1",
		ThreadID:        "thr_1",
		Provider:        "continuity-dedupe",
		Model:           "model-a",
		Query:           "connector message",
		SourceKind:      threads.SourceKindChannel,
		SourceLinkageID: "src_1",
		SourceMessageID: "msg_1",
		SourceTimestamp: &now,
		SourceEventKey:  "connector:delivery_1",
	}
	if _, err := service.Query(ctx, input); err != nil {
		t.Fatalf("first Query returned error: %v", err)
	}
	if _, err := service.Query(ctx, input); err != nil {
		t.Fatalf("second Query returned error: %v", err)
	}
	turns, err := sqliteStore.ListContinuityTurns(ctx, store.ContinuityLookupQuery{TenantID: "ten_1", ThreadID: "thr_1", SessionSegmentID: "seg_1", Limit: 10, Now: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("ListContinuityTurns returned error: %v", err)
	}
	if len(turns) != 2 {
		t.Fatalf("expected duplicate connector event to keep one request/response pair, got %+v", turns)
	}
}

func TestQueryBlocksOpenAICompatibleWhenSetupSessionBlocksDependentUse(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()

	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&testProvider{name: llm.OpenAICompatibleProviderName})
	providerManager := providers.NewManager(config.Config{LLM: config.LLMConfig{
		DefaultProvider: llm.OpenAICompatibleProviderName,
		OpenAICompatible: config.OpenAICompatibleProviderConfig{
			BaseURL: "https://example.com",
			APIKey:  "secret",
			Model:   "gpt-4.1-mini",
		},
	}}, dispatcher)
	service := NewService(dispatcher, providerManager, nil, events.NewBus(), sqliteStore)
	if err := sqliteStore.SaveSetupSession(context.Background(), setupwizard.SetupSession{
		SetupSessionID:   "setup_blocked_openai",
		TenantID:         "ten_chat_setup",
		TargetID:         setupwizard.TargetOpenAICompatible,
		TargetKind:       setupwizard.TargetKindProvider,
		SetupStyle:       setupwizard.SetupStyleSubmittedSecret,
		State:            setupwizard.StateActionRequired,
		ReasonCode:       setupwizard.ReasonCredentialMissing,
		Retryable:        true,
		RemediationOwner: setupwizard.OwnerTenantAdmin,
		SafeUseMode:      setupwizard.SafeUseBlocked,
		RedactionStatus:  setupwizard.RedactionRedacted,
	}); err != nil {
		t.Fatalf("SaveSetupSession returned error: %v", err)
	}

	_, err = service.Query(context.Background(), QueryInput{
		TenantID: "ten_chat_setup",
		Query:    "hello",
	})
	if !errors.Is(err, providers.ErrProviderAuthUnavailable) {
		t.Fatalf("Query error=%v, want ErrProviderAuthUnavailable", err)
	}
}

type capturingProvider struct {
	name     string
	requests []llm.ProviderRequest
}

func (p *capturingProvider) Name() string { return p.name }

func (p *capturingProvider) Complete(_ context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
	p.requests = append(p.requests, request)
	return llm.ProviderResponse{
		Output:       "reply",
		FinishReason: "stop",
		Usage:        llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
	}, nil
}

func (p *capturingProvider) Stream(_ context.Context, request llm.ProviderRequest, _ llm.StreamEmitter) (llm.ProviderResponse, error) {
	p.requests = append(p.requests, request)
	return llm.ProviderResponse{Output: "reply", FinishReason: "stop"}, nil
}

func (p *capturingProvider) sawMessage(content string) bool {
	for _, request := range p.requests {
		for _, message := range request.Messages {
			if message.Content == content {
				return true
			}
		}
	}
	return false
}

func seedChatContinuityThread(t *testing.T, ctx context.Context, sqliteStore *store.SQLiteStore, now time.Time) {
	t.Helper()
	if err := sqliteStore.UpsertThread(ctx, threads.Thread{
		ThreadID:                "thr_1",
		TenantID:                "ten_1",
		LifecycleState:          threads.LifecycleStateActive,
		CurrentSessionSegmentID: "seg_1",
		SourceKind:              threads.SourceKindChat,
		LastActivityAt:          now,
		CreatedAt:               now,
		UpdatedAt:               now,
		RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
		RedactionStatus:         threads.RedactionStatusRedacted,
	}); err != nil {
		t.Fatalf("UpsertThread returned error: %v", err)
	}
	for _, segmentID := range []string{"seg_1", "seg_old"} {
		if err := sqliteStore.UpsertThreadSessionSegment(ctx, threads.SessionSegment{
			SessionSegmentID: segmentID,
			ThreadID:         "thr_1",
			TenantID:         "ten_1",
			Generation:       1,
			State:            "active",
			StartedAt:        now,
			LastActiveAt:     now,
		}); err != nil {
			t.Fatalf("UpsertThreadSessionSegment returned error: %v", err)
		}
	}
}

func chatContinuityTurns(segmentID string, count int, now time.Time) []threads.ContinuityTurn {
	turns := make([]threads.ContinuityTurn, 0, count)
	for i := 1; i <= count; i++ {
		turns = append(turns, chatContinuityTurn(fmt.Sprintf("turn_%02d", i), segmentID, int64(i), threads.ContinuityRoleUser, fmt.Sprintf("prior-%02d", i), now.Add(time.Duration(i)*time.Minute)))
	}
	return turns
}

func chatContinuityTurn(id, segmentID string, seq int64, role threads.ContinuityRole, content string, recordedAt time.Time) threads.ContinuityTurn {
	return threads.ContinuityTurn{
		ContinuityTurnID:       id,
		TenantID:               "ten_1",
		ThreadID:               "thr_1",
		SessionSegmentID:       segmentID,
		AcceptanceSequence:     seq,
		Role:                   role,
		SourceKind:             threads.SourceKindChat,
		SafeContent:            content,
		ContentRedactionStatus: threads.RedactionStatusRedacted,
		RecordedAt:             recordedAt,
		RetentionExpiresAt:     recordedAt.Add(90 * 24 * time.Hour),
	}
}

func newChatSkillRegistry(t *testing.T) *skills.Registry {
	t.Helper()

	homeRoot := filepath.Join(t.TempDir(), ".agents")
	dataRoot := filepath.Join(t.TempDir(), "dope-data")
	writeChatSkillFile(t, filepath.Join(homeRoot, "AGENTS.md"), "home overlay")
	writeChatSkillFile(t, filepath.Join(dataRoot, "AGENTS.md"), "data overlay")
	writeChatSkillFile(t, filepath.Join(dataRoot, "skills", "shared", "SKILL.md"), `---
name: shared
description: "data skill"
---
data instructions`)

	registry, err := skills.NewRegistryWithRoots(homeRoot, dataRoot)
	if err != nil {
		t.Fatalf("NewRegistryWithRoots returned error: %v", err)
	}
	return registry
}

func writeChatSkillFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}
