package routine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/managerdoc"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
)

const docKindRoutine = "routine"

var (
	ErrRoutineNotFound  = errors.New("routine not found")
	ErrInvalidRoutine   = errors.New("routine definition is invalid")
	ErrRoutineCancelled = errors.New("routine is cancelled")
)

// Scheduler is the subset of the scheduler the routine builder compiles to. The concrete
// *scheduler.Scheduler satisfies it; tests use a fake.
type Scheduler interface {
	Create(ctx context.Context, input scheduler.CreateInput) (scheduler.Schedule, error)
	Pause(ctx context.Context, scheduleID string) (scheduler.Schedule, bool, error)
	Resume(ctx context.Context, scheduleID string) (scheduler.Schedule, bool, error)
	Cancel(ctx context.Context, scheduleID string) (scheduler.Schedule, bool, error)
	Get(ctx context.Context, scheduleID string) (scheduler.Schedule, bool, error)
}

// Manager owns routines and compiles them to the scheduler/workflow plane. Routines are stored
// in-memory with Restore for this slice; the compiled schedules (and their attempt evidence)
// persist in the scheduler's own store.
type Manager struct {
	mu       sync.RWMutex
	env      string
	sched    Scheduler
	docs     managerdoc.Store
	routines map[string]Routine
}

func NewManager(environmentScope string, sched Scheduler) *Manager {
	return &Manager{env: strings.TrimSpace(environmentScope), sched: sched, routines: make(map[string]Routine)}
}

// WithStore installs durable persistence for routines and returns the manager.
func (m *Manager) WithStore(s managerdoc.Store) *Manager {
	m.docs = s
	return m
}

// Restore reloads routines from an in-memory slice.
func (m *Manager) Restore(routines []Routine) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.routines = make(map[string]Routine, len(routines))
	for _, r := range routines {
		m.routines[r.RoutineID] = r
	}
}

// LoadFromStore reloads persisted routines from the document store on startup.
func (m *Manager) LoadFromStore(ctx context.Context) error {
	routines, err := managerdoc.List[Routine](ctx, m.docs, docKindRoutine)
	if err != nil {
		return err
	}
	m.Restore(routines)
	return nil
}

// Preview compiles a definition without activating it and returns the schedule/workflow/approval/
// delivery/quota expectations to confirm before activation (FR-004).
func (m *Manager) Preview(def Definition) (Preview, error) {
	if err := validateDefinition(def); err != nil {
		return Preview{}, err
	}
	kind := "recurring"
	if def.Trigger.Kind == TriggerKindOnce {
		kind = "one_time"
	}
	approval := strings.TrimSpace(def.ApprovalExpectation)
	if approval == "" {
		approval = "ask"
	}
	return Preview{
		ScheduleKind:         kind,
		TriggerSummary:       triggerSummary(def.Trigger),
		WorkflowSummary:      strings.TrimSpace(def.Workflow.Goal),
		ApprovalExpectation:  approval,
		DeliveryPreferenceID: strings.TrimSpace(def.DeliveryPreferenceID),
		RetrySummary:         fmt.Sprintf("max %d retries", maxRetries(def)),
	}, nil
}

// Create validates a definition, compiles it to a schedule, and stores the routine at version 1.
func (m *Manager) Create(ctx context.Context, def Definition) (Routine, error) {
	if err := validateDefinition(def); err != nil {
		return Routine{}, err
	}
	schedule, err := m.sched.Create(ctx, compile(def))
	if err != nil {
		return Routine{}, fmt.Errorf("compile routine to schedule: %w", err)
	}
	now := time.Now().UTC()
	routine := Routine{
		RoutineID:         newID("routine"),
		EnvironmentScope:  m.env,
		Name:              strings.TrimSpace(def.Name),
		State:             StateActive,
		CurrentVersion:    1,
		CurrentScheduleID: schedule.ScheduleID,
		Definition:        def,
		Versions:          []Version{{Version: 1, Definition: def, ScheduleID: schedule.ScheduleID, CreatedAt: now}},
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	m.store(routine)
	return routine, nil
}

func (m *Manager) Get(routineID string) (Routine, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.routines[strings.TrimSpace(routineID)]
	return r, ok
}

func (m *Manager) List() []Routine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Routine, 0, len(m.routines))
	for _, r := range m.routines {
		out = append(out, r)
	}
	return out
}

// Update creates a new routine version. It compiles a new schedule and cancels the previous one;
// the prior version keeps its schedule id so its execution evidence is preserved (FR-003).
func (m *Manager) Update(ctx context.Context, routineID string, def Definition) (Routine, error) {
	routine, ok := m.Get(routineID)
	if !ok {
		return Routine{}, ErrRoutineNotFound
	}
	if routine.State == StateCancelled {
		return Routine{}, ErrRoutineCancelled
	}
	if err := validateDefinition(def); err != nil {
		return Routine{}, err
	}
	schedule, err := m.sched.Create(ctx, compile(def))
	if err != nil {
		return Routine{}, fmt.Errorf("compile routine to schedule: %w", err)
	}
	if routine.CurrentScheduleID != "" {
		_, _, _ = m.sched.Cancel(ctx, routine.CurrentScheduleID) // prior schedule + its attempts remain as evidence
	}
	now := time.Now().UTC()
	routine.CurrentVersion++
	routine.Definition = def
	routine.Name = strings.TrimSpace(def.Name)
	routine.CurrentScheduleID = schedule.ScheduleID
	routine.UpdatedAt = now
	if routine.State == StatePaused {
		// A paused routine that is edited recompiles paused.
		_, _, _ = m.sched.Pause(ctx, schedule.ScheduleID)
	}
	routine.Versions = append(routine.Versions, Version{Version: routine.CurrentVersion, Definition: def, ScheduleID: schedule.ScheduleID, CreatedAt: now})
	m.store(routine)
	return routine, nil
}

