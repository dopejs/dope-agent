package api

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func handleRunComputerUseSessions(manager *computeruse.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, runtimeManager *runtime.Manager, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request, runID string) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "computer-use manager is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		sessions, err := manager.ListSessions(r.Context(), runID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ComputerUseSessionListResponse{Items: sessions})
	case http.MethodPost:
		var request CreateComputerUseSessionRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		session, err := manager.CreateSession(r.Context(), runID, computeruse.CreateSessionInput{
			WorkflowID:     request.WorkflowID,
			WorkflowStepID: request.WorkflowStepID,
			DriverKind:     request.DriverKind,
			InitialURL:     request.InitialURL,
		})
		if err != nil {
			switch {
			case errors.Is(err, runtime.ErrRunNotFound):
				http.NotFound(w, r)
			default:
				writeError(w, http.StatusBadRequest, err.Error())
			}
			return
		}
		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
			Category: "capability",
			Name:     "computer_use.session_created",
			Scope:    events.Scope{RunID: runID, ComputerUseSessionID: session.ComputerUseSessionID},
			Resource: events.Resource{Kind: "computer_use_session", ID: session.ComputerUseSessionID},
			Payload: map[string]any{
				"status":               session.Status,
				"computerUseSessionId": session.ComputerUseSessionID,
			},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = runtimeManager
		_ = checkpointManager
		writeJSON(w, http.StatusCreated, session)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleRunComputerUseSessionByID(manager *computeruse.Manager, w http.ResponseWriter, r *http.Request, runID, sessionID string) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "computer-use manager is not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok, err := manager.GetSession(r.Context(), runID, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func handleRunComputerUseSessionClose(manager *computeruse.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, runID, sessionID string) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "computer-use manager is not configured")
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, err := manager.CloseSession(r.Context(), runID, sessionID)
	if err != nil {
		if errors.Is(err, computeruse.ErrSessionNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "capability",
		Name:     "computer_use.session_status_changed",
		Scope:    events.Scope{RunID: runID, ComputerUseSessionID: session.ComputerUseSessionID},
		Resource: events.Resource{Kind: "computer_use_session", ID: session.ComputerUseSessionID},
		Payload: map[string]any{
			"status":               session.Status,
			"computerUseSessionId": session.ComputerUseSessionID,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func handleRunComputerUseActions(manager *computeruse.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, runtimeManager *runtime.Manager, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request, runID, sessionID string) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "computer-use manager is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		session, ok, err := manager.GetSession(r.Context(), runID, sessionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, ComputerUseActionListResponse{Items: session.Actions})
	case http.MethodPost:
		var request CreateComputerUseActionRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		result, approval, decision, err := manager.CreateAction(r.Context(), runID, sessionID, currentActor(r.Context()), computeruse.CreateActionInput{
			ActionKind:         request.ActionKind,
			URL:                request.URL,
			Value:              request.Value,
			SelectedValue:      request.SelectedValue,
			WaitMs:             request.WaitMs,
			PageTarget:         request.PageTarget,
			TargetMatchContext: request.TargetMatchContext,
			Rationale:          request.Rationale,
		})
		if err != nil {
			switch {
			case errors.Is(err, computeruse.ErrSessionNotFound):
				http.NotFound(w, r)
			default:
				writeError(w, http.StatusBadRequest, err.Error())
			}
			return
		}
		if err := persistComputerUseRuntimeTracking(r.Context(), sqliteStore, runtimeManager, checkpointManager, result.Action); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		publishComputerUseArtifacts(r.Context(), eventBus, sqliteStore, result.Action)
		if result.Action.FailureClass == string(computeruse.FailureClassTargetMismatch) {
			publishComputerUseTargetMismatch(r.Context(), eventBus, sqliteStore, result.Action)
		}
		if approval != nil {
			if err := persistApproval(r.Context(), sqliteStore, *approval); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if err := recordThreadApprovalProjection(r.Context(), eventBus, sqliteStore, *approval, "policy.approval_requested"); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		if decision != nil {
			if err := persistDecision(r.Context(), sqliteStore, *decision); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
			Category: "capability",
			Name:     "computer_use.action_requested",
			Scope: events.Scope{
				RunID:                runID,
				StepID:               result.Action.StepID,
				ComputerUseSessionID: result.Action.ComputerUseSessionID,
				ComputerUseActionID:  result.Action.ComputerUseActionID,
			},
			Resource: events.Resource{Kind: "computer_use_action", ID: result.Action.ComputerUseActionID},
			Payload: map[string]any{
				"status":               result.Action.Status,
				"actionKind":           result.Action.ActionKind,
				"approvalId":           result.Action.ApprovalID,
				"computerUseSessionId": result.Action.ComputerUseSessionID,
				"computerUseActionId":  result.Action.ComputerUseActionID,
			},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if result.Pending {
			payload := map[string]any{"action": result.Action}
			if approval != nil {
				payload["approval"] = approval
			}
			if decision != nil {
				payload["decision"] = decision
			}
			writeJSON(w, http.StatusConflict, payload)
			return
		}
		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
			Category: "capability",
			Name:     "computer_use.action_status_changed",
			Scope: events.Scope{
				RunID:                runID,
				StepID:               result.Action.StepID,
				ComputerUseSessionID: result.Action.ComputerUseSessionID,
				ComputerUseActionID:  result.Action.ComputerUseActionID,
			},
			Resource: events.Resource{Kind: "computer_use_action", ID: result.Action.ComputerUseActionID},
			Payload: map[string]any{
				"status":               result.Action.Status,
				"failureClass":         result.Action.FailureClass,
				"computerUseSessionId": result.Action.ComputerUseSessionID,
				"computerUseActionId":  result.Action.ComputerUseActionID,
			},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, result.Action)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleRunComputerUseActionByID(manager *computeruse.Manager, w http.ResponseWriter, r *http.Request, runID, sessionID, actionID string) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "computer-use manager is not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok, err := manager.GetSession(r.Context(), runID, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	for _, action := range session.Actions {
		if action.ComputerUseActionID == actionID {
			writeJSON(w, http.StatusOK, action)
			return
		}
	}
	http.NotFound(w, r)
}

func handleComputerUseArtifactRoutes(manager *computeruse.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "computer-use manager is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/computer-use/artifacts/")
	parts := strings.Split(path, "/")
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		artifact, ok, err := manager.GetArtifact(r.Context(), parts[0])
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, artifact)
		return
	}
	if len(parts) == 2 && parts[1] == "content" {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		artifact, content, ok, err := manager.ReadArtifactContent(r.Context(), parts[0])
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, ComputerUseArtifactContentResponse{
			ArtifactID: artifact.ArtifactID,
			MIMEType:   artifact.MIMEType,
			FileName:   artifact.FileName,
			Status:     string(artifact.Status),
			Content:    base64.StdEncoding.EncodeToString(content),
		})
		return
	}
	http.NotFound(w, r)
}

func persistComputerUseRuntimeTracking(ctx context.Context, sqliteStore *store.SQLiteStore, runtimeManager *runtime.Manager, checkpointManager *checkpoints.Manager, action computeruse.Action) error {
	if sqliteStore == nil || runtimeManager == nil {
		return nil
	}
	step, ok := runtimeManager.GetStep(action.RunID, action.StepID)
	if ok {
		if err := persistStep(ctx, sqliteStore, step); err != nil {
			return err
		}
	}
	toolCall, ok := runtimeManager.GetToolCall(action.RunID, action.StepID, action.ToolCallID)
	if ok {
		if err := persistToolCall(ctx, sqliteStore, runtimeManager, toolCall); err != nil {
			return err
		}
	}
	return persistCheckpoint(ctx, checkpointManager, action.RunID)
}
