package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type Dependencies struct {
	Config       config.Config
	Logger       *slog.Logger
	EventBus     *events.Bus
	Policy       *policy.Engine
	Router       *router.SessionRouter
	Runtime      *runtime.Manager
	LLM          *llm.Dispatcher
	Connectors   *connectors.Supervisor
	Capabilities *capabilities.Supervisor
	Store        *store.SQLiteStore
	Checkpoints  *checkpoints.Manager
}

type Server struct {
	cfg          config.Config
	logger       *slog.Logger
	eventBus     *events.Bus
	policy       *policy.Engine
	router       *router.SessionRouter
	runtime      *runtime.Manager
	llm          *llm.Dispatcher
	connectors   *connectors.Supervisor
	capabilities *capabilities.Supervisor
	store        *store.SQLiteStore
	checkpoints  *checkpoints.Manager
	server       *http.Server
}

func NewServer(deps Dependencies) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"service": "dope",
		})
	})
	mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"version": deps.Config.Version,
		})
	})
	mux.HandleFunc("/v1/system/info", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, buildSystemInfoResponse(deps.Config))
	})
	mux.HandleFunc("/v1/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, buildConfigResponse(deps.Config))
	})
	mux.HandleFunc("/v1/events/stream", func(w http.ResponseWriter, r *http.Request) {
		streamEvents(deps.EventBus, deps.Store, w, r)
	})
	mux.HandleFunc("/v1/runs", func(w http.ResponseWriter, r *http.Request) {
		handleRuns(deps.Router, deps.Runtime, deps.EventBus, deps.Store, deps.Checkpoints, w, r)
	})
	mux.HandleFunc("/v1/runs/", func(w http.ResponseWriter, r *http.Request) {
		handleRunRoutes(deps.Runtime, deps.Capabilities, deps.EventBus, deps.Store, deps.Checkpoints, w, r)
	})
	mux.HandleFunc("/v1/sessions", func(w http.ResponseWriter, r *http.Request) {
		handleSessions(deps.Router, deps.EventBus, deps.Store, w, r)
	})
	mux.HandleFunc("/v1/sessions/", func(w http.ResponseWriter, r *http.Request) {
		handleSessionRoutes(deps.Router, deps.EventBus, deps.Store, w, r)
	})
	mux.HandleFunc("/v1/policy/approvals", func(w http.ResponseWriter, r *http.Request) {
		handlePolicyApprovals(deps.Policy, deps.EventBus, deps.Store, w, r)
	})
	mux.HandleFunc("/v1/policy/approvals/", func(w http.ResponseWriter, r *http.Request) {
		handlePolicyApprovalRoutes(deps.Policy, deps.EventBus, deps.Store, w, r)
	})
	mux.HandleFunc("/v1/llm/dispatches/stream", func(w http.ResponseWriter, r *http.Request) {
		handleLLMDispatchStream(deps.LLM, deps.EventBus, deps.Store, w, r)
	})
	mux.HandleFunc("/v1/llm/dispatches", func(w http.ResponseWriter, r *http.Request) {
		handleLLMDispatches(deps.LLM, deps.EventBus, deps.Store, w, r)
	})
	mux.HandleFunc("/v1/llm/dispatches/", func(w http.ResponseWriter, r *http.Request) {
		handleLLMDispatchRoutes(deps.Store, w, r)
	})
	mux.HandleFunc("/v1/connectors", func(w http.ResponseWriter, r *http.Request) {
		handleConnectors(deps.Connectors, deps.EventBus, deps.Store, w, r)
	})
	mux.HandleFunc("/v1/connectors/", func(w http.ResponseWriter, r *http.Request) {
		handleConnectorRoutes(deps.Connectors, deps.EventBus, deps.Store, w, r)
	})
	mux.HandleFunc("/v1/capabilities", func(w http.ResponseWriter, r *http.Request) {
		handleCapabilities(deps.Capabilities, deps.EventBus, deps.Store, w, r)
	})
	mux.HandleFunc("/v1/capabilities/", func(w http.ResponseWriter, r *http.Request) {
		handleCapabilityRoutes(deps.Capabilities, deps.EventBus, deps.Store, w, r)
	})

	return &Server{
		cfg:          deps.Config,
		logger:       deps.Logger,
		eventBus:     deps.EventBus,
		policy:       deps.Policy,
		router:       deps.Router,
		runtime:      deps.Runtime,
		llm:          deps.LLM,
		connectors:   deps.Connectors,
		capabilities: deps.Capabilities,
		store:        deps.Store,
		checkpoints:  deps.Checkpoints,
		server: &http.Server{
			Addr:              deps.Config.BindAddr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	s.Start(errCh)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) Start(errCh chan<- error) {
	go func() {
		s.logger.Info("http server listening", "addr", s.cfg.BindAddr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) Handler() http.Handler {
	return s.server.Handler
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error": message,
	})
}

func decodeJSONBody(r *http.Request, target any) error {
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return errors.New("request body is required")
	}

	return json.Unmarshal(body, target)
}

