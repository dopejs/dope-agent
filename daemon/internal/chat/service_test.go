package chat

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/skills"
	"github.com/dopejs/dope-agent/daemon/internal/store"
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
