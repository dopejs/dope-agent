package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type channelManagementActionRequest struct {
	ReasonCode              string                          `json:"reasonCode,omitempty"`
	Note                    string                          `json:"note,omitempty"`
	ActionKind              connectors.ManagementActionKind `json:"actionKind,omitempty"`
	SourceDiagnosticStateID string                          `json:"sourceDiagnosticStateId,omitempty"`
	EligibleSenders         []string                        `json:"eligibleSenders,omitempty"`
	EligibleConversations   []string                        `json:"eligibleConversations,omitempty"`
	EligibleRooms           []string                        `json:"eligibleRooms,omitempty"`
	EligibleChannels        []string                        `json:"eligibleChannels,omitempty"`
	InvocationGates         []string                        `json:"invocationGates,omitempty"`
	BackgroundDelivery      *bool                           `json:"backgroundDeliveryEligible,omitempty"`
}

func handleChannelManagementRoutes(supervisor *connectors.Supervisor, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if supervisor == nil {
		writeError(w, http.StatusInternalServerError, "connector supervisor is not configured")
		return
	}
	const prefix = "/v1/channel-management/connectors"
	if r.URL.Path == prefix {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleChannelManagementList(supervisor, sqliteStore, w, r)
		return
	}
	if !strings.HasPrefix(r.URL.Path, prefix+"/") {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, prefix+"/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	connectorID := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleChannelManagementDetail(supervisor, sqliteStore, w, r, connectorID)
		return
	}
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	switch parts[1] {
	case "diagnostics":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleChannelManagementDiagnostics(sqliteStore, w, r, connectorID)
	case "disable":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleChannelManagementDisable(supervisor, sqliteStore, w, r, connectorID)
	case "re-enable":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleChannelManagementReEnable(supervisor, sqliteStore, w, r, connectorID)
	case "repair-actions":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleChannelManagementRepair(supervisor, sqliteStore, w, r, connectorID)
	case "route-policy":
		handleChannelManagementRoutePolicy(supervisor, sqliteStore, w, r, connectorID)
	case "reply-outcomes":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleChannelManagementReplyOutcomes(sqliteStore, w, r, connectorID)
	case "delivery-outcomes":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleChannelManagementDeliveryOutcomes(sqliteStore, w, r, connectorID)
	case "support-evidence":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleChannelManagementSupportEvidence(supervisor, eventBus, sqliteStore, w, r, connectorID)
	default:
		http.NotFound(w, r)
	}
}

func handleChannelManagementList(supervisor *connectors.Supervisor, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireChannelManagementPermission(r, sqliteStore, "", identity.PermissionCredentialsInspect, "channel_management.list")
	if !ok {
		writeChannelManagementDenial(w)
		return
	}
	items := supervisor.ListForTenant(tenantContext.TenantID)
	now := time.Now().UTC()
	diagnosticsByConnector := map[string][]connectors.ConnectorDiagnosticState{}
	if sqliteStore != nil {
		for _, connector := range items {
			diagnostics, err := sqliteStore.ListConnectorDiagnosticStates(r.Context(), tenantContext.TenantID, connector.ConnectorID, now)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			diagnosticsByConnector[connector.ConnectorID] = diagnostics
		}
	}
	limit := parseChannelManagementLimit(r.URL.Query().Get("limit"))
	response := connectors.BuildConnectorPage(connectors.ProjectionInput{
		TenantID:    tenantContext.TenantID,
		Connectors:  items,
		Diagnostics: diagnosticsByConnector,
		Now:         now,
		Limit:       limit,
		Cursor:      r.URL.Query().Get("cursor"),
		StateFilter: r.URL.Query().Get("state"),
		KindFilter:  r.URL.Query().Get("kind"),
	})
	writeJSON(w, http.StatusOK, response)
}

