package chat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/skills"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type QueryInput struct {
	Query      string
	Provider   string
	Model      string
	Skills     []string
	TimeoutMs  int
	MaxRetries int
	Scope      events.Scope
}

type QueryResult struct {
	Query          string
	Skills         []string
	SkillContracts []map[string]any
	Dispatch       llm.Dispatch
}

type StreamChunk struct {
	DispatchID     string
	Provider       string
	Model          string
	Skills         []string
	SkillContracts []map[string]any
	Delta          string
	Reply          string
	FinishReason   string
	Usage          *llm.Usage
}

type Service struct {
	dispatcher *llm.Dispatcher
	providers  *providers.Manager
	skills     *skills.Registry
	eventBus   *events.Bus
	store      *store.SQLiteStore
}

func NewService(dispatcher *llm.Dispatcher, providerManager *providers.Manager, skillRegistry *skills.Registry, eventBus *events.Bus, sqliteStore *store.SQLiteStore) *Service {
	return &Service{
		dispatcher: dispatcher,
		providers:  providerManager,
		skills:     skillRegistry,
		eventBus:   eventBus,
		store:      sqliteStore,
	}
}

func (s *Service) Query(ctx context.Context, input QueryInput) (QueryResult, error) {
	if s == nil || s.dispatcher == nil {
		return QueryResult{}, errors.New("chat service is not configured")
	}

	dispatchInput, selectedSkills, err := s.buildDispatchInput(input)
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
	if _, err := publishDispatchEvent(ctx, s.eventBus, s.store, input.Scope, dispatch, selectedSkills, "llm.dispatch.requested"); err != nil {
		return QueryResult{}, err
	}

	finalDispatch, execErr := s.dispatcher.Dispatch(ctx, dispatch)
	if err := persistDispatch(ctx, s.store, finalDispatch); err != nil {
		return QueryResult{}, err
	}
	if _, err := publishDispatchEvent(ctx, s.eventBus, s.store, input.Scope, finalDispatch, selectedSkills, terminalDispatchEvent(finalDispatch)); err != nil {
		return QueryResult{}, err
	}

	result := QueryResult{
		Query:          strings.TrimSpace(input.Query),
		Skills:         selectedSkillIDsFromSkills(selectedSkills),
		SkillContracts: selectedSkillContracts(selectedSkills),
		Dispatch:       finalDispatch,
	}
	if execErr != nil {
		return result, execErr
	}
	return result, nil
}

func (s *Service) Stream(ctx context.Context, input QueryInput, emit func(StreamChunk) error) (QueryResult, error) {
	if s == nil || s.dispatcher == nil {
		return QueryResult{}, errors.New("chat service is not configured")
	}

	dispatchInput, selectedSkills, err := s.buildDispatchInput(input)
	if err != nil {
		return QueryResult{}, err
	}
	if s.providers != nil {
		_, dispatchInput, err = s.providers.ResolveDispatchInput(dispatchInput)
		if err != nil {
			return QueryResult{}, err
		}
	}

	dispatch, err := s.dispatcher.Prepare(dispatchInput, true)
	if err != nil {
		return QueryResult{}, err
	}
	if err := persistDispatch(ctx, s.store, dispatch); err != nil {
		return QueryResult{}, err
	}
	if _, err := publishDispatchEvent(ctx, s.eventBus, s.store, input.Scope, dispatch, selectedSkills, "llm.dispatch.requested"); err != nil {
		return QueryResult{}, err
	}

	finalDispatch, execErr := s.dispatcher.DispatchStream(ctx, dispatch, func(chunk llm.StreamChunk) error {
		if emit == nil {
			return nil
		}
		return emit(StreamChunk{
			DispatchID:     dispatch.DispatchID,
			Provider:       dispatch.Provider,
			Model:          dispatch.Model,
			Skills:         selectedSkillIDsFromSkills(selectedSkills),
			SkillContracts: selectedSkillContracts(selectedSkills),
			Delta:          chunk.Delta,
			Reply:          chunk.Output,
			FinishReason:   chunk.FinishReason,
			Usage:          chunk.Usage,
		})
	})
	if err := persistDispatch(ctx, s.store, finalDispatch); err != nil {
		return QueryResult{}, err
	}
	if _, err := publishDispatchEvent(ctx, s.eventBus, s.store, input.Scope, finalDispatch, selectedSkills, terminalDispatchEvent(finalDispatch)); err != nil {
		return QueryResult{}, err
	}

	result := QueryResult{
		Query:          strings.TrimSpace(input.Query),
		Skills:         selectedSkillIDsFromSkills(selectedSkills),
		SkillContracts: selectedSkillContracts(selectedSkills),
		Dispatch:       finalDispatch,
	}
	if execErr != nil {
		return result, execErr
	}
	return result, nil
}

