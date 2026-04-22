package runtime

import (
	"errors"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

func TestCreateRun(t *testing.T) {
	manager := NewManager()

	run, err := manager.CreateRun(CreateRunInput{
		Entrypoint: "chat",
		Goal:       "help the user ship a task",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}

	if run.RunID == "" {
		t.Fatal("expected run ID to be set")
	}
	if run.Status != RunStatusQueued {
		t.Fatalf("expected queued status, got %s", run.Status)
	}
	if run.Entrypoint != "chat" {
		t.Fatalf("expected entrypoint chat, got %s", run.Entrypoint)
	}
}

func TestCreateRunRequiresEntrypoint(t *testing.T) {
	manager := NewManager()

	_, err := manager.CreateRun(CreateRunInput{})
	if err == nil {
		t.Fatal("expected error when entrypoint is missing")
	}
}

func TestListAndGetRuns(t *testing.T) {
	manager := NewManager()

	first, err := manager.CreateRun(CreateRunInput{
		Entrypoint: "chat",
		Goal:       "first",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	_, err = manager.CreateRun(CreateRunInput{
		Entrypoint: "cron",
		Goal:       "second",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}

	runs := manager.ListRuns()
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}

	got, ok := manager.GetRun(first.RunID)
	if !ok {
		t.Fatal("expected GetRun to find first run")
	}
	if got.Goal != "first" {
		t.Fatalf("expected goal first, got %s", got.Goal)
	}
}

func TestCreateListAndGetSteps(t *testing.T) {
	manager := NewManager()

	run, err := manager.CreateRun(CreateRunInput{
		Entrypoint: "chat",
		Goal:       "step test",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}

	step, err := manager.CreateStep(run.RunID, CreateStepInput{
		Title: "plan the task",
		Kind:  "task",
		Input: map[string]any{"goal": "step test"},
	})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	if step.Status != StepStatusQueued {
		t.Fatalf("expected queued step status, got %s", step.Status)
	}

	steps, err := manager.ListSteps(run.RunID)
	if err != nil {
		t.Fatalf("ListSteps returned error: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}

	got, ok := manager.GetStep(run.RunID, step.StepID)
	if !ok {
		t.Fatal("expected GetStep to find step")
	}
	if got.Title != "plan the task" {
		t.Fatalf("expected step title plan the task, got %s", got.Title)
	}
}

func TestCreateStepRequiresRunAndTitle(t *testing.T) {
	manager := NewManager()

	_, err := manager.CreateStep("run_missing", CreateStepInput{
		Title: "bad step",
	})
	if !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("expected ErrRunNotFound, got %v", err)
	}

	run, err := manager.CreateRun(CreateRunInput{
		Entrypoint: "chat",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}

	_, err = manager.CreateStep(run.RunID, CreateStepInput{})
	if !errors.Is(err, ErrTitleRequired) {
		t.Fatalf("expected ErrTitleRequired, got %v", err)
	}
}

func TestUpdateStepStatus(t *testing.T) {
	manager := NewManager()

	run, err := manager.CreateRun(CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, CreateStepInput{
		Title: "plan the task",
	})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	step, runUpdate, err := manager.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, UpdateStepStatusInput{
		Status: StepStatusPlanning,
	})
	if err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun returned error: %v", err)
	}
	if step.Status != StepStatusPlanning {
		t.Fatalf("expected planning status, got %s", step.Status)
	}
	if runUpdate == nil || runUpdate.Status != RunStatusRunning {
		t.Fatalf("expected run status to change to running, got %+v", runUpdate)
	}

	step, runUpdate, err = manager.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, UpdateStepStatusInput{
		Status: StepStatusCallingModel,
	})
	if err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun returned error: %v", err)
	}
	if runUpdate != nil {
		t.Fatalf("expected no run status change, got %+v", runUpdate)
	}

	step, runUpdate, err = manager.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, UpdateStepStatusInput{
		Status: StepStatusCompleted,
		Output: map[string]any{"summary": "done"},
	})
	if err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun returned error: %v", err)
	}
	if step.Status != StepStatusCompleted {
		t.Fatalf("expected completed status, got %s", step.Status)
	}
	if step.Output == nil {
		t.Fatal("expected output to be set")
	}
	if runUpdate == nil || runUpdate.Status != RunStatusCompleted {
		t.Fatalf("expected run status to change to completed, got %+v", runUpdate)
	}
}

func TestUpdateStepStatusRejectsInvalidTransition(t *testing.T) {
	manager := NewManager()

	run, err := manager.CreateRun(CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, CreateStepInput{
		Title: "plan the task",
	})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	_, _, err = manager.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, UpdateStepStatusInput{
		Status: StepStatusCompleted,
	})
	if !errors.Is(err, ErrInvalidStepTransition) {
		t.Fatalf("expected ErrInvalidStepTransition, got %v", err)
	}
}

