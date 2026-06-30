package routine

import (
	"context"
	"errors"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
)

// fakeScheduler records compiled schedules and lifecycle calls.
type fakeScheduler struct {
	seq       int
	created   []scheduler.CreateInput
	paused    []string
	resumed   []string
	cancelled []string
	missing   map[string]bool
}

func (f *fakeScheduler) Create(ctx context.Context, in scheduler.CreateInput) (scheduler.Schedule, error) {
	f.seq++
	f.created = append(f.created, in)
	id := "sched_" + string(rune('a'+f.seq-1))
	return scheduler.Schedule{ScheduleID: id}, nil
}
func (f *fakeScheduler) Pause(ctx context.Context, id string) (scheduler.Schedule, bool, error) {
	f.paused = append(f.paused, id)
	return scheduler.Schedule{ScheduleID: id}, true, nil
}
func (f *fakeScheduler) Resume(ctx context.Context, id string) (scheduler.Schedule, bool, error) {
	f.resumed = append(f.resumed, id)
	return scheduler.Schedule{ScheduleID: id}, true, nil
}
func (f *fakeScheduler) Cancel(ctx context.Context, id string) (scheduler.Schedule, bool, error) {
	f.cancelled = append(f.cancelled, id)
	return scheduler.Schedule{ScheduleID: id}, true, nil
}
func (f *fakeScheduler) Get(ctx context.Context, id string) (scheduler.Schedule, bool, error) {
	if f.missing[id] {
		return scheduler.Schedule{}, false, nil
	}
	return scheduler.Schedule{ScheduleID: id}, true, nil
}

func dailyDef() Definition {
	return Definition{
		Name:                "Daily summary",
		Trigger:             Trigger{Kind: TriggerKindCron, CronExpr: "0 8 * * *", Timezone: "UTC"},
		Workflow:            Workflow{Goal: "summarize my day"},
		ApprovalExpectation: "ask",
	}
}

// FR-001/FR-002, US1: create compiles to a workflow schedule and stores version 1.
func TestRoutineCreateCompiles(t *testing.T) {
	f := &fakeScheduler{}
	m := NewManager("test", f)
	r, err := m.Create(context.Background(), dailyDef())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if r.State != StateActive || r.CurrentVersion != 1 || r.CurrentScheduleID == "" {
		t.Fatalf("routine not active/compiled: %+v", r)
	}
	if len(f.created) != 1 || f.created[0].Target.Kind != scheduler.TargetKindWorkflow || f.created[0].Target.Workflow.WorkflowGoal != "summarize my day" {
		t.Fatalf("did not compile to a workflow target: %+v", f.created)
	}
	if f.created[0].Trigger.Kind != scheduler.TriggerKindCron || f.created[0].Trigger.CronExpr != "0 8 * * *" {
		t.Fatalf("trigger not compiled: %+v", f.created[0].Trigger)
	}
}

// FR-003, US2: editing creates a new version, cancels the prior schedule, and preserves the prior
// version's schedule id (its execution evidence).
func TestRoutineUpdatePreservesPriorEvidence(t *testing.T) {
	f := &fakeScheduler{}
	m := NewManager("test", f)
	r, _ := m.Create(context.Background(), dailyDef())
	priorScheduleID := r.CurrentScheduleID

	def2 := dailyDef()
	def2.Workflow.Goal = "summarize my day and inbox"
	updated, err := m.Update(context.Background(), r.RoutineID, def2)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.CurrentVersion != 2 || updated.CurrentScheduleID == priorScheduleID {
		t.Fatalf("update did not create a new compiled version: %+v", updated)
	}
	if len(f.cancelled) != 1 || f.cancelled[0] != priorScheduleID {
		t.Fatalf("prior schedule not cancelled: %+v", f.cancelled)
	}
	if updated.Versions[0].ScheduleID != priorScheduleID {
		t.Fatalf("prior version evidence (schedule id) was rewritten: %+v", updated.Versions[0])
	}
}

// US2: pause/resume/cancel drive the compiled schedule and routine state.
func TestRoutineLifecycle(t *testing.T) {
	f := &fakeScheduler{}
	m := NewManager("test", f)
	r, _ := m.Create(context.Background(), dailyDef())

	if paused, _ := m.Pause(context.Background(), r.RoutineID); paused.State != StatePaused {
		t.Fatal("pause did not set state")
	}
	if len(f.paused) != 1 {
		t.Fatalf("schedule not paused: %+v", f.paused)
	}
	if resumed, _ := m.Resume(context.Background(), r.RoutineID); resumed.State != StateActive {
		t.Fatal("resume did not set state")
	}
	cancelled, _ := m.Cancel(context.Background(), r.RoutineID)
	if cancelled.State != StateCancelled {
		t.Fatal("cancel did not set state")
	}
	if _, err := m.Pause(context.Background(), r.RoutineID); !errors.Is(err, ErrRoutineCancelled) {
		t.Fatalf("cancelled routine should reject further transitions, got %v", err)
	}
}

// US3: repair recreates a missing compiled schedule without rewriting versions.
func TestRoutineRepair(t *testing.T) {
	f := &fakeScheduler{}
	m := NewManager("test", f)
	r, _ := m.Create(context.Background(), dailyDef())
	f.missing = map[string]bool{r.CurrentScheduleID: true}

	repaired, err := m.Repair(context.Background(), r.RoutineID)
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if repaired.CurrentScheduleID == r.CurrentScheduleID {
		t.Fatalf("repair did not recreate the schedule: %+v", repaired)
	}
	if repaired.CurrentVersion != 1 {
		t.Fatalf("repair must not bump version: %+v", repaired)
	}
}

// FR-004: preview compiles without activating and reports expectations.
func TestRoutinePreviewAndValidation(t *testing.T) {
	f := &fakeScheduler{}
	m := NewManager("test", f)
	preview, err := m.Preview(dailyDef())
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if preview.ScheduleKind != "recurring" || preview.WorkflowSummary == "" || preview.ApprovalExpectation != "ask" {
		t.Fatalf("preview missing expectations: %+v", preview)
	}
	if len(f.created) != 0 {
		t.Fatal("preview must not activate (create) a schedule")
	}
	if _, err := m.Create(context.Background(), Definition{Name: "bad"}); !errors.Is(err, ErrInvalidRoutine) {
		t.Fatalf("invalid definition should be rejected, got %v", err)
	}
}
