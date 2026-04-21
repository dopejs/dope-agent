package orchestration_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/mcp"
	orch "github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/skills"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestOrchestrationMCPHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_ORCHESTRATION_MCP_HELPER") != "1" {
		return
	}
	reader := bufio.NewReader(os.Stdin)
	toolsPayload := os.Getenv("ORCHESTRATION_MCP_HELPER_TOOLS")
	if strings.TrimSpace(toolsPayload) == "" {
		toolsPayload = `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`
	}
	for {
		payload, err := readOrchestrationHelperFrame(reader)
		if err != nil {
			return
		}
		var req struct {
			ID     string `json:"id"`
			Method string `json:"method"`
		}
		if err := json.Unmarshal(payload, &req); err != nil {
			return
		}
		switch req.Method {
		case "initialize":
			writeOrchestrationHelperFrame(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "orchestration-test-mcp", "version": "1.0.0"}}})
		case "notifications/initialized":
		case "tools/list":
			var tools []map[string]any
			_ = json.Unmarshal([]byte(toolsPayload), &tools)
			writeOrchestrationHelperFrame(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"tools": tools}})
		case "tools/call":
			writeOrchestrationHelperFrame(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"content": []map[string]any{{"type": "text", "text": "lookup ok"}}}})
		default:
			writeOrchestrationHelperFrame(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32601, "message": "method not found"}})
		}
	}
}

func TestManagerBuildPlanGeneratesGoalDrivenWorkflowWithinTimingBudget(t *testing.T) {
	cfg, _, _, _, mcpManager := newOrchestrationMCPHarness(t)
	skillRegistry := newOrchestrationSkillRegistryForTest(t, cfg.DataDir, "exec-skill", "allow", "#!/bin/sh\nprintf 'ok %s' \"$1\"")
	run := runtime.Run{RunID: "run_plan_1", Goal: "Use MCP and a skill to complete a deterministic workflow."}

	startedAt := time.Now()
	workflow := orch.NewManager().Plan(cfg, run, orch.CreateWorkflowInput{}, nil, orchestrationSkillPlanningAdapter{registry: skillRegistry}, orchestrationMCPPlanningAdapter{manager: mcpManager})
	if elapsed := time.Since(startedAt); elapsed > 2*time.Second {
		t.Fatalf("expected planning to finish within 2s, took %s", elapsed)
	}
	if workflow.Status != orch.WorkflowStatusPlanned {
		t.Fatalf("expected planned workflow, got %+v", workflow)
	}
	if len(workflow.Steps) != 2 {
		t.Fatalf("expected mixed-family workflow, got %+v", workflow.Steps)
	}
	if workflow.Steps[0].ConsumerKind != string(runtime.ToolCallInvocationKindMCPTool) || workflow.Steps[1].ConsumerKind != string(runtime.ToolCallInvocationKindSkill) {
		t.Fatalf("expected MCP then skill workflow, got %+v", workflow.Steps)
	}
	if len(workflow.Dependencies) != 1 || len(workflow.Handoffs) != 1 {
		t.Fatalf("expected bounded graph and handoff truth, got deps=%+v handoffs=%+v", workflow.Dependencies, workflow.Handoffs)
	}
	if workflow.Steps[1].DependencyIDs[0] != workflow.Dependencies[0].DependencyID {
		t.Fatalf("expected dependency linkage on dependent step, got %+v", workflow.Steps[1])
	}
	if workflow.StartedAt != nil || workflow.Steps[0].RuntimeStepID != "" || workflow.Steps[0].ActiveToolCallID != "" {
		t.Fatalf("expected frozen inspect-only plan, got %+v", workflow)
	}
}

func TestManagerBuildPlanRecordsPlanningFailureTruth(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Environment: config.EnvironmentTest, DataDir: t.TempDir()}
	run := runtime.Run{RunID: "run_plan_fail", Goal: "No consumers available"}
	workflow := orch.NewManager().Plan(cfg, run, orch.CreateWorkflowInput{}, nil, nil, nil)
	if workflow.Status != orch.WorkflowStatusPlanningFailed {
		t.Fatalf("expected planning_failed workflow, got %+v", workflow)
	}
	if workflow.FailureSummary == "" || len(workflow.Steps) != 0 {
		t.Fatalf("expected planning failure truth without steps, got %+v", workflow)
	}
}

