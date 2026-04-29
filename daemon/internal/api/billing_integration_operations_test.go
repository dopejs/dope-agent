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
	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestCalendarIntegrationOperationQuotaDeniesBeforeBackendOperation(t *testing.T) {
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
	tenantID := "ten_r38_calendar_operation_denied"
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{PlanID: "plan_" + tenantID, TenantID: tenantID, PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	limit := int64(0)
	if err := sqliteStore.SaveQuotaOverride(ctx, billing.QuotaOverride{
		QuotaOverrideID: "override_" + tenantID,
		TenantID:        tenantID,
		Category:        billing.CategoryIntegrationOperations,
		Limit:           &limit,
		EffectiveAt:     now.Add(-time.Minute),
		Reason:          "test exhausted integration operation quota",
	}); err != nil {
		t.Fatalf("SaveQuotaOverride returned error: %v", err)
	}

	integrationManager := integrations.NewManager("test")
	seedHealthyCalendarIntegration(t, integrationManager, sqliteStore, "calendar-quota-denied", true)
	calendarManager := calendar.NewManager("test")
	req := httptest.NewRequest(http.MethodPost, "/v1/calendar/availability/queries", bytes.NewBufferString(`{
		"integrationId":"calendar-quota-denied",
		"windowStart":"2026-04-29T00:00:00Z",
		"windowEnd":"2026-04-29T01:00:00Z",
		"timezone":"UTC"
	}`))
	req = req.WithContext(withTenantContext(req.Context(), r38BillingOwnerContext(tenantID)))
	rec := httptest.NewRecorder()

	handleCalendarAvailabilityQueries(config.Config{Environment: config.EnvironmentTest}, calendarManager, integrationManager, events.NewBus(), billing.NewManager(sqliteStore), sqliteStore, rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected quota denial status 429, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`quota_denied:integration_operations_exhausted`)) {
		t.Fatalf("expected stable integration quota denial, got %s", rec.Body.String())
	}
	if operations := calendarManager.ListOperations(calendar.OperationFilter{}); len(operations) != 0 {
		t.Fatalf("expected no calendar operation before quota denial, got %+v", operations)
	}
}

func TestMailIntegrationOperationQuotaDeniesBeforeBackendOperation(t *testing.T) {
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
	tenantID := "ten_r38_mail_operation_denied"
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{PlanID: "plan_" + tenantID, TenantID: tenantID, PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	limit := int64(0)
	if err := sqliteStore.SaveQuotaOverride(ctx, billing.QuotaOverride{
		QuotaOverrideID: "override_" + tenantID,
		TenantID:        tenantID,
		Category:        billing.CategoryIntegrationOperations,
		Limit:           &limit,
		EffectiveAt:     now.Add(-time.Minute),
		Reason:          "test exhausted integration operation quota",
	}); err != nil {
		t.Fatalf("SaveQuotaOverride returned error: %v", err)
	}

	integrationManager := integrations.NewManager("test")
	seedHealthyMailIntegration(t, integrationManager, sqliteStore, "mail-quota-denied", true)
	mailManager := mail.NewManager("test")
	req := httptest.NewRequest(http.MethodPost, "/v1/mail/drafts", bytes.NewBufferString(`{
		"integrationId":"mail-quota-denied",
		"to":["bob@example.com"],
		"subject":"quota",
		"body":"denied"
	}`))
	req = req.WithContext(withTenantContext(req.Context(), r38BillingOwnerContext(tenantID)))
	rec := httptest.NewRecorder()

	handleMailDrafts(config.Config{Environment: config.EnvironmentTest}, mailManager, integrationManager, events.NewBus(), billing.NewManager(sqliteStore), sqliteStore, rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected quota denial status 429, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`quota_denied:integration_operations_exhausted`)) {
		t.Fatalf("expected stable integration quota denial, got %s", rec.Body.String())
	}
	if operations := mailManager.ListOperations(mail.OperationFilter{}); len(operations) != 0 {
		t.Fatalf("expected no mail operation before quota denial, got %+v", operations)
	}
}

