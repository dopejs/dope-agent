package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

type threadLifecycleActionRequest struct {
	ReasonCode string `json:"reasonCode"`
	Note       string `json:"note"`
}

type threadLifecycleActionResponse struct {
	ThreadID                 string                        `json:"threadId"`
	LifecycleState           threads.LifecycleState        `json:"lifecycleState"`
	PreviousSessionSegmentID string                        `json:"previousSessionSegmentId,omitempty"`
	CurrentSessionSegmentID  string                        `json:"currentSessionSegmentId,omitempty"`
	AuditEventID             string                        `json:"auditEventId"`
	ChangedAt                time.Time                     `json:"changedAt"`
	Action                   threads.LifecycleActionKind   `json:"action"`
	AvailableActions         []threads.LifecycleActionKind `json:"availableActions"`
}

func handleThreadLifecycleRoutes(sqliteStore *store.SQLiteStore, eventBus *events.Bus, w http.ResponseWriter, r *http.Request) {
	const prefix = "/v1/threads"
	if r.URL.Path == prefix {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleThreadList(sqliteStore, w, r)
		return
	}
	if !strings.HasPrefix(r.URL.Path, prefix+"/") {
		http.NotFound(w, r)
		return
	}
	remaining := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix+"/"), "/")
	parts := strings.Split(remaining, "/")
	threadID := parts[0]
	if threadID == "" || len(parts) > 3 {
		http.NotFound(w, r)
		return
	}
	if len(parts) == 3 {
		if parts[1] != "continuity-previews" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		handleThreadContinuityPreviewDetail(sqliteStore, w, r, threadID, parts[2])
		return
	}
	if len(parts) == 2 {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleThreadLifecycleAction(sqliteStore, eventBus, w, r, threadID, parts[1])
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	handleThreadDetail(sqliteStore, w, r, threadID)
}

func handleThreadList(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireThreadPermission(r, identity.PermissionCredentialsInspect)
	if !ok {
		writeCredentialDenial(w, http.StatusForbidden, "permission_missing")
		return
	}
	if sqliteStore == nil {
		writeJSON(w, http.StatusOK, store.EmptyThreadListResponse(tenantContext.TenantID))
		return
	}
	response, err := sqliteStore.ListThreadsForTenant(r.Context(), store.ThreadListQuery{
		TenantID:     tenantContext.TenantID,
		Limit:        parseThreadLifecycleLimit(r.URL.Query().Get("limit")),
		Cursor:       r.URL.Query().Get("cursor"),
		StateFilter:  r.URL.Query().Get("state"),
		SourceFilter: r.URL.Query().Get("sourceKind"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func handleThreadLifecycleAction(sqliteStore *store.SQLiteStore, eventBus *events.Bus, w http.ResponseWriter, r *http.Request, threadID, actionName string) {
	tenantContext, ok := requireThreadPermission(r, identity.PermissionConnectorsManage)
	if !ok {
		writeCredentialDenial(w, http.StatusForbidden, "permission_missing")
		return
	}
	if sqliteStore == nil {
		http.NotFound(w, r)
		return
	}
	kind, ok := threadLifecycleActionKind(actionName)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var input threadLifecycleActionRequest
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	now := time.Now().UTC()
	result, found, err := sqliteStore.ApplyThreadLifecycleAction(r.Context(), tenantContext.TenantID, threadID, kind, threads.LifecycleMutationInput{
		ActorPrincipalID: tenantContext.PrincipalID,
		ReasonCode:       coalesceReason(input.ReasonCode, string(kind)),
		AuditEventID:     fmt.Sprintf("audit_thread_%s_%d", kind, now.UnixNano()),
		Now:              now,
	})
	if err != nil {
		if eventBus != nil && (errors.Is(err, threads.ErrAuditEvidenceRequired) || errors.Is(err, threads.ErrLifecycleMutationConflict)) {
			_, _ = publishEvent(r.Context(), eventBus, sqliteStore, events.ThreadAuditFailedClosedEvent(tenantContext.TenantID, threadID, err.Error()))
		}
		handleThreadLifecycleMutationError(w, err)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	if eventBus != nil {
		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.ThreadLifecycleEvent(result.Action)); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, threadLifecycleActionResponse{
		ThreadID:                 result.Thread.ThreadID,
		LifecycleState:           result.Thread.LifecycleState,
		PreviousSessionSegmentID: result.Action.PriorSessionSegmentID,
		CurrentSessionSegmentID:  result.Thread.CurrentSessionSegmentID,
		AuditEventID:             result.Action.AuditEventID,
		ChangedAt:                result.Action.CompletedAt,
		Action:                   kind,
		AvailableActions:         threads.AvailableActions(result.Thread.LifecycleState),
	})
}

func handleThreadDetail(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, threadID string) {
	tenantContext, ok := requireThreadPermission(r, identity.PermissionCredentialsInspect)
	if !ok {
		writeCredentialDenial(w, http.StatusForbidden, "permission_missing")
		return
	}
	if sqliteStore == nil {
		http.NotFound(w, r)
		return
	}
	response, found, err := sqliteStore.GetThreadDetailForTenant(r.Context(), tenantContext.TenantID, threadID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func handleThreadContinuityPreviewDetail(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, threadID, previewID string) {
	tenantContext, ok := requireThreadPermission(r, identity.PermissionCredentialsInspect)
	if !ok {
		writeCredentialDenial(w, http.StatusForbidden, "permission_missing")
		return
	}
	if sqliteStore == nil {
		http.NotFound(w, r)
		return
	}
	response, found, err := sqliteStore.GetContinuityPreviewDetail(r.Context(), tenantContext.TenantID, threadID, previewID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func requireThreadPermission(r *http.Request, permission identity.Permission) (identity.TenantContext, bool) {
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok || tenantContext.TenantID == "" {
		return identity.TenantContext{}, false
	}
	if !identity.HasPermission(tenantContext.Permissions, permission) {
		return identity.TenantContext{}, false
	}
	return tenantContext, true
}

func parseThreadLifecycleLimit(raw string) int {
	limit := parseChannelManagementLimit(raw)
	if limit == 0 {
		return 20
	}
	return limit
}

func threadLifecycleActionKind(name string) (threads.LifecycleActionKind, bool) {
	switch name {
	case "reset":
		return threads.LifecycleActionReset, true
	case "archive":
		return threads.LifecycleActionArchive, true
	case "reopen":
		return threads.LifecycleActionReopen, true
	default:
		return "", false
	}
}

func handleThreadLifecycleMutationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, threads.ErrAuditEvidenceRequired):
		writeError(w, http.StatusInternalServerError, "thread lifecycle audit evidence is required")
	case errors.Is(err, threads.ErrLifecycleTransitionNotAllowed):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, threads.ErrLifecycleMutationConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, threads.ErrLifecycleReopenNotEligible):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
