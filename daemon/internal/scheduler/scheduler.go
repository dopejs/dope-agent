package scheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

type Clock interface {
	Now() time.Time
}

type WorkflowLaunchResult struct {
	RunID            string
	WorkflowID       string
	DownstreamStatus DownstreamStatus
}

type WorkflowLauncher interface {
	LaunchScheduledWorkflow(ctx context.Context, target WorkflowTarget, scheduleID, scheduleAttemptID string) (WorkflowLaunchResult, error)
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

type CreateInput struct {
	Trigger     Trigger
	Target      Target
	RetryPolicy RetryPolicy
}

type Dependencies struct {
	Config           config.Config
	Runtime          *runtime.Manager
	EventBus         *events.Bus
	Store            *store.SQLiteStore
	Checkpoints      *checkpoints.Manager
	WorkflowLauncher WorkflowLauncher
	Billing          *billing.Manager
	Clock            Clock
	TickInterval     time.Duration
}

type Scheduler struct {
	cfg          config.Config
	runtime      *runtime.Manager
	eventBus     *events.Bus
	store        *store.SQLiteStore
	checkpoints  *checkpoints.Manager
	workflow     WorkflowLauncher
	billing      *billing.Manager
	clock        Clock
	tickInterval time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(deps Dependencies) *Scheduler {
	clock := deps.Clock
	if clock == nil {
		clock = realClock{}
	}
	interval := deps.TickInterval
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	return &Scheduler{
		cfg:          deps.Config,
		runtime:      deps.Runtime,
		eventBus:     deps.EventBus,
		store:        deps.Store,
		checkpoints:  deps.Checkpoints,
		workflow:     deps.WorkflowLauncher,
		billing:      deps.Billing,
		clock:        clock,
		tickInterval: interval,
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	if s == nil || s.store == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		_ = s.CatchUp(runCtx)
		ticker := time.NewTicker(s.tickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				_ = s.Tick(runCtx)
			}
		}
	}()
	return nil
}

func (s *Scheduler) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
	return nil
}

func (s *Scheduler) Store() *store.SQLiteStore {
	if s == nil {
		return nil
	}
	return s.store
}

func (s *Scheduler) Create(ctx context.Context, input CreateInput) (Schedule, error) {
	if s == nil || s.store == nil {
		return Schedule{}, fmt.Errorf("scheduler store is not configured")
	}
	now := s.clock.Now().UTC()
	if input.Target.UpdatedAt.IsZero() {
		input.Target.UpdatedAt = now
	}
	nextDueAt, err := NextDueAfter(input.Trigger, now.Add(-time.Second))
	if err != nil {
		return Schedule{}, err
	}
	schedule := Schedule{
		ScheduleID:       newScheduleID(),
		EnvironmentScope: string(s.cfg.Environment),
		TargetRefID:      newTargetRefID(),
		Trigger:          input.Trigger,
		Target:           input.Target,
		RetryPolicy:      normalizeRetryPolicy(input.RetryPolicy),
		CreatedAt:        now,
		UpdatedAt:        now,
		NextDueAt:        nextDueAt,
	}
	if tenantContext, ok := tenantctx.FromContext(ctx); ok {
		schedule.TenantID = strings.TrimSpace(tenantContext.TenantID)
	}
	schedule.Target.UpdatedAt = now
	schedule.Target.Revision = 1
	schedule.Target.Active = true
	schedule.Kind = deriveScheduleKind(schedule.Trigger.Kind)
	schedule.Status = initialScheduleStatus(schedule.Kind)
	schedule.Trigger.NextDueAt = nextDueAt
	schedule.Target.Summary = targetSummary(schedule.Target)

	if err := s.persistSchedule(ctx, schedule); err != nil {
		return Schedule{}, err
	}
	if err := s.publishEvent(ctx, "schedule.created", schedule, nil, map[string]any{
		"status":      schedule.Status,
		"targetKind":  schedule.Target.Kind,
		"targetRefId": schedule.TargetRefID,
	}); err != nil {
		return Schedule{}, err
	}
	return schedule, nil
}

func (s *Scheduler) List(ctx context.Context) ([]Schedule, error) {
	if s == nil || s.store == nil {
		return nil, nil
	}
	records, err := s.store.ListSchedules(ctx, string(s.cfg.Environment))
	if err != nil {
		return nil, err
	}
	items := make([]Schedule, 0, len(records))
	for _, record := range records {
		schedule, hydrateErr := s.hydrateSchedule(ctx, record)
		if hydrateErr != nil {
			return nil, hydrateErr
		}
		items = append(items, schedule)
	}
	return items, nil
}

func (s *Scheduler) Get(ctx context.Context, scheduleID string) (Schedule, bool, error) {
	if s == nil || s.store == nil {
		return Schedule{}, false, nil
	}
	record, ok, err := s.store.GetSchedule(ctx, string(s.cfg.Environment), scheduleID)
	if err != nil || !ok {
		return Schedule{}, ok, err
	}
	schedule, err := s.hydrateSchedule(ctx, record)
	if err != nil {
		return Schedule{}, false, err
	}
	return schedule, true, nil
}

