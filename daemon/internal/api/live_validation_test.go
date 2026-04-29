package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestLiveValidationRouteStartDenialsAndAwaitingApproval(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		context    identity.TenantContext
		manager    *livevalidation.Manager
		body       string
		wantStatus int
		wantGate   string
		wantState  string
	}{
		{
			name: "permission",
			context: identity.TenantContext{
				TenantID:    "ten_1",
				PrincipalID: "prn_viewer",
				Role:        identity.RoleViewer,
				Permissions: identity.PermissionsForRole(identity.RoleViewer, identity.StatusActive),
			},
			manager:    livevalidation.NewManager(livevalidation.Dependencies{Enabled: true, Clock: func() time.Time { return now }}),
			body:       liveValidationStartBody("lv_permission", "daemon.inspection.read", nil),
			wantStatus: http.StatusConflict,
			wantGate:   "permission",
		},
		{
			name:       "quota unavailable",
			context:    liveValidationOperatorTenantContext(),
			manager:    livevalidation.NewManager(livevalidation.Dependencies{Enabled: true, HostedBilling: true, Clock: func() time.Time { return now }}),
			body:       liveValidationStartBody("lv_quota", "daemon.inspection.read", nil),
			wantStatus: http.StatusConflict,
			wantGate:   "quota",
		},
		{
			name:       "support matrix",
			context:    liveValidationOperatorTenantContext(),
			manager:    livevalidation.NewManager(livevalidation.Dependencies{Enabled: true, Clock: func() time.Time { return now }}),
			body:       liveValidationStartBody("lv_support", "mcp.tool_call", nil),
			wantStatus: http.StatusConflict,
			wantGate:   "support_matrix",
		},
		{
			name:       "awaiting approval",
			context:    liveValidationOperatorTenantContext(),
			manager:    livevalidation.NewManager(livevalidation.Dependencies{Enabled: true, Clock: func() time.Time { return now }}),
			body:       liveValidationStartBody("lv_approval", "daemon.inspection.read", nil),
			wantStatus: http.StatusAccepted,
			wantState:  "awaiting_approval",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := exerciseLiveValidationRoute(t, tt.manager, tt.context, tt.body)
			if response.Code != tt.wantStatus {
				t.Fatalf("status=%d body=%s, want %d", response.Code, response.Body.String(), tt.wantStatus)
			}
			var payload map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if tt.wantGate != "" {
				denials := payload["denials"].([]any)
				denial := denials[0].(map[string]any)
				if denial["gate"] != tt.wantGate {
					t.Fatalf("gate=%v, want %s", denial["gate"], tt.wantGate)
				}
			}
			if tt.wantState != "" {
				attempt := payload["attempt"].(map[string]any)
				if attempt["status"] != tt.wantState {
					t.Fatalf("status=%v, want %s", attempt["status"], tt.wantState)
				}
			}
		})
	}
}

func TestLiveValidationRouteStartDeniesKillSwitch(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.UpsertLiveValidationKillSwitch(context.Background(), livevalidation.KillSwitch{
		KillSwitchID: "kill_1",
		Scope:        livevalidation.KillSwitchScopeTenant,
		TenantID:     "ten_1",
		Enabled:      true,
		Reason:       "containment",
		ChangedBy:    "prn_owner",
		ChangedAt:    now,
	}); err != nil {
		t.Fatalf("UpsertLiveValidationKillSwitch: %v", err)
	}
	manager := livevalidation.NewManager(livevalidation.Dependencies{Enabled: true, Store: sqliteStore, Clock: func() time.Time { return now }})
	response := exerciseLiveValidationRoute(t, manager, liveValidationOperatorTenantContext(), liveValidationStartBody("lv_kill", "daemon.inspection.read", nil))
	if response.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want 409", response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	denial := payload["denials"].([]any)[0].(map[string]any)
	if denial["gate"] != "kill_switch" {
		t.Fatalf("gate=%v, want kill_switch", denial["gate"])
	}
}

