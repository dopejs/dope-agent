package delivery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type Manager struct {
	environmentScope string
	eventBus         *events.Bus
	sqliteStore      *store.SQLiteStore
	adapters         []Adapter
	mu               sync.Mutex
	retryScheduled   map[string]struct{}
	windowScheduled  map[string]struct{}
	maxAttempts      int
	baseRetryDelay   time.Duration
	maxRetryDelay    time.Duration
}

func NewManager(environmentScope string, eventBus *events.Bus, sqliteStore *store.SQLiteStore, adapters ...Adapter) *Manager {
	items := make([]Adapter, 0, len(adapters))
	items = append(items, adapters...)
	return &Manager{
		environmentScope: environmentScope,
		eventBus:         eventBus,
		sqliteStore:      sqliteStore,
		adapters:         items,
		retryScheduled:   map[string]struct{}{},
		windowScheduled:  map[string]struct{}{},
		maxAttempts:      3,
		baseRetryDelay:   5 * time.Second,
		maxRetryDelay:    time.Minute,
	}
}

func (m *Manager) ConfigureForTesting(maxAttempts int, baseRetryDelay, maxRetryDelay time.Duration) {
	if m == nil {
		return
	}
	if maxAttempts > 0 {
		m.maxAttempts = maxAttempts
	}
	if baseRetryDelay > 0 {
		m.baseRetryDelay = baseRetryDelay
	}
	if maxRetryDelay > 0 {
		m.maxRetryDelay = maxRetryDelay
	}
}

func (m *Manager) CreateTarget(ctx context.Context, target DeliveryTarget) (DeliveryTarget, error) {
	if m == nil || m.sqliteStore == nil {
		return DeliveryTarget{}, errors.New("delivery manager is not configured")
	}
	now := time.Now().UTC()
	if strings.TrimSpace(target.TargetID) == "" {
		return DeliveryTarget{}, errors.New("targetId is required")
	}
	if strings.TrimSpace(target.DisplayName) == "" {
		return DeliveryTarget{}, errors.New("displayName is required")
	}
	if target.TargetKind == "" {
		return DeliveryTarget{}, errors.New("targetKind is required")
	}
	if target.EnvironmentScope == "" {
		target.EnvironmentScope = m.environmentScope
	}
	if target.Status == "" {
		target.Status = TargetStatusActive
	}
	if target.CreatedAt.IsZero() {
		target.CreatedAt = now
	}
	target.UpdatedAt = now
	target.SupportsImmediate = true
	if target.TargetKind == TargetKindTestSink {
		target.SupportsDigest = true
	}
	if err := m.sqliteStore.UpsertDeliveryTarget(ctx, store.DeliveryTargetRecord{
		TargetID:         target.TargetID,
		EnvironmentScope: target.EnvironmentScope,
		TargetKind:       string(target.TargetKind),
		Status:           string(target.Status),
		UpdatedAt:        target.UpdatedAt,
		Document:         mustMarshal(target),
	}); err != nil {
		return DeliveryTarget{}, err
	}
	if err := m.publishEvent(ctx, "delivery.target_registered", events.Resource{Kind: "delivery_target", ID: target.TargetID}, map[string]any{
		"targetId":         target.TargetID,
		"targetKind":       target.TargetKind,
		"environmentScope": target.EnvironmentScope,
		"status":           target.Status,
	}); err != nil {
		return DeliveryTarget{}, err
	}
	return target, nil
}

func (m *Manager) ListTargets(ctx context.Context) ([]DeliveryTarget, error) {
	if m == nil || m.sqliteStore == nil {
		return nil, nil
	}
	records, err := m.sqliteStore.ListDeliveryTargets(ctx, m.environmentScope)
	if err != nil {
		return nil, err
	}
	items := make([]DeliveryTarget, 0, len(records))
	for _, record := range records {
		var target DeliveryTarget
		if err := unmarshalDocument(record.Document, &target); err != nil {
			return nil, err
		}
		items = append(items, target)
	}
	return items, nil
}

