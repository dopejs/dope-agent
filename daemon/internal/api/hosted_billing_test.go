package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func r38BillingTenantContext(tenantID string, role identity.Role, permissions ...identity.Permission) identity.TenantContext {
	if len(permissions) == 0 {
		permissions = identity.PermissionsForRole(role, identity.StatusActive)
	}
	return identity.TenantContext{
		TenantID:    tenantID,
		PrincipalID: "prn_" + tenantID,
		TokenID:     "tok_" + tenantID,
		Role:        role,
		Permissions: append([]identity.Permission(nil), permissions...),
		ResolvedAt:  time.Now().UTC(),
	}
}

func r38BillingOwnerContext(tenantID string) identity.TenantContext {
	return r38BillingTenantContext(tenantID, identity.RoleOwner)
}

func r38BillingAdminContext(tenantID string) identity.TenantContext {
	return r38BillingTenantContext(tenantID, identity.RoleAdmin)
}

func r38BillingOperatorContext(tenantID string) identity.TenantContext {
	return r38BillingTenantContext(tenantID, identity.RoleOperator)
}

func r38BillingViewerContext(tenantID string) identity.TenantContext {
	return r38BillingTenantContext(tenantID, identity.RoleViewer)
}

func r38BillingRequest(method, target string, tenantContext identity.TenantContext, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	return req.WithContext(withTenantContext(req.Context(), tenantContext))
}

func r47BillingSeedTenant(t *testing.T, ctx context.Context, sqliteStore *store.SQLiteStore, tenantID string, now time.Time) {
	t.Helper()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{
		PlanID:          "plan_" + tenantID,
		TenantID:        tenantID,
		PlanKey:         "finite",
		Status:          billing.PlanStatusActive,
		EnforcementMode: billing.EnforcementModeEnforced,
		EffectiveAt:     now.Add(-time.Hour),
	}); err != nil {
		t.Fatalf("SavePlan(%s) returned error: %v", tenantID, err)
	}
}