func TestWorkflowMailIntegrationOperationQuotaDeniesBeforeBackendOperation(t *testing.T) {
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
	tenantID := "ten_r38_workflow_mail_operation_denied"
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{PlanID: "plan_" + tenantID, TenantID: tenantID, PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	limit := int64(0)
	if err := sqliteStore.SaveQuotaOverride(ctx, billing.QuotaOverride{
		QuotaOverrideID: "override_" + tenantID,
		TenantID:        tenantID,
		Category:        billing.CategoryIntegrationOperations,
		Limit:           &limit,
		EffectiveAt:     now.Add(-time.Minute),
		Reason:          "test exhausted integration operation quota",
	}); err != nil {
		t.Fatalf("SaveQuotaOverride returned error: %v", err)
	}

	runtimeManager := runtime.NewManager()
	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "deny workflow mail operation"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRunForTenantSafe(ctx, run, tenantID); err != nil {
		t.Fatalf("UpsertRunForTenantSafe returned error: %v", err)
	}
	workflow := orchestration.Workflow{
		WorkflowID:       "wf_r38_mail_operation_denied",
		RunID:            run.RunID,
		EnvironmentScope: string(config.EnvironmentTest),
		Goal:             "create a draft after quota check",
		Status:           orchestration.WorkflowStatusPlanned,
		PlanSummary:      "single mail operation step",
		CreatedAt:        now,
		UpdatedAt:        now,
		Steps: []orchestration.WorkflowStep{{
			WorkflowStepID: "wfstep_r38_mail_operation_denied",
			WorkflowID:     "wf_r38_mail_operation_denied",
			Title:          "Create draft",
			Position:       1,
			ConsumerKind:   "mail",
			ConsumerID:     "mail-workflow-quota-denied",
			ToolName:       string(mail.OperationClassCreateDraft),
			Status:         orchestration.StepStatusPlanned,
			Input: mail.Action{
				OperationClass: mail.OperationClassCreateDraft,
				IntegrationID:  "mail-workflow-quota-denied",
				To:             []string{"bob@example.com"},
				Subject:        "quota",
				Body:           "denied",
			},
			MaxAttempts: 1,
			CreatedAt:   now,
			UpdatedAt:   now,
		}},
	}
	if err := persistWorkflowDetail(ctx, sqliteStore, workflow); err != nil {
		t.Fatalf("persistWorkflowDetail returned error: %v", err)
	}

	integrationManager := integrations.NewManager("test")
	seedHealthyMailIntegration(t, integrationManager, sqliteStore, "mail-workflow-quota-denied", true)
	mailManager := mail.NewManager("test")
	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/workflows/"+workflow.WorkflowID+"/start", nil)
	req = req.WithContext(withTenantContext(req.Context(), r38BillingOwnerContext(tenantID)))
	rec := httptest.NewRecorder()

	handleRunWorkflowStart(config.Config{Environment: config.EnvironmentTest}, runtimeManager, nil, nil, nil, nil, nil, integrationManager, nil, mailManager, events.NewBus(), nil, billing.NewManager(sqliteStore), sqliteStore, checkpoints.NewManager(sqliteStore, runtimeManager), nil, rec, req, run.RunID, workflow.WorkflowID)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected quota denial status 429, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`quota_denied:integration_operations_exhausted`)) {
		t.Fatalf("expected stable integration quota denial, got %s", rec.Body.String())
	}
	if operations := mailManager.ListOperations(mail.OperationFilter{}); len(operations) != 0 {
		t.Fatalf("expected no workflow mail operation before quota denial, got %+v", operations)
	}
}

func TestBackgroundWorkflowMailStepRestoresTenantContextBeforeQuotaGate(t *testing.T) {
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
	tenantID := "ten_r38_background_mail_denied"
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{PlanID: "plan_" + tenantID, TenantID: tenantID, PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	limit := int64(0)
	if err := sqliteStore.SaveQuotaOverride(ctx, billing.QuotaOverride{
		QuotaOverrideID: "override_" + tenantID,
		TenantID:        tenantID,
		Category:        billing.CategoryIntegrationOperations,
		Limit:           &limit,
		EffectiveAt:     now.Add(-time.Minute),
		Reason:          "test exhausted background mail quota",
	}); err != nil {
		t.Fatalf("SaveQuotaOverride returned error: %v", err)
	}

	runtimeManager := runtime.NewManager()
	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "background workflow mail quota"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRunForTenantSafe(ctx, run, tenantID); err != nil {
		t.Fatalf("UpsertRunForTenantSafe returned error: %v", err)
	}
	workflow := orchestration.Workflow{
		WorkflowID:       "wf_background_mail_quota",
		RunID:            run.RunID,
		EnvironmentScope: string(config.EnvironmentTest),
		Goal:             "create draft from background continuation",
		Status:           orchestration.WorkflowStatusRunning,
		CreatedAt:        now,
		UpdatedAt:        now,
		StartedAt:        &now,
		Steps: []orchestration.WorkflowStep{{
			WorkflowStepID: "wfstep_background_mail_quota",
			WorkflowID:     "wf_background_mail_quota",
			Title:          "Create draft",
			Position:       1,
			ConsumerKind:   "mail",
			ConsumerID:     "mail-background-quota-denied",
			ToolName:       string(mail.OperationClassCreateDraft),
			Status:         orchestration.StepStatusReady,
			Input: mail.Action{
				OperationClass: mail.OperationClassCreateDraft,
				IntegrationID:  "mail-background-quota-denied",
				To:             []string{"bob@example.com"},
				Subject:        "quota",
				Body:           "denied",
			},
			MaxAttempts: 1,
			CreatedAt:   now,
			UpdatedAt:   now,
		}},
	}
	if err := persistWorkflowDetail(ctx, sqliteStore, workflow); err != nil {
		t.Fatalf("persistWorkflowDetail returned error: %v", err)
	}
	integrationManager := integrations.NewManager("test")
	seedHealthyMailIntegration(t, integrationManager, sqliteStore, "mail-background-quota-denied", true)
	mailManager := mail.NewManager("test")

	_, err = advanceWorkflowExecution(events.WithEnvironmentScope(context.Background(), string(config.EnvironmentTest)), config.Config{Environment: config.EnvironmentTest}, runtimeManager, nil, nil, nil, nil, nil, integrationManager, nil, mailManager, events.NewBus(), nil, billing.NewManager(sqliteStore), sqliteStore, checkpoints.NewManager(sqliteStore, runtimeManager), nil, workflow)
	if !errors.Is(err, billing.ErrQuotaDenied) {
		t.Fatalf("expected ErrQuotaDenied from restored tenant context, got %v", err)
	}
	if operations := mailManager.ListOperations(mail.OperationFilter{}); len(operations) != 0 {
		t.Fatalf("expected no background workflow mail operation before quota denial, got %+v", operations)
	}
}
