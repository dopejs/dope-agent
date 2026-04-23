package api

import (
	"net/http"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/delivery"
)

func handleDeliveryTargets(manager *delivery.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "delivery manager is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := manager.ListTargets(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, DeliveryTargetListResponse{Items: items})
	case http.MethodPost:
		var input CreateDeliveryTargetRequest
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		target, err := manager.CreateTarget(r.Context(), delivery.DeliveryTarget{
			TargetID:         input.TargetID,
			DisplayName:      input.DisplayName,
			TargetKind:       input.TargetKind,
			ConnectorBinding: input.ConnectorBinding,
			AddressSummary:   input.AddressSummary,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, target)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleDeliveryTargetRoutes(manager *delivery.Manager, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/delivery/targets/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		target, ok, err := manager.GetTarget(r.Context(), parts[0])
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, target)
	case len(parts) == 2 && parts[1] == "activate" && r.Method == http.MethodPost:
		target, ok, err := manager.UpdateTargetStatus(r.Context(), parts[0], delivery.TargetStatusActive)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, target)
	case len(parts) == 2 && parts[1] == "disable" && r.Method == http.MethodPost:
		target, ok, err := manager.UpdateTargetStatus(r.Context(), parts[0], delivery.TargetStatusDisabled)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, target)
	default:
		http.NotFound(w, r)
	}
}

func handleDeliveryPreferences(manager *delivery.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "delivery manager is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := manager.ListPreferences(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, DeliveryPreferenceListResponse{Items: items})
	case http.MethodPost:
		var input UpsertDeliveryPreferenceRequest
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		pref, err := manager.UpsertPreference(r.Context(), delivery.DeliveryPreference{
			PreferenceID:            input.PreferenceID,
			EnvironmentScope:        input.EnvironmentScope,
			ScopeKind:               input.ScopeKind,
			IntegrationID:           input.IntegrationID,
			PreferredTargetsByClass: input.PreferredTargetsByClass,
			SummaryPolicy:           input.SummaryPolicy,
			SuppressionPolicy:       input.SuppressionPolicy,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, pref)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleDeliveryPreferenceRoutes(manager *delivery.Manager, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/delivery/preferences/")
	if path == "" || r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	item, ok, err := manager.GetPreference(r.Context(), path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func handleDeliveries(manager *delivery.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "delivery manager is not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	items, err := manager.ListOutcomes(r.Context(), delivery.OutcomeFilter{
		SourceKind:    strings.TrimSpace(r.URL.Query().Get("sourceKind")),
		SourceID:      strings.TrimSpace(r.URL.Query().Get("sourceId")),
		RunID:         strings.TrimSpace(r.URL.Query().Get("runId")),
		WorkflowID:    strings.TrimSpace(r.URL.Query().Get("workflowId")),
		ScheduleID:    strings.TrimSpace(r.URL.Query().Get("scheduleId")),
		IntegrationID: strings.TrimSpace(r.URL.Query().Get("integrationId")),
		Status:        delivery.OutcomeStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
		TargetID:      strings.TrimSpace(r.URL.Query().Get("targetId")),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items, err = projectDeliveryOutcomesCalendarLinkage(r.Context(), manager.Store(), items)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, DeliveryOutcomeListResponse{Items: items})
}

func handleDeliveryRoutes(manager *delivery.Manager, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/deliveries/")
	if path == "" || r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	item, ok, err := manager.GetOutcome(r.Context(), path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	item, err = projectDeliveryOutcomeCalendarLinkage(r.Context(), manager.Store(), item)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func handleDeliveryWindows(manager *delivery.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "delivery manager is not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	items, err := manager.ListSummaryWindows(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, DeliverySummaryWindowListResponse{Items: items})
}

func handleDeliveryWindowRoutes(manager *delivery.Manager, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/delivery/windows/")
	if path == "" || r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	item, ok, err := manager.GetSummaryWindow(r.Context(), path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
