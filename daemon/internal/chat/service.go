package chat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/setupwizard"
	"github.com/dopejs/dope-agent/daemon/internal/skills"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

type QueryInput struct {
	Query           string
	Provider        string
	Model           string
	Skills          []string
	TimeoutMs       int
	MaxRetries      int
	TenantID        string
	ThreadID        string
	ContinuityMode  threads.ContinuityMode
	Scope           events.Scope
	SourceKind      threads.SourceKind
	SourceLinkageID string
	SourceMessageID string
	SourceTimestamp *time.Time
	SourceEventKey  string
}

type QueryResult struct {
	Query                   string
	Skills                  []string
	SkillContracts          []map[string]any
	Dispatch                llm.Dispatch
	ThreadID                string
	SessionSegmentID        string
	RequestTurnID           string
	ResponseTurnID          string
	ContinuityPreviewID     string
	ContinuityApplied       bool
	ContinuityStatus        threads.ContinuityStatus
	ContinuityIncludedCount int
	ContinuityExcludedCount int
}

type StreamChunk struct {
	DispatchID          string
	Provider            string
	Model               string
	Skills              []string
	SkillContracts      []map[string]any
	Delta               string
	Reply               string
	FinishReason        string
	Usage               *llm.Usage
	ThreadID            string
	SessionSegmentID    string
	RequestTurnID       string
	ContinuityPreviewID string
	ContinuityApplied   bool
	ContinuityStatus    threads.ContinuityStatus
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
	if err := s.enforceProviderSetupGate(ctx, input.TenantID, dispatchInput.Provider, "chat"); err != nil {
		return QueryResult{}, err
	}
	continuity, err := s.prepareContinuity(ctx, input, &dispatchInput)
	if err != nil {
		return QueryResult{}, err
	}

	dispatch, err := s.dispatcher.Prepare(dispatchInput, false)
	if err != nil {
		return QueryResult{}, err
	}
	if err := persistDispatch(ctx, s.store, dispatch); err != nil {
		return QueryResult{}, err
	}
	if err := s.persistContinuityRequest(ctx, &continuity, input, dispatch.DispatchID, strings.TrimSpace(input.Query)); err != nil {
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
	if err := s.persistContinuityResponse(ctx, &continuity, input, finalDispatch); err != nil {
		return QueryResult{}, err
	}

	result := QueryResult{
		Query:          strings.TrimSpace(input.Query),
		Skills:         selectedSkillIDsFromSkills(selectedSkills),
		SkillContracts: selectedSkillContracts(selectedSkills),
		Dispatch:       finalDispatch,
	}
	applyContinuityResult(&result, continuity)
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
	if err := s.enforceProviderSetupGate(ctx, input.TenantID, dispatchInput.Provider, "chat"); err != nil {
		return QueryResult{}, err
	}
	continuity, err := s.prepareContinuity(ctx, input, &dispatchInput)
	if err != nil {
		return QueryResult{}, err
	}

	dispatch, err := s.dispatcher.Prepare(dispatchInput, true)
	if err != nil {
		return QueryResult{}, err
	}
	if err := persistDispatch(ctx, s.store, dispatch); err != nil {
		return QueryResult{}, err
	}
	if err := s.persistContinuityRequest(ctx, &continuity, input, dispatch.DispatchID, strings.TrimSpace(input.Query)); err != nil {
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
			DispatchID:          dispatch.DispatchID,
			Provider:            dispatch.Provider,
			Model:               dispatch.Model,
			Skills:              selectedSkillIDsFromSkills(selectedSkills),
			SkillContracts:      selectedSkillContracts(selectedSkills),
			Delta:               chunk.Delta,
			Reply:               chunk.Output,
			FinishReason:        chunk.FinishReason,
			Usage:               chunk.Usage,
			ThreadID:            continuity.ThreadID,
			SessionSegmentID:    continuity.SessionSegmentID,
			RequestTurnID:       continuity.RequestTurnID,
			ContinuityPreviewID: continuity.PreviewID,
			ContinuityApplied:   continuity.Applied,
			ContinuityStatus:    continuity.Status,
		})
	})
	if err := persistDispatch(ctx, s.store, finalDispatch); err != nil {
		return QueryResult{}, err
	}
	if _, err := publishDispatchEvent(ctx, s.eventBus, s.store, input.Scope, finalDispatch, selectedSkills, terminalDispatchEvent(finalDispatch)); err != nil {
		return QueryResult{}, err
	}
	if err := s.persistContinuityResponse(ctx, &continuity, input, finalDispatch); err != nil {
		return QueryResult{}, err
	}

	result := QueryResult{
		Query:          strings.TrimSpace(input.Query),
		Skills:         selectedSkillIDsFromSkills(selectedSkills),
		SkillContracts: selectedSkillContracts(selectedSkills),
		Dispatch:       finalDispatch,
	}
	applyContinuityResult(&result, continuity)
	if execErr != nil {
		return result, execErr
	}
	return result, nil
}

