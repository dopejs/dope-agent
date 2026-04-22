package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/artifacts"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/skills"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

func TestWorkflowPlanningRoutesExposeInspectablePlanAndEnvironmentIsolation(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     filepath.Join(t.TempDir(), "dope-data"),
	}
	sqliteStore, err := store.NewSQLiteStore(cfg.DataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	manager := runtime.NewManager()
	run, err := manager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "Use a skill to complete a deterministic workflow."})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	registry := newAllowSkillRegistryForWorkflowTest(t, cfg.DataDir)
	eventBus := events.NewBus()
	policyEngine := policy.NewEngine()
	sandboxManager := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	server := NewServer(Dependencies{
		Config:      cfg,
		Logger:      telemetry.New("error").Slog(),
		EventBus:    eventBus,
		Policy:      policyEngine,
		Runtime:     manager,
		Skills:      registry,
		Sandboxes:   sandboxManager,
		Store:       sqliteStore,
		Checkpoints: checkpoints.NewManager(sqliteStore, manager),
	})

	prodWorkflow := orchestration.Workflow{
		WorkflowID:       "wf_prod_hidden",
		RunID:            run.RunID,
		EnvironmentScope: "prod",
		Goal:             "hidden",
		Status:           orchestration.WorkflowStatusPlanned,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	if err := sqliteStore.UpsertWorkflow(context.Background(), prodWorkflow); err != nil {
		t.Fatalf("UpsertWorkflow returned error: %v", err)
	}

	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/workflows", strings.NewReader(`{}`)))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	created := decodeStrictResponse[orchestration.Workflow](t, createRec.Body.Bytes())
	if created.Status != orchestration.WorkflowStatusPlanned {
		t.Fatalf("expected planned workflow, got %s", created.Status)
	}
	if len(created.Steps) != 1 {
		t.Fatalf("expected one planned step, got %d", len(created.Steps))
	}
	if created.Steps[0].RuntimeStepID != "" || created.Steps[0].ActiveToolCallID != "" {
		t.Fatalf("expected inspect-only planning state, got %+v", created.Steps[0])
	}

	listRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/v1/runs/"+run.RunID+"/workflows", nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	list := decodeStrictResponse[WorkflowListResponse](t, listRec.Body.Bytes())
	if len(list.Items) != 1 {
		t.Fatalf("expected only test-environment workflow, got %d", len(list.Items))
	}

	getRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/v1/runs/"+run.RunID+"/workflows/"+created.WorkflowID, nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	got := decodeStrictResponse[orchestration.Workflow](t, getRec.Body.Bytes())
	if got.EnvironmentScope != "test" {
		t.Fatalf("expected test environment scope, got %s", got.EnvironmentScope)
	}
	if got.PlanSummary == "" || got.Steps[0].SelectionRationale == "" {
		t.Fatalf("expected inspectable planning truth, got %+v", got)
	}
}

func TestWorkflowStartExecutesAllowModeSkillAndLinksRuntimeTruth(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     filepath.Join(t.TempDir(), "dope-data"),
	}
	sqliteStore, err := store.NewSQLiteStore(cfg.DataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	manager := runtime.NewManager()
	run, err := manager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "Use a skill to complete a deterministic workflow."})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	registry := newAllowSkillRegistryForWorkflowTest(t, cfg.DataDir)
	eventBus := events.NewBus()
	policyEngine := policy.NewEngine()
	sandboxManager := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	server := NewServer(Dependencies{
		Config:      cfg,
		Logger:      telemetry.New("error").Slog(),
		EventBus:    eventBus,
		Policy:      policyEngine,
		Runtime:     manager,
		Skills:      registry,
		Sandboxes:   sandboxManager,
		Store:       sqliteStore,
		Checkpoints: checkpoints.NewManager(sqliteStore, manager),
	})

	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/workflows", strings.NewReader(`{}`)))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	created := decodeStrictResponse[orchestration.Workflow](t, createRec.Body.Bytes())

	startRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(startRec, httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/workflows/"+created.WorkflowID+"/start", nil))
	if startRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", startRec.Code, startRec.Body.String())
	}

	var final orchestration.Workflow
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		getRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/v1/runs/"+run.RunID+"/workflows/"+created.WorkflowID, nil))
		if getRec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", getRec.Code, getRec.Body.String())
		}
		final = decodeStrictResponse[orchestration.Workflow](t, getRec.Body.Bytes())
		if final.Status == orchestration.WorkflowStatusCompleted {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if final.Status != orchestration.WorkflowStatusCompleted {
		t.Fatalf("expected completed workflow, got %+v", final)
	}
	if len(final.Steps) != 1 || final.Steps[0].RuntimeStepID == "" || final.Steps[0].ActiveToolCallID == "" {
		t.Fatalf("expected runtime linkage on workflow step, got %+v", final.Steps)
	}

	step, ok := manager.GetStep(run.RunID, final.Steps[0].RuntimeStepID)
	if !ok {
		t.Fatal("expected linked runtime step")
	}
	if step.WorkflowID != created.WorkflowID || step.WorkflowStepID != final.Steps[0].WorkflowStepID || step.Attempt != 1 {
		t.Fatalf("unexpected runtime step linkage %+v", step)
	}
	toolCall, ok := manager.GetToolCall(run.RunID, step.StepID, final.Steps[0].ActiveToolCallID)
	if !ok {
		t.Fatal("expected linked tool call")
	}
	if toolCall.WorkflowID != created.WorkflowID || toolCall.WorkflowStepID != final.Steps[0].WorkflowStepID || toolCall.Attempt != 1 {
		t.Fatalf("unexpected tool call linkage %+v", toolCall)
	}
}