func (m *Manager) GetTarget(ctx context.Context, targetID string) (DeliveryTarget, bool, error) {
	record, ok, err := m.sqliteStore.GetDeliveryTarget(ctx, m.environmentScope, targetID)
	if err != nil || !ok {
		return DeliveryTarget{}, ok, err
	}
	var target DeliveryTarget
	if err := unmarshalDocument(record.Document, &target); err != nil {
		return DeliveryTarget{}, false, err
	}
	return target, true, nil
}

func (m *Manager) UpdateTargetStatus(ctx context.Context, targetID string, status TargetStatus) (DeliveryTarget, bool, error) {
	target, ok, err := m.GetTarget(ctx, targetID)
	if err != nil || !ok {
		return DeliveryTarget{}, ok, err
	}
	target.Status = status
	now := time.Now().UTC()
	target.UpdatedAt = now
	if err := m.sqliteStore.UpsertDeliveryTarget(ctx, store.DeliveryTargetRecord{
		TargetID:         target.TargetID,
		EnvironmentScope: target.EnvironmentScope,
		TargetKind:       string(target.TargetKind),
		Status:           string(target.Status),
		UpdatedAt:        target.UpdatedAt,
		Document:         mustMarshal(target),
	}); err != nil {
		return DeliveryTarget{}, false, err
	}
	if err := m.publishEvent(ctx, "delivery.target_status_changed", events.Resource{Kind: "delivery_target", ID: target.TargetID}, map[string]any{
		"targetId":         target.TargetID,
		"targetKind":       target.TargetKind,
		"environmentScope": target.EnvironmentScope,
		"status":           target.Status,
	}); err != nil {
		return DeliveryTarget{}, false, err
	}
	return target, true, nil
}

func (m *Manager) UpsertPreference(ctx context.Context, pref DeliveryPreference) (DeliveryPreference, error) {
	if m == nil || m.sqliteStore == nil {
		return DeliveryPreference{}, errors.New("delivery manager is not configured")
	}
	now := time.Now().UTC()
	if strings.TrimSpace(pref.PreferenceID) == "" {
		return DeliveryPreference{}, errors.New("preferenceId is required")
	}
	if pref.EnvironmentScope == "" {
		pref.EnvironmentScope = m.environmentScope
	}
	if pref.ScopeKind == "" {
		return DeliveryPreference{}, errors.New("scopeKind is required")
	}
	if pref.PreferredTargetsByClass == nil {
		pref.PreferredTargetsByClass = map[ResultClass]string{}
	}
	if pref.CreatedAt.IsZero() {
		pref.CreatedAt = now
	}
	pref.Active = true
	pref.UpdatedAt = now
	if err := m.sqliteStore.UpsertDeliveryPreference(ctx, store.DeliveryPreferenceRecord{
		PreferenceID:     pref.PreferenceID,
		EnvironmentScope: pref.EnvironmentScope,
		ScopeKind:        string(pref.ScopeKind),
		IntegrationID:    pref.IntegrationID,
		Active:           pref.Active,
		UpdatedAt:        pref.UpdatedAt,
		Document:         mustMarshal(pref),
	}); err != nil {
		return DeliveryPreference{}, err
	}
	if err := m.publishEvent(ctx, "delivery.preference_updated", events.Resource{Kind: "delivery_preference", ID: pref.PreferenceID}, map[string]any{
		"preferenceId":     pref.PreferenceID,
		"environmentScope": pref.EnvironmentScope,
		"scopeKind":        pref.ScopeKind,
		"integrationId":    pref.IntegrationID,
	}); err != nil {
		return DeliveryPreference{}, err
	}
	return pref, nil
}

func (m *Manager) ListPreferences(ctx context.Context) ([]DeliveryPreference, error) {
	records, err := m.sqliteStore.ListDeliveryPreferences(ctx, m.environmentScope)
	if err != nil {
		return nil, err
	}
	items := make([]DeliveryPreference, 0, len(records))
	for _, record := range records {
		var pref DeliveryPreference
		if err := unmarshalDocument(record.Document, &pref); err != nil {
			return nil, err
		}
		items = append(items, pref)
	}
	return items, nil
}