func TestManagerExecutionSchedulingRetryAndFrozenPlanTruth(t *testing.T) {
	t.Parallel()

	manager := orch.NewManager()
	workflow := newWorkflowFixture(
		"run_exec_1",
		workflowStepFixture{id: "wfstep_a", consumerKind: string(runtime.ToolCallInvocationKindSkill), consumerID: "exec-skill", maxAttempts: 2},
		workflowStepFixture{id: "wfstep_b", consumerKind: string(runtime.ToolCallInvocationKindSkill), consumerID: "exec-skill", maxAttempts: 1},
	)
	workflow = withSuccessDependency(workflow, "wfstep_a", "wfstep_b")
	workflow = withHandoff(workflow, "wfstep_a", "wfstep_b")
	graphShape := fmt.Sprintf("%d/%d/%d", len(workflow.Steps), len(workflow.Dependencies), len(workflow.Handoffs))

	startedAt := time.Now().UTC()
	workflow = manager.InitializeExecution(workflow, startedAt)
	if workflow.Status != orch.WorkflowStatusRunning || workflow.Steps[0].Status != orch.StepStatusReady || workflow.Steps[1].Status != orch.StepStatusWaitingDependency {
		t.Fatalf("expected initialized sequential workflow, got %+v", workflow)
	}
	if len(manager.DependenciesMissing(workflow, workflow.Steps[1])) != 1 {
		t.Fatalf("expected downstream dependency to remain unsatisfied, got %+v", workflow.Steps[1])
	}

	workflow = manager.StartStepAttempt(workflow, "wfstep_a", "step_runtime_a_1", startedAt.Add(time.Millisecond))
	workflow = manager.ApplyToolCallResult(workflow, runtime.ToolCall{
		WorkflowStepID: "wfstep_a",
		StepID:         "step_runtime_a_1",
		ToolCallID:     "tool_call_a_1",
		Status:         runtime.ToolCallStatusFailed,
		FailureClass:   "transient_error",
	}, "", "", startedAt.Add(2*time.Millisecond))
	assertRetryState(t, workflow, "wfstep_a", orch.StepStatusReady, 1, "")

	workflow = manager.StartStepAttempt(workflow, "wfstep_a", "step_runtime_a_2", startedAt.Add(3*time.Millisecond))
	workflow = manager.ApplyToolCallResult(workflow, runtime.ToolCall{
		WorkflowStepID: "wfstep_a",
		StepID:         "step_runtime_a_2",
		ToolCallID:     "tool_call_a_2",
		Status:         runtime.ToolCallStatusCompleted,
		Output:         map[string]any{"result": "ready"},
	}, "", "", startedAt.Add(4*time.Millisecond))
	if workflow.Steps[0].Status != orch.StepStatusCompleted || workflow.Handoffs[0].Status != orch.HandoffStatusAvailable {
		t.Fatalf("expected completed upstream step and visible handoff, got %+v", workflow)
	}

	readyStartedAt := time.Now()
	workflow, changed := manager.AdvanceReadySteps(workflow, startedAt.Add(5*time.Millisecond))
	if elapsed := time.Since(readyStartedAt); elapsed > 100*time.Millisecond {
		t.Fatalf("expected ready-step scheduling under 100ms, took %s", elapsed)
	}
	if !changed || workflow.Steps[1].Status != orch.StepStatusReady {
		t.Fatalf("expected dependent step to become ready, got %+v", workflow)
	}
	if got := fmt.Sprintf("%d/%d/%d", len(workflow.Steps), len(workflow.Dependencies), len(workflow.Handoffs)); got != graphShape {
		t.Fatalf("expected frozen plan graph %s, got %s", graphShape, got)
	}
}