func TestReplayAttemptRouteRejectsLiveValidationModeBypass(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	manager := evaluation.NewManager(evaluation.Dependencies{EnvironmentScope: "test", Store: sqliteStore})
	if err := manager.UpsertReplayCandidate(context.Background(), evaluation.ReplayCandidate{
		CandidateID:       "candidate_live_bypass",
		CandidateKind:     evaluation.CandidateKindCuratedWork,
		DisplayName:       "candidate",
		SourceKind:        evaluation.SourceKindRun,
		SourceID:          "run_1",
		SourceRefs:        []evaluation.SourceRef{{Kind: evaluation.SourceKindRun, ID: "run_1"}},
		EnvironmentScope:  "test",
		ReadinessStatus:   evaluation.ReadinessFullyReplayable,
		DefaultReplayMode: evaluation.ReplayModeNonLive,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertReplayCandidate: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/evaluation/replay-candidates/candidate_live_bypass/attempts", bytes.NewBufferString(`{"mode":"live_validation"}`))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handleEvaluationReplayCandidateRoutes(manager, nil, nil, sqliteStore, "candidate_live_bypass/attempts", response, req)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400", response.Code, response.Body.String())
	}
}

func TestReplayCandidateLiveValidationRouteHandsOffToLiveValidationManager(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	evaluationManager := evaluation.NewManager(evaluation.Dependencies{EnvironmentScope: "test", Store: sqliteStore, Clock: func() time.Time { return now }})
	if err := evaluationManager.UpsertReplayCandidate(context.Background(), evaluation.ReplayCandidate{
		CandidateID:       "candidate_live",
		CandidateKind:     evaluation.CandidateKindCuratedWork,
		DisplayName:       "candidate",
		SourceKind:        evaluation.SourceKindRun,
		SourceID:          "run_1",
		SourceRefs:        []evaluation.SourceRef{{Kind: evaluation.SourceKindRun, ID: "run_1"}},
		EnvironmentScope:  "test",
		ReadinessStatus:   evaluation.ReadinessFullyReplayable,
		DefaultReplayMode: evaluation.ReplayModeNonLive,
		CreatedAt:         now,
		UpdatedAt:         now,
	}); err != nil {
		t.Fatalf("UpsertReplayCandidate: %v", err)
	}
	liveValidationManager := livevalidation.NewManager(livevalidation.Dependencies{
		EnvironmentScope: "test",
		Store:            sqliteStore,
		Enabled:          true,
		Clock:            func() time.Time { return now },
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/evaluation/replay-candidates/candidate_live/live-validations", bytes.NewBufferString(liveValidationStartBodyForCandidate("lv_nested", "candidate_live", "daemon.inspection.read", []map[string]any{{
		"approvalId":     "approval_1",
		"validationId":   "lv_nested",
		"approvalTarget": "scope",
		"toolClass":      "daemon.inspection.read",
		"safetyClass":    "read_only",
		"approvedScope":  "scope_lv_nested",
		"status":         "approved",
		"requestedBy":    "prn_operator",
		"requestedAt":    "2026-04-29T09:59:00Z",
		"resolvedAt":     "2026-04-29T09:59:30Z",
	}})))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(tenantctx.WithContext(req.Context(), liveValidationOperatorTenantContext()))
	response := httptest.NewRecorder()
	handleEvaluationReplayCandidateRoutes(evaluationManager, liveValidationManager, nil, sqliteStore, "candidate_live/live-validations", response, req)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s, want 202", response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	attempt := payload["attempt"].(map[string]any)
	if attempt["candidateId"] != "candidate_live" || attempt["status"] != "running" {
		t.Fatalf("unexpected nested live validation attempt: %+v", attempt)
	}
}

func TestReplayCandidateLiveValidationRouteDerivesCandidateToolClasses(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	evaluationManager := evaluation.NewManager(evaluation.Dependencies{EnvironmentScope: "test", Store: sqliteStore, Clock: func() time.Time { return now }})
	if err := evaluationManager.UpsertReplayCandidate(context.Background(), evaluation.ReplayCandidate{
		CandidateID:       "candidate_mixed",
		CandidateKind:     evaluation.CandidateKindCuratedWork,
		DisplayName:       "candidate",
		SourceKind:        evaluation.SourceKindRun,
		SourceID:          "run_1",
		SourceRefs:        []evaluation.SourceRef{{Kind: evaluation.SourceKindRun, ID: "run_1"}},
		ToolClasses:       []string{"daemon.inspection.read", "mcp.tool_call"},
		EnvironmentScope:  "test",
		ReadinessStatus:   evaluation.ReadinessFullyReplayable,
		DefaultReplayMode: evaluation.ReplayModeNonLive,
		CreatedAt:         now,
		UpdatedAt:         now,
	}); err != nil {
		t.Fatalf("UpsertReplayCandidate: %v", err)
	}
	liveValidationManager := livevalidation.NewManager(livevalidation.Dependencies{
		EnvironmentScope: "test",
		Store:            sqliteStore,
		Enabled:          true,
		Clock:            func() time.Time { return now },
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/evaluation/replay-candidates/candidate_mixed/live-validations", bytes.NewBufferString(liveValidationStartBodyForCandidateWithoutClasses("lv_mixed", "candidate_mixed", "daemon.inspection.read", []map[string]any{{
		"approvalId":     "approval_1",
		"validationId":   "lv_mixed",
		"approvalTarget": "scope",
		"toolClass":      "daemon.inspection.read",
		"safetyClass":    "read_only",
		"approvedScope":  "scope_lv_mixed",
		"status":         "approved",
		"requestedBy":    "prn_operator",
		"requestedAt":    "2026-04-29T09:59:00Z",
		"resolvedAt":     "2026-04-29T09:59:30Z",
	}})))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(tenantctx.WithContext(req.Context(), liveValidationOperatorTenantContext()))
	response := httptest.NewRecorder()
	handleEvaluationReplayCandidateRoutes(evaluationManager, liveValidationManager, nil, sqliteStore, "candidate_mixed/live-validations", response, req)
	if response.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want 409", response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	denial := payload["denials"].([]any)[0].(map[string]any)
	if denial["gate"] != "support_matrix" || denial["reference"] != "mcp.tool_call" {
		t.Fatalf("unexpected support denial: %+v", denial)
	}
}

