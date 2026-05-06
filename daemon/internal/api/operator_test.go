package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/activation"
	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
	"github.com/dopejs/dope-agent/daemon/internal/setupwizard"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

type operatorHarness struct {
	server       *Server
	authHeader   string
	runtime      *runtime.Manager
	integrations *integrations.Manager
	connectors   *connectors.Supervisor
	capabilities *capabilities.Supervisor
	policy       *policy.Engine
	scheduler    *scheduler.Scheduler
	delivery     *delivery.Manager
	store        *store.SQLiteStore
}

type operatorFailingDeliveryAdapter struct {
	targetKind delivery.TargetKind
	err        error
}

func (a *operatorFailingDeliveryAdapter) Supports(kind delivery.TargetKind) bool {
	return kind == a.targetKind
}

func (a *operatorFailingDeliveryAdapter) Send(context.Context, delivery.DeliveryTarget, delivery.DeliveryOutcome) (delivery.SendResult, error) {
	return delivery.SendResult{TransportKind: string(a.targetKind)}, a.err
}

func newOperatorHarness(t *testing.T, adapters ...delivery.Adapter) *operatorHarness {
	t.Helper()

	cfg := config.Config{
		Environment: config.EnvironmentTest,
		BindAddr:    "127.0.0.1:0",
		DataDir:     t.TempDir(),
	}
	eventBus := events.NewBus()
	t.Cleanup(eventBus.Close)

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	authManager := auth.NewManager()
	runtimeManager := runtime.NewManager()
	policyEngine := policy.NewEngine()
	integrationManager := integrations.NewManager("test")
	connectorSupervisor := connectors.NewSupervisor()
	capabilitySupervisor := capabilities.NewSupervisor()
	dispatcher := llm.NewDispatcher()
	providerManager := providers.NewManager(config.Config{
		Environment: config.EnvironmentTest,
		LLM: config.LLMConfig{
			DefaultProvider: "echo",
		},
	}, dispatcher)
	checkpointManager := checkpoints.NewManager(sqliteStore, runtimeManager)
	t.Cleanup(func() { _ = checkpointManager.Close() })

	if len(adapters) == 0 {
		adapters = []delivery.Adapter{delivery.NewTestSinkAdapter()}
	}
	deliveryManager := delivery.NewManager("test", eventBus, sqliteStore, adapters...)
	schedulerManager := scheduler.New(scheduler.Dependencies{
		Config:      cfg,
		Runtime:     runtimeManager,
		EventBus:    eventBus,
		Store:       sqliteStore,
		Checkpoints: checkpointManager,
	})

	server := NewServer(Dependencies{
		Config:       cfg,
		Logger:       telemetry.New("error").Slog(),
		EventBus:     eventBus,
		Auth:         authManager,
		Providers:    providerManager,
		Integrations: integrationManager,
		Connectors:   connectorSupervisor,
		Capabilities: capabilitySupervisor,
		Policy:       policyEngine,
		Runtime:      runtimeManager,
		Scheduler:    schedulerManager,
		Delivery:     deliveryManager,
		Store:        sqliteStore,
		Checkpoints:  checkpointManager,
	})

	return &operatorHarness{
		server:       server,
		authHeader:   issueAuthHeaderForTest(t, authManager, "operator-web"),
		runtime:      runtimeManager,
		integrations: integrationManager,
		connectors:   connectorSupervisor,
		capabilities: capabilitySupervisor,
		policy:       policyEngine,
		scheduler:    schedulerManager,
		delivery:     deliveryManager,
		store:        sqliteStore,
	}
}

func (h *operatorHarness) createRun(t *testing.T, entrypoint, goal string, status runtime.RunStatus) runtime.Run {
	t.Helper()

	run, err := h.runtime.CreateRun(runtime.CreateRunInput{Entrypoint: entrypoint, Goal: goal})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	run.Status = status
	run.UpdatedAt = time.Now().UTC()
	h.runtime.RestoreRunCheckpoint(runtime.RunCheckpoint{Run: run})
	if err := h.store.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}
	return run
}

