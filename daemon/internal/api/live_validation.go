package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

type LiveValidationAttemptResource = livevalidation.Attempt
type LiveValidationAttemptListResponse struct {
	TenantID         string                          `json:"tenantId,omitempty"`
	EnvironmentScope string                          `json:"environmentScope,omitempty"`
	Items            []LiveValidationAttemptResource `json:"items"`
}

type LiveValidationSupportMatrixResponse struct {
	EnvironmentScope string                     `json:"environmentScope,omitempty"`
	Version          string                     `json:"version"`
	Items            []livevalidation.MatrixRow `json:"items"`
}

type CreateLiveValidationRequest = livevalidation.StartInput
type CreateLiveValidationResponse = livevalidation.StartResult
type ResolveLiveValidationReconciliationRequest struct {
	Resolution   livevalidation.ReconciliationResolutionValue `json:"resolution"`
	Reason       string                                       `json:"reason"`
	EvidenceRefs []string                                     `json:"evidenceRefs,omitempty"`
}
type UpdateLiveValidationKillSwitchRequest struct {
	Scope     livevalidation.KillSwitchScope `json:"scope"`
	TenantID  string                         `json:"tenantId,omitempty"`
	Enabled   bool                           `json:"enabled"`
	Reason    string                         `json:"reason"`
	ExpiresAt *time.Time                     `json:"expiresAt,omitempty"`
}

func handleLiveValidationRoutes(manager *livevalidation.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "live validation manager is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/live-validations")
	path = strings.Trim(path, "/")
	if path == "" {
		handleLiveValidationCollection(manager, eventBus, sqliteStore, w, r)
		return
	}
	if path == "support-matrix" {
		handleLiveValidationSupportMatrix(manager, w, r)
		return
	}
	if path == "kill-switches" {
		handleLiveValidationKillSwitches(manager, eventBus, sqliteStore, w, r)
		return
	}
	handleLiveValidationItem(manager, eventBus, sqliteStore, path, w, r)
}

