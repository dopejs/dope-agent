package checkpoints

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestManagerSavesAndRestoresRunCheckpoint(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	original := runtime.NewManager()
	run, err := original.CreateRun(runtime.CreateRunInput{
		Entrypoint: "chat",
		Goal:       "resume after reboot",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := original.CreateStep(run.RunID, runtime.CreateStepInput{
		Title: "call a tool",
		Kind:  "task",
	})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	if _, _, err := original.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusPlanning,
	}); err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun returned error: %v", err)
	}
	toolCall, err := original.CreateToolCall(run.RunID, step.StepID, runtime.CreateToolCallInput{
		ToolName: "shell",
	})
	if err != nil {
		t.Fatalf("CreateToolCall returned error: %v", err)
	}
	if _, err := original.CompleteToolCall(run.RunID, step.StepID, toolCall.ToolCallID, runtime.CompleteToolCallInput{}); err != nil {
		t.Fatalf("CompleteToolCall returned error: %v", err)
	}

	manager := NewManager(sqliteStore, original)
	if err := manager.SaveRunCheckpoint(context.Background(), run.RunID); err != nil {
		t.Fatalf("SaveRunCheckpoint returned error: %v", err)
	}

	restoredRuntime := runtime.NewManager()
	restoreManager := NewManager(sqliteStore, restoredRuntime)
	stats, err := restoreManager.Restore(context.Background())
	if err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	if stats.RunCount != 1 {
		t.Fatalf("expected 1 restored run, got %d", stats.RunCount)
	}

	gotRun, ok := restoredRuntime.GetRun(run.RunID)
	if !ok {
		t.Fatal("expected restored run")
	}
	if gotRun.Status != runtime.RunStatusRunning {
		t.Fatalf("expected restored run status running, got %s", gotRun.Status)
	}

	gotStep, ok := restoredRuntime.GetStep(run.RunID, step.StepID)
	if !ok {
		t.Fatal("expected restored step")
	}
	if gotStep.Status != runtime.StepStatusPlanning {
		t.Fatalf("expected restored step status planning, got %s", gotStep.Status)
	}

	gotToolCall, ok := restoredRuntime.GetToolCall(run.RunID, step.StepID, toolCall.ToolCallID)
	if !ok {
		t.Fatal("expected restored tool call")
	}
	if gotToolCall.Status != runtime.ToolCallStatusCompleted {
		t.Fatalf("expected restored tool call status completed, got %s", gotToolCall.Status)
	}
}
