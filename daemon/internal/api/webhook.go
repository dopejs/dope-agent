package api

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/webhook"
)

// CreateWebhookRequest registers a webhook trigger endpoint (Roadmap 67).
type CreateWebhookRequest struct {
	TenantID   string             `json:"tenantId,omitempty"`
	Name       string             `json:"name,omitempty"`
	TargetKind webhook.TargetKind `json:"targetKind,omitempty"`
	TargetRef  string             `json:"targetRef,omitempty"`
}

type WebhookTenantRequest struct {
	TenantID string `json:"tenantId,omitempty"`
}

type WebhookListResponse struct {
	Items []webhook.Endpoint `json:"items"`
}

func webhookTenant(r *http.Request, bodyTenant string) string {
	if t := strings.TrimSpace(bodyTenant); t != "" {
		return t
	}
	return strings.TrimSpace(r.URL.Query().Get("tenantId"))
}

func handleWebhooks(manager *webhook.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "webhook manager is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, WebhookListResponse{Items: manager.ListForTenant(webhookTenant(r, ""))})
	case http.MethodPost:
		var request CreateWebhookRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		created, err := manager.Create(webhookTenant(r, request.TenantID), strings.TrimSpace(request.Name), request.TargetKind, strings.TrimSpace(request.TargetRef))
		if err != nil {
			writeWebhookError(w, err)
			return
		}
		// The plaintext secret is returned exactly once here.
		writeJSON(w, http.StatusCreated, created)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleWebhookRoutes(manager *webhook.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "webhook manager is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/webhooks/")
	parts := strings.Split(path, "/")
	webhookID := strings.TrimSpace(parts[0])
	if webhookID == "" {
		http.NotFound(w, r)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodGet {
		endpoint, ok := manager.Get(webhookTenant(r, ""), webhookID)
		if !ok {
			writeWebhookError(w, webhook.ErrEndpointNotFound)
			return
		}
		writeJSON(w, http.StatusOK, endpoint)
		return
	}
	if len(parts) == 2 && r.Method == http.MethodPost {
		var request WebhookTenantRequest
		if err := decodeJSONBody(r, &request); err != nil && !strings.Contains(err.Error(), "EOF") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		tenant := webhookTenant(r, request.TenantID)
		switch parts[1] {
		case "rotate":
			rotated, err := manager.Rotate(tenant, webhookID)
			if err != nil {
				writeWebhookError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, rotated)
		case "disable":
			disabled, err := manager.Disable(tenant, webhookID)
			if err != nil {
				writeWebhookError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, disabled)
		default:
			http.NotFound(w, r)
		}
		return
	}
	http.NotFound(w, r)
}

// handleWebhookTrigger is the inbound ingress, authenticated by the webhook signature header
// rather than a bearer principal. It resolves tenant from the signed endpoint.
func handleWebhookTrigger(manager *webhook.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "webhook manager is not configured")
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	webhookID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/triggers/webhook/"))
	if webhookID == "" {
		http.NotFound(w, r)
		return
	}
	payload, err := io.ReadAll(io.LimitReader(r.Body, webhook.MaxPayloadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "unable to read payload")
		return
	}
	record, triggerErr := manager.TriggerSigned(
		r.Context(),
		webhookID,
		r.Header.Get("X-Webhook-Signature"),
		r.Header.Get("X-Webhook-Idempotency-Key"),
		payload,
	)
	if triggerErr != nil {
		// The record carries the redacted outcome; map auth/quota/size failures to status codes.
		writeJSON(w, webhookTriggerStatusCode(triggerErr), record)
		return
	}
	writeJSON(w, http.StatusAccepted, record)
}

func webhookTriggerStatusCode(err error) int {
	switch {
	case errors.Is(err, webhook.ErrMissingAuth), errors.Is(err, webhook.ErrBadSignature), errors.Is(err, webhook.ErrCrossTenant):
		return http.StatusUnauthorized
	case errors.Is(err, webhook.ErrEndpointNotFound):
		return http.StatusNotFound
	case errors.Is(err, webhook.ErrDisabled):
		return http.StatusForbidden
	case errors.Is(err, webhook.ErrPayloadTooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, webhook.ErrQuotaDenied):
		return http.StatusTooManyRequests
	default:
		return http.StatusBadRequest
	}
}

func writeWebhookError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, webhook.ErrEndpointNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, webhook.ErrCrossTenant):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, webhook.ErrInvalidEndpoint):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