func TestCancelRunAndResumeRun(t *testing.T) {
	manager := NewManager()

	run, err := manager.CreateRun(CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	first, err := manager.CreateStep(run.RunID, CreateStepInput{Title: "first"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	second, err := manager.CreateStep(run.RunID, CreateStepInput{Title: "second"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	if _, _, err := manager.UpdateStepStatusAndReconcileRun(run.RunID, first.StepID, UpdateStepStatusInput{Status: StepStatusPlanning}); err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun returned error: %v", err)
	}
	if _, _, err := manager.UpdateStepStatusAndReconcileRun(run.RunID, second.StepID, UpdateStepStatusInput{Status: StepStatusPlanning}); err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun returned error: %v", err)
	}
	if _, _, err := manager.UpdateStepStatusAndReconcileRun(run.RunID, second.StepID, UpdateStepStatusInput{Status: StepStatusWaitingInput}); err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun returned error: %v", err)
	}

	cancelledRun, cancelledSteps, idempotent, err := manager.CancelRun(run.RunID)
	if err != nil {
		t.Fatalf("CancelRun returned error: %v", err)
	}
	if idempotent {
		t.Fatal("expected first cancel to apply changes")
	}
	if cancelledRun.Status != RunStatusCancelled {
		t.Fatalf("expected cancelled run status, got %s", cancelledRun.Status)
	}
	if len(cancelledSteps) != 2 {
		t.Fatalf("expected 2 cancelled steps, got %d", len(cancelledSteps))
	}

	cancelledRun, cancelledSteps, idempotent, err = manager.CancelRun(run.RunID)
	if err != nil {
		t.Fatalf("CancelRun second call returned error: %v", err)
	}
	if !idempotent {
		t.Fatal("expected second cancel to be idempotent")
	}
	if len(cancelledSteps) != 0 {
		t.Fatalf("expected no changed steps on idempotent cancel, got %d", len(cancelledSteps))
	}

	resumedRun, resumedSteps, idempotent, err := manager.ResumeRun(run.RunID)
	if err != nil {
		t.Fatalf("ResumeRun returned error: %v", err)
	}
	if idempotent {
		t.Fatal("expected first resume to apply changes")
	}
	if resumedRun.Status != RunStatusQueued {
		t.Fatalf("expected queued run after resume, got %s", resumedRun.Status)
	}
	if len(resumedSteps) != 2 {
		t.Fatalf("expected 2 resumed steps, got %d", len(resumedSteps))
	}
}

func TestCreateToolCallClonesIntegrationBindingsAndKeepsThemOptional(t *testing.T) {
	manager := NewManager()

	run, err := manager.CreateRun(CreateRunInput{Entrypoint: "chat", Goal: "integration binding clone"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, CreateStepInput{Title: "probe integration"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	inputBindings := []integrations.BindingSummary{{
		IntegrationID:         "calendar-a",
		DomainKind:            "calendar",
		DisplayName:           "Calendar A",
		AccountKey:            "acct_calendar",
		CanonicalDefault:      true,
		ReadinessAtInvocation: integrations.ReadinessStatusDegraded,
		BackendKind:           integrations.BackendKindFakeLocal,
		SecretResolution:      "resolved",
		EnvironmentScope:      "test",
	}}
	toolCall, err := manager.CreateToolCall(run.RunID, step.StepID, CreateToolCallInput{
		CapabilityID:        "integration_probe",
		ToolName:            "inspect",
		IntegrationBindings: inputBindings,
	})
	if err != nil {
		t.Fatalf("CreateToolCall returned error: %v", err)
	}

	inputBindings[0].DisplayName = "mutated outside runtime"
	if toolCall.IntegrationBindings[0].DisplayName != "Calendar A" {
		t.Fatalf("expected runtime tool call to clone integration bindings, got %+v", toolCall.IntegrationBindings)
	}

	plainToolCall, err := manager.CreateToolCall(run.RunID, step.StepID, CreateToolCallInput{
		CapabilityID: "docs",
		ToolName:     "lookup",
	})
	if err != nil {
		t.Fatalf("CreateToolCall(plain) returned error: %v", err)
	}
	if len(plainToolCall.IntegrationBindings) != 0 {
		t.Fatalf("expected non-integration tool call to omit bindings, got %+v", plainToolCall)
	}
}

func TestCancelStepReconcilesRunAndIdempotency(t *testing.T) {
	manager := NewManager()

	run, err := manager.CreateRun(CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	first, err := manager.CreateStep(run.RunID, CreateStepInput{Title: "first"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	second, err := manager.CreateStep(run.RunID, CreateStepInput{Title: "second"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	cancelledStep, runUpdate, idempotent, err := manager.CancelStep(run.RunID, first.StepID)
	if err != nil {
		t.Fatalf("CancelStep returned error: %v", err)
	}
	if idempotent {
		t.Fatal("expected first cancel step to apply changes")
	}
	if cancelledStep.Status != StepStatusCancelled {
		t.Fatalf("expected cancelled step, got %s", cancelledStep.Status)
	}
	if runUpdate != nil {
		t.Fatalf("expected no run update while run stays queued, got %+v", runUpdate)
	}

	cancelledStep, runUpdate, idempotent, err = manager.CancelStep(run.RunID, first.StepID)
	if err != nil {
		t.Fatalf("CancelStep idempotent call returned error: %v", err)
	}
	if !idempotent {
		t.Fatal("expected second cancel step to be idempotent")
	}
	if runUpdate != nil {
		t.Fatalf("expected no run update on idempotent cancel, got %+v", runUpdate)
	}

	_, runUpdate, _, err = manager.CancelStep(run.RunID, second.StepID)
	if err != nil {
		t.Fatalf("CancelStep second step returned error: %v", err)
	}
	if runUpdate == nil || runUpdate.Status != RunStatusCancelled {
		t.Fatalf("expected cancelled run after all steps cancelled, got %+v", runUpdate)
	}
}

func TestTerminalStateValidation(t *testing.T) {
	manager := NewManager()

	run, err := manager.CreateRun(CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, CreateStepInput{Title: "terminal"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	if _, _, err := manager.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, UpdateStepStatusInput{Status: StepStatusPlanning}); err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun returned error: %v", err)
	}
	if _, _, err := manager.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, UpdateStepStatusInput{Status: StepStatusCallingModel}); err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun returned error: %v", err)
	}
	if _, _, err := manager.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, UpdateStepStatusInput{Status: StepStatusCompleted}); err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun returned error: %v", err)
	}

	if _, _, err := manager.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, UpdateStepStatusInput{Status: StepStatusPlanning}); !errors.Is(err, ErrStepTerminal) {
		t.Fatalf("expected ErrStepTerminal after completed step, got %v", err)
	}
	if _, _, _, err := manager.ResumeRun(run.RunID); !errors.Is(err, ErrRunTerminal) {
		t.Fatalf("expected ErrRunTerminal when resuming completed run, got %v", err)
	}
}

func TestToolCallLifecycle(t *testing.T) {
	manager := NewManager()

	run, err := manager.CreateRun(CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, CreateStepInput{
		Title: "execute a tool",
	})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	toolCall, err := manager.CreateToolCall(run.RunID, step.StepID, CreateToolCallInput{
		CapabilityID: "shell",
		ToolName:     "shell",
		Input:        map[string]any{"cmd": "pwd"},
	})
	if err != nil {
		t.Fatalf("CreateToolCall returned error: %v", err)
	}
	if toolCall.Status != ToolCallStatusRequested {
		t.Fatalf("expected requested tool call status, got %s", toolCall.Status)
	}

	toolCalls, err := manager.ListToolCalls(run.RunID, step.StepID)
	if err != nil {
		t.Fatalf("ListToolCalls returned error: %v", err)
	}
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}

	got, ok := manager.GetToolCall(run.RunID, step.StepID, toolCall.ToolCallID)
	if !ok {
		t.Fatal("expected GetToolCall to find tool call")
	}
	if got.ToolName != "shell" {
		t.Fatalf("expected tool name shell, got %s", got.ToolName)
	}
	if got.CapabilityID != "shell" {
		t.Fatalf("expected capability id shell, got %s", got.CapabilityID)
	}

	toolCall, err = manager.CompleteToolCall(run.RunID, step.StepID, toolCall.ToolCallID, CompleteToolCallInput{
		Output: map[string]any{"exitCode": 0},
	})
	if err != nil {
		t.Fatalf("CompleteToolCall returned error: %v", err)
	}
	if toolCall.Status != ToolCallStatusCompleted {
		t.Fatalf("expected completed status, got %s", toolCall.Status)
	}
}

func TestToolCallFailAndInvalidTransition(t *testing.T) {
	manager := NewManager()

	run, err := manager.CreateRun(CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, CreateStepInput{
		Title: "execute a tool",
	})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	toolCall, err := manager.CreateToolCall(run.RunID, step.StepID, CreateToolCallInput{
		CapabilityID: "shell",
		ToolName:     "shell",
	})
	if err != nil {
		t.Fatalf("CreateToolCall returned error: %v", err)
	}

	toolCall, err = manager.FailToolCall(run.RunID, step.StepID, toolCall.ToolCallID, FailToolCallInput{
		Error: "command failed",
	})
	if err != nil {
		t.Fatalf("FailToolCall returned error: %v", err)
	}
	if toolCall.Status != ToolCallStatusFailed {
		t.Fatalf("expected failed status, got %s", toolCall.Status)
	}

	_, err = manager.CompleteToolCall(run.RunID, step.StepID, toolCall.ToolCallID, CompleteToolCallInput{
		Output: map[string]any{"exitCode": 0},
	})
	if !errors.Is(err, ErrInvalidToolCallStatus) {
		t.Fatalf("expected ErrInvalidToolCallStatus, got %v", err)
	}
}

func TestToolCallCancelAndDenyTransitions(t *testing.T) {
	manager := NewManager()

	run, err := manager.CreateRun(CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, CreateStepInput{Title: "execute a tool"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	cancelled, err := manager.CreateToolCall(run.RunID, step.StepID, CreateToolCallInput{
		CapabilityID: "shell",
		ToolName:     "shell",
	})
	if err != nil {
		t.Fatalf("CreateToolCall returned error: %v", err)
	}
	cancelled, err = manager.CancelToolCall(run.RunID, step.StepID, cancelled.ToolCallID, CancelToolCallInput{
		Error:        "daemon restarted",
		FailureClass: "cancelled",
	})
	if err != nil {
		t.Fatalf("CancelToolCall returned error: %v", err)
	}
	if cancelled.Status != ToolCallStatusCancelled {
		t.Fatalf("expected cancelled status, got %+v", cancelled)
	}

	denied, err := manager.CreateToolCall(run.RunID, step.StepID, CreateToolCallInput{
		CapabilityID: "exec",
		ToolName:     "exec",
	})
	if err != nil {
		t.Fatalf("CreateToolCall returned error: %v", err)
	}
	denied, err = manager.DenyToolCall(run.RunID, step.StepID, denied.ToolCallID, DenyToolCallInput{
		Error:        "policy denied",
		FailureClass: "policy_denied",
	})
	if err != nil {
		t.Fatalf("DenyToolCall returned error: %v", err)
	}
	if denied.Status != ToolCallStatusDenied {
		t.Fatalf("expected denied status, got %+v", denied)
	}
}

func TestToolCallPreservesSandboxLinkageAndFailureClass(t *testing.T) {
	manager := NewManager()

	run, err := manager.CreateRun(CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, CreateStepInput{Title: "execute docker skill"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	toolCall, err := manager.CreateToolCall(run.RunID, step.StepID, CreateToolCallInput{
		InvocationKind:     ToolCallInvocationKindSkill,
		SkillID:            "docker-skill",
		ToolName:           "docker-skill",
		SandboxExecutionID: "sandbox_exec_docker_1",
		Sandbox: map[string]any{
			"policyRecord": map[string]any{"policyRecordId": "policy_docker_1"},
		},
	})
	if err != nil {
		t.Fatalf("CreateToolCall returned error: %v", err)
	}
	if toolCall.InvocationKind != ToolCallInvocationKindSkill || toolCall.SkillID != "docker-skill" {
		t.Fatalf("expected skill invocation linkage, got %+v", toolCall)
	}
	if toolCall.SandboxExecutionID != "sandbox_exec_docker_1" {
		t.Fatalf("expected sandbox execution linkage, got %+v", toolCall)
	}

	toolCall, err = manager.FailToolCall(run.RunID, step.StepID, toolCall.ToolCallID, FailToolCallInput{
		Output:       map[string]any{"status": "unsupported", "backendKind": "docker", "mismatchReason": "backend_unavailable"},
		Error:        "docker backend is not available on this host",
		FailureClass: "backend_unavailable",
	})
	if err != nil {
		t.Fatalf("FailToolCall returned error: %v", err)
	}
	if toolCall.Status != ToolCallStatusFailed || toolCall.FailureClass != "backend_unavailable" {
		t.Fatalf("expected failed unsupported linkage, got %+v", toolCall)
	}
	if toolCall.SandboxExecutionID != "sandbox_exec_docker_1" {
		t.Fatalf("expected sandbox execution id to remain attached, got %+v", toolCall)
	}
}

func TestMCPToolCallPreservesInvocationProvenance(t *testing.T) {
	manager := NewManager()

	run, err := manager.CreateRun(CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, CreateStepInput{Title: "invoke mcp tool"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	toolCall, err := manager.CreateToolCall(run.RunID, step.StepID, CreateToolCallInput{
		InvocationKind:      ToolCallInvocationKindMCPTool,
		MCPServerID:         "filesystem-test",
		MCPServerName:       "Filesystem",
		MCPToolName:         "lookup",
		MCPTransportKind:    "stdio",
		MCPSessionID:        "session_1",
		AuthorizationResult: "allowed",
		ToolName:            "lookup",
		Input:               map[string]any{"query": "hello"},
	})
	if err != nil {
		t.Fatalf("CreateToolCall returned error: %v", err)
	}
	if toolCall.InvocationKind != ToolCallInvocationKindMCPTool || toolCall.MCPServerID != "filesystem-test" || toolCall.MCPToolName != "lookup" {
		t.Fatalf("expected mcp invocation provenance, got %+v", toolCall)
	}
	if _, err := manager.CompleteToolCall(run.RunID, step.StepID, toolCall.ToolCallID, CompleteToolCallInput{
		Output: map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}}},
	}); err != nil {
		t.Fatalf("CompleteToolCall returned error: %v", err)
	}
	persisted, ok := manager.GetToolCall(run.RunID, step.StepID, toolCall.ToolCallID)
	if !ok || persisted.AuthorizationResult != "allowed" || persisted.MCPSessionID != "session_1" {
		t.Fatalf("expected persisted mcp tool call linkage, got %+v", persisted)
	}
}

func TestSnapshotAndRestoreCheckpoint(t *testing.T) {
	manager := NewManager()

	run, err := manager.CreateRun(CreateRunInput{
		SessionID:  "session_1",
		Entrypoint: "chat",
		Goal:       "recover after restart",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, CreateStepInput{
		Title: "execute shell command",
		Kind:  "task",
		Input: map[string]any{"cmd": "pwd"},
	})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	step, _, err = manager.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, UpdateStepStatusInput{
		Status: StepStatusPlanning,
	})
	if err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun returned error: %v", err)
	}
	toolCall, err := manager.CreateToolCall(run.RunID, step.StepID, CreateToolCallInput{
		CapabilityID: "shell",
		ToolName:     "shell",
		Input:        map[string]any{"cmd": "pwd"},
	})
	if err != nil {
		t.Fatalf("CreateToolCall returned error: %v", err)
	}
	toolCall, err = manager.CompleteToolCall(run.RunID, step.StepID, toolCall.ToolCallID, CompleteToolCallInput{
		Output: map[string]any{"stdout": "/tmp"},
	})
	if err != nil {
		t.Fatalf("CompleteToolCall returned error: %v", err)
	}

	checkpoint, err := manager.SnapshotRun(run.RunID)
	if err != nil {
		t.Fatalf("SnapshotRun returned error: %v", err)
	}
	if checkpoint.Run.RunID != run.RunID {
		t.Fatalf("expected checkpoint run ID %s, got %s", run.RunID, checkpoint.Run.RunID)
	}
	if len(checkpoint.Steps) != 1 {
		t.Fatalf("expected 1 checkpoint step, got %d", len(checkpoint.Steps))
	}
	if len(checkpoint.ToolCalls) != 1 {
		t.Fatalf("expected 1 checkpoint tool call, got %d", len(checkpoint.ToolCalls))
	}

	restored := NewManager()
	restored.RestoreCheckpoints([]RunCheckpoint{checkpoint})

	gotRun, ok := restored.GetRun(run.RunID)
	if !ok {
		t.Fatal("expected restored run")
	}
	if gotRun.SessionID != run.SessionID {
		t.Fatalf("expected restored session ID %s, got %s", run.SessionID, gotRun.SessionID)
	}

	gotStep, ok := restored.GetStep(run.RunID, step.StepID)
	if !ok {
		t.Fatal("expected restored step")
	}
	if gotStep.Status != StepStatusPlanning {
		t.Fatalf("expected restored step status planning, got %s", gotStep.Status)
	}

	gotToolCall, ok := restored.GetToolCall(run.RunID, step.StepID, toolCall.ToolCallID)
	if !ok {
		t.Fatal("expected restored tool call")
	}
	if gotToolCall.Status != ToolCallStatusCompleted {
		t.Fatalf("expected restored tool call status completed, got %s", gotToolCall.Status)
	}
}
