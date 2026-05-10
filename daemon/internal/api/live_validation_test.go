package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
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

func TestLiveValidationSlackSmokeProjectsTenantSafeEvidence(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	manager := livevalidation.NewManager(livevalidation.Dependencies{Enabled: true})
	tenantContext := identity.TenantContext{
		TenantID:    "ten_slack",
		PrincipalID: "prn_operator",
		Permissions: []identity.Permission{
			identity.PermissionLiveValidationExecute,
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}
	validatedAt := time.Date(2026, 5, 8, 14, 0, 0, 0, time.UTC)
	postBody := `{"connectorId":"slack-main","workspaceBindingId":"workspace_binding_redacted","status":"passed","authorizationMode":"fake_oauth","validatedAt":"` + validatedAt.Format(time.RFC3339) + `","safeEvidence":{"mode":"fake"}}`
	postReq := httptest.NewRequest(http.MethodPost, "/v1/live-validations/slack-smoke", bytes.NewBufferString(postBody))
	postReq.Header.Set("Content-Type", "application/json")
	postReq = postReq.WithContext(tenantctx.WithContext(postReq.Context(), tenantContext))
	postRec := httptest.NewRecorder()
	handleLiveValidationRoutes(manager, nil, sqliteStore, postRec, postReq)
	if postRec.Code != http.StatusCreated {
		t.Fatalf("POST slack-smoke status=%d body=%s", postRec.Code, postRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/live-validations/slack-smoke?connectorId=slack-main", nil)
	getReq = getReq.WithContext(tenantctx.WithContext(getReq.Context(), tenantContext))
	getRec := httptest.NewRecorder()
	handleLiveValidationRoutes(manager, nil, sqliteStore, getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET slack-smoke status=%d body=%s", getRec.Code, getRec.Body.String())
	}
	body := getRec.Body.String()
	if !bytes.Contains([]byte(body), []byte(`"status":"passed"`)) || !bytes.Contains([]byte(body), []byte(`"authorizationMode":"fake_oauth"`)) {
		t.Fatalf("Slack smoke projection missing expected fields: %s", body)
	}
	if bytes.Contains([]byte(body), []byte("xoxb-")) || bytes.Contains([]byte(body), []byte("secret")) {
		t.Fatalf("Slack smoke projection leaked unsafe evidence: %s", body)
	}
}

func TestLiveValidationMatrixSmokeRecordsStructuredSkipEvidence(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	manager := livevalidation.NewManager(livevalidation.Dependencies{Enabled: true})
	tenantContext := identity.TenantContext{
		TenantID:    "ten_matrix",
		PrincipalID: "prn_operator",
		Permissions: []identity.Permission{
			identity.PermissionLiveValidationExecute,
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}
	validatedAt := time.Date(2026, 5, 10, 14, 0, 0, 0, time.UTC)
	postBody := `{"connectorId":"matrix-main","homeserverBindingId":"matrix_hs_1","status":"skipped","authorizationMode":"unavailable","owner":"operator","reason":"safe Matrix credentials unavailable","validatedAt":"` + validatedAt.Format(time.RFC3339) + `","safeEvidence":{"policy":"structured_skip"}}`
	postReq := httptest.NewRequest(http.MethodPost, "/v1/live-validations/matrix-smoke", bytes.NewBufferString(postBody))
	postReq.Header.Set("Content-Type", "application/json")
	postReq = postReq.WithContext(tenantctx.WithContext(postReq.Context(), tenantContext))
	postRec := httptest.NewRecorder()
	handleLiveValidationRoutes(manager, nil, sqliteStore, postRec, postReq)
	if postRec.Code != http.StatusCreated {
		t.Fatalf("POST matrix-smoke status=%d body=%s", postRec.Code, postRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/live-validations/matrix-smoke?connectorId=matrix-main", nil)
	getReq = getReq.WithContext(tenantctx.WithContext(getReq.Context(), tenantContext))
	getRec := httptest.NewRecorder()
	handleLiveValidationRoutes(manager, nil, sqliteStore, getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET matrix-smoke status=%d body=%s", getRec.Code, getRec.Body.String())
	}
	body := getRec.Body.String()
	if !bytes.Contains([]byte(body), []byte(`"status":"skipped"`)) || !bytes.Contains([]byte(body), []byte(`"authorizationMode":"unavailable"`)) {
		t.Fatalf("Matrix smoke projection missing expected fields: %s", body)
	}
	if bytes.Contains([]byte(body), []byte("matrix-token-do-not-leak")) || bytes.Contains([]byte(body), []byte("accessToken")) {
		t.Fatalf("Matrix smoke projection leaked unsafe evidence: %s", body)
	}
}

func TestLiveValidationMatrixSmokeSafeLiveExecutesProviderProbe(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	var sentBody map[string]string
	matrixServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.EscapedPath() {
		case "/_matrix/client/v3/account/whoami":
			_, _ = w.Write([]byte(`{"user_id":"@bot:example.org","device_id":"DEVICE1"}`))
		case "/_matrix/client/v3/rooms/%21room:example.org/state/m.room.member/@bot:example.org":
			_, _ = w.Write([]byte(`{"membership":"join"}`))
		default:
			if r.Method != http.MethodPut {
				t.Fatalf("unexpected Matrix smoke request: %s %s", r.Method, r.URL.String())
			}
			if err := json.NewDecoder(r.Body).Decode(&sentBody); err != nil {
				t.Fatalf("decode Matrix smoke send body: %v", err)
			}
			_, _ = w.Write([]byte(`{"event_id":"$smoke_reply"}`))
		}
	}))
	t.Cleanup(matrixServer.Close)

	now := time.Date(2026, 5, 10, 14, 0, 0, 0, time.UTC)
	if err := sqliteStore.SaveMatrixRoutePolicy(context.Background(), store.MatrixRoutePolicyRecord{
		TenantID:            "ten_matrix",
		ConnectorID:         "matrix-main",
		HomeserverBindingID: "matrix_hs_1",
		SelectedRooms: []store.MatrixConversationRouteRecord{{
			ConversationID:     "!room:example.org",
			ConversationType:   "room",
			RoomSelectionState: "selected",
			ValidationState:    "valid",
			RedactionStatus:    "redacted",
		}},
		RoomInvocationGate:  "bot_mention_or_command_required",
		ConfiguredCommands:  []string{"!dope"},
		EncryptedRoomPolicy: "unsupported",
		ValidationState:     "valid",
		ValidatedAt:         now,
		RedactionStatus:     "redacted",
	}); err != nil {
		t.Fatalf("SaveMatrixRoutePolicy returned error: %v", err)
	}
	if err := sqliteStore.SaveMatrixHostedSetup(context.Background(), store.MatrixHostedSetupRecord{
		TenantID:            "ten_matrix",
		ConnectorID:         "matrix-main",
		ConnectorKind:       "matrix",
		DisplayName:         "Matrix Main",
		Status:              "healthy",
		TerminalState:       "ready",
		BotCredentialState:  "valid",
		HomeserverState:     "reachable",
		RoutePolicyState:    "valid",
		DeliveryEligible:    true,
		HomeserverBindingID: "matrix_hs_1",
		ReasonCode:          "healthy",
		RedactionStatus:     "redacted",
		CreatedAt:           now,
		UpdatedAt:           now,
		ValidatedAt:         now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
		HomeserverBinding: &store.MatrixHomeserverBindingRecord{
			HomeserverBindingID:       "matrix_hs_1",
			HomeserverURL:             matrixServer.URL,
			BotUserID:                 "@bot:example.org",
			AuthorizationState:        "valid",
			HomeserverCapabilityState: "valid",
			ValidatedAt:               now,
			RedactionStatus:           "redacted",
		},
	}); err != nil {
		t.Fatalf("SaveMatrixHostedSetup returned error: %v", err)
	}

	manager := livevalidation.NewManager(livevalidation.Dependencies{Enabled: true})
	executor := newMatrixSmokeExecutor(sqliteStore, nil, config.MatrixConnectorConfig{
		ConnectorID:     "matrix-main",
		HomeserverURL:   matrixServer.URL,
		BotAccessToken:  "matrix-token-do-not-leak",
		BotUserID:       "@bot:example.org",
		SelectedRoomIDs: []string{"!room:example.org"},
	})
	tenantContext := identity.TenantContext{
		TenantID:    "ten_matrix",
		PrincipalID: "prn_operator",
		Permissions: []identity.Permission{
			identity.PermissionLiveValidationExecute,
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}
	postBody := `{"connectorId":"matrix-main","homeserverBindingId":"matrix_hs_1","status":"passed","authorizationMode":"safe_live","owner":"operator","validatedAt":"` + now.Format(time.RFC3339) + `"}`
	postReq := httptest.NewRequest(http.MethodPost, "/v1/live-validations/matrix-smoke", bytes.NewBufferString(postBody))
	postReq.Header.Set("Content-Type", "application/json")
	postReq = postReq.WithContext(tenantctx.WithContext(postReq.Context(), tenantContext))
	postRec := httptest.NewRecorder()
	handleLiveValidationRoutes(manager, nil, sqliteStore, postRec, postReq, executor)
	if postRec.Code != http.StatusCreated {
		t.Fatalf("POST matrix safe-live smoke status=%d body=%s", postRec.Code, postRec.Body.String())
	}
	if sentBody["msgtype"] != "m.text" || sentBody["body"] == "" {
		t.Fatalf("expected Matrix safe-live smoke to send m.text body, got %+v", sentBody)
	}
	if !bytes.Contains(postRec.Body.Bytes(), []byte(`"authorizationMode":"safe_live"`)) || !bytes.Contains(postRec.Body.Bytes(), []byte(`"$smoke_reply"`)) {
		t.Fatalf("safe-live Matrix smoke response missing execution evidence: %s", postRec.Body.String())
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