func TestWorkflowStartExecutesComputerUseStepAndProjectsEvidence(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     filepath.Join(t.TempDir(), "dope-data"),
	}
	sqliteStore, err := store.NewSQLiteStore(cfg.DataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	manager := runtime.NewManager()
	run, err := manager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "Use browser and a skill to complete a deterministic workflow."})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	registry := newAllowSkillRegistryForWorkflowTest(t, cfg.DataDir)
	eventBus := events.NewBus()
	policyEngine := policy.NewEngine()
	sandboxManager := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	checkpointManager := checkpoints.NewManager(sqliteStore, manager)
	server := NewServer(Dependencies{
		Config:      cfg,
		Logger:      telemetry.New("error").Slog(),
		EventBus:    eventBus,
		Policy:      policyEngine,
		Runtime:     manager,
		Skills:      registry,
		Sandboxes:   sandboxManager,
		Store:       sqliteStore,
		Checkpoints: checkpointManager,
		ComputerUse: computeruse.NewManager(computeruse.Dependencies{
			EnvironmentScope: "test",
			Runtime:          manager,
			Policy:           policyEngine,
			Store:            sqliteStore,
			Artifacts:        artifacts.NewService(t.TempDir()),
		}),
	})

	workflow := orchestration.Workflow{
		WorkflowID:       "wf_computer_use",
		RunID:            run.RunID,
		EnvironmentScope: "test",
		Goal:             "Use browser and a skill to complete a deterministic workflow.",
		Status:           orchestration.WorkflowStatusPlanned,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		PlanSummary:      "Plan one browser step followed by one executable skill handoff.",
	}
	workflow.Steps = []orchestration.WorkflowStep{
		{
			WorkflowStepID:     "wfstep_browser",
			WorkflowID:         workflow.WorkflowID,
			Title:              "Inspect deterministic browser fixture",
			Position:           1,
			ConsumerKind:       "computer_use",
			ConsumerID:         "browser",
			ToolName:           "browser",
			Status:             orchestration.StepStatusPlanned,
			SelectionRationale: "Selected the browser-first computer-use plane for deterministic page inspection.",
			Input: map[string]any{
				"driverKind": "browser",
				"initialUrl": "https://example.test/browser",
				"actions": []any{
					map[string]any{"actionKind": "navigate", "url": "https://example.test/browser"},
					map[string]any{"actionKind": "snapshot"},
				},
			},
			AttemptCount: 0,
			MaxAttempts:  1,
			CreatedAt:    workflow.CreatedAt,
			UpdatedAt:    workflow.UpdatedAt,
		},
		{
			WorkflowStepID:       "wfstep_skill",
			WorkflowID:           workflow.WorkflowID,
			Title:                "Run executable skill exec-skill",
			Position:             2,
			ConsumerKind:         "skill",
			ConsumerID:           "exec-skill",
			ToolName:             "exec-skill",
			Status:               orchestration.StepStatusPlanned,
			SelectionRationale:   "Continue the workflow through the existing skill runtime plane.",
			ApprovalModeExpected: "allow",
			Input:                map[string]any{"args": []any{"workflow browser step"}},
			DependencyIDs:        []string{"wfdep_browser_skill"},
			AttemptCount:         0,
			MaxAttempts:          1,
			CreatedAt:            workflow.CreatedAt,
			UpdatedAt:            workflow.UpdatedAt,
		},
	}
	workflow.Dependencies = []orchestration.Dependency{{
		DependencyID:       "wfdep_browser_skill",
		WorkflowID:         workflow.WorkflowID,
		FromWorkflowStepID: "wfstep_browser",
		ToWorkflowStepID:   "wfstep_skill",
		DependencyType:     orchestration.DependencyTypeSuccess,
		Reason:             "workflow consumes browser evidence before local continuation",
	}}
	if err := persistWorkflowDetail(context.Background(), sqliteStore, workflow); err != nil {
		t.Fatalf("persistWorkflowDetail returned error: %v", err)
	}

	startRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(startRec, httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/workflows/"+workflow.WorkflowID+"/start", nil))
	if startRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", startRec.Code, startRec.Body.String())
	}

	var final orchestration.Workflow
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		getRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/v1/runs/"+run.RunID+"/workflows/"+workflow.WorkflowID, nil))
		if getRec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", getRec.Code, getRec.Body.String())
		}
		final = decodeStrictResponse[orchestration.Workflow](t, getRec.Body.Bytes())
		if final.Status == orchestration.WorkflowStatusCompleted {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if final.Status != orchestration.WorkflowStatusCompleted {
		t.Fatalf("expected completed workflow, got %+v", final)
	}
	if len(final.Steps) != 2 {
		t.Fatalf("expected two steps, got %+v", final.Steps)
	}
	browserStep := final.Steps[0]
	if browserStep.ConsumerKind != "computer_use" || browserStep.ComputerUseSessionID == "" || len(browserStep.ComputerUseActionIDs) != 2 {
		t.Fatalf("expected projected computer-use linkage, got %+v", browserStep)
	}
	if len(browserStep.ComputerUseArtifacts) == 0 {
		t.Fatalf("expected workflow-visible evidence summaries, got %+v", browserStep)
	}
	if browserStep.RuntimeStepID == "" || browserStep.ActiveToolCallID == "" {
		t.Fatalf("expected runtime linkage for browser step, got %+v", browserStep)
	}
	if final.Steps[1].Status != orchestration.StepStatusCompleted {
		t.Fatalf("expected dependent skill step to complete, got %+v", final.Steps[1])
	}
}

