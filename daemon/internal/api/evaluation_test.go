package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

func TestEvaluationRoutesLaunchReplayAndCompare(t *testing.T) {
	cfg := config.Config{Environment: config.EnvironmentTest, DataDir: filepath.Join(t.TempDir(), "dope-data")}
	sqliteStore, err := store.NewSQLiteStore(cfg.DataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	eventBus := events.NewBus()
	t.Cleanup(eventBus.Close)
	runtimeManager := runtime.NewManager()
	manager := evaluation.NewManager(evaluation.Dependencies{
		EnvironmentScope: "test",
		Store:            sqliteStore,
		FixturesDir:      filepath.Join("..", "evaluation", "testdata", "fixtures"),
		RuntimeRecorder:  evaluation.NewRuntimeReplayRecorder(runtimeManager, sqliteStore),
	})
	if err := manager.LoadFixtures(context.Background()); err != nil {
		t.Fatalf("LoadFixtures returned error: %v", err)
	}
	authManager := auth.NewManager()
	pairing, code, err := authManager.StartPairing(auth.StartPairingInput{Label: "eval-test", Mode: auth.PairingModeLocal})
	if err != nil {
		t.Fatalf("StartPairing returned error: %v", err)
	}
	_, _, tokenSecret, err := authManager.CompletePairing(pairing.PairingID, auth.CompletePairingInput{Code: code})
	if err != nil {
		t.Fatalf("CompletePairing returned error: %v", err)
	}
	server := NewServer(Dependencies{
		Config:     cfg,
		Logger:     telemetry.New("error").Slog(),
		EventBus:   eventBus,
		Auth:       authManager,
		Store:      sqliteStore,
		Runtime:    runtimeManager,
		Evaluation: manager,
	})

	candidatesRec := requestEvaluationRoute(t, server, http.MethodGet, "/v1/evaluation/replay-candidates", "", tokenSecret)
	if candidatesRec.Code != http.StatusOK {
		t.Fatalf("expected 200 candidates, got %d body=%s", candidatesRec.Code, candidatesRec.Body.String())
	}
	candidates := decodeStrictResponse[ReplayCandidateListResponse](t, candidatesRec.Body.Bytes())
	if len(candidates.Items) != 3 {
		t.Fatalf("expected 3 candidates, got %+v", candidates.Items)
	}
	createCandidateRec := requestEvaluationRoute(t, server, http.MethodPost, "/v1/evaluation/replay-candidates", `{
		"candidateId": "candidate_curated_run",
		"candidateKind": "curated_work",
		"displayName": "Curated Run",
		"sourceKind": "run",
		"sourceId": "run_curated",
		"sourceRefs": [{"kind":"run","id":"run_curated","route":"/v1/runs/run_curated"}],
		"environmentScope": "test",
		"readinessStatus": "partially_replayable",
		"readinessReasons": ["curated run has captured summaries"],
		"limitations": ["evidence-only replay"],
		"defaultReplayMode": "non_live",
		"expectedComparisonSummary": {"runtime":"runtime captured","policy":"policy captured","evidence":"evidence captured"}
	}`, tokenSecret)
	if createCandidateRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 create candidate, got %d body=%s", createCandidateRec.Code, createCandidateRec.Body.String())
	}
	createdCandidate := decodeStrictResponse[ReplayCandidateResource](t, createCandidateRec.Body.Bytes())
	if createdCandidate.CandidateID != "candidate_curated_run" || createdCandidate.CandidateKind != evaluation.CandidateKindCuratedWork {
		t.Fatalf("expected curated candidate, got %+v", createdCandidate)
	}
	missingSourceRec := requestEvaluationRoute(t, server, http.MethodPost, "/v1/evaluation/replay-candidates", `{
		"candidateId": "candidate_missing_source",
		"candidateKind": "curated_work",
		"displayName": "Missing Source"
	}`, tokenSecret)
	if missingSourceRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 missing source candidate, got %d body=%s", missingSourceRec.Code, missingSourceRec.Body.String())
	}
	fixtureCandidateRec := requestEvaluationRoute(t, server, http.MethodPost, "/v1/evaluation/replay-candidates", `{
		"candidateId": "candidate_api_fixture",
		"candidateKind": "fixture",
		"displayName": "API Fixture",
		"sourceKind": "fixture",
		"sourceId": "fixture_api",
		"sourceRefs": [{"kind":"fixture","id":"fixture_api"}],
		"environmentScope": "test",
		"readinessStatus": "fully_replayable",
		"readinessReasons": [],
		"limitations": [],
		"defaultReplayMode": "non_live"
	}`, tokenSecret)
	if fixtureCandidateRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 API fixture candidate, got %d body=%s", fixtureCandidateRec.Code, fixtureCandidateRec.Body.String())
	}
	fixturesRec := requestEvaluationRoute(t, server, http.MethodGet, "/v1/evaluation/fixtures", "", tokenSecret)
	if fixturesRec.Code != http.StatusOK {
		t.Fatalf("expected 200 fixtures, got %d body=%s", fixturesRec.Code, fixturesRec.Body.String())
	}
	fixtures := decodeStrictResponse[ReplayFixtureListResponse](t, fixturesRec.Body.Bytes())
	if len(fixtures.Items) != 3 {
		t.Fatalf("expected 3 fixtures, got %+v", fixtures.Items)
	}

	candidateRec := requestEvaluationRoute(t, server, http.MethodGet, "/v1/evaluation/replay-candidates/"+candidates.Items[0].CandidateID, "", tokenSecret)
	if candidateRec.Code != http.StatusOK {
		t.Fatalf("expected 200 candidate detail, got %d body=%s", candidateRec.Code, candidateRec.Body.String())
	}

	attemptRec := requestEvaluationRoute(t, server, http.MethodPost, "/v1/evaluation/replay-candidates/"+candidates.Items[0].CandidateID+"/attempts", "", tokenSecret)
	if attemptRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 attempt, got %d body=%s", attemptRec.Code, attemptRec.Body.String())
	}
	attempt := decodeStrictResponse[ReplayAttemptResource](t, attemptRec.Body.Bytes())
	if attempt.Mode != evaluation.ReplayModeNonLive || attempt.Status != evaluation.ReplayAttemptStatusCompleted {
		t.Fatalf("expected completed non-live attempt, got %+v", attempt)
	}
	if attempt.SourceRefs == nil || attempt.EvidenceRefs == nil || attempt.BlockedReasons == nil {
		t.Fatalf("expected attempt response collections to encode as arrays, got %+v", attempt)
	}
	if attempt.ResultRunID == "" {
		t.Fatalf("expected attempt resultRunId")
	}
	resultRunRec := requestEvaluationRoute(t, server, http.MethodGet, "/v1/runs/"+attempt.ResultRunID, "", tokenSecret)
	if resultRunRec.Code != http.StatusOK {
		t.Fatalf("expected 200 replay result run, got %d body=%s", resultRunRec.Code, resultRunRec.Body.String())
	}
	if attempt.ResultWorkflowID == "" {
		t.Fatalf("expected attempt resultWorkflowId")
	}
	resultWorkflowRec := requestEvaluationRoute(t, server, http.MethodGet, "/v1/runs/"+attempt.ResultRunID+"/workflows/"+attempt.ResultWorkflowID, "", tokenSecret)
	if resultWorkflowRec.Code != http.StatusOK {
		t.Fatalf("expected 200 replay result workflow, got %d body=%s", resultWorkflowRec.Code, resultWorkflowRec.Body.String())
	}
	attemptsRec := requestEvaluationRoute(t, server, http.MethodGet, "/v1/evaluation/replay-attempts", "", tokenSecret)
	if attemptsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 attempts, got %d body=%s", attemptsRec.Code, attemptsRec.Body.String())
	}
	attempts := decodeStrictResponse[ReplayAttemptListResponse](t, attemptsRec.Body.Bytes())
	if len(attempts.Items) != 1 {
		t.Fatalf("expected 1 attempt, got %+v", attempts.Items)
	}

	compareRec := requestEvaluationRoute(t, server, http.MethodPost, "/v1/evaluation/replay-attempts/"+attempt.AttemptID+"/compare", "", tokenSecret)
	if compareRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 comparison, got %d body=%s", compareRec.Code, compareRec.Body.String())
	}
	comparison := decodeStrictResponse[ReplayComparisonResource](t, compareRec.Body.Bytes())
	if comparison.TerminalStatus != evaluation.ComparisonMatched {
		t.Fatalf("expected matched comparison, got %+v", comparison)
	}
	if comparison.Limitations == nil || comparison.DriftFindings == nil {
		t.Fatalf("expected comparison response collections to encode as arrays, got %+v", comparison)
	}
	comparisonsRec := requestEvaluationRoute(t, server, http.MethodGet, "/v1/evaluation/comparisons", "", tokenSecret)
	if comparisonsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 comparisons, got %d body=%s", comparisonsRec.Code, comparisonsRec.Body.String())
	}
	comparisons := decodeStrictResponse[ReplayComparisonListResponse](t, comparisonsRec.Body.Bytes())
	if len(comparisons.Items) != 1 {
		t.Fatalf("expected 1 comparison, got %+v", comparisons.Items)
	}
	comparisonRec := requestEvaluationRoute(t, server, http.MethodGet, "/v1/evaluation/comparisons/"+comparison.ComparisonID, "", tokenSecret)
	if comparisonRec.Code != http.StatusOK {
		t.Fatalf("expected 200 comparison detail, got %d body=%s", comparisonRec.Code, comparisonRec.Body.String())
	}

	eventItems := eventBus.List(events.Filter{Category: "evaluation"})
	names := make([]string, 0, len(eventItems))
	var completedReplay *events.Event
	for _, item := range eventItems {
		names = append(names, item.Name)
		if item.Name == "evaluation.replay_completed" {
			completed := item
			completedReplay = &completed
		}
	}
	for _, expected := range []string{"evaluation.replay_started", "evaluation.replay_completed", "evaluation.comparison_completed"} {
		if !strings.Contains(strings.Join(names, ","), expected) {
			t.Fatalf("expected event %s in %+v", expected, names)
		}
	}
	if completedReplay == nil {
		t.Fatal("expected replay completed event")
	}
	if completedReplay.Scope.RunID != attempt.ResultRunID || completedReplay.Scope.WorkflowID != attempt.ResultWorkflowID {
		t.Fatalf("expected replay completed event scope to link result run/workflow, got %+v", completedReplay.Scope)
	}
	if completedReplay.Payload["resultRunId"] != attempt.ResultRunID || completedReplay.Payload["resultWorkflowId"] != attempt.ResultWorkflowID {
		t.Fatalf("expected replay completed event payload to link result run/workflow, got %+v", completedReplay.Payload)
	}
}

func requestEvaluationRoute(t *testing.T, server *Server, method string, route string, body string, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, route, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	return rec
}