func TestManagerMixedFamilyBlockedAndPartialFailureTruth(t *testing.T) {
	t.Parallel()

	manager := orch.NewManager()

	t.Run("consumer unavailable stays explicit", func(t *testing.T) {
		workflow := newWorkflowFixture(
			"run_mix_blocked",
			workflowStepFixture{id: "wfstep_mcp", consumerKind: string(runtime.ToolCallInvocationKindMCPTool), consumerID: "filesystem-test", maxAttempts: 1},
			workflowStepFixture{id: "wfstep_skill", consumerKind: string(runtime.ToolCallInvocationKindSkill), consumerID: "exec-skill", maxAttempts: 1},
		)
		workflow = withSuccessDependency(workflow, "wfstep_mcp", "wfstep_skill")
		workflow = withHandoff(workflow, "wfstep_mcp", "wfstep_skill")
		now := time.Now().UTC()

		workflow = manager.InitializeExecution(workflow, now)
		workflow = manager.StartStepAttempt(workflow, "wfstep_mcp", "step_mcp_1", now.Add(time.Millisecond))
		workflow = manager.ApplyToolCallResult(workflow, runtime.ToolCall{
			WorkflowStepID: "wfstep_mcp",
			StepID:         "step_mcp_1",
			ToolCallID:     "tool_call_mcp_1",
			Status:         runtime.ToolCallStatusCompleted,
			Output:         map[string]any{"result": "mcp ok"},
		}, "", "", now.Add(2*time.Millisecond))
		workflow, _ = manager.AdvanceReadySteps(workflow, now.Add(3*time.Millisecond))
		workflow = manager.StartStepAttempt(workflow, "wfstep_skill", "step_skill_1", now.Add(4*time.Millisecond))
		workflow = manager.ApplyToolCallResult(workflow, runtime.ToolCall{
			WorkflowStepID: "wfstep_skill",
			StepID:         "step_skill_1",
			ToolCallID:     "tool_call_skill_1",
			Status:         runtime.ToolCallStatusFailed,
			FailureClass:   "consumer_unavailable",
		}, "", "", now.Add(5*time.Millisecond))

		if workflow.Status != orch.WorkflowStatusBlocked {
			t.Fatalf("expected blocked workflow, got %+v", workflow)
		}
		if workflow.Steps[1].BlockedReason != string(orch.BlockedReasonConsumerUnavailable) {
			t.Fatalf("expected consumer_unavailable blocked reason, got %+v", workflow.Steps[1])
		}
		if workflow.Handoffs[0].Status != orch.HandoffStatusConsumed {
			t.Fatalf("expected consumed handoff after dependent execution started, got %+v", workflow.Handoffs)
		}
	})

	t.Run("partial failure preserves prior side effects", func(t *testing.T) {
		workflow := newWorkflowFixture(
			"run_mix_partial",
			workflowStepFixture{id: "wfstep_a", consumerKind: string(runtime.ToolCallInvocationKindSkill), consumerID: "exec-skill", maxAttempts: 1},
			workflowStepFixture{id: "wfstep_b", consumerKind: string(runtime.ToolCallInvocationKindSkill), consumerID: "exec-skill", maxAttempts: 1},
		)
		now := time.Now().UTC()
		workflow.Steps[0].Status = orch.StepStatusCompleted
		workflow.Steps[0].SideEffectsVisible = true
		workflow.Steps[0].UpdatedAt = now
		workflow.Steps[1].Status = orch.StepStatusRunning
		workflow.Steps[1].AttemptCount = 1
		workflow.Steps[1].UpdatedAt = now
		workflow.Status = orch.WorkflowStatusRunning

		workflow = manager.ApplyToolCallResult(workflow, runtime.ToolCall{
			WorkflowStepID: "wfstep_b",
			StepID:         "step_b_1",
			ToolCallID:     "tool_call_b_1",
			Status:         runtime.ToolCallStatusFailed,
			FailureClass:   "execution_failed",
		}, "", "", now.Add(time.Millisecond))
		if workflow.Status != orch.WorkflowStatusPartialFailed {
			t.Fatalf("expected partial_failed workflow, got %+v", workflow)
		}
		if workflow.CompletedAt == nil || workflow.Steps[1].Status != orch.StepStatusFailed {
			t.Fatalf("expected terminal failed step with workflow completion, got %+v", workflow)
		}
	})
}

type workflowStepFixture struct {
	id           string
	consumerKind string
	consumerID   string
	maxAttempts  int
}

