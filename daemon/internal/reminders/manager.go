package reminders

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

type Dependencies struct {
	EnvironmentScope string
	Store            *store.SQLiteStore
	EventBus         *events.Bus
	Delivery         *delivery.Manager
	WorkflowLauncher WorkflowLauncher
	Clock            Clock
	TickInterval     time.Duration
}

type Manager struct {
	env          string
	store        *store.SQLiteStore
	eventBus     *events.Bus
	delivery     *delivery.Manager
	workflow     WorkflowLauncher
	clock        Clock
	tickInterval time.Duration
	overdueAfter time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewManager(deps Dependencies) *Manager {
	clock := deps.Clock
	if clock == nil {
		clock = realClock{}
	}
	interval := deps.TickInterval
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	return &Manager{
		env:          strings.TrimSpace(deps.EnvironmentScope),
		store:        deps.Store,
		eventBus:     deps.EventBus,
		delivery:     deps.Delivery,
		workflow:     deps.WorkflowLauncher,
		clock:        clock,
		tickInterval: interval,
		overdueAfter: interval,
	}
}

func (m *Manager) Start(ctx context.Context) error {
	if m == nil || m.store == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		_ = m.CatchUp(runCtx)
		ticker := time.NewTicker(m.tickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				_ = m.Tick(runCtx)
			}
		}
	}()
	return nil
}

func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	m.wg.Wait()
	return nil
}

func (m *Manager) CatchUp(ctx context.Context) error {
	return m.Tick(ctx)
}

func (m *Manager) Tick(ctx context.Context) error {
	if m == nil || m.store == nil {
		return nil
	}
	items, err := m.listReminderDocs(ctx)
	if err != nil {
		return err
	}
	now := m.clock.Now().UTC()
	for _, item := range items {
		if err := m.processReminder(ctx, item, now); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) List(ctx context.Context) ([]Reminder, error) {
	if m == nil || m.store == nil {
		return nil, nil
	}
	items, err := m.listReminderDocs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Reminder, 0, len(items))
	for _, item := range items {
		next, err := m.refreshReminderProjection(ctx, item)
		if err != nil {
			return nil, err
		}
		out = append(out, next)
	}
	return out, nil
}

func (m *Manager) Get(ctx context.Context, reminderID string) (Reminder, bool, error) {
	if m == nil || m.store == nil {
		return Reminder{}, false, nil
	}
	item, ok, err := m.getReminderDoc(ctx, reminderID)
	if err != nil || !ok {
		return Reminder{}, ok, err
	}
	item, err = m.refreshReminderProjection(ctx, item)
	if err != nil {
		return Reminder{}, false, err
	}
	return item, true, nil
}

func (m *Manager) ListOccurrences(ctx context.Context, filter OccurrenceFilter) ([]Occurrence, error) {
	if m == nil || m.store == nil {
		return nil, nil
	}
	return m.listOccurrenceDocs(ctx, filter)
}

func (m *Manager) GetOccurrence(ctx context.Context, occurrenceID string) (Occurrence, bool, error) {
	if m == nil || m.store == nil {
		return Occurrence{}, false, nil
	}
	return m.getOccurrenceDoc(ctx, occurrenceID)
}

func (m *Manager) ListActions(ctx context.Context, reminderID string) ([]ActionRecord, error) {
	if m == nil || m.store == nil {
		return nil, nil
	}
	return m.listActionDocs(ctx, reminderID)
}