func TestLiveValidationSupportMatrixRoute(t *testing.T) {
	t.Parallel()

	manager := livevalidation.NewManager(livevalidation.Dependencies{Enabled: true, EnvironmentScope: "test"})
	req := httptest.NewRequest(http.MethodGet, "/v1/live-validations/support-matrix", nil)
	response := httptest.NewRecorder()
	handleLiveValidationRoutes(manager, nil, nil, response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", response.Code, response.Body.String())
	}
	var payload struct {
		Version string `json:"version"`
		Items   []struct {
			ToolClass   string `json:"toolClass"`
			SafetyClass string `json:"safetyClass"`
		} `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Version != "v1" || len(payload.Items) == 0 {
		t.Fatalf("unexpected support matrix response: %+v", payload)
	}
	var sawUnsupported bool
	for _, row := range payload.Items {
		if row.ToolClass == string(livevalidation.ToolClassMCPToolCall) && row.SafetyClass == string(livevalidation.SafetyClassUnsupported) {
			sawUnsupported = true
		}
	}
	if !sawUnsupported {
		t.Fatalf("support matrix response did not include unsupported MCP row: %+v", payload.Items)
	}
}

func liveValidationOperatorTenantContext() identity.TenantContext {
	return identity.TenantContext{
		TenantID:    "ten_1",
		PrincipalID: "prn_operator",
		Role:        identity.RoleOperator,
		Permissions: identity.PermissionsForRole(identity.RoleOperator, identity.StatusActive),
	}
}

func exerciseLiveValidationRoute(t *testing.T, manager *livevalidation.Manager, tenantContext identity.TenantContext, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/live-validations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(tenantctx.WithContext(req.Context(), tenantContext))
	response := httptest.NewRecorder()
	handleLiveValidationRoutes(manager, nil, nil, response, req)
	return response
}

func liveValidationStartBody(validationID, toolClass string, approvals []map[string]any) string {
	return liveValidationStartBodyForCandidate(validationID, "candidate_1", toolClass, approvals)
}

func liveValidationStartBodyForCandidate(validationID, candidateID, toolClass string, approvals []map[string]any) string {
	payload := map[string]any{
		"validationId":         validationID,
		"candidateId":          candidateID,
		"candidateToolClasses": []string{toolClass},
		"requestedScope": map[string]any{
			"scopeId":             "scope_" + validationID,
			"includedToolClasses": []string{toolClass},
			"approvalMode":        "scope_level",
			"declaredBy":          "prn_operator",
			"declaredAt":          "2026-04-29T10:00:00Z",
		},
	}
	if approvals != nil {
		payload["freshApprovals"] = approvals
	}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}

func liveValidationStartBodyForCandidateWithoutClasses(validationID, candidateID, toolClass string, approvals []map[string]any) string {
	payload := map[string]any{
		"validationId": validationID,
		"candidateId":  candidateID,
		"requestedScope": map[string]any{
			"scopeId":             "scope_" + validationID,
			"includedToolClasses": []string{toolClass},
			"approvalMode":        "scope_level",
			"declaredBy":          "prn_operator",
			"declaredAt":          "2026-04-29T10:00:00Z",
		},
	}
	if approvals != nil {
		payload["freshApprovals"] = approvals
	}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}
