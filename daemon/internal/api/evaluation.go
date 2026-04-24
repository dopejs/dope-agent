package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type ReplayCandidateResource = evaluation.ReplayCandidate
type ReplayAttemptResource = evaluation.ReplayAttempt
type ReplayComparisonResource = evaluation.ComparisonResult
type ReplayFixtureResource = evaluation.RegressionFixture

type ReplayCandidateListResponse struct {
	EnvironmentScope string                    `json:"environmentScope"`
	Items            []ReplayCandidateResource `json:"items"`
}

type ReplayAttemptListResponse struct {
	EnvironmentScope string                  `json:"environmentScope"`
	Items            []ReplayAttemptResource `json:"items"`
}

type ReplayComparisonListResponse struct {
	EnvironmentScope string                     `json:"environmentScope"`
	Items            []ReplayComparisonResource `json:"items"`
}

type ReplayFixtureListResponse struct {
	EnvironmentScope string                  `json:"environmentScope"`
	Items            []ReplayFixtureResource `json:"items"`
}

type CreateReplayAttemptRequest = evaluation.CreateReplayAttemptInput
type CreateReplayComparisonRequest = evaluation.CreateComparisonInput

func handleEvaluationRoutes(manager *evaluation.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "evaluation manager is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/evaluation/")
	switch {
	case path == "fixtures":
		handleEvaluationFixtures(manager, w, r)
	case path == "replay-candidates":
		handleEvaluationReplayCandidates(manager, w, r)
	case strings.HasPrefix(path, "replay-candidates/"):
		handleEvaluationReplayCandidateRoutes(manager, eventBus, sqliteStore, strings.TrimPrefix(path, "replay-candidates/"), w, r)
	case path == "replay-attempts":
		handleEvaluationReplayAttempts(manager, w, r)
	case strings.HasPrefix(path, "replay-attempts/"):
		handleEvaluationReplayAttemptRoutes(manager, eventBus, sqliteStore, strings.TrimPrefix(path, "replay-attempts/"), w, r)
	case path == "comparisons":
		handleEvaluationComparisons(manager, w, r)
	case strings.HasPrefix(path, "comparisons/"):
		handleEvaluationComparisonRoutes(manager, strings.TrimPrefix(path, "comparisons/"), w, r)
	default:
		writeError(w, http.StatusNotFound, "evaluation route not found")
	}
}

