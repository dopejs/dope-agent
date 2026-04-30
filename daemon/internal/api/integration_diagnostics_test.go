package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/opsreadiness"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestIntegrationDiagnosticsAPIRequiresPermissionAndDoesNotDiscloseCrossTenantState(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	manager := integrations.NewManager("test")
	if _, err := manager.Create(integrations.CreateInput{
		TenantID:      "ten_a",
		IntegrationID: "integration_a",
		DomainKind:    "calendar",
		DisplayName:   "Tenant A Calendar",
		BackendBinding: integrations.BackendBinding{
			BackendKind: integrations.BackendKind("feishu_lark"),
		},
	}); err != nil {
		t.Fatalf("Create tenant A integration: %v", err)
	}
	if _, err := manager.Create(integrations.CreateInput{
		TenantID:      "ten_b",
		IntegrationID: "integration_b",
		DomainKind:    "calendar",
		DisplayName:   "Tenant B Secret Calendar",
		BackendBinding: integrations.BackendBinding{
			BackendKind: integrations.BackendKind("feishu_lark"),
		},
	}); err != nil {
		t.Fatalf("Create tenant B integration: %v", err)
	}

	deniedCtx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_a", PrincipalID: "prn_a"})
	denied := httptest.NewRequest(http.MethodGet, "/v1/integrations/integration_a/diagnostics", nil).WithContext(deniedCtx)
	deniedRec := httptest.NewRecorder()
	handleIntegrationRoutes(config.Config{}, manager, nil, sqliteStore, deniedRec, denied)
	if deniedRec.Code != http.StatusForbidden || !strings.Contains(deniedRec.Body.String(), "permission_missing") {
		t.Fatalf("expected permission denial, status=%d body=%s", deniedRec.Code, deniedRec.Body.String())
	}

	allowedCtx := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_a",
		PrincipalID: "prn_a",
		Permissions: []identity.Permission{identity.PermissionIntegrationDiagnosticsRead},
	})
	crossTenant := httptest.NewRequest(http.MethodGet, "/v1/integrations/integration_b/diagnostics", nil).WithContext(allowedCtx)
	crossTenantRec := httptest.NewRecorder()
	handleIntegrationRoutes(config.Config{}, manager, nil, sqliteStore, crossTenantRec, crossTenant)
	if crossTenantRec.Code != http.StatusNotFound {
		t.Fatalf("expected non-disclosing not found, status=%d body=%s", crossTenantRec.Code, crossTenantRec.Body.String())
	}
	if strings.Contains(crossTenantRec.Body.String(), "Tenant B Secret Calendar") || strings.Contains(crossTenantRec.Body.String(), "integration_b") {
		t.Fatalf("cross-tenant response disclosed resource: %s", crossTenantRec.Body.String())
	}
}