func (m *Manager) Create(ctx context.Context, input CreateInput) (Reminder, error) {
	if m == nil || m.store == nil {
		return Reminder{}, fmt.Errorf("reminder store is not configured")
	}
	if strings.TrimSpace(input.Title) == "" {
		return Reminder{}, fmt.Errorf("title is required")
	}
	if input.BehaviorMode == BehaviorModeLaunchWorkflow && input.WorkflowLaunchConfig == nil {
		return Reminder{}, ErrReminderWorkflowConfig
	}
	if input.Trigger.Kind == "" {
		return Reminder{}, ErrReminderInvalidTrigger
	}
	now := m.clock.Now().UTC()
	nextDueAt, err := scheduler.NextDueAfter(input.Trigger, now.Add(-time.Second))
	if err != nil {
		return Reminder{}, err
	}
	reminder := Reminder{
		ReminderID:           newReminderID(),
		EnvironmentScope:     m.env,
		Title:                strings.TrimSpace(input.Title),
		Details:              strings.TrimSpace(input.Details),
		BehaviorMode:         input.BehaviorMode,
		Trigger:              input.Trigger,
		CurrentState:         StatePending,
		NextDueAt:            nextDueAt,
		WorkflowLaunchConfig: cloneWorkflowLaunchConfig(input.WorkflowLaunchConfig),
		FollowUpLink:         cloneFollowUpLink(input.FollowUpLink),
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := m.putReminder(ctx, reminder); err != nil {
		return Reminder{}, err
	}
	if _, err := m.appendAction(ctx, reminder, "", ActionKindCreated, ActorKindUser, "", StatePending, "", "", "", "", now); err != nil {
		return Reminder{}, err
	}
	if err := m.publishReminderEvent(ctx, "reminder.created", reminder, nil, nil); err != nil {
		return Reminder{}, err
	}
	return m.refreshReminderProjection(ctx, reminder)
}

func (m *Manager) Acknowledge(ctx context.Context, reminderID string, input TransitionInput) (Reminder, Occurrence, ActionRecord, error) {
	return m.transitionOccurrence(ctx, reminderID, input, ActionKindAcknowledged, StateAcknowledged)
}

func (m *Manager) Complete(ctx context.Context, reminderID string, input TransitionInput) (Reminder, Occurrence, ActionRecord, error) {
	return m.transitionOccurrence(ctx, reminderID, input, ActionKindCompleted, StateCompleted)
}

func (m *Manager) Dismiss(ctx context.Context, reminderID string, input TransitionInput) (Reminder, Occurrence, ActionRecord, error) {
	return m.transitionOccurrence(ctx, reminderID, input, ActionKindDismissed, StateDismissed)
}

func (m *Manager) Cancel(ctx context.Context, reminderID string, input TransitionInput) (Reminder, Occurrence, ActionRecord, error) {
	reminder, ok, err := m.Get(ctx, reminderID)
	if err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	if !ok {
		return Reminder{}, Occurrence{}, ActionRecord{}, ErrReminderNotFound
	}
	now := m.clock.Now().UTC()
	reminder.CurrentState = StateCancelled
	reminder.NextDueAt = nil
	reminder.ActiveOccurrenceID = ""
	reminder.CancelledAt = &now
	reminder.UpdatedAt = now
	if err := m.putReminder(ctx, reminder); err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	var occurrence Occurrence
	if strings.TrimSpace(input.OccurrenceID) != "" {
		if occurrence, ok, err = m.getOccurrenceDoc(ctx, input.OccurrenceID); err != nil {
			return Reminder{}, Occurrence{}, ActionRecord{}, err
		}
		if ok {
			occurrence.State = StateCancelled
			occurrence.CancelledAt = &now
			occurrence.UpdatedAt = now
			if err := m.putOccurrence(ctx, occurrence); err != nil {
				return Reminder{}, Occurrence{}, ActionRecord{}, err
			}
		}
	}
	action, err := m.appendAction(ctx, reminder, occurrence.OccurrenceID, ActionKindCancelled, nonEmptyActor(input.ActorKind, ActorKindUser), occurrence.State, StateCancelled, input.Reason, occurrence.RunID, occurrence.WorkflowID, "", now)
	if err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	if err := m.publishReminderEvent(ctx, "reminder.updated", reminder, &occurrence, &action); err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	return reminder, occurrence, action, nil
}

func (m *Manager) Snooze(ctx context.Context, reminderID string, input TransitionInput) (Reminder, Occurrence, ActionRecord, error) {
	if input.SnoozedUntil == nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, ErrReminderSnoozeRequired
	}
	reminder, occurrence, err := m.getActionableOccurrence(ctx, reminderID, input.OccurrenceID)
	if err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	now := m.clock.Now().UTC()
	occurrence.State = StateSnoozed
	occurrence.SnoozedUntil = input.SnoozedUntil
	occurrence.UpdatedAt = now
	if err := m.putOccurrence(ctx, occurrence); err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	reminder.CurrentState = StateSnoozed
	reminder.NextDueAt = ptrTime(input.SnoozedUntil.UTC())
	reminder.ActiveOccurrenceID = occurrence.OccurrenceID
	reminder.UpdatedAt = now
	if reminder.Trigger.Kind == scheduler.TriggerKindOnce {
		reminder.Trigger.FireAt = ptrTime(input.SnoozedUntil.UTC())
	}
	if err := m.putReminder(ctx, reminder); err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	action, err := m.appendAction(ctx, reminder, occurrence.OccurrenceID, ActionKindSnoozed, nonEmptyActor(input.ActorKind, ActorKindUser), StateDue, StateSnoozed, input.Reason, occurrence.RunID, occurrence.WorkflowID, "", now)
	if err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	if err := m.publishReminderEvent(ctx, "reminder.occurrence_transitioned", reminder, &occurrence, &action); err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	return reminder, occurrence, action, nil
}