func (h *operatorHarness) upsertWorkflow(t *testing.T, run runtime.Run, status orchestration.WorkflowStatus, planSummary string) orchestration.Workflow {
	t.Helper()

	now := time.Now().UTC()
	workflow := orchestration.Workflow{
		WorkflowID:       "wf_" + run.RunID,
		RunID:            run.RunID,
		EnvironmentScope: "test",
		Goal:             "Inspect operator workflow state",
		Status:           status,
		PlanSummary:      planSummary,
		FailureSummary:   "workflow is blocked on operator follow-up",
		CreatedAt:        now.Add(-time.Minute),
		UpdatedAt:        now,
		Steps: []orchestration.WorkflowStep{{
			WorkflowStepID:       "wfstep_" + run.RunID,
			WorkflowID:           "wf_" + run.RunID,
			Title:                "Review operator state",
			Position:             1,
			ConsumerKind:         "skill",
			ConsumerID:           "operator",
			ToolName:             "inspect",
			Status:               orchestration.StepStatusBlocked,
			ApprovalModeExpected: "allow",
			AttemptCount:         1,
			MaxAttempts:          1,
			CreatedAt:            now.Add(-time.Minute),
			UpdatedAt:            now,
		}},
	}
	if err := h.store.UpsertWorkflow(context.Background(), workflow); err != nil {
		t.Fatalf("UpsertWorkflow returned error: %v", err)
	}
	return workflow
}

func (h *operatorHarness) createPausedSchedule(t *testing.T, run runtime.Run) scheduler.Schedule {
	t.Helper()

	fireAt := time.Now().UTC().Add(10 * time.Minute)
	scheduleItem, err := h.scheduler.Create(context.Background(), scheduler.CreateInput{
		Trigger: scheduler.Trigger{
			Kind:   scheduler.TriggerKindOnce,
			FireAt: &fireAt,
		},
		Target: scheduler.Target{
			Kind: scheduler.TargetKindRun,
			Run: &scheduler.RunTarget{
				Entrypoint: run.Entrypoint,
				Goal:       run.Goal,
			},
		},
		RetryPolicy: scheduler.RetryPolicy{
			MaxRetries:       1,
			BackoffKind:      scheduler.RetryBackoffFixed,
			BaseDelaySeconds: 5,
			MaxDelaySeconds:  5,
		},
	})
	if err != nil {
		t.Fatalf("Create schedule returned error: %v", err)
	}
	paused, ok, err := h.scheduler.Pause(context.Background(), scheduleItem.ScheduleID)
	if err != nil {
		t.Fatalf("Pause returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected schedule %s to exist", scheduleItem.ScheduleID)
	}
	return paused
}