func newWorkflowFixture(runID string, steps ...workflowStepFixture) orch.Workflow {
	now := time.Now().UTC()
	workflow := orch.Workflow{
		WorkflowID:       "wf_fixture_" + strings.ReplaceAll(runID, "-", "_"),
		RunID:            runID,
		EnvironmentScope: "test",
		Goal:             "fixture goal",
		Status:           orch.WorkflowStatusPlanned,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	for idx, step := range steps {
		workflow.Steps = append(workflow.Steps, orch.WorkflowStep{
			WorkflowStepID:       step.id,
			WorkflowID:           workflow.WorkflowID,
			Title:                step.id,
			Position:             idx + 1,
			ConsumerKind:         step.consumerKind,
			ConsumerID:           step.consumerID,
			ToolName:             step.consumerID,
			Status:               orch.StepStatusPlanned,
			ApprovalModeExpected: "allow",
			MaxAttempts:          max(1, step.maxAttempts),
			CreatedAt:            now,
			UpdatedAt:            now,
		})
	}
	return workflow
}

func withSuccessDependency(workflow orch.Workflow, fromStepID, toStepID string) orch.Workflow {
	dependencyID := "wfdep_" + fromStepID + "_" + toStepID
	workflow.Dependencies = append(workflow.Dependencies, orch.Dependency{
		DependencyID:       dependencyID,
		WorkflowID:         workflow.WorkflowID,
		FromWorkflowStepID: fromStepID,
		ToWorkflowStepID:   toStepID,
		DependencyType:     orch.DependencyTypeSuccess,
		Reason:             "fixture dependency",
	})
	for idx := range workflow.Steps {
		if workflow.Steps[idx].WorkflowStepID == toStepID {
			workflow.Steps[idx].DependencyIDs = append(workflow.Steps[idx].DependencyIDs, dependencyID)
		}
	}
	return workflow
}

func withHandoff(workflow orch.Workflow, fromStepID, toStepID string) orch.Workflow {
	workflow.Handoffs = append(workflow.Handoffs, orch.Handoff{
		HandoffID:          "wfhandoff_" + fromStepID + "_" + toStepID,
		WorkflowID:         workflow.WorkflowID,
		FromWorkflowStepID: fromStepID,
		ToWorkflowStepID:   toStepID,
		Status:             orch.HandoffStatusPending,
		PayloadSummary:     "fixture handoff",
		SourcePath:         "step.output.result",
	})
	return workflow
}

func assertRetryState(t *testing.T, workflow orch.Workflow, workflowStepID string, wantStatus orch.StepStatus, wantAttemptCount int, wantActiveToolCallID string) {
	t.Helper()
	step := orch.WorkflowStepByID(workflow, workflowStepID)
	if step == nil {
		t.Fatalf("expected workflow step %s", workflowStepID)
	}
	if step.Status != wantStatus || step.AttemptCount != wantAttemptCount || step.ActiveToolCallID != wantActiveToolCallID {
		t.Fatalf("unexpected retry state %+v", *step)
	}
}

type orchestrationMCPPlanningAdapter struct {
	manager *mcp.Manager
}

type orchestrationSkillPlanningAdapter struct {
	registry *skills.Registry
}

func (a orchestrationSkillPlanningAdapter) ListSkills() []orch.SkillPlanningCandidate {
	if a.registry == nil {
		return nil
	}
	skillsList := a.registry.List()
	items := make([]orch.SkillPlanningCandidate, 0, len(skillsList))
	for _, skill := range skillsList {
		item := orch.SkillPlanningCandidate{
			SkillID:    skill.SkillID,
			Executable: skill.ExecutionManifest != nil,
			Available:  skill.AvailabilityStatus == skills.SkillAvailabilityStatusAvailable,
		}
		if skill.ExecutionManifest != nil {
			item.ApprovalModeExpected = string(skill.ExecutionManifest.ApprovalMode)
		}
		items = append(items, item)
	}
	return items
}

func (a orchestrationMCPPlanningAdapter) ListServers() []orch.MCPPlanningServer {
	if a.manager == nil {
		return nil
	}
	servers := a.manager.ListServers()
	items := make([]orch.MCPPlanningServer, 0, len(servers))
	for _, server := range servers {
		item := orch.MCPPlanningServer{ServerID: server.ServerID}
		for _, tool := range server.Tools {
			item.Tools = append(item.Tools, orch.MCPPlanningTool{ToolName: tool.ToolName})
		}
		items = append(items, item)
	}
	return items
}

func (a orchestrationMCPPlanningAdapter) ListTools(serverID string) ([]orch.MCPPlanningTool, error) {
	if a.manager == nil {
		return nil, nil
	}
	tools, err := a.manager.ListTools(serverID)
	if err != nil {
		return nil, err
	}
	items := make([]orch.MCPPlanningTool, 0, len(tools))
	for _, tool := range tools {
		items = append(items, orch.MCPPlanningTool{ToolName: tool.ToolName})
	}
	return items, nil
}

func newOrchestrationMCPHarness(t *testing.T) (config.Config, *store.SQLiteStore, *events.Bus, *sandbox.Manager, *mcp.Manager) {
	t.Helper()

	dataDir := filepath.Join(t.TempDir(), "dope")
	writeOrchestrationMCPSecretsFileForTest(t, dataDir, map[string]string{
		"GO_WANT_ORCHESTRATION_MCP_HELPER": "1",
		"ORCHESTRATION_MCP_HELPER_TOOLS":   `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`,
	})
	cfg := config.Config{Environment: config.EnvironmentTest, DataDir: dataDir}
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	eventBus := events.NewBus()
	t.Cleanup(eventBus.Close)
	policyEngine := policy.NewEngine()
	sandboxes := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	t.Cleanup(func() { _ = sandboxes.Close(context.Background()) })

	manager := mcp.NewManager(cfg, sqliteStore, eventBus, sandboxes, policyEngine, mcp.NewTransportMux(mcp.NewStdioTransport(), nil))
	if _, _, err := manager.CreateServer(context.Background(), mcp.CreateServerInput{
		ServerID:         "orchestration-mcp",
		DisplayName:      "Orchestration MCP",
		Enabled:          true,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:orchestration-mcp:lifecycle.start",
		TransportKind:    mcp.TransportKindStdio,
		Command:          os.Args[0],
		Args:             []string{"-test.run=TestOrchestrationMCPHelperProcess", "--"},
		WorkingDir:       t.TempDir(),
		SecretRefs:       []string{"GO_WANT_ORCHESTRATION_MCP_HELPER", "ORCHESTRATION_MCP_HELPER_TOOLS"},
		AutoRestart:      true,
	}); err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}
	started, err := manager.Start(context.Background(), "orchestration-mcp", "workflow:test")
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if started.Server.State.Status != mcp.LifecycleStatusHealthy {
		t.Fatalf("expected healthy mcp server, got %+v", started.Server.State)
	}
	return cfg, sqliteStore, eventBus, sandboxes, manager
}