func (s *Service) buildDispatchInput(input QueryInput) (llm.CreateDispatchInput, []skills.Skill, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return llm.CreateDispatchInput{}, nil, errors.New("query is required")
	}

	selectedSkills, err := resolveSelectedSkills(s.skills, input.Skills)
	if err != nil {
		return llm.CreateDispatchInput{}, nil, err
	}
	messages := compilePromptMessages(query, selectedSkills, availableOverlays(s.skills))

	return llm.CreateDispatchInput{
		Provider:   strings.TrimSpace(input.Provider),
		Model:      strings.TrimSpace(input.Model),
		Messages:   messages,
		TimeoutMs:  input.TimeoutMs,
		MaxRetries: input.MaxRetries,
	}, selectedSkills, nil
}

func resolveSelectedSkills(registry *skills.Registry, selected []string) ([]skills.Skill, error) {
	if len(selected) == 0 {
		return nil, nil
	}
	if registry == nil {
		return nil, skills.ErrSkillsRegistryMissing
	}
	return registry.ResolveSelected(selected)
}

func availableOverlays(registry *skills.Registry) []skills.Overlay {
	if registry == nil {
		return nil
	}
	return registry.Overlays()
}

func compilePromptMessages(query string, selected []skills.Skill, overlays []skills.Overlay) []llm.Message {
	messages := make([]llm.Message, 0, len(overlays)+len(selected)+1)
	for _, overlay := range overlays {
		if strings.TrimSpace(overlay.Body) == "" {
			continue
		}
		messages = append(messages, llm.Message{
			Role: llm.RoleSystem,
			Content: strings.TrimSpace(
				"Agent overlay (" + string(overlay.Source) + "):\n" + overlay.Body,
			),
		})
	}
	for _, skill := range selected {
		builder := strings.Builder{}
		builder.WriteString("Skill: ")
		builder.WriteString(skill.Name)
		if description := strings.TrimSpace(skill.Description); description != "" {
			builder.WriteString("\nDescription: ")
			builder.WriteString(description)
		}
		if body := strings.TrimSpace(skill.Body); body != "" {
			builder.WriteString("\nInstructions:\n")
			builder.WriteString(body)
		}
		messages = append(messages, llm.Message{
			Role:    llm.RoleSystem,
			Content: builder.String(),
		})
	}
	messages = append(messages, llm.Message{Role: llm.RoleUser, Content: strings.TrimSpace(query)})
	return messages
}

func selectedSkillIDsFromSkills(selected []skills.Skill) []string {
	items := make([]string, 0, len(selected))
	for _, skill := range selected {
		items = append(items, skill.SkillID)
	}
	return items
}

func persistDispatch(ctx context.Context, sqliteStore *store.SQLiteStore, dispatch llm.Dispatch) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertLLMDispatch(ctx, dispatch)
}

func publishDispatchEvent(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, scope events.Scope, dispatch llm.Dispatch, selected []skills.Skill, name string) (events.Event, error) {
	if eventBus == nil {
		return events.Event{}, nil
	}

	payload := map[string]any{
		"provider": dispatch.Provider,
		"model":    dispatch.Model,
	}
	switch name {
	case "llm.dispatch.requested":
		payload["stream"] = dispatch.Stream
		payload["timeoutMs"] = dispatch.TimeoutMs
		payload["maxRetries"] = dispatch.MaxRetries
		payload["status"] = dispatch.Status
	default:
		payload["status"] = dispatch.Status
		if name != "llm.dispatch.cancelled" {
			payload["partial"] = dispatch.Partial
		}
		payload["attemptCount"] = dispatch.AttemptCount
		payload["finishReason"] = dispatch.FinishReason
		payload["usage"] = dispatch.Usage
		payload["errorCode"] = dispatch.ErrorCode
		payload["error"] = dispatch.Error
	}
	if skillIDs := selectedSkillIDsFromSkills(selected); len(skillIDs) > 0 {
		payload["skills"] = skillIDs
	}
	if contracts := selectedSkillContracts(selected); len(contracts) > 0 {
		payload["skillContracts"] = contracts
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

func selectedSkillContracts(selected []skills.Skill) []map[string]any {
	if len(selected) == 0 {
		return nil
	}
	items := make([]map[string]any, 0, len(selected))
	for _, skill := range selected {
		if skill.Sandbox == nil {
			continue
		}
		payload, err := json.Marshal(skill.Sandbox)
		if err != nil {
			items = append(items, skill.Sandbox)
			continue
		}
		var cloned map[string]any
		if err := json.Unmarshal(payload, &cloned); err != nil {
			items = append(items, skill.Sandbox)
			continue
		}
		items = append(items, cloned)
	}
	return items
}

func terminalDispatchEvent(dispatch llm.Dispatch) string {
	switch dispatch.Status {
	case llm.DispatchStatusPartialFailed:
		return "llm.dispatch.partial_failed"
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