func TestIntegrationDiagnosticsAPIReadsStartsListsAndInspectsRuns(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	manager := integrations.NewManager("test")
	resource, err := manager.Create(integrations.CreateInput{
		TenantID:      "ten_diag",
		IntegrationID: "integration_feishu",
		DomainKind:    "calendar",
		DisplayName:   "Feishu Calendar",
		AccountBinding: integrations.AccountBinding{
			AccountKey: "acct_feishu",
		},
		BackendBinding: integrations.BackendBinding{
			BackendKind: integrations.BackendKind("feishu_lark"),
		},
	})
	if err != nil {
		t.Fatalf("Create integration: %v", err)
	}
	if _, err := manager.UpdateReadinessForTenant(resource.IntegrationID, "ten_diag", integrations.UpdateReadinessInput{
		ReadinessStatus:        integrations.ReadinessStatusDegraded,
		AuthState:              integrations.AuthStateAuthorized,
		HealthState:            integrations.HealthStateDegraded,
		ReadinessReason:        "scope missing for calendar.read with bearer secret-token",
		RequiredOperatorAction: "grant calendar scope",
	}); err != nil {
		t.Fatalf("UpdateReadinessForTenant: %v", err)
	}

	eventBus := events.NewBus()
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_diag",
		PrincipalID: "prn_operator",
		Permissions: []identity.Permission{
			identity.PermissionIntegrationDiagnosticsRead,
			identity.PermissionIntegrationDiagnosticsRun,
		},
	})
	start := httptest.NewRequest(http.MethodPost, "/v1/integrations/integration_feishu/diagnostics/runs", jsonBody(map[string]any{
		"clientKey":    "client-key-1",
		"capabilities": []string{"calendar.read"},
	})).WithContext(ctx)
	startRec := httptest.NewRecorder()
	handleIntegrationRoutes(config.Config{}, manager, eventBus, sqliteStore, startRec, start)
	if startRec.Code != http.StatusCreated {
		t.Fatalf("POST diagnostics run status=%d body=%s", startRec.Code, startRec.Body.String())
	}
	var run integrations.DiagnosticRun
	if err := json.Unmarshal(startRec.Body.Bytes(), &run); err != nil {
		t.Fatalf("decode run: %v", err)
	}
	if run.Status != integrations.DiagnosticRunCompleted || len(run.ResultIDs) != 1 {
		t.Fatalf("expected completed run with result, got %+v", run)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/integrations/integration_feishu/diagnostics", nil).WithContext(ctx)
	listRec := httptest.NewRecorder()
	handleIntegrationRoutes(config.Config{}, manager, eventBus, sqliteStore, listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET diagnostics status=%d body=%s", listRec.Code, listRec.Body.String())
	}
	body := listRec.Body.String()
	if !strings.Contains(body, `"reasonCode":"scope_missing"`) || !strings.Contains(body, `"remediationOwner":"tenant_admin"`) {
		t.Fatalf("diagnostics response missing expected classification: %s", body)
	}
	if strings.Contains(strings.ToLower(body), "secret-token") || strings.Contains(strings.ToLower(body), "bearer") {
		t.Fatalf("diagnostics response leaked credential material: %s", body)
	}

	runListReq := httptest.NewRequest(http.MethodGet, "/v1/integration-diagnostics/runs?integrationId=integration_feishu", nil).WithContext(ctx)
	runListRec := httptest.NewRecorder()
	handleIntegrationDiagnosticRuns(sqliteStore, runListRec, runListReq, []string{"runs"})
	if runListRec.Code != http.StatusOK || !strings.Contains(runListRec.Body.String(), run.DiagnosticRunID) {
		t.Fatalf("GET runs status=%d body=%s", runListRec.Code, runListRec.Body.String())
	}

	runDetailReq := httptest.NewRequest(http.MethodGet, "/v1/integration-diagnostics/runs/"+run.DiagnosticRunID, nil).WithContext(ctx)
	runDetailRec := httptest.NewRecorder()
	handleIntegrationDiagnosticRuns(sqliteStore, runDetailRec, runDetailReq, []string{"runs", run.DiagnosticRunID})
	if runDetailRec.Code != http.StatusOK || !strings.Contains(runDetailRec.Body.String(), `"diagnosticRunId":"`+run.DiagnosticRunID+`"`) {
		t.Fatalf("GET run detail status=%d body=%s", runDetailRec.Code, runDetailRec.Body.String())
	}
}

