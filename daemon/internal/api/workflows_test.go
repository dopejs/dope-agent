package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
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
		WorkflowID:        "wf_prod_hidden",
		RunID:             run.RunID,
		EnvironmentScope:  "prod",
		Goal:              "hidden",
		Status:            orchestration.WorkflowStatusPlanned,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
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