func TestWorkflowStepBindingsTrackLinkedIntegrationToolCall(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	workflow := orchestration.Workflow{
		WorkflowID:       "wf_integration",
		RunID:            "run_integration",
		EnvironmentScope: "test",
		Status:           orchestration.WorkflowStatusRunning,
		CreatedAt:        now,
		UpdatedAt:        now,
		Steps: []orchestration.WorkflowStep{{
			WorkflowStepID: "wfstep_integration",
			WorkflowID:     "wf_integration",
			Title:          "Inspect integration",
			Position:       1,
			ConsumerKind:   "skill",
			ConsumerID:     "integration-probe",
			ToolName:       "inspect",
			Status:         orchestration.StepStatusRunning,
			AttemptCount:   1,
			MaxAttempts:    1,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	toolCall := runtime.ToolCall{
		ToolCallID:     "tool_call_integration",
		RunID:          "run_integration",
		StepID:         "step_integration",
		WorkflowID:     "wf_integration",
		WorkflowStepID: "wfstep_integration",
		ToolName:       "inspect",
		Status:         runtime.ToolCallStatusCompleted,
		Output:         map[string]any{"message": "ok"},
		IntegrationBindings: []integrations.BindingSummary{{
			IntegrationID:         "calendar-a",
			DomainKind:            "calendar",
			DisplayName:           "Calendar A",
			AccountKey:            "acct_calendar",
			CanonicalDefault:      true,
			ReadinessAtInvocation: integrations.ReadinessStatusDegraded,
			BackendKind:           integrations.BackendKindFakeLocal,
			SecretResolution:      "resolved",
			EnvironmentScope:      "test",
			CapturedAt:            now,
		}},
	}

	manager := orchestration.NewManager()
	workflow = manager.BindToolCall(workflow, "wfstep_integration", toolCall, now)
	if len(workflow.Steps[0].IntegrationBindings) != 1 || workflow.Steps[0].IntegrationBindings[0].IntegrationID != "calendar-a" {
		t.Fatalf("expected workflow step to inherit integration bindings on bind, got %+v", workflow.Steps[0])
	}

	workflow = manager.ApplyToolCallResult(workflow, toolCall, orchestration.StepStatusCompleted, "", now)
	if workflow.Steps[0].Status != orchestration.StepStatusCompleted {
		t.Fatalf("expected completed workflow step, got %+v", workflow.Steps[0])
	}
	if len(workflow.Steps[0].IntegrationBindings) != 1 || workflow.Steps[0].IntegrationBindings[0].ReadinessAtInvocation != integrations.ReadinessStatusDegraded {
		t.Fatalf("expected workflow step to retain integration bindings after result application, got %+v", workflow.Steps[0])
	}
}

func newAllowSkillRegistryForWorkflowTest(t *testing.T, dataRoot string) *skills.Registry {
	t.Helper()

	homeRoot := filepath.Join(t.TempDir(), ".agents")
	writeSkillFileForTest(t, filepath.Join(homeRoot, "AGENTS.md"), "home overlay")
	writeSkillFileForTest(t, filepath.Join(dataRoot, "AGENTS.md"), "data overlay")
	writeExecutableSkillForTest(t, filepath.Join(dataRoot, "skills", "exec-skill"), `
---
name: exec-skill
description: executable skill
execution.entrypoint: scripts/run.sh
execution.working_dir: .
execution.profile_id: subprocess_default
execution.read_roots: .
execution.write_roots: .
execution.network_mode: deny
execution.timeout_ms: 5000
execution.approval_mode: allow
---
workflow test skill
`, "#!/bin/sh\nprintf 'workflow-ok %s' \"$1\"")

	registry, err := skills.NewRegistryWithRoots(homeRoot, dataRoot)
	if err != nil {
		t.Fatalf("NewRegistryWithRoots returned error: %v", err)
	}
	return registry
}