func (m *Manager) Reschedule(ctx context.Context, reminderID string, input TransitionInput) (Reminder, Occurrence, ActionRecord, error) {
	reminder, ok, err := m.Get(ctx, reminderID)
	if err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	if !ok {
		return Reminder{}, Occurrence{}, ActionRecord{}, ErrReminderNotFound
	}
	if input.Trigger == nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, ErrReminderInvalidTrigger
	}
	now := m.clock.Now().UTC()
	reminder.Trigger = *input.Trigger
	nextDueAt, err := scheduler.NextDueAfter(reminder.Trigger, now.Add(-time.Second))
	if err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	reminder.NextDueAt = nextDueAt
	reminder.CurrentState = StatePending
	reminder.ActiveOccurrenceID = ""
	reminder.UpdatedAt = now
	if err := m.putReminder(ctx, reminder); err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	var occurrence Occurrence
	if strings.TrimSpace(input.OccurrenceID) != "" {
		occurrence, _, _ = m.getOccurrenceDoc(ctx, input.OccurrenceID)
	}
	action, err := m.appendAction(ctx, reminder, occurrence.OccurrenceID, ActionKindRescheduled, nonEmptyActor(input.ActorKind, ActorKindUser), occurrence.State, StatePending, input.Reason, occurrence.RunID, occurrence.WorkflowID, "", now)
	if err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	if err := m.publishReminderEvent(ctx, "reminder.updated", reminder, &occurrence, &action); err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	return reminder, occurrence, action, nil
}

func (m *Manager) transitionOccurrence(ctx context.Context, reminderID string, input TransitionInput, actionKind ActionKind, target State) (Reminder, Occurrence, ActionRecord, error) {
	reminder, occurrence, err := m.getActionableOccurrence(ctx, reminderID, input.OccurrenceID)
	if err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	now := m.clock.Now().UTC()
	prev := occurrence.State
	occurrence.State = target
	occurrence.UpdatedAt = now
	switch target {
	case StateAcknowledged:
		occurrence.AcknowledgedAt = &now
	case StateCompleted:
		occurrence.CompletedAt = &now
	case StateDismissed:
		occurrence.DismissedAt = &now
	}
	if err := m.putOccurrence(ctx, occurrence); err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	reminder.CurrentState = target
	reminder.ActiveOccurrenceID = occurrence.OccurrenceID
	reminder.UpdatedAt = now
	if err := m.putReminder(ctx, reminder); err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	action, err := m.appendAction(ctx, reminder, occurrence.OccurrenceID, actionKind, nonEmptyActor(input.ActorKind, ActorKindUser), prev, target, input.Reason, occurrence.RunID, occurrence.WorkflowID, "", now)
	if err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	if err := m.publishReminderEvent(ctx, "reminder.occurrence_transitioned", reminder, &occurrence, &action); err != nil {
		return Reminder{}, Occurrence{}, ActionRecord{}, err
	}
	return reminder, occurrence, action, nil
}

func (m *Manager) processReminder(ctx context.Context, reminder Reminder, now time.Time) error {
	reminder, err := m.refreshReminderProjection(ctx, reminder)
	if err != nil {
		return err
	}
	if reminder.CurrentState == StateCancelled {
		return nil
	}
	if reminder.ActiveOccurrenceID != "" {
		occurrence, ok, err := m.getOccurrenceDoc(ctx, reminder.ActiveOccurrenceID)
		if err != nil {
			return err
		}
		if ok {
			switch occurrence.State {
			case StateSnoozed:
				if occurrence.SnoozedUntil != nil && !occurrence.SnoozedUntil.After(now) {
					return m.makeOccurrenceDue(ctx, reminder, occurrence, now)
				}
			case StateDue:
				if now.Sub(occurrence.ScheduledFor) >= m.overdueAfter {
					return m.markOccurrenceOverdue(ctx, reminder, occurrence, now)
				}
			case StateOverdue:
			}
		}
	}
	if reminder.NextDueAt != nil && !reminder.NextDueAt.After(now) {
		return m.createDueOccurrence(ctx, reminder, now)
	}
	return nil
}

