package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestRunLaunchQuotaDeniesBeforeRuntimeCreateRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	if err := sqliteStore.EnsureBillingCatalog(ctx); err != nil {
		t.Fatalf("EnsureBillingCatalog returned error: %v", err)
	}
	tenantID := "ten_r38_run_denied"
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{
		PlanID:          "plan_run_denied",
		TenantID:        tenantID,
		PlanKey:         "finite",
		Status:          billing.PlanStatusActive,
		EnforcementMode: billing.EnforcementModeEnforced,
		EffectiveAt:     now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	limit := int64(0)
	if err := sqliteStore.SaveQuotaOverride(ctx, billing.QuotaOverride{
		QuotaOverrideID: "override_run_denied",
		TenantID:        tenantID,
		Category:        billing.CategoryRunLaunches,
		Limit:           &limit,
		EffectiveAt:     now.Add(-time.Minute),
		Reason:          "test exhausted quota",
	}); err != nil {
		t.Fatalf("SaveQuotaOverride returned error: %v", err)
	}
	runtimeManager := runtime.NewManager()
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{"entrypoint":"operator.shell.test","goal":"deny before create"}`))
	req = req.WithContext(withTenantContext(req.Context(), r38BillingOwnerContext(tenantID)))
	rec := httptest.NewRecorder()

	handleRuns(config.Config{Environment: config.EnvironmentTest}, router.NewSessionRouter(), runtimeManager, events.NewBus(), nil, billing.NewManager(sqliteStore), sqliteStore, checkpoints.NewManager(sqliteStore, runtimeManager), rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected quota denial status 429, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`quota_denied:run_launches_exhausted`)) {
		t.Fatalf("expected stable run quota denial, got %s", rec.Body.String())
	}
	if runs := runtimeManager.ListRuns(); len(runs) != 0 {
		t.Fatalf("expected no runtime runs to be created before quota denial, got %#v", runs)
	}
}

func TestBackgroundWorkflowLaunchQuotaDeniesBeforeRuntimeRunCreation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	if err := sqliteStore.EnsureBillingCatalog(ctx); err != nil {
		t.Fatalf("EnsureBillingCatalog returned error: %v", err)
	}
	tenantID := "ten_r38_background_workflow_denied"
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{PlanID: "plan_" + tenantID, TenantID: tenantID, PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	limit := int64(0)
	if err := sqliteStore.SaveQuotaOverride(ctx, billing.QuotaOverride{
		QuotaOverrideID: "override_" + tenantID,
		TenantID:        tenantID,
		Category:        billing.CategoryRunLaunches,
		Limit:           &limit,
		EffectiveAt:     now.Add(-time.Minute),
		Reason:          "test exhausted background workflow run quota",
	}); err != nil {
		t.Fatalf("SaveQuotaOverride returned error: %v", err)
	}

	runtimeManager := runtime.NewManager()
	launcher := NewScheduleWorkflowLauncher(ScheduleWorkflowLauncherDependencies{
		Config:      config.Config{Environment: config.EnvironmentTest},
		Runtime:     runtimeManager,
		Billing:     billing.NewManager(sqliteStore),
		EventBus:    events.NewBus(),
		Store:       sqliteStore,
		Checkpoints: checkpoints.NewManager(sqliteStore, runtimeManager),
	})
	_, err = launcher.LaunchScheduledWorkflow(withTenantContext(ctx, r38BillingOwnerContext(tenantID)), scheduler.WorkflowTarget{
		Entrypoint:   "operator.scheduler",
		RunGoal:      "deny background workflow run",
		WorkflowGoal: "quota denial before run creation",
	}, "sched_quota_denied", "attempt_quota_denied")
	if !errors.Is(err, billing.ErrQuotaDenied) {
		t.Fatalf("expected ErrQuotaDenied, got %v", err)
	}
	if runs := runtimeManager.ListRuns(); len(runs) != 0 {
		t.Fatalf("expected no runtime run before background workflow quota denial, got %+v", runs)
	}
}

func TestBackgroundWorkflowLaunchWorkflowQuotaDeniesBeforeRuntimeRunCreation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	if err := sqliteStore.EnsureBillingCatalog(ctx); err != nil {
		t.Fatalf("EnsureBillingCatalog returned error: %v", err)
	}
	tenantID := "ten_r38_background_workflow_quota_denied"
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{PlanID: "plan_" + tenantID, TenantID: tenantID, PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	runLimit := int64(1)
	if err := sqliteStore.SaveQuotaOverride(ctx, billing.QuotaOverride{
		QuotaOverrideID: "override_run_" + tenantID,
		TenantID:        tenantID,
		Category:        billing.CategoryRunLaunches,
		Limit:           &runLimit,
		EffectiveAt:     now.Add(-time.Minute),
		Reason:          "test available background run quota",
	}); err != nil {
		t.Fatalf("SaveQuotaOverride(run) returned error: %v", err)
	}
	workflowLimit := int64(0)
	if err := sqliteStore.SaveQuotaOverride(ctx, billing.QuotaOverride{
		QuotaOverrideID: "override_workflow_" + tenantID,
		TenantID:        tenantID,
		Category:        billing.CategoryWorkflowLaunches,
		Limit:           &workflowLimit,
		EffectiveAt:     now.Add(-time.Minute),
		Reason:          "test exhausted background workflow quota",
	}); err != nil {
		t.Fatalf("SaveQuotaOverride(workflow) returned error: %v", err)
	}

	runtimeManager := runtime.NewManager()
	launcher := NewScheduleWorkflowLauncher(ScheduleWorkflowLauncherDependencies{
		Config:      config.Config{Environment: config.EnvironmentTest},
		Runtime:     runtimeManager,
		Billing:     billing.NewManager(sqliteStore),
		EventBus:    events.NewBus(),
		Store:       sqliteStore,
		Checkpoints: checkpoints.NewManager(sqliteStore, runtimeManager),
	})
	_, err = launcher.LaunchScheduledWorkflow(withTenantContext(ctx, r38BillingOwnerContext(tenantID)), scheduler.WorkflowTarget{
		Entrypoint:   "operator.scheduler",
		RunGoal:      "deny background workflow launch",
		WorkflowGoal: "quota denial before run creation",
	}, "sched_workflow_quota_denied", "attempt_workflow_quota_denied")
	if !errors.Is(err, billing.ErrQuotaDenied) {
		t.Fatalf("expected ErrQuotaDenied, got %v", err)
	}
	if runs := runtimeManager.ListRuns(); len(runs) != 0 {
		t.Fatalf("expected no runtime run before background workflow quota denial, got %+v", runs)
	}
}

func TestRunLaunchQuotaCommitsAfterRunPersistence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	if err := sqliteStore.EnsureBillingCatalog(ctx); err != nil {
		t.Fatalf("EnsureBillingCatalog returned error: %v", err)
	}
	tenantID := "ten_r38_run_allowed"
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{
		PlanID:          "plan_run_allowed",
		TenantID:        tenantID,
		PlanKey:         "finite",
		Status:          billing.PlanStatusActive,
		EnforcementMode: billing.EnforcementModeEnforced,
		EffectiveAt:     now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	runtimeManager := runtime.NewManager()
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{"entrypoint":"operator.shell.test","goal":"commit after persist"}`))
	req = req.WithContext(withTenantContext(req.Context(), r38BillingOwnerContext(tenantID)))
	rec := httptest.NewRecorder()

	handleRuns(config.Config{Environment: config.EnvironmentTest}, router.NewSessionRouter(), runtimeManager, events.NewBus(), nil, billing.NewManager(sqliteStore), sqliteStore, checkpoints.NewManager(sqliteStore, runtimeManager), rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected run creation status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	definition, _ := billing.DefinitionFor(billing.CategoryRunLaunches)
	period, err := sqliteStore.OpenPeriod(ctx, tenantID, definition, now)
	if err != nil {
		t.Fatalf("OpenPeriod returned error: %v", err)
	}
	counter, found, err := sqliteStore.UsageCounter(ctx, tenantID, billing.CategoryRunLaunches, period.QuotaPeriodID)
	if err != nil {
		t.Fatalf("UsageCounter returned error: %v", err)
	}
	if !found || counter.CommittedAmount != 1 || counter.ReservedAmount != 0 {
		t.Fatalf("expected committed run launch usage, found=%v counter=%#v", found, counter)
	}
}