func (s *Scheduler) Pause(ctx context.Context, scheduleID string) (Schedule, bool, error) {
	schedule, ok, err := s.Get(ctx, scheduleID)
	if err != nil || !ok {
		return Schedule{}, ok, err
	}
	if IsTerminalScheduleStatus(schedule.Status) {
		return schedule, true, nil
	}
	now := s.clock.Now().UTC()
	schedule.Status = ScheduleStatusPaused
	schedule.PausedAt = &now
	schedule.UpdatedAt = now
	if err := s.persistSchedule(ctx, schedule); err != nil {
		return Schedule{}, false, err
	}
	if err := s.publishEvent(ctx, "schedule.status_changed", schedule, nil, map[string]any{"status": schedule.Status}); err != nil {
		return Schedule{}, false, err
	}
	return schedule, true, nil
}

func (s *Scheduler) Resume(ctx context.Context, scheduleID string) (Schedule, bool, error) {
	schedule, ok, err := s.Get(ctx, scheduleID)
	if err != nil || !ok {
		return Schedule{}, ok, err
	}
	if schedule.Status != ScheduleStatusPaused {
		return schedule, true, nil
	}
	now := s.clock.Now().UTC()
	schedule.PausedAt = nil
	schedule.UpdatedAt = now
	switch schedule.Kind {
	case ScheduleKindRecurring:
		schedule.Status = ScheduleStatusActive
		nextDueAt, nextErr := NextDueAfter(schedule.Trigger, now)
		if nextErr != nil {
			return Schedule{}, false, nextErr
		}
		schedule.NextDueAt = nextDueAt
		schedule.Trigger.NextDueAt = nextDueAt
	case ScheduleKindOneTime:
		schedule.Status = ScheduleStatusScheduled
		if schedule.Trigger.FireAt != nil {
			fireAt := schedule.Trigger.FireAt.UTC()
			if fireAt.After(now) {
				schedule.NextDueAt = &fireAt
				schedule.Trigger.NextDueAt = &fireAt
			} else {
				schedule.NextDueAt = nil
				schedule.Trigger.NextDueAt = nil
			}
		}
	}
	if err := s.persistSchedule(ctx, schedule); err != nil {
		return Schedule{}, false, err
	}
	if err := s.publishEvent(ctx, "schedule.status_changed", schedule, nil, map[string]any{"status": schedule.Status}); err != nil {
		return Schedule{}, false, err
	}
	return schedule, true, nil
}

func (s *Scheduler) Cancel(ctx context.Context, scheduleID string) (Schedule, bool, error) {
	schedule, ok, err := s.Get(ctx, scheduleID)
	if err != nil || !ok {
		return Schedule{}, ok, err
	}
	if schedule.Status == ScheduleStatusCancelled {
		return schedule, true, nil
	}
	now := s.clock.Now().UTC()
	schedule.Status = ScheduleStatusCancelled
	schedule.CancelledAt = &now
	schedule.UpdatedAt = now
	if err := s.persistSchedule(ctx, schedule); err != nil {
		return Schedule{}, false, err
	}
	if err := s.publishEvent(ctx, "schedule.status_changed", schedule, nil, map[string]any{"status": schedule.Status}); err != nil {
		return Schedule{}, false, err
	}
	return schedule, true, nil
}

