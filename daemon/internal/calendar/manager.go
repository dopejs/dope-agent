package calendar

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

type Manager struct {
	mu         sync.RWMutex
	env        string
	backends   map[integrations.BackendKind]Backend
	accounts   map[string]AccountProjection
	operations map[string]Operation
	opOrder    []string
	artifacts  map[string]Artifact
}

func NewManager(environmentScope string) *Manager {
	return &Manager{
		env:        strings.TrimSpace(environmentScope),
		backends:   map[integrations.BackendKind]Backend{integrations.BackendKindFakeLocal: NewFakeBackend()},
		accounts:   make(map[string]AccountProjection),
		operations: make(map[string]Operation),
		artifacts:  make(map[string]Artifact),
	}
}

func (m *Manager) Restore(accounts []AccountProjection, operations []Operation, artifacts []Artifact) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts = make(map[string]AccountProjection, len(accounts))
	for _, item := range accounts {
		m.accounts[item.IntegrationID] = item
	}
	m.operations = make(map[string]Operation, len(operations))
	m.opOrder = m.opOrder[:0]
	for _, item := range operations {
		m.operations[item.OperationID] = item
		m.opOrder = append(m.opOrder, item.OperationID)
	}
	m.artifacts = make(map[string]Artifact, len(artifacts))
	eventsByIntegration := make(map[string][]Event)
	for _, item := range artifacts {
		m.artifacts[item.ArtifactID] = item
		if item.Kind != ArtifactKindEventSnapshot || item.ExternalEventID == "" || item.StartsAt == nil || item.EndsAt == nil {
			continue
		}
		eventsByIntegration[item.IntegrationID] = append(eventsByIntegration[item.IntegrationID], Event{
			ExternalEventID:         item.ExternalEventID,
			IntegrationID:           item.IntegrationID,
			CalendarRef:             item.CalendarRef,
			Title:                   item.Title,
			StartsAt:                item.StartsAt.UTC(),
			EndsAt:                  item.EndsAt.UTC(),
			Timezone:                item.Timezone,
			AllDay:                  item.AllDay,
			RecurrenceSummary:       item.RecurrenceSummary,
			MutationEligibleInPhase: item.MutationEligibleInPhase,
			LifecycleState:          item.LifecycleState,
			CreatedAt:               item.CreatedAt,
			UpdatedAt:               item.CreatedAt,
		})
	}
	for integrationID, items := range eventsByIntegration {
		if backend, ok := m.backends[integrations.BackendKindFakeLocal]; ok {
			backend.RestoreIntegrationState(integrationID, items)
		}
	}
}

func (m *Manager) ListAccounts(resources []integrations.Resource, selection Selection) ([]AccountProjection, error) {
	if trimmed := strings.TrimSpace(selection.IntegrationID); trimmed != "" {
		account, _, _, _, err := m.selectAccount(resources, selection)
		if err != nil {
			return nil, err
		}
		return []AccountProjection{account}, nil
	}
	items := make([]AccountProjection, 0)
	for _, resource := range resources {
		if resource.DomainKind != "calendar" || strings.TrimSpace(resource.EnvironmentScope) != m.env {
			continue
		}
		account, _, _, _, err := m.selectAccount(resources, Selection{IntegrationID: resource.IntegrationID})
		if err != nil {
			return nil, err
		}
		items = append(items, account)
	}
	return items, nil
}

func (m *Manager) ListEvents(resources []integrations.Resource, input ListEventsInput) (AccountProjection, []Event, Operation, []Artifact, error) {
	account, resource, backend, selectionMode, err := m.selectAccount(resources, input.Selection)
	if err != nil {
		return AccountProjection{}, nil, Operation{}, nil, err
	}
	operation := m.newOperation(account, resource, OperationClassListEvents, selectionMode, account.PrimaryTimezone, summarizeWindow(input.StartsAt, input.EndsAt), input.Source)
	items, err := backend.ListEvents(resource, account, input)
	if err != nil {
		return account, nil, m.failOperation(operation, "backend_error", err.Error()), nil, err
	}
	artifacts := make([]Artifact, 0, len(items))
	for _, item := range items {
		artifacts = append(artifacts, EventArtifact(operation, item))
	}
	operation = m.completeOperation(operation, artifacts, nil, "")
	return account, items, operation, artifacts, nil
}