func (m *Manager) createDueOccurrence(ctx context.Context, reminder Reminder, now time.Time) error {
	previous, ok, err := m.currentOccurrence(ctx, reminder)
	if err != nil {
		return err
	}
	if ok && IsUnresolvedState(previous.State) {
		if err := m.markOccurrenceMissed(ctx, reminder, previous, now); err != nil {
			return err
		}
	}
	scheduledFor := now
	if reminder.NextDueAt != nil {
		scheduledFor = reminder.NextDueAt.UTC()
	}
	occurrence := Occurrence{
		OccurrenceID:     newOccurrenceID(),
		ReminderID:       reminder.ReminderID,
		EnvironmentScope: reminder.EnvironmentScope,
		State:            StateDue,
		ScheduledFor:     scheduledFor,
		BecameDueAt:      ptrTime(now),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := m.putOccurrence(ctx, occurrence); err != nil {
		return err
	}
	reminder.ActiveOccurrenceID = occurrence.OccurrenceID
	reminder.CurrentState = StateDue
	reminder.UpdatedAt = now
	nextDueAt, err := nextReminderDueAfter(reminder.Trigger, scheduledFor)
	if err != nil {
		return err
	}
	reminder.NextDueAt = nextDueAt
	reminder.Trigger.NextDueAt = nextDueAt
	if err := m.putReminder(ctx, reminder); err != nil {
		return err
	}
	action, err := m.appendAction(ctx, reminder, occurrence.OccurrenceID, ActionKindDue, ActorKindSystem, "", StateDue, "", "", "", "", now)
	if err != nil {
		return err
	}
	if err := m.publishReminderEvent(ctx, "reminder.occurrence_created", reminder, &occurrence, &action); err != nil {
		return err
	}
	return m.handleDueOccurrence(ctx, reminder, occurrence, now)
}

func (m *Manager) handleDueOccurrence(ctx context.Context, reminder Reminder, occurrence Occurrence, now time.Time) error {
	ctx = m.withReminderTenantContext(ctx, reminder)
	switch reminder.BehaviorMode {
	case BehaviorModeNotifyOnly:
		return m.emitReminderDelivery(ctx, reminder, occurrence, now)
	case BehaviorModeLaunchWorkflow:
		if reminder.WorkflowLaunchConfig == nil || m.workflow == nil {
			return m.recordWorkflowLaunchFailure(ctx, reminder, occurrence, now, "workflow launcher is not configured")
		}
		result, err := m.workflow.LaunchReminderWorkflow(ctx, *reminder.WorkflowLaunchConfig, reminder.ReminderID, occurrence.OccurrenceID)
		if err != nil {
			return m.recordWorkflowLaunchFailure(ctx, reminder, occurrence, now, err.Error())
		}
		occurrence.RunID = result.RunID
		occurrence.WorkflowID = result.WorkflowID
		occurrence.State = StateAcknowledged
		occurrence.AcknowledgedAt = ptrTime(now)
		occurrence.UpdatedAt = now
		if err := m.putOccurrence(ctx, occurrence); err != nil {
			return err
		}
		reminder.CurrentState = StateAcknowledged
		reminder.UpdatedAt = now
		if err := m.putReminder(ctx, reminder); err != nil {
			return err
		}
		action, err := m.appendAction(ctx, reminder, occurrence.OccurrenceID, ActionKindWorkflowStarted, ActorKindSystem, StateDue, StateAcknowledged, "", result.RunID, result.WorkflowID, "", now)
		if err != nil {
			return err
		}
		return m.publishReminderEvent(ctx, "reminder.workflow_launch_started", reminder, &occurrence, &action)
	default:
		return ErrReminderUnsupportedBehavior
	}
}

func (m *Manager) withReminderTenantContext(ctx context.Context, reminder Reminder) context.Context {
	if _, ok := tenantctx.FromContext(ctx); ok {
		return ctx
	}
	tenantID := strings.TrimSpace(reminder.TenantID)
	if tenantID == "" {
		return ctx
	}
	return tenantctx.WithContext(ctx, identity.TenantContext{
		TenantID:    tenantID,
		PrincipalID: "system:reminder",
	})
}

func (m *Manager) emitReminderDelivery(ctx context.Context, reminder Reminder, occurrence Occurrence, now time.Time) error {
	if m.delivery == nil {
		return nil
	}
	outcome, err := m.delivery.EmitOutcome(ctx, delivery.OutcomeInput{
		SourceKind:     "reminder_occurrence",
		SourceID:       occurrence.OccurrenceID,
		IntegrationID:  "",
		ResultClass:    delivery.ResultClassRoutineSuccess,
		PayloadPreview: reminder.Title,
	})
	if err != nil {
		return err
	}
	occurrence.LatestDeliveryID = outcome.DeliveryID
	occurrence.LatestDeliveryStatus = string(outcome.Status)
	occurrence.LatestDeliveryTargetID = outcome.ChosenTargetID
	occurrence.UpdatedAt = now
	if err := m.putOccurrence(ctx, occurrence); err != nil {
		return err
	}
	action, err := m.appendAction(ctx, reminder, occurrence.OccurrenceID, ActionKindDeliveryLinked, ActorKindSystem, occurrence.State, occurrence.State, "", occurrence.RunID, occurrence.WorkflowID, outcome.DeliveryID, now)
	if err != nil {
		return err
	}
	return m.publishReminderEvent(ctx, "reminder.delivery_linked", reminder, &occurrence, &action)
}

func (m *Manager) markOccurrenceOverdue(ctx context.Context, reminder Reminder, occurrence Occurrence, now time.Time) error {
	if occurrence.State != StateDue {
		return nil
	}
	occurrence.State = StateOverdue
	occurrence.OverdueAt = ptrTime(now)
	occurrence.UpdatedAt = now
	if err := m.putOccurrence(ctx, occurrence); err != nil {
		return err
	}
	reminder.CurrentState = StateOverdue
	reminder.UpdatedAt = now
	if err := m.putReminder(ctx, reminder); err != nil {
		return err
	}
	action, err := m.appendAction(ctx, reminder, occurrence.OccurrenceID, ActionKindOverdue, ActorKindSystem, StateDue, StateOverdue, "", occurrence.RunID, occurrence.WorkflowID, "", now)
	if err != nil {
		return err
	}
	return m.publishReminderEvent(ctx, "reminder.occurrence_transitioned", reminder, &occurrence, &action)
}

func (m *Manager) markOccurrenceMissed(ctx context.Context, reminder Reminder, occurrence Occurrence, now time.Time) error {
	occurrence.State = StateMissed
	occurrence.MissedAt = ptrTime(now)
	occurrence.UpdatedAt = now
	if err := m.putOccurrence(ctx, occurrence); err != nil {
		return err
	}
	action, err := m.appendAction(ctx, reminder, occurrence.OccurrenceID, ActionKindMissed, ActorKindSystem, occurrence.State, StateMissed, "", occurrence.RunID, occurrence.WorkflowID, "", now)
	if err != nil {
		return err
	}
	return m.publishReminderEvent(ctx, "reminder.occurrence_transitioned", reminder, &occurrence, &action)
}

func (m *Manager) makeOccurrenceDue(ctx context.Context, reminder Reminder, occurrence Occurrence, now time.Time) error {
	occurrence.State = StateDue
	occurrence.BecameDueAt = ptrTime(now)
	occurrence.UpdatedAt = now
	if err := m.putOccurrence(ctx, occurrence); err != nil {
		return err
	}
	reminder.CurrentState = StateDue
	reminder.UpdatedAt = now
	if err := m.putReminder(ctx, reminder); err != nil {
		return err
	}
	action, err := m.appendAction(ctx, reminder, occurrence.OccurrenceID, ActionKindDue, ActorKindSystem, StateSnoozed, StateDue, "", occurrence.RunID, occurrence.WorkflowID, "", now)
	if err != nil {
		return err
	}
	if err := m.publishReminderEvent(ctx, "reminder.occurrence_transitioned", reminder, &occurrence, &action); err != nil {
		return err
	}
	return m.handleDueOccurrence(ctx, reminder, occurrence, now)
}

func (m *Manager) recordWorkflowLaunchFailure(ctx context.Context, reminder Reminder, occurrence Occurrence, now time.Time, reason string) error {
	action, err := m.appendAction(ctx, reminder, occurrence.OccurrenceID, ActionKindWorkflowStartFailed, ActorKindSystem, occurrence.State, occurrence.State, reason, occurrence.RunID, occurrence.WorkflowID, "", now)
	if err != nil {
		return err
	}
	return m.publishReminderEvent(ctx, "reminder.workflow_launch_failed", reminder, &occurrence, &action)
}

func (m *Manager) getActionableOccurrence(ctx context.Context, reminderID, occurrenceID string) (Reminder, Occurrence, error) {
	reminder, ok, err := m.Get(ctx, reminderID)
	if err != nil {
		return Reminder{}, Occurrence{}, err
	}
	if !ok {
		return Reminder{}, Occurrence{}, ErrReminderNotFound
	}
	targetID := strings.TrimSpace(occurrenceID)
	if targetID == "" {
		targetID = reminder.ActiveOccurrenceID
	}
	if targetID == "" {
		return Reminder{}, Occurrence{}, ErrReminderOccurrenceNotFound
	}
	occurrence, ok, err := m.getOccurrenceDoc(ctx, targetID)
	if err != nil {
		return Reminder{}, Occurrence{}, err
	}
	if !ok || occurrence.ReminderID != reminderID {
		return Reminder{}, Occurrence{}, ErrReminderOccurrenceNotFound
	}
	if occurrence.State != StateDue && occurrence.State != StateOverdue && occurrence.State != StateSnoozed {
		return Reminder{}, Occurrence{}, ErrReminderInvalidState
	}
	return reminder, occurrence, nil
}

func (m *Manager) currentOccurrence(ctx context.Context, reminder Reminder) (Occurrence, bool, error) {
	if strings.TrimSpace(reminder.ActiveOccurrenceID) == "" {
		return Occurrence{}, false, nil
	}
	return m.getOccurrenceDoc(ctx, reminder.ActiveOccurrenceID)
}

func (m *Manager) refreshReminderProjection(ctx context.Context, reminder Reminder) (Reminder, error) {
	link, err := refreshFollowUpLink(ctx, m.store, m.env, reminder.FollowUpLink)
	if err != nil {
		return Reminder{}, err
	}
	reminder.FollowUpLink = link
	if reminder.ActiveOccurrenceID != "" {
		if occurrence, ok, err := m.getOccurrenceDoc(ctx, reminder.ActiveOccurrenceID); err != nil {
			return Reminder{}, err
		} else if ok {
			reminder.CurrentState = occurrence.State
			return reminder, nil
		}
	}
	if reminder.CancelledAt != nil {
		reminder.CurrentState = StateCancelled
		return reminder, nil
	}
	if reminder.NextDueAt != nil {
		reminder.CurrentState = StatePending
		return reminder, nil
	}
	items, err := m.listOccurrenceDocs(ctx, OccurrenceFilter{ReminderID: reminder.ReminderID})
	if err != nil {
		return Reminder{}, err
	}
	if len(items) > 0 {
		reminder.CurrentState = items[0].State
	}
	return reminder, nil
}

func (m *Manager) appendAction(ctx context.Context, reminder Reminder, occurrenceID string, kind ActionKind, actor ActorKind, previousState, newState State, reason, runID, workflowID, deliveryID string, createdAt time.Time) (ActionRecord, error) {
	item := ActionRecord{
		ActionID:      newActionID(),
		ReminderID:    reminder.ReminderID,
		OccurrenceID:  occurrenceID,
		ActionKind:    kind,
		ActorKind:     actor,
		PreviousState: previousState,
		NewState:      newState,
		Reason:        strings.TrimSpace(reason),
		RunID:         strings.TrimSpace(runID),
		WorkflowID:    strings.TrimSpace(workflowID),
		DeliveryID:    strings.TrimSpace(deliveryID),
		CreatedAt:     createdAt,
	}
	return item, m.putAction(ctx, item)
}

func (m *Manager) publishReminderEvent(ctx context.Context, name string, reminder Reminder, occurrence *Occurrence, action *ActionRecord) error {
	if m.eventBus == nil {
		return nil
	}
	payload := map[string]any{
		"reminderId":         reminder.ReminderID,
		"behaviorMode":       reminder.BehaviorMode,
		"nextDueAt":          reminder.NextDueAt,
		"currentState":       reminder.CurrentState,
		"activeOccurrenceId": reminder.ActiveOccurrenceID,
	}
	scope := events.Scope{}
	if occurrence != nil {
		payload["occurrenceId"] = occurrence.OccurrenceID
		payload["state"] = occurrence.State
		payload["scheduledFor"] = occurrence.ScheduledFor
		scope.RunID = occurrence.RunID
		scope.WorkflowID = occurrence.WorkflowID
	}
	if action != nil {
		payload["actionKind"] = action.ActionKind
		payload["reason"] = action.Reason
		if action.DeliveryID != "" {
			payload["deliveryId"] = action.DeliveryID
		}
	}
	event := events.Event{
		Category: "reminder",
		Name:     name,
		Scope:    scope,
		Resource: events.Resource{Kind: "reminder", ID: reminder.ReminderID},
		Payload:  payload,
	}
	if event.EnvironmentScope == "" {
		event.EnvironmentScope = events.EnvironmentScopeFromContext(ctx)
	}
	if event.EnvironmentScope == "" {
		event.EnvironmentScope = m.env
	}
	if event.EventID == "" {
		event.EventID = "evt_" + randomHex(8)
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = m.clock.Now().UTC()
	}
	if m.store != nil {
		persisted, err := m.store.AppendEvent(ctx, event)
		if err != nil {
			return err
		}
		event = persisted
	}
	m.eventBus.Publish(event)
	return nil
}

func (m *Manager) listReminderDocs(ctx context.Context) ([]Reminder, error) {
	records, err := m.store.ListReminders(ctx, m.env)
	if err != nil {
		return nil, err
	}
	items := make([]Reminder, 0, len(records))
	for _, record := range records {
		item, decodeErr := decodeReminderRecord(record)
		if decodeErr != nil {
			return nil, decodeErr
		}
		items = append(items, item)
	}
	return items, nil
}

func (m *Manager) getReminderDoc(ctx context.Context, reminderID string) (Reminder, bool, error) {
	record, ok, err := m.store.GetReminder(ctx, m.env, reminderID)
	if err != nil || !ok {
		return Reminder{}, ok, err
	}
	item, err := decodeReminderRecord(record)
	if err != nil {
		return Reminder{}, false, err
	}
	return item, true, nil
}

func (m *Manager) putReminder(ctx context.Context, reminder Reminder) error {
	record, err := encodeReminderRecord(reminder)
	if err != nil {
		return err
	}
	return m.store.UpsertReminder(ctx, record)
}

func (m *Manager) listOccurrenceDocs(ctx context.Context, filter OccurrenceFilter) ([]Occurrence, error) {
	records, err := m.store.ListReminderOccurrences(ctx, m.env, store.ReminderOccurrenceFilter{
		ReminderID:      filter.ReminderID,
		State:           string(filter.State),
		RunID:           filter.RunID,
		WorkflowID:      filter.WorkflowID,
		DeliveryID:      filter.DeliveryID,
		ScheduledBefore: filter.ScheduledBefore,
		ScheduledAfter:  filter.ScheduledAfter,
	})
	if err != nil {
		return nil, err
	}
	items := make([]Occurrence, 0, len(records))
	for _, record := range records {
		item, decodeErr := decodeOccurrenceRecord(record)
		if decodeErr != nil {
			return nil, decodeErr
		}
		items = append(items, item)
	}
	return items, nil
}

func (m *Manager) getOccurrenceDoc(ctx context.Context, occurrenceID string) (Occurrence, bool, error) {
	record, ok, err := m.store.GetReminderOccurrence(ctx, m.env, occurrenceID)
	if err != nil || !ok {
		return Occurrence{}, ok, err
	}
	item, err := decodeOccurrenceRecord(record)
	if err != nil {
		return Occurrence{}, false, err
	}
	return item, true, nil
}

func (m *Manager) putOccurrence(ctx context.Context, occurrence Occurrence) error {
	record, err := encodeOccurrenceRecord(occurrence)
	if err != nil {
		return err
	}
	return m.store.UpsertReminderOccurrence(ctx, record)
}

func (m *Manager) listActionDocs(ctx context.Context, reminderID string) ([]ActionRecord, error) {
	records, err := m.store.ListReminderActions(ctx, m.env, reminderID)
	if err != nil {
		return nil, err
	}
	items := make([]ActionRecord, 0, len(records))
	for _, record := range records {
		item, decodeErr := decodeActionRecord(record)
		if decodeErr != nil {
			return nil, decodeErr
		}
		items = append(items, item)
	}
	return items, nil
}

func (m *Manager) putAction(ctx context.Context, action ActionRecord) error {
	record, err := encodeActionRecord(action)
	if err != nil {
		return err
	}
	return m.store.AppendReminderAction(ctx, record)
}

func encodeReminderRecord(item Reminder) (store.ReminderRecord, error) {
	document, err := json.Marshal(item)
	if err != nil {
		return store.ReminderRecord{}, fmt.Errorf("marshal reminder %s: %w", item.ReminderID, err)
	}
	return store.ReminderRecord{
		ReminderID:         item.ReminderID,
		EnvironmentScope:   item.EnvironmentScope,
		TenantID:           item.TenantID,
		BehaviorMode:       string(item.BehaviorMode),
		CurrentState:       string(item.CurrentState),
		NextDueAt:          item.NextDueAt,
		ActiveOccurrenceID: item.ActiveOccurrenceID,
		UpdatedAt:          item.UpdatedAt,
		Document:           document,
	}, nil
}

func decodeReminderRecord(record store.ReminderRecord) (Reminder, error) {
	var item Reminder
	if err := json.Unmarshal(record.Document, &item); err != nil {
		return Reminder{}, fmt.Errorf("decode reminder %s: %w", record.ReminderID, err)
	}
	item.TenantID = record.TenantID
	return item, nil
}

func encodeOccurrenceRecord(item Occurrence) (store.ReminderOccurrenceRecord, error) {
	document, err := json.Marshal(item)
	if err != nil {
		return store.ReminderOccurrenceRecord{}, fmt.Errorf("marshal reminder occurrence %s: %w", item.OccurrenceID, err)
	}
	return store.ReminderOccurrenceRecord{
		OccurrenceID:         item.OccurrenceID,
		ReminderID:           item.ReminderID,
		EnvironmentScope:     item.EnvironmentScope,
		State:                string(item.State),
		ScheduledFor:         item.ScheduledFor,
		RunID:                item.RunID,
		WorkflowID:           item.WorkflowID,
		LatestDeliveryID:     item.LatestDeliveryID,
		LatestDeliveryStatus: item.LatestDeliveryStatus,
		UpdatedAt:            item.UpdatedAt,
		Document:             document,
	}, nil
}

func decodeOccurrenceRecord(record store.ReminderOccurrenceRecord) (Occurrence, error) {
	var item Occurrence
	if err := json.Unmarshal(record.Document, &item); err != nil {
		return Occurrence{}, fmt.Errorf("decode reminder occurrence %s: %w", record.OccurrenceID, err)
	}
	return item, nil
}

func encodeActionRecord(item ActionRecord) (store.ReminderActionRecord, error) {
	document, err := json.Marshal(item)
	if err != nil {
		return store.ReminderActionRecord{}, fmt.Errorf("marshal reminder action %s: %w", item.ActionID, err)
	}
	return store.ReminderActionRecord{
		ActionID:     item.ActionID,
		ReminderID:   item.ReminderID,
		OccurrenceID: item.OccurrenceID,
		ActionKind:   string(item.ActionKind),
		NewState:     string(item.NewState),
		RunID:        item.RunID,
		WorkflowID:   item.WorkflowID,
		DeliveryID:   item.DeliveryID,
		CreatedAt:    item.CreatedAt,
		Document:     document,
	}, nil
}

func decodeActionRecord(record store.ReminderActionRecord) (ActionRecord, error) {
	var item ActionRecord
	if err := json.Unmarshal(record.Document, &item); err != nil {
		return ActionRecord{}, fmt.Errorf("decode reminder action %s: %w", record.ActionID, err)
	}
	return item, nil
}

func nextReminderDueAfter(trigger scheduler.Trigger, after time.Time) (*time.Time, error) {
	if trigger.Kind == scheduler.TriggerKindOnce {
		return nil, nil
	}
	return scheduler.NextDueAfter(trigger, after)
}

func cloneWorkflowLaunchConfig(item *WorkflowLaunchConfig) *WorkflowLaunchConfig {
	if item == nil {
		return nil
	}
	out := *item
	return &out
}

func cloneFollowUpLink(item *FollowUpLink) *FollowUpLink {
	if item == nil {
		return nil
	}
	out := *item
	return &out
}

func nonEmptyActor(actor, fallback ActorKind) ActorKind {
	if actor != "" {
		return actor
	}
	return fallback
}

func ptrTime(value time.Time) *time.Time {
	normalized := value.UTC()
	return &normalized
}

func newReminderID() string   { return "rem_" + randomHex(8) }
func newOccurrenceID() string { return "rem_occ_" + randomHex(8) }
func newActionID() string     { return "rem_act_" + randomHex(8) }

func randomHex(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(buf)
}
