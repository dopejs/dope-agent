package api

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func mailOperationFixture(id string) mail.Operation {
	now := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)
	completed := now.Add(time.Second)
	return mail.Operation{
		OperationID:      id,
		OperationClass:   mail.OperationClassSendMessage,
		Status:           mail.OperationStatusCompleted,
		ResultMode:       mail.ResultModeSent,
		SendPath:         mail.SendPathDirect,
		IntegrationID:    "mail-a",
		MailAccountID:    "mail_acct_mail-a",
		EnvironmentScope: "test",
		ThreadID:         "thread_seed",
		MessageID:        "msg_seed",
		CreatedAt:        now,
		UpdatedAt:        completed,
		CompletedAt:      &completed,
	}
}

func TestProjectToolCallMailSummariesFromStore(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	operation := mailOperationFixture("mail_op_tool")
	operation.RunID = "run_1"
	operation.StepID = "step_1"
	operation.ToolCallID = "tool_call_1"
	if err := sqliteStore.UpsertMailOperation(context.Background(), operation); err != nil {
		t.Fatalf("UpsertMailOperation returned error: %v", err)
	}

	toolCall, err := projectToolCallMailSummaries(events.WithEnvironmentScope(context.Background(), "test"), sqliteStore, runtime.ToolCall{
		ToolCallID: "tool_call_1",
		RunID:      "run_1",
		StepID:     "step_1",
	})
	if err != nil {
		t.Fatalf("projectToolCallMailSummaries returned error: %v", err)
	}
	if len(toolCall.MailOperationSummaries) != 1 || toolCall.MailOperationSummaries[0].OperationID != operation.OperationID {
		t.Fatalf("expected projected mail operation summary, got %+v", toolCall.MailOperationSummaries)
	}
}

func TestProjectWorkflowAndScheduleMailSummariesFromStore(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	workflowOp := mailOperationFixture("mail_op_workflow")
	workflowOp.WorkflowID = "wf_1"
	workflowOp.WorkflowStepID = "wfstep_1"
	workflowOp.RunID = "run_1"
	if err := sqliteStore.UpsertMailOperation(context.Background(), workflowOp); err != nil {
		t.Fatalf("UpsertMailOperation(workflow) returned error: %v", err)
	}

	scheduleOp := mailOperationFixture("mail_op_schedule")
	scheduleOp.ScheduleID = "sched_1"
	scheduleOp.ScheduleAttemptID = "sched_attempt_1"
	if err := sqliteStore.UpsertMailOperation(context.Background(), scheduleOp); err != nil {
		t.Fatalf("UpsertMailOperation(schedule) returned error: %v", err)
	}

	workflow, err := projectWorkflowMailSummaries(context.Background(), sqliteStore, orchestration.Workflow{
		WorkflowID:       "wf_1",
		RunID:            "run_1",
		EnvironmentScope: "test",
		Steps:            []orchestration.WorkflowStep{{WorkflowStepID: "wfstep_1"}},
	})
	if err != nil {
		t.Fatalf("projectWorkflowMailSummaries returned error: %v", err)
	}
	if len(workflow.Steps[0].MailOperationSummaries) != 1 || workflow.Steps[0].MailOperationSummaries[0].OperationID != workflowOp.OperationID {
		t.Fatalf("expected workflow step mail summary, got %+v", workflow.Steps[0].MailOperationSummaries)
	}

	schedule, err := projectScheduleMailSummaries(context.Background(), sqliteStore, scheduler.Schedule{
		ScheduleID:       "sched_1",
		EnvironmentScope: "test",
		Attempts:         []scheduler.DispatchAttempt{{AttemptID: "sched_attempt_1"}},
	})
	if err != nil {
		t.Fatalf("projectScheduleMailSummaries returned error: %v", err)
	}
	if len(schedule.Attempts[0].MailOperationSummaries) != 1 || schedule.Attempts[0].MailOperationSummaries[0].OperationID != scheduleOp.OperationID {
		t.Fatalf("expected schedule attempt mail summary, got %+v", schedule.Attempts[0].MailOperationSummaries)
	}
}

func TestProjectDeliveryOutcomeMailLinkageFromStore(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	operation := mailOperationFixture("mail_op_delivery")
	operation.DeliveryID = "delivery_1"
	operation.WorkflowID = "wf_1"
	if err := sqliteStore.UpsertMailOperation(context.Background(), operation); err != nil {
		t.Fatalf("UpsertMailOperation returned error: %v", err)
	}

	outcome, err := projectDeliveryOutcomeMailLinkage(context.Background(), sqliteStore, delivery.DeliveryOutcome{
		DeliveryID:       "delivery_1",
		EnvironmentScope: "test",
		WorkflowID:       "wf_1",
	})
	if err != nil {
		t.Fatalf("projectDeliveryOutcomeMailLinkage returned error: %v", err)
	}
	if len(outcome.MailOperationIDs) != 1 || outcome.MailOperationIDs[0] != operation.OperationID {
		t.Fatalf("expected mail operation ids, got %+v", outcome.MailOperationIDs)
	}
	if len(outcome.MailOperationSummaries) != 1 || outcome.MailOperationSummaries[0].OperationID != operation.OperationID {
		t.Fatalf("expected mail operation summaries, got %+v", outcome.MailOperationSummaries)
	}
}
