package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/store/tenancy"
)

func handleIntegrations(cfg config.Config, manager *integrations.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "integrations manager is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if tenantContext, ok := tenantContextFromContext(r.Context()); ok && tenantContext.TenantID != "" {
			if _, reason := requireHostedCredentialReadAny(r, identity.PermissionIntegrationsManage); reason != "" {
				writeCredentialDenial(w, http.StatusForbidden, reason)
				return
			}
			writeJSON(w, http.StatusOK, IntegrationListResponse{Items: manager.ListForTenant(tenantContext.TenantID)})
			return
		}
		writeJSON(w, http.StatusOK, IntegrationListResponse{Items: manager.List()})
	case http.MethodPost:
		tenantID := ""
		if tenantContext, ok := tenantContextFromContext(r.Context()); ok && tenantContext.TenantID != "" {
			if _, reason := requireHostedCredentialPermission(r, identity.PermissionIntegrationsManage, ""); reason != "" {
				writeCredentialDenial(w, http.StatusForbidden, reason)
				return
			}
			tenantID = tenantContext.TenantID
		}
		var input CreateIntegrationRequest
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		item, err := manager.Create(integrations.CreateInput{
			TenantID:         tenantID,
			IntegrationID:    input.IntegrationID,
			DomainKind:       input.DomainKind,
			DisplayName:      input.DisplayName,
			AccountBinding:   input.AccountBinding,
			CanonicalDefault: input.CanonicalDefault,
			EnvironmentScope: string(cfg.Environment),
			BackendBinding: integrations.BackendBinding{
				BackendKind:           input.BackendKind,
				BackendRefID:          input.BackendRefID,
				BackendDisplayName:    input.BackendDisplayName,
				SupportsProbeRead:     input.BackendKind == integrations.BackendKindFakeLocal,
				SupportsProbeMutation: input.BackendKind == integrations.BackendKindFakeLocal,
			},
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := persistIntegration(r.Context(), sqliteStore, item); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
			Category: "integration",
			Name:     "integration.registered",
			Resource: events.Resource{Kind: "integration", ID: item.IntegrationID},
			Payload: map[string]any{
				"integrationId":    item.IntegrationID,
				"domainKind":       item.DomainKind,
				"displayName":      item.DisplayName,
				"environmentScope": item.EnvironmentScope,
				"readinessStatus":  item.ReadinessStatus,
				"canonicalDefault": item.CanonicalDefault,
				"backendKind":      item.BackendBinding.BackendKind,
				"accountKey":       item.AccountBinding.AccountKey,
			},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleIntegrationRoutes(cfg config.Config, manager *integrations.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "integrations manager is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/integrations/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) == 1 && r.Method == http.MethodGet {
		var (
			item integrations.Resource
			ok   bool
		)
		if tenantContext, tenantOK := tenantContextFromContext(r.Context()); tenantOK && tenantContext.TenantID != "" {
			if _, reason := requireHostedCredentialReadAny(r, identity.PermissionIntegrationsManage); reason != "" {
				writeCredentialDenial(w, http.StatusForbidden, reason)
				return
			}
			item, ok = manager.GetForTenant(parts[0], tenantContext.TenantID)
		} else {
			item, ok = manager.Get(parts[0])
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodDelete {
		handleIntegrationDisconnect(manager, eventBus, sqliteStore, w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "readiness" {
		handleIntegrationReadiness(manager, eventBus, sqliteStore, w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "default" {
		handleIntegrationDefault(manager, eventBus, sqliteStore, w, r, parts[0])
		return
	}
	http.NotFound(w, r)
}

func handleIntegrationReadiness(manager *integrations.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, integrationID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if tenantContext, ok := tenantContextFromContext(r.Context()); ok && tenantContext.TenantID != "" {
		if _, reason := requireHostedCredentialPermission(r, identity.PermissionIntegrationsManage, ""); reason != "" {
			writeCredentialDenial(w, http.StatusForbidden, reason)
			return
		}
		if _, ok := manager.GetForTenant(integrationID, tenantContext.TenantID); !ok {
			http.NotFound(w, r)
			return
		}
	} else if _, ok := manager.Get(integrationID); !ok {
		http.NotFound(w, r)
		return
	}
	var input ReportIntegrationReadinessRequest
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	readinessInput := integrations.UpdateReadinessInput{
		ReadinessStatus:        input.ReadinessStatus,
		AuthState:              input.AuthState,
		HealthState:            input.HealthState,
		ReadinessReason:        input.Reason,
		RequiredOperatorAction: input.RequiredOperatorAction,
		AccountBinding:         input.AccountBinding,
		SecretResolution:       input.SecretResolution,
	}
	var item integrations.Resource
	var err error
	if tenantContext, ok := tenantContextFromContext(r.Context()); ok && tenantContext.TenantID != "" {
		item, err = manager.UpdateReadinessForTenant(integrationID, tenantContext.TenantID, readinessInput)
	} else {
		item, err = manager.UpdateReadiness(integrationID, readinessInput)
	}
	if err != nil {
		switch {
		case errors.Is(err, integrations.ErrIntegrationNotFound):
			http.NotFound(w, r)
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	if err := persistIntegration(r.Context(), sqliteStore, item); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "integration",
		Name:     "integration.updated",
		Resource: events.Resource{Kind: "integration", ID: item.IntegrationID},
		Payload: map[string]any{
			"integrationId":    item.IntegrationID,
			"domainKind":       item.DomainKind,
			"displayName":      item.DisplayName,
			"environmentScope": item.EnvironmentScope,
			"readinessStatus":  item.ReadinessStatus,
			"canonicalDefault": item.CanonicalDefault,
			"backendKind":      item.BackendBinding.BackendKind,
			"accountKey":       item.AccountBinding.AccountKey,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "integration",
		Name:     "integration.readiness_changed",
		Resource: events.Resource{Kind: "integration", ID: item.IntegrationID},
		Payload: map[string]any{
			"integrationId":          item.IntegrationID,
			"readinessStatus":        item.ReadinessStatus,
			"authState":              item.AuthState,
			"healthState":            item.HealthState,
			"reason":                 item.ReadinessReason,
			"requiredOperatorAction": item.RequiredOperatorAction,
			"accountKey":             item.AccountBinding.AccountKey,
			"backendKind":            item.BackendBinding.BackendKind,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func handleIntegrationDefault(manager *integrations.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, integrationID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tenantID := ""
	if tenantContext, ok := tenantContextFromContext(r.Context()); ok && tenantContext.TenantID != "" {
		if _, reason := requireHostedCredentialPermission(r, identity.PermissionIntegrationsManage, ""); reason != "" {
			writeCredentialDenial(w, http.StatusForbidden, reason)
			return
		}
		if _, ok := manager.GetForTenant(integrationID, tenantContext.TenantID); !ok {
			http.NotFound(w, r)
			return
		}
		tenantID = tenantContext.TenantID
	} else if _, ok := manager.Get(integrationID); !ok {
		http.NotFound(w, r)
		return
	}
	var item integrations.Resource
	var err error
	if tenantID != "" {
		item, err = manager.SetCanonicalDefaultForTenant(integrationID, tenantID)
	} else {
		item, err = manager.SetCanonicalDefault(integrationID)
	}
	if err != nil {
		switch {
		case errors.Is(err, integrations.ErrIntegrationNotFound):
			http.NotFound(w, r)
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	integrationsToPersist := manager.List()
	if tenantID != "" {
		integrationsToPersist = manager.ListForTenant(tenantID)
	}
	for _, integration := range integrationsToPersist {
		if integration.DomainKind == item.DomainKind && integration.EnvironmentScope == item.EnvironmentScope && integration.AccountBinding.AccountKey == item.AccountBinding.AccountKey {
			if err := persistIntegration(r.Context(), sqliteStore, integration); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "integration",
		Name:     "integration.updated",
		Resource: events.Resource{Kind: "integration", ID: item.IntegrationID},
		Payload: map[string]any{
			"integrationId":    item.IntegrationID,
			"domainKind":       item.DomainKind,
			"displayName":      item.DisplayName,
			"environmentScope": item.EnvironmentScope,
			"readinessStatus":  item.ReadinessStatus,
			"canonicalDefault": item.CanonicalDefault,
			"backendKind":      item.BackendBinding.BackendKind,
			"accountKey":       item.AccountBinding.AccountKey,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "integration",
		Name:     "integration.default_changed",
		Resource: events.Resource{Kind: "integration", ID: item.IntegrationID},
		Payload: map[string]any{
			"integrationId":    item.IntegrationID,
			"domainKind":       item.DomainKind,
			"environmentScope": item.EnvironmentScope,
			"accountKey":       item.AccountBinding.AccountKey,
			"canonicalDefault": item.CanonicalDefault,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func handleIntegrationDisconnect(manager *integrations.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, integrationID string) {
	reason := strings.TrimSpace(r.URL.Query().Get("reason"))
	if reason == "" {
		reason = "operator disconnected integration"
	}
	var item integrations.Resource
	var err error
	if tenantContext, ok := tenantContextFromContext(r.Context()); ok && tenantContext.TenantID != "" {
		if _, reasonCode := requireHostedCredentialPermission(r, identity.PermissionIntegrationsManage, ""); reasonCode != "" {
			writeCredentialDenial(w, http.StatusForbidden, reasonCode)
			return
		}
		item, err = manager.DisconnectForTenant(integrationID, tenantContext.TenantID, reason)
	} else {
		item, err = manager.Disconnect(integrationID, reason)
	}
	if err != nil {
		if errors.Is(err, integrations.ErrIntegrationNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := persistIntegration(r.Context(), sqliteStore, item); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.Event{
		Category: "integration",
		Name:     "integration.disconnected",
		Resource: events.Resource{Kind: "integration", ID: item.IntegrationID},
		Payload: map[string]any{
			"tenantId":        item.TenantID,
			"integrationId":   item.IntegrationID,
			"readinessStatus": item.ReadinessStatus,
			"authState":       item.AuthState,
			"healthState":     item.HealthState,
			"disabledReason":  item.DisabledReason,
		},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func handleRunIntegrationProbes(cfg config.Config, runtimeManager *runtime.Manager, policyEngine *policy.Engine, integrationsManager *integrations.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request, runID, integrationID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if integrationsManager == nil || runtimeManager == nil || policyEngine == nil {
		writeError(w, http.StatusInternalServerError, "integration probe dependencies are not configured")
		return
	}
	var input CreateIntegrationProbeRequest
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resource, ok := integrationsManager.Get(integrationID)
	if !ok || resource.EnvironmentScope != string(cfg.Environment) {
		http.NotFound(w, r)
		return
	}
	if _, ok := runtimeManager.GetRun(runID); !ok {
		http.NotFound(w, r)
		return
	}
	binding, err := integrationsManager.BindingSummary(integrationID, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.ProbeKind == integrations.ProbeKindMutate {
		if strings.TrimSpace(input.ApprovalID) == "" {
			approval, decision, err := policyEngine.RequestApproval(policy.RequestApprovalInput{
				Action:              "integration.probe.mutate",
				ResourceKind:        "integration",
				ResourceID:          integrationID,
				Reason:              "integration mutate probe requires approval",
				RequestedBy:         "run:" + runID,
				IntegrationBindings: []integrations.BindingSummary{binding},
			})
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := persistApproval(r.Context(), sqliteStore, approval); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if err := persistDecision(r.Context(), sqliteStore, decision); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusAccepted, IntegrationProbeResponse{
				RunID:               runID,
				Status:              "approval_pending",
				IntegrationBindings: []integrations.BindingSummary{binding},
				Approval:            &approval,
			})
			return
		}
		approval, ok := policyEngine.GetApproval(input.ApprovalID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		if approval.Action != "integration.probe.mutate" || approval.ResourceKind != "integration" || approval.ResourceID != integrationID {
			writeError(w, http.StatusBadRequest, "approval does not authorize this integration mutate probe")
			return
		}
		if approval.Status == policy.ApprovalStatusRejected {
			writeError(w, http.StatusConflict, "integration mutate probe approval was rejected")
			return
		}
		if approval.Status != policy.ApprovalStatusApproved {
			writeJSON(w, http.StatusAccepted, IntegrationProbeResponse{
				RunID:               runID,
				Status:              "approval_pending",
				IntegrationBindings: []integrations.BindingSummary{binding},
				Approval:            &approval,
			})
			return
		}
	}

	step, err := runtimeManager.CreateStep(runID, runtime.CreateStepInput{
		Title: "Integration probe " + integrationID,
		Kind:  "integration_probe",
		Input: map[string]any{
			"integrationId": integrationID,
			"probeKind":     input.ProbeKind,
		},
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, runUpdate, err := runtimeManager.UpdateStepStatusAndReconcileRun(runID, step.StepID, runtime.UpdateStepStatusInput{Status: runtime.StepStatusExecutingTool}); err == nil && runUpdate != nil {
		_ = persistRun(r.Context(), sqliteStore, *runUpdate)
	}
	if err := persistStep(r.Context(), sqliteStore, step); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resource, result, binding, err := integrationsManager.RunProbe(integrationID, input.ProbeKind, input.Input)
	if err != nil {
		if integrations.IsUnavailableProbeError(err) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	toolCall, err := runtimeManager.CreateToolCall(runID, step.StepID, runtime.CreateToolCallInput{
		CapabilityID:        "integration_probe",
		ToolName:            string(input.ProbeKind),
		Input:               map[string]any{"integrationId": integrationID, "probeKind": input.ProbeKind},
		IntegrationBindings: []integrations.BindingSummary{binding},
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := persistToolCall(r.Context(), sqliteStore, runtimeManager, toolCall); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishToolCallEvent(r.Context(), eventBus, sqliteStore, "tool_call.requested", runID, step.StepID, toolCall); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if result.FailureClass == "" {
		toolCall, err = runtimeManager.CompleteToolCall(runID, step.StepID, toolCall.ToolCallID, runtime.CompleteToolCallInput{
			Output: map[string]any{
				"resource": resource,
				"probe":    result.ResultSummary,
			},
		})
	} else {
		toolCall, err = runtimeManager.FailToolCall(runID, step.StepID, toolCall.ToolCallID, runtime.FailToolCallInput{
			Output:       result.ResultSummary,
			Error:        result.FailureClass,
			FailureClass: result.FailureClass,
		})
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	finalStepStatus := runtime.StepStatusCompleted
	if toolCall.Status == runtime.ToolCallStatusFailed {
		finalStepStatus = runtime.StepStatusFailed
	}
	if step, runUpdate, err := runtimeManager.UpdateStepStatusAndReconcileRun(runID, step.StepID, runtime.UpdateStepStatusInput{
		Status: finalStepStatus,
		Output: result.ResultSummary,
	}); err == nil {
		_ = persistStep(r.Context(), sqliteStore, step)
		if runUpdate != nil {
			_ = persistRun(r.Context(), sqliteStore, *runUpdate)
		}
	}
	if err := persistToolCall(r.Context(), sqliteStore, runtimeManager, toolCall); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishToolCallEvent(r.Context(), eventBus, sqliteStore, "tool_call.completed", runID, step.StepID, toolCall); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := persistCheckpoint(r.Context(), checkpointManager, runID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, IntegrationProbeResponse{
		RunID:               runID,
		StepID:              step.StepID,
		ToolCallID:          toolCall.ToolCallID,
		Status:              result.Status,
		IntegrationBindings: []integrations.BindingSummary{binding},
	})
}

func persistIntegration(ctx context.Context, sqliteStore *store.SQLiteStore, item integrations.Resource) error {
	if sqliteStore == nil {
		return nil
	}
	if tc, ok := tenantContextFromContext(ctx); ok && tc.TenantID != "" {
		item.TenantID = tc.TenantID
		return tenancy.NewIntegrations(sqliteStore, nil).UpsertIntegrationForTenant(ctx, item)
	}
	return sqliteStore.UpsertIntegration(ctx, item)
}
