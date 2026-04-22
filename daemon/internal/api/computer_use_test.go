package api

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/artifacts"
	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

func TestComputerUseSessionAndApprovalRoutes(t *testing.T) {
	eventBus := events.NewBus()
	runtimeManager := runtime.NewManager()
	policyEngine := policy.NewEngine()
	authManager := auth.NewManager()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	checkpointManager := checkpoints.NewManager(sqliteStore, runtimeManager)
	server := NewServer(Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			BindAddr:    "127.0.0.1:0",
			DataDir:     t.TempDir(),
		},
		Logger:      telemetry.New("error").Slog(),
		EventBus:    eventBus,
		Policy:      policyEngine,
		Auth:        authManager,
		Runtime:     runtimeManager,
		Store:       sqliteStore,
		Checkpoints: checkpointManager,
		ComputerUse: computeruse.NewManager(computeruse.Dependencies{
			EnvironmentScope: "test",
			Runtime:          runtimeManager,
			Policy:           policyEngine,
			Store:            sqliteStore,
			Artifacts:        artifacts.NewService(t.TempDir()),
		}),
	})

	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "computer-use api"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	authHeader := issueAuthHeaderForTest(t, authManager, "web-ui")

	createSessionReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/computer-use/sessions", strings.NewReader(`{"driverKind":"browser"}`))
	createSessionReq.Header.Set("Authorization", authHeader)
	createSessionReq.Header.Set("Content-Type", "application/json")
	createSessionRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createSessionRec, createSessionReq)
	if createSessionRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createSessionRec.Code, createSessionRec.Body.String())
	}
	session := decodeStrictResponse[computeruse.Session](t, createSessionRec.Body.Bytes())

	createActionReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/computer-use/sessions/"+session.ComputerUseSessionID+"/actions", strings.NewReader(`{"actionKind":"input","value":"Phase 26","targetMatchContext":{"matchStrategy":"dom_selector","expectedSelector":"#name"}}`))
	createActionReq.Header.Set("Authorization", authHeader)
	createActionReq.Header.Set("Content-Type", "application/json")
	createActionRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createActionRec, createActionReq)
	if createActionRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 approval gate, got %d body=%s", createActionRec.Code, createActionRec.Body.String())
	}
	gated := decodeStrictResponse[map[string]any](t, createActionRec.Body.Bytes())
	approval := gated["approval"].(map[string]any)
	action := gated["action"].(map[string]any)

	resolveReq := httptest.NewRequest(http.MethodPost, "/v1/policy/approvals/"+approval["approvalId"].(string)+"/resolve", strings.NewReader(`{"resolution":"approved","comment":"allow"}`))
	resolveReq.Header.Set("Authorization", authHeader)
	resolveReq.Header.Set("Content-Type", "application/json")
	resolveRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("expected 200 approval resolution, got %d body=%s", resolveRec.Code, resolveRec.Body.String())
	}

	getActionReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+run.RunID+"/computer-use/sessions/"+session.ComputerUseSessionID+"/actions/"+action["computerUseActionId"].(string), nil)
	getActionReq.Header.Set("Authorization", authHeader)
	getActionRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getActionRec, getActionReq)
	if getActionRec.Code != http.StatusOK {
		t.Fatalf("expected 200 action detail, got %d body=%s", getActionRec.Code, getActionRec.Body.String())
	}
	completed := decodeStrictResponse[computeruse.Action](t, getActionRec.Body.Bytes())
	if completed.Status != computeruse.ActionStatusCompleted {
		t.Fatalf("expected completed action, got %+v", completed)
	}
	if len(completed.Artifacts) == 0 {
		t.Fatalf("expected evidence artifacts, got %+v", completed)
	}

	artifactID := completed.Artifacts[0].ArtifactID
	getArtifactReq := httptest.NewRequest(http.MethodGet, "/v1/computer-use/artifacts/"+artifactID, nil)
	getArtifactReq.Header.Set("Authorization", authHeader)
	getArtifactRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getArtifactRec, getArtifactReq)
	if getArtifactRec.Code != http.StatusOK {
		t.Fatalf("expected 200 artifact detail, got %d body=%s", getArtifactRec.Code, getArtifactRec.Body.String())
	}

	getContentReq := httptest.NewRequest(http.MethodGet, "/v1/computer-use/artifacts/"+artifactID+"/content", nil)
	getContentReq.Header.Set("Authorization", authHeader)
	getContentRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getContentRec, getContentReq)
	if getContentRec.Code != http.StatusOK {
		t.Fatalf("expected 200 artifact content, got %d body=%s", getContentRec.Code, getContentRec.Body.String())
	}
	content := decodeStrictResponse[ComputerUseArtifactContentResponse](t, getContentRec.Body.Bytes())
	if content.Status != string(completed.Artifacts[0].Status) {
		t.Fatalf("expected artifact status %s, got %+v", completed.Artifacts[0].Status, content)
	}
	if _, err := base64.StdEncoding.DecodeString(content.Content); err != nil {
		t.Fatalf("expected base64 content, got %v", err)
	}
	capabilityEvents := eventBus.List(events.Filter{RunID: run.RunID, Category: "capability"})
	var artifactEventFound bool
	for _, event := range capabilityEvents {
		if event.Name == "computer_use.artifact_recorded" {
			artifactEventFound = true
			break
		}
	}
	if !artifactEventFound {
		t.Fatalf("expected artifact recorded event, got %+v", capabilityEvents)
	}
}

