package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestRecoverPersistedStateRestoresRuntimeAndEventHistory(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	seedRuntime := runtime.NewManager()
	run, err := seedRuntime.CreateRun(runtime.CreateRunInput{
		SessionID:  "session_recovery",
		Entrypoint: "chat",
		Goal:       "recover after daemon restart",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := seedRuntime.CreateStep(run.RunID, runtime.CreateStepInput{
		Title: "plan recovery",
	})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	step, runUpdate, err := seedRuntime.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusPlanning,
	})
	if err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun returned error: %v", err)
	}
	if runUpdate != nil {
		run = *runUpdate
	}

	session := router.Session{
		SessionID:    run.SessionID,
		Kind:         router.SessionKindDirect,
		Status:       router.SessionStatusActive,
		Channel:      "local",
		AccountID:    "local",
		PeerID:       "chat",
		RoutingKey:   "direct:local:local:chat",
		Generation:   1,
		CreatedAt:    time.Now().UTC().Add(-time.Minute),
		UpdatedAt:    time.Now().UTC().Add(-time.Minute),
		LastActiveAt: time.Now().UTC().Add(-time.Minute),
	}
	if err := sqliteStore.UpsertSession(ctx, session); err != nil {
		t.Fatalf("UpsertSession returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(ctx, run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertStep(ctx, step); err != nil {
		t.Fatalf("UpsertStep returned error: %v", err)
	}

	checkpointManager := checkpoints.NewManager(sqliteStore, seedRuntime)
	if err := checkpointManager.SaveRunCheckpoint(ctx, run.RunID); err != nil {
		t.Fatalf("SaveRunCheckpoint returned error: %v", err)
	}

	persistedEvent := events.Event{
		EventID:    "evt_recovery",
		Category:   "run",
		Name:       "run.status_changed",
		OccurredAt: time.Now().UTC(),
		Scope: events.Scope{
			SessionID: run.SessionID,
			RunID:     run.RunID,
		},
		Resource: events.Resource{
			Kind: "run",
			ID:   run.RunID,
		},
		Payload: map[string]any{
			"status": run.Status,
		},
	}
	if _, err := sqliteStore.AppendEvent(ctx, persistedEvent); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}

	restoredRuntime := runtime.NewManager()
	restoredRouter := router.NewSessionRouter()
	restoredEventBus := events.NewBus()
	restoreCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)

	if err := recoverPersistedState(ctx, sqliteStore, restoredRouter, restoreCheckpoints, restoredEventBus); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	gotRun, ok := restoredRuntime.GetRun(run.RunID)
	if !ok {
		t.Fatal("expected restored run")
	}
	if gotRun.Status != runtime.RunStatusRunning {
		t.Fatalf("expected restored run status running, got %s", gotRun.Status)
	}

	gotStep, ok := restoredRuntime.GetStep(run.RunID, step.StepID)
	if !ok {
		t.Fatal("expected restored step")
	}
	if gotStep.Status != runtime.StepStatusPlanning {
		t.Fatalf("expected restored step status planning, got %s", gotStep.Status)
	}

	items := restoredEventBus.List(events.Filter{RunID: run.RunID})
	if len(items) != 1 {
		t.Fatalf("expected 1 restored event, got %d", len(items))
	}
	if items[0].EventID != persistedEvent.EventID {
		t.Fatalf("expected restored event ID %s, got %s", persistedEvent.EventID, items[0].EventID)
	}

	if _, ok := restoredRouter.GetSession(run.SessionID); !ok {
		t.Fatal("expected restored session")
	}
}

func TestAppRunPublishesLifecycleEvents(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")

	application, err := New()
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- application.Run(ctx)
	}()

	time.Sleep(150 * time.Millisecond)
	cancel()

	if err := <-errCh; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	reopenedStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := reopenedStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	items, err := reopenedStore.ListEvents(context.Background(), events.Filter{Category: "system"})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 system events, got %d", len(items))
	}
	if items[0].Name != "system.started" {
		t.Fatalf("expected system.started, got %s", items[0].Name)
	}
	if items[1].Name != "system.stopped" {
		t.Fatalf("expected system.stopped, got %s", items[1].Name)
	}
	if items[1].Sequence <= items[0].Sequence {
		t.Fatalf("expected monotonic system event sequence, got %d then %d", items[0].Sequence, items[1].Sequence)
	}
}

func TestAppRestartRestoresRuntimeBoundary(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "dope")
	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")

	first, err := New()
	if err != nil {
		t.Fatalf("first New returned error: %v", err)
	}

	createRunRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(createRunRec, httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(`{"entrypoint":"chat","goal":"restart-safe"}`)))
	if createRunRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for run create, got %d", createRunRec.Code)
	}
	var createdRun runtime.Run
	if err := json.Unmarshal(createRunRec.Body.Bytes(), &createdRun); err != nil {
		t.Fatalf("failed to decode created run: %v", err)
	}

	createStepRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(createStepRec, httptest.NewRequest(http.MethodPost, "/v1/runs/"+createdRun.RunID+"/steps", strings.NewReader(`{"title":"recover me","kind":"task"}`)))
	if createStepRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for step create, got %d", createStepRec.Code)
	}
	var createdStep runtime.Step
	if err := json.Unmarshal(createStepRec.Body.Bytes(), &createdStep); err != nil {
		t.Fatalf("failed to decode created step: %v", err)
	}

	updateStepRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(updateStepRec, httptest.NewRequest(http.MethodPost, "/v1/runs/"+createdRun.RunID+"/steps/"+createdStep.StepID+"/status", strings.NewReader(`{"status":"planning"}`)))
	if updateStepRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for step status update, got %d", updateStepRec.Code)
	}

	if err := first.Close(context.Background()); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}

	second, err := New()
	if err != nil {
		t.Fatalf("second New returned error: %v", err)
	}
	defer func() {
		if err := second.Close(context.Background()); err != nil {
			t.Fatalf("second Close returned error: %v", err)
		}
	}()

	gotRun, ok := second.Runtime.GetRun(createdRun.RunID)
	if !ok {
		t.Fatal("expected restored run")
	}
	if gotRun.Status != runtime.RunStatusRunning {
		t.Fatalf("expected restored run status running, got %s", gotRun.Status)
	}

	gotStep, ok := second.Runtime.GetStep(createdRun.RunID, createdStep.StepID)
	if !ok {
		t.Fatal("expected restored step")
	}
	if gotStep.Status != runtime.StepStatusPlanning {
		t.Fatalf("expected restored step status planning, got %s", gotStep.Status)
	}

	eventsRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(eventsRec, httptest.NewRequest(http.MethodGet, "/v1/runs/"+createdRun.RunID+"/events", nil))
	if eventsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for run events after restart, got %d", eventsRec.Code)
	}
	var eventList struct {
		Items      []events.Event `json:"items"`
		NextCursor int64          `json:"nextCursor"`
	}
	if err := json.Unmarshal(eventsRec.Body.Bytes(), &eventList); err != nil {
		t.Fatalf("failed to decode run events response: %v", err)
	}
	if len(eventList.Items) < 3 {
		t.Fatalf("expected at least 3 restored events, got %d", len(eventList.Items))
	}
	if eventList.NextCursor == 0 {
		t.Fatal("expected next cursor after restart")
	}
}