func newOrchestrationSkillRegistryForTest(t *testing.T, dataRoot, skillID, approvalMode, script string) *skills.Registry {
	t.Helper()

	homeRoot := filepath.Join(t.TempDir(), ".agents")
	writeOrchestrationSkillFileForTest(t, filepath.Join(homeRoot, "AGENTS.md"), "home overlay")
	writeOrchestrationSkillFileForTest(t, filepath.Join(dataRoot, "AGENTS.md"), "data overlay")
	writeOrchestrationExecutableSkillForTest(t, filepath.Join(dataRoot, "skills", skillID), fmt.Sprintf(`
---
name: %s
description: executable skill
execution.entrypoint: scripts/run.sh
execution.working_dir: .
execution.profile_id: subprocess_default
execution.read_roots: .
execution.write_roots: .
execution.network_mode: deny
execution.timeout_ms: 5000
execution.approval_mode: %s
---
workflow test skill
`, skillID, approvalMode), script)

	registry, err := skills.NewRegistryWithRoots(homeRoot, dataRoot)
	if err != nil {
		t.Fatalf("NewRegistryWithRoots returned error: %v", err)
	}
	return registry
}

func writeOrchestrationSkillFileForTest(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(contents)+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeOrchestrationExecutableSkillForTest(t *testing.T, dir, markdown, script string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatalf("mkdir skill scripts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(strings.TrimSpace(markdown)+"\n"), 0o644); err != nil {
		t.Fatalf("write skill markdown: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "run.sh"), []byte(script), 0o755); err != nil {
		t.Fatalf("write skill script: %v", err)
	}
}

func writeOrchestrationMCPSecretsFileForTest(t *testing.T, dataDir string, values map[string]string) {
	t.Helper()
	payload, err := json.Marshal(values)
	if err != nil {
		t.Fatalf("marshal mcp secrets: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "mcp-secrets.json"), payload, 0o600); err != nil {
		t.Fatalf("write mcp secrets: %v", err)
	}
}

func readOrchestrationHelperFrame(reader *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if !strings.HasPrefix(strings.ToLower(line), "content-length:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "Content-Length:"), "content-length:"))
		if _, err := fmt.Sscanf(value, "%d", &length); err != nil {
			return nil, err
		}
	}
	if length < 0 {
		return nil, io.EOF
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func writeOrchestrationHelperFrame(value any) {
	payload, _ := json.Marshal(value)
	_, _ = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(payload), payload)
}