func (h *operatorHarness) createFailedDelivery(t *testing.T, run runtime.Run, workflow orchestration.Workflow) delivery.DeliveryOutcome {
	t.Helper()

	h.delivery.ConfigureForTesting(1, time.Millisecond, time.Millisecond)
	target, err := h.delivery.CreateTarget(context.Background(), delivery.DeliveryTarget{
		TargetID:         "target-operator",
		DisplayName:      "Operator Target",
		TargetKind:       delivery.TargetKindTestSink,
		EnvironmentScope: "test",
	})
	if err != nil {
		t.Fatalf("CreateTarget returned error: %v", err)
	}
	if _, err := h.delivery.UpsertPreference(context.Background(), delivery.DeliveryPreference{
		PreferenceID:     "pref-operator",
		EnvironmentScope: "test",
		ScopeKind:        delivery.PreferenceScopeUserDefault,
		PreferredTargetsByClass: map[delivery.ResultClass]string{
			delivery.ResultClassFailure: target.TargetID,
		},
	}); err != nil {
		t.Fatalf("UpsertPreference returned error: %v", err)
	}
	outcome, err := h.delivery.EmitOutcome(context.Background(), delivery.OutcomeInput{
		SourceKind:     "workflow",
		SourceID:       workflow.WorkflowID,
		RunID:          run.RunID,
		WorkflowID:     workflow.WorkflowID,
		ResultClass:    delivery.ResultClassFailure,
		PayloadPreview: "transport failure",
	})
	if err != nil {
		t.Fatalf("EmitOutcome returned error: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		current, ok, err := h.delivery.GetOutcome(context.Background(), outcome.DeliveryID)
		if err != nil {
			t.Fatalf("GetOutcome returned error: %v", err)
		}
		if ok && current.Status == delivery.OutcomeStatusFailed {
			return current
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("delivery %s did not reach failed status", outcome.DeliveryID)
	return delivery.DeliveryOutcome{}
}

func TestOperatorOnboardingRouteProjectsReadinessAndFirstActions(t *testing.T) {
	t.Parallel()

	h := newOperatorHarness(t)

	if _, err := h.integrations.Create(integrations.CreateInput{
		IntegrationID:    "calendar-a",
		DomainKind:       "calendar",
		DisplayName:      "Calendar A",
		CanonicalDefault: true,
		EnvironmentScope: "test",
		BackendBinding: integrations.BackendBinding{
			BackendKind: integrations.BackendKindFakeLocal,
		},
	}); err != nil {
		t.Fatalf("Create integration returned error: %v", err)
	}
	if _, err := h.integrations.UpdateReadiness("calendar-a", integrations.UpdateReadinessInput{
		ReadinessStatus:        integrations.ReadinessStatusDegraded,
		AuthState:              integrations.AuthStateAuthorized,
		HealthState:            integrations.HealthStateDegraded,
		ReadinessReason:        "reauth required",
		RequiredOperatorAction: "Refresh calendar auth.",
	}); err != nil {
		t.Fatalf("UpdateReadiness returned error: %v", err)
	}
	if _, _, err := h.connectors.Register(connectors.RegisterInput{
		ConnectorID: "telegram-main",
		Kind:        "telegram",
		DisplayName: "Telegram Main",
	}); err != nil {
		t.Fatalf("Register connector returned error: %v", err)
	}
	if _, err := h.connectors.ReportFailure("telegram-main", connectors.ReportFailureInput{Reason: "network backoff"}); err != nil {
		t.Fatalf("ReportFailure returned error: %v", err)
	}
	if _, _, err := h.capabilities.Register(capabilities.RegisterInput{
		CapabilityID: "browser",
		Kind:         "browser",
		DisplayName:  "Browser",
	}); err != nil {
		t.Fatalf("Register capability returned error: %v", err)
	}
	if _, err := h.capabilities.ReportHealth("browser", capabilities.ReportHealthInput{Status: capabilities.StatusDegraded}); err != nil {
		t.Fatalf("ReportHealth returned error: %v", err)
	}
	h.createRun(t, operatorShellTestRunEntrypoint, "operator smoke", runtime.RunStatusQueued)

	req := httptest.NewRequest(http.MethodGet, "/v1/operator/onboarding", nil)
	req.Header.Set("Authorization", h.authHeader)
	rec := httptest.NewRecorder()
	h.server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	response := decodeStrictResponse[OperatorOnboardingResponse](t, rec.Body.Bytes())
	if response.EnvironmentScope != "test" || response.Status != "completed" {
		t.Fatalf("expected completed test onboarding, got %+v", response)
	}
	if response.RecommendedActionID != "test_run" {
		t.Fatalf("expected test_run recommendation, got %+v", response)
	}
	if len(response.BlockingItemIDs) != 0 {
		t.Fatalf("expected no blockers, got %+v", response.BlockingItemIDs)
	}

	foundQueryAction := false
	foundActivationAction := false
	foundConnectorFollowUp := false
	foundCapabilityFollowUp := false
	foundIntegrationFollowUp := false
	for _, action := range response.FirstUsefulActions {
		if action.ActionID == "test_query" {
			foundQueryAction = action.Available
		}
		if action.ActionID == "test_chat" {
			foundActivationAction = action.Available && action.InvokeRoute == "/v1/activation/test-chat"
		}
	}
	for _, itemID := range response.OptionalFollowUpItemIDs {
		switch itemID {
		case "connector-telegram-main":
			foundConnectorFollowUp = true
		case "capability-browser":
			foundCapabilityFollowUp = true
		case "integration-calendar-a":
			foundIntegrationFollowUp = true
		}
	}
	if !foundQueryAction || !foundActivationAction || !foundConnectorFollowUp || !foundCapabilityFollowUp || !foundIntegrationFollowUp {
		t.Fatalf("expected projected query action and optional follow-up ids, got %+v", response)
	}
	if len(response.CompletedStepIDs) == 0 || response.CompletedStepIDs[len(response.CompletedStepIDs)-1] != "test-run-recorded" {
		t.Fatalf("expected recorded test run completion marker, got %+v", response.CompletedStepIDs)
	}
}

func TestOperatorActivityRouteIncludesApprovalsSchedulesRunsWorkflowsAndDeliveries(t *testing.T) {
	t.Parallel()

	h := newOperatorHarness(t, &operatorFailingDeliveryAdapter{
		targetKind: delivery.TargetKindTestSink,
		err:        errors.New("transport offline"),
	})

	approval, _, err := h.policy.RequestApproval(policy.RequestApprovalInput{
		Action:       "workflow.launch",
		ResourceKind: "workflow",
		ResourceID:   "wf_manual",
		Reason:       "manual review required",
		RequestedBy:  "operator-test",
	})
	if err != nil {
		t.Fatalf("RequestApproval returned error: %v", err)
	}
	run := h.createRun(t, "operator", "follow up delivery", runtime.RunStatusBlocked)
	workflow := h.upsertWorkflow(t, run, orchestration.WorkflowStatusFailed, "workflow failed after operator review")
	scheduleItem := h.createPausedSchedule(t, run)
	failedOutcome := h.createFailedDelivery(t, run, workflow)
	if _, err := h.store.AppendEvent(context.Background(), events.Event{
		EventID:          "evt_operator_delivery_orphan",
		EnvironmentScope: "test",
		Category:         "delivery",
		Name:             "delivery.failed",
		OccurredAt:       time.Now().UTC().Add(2 * time.Minute),
		Resource:         events.Resource{Kind: "delivery", ID: "delivery_orphan"},
		Payload: map[string]any{
			"status": "failed",
		},
	}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/operator/activity?attentionOnly=true&limit=10", nil)
	req.Header.Set("Authorization", h.authHeader)
	rec := httptest.NewRecorder()
	h.server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	response := decodeStrictResponse[OperatorActivityListResponse](t, rec.Body.Bytes())
	expectedKinds := map[string]bool{
		"approval": false,
		"schedule": false,
		"run":      false,
		"workflow": false,
		"delivery": false,
	}
	foundEventBackedDelivery := false
	for _, item := range response.Items {
		if item.AttentionLevel == "info" {
			t.Fatalf("expected attentionOnly filter to remove info items, got %+v", item)
		}
		switch item.SourceKind {
		case "approval":
			expectedKinds["approval"] = item.SourceID == approval.ApprovalID
		case "schedule":
			expectedKinds["schedule"] = item.SourceID == scheduleItem.ScheduleID
		case "run":
			expectedKinds["run"] = item.SourceID == run.RunID && item.DetailRoute == "/v1/runs/"+run.RunID
		case "workflow":
			expectedKinds["workflow"] = item.SourceID == workflow.WorkflowID && item.DetailRoute == "/v1/runs/"+run.RunID+"/workflows/"+workflow.WorkflowID
		case "delivery":
			if item.SourceID == failedOutcome.DeliveryID && item.DetailRoute == "/v1/deliveries/"+failedOutcome.DeliveryID {
				expectedKinds["delivery"] = true
			}
			if item.SourceID == "delivery_orphan" && item.DetailRoute == "/v1/deliveries/delivery_orphan" {
				foundEventBackedDelivery = true
			}
		}
	}
	for kind, found := range expectedKinds {
		if !found {
			t.Fatalf("expected %s activity record in %+v", kind, response.Items)
		}
	}
	if !foundEventBackedDelivery {
		t.Fatalf("expected persisted-event-backed delivery history in %+v", response.Items)
	}
}

func TestOperatorDiagnosticsRouteSupportsPlaneAndSeverityFilters(t *testing.T) {
	t.Parallel()

	h := newOperatorHarness(t, &operatorFailingDeliveryAdapter{
		targetKind: delivery.TargetKindTestSink,
		err:        errors.New("transport offline"),
	})

	if _, err := h.integrations.Create(integrations.CreateInput{
		IntegrationID:    "mail-a",
		DomainKind:       "mail",
		DisplayName:      "Mail A",
		CanonicalDefault: true,
		EnvironmentScope: "test",
		BackendBinding: integrations.BackendBinding{
			BackendKind: integrations.BackendKindFakeLocal,
		},
	}); err != nil {
		t.Fatalf("Create integration returned error: %v", err)
	}
	if _, err := h.integrations.UpdateReadiness("mail-a", integrations.UpdateReadinessInput{
		ReadinessStatus:        integrations.ReadinessStatusUnavailable,
		AuthState:              integrations.AuthStateExpired,
		HealthState:            integrations.HealthStateUnavailable,
		ReadinessReason:        "mail auth expired",
		RequiredOperatorAction: "Reconnect mail integration.",
	}); err != nil {
		t.Fatalf("UpdateReadiness returned error: %v", err)
	}
	if _, _, err := h.policy.RequestApproval(policy.RequestApprovalInput{
		Action:       "delivery.override",
		ResourceKind: "delivery",
		ResourceID:   "delivery_manual",
		Reason:       "operator approval required",
		RequestedBy:  "operator-test",
	}); err != nil {
		t.Fatalf("RequestApproval returned error: %v", err)
	}
	run := h.createRun(t, "operator", "diagnose failed delivery", runtime.RunStatusFailed)
	workflow := h.upsertWorkflow(t, run, orchestration.WorkflowStatusFailed, "workflow failed after retries")
	failedOutcome := h.createFailedDelivery(t, run, workflow)

	unfilteredReq := httptest.NewRequest(http.MethodGet, "/v1/operator/diagnostics", nil)
	unfilteredReq.Header.Set("Authorization", h.authHeader)
	unfilteredRec := httptest.NewRecorder()
	h.server.Handler().ServeHTTP(unfilteredRec, unfilteredReq)
	if unfilteredRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", unfilteredRec.Code, unfilteredRec.Body.String())
	}
	allFindings := decodeStrictResponse[OperatorDiagnosticListResponse](t, unfilteredRec.Body.Bytes())
	if len(allFindings.Items) < 4 {
		t.Fatalf("expected multiple findings, got %+v", allFindings)
	}

	filteredReq := httptest.NewRequest(http.MethodGet, "/v1/operator/diagnostics?plane=delivery&severity=critical", nil)
	filteredReq.Header.Set("Authorization", h.authHeader)
	filteredRec := httptest.NewRecorder()
	h.server.Handler().ServeHTTP(filteredRec, filteredReq)
	if filteredRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", filteredRec.Code, filteredRec.Body.String())
	}

	response := decodeStrictResponse[OperatorDiagnosticListResponse](t, filteredRec.Body.Bytes())
	if len(response.Items) != 1 {
		t.Fatalf("expected exactly one filtered finding, got %+v", response.Items)
	}
	item := response.Items[0]
	if item.SourceKind != "delivery" || item.SourceID != failedOutcome.DeliveryID || item.Plane != "delivery" || item.Severity != "critical" {
		t.Fatalf("expected filtered delivery finding, got %+v", item)
	}
	if item.DetailRoute != "/v1/deliveries/"+failedOutcome.DeliveryID {
		t.Fatalf("expected delivery detail route, got %+v", item)
	}
}

func TestOperatorDiagnosticsRouteIncludesActivationFailures(t *testing.T) {
	t.Parallel()

	h := newOperatorHarness(t)
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	if err := h.store.UpsertActivationState(context.Background(), activation.State{
		ActivationID:        "act_operator_gap",
		PrincipalID:         "prn_operator_gap",
		TenantID:            "ten_operator_gap",
		EnvironmentScope:    "test",
		Status:              activation.StatusBlocked,
		CurrentStepID:       activation.StepQuotaBaseline,
		CompletedStepIDs:    []string{activation.StepTenantResolved},
		BlockingReasonCodes: []activation.ReasonCode{activation.ReasonQuotaBaselineUnavailable},
		ReadinessItems: []activation.ReadinessItem{{
			ItemID:                "quota-baseline",
			ItemKind:              activation.ReadinessKindQuotaBaseline,
			Status:                activation.ReadinessStatusBlocked,
			ReasonCode:            activation.ReasonQuotaBaselineUnavailable,
			DisplayName:           "Quota baseline",
			RequiredForActivation: true,
			Retryable:             true,
			RemediationOwner:      activation.RemediationOwnerOperator,
			UpdatedAt:             now,
		}},
		QuotaBaseline: &activation.QuotaBaseline{
			TenantID:         "ten_operator_gap",
			PlanKey:          "unknown",
			EnforcementMode:  "not_measurable",
			Status:           activation.QuotaBaselineStatusUnavailable,
			Quotas:           []activation.QuotaProjection{},
			ProjectedAt:      now,
			ProjectionSource: "billing_usage_summary",
			ReasonCode:       activation.ReasonQuotaBaselineUnavailable,
		},
		FirstAction:     activation.DefaultTestChatFirstAction(false, []string{"quota-baseline"}),
		FailureReason:   &activation.FailureReason{ReasonCode: activation.ReasonQuotaBaselineUnavailable, Stage: activation.FailureStageQuotaBaseline, Retryable: true, RemediationOwner: activation.RemediationOwnerOperator},
		CreatedAt:       now,
		UpdatedAt:       now,
		LastEvaluatedAt: now,
	}); err != nil {
		t.Fatalf("UpsertActivationState returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/operator/diagnostics?plane=readiness", nil)
	req.Header.Set("Authorization", h.authHeader)
	rec := httptest.NewRecorder()
	h.server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	response := decodeStrictResponse[OperatorDiagnosticListResponse](t, rec.Body.Bytes())
	for _, item := range response.Items {
		if item.SourceKind == "activation" && item.SourceID == "act_operator_gap" {
			if item.Status != string(activation.StatusBlocked) || !strings.Contains(item.Reason, string(activation.ReasonQuotaBaselineUnavailable)) || item.DetailRoute != "/v1/activation/diagnostics" {
				t.Fatalf("unexpected activation diagnostic item: %+v", item)
			}
			return
		}
	}
	t.Fatalf("expected activation diagnostic finding, got %+v", response.Items)
}

func TestOperatorDiagnosticsProjectionIncludesTenantScopedSetupFindings(t *testing.T) {
	t.Parallel()

	h := newOperatorHarness(t)
	now := time.Date(2026, 5, 6, 11, 0, 0, 0, time.UTC)
	ctx := withTenantContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_setup_operator",
		PrincipalID: "prn_setup_operator",
		Permissions: []identity.Permission{
			identity.PermissionCredentialsInspect,
		},
	})
	if err := h.store.SaveSetupSession(ctx, setupwizard.SetupSession{
		SetupSessionID:      "setup_operator_lark",
		TenantID:            "ten_setup_operator",
		ActorPrincipalID:    "prn_setup_operator",
		TargetID:            setupwizard.TargetFeishuLark,
		TargetKind:          setupwizard.TargetKindIntegration,
		SetupStyle:          setupwizard.SetupStyleOAuth,
		State:               setupwizard.StateActionRequired,
		ReasonCode:          setupwizard.ReasonOAuthDenied,
		Retryable:           true,
		RemediationOwner:    setupwizard.OwnerTenantAdmin,
		SafeUseMode:         setupwizard.SafeUseBlocked,
		AllowedCapabilities: []string{},
		DiagnosticResultID:  "diag_setup_operator_lark",
		RedactionStatus:     setupwizard.RedactionRedacted,
		CreatedAt:           now.Add(-time.Minute),
		UpdatedAt:           now,
		LastTransitionAt:    now,
	}); err != nil {
		t.Fatalf("SaveSetupSession returned error: %v", err)
	}
	if err := h.store.SaveSetupSession(context.Background(), setupwizard.SetupSession{
		SetupSessionID:   "setup_other_tenant",
		TenantID:         "ten_other",
		TargetID:         setupwizard.TargetOpenAICompatible,
		TargetKind:       setupwizard.TargetKindProvider,
		SetupStyle:       setupwizard.SetupStyleSubmittedSecret,
		State:            setupwizard.StateUnavailable,
		ReasonCode:       setupwizard.ReasonProviderUnavailable,
		Retryable:        true,
		RemediationOwner: setupwizard.OwnerProvider,
		SafeUseMode:      setupwizard.SafeUseBlocked,
		RedactionStatus:  setupwizard.RedactionRedacted,
		CreatedAt:        now,
		UpdatedAt:        now,
		LastTransitionAt: now,
	}); err != nil {
		t.Fatalf("SaveSetupSession other tenant returned error: %v", err)
	}

	builder := newOperatorProjectionBuilder(config.Config{Environment: config.EnvironmentTest}, h.store, nil, h.integrations, h.connectors, h.capabilities, h.policy, h.runtime, h.scheduler, h.delivery, nil)
	response, err := builder.buildDiagnostics(ctx)
	if err != nil {
		t.Fatalf("buildDiagnostics returned error: %v", err)
	}
	for _, item := range response.Items {
		if item.SourceKind != "credential_setup" {
			continue
		}
		if item.SourceID != "setup_operator_lark" || item.Plane != "readiness" || item.Status != string(setupwizard.StateActionRequired) || item.Reason != setupwizard.ReasonOAuthDenied {
			t.Fatalf("unexpected setup diagnostic finding: %+v", item)
		}
		if item.DetailRoute != "/v1/setup/sessions/setup_operator_lark/diagnostics" || !strings.Contains(item.RecommendedAction, "permissions") {
			t.Fatalf("unexpected setup diagnostic action or route: %+v", item)
		}
		encoded, err := json.Marshal(item)
		if err != nil {
			t.Fatalf("json.Marshal returned error: %v", err)
		}
		if strings.Contains(string(encoded), "setup_other_tenant") || strings.Contains(string(encoded), "R46_FAKE_OPENAI_COMPATIBLE_KEY_DO_NOT_LEAK") {
			t.Fatalf("setup diagnostic leaked cross-tenant or credential evidence: %s", string(encoded))
		}
		return
	}
	t.Fatalf("expected setup diagnostic finding, got %+v", response.Items)
}

func TestOperatorRoutesAllowLocalWebOriginCORS(t *testing.T) {
	t.Parallel()

	h := newOperatorHarness(t)

	preflightReq := httptest.NewRequest(http.MethodOptions, "/v1/operator/onboarding", nil)
	preflightReq.Header.Set("Origin", "http://127.0.0.1:4173")
	preflightReq.Header.Set("Access-Control-Request-Method", http.MethodGet)
	preflightRec := httptest.NewRecorder()
	h.server.Handler().ServeHTTP(preflightRec, preflightReq)
	if preflightRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 preflight, got %d body=%s", preflightRec.Code, preflightRec.Body.String())
	}
	if got := preflightRec.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:4173" {
		t.Fatalf("expected allow-origin header, got %q", got)
	}
	if got := preflightRec.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "X-Dope-Tenant-ID") {
		t.Fatalf("expected tenant header to be allowed, got %q", got)
	}
	if got := preflightRec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "PATCH") {
		t.Fatalf("expected membership update method to be allowed, got %q", got)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/operator/onboarding", nil)
	req.Header.Set("Authorization", h.authHeader)
	req.Header.Set("Origin", "http://127.0.0.1:4173")
	rec := httptest.NewRecorder()
	h.server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:4173" {
		t.Fatalf("expected allow-origin header on get, got %q", got)
	}
}
