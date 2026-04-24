package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func handleOperatorOnboarding(
	cfg config.Config,
	_ *auth.Manager,
	providerManager *providers.Manager,
	integrationsManager *integrations.Manager,
	connectorSupervisor *connectors.Supervisor,
	capabilitySupervisor *capabilities.Supervisor,
	runtimeManager *runtime.Manager,
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	token, ok := authenticatedToken(r.Context())
	builder := newOperatorProjectionBuilder(cfg, nil, providerManager, integrationsManager, connectorSupervisor, capabilitySupervisor, nil, runtimeManager, nil, nil, nil)
	writeJSON(w, http.StatusOK, builder.buildOnboarding(token, ok))
}

func handleOperatorActivity(
	cfg config.Config,
	policyEngine *policy.Engine,
	runtimeManager *runtime.Manager,
	schedulerManager *scheduler.Scheduler,
	deliveryManager *delivery.Manager,
	sqliteStore *store.SQLiteStore,
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	builder := newOperatorProjectionBuilder(cfg, sqliteStore, nil, nil, nil, nil, policyEngine, runtimeManager, schedulerManager, deliveryManager, nil)
	response, err := builder.buildActivity(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sourceKind := strings.TrimSpace(r.URL.Query().Get("sourceKind"))
	attentionOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("attentionOnly")), "true")
	limit := parseOperatorLimit(r.URL.Query().Get("limit"), 25)
	filtered := make([]OperatorActivityRecord, 0, len(response.Items))
	for _, item := range response.Items {
		if sourceKind != "" && item.SourceKind != sourceKind {
			continue
		}
		if attentionOnly && item.AttentionLevel == "info" {
			continue
		}
		filtered = append(filtered, item)
		if len(filtered) >= limit {
			break
		}
	}
	response.Items = filtered
	writeJSON(w, http.StatusOK, response)
}

func handleOperatorDiagnostics(
	cfg config.Config,
	providerManager *providers.Manager,
	integrationsManager *integrations.Manager,
	connectorSupervisor *connectors.Supervisor,
	capabilitySupervisor *capabilities.Supervisor,
	policyEngine *policy.Engine,
	runtimeManager *runtime.Manager,
	schedulerManager *scheduler.Scheduler,
	deliveryManager *delivery.Manager,
	computerUseManager *computeruse.Manager,
	sqliteStore *store.SQLiteStore,
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	builder := newOperatorProjectionBuilder(cfg, sqliteStore, providerManager, integrationsManager, connectorSupervisor, capabilitySupervisor, policyEngine, runtimeManager, schedulerManager, deliveryManager, computerUseManager)
	response, err := builder.buildDiagnostics(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sourceKind := strings.TrimSpace(r.URL.Query().Get("sourceKind"))
	plane := strings.TrimSpace(r.URL.Query().Get("plane"))
	severity := strings.TrimSpace(r.URL.Query().Get("severity"))
	filtered := make([]OperatorDiagnosticFinding, 0, len(response.Items))
	for _, item := range response.Items {
		if sourceKind != "" && item.SourceKind != sourceKind {
			continue
		}
		if plane != "" && item.Plane != plane {
			continue
		}
		if severity != "" && item.Severity != severity {
			continue
		}
		filtered = append(filtered, item)
	}
	response.Items = filtered
	writeJSON(w, http.StatusOK, response)
}

func parseOperatorLimit(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	if value > 200 {
		return 200
	}
	return value
}
