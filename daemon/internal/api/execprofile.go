package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/execprofile"
)

type ExecutionProfileListResponse struct {
	Items []execprofile.ProfileProjection `json:"items"`
}

// SelectExecutionProfileRequest selects a tenant's execution profile (Roadmap 69).
type SelectExecutionProfileRequest struct {
	TenantID string `json:"tenantId,omitempty"`
	Actor    string `json:"actor,omitempty"`
}

// ExplainExecutionRequest asks which profiles can run a tool needing the given capabilities.
type ExplainExecutionRequest struct {
	RequiredCapabilities []string `json:"requiredCapabilities,omitempty"`
}

func execTenant(r *http.Request, bodyTenant string) string {
	if t := strings.TrimSpace(bodyTenant); t != "" {
		return t
	}
	return strings.TrimSpace(r.URL.Query().Get("tenantId"))
}

func handleExecutionProfiles(manager *execprofile.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "execution profile manager is not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, ExecutionProfileListResponse{Items: manager.ListProfiles(r.Context())})
}

func handleExecutionProfileRoutes(manager *execprofile.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "execution profile manager is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/execution/profiles/")
	parts := strings.Split(path, "/")
	profileID := strings.TrimSpace(parts[0])
	if profileID == "" {
		http.NotFound(w, r)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodGet {
		proj, err := manager.GetProfile(r.Context(), profileID)
		if err != nil {
			writeExecError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, proj)
		return
	}
	if len(parts) == 2 && parts[1] == "select" && r.Method == http.MethodPost {
		var request SelectExecutionProfileRequest
		if err := decodeJSONBody(r, &request); err != nil && !strings.Contains(err.Error(), "EOF") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		selection, err := manager.SelectProfile(r.Context(), execTenant(r, request.TenantID), profileID, strings.TrimSpace(request.Actor))
		if err != nil {
			writeExecError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, selection)
		return
	}
	http.NotFound(w, r)
}

func handleExecutionExplain(manager *execprofile.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "execution profile manager is not configured")
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var request ExplainExecutionRequest
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, manager.ExplainDenial(r.Context(), request.RequiredCapabilities))
}

func writeExecError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, execprofile.ErrProfileNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, execprofile.ErrPermissionDenied):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, execprofile.ErrProfileUnavailable), errors.Is(err, execprofile.ErrInvalidProfile):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
