package chat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type QueryInput struct {
	Query      string
	Provider   string
	Model      string
	TimeoutMs  int
	MaxRetries int
	Scope      events.Scope
}

type QueryResult struct {
	Query    string
	Dispatch llm.Dispatch
}

type Service struct {
	dispatcher *llm.Dispatcher
	providers  *providers.Manager
	eventBus   *events.Bus
	store      *store.SQLiteStore
}

func NewService(dispatcher *llm.Dispatcher, providerManager *providers.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore) *Service {
	return &Service{
		dispatcher: dispatcher,
		providers:  providerManager,
		eventBus:   eventBus,
		store:      sqliteStore,
	}
}

func (s *Service) Query(ctx context.Context, input QueryInput) (QueryResult, error) {
	if s == nil || s.dispatcher == nil {
		return QueryResult{}, errors.New("chat service is not configured")
	}

	dispatchInput, err := buildDispatchInput(input)
	if err != nil {
		return QueryResult{}, err
	}
	if s.providers != nil {
		_, dispatchInput, err = s.providers.ResolveDispatchInput(dispatchInput)
		if err != nil {
			return QueryResult{}, err
		}
	}

	dispatch, err := s.dispatcher.Prepare(dispatchInput, false)
	if err != nil {
		return QueryResult{}, err
	}
	if err := persistDispatch(ctx, s.store, dispatch); err != nil {
		return QueryResult{}, err
	}
	if _, err := publishDispatchEvent(ctx, s.eventBus, s.store, input.Scope, dispatch, "llm.dispatch.requested"); err != nil {
		return QueryResult{}, err
	}

	finalDispatch, execErr := s.dispatcher.Dispatch(ctx, dispatch)
	if err := persistDispatch(ctx, s.store, finalDispatch); err != nil {
		return QueryResult{}, err
	}
	if _, err := publishDispatchEvent(ctx, s.eventBus, s.store, input.Scope, finalDispatch, terminalDispatchEvent(finalDispatch)); err != nil {
		return QueryResult{}, err
	}

	result := QueryResult{
		Query:    input.Query,
		Dispatch: finalDispatch,
	}
	if execErr != nil {
		return result, execErr
	}
	return result, nil
}

func buildDispatchInput(input QueryInput) (llm.CreateDispatchInput, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return llm.CreateDispatchInput{}, errors.New("query is required")
	}
	return llm.CreateDispatchInput{
		Provider:   strings.TrimSpace(input.Provider),
		Model:      strings.TrimSpace(input.Model),
		Messages:   []llm.Message{{Role: llm.RoleUser, Content: query}},
		TimeoutMs:  input.TimeoutMs,
		MaxRetries: input.MaxRetries,
	}, nil
}

func persistDispatch(ctx context.Context, sqliteStore *store.SQLiteStore, dispatch llm.Dispatch) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertLLMDispatch(ctx, dispatch)
}

func publishDispatchEvent(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, scope events.Scope, dispatch llm.Dispatch, name string) (events.Event, error) {
	if eventBus == nil {
		return events.Event{}, nil
	}

	payload := map[string]any{
		"provider":     dispatch.Provider,
		"model":        dispatch.Model,
		"stream":       dispatch.Stream,
		"status":       dispatch.Status,
		"timeoutMs":    dispatch.TimeoutMs,
		"maxRetries":   dispatch.MaxRetries,
		"attemptCount": dispatch.AttemptCount,
		"finishReason": dispatch.FinishReason,
		"usage":        dispatch.Usage,
		"errorCode":    dispatch.ErrorCode,
		"error":        dispatch.Error,
	}

	event := events.Event{
		Category: "llm",
		Name:     name,
		Scope:    scope,
		Resource: events.Resource{
			Kind: "llm_dispatch",
			ID:   dispatch.DispatchID,
		},
		Payload: payload,
	}
	if event.EventID == "" {
		event.EventID = newEventID()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}

	if sqliteStore != nil {
		persisted, err := sqliteStore.AppendEvent(ctx, event)
		if err != nil {
			return events.Event{}, err
		}
		event = persisted
	}
	return eventBus.Publish(event), nil
}

func terminalDispatchEvent(dispatch llm.Dispatch) string {
	switch dispatch.Status {
	case llm.DispatchStatusFailed:
		return "llm.dispatch.failed"
	case llm.DispatchStatusCancelled:
		return "llm.dispatch.cancelled"
	default:
		return "llm.dispatch.completed"
	}
}

func newEventID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "evt_fallback"
	}
	return "evt_" + hex.EncodeToString(buf)
}
