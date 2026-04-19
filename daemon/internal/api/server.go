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

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/chat"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/skills"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type Dependencies struct {
	Config       config.Config
	Logger       *slog.Logger
	EventBus     *events.Bus
	Policy       *policy.Engine
	Auth         *auth.Manager
	Router       *router.SessionRouter
	Runtime      *runtime.Manager
	LLM          *llm.Dispatcher
	Chat         *chat.Service
	Providers    *providers.Manager
	Skills       *skills.Registry
	Sandboxes    *sandbox.Manager
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
	auth         *auth.Manager
	router       *router.SessionRouter
	runtime      *runtime.Manager
	llm          *llm.Dispatcher
	chat         *chat.Service
	providers    *providers.Manager
	skills       *skills.Registry
	sandboxes    *sandbox.Manager
	connectors   *connectors.Supervisor
	capabilities *capabilities.Supervisor
	store        *store.SQLiteStore
	checkpoints  *checkpoints.Manager
	server       *http.Server
}

func NewServer(deps Dependencies) *Server {
	mux := http.NewServeMux()
	protected := func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			token, ok, err := authenticateRequest(deps.Auth, r)
			if err != nil {
				writeError(w, http.StatusUnauthorized, err.Error())
				return
			}
			if deps.Auth != nil && !ok {
				writeError(w, http.StatusUnauthorized, auth.ErrAuthRequired.Error())
				return
			}
			if ok {
				if err := persistAccessToken(r.Context(), deps.Store, token); err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				r = r.WithContext(withAuthenticatedToken(r.Context(), token))
			}
			handler(w, r)
		}
	}
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
	mux.HandleFunc("/v1/auth/pairings/start", func(w http.ResponseWriter, r *http.Request) {
		handleAuthPairingStart(deps.Auth, deps.EventBus, deps.Store, w, r)
	})
	mux.HandleFunc("/v1/auth/pairings/", func(w http.ResponseWriter, r *http.Request) {
		handleAuthPairingRoutes(deps.Auth, deps.EventBus, deps.Store, w, r)
	})
	mux.HandleFunc("/v1/auth/me", protected(func(w http.ResponseWriter, r *http.Request) {
		handleAuthMe(deps.Auth, deps.Store, w, r)
	}))
	mux.HandleFunc("/v1/config", protected(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, buildConfigResponse(deps.Config))
	}))
	mux.HandleFunc("/v1/events/stream", protected(func(w http.ResponseWriter, r *http.Request) {
		streamEvents(deps.EventBus, deps.Store, w, r)
	}))
	mux.HandleFunc("/v1/runs", protected(func(w http.ResponseWriter, r *http.Request) {
		handleRuns(deps.Router, deps.Runtime, deps.EventBus, deps.Store, deps.Checkpoints, w, r)
	}))
	mux.HandleFunc("/v1/runs/", protected(func(w http.ResponseWriter, r *http.Request) {
		handleRunRoutes(deps.Runtime, deps.Policy, deps.Capabilities, deps.EventBus, deps.Store, deps.Checkpoints, w, r)
	}))
	mux.HandleFunc("/v1/sessions", protected(func(w http.ResponseWriter, r *http.Request) {
		handleSessions(deps.Router, deps.EventBus, deps.Store, w, r)
	}))
	mux.HandleFunc("/v1/sessions/", protected(func(w http.ResponseWriter, r *http.Request) {
		handleSessionRoutes(deps.Router, deps.EventBus, deps.Store, w, r)
	}))
	mux.HandleFunc("/v1/policy/approvals", protected(func(w http.ResponseWriter, r *http.Request) {
		handlePolicyApprovals(deps.Policy, deps.EventBus, deps.Store, w, r)
	}))
	mux.HandleFunc("/v1/policy/approvals/", protected(func(w http.ResponseWriter, r *http.Request) {
		handlePolicyApprovalRoutes(deps.Policy, deps.EventBus, deps.Store, w, r)
	}))
	mux.HandleFunc("/v1/llm/dispatches/stream", protected(func(w http.ResponseWriter, r *http.Request) {
		handleLLMDispatchStream(deps.LLM, deps.Providers, deps.EventBus, deps.Store, w, r)
	}))
	mux.HandleFunc("/v1/llm/dispatches", protected(func(w http.ResponseWriter, r *http.Request) {
		handleLLMDispatches(deps.LLM, deps.Providers, deps.EventBus, deps.Store, w, r)
	}))
	mux.HandleFunc("/v1/llm/dispatches/", protected(func(w http.ResponseWriter, r *http.Request) {
		handleLLMDispatchRoutes(deps.Store, w, r)
	}))
	mux.HandleFunc("/v1/chat/query/stream", protected(func(w http.ResponseWriter, r *http.Request) {
		handleChatQueryStream(deps.Chat, w, r)
	}))
	mux.HandleFunc("/v1/chat/query", protected(func(w http.ResponseWriter, r *http.Request) {
		handleChatQuery(deps.Chat, w, r)
	}))
	mux.HandleFunc("/v1/skills", protected(func(w http.ResponseWriter, r *http.Request) {
		handleSkills(deps.Skills, w, r)
	}))
	mux.HandleFunc("/v1/skills/", protected(func(w http.ResponseWriter, r *http.Request) {
		handleSkillRoutes(deps.Skills, w, r)
	}))
	mux.HandleFunc("/v1/sandboxes/profiles", protected(func(w http.ResponseWriter, r *http.Request) {
		handleSandboxProfiles(deps.Sandboxes, w, r)
	}))
	mux.HandleFunc("/v1/sandboxes/profiles/", protected(func(w http.ResponseWriter, r *http.Request) {
		handleSandboxProfileRoutes(deps.Sandboxes, w, r)
	}))
	mux.HandleFunc("/v1/sandboxes/executions", protected(func(w http.ResponseWriter, r *http.Request) {
		handleSandboxExecutions(deps.Sandboxes, w, r)
	}))
	mux.HandleFunc("/v1/sandboxes/executions/", protected(func(w http.ResponseWriter, r *http.Request) {
		handleSandboxExecutionRoutes(deps.Sandboxes, w, r)
	}))
	mux.HandleFunc("/v1/sandboxes/explain", protected(func(w http.ResponseWriter, r *http.Request) {
		handleSandboxExplain(deps.Sandboxes, w, r)
	}))
	mux.HandleFunc("/v1/providers", protected(func(w http.ResponseWriter, r *http.Request) {
		handleProviders(deps.Providers, w, r)
	}))
	mux.HandleFunc("/v1/providers/", protected(func(w http.ResponseWriter, r *http.Request) {
		handleProviderRoutes(deps.Providers, deps.EventBus, deps.Store, w, r)
	}))
	mux.HandleFunc("/v1/connectors", protected(func(w http.ResponseWriter, r *http.Request) {
		handleConnectors(deps.Connectors, deps.EventBus, deps.Store, w, r)
	}))
	mux.HandleFunc("/v1/connectors/", protected(func(w http.ResponseWriter, r *http.Request) {
		handleConnectorRoutes(deps.Connectors, deps.Router, deps.Runtime, deps.EventBus, deps.Store, deps.Checkpoints, w, r)
	}))
	mux.HandleFunc("/v1/capabilities", protected(func(w http.ResponseWriter, r *http.Request) {
		handleCapabilities(deps.Capabilities, deps.EventBus, deps.Store, w, r)
	}))
	mux.HandleFunc("/v1/capabilities/", protected(func(w http.ResponseWriter, r *http.Request) {
		handleCapabilityRoutes(deps.Capabilities, deps.EventBus, deps.Store, w, r)
	}))

	return &Server{
		cfg:          deps.Config,
		logger:       deps.Logger,
		eventBus:     deps.EventBus,
		policy:       deps.Policy,
		auth:         deps.Auth,
		router:       deps.Router,
		runtime:      deps.Runtime,
		llm:          deps.LLM,
		chat:         deps.Chat,
		providers:    deps.Providers,
		skills:       deps.Skills,
		sandboxes:    deps.Sandboxes,
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
		var request CreateRunRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		session, createdSession, err := resolveRunSession(sessionRouter, request)
		if err != nil {
			switch {
			case errors.Is(err, router.ErrSessionNotFound):
				http.NotFound(w, r)
			default:
				writeError(w, http.StatusBadRequest, err.Error())
			}
			return
		}
		input := runtime.CreateRunInput{
			SessionID:  session.SessionID,
			Entrypoint: request.Entrypoint,
			Goal:       request.Goal,
		}

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

		if err := publishSessionRouteEvents(r.Context(), eventBus, sqliteStore, session, createdSession, map[string]any{
			"source": "run.create",
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

func handleRunRoutes(manager *runtime.Manager, policyEngine *policy.Engine, capabilitySupervisor *capabilities.Supervisor, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request) {
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
		handleRunStepToolCalls(manager, policyEngine, capabilitySupervisor, eventBus, sqliteStore, checkpointManager, w, r, parts[0], parts[2])
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
		if input.RequestedBy == "" {
			input.RequestedBy = currentActor(r.Context())
		}

		approval, decision, err := policyEngine.RequestApproval(input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := persistApproval(r.Context(), sqliteStore, approval); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := persistDecision(r.Context(), sqliteStore, decision); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
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
	if err := persistApproval(r.Context(), sqliteStore, approval); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := persistDecision(r.Context(), sqliteStore, decision); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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

func handleAuthPairingStart(authManager *auth.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if authManager == nil {
		writeError(w, http.StatusInternalServerError, "auth manager is not configured")
		return
	}

	var input auth.StartPairingInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	pairing, code, err := authManager.StartPairing(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := persistPairing(r.Context(), sqliteStore, pairing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "system",
		Name:     "auth.pairing_started",
		Resource: events.Resource{Kind: "pairing", ID: pairing.PairingID},
		Payload: map[string]any{
			"mode":      pairing.Mode,
			"status":    pairing.Status,
			"expiresAt": pairing.ExpiresAt,
			"label":     pairing.Label,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"pairing":     pairing,
		"pairingCode": code,
	})
}

func handleAuthPairingRoutes(authManager *auth.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if authManager == nil {
		writeError(w, http.StatusInternalServerError, "auth manager is not configured")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/auth/pairings/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) == 2 && parts[1] == "complete" {
		handleAuthPairingComplete(authManager, eventBus, sqliteStore, w, r, parts[0])
		return
	}
	http.NotFound(w, r)
}

func handleAuthPairingComplete(authManager *auth.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, pairingID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var input auth.CompletePairingInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	pairing, token, tokenSecret, err := authManager.CompletePairing(pairingID, input)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrPairingNotFound):
			http.NotFound(w, r)
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	if err := persistPairing(r.Context(), sqliteStore, pairing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := persistAccessToken(r.Context(), sqliteStore, token); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "system",
		Name:     "auth.pairing_completed",
		Resource: events.Resource{Kind: "pairing", ID: pairing.PairingID},
		Payload: map[string]any{
			"mode":    pairing.Mode,
			"status":  pairing.Status,
			"tokenId": token.TokenID,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"pairing":     pairing,
		"token":       token,
		"accessToken": tokenSecret,
	})
}

func handleAuthMe(authManager *auth.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	token, ok := authenticatedToken(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, auth.ErrAuthRequired.Error())
		return
	}
	if err := persistAccessToken(r.Context(), sqliteStore, token); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, token)
}

func handleProviders(manager *providers.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "provider manager is not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, ProviderListResponse{Items: manager.ListProfiles()})
}

func handleProviderRoutes(manager *providers.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "provider manager is not configured")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/providers/")
	parts := splitPath(path)
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}

	providerID := parts[0]
	profile, ok := manager.GetProfile(providerID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, profile)
		return
	}

	switch {
	case parts[1] == "auth" && len(parts) == 2 && r.Method == http.MethodGet:
		state, ok := manager.GetAuthState(providerID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, ProviderAuthStateResponse{Auth: state})
		return
	case parts[1] == "auth" && len(parts) == 3 && r.Method == http.MethodPost:
		if sqliteStore == nil {
			writeError(w, http.StatusInternalServerError, "store is not configured")
			return
		}
		var (
			state  providers.AuthState
			models []providers.Model
			err    error
			event  string
		)
		switch parts[2] {
		case "start":
			state, models, err = manager.StartManagedAuth(r.Context(), providerID)
			event = "provider.auth_started"
		case "complete":
			state, models, err = manager.CompleteManagedAuth(r.Context(), providerID)
			event = "provider.auth_completed"
		case "refresh":
			state, models, err = manager.RefreshManagedAuth(r.Context(), providerID)
			event = "provider.auth_refreshed"
		case "revoke":
			state, models, err = manager.RevokeManagedAuth(r.Context(), providerID)
			event = "provider.auth_revoked"
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			writeError(w, llmPrepareStatusCode(err), err.Error())
			return
		}
		if err := persistManagedProviderState(r.Context(), sqliteStore, state, models); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := publishProviderAuthEvent(r.Context(), eventBus, sqliteStore, state, event); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ProviderAuthStateResponse{Auth: state})
		return
	case parts[1] == "models" && len(parts) == 2 && r.Method == http.MethodGet:
		items, ok := manager.ListModels(providerID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, ProviderModelListResponse{Items: items})
		return
	case parts[1] == "default-model" && len(parts) == 2 && r.Method == http.MethodPost:
		if sqliteStore == nil {
			writeError(w, http.StatusInternalServerError, "store is not configured")
			return
		}
		var input ProviderDefaultModelRequest
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		preference, err := manager.SetDefaultModel(providerID, input.Model)
		if err != nil {
			writeError(w, llmPrepareStatusCode(err), err.Error())
			return
		}
		if err := sqliteStore.UpsertProviderPreference(r.Context(), preference); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := publishProviderDefaultModelEvent(r.Context(), eventBus, sqliteStore, preference); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ProviderDefaultModelResponse{
			ProviderID:   preference.ProviderID,
			DefaultModel: preference.DefaultModel,
			UpdatedAt:    preference.UpdatedAt,
		})
		return
	case parts[1] != "checks":
		http.NotFound(w, r)
		return
	case len(parts) == 2 && r.Method == http.MethodGet:
		if sqliteStore == nil {
			writeJSON(w, http.StatusOK, ProviderCheckListResponse{Items: []providers.Check{}})
			return
		}
		items, err := sqliteStore.ListProviderChecks(r.Context(), providerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ProviderCheckListResponse{Items: items})
		return
	case len(parts) == 2 && r.Method == http.MethodPost:
		if sqliteStore == nil {
			writeError(w, http.StatusInternalServerError, "store is not configured")
			return
		}
		var input providers.CheckInput
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		checkID := providers.NewCheckID()
		check, runErr := manager.RunCheck(r.Context(), providerID, checkID, input)
		if err := sqliteStore.UpsertProviderCheck(r.Context(), check); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if runErr != nil {
			if _, err := publishProviderCheckEvent(r.Context(), eventBus, sqliteStore, check, "provider.check_failed"); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, check)
			return
		}

		if _, err := publishProviderCheckEvent(r.Context(), eventBus, sqliteStore, check, "provider.check_completed"); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, check)
		return
	case len(parts) == 3 && r.Method == http.MethodGet:
		if sqliteStore == nil {
			http.NotFound(w, r)
			return
		}
		item, found, err := sqliteStore.GetProviderCheck(r.Context(), providerID, parts[2])
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !found {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func handleLLMDispatches(dispatcher *llm.Dispatcher, providerManager *providers.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
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

		resolvedInput, err := resolveProviderDispatchInput(providerManager, input)
		if err != nil {
			writeError(w, llmPrepareStatusCode(err), err.Error())
			return
		}

		dispatch, err := dispatcher.Prepare(resolvedInput, false)
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

func resolveProviderDispatchInput(manager *providers.Manager, input llm.CreateDispatchInput) (llm.CreateDispatchInput, error) {
	if manager == nil {
		return input, nil
	}

	_, effective, err := manager.ResolveDispatchInput(input)
	if err != nil {
		return llm.CreateDispatchInput{}, err
	}
	return effective, nil
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

func handleLLMDispatchStream(dispatcher *llm.Dispatcher, providerManager *providers.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
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

	resolvedInput, err := resolveProviderDispatchInput(providerManager, input)
	if err != nil {
		writeError(w, llmPrepareStatusCode(err), err.Error())
		return
	}

	dispatch, err := dispatcher.Prepare(resolvedInput, true)
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

type chatQueryRequest struct {
	Provider   string   `json:"provider"`
	Model      string   `json:"model"`
	Skills     []string `json:"skills"`
	Query      string   `json:"query"`
	TimeoutMs  int      `json:"timeoutMs"`
	MaxRetries int      `json:"maxRetries"`
}

func handleChatQuery(chatService *chat.Service, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if chatService == nil {
		writeError(w, http.StatusInternalServerError, "chat service is not configured")
		return
	}

	var input chatQueryRequest
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := chatService.Query(r.Context(), chat.QueryInput{
		Query:      strings.TrimSpace(input.Query),
		Provider:   strings.TrimSpace(input.Provider),
		Model:      strings.TrimSpace(input.Model),
		Skills:     append([]string(nil), input.Skills...),
		TimeoutMs:  input.TimeoutMs,
		MaxRetries: input.MaxRetries,
	})
	if err != nil {
		if result.Dispatch.DispatchID == "" {
			writeError(w, llmPrepareStatusCode(err), err.Error())
			return
		}
		response := buildChatQueryResponse(result)
		writeJSON(w, llmDispatchStatusCode(result.Dispatch), response)
		return
	}
	writeJSON(w, http.StatusOK, buildChatQueryResponse(result))
}

func handleChatQueryStream(chatService *chat.Service, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if chatService == nil {
		writeError(w, http.StatusInternalServerError, "chat service is not configured")
		return
	}

	var input chatQueryRequest
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
	var reply strings.Builder
	started := false
	result, execErr := chatService.Stream(r.Context(), chat.QueryInput{
		Query:      strings.TrimSpace(input.Query),
		Provider:   strings.TrimSpace(input.Provider),
		Model:      strings.TrimSpace(input.Model),
		Skills:     append([]string(nil), input.Skills...),
		TimeoutMs:  input.TimeoutMs,
		MaxRetries: input.MaxRetries,
	}, func(chunk chat.StreamChunk) error {
		if !started {
			started = true
			writeSSEEvent(w, "chat.query.started", "", ChatQueryStreamStarted{
				DispatchID: chunk.DispatchID,
				Provider:   chunk.Provider,
				Model:      chunk.Model,
				Skills:     cloneStringSlice(chunk.Skills),
				Query:      strings.TrimSpace(input.Query),
			})
			flusher.Flush()
		}
		reply.WriteString(chunk.Delta)
		writeSSEEvent(w, "chat.query.delta", "", ChatQueryStreamDelta{
			DispatchID: chunk.DispatchID,
			Delta:      chunk.Delta,
			Reply:      reply.String(),
		})
		flusher.Flush()
		return nil
	})
	if !started && result.Dispatch.DispatchID != "" {
		writeSSEEvent(w, "chat.query.started", "", ChatQueryStreamStarted{
			DispatchID: result.Dispatch.DispatchID,
			Provider:   result.Dispatch.Provider,
			Model:      result.Dispatch.Model,
			Skills:     cloneStringSlice(result.Skills),
			Query:      strings.TrimSpace(input.Query),
		})
		flusher.Flush()
	}

	terminalName := "chat.query.completed"
	if execErr != nil || result.Dispatch.Status == llm.DispatchStatusFailed {
		terminalName = "chat.query.failed"
	}
	if result.Dispatch.Status == llm.DispatchStatusCancelled {
		terminalName = "chat.query.cancelled"
	}
	if result.Dispatch.Status == llm.DispatchStatusPartialFailed {
		terminalName = "chat.query.partial_failed"
	}
	writeSSEEvent(w, terminalName, result.Dispatch.DispatchID, buildChatQueryResponse(result))
	flusher.Flush()
}

func buildChatQueryResponse(result chat.QueryResult) ChatQueryResponse {
	return ChatQueryResponse{
		DispatchID:   result.Dispatch.DispatchID,
		Provider:     result.Dispatch.Provider,
		Model:        result.Dispatch.Model,
		Skills:       cloneStringSlice(result.Skills),
		Query:        strings.TrimSpace(result.Query),
		Status:       string(result.Dispatch.Status),
		Partial:      result.Dispatch.Partial,
		Reply:        result.Dispatch.Output,
		FinishReason: result.Dispatch.FinishReason,
		Usage:        result.Dispatch.Usage,
		ErrorCode:    result.Dispatch.ErrorCode,
		Error:        result.Dispatch.Error,
	}
}

func handleSkills(registry *skills.Registry, w http.ResponseWriter, r *http.Request) {
	if registry == nil {
		writeError(w, http.StatusInternalServerError, "skills registry is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, buildSkillRegistryResponse(registry.Snapshot()))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleSkillRoutes(registry *skills.Registry, w http.ResponseWriter, r *http.Request) {
	if registry == nil {
		writeError(w, http.StatusInternalServerError, "skills registry is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/skills/")
	switch {
	case path == "reload":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := registry.Reload(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, buildSkillRegistryResponse(registry.Snapshot()))
	case path != "":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		skill, ok := registry.Get(path)
		if !ok {
			writeError(w, http.StatusNotFound, "skill not found")
			return
		}
		writeJSON(w, http.StatusOK, buildSkillDetailResponse(skill))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func handleSandboxProfiles(manager *sandbox.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "sandbox manager is not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, ListResponse[sandbox.Profile]{Items: manager.ListProfiles()})
}

func handleSandboxProfileRoutes(manager *sandbox.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "sandbox manager is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/sandboxes/profiles/")
	switch {
	case path == "reload":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, ListResponse[sandbox.Profile]{Items: manager.Reload()})
	case path != "":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		profile, ok := manager.GetProfile(path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, profile)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func handleSandboxExecutions(manager *sandbox.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "sandbox manager is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, ListResponse[sandbox.Execution]{Items: manager.ListExecutions()})
	case http.MethodPost:
		var request sandbox.ExecutionRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if strings.TrimSpace(request.RequestedBy) == "" {
			request.RequestedBy = currentActor(r.Context())
		}
		execution, err := manager.StartExecution(r.Context(), request)
		if err != nil {
			switch {
			case errors.Is(err, sandbox.ErrCommandRequired):
				writeError(w, http.StatusBadRequest, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		writeJSON(w, http.StatusCreated, execution)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleSandboxExecutionRoutes(manager *sandbox.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "sandbox manager is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/sandboxes/executions/")
	switch {
	case strings.HasSuffix(path, "/cancel"):
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		executionID := strings.TrimSuffix(path, "/cancel")
		execution, _, err := manager.CancelExecution(executionID)
		if err != nil {
			if errors.Is(err, sandbox.ErrExecutionNotFound) {
				http.NotFound(w, r)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, execution)
	case path != "":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		execution, ok := manager.GetExecution(path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, execution)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func handleSandboxExplain(manager *sandbox.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "sandbox manager is not configured")
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var request sandbox.ExecutionRequest
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(request.RequestedBy) == "" {
		request.RequestedBy = currentActor(r.Context())
	}
	decision, err := manager.Explain(r.Context(), request)
	if err != nil {
		switch {
		case errors.Is(err, sandbox.ErrCommandRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, SandboxExplainResponse{Decision: decision})
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

func handleConnectorRoutes(supervisor *connectors.Supervisor, sessionRouter *router.SessionRouter, manager *runtime.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request) {
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
	if len(parts) == 3 && parts[1] == "ingress" && parts[2] == "messages" {
		handleConnectorIngressMessages(supervisor, sessionRouter, manager, eventBus, sqliteStore, checkpointManager, w, r, parts[0])
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

func handleConnectorIngressMessages(supervisor *connectors.Supervisor, sessionRouter *router.SessionRouter, manager *runtime.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request, connectorID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	connector, ok := supervisor.Get(connectorID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if connector.Status == connectors.StatusFailed || connector.Status == connectors.StatusBackingOff {
		writeError(w, http.StatusConflict, "connector is not accepting ingress")
		return
	}

	var request ConnectorIngressMessageRequest
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.Message.MessageID == "" {
		writeError(w, http.StatusBadRequest, "messageId is required")
		return
	}
	if request.Run != nil && request.Run.Entrypoint == "" {
		writeError(w, http.StatusBadRequest, "run entrypoint is required")
		return
	}

	routeInput, err := resolveConnectorRouteInput(connector, request.Route)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	session, createdSession, err := sessionRouter.Route(routeInput)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := persistSession(r.Context(), sqliteStore, session); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := publishSessionRouteEvents(r.Context(), eventBus, sqliteStore, session, createdSession, map[string]any{
		"source":      "connector.ingress",
		"connectorId": connector.ConnectorID,
		"messageId":   request.Message.MessageID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var (
		run        *runtime.Run
		runCreated bool
	)
	if request.Run != nil {
		createdRun, err := manager.CreateRun(runtime.CreateRunInput{
			SessionID:  session.SessionID,
			Entrypoint: request.Run.Entrypoint,
			Goal:       request.Run.Goal,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := persistRun(r.Context(), sqliteStore, createdRun); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := persistCheckpoint(r.Context(), checkpointManager, createdRun.RunID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
			Category: "run",
			Name:     "run.created",
			Scope: events.Scope{
				SessionID:   createdRun.SessionID,
				RunID:       createdRun.RunID,
				ConnectorID: connector.ConnectorID,
			},
			Resource: events.Resource{
				Kind: "run",
				ID:   createdRun.RunID,
			},
			Payload: map[string]any{
				"entrypoint": createdRun.Entrypoint,
				"goal":       createdRun.Goal,
				"status":     createdRun.Status,
				"source":     "connector.ingress",
				"messageId":  request.Message.MessageID,
			},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		run = &createdRun
		runCreated = true
	}

	acceptedAt := time.Now().UTC()
	response := ConnectorIngressMessageResponse{
		IngressID:      newIngressID(),
		ConnectorID:    connector.ConnectorID,
		AcceptedAt:     acceptedAt,
		Session:        session,
		SessionCreated: createdSession,
		Run:            run,
		RunCreated:     runCreated,
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "connector",
		Name:     "connector.ingress_accepted",
		Scope: events.Scope{
			SessionID:   session.SessionID,
			ConnectorID: connector.ConnectorID,
			RunID:       optionalRunID(run),
		},
		Resource: events.Resource{
			Kind: "connector",
			ID:   connector.ConnectorID,
		},
		Payload: map[string]any{
			"ingressId":      response.IngressID,
			"kind":           session.Kind,
			"channel":        session.Channel,
			"messageId":      request.Message.MessageID,
			"sessionCreated": createdSession,
			"runCreated":     runCreated,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, response)
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

func handleRunStepToolCalls(manager *runtime.Manager, policyEngine *policy.Engine, capabilitySupervisor *capabilities.Supervisor, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request, runID, stepID string) {
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
		var request struct {
			CapabilityID string `json:"capabilityId"`
			ToolName     string `json:"toolName"`
			Input        any    `json:"input"`
			ApprovalID   string `json:"approvalId"`
		}
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if capabilitySupervisor == nil {
			writeError(w, http.StatusInternalServerError, "capability supervisor is not configured")
			return
		}
		capability, ok := capabilitySupervisor.Get(request.CapabilityID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		if policyEngine != nil && requiresApprovalForCapability(capability) {
			approvalResponse, approved, err := authorizeHighRiskToolCall(r, policyEngine, sqliteStore, eventBus, request.ApprovalID, capability, currentActor(r.Context()))
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !approved {
				writeJSON(w, approvalResponse.StatusCode, approvalResponse.Body)
				return
			}
		}
		input := runtime.CreateToolCallInput{
			CapabilityID: request.CapabilityID,
			ToolName:     request.ToolName,
			Input:        request.Input,
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

func persistPairing(ctx context.Context, sqliteStore *store.SQLiteStore, pairing auth.Pairing) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertPairing(ctx, pairing)
}

func persistAccessToken(ctx context.Context, sqliteStore *store.SQLiteStore, token auth.AccessToken) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertAccessToken(ctx, token)
}

func persistApproval(ctx context.Context, sqliteStore *store.SQLiteStore, approval policy.Approval) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertApproval(ctx, approval)
}

func persistDecision(ctx context.Context, sqliteStore *store.SQLiteStore, decision policy.Decision) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertDecision(ctx, decision)
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

type contextKey string

const authenticatedTokenKey contextKey = "authenticated_token"

func withAuthenticatedToken(ctx context.Context, token auth.AccessToken) context.Context {
	return context.WithValue(ctx, authenticatedTokenKey, token)
}

func authenticatedToken(ctx context.Context) (auth.AccessToken, bool) {
	token, ok := ctx.Value(authenticatedTokenKey).(auth.AccessToken)
	return token, ok
}

func currentActor(ctx context.Context) string {
	token, ok := authenticatedToken(ctx)
	if !ok {
		return ""
	}
	if token.Label != "" {
		return token.Label
	}
	return token.TokenID
}

func authenticateRequest(authManager *auth.Manager, r *http.Request) (auth.AccessToken, bool, error) {
	if authManager == nil {
		return auth.AccessToken{}, false, nil
	}
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return auth.AccessToken{}, false, nil
	}
	tokenSecret, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || strings.TrimSpace(tokenSecret) == "" {
		return auth.AccessToken{}, false, auth.ErrTokenInvalid
	}
	token, err := authManager.Authenticate(strings.TrimSpace(tokenSecret))
	if err != nil {
		return auth.AccessToken{}, false, err
	}
	return token, true, nil
}

type approvalGateResponse struct {
	StatusCode int
	Body       any
}

func requiresApprovalForCapability(capability capabilities.Capability) bool {
	switch capability.Kind {
	case "exec", "shell", "browser":
		return true
	default:
		return false
	}
}

func authorizeHighRiskToolCall(r *http.Request, policyEngine *policy.Engine, sqliteStore *store.SQLiteStore, eventBus *events.Bus, approvalID string, capability capabilities.Capability, requestedBy string) (approvalGateResponse, bool, error) {
	if policyEngine == nil {
		return approvalGateResponse{}, false, errors.New("policy engine is not configured")
	}

	if approvalID == "" {
		approval, decision, err := policyEngine.RequestApproval(policy.RequestApprovalInput{
			Action:       "tool_call.execute",
			ResourceKind: "capability",
			ResourceID:   capability.CapabilityID,
			Reason:       "high-risk capability execution requires approval",
			RequestedBy:  requestedBy,
		})
		if err != nil {
			return approvalGateResponse{}, false, err
		}
		if err := persistApproval(r.Context(), sqliteStore, approval); err != nil {
			return approvalGateResponse{}, false, err
		}
		if err := persistDecision(r.Context(), sqliteStore, decision); err != nil {
			return approvalGateResponse{}, false, err
		}
		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
			Category: "policy",
			Name:     "policy.approval_requested",
			Resource: events.Resource{Kind: "approval", ID: approval.ApprovalID},
			Payload: map[string]any{
				"action":       approval.Action,
				"resourceKind": approval.ResourceKind,
				"resourceId":   approval.ResourceID,
				"status":       approval.Status,
			},
		}); err != nil {
			return approvalGateResponse{}, false, err
		}
		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
			Category: "policy",
			Name:     "policy.decision_recorded",
			Resource: events.Resource{Kind: "decision", ID: decision.DecisionID},
			Payload: map[string]any{
				"action":       decision.Action,
				"resourceKind": decision.ResourceKind,
				"resourceId":   decision.ResourceID,
				"outcome":      decision.Outcome,
				"approvalId":   decision.ApprovalID,
			},
		}); err != nil {
			return approvalGateResponse{}, false, err
		}

		return approvalGateResponse{
			StatusCode: http.StatusConflict,
			Body: map[string]any{
				"approval": approval,
				"decision": decision,
			},
		}, false, nil
	}

	approval, ok := policyEngine.GetApproval(approvalID)
	if !ok {
		return approvalGateResponse{
			StatusCode: http.StatusNotFound,
			Body:       map[string]any{"error": policy.ErrApprovalNotFound.Error()},
		}, false, nil
	}
	if approval.Action != "tool_call.execute" || approval.ResourceKind != "capability" || approval.ResourceID != capability.CapabilityID {
		return approvalGateResponse{
			StatusCode: http.StatusBadRequest,
			Body:       map[string]any{"error": "approval does not authorize this tool call"},
		}, false, nil
	}
	switch approval.Status {
	case policy.ApprovalStatusApproved:
		return approvalGateResponse{}, true, nil
	case policy.ApprovalStatusRejected:
		return approvalGateResponse{
			StatusCode: http.StatusForbidden,
			Body: map[string]any{
				"approval": approval,
				"error":    "approval was rejected",
			},
		}, false, nil
	default:
		return approvalGateResponse{
			StatusCode: http.StatusConflict,
			Body: map[string]any{
				"approval": approval,
				"error":    "approval is still pending",
			},
		}, false, nil
	}
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

func publishProviderCheckEvent(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, check providers.Check, eventName string) (events.Event, error) {
	payload := map[string]any{
		"providerId": check.ProviderID,
		"family":     check.Family,
		"authMode":   check.AuthMode,
		"status":     check.Status,
		"model":      check.Model,
		"endpoint":   check.Endpoint,
		"usage":      check.Usage,
	}
	if check.ErrorClass != "" {
		payload["errorClass"] = check.ErrorClass
	}
	if check.ErrorCode != "" {
		payload["errorCode"] = check.ErrorCode
	}
	if check.ErrorMessage != "" {
		payload["errorMessage"] = check.ErrorMessage
	}

	return publishEvent(ctx, eventBus, sqliteStore, events.Event{
		Category: "provider",
		Name:     eventName,
		Resource: events.Resource{Kind: "provider_check", ID: check.CheckID},
		Payload:  payload,
	})
}

func persistManagedProviderState(ctx context.Context, sqliteStore *store.SQLiteStore, state providers.AuthState, models []providers.Model) error {
	if sqliteStore == nil {
		return nil
	}
	if err := sqliteStore.UpsertProviderAuthState(ctx, state); err != nil {
		return err
	}
	return sqliteStore.ReplaceProviderModels(ctx, state.ProviderID, models)
}

func publishProviderAuthEvent(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, state providers.AuthState, eventName string) (events.Event, error) {
	payload := map[string]any{
		"providerId":   state.ProviderID,
		"family":       state.Family,
		"authMode":     state.AuthMode,
		"status":       state.Status,
		"cliAvailable": state.CLIAvailable,
		"accountLabel": state.AccountLabel,
		"accountId":    state.AccountID,
		"plan":         state.Plan,
		"authMethod":   state.AuthMethod,
		"lastError":    state.LastError,
	}
	if len(state.Metadata) > 0 {
		metadata := make(map[string]string, len(state.Metadata))
		for key, value := range state.Metadata {
			metadata[key] = value
		}
		payload["metadata"] = metadata
	}
	return publishEvent(ctx, eventBus, sqliteStore, events.Event{
		Category: "provider",
		Name:     eventName,
		Resource: events.Resource{Kind: "provider_auth", ID: state.ProviderID},
		Payload:  payload,
	})
}

func publishProviderDefaultModelEvent(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, preference providers.Preference) (events.Event, error) {
	return publishEvent(ctx, eventBus, sqliteStore, events.Event{
		Category: "provider",
		Name:     "provider.default_model_updated",
		Resource: events.Resource{Kind: "provider", ID: preference.ProviderID},
		Payload: map[string]any{
			"providerId":   preference.ProviderID,
			"defaultModel": preference.DefaultModel,
			"updatedAt":    preference.UpdatedAt.UTC().Format(time.RFC3339Nano),
		},
	})
}

func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	parts := strings.Split(path, "/")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func llmPrepareStatusCode(err error) int {
	switch {
	case errors.Is(err, llm.ErrProviderRequired),
		errors.Is(err, llm.ErrProviderNotFound),
		errors.Is(err, llm.ErrModelRequired),
		errors.Is(err, llm.ErrMessagesRequired),
		errors.Is(err, providers.ErrModelNotSupported),
		errors.Is(err, providers.ErrManagedAuthUnsupported),
		errors.Is(err, skills.ErrSkillNotFound):
		return http.StatusBadRequest
	case errors.Is(err, skills.ErrSkillsRegistryMissing):
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

func llmDispatchStatusCode(dispatch llm.Dispatch) int {
	switch dispatch.ErrorCode {
	case "timeout", "connect_timeout", "first_chunk_timeout", "idle_timeout", "max_duration_exceeded":
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
	case llm.DispatchStatusPartialFailed:
		return "llm.dispatch.partial_failed"
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
			"partial":      dispatch.Partial,
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

func resolveRunSession(sessionRouter *router.SessionRouter, request CreateRunRequest) (router.Session, bool, error) {
	if sessionRouter == nil {
		return router.Session{}, false, errors.New("session router is required")
	}

	if request.SessionID != "" && request.Route != nil {
		return router.Session{}, false, errors.New("sessionId and route cannot be provided together")
	}

	if request.SessionID != "" {
		session, ok := sessionRouter.GetSession(request.SessionID)
		if !ok {
			return router.Session{}, false, router.ErrSessionNotFound
		}
		session, err := sessionRouter.TouchSession(request.SessionID)
		if err != nil {
			return router.Session{}, false, err
		}
		return session, false, nil
	}

	if request.Route != nil {
		routeInput, err := toRouteInput(*request.Route)
		if err != nil {
			return router.Session{}, false, err
		}
		session, created, err := sessionRouter.Route(routeInput)
		if err != nil {
			return router.Session{}, false, err
		}
		return session, created, nil
	}

	channel := "local"
	peerID := request.Entrypoint
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

func toRouteInput(request SessionRouteRequest) (router.RouteInput, error) {
	return router.RouteInput{
		Kind:      request.Kind,
		Channel:   request.Channel,
		AccountID: request.AccountID,
		PeerID:    request.PeerID,
		ThreadID:  request.ThreadID,
	}, nil
}

func resolveConnectorRouteInput(connector connectors.Connector, request SessionRouteRequest) (router.RouteInput, error) {
	channel := connector.Kind
	if request.Channel != "" && request.Channel != connector.Kind {
		return router.RouteInput{}, errors.New("route channel must match connector kind")
	}

	return router.RouteInput{
		Kind:      request.Kind,
		Channel:   channel,
		AccountID: request.AccountID,
		PeerID:    request.PeerID,
		ThreadID:  request.ThreadID,
	}, nil
}

func publishSessionRouteEvents(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, session router.Session, createdSession bool, extraPayload map[string]any) error {
	if createdSession {
		payload := map[string]any{
			"kind":       session.Kind,
			"channel":    session.Channel,
			"routingKey": session.RoutingKey,
			"generation": session.Generation,
		}
		for key, value := range extraPayload {
			payload[key] = value
		}
		if _, err := publishEvent(ctx, eventBus, sqliteStore, events.Event{
			Category: "session",
			Name:     "session.created",
			Scope: events.Scope{
				SessionID: session.SessionID,
			},
			Resource: events.Resource{
				Kind: "session",
				ID:   session.SessionID,
			},
			Payload: payload,
		}); err != nil {
			return err
		}
	}

	payload := map[string]any{
		"kind":       session.Kind,
		"channel":    session.Channel,
		"routingKey": session.RoutingKey,
		"generation": session.Generation,
	}
	for key, value := range extraPayload {
		payload[key] = value
	}
	_, err := publishEvent(ctx, eventBus, sqliteStore, events.Event{
		Category: "session",
		Name:     "session.routed",
		Scope: events.Scope{
			SessionID: session.SessionID,
		},
		Resource: events.Resource{
			Kind: "session",
			ID:   session.SessionID,
		},
		Payload: payload,
	})
	return err
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

func optionalRunID(run *runtime.Run) string {
	if run == nil {
		return ""
	}
	return run.RunID
}

func newIngressID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "ingress_fallback"
	}

	return "ingress_" + hex.EncodeToString(buf)
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