func (s *Service) enforceProviderSetupGate(ctx context.Context, tenantID, providerID, capability string) error {
	if s == nil || s.store == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(providerID) != llm.OpenAICompatibleProviderName {
		return nil
	}
	sessions, err := s.store.ListSetupSessions(ctx, strings.TrimSpace(tenantID))
	if err != nil {
		return err
	}
	for _, session := range sessions {
		if session.TargetID != setupwizard.TargetOpenAICompatible {
			continue
		}
		decision := setupwizard.NewService(setupwizard.ServiceDependencies{}).DependentUseDecision(ctx, session, capability)
		if decision.SafeUseMode == setupwizard.SafeUseBlocked {
			reason := decision.ReasonCode
			if reason == "" {
				reason = string(session.State)
			}
			return fmt.Errorf("%w: %s", providers.ErrProviderAuthUnavailable, reason)
		}
		return nil
	}
	return nil
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

type continuityAssembly struct {
	Enabled          bool
	TenantID         string
	ThreadID         string
	SessionSegmentID string
	RequestTurnID    string
	ResponseTurnID   string
	PreviewID        string
	Applied          bool
	Status           threads.ContinuityStatus
	Included         []threads.ContinuityTurn
	ExcludedItems    []threads.ContinuityPreviewItem
	HandoffLinkIDs   []string
	HandoffItems     []threads.ContinuityPreviewItem
	StartedAt        time.Time
	CompletedAt      time.Time
}

func (s *Service) prepareContinuity(ctx context.Context, input QueryInput, dispatchInput *llm.CreateDispatchInput) (continuityAssembly, error) {
	threadID := strings.TrimSpace(input.ThreadID)
	tenantID := strings.TrimSpace(input.TenantID)
	if s == nil || s.store == nil || threadID == "" || tenantID == "" {
		return continuityAssembly{}, nil
	}
	started := time.Now().UTC()
	mode := threads.NormalizeContinuityMode(input.ContinuityMode)
	thread, found, err := s.store.GetThreadForTenant(ctx, tenantID, threadID)
	if err != nil {
		return continuityAssembly{}, err
	}
	if !found || strings.TrimSpace(thread.CurrentSessionSegmentID) == "" {
		return continuityAssembly{}, nil
	}
	assembly := continuityAssembly{
		Enabled:          true,
		TenantID:         tenantID,
		ThreadID:         thread.ThreadID,
		SessionSegmentID: thread.CurrentSessionSegmentID,
		PreviewID:        newContinuityPreviewID(),
		Status:           threads.ContinuityStatusEmpty,
		StartedAt:        started,
	}
	if mode == threads.ContinuityModeDisabled {
		assembly.Status = threads.ContinuityStatusDisabled
		assembly.CompletedAt = time.Now().UTC()
		return assembly, nil
	}
	if thread.LifecycleState == threads.LifecycleStateArchived {
		assembly.Status = threads.ContinuityStatusBlocked
		assembly.ExcludedItems = append(assembly.ExcludedItems, threads.ContinuityPreviewItem{
			TenantID:        tenantID,
			ThreadID:        threadID,
			ItemKind:        threads.ContinuityItemTurn,
			Decision:        threads.ContinuityDecisionExcluded,
			ReasonCode:      threads.ContinuityReasonLifecycleBlocked,
			SafeSummary:     "Continuity blocked while thread is archived",
			RedactionStatus: threads.RedactionStatusRedacted,
		})
		assembly.CompletedAt = time.Now().UTC()
		return assembly, nil
	}
	turns, err := s.store.ListContinuityTurns(ctx, store.ContinuityLookupQuery{
		TenantID:         tenantID,
		ThreadID:         threadID,
		SessionSegmentID: thread.CurrentSessionSegmentID,
		Now:              started,
	})
	if err != nil {
		return continuityAssembly{}, err
	}
	if thread.LifecycleState == threads.LifecycleStateReset {
		resetTurns, err := s.store.ListContinuityTurnsOutsideSessionSegment(ctx, store.ContinuityLookupQuery{
			TenantID:         tenantID,
			ThreadID:         threadID,
			SessionSegmentID: thread.CurrentSessionSegmentID,
			Now:              started,
		})
		if err != nil {
			return continuityAssembly{}, err
		}
		assembly.ExcludedItems = append(assembly.ExcludedItems, threads.ResetBoundaryPreviewItems(resetTurns, len(assembly.ExcludedItems))...)
	}
	handoffRefs, handoffLinkIDs, handoffItems, err := s.availableHandoffSourceReferences(ctx, tenantID, threadID)
	if err != nil {
		return continuityAssembly{}, err
	}
	if len(handoffRefs) > 0 {
		dispatchInput.Messages = injectHandoffSourceReferenceMessages(dispatchInput.Messages, handoffRefs)
		assembly.HandoffLinkIDs = handoffLinkIDs
	}
	assembly.HandoffItems = handoffItems
	included, excluded := threads.EligibleContinuityTurns(turns, threads.DefaultContinuityPolicy(), started)
	assembly.Included = included
	assembly.ExcludedItems = append(assembly.ExcludedItems, excluded...)
	if len(included) > 0 {
		dispatchInput.Messages = injectContinuityMessages(dispatchInput.Messages, included)
		assembly.Applied = true
		assembly.Status = threads.ContinuityStatusApplied
	}
	assembly.CompletedAt = time.Now().UTC()
	return assembly, nil
}

func (s *Service) availableHandoffSourceReferences(ctx context.Context, tenantID, destinationThreadID string) ([]threads.HandoffSourceReference, []string, []threads.ContinuityPreviewItem, error) {
	links, err := s.store.ListHandoffLinksForThread(ctx, tenantID, destinationThreadID, 20)
	if err != nil {
		return nil, nil, nil, err
	}
	refs := []threads.HandoffSourceReference{}
	linkIDs := []string{}
	items := []threads.ContinuityPreviewItem{}
	now := time.Now().UTC()
	for _, link := range links {
		if link.DestinationThreadID != destinationThreadID || link.Status != threads.HandoffStatusSucceeded || link.SourceReferenceStatus != threads.HandoffSourceReferenceAvailable {
			continue
		}
		linkRefs, err := s.store.ListHandoffSourceReferencesForLink(ctx, tenantID, link.HandoffLinkID)
		if err != nil {
			return nil, nil, nil, err
		}
		hasReferenced := false
		for _, ref := range linkRefs {
			eligible := ref.Decision == threads.HandoffReferenceDecisionReferenced &&
				ref.EligibilityStatus == threads.HandoffReferenceEligible &&
				strings.TrimSpace(ref.SafeSummary) != "" &&
				(ref.RetentionExpiresAt.IsZero() || ref.RetentionExpiresAt.After(now))
			items = append(items, previewItemForHandoffSourceReference(ref, eligible, len(items)))
			if !eligible {
				continue
			}
			refs = append(refs, ref)
			hasReferenced = true
		}
		if hasReferenced {
			linkIDs = append(linkIDs, link.HandoffLinkID)
		}
	}
	return refs, linkIDs, items, nil
}

func previewItemForHandoffSourceReference(ref threads.HandoffSourceReference, included bool, order int) threads.ContinuityPreviewItem {
	decision := threads.ContinuityDecisionExcluded
	reason := handoffReferenceContinuityReason(ref)
	if included {
		decision = threads.ContinuityDecisionIncluded
		reason = threads.ContinuityReasonIncludedRecent
	}
	return threads.ContinuityPreviewItem{
		TenantID:           ref.TenantID,
		ThreadID:           ref.DestinationThreadID,
		ItemKind:           threads.ContinuityItemHandoffSource,
		ContinuityTurnID:   ref.ContinuityTurnID,
		HandoffSourceRefID: ref.HandoffSourceReferenceID,
		Decision:           decision,
		ReasonCode:         reason,
		SafeSummary:        ref.SafeSummary,
		RedactionStatus:    ref.RedactionStatus,
		ItemOrder:          order,
	}
}

func handoffReferenceContinuityReason(ref threads.HandoffSourceReference) threads.ContinuityReason {
	if !ref.RetentionExpiresAt.IsZero() && !ref.RetentionExpiresAt.After(time.Now().UTC()) {
		return threads.ContinuityReasonRetentionExpired
	}
	switch ref.EligibilityStatus {
	case threads.HandoffReferencePermissionDenied:
		return threads.ContinuityReasonPermissionDenied
	case threads.HandoffReferenceRedactionFailed:
		return threads.ContinuityReasonRedactionFailed
	case threads.HandoffReferenceRetentionExpired:
		return threads.ContinuityReasonRetentionExpired
	case threads.HandoffReferenceResetBoundary:
		return threads.ContinuityReasonResetBoundary
	case threads.HandoffReferenceIncompleteEvidence:
		return threads.ContinuityReasonIncompleteEvidence
	case threads.HandoffReferenceUnsupported:
		return threads.ContinuityReasonUnsupportedSource
	default:
		if ref.Decision != threads.HandoffReferenceDecisionReferenced {
			return threads.ContinuityReasonContinuityUnavailable
		}
		return threads.ContinuityReasonIncludedRecent
	}
}

func injectHandoffSourceReferenceMessages(messages []llm.Message, refs []threads.HandoffSourceReference) []llm.Message {
	if len(refs) == 0 {
		return messages
	}
	prior := make([]llm.Message, 0, len(refs))
	for _, ref := range refs {
		summary := strings.TrimSpace(ref.SafeSummary)
		if summary != "" {
			prior = append(prior, llm.Message{Role: llm.RoleUser, Content: summary})
		}
	}
	if len(prior) == 0 {
		return messages
	}
	out := make([]llm.Message, 0, len(messages)+len(prior))
	inserted := false
	for _, message := range messages {
		if !inserted && message.Role == llm.RoleUser {
			out = append(out, prior...)
			inserted = true
		}
		out = append(out, message)
	}
	if !inserted {
		out = append(out, prior...)
	}
	return out
}

func injectContinuityMessages(messages []llm.Message, turns []threads.ContinuityTurn) []llm.Message {
	if len(turns) == 0 {
		return messages
	}
	prior := make([]llm.Message, 0, len(turns))
	for _, turn := range turns {
		role := llm.RoleUser
		if turn.Role == threads.ContinuityRoleAssistant {
			role = llm.RoleAssistant
		}
		content := strings.TrimSpace(turn.SafeContent)
		if content != "" {
			prior = append(prior, llm.Message{Role: role, Content: content})
		}
		for _, excerpt := range turn.ArtifactExcerptRefs {
			if excerpt.RedactionStatus != threads.RedactionStatusRedacted {
				continue
			}
			summary := threads.SafeContinuityContent(excerpt.ExcerptText)
			if summary.Status != threads.RedactionStatusRedacted || strings.TrimSpace(summary.Text) == "" {
				continue
			}
			prior = append(prior, llm.Message{Role: role, Content: summary.Text})
		}
	}
	if len(prior) == 0 {
		return messages
	}
	out := make([]llm.Message, 0, len(messages)+len(prior))
	inserted := false
	for _, message := range messages {
		if !inserted && message.Role == llm.RoleUser {
			out = append(out, prior...)
			inserted = true
		}
		out = append(out, message)
	}
	if !inserted {
		out = append(out, prior...)
	}
	return out
}

func (s *Service) persistContinuityRequest(ctx context.Context, assembly *continuityAssembly, input QueryInput, dispatchID, query string) error {
	if assembly == nil || !assembly.Enabled || s == nil || s.store == nil {
		return nil
	}
	now := time.Now().UTC()
	safeContent := threads.SafeContinuityContent(query)
	turn, err := s.store.SaveContinuityTurn(ctx, threads.ContinuityTurn{
		TenantID:               assembly.TenantID,
		ThreadID:               assembly.ThreadID,
		SessionSegmentID:       assembly.SessionSegmentID,
		Role:                   threads.ContinuityRoleUser,
		SourceKind:             continuitySourceKind(input.SourceKind),
		SourceLinkageID:        strings.TrimSpace(input.SourceLinkageID),
		SourceMessageID:        strings.TrimSpace(input.SourceMessageID),
		SourceTimestamp:        input.SourceTimestamp,
		DispatchID:             dispatchID,
		SafeContent:            safeContent.Text,
		ContentRedactionStatus: safeContent.Status,
		RecordedAt:             now,
		SourceEventKey:         strings.TrimSpace(input.SourceEventKey),
	})
	if err != nil {
		return err
	}
	assembly.RequestTurnID = turn.ContinuityTurnID
	return s.publishContinuityEvent(ctx, events.ThreadContinuityTurnRecordedEvent(turn, "recorded"))
}

func (s *Service) persistContinuityResponse(ctx context.Context, assembly *continuityAssembly, input QueryInput, dispatch llm.Dispatch) error {
	if assembly == nil || !assembly.Enabled || s == nil || s.store == nil {
		return nil
	}
	tenantID := assembly.TenantID
	now := time.Now().UTC()
	if dispatch.Output != "" {
		safeContent := threads.SafeContinuityContent(dispatch.Output)
		turn, err := s.store.SaveContinuityTurn(ctx, threads.ContinuityTurn{
			TenantID:               tenantID,
			ThreadID:               assembly.ThreadID,
			SessionSegmentID:       assembly.SessionSegmentID,
			Role:                   threads.ContinuityRoleAssistant,
			SourceKind:             continuitySourceKind(input.SourceKind),
			SourceLinkageID:        strings.TrimSpace(input.SourceLinkageID),
			SourceTimestamp:        input.SourceTimestamp,
			DispatchID:             dispatch.DispatchID,
			ResponseToTurnID:       assembly.RequestTurnID,
			SafeContent:            safeContent.Text,
			ContentRedactionStatus: safeContent.Status,
			RecordedAt:             now,
			SourceEventKey:         responseContinuitySourceEventKey(input.SourceEventKey),
		})
		if err != nil {
			return err
		}
		assembly.ResponseTurnID = turn.ContinuityTurnID
		if err := s.publishContinuityEvent(ctx, events.ThreadContinuityTurnRecordedEvent(turn, "recorded")); err != nil {
			return err
		}
	}
	for _, handoffLinkID := range assembly.HandoffLinkIDs {
		if err := s.store.MarkHandoffSourceReferencesConsumed(ctx, tenantID, handoffLinkID, assembly.ResponseTurnID, now); err != nil {
			return err
		}
	}
	items := make([]threads.ContinuityPreviewItem, 0, len(assembly.HandoffItems)+len(assembly.Included)+len(assembly.ExcludedItems))
	for _, item := range assembly.HandoffItems {
		item.ItemOrder = len(items)
		items = append(items, item)
	}
	for _, turn := range assembly.Included {
		items = append(items, threads.PreviewItemForTurn(turn, threads.ContinuityDecisionIncluded, threads.ContinuityReasonIncludedRecent, len(items)))
		items = append(items, threads.PreviewItemsForArtifactExcerpts(turn, len(items), now)...)
	}
	for _, item := range assembly.ExcludedItems {
		item.ItemOrder = len(items)
		items = append(items, item)
	}
	itemIncluded, itemExcluded := continuityPreviewItemCounts(items)
	preview, err := s.store.SaveContinuityPreview(ctx, threads.ContinuityPreview{
		ContinuityPreviewID: assembly.PreviewID,
		TenantID:            tenantID,
		ThreadID:            assembly.ThreadID,
		SessionSegmentID:    assembly.SessionSegmentID,
		DispatchID:          dispatch.DispatchID,
		RequestTurnID:       assembly.RequestTurnID,
		ResponseTurnID:      assembly.ResponseTurnID,
		IncludedCount:       itemIncluded,
		ExcludedCount:       itemExcluded,
		ContinuityApplied:   assembly.Applied,
		Status:              assembly.Status,
		AssemblyStartedAt:   assembly.StartedAt,
		AssemblyCompletedAt: assembly.CompletedAt,
		RedactionStatus:     threads.RedactionStatusRedacted,
	}, items)
	if err != nil {
		return err
	}
	assembly.PreviewID = preview.ContinuityPreviewID
	return s.publishContinuityEvent(ctx, events.ThreadContinuityPreviewRecordedEvent(preview))
}

func (s *Service) publishContinuityEvent(ctx context.Context, event events.Event) error {
	if s == nil {
		return nil
	}
	if event.EventID == "" {
		event.EventID = newEventID()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if s.store != nil {
		persisted, err := s.store.AppendEvent(ctx, event)
		if err != nil {
			return err
		}
		event = persisted
	}
	if s.eventBus == nil {
		return nil
	}
	s.eventBus.Publish(event)
	return nil
}

func applyContinuityResult(result *QueryResult, assembly continuityAssembly) {
	if result == nil || !assembly.Enabled {
		return
	}
	result.ThreadID = assembly.ThreadID
	result.SessionSegmentID = assembly.SessionSegmentID
	result.RequestTurnID = assembly.RequestTurnID
	result.ResponseTurnID = assembly.ResponseTurnID
	result.ContinuityPreviewID = assembly.PreviewID
	result.ContinuityApplied = assembly.Applied
	result.ContinuityStatus = assembly.Status
	result.ContinuityIncludedCount = len(assembly.Included)
	result.ContinuityExcludedCount = len(assembly.ExcludedItems)
}

func continuityPreviewItemCounts(items []threads.ContinuityPreviewItem) (int, int) {
	included := 0
	excluded := 0
	for _, item := range items {
		switch item.Decision {
		case threads.ContinuityDecisionIncluded:
			included++
		case threads.ContinuityDecisionExcluded:
			excluded++
		}
	}
	return included, excluded
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

func continuitySourceKind(kind threads.SourceKind) threads.SourceKind {
	switch kind {
	case threads.SourceKindChannel, threads.SourceKindWorkflow, threads.SourceKindSchedule, threads.SourceKindShell, threads.SourceKindLegacy:
		return kind
	default:
		return threads.SourceKindChat
	}
}

func responseContinuitySourceEventKey(sourceEventKey string) string {
	sourceEventKey = strings.TrimSpace(sourceEventKey)
	if sourceEventKey == "" {
		return ""
	}
	return sourceEventKey + ":assistant"
}

func newContinuityPreviewID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "contprev_fallback"
	}
	return "contprev_" + hex.EncodeToString(buf)
}

func newEventID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "evt_fallback"
	}
	return "evt_" + hex.EncodeToString(buf)
}
