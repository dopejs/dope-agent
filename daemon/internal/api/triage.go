package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/triage"
)

// CreateTriagePolicyRequest creates an explicit-rule triage policy (Roadmap 65).
type CreateTriagePolicyRequest struct {
	Name                  string                `json:"name,omitempty"`
	Rules                 []triage.Rule         `json:"rules,omitempty"`
	DefaultClassification triage.Classification `json:"defaultClassification,omitempty"`
}

// RunTriageRequest evaluates a triage policy over an explicit message set. The caller selects
// which (unread/selected) messages to triage; triage does not silently scan the mailbox.
type RunTriageRequest struct {
	Messages []triage.Message `json:"messages,omitempty"`
}

type TriagePolicyListResponse struct {
	Items []triage.Policy `json:"items"`
}

func handleTriagePolicies(manager *triage.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "triage manager is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, TriagePolicyListResponse{Items: manager.ListPolicies()})
	case http.MethodPost:
		var request CreateTriagePolicyRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		policy, err := manager.CreatePolicy(strings.TrimSpace(request.Name), request.Rules, request.DefaultClassification)
		if err != nil {
			writeTriageError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, policy)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleTriagePolicyRoutes(manager *triage.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "triage manager is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/triage/policies/")
	parts := strings.Split(path, "/")
	policyID := strings.TrimSpace(parts[0])
	if policyID == "" {
		http.NotFound(w, r)
		return
	}
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		policy, ok := manager.GetPolicy(policyID)
		if !ok {
			writeTriageError(w, triage.ErrPolicyNotFound)
			return
		}
		writeJSON(w, http.StatusOK, policy)
	case len(parts) == 2 && parts[1] == "run" && r.Method == http.MethodPost:
		var request RunTriageRequest
		if err := decodeJSONBody(r, &request); err != nil && !strings.Contains(err.Error(), "EOF") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		run, err := manager.Run(policyID, request.Messages)
		if err != nil {
			writeTriageError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, run)
	default:
		http.NotFound(w, r)
	}
}

func writeTriageError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, triage.ErrPolicyNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, triage.ErrInvalidRule), errors.Is(err, triage.ErrInvalidPolicy):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