func r47BillingSeedDenial(t *testing.T, ctx context.Context, sqliteStore *store.SQLiteStore, tenantID string, category billing.Category, denialID string, reasonCode string, now time.Time) billing.QuotaDenial {
	t.Helper()
	definition, ok := billing.DefinitionFor(category)
	if !ok {
		t.Fatalf("missing definition for %s", category)
	}
	period, err := sqliteStore.OpenPeriod(ctx, tenantID, definition, now)
	if err != nil {
		t.Fatalf("OpenPeriod(%s, %s) returned error: %v", tenantID, category, err)
	}
	denial := billing.QuotaDenial{
		DenialID:          denialID,
		TenantID:          tenantID,
		Category:          category,
		QuotaPeriodID:     period.QuotaPeriodID,
		OperationKey:      "tenant:" + tenantID + ":" + string(category) + ":client_1",
		ReasonCode:        reasonCode,
		RequestedAmount:   1,
		RemainingAmount:   0,
		GuardedEntryPoint: definition.ReservationRule,
		CreatedAt:         now,
	}
	if denial.ReasonCode == "" {
		denial.ReasonCode = definition.DenialReasonCode
	}
	if err := sqliteStore.AppendQuotaDenial(ctx, denial); err != nil {
		t.Fatalf("AppendQuotaDenial(%s) returned error: %v", denialID, err)
	}
	if err := sqliteStore.AppendUsageEvent(ctx, billing.UsageEvent{
		UsageEventID:  "usage_event_" + denialID,
		TenantID:      tenantID,
		Category:      category,
		QuotaPeriodID: period.QuotaPeriodID,
		OperationKey:  denial.OperationKey,
		EventKind:     billing.UsageEventDenial,
		ReasonCode:    denial.ReasonCode,
		Outcome:       "denied",
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("AppendUsageEvent(%s) returned error: %v", denialID, err)
	}
	return denial
}

func TestHostedBillingInspectionIsTenantScoped(t *testing.T) {
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
	now := time.Now().UTC().Add(-time.Minute)
	for _, tenantID := range []string{"ten_r38_a", "ten_r38_b"} {
		if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{
			PlanID:          "plan_" + tenantID,
			TenantID:        tenantID,
			PlanKey:         "finite",
			Status:          billing.PlanStatusActive,
			EnforcementMode: billing.EnforcementModeEnforced,
			EffectiveAt:     now,
		}); err != nil {
			t.Fatalf("SavePlan(%s) returned error: %v", tenantID, err)
		}
	}
	manager := billing.NewManager(sqliteStore)

	rec := httptest.NewRecorder()
	handleHostedBilling(config.Config{Environment: config.EnvironmentTest}, manager, rec, r38BillingRequest(http.MethodGet, "/v1/billing/usage", r38BillingOwnerContext("ten_r38_a"), ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected usage status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"tenantId":"ten_r38_a"`)) || bytes.Contains(rec.Body.Bytes(), []byte(`"tenantId":"ten_r38_b"`)) {
		t.Fatalf("expected tenant A-only billing response, got %s", rec.Body.String())
	}

	viewerRec := httptest.NewRecorder()
	handleHostedBilling(config.Config{Environment: config.EnvironmentTest}, manager, viewerRec, r38BillingRequest(http.MethodGet, "/v1/billing/plan", r38BillingViewerContext("ten_r38_a"), ""))
	if viewerRec.Code != http.StatusForbidden {
		t.Fatalf("expected viewer billing.view denial, got %d: %s", viewerRec.Code, viewerRec.Body.String())
	}
}

func TestHostedBillingInspectionProjectsFiniteUnlimitedAndDevelopmentPlans(t *testing.T) {
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
	now := time.Now().UTC().Add(-time.Minute)
	for _, plan := range []billing.TenantPlan{
		{PlanID: "plan_finite", TenantID: "ten_r38_finite", PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now},
		{PlanID: "plan_unlimited", TenantID: "ten_r38_unlimited", PlanKey: "unlimited", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeUnlimited, EffectiveAt: now},
	} {
		if err := sqliteStore.SavePlan(ctx, plan); err != nil {
			t.Fatalf("SavePlan(%s) returned error: %v", plan.TenantID, err)
		}
	}
	manager := billing.NewManager(sqliteStore)

	tests := []struct {
		tenantID        string
		wantPlanKey     string
		wantEnforcement billing.EnforcementMode
	}{
		{tenantID: "ten_r38_finite", wantPlanKey: "finite", wantEnforcement: billing.EnforcementModeEnforced},
		{tenantID: "ten_r38_unlimited", wantPlanKey: "unlimited", wantEnforcement: billing.EnforcementModeUnlimited},
		{tenantID: "ten_r38_development", wantPlanKey: "development", wantEnforcement: billing.EnforcementModeUnlimited},
	}
	for _, tt := range tests {
		t.Run(tt.tenantID, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handleHostedBilling(config.Config{Environment: config.EnvironmentTest}, manager, rec, r38BillingRequest(http.MethodGet, "/v1/billing/usage", r38BillingOwnerContext(tt.tenantID), ""))
			if rec.Code != http.StatusOK {
				t.Fatalf("expected usage status 200, got %d: %s", rec.Code, rec.Body.String())
			}
			var summary billing.UsageSummary
			if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
				t.Fatalf("decode usage summary: %v", err)
			}
			if summary.TenantID != tt.tenantID || summary.PlanKey != tt.wantPlanKey || summary.EnforcementMode != tt.wantEnforcement {
				t.Fatalf("unexpected usage summary: %#v", summary)
			}
			if len(summary.Quotas) != len(billing.RequiredCategories()) {
				t.Fatalf("expected all quota categories, got %d", len(summary.Quotas))
			}
		})
	}
}

func TestHostedBillingInspectionListsOnlyCurrentTenantEvidence(t *testing.T) {
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
	now := time.Now().UTC().Add(-time.Minute)
	for _, tenantID := range []string{"ten_r38_evidence_a", "ten_r38_evidence_b"} {
		if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{PlanID: "plan_" + tenantID, TenantID: tenantID, PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now}); err != nil {
			t.Fatalf("SavePlan(%s) returned error: %v", tenantID, err)
		}
		definition, _ := billing.DefinitionFor(billing.CategoryRunLaunches)
		period, err := sqliteStore.OpenPeriod(ctx, tenantID, definition, now)
		if err != nil {
			t.Fatalf("OpenPeriod(%s) returned error: %v", tenantID, err)
		}
		if err := sqliteStore.AppendQuotaDenial(ctx, billing.QuotaDenial{
			DenialID:          "denial_" + tenantID,
			TenantID:          tenantID,
			Category:          billing.CategoryRunLaunches,
			QuotaPeriodID:     period.QuotaPeriodID,
			OperationKey:      "tenant:" + tenantID + ":run:denied",
			ReasonCode:        "quota_denied:run_launches_exhausted",
			RequestedAmount:   1,
			RemainingAmount:   0,
			GuardedEntryPoint: "POST /v1/runs",
			CreatedAt:         now,
		}); err != nil {
			t.Fatalf("AppendQuotaDenial(%s) returned error: %v", tenantID, err)
		}
		if err := sqliteStore.SaveManualAdjustment(ctx, billing.ManualAdjustment{
			AdjustmentID:         "adjustment_" + tenantID,
			TenantID:             tenantID,
			Category:             billing.CategoryRunLaunches,
			QuotaPeriodID:        period.QuotaPeriodID,
			AmountDelta:          -1,
			Reason:               "tenant scoped correction",
			CreatedByPrincipalID: "prn_admin",
			CreatedAt:            now,
		}); err != nil {
			t.Fatalf("SaveManualAdjustment(%s) returned error: %v", tenantID, err)
		}
	}
	manager := billing.NewManager(sqliteStore)

	usageRec := httptest.NewRecorder()
	handleHostedBilling(config.Config{Environment: config.EnvironmentTest}, manager, usageRec, r38BillingRequest(http.MethodGet, "/v1/billing/usage", r38BillingOwnerContext("ten_r38_evidence_a"), ""))
	if usageRec.Code != http.StatusOK {
		t.Fatalf("expected usage status 200, got %d: %s", usageRec.Code, usageRec.Body.String())
	}
	if !bytes.Contains(usageRec.Body.Bytes(), []byte(`denial_ten_r38_evidence_a`)) || bytes.Contains(usageRec.Body.Bytes(), []byte(`denial_ten_r38_evidence_b`)) {
		t.Fatalf("expected tenant A-only usage evidence, got %s", usageRec.Body.String())
	}
	if !bytes.Contains(usageRec.Body.Bytes(), []byte(`adjustment_ten_r38_evidence_a`)) || bytes.Contains(usageRec.Body.Bytes(), []byte(`adjustment_ten_r38_evidence_b`)) {
		t.Fatalf("expected tenant A-only adjustment evidence, got %s", usageRec.Body.String())
	}

	denialRec := httptest.NewRecorder()
	handleHostedBilling(config.Config{Environment: config.EnvironmentTest}, manager, denialRec, r38BillingRequest(http.MethodGet, "/v1/billing/denials", r38BillingOwnerContext("ten_r38_evidence_a"), ""))
	if denialRec.Code != http.StatusOK {
		t.Fatalf("expected denials status 200, got %d: %s", denialRec.Code, denialRec.Body.String())
	}
	if !bytes.Contains(denialRec.Body.Bytes(), []byte(`denial_ten_r38_evidence_a`)) || bytes.Contains(denialRec.Body.Bytes(), []byte(`denial_ten_r38_evidence_b`)) {
		t.Fatalf("expected tenant A-only denial list, got %s", denialRec.Body.String())
	}
}

func TestHostedBillingPublicQuotaUXRoutesAreTenantScopedAndPermissionGated(t *testing.T) {
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
	now := time.Now().UTC().Add(-time.Minute)
	for _, tenantID := range []string{"ten_r47_a", "ten_r47_b"} {
		if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{PlanID: "plan_" + tenantID, TenantID: tenantID, PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now}); err != nil {
			t.Fatalf("SavePlan(%s) returned error: %v", tenantID, err)
		}
		definition, _ := billing.DefinitionFor(billing.CategoryRunLaunches)
		period, err := sqliteStore.OpenPeriod(ctx, tenantID, definition, now)
		if err != nil {
			t.Fatalf("OpenPeriod(%s) returned error: %v", tenantID, err)
		}
		if err := sqliteStore.AppendQuotaDenial(ctx, billing.QuotaDenial{
			DenialID:          "denial_" + tenantID,
			TenantID:          tenantID,
			Category:          billing.CategoryRunLaunches,
			QuotaPeriodID:     period.QuotaPeriodID,
			OperationKey:      "tenant:" + tenantID + ":run:client_1",
			ReasonCode:        "quota_denied:run_launches_exhausted",
			RequestedAmount:   1,
			RemainingAmount:   0,
			GuardedEntryPoint: "POST /v1/runs",
			CreatedAt:         now,
		}); err != nil {
			t.Fatalf("AppendQuotaDenial(%s) returned error: %v", tenantID, err)
		}
	}
	manager := billing.NewManager(sqliteStore)

	dashboardRec := httptest.NewRecorder()
	handleHostedBilling(config.Config{Environment: config.EnvironmentTest}, manager, dashboardRec, r38BillingRequest(http.MethodGet, "/v1/billing/quota-dashboard", r38BillingOwnerContext("ten_r47_a"), ""))
	if dashboardRec.Code != http.StatusOK {
		t.Fatalf("expected dashboard status 200, got %d: %s", dashboardRec.Code, dashboardRec.Body.String())
	}
	var dashboard billing.TenantQuotaDashboard
	if err := json.Unmarshal(dashboardRec.Body.Bytes(), &dashboard); err != nil {
		t.Fatalf("decode dashboard: %v", err)
	}
	if dashboard.TenantID != "ten_r47_a" || len(dashboard.Sections) == 0 {
		t.Fatalf("unexpected dashboard response: %+v", dashboard)
	}

	detailRec := httptest.NewRecorder()
	handleHostedBilling(config.Config{Environment: config.EnvironmentTest}, manager, detailRec, r38BillingRequest(http.MethodGet, "/v1/billing/denials/denial_ten_r47_a", r38BillingOwnerContext("ten_r47_a"), ""))
	if detailRec.Code != http.StatusOK {
		t.Fatalf("expected denial detail status 200, got %d: %s", detailRec.Code, detailRec.Body.String())
	}
	if bytes.Contains(detailRec.Body.Bytes(), []byte("ten_r47_b")) {
		t.Fatalf("detail leaked other tenant data: %s", detailRec.Body.String())
	}

	crossTenantRec := httptest.NewRecorder()
	handleHostedBilling(config.Config{Environment: config.EnvironmentTest}, manager, crossTenantRec, r38BillingRequest(http.MethodGet, "/v1/billing/denials/denial_ten_r47_b", r38BillingOwnerContext("ten_r47_a"), ""))
	if crossTenantRec.Code != http.StatusNotFound {
		t.Fatalf("expected cross-tenant denial lookup to hide record, got %d: %s", crossTenantRec.Code, crossTenantRec.Body.String())
	}

	viewOnlyRec := httptest.NewRecorder()
	handleHostedBilling(config.Config{Environment: config.EnvironmentTest}, manager, viewOnlyRec, r38BillingRequest(http.MethodPost, "/v1/billing/denials/denial_ten_r47_a/evidence-export", r38BillingTenantContext("ten_r47_a", identity.RoleAdmin, identity.PermissionBillingView), ""))
	if viewOnlyRec.Code != http.StatusForbidden {
		t.Fatalf("expected billing.view-only evidence export denial, got %d: %s", viewOnlyRec.Code, viewOnlyRec.Body.String())
	}

	exportRec := httptest.NewRecorder()
	handleHostedBilling(config.Config{Environment: config.EnvironmentTest}, manager, exportRec, r38BillingRequest(http.MethodPost, "/v1/billing/denials/denial_ten_r47_a/evidence-export", r38BillingTenantContext("ten_r47_a", identity.RoleOperator, identity.PermissionBillingEvidenceExport), ""))
	if exportRec.Code != http.StatusOK {
		t.Fatalf("expected evidence export status 200, got %d: %s", exportRec.Code, exportRec.Body.String())
	}
	if !bytes.Contains(exportRec.Body.Bytes(), []byte(`"redactions"`)) || bytes.Contains(exportRec.Body.Bytes(), []byte("ten_r47_b")) {
		t.Fatalf("unexpected evidence export body: %s", exportRec.Body.String())
	}
}

func TestHostedBillingDenialDetailCoversGuardedCategoriesAndStableClassifications(t *testing.T) {
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
	now := time.Now().UTC().Add(-time.Minute)
	tenantID := "ten_r47_detail"
	r47BillingSeedTenant(t, ctx, sqliteStore, tenantID, now)
	for _, category := range billing.RequiredCategories() {
		denial := r47BillingSeedDenial(t, ctx, sqliteStore, tenantID, category, "denial_"+string(category), "", now)
		rec := httptest.NewRecorder()
		handleHostedBilling(config.Config{Environment: config.EnvironmentTest}, billing.NewManager(sqliteStore), rec, r38BillingRequest(http.MethodGet, "/v1/billing/denials/"+denial.DenialID, r38BillingOwnerContext(tenantID), ""))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected denial detail status 200 for %s, got %d: %s", category, rec.Code, rec.Body.String())
		}
		var detail billing.QuotaDenialDetail
		if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
			t.Fatalf("decode denial detail for %s: %v", category, err)
		}
		if detail.Category != category || detail.Classification != billing.DenialClassificationQuotaExhaustion || detail.OperationRef == detail.OperationKey {
			t.Fatalf("unexpected detail for %s: %+v", category, detail)
		}
	}
	for _, item := range []struct {
		denialID string
		reason   string
		want     billing.DenialClassification
	}{
		{denialID: "denial_unavailable", reason: billing.ReasonQuotaStateUnavailable, want: billing.DenialClassificationQuotaStateUnavailable},
		{denialID: "denial_operator", reason: "quota_denied:operator_action_needed", want: billing.DenialClassificationOperatorActionNeeded},
		{denialID: "denial_abuse", reason: "abuse_restriction:temporary", want: billing.DenialClassificationAbuseRestriction},
	} {
		denial := r47BillingSeedDenial(t, ctx, sqliteStore, tenantID, billing.CategoryRunLaunches, item.denialID, item.reason, now)
		rec := httptest.NewRecorder()
		handleHostedBilling(config.Config{Environment: config.EnvironmentTest}, billing.NewManager(sqliteStore), rec, r38BillingRequest(http.MethodGet, "/v1/billing/denials/"+denial.DenialID, r38BillingOwnerContext(tenantID), ""))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected denial detail status 200 for %s, got %d: %s", item.denialID, rec.Code, rec.Body.String())
		}
		var detail billing.QuotaDenialDetail
		if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
			t.Fatalf("decode denial detail %s: %v", item.denialID, err)
		}
		if detail.Classification != item.want {
			t.Fatalf("classification=%s, want %s: %+v", detail.Classification, item.want, detail)
		}
	}
	viewerRec := httptest.NewRecorder()
	handleHostedBilling(config.Config{Environment: config.EnvironmentTest}, billing.NewManager(sqliteStore), viewerRec, r38BillingRequest(http.MethodGet, "/v1/billing/denials/denial_run_launches", r38BillingViewerContext(tenantID), ""))
	if viewerRec.Code != http.StatusForbidden {
		t.Fatalf("expected unauthorized denial detail status 403, got %d: %s", viewerRec.Code, viewerRec.Body.String())
	}
}

func TestHostedBillingQuotaDashboardProjectsOverridesAndExplicitRestrictions(t *testing.T) {
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
	now := time.Now().UTC().Add(-time.Minute)
	tenantID := "ten_r47_overrides"
	r47BillingSeedTenant(t, ctx, sqliteStore, tenantID, now)
	loweredLimit := int64(3)
	if err := sqliteStore.SaveQuotaOverride(ctx, billing.QuotaOverride{
		QuotaOverrideID: "override_lowered",
		TenantID:        tenantID,
		Category:        billing.CategoryRunLaunches,
		Limit:           &loweredLimit,
		Reason:          "temporary lowered limit",
		EffectiveAt:     now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("SaveQuotaOverride returned error: %v", err)
	}
	if err := sqliteStore.SaveAbuseRestriction(ctx, billing.AbuseRestrictionRecord{
		RestrictionID:         "restriction_runtime",
		TenantID:              tenantID,
		Status:                billing.AbuseRestrictionStatusActive,
		AffectedCategory:      billing.CategoryRuntimeToolCalls,
		RecoveryAction:        billing.RecoveryActionContactSupport,
		VisibleReasonCode:     "abuse_restriction:temporary",
		SupportContactAllowed: true,
		StartedAt:             now.Add(-time.Minute),
		Document:              map[string]any{"detectionSignals": "must not render"},
	}); err != nil {
		t.Fatalf("SaveAbuseRestriction returned error: %v", err)
	}
	rec := httptest.NewRecorder()
	handleHostedBilling(config.Config{Environment: config.EnvironmentTest}, billing.NewManager(sqliteStore), rec, r38BillingRequest(http.MethodGet, "/v1/billing/quota-dashboard", r38BillingOwnerContext(tenantID), ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected dashboard status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("detectionSignals")) || bytes.Contains(rec.Body.Bytes(), []byte("must not render")) {
		t.Fatalf("dashboard leaked restriction internals: %s", rec.Body.String())
	}
	var dashboard billing.TenantQuotaDashboard
	if err := json.Unmarshal(rec.Body.Bytes(), &dashboard); err != nil {
		t.Fatalf("decode dashboard: %v", err)
	}
	var sawOverride bool
	var sawRestriction bool
	for _, section := range dashboard.Sections {
		for _, item := range section.Items {
			if item.Category == billing.CategoryRunLaunches && item.Override != nil && item.Override.EffectiveLimit == loweredLimit {
				sawOverride = true
			}
			if item.Category == billing.CategoryRuntimeToolCalls && item.Restriction != nil && item.Status == billing.QuotaStatusRestricted {
				sawRestriction = true
			}
		}
	}
	if !sawOverride || !sawRestriction {
		t.Fatalf("expected override and restriction in dashboard: %+v", dashboard)
	}
}

func TestHostedBillingEvidenceExportIsPermissionedStructuredAndTenantScoped(t *testing.T) {
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
	now := time.Now().UTC().Add(-time.Minute)
	r47BillingSeedTenant(t, ctx, sqliteStore, "ten_r47_export_a", now)
	r47BillingSeedTenant(t, ctx, sqliteStore, "ten_r47_export_b", now)
	r47BillingSeedDenial(t, ctx, sqliteStore, "ten_r47_export_a", billing.CategoryRunLaunches, "denial_export_a", "", now)
	r47BillingSeedDenial(t, ctx, sqliteStore, "ten_r47_export_b", billing.CategoryRunLaunches, "denial_export_b", "", now)
	manager := billing.NewManager(sqliteStore)

	forbiddenRec := httptest.NewRecorder()
	handleHostedBilling(config.Config{Environment: config.EnvironmentTest}, manager, forbiddenRec, r38BillingRequest(http.MethodPost, "/v1/billing/denials/denial_export_a/evidence-export", r38BillingTenantContext("ten_r47_export_a", identity.RoleAdmin, identity.PermissionBillingView), ""))
	if forbiddenRec.Code != http.StatusForbidden {
		t.Fatalf("expected billing.view-only export denial, got %d: %s", forbiddenRec.Code, forbiddenRec.Body.String())
	}
	crossTenantRec := httptest.NewRecorder()
	handleHostedBilling(config.Config{Environment: config.EnvironmentTest}, manager, crossTenantRec, r38BillingRequest(http.MethodPost, "/v1/billing/denials/denial_export_b/evidence-export", r38BillingTenantContext("ten_r47_export_a", identity.RoleOperator, identity.PermissionBillingEvidenceExport), ""))
	if crossTenantRec.Code != http.StatusNotFound {
		t.Fatalf("expected cross-tenant export to hide record, got %d: %s", crossTenantRec.Code, crossTenantRec.Body.String())
	}
	rec := httptest.NewRecorder()
	handleHostedBilling(config.Config{Environment: config.EnvironmentTest}, manager, rec, r38BillingRequest(http.MethodPost, "/v1/billing/denials/denial_export_a/evidence-export", r38BillingTenantContext("ten_r47_export_a", identity.RoleOperator, identity.PermissionBillingEvidenceExport), ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected export status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var export billing.BillingEvidenceExport
	if err := json.Unmarshal(rec.Body.Bytes(), &export); err != nil {
		t.Fatalf("decode evidence export: %v", err)
	}
	if export.SchemaVersion == "" || export.Denial.DenialID != "denial_export_a" || len(export.UsageSnapshot) == 0 || len(export.AuditRefs) < 2 {
		t.Fatalf("unexpected export: %+v", export)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("ten_r47_export_b")) {
		t.Fatalf("export leaked other tenant data: %s", rec.Body.String())
	}
}

func TestHostedBillingAdminRequiresManagePermission(t *testing.T) {
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
	if err := sqliteStore.SavePlan(ctx, billing.DevelopmentPlan("ten_r38_admin", time.Now().UTC().Add(-time.Minute))); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	manager := billing.NewManager(sqliteStore)

	operatorRec := httptest.NewRecorder()
	handleHostedBillingAdmin(manager, operatorRec, r38BillingRequest(http.MethodPost, "/v1/admin/billing/tenants/ten_r38_admin/plan", r38BillingOperatorContext("ten_r38_admin"), `{"planKey":"finite","enforcementMode":"enforced","reason":"test"}`))
	if operatorRec.Code != http.StatusForbidden {
		t.Fatalf("expected operator billing.manage denial, got %d: %s", operatorRec.Code, operatorRec.Body.String())
	}

	adminRec := httptest.NewRecorder()
	handleHostedBillingAdmin(manager, adminRec, r38BillingRequest(http.MethodPost, "/v1/admin/billing/tenants/ten_r38_admin/plan", r38BillingAdminContext("ten_r38_admin"), `{"planKey":"finite","enforcementMode":"enforced","reason":"test assignment"}`))
	if adminRec.Code != http.StatusOK {
		t.Fatalf("expected admin status 200, got %d: %s", adminRec.Code, adminRec.Body.String())
	}
	if !bytes.Contains(adminRec.Body.Bytes(), []byte(`"planKey":"finite"`)) {
		t.Fatalf("expected assigned plan response, got %s", adminRec.Body.String())
	}

	crossTenantRec := httptest.NewRecorder()
	handleHostedBillingAdmin(manager, crossTenantRec, r38BillingRequest(http.MethodPost, "/v1/admin/billing/tenants/ten_other/plan", r38BillingAdminContext("ten_r38_admin"), `{"planKey":"finite","enforcementMode":"enforced","reason":"test"}`))
	if crossTenantRec.Code != http.StatusForbidden {
		t.Fatalf("expected cross-tenant admin denial, got %d: %s", crossTenantRec.Code, crossTenantRec.Body.String())
	}
}

func TestHostedBillingAdminPlanAssignmentPersistsEvidence(t *testing.T) {
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
	tenantID := "ten_r38_plan_assignment"
	if err := sqliteStore.SavePlan(ctx, billing.DevelopmentPlan(tenantID, time.Now().UTC().Add(-time.Minute))); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	manager := billing.NewManager(sqliteStore)

	rec := httptest.NewRecorder()
	handleHostedBillingAdmin(manager, rec, r38BillingRequest(http.MethodPost, "/v1/admin/billing/tenants/"+tenantID+"/plan", r38BillingAdminContext(tenantID), `{"planKey":"finite","enforcementMode":"enforced","reason":"customer upgraded"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected plan assignment status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	active, found, err := sqliteStore.ActivePlan(ctx, tenantID)
	if err != nil || !found {
		t.Fatalf("ActivePlan err=%v found=%v", err, found)
	}
	if active.PlanKey != "finite" || active.AssignmentReason != "customer upgraded" || active.AssignedByPrincipalID == "" {
		t.Fatalf("expected assignment evidence on active plan, got %#v", active)
	}
}

func TestHostedBillingAdminQuotaOverrideLoweredBelowUsageDeniesNewWork(t *testing.T) {
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
	tenantID := "ten_r38_lowered_override"
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{PlanID: "plan_" + tenantID, TenantID: tenantID, PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	definition, _ := billing.DefinitionFor(billing.CategoryRunLaunches)
	period, err := sqliteStore.OpenPeriod(ctx, tenantID, definition, now)
	if err != nil {
		t.Fatalf("OpenPeriod returned error: %v", err)
	}
	if err := sqliteStore.SaveUsageCounter(ctx, billing.UsageCounter{
		UsageCounterID:  "counter_lowered_override",
		TenantID:        tenantID,
		Category:        billing.CategoryRunLaunches,
		QuotaPeriodID:   period.QuotaPeriodID,
		CommittedAmount: 1,
		ReservedAmount:  1,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatalf("SaveUsageCounter returned error: %v", err)
	}
	manager := billing.NewManager(sqliteStore)

	rec := httptest.NewRecorder()
	handleHostedBillingAdmin(manager, rec, r38BillingRequest(http.MethodPost, "/v1/admin/billing/tenants/"+tenantID+"/quota-overrides", r38BillingAdminContext(tenantID), `{"category":"run_launches","limit":1,"reason":"downgrade"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected quota override status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	result, err := manager.Reserve(ctx, billing.ReserveInput{TenantID: tenantID, Category: billing.CategoryRunLaunches, Amount: 1, OperationKey: "tenant:" + tenantID + ":run:after_lowering", Hosted: true})
	if err != billing.ErrQuotaDenied || result.Denial == nil || !result.Quota.OverLimit {
		t.Fatalf("expected lowered quota to deny new work, err=%v result=%+v", err, result)
	}
}

func TestHostedBillingAdminMutationRoutesDenyViewerAndOperator(t *testing.T) {
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
	tenantID := "ten_r38_admin_denied"
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{PlanID: "plan_" + tenantID, TenantID: tenantID, PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	definition, _ := billing.DefinitionFor(billing.CategoryRunLaunches)
	period, err := sqliteStore.OpenPeriod(ctx, tenantID, definition, now)
	if err != nil {
		t.Fatalf("OpenPeriod returned error: %v", err)
	}
	if err := sqliteStore.SaveReservation(ctx, billing.UsageReservation{ReservationID: "reservation_denied_admin_route", TenantID: tenantID, Category: billing.CategoryRunLaunches, QuotaPeriodID: period.QuotaPeriodID, OperationKey: "tenant:" + tenantID + ":run:pending", AmountReserved: 1, Status: billing.ReservationStatusOperatorActionNeeded, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveReservation returned error: %v", err)
	}
	manager := billing.NewManager(sqliteStore)
	routes := []struct {
		name string
		path string
		body string
	}{
		{name: "plan", path: "/v1/admin/billing/tenants/" + tenantID + "/plan", body: `{"planKey":"finite","enforcementMode":"enforced","reason":"test"}`},
		{name: "override", path: "/v1/admin/billing/tenants/" + tenantID + "/quota-overrides", body: `{"category":"run_launches","limit":1,"reason":"test"}`},
		{name: "adjustment", path: "/v1/admin/billing/tenants/" + tenantID + "/manual-adjustments", body: `{"category":"run_launches","quotaPeriodId":"` + period.QuotaPeriodID + `","amountDelta":1,"reason":"test"}`},
		{name: "resolve", path: "/v1/admin/billing/tenants/" + tenantID + "/reservations/reservation_denied_admin_route/resolve", body: `{"outcome":"released","reason":"test"}`},
	}
	contexts := []identity.TenantContext{r38BillingViewerContext(tenantID), r38BillingOperatorContext(tenantID)}
	for _, route := range routes {
		for _, tenantContext := range contexts {
			rec := httptest.NewRecorder()
			handleHostedBillingAdmin(manager, rec, r38BillingRequest(http.MethodPost, route.path, tenantContext, route.body))
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s %s expected status 403, got %d: %s", tenantContext.Role, route.name, rec.Code, rec.Body.String())
			}
		}
	}
}

func TestHostedBillingAdminResolvesOperatorActionReservation(t *testing.T) {
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
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{
		PlanID:          "plan_ten_r38_resolve",
		TenantID:        "ten_r38_resolve",
		PlanKey:         "finite",
		Status:          billing.PlanStatusActive,
		EnforcementMode: billing.EnforcementModeEnforced,
		EffectiveAt:     time.Now().UTC().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	definition, ok := billing.DefinitionFor(billing.CategoryRunLaunches)
	if !ok {
		t.Fatal("expected run launch definition")
	}
	period, err := sqliteStore.OpenPeriod(ctx, "ten_r38_resolve", definition, time.Now().UTC())
	if err != nil {
		t.Fatalf("OpenPeriod returned error: %v", err)
	}
	counter := billing.UsageCounter{
		UsageCounterID: "counter_resolve",
		TenantID:       "ten_r38_resolve",
		Category:       billing.CategoryRunLaunches,
		QuotaPeriodID:  period.QuotaPeriodID,
		ReservedAmount: 1,
		UpdatedAt:      time.Now().UTC(),
	}
	if err := sqliteStore.SaveUsageCounter(ctx, counter); err != nil {
		t.Fatalf("SaveUsageCounter returned error: %v", err)
	}
	reservation := billing.UsageReservation{
		ReservationID:  "reservation_resolve",
		TenantID:       "ten_r38_resolve",
		Category:       billing.CategoryRunLaunches,
		QuotaPeriodID:  period.QuotaPeriodID,
		OperationKey:   "tenant:ten_r38_resolve:run:client_1",
		AmountReserved: 1,
		Status:         billing.ReservationStatusOperatorActionNeeded,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := sqliteStore.SaveReservation(ctx, reservation); err != nil {
		t.Fatalf("SaveReservation returned error: %v", err)
	}
	manager := billing.NewManager(sqliteStore)

	rec := httptest.NewRecorder()
	handleHostedBillingAdmin(manager, rec, r38BillingRequest(http.MethodPost, "/v1/admin/billing/tenants/ten_r38_resolve/reservations/reservation_resolve/resolve", r38BillingAdminContext("ten_r38_resolve"), `{"outcome":"released","reason":"operator verified work never started"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected reservation resolution status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	updated, found, err := sqliteStore.ReservationByID(ctx, "ten_r38_resolve", "reservation_resolve")
	if err != nil {
		t.Fatalf("ReservationByID returned error: %v", err)
	}
	if !found || updated.Status != billing.ReservationStatusReleased {
		t.Fatalf("expected released reservation, found=%v value=%#v", found, updated)
	}
	updatedCounter, found, err := sqliteStore.UsageCounter(ctx, "ten_r38_resolve", billing.CategoryRunLaunches, period.QuotaPeriodID)
	if err != nil {
		t.Fatalf("UsageCounter returned error: %v", err)
	}
	if !found || updatedCounter.ReservedAmount != 0 {
		t.Fatalf("expected reserved amount to be released, found=%v counter=%#v", found, updatedCounter)
	}
}