func (m *Manager) Pause(ctx context.Context, routineID string) (Routine, error) {
	return m.transition(ctx, routineID, StatePaused, func(id string) { _, _, _ = m.sched.Pause(ctx, id) })
}

func (m *Manager) Resume(ctx context.Context, routineID string) (Routine, error) {
	return m.transition(ctx, routineID, StateActive, func(id string) { _, _, _ = m.sched.Resume(ctx, id) })
}

func (m *Manager) Cancel(ctx context.Context, routineID string) (Routine, error) {
	return m.transition(ctx, routineID, StateCancelled, func(id string) { _, _, _ = m.sched.Cancel(ctx, id) })
}

// Repair re-creates the routine's compiled schedule when it has gone missing (e.g. external
// cancellation), restoring the active routine to a working state without rewriting versions.
func (m *Manager) Repair(ctx context.Context, routineID string) (Routine, error) {
	routine, ok := m.Get(routineID)
	if !ok {
		return Routine{}, ErrRoutineNotFound
	}
	if routine.State == StateCancelled {
		return Routine{}, ErrRoutineCancelled
	}
	if routine.CurrentScheduleID != "" {
		if _, exists, _ := m.sched.Get(ctx, routine.CurrentScheduleID); exists {
			return routine, nil // healthy; nothing to repair
		}
	}
	schedule, err := m.sched.Create(ctx, compile(routine.Definition))
	if err != nil {
		return Routine{}, fmt.Errorf("repair routine schedule: %w", err)
	}
	if routine.State == StatePaused {
		_, _, _ = m.sched.Pause(ctx, schedule.ScheduleID)
	}
	now := time.Now().UTC()
	routine.CurrentScheduleID = schedule.ScheduleID
	routine.UpdatedAt = now
	// Reflect the repaired schedule id on the current version.
	if n := len(routine.Versions); n > 0 {
		routine.Versions[n-1].ScheduleID = schedule.ScheduleID
	}
	m.store(routine)
	return routine, nil
}

func (m *Manager) transition(ctx context.Context, routineID string, state State, schedAction func(string)) (Routine, error) {
	routine, ok := m.Get(routineID)
	if !ok {
		return Routine{}, ErrRoutineNotFound
	}
	if routine.State == StateCancelled {
		return Routine{}, ErrRoutineCancelled
	}
	if routine.CurrentScheduleID != "" {
		schedAction(routine.CurrentScheduleID)
	}
	routine.State = state
	routine.UpdatedAt = time.Now().UTC()
	m.store(routine)
	return routine, nil
}

func (m *Manager) store(routine Routine) {
	m.mu.Lock()
	m.routines[routine.RoutineID] = routine
	m.mu.Unlock()
	_ = managerdoc.Put(context.Background(), m.docs, docKindRoutine, routine.RoutineID, m.env, "", routine)
}

// compile maps a routine definition onto an existing scheduler create input (workflow target).
func compile(def Definition) scheduler.CreateInput {
	trigger := scheduler.Trigger{Timezone: def.Trigger.Timezone}
	if def.Trigger.Kind == TriggerKindOnce {
		trigger.Kind = scheduler.TriggerKindOnce
		trigger.FireAt = def.Trigger.FireAt
	} else {
		trigger.Kind = scheduler.TriggerKindCron
		trigger.CronExpr = def.Trigger.CronExpr
	}
	entrypoint := strings.TrimSpace(def.Workflow.Entrypoint)
	if entrypoint == "" {
		entrypoint = "operator"
	}
	return scheduler.CreateInput{
		Trigger: trigger,
		Target: scheduler.Target{
			Kind:     scheduler.TargetKindWorkflow,
			Active:   true,
			Workflow: &scheduler.WorkflowTarget{Entrypoint: entrypoint, WorkflowGoal: strings.TrimSpace(def.Workflow.Goal)},
			Summary:  strings.TrimSpace(def.Name),
		},
		RetryPolicy: scheduler.RetryPolicy{MaxRetries: maxRetries(def), BackoffKind: scheduler.RetryBackoffFixed, BaseDelaySeconds: 5, MaxDelaySeconds: 5},
	}
}

func validateDefinition(def Definition) error {
	if strings.TrimSpace(def.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidRoutine)
	}
	if strings.TrimSpace(def.Workflow.Goal) == "" {
		return fmt.Errorf("%w: workflow goal is required", ErrInvalidRoutine)
	}
	switch def.Trigger.Kind {
	case TriggerKindCron:
		if strings.TrimSpace(def.Trigger.CronExpr) == "" {
			return fmt.Errorf("%w: cron trigger requires a cron expression", ErrInvalidRoutine)
		}
	case TriggerKindOnce:
		if def.Trigger.FireAt == nil {
			return fmt.Errorf("%w: once trigger requires a fire time", ErrInvalidRoutine)
		}
	default:
		return fmt.Errorf("%w: unsupported trigger kind", ErrInvalidRoutine)
	}
	return nil
}

func maxRetries(def Definition) int {
	if def.MaxRetries > 0 {
		return def.MaxRetries
	}
	return 1
}

func triggerSummary(t Trigger) string {
	if t.Kind == TriggerKindOnce && t.FireAt != nil {
		return "once at " + t.FireAt.UTC().Format(time.RFC3339)
	}
	return "cron " + strings.TrimSpace(t.CronExpr)
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_fallback"
	}
	return prefix + "_" + hex.EncodeToString(buf)
}
