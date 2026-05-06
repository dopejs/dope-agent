package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dopejs/dope-agent/daemon/internal/activation"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

const activationNotImplementedCode = "activation_not_implemented"

type activationStartRequest struct {
	Source string `json:"source"`
}

type activationTestChatRequest struct {
	Message string `json:"message"`
}

func handleActivation(service *activation.Service, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleActivationGet(service, w, r)
	case http.MethodPost:
		handleActivationStart(service, w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleActivationGet(service *activation.Service, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	if service == nil {
		writeActivationNotImplemented(w)
		return
	}
	input := activation.GetInput{TenantContext: tenantContext}
	if token, ok := authenticatedToken(r.Context()); ok {
		input.Token = authTokenAuthority(token)
	}
	state, err := service.Get(r.Context(), input)
	if err != nil {
		writeActivationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"activation": state})
}

func handleActivationStart(service *activation.Service, w http.ResponseWriter, r *http.Request) {
	if service == nil {
		if _, ok := tenantContextFromContext(r.Context()); !ok {
			writeTenantDenial(w, http.StatusForbidden)
			return
		}
		writeActivationNotImplemented(w)
		return
	}
	var request activationStartRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	token, hasToken := authenticatedToken(r.Context())
	tenantContext, hasTenantContext := tenantContextFromContext(r.Context())
	if !hasToken && !hasTenantContext {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	input := activation.ActivateInput{
		Source:        request.Source,
		TenantContext: tenantContext,
	}
	if hasToken {
		input.Token = authTokenAuthority(token)
	} else {
		input.Token = identity.TokenAuthority{
			TokenID:         tenantContext.TokenID,
			PrincipalID:     tenantContext.PrincipalID,
			DefaultTenantID: tenantContext.TenantID,
			Status:          identity.StatusActive,
		}
	}
	state, err := service.Activate(r.Context(), input)
	if err != nil {
		writeActivationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"activation": state})
}

func handleActivationTestChat(service *activation.Service, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if service == nil {
		writeActivationNotImplemented(w)
		return
	}
	var request activationTestChatRequest
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	input := activation.RunTestChatInput{
		TenantContext: tenantContext,
		Message:       request.Message,
	}
	if token, ok := authenticatedToken(r.Context()); ok {
		input.Token = authTokenAuthority(token)
	}
	state, testChat, err := service.RunTestChat(r.Context(), input)
	if err != nil {
		writeActivationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"activation": state, "testChat": testChat})
}

func handleActivationDiagnostics(service *activation.Service, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if service == nil {
		writeActivationNotImplemented(w)
		return
	}
	input := activation.GetInput{TenantContext: tenantContext}
	if token, ok := authenticatedToken(r.Context()); ok {
		input.Token = authTokenAuthority(token)
	}
	items, err := service.Diagnostics(r.Context(), input)
	if err != nil {
		writeActivationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func writeActivationNotImplemented(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotImplemented, map[string]any{
		"error": activationNotImplementedCode,
		"code":  activationNotImplementedCode,
	})
}

func writeActivationError(w http.ResponseWriter, err error) {
	var activationErr *activation.Error
	if errors.As(err, &activationErr) {
		writeJSON(w, http.StatusForbidden, map[string]any{
			"error":            activationErr.Error(),
			"code":             string(activationErr.ReasonCode),
			"reasonCode":       string(activationErr.ReasonCode),
			"stage":            string(activationErr.Stage),
			"retryable":        activationErr.Retryable,
			"remediationOwner": string(activationErr.RemediationOwner),
		})
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}