func TestComputerUseRoutesFilterArtifactsByEnvironmentAndExposeTargetMismatch(t *testing.T) {
	eventBus := events.NewBus()
	runtimeManager := runtime.NewManager()
	policyEngine := policy.NewEngine()
	authManager := auth.NewManager()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	checkpointManager := checkpoints.NewManager(sqliteStore, runtimeManager)
	server := NewServer(Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			BindAddr:    "127.0.0.1:0",
			DataDir:     t.TempDir(),
		},
		Logger:      telemetry.New("error").Slog(),
		EventBus:    eventBus,
		Policy:      policyEngine,
		Auth:        authManager,
		Runtime:     runtimeManager,
		Store:       sqliteStore,
		Checkpoints: checkpointManager,
		ComputerUse: computeruse.NewManager(computeruse.Dependencies{
			EnvironmentScope: "test",
			Runtime:          runtimeManager,
			Policy:           policyEngine,
			Store:            sqliteStore,
			Artifacts:        artifacts.NewService(t.TempDir()),
		}),
	})

	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "computer-use mismatch"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}
	authHeader := issueAuthHeaderForTest(t, authManager, "web-ui")

	createSessionReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/computer-use/sessions", strings.NewReader(`{"driverKind":"browser"}`))
	createSessionReq.Header.Set("Authorization", authHeader)
	createSessionReq.Header.Set("Content-Type", "application/json")
	createSessionRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createSessionRec, createSessionReq)
	if createSessionRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createSessionRec.Code, createSessionRec.Body.String())
	}
	session := decodeStrictResponse[computeruse.Session](t, createSessionRec.Body.Bytes())

	mismatchReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/computer-use/sessions/"+session.ComputerUseSessionID+"/actions", strings.NewReader(`{"actionKind":"click","targetMatchContext":{"matchStrategy":"dom_selector","expectedSelector":"#missing-button"}}`))
	mismatchReq.Header.Set("Authorization", authHeader)
	mismatchReq.Header.Set("Content-Type", "application/json")
	mismatchRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(mismatchRec, mismatchReq)
	if mismatchRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 approval gate, got %d body=%s", mismatchRec.Code, mismatchRec.Body.String())
	}
	gated := decodeStrictResponse[map[string]any](t, mismatchRec.Body.Bytes())
	approval := gated["approval"].(map[string]any)
	action := gated["action"].(map[string]any)
	resolveReq := httptest.NewRequest(http.MethodPost, "/v1/policy/approvals/"+approval["approvalId"].(string)+"/resolve", strings.NewReader(`{"resolution":"approved","comment":"allow"}`))
	resolveReq.Header.Set("Authorization", authHeader)
	resolveReq.Header.Set("Content-Type", "application/json")
	resolveRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("expected 200 approval resolution, got %d body=%s", resolveRec.Code, resolveRec.Body.String())
	}

	getActionReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+run.RunID+"/computer-use/sessions/"+session.ComputerUseSessionID+"/actions/"+action["computerUseActionId"].(string), nil)
	getActionReq.Header.Set("Authorization", authHeader)
	getActionRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getActionRec, getActionReq)
	if getActionRec.Code != http.StatusOK {
		t.Fatalf("expected 200 action detail, got %d body=%s", getActionRec.Code, getActionRec.Body.String())
	}
	mismatch := decodeStrictResponse[computeruse.Action](t, getActionRec.Body.Bytes())
	if mismatch.Status != computeruse.ActionStatusFailed || mismatch.FailureClass != string(computeruse.FailureClassTargetMismatch) {
		t.Fatalf("expected target mismatch failure, got %+v", mismatch)
	}
	if len(mismatch.Artifacts) == 0 {
		t.Fatalf("expected mismatch evidence, got %+v", mismatch)
	}
	capabilityEvents := eventBus.List(events.Filter{RunID: run.RunID, Category: "capability"})
	var mismatchEventFound bool
	for _, event := range capabilityEvents {
		if event.Name == "computer_use.action_target_mismatch" {
			mismatchEventFound = true
			break
		}
	}
	if !mismatchEventFound {
		t.Fatalf("expected target mismatch event, got %+v", capabilityEvents)
	}

	prodArtifact := computeruse.Artifact{
		ArtifactID:           "cuart_prod_hidden",
		EnvironmentScope:     "prod",
		ComputerUseSessionID: session.ComputerUseSessionID,
		ComputerUseActionID:  mismatch.ComputerUseActionID,
		RunID:                run.RunID,
		Kind:                 computeruse.ArtifactKindPageSnapshot,
		Status:               computeruse.ArtifactStatusAvailable,
		StorageKey:           "computer-use/prod/hidden",
		CreatedAt:            mismatch.UpdatedAt,
	}
	if err := sqliteStore.UpsertComputerUseArtifact(context.Background(), prodArtifact); err != nil {
		t.Fatalf("UpsertComputerUseArtifact returned error: %v", err)
	}

	getHiddenReq := httptest.NewRequest(http.MethodGet, "/v1/computer-use/artifacts/"+prodArtifact.ArtifactID, nil)
	getHiddenReq.Header.Set("Authorization", authHeader)
	getHiddenRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getHiddenRec, getHiddenReq)
	if getHiddenRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-environment artifact, got %d body=%s", getHiddenRec.Code, getHiddenRec.Body.String())
	}
}