func TestRunLaunchQuotaStateUnavailableFailsClosedForHosted(t *testing.T) {
	t.Parallel()

	runtimeManager := runtime.NewManager()
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{"entrypoint":"operator.shell.test","goal":"fail closed"}`))
	req = req.WithContext(withTenantContext(req.Context(), r38BillingOwnerContext("ten_r38_missing_billing_state")))
	rec := httptest.NewRecorder()

	handleRuns(config.Config{Environment: config.EnvironmentProd}, router.NewSessionRouter(), runtimeManager, events.NewBus(), nil, billing.NewManager(nil), nil, nil, rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected hosted quota-state-unavailable status 503, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`quota_denied:quota_state_unavailable`)) {
		t.Fatalf("expected stable quota-state-unavailable denial, got %s", rec.Body.String())
	}
	if runs := runtimeManager.ListRuns(); len(runs) != 0 {
		t.Fatalf("expected no runtime runs to be created while quota state is unavailable, got %#v", runs)
	}
}

func TestRunLaunchQuotaStateUnavailableAllowsLocalDevelopment(t *testing.T) {
	t.Parallel()

	runtimeManager := runtime.NewManager()
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewBufferString(`{"entrypoint":"operator.shell.test","goal":"local allow"}`))
	req = req.WithContext(withTenantContext(req.Context(), r38BillingOwnerContext("ten_r38_local_dev")))
	rec := httptest.NewRecorder()

	handleRuns(config.Config{Environment: config.EnvironmentTest}, router.NewSessionRouter(), runtimeManager, events.NewBus(), nil, billing.NewManager(nil), nil, nil, rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected local development run creation status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if runs := runtimeManager.ListRuns(); len(runs) != 1 {
		t.Fatalf("expected local development run to be created, got %#v", runs)
	}
}

func TestReleaseBillingReservationRefundsFailureBeforeConsumption(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	if err := sqliteStore.EnsureBillingCatalog(ctx); err != nil {
		t.Fatalf("EnsureBillingCatalog returned error: %v", err)
	}
	tenantID := "ten_r38_release_helper"
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{PlanID: "plan_" + tenantID, TenantID: tenantID, PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	manager := billing.NewManager(sqliteStore)
	reserved, err := manager.Reserve(ctx, billing.ReserveInput{
		TenantID:          tenantID,
		Category:          billing.CategoryRunLaunches,
		Amount:            1,
		OperationKey:      billing.RunOperationKey(tenantID, "client_failure_before_consumption", ""),
		ReservationPoint:  "POST /v1/runs before runtime.CreateRun",
		GuardedEntryPoint: "POST /v1/runs",
		Hosted:            true,
	})
	if err != nil || !reserved.Allowed {
		t.Fatalf("Reserve returned err=%v result=%+v", err, reserved)
	}

	releaseBillingReservation(ctx, manager, reserved.Reservation, "test failure before consumption")

	updated, found, err := sqliteStore.ReservationByOperation(ctx, tenantID, billing.CategoryRunLaunches, reserved.Reservation.OperationKey)
	if err != nil || !found {
		t.Fatalf("ReservationByOperation err=%v found=%v", err, found)
	}
	if updated.Status != billing.ReservationStatusReleased {
		t.Fatalf("expected released reservation, got %+v", updated)
	}
	definition, _ := billing.DefinitionFor(billing.CategoryRunLaunches)
	period, err := sqliteStore.OpenPeriod(ctx, tenantID, definition, now)
	if err != nil {
		t.Fatalf("OpenPeriod returned error: %v", err)
	}
	counter, found, err := sqliteStore.UsageCounter(ctx, tenantID, billing.CategoryRunLaunches, period.QuotaPeriodID)
	if err != nil || !found {
		t.Fatalf("UsageCounter err=%v found=%v", err, found)
	}
	if counter.ReservedAmount != 0 || counter.CommittedAmount != 0 {
		t.Fatalf("expected failure-before-consumption release to clear reserved usage, got %+v", counter)
	}
}

func TestReserveDeniedForOperatorActionNeededDuplicateWork(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	if err := sqliteStore.EnsureBillingCatalog(ctx); err != nil {
		t.Fatalf("EnsureBillingCatalog returned error: %v", err)
	}
	tenantID := "ten_r38_operator_action_duplicate"
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{PlanID: "plan_" + tenantID, TenantID: tenantID, PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	definition, _ := billing.DefinitionFor(billing.CategoryRunLaunches)
	period, err := sqliteStore.OpenPeriod(ctx, tenantID, definition, now)
	if err != nil {
		t.Fatalf("OpenPeriod returned error: %v", err)
	}
	operationKey := billing.RunOperationKey(tenantID, "ambiguous_restart", "")
	if err := sqliteStore.SaveReservation(ctx, billing.UsageReservation{
		ReservationID:    "reservation_operator_action_duplicate",
		TenantID:         tenantID,
		Category:         billing.CategoryRunLaunches,
		QuotaPeriodID:    period.QuotaPeriodID,
		OperationKey:     operationKey,
		AmountReserved:   1,
		Status:           billing.ReservationStatusOperatorActionNeeded,
		CreatedAt:        now,
		UpdatedAt:        now,
		RecoveryReason:   "restart outcome could not be proven",
		ReservationPoint: "POST /v1/runs before runtime.CreateRun",
	}); err != nil {
		t.Fatalf("SaveReservation returned error: %v", err)
	}

	result, err := billing.NewManager(sqliteStore).Reserve(ctx, billing.ReserveInput{TenantID: tenantID, Category: billing.CategoryRunLaunches, Amount: 1, OperationKey: operationKey, Hosted: true})
	if !errors.Is(err, billing.ErrOperatorActionRequired) || result.Allowed || result.Denial == nil || result.Reservation.Status != billing.ReservationStatusOperatorActionNeeded {
		t.Fatalf("expected operator-action-needed duplicate denial, err=%v result=%+v", err, result)
	}
}

func TestWorkflowLaunchQuotaDeniesBeforeWorkflowPersistence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	if err := sqliteStore.EnsureBillingCatalog(ctx); err != nil {
		t.Fatalf("EnsureBillingCatalog returned error: %v", err)
	}
	tenantID := "ten_r38_workflow_launch_denied"
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{PlanID: "plan_" + tenantID, TenantID: tenantID, PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	limit := int64(0)
	if err := sqliteStore.SaveQuotaOverride(ctx, billing.QuotaOverride{
		QuotaOverrideID: "override_" + tenantID,
		TenantID:        tenantID,
		Category:        billing.CategoryWorkflowLaunches,
		Limit:           &limit,
		EffectiveAt:     now.Add(-time.Minute),
		Reason:          "test exhausted workflow quota",
	}); err != nil {
		t.Fatalf("SaveQuotaOverride returned error: %v", err)
	}
	runtimeManager := runtime.NewManager()
	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "deny workflow launch"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRunForTenantSafe(ctx, run, tenantID); err != nil {
		t.Fatalf("UpsertRunForTenantSafe returned error: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/workflows", bytes.NewBufferString(`{"goal":"deny before workflow persistence"}`))
	req = req.WithContext(withTenantContext(req.Context(), r38BillingOwnerContext(tenantID)))
	rec := httptest.NewRecorder()

	handleRunWorkflows(config.Config{Environment: config.EnvironmentTest}, runtimeManager, nil, nil, nil, nil, nil, nil, nil, nil, events.NewBus(), nil, billing.NewManager(sqliteStore), sqliteStore, nil, nil, rec, req, run.RunID)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected quota denial status 429, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`quota_denied:workflow_launches_exhausted`)) {
		t.Fatalf("expected stable workflow quota denial, got %s", rec.Body.String())
	}
	workflows, err := sqliteStore.ListWorkflows(ctx, string(config.EnvironmentTest), run.RunID)
	if err != nil {
		t.Fatalf("ListWorkflows returned error: %v", err)
	}
	if len(workflows) != 0 {
		t.Fatalf("expected no workflow persisted before quota denial, got %+v", workflows)
	}
}

func TestWorkflowStartQuotaDeniesBeforeRunningPersistence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	if err := sqliteStore.EnsureBillingCatalog(ctx); err != nil {
		t.Fatalf("EnsureBillingCatalog returned error: %v", err)
	}
	tenantID := "ten_r38_workflow_start_denied"
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{PlanID: "plan_" + tenantID, TenantID: tenantID, PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	limit := int64(0)
	if err := sqliteStore.SaveQuotaOverride(ctx, billing.QuotaOverride{
		QuotaOverrideID: "override_" + tenantID,
		TenantID:        tenantID,
		Category:        billing.CategoryWorkflowLaunches,
		Limit:           &limit,
		EffectiveAt:     now.Add(-time.Minute),
		Reason:          "test exhausted workflow quota",
	}); err != nil {
		t.Fatalf("SaveQuotaOverride returned error: %v", err)
	}
	cfg := config.Config{Environment: config.EnvironmentTest}
	runtimeManager := runtime.NewManager()
	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "deny workflow start"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRunForTenantSafe(ctx, run, tenantID); err != nil {
		t.Fatalf("UpsertRunForTenantSafe returned error: %v", err)
	}
	workflow := orchestration.Workflow{
		WorkflowID:       "wf_r38_start_denied",
		RunID:            run.RunID,
		EnvironmentScope: string(config.EnvironmentTest),
		Goal:             "planned before denied start",
		Status:           orchestration.WorkflowStatusPlanned,
		PlanSummary:      "test planned workflow",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := persistWorkflowDetail(ctx, sqliteStore, workflow); err != nil {
		t.Fatalf("persistWorkflowDetail returned error: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/workflows/"+workflow.WorkflowID+"/start", nil)
	req = req.WithContext(withTenantContext(req.Context(), r38BillingOwnerContext(tenantID)))
	rec := httptest.NewRecorder()

	handleRunWorkflowStart(cfg, runtimeManager, nil, nil, nil, nil, nil, nil, nil, nil, events.NewBus(), nil, billing.NewManager(sqliteStore), sqliteStore, nil, nil, rec, req, run.RunID, workflow.WorkflowID)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected quota denial status 429, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`quota_denied:workflow_launches_exhausted`)) {
		t.Fatalf("expected stable workflow quota denial, got %s", rec.Body.String())
	}
	got, ok, err := sqliteStore.GetWorkflow(ctx, string(config.EnvironmentTest), run.RunID, workflow.WorkflowID)
	if err != nil || !ok {
		t.Fatalf("GetWorkflow err=%v ok=%v", err, ok)
	}
	if got.Status != orchestration.WorkflowStatusPlanned {
		t.Fatalf("expected workflow to remain planned after quota denial, got %s", got.Status)
	}
}

func TestRuntimeToolCallQuotaDeniesBeforeToolCallCreation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	if err := sqliteStore.EnsureBillingCatalog(ctx); err != nil {
		t.Fatalf("EnsureBillingCatalog returned error: %v", err)
	}
	tenantID := "ten_r38_tool_call_denied"
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{PlanID: "plan_" + tenantID, TenantID: tenantID, PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	limit := int64(0)
	if err := sqliteStore.SaveQuotaOverride(ctx, billing.QuotaOverride{
		QuotaOverrideID: "override_" + tenantID,
		TenantID:        tenantID,
		Category:        billing.CategoryRuntimeToolCalls,
		Limit:           &limit,
		EffectiveAt:     now.Add(-time.Minute),
		Reason:          "test exhausted tool call quota",
	}); err != nil {
		t.Fatalf("SaveQuotaOverride returned error: %v", err)
	}
	runtimeManager := runtime.NewManager()
	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "deny tool call"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := runtimeManager.CreateStep(run.RunID, runtime.CreateStepInput{Title: "deny tool call"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	capabilitySupervisor := capabilities.NewSupervisor()
	if _, _, err := capabilitySupervisor.Register(capabilities.RegisterInput{CapabilityID: "search", Kind: "knowledge", DisplayName: "Search"}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", bytes.NewBufferString(`{"capabilityId":"search","toolName":"search","input":{"q":"quota"}}`))
	req = req.WithContext(withTenantContext(req.Context(), r38BillingOwnerContext(tenantID)))
	rec := httptest.NewRecorder()

	handleRunStepToolCalls(config.Config{Environment: config.EnvironmentTest}, runtimeManager, nil, capabilitySupervisor, nil, nil, nil, nil, events.NewBus(), billing.NewManager(sqliteStore), sqliteStore, checkpoints.NewManager(sqliteStore, runtimeManager), rec, req, run.RunID, step.StepID)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected quota denial status 429, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`quota_denied:runtime_tool_calls_exhausted`)) {
		t.Fatalf("expected stable runtime tool-call quota denial, got %s", rec.Body.String())
	}
	toolCalls, err := runtimeManager.ListToolCalls(run.RunID, step.StepID)
	if err != nil {
		t.Fatalf("ListToolCalls returned error: %v", err)
	}
	if len(toolCalls) != 0 {
		t.Fatalf("expected no tool calls to be created before quota denial, got %+v", toolCalls)
	}
}

func TestRuntimeToolCallQuotaCommitsAfterToolCallPersistence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	if err := sqliteStore.EnsureBillingCatalog(ctx); err != nil {
		t.Fatalf("EnsureBillingCatalog returned error: %v", err)
	}
	tenantID := "ten_r38_tool_call_allowed"
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{PlanID: "plan_" + tenantID, TenantID: tenantID, PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	runtimeManager := runtime.NewManager()
	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "allow tool call"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := runtimeManager.CreateStep(run.RunID, runtime.CreateStepInput{Title: "allow tool call"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	capabilitySupervisor := capabilities.NewSupervisor()
	if _, _, err := capabilitySupervisor.Register(capabilities.RegisterInput{CapabilityID: "search", Kind: "knowledge", DisplayName: "Search"}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", bytes.NewBufferString(`{"capabilityId":"search","toolName":"search","input":{"q":"quota"}}`))
	req = req.WithContext(withTenantContext(req.Context(), r38BillingOwnerContext(tenantID)))
	rec := httptest.NewRecorder()

	handleRunStepToolCalls(config.Config{Environment: config.EnvironmentTest}, runtimeManager, nil, capabilitySupervisor, nil, nil, nil, nil, events.NewBus(), billing.NewManager(sqliteStore), sqliteStore, checkpoints.NewManager(sqliteStore, runtimeManager), rec, req, run.RunID, step.StepID)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected tool call creation status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	definition, _ := billing.DefinitionFor(billing.CategoryRuntimeToolCalls)
	period, err := sqliteStore.OpenPeriod(ctx, tenantID, definition, now)
	if err != nil {
		t.Fatalf("OpenPeriod returned error: %v", err)
	}
	counter, found, err := sqliteStore.UsageCounter(ctx, tenantID, billing.CategoryRuntimeToolCalls, period.QuotaPeriodID)
	if err != nil || !found {
		t.Fatalf("UsageCounter err=%v found=%v", err, found)
	}
	if counter.CommittedAmount != 1 || counter.ReservedAmount != 0 {
		t.Fatalf("expected committed runtime tool-call usage, got %+v", counter)
	}
}
