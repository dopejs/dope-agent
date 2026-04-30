package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/opsreadiness"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func handleIntegrationDiagnostics(manager *integrations.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, integrationID string, nested []string) {
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "diagnostic store is not configured")
		return
	}
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "integrations manager is not configured")
		return
	}
	if len(nested) == 1 {
		switch r.Method {
		case http.MethodGet:
			handleIntegrationDiagnosticList(manager, eventBus, sqliteStore, w, r, integrationID)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	if len(nested) == 2 && nested[1] == "runs" {
		switch r.Method {
		case http.MethodPost:
			handleCreateIntegrationDiagnosticRun(manager, eventBus, sqliteStore, w, r, integrationID)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	http.NotFound(w, r)
}

func handleIntegrationDiagnosticList(manager *integrations.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, integrationID string) {
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok || tenantContext.TenantID == "" {
		writeCredentialDenial(w, http.StatusForbidden, "tenant_context_missing")
		return
	}
	if !identity.HasPermission(tenantContext.Permissions, identity.PermissionIntegrationDiagnosticsRead) {
		recordIntegrationDiagnosticAudit(r.Context(), sqliteStore, audit.IntegrationDiagnosticAuditInput{
			TenantID:    tenantContext.TenantID,
			PrincipalID: tenantContext.PrincipalID,
			Action:      "diagnostic_read.denied",
			TargetKind:  "integration",
			TargetID:    integrationID,
			Outcome:     identity.AuditOutcomeDenied,
			ReasonCode:  integrations.ReasonOperatorActionNeeded,
		})
		writeCredentialDenial(w, http.StatusForbidden, "permission_missing")
		return
	}
	resource, ok := manager.GetForTenant(integrationID, tenantContext.TenantID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	items, err := sqliteStore.LatestIntegrationDiagnosticResults(r.Context(), integrations.DiagnosticResultFilter{
		TenantID:       tenantContext.TenantID,
		IntegrationID:  integrationID,
		Limit:          parseIntDefault(r.URL.Query().Get("limit"), 50),
		IncludeExpired: false,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(items) == 0 || strings.EqualFold(r.URL.Query().Get("forceRefresh"), "true") {
		result := integrations.NewDiagnosticManager().Inspect(integrations.DiagnosticInspectionInput{
			Resource:     resource,
			Capability:   r.URL.Query().Get("capability"),
			CheckedAt:    time.Now().UTC(),
			EvidenceText: resource.ReadinessReason,
		})
		if err := sqliteStore.SaveIntegrationDiagnosticResult(r.Context(), result); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		publishIntegrationDiagnosticResultEventsAndAudit(r, eventBus, sqliteStore, result, tenantContext, "diagnostic_state.inspected")
		items = []integrations.DiagnosticResult{result}
	}
	writeJSON(w, http.StatusOK, IntegrationDiagnosticListResponse{
		IntegrationID:    integrationID,
		TenantID:         tenantContext.TenantID,
		FreshnessSummary: "latest diagnostic state",
		Items:            items,
	})
}

func handleCreateIntegrationDiagnosticRun(manager *integrations.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, integrationID string) {
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok || tenantContext.TenantID == "" {
		writeCredentialDenial(w, http.StatusForbidden, "tenant_context_missing")
		return
	}
	if !identity.HasPermission(tenantContext.Permissions, identity.PermissionIntegrationDiagnosticsRun) {
		recordIntegrationDiagnosticAudit(r.Context(), sqliteStore, audit.IntegrationDiagnosticAuditInput{
			TenantID:    tenantContext.TenantID,
			PrincipalID: tenantContext.PrincipalID,
			Action:      "diagnostic_run.denied",
			TargetKind:  "integration",
			TargetID:    integrationID,
			Outcome:     identity.AuditOutcomeDenied,
			ReasonCode:  integrations.ReasonOperatorActionNeeded,
		})
		writeCredentialDenial(w, http.StatusForbidden, "permission_missing")
		return
	}
	resource, ok := manager.GetForTenant(integrationID, tenantContext.TenantID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var input CreateIntegrationDiagnosticRunRequest
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(input.ClientKey) == "" {
		writeError(w, http.StatusBadRequest, "clientKey is required")
		return
	}
	now := time.Now().UTC()
	diagnosticManager := integrations.NewDiagnosticManager()
	run := diagnosticManager.CreateRun(integrations.DiagnosticRunInput{
		Resource:     resource,
		RequestedBy:  tenantContext.PrincipalID,
		ClientKey:    input.ClientKey,
		Capabilities: input.Capabilities,
		Trigger:      "operator_inspection",
		StartedAt:    now,
	})
	if err := sqliteStore.SaveIntegrationDiagnosticRun(r.Context(), run); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if eventBus != nil {
		eventBus.Publish(events.IntegrationDiagnosticRunEvent(events.IntegrationDiagnosticRunStartedName, run))
	}
	recordIntegrationDiagnosticAudit(r.Context(), sqliteStore, audit.IntegrationDiagnosticAuditInput{
		TenantID:        run.TenantID,
		PrincipalID:     tenantContext.PrincipalID,
		Action:          "diagnostic_run.started",
		TargetKind:      "integration",
		TargetID:        integrationID,
		Outcome:         identity.AuditOutcomeSucceeded,
		ReasonCode:      integrations.ReasonOperatorActionNeeded,
		DiagnosticRunID: run.DiagnosticRunID,
		RedactionStatus: run.RedactionStatus,
	})
	results := make([]integrations.DiagnosticResult, 0, len(run.CheckedCapabilities))
	for _, capability := range run.CheckedCapabilities {
		result := inspectDiagnosticCapability(manager, diagnosticManager, resource, integrations.DiagnosticInspectionInput{
			Resource:     resource,
			Capability:   capability,
			RunID:        run.DiagnosticRunID,
			CheckedAt:    now,
			EvidenceText: firstNonEmpty(resource.ReadinessReason, input.Reason),
		})
		if err := sqliteStore.SaveIntegrationDiagnosticResult(r.Context(), result); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		results = append(results, result)
		publishIntegrationDiagnosticResultEventsAndAudit(r, eventBus, sqliteStore, result, tenantContext, "diagnostic_state.changed")
	}
	run = integrations.CompleteDiagnosticRun(run, results, now)
	if err := sqliteStore.SaveIntegrationDiagnosticRun(r.Context(), run); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if eventBus != nil {
		eventBus.Publish(events.IntegrationDiagnosticRunEvent(events.IntegrationDiagnosticRunCompletedName, run))
	}
	recordIntegrationDiagnosticAudit(r.Context(), sqliteStore, audit.IntegrationDiagnosticAuditInput{
		TenantID:        run.TenantID,
		PrincipalID:     tenantContext.PrincipalID,
		Action:          "diagnostic_run.completed",
		TargetKind:      "integration",
		TargetID:        integrationID,
		Outcome:         identity.AuditOutcomeSucceeded,
		ReasonCode:      run.FailureReasonCode,
		DiagnosticRunID: run.DiagnosticRunID,
		RedactionStatus: run.RedactionStatus,
	})
	writeJSON(w, http.StatusCreated, run)
}

func handleIntegrationDiagnosticSmoke(manager *integrations.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if manager == nil || sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "diagnostic dependencies are not configured")
		return
	}
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok || tenantContext.TenantID == "" {
		writeCredentialDenial(w, http.StatusForbidden, "tenant_context_missing")
		return
	}
	if !identity.HasPermission(tenantContext.Permissions, identity.PermissionIntegrationDiagnosticsSmoke) {
		recordIntegrationDiagnosticAudit(r.Context(), sqliteStore, audit.IntegrationDiagnosticAuditInput{
			TenantID:    tenantContext.TenantID,
			PrincipalID: tenantContext.PrincipalID,
			Action:      "diagnostic_smoke.denied",
			TargetKind:  "integration",
			Outcome:     identity.AuditOutcomeDenied,
			ReasonCode:  integrations.ReasonOperatorActionNeeded,
		})
		writeCredentialDenial(w, http.StatusForbidden, "permission_missing")
		return
	}
	var input CreateIntegrationDiagnosticSmokeRequest
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if smokeRequestContainsRiskyProbe(input) && !identity.HasPermission(tenantContext.Permissions, identity.PermissionIntegrationDiagnosticsSmokeRisky) {
		recordIntegrationDiagnosticAudit(r.Context(), sqliteStore, audit.IntegrationDiagnosticAuditInput{
			TenantID:    tenantContext.TenantID,
			PrincipalID: tenantContext.PrincipalID,
			Action:      "diagnostic_smoke_risky.denied",
			TargetKind:  "integration",
			TargetID:    strings.TrimSpace(input.IntegrationID),
			Outcome:     identity.AuditOutcomeDenied,
			ReasonCode:  integrations.ReasonUnsafeToRetry,
		})
		writeCredentialDenial(w, http.StatusForbidden, "permission_missing")
		return
	}
	reportID := strings.TrimSpace(input.ReportID)
	if reportID == "" {
		reportID = "smoke_" + strconv.FormatInt(time.Now().UTC().UnixNano(), 36)
	}
	probes, resources, ok := buildSmokeProbeInputs(manager, tenantContext, input, w, r)
	if !ok {
		return
	}
	now := time.Now().UTC()
	report := opsreadiness.BuildIntegrationDiagnosticSmokeReport(reportID, tenantContext.PrincipalID, probes, now)
	publishedAt := now
	report.PublishedAt = &publishedAt
	if err := sqliteStore.SaveSmokeMatrixReport(r.Context(), report); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	diagnosticManager := integrations.NewDiagnosticManager()
	for idx, outcome := range report.ProbeOutcomes {
		resource := resources[idx]
		result := diagnosticManager.Inspect(integrations.DiagnosticInspectionInput{
			Resource:     resource,
			Capability:   outcome.ProbeAction,
			CheckedAt:    outcome.CheckedAt,
			EvidenceText: outcome.ReasonCode,
		})
		reason := integrations.DiagnosticReasonCode(outcome.ReasonCode)
		status, owner, retrySafety := integrations.DiagnosticDefaults(reason)
		result.Status = status
		result.ReasonCode = reason
		result.RemediationOwner = owner
		result.RetrySafety = retrySafety
		result.RemediationHint = integrations.DiagnosticRemediationHint(reason)
		result.SmokeReportID = report.SmokeReportID
		result.ArtifactRefs = append([]string(nil), outcome.ArtifactRefs...)
		if err := sqliteStore.SaveIntegrationDiagnosticResult(r.Context(), result); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		publishIntegrationDiagnosticResultEventsAndAudit(r, eventBus, sqliteStore, result, tenantContext, "diagnostic_smoke.probe_recorded")
	}
	if eventBus != nil {
		eventBus.Publish(events.IntegrationDiagnosticSmokeCompletedEvent(report))
	}
	recordIntegrationDiagnosticAudit(r.Context(), sqliteStore, audit.IntegrationDiagnosticAuditInput{
		TenantID:        report.TenantID,
		PrincipalID:     tenantContext.PrincipalID,
		Action:          "diagnostic_smoke.published",
		TargetKind:      "integration",
		TargetID:        strings.TrimSpace(input.IntegrationID),
		Outcome:         identity.AuditOutcomeSucceeded,
		ReasonCode:      integrations.ReasonHealthy,
		SmokeReportID:   report.SmokeReportID,
		RedactionStatus: integrations.RedactionStatusRedacted,
	})
	writeJSON(w, http.StatusCreated, report)
}

func buildSmokeProbeInputs(manager *integrations.Manager, tenantContext identity.TenantContext, input CreateIntegrationDiagnosticSmokeRequest, w http.ResponseWriter, r *http.Request) ([]opsreadiness.SmokeProbeInput, []integrations.Resource, bool) {
	probeRequests := input.Probes
	if len(probeRequests) == 0 {
		probeRequests = []CreateIntegrationDiagnosticSmokeProbe{{IntegrationID: input.IntegrationID, Supported: true, ProviderAvailable: true, SafeCredentialsAvailable: true, TenantApprovalAvailable: true, ReadOnlyOrReversible: true}}
	}
	probes := make([]opsreadiness.SmokeProbeInput, 0, len(probeRequests))
	resources := make([]integrations.Resource, 0, len(probeRequests))
	for _, probeRequest := range probeRequests {
		integrationID := firstNonEmpty(strings.TrimSpace(probeRequest.IntegrationID), strings.TrimSpace(input.IntegrationID))
		resource, ok := manager.GetForTenant(integrationID, tenantContext.TenantID)
		if !ok {
			http.NotFound(w, r)
			return nil, nil, false
		}
		reason := integrations.DiagnosticReasonCode(strings.TrimSpace(probeRequest.ReasonCode))
		artifactRefs := append([]string(nil), probeRequest.ArtifactRefs...)
		if shouldRunSmokeProbe(probeRequest) {
			probeKind := integrations.ProbeKindInspect
			if !probeRequest.ReadOnlyOrReversible {
				probeKind = integrations.ProbeKindMutate
			}
			probeInput := map[string]any{"probeAction": probeRequest.ProbeAction, "operationClass": probeRequest.ProbeAction}
			if len(probeRequest.ProviderEvidence) > 0 {
				probeInput["providerEvidence"] = probeRequest.ProviderEvidence
			}
			_, result, _, err := manager.RunProbe(resource.IntegrationID, probeKind, probeInput)
			switch {
			case err == nil:
				if reason == "" {
					reason = diagnosticReasonFromProbeResult(result)
				}
				artifactRefs = append(artifactRefs, "probe:"+resource.IntegrationID+":"+string(probeKind))
			case integrations.IsUnavailableProbeError(err):
				reason = integrations.ReasonOperatorActionNeeded
			default:
				reason = integrations.ReasonUnsupportedDiagnostic
			}
		}
		probes = append(probes, opsreadiness.SmokeProbeInput{
			TenantID:                 tenantContext.TenantID,
			IntegrationID:            resource.IntegrationID,
			IntegrationAccountID:     resource.AccountBinding.AccountKey,
			DomainKind:               firstNonEmpty(strings.TrimSpace(probeRequest.DomainKind), resource.DomainKind),
			ProviderKind:             string(resource.BackendBinding.BackendKind),
			ProbeAction:              strings.TrimSpace(probeRequest.ProbeAction),
			RequestedBy:              tenantContext.PrincipalID,
			SafeCredentialsAvailable: probeRequest.SafeCredentialsAvailable,
			TenantApprovalAvailable:  probeRequest.TenantApprovalAvailable,
			ProviderAvailable:        probeRequest.ProviderAvailable,
			Supported:                probeRequest.Supported,
			ReadOnlyOrReversible:     probeRequest.ReadOnlyOrReversible,
			TenantAdminApproved:      probeRequest.TenantAdminApproved,
			OperatorApproved:         probeRequest.OperatorApproved,
			OperatorDeferred:         probeRequest.OperatorDeferred,
			ReasonCode:               reason,
			ArtifactRefs:             artifactRefs,
		})
		resources = append(resources, resource)
	}
	return probes, resources, true
}

func inspectDiagnosticCapability(manager *integrations.Manager, diagnosticManager *integrations.DiagnosticManager, resource integrations.Resource, input integrations.DiagnosticInspectionInput) integrations.DiagnosticResult {
	result := diagnosticManager.Inspect(input)
	if manager == nil || !resource.BackendBinding.SupportsProbeRead {
		return result
	}
	probeInput := map[string]any{"operationClass": input.Capability}
	if strings.TrimSpace(input.EvidenceText) != "" {
		probeInput["providerEvidence"] = map[string]any{
			"code":    input.EvidenceText,
			"message": input.EvidenceText,
		}
	}
	_, probeResult, _, err := manager.RunProbe(resource.IntegrationID, integrations.ProbeKindInspect, probeInput)
	if err != nil {
		return result
	}
	reason := diagnosticReasonFromProbeResult(probeResult)
	status, owner, retrySafety := integrations.DiagnosticDefaults(reason)
	result.Status = status
	result.ReasonCode = reason
	result.RemediationOwner = owner
	result.RetrySafety = retrySafety
	result.RemediationHint = integrations.DiagnosticRemediationHint(reason)
	result.EvidenceSummary = string(reason)
	if redactionStatus, ok := probeResult.ResultSummary["redactionStatus"].(string); ok && redactionStatus != "" {
		result.RedactionStatus = integrations.RedactionStatus(redactionStatus)
	}
	return result
}

func diagnosticReasonFromProbeResult(result integrations.ProbeResult) integrations.DiagnosticReasonCode {
	if result.ResultSummary != nil {
		if raw, ok := result.ResultSummary["reasonCode"].(string); ok && strings.TrimSpace(raw) != "" {
			return integrations.DiagnosticReasonCode(strings.TrimSpace(raw))
		}
	}
	if strings.TrimSpace(result.FailureClass) != "" {
		return integrations.ClassifyProviderEvidence(integrations.ProviderDiagnosticEvidence{
			ProviderErrorClass: result.FailureClass,
			Message:            result.FailureClass,
		}).ReasonCode
	}
	return integrations.ReasonHealthy
}

func shouldRunSmokeProbe(probe CreateIntegrationDiagnosticSmokeProbe) bool {
	return !probe.OperatorDeferred &&
		probe.Supported &&
		probe.SafeCredentialsAvailable &&
		probe.TenantApprovalAvailable &&
		probe.ProviderAvailable &&
		(probe.ReadOnlyOrReversible || (probe.TenantAdminApproved && probe.OperatorApproved))
}

func smokeRequestContainsRiskyProbe(input CreateIntegrationDiagnosticSmokeRequest) bool {
	for _, probe := range input.Probes {
		if !probe.ReadOnlyOrReversible {
			return true
		}
	}
	return false
}

func handleIntegrationDiagnosticRuns(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, nested []string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok || tenantContext.TenantID == "" {
		writeCredentialDenial(w, http.StatusForbidden, "tenant_context_missing")
		return
	}
	if !identity.HasPermission(tenantContext.Permissions, identity.PermissionIntegrationDiagnosticsRead) {
		writeCredentialDenial(w, http.StatusForbidden, "permission_missing")
		return
	}
	if len(nested) == 1 {
		items, err := sqliteStore.ListIntegrationDiagnosticRuns(r.Context(), integrations.DiagnosticRunFilter{
			TenantID:       tenantContext.TenantID,
			IntegrationID:  r.URL.Query().Get("integrationId"),
			ProviderKind:   r.URL.Query().Get("providerKind"),
			DomainKind:     r.URL.Query().Get("domainKind"),
			Status:         integrations.DiagnosticRunStatus(r.URL.Query().Get("status")),
			ReasonCode:     integrations.DiagnosticReasonCode(r.URL.Query().Get("reasonCode")),
			Limit:          parseIntDefault(r.URL.Query().Get("limit"), 50),
			IncludeExpired: false,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, IntegrationDiagnosticRunListResponse{Items: items})
		return
	}
	if len(nested) == 2 {
		item, ok, err := sqliteStore.GetIntegrationDiagnosticRun(r.Context(), tenantContext.TenantID, nested[1], false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	http.NotFound(w, r)
}

func handleIntegrationDiagnosticRetentionApply(eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "diagnostic store is not configured")
		return
	}
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok || tenantContext.TenantID == "" {
		writeCredentialDenial(w, http.StatusForbidden, "tenant_context_missing")
		return
	}
	if !identity.HasPermission(tenantContext.Permissions, identity.PermissionIntegrationDiagnosticsRun) {
		recordIntegrationDiagnosticAudit(r.Context(), sqliteStore, audit.IntegrationDiagnosticAuditInput{
			TenantID:    tenantContext.TenantID,
			PrincipalID: tenantContext.PrincipalID,
			Action:      "diagnostic_retention.denied",
			Outcome:     identity.AuditOutcomeDenied,
			ReasonCode:  integrations.ReasonOperatorActionNeeded,
		})
		writeCredentialDenial(w, http.StatusForbidden, "permission_missing")
		return
	}
	items, err := sqliteStore.ApplyExpiredDiagnosticRetentionRecords(r.Context(), tenantContext.TenantID, time.Now().UTC(), parseIntDefault(r.URL.Query().Get("limit"), 50))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, item := range items {
		if eventBus != nil {
			eventBus.Publish(events.IntegrationDiagnosticRetentionAppliedEvent(item))
		}
		recordIntegrationDiagnosticAudit(r.Context(), sqliteStore, audit.IntegrationDiagnosticAuditInput{
			TenantID:        item.TenantID,
			PrincipalID:     tenantContext.PrincipalID,
			Action:          "diagnostic_retention.applied",
			TargetKind:      item.TargetKind,
			TargetID:        item.TargetID,
			Outcome:         identity.AuditOutcomeSucceeded,
			ReasonCode:      integrations.ReasonOperatorActionNeeded,
			RedactionStatus: integrations.RedactionStatusRedacted,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleIntegrationDiagnosticReasonCodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": integrations.DefaultDiagnosticReasonCodeCatalog()})
}

func publishIntegrationDiagnosticResultEventsAndAudit(r *http.Request, eventBus *events.Bus, sqliteStore *store.SQLiteStore, result integrations.DiagnosticResult, tenantContext identity.TenantContext, action string) {
	if eventBus != nil {
		eventBus.Publish(events.IntegrationDiagnosticStateChangedEvent(result, integrations.DiagnosticStatusUnknown))
		if result.RedactionStatus == integrations.RedactionStatusFailedClosed {
			eventBus.Publish(events.IntegrationDiagnosticRedactionFailedEvent(result))
		}
	}
	outcome := identity.AuditOutcomeSucceeded
	if result.RedactionStatus == integrations.RedactionStatusFailedClosed {
		outcome = identity.AuditOutcomeFailedClosed
	}
	recordIntegrationDiagnosticAudit(r.Context(), sqliteStore, audit.IntegrationDiagnosticAuditInput{
		TenantID:        result.TenantID,
		PrincipalID:     tenantContext.PrincipalID,
		Action:          action,
		TargetKind:      "integration_diagnostic_result",
		TargetID:        result.DiagnosticResultID,
		Outcome:         outcome,
		ReasonCode:      result.ReasonCode,
		DiagnosticRunID: result.RunID,
		SmokeReportID:   result.SmokeReportID,
		RedactionStatus: result.RedactionStatus,
	})
}

func recordIntegrationDiagnosticAudit(ctx context.Context, sqliteStore *store.SQLiteStore, input audit.IntegrationDiagnosticAuditInput) {
	if sqliteStore == nil {
		return
	}
	_, _ = sqliteStore.AppendTenantAuditEvent(ctx, audit.BuildIntegrationDiagnosticAuditEvent(input))
}

func parseIntDefault(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