func (s *Scheduler) CatchUp(ctx context.Context) error {
	items, err := s.List(ctx)
	if err != nil {
		return err
	}
	now := s.clock.Now().UTC()
	for _, schedule := range items {
		if schedule.NextDueAt == nil || schedule.NextDueAt.After(now) || IsTerminalScheduleStatus(schedule.Status) {
			continue
		}
		if schedule.Kind == ScheduleKindRecurring && schedule.Status != ScheduleStatusCancelled {
			if err := s.recordMissedIntervals(ctx, &schedule, now); err != nil {
				return err
			}
		}
		if err := s.processDueSchedule(ctx, &schedule, now, TriggerSourceCatchUp); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) Tick(ctx context.Context) error {
	items, err := s.List(ctx)
	if err != nil {
		return err
	}
	now := s.clock.Now().UTC()
	for idx := range items {
		if err := s.reconcileDownstream(ctx, &items[idx]); err != nil {
			return err
		}
		if err := s.processRetries(ctx, &items[idx], now); err != nil {
			return err
		}
		if items[idx].NextDueAt == nil || items[idx].NextDueAt.After(now) {
			continue
		}
		if err := s.processDueSchedule(ctx, &items[idx], now, TriggerSourceNormal); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) processRetries(ctx context.Context, schedule *Schedule, now time.Time) error {
	for idx := range schedule.Attempts {
		attempt := &schedule.Attempts[idx]
		if attempt.DispatchStatus != DispatchStatusFailed || attempt.NextRetryAt == nil || attempt.NextRetryAt.After(now) {
			continue
		}
		return s.dispatchAttempt(ctx, schedule, attempt, TriggerSourceRetry)
	}
	return nil
}

func (s *Scheduler) processDueSchedule(ctx context.Context, schedule *Schedule, now time.Time, source TriggerSource) error {
	if schedule.NextDueAt == nil {
		return nil
	}

	attempt := DispatchAttempt{
		AttemptID:        newAttemptID(),
		ScheduleID:       schedule.ScheduleID,
		DueAt:            schedule.NextDueAt.UTC(),
		TriggerSource:    source,
		DispatchStatus:   DispatchStatusPending,
		RetryBudget:      schedule.RetryPolicy.MaxRetries,
		DownstreamStatus: DownstreamStatusNone,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if schedule.Status == ScheduleStatusPaused {
		attempt.DispatchStatus = DispatchStatusSkippedPaused
		attempt.SkippedReason = "schedule_paused"
		return s.finishNonDispatchAttempt(ctx, schedule, attempt, now, true)
	}
	if schedule.Status == ScheduleStatusCancelled {
		attempt.DispatchStatus = DispatchStatusSkippedCancelled
		attempt.SkippedReason = "schedule_cancelled"
		return s.finishNonDispatchAttempt(ctx, schedule, attempt, now, true)
	}
	if IsTerminalScheduleStatus(schedule.Status) {
		return nil
	}
	if hasActiveAttempt(schedule.Attempts) {
		attempt.DispatchStatus = DispatchStatusSkippedOverlap
		attempt.SkippedReason = "schedule_execution_in_progress"
		return s.finishNonDispatchAttempt(ctx, schedule, attempt, now, true)
	}

	schedule.Attempts = append([]DispatchAttempt{attempt}, schedule.Attempts...)
	return s.dispatchAttempt(ctx, schedule, &schedule.Attempts[0], source)
}

func (s *Scheduler) finishNonDispatchAttempt(ctx context.Context, schedule *Schedule, attempt DispatchAttempt, now time.Time, advance bool) error {
	schedule.Attempts = append([]DispatchAttempt{attempt}, schedule.Attempts...)
	schedule.LastAttemptAt = &now
	schedule.LastOutcome = string(attempt.DispatchStatus)
	schedule.UpdatedAt = now
	if advance {
		if err := s.advanceScheduleAfterDue(schedule, now); err != nil {
			return err
		}
	}
	if err := s.persistSchedule(ctx, *schedule); err != nil {
		return err
	}
	return s.publishEvent(ctx, "schedule.dispatch_recorded", *schedule, &attempt, map[string]any{
		"dispatchStatus": attempt.DispatchStatus,
		"skippedReason":  attempt.SkippedReason,
	})
}

func (s *Scheduler) dispatchAttempt(ctx context.Context, schedule *Schedule, attempt *DispatchAttempt, source TriggerSource) error {
	ctx = s.withScheduleTenantContext(ctx, *schedule)
	now := s.clock.Now().UTC()
	attempt.TriggerSource = source
	attempt.DispatchStatus = DispatchStatusDispatching
	attempt.UpdatedAt = now
	schedule.LastAttemptAt = &now
	schedule.UpdatedAt = now
	if err := s.persistSchedule(ctx, *schedule); err != nil {
		return err
	}
	if err := s.publishEvent(ctx, "schedule.dispatch_attempted", *schedule, attempt, map[string]any{
		"dispatchStatus": attempt.DispatchStatus,
		"dueAt":          attempt.DueAt,
		"triggerSource":  attempt.TriggerSource,
	}); err != nil {
		return err
	}

	targetRecord, ok, err := s.store.GetScheduleTarget(ctx, schedule.ScheduleID, schedule.TargetRefID)
	if err != nil {
		return err
	}
	if !ok || !targetRecord.Active {
		return s.recordDispatchFailure(ctx, schedule, attempt, "invalid_target", "schedule target reference is not available", true)
	}
	target, err := decodeTargetRecord(targetRecord)
	if err != nil {
		return s.recordDispatchFailure(ctx, schedule, attempt, "invalid_target_document", err.Error(), true)
	}

	switch target.Kind {
	case TargetKindRun:
		input := runtime.CreateRunInput{
			SessionID:         target.Run.SessionID,
			ScheduleID:        schedule.ScheduleID,
			ScheduleAttemptID: attempt.AttemptID,
			Entrypoint:        target.Run.Entrypoint,
			Goal:              target.Run.Goal,
		}
		var reservation billing.UsageReservation
		if s.billing != nil && strings.TrimSpace(schedule.TenantID) != "" {
			input.RunID = runtime.NewRunID()
			result, reserveErr := s.billing.Reserve(ctx, billing.ReserveInput{
				TenantID:          strings.TrimSpace(schedule.TenantID),
				Category:          billing.CategoryRunLaunches,
				Amount:            1,
				OperationKey:      billing.RunOperationKey(schedule.TenantID, "schedule:"+attempt.AttemptID, input.RunID),
				ReservationPoint:  "scheduler run target before runtime.CreateRun",
				GuardedEntryPoint: "scheduler run target",
				Hosted:            s.cfg.Environment == config.EnvironmentProd,
			})
			if reserveErr != nil {
				return s.recordDispatchFailure(ctx, schedule, attempt, "quota_denied", reserveErr.Error(), true)
			}
			reservation = result.Reservation
		}
		run, createErr := s.runtime.CreateRun(input)
		if createErr != nil {
			releaseSchedulerBillingReservation(ctx, s.billing, reservation, "scheduled run creation failed before persistence")
			return s.recordDispatchFailure(ctx, schedule, attempt, "run_create_failed", createErr.Error(), true)
		}
		attempt.RunID = run.RunID
		attempt.ResolvedTargetRevision = target.Revision
		attempt.DispatchStatus = DispatchStatusDispatched
		attempt.DownstreamStatus = mapRunStatus(run.Status)
		attempt.UpdatedAt = now
		schedule.LastOutcome = string(attempt.DispatchStatus)
		if err := s.advanceScheduleAfterDispatch(schedule, now); err != nil {
			return err
		}
		if err := s.store.UpsertRun(ctx, run); err != nil {
			releaseSchedulerBillingReservation(ctx, s.billing, reservation, "scheduled run persistence failed before commit")
			return err
		}
		if err := commitSchedulerBillingReservation(ctx, s.billing, reservation, "scheduled run persisted"); err != nil {
			return err
		}
		if s.checkpoints != nil {
			if err := s.checkpoints.SaveRunCheckpoint(ctx, run.RunID); err != nil {
				return err
			}
		}
	case TargetKindWorkflow:
		if s.workflow == nil {
			return s.recordDispatchFailure(ctx, schedule, attempt, "workflow_launcher_unavailable", "scheduler workflow launcher is not configured", true)
		}
		result, launchErr := s.workflow.LaunchScheduledWorkflow(ctx, *target.Workflow, schedule.ScheduleID, attempt.AttemptID)
		if launchErr != nil {
			return s.recordDispatchFailure(ctx, schedule, attempt, "workflow_dispatch_failed", launchErr.Error(), true)
		}
		attempt.RunID = result.RunID
		attempt.WorkflowID = result.WorkflowID
		attempt.ResolvedTargetRevision = target.Revision
		attempt.DispatchStatus = DispatchStatusDispatched
		attempt.DownstreamStatus = result.DownstreamStatus
		attempt.UpdatedAt = now
		schedule.LastOutcome = string(attempt.DispatchStatus)
		if err := s.advanceScheduleAfterDispatch(schedule, now); err != nil {
			return err
		}
	default:
		return s.recordDispatchFailure(ctx, schedule, attempt, "unsupported_target_kind", fmt.Sprintf("unsupported target kind %q", target.Kind), true)
	}

	if err := s.persistSchedule(ctx, *schedule); err != nil {
		return err
	}
	return s.publishEvent(ctx, "schedule.dispatch_recorded", *schedule, attempt, map[string]any{
		"dispatchStatus":         attempt.DispatchStatus,
		"resolvedTargetRevision": attempt.ResolvedTargetRevision,
		"runId":                  attempt.RunID,
		"workflowId":             attempt.WorkflowID,
	})
}

func (s *Scheduler) recordDispatchFailure(ctx context.Context, schedule *Schedule, attempt *DispatchAttempt, class, reason string, allowRetry bool) error {
	now := s.clock.Now().UTC()
	attempt.FailureClass = class
	attempt.FailureReason = reason
	attempt.UpdatedAt = now
	nextRetryAt := nextRetryTime(schedule.RetryPolicy, attempt.RetryCount)
	if allowRetry && attempt.RetryCount < schedule.RetryPolicy.MaxRetries && nextRetryAt != nil {
		attempt.DispatchStatus = DispatchStatusFailed
		attempt.NextRetryAt = nextRetryAt
		attempt.RetryCount++
		schedule.LastOutcome = string(attempt.DispatchStatus)
	} else {
		attempt.DispatchStatus = DispatchStatusExhausted
		attempt.NextRetryAt = nil
		schedule.LastOutcome = string(attempt.DispatchStatus)
		if schedule.Kind == ScheduleKindOneTime {
			schedule.Status = ScheduleStatusDispatchFailed
			schedule.CompletedAt = &now
			schedule.NextDueAt = nil
			schedule.Trigger.NextDueAt = nil
		} else {
			if err := s.advanceScheduleAfterDispatch(schedule, now); err != nil {
				return err
			}
		}
	}
	schedule.LastAttemptAt = &now
	schedule.UpdatedAt = now
	if err := s.persistSchedule(ctx, *schedule); err != nil {
		return err
	}
	if attempt.NextRetryAt != nil {
		if err := s.publishEvent(ctx, "schedule.retry_scheduled", *schedule, attempt, map[string]any{
			"dispatchStatus": attempt.DispatchStatus,
			"failureClass":   attempt.FailureClass,
			"failureReason":  attempt.FailureReason,
			"retryCount":     attempt.RetryCount,
			"nextRetryAt":    attempt.NextRetryAt,
		}); err != nil {
			return err
		}
	}
	return s.publishEvent(ctx, "schedule.dispatch_recorded", *schedule, attempt, map[string]any{
		"dispatchStatus": attempt.DispatchStatus,
		"failureClass":   attempt.FailureClass,
		"failureReason":  attempt.FailureReason,
		"retryCount":     attempt.RetryCount,
		"nextRetryAt":    attempt.NextRetryAt,
	})
}

func (s *Scheduler) reconcileDownstream(ctx context.Context, schedule *Schedule) error {
	changed := false
	for idx := range schedule.Attempts {
		attempt := &schedule.Attempts[idx]
		if attempt.DispatchStatus != DispatchStatusDispatched || attempt.RunID == "" {
			continue
		}
		next := attempt.DownstreamStatus
		if attempt.WorkflowID != "" {
			workflow, ok, err := s.store.GetWorkflow(ctx, string(s.cfg.Environment), attempt.RunID, attempt.WorkflowID)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			next = mapWorkflowStatus(workflow.Status)
		} else {
			run, ok := s.runtime.GetRun(attempt.RunID)
			if !ok {
				continue
			}
			next = mapRunStatus(run.Status)
		}
		if next != attempt.DownstreamStatus {
			attempt.DownstreamStatus = next
			attempt.UpdatedAt = s.clock.Now().UTC()
			changed = true
			if isTerminalDownstream(next) {
				schedule.LastOutcome = string(next)
				schedule.UpdatedAt = attempt.UpdatedAt
			}
		}
	}
	if !changed {
		return nil
	}
	return s.persistSchedule(ctx, *schedule)
}

func (s *Scheduler) recordMissedIntervals(ctx context.Context, schedule *Schedule, now time.Time) error {
	if schedule.NextDueAt == nil || schedule.Kind != ScheduleKindRecurring {
		return nil
	}
	due := schedule.NextDueAt.UTC()
	missedCount := 0
	cursor := due
	for {
		nextDueAt, err := NextDueAfter(schedule.Trigger, cursor)
		if err != nil || nextDueAt == nil || !nextDueAt.Before(now) {
			break
		}
		missedCount++
		cursor = nextDueAt.UTC()
	}
	if missedCount == 0 {
		return nil
	}
	attempt := DispatchAttempt{
		AttemptID:        newAttemptID(),
		ScheduleID:       schedule.ScheduleID,
		DueAt:            due,
		TriggerSource:    TriggerSourceCatchUp,
		DispatchStatus:   DispatchStatusMissed,
		MissedCount:      missedCount,
		DownstreamStatus: DownstreamStatusNone,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	schedule.Attempts = append([]DispatchAttempt{attempt}, schedule.Attempts...)
	schedule.LastAttemptAt = &now
	schedule.LastOutcome = string(attempt.DispatchStatus)
	cursorCopy := cursor.UTC()
	schedule.NextDueAt = &cursorCopy
	schedule.Trigger.NextDueAt = &cursorCopy
	schedule.UpdatedAt = now
	if err := s.persistSchedule(ctx, *schedule); err != nil {
		return err
	}
	return s.publishEvent(ctx, "schedule.dispatch_recorded", *schedule, &attempt, map[string]any{
		"dispatchStatus": attempt.DispatchStatus,
		"missedCount":    attempt.MissedCount,
	})
}

func (s *Scheduler) advanceScheduleAfterDue(schedule *Schedule, now time.Time) error {
	switch schedule.Kind {
	case ScheduleKindOneTime:
		schedule.NextDueAt = nil
		schedule.Trigger.NextDueAt = nil
	case ScheduleKindRecurring:
		nextDueAt, err := NextDueAfter(schedule.Trigger, now)
		if err != nil {
			return err
		}
		schedule.NextDueAt = nextDueAt
		schedule.Trigger.NextDueAt = nextDueAt
	}
	return nil
}

func (s *Scheduler) advanceScheduleAfterDispatch(schedule *Schedule, now time.Time) error {
	switch schedule.Kind {
	case ScheduleKindOneTime:
		schedule.Status = ScheduleStatusCompleted
		schedule.CompletedAt = &now
		schedule.NextDueAt = nil
		schedule.Trigger.NextDueAt = nil
	case ScheduleKindRecurring:
		schedule.Status = ScheduleStatusActive
		nextDueAt, err := NextDueAfter(schedule.Trigger, now)
		if err != nil {
			return err
		}
		schedule.NextDueAt = nextDueAt
		schedule.Trigger.NextDueAt = nextDueAt
	}
	schedule.UpdatedAt = now
	return nil
}

func (s *Scheduler) hydrateSchedule(ctx context.Context, record store.ScheduleRecord) (Schedule, error) {
	var schedule Schedule
	if len(record.Document) > 0 {
		if err := json.Unmarshal(record.Document, &schedule); err != nil {
			return Schedule{}, fmt.Errorf("decode schedule %s: %w", record.ScheduleID, err)
		}
	}
	if schedule.ScheduleID == "" {
		schedule = Schedule{
			ScheduleID:       record.ScheduleID,
			EnvironmentScope: record.EnvironmentScope,
			TenantID:         record.TenantID,
			Kind:             ScheduleKind(record.Kind),
			Status:           ScheduleStatus(record.Status),
			TargetRefID:      record.TargetRefID,
			CreatedAt:        record.CreatedAt,
			UpdatedAt:        record.UpdatedAt,
			NextDueAt:        record.NextDueAt,
			LastAttemptAt:    record.LastAttemptAt,
			LastOutcome:      record.LastOutcome,
			PausedAt:         record.PausedAt,
			CancelledAt:      record.CancelledAt,
			CompletedAt:      record.CompletedAt,
		}
	}
	targetRecord, ok, err := s.store.GetScheduleTarget(ctx, record.ScheduleID, record.TargetRefID)
	if err != nil {
		return Schedule{}, err
	}
	if ok {
		target, decodeErr := decodeTargetRecord(targetRecord)
		if decodeErr != nil {
			return Schedule{}, decodeErr
		}
		schedule.Target = target
	}
	attemptRecords, err := s.store.ListScheduleDispatchAttempts(ctx, record.ScheduleID)
	if err != nil {
		return Schedule{}, err
	}
	attempts := make([]DispatchAttempt, 0, len(attemptRecords))
	for _, attemptRecord := range attemptRecords {
		attempt, decodeErr := decodeAttemptRecord(attemptRecord)
		if decodeErr != nil {
			return Schedule{}, decodeErr
		}
		attempts = append(attempts, attempt)
	}
	schedule.Attempts = attempts
	schedule.EnvironmentScope = record.EnvironmentScope
	schedule.TenantID = record.TenantID
	schedule.Kind = ScheduleKind(record.Kind)
	schedule.Status = ScheduleStatus(record.Status)
	schedule.TargetRefID = record.TargetRefID
	schedule.NextDueAt = record.NextDueAt
	schedule.LastAttemptAt = record.LastAttemptAt
	schedule.LastOutcome = record.LastOutcome
	schedule.CreatedAt = record.CreatedAt
	schedule.UpdatedAt = record.UpdatedAt
	schedule.PausedAt = record.PausedAt
	schedule.CancelledAt = record.CancelledAt
	schedule.CompletedAt = record.CompletedAt
	schedule.Trigger.NextDueAt = record.NextDueAt
	return schedule, nil
}

func (s *Scheduler) persistSchedule(ctx context.Context, schedule Schedule) error {
	scheduleDoc, err := json.Marshal(schedule)
	if err != nil {
		return fmt.Errorf("marshal schedule %s: %w", schedule.ScheduleID, err)
	}
	scheduleRecord := store.ScheduleRecord{
		ScheduleID:       schedule.ScheduleID,
		EnvironmentScope: schedule.EnvironmentScope,
		TenantID:         schedule.TenantID,
		Kind:             string(schedule.Kind),
		Status:           string(schedule.Status),
		TargetRefID:      schedule.TargetRefID,
		Timezone:         schedule.Trigger.Timezone,
		NextDueAt:        schedule.NextDueAt,
		LastAttemptAt:    schedule.LastAttemptAt,
		LastOutcome:      schedule.LastOutcome,
		CreatedAt:        schedule.CreatedAt,
		UpdatedAt:        schedule.UpdatedAt,
		PausedAt:         schedule.PausedAt,
		CancelledAt:      schedule.CancelledAt,
		CompletedAt:      schedule.CompletedAt,
		Document:         scheduleDoc,
	}
	if err := s.store.UpsertSchedule(ctx, scheduleRecord); err != nil {
		return err
	}

	targetDoc, err := json.Marshal(schedule.Target)
	if err != nil {
		return fmt.Errorf("marshal schedule target %s: %w", schedule.TargetRefID, err)
	}
	if err := s.store.UpsertScheduleTarget(ctx, store.ScheduleTargetRecord{
		TargetRefID: schedule.TargetRefID,
		ScheduleID:  schedule.ScheduleID,
		TargetKind:  string(schedule.Target.Kind),
		Revision:    schedule.Target.Revision,
		Active:      schedule.Target.Active,
		UpdatedAt:   schedule.Target.UpdatedAt,
		Document:    targetDoc,
	}); err != nil {
		return err
	}
	for _, attempt := range schedule.Attempts {
		attemptDoc, marshalErr := json.Marshal(attempt)
		if marshalErr != nil {
			return fmt.Errorf("marshal schedule attempt %s: %w", attempt.AttemptID, marshalErr)
		}
		if err := s.store.UpsertScheduleDispatchAttempt(ctx, store.ScheduleDispatchAttemptRecord{
			AttemptID:              attempt.AttemptID,
			ScheduleID:             schedule.ScheduleID,
			DueAt:                  attempt.DueAt,
			TriggerSource:          string(attempt.TriggerSource),
			DispatchStatus:         string(attempt.DispatchStatus),
			FailureClass:           attempt.FailureClass,
			FailureReason:          attempt.FailureReason,
			RetryCount:             attempt.RetryCount,
			RetryBudget:            attempt.RetryBudget,
			NextRetryAt:            attempt.NextRetryAt,
			ResolvedTargetRevision: attempt.ResolvedTargetRevision,
			RunID:                  attempt.RunID,
			WorkflowID:             attempt.WorkflowID,
			DownstreamStatus:       string(attempt.DownstreamStatus),
			SkippedReason:          attempt.SkippedReason,
			MissedCount:            attempt.MissedCount,
			CreatedAt:              attempt.CreatedAt,
			UpdatedAt:              attempt.UpdatedAt,
			Document:               attemptDoc,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) publishEvent(ctx context.Context, name string, schedule Schedule, attempt *DispatchAttempt, payload map[string]any) error {
	if s.eventBus == nil && s.store == nil {
		return nil
	}
	event := events.Event{
		EnvironmentScope: firstNonEmpty(schedule.EnvironmentScope, events.EnvironmentScopeFromContext(ctx)),
		Category:         "schedule",
		Name:             name,
		Scope: events.Scope{
			ScheduleID: schedule.ScheduleID,
		},
		Resource: events.Resource{
			Kind: "schedule",
			ID:   schedule.ScheduleID,
		},
		Payload: map[string]any{
			"scheduleId": schedule.ScheduleID,
			"status":     schedule.Status,
		},
	}
	if attempt != nil {
		event.Scope.ScheduleAttemptID = attempt.AttemptID
		event.Payload["scheduleAttemptId"] = attempt.AttemptID
		event.Payload["dispatchStatus"] = attempt.DispatchStatus
		event.Payload["dueAt"] = attempt.DueAt
		event.Payload["triggerSource"] = attempt.TriggerSource
		if attempt.RunID != "" {
			event.Scope.RunID = attempt.RunID
			event.Payload["runId"] = attempt.RunID
		}
		if attempt.WorkflowID != "" {
			event.Scope.WorkflowID = attempt.WorkflowID
			event.Payload["workflowId"] = attempt.WorkflowID
		}
	}
	for key, value := range payload {
		event.Payload[key] = value
	}
	if s.store != nil {
		persisted, err := s.store.AppendEvent(ctx, event)
		if err != nil {
			return err
		}
		event = persisted
	}
	if s.eventBus != nil {
		s.eventBus.Publish(event)
	}
	return nil
}

func deriveScheduleKind(kind TriggerKind) ScheduleKind {
	if kind == TriggerKindCron {
		return ScheduleKindRecurring
	}
	return ScheduleKindOneTime
}

func (s *Scheduler) withScheduleTenantContext(ctx context.Context, schedule Schedule) context.Context {
	if _, ok := tenantctx.FromContext(ctx); ok {
		return ctx
	}
	tenantID := strings.TrimSpace(schedule.TenantID)
	if tenantID == "" {
		return ctx
	}
	return tenantctx.WithContext(ctx, identity.TenantContext{
		TenantID:    tenantID,
		PrincipalID: "system:scheduler",
	})
}

func releaseSchedulerBillingReservation(ctx context.Context, manager *billing.Manager, reservation billing.UsageReservation, reason string) {
	if manager == nil || reservation.ReservationID == "" {
		return
	}
	_, _ = manager.Release(ctx, billing.ResolveInput{
		TenantID:     reservation.TenantID,
		Category:     reservation.Category,
		OperationKey: reservation.OperationKey,
		Amount:       reservation.AmountReserved,
		ReasonCode:   "billing.scheduled_run_released",
		Reason:       reason,
	})
}

func commitSchedulerBillingReservation(ctx context.Context, manager *billing.Manager, reservation billing.UsageReservation, reason string) error {
	if manager == nil || reservation.ReservationID == "" {
		return nil
	}
	_, err := manager.Commit(ctx, billing.ResolveInput{
		TenantID:     reservation.TenantID,
		Category:     reservation.Category,
		OperationKey: reservation.OperationKey,
		Amount:       reservation.AmountReserved,
		ReasonCode:   "billing.scheduled_run_committed",
		Reason:       reason,
	})
	if errors.Is(err, billing.ErrReservationNotFound) {
		return nil
	}
	return err
}

func initialScheduleStatus(kind ScheduleKind) ScheduleStatus {
	if kind == ScheduleKindRecurring {
		return ScheduleStatusActive
	}
	return ScheduleStatusScheduled
}

func normalizeRetryPolicy(policy RetryPolicy) RetryPolicy {
	if policy.BackoffKind == "" {
		policy.BackoffKind = RetryBackoffFixed
	}
	if policy.BaseDelaySeconds <= 0 {
		policy.BaseDelaySeconds = 5
	}
	if policy.MaxDelaySeconds <= 0 {
		policy.MaxDelaySeconds = policy.BaseDelaySeconds
	}
	if policy.MaxRetries < 0 {
		policy.MaxRetries = 0
	}
	return policy
}

func nextRetryTime(policy RetryPolicy, retryCount int) *time.Time {
	if retryCount >= policy.MaxRetries {
		return nil
	}
	delay := policy.BaseDelaySeconds
	if policy.BackoffKind == RetryBackoffExponential {
		delay = policy.BaseDelaySeconds << retryCount
	}
	if delay > policy.MaxDelaySeconds {
		delay = policy.MaxDelaySeconds
	}
	next := time.Now().UTC().Add(time.Duration(delay) * time.Second)
	return &next
}

func decodeTargetRecord(record store.ScheduleTargetRecord) (Target, error) {
	var target Target
	if len(record.Document) > 0 {
		if err := json.Unmarshal(record.Document, &target); err != nil {
			return Target{}, fmt.Errorf("decode schedule target %s: %w", record.TargetRefID, err)
		}
	}
	target.Kind = TargetKind(record.TargetKind)
	target.Revision = record.Revision
	target.Active = record.Active
	target.UpdatedAt = record.UpdatedAt
	target.Summary = targetSummary(target)
	return target, nil
}

func decodeAttemptRecord(record store.ScheduleDispatchAttemptRecord) (DispatchAttempt, error) {
	var attempt DispatchAttempt
	if len(record.Document) > 0 {
		if err := json.Unmarshal(record.Document, &attempt); err != nil {
			return DispatchAttempt{}, fmt.Errorf("decode schedule attempt %s: %w", record.AttemptID, err)
		}
	}
	attempt.AttemptID = record.AttemptID
	attempt.ScheduleID = record.ScheduleID
	attempt.DueAt = record.DueAt
	attempt.TriggerSource = TriggerSource(record.TriggerSource)
	attempt.DispatchStatus = DispatchStatus(record.DispatchStatus)
	attempt.FailureClass = record.FailureClass
	attempt.FailureReason = record.FailureReason
	attempt.RetryCount = record.RetryCount
	attempt.RetryBudget = record.RetryBudget
	attempt.NextRetryAt = record.NextRetryAt
	attempt.ResolvedTargetRevision = record.ResolvedTargetRevision
	attempt.RunID = record.RunID
	attempt.WorkflowID = record.WorkflowID
	attempt.DownstreamStatus = DownstreamStatus(record.DownstreamStatus)
	attempt.SkippedReason = record.SkippedReason
	attempt.MissedCount = record.MissedCount
	attempt.CreatedAt = record.CreatedAt
	attempt.UpdatedAt = record.UpdatedAt
	return attempt, nil
}

func hasActiveAttempt(items []DispatchAttempt) bool {
	for _, item := range items {
		if item.DispatchStatus == DispatchStatusDispatched && IsActiveDownstreamStatus(item.DownstreamStatus) {
			return true
		}
	}
	return false
}

func mapRunStatus(status runtime.RunStatus) DownstreamStatus {
	switch status {
	case runtime.RunStatusCompleted:
		return DownstreamStatusCompleted
	case runtime.RunStatusFailed:
		return DownstreamStatusFailed
	case runtime.RunStatusCancelled:
		return DownstreamStatusCancelled
	default:
		return DownstreamStatusRunning
	}
}

func mapWorkflowStatus(status orchestration.WorkflowStatus) DownstreamStatus {
	switch status {
	case orchestration.WorkflowStatusCompleted:
		return DownstreamStatusCompleted
	case orchestration.WorkflowStatusPlanningFailed, orchestration.WorkflowStatusFailed, orchestration.WorkflowStatusPartialFailed, orchestration.WorkflowStatusBlocked:
		return DownstreamStatusFailed
	case orchestration.WorkflowStatusCancelled:
		return DownstreamStatusCancelled
	case orchestration.WorkflowStatusInterrupted:
		return DownstreamStatusInterrupted
	default:
		return DownstreamStatusRunning
	}
}

func isTerminalDownstream(status DownstreamStatus) bool {
	switch status {
	case DownstreamStatusCompleted, DownstreamStatusFailed, DownstreamStatusCancelled, DownstreamStatusInterrupted:
		return true
	default:
		return false
	}
}

func targetSummary(target Target) string {
	switch target.Kind {
	case TargetKindWorkflow:
		if target.Workflow != nil {
			return firstNonEmpty(target.Workflow.WorkflowGoal, target.Workflow.RunGoal, target.Workflow.Entrypoint)
		}
	case TargetKindRun:
		if target.Run != nil {
			return firstNonEmpty(target.Run.Goal, target.Run.Entrypoint)
		}
	}
	return string(target.Kind)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func newScheduleID() string  { return "sched_" + newID() }
func newTargetRefID() string { return "sched_target_" + newID() }
func newAttemptID() string   { return "sched_attempt_" + newID() }

func newID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(buf)
}