func (m *Manager) GetPreference(ctx context.Context, preferenceID string) (DeliveryPreference, bool, error) {
	record, ok, err := m.sqliteStore.GetDeliveryPreference(ctx, m.environmentScope, preferenceID)
	if err != nil || !ok {
		return DeliveryPreference{}, ok, err
	}
	var pref DeliveryPreference
	if err := unmarshalDocument(record.Document, &pref); err != nil {
		return DeliveryPreference{}, false, err
	}
	return pref, true, nil
}

func (m *Manager) EmitOutcome(ctx context.Context, input OutcomeInput) (DeliveryOutcome, error) {
	if m == nil || m.sqliteStore == nil {
		return DeliveryOutcome{}, errors.New("delivery manager is not configured")
	}
	existing, err := m.ListOutcomes(ctx, OutcomeFilter{SourceKind: input.SourceKind, SourceID: input.SourceID})
	if err != nil {
		return DeliveryOutcome{}, err
	}
	if len(existing) > 0 {
		return existing[0], nil
	}
	pref, target, mode, suppressionReason, err := m.resolvePreference(ctx, input.IntegrationID, input.ResultClass)
	if err != nil {
		return DeliveryOutcome{}, err
	}
	now := time.Now().UTC()
	outcome := DeliveryOutcome{
		DeliveryID:        newDeliveryID(),
		EnvironmentScope:  m.environmentScope,
		SourceKind:        input.SourceKind,
		SourceID:          input.SourceID,
		RunID:             input.RunID,
		WorkflowID:        input.WorkflowID,
		ScheduleID:        input.ScheduleID,
		ScheduleAttemptID: input.ScheduleAttemptID,
		IntegrationID:     input.IntegrationID,
		ResultClass:       input.ResultClass,
		Mode:              mode,
		Status:            OutcomeStatusPending,
		ChosenTargetID:    target.TargetID,
		PreferenceID:      pref.PreferenceID,
		PayloadPreview:    input.PayloadPreview,
		SuppressionReason: suppressionReason,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := m.storeOutcome(ctx, outcome); err != nil {
		return DeliveryOutcome{}, err
	}
	if err := m.publishOutcomeCreated(ctx, outcome); err != nil {
		return DeliveryOutcome{}, err
	}
	switch mode {
	case DeliveryModeSuppressed:
		outcome.Status = OutcomeStatusSuppressed
		outcome.FinalizedAt = &now
		outcome.UpdatedAt = now
		if err := m.storeOutcome(ctx, outcome); err != nil {
			return DeliveryOutcome{}, err
		}
		if err := m.publishOutcomeStatusChanged(ctx, outcome); err != nil {
			return DeliveryOutcome{}, err
		}
		return m.attachAttempts(ctx, outcome)
	case DeliveryModeDigest:
		return m.queueDigestOutcome(ctx, outcome, pref, target)
	default:
		return m.dispatchImmediate(ctx, outcome, target)
	}
}

func (m *Manager) ListOutcomes(ctx context.Context, filter OutcomeFilter) ([]DeliveryOutcome, error) {
	records, err := m.sqliteStore.ListDeliveryOutcomes(ctx, m.environmentScope, store.DeliveryOutcomeFilter{
		SourceKind:    filter.SourceKind,
		SourceID:      filter.SourceID,
		RunID:         filter.RunID,
		WorkflowID:    filter.WorkflowID,
		ScheduleID:    filter.ScheduleID,
		IntegrationID: filter.IntegrationID,
		Status:        string(filter.Status),
		TargetID:      filter.TargetID,
	})
	if err != nil {
		return nil, err
	}
	items := make([]DeliveryOutcome, 0, len(records))
	for _, record := range records {
		var outcome DeliveryOutcome
		if err := unmarshalDocument(record.Document, &outcome); err != nil {
			return nil, err
		}
		full, err := m.attachAttempts(ctx, outcome)
		if err != nil {
			return nil, err
		}
		items = append(items, full)
	}
	return items, nil
}

func (m *Manager) GetOutcome(ctx context.Context, deliveryID string) (DeliveryOutcome, bool, error) {
	record, ok, err := m.sqliteStore.GetDeliveryOutcome(ctx, m.environmentScope, deliveryID)
	if err != nil || !ok {
		return DeliveryOutcome{}, ok, err
	}
	var outcome DeliveryOutcome
	if err := unmarshalDocument(record.Document, &outcome); err != nil {
		return DeliveryOutcome{}, false, err
	}
	full, err := m.attachAttempts(ctx, outcome)
	if err != nil {
		return DeliveryOutcome{}, false, err
	}
	return full, true, nil
}

func (m *Manager) ListSummaryWindows(ctx context.Context) ([]SummaryWindow, error) {
	records, err := m.sqliteStore.ListDeliverySummaryWindows(ctx, m.environmentScope)
	if err != nil {
		return nil, err
	}
	items := make([]SummaryWindow, 0, len(records))
	for _, record := range records {
		var window SummaryWindow
		if err := unmarshalDocument(record.Document, &window); err != nil {
			return nil, err
		}
		items = append(items, window)
	}
	return items, nil
}

func (m *Manager) GetSummaryWindow(ctx context.Context, summaryWindowID string) (SummaryWindow, bool, error) {
	record, ok, err := m.sqliteStore.GetDeliverySummaryWindow(ctx, m.environmentScope, summaryWindowID)
	if err != nil || !ok {
		return SummaryWindow{}, ok, err
	}
	var window SummaryWindow
	if err := unmarshalDocument(record.Document, &window); err != nil {
		return SummaryWindow{}, false, err
	}
	return window, true, nil
}

func (m *Manager) resolvePreference(ctx context.Context, integrationID string, resultClass ResultClass) (DeliveryPreference, DeliveryTarget, DeliveryMode, string, error) {
	prefs, err := m.ListPreferences(ctx)
	if err != nil {
		return DeliveryPreference{}, DeliveryTarget{}, "", "", err
	}
	var selected DeliveryPreference
	for _, pref := range prefs {
		if !pref.Active {
			continue
		}
		if integrationID != "" && pref.ScopeKind == PreferenceScopeIntegrationOverride && pref.IntegrationID == integrationID {
			selected = pref
			break
		}
	}
	if selected.PreferenceID == "" {
		for _, pref := range prefs {
			if pref.Active && pref.ScopeKind == PreferenceScopeUserDefault {
				selected = pref
				break
			}
		}
	}
	if selected.PreferenceID == "" {
		return DeliveryPreference{}, DeliveryTarget{}, DeliveryModeSuppressed, "no active delivery preference", nil
	}
	if suppressed := selected.suppressionReason(resultClass); suppressed != "" {
		return selected, DeliveryTarget{}, DeliveryModeSuppressed, suppressed, nil
	}
	targetID := strings.TrimSpace(selected.PreferredTargetsByClass[resultClass])
	if targetID == "" {
		return selected, DeliveryTarget{}, DeliveryModeSuppressed, "no target configured for result class", nil
	}
	target, ok, err := m.GetTarget(ctx, targetID)
	if err != nil {
		return DeliveryPreference{}, DeliveryTarget{}, "", "", err
	}
	if !ok {
		return selected, DeliveryTarget{}, DeliveryModeSuppressed, "configured target is missing", nil
	}
	if target.Status != TargetStatusActive {
		return selected, target, DeliveryModeImmediate, "", nil
	}
	if resultClass == ResultClassRoutineSuccess && selected.SummaryPolicy.RoutineSuccessMode == DeliveryModeDigest {
		return selected, target, DeliveryModeDigest, "", nil
	}
	return selected, target, DeliveryModeImmediate, "", nil
}

func (p DeliveryPreference) suppressionReason(resultClass ResultClass) string {
	switch resultClass {
	case ResultClassRoutineSuccess:
		if p.SuppressionPolicy.SuppressRoutineSuccess {
			return "routine success suppressed by policy"
		}
	case ResultClassUrgent:
		if p.SuppressionPolicy.SuppressUrgent {
			return "urgent result suppressed by policy"
		}
	case ResultClassFailure:
		if p.SuppressionPolicy.SuppressFailure {
			return "failure result suppressed by policy"
		}
	}
	return ""
}

func (m *Manager) adapterFor(kind TargetKind) Adapter {
	for _, adapter := range m.adapters {
		if adapter != nil && adapter.Supports(kind) {
			return adapter
		}
	}
	return nil
}

func (m *Manager) attachAttempts(ctx context.Context, outcome DeliveryOutcome) (DeliveryOutcome, error) {
	records, err := m.sqliteStore.ListDeliveryAttempts(ctx, outcome.DeliveryID)
	if err != nil {
		return DeliveryOutcome{}, err
	}
	outcome.Attempts = make([]DeliveryAttempt, 0, len(records))
	for _, record := range records {
		var attempt DeliveryAttempt
		if err := unmarshalDocument(record.Document, &attempt); err != nil {
			return DeliveryOutcome{}, err
		}
		outcome.Attempts = append(outcome.Attempts, attempt)
	}
	return outcome, nil
}

func (m *Manager) storeOutcome(ctx context.Context, outcome DeliveryOutcome) error {
	return m.sqliteStore.UpsertDeliveryOutcome(ctx, store.DeliveryOutcomeRecord{
		DeliveryID:       outcome.DeliveryID,
		EnvironmentScope: outcome.EnvironmentScope,
		SourceKind:       outcome.SourceKind,
		SourceID:         outcome.SourceID,
		RunID:            outcome.RunID,
		WorkflowID:       outcome.WorkflowID,
		ScheduleID:       outcome.ScheduleID,
		IntegrationID:    outcome.IntegrationID,
		Status:           string(outcome.Status),
		ChosenTargetID:   outcome.ChosenTargetID,
		PreferenceID:     outcome.PreferenceID,
		SummaryWindowID:  outcome.SummaryWindowID,
		UpdatedAt:        outcome.UpdatedAt,
		Document:         mustMarshal(outcome),
	})
}

func (m *Manager) storeAttempt(ctx context.Context, attempt DeliveryAttempt) error {
	return m.sqliteStore.UpsertDeliveryAttempt(ctx, store.DeliveryAttemptRecord{
		AttemptID:     attempt.AttemptID,
		DeliveryID:    attempt.DeliveryID,
		AttemptNumber: attempt.AttemptNumber,
		TargetID:      attempt.TargetID,
		Status:        string(attempt.Status),
		NextRetryAt:   attempt.NextRetryAt,
		Document:      mustMarshal(attempt),
	})
}

func (m *Manager) storeWindow(ctx context.Context, window SummaryWindow) error {
	return m.sqliteStore.UpsertDeliverySummaryWindow(ctx, store.DeliverySummaryWindowRecord{
		SummaryWindowID:  window.SummaryWindowID,
		EnvironmentScope: window.EnvironmentScope,
		TargetID:         window.TargetID,
		PreferenceID:     window.PreferenceID,
		Status:           string(window.Status),
		WindowEndsAt:     window.WindowEndsAt,
		UpdatedAt:        window.UpdatedAt,
		Document:         mustMarshal(window),
	})
}

func (m *Manager) publishOutcomeCreated(ctx context.Context, outcome DeliveryOutcome) error {
	return m.publishEvent(ctx, "delivery.outcome_created", events.Resource{Kind: "delivery", ID: outcome.DeliveryID}, map[string]any{
		"deliveryId":        outcome.DeliveryID,
		"sourceKind":        outcome.SourceKind,
		"sourceId":          outcome.SourceID,
		"runId":             outcome.RunID,
		"workflowId":        outcome.WorkflowID,
		"scheduleId":        outcome.ScheduleID,
		"scheduleAttemptId": outcome.ScheduleAttemptID,
		"integrationId":     outcome.IntegrationID,
		"resultClass":       outcome.ResultClass,
		"mode":              outcome.Mode,
		"status":            outcome.Status,
		"chosenTargetId":    outcome.ChosenTargetID,
		"suppressionReason": outcome.SuppressionReason,
	})
}

func (m *Manager) publishAttemptRecorded(ctx context.Context, outcome DeliveryOutcome, attempt DeliveryAttempt) error {
	return m.publishEvent(ctx, "delivery.attempt_recorded", events.Resource{Kind: "delivery", ID: outcome.DeliveryID}, map[string]any{
		"sourceKind":                 outcome.SourceKind,
		"sourceId":                   outcome.SourceID,
		"runId":                      outcome.RunID,
		"workflowId":                 outcome.WorkflowID,
		"scheduleId":                 outcome.ScheduleID,
		"scheduleAttemptId":          outcome.ScheduleAttemptID,
		"integrationId":              outcome.IntegrationID,
		"deliveryId":                 outcome.DeliveryID,
		"attemptId":                  attempt.AttemptID,
		"attemptNumber":              attempt.AttemptNumber,
		"transportKind":              attempt.TransportKind,
		"status":                     attempt.Status,
		"failureClass":               attempt.FailureClass,
		"nextRetryAt":                attempt.NextRetryAt,
		"connectorMessageDeliveryId": attempt.ConnectorMessageDeliveryID,
	})
}

func (m *Manager) publishOutcomeStatusChanged(ctx context.Context, outcome DeliveryOutcome) error {
	return m.publishEvent(ctx, "delivery.outcome_status_changed", events.Resource{Kind: "delivery", ID: outcome.DeliveryID}, map[string]any{
		"sourceKind":        outcome.SourceKind,
		"sourceId":          outcome.SourceID,
		"runId":             outcome.RunID,
		"workflowId":        outcome.WorkflowID,
		"scheduleId":        outcome.ScheduleID,
		"scheduleAttemptId": outcome.ScheduleAttemptID,
		"integrationId":     outcome.IntegrationID,
		"deliveryId":        outcome.DeliveryID,
		"resultClass":       outcome.ResultClass,
		"mode":              outcome.Mode,
		"status":            outcome.Status,
		"chosenTargetId":    outcome.ChosenTargetID,
		"suppressionReason": outcome.SuppressionReason,
	})
}

func (m *Manager) publishEvent(ctx context.Context, name string, resource events.Resource, payload map[string]any) error {
	if m.eventBus == nil {
		return nil
	}
	event := events.Event{
		EnvironmentScope: m.environmentScope,
		Category:         "delivery",
		Name:             name,
		OccurredAt:       time.Now().UTC(),
		Resource:         resource,
		Payload:          payload,
	}
	if runID, _ := payload["runId"].(string); runID != "" {
		event.Scope.RunID = runID
	}
	if workflowID, _ := payload["workflowId"].(string); workflowID != "" {
		event.Scope.WorkflowID = workflowID
	}
	if scheduleID, _ := payload["scheduleId"].(string); scheduleID != "" {
		event.Scope.ScheduleID = scheduleID
	}
	if attemptID, _ := payload["scheduleAttemptId"].(string); attemptID != "" {
		event.Scope.ScheduleAttemptID = attemptID
	}
	published := m.eventBus.Publish(event)
	if m.sqliteStore != nil {
		if _, err := m.sqliteStore.AppendEvent(ctx, published); err != nil {
			return fmt.Errorf("append delivery event %s: %w", name, err)
		}
	}
	return nil
}

func mustMarshal(value any) []byte {
	data, err := marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func newDeliveryID() string {
	return "delivery_" + randomSuffix()
}

func newAttemptID() string {
	return "delivery_attempt_" + randomSuffix()
}

func newSummaryWindowID() string {
	return "summary_window_" + randomSuffix()
}

func randomSuffix() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(buf)
}

func nonEmpty(items ...string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return item
		}
	}
	return ""
}
