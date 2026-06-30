package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/evidence"
)

// GenerateEvidenceBundleRequest requests a redacted support evidence bundle (Roadmap 71).
type GenerateEvidenceBundleRequest struct {
	TenantID string         `json:"tenantId,omitempty"`
	Actor    string         `json:"actor,omitempty"`
	Scope    evidence.Scope `json:"scope"`
}

type EvidenceBundleListResponse struct {
	Items []evidence.Bundle `json:"items"`
}

func evidenceTenant(r *http.Request, bodyTenant string) string {
	if t := strings.TrimSpace(bodyTenant); t != "" {
		return t
	}
	return strings.TrimSpace(r.URL.Query().Get("tenantId"))
}

func evidenceActor(r *http.Request, bodyActor string) string {
	if a := strings.TrimSpace(bodyActor); a != "" {
		return a
	}
	return strings.TrimSpace(r.URL.Query().Get("actor"))
}

func handleEvidenceBundles(manager *evidence.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "evidence manager is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		bundles, err := manager.ListForTenant(r.Context(), evidenceTenant(r, ""), evidenceActor(r, ""))
		if err != nil {
			writeEvidenceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, EvidenceBundleListResponse{Items: bundles})
	case http.MethodPost:
		var request GenerateEvidenceBundleRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		bundle, err := manager.Generate(r.Context(), evidenceTenant(r, request.TenantID), evidenceActor(r, request.Actor), request.Scope)
		if err != nil {
			writeEvidenceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, bundle)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleEvidenceBundleRoutes(manager *evidence.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "evidence manager is not configured")
		return
	}
	bundleID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/support/evidence-bundles/"))
	if bundleID == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	bundle, err := manager.Get(r.Context(), evidenceTenant(r, ""), evidenceActor(r, ""), bundleID)
	if err != nil {
		writeEvidenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bundle)
}

func writeEvidenceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, evidence.ErrBundleNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, evidence.ErrPermissionDenied), errors.Is(err, evidence.ErrCrossTenantAccess):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, evidence.ErrInvalidScope):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, evidence.ErrRedactionFailed):
		// Fail closed: redaction could not guarantee secret removal.
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