func handleChannelManagementDetail(supervisor *connectors.Supervisor, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, connectorID string) {
	tenantContext, ok := requireChannelManagementPermission(r, sqliteStore, connectorID, identity.PermissionCredentialsInspect, "channel_management.detail")
	if !ok {
		writeChannelManagementDenial(w)
		return
	}
	connector, found := supervisor.GetForTenant(connectorID, tenantContext.TenantID)
	if !found {
		http.NotFound(w, r)
		return
	}
	detail, err := buildChannelConnectorDetail(r, sqliteStore, tenantContext, connector)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func handleChannelManagementDiagnostics(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, connectorID string) {
	tenantContext, ok := requireChannelManagementPermissions(r, sqliteStore, connectorID, "channel_management.diagnostics", identity.PermissionCredentialsInspect, identity.PermissionIntegrationDiagnosticsRead)
	if !ok {
		writeChannelManagementDenial(w)
		return
	}
	if sqliteStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []connectors.ConnectorDiagnosticState{}})
		return
	}
	items, err := sqliteStore.ListConnectorDiagnosticStates(r.Context(), tenantContext.TenantID, connectorID, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleChannelManagementDisable(supervisor *connectors.Supervisor, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, connectorID string) {
	tenantContext, ok := requireChannelManagementPermission(r, sqliteStore, connectorID, identity.PermissionConnectorsManage, "channel_management.disable")
	if !ok {
		writeChannelManagementDenial(w)
		return
	}
	var input channelManagementActionRequest
	if r.Body != nil && r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&input)
	}
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "channel management audit store is not configured")
		return
	}
	var result connectors.EnablementMutationResult
	if err := supervisor.WithConnectorMutation(connectorID, func() error {
		if _, found := supervisor.GetForTenant(connectorID, tenantContext.TenantID); !found {
			return connectors.ErrConnectorNotFound
		}
		now := time.Now().UTC()
		auditRecord, err := recordChannelManagementAudit(r, sqliteStore, connectors.ConnectorAuditRecord{
			TenantID:        tenantContext.TenantID,
			ConnectorID:     connectorID,
			PrincipalID:     tenantContext.PrincipalID,
			Action:          "channel_management.disable",
			PermissionGate:  string(identity.PermissionConnectorsManage),
			Outcome:         "succeeded",
			ReasonCode:      coalesceReason(input.ReasonCode, "tenant_disabled"),
			RedactionStatus: connectors.RedactionStatusRedacted,
		})
		if err != nil {
			return err
		}
		state := connectors.EnablementState{
			TenantID:             tenantContext.TenantID,
			ConnectorID:          connectorID,
			State:                "disabled",
			ReasonCode:           coalesceReason(input.ReasonCode, "tenant_disabled"),
			ChangedByPrincipalID: tenantContext.PrincipalID,
			ChangedAt:            now,
			AuditEventID:         auditRecord.AuditEventID,
		}
		if err := sqliteStore.SaveChannelConnectorEnablementState(r.Context(), state); err != nil {
			return err
		}
		if _, err := supervisor.Disable(connectorID, coalesceReason(input.ReasonCode, "tenant_disabled")); err != nil {
			return err
		}
		result = connectors.EnablementMutationResult{
			ConnectorID:      connectorID,
			EnablementState:  connectors.ManagementStateDisabled,
			DeliveryEligible: false,
			AuditEventID:     auditRecord.AuditEventID,
			ChangedAt:        now,
		}
		return nil
	}); err != nil {
		handleChannelManagementMutationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func handleChannelManagementReEnable(supervisor *connectors.Supervisor, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, connectorID string) {
	tenantContext, ok := requireChannelManagementPermission(r, sqliteStore, connectorID, identity.PermissionConnectorsManage, "channel_management.re_enable")
	if !ok {
		writeChannelManagementDenial(w)
		return
	}
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "channel management audit store is not configured")
		return
	}
	now := time.Now().UTC()
	diagnostics, err := sqliteStore.ListConnectorDiagnosticStates(r.Context(), tenantContext.TenantID, connectorID, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(diagnostics) > 0 && connectors.FreshnessAt(diagnostics[0].EvidenceTimestamp, now) == connectors.FreshnessStale {
		writeError(w, http.StatusConflict, "diagnostic state is stale")
		return
	}
	if len(diagnostics) > 0 && connectors.ManagementStateForConnector(connectors.Connector{Status: connectors.StatusRegistered}, &diagnostics[0]) != connectors.ManagementStateReady {
		writeError(w, http.StatusConflict, "diagnostic state is not ready")
		return
	}
	if policy, found, err := getOrDefaultChannelRoutePolicy(r, sqliteStore, tenantContext.TenantID, connectorID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	} else if found && policy.ValidationState != "valid" {
		writeError(w, http.StatusConflict, "route policy is not valid")
		return
	}
	var result connectors.EnablementMutationResult
	if err := supervisor.WithConnectorMutation(connectorID, func() error {
		connector, found := supervisor.GetForTenant(connectorID, tenantContext.TenantID)
		if !found {
			return connectors.ErrConnectorNotFound
		}
		auditRecord, err := recordChannelManagementAudit(r, sqliteStore, connectors.ConnectorAuditRecord{
			TenantID:        tenantContext.TenantID,
			ConnectorID:     connectorID,
			PrincipalID:     tenantContext.PrincipalID,
			Action:          "channel_management.re_enable",
			PermissionGate:  string(identity.PermissionConnectorsManage),
			Outcome:         "succeeded",
			ReasonCode:      "validated_re_enable",
			RedactionStatus: connectors.RedactionStatusRedacted,
		})
		if err != nil {
			return err
		}
		validatedAt := now
		if err := sqliteStore.SaveChannelConnectorEnablementState(r.Context(), connectors.EnablementState{
			TenantID:             tenantContext.TenantID,
			ConnectorID:          connectorID,
			State:                "enabled",
			ReasonCode:           "validated_re_enable",
			ChangedByPrincipalID: tenantContext.PrincipalID,
			ChangedAt:            now,
			ValidatedAt:          &validatedAt,
			AuditEventID:         auditRecord.AuditEventID,
		}); err != nil {
			return err
		}
		if _, err := supervisor.ReEnable(connector.ConnectorID); err != nil {
			return err
		}
		result = connectors.EnablementMutationResult{
			ConnectorID:      connectorID,
			EnablementState:  connectors.ManagementStateReady,
			DeliveryEligible: true,
			AuditEventID:     auditRecord.AuditEventID,
			ChangedAt:        now,
		}
		return nil
	}); err != nil {
		handleChannelManagementMutationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func handleChannelManagementRepair(supervisor *connectors.Supervisor, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, connectorID string) {
	var input channelManagementActionRequest
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	actionKind := input.ActionKind
	if actionKind == "" {
		actionKind = connectors.ManagementActionRepair
	}
	required := []identity.Permission{identity.PermissionConnectorsManage}
	if actionKind == connectors.ManagementActionReconnect || actionKind == connectors.ManagementActionCredentialRotation {
		required = append(required, identity.PermissionSecretsManage)
	}
	tenantContext, ok := requireChannelManagementPermissions(r, sqliteStore, connectorID, "channel_management.repair", required...)
	if !ok {
		writeChannelManagementDenial(w)
		return
	}
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "channel management audit store is not configured")
		return
	}
	connector, found := supervisor.GetForTenant(connectorID, tenantContext.TenantID)
	if !found {
		http.NotFound(w, r)
		return
	}
	auditRecord, err := recordChannelManagementAudit(r, sqliteStore, connectors.ConnectorAuditRecord{
		TenantID:        tenantContext.TenantID,
		ConnectorID:     connectorID,
		PrincipalID:     tenantContext.PrincipalID,
		Action:          "channel_management." + string(actionKind),
		PermissionGate:  strings.Join(permissionStrings(required), "+"),
		Outcome:         "succeeded",
		ReasonCode:      "repair_started",
		RedactionStatus: connectors.RedactionStatusRedacted,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	now := time.Now().UTC()
	action, err := sqliteStore.SaveChannelRepairAction(r.Context(), connectors.RepairAction{
		TenantID:                tenantContext.TenantID,
		ConnectorID:             connectorID,
		ConnectorKind:           connector.Kind,
		ActorPrincipalID:        tenantContext.PrincipalID,
		ActionKind:              actionKind,
		SourceDiagnosticStateID: input.SourceDiagnosticStateID,
		Status:                  connectors.TerminalStateForRepairAction(actionKind, connector.Status == connectors.StatusDisabled),
		RetrySafety:             connectors.RetrySafetyForRepairAction(actionKind),
		RemediationOwner:        connectors.RemediationOwnerAdmin,
		StartedAt:               now,
		AuditEventID:            auditRecord.AuditEventID,
		RedactionStatus:         connectors.RedactionStatusRedacted,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, action)
}

func handleChannelManagementRoutePolicy(supervisor *connectors.Supervisor, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, connectorID string) {
	switch r.Method {
	case http.MethodGet:
		tenantContext, ok := requireChannelManagementPermission(r, sqliteStore, connectorID, identity.PermissionCredentialsInspect, "channel_management.route_policy.read")
		if !ok {
			writeChannelManagementDenial(w)
			return
		}
		policy, found, err := getOrDefaultChannelRoutePolicy(r, sqliteStore, tenantContext.TenantID, connectorID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = found
		writeJSON(w, http.StatusOK, policy)
	case http.MethodPut:
		tenantContext, ok := requireChannelManagementPermission(r, sqliteStore, connectorID, identity.PermissionConnectorsManage, "channel_management.route_policy.update")
		if !ok {
			writeChannelManagementDenial(w)
			return
		}
		if sqliteStore == nil {
			writeError(w, http.StatusInternalServerError, "channel management audit store is not configured")
			return
		}
		var input channelManagementActionRequest
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		connector, found := supervisor.GetForTenant(connectorID, tenantContext.TenantID)
		if !found {
			writeError(w, http.StatusNotFound, "connector not found")
			return
		}
		if connectors.CapabilityProfileForKind(connector.Kind)["route-edit"] == connectors.CapabilityUnsupported {
			writeError(w, http.StatusConflict, "route editing is unsupported for connector kind "+connector.Kind)
			return
		}
		backgroundDelivery := true
		if input.BackgroundDelivery != nil {
			backgroundDelivery = *input.BackgroundDelivery
		}
		if err := supervisor.WithConnectorMutation(connectorID, func() error {
			auditRecord, err := recordChannelManagementAudit(r, sqliteStore, connectors.ConnectorAuditRecord{
				TenantID:        tenantContext.TenantID,
				ConnectorID:     connectorID,
				PrincipalID:     tenantContext.PrincipalID,
				Action:          "channel_management.route_policy.update",
				PermissionGate:  string(identity.PermissionConnectorsManage),
				Outcome:         "succeeded",
				ReasonCode:      "route_policy_updated",
				RedactionStatus: connectors.RedactionStatusRedacted,
			})
			if err != nil {
				return err
			}
			policy := connectors.RoutePolicy{
				TenantID:                   tenantContext.TenantID,
				ConnectorID:                connectorID,
				EligibleSenders:            input.EligibleSenders,
				EligibleConversations:      input.EligibleConversations,
				EligibleRooms:              input.EligibleRooms,
				EligibleChannels:           input.EligibleChannels,
				InvocationGates:            input.InvocationGates,
				BackgroundDeliveryEligible: backgroundDelivery,
				ValidationState:            "valid",
				ValidatedAt:                time.Now().UTC(),
				AuditEventID:               auditRecord.AuditEventID,
				RedactionStatus:            connectors.RedactionStatusRedacted,
			}
			policy = connectors.NormalizeRoutePolicy(policy, time.Now().UTC())
			return sqliteStore.SaveChannelRoutePolicy(r.Context(), policy)
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		stored, _, err := sqliteStore.GetChannelRoutePolicy(r.Context(), tenantContext.TenantID, connectorID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, stored)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleChannelManagementReplyOutcomes(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, connectorID string) {
	tenantContext, ok := requireChannelManagementPermission(r, sqliteStore, connectorID, identity.PermissionCredentialsInspect, "channel_management.reply_outcomes")
	if !ok {
		writeChannelManagementDenial(w)
		return
	}
	items := []connectors.ForegroundReplyOutcome{}
	if sqliteStore != nil {
		var err error
		items, err = sqliteStore.ListChannelForegroundReplyOutcomes(r.Context(), tenantContext.TenantID, connectorID, time.Now().UTC())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleChannelManagementDeliveryOutcomes(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, connectorID string) {
	tenantContext, ok := requireChannelManagementPermission(r, sqliteStore, connectorID, identity.PermissionCredentialsInspect, "channel_management.delivery_outcomes")
	if !ok {
		writeChannelManagementDenial(w)
		return
	}
	items := []connectors.BackgroundDeliveryOutcome{}
	if sqliteStore != nil {
		var err error
		items, err = sqliteStore.ListChannelBackgroundDeliveryOutcomes(r.Context(), tenantContext.TenantID, connectorID, time.Now().UTC())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func handleChannelManagementSupportEvidence(supervisor *connectors.Supervisor, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, connectorID string) {
	tenantContext, ok := requireChannelManagementPermission(r, sqliteStore, connectorID, identity.PermissionCredentialsInspect, "channel_management.support_evidence")
	if !ok {
		writeChannelManagementDenial(w)
		return
	}
	connector, found := supervisor.GetForTenant(connectorID, tenantContext.TenantID)
	if !found {
		http.NotFound(w, r)
		return
	}
	now := time.Now().UTC()
	if sqliteStore != nil {
		if existing, found, err := sqliteStore.GetLatestChannelSupportEvidence(r.Context(), tenantContext.TenantID, connectorID, now); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		} else if found {
			writeJSON(w, http.StatusOK, existing)
			return
		}
	}
	bundle := connectors.BuildSupportEvidenceBundle(connectors.ProjectionInput{TenantID: tenantContext.TenantID}, connector, tenantContext.PrincipalID, now)
	if sqliteStore != nil {
		if err := enrichChannelSupportEvidenceBundle(r, sqliteStore, eventBus, tenantContext.TenantID, connectorID, now, &bundle); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		var err error
		bundle, err = sqliteStore.SaveChannelSupportEvidence(r.Context(), bundle)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if eventBus != nil {
		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.ConnectorManagementSupportEvidenceGenerated(events.ConnectorManagementEventInput{
			TenantID:        tenantContext.TenantID,
			ConnectorID:     connectorID,
			EvidenceID:      bundle.SupportEvidenceID,
			Action:          "channel_management.support_evidence.generated",
			Outcome:         "succeeded",
			ReasonCode:      "support_evidence_generated",
			RedactionStatus: string(bundle.RedactionStatus),
			OccurredAt:      bundle.GeneratedAt,
		})); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, bundle)
}

func enrichChannelSupportEvidenceBundle(r *http.Request, sqliteStore *store.SQLiteStore, eventBus *events.Bus, tenantID, connectorID string, now time.Time, bundle *connectors.SupportEvidenceBundle) error {
	diagnostics, err := sqliteStore.ListConnectorDiagnosticStates(r.Context(), tenantID, connectorID, now)
	if err != nil {
		return err
	}
	for _, item := range diagnostics {
		bundle.DiagnosticRefs = append(bundle.DiagnosticRefs, item.DiagnosticStateID)
		if item.RedactionStatus == connectors.RedactionStatusFailed || item.RedactionStatus == connectors.RedactionStatusSuppressed {
			bundle.RedactionStatus = connectors.RedactionStatusSuppressed
			bundle.Redactions = append(bundle.Redactions, "diagnostic_evidence")
			if eventBus != nil {
				if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.ConnectorManagementRedactionFailed(events.ConnectorManagementEventInput{
					TenantID:        tenantID,
					ConnectorID:     connectorID,
					EvidenceID:      item.DiagnosticStateID,
					Action:          "channel_management.support_evidence.redaction",
					Outcome:         "suppressed",
					ReasonCode:      string(item.ReasonCode),
					RedactionStatus: string(item.RedactionStatus),
					OccurredAt:      now,
				})); err != nil {
					return err
				}
			}
		}
	}
	repairs, err := sqliteStore.ListChannelRepairActions(r.Context(), tenantID, connectorID)
	if err != nil {
		return err
	}
	for _, item := range repairs {
		bundle.RepairRefs = append(bundle.RepairRefs, item.RepairActionID)
	}
	decisions, err := sqliteStore.ListChannelRoutingDecisions(r.Context(), tenantID, connectorID, now)
	if err != nil {
		return err
	}
	for _, item := range decisions {
		bundle.RoutingDecisionRefs = append(bundle.RoutingDecisionRefs, item.RoutingDecisionID)
	}
	replies, err := sqliteStore.ListChannelForegroundReplyOutcomes(r.Context(), tenantID, connectorID, now)
	if err != nil {
		return err
	}
	for _, item := range replies {
		bundle.ReplyOutcomeRefs = append(bundle.ReplyOutcomeRefs, item.ReplyOutcomeID)
	}
	deliveries, err := sqliteStore.ListChannelBackgroundDeliveryOutcomes(r.Context(), tenantID, connectorID, now)
	if err != nil {
		return err
	}
	for _, item := range deliveries {
		bundle.DeliveryOutcomeRefs = append(bundle.DeliveryOutcomeRefs, item.DeliveryOutcomeID)
	}
	audits, err := sqliteStore.ListChannelManagementAuditRecords(r.Context(), tenantID, connectorID)
	if err != nil {
		return err
	}
	for _, item := range audits {
		bundle.AuditRefs = append(bundle.AuditRefs, item.AuditEventID)
	}
	expired, err := sqliteStore.ListExpiredChannelSupportEvidence(r.Context(), tenantID, connectorID, now)
	if err != nil {
		return err
	}
	for _, item := range expired {
		if eventBus == nil {
			continue
		}
		if _, err := publishEvent(r.Context(), eventBus, sqliteStore, events.ConnectorManagementRetentionApplied(events.ConnectorManagementEventInput{
			TenantID:        tenantID,
			ConnectorID:     connectorID,
			EvidenceID:      item.SupportEvidenceID,
			Action:          "channel_management.support_evidence.retention",
			Outcome:         "expired",
			ReasonCode:      "retention_expired",
			RedactionStatus: string(item.RedactionStatus),
			OccurredAt:      now,
		})); err != nil {
			return err
		}
	}
	return nil
}

func buildChannelConnectorDetail(r *http.Request, sqliteStore *store.SQLiteStore, tenantContext identity.TenantContext, connector connectors.Connector) (connectors.ChannelConnectorDetail, error) {
	now := time.Now().UTC()
	var diagnostics []connectors.ConnectorDiagnosticState
	if sqliteStore != nil {
		items, err := sqliteStore.ListConnectorDiagnosticStates(r.Context(), tenantContext.TenantID, connector.ConnectorID, now)
		if err != nil {
			return connectors.ChannelConnectorDetail{}, err
		}
		diagnostics = items
	}
	projection := connectors.BuildConnectorProjection(connector, latestChannelDiagnostic(diagnostics), now)
	routePolicy, _, err := getOrDefaultChannelRoutePolicy(r, sqliteStore, tenantContext.TenantID, connector.ConnectorID)
	if err != nil {
		return connectors.ChannelConnectorDetail{}, err
	}
	var repairActions []connectors.RepairAction
	var recentRouteDecisions []connectors.RoutingDecision
	var foregroundReplyOutcomes []connectors.ForegroundReplyOutcome
	var backgroundDeliveryOutcomes []connectors.BackgroundDeliveryOutcome
	if sqliteStore != nil {
		repairActions, err = sqliteStore.ListChannelRepairActions(r.Context(), tenantContext.TenantID, connector.ConnectorID)
		if err != nil {
			return connectors.ChannelConnectorDetail{}, err
		}
		recentRouteDecisions, err = sqliteStore.ListChannelRoutingDecisions(r.Context(), tenantContext.TenantID, connector.ConnectorID, now)
		if err != nil {
			return connectors.ChannelConnectorDetail{}, err
		}
		foregroundReplyOutcomes, err = sqliteStore.ListChannelForegroundReplyOutcomes(r.Context(), tenantContext.TenantID, connector.ConnectorID, now)
		if err != nil {
			return connectors.ChannelConnectorDetail{}, err
		}
		backgroundDeliveryOutcomes, err = sqliteStore.ListChannelBackgroundDeliveryOutcomes(r.Context(), tenantContext.TenantID, connector.ConnectorID, now)
		if err != nil {
			return connectors.ChannelConnectorDetail{}, err
		}
	}
	return connectors.ChannelConnectorDetail{
		ChannelConnectorProjection: projection,
		DiagnosticSummary:          latestChannelDiagnostic(diagnostics),
		RoutePolicy:                &routePolicy,
		RecentRouteDecisions:       recentRouteDecisions,
		ForegroundReplyOutcomes:    foregroundReplyOutcomes,
		BackgroundDelivery:         backgroundDeliveryOutcomes,
		RepairActions:              repairActions,
		SupportEvidenceAvailable:   true,
		Retention: map[string]string{
			"defaultDays": "90",
		},
	}, nil
}

func getOrDefaultChannelRoutePolicy(r *http.Request, sqliteStore *store.SQLiteStore, tenantID, connectorID string) (connectors.RoutePolicy, bool, error) {
	if sqliteStore != nil {
		policy, found, err := sqliteStore.GetChannelRoutePolicy(r.Context(), tenantID, connectorID)
		if err != nil || found {
			return policy, found, err
		}
	}
	return connectors.DefaultRoutePolicy(tenantID, connectorID, time.Now().UTC()), false, nil
}

func recordChannelManagementAudit(r *http.Request, sqliteStore *store.SQLiteStore, record connectors.ConnectorAuditRecord) (connectors.ConnectorAuditRecord, error) {
	if sqliteStore == nil {
		return connectors.ConnectorAuditRecord{}, errors.New("channel management audit store is not configured")
	}
	if record.PrincipalID == "" {
		record.PrincipalID = currentActor(r.Context())
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	return sqliteStore.SaveChannelManagementAuditRecord(r.Context(), record)
}

func handleChannelConnectorError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, connectors.ErrConnectorNotFound):
		http.NotFound(w, r)
	case errors.Is(err, connectors.ErrConnectorDisabled):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func handleChannelManagementMutationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, connectors.ErrConnectorNotFound), errors.Is(err, connectors.ErrConnectorDisabled):
		handleChannelConnectorError(w, r, err)
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func parseChannelManagementLimit(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func latestChannelDiagnostic(items []connectors.ConnectorDiagnosticState) *connectors.ConnectorDiagnosticState {
	if len(items) == 0 {
		return nil
	}
	latest := items[0]
	for _, item := range items[1:] {
		if item.EvidenceTimestamp.After(latest.EvidenceTimestamp) {
			latest = item
		}
	}
	return &latest
}

func coalesceReason(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func permissionStrings(items []identity.Permission) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, string(item))
	}
	return out
}