func handleEvaluationReplayCandidates(manager *evaluation.Manager, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var input ReplayCandidateResource
		if err := decodeOptionalJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if input.CandidateKind == "" {
			input.CandidateKind = evaluation.CandidateKindCuratedWork
		}
		if input.CandidateKind == evaluation.CandidateKindFixture {
			writeError(w, http.StatusBadRequest, "fixture replay candidates are managed by repo fixtures")
			return
		}
		if strings.TrimSpace(input.CandidateID) == "" {
			writeError(w, http.StatusBadRequest, "candidateId is required")
			return
		}
		if err := manager.UpsertReplayCandidate(r.Context(), input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		created, ok, err := manager.GetReplayCandidate(r.Context(), input.CandidateID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusInternalServerError, "created replay candidate not found")
			return
		}
		writeJSON(w, http.StatusCreated, created)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	items, err := manager.ListReplayCandidates(r.Context(), evaluation.CandidateFilter{
		CandidateKind:   evaluation.CandidateKind(r.URL.Query().Get("candidateKind")),
		SourceKind:      evaluation.SourceKind(r.URL.Query().Get("sourceKind")),
		ReadinessStatus: evaluation.ReadinessStatus(r.URL.Query().Get("readinessStatus")),
		Limit:           queryInt(r, "limit"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ReplayCandidateListResponse{EnvironmentScope: events.EnvironmentScopeFromContext(r.Context()), Items: items})
}

func handleEvaluationReplayCandidateRoutes(manager *evaluation.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, path string, w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 1 && r.Method == http.MethodGet {
		item, ok, err := manager.GetReplayCandidate(r.Context(), parts[0])
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "replay candidate not found")
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	if len(parts) == 2 && parts[1] == "attempts" && r.Method == http.MethodPost {
		var input CreateReplayAttemptRequest
		if err := decodeOptionalJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		attempt, err := manager.CreateReplayAttempt(r.Context(), parts[0], input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		publishEvaluationReplayEvent(r.Context(), eventBus, sqliteStore, "evaluation.replay_started", attempt)
		switch attempt.Status {
		case evaluation.ReplayAttemptStatusCompleted:
			publishEvaluationReplayEvent(r.Context(), eventBus, sqliteStore, "evaluation.replay_completed", attempt)
		case evaluation.ReplayAttemptStatusBlocked:
			publishEvaluationReplayEvent(r.Context(), eventBus, sqliteStore, "evaluation.replay_blocked", attempt)
		case evaluation.ReplayAttemptStatusUnreplayable:
			publishEvaluationReplayEvent(r.Context(), eventBus, sqliteStore, "evaluation.replay_unreplayable", attempt)
		case evaluation.ReplayAttemptStatusFailed:
			publishEvaluationReplayEvent(r.Context(), eventBus, sqliteStore, "evaluation.replay_failed", attempt)
		}
		writeJSON(w, http.StatusAccepted, attempt)
		return
	}
	writeError(w, http.StatusNotFound, "replay candidate route not found")
}

func handleEvaluationReplayAttempts(manager *evaluation.Manager, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	items, err := manager.ListReplayAttempts(r.Context(), evaluation.AttemptFilter{
		CandidateID: r.URL.Query().Get("candidateId"),
		Status:      evaluation.ReplayAttemptStatus(r.URL.Query().Get("status")),
		Limit:       queryInt(r, "limit"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ReplayAttemptListResponse{EnvironmentScope: events.EnvironmentScopeFromContext(r.Context()), Items: items})
}

func handleEvaluationReplayAttemptRoutes(manager *evaluation.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, path string, w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 1 && r.Method == http.MethodGet {
		item, ok, err := manager.GetReplayAttempt(r.Context(), parts[0])
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "replay attempt not found")
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	if len(parts) == 2 && parts[1] == "compare" && r.Method == http.MethodPost {
		var input CreateReplayComparisonRequest
		if err := decodeOptionalJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		comparison, err := manager.CreateComparison(r.Context(), parts[0], input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		publishEvaluationComparisonEvent(r.Context(), eventBus, sqliteStore, comparison)
		writeJSON(w, http.StatusCreated, comparison)
		return
	}
	writeError(w, http.StatusNotFound, "replay attempt route not found")
}

func handleEvaluationComparisons(manager *evaluation.Manager, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	items, err := manager.ListComparisons(r.Context(), evaluation.ComparisonFilter{
		CandidateID:    r.URL.Query().Get("candidateId"),
		AttemptID:      r.URL.Query().Get("attemptId"),
		TerminalStatus: evaluation.ComparisonTerminalStatus(r.URL.Query().Get("terminalStatus")),
		Limit:          queryInt(r, "limit"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ReplayComparisonListResponse{EnvironmentScope: events.EnvironmentScopeFromContext(r.Context()), Items: items})
}

func handleEvaluationComparisonRoutes(manager *evaluation.Manager, path string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	item, ok, err := manager.GetComparison(r.Context(), strings.Trim(path, "/"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "comparison not found")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func handleEvaluationFixtures(manager *evaluation.Manager, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	items, err := manager.ListFixtures(r.Context(), evaluation.FixtureFilter{
		DomainClass: evaluation.FixtureDomainClass(r.URL.Query().Get("domainClass")),
		Limit:       queryInt(r, "limit"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ReplayFixtureListResponse{EnvironmentScope: events.EnvironmentScopeFromContext(r.Context()), Items: items})
}

func publishEvaluationReplayEvent(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, name string, attempt evaluation.ReplayAttempt) {
	if eventBus == nil {
		return
	}
	_, _ = publishEvent(ctx, eventBus, sqliteStore, events.Event{
		Category: "evaluation",
		Name:     name,
		Scope: events.Scope{
			RunID:      attempt.ResultRunID,
			WorkflowID: attempt.ResultWorkflowID,
		},
		Resource: events.Resource{Kind: "replay_attempt", ID: attempt.AttemptID},
		Payload: map[string]any{
			"candidateId":      attempt.CandidateID,
			"attemptId":        attempt.AttemptID,
			"mode":             attempt.Mode,
			"status":           attempt.Status,
			"environmentScope": attempt.EnvironmentScope,
			"resultRunId":      attempt.ResultRunID,
			"resultWorkflowId": attempt.ResultWorkflowID,
			"blockedReasons":   attempt.BlockedReasons,
		},
	})
}

func publishEvaluationComparisonEvent(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, comparison evaluation.ComparisonResult) {
	if eventBus == nil {
		return
	}
	planes := make([]string, 0, len(comparison.DriftFindings))
	for _, finding := range comparison.DriftFindings {
		planes = append(planes, string(finding.Plane))
	}
	_, _ = publishEvent(ctx, eventBus, sqliteStore, events.Event{
		Category: "evaluation",
		Name:     "evaluation.comparison_completed",
		Resource: events.Resource{Kind: "replay_comparison", ID: comparison.ComparisonID},
		Payload: map[string]any{
			"candidateId":      comparison.CandidateID,
			"attemptId":        comparison.AttemptID,
			"comparisonId":     comparison.ComparisonID,
			"terminalStatus":   comparison.TerminalStatus,
			"environmentScope": comparison.EnvironmentScope,
			"driftPlanes":      planes,
		},
	})
}

func decodeOptionalJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

func queryInt(r *http.Request, name string) int {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return value
}