func TestComputerUseApprovalDenialAndNavigationFailureAreInspectable(t *testing.T) {
	eventBus := events.NewBus()
	runtimeManager := runtime.NewManager()
	policyEngine := policy.NewEngine()
	authManager := auth.NewManager()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	checkpointManager := checkpoints.NewManager(sqliteStore, runtimeManager)
	server := NewServer(Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			BindAddr:    "127.0.0.1:0",
			DataDir:     t.TempDir(),
		},
		Logger:      telemetry.New("error").Slog(),
		EventBus:    eventBus,
		Policy:      policyEngine,
		Auth:        authManager,
		Runtime:     runtimeManager,
		Store:       sqliteStore,
		Checkpoints: checkpointManager,
		ComputerUse: computeruse.NewManager(computeruse.Dependencies{
			EnvironmentScope: "test",
			Runtime:          runtimeManager,
			Policy:           policyEngine,
			Store:            sqliteStore,
			Artifacts:        artifacts.NewService(t.TempDir()),
		}),
	})

	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "computer-use denial"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}
	authHeader := issueAuthHeaderForTest(t, authManager, "web-ui")

	createSessionReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/computer-use/sessions", strings.NewReader(`{"driverKind":"browser"}`))
	createSessionReq.Header.Set("Authorization", authHeader)
	createSessionReq.Header.Set("Content-Type", "application/json")
	createSessionRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createSessionRec, createSessionReq)
	if createSessionRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createSessionRec.Code, createSessionRec.Body.String())
	}
	session := decodeStrictResponse[computeruse.Session](t, createSessionRec.Body.Bytes())

	inputReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/computer-use/sessions/"+session.ComputerUseSessionID+"/actions", strings.NewReader(`{"actionKind":"input","value":"deny me","targetMatchContext":{"matchStrategy":"dom_selector","expectedSelector":"#name"}}`))
	inputReq.Header.Set("Authorization", authHeader)
	inputReq.Header.Set("Content-Type", "application/json")
	inputRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(inputRec, inputReq)
	if inputRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 approval gate, got %d body=%s", inputRec.Code, inputRec.Body.String())
	}
	gated := decodeStrictResponse[map[string]any](t, inputRec.Body.Bytes())
	approval := gated["approval"].(map[string]any)
	action := gated["action"].(map[string]any)

	resolveReq := httptest.NewRequest(http.MethodPost, "/v1/policy/approvals/"+approval["approvalId"].(string)+"/resolve", strings.NewReader(`{"resolution":"rejected","comment":"deny"}`))
	resolveReq.Header.Set("Authorization", authHeader)
	resolveReq.Header.Set("Content-Type", "application/json")
	resolveRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("expected 200 approval resolution, got %d body=%s", resolveRec.Code, resolveRec.Body.String())
	}

	getActionReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+run.RunID+"/computer-use/sessions/"+session.ComputerUseSessionID+"/actions/"+action["computerUseActionId"].(string), nil)
	getActionReq.Header.Set("Authorization", authHeader)
	getActionRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getActionRec, getActionReq)
	if getActionRec.Code != http.StatusOK {
		t.Fatalf("expected 200 action detail, got %d body=%s", getActionRec.Code, getActionRec.Body.String())
	}
	denied := decodeStrictResponse[computeruse.Action](t, getActionRec.Body.Bytes())
	if denied.Status != computeruse.ActionStatusDenied || denied.FailureClass != string(computeruse.FailureClassPolicyDenied) {
		t.Fatalf("expected policy denial, got %+v", denied)
	}

	navigateReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/computer-use/sessions/"+session.ComputerUseSessionID+"/actions", strings.NewReader(`{"actionKind":"navigate"}`))
	navigateReq.Header.Set("Authorization", authHeader)
	navigateReq.Header.Set("Content-Type", "application/json")
	navigateRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(navigateRec, navigateReq)
	if navigateRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 navigation failure result, got %d body=%s", navigateRec.Code, navigateRec.Body.String())
	}
	navigate := decodeStrictResponse[computeruse.Action](t, navigateRec.Body.Bytes())
	if navigate.Status != computeruse.ActionStatusFailed || navigate.FailureClass != string(computeruse.FailureClassNavigationFailure) {
		t.Fatalf("expected navigation failure, got %+v", navigate)
	}
}