func handleRuns(sessionRouter *router.SessionRouter, manager *runtime.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, ListResponse[runtime.Run]{Items: manager.ListRuns()})
	case http.MethodPost:
		var input runtime.CreateRunInput
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		session, createdSession, err := resolveRunSession(sessionRouter, input)
		if err != nil {
			switch {
			case errors.Is(err, router.ErrSessionNotFound):
				http.NotFound(w, r)
			default:
				writeError(w, http.StatusBadRequest, err.Error())
			}
			return
		}
		input.SessionID = session.SessionID

		run, err := manager.CreateRun(input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := persistSession(r.Context(), sqliteStore, session); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := persistRun(r.Context(), sqliteStore, run); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := persistCheckpoint(r.Context(), checkpointManager, run.RunID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if createdSession {
			if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
				Category: "session",
				Name:     "session.created",
				Scope: events.Scope{
					SessionID: session.SessionID,
				},
				Resource: events.Resource{
					Kind: "session",
					ID:   session.SessionID,
				},
				Payload: map[string]any{
					"kind":       session.Kind,
					"channel":    session.Channel,
					"routingKey": session.RoutingKey,
					"generation": session.Generation,
				},
			}); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
			Category: "session",
			Name:     "session.routed",
			Scope: events.Scope{
				SessionID: session.SessionID,
			},
			Resource: events.Resource{
				Kind: "session",
				ID:   session.SessionID,
			},
			Payload: map[string]any{
				"kind":       session.Kind,
				"channel":    session.Channel,
				"routingKey": session.RoutingKey,
				"generation": session.Generation,
			},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
			Category: "run",
			Name:     "run.created",
			Scope: events.Scope{
				SessionID: run.SessionID,
				RunID:     run.RunID,
			},
			Resource: events.Resource{
				Kind: "run",
				ID:   run.RunID,
			},
			Payload: map[string]any{
				"entrypoint": run.Entrypoint,
				"goal":       run.Goal,
				"status":     run.Status,
			},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, run)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleRunRoutes(manager *runtime.Manager, capabilitySupervisor *capabilities.Supervisor, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/runs/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(path, "/")
	if len(parts) == 1 {
		handleRunByID(manager, w, r, parts[0])
		return
	}

	if len(parts) == 2 && parts[1] == "cancel" {
		handleRunCancel(manager, eventBus, sqliteStore, checkpointManager, w, r, parts[0])
		return
	}

	if len(parts) == 2 && parts[1] == "resume" {
		handleRunResume(manager, eventBus, sqliteStore, checkpointManager, w, r, parts[0])
		return
	}

	if len(parts) == 2 && parts[1] == "events" {
		handleRunEvents(eventBus, sqliteStore, w, r, parts[0])
		return
	}

	if len(parts) == 2 && parts[1] == "steps" {
		handleRunSteps(manager, eventBus, sqliteStore, checkpointManager, w, r, parts[0])
		return
	}

	if len(parts) == 3 && parts[1] == "steps" {
		handleRunStepByID(manager, w, r, parts[0], parts[2])
		return
	}

	if len(parts) == 4 && parts[1] == "steps" && parts[3] == "status" {
		handleRunStepStatus(manager, eventBus, sqliteStore, checkpointManager, w, r, parts[0], parts[2])
		return
	}

	if len(parts) == 4 && parts[1] == "steps" && parts[3] == "cancel" {
		handleRunStepCancel(manager, eventBus, sqliteStore, checkpointManager, w, r, parts[0], parts[2])
		return
	}

	if len(parts) == 4 && parts[1] == "steps" && parts[3] == "tool-calls" {
		handleRunStepToolCalls(manager, capabilitySupervisor, eventBus, sqliteStore, checkpointManager, w, r, parts[0], parts[2])
		return
	}

	if len(parts) == 5 && parts[1] == "steps" && parts[3] == "tool-calls" {
		handleRunStepToolCallByID(manager, w, r, parts[0], parts[2], parts[4])
		return
	}

	if len(parts) == 6 && parts[1] == "steps" && parts[3] == "tool-calls" && parts[5] == "complete" {
		handleRunStepToolCallComplete(manager, eventBus, sqliteStore, checkpointManager, w, r, parts[0], parts[2], parts[4])
		return
	}

	if len(parts) == 6 && parts[1] == "steps" && parts[3] == "tool-calls" && parts[5] == "fail" {
		handleRunStepToolCallFail(manager, eventBus, sqliteStore, checkpointManager, w, r, parts[0], parts[2], parts[4])
		return
	}

	http.NotFound(w, r)
}

func handleRunByID(manager *runtime.Manager, w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if runID == "" {
		http.NotFound(w, r)
		return
	}

	run, ok := manager.GetRun(runID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, http.StatusOK, run)
}