func (m *Manager) GetEvent(resources []integrations.Resource, input GetEventInput) (AccountProjection, Event, Operation, []Artifact, error) {
	account, resource, backend, selectionMode, err := m.selectAccount(resources, input.Selection)
	if err != nil {
		return AccountProjection{}, Event{}, Operation{}, nil, err
	}
	operation := m.newOperation(account, resource, OperationClassGetEvent, selectionMode, account.PrimaryTimezone, input.ExternalEventID, input.Source)
	item, err := backend.GetEvent(resource, account, input.ExternalEventID)
	if err != nil {
		return account, Event{}, m.failOperation(operation, "not_found", err.Error()), nil, err
	}
	artifact := EventArtifact(operation, item)
	operation = m.completeOperation(operation, []Artifact{artifact}, nil, item.ExternalEventID)
	return account, item, operation, []Artifact{artifact}, nil
}

func (m *Manager) BusyFree(resources []integrations.Resource, input BusyFreeInput) (AccountProjection, AvailabilityQuery, Operation, []Artifact, error) {
	account, resource, backend, selectionMode, err := m.selectAccount(resources, input.Selection)
	if err != nil {
		return AccountProjection{}, AvailabilityQuery{}, Operation{}, nil, err
	}
	timezone := normalizeTimezone(input.Timezone, account.PrimaryTimezone)
	operation := m.newOperation(account, resource, OperationClassBusyFree, selectionMode, timezone, summarizeWindow(&input.WindowStart, &input.WindowEnd), input.Source)
	query, err := backend.BusyFree(resource, account, input)
	if err != nil {
		return account, AvailabilityQuery{}, m.failOperation(operation, "backend_error", err.Error()), nil, err
	}
	query.QueryID = operation.OperationID
	query.OperationID = operation.OperationID
	query.CreatedAt = operation.UpdatedAt
	artifact := AvailabilityArtifact(operation, query)
	operation = m.completeOperation(operation, []Artifact{artifact}, &query, "")
	return account, query, operation, []Artifact{artifact}, nil
}

func (m *Manager) CreateEvent(resources []integrations.Resource, input CreateEventInput) (AccountProjection, Event, Operation, []Artifact, error) {
	if err := validateMutationInput(input.AllDay, input.Recurring, input.Attendees); err != nil {
		return AccountProjection{}, Event{}, Operation{}, nil, err
	}
	if !input.EndsAt.After(input.StartsAt) {
		return AccountProjection{}, Event{}, Operation{}, nil, ErrCalendarInvalidTimeRange
	}
	account, resource, backend, selectionMode, err := m.selectAccount(resources, input.Selection)
	if err != nil {
		return AccountProjection{}, Event{}, Operation{}, nil, err
	}
	operation := m.newOperation(account, resource, OperationClassCreateEvent, selectionMode, normalizeTimezone(input.Timezone, account.PrimaryTimezone), input.Title, input.Source)
	item, err := backend.CreateEvent(resource, account, input)
	if err != nil {
		return account, Event{}, m.failOperation(operation, "backend_error", err.Error()), nil, err
	}
	artifact := EventArtifact(operation, item)
	operation = m.completeOperation(operation, []Artifact{artifact}, nil, item.ExternalEventID)
	return account, item, operation, []Artifact{artifact}, nil
}

func (m *Manager) UpdateEvent(resources []integrations.Resource, input UpdateEventInput) (AccountProjection, Event, Operation, []Artifact, error) {
	if err := validateMutationInput(input.AllDay, input.Recurring, input.Attendees); err != nil {
		return AccountProjection{}, Event{}, Operation{}, nil, err
	}
	if !input.EndsAt.After(input.StartsAt) {
		return AccountProjection{}, Event{}, Operation{}, nil, ErrCalendarInvalidTimeRange
	}
	account, resource, backend, selectionMode, err := m.selectAccount(resources, input.Selection)
	if err != nil {
		return AccountProjection{}, Event{}, Operation{}, nil, err
	}
	operation := m.newOperation(account, resource, OperationClassUpdateEvent, selectionMode, normalizeTimezone(input.Timezone, account.PrimaryTimezone), input.ExternalEventID, input.Source)
	item, err := backend.UpdateEvent(resource, account, input)
	if err != nil {
		return account, Event{}, m.failOperation(operation, "not_found", err.Error()), nil, err
	}
	artifact := EventArtifact(operation, item)
	operation = m.completeOperation(operation, []Artifact{artifact}, nil, item.ExternalEventID)
	return account, item, operation, []Artifact{artifact}, nil
}

