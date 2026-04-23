package api

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func calendarOperationFixture(id string) calendar.Operation {
	now := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)
	completed := now.Add(time.Second)
	return calendar.Operation{
		OperationID:       id,
		OperationClass:    calendar.OperationClassListEvents,
		Status:            calendar.OperationStatusCompleted,
		IntegrationID:     "calendar-a",
		CalendarAccountID: "acct_calendar-a",
		EnvironmentScope:  "test",
		CalendarRef:       "primary",
		TimezoneUsed:      "America/Los_Angeles",
		ExternalEventID:   "evt_ext_1",
		CreatedAt:         now,
		UpdatedAt:         completed,
		CompletedAt:       &completed,
	}
}

func TestProjectToolCallCalendarSummariesFromStore(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	operation := calendarOperationFixture("calendar_op_tool")
	operation.RunID = "run_1"
	operation.StepID = "step_1"
	operation.ToolCallID = "tool_call_1"
	if err := sqliteStore.UpsertCalendarOperation(context.Background(), operation); err != nil {
		t.Fatalf("UpsertCalendarOperation returned error: %v", err)
	}

	toolCall, err := projectToolCallCalendarSummaries(events.WithEnvironmentScope(context.Background(), "test"), sqliteStore, runtime.ToolCall{
		ToolCallID: "tool_call_1",
		RunID:      "run_1",
		StepID:     "step_1",
	})
	if err != nil {
		t.Fatalf("projectToolCallCalendarSummaries returned error: %v", err)
	}
	if len(toolCall.CalendarOperationSummaries) != 1 || toolCall.CalendarOperationSummaries[0].OperationID != operation.OperationID {
		t.Fatalf("expected projected calendar operation summary, got %+v", toolCall.CalendarOperationSummaries)
	}
}

func TestProjectWorkflowAndScheduleCalendarSummariesFromStore(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	workflowOp := calendarOperationFixture("calendar_op_workflow")
	workflowOp.WorkflowID = "wf_1"
	workflowOp.WorkflowStepID = "wfstep_1"
	workflowOp.RunID = "run_1"
	if err := sqliteStore.UpsertCalendarOperation(context.Background(), workflowOp); err != nil {
		t.Fatalf("UpsertCalendarOperation(workflow) returned error: %v", err)
	}

	scheduleOp := calendarOperationFixture("calendar_op_schedule")
	scheduleOp.ScheduleID = "sched_1"
	scheduleOp.ScheduleAttemptID = "sched_attempt_1"
	if err := sqliteStore.UpsertCalendarOperation(context.Background(), scheduleOp); err != nil {
		t.Fatalf("UpsertCalendarOperation(schedule) returned error: %v", err)
	}

	workflow, err := projectWorkflowCalendarSummaries(context.Background(), sqliteStore, orchestration.Workflow{
		WorkflowID:       "wf_1",
		RunID:            "run_1",
		EnvironmentScope: "test",
		Steps:            []orchestration.WorkflowStep{{WorkflowStepID: "wfstep_1"}},
	})
	if err != nil {
		t.Fatalf("projectWorkflowCalendarSummaries returned error: %v", err)
	}
	if len(workflow.Steps[0].CalendarOperationSummaries) != 1 || workflow.Steps[0].CalendarOperationSummaries[0].OperationID != workflowOp.OperationID {
		t.Fatalf("expected workflow step summary, got %+v", workflow.Steps[0].CalendarOperationSummaries)
	}

	schedule, err := projectScheduleCalendarSummaries(context.Background(), sqliteStore, scheduler.Schedule{
		ScheduleID:       "sched_1",
		EnvironmentScope: "test",
		Attempts:         []scheduler.DispatchAttempt{{AttemptID: "sched_attempt_1"}},
	})
	if err != nil {
		t.Fatalf("projectScheduleCalendarSummaries returned error: %v", err)
	}
	if len(schedule.Attempts[0].CalendarOperationSummaries) != 1 || schedule.Attempts[0].CalendarOperationSummaries[0].OperationID != scheduleOp.OperationID {
		t.Fatalf("expected schedule attempt summary, got %+v", schedule.Attempts[0].CalendarOperationSummaries)
	}
}

func TestProjectDeliveryOutcomeCalendarLinkageFromStore(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	operation := calendarOperationFixture("calendar_op_delivery")
	operation.DeliveryID = "delivery_1"
	operation.WorkflowID = "wf_1"
	if err := sqliteStore.UpsertCalendarOperation(context.Background(), operation); err != nil {
		t.Fatalf("UpsertCalendarOperation returned error: %v", err)
	}

	outcome, err := projectDeliveryOutcomeCalendarLinkage(context.Background(), sqliteStore, delivery.DeliveryOutcome{
		DeliveryID:       "delivery_1",
		EnvironmentScope: "test",
		WorkflowID:       "wf_1",
	})
	if err != nil {
		t.Fatalf("projectDeliveryOutcomeCalendarLinkage returned error: %v", err)
	}
	if len(outcome.CalendarOperationIDs) != 1 || outcome.CalendarOperationIDs[0] != operation.OperationID {
		t.Fatalf("expected calendar operation ids, got %+v", outcome.CalendarOperationIDs)
	}
	if len(outcome.CalendarOperationSummaries) != 1 || outcome.CalendarOperationSummaries[0].OperationID != operation.OperationID {
		t.Fatalf("expected calendar operation summaries, got %+v", outcome.CalendarOperationSummaries)
	}
}