func TestIntegrationDiagnosticRunUsesFeishuLarkProbeEvidence(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	manager := integrations.NewManager("test")
	if _, err := manager.Create(integrations.CreateInput{
		TenantID:      "ten_probe",
		IntegrationID: "integration_feishu_probe",
		DomainKind:    "calendar",
		DisplayName:   "Feishu Probe Calendar",
		AccountBinding: integrations.AccountBinding{
			AccountKey: "acct_feishu_probe",
		},
		BackendBinding: integrations.BackendBinding{
			BackendKind:       integrations.BackendKindFeishuLark,
			SupportsProbeRead: true,
		},
	}); err != nil {
		t.Fatalf("Create integration: %v", err)
	}
	if _, err := manager.UpdateReadinessForTenant("integration_feishu_probe", "ten_probe", integrations.UpdateReadinessInput{
		ReadinessStatus: integrations.ReadinessStatusHealthy,
		AuthState:       integrations.AuthStateAuthorized,
		HealthState:     integrations.HealthStateHealthy,
	}); err != nil {
		t.Fatalf("UpdateReadinessForTenant: %v", err)
	}

	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_probe",
		PrincipalID: "prn_operator",
		Permissions: []identity.Permission{
			identity.PermissionIntegrationDiagnosticsRead,
			identity.PermissionIntegrationDiagnosticsRun,
		},
	})
	start := httptest.NewRequest(http.MethodPost, "/v1/integrations/integration_feishu_probe/diagnostics/runs", jsonBody(map[string]any{
		"clientKey":    "client-key-probe",
		"capabilities": []string{"calendar.read"},
		"reason":       "scope_not_granted",
	})).WithContext(ctx)
	startRec := httptest.NewRecorder()
	handleIntegrationRoutes(config.Config{}, manager, events.NewBus(), sqliteStore, startRec, start)
	if startRec.Code != http.StatusCreated {
		t.Fatalf("POST diagnostics run status=%d body=%s", startRec.Code, startRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/integrations/integration_feishu_probe/diagnostics", nil).WithContext(ctx)
	listRec := httptest.NewRecorder()
	handleIntegrationRoutes(config.Config{}, manager, events.NewBus(), sqliteStore, listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET diagnostics status=%d body=%s", listRec.Code, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), `"reasonCode":"scope_missing"`) {
		t.Fatalf("expected probe evidence to drive scope_missing diagnostic, got %s", listRec.Body.String())
	}
}

func TestIntegrationDiagnosticSmokeUsesFeishuLarkProbeEvidence(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	manager := integrations.NewManager("test")
	if _, err := manager.Create(integrations.CreateInput{
		TenantID:      "ten_smoke_probe",
		IntegrationID: "integration_feishu_smoke",
		DomainKind:    "calendar",
		DisplayName:   "Feishu Smoke Calendar",
		AccountBinding: integrations.AccountBinding{
			AccountKey: "acct_feishu_smoke",
		},
		BackendBinding: integrations.BackendBinding{
			BackendKind:       integrations.BackendKindFeishuLark,
			SupportsProbeRead: true,
		},
	}); err != nil {
		t.Fatalf("Create integration: %v", err)
	}
	if _, err := manager.UpdateReadinessForTenant("integration_feishu_smoke", "ten_smoke_probe", integrations.UpdateReadinessInput{
		ReadinessStatus: integrations.ReadinessStatusHealthy,
		AuthState:       integrations.AuthStateAuthorized,
		HealthState:     integrations.HealthStateHealthy,
	}); err != nil {
		t.Fatalf("UpdateReadinessForTenant: %v", err)
	}

	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_smoke_probe",
		PrincipalID: "prn_operator",
		Permissions: []identity.Permission{identity.PermissionIntegrationDiagnosticsSmoke},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/integration-diagnostics/smoke", jsonBody(map[string]any{
		"reportId":      "smoke_feishu_probe",
		"integrationId": "integration_feishu_smoke",
		"probes": []map[string]any{{
			"domainKind":               "calendar",
			"probeAction":              "calendar.read",
			"safeCredentialsAvailable": true,
			"tenantApprovalAvailable":  true,
			"providerAvailable":        true,
			"supported":                true,
			"readOnlyOrReversible":     true,
			"providerEvidence":         map[string]any{"code": "scope_not_granted"},
		}},
	})).WithContext(ctx)
	rec := httptest.NewRecorder()
	handleIntegrationDiagnosticSmoke(manager, events.NewBus(), sqliteStore, rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST smoke status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"reasonCode":"scope_missing"`) || !strings.Contains(rec.Body.String(), `"result":"failed"`) {
		t.Fatalf("expected smoke report to use Feishu/Lark probe evidence, got %s", rec.Body.String())
	}
}

func TestIntegrationDiagnosticSmokeAPIPersistsPublishesAndAuditsReport(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	manager := integrations.NewManager("test")
	if _, err := manager.Create(integrations.CreateInput{
		TenantID:      "ten_smoke",
		IntegrationID: "integration_smoke",
		DomainKind:    "calendar",
		DisplayName:   "Smoke Calendar",
		AccountBinding: integrations.AccountBinding{
			AccountKey: "acct_smoke",
		},
		BackendBinding: integrations.BackendBinding{
			BackendKind:           integrations.BackendKindFakeLocal,
			SupportsProbeRead:     true,
			SupportsProbeMutation: false,
		},
	}); err != nil {
		t.Fatalf("Create integration: %v", err)
	}
	if _, err := manager.UpdateReadinessForTenant("integration_smoke", "ten_smoke", integrations.UpdateReadinessInput{
		ReadinessStatus: integrations.ReadinessStatusHealthy,
		AuthState:       integrations.AuthStateAuthorized,
		HealthState:     integrations.HealthStateHealthy,
	}); err != nil {
		t.Fatalf("UpdateReadinessForTenant: %v", err)
	}

	eventBus := events.NewBus()
	ch, cancel := eventBus.Subscribe(events.Filter{Category: "integration"})
	defer cancel()
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_smoke",
		PrincipalID: "prn_operator",
		Permissions: []identity.Permission{identity.PermissionIntegrationDiagnosticsSmoke},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/integration-diagnostics/smoke", jsonBody(map[string]any{
		"reportId":      "smoke_api_1",
		"integrationId": "integration_smoke",
		"probes": []map[string]any{{
			"domainKind":               "calendar",
			"probeAction":              "calendar.readiness.inspect",
			"safeCredentialsAvailable": true,
			"tenantApprovalAvailable":  true,
			"providerAvailable":        true,
			"supported":                true,
			"readOnlyOrReversible":     true,
		}},
	})).WithContext(ctx)
	rec := httptest.NewRecorder()
	handleIntegrationDiagnosticSmoke(manager, eventBus, sqliteStore, rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST smoke status=%d body=%s", rec.Code, rec.Body.String())
	}
	var report opsreadiness.SmokeMatrixReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode smoke report: %v", err)
	}
	if report.Status != opsreadiness.SmokeReportCompleted || len(report.ProbeOutcomes) != 1 || report.ProbeOutcomes[0].Result != opsreadiness.SmokeProbePassed {
		t.Fatalf("expected passed smoke report, got %+v", report)
	}
	foundSmokeEvent := false
	for !foundSmokeEvent {
		select {
		case event := <-ch:
			if event.Name == events.IntegrationDiagnosticSmokeCompletedName {
				if event.Resource.ID != report.SmokeReportID {
					t.Fatalf("expected smoke event for report %s, got %+v", report.SmokeReportID, event)
				}
				foundSmokeEvent = true
			}
		default:
			t.Fatalf("expected smoke completed event")
		}
	}
	audits, err := sqliteStore.ListTenantAuditEvents(context.Background(), identity.AuditEventFilter{TenantID: "ten_smoke", EventKind: "integration_diagnostic.audit_recorded", Limit: 10})
	if err != nil {
		t.Fatalf("ListTenantAuditEvents: %v", err)
	}
	foundAudit := false
	for _, item := range audits {
		if item.Document["action"] == "diagnostic_smoke.published" && item.Document["smokeReportId"] == "smoke_api_1" {
			foundAudit = true
		}
	}
	if !foundAudit {
		t.Fatalf("expected smoke publication audit, got %+v", audits)
	}
}

func TestIntegrationDiagnosticRetentionApplyPublishesEventAndAudit(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	createdAt := time.Now().UTC().Add(-91 * 24 * time.Hour)
	record := integrations.NewDiagnosticRetentionRecord("ten_retention", "diagnostic_run", "diag_run_expired", createdAt)
	if err := sqliteStore.SaveDiagnosticRetentionRecord(context.Background(), record); err != nil {
		t.Fatalf("SaveDiagnosticRetentionRecord: %v", err)
	}

	eventBus := events.NewBus()
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_retention",
		PrincipalID: "prn_operator",
		Permissions: []identity.Permission{identity.PermissionIntegrationDiagnosticsRun},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/integration-diagnostics/retention/apply", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handleIntegrationDiagnosticRetentionApply(eventBus, sqliteStore, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST retention apply status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"retentionState":"expired"`) {
		t.Fatalf("expected expired retention response, got %s", rec.Body.String())
	}
	eventItems := eventBus.List(events.Filter{Category: "integration"})
	foundEvent := false
	for _, item := range eventItems {
		if item.Name == events.IntegrationDiagnosticRetentionAppliedName && item.Resource.ID == record.RetentionRecordID {
			foundEvent = true
		}
	}
	if !foundEvent {
		t.Fatalf("expected retention applied event, got %+v", eventItems)
	}
	audits, err := sqliteStore.ListTenantAuditEvents(context.Background(), identity.AuditEventFilter{TenantID: "ten_retention", EventKind: "integration_diagnostic.audit_recorded", Limit: 10})
	if err != nil {
		t.Fatalf("ListTenantAuditEvents: %v", err)
	}
	foundAudit := false
	for _, item := range audits {
		if item.Document["action"] == "diagnostic_retention.applied" && item.Document["targetId"] == record.TargetID {
			foundAudit = true
		}
	}
	if !foundAudit {
		t.Fatalf("expected retention audit, got %+v", audits)
	}
}