func handleLiveValidationKillSwitches(manager *livevalidation.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tenantID := r.URL.Query().Get("tenantId")
		if tenantID == "" {
			if tenantContext, ok := tenantctx.FromContext(r.Context()); ok {
				tenantID = tenantContext.TenantID
			}
		}
		items, err := manager.ListKillSwitches(r.Context(), livevalidation.KillSwitchFilter{
			TenantID: tenantID,
			Scope:    livevalidation.KillSwitchScope(r.URL.Query().Get("scope")),
			Limit:    queryInt(r, "limit"),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"tenantId": tenantID, "items": items})
	case http.MethodPost:
		var input UpdateLiveValidationKillSwitchRequest
		if err := decodeOptionalJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		item, err := manager.SetKillSwitch(r.Context(), livevalidation.KillSwitch{
			Scope:     input.Scope,
			TenantID:  input.TenantID,
			Enabled:   input.Enabled,
			Reason:    input.Reason,
			ExpiresAt: input.ExpiresAt,
		})
		if err != nil {
			if errors.Is(err, livevalidation.ErrKillSwitchPermissionDenied) {
				writeError(w, http.StatusForbidden, err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		publishLiveValidationAttemptEvent(r.Context(), eventBus, sqliteStore, events.LiveValidationKillSwitchChangedName, livevalidation.Attempt{TenantID: item.TenantID, ValidationID: item.KillSwitchID, Status: livevalidation.AttemptStatusAborted, UpdatedAt: item.ChangedAt}, nil)
		writeJSON(w, http.StatusOK, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleLiveValidationSupportMatrix(manager *livevalidation.Manager, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	matrix, err := manager.SupportMatrix()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, LiveValidationSupportMatrixResponse{
		EnvironmentScope: manager.EnvironmentScope(),
		Version:          "v1",
		Items:            matrix.Rows(),
	})
}

func handleLiveValidationCollection(manager *livevalidation.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var input CreateLiveValidationRequest
		if err := decodeOptionalJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		result, err := manager.Start(r.Context(), input)
		if err != nil {
			if errors.Is(err, livevalidation.ErrLiveValidationDisabled) {
				writeError(w, http.StatusServiceUnavailable, err.Error())
				return
			}
			if errors.Is(err, livevalidation.ErrLiveValidationBlocked) {
				publishLiveValidationStartEvent(r.Context(), eventBus, sqliteStore, result)
				recordLiveValidationAudit(r.Context(), sqliteStore, result, identity.AuditOutcomeDenied)
				writeJSON(w, http.StatusConflict, result)
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		publishLiveValidationStartEvent(r.Context(), eventBus, sqliteStore, result)
		recordLiveValidationAudit(r.Context(), sqliteStore, result, identity.AuditOutcomeSucceeded)
		writeJSON(w, http.StatusAccepted, result)
	case http.MethodGet:
		tenantID := ""
		if tenantContext, ok := tenantctx.FromContext(r.Context()); ok {
			tenantID = tenantContext.TenantID
		}
		items, err := manager.ListAttempts(r.Context(), livevalidation.AttemptFilter{
			TenantID:    tenantID,
			CandidateID: r.URL.Query().Get("candidateId"),
			Status:      livevalidation.AttemptStatus(r.URL.Query().Get("status")),
			Limit:       queryInt(r, "limit"),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, LiveValidationAttemptListResponse{TenantID: tenantID, EnvironmentScope: manager.EnvironmentScope(), Items: items})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleLiveValidationItem(manager *livevalidation.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, path string, w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 1 && r.Method == http.MethodGet {
		item, ok, err := manager.GetAttempt(r.Context(), parts[0])
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "live validation not found")
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	if len(parts) == 2 && parts[1] == "ledger" && r.Method == http.MethodGet {
		tenantID := ""
		if tenantContext, ok := tenantctx.FromContext(r.Context()); ok {
			tenantID = tenantContext.TenantID
		}
		items, err := manager.ListLedgerEntries(r.Context(), livevalidation.LedgerFilter{
			TenantID:     tenantID,
			ValidationID: parts[0],
			ToolClass:    livevalidation.ToolClass(r.URL.Query().Get("toolClass")),
			Outcome:      livevalidation.LedgerOutcome(r.URL.Query().Get("outcome")),
			Limit:        queryInt(r, "limit"),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"validationId": parts[0], "tenantId": tenantID, "items": items})
		return
	}
	if len(parts) == 2 && parts[1] == "abort" && r.Method == http.MethodPost {
		item, err := manager.Abort(r.Context(), parts[0])
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		publishLiveValidationAttemptEvent(r.Context(), eventBus, sqliteStore, events.LiveValidationAbortedName, item, nil)
		writeJSON(w, http.StatusOK, item)
		return
	}
	if len(parts) == 2 && parts[1] == "retention" && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, manager.DefaultRetentionPolicy(r.Context()))
		return
	}
	if len(parts) == 2 && parts[1] == "compare" && r.Method == http.MethodPost {
		comparison, err := manager.CreateComparison(r.Context(), parts[0])
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if eventBus != nil {
			_, _ = publishEvent(r.Context(), eventBus, sqliteStore, events.LiveValidationComparisonEvent(comparison))
		}
		writeJSON(w, http.StatusAccepted, comparison)
		return
	}
	if len(parts) == 4 && parts[1] == "reconciliations" && parts[3] == "resolve" && r.Method == http.MethodPost {
		var input ResolveLiveValidationReconciliationRequest
		if err := decodeOptionalJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		resolution, err := manager.ResolveReconciliation(r.Context(), livevalidation.ReconciliationResolution{
			AmbiguousCommitID: parts[2],
			Resolution:        input.Resolution,
			Reason:            input.Reason,
			EvidenceRefs:      input.EvidenceRefs,
		})
		if err != nil {
			if errors.Is(err, livevalidation.ErrReconciliationPermissionDenied) {
				writeError(w, http.StatusForbidden, err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if eventBus != nil {
			_, _ = publishEvent(r.Context(), eventBus, sqliteStore, events.LiveValidationReconciliationEvent(resolution))
		}
		writeJSON(w, http.StatusOK, resolution)
		return
	}
	writeError(w, http.StatusNotFound, "live validation route not found")
}

func publishLiveValidationStartEvent(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, result livevalidation.StartResult) {
	switch result.Attempt.Status {
	case livevalidation.AttemptStatusBlocked:
		publishLiveValidationAttemptEvent(ctx, eventBus, sqliteStore, events.LiveValidationBlockedName, result.Attempt, result.Denials)
	case livevalidation.AttemptStatusAwaitingApproval:
		publishLiveValidationAttemptEvent(ctx, eventBus, sqliteStore, events.LiveValidationAwaitingApprovalName, result.Attempt, nil)
	default:
		publishLiveValidationAttemptEvent(ctx, eventBus, sqliteStore, events.LiveValidationStartedName, result.Attempt, nil)
	}
}

func publishLiveValidationAttemptEvent(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, name string, attempt livevalidation.Attempt, denials []livevalidation.Denial) {
	if eventBus == nil {
		return
	}
	_, _ = publishEvent(ctx, eventBus, sqliteStore, events.LiveValidationAttemptEvent(name, attempt, denials))
}

func recordLiveValidationAudit(ctx context.Context, sqliteStore *store.SQLiteStore, result livevalidation.StartResult, outcome string) {
	if sqliteStore == nil || result.Attempt.TenantID == "" {
		return
	}
	reasonCode := "live_validation_started"
	if outcome == identity.AuditOutcomeDenied && len(result.Denials) > 0 {
		reasonCode = result.Denials[0].ReasonCode
	}
	_, _ = sqliteStore.AppendTenantAuditEvent(ctx, audit.BuildLiveValidationAuditEvent(audit.LiveValidationAuditInput{
		Attempt:    result.Attempt,
		Denials:    result.Denials,
		Action:     "live_validation.start",
		Outcome:    outcome,
		ReasonCode: reasonCode,
		CreatedAt:  result.Attempt.UpdatedAt,
	}))
}