func handleRunCancel(manager *runtime.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	before, rollbackEnabled, err := snapshotForRollback(manager, runID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	run, cancelledSteps, idempotent, err := manager.CancelRun(runID)
	if err != nil {
		switch {
		case errors.Is(err, runtime.ErrRunNotFound):
			http.NotFound(w, r)
		case errors.Is(err, runtime.ErrRunTerminal):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	if err := persistRunCommandMutation(r.Context(), sqliteStore, checkpointManager, run, cancelledSteps); err != nil {
		rollbackRunMutation(r.Context(), checkpointManager, before, rollbackEnabled)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	published, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "run",
		Name:     "run.cancelled",
		Scope: events.Scope{
			SessionID: run.SessionID,
			RunID:     run.RunID,
		},
		Resource: events.Resource{
			Kind: "run",
			ID:   run.RunID,
		},
		Payload: map[string]any{
			"status":           run.Status,
			"idempotent":       idempotent,
			"cancelledStepIds": stepIDs(cancelledSteps),
		},
	})
	if err != nil {
		rollbackRunMutation(r.Context(), checkpointManager, before, rollbackEnabled)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = published
	writeJSON(w, http.StatusOK, run)
}

func handleRunResume(manager *runtime.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	before, rollbackEnabled, err := snapshotForRollback(manager, runID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	run, resumedSteps, idempotent, err := manager.ResumeRun(runID)
	if err != nil {
		switch {
		case errors.Is(err, runtime.ErrRunNotFound):
			http.NotFound(w, r)
		case errors.Is(err, runtime.ErrRunTerminal):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	if err := persistRunCommandMutation(r.Context(), sqliteStore, checkpointManager, run, resumedSteps); err != nil {
		rollbackRunMutation(r.Context(), checkpointManager, before, rollbackEnabled)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "run",
		Name:     "run.resumed",
		Scope: events.Scope{
			SessionID: run.SessionID,
			RunID:     run.RunID,
		},
		Resource: events.Resource{
			Kind: "run",
			ID:   run.RunID,
		},
		Payload: map[string]any{
			"status":         run.Status,
			"idempotent":     idempotent,
			"resumedStepIds": stepIDs(resumedSteps),
		},
	}); err != nil {
		rollbackRunMutation(r.Context(), checkpointManager, before, rollbackEnabled)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, run)
}

func handleSessions(sessionRouter *router.SessionRouter, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, ListResponse[router.Session]{Items: sessionRouter.ListSessions()})
}

func handleSessionRoutes(sessionRouter *router.SessionRouter, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/sessions/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(path, "/")
	if len(parts) == 1 {
		handleSessionByID(sessionRouter, w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "reset" {
		handleSessionReset(sessionRouter, eventBus, sqliteStore, w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "events" {
		handleSessionEvents(eventBus, sqliteStore, w, r, parts[0])
		return
	}

	http.NotFound(w, r)
}

func handleSessionByID(sessionRouter *router.SessionRouter, w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	session, ok := sessionRouter.GetSession(sessionID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, http.StatusOK, session)
}

func handleSessionReset(sessionRouter *router.SessionRouter, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	session, err := sessionRouter.ResetSession(sessionID)
	if err != nil {
		if errors.Is(err, router.ErrSessionNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := persistSession(r.Context(), sqliteStore, session); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "session",
		Name:     "session.reset",
		Scope: events.Scope{
			SessionID: session.SessionID,
		},
		Resource: events.Resource{
			Kind: "session",
			ID:   session.SessionID,
		},
		Payload: map[string]any{
			"generation": session.Generation,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, session)
}

func handleSessionEvents(eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	cursor, err := parseEventCursor(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	items, err := listEvents(r.Context(), eventBus, sqliteStore, events.Filter{SessionID: sessionID, Cursor: cursor})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, buildEventListResponse(items))
}

func handlePolicyApprovals(policyEngine *policy.Engine, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if policyEngine == nil {
		writeError(w, http.StatusInternalServerError, "policy engine is not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		var status policy.ApprovalStatus
		if raw := r.URL.Query().Get("status"); raw != "" {
			status = policy.ApprovalStatus(raw)
		}
		writeJSON(w, http.StatusOK, ListResponse[policy.Approval]{Items: policyEngine.ListApprovals(status)})
	case http.MethodPost:
		var input policy.RequestApprovalInput
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		approval, decision, err := policyEngine.RequestApproval(input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
			Category: "policy",
			Name:     "policy.approval_requested",
			Resource: events.Resource{
				Kind: "approval",
				ID:   approval.ApprovalID,
			},
			Payload: map[string]any{
				"action":       approval.Action,
				"resourceKind": approval.ResourceKind,
				"resourceId":   approval.ResourceID,
				"status":       approval.Status,
			},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
			Category: "policy",
			Name:     "policy.decision_recorded",
			Resource: events.Resource{
				Kind: "decision",
				ID:   decision.DecisionID,
			},
			Payload: map[string]any{
				"action":       decision.Action,
				"resourceKind": decision.ResourceKind,
				"resourceId":   decision.ResourceID,
				"outcome":      decision.Outcome,
				"approvalId":   decision.ApprovalID,
			},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"approval": approval,
			"decision": decision,
		})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handlePolicyApprovalRoutes(policyEngine *policy.Engine, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if policyEngine == nil {
		writeError(w, http.StatusInternalServerError, "policy engine is not configured")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/policy/approvals/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(path, "/")
	if len(parts) == 1 {
		handlePolicyApprovalByID(policyEngine, w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "resolve" {
		handlePolicyApprovalResolve(policyEngine, eventBus, sqliteStore, w, r, parts[0])
		return
	}

	http.NotFound(w, r)
}

func handlePolicyApprovalByID(policyEngine *policy.Engine, w http.ResponseWriter, r *http.Request, approvalID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	approval, ok := policyEngine.GetApproval(approvalID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, http.StatusOK, approval)
}

func handlePolicyApprovalResolve(policyEngine *policy.Engine, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, approvalID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var input policy.ResolveApprovalInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	approval, decision, err := policyEngine.ResolveApproval(approvalID, input)
	if err != nil {
		switch {
		case errors.Is(err, policy.ErrApprovalNotFound):
			http.NotFound(w, r)
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "policy",
		Name:     "policy.approval_resolved",
		Resource: events.Resource{
			Kind: "approval",
			ID:   approval.ApprovalID,
		},
		Payload: map[string]any{
			"action":       approval.Action,
			"resourceKind": approval.ResourceKind,
			"resourceId":   approval.ResourceID,
			"status":       approval.Status,
			"resolution":   approval.Resolution,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "policy",
		Name:     "policy.decision_recorded",
		Resource: events.Resource{
			Kind: "decision",
			ID:   decision.DecisionID,
		},
		Payload: map[string]any{
			"action":       decision.Action,
			"resourceKind": decision.ResourceKind,
			"resourceId":   decision.ResourceID,
			"outcome":      decision.Outcome,
			"approvalId":   decision.ApprovalID,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"approval": approval,
		"decision": decision,
	})
}

func handleLLMDispatches(dispatcher *llm.Dispatcher, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if dispatcher == nil {
		writeError(w, http.StatusInternalServerError, "llm dispatcher is not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		items, err := listLLMDispatches(r.Context(), sqliteStore)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ListResponse[llm.Dispatch]{Items: items})
	case http.MethodPost:
		var input llm.CreateDispatchInput
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		dispatch, err := dispatcher.Prepare(input, false)
		if err != nil {
			writeError(w, llmPrepareStatusCode(err), err.Error())
			return
		}
		if err := persistLLMDispatch(r.Context(), sqliteStore, dispatch); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := publishLLMDispatchRequested(r.Context(), eventBus, sqliteStore, dispatch); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		finalDispatch, execErr := dispatcher.Dispatch(r.Context(), dispatch)
		if err := persistLLMDispatch(r.Context(), sqliteStore, finalDispatch); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := publishLLMDispatchTerminal(r.Context(), eventBus, sqliteStore, finalDispatch); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if execErr != nil {
			writeJSON(w, llmDispatchStatusCode(finalDispatch), finalDispatch)
			return
		}

		writeJSON(w, http.StatusCreated, finalDispatch)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleLLMDispatchRoutes(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/llm/dispatches/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	dispatch, ok, err := getLLMDispatch(r.Context(), sqliteStore, path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, http.StatusOK, dispatch)
}

func handleLLMDispatchStream(dispatcher *llm.Dispatcher, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if dispatcher == nil {
		writeError(w, http.StatusInternalServerError, "llm dispatcher is not configured")
		return
	}

	var input llm.CreateDispatchInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dispatch, err := dispatcher.Prepare(input, true)
	if err != nil {
		writeError(w, llmPrepareStatusCode(err), err.Error())
		return
	}
	if err := persistLLMDispatch(r.Context(), sqliteStore, dispatch); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishLLMDispatchRequested(r.Context(), eventBus, sqliteStore, dispatch); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	writeSSEEvent(w, "llm.dispatch.started", dispatch.DispatchID, dispatch)
	flusher.Flush()

	finalDispatch, execErr := dispatcher.DispatchStream(r.Context(), dispatch, func(chunk llm.StreamChunk) error {
		writeSSEEvent(w, "llm.dispatch.delta", "", chunk)
		flusher.Flush()
		return nil
	})

	if err := persistLLMDispatch(context.Background(), sqliteStore, finalDispatch); err != nil {
		return
	}
	if _, err := publishLLMDispatchTerminal(context.Background(), eventBus, sqliteStore, finalDispatch); err != nil {
		return
	}

	if execErr == nil || finalDispatch.Status != llm.DispatchStatusCancelled {
		writeSSEEvent(w, llmDispatchTerminalEventName(finalDispatch), dispatch.DispatchID, finalDispatch)
		flusher.Flush()
	}
}

func handleConnectors(supervisor *connectors.Supervisor, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if supervisor == nil {
		writeError(w, http.StatusInternalServerError, "connector supervisor is not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, ListResponse[connectors.Connector]{Items: supervisor.List()})
	case http.MethodPost:
		var input connectors.RegisterInput
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		connector, created, err := supervisor.Register(input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := persistConnector(r.Context(), sqliteStore, connector); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
			Category: "connector",
			Name:     "connector.registered",
			Scope:    events.Scope{ConnectorID: connector.ConnectorID},
			Resource: events.Resource{Kind: "connector", ID: connector.ConnectorID},
			Payload: map[string]any{
				"kind":        connector.Kind,
				"status":      connector.Status,
				"created":     created,
				"displayName": connector.DisplayName,
			},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		writeJSON(w, status, connector)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleConnectorRoutes(supervisor *connectors.Supervisor, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/connectors/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(path, "/")
	if len(parts) == 1 {
		handleConnectorByID(supervisor, w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "health" {
		handleConnectorHealth(supervisor, eventBus, sqliteStore, w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "fail" {
		handleConnectorFail(supervisor, eventBus, sqliteStore, w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "restart" {
		handleConnectorRestart(supervisor, eventBus, sqliteStore, w, r, parts[0])
		return
	}

	http.NotFound(w, r)
}

func handleConnectorByID(supervisor *connectors.Supervisor, w http.ResponseWriter, r *http.Request, connectorID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	connector, ok := supervisor.Get(connectorID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, connector)
}

func handleConnectorHealth(supervisor *connectors.Supervisor, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, connectorID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var input connectors.ReportHealthInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	connector, err := supervisor.ReportHealth(connectorID, input)
	if err != nil {
		switch {
		case errors.Is(err, connectors.ErrConnectorNotFound):
			http.NotFound(w, r)
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	if err := persistConnector(r.Context(), sqliteStore, connector); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "connector",
		Name:     "connector.health_changed",
		Scope:    events.Scope{ConnectorID: connector.ConnectorID},
		Resource: events.Resource{Kind: "connector", ID: connector.ConnectorID},
		Payload: map[string]any{
			"status": connector.Status,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, connector)
}

func handleConnectorFail(supervisor *connectors.Supervisor, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, connectorID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var input connectors.ReportFailureInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	connector, err := supervisor.ReportFailure(connectorID, input)
	if err != nil {
		switch {
		case errors.Is(err, connectors.ErrConnectorNotFound):
			http.NotFound(w, r)
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	if err := persistConnector(r.Context(), sqliteStore, connector); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "connector",
		Name:     "connector.failure_reported",
		Scope:    events.Scope{ConnectorID: connector.ConnectorID},
		Resource: events.Resource{Kind: "connector", ID: connector.ConnectorID},
		Payload: map[string]any{
			"status":         connector.Status,
			"failureCount":   connector.FailureCount,
			"backoffSeconds": connector.BackoffSeconds,
			"reason":         connector.LastFailureReason,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, connector)
}

func handleConnectorRestart(supervisor *connectors.Supervisor, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, connectorID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	connector, err := supervisor.Restart(connectorID)
	if err != nil {
		if errors.Is(err, connectors.ErrConnectorNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := persistConnector(r.Context(), sqliteStore, connector); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "connector",
		Name:     "connector.restart_scheduled",
		Scope:    events.Scope{ConnectorID: connector.ConnectorID},
		Resource: events.Resource{Kind: "connector", ID: connector.ConnectorID},
		Payload: map[string]any{
			"status":       connector.Status,
			"restartCount": connector.RestartCount,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, connector)
}

func handleCapabilities(supervisor *capabilities.Supervisor, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if supervisor == nil {
		writeError(w, http.StatusInternalServerError, "capability supervisor is not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, ListResponse[capabilities.Capability]{Items: supervisor.List()})
	case http.MethodPost:
		var input capabilities.RegisterInput
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		capability, created, err := supervisor.Register(input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := persistCapability(r.Context(), sqliteStore, capability); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
			Category: "capability",
			Name:     "capability.registered",
			Scope:    events.Scope{CapabilityID: capability.CapabilityID},
			Resource: events.Resource{Kind: "capability", ID: capability.CapabilityID},
			Payload: map[string]any{
				"kind":        capability.Kind,
				"status":      capability.Status,
				"created":     created,
				"displayName": capability.DisplayName,
			},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		writeJSON(w, status, capability)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleCapabilityRoutes(supervisor *capabilities.Supervisor, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/capabilities/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(path, "/")
	if len(parts) == 1 {
		handleCapabilityByID(supervisor, w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "health" {
		handleCapabilityHealth(supervisor, eventBus, sqliteStore, w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "fail" {
		handleCapabilityFail(supervisor, eventBus, sqliteStore, w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "restart" {
		handleCapabilityRestart(supervisor, eventBus, sqliteStore, w, r, parts[0])
		return
	}

	http.NotFound(w, r)
}

func handleCapabilityByID(supervisor *capabilities.Supervisor, w http.ResponseWriter, r *http.Request, capabilityID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	capability, ok := supervisor.Get(capabilityID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, capability)
}

func handleCapabilityHealth(supervisor *capabilities.Supervisor, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, capabilityID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var input capabilities.ReportHealthInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	capability, err := supervisor.ReportHealth(capabilityID, input)
	if err != nil {
		switch {
		case errors.Is(err, capabilities.ErrCapabilityNotFound):
			http.NotFound(w, r)
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	if err := persistCapability(r.Context(), sqliteStore, capability); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "capability",
		Name:     "capability.health_changed",
		Scope:    events.Scope{CapabilityID: capability.CapabilityID},
		Resource: events.Resource{Kind: "capability", ID: capability.CapabilityID},
		Payload: map[string]any{
			"status": capability.Status,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, capability)
}

func handleCapabilityFail(supervisor *capabilities.Supervisor, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, capabilityID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var input capabilities.ReportFailureInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	capability, err := supervisor.ReportFailure(capabilityID, input)
	if err != nil {
		switch {
		case errors.Is(err, capabilities.ErrCapabilityNotFound):
			http.NotFound(w, r)
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	if err := persistCapability(r.Context(), sqliteStore, capability); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "capability",
		Name:     "capability.failure_reported",
		Scope:    events.Scope{CapabilityID: capability.CapabilityID},
		Resource: events.Resource{Kind: "capability", ID: capability.CapabilityID},
		Payload: map[string]any{
			"status":         capability.Status,
			"failureCount":   capability.FailureCount,
			"backoffSeconds": capability.BackoffSeconds,
			"reason":         capability.LastFailureReason,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, capability)
}

func handleCapabilityRestart(supervisor *capabilities.Supervisor, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, capabilityID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	capability, err := supervisor.Restart(capabilityID)
	if err != nil {
		if errors.Is(err, capabilities.ErrCapabilityNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := persistCapability(r.Context(), sqliteStore, capability); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "capability",
		Name:     "capability.restart_scheduled",
		Scope:    events.Scope{CapabilityID: capability.CapabilityID},
		Resource: events.Resource{Kind: "capability", ID: capability.CapabilityID},
		Payload: map[string]any{
			"status":       capability.Status,
			"restartCount": capability.RestartCount,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, capability)
}

func handleRunEvents(eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	cursor, err := parseEventCursor(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	items, err := listEvents(r.Context(), eventBus, sqliteStore, events.Filter{RunID: runID, Cursor: cursor})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, buildEventListResponse(items))
}

func handleRunSteps(manager *runtime.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request, runID string) {
	switch r.Method {
	case http.MethodGet:
		steps, err := manager.ListSteps(runID)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, ListResponse[runtime.Step]{Items: steps})
	case http.MethodPost:
		var input runtime.CreateStepInput
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		step, err := manager.CreateStep(runID, input)
		if err != nil {
			if errors.Is(err, runtime.ErrRunNotFound) {
				http.NotFound(w, r)
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := persistStep(r.Context(), sqliteStore, step); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := persistCheckpoint(r.Context(), checkpointManager, runID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
			Category: "step",
			Name:     "step.created",
			Scope: events.Scope{
				RunID:  runID,
				StepID: step.StepID,
			},
			Resource: events.Resource{
				Kind: "step",
				ID:   step.StepID,
			},
			Payload: map[string]any{
				"title":  step.Title,
				"kind":   step.Kind,
				"status": step.Status,
			},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, step)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleRunStepByID(manager *runtime.Manager, w http.ResponseWriter, r *http.Request, runID, stepID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	step, ok := manager.GetStep(runID, stepID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, http.StatusOK, step)
}

func handleRunStepCancel(manager *runtime.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request, runID, stepID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	before, rollbackEnabled, err := snapshotForRollback(manager, runID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	step, runUpdate, idempotent, err := manager.CancelStep(runID, stepID)
	if err != nil {
		switch {
		case errors.Is(err, runtime.ErrRunNotFound), errors.Is(err, runtime.ErrStepNotFound):
			http.NotFound(w, r)
		case errors.Is(err, runtime.ErrRunTerminal), errors.Is(err, runtime.ErrStepTerminal):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	if err := persistStepCancelMutation(r.Context(), sqliteStore, checkpointManager, step, runUpdate); err != nil {
		rollbackRunMutation(r.Context(), checkpointManager, before, rollbackEnabled)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	payload := map[string]any{
		"status":     step.Status,
		"idempotent": idempotent,
	}
	if runUpdate != nil {
		payload["runStatus"] = runUpdate.Status
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "step",
		Name:     "step.cancelled",
		Scope: events.Scope{
			RunID:  runID,
			StepID: step.StepID,
		},
		Resource: events.Resource{
			Kind: "step",
			ID:   step.StepID,
		},
		Payload: payload,
	}); err != nil {
		rollbackRunMutation(r.Context(), checkpointManager, before, rollbackEnabled)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, step)
}

func handleRunStepStatus(manager *runtime.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request, runID, stepID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var input runtime.UpdateStepStatusInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	step, runUpdate, err := manager.UpdateStepStatusAndReconcileRun(runID, stepID, input)
	if err != nil {
		switch {
		case errors.Is(err, runtime.ErrRunNotFound), errors.Is(err, runtime.ErrStepNotFound):
			http.NotFound(w, r)
		case errors.Is(err, runtime.ErrInvalidStepTransition):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	if err := persistStep(r.Context(), sqliteStore, step); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if runUpdate != nil {
		if err := persistRun(r.Context(), sqliteStore, *runUpdate); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if err := persistCheckpoint(r.Context(), checkpointManager, runID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "step",
		Name:     "step.status_changed",
		Scope: events.Scope{
			RunID:  runID,
			StepID: step.StepID,
		},
		Resource: events.Resource{
			Kind: "step",
			ID:   step.StepID,
		},
		Payload: map[string]any{
			"status": step.Status,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if runUpdate != nil {
		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
			Category: "run",
			Name:     "run.status_changed",
			Scope: events.Scope{
				RunID:     runID,
				SessionID: runUpdate.SessionID,
			},
			Resource: events.Resource{
				Kind: "run",
				ID:   runID,
			},
			Payload: map[string]any{
				"status": runUpdate.Status,
			},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, step)
}

func handleRunStepToolCalls(manager *runtime.Manager, capabilitySupervisor *capabilities.Supervisor, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request, runID, stepID string) {
	switch r.Method {
	case http.MethodGet:
		toolCalls, err := manager.ListToolCalls(runID, stepID)
		if err != nil {
			switch {
			case errors.Is(err, runtime.ErrRunNotFound), errors.Is(err, runtime.ErrStepNotFound):
				http.NotFound(w, r)
			default:
				writeError(w, http.StatusBadRequest, err.Error())
			}
			return
		}
		writeJSON(w, http.StatusOK, ListResponse[runtime.ToolCall]{Items: toolCalls})
	case http.MethodPost:
		var input runtime.CreateToolCallInput
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if capabilitySupervisor == nil {
			writeError(w, http.StatusInternalServerError, "capability supervisor is not configured")
			return
		}
		if _, ok := capabilitySupervisor.Get(input.CapabilityID); !ok {
			http.NotFound(w, r)
			return
		}
		toolCall, err := manager.CreateToolCall(runID, stepID, input)
		if err != nil {
			switch {
			case errors.Is(err, runtime.ErrRunNotFound), errors.Is(err, runtime.ErrStepNotFound):
				http.NotFound(w, r)
			default:
				writeError(w, http.StatusBadRequest, err.Error())
			}
			return
		}
		if err := persistToolCall(r.Context(), sqliteStore, manager, toolCall); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := persistCheckpoint(r.Context(), checkpointManager, runID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
			Category: "tool_call",
			Name:     "tool_call.requested",
			Scope: events.Scope{
				RunID:  runID,
				StepID: stepID,
			},
			Resource: events.Resource{
				Kind: "tool_call",
				ID:   toolCall.ToolCallID,
			},
			Payload: map[string]any{
				"capabilityId": toolCall.CapabilityID,
				"toolName":     toolCall.ToolName,
				"status":       toolCall.Status,
			},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, toolCall)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleRunStepToolCallByID(manager *runtime.Manager, w http.ResponseWriter, r *http.Request, runID, stepID, toolCallID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	toolCall, ok := manager.GetToolCall(runID, stepID, toolCallID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, http.StatusOK, toolCall)
}

func handleRunStepToolCallComplete(manager *runtime.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request, runID, stepID, toolCallID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var input runtime.CompleteToolCallInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	toolCall, err := manager.CompleteToolCall(runID, stepID, toolCallID, input)
	if err != nil {
		switch {
		case errors.Is(err, runtime.ErrRunNotFound), errors.Is(err, runtime.ErrStepNotFound), errors.Is(err, runtime.ErrToolCallNotFound):
			http.NotFound(w, r)
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	if err := persistToolCall(r.Context(), sqliteStore, manager, toolCall); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := persistCheckpoint(r.Context(), checkpointManager, runID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "tool_call",
		Name:     "tool_call.completed",
		Scope: events.Scope{
			RunID:  runID,
			StepID: stepID,
		},
		Resource: events.Resource{
			Kind: "tool_call",
			ID:   toolCall.ToolCallID,
		},
		Payload: map[string]any{
			"capabilityId": toolCall.CapabilityID,
			"toolName":     toolCall.ToolName,
			"status":       toolCall.Status,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toolCall)
}

func handleRunStepToolCallFail(manager *runtime.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request, runID, stepID, toolCallID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var input runtime.FailToolCallInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	toolCall, err := manager.FailToolCall(runID, stepID, toolCallID, input)
	if err != nil {
		switch {
		case errors.Is(err, runtime.ErrRunNotFound), errors.Is(err, runtime.ErrStepNotFound), errors.Is(err, runtime.ErrToolCallNotFound):
			http.NotFound(w, r)
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	if err := persistToolCall(r.Context(), sqliteStore, manager, toolCall); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := persistCheckpoint(r.Context(), checkpointManager, runID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "tool_call",
		Name:     "tool_call.failed",
		Scope: events.Scope{
			RunID:  runID,
			StepID: stepID,
		},
		Resource: events.Resource{
			Kind: "tool_call",
			ID:   toolCall.ToolCallID,
		},
		Payload: map[string]any{
			"capabilityId": toolCall.CapabilityID,
			"toolName":     toolCall.ToolName,
			"status":       toolCall.Status,
			"error":        toolCall.Error,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toolCall)
}

func persistRun(ctx context.Context, sqliteStore *store.SQLiteStore, run runtime.Run) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertRun(ctx, run)
}

func persistSession(ctx context.Context, sqliteStore *store.SQLiteStore, session router.Session) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertSession(ctx, session)
}

func persistStep(ctx context.Context, sqliteStore *store.SQLiteStore, step runtime.Step) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertStep(ctx, step)
}

func persistConnector(ctx context.Context, sqliteStore *store.SQLiteStore, connector connectors.Connector) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertConnector(ctx, connector)
}

func persistCapability(ctx context.Context, sqliteStore *store.SQLiteStore, capability capabilities.Capability) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertCapability(ctx, capability)
}

func persistLLMDispatch(ctx context.Context, sqliteStore *store.SQLiteStore, dispatch llm.Dispatch) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertLLMDispatch(ctx, dispatch)
}

func persistToolCall(ctx context.Context, sqliteStore *store.SQLiteStore, manager *runtime.Manager, toolCall runtime.ToolCall) error {
	if sqliteStore == nil {
		return nil
	}
	if manager != nil {
		run, ok := manager.GetRun(toolCall.RunID)
		if !ok {
			return runtime.ErrRunNotFound
		}
		if err := persistRun(ctx, sqliteStore, run); err != nil {
			return err
		}
		step, ok := manager.GetStep(toolCall.RunID, toolCall.StepID)
		if !ok {
			return runtime.ErrStepNotFound
		}
		if err := persistStep(ctx, sqliteStore, step); err != nil {
			return err
		}
	}
	return sqliteStore.UpsertToolCall(ctx, toolCall)
}

func persistRunCommandMutation(ctx context.Context, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, run runtime.Run, steps []runtime.Step) error {
	if err := persistRun(ctx, sqliteStore, run); err != nil {
		return err
	}
	for _, step := range steps {
		if err := persistStep(ctx, sqliteStore, step); err != nil {
			return err
		}
	}
	return persistCheckpoint(ctx, checkpointManager, run.RunID)
}

func persistStepCancelMutation(ctx context.Context, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, step runtime.Step, runUpdate *runtime.Run) error {
	if err := persistStep(ctx, sqliteStore, step); err != nil {
		return err
	}
	runID := step.RunID
	if runUpdate != nil {
		runID = runUpdate.RunID
		if err := persistRun(ctx, sqliteStore, *runUpdate); err != nil {
			return err
		}
	}
	return persistCheckpoint(ctx, checkpointManager, runID)
}

func persistCheckpoint(ctx context.Context, checkpointManager *checkpoints.Manager, runID string) error {
	if checkpointManager == nil {
		return nil
	}
	return checkpointManager.SaveRunCheckpoint(ctx, runID)
}

func publishEvent(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, event events.Event) (events.Event, error) {
	prepared := ensureEventDefaults(event)

	if sqliteStore != nil {
		persisted, err := sqliteStore.AppendEvent(ctx, prepared)
		if err != nil {
			return events.Event{}, err
		}
		prepared = persisted
	}

	return eventBus.Publish(prepared), nil
}

func listLLMDispatches(ctx context.Context, sqliteStore *store.SQLiteStore) ([]llm.Dispatch, error) {
	if sqliteStore == nil {
		return []llm.Dispatch{}, nil
	}
	return sqliteStore.ListLLMDispatches(ctx)
}

func getLLMDispatch(ctx context.Context, sqliteStore *store.SQLiteStore, dispatchID string) (llm.Dispatch, bool, error) {
	if sqliteStore == nil {
		return llm.Dispatch{}, false, nil
	}
	return sqliteStore.GetLLMDispatch(ctx, dispatchID)
}

func llmPrepareStatusCode(err error) int {
	switch {
	case errors.Is(err, llm.ErrProviderRequired), errors.Is(err, llm.ErrProviderNotFound), errors.Is(err, llm.ErrModelRequired), errors.Is(err, llm.ErrMessagesRequired):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func llmDispatchStatusCode(dispatch llm.Dispatch) int {
	switch dispatch.ErrorCode {
	case "timeout":
		return http.StatusGatewayTimeout
	case "provider_not_found":
		return http.StatusBadRequest
	case "cancelled":
		return http.StatusRequestTimeout
	default:
		return http.StatusBadGateway
	}
}

func llmDispatchTerminalEventName(dispatch llm.Dispatch) string {
	switch dispatch.Status {
	case llm.DispatchStatusFailed:
		return "llm.dispatch.failed"
	case llm.DispatchStatusCancelled:
		return "llm.dispatch.cancelled"
	default:
		return "llm.dispatch.completed"
	}
}

func publishLLMDispatchRequested(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, dispatch llm.Dispatch) (events.Event, error) {
	return publishEvent(ctx, eventBus, sqliteStore, events.Event{
		Category: "llm",
		Name:     "llm.dispatch.requested",
		Resource: events.Resource{Kind: "llm_dispatch", ID: dispatch.DispatchID},
		Payload: map[string]any{
			"provider":   dispatch.Provider,
			"model":      dispatch.Model,
			"stream":     dispatch.Stream,
			"timeoutMs":  dispatch.TimeoutMs,
			"maxRetries": dispatch.MaxRetries,
			"status":     dispatch.Status,
		},
	})
}

func publishLLMDispatchTerminal(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, dispatch llm.Dispatch) (events.Event, error) {
	return publishEvent(ctx, eventBus, sqliteStore, events.Event{
		Category: "llm",
		Name:     llmDispatchTerminalEventName(dispatch),
		Resource: events.Resource{Kind: "llm_dispatch", ID: dispatch.DispatchID},
		Payload: map[string]any{
			"provider":     dispatch.Provider,
			"model":        dispatch.Model,
			"status":       dispatch.Status,
			"attemptCount": dispatch.AttemptCount,
			"finishReason": dispatch.FinishReason,
			"usage":        dispatch.Usage,
			"errorCode":    dispatch.ErrorCode,
			"error":        dispatch.Error,
		},
	})
}

func writeSSEEvent(w io.Writer, eventName, eventID string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	if eventID != "" {
		_, _ = fmt.Fprintf(w, "id: %s\n", eventID)
	}
	if eventName != "" {
		_, _ = fmt.Fprintf(w, "event: %s\n", eventName)
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", string(data))
}

func resolveRunSession(sessionRouter *router.SessionRouter, input runtime.CreateRunInput) (router.Session, bool, error) {
	if sessionRouter == nil {
		return router.Session{}, false, errors.New("session router is required")
	}

	if input.SessionID != "" {
		session, ok := sessionRouter.GetSession(input.SessionID)
		if !ok {
			return router.Session{}, false, router.ErrSessionNotFound
		}
		session, err := sessionRouter.TouchSession(input.SessionID)
		if err != nil {
			return router.Session{}, false, err
		}
		return session, false, nil
	}

	channel := "local"
	peerID := input.Entrypoint
	if peerID == "" {
		peerID = "chat"
	}

	session, created, err := sessionRouter.Route(router.RouteInput{
		Kind:      router.SessionKindDirect,
		Channel:   channel,
		AccountID: "local",
		PeerID:    peerID,
	})
	if err != nil {
		return router.Session{}, false, err
	}

	return session, created, nil
}

func ensureEventDefaults(event events.Event) events.Event {
	if event.EventID == "" {
		event.EventID = newEventID()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if event.Payload == nil {
		event.Payload = map[string]any{}
	}
	return event
}

func listEvents(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, filter events.Filter) ([]events.Event, error) {
	if sqliteStore != nil {
		return sqliteStore.ListEvents(ctx, filter)
	}
	return eventBus.List(filter), nil
}

func parseEventCursor(r *http.Request) (int64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("cursor"))
	if raw == "" {
		raw = strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	}
	if raw == "" {
		return 0, nil
	}

	cursor, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || cursor < 0 {
		return 0, errors.New("cursor must be a non-negative integer")
	}
	return cursor, nil
}

func snapshotForRollback(manager *runtime.Manager, runID string) (runtime.RunCheckpoint, bool, error) {
	if manager == nil {
		return runtime.RunCheckpoint{}, false, nil
	}

	checkpoint, err := manager.SnapshotRun(runID)
	if err != nil {
		return runtime.RunCheckpoint{}, false, err
	}
	return checkpoint, true, nil
}

func rollbackRunMutation(ctx context.Context, checkpointManager *checkpoints.Manager, checkpoint runtime.RunCheckpoint, enabled bool) {
	if !enabled || checkpointManager == nil {
		return
	}
	_ = checkpointManager.RestoreRunCheckpoint(ctx, checkpoint)
}

func stepIDs(steps []runtime.Step) []string {
	ids := make([]string, 0, len(steps))
	for _, step := range steps {
		ids = append(ids, step.StepID)
	}
	return ids
}

func newEventID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "evt_fallback"
	}

	return "evt_" + hex.EncodeToString(buf)
}

func streamEvents(eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	filter := events.Filter{
		Category:  r.URL.Query().Get("category"),
		RunID:     r.URL.Query().Get("runId"),
		SessionID: r.URL.Query().Get("sessionId"),
	}
	if resourceKind := r.URL.Query().Get("resourceKind"); resourceKind != "" {
		filter.ResourceKind = resourceKind
	}
	cursor, err := parseEventCursor(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filter.Cursor = cursor

	history, err := listEvents(r.Context(), eventBus, sqliteStore, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	_, _ = fmt.Fprint(w, ": stream-open\n\n")
	flusher.Flush()

	for _, event := range history {
		writeRuntimeSSEEvent(w, flusher, event)
	}

	ch, unsubscribe := eventBus.Subscribe(filter)
	defer unsubscribe()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			writeRuntimeSSEEvent(w, flusher, event)
		case <-ticker.C:
			_, _ = fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

func writeRuntimeSSEEvent(w http.ResponseWriter, flusher http.Flusher, event events.Event) {
	payload, _ := json.Marshal(event)
	if event.Sequence > 0 {
		_, _ = fmt.Fprintf(w, "id: %d\n", event.Sequence)
	}
	_, _ = fmt.Fprintf(w, "event: %s\n", event.Name)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
	flusher.Flush()
}