func TestComputerUseRoutesLatencyStaysLocal(t *testing.T) {
	eventBus := events.NewBus()
	runtimeManager := runtime.NewManager()
	policyEngine := policy.NewEngine()
	authManager := auth.NewManager()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	checkpointManager := checkpoints.NewManager(sqliteStore, runtimeManager)
	server := NewServer(Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			BindAddr:    "127.0.0.1:0",
			DataDir:     t.TempDir(),
		},
		Logger:      telemetry.New("error").Slog(),
		EventBus:    eventBus,
		Policy:      policyEngine,
		Auth:        authManager,
		Runtime:     runtimeManager,
		Store:       sqliteStore,
		Checkpoints: checkpointManager,
		ComputerUse: computeruse.NewManager(computeruse.Dependencies{
			EnvironmentScope: "test",
			Runtime:          runtimeManager,
			Policy:           policyEngine,
			Store:            sqliteStore,
			Artifacts:        artifacts.NewService(t.TempDir()),
		}),
	})

	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "computer-use latency api"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	authHeader := issueAuthHeaderForTest(t, authManager, "web-ui")

	createSessionReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/computer-use/sessions", strings.NewReader(`{"driverKind":"browser"}`))
	createSessionReq.Header.Set("Authorization", authHeader)
	createSessionReq.Header.Set("Content-Type", "application/json")
	createStarted := time.Now()
	createSessionRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createSessionRec, createSessionReq)
	if createSessionRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createSessionRec.Code, createSessionRec.Body.String())
	}
	if elapsed := time.Since(createStarted); elapsed > 500*time.Millisecond {
		t.Fatalf("expected session create under 500ms, got %s", elapsed)
	}
	session := decodeStrictResponse[computeruse.Session](t, createSessionRec.Body.Bytes())

	createActionReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/computer-use/sessions/"+session.ComputerUseSessionID+"/actions", strings.NewReader(`{"actionKind":"snapshot"}`))
	createActionReq.Header.Set("Authorization", authHeader)
	createActionReq.Header.Set("Content-Type", "application/json")
	actionStarted := time.Now()
	createActionRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createActionRec, createActionReq)
	if createActionRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createActionRec.Code, createActionRec.Body.String())
	}
	if elapsed := time.Since(actionStarted); elapsed > time.Second {
		t.Fatalf("expected action completion under 1s, got %s", elapsed)
	}
	action := decodeStrictResponse[computeruse.Action](t, createActionRec.Body.Bytes())
	if len(action.Artifacts) == 0 {
		t.Fatalf("expected artifacts, got %+v", action)
	}

	artifactStarted := time.Now()
	getArtifactReq := httptest.NewRequest(http.MethodGet, "/v1/computer-use/artifacts/"+action.Artifacts[0].ArtifactID, nil)
	getArtifactReq.Header.Set("Authorization", authHeader)
	getArtifactRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getArtifactRec, getArtifactReq)
	if getArtifactRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", getArtifactRec.Code, getArtifactRec.Body.String())
	}
	if elapsed := time.Since(artifactStarted); elapsed > 500*time.Millisecond {
		t.Fatalf("expected artifact lookup under 500ms, got %s", elapsed)
	}
}