func (m *Manager) CancelEvent(resources []integrations.Resource, input CancelEventInput) (AccountProjection, Event, Operation, []Artifact, error) {
	account, resource, backend, selectionMode, err := m.selectAccount(resources, input.Selection)
	if err != nil {
		return AccountProjection{}, Event{}, Operation{}, nil, err
	}
	operation := m.newOperation(account, resource, OperationClassCancelEvent, selectionMode, account.PrimaryTimezone, input.ExternalEventID, input.Source)
	item, err := backend.CancelEvent(resource, account, input)
	if err != nil {
		return account, Event{}, m.failOperation(operation, "not_found", err.Error()), nil, err
	}
	artifact := EventArtifact(operation, item)
	operation = m.completeOperation(operation, []Artifact{artifact}, nil, item.ExternalEventID)
	return account, item, operation, []Artifact{artifact}, nil
}

func (m *Manager) ListOperations(filter OperationFilter) []Operation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]Operation, 0)
	for _, id := range m.opOrder {
		item := m.operations[id]
		if filter.IntegrationID != "" && item.IntegrationID != filter.IntegrationID {
			continue
		}
		if filter.RunID != "" && item.RunID != filter.RunID {
			continue
		}
		if filter.WorkflowID != "" && item.WorkflowID != filter.WorkflowID {
			continue
		}
		if filter.ScheduleID != "" && item.ScheduleID != filter.ScheduleID {
			continue
		}
		if filter.DeliveryID != "" && item.DeliveryID != filter.DeliveryID {
			continue
		}
		if filter.OperationClass != "" && item.OperationClass != filter.OperationClass {
			continue
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		if filter.ExternalEventID != "" && item.ExternalEventID != filter.ExternalEventID {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (m *Manager) GetOperation(operationID string) (Operation, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.operations[strings.TrimSpace(operationID)]
	return item, ok
}

func (m *Manager) GetAccount(integrationID string) (AccountProjection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.accounts[strings.TrimSpace(integrationID)]
	return item, ok
}

func (m *Manager) StoreOperation(item Operation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.operations[item.OperationID]; !exists {
		m.opOrder = append(m.opOrder, item.OperationID)
	}
	m.operations[item.OperationID] = item
}

func (m *Manager) ListArtifacts(operationID string) []Artifact {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]Artifact, 0)
	for _, item := range m.artifacts {
		if operationID != "" && item.OperationID != operationID {
			continue
		}
		items = append(items, item)
	}
	return items
}

func validateMutationInput(allDay, recurring bool, attendees []string) error {
	switch {
	case recurring:
		return ErrCalendarRecurringUnsupported
	case allDay:
		return ErrCalendarAllDayUnsupported
	case len(attendees) > 0:
		return ErrCalendarAttendeesUnsupported
	default:
		return nil
	}
}

func (m *Manager) selectAccount(resources []integrations.Resource, selection Selection) (AccountProjection, integrations.Resource, Backend, string, error) {
	explicit := strings.TrimSpace(selection.IntegrationID)
	if explicit != "" {
		for _, resource := range resources {
			if resource.IntegrationID != explicit {
				continue
			}
			return m.projectResource(resource, "explicit")
		}
		return AccountProjection{}, integrations.Resource{}, nil, "", ErrCalendarIntegrationNotFound
	}
	var selected *integrations.Resource
	for idx := range resources {
		resource := resources[idx]
		if resource.DomainKind != "calendar" || strings.TrimSpace(resource.EnvironmentScope) != m.env {
			continue
		}
		if resource.CanonicalDefault {
			selected = &resource
			break
		}
	}
	if selected == nil {
		return AccountProjection{}, integrations.Resource{}, nil, "", ErrCalendarSelectionInvalid
	}
	return m.projectResource(*selected, "canonical_default")
}

func (m *Manager) projectResource(resource integrations.Resource, selectionMode string) (AccountProjection, integrations.Resource, Backend, string, error) {
	if resource.DomainKind != "calendar" || strings.TrimSpace(resource.EnvironmentScope) != m.env {
		return AccountProjection{}, integrations.Resource{}, nil, "", ErrCalendarSelectionInvalid
	}
	if resource.ReadinessStatus != integrations.ReadinessStatusHealthy && resource.ReadinessStatus != integrations.ReadinessStatusDegraded {
		return AccountProjection{}, integrations.Resource{}, nil, "", ErrCalendarUnavailable
	}
	backend := m.backends[resource.BackendBinding.BackendKind]
	if backend == nil {
		return AccountProjection{}, integrations.Resource{}, nil, "", errors.New("calendar backend is not configured")
	}
	account, err := backend.ProjectAccount(resource)
	if err != nil {
		return AccountProjection{}, integrations.Resource{}, nil, "", err
	}
	account.SelectionMode = selectionMode
	m.mu.Lock()
	m.accounts[account.IntegrationID] = account
	m.mu.Unlock()
	return account, resource, backend, selectionMode, nil
}

func (m *Manager) newOperation(account AccountProjection, resource integrations.Resource, class OperationClass, selectionMode, timezone, requestSummary string, source SourceLinkage) Operation {
	now := time.Now().UTC()
	item := Operation{
		OperationID:       newID("calendar_op"),
		OperationClass:    class,
		Status:            OperationStatusRequested,
		IntegrationID:     resource.IntegrationID,
		CalendarAccountID: account.CalendarAccountID,
		EnvironmentScope:  resource.EnvironmentScope,
		CalendarRef:       account.PrimaryCalendarRef,
		SelectionMode:     selectionMode,
		TimezoneUsed:      timezone,
		RequestSummary:    requestSummary,
		RunID:             strings.TrimSpace(source.RunID),
		StepID:            strings.TrimSpace(source.StepID),
		ToolCallID:        strings.TrimSpace(source.ToolCallID),
		WorkflowID:        strings.TrimSpace(source.WorkflowID),
		WorkflowStepID:    strings.TrimSpace(source.WorkflowStepID),
		ScheduleID:        strings.TrimSpace(source.ScheduleID),
		ScheduleAttemptID: strings.TrimSpace(source.ScheduleAttemptID),
		DeliveryID:        strings.TrimSpace(source.DeliveryID),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	m.mu.Lock()
	m.operations[item.OperationID] = item
	m.opOrder = append(m.opOrder, item.OperationID)
	m.mu.Unlock()
	return item
}

func (m *Manager) completeOperation(operation Operation, artifacts []Artifact, query *AvailabilityQuery, externalEventID string) Operation {
	now := time.Now().UTC()
	operation.Status = OperationStatusCompleted
	operation.ExternalEventID = externalEventID
	operation.CompletedAt = &now
	operation.UpdatedAt = now
	operation.ArtifactIDs = make([]string, 0, len(artifacts))
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range artifacts {
		operation.ArtifactIDs = append(operation.ArtifactIDs, item.ArtifactID)
		m.artifacts[item.ArtifactID] = item
	}
	if query != nil {
		operation.AvailabilityQueryID = query.QueryID
	}
	m.operations[operation.OperationID] = operation
	return operation
}

func (m *Manager) failOperation(operation Operation, class, reason string) Operation {
	now := time.Now().UTC()
	operation.Status = OperationStatusFailed
	operation.FailureClass = class
	operation.FailureReason = reason
	operation.CompletedAt = &now
	operation.UpdatedAt = now
	m.mu.Lock()
	m.operations[operation.OperationID] = operation
	m.mu.Unlock()
	return operation
}

func summarizeWindow(startsAt, endsAt *time.Time) string {
	if startsAt == nil && endsAt == nil {
		return ""
	}
	switch {
	case startsAt != nil && endsAt != nil:
		return fmt.Sprintf("%s/%s", startsAt.UTC().Format(time.RFC3339), endsAt.UTC().Format(time.RFC3339))
	case startsAt != nil:
		return startsAt.UTC().Format(time.RFC3339)
	default:
		return endsAt.UTC().Format(time.RFC3339)
	}
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_fallback"
	}
	return prefix + "_" + hex.EncodeToString(buf)
}
