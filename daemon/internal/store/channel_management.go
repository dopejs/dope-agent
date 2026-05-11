package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
)

type ChannelConnectorProjectionQuery struct {
	TenantID    string
	Limit       int
	Cursor      string
	StateFilter string
	KindFilter  string
	Now         time.Time
}

func (s *SQLiteStore) ListChannelConnectorProjectionPage(ctx context.Context, query ChannelConnectorProjectionQuery) (connectors.ChannelConnectorListResponse, error) {
	now := query.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if s == nil {
		return connectors.BuildConnectorPage(connectors.ProjectionInput{
			TenantID:    query.TenantID,
			Now:         now,
			Limit:       query.Limit,
			Cursor:      query.Cursor,
			StateFilter: query.StateFilter,
			KindFilter:  query.KindFilter,
		}), nil
	}
	items, err := s.ListConnectors(ctx)
	if err != nil {
		return connectors.ChannelConnectorListResponse{}, err
	}
	diagnosticsByConnector := map[string][]connectors.ConnectorDiagnosticState{}
	for _, connector := range items {
		if query.TenantID != "" && connector.TenantID != query.TenantID {
			continue
		}
		diagnostics, err := s.ListConnectorDiagnosticStates(ctx, query.TenantID, connector.ConnectorID, now)
		if err != nil {
			return connectors.ChannelConnectorListResponse{}, err
		}
		diagnosticsByConnector[connector.ConnectorID] = diagnostics
	}
	return connectors.BuildConnectorPage(connectors.ProjectionInput{
		TenantID:    query.TenantID,
		Connectors:  items,
		Diagnostics: diagnosticsByConnector,
		Now:         now,
		Limit:       query.Limit,
		Cursor:      query.Cursor,
		StateFilter: query.StateFilter,
		KindFilter:  query.KindFilter,
	}), nil
}

func (s *SQLiteStore) SaveChannelManagementAuditRecord(ctx context.Context, record connectors.ConnectorAuditRecord) (connectors.ConnectorAuditRecord, error) {
	if s == nil {
		return record, nil
	}
	now := record.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if record.AuditEventID == "" {
		record.AuditEventID = newStoreID("connector_management_audit")
	}
	record.CreatedAt = now
	if record.RedactionStatus == "" {
		record.RedactionStatus = connectors.RedactionStatusRedacted
	}
	documentJSON, err := json.Marshal(record)
	if err != nil {
		return connectors.ConnectorAuditRecord{}, fmt.Errorf("marshal channel management audit: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO channel_management_audit_records (
			audit_event_id, tenant_id, connector_id, principal_id, action, permission_gate,
			outcome, reason_code, created_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(audit_event_id) DO UPDATE SET
			tenant_id = excluded.tenant_id,
			connector_id = excluded.connector_id,
			principal_id = excluded.principal_id,
			action = excluded.action,
			permission_gate = excluded.permission_gate,
			outcome = excluded.outcome,
			reason_code = excluded.reason_code,
			created_at = excluded.created_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, record.AuditEventID, record.TenantID, record.ConnectorID, record.PrincipalID, record.Action, record.PermissionGate, record.Outcome, record.ReasonCode, record.CreatedAt.UTC().Format(time.RFC3339Nano), record.RedactionStatus, string(documentJSON)); err != nil {
		return connectors.ConnectorAuditRecord{}, fmt.Errorf("save channel management audit %s: %w", record.AuditEventID, err)
	}
	return record, nil
}

func (s *SQLiteStore) ListChannelManagementAuditRecords(ctx context.Context, tenantID, connectorID string) ([]connectors.ConnectorAuditRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM channel_management_audit_records
		WHERE tenant_id = ? AND connector_id = ?
		ORDER BY created_at DESC, audit_event_id DESC
		LIMIT 50
	`, tenantID, connectorID)
	if err != nil {
		return nil, fmt.Errorf("list channel management audit %s/%s: %w", tenantID, connectorID, err)
	}
	defer rows.Close()
	items := []connectors.ConnectorAuditRecord{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan channel management audit: %w", err)
		}
		var item connectors.ConnectorAuditRecord
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode channel management audit: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) SaveChannelConnectorEnablementState(ctx context.Context, state connectors.EnablementState) error {
	if s == nil {
		return nil
	}
	if state.ChangedAt.IsZero() {
		state.ChangedAt = time.Now().UTC()
	}
	documentJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal channel connector enablement: %w", err)
	}
	var validatedAt sql.NullString
	if state.ValidatedAt != nil {
		validatedAt = sql.NullString{String: state.ValidatedAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO channel_connector_enablement_states (
			tenant_id, connector_id, state, reason_code, changed_by_principal_id,
			changed_at, validated_at, audit_event_id, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, connector_id) DO UPDATE SET
			state = excluded.state,
			reason_code = excluded.reason_code,
			changed_by_principal_id = excluded.changed_by_principal_id,
			changed_at = excluded.changed_at,
			validated_at = excluded.validated_at,
			audit_event_id = excluded.audit_event_id,
			document_json = excluded.document_json
	`, state.TenantID, state.ConnectorID, state.State, nullString(state.ReasonCode), nullString(state.ChangedByPrincipalID), state.ChangedAt.UTC().Format(time.RFC3339Nano), validatedAt, state.AuditEventID, string(documentJSON)); err != nil {
		return fmt.Errorf("save channel connector enablement %s/%s: %w", state.TenantID, state.ConnectorID, err)
	}
	return nil
}

func (s *SQLiteStore) GetChannelConnectorEnablementState(ctx context.Context, tenantID, connectorID string) (connectors.EnablementState, bool, error) {
	if s == nil {
		return connectors.EnablementState{}, false, nil
	}
	var raw string
	if err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM channel_connector_enablement_states
		WHERE tenant_id = ? AND connector_id = ?
	`, tenantID, connectorID).Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return connectors.EnablementState{}, false, nil
		}
		return connectors.EnablementState{}, false, fmt.Errorf("get channel connector enablement %s/%s: %w", tenantID, connectorID, err)
	}
	var state connectors.EnablementState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return connectors.EnablementState{}, false, fmt.Errorf("decode channel connector enablement %s/%s: %w", tenantID, connectorID, err)
	}
	return state, true, nil
}

func (s *SQLiteStore) SaveChannelRepairAction(ctx context.Context, action connectors.RepairAction) (connectors.RepairAction, error) {
	if s == nil {
		return action, nil
	}
	if action.RepairActionID == "" {
		action.RepairActionID = newStoreID("channel_repair_action")
	}
	if action.StartedAt.IsZero() {
		action.StartedAt = time.Now().UTC()
	}
	if action.RedactionStatus == "" {
		action.RedactionStatus = connectors.RedactionStatusRedacted
	}
	documentJSON, err := json.Marshal(action)
	if err != nil {
		return connectors.RepairAction{}, fmt.Errorf("marshal channel repair action: %w", err)
	}
	var completedAt sql.NullString
	if action.CompletedAt != nil {
		completedAt = sql.NullString{String: action.CompletedAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO channel_repair_actions (
			repair_action_id, tenant_id, connector_id, connector_kind, actor_principal_id,
			action_kind, source_diagnostic_state_id, setup_session_id, status, retry_safety,
			remediation_owner, started_at, completed_at, audit_event_id, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(repair_action_id) DO UPDATE SET
			status = excluded.status,
			completed_at = excluded.completed_at,
			audit_event_id = excluded.audit_event_id,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, action.RepairActionID, action.TenantID, action.ConnectorID, action.ConnectorKind, nullString(action.ActorPrincipalID), action.ActionKind, nullString(action.SourceDiagnosticStateID), nullString(action.SetupSessionID), action.Status, action.RetrySafety, action.RemediationOwner, action.StartedAt.UTC().Format(time.RFC3339Nano), completedAt, action.AuditEventID, action.RedactionStatus, string(documentJSON)); err != nil {
		return connectors.RepairAction{}, fmt.Errorf("save channel repair action %s: %w", action.RepairActionID, err)
	}
	return action, nil
}

func (s *SQLiteStore) ListChannelRepairActions(ctx context.Context, tenantID, connectorID string) ([]connectors.RepairAction, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM channel_repair_actions
		WHERE tenant_id = ? AND connector_id = ?
		ORDER BY started_at DESC, repair_action_id DESC
		LIMIT 50
	`, tenantID, connectorID)
	if err != nil {
		return nil, fmt.Errorf("list channel repair actions %s/%s: %w", tenantID, connectorID, err)
	}
	defer rows.Close()
	var items []connectors.RepairAction
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan channel repair action: %w", err)
		}
		var item connectors.RepairAction
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode channel repair action: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) SaveChannelRoutePolicy(ctx context.Context, policy connectors.RoutePolicy) error {
	if s == nil {
		return nil
	}
	if policy.RoutePolicyID == "" {
		policy.RoutePolicyID = newStoreID("channel_route_policy")
	}
	if policy.ValidatedAt.IsZero() {
		policy.ValidatedAt = time.Now().UTC()
	}
	if policy.RedactionStatus == "" {
		policy.RedactionStatus = connectors.RedactionStatusRedacted
	}
	documentJSON, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("marshal channel route policy: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO channel_route_policies (
			tenant_id, connector_id, route_policy_id, validation_state, reason_code,
			background_delivery_eligible, validated_at, audit_event_id, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, connector_id) DO UPDATE SET
			route_policy_id = excluded.route_policy_id,
			validation_state = excluded.validation_state,
			reason_code = excluded.reason_code,
			background_delivery_eligible = excluded.background_delivery_eligible,
			validated_at = excluded.validated_at,
			audit_event_id = excluded.audit_event_id,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, policy.TenantID, policy.ConnectorID, policy.RoutePolicyID, policy.ValidationState, nullString(policy.ReasonCode), boolToInt(policy.BackgroundDeliveryEligible), policy.ValidatedAt.UTC().Format(time.RFC3339Nano), nullString(policy.AuditEventID), policy.RedactionStatus, string(documentJSON)); err != nil {
		return fmt.Errorf("save channel route policy %s/%s: %w", policy.TenantID, policy.ConnectorID, err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO channel_route_policy_snapshots (
			route_policy_id, tenant_id, connector_id, validated_at, audit_event_id, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(route_policy_id) DO UPDATE SET
			validated_at = excluded.validated_at,
			audit_event_id = excluded.audit_event_id,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, policy.RoutePolicyID, policy.TenantID, policy.ConnectorID, policy.ValidatedAt.UTC().Format(time.RFC3339Nano), nullString(policy.AuditEventID), policy.RedactionStatus, string(documentJSON)); err != nil {
		return fmt.Errorf("save channel route policy snapshot %s: %w", policy.RoutePolicyID, err)
	}
	return nil
}

func (s *SQLiteStore) GetChannelRoutePolicy(ctx context.Context, tenantID, connectorID string) (connectors.RoutePolicy, bool, error) {
	if s == nil {
		return connectors.RoutePolicy{}, false, nil
	}
	var raw string
	if err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM channel_route_policies
		WHERE tenant_id = ? AND connector_id = ?
	`, tenantID, connectorID).Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return connectors.RoutePolicy{}, false, nil
		}
		return connectors.RoutePolicy{}, false, fmt.Errorf("get channel route policy %s/%s: %w", tenantID, connectorID, err)
	}
	var policy connectors.RoutePolicy
	if err := json.Unmarshal([]byte(raw), &policy); err != nil {
		return connectors.RoutePolicy{}, false, fmt.Errorf("decode channel route policy %s/%s: %w", tenantID, connectorID, err)
	}
	return policy, true, nil
}

func (s *SQLiteStore) SaveChannelRoutingDecision(ctx context.Context, decision connectors.RoutingDecision) (connectors.RoutingDecision, error) {
	if s == nil {
		return decision, nil
	}
	if decision.RoutingDecisionID == "" {
		decision.RoutingDecisionID = newStoreID("channel_routing_decision")
	}
	if decision.OccurredAt.IsZero() {
		decision.OccurredAt = time.Now().UTC()
	}
	if decision.RetentionExpiresAt.IsZero() {
		decision.RetentionExpiresAt = decision.OccurredAt.Add(90 * 24 * time.Hour)
	}
	if decision.RedactionStatus == "" {
		decision.RedactionStatus = connectors.RedactionStatusRedacted
	}
	documentJSON, err := json.Marshal(decision)
	if err != nil {
		return connectors.RoutingDecision{}, fmt.Errorf("marshal channel routing decision: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO channel_routing_decisions (
			routing_decision_id, tenant_id, connector_id, connector_kind, outcome, reason_code,
			occurred_at, retention_expires_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(routing_decision_id) DO UPDATE SET
			outcome = excluded.outcome,
			reason_code = excluded.reason_code,
			occurred_at = excluded.occurred_at,
			retention_expires_at = excluded.retention_expires_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, decision.RoutingDecisionID, decision.TenantID, decision.ConnectorID, decision.ConnectorKind, decision.Outcome, nullString(decision.ReasonCode), decision.OccurredAt.UTC().Format(time.RFC3339Nano), decision.RetentionExpiresAt.UTC().Format(time.RFC3339Nano), decision.RedactionStatus, string(documentJSON)); err != nil {
		return connectors.RoutingDecision{}, fmt.Errorf("save channel routing decision %s: %w", decision.RoutingDecisionID, err)
	}
	return decision, nil
}

func (s *SQLiteStore) ListChannelRoutingDecisions(ctx context.Context, tenantID, connectorID string, now time.Time) ([]connectors.RoutingDecision, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.listChannelOutcomeDocuments(ctx, "channel_routing_decisions", tenantID, connectorID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []connectors.RoutingDecision{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan channel routing decision: %w", err)
		}
		var item connectors.RoutingDecision
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode channel routing decision: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) SaveChannelForegroundReplyOutcome(ctx context.Context, outcome connectors.ForegroundReplyOutcome) (connectors.ForegroundReplyOutcome, error) {
	if s == nil {
		return outcome, nil
	}
	if outcome.ReplyOutcomeID == "" {
		outcome.ReplyOutcomeID = newStoreID("channel_reply_outcome")
	}
	if outcome.OccurredAt.IsZero() {
		outcome.OccurredAt = time.Now().UTC()
	}
	if outcome.RetentionExpiresAt.IsZero() {
		outcome.RetentionExpiresAt = outcome.OccurredAt.Add(90 * 24 * time.Hour)
	}
	if outcome.RedactionStatus == "" {
		outcome.RedactionStatus = connectors.RedactionStatusRedacted
	}
	documentJSON, err := json.Marshal(outcome)
	if err != nil {
		return connectors.ForegroundReplyOutcome{}, fmt.Errorf("marshal channel reply outcome: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO channel_reply_outcomes (
			reply_outcome_id, tenant_id, connector_id, routing_decision_id, status, reason_code,
			occurred_at, retention_expires_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(reply_outcome_id) DO UPDATE SET
			routing_decision_id = excluded.routing_decision_id,
			status = excluded.status,
			reason_code = excluded.reason_code,
			occurred_at = excluded.occurred_at,
			retention_expires_at = excluded.retention_expires_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, outcome.ReplyOutcomeID, outcome.TenantID, outcome.ConnectorID, nullString(outcome.RoutingDecisionID), outcome.Status, nullString(outcome.ReasonCode), outcome.OccurredAt.UTC().Format(time.RFC3339Nano), outcome.RetentionExpiresAt.UTC().Format(time.RFC3339Nano), outcome.RedactionStatus, string(documentJSON)); err != nil {
		return connectors.ForegroundReplyOutcome{}, fmt.Errorf("save channel reply outcome %s: %w", outcome.ReplyOutcomeID, err)
	}
	return outcome, nil
}

func (s *SQLiteStore) ListChannelForegroundReplyOutcomes(ctx context.Context, tenantID, connectorID string, now time.Time) ([]connectors.ForegroundReplyOutcome, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.listChannelOutcomeDocuments(ctx, "channel_reply_outcomes", tenantID, connectorID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []connectors.ForegroundReplyOutcome{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan channel reply outcome: %w", err)
		}
		var item connectors.ForegroundReplyOutcome
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode channel reply outcome: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) SaveChannelBackgroundDeliveryOutcome(ctx context.Context, outcome connectors.BackgroundDeliveryOutcome) (connectors.BackgroundDeliveryOutcome, error) {
	if s == nil {
		return outcome, nil
	}
	if outcome.DeliveryOutcomeID == "" {
		outcome.DeliveryOutcomeID = newStoreID("channel_delivery_outcome")
	}
	if outcome.OccurredAt.IsZero() {
		outcome.OccurredAt = time.Now().UTC()
	}
	if outcome.RetentionExpiresAt.IsZero() {
		outcome.RetentionExpiresAt = outcome.OccurredAt.Add(90 * 24 * time.Hour)
	}
	if outcome.RedactionStatus == "" {
		outcome.RedactionStatus = connectors.RedactionStatusRedacted
	}
	documentJSON, err := json.Marshal(outcome)
	if err != nil {
		return connectors.BackgroundDeliveryOutcome{}, fmt.Errorf("marshal channel delivery outcome: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO channel_delivery_outcomes (
			delivery_outcome_id, tenant_id, connector_id, delivery_target_id, status, reason_code,
			occurred_at, retention_expires_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(delivery_outcome_id) DO UPDATE SET
			delivery_target_id = excluded.delivery_target_id,
			status = excluded.status,
			reason_code = excluded.reason_code,
			occurred_at = excluded.occurred_at,
			retention_expires_at = excluded.retention_expires_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, outcome.DeliveryOutcomeID, outcome.TenantID, outcome.ConnectorID, nullString(outcome.DeliveryTargetID), outcome.Status, nullString(outcome.ReasonCode), outcome.OccurredAt.UTC().Format(time.RFC3339Nano), outcome.RetentionExpiresAt.UTC().Format(time.RFC3339Nano), outcome.RedactionStatus, string(documentJSON)); err != nil {
		return connectors.BackgroundDeliveryOutcome{}, fmt.Errorf("save channel delivery outcome %s: %w", outcome.DeliveryOutcomeID, err)
	}
	return outcome, nil
}

func (s *SQLiteStore) ListChannelBackgroundDeliveryOutcomes(ctx context.Context, tenantID, connectorID string, now time.Time) ([]connectors.BackgroundDeliveryOutcome, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.listChannelOutcomeDocuments(ctx, "channel_delivery_outcomes", tenantID, connectorID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []connectors.BackgroundDeliveryOutcome{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan channel delivery outcome: %w", err)
		}
		var item connectors.BackgroundDeliveryOutcome
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode channel delivery outcome: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) SaveChannelSupportEvidence(ctx context.Context, bundle connectors.SupportEvidenceBundle) (connectors.SupportEvidenceBundle, error) {
	if s == nil {
		return bundle, nil
	}
	if bundle.SupportEvidenceID == "" {
		bundle.SupportEvidenceID = newStoreID("channel_support_evidence")
	}
	if bundle.GeneratedAt.IsZero() {
		bundle.GeneratedAt = time.Now().UTC()
	}
	if bundle.RetentionExpiresAt.IsZero() {
		bundle.RetentionExpiresAt = bundle.GeneratedAt.Add(90 * 24 * time.Hour)
	}
	if bundle.RedactionStatus == "" {
		bundle.RedactionStatus = connectors.RedactionStatusRedacted
	}
	documentJSON, err := json.Marshal(bundle)
	if err != nil {
		return connectors.SupportEvidenceBundle{}, fmt.Errorf("marshal channel support evidence: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO channel_support_evidence (
			support_evidence_id, tenant_id, connector_id, generated_by_principal_id,
			generated_at, current_state, retention_expires_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(support_evidence_id) DO UPDATE SET
			document_json = excluded.document_json
	`, bundle.SupportEvidenceID, bundle.TenantID, bundle.ConnectorID, nullString(bundle.GeneratedByPrincipalID), bundle.GeneratedAt.UTC().Format(time.RFC3339Nano), bundle.CurrentState, bundle.RetentionExpiresAt.UTC().Format(time.RFC3339Nano), bundle.RedactionStatus, string(documentJSON)); err != nil {
		return connectors.SupportEvidenceBundle{}, fmt.Errorf("save channel support evidence %s: %w", bundle.SupportEvidenceID, err)
	}
	return bundle, nil
}

func (s *SQLiteStore) GetLatestChannelSupportEvidence(ctx context.Context, tenantID, connectorID string, now time.Time) (connectors.SupportEvidenceBundle, bool, error) {
	if s == nil {
		return connectors.SupportEvidenceBundle{}, false, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var raw string
	if err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM channel_support_evidence
		WHERE tenant_id = ? AND connector_id = ? AND retention_expires_at > ?
		ORDER BY generated_at DESC, support_evidence_id DESC
		LIMIT 1
	`, tenantID, connectorID, now.UTC().Format(time.RFC3339Nano)).Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return connectors.SupportEvidenceBundle{}, false, nil
		}
		return connectors.SupportEvidenceBundle{}, false, fmt.Errorf("get channel support evidence %s/%s: %w", tenantID, connectorID, err)
	}
	var bundle connectors.SupportEvidenceBundle
	if err := json.Unmarshal([]byte(raw), &bundle); err != nil {
		return connectors.SupportEvidenceBundle{}, false, fmt.Errorf("decode channel support evidence %s/%s: %w", tenantID, connectorID, err)
	}
	return bundle, true, nil
}

func (s *SQLiteStore) ListExpiredChannelSupportEvidence(ctx context.Context, tenantID, connectorID string, now time.Time) ([]connectors.SupportEvidenceBundle, error) {
	if s == nil {
		return nil, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM channel_support_evidence
		WHERE tenant_id = ? AND connector_id = ? AND retention_expires_at <= ?
		ORDER BY retention_expires_at DESC, support_evidence_id DESC
		LIMIT 50
	`, tenantID, connectorID, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, fmt.Errorf("list expired channel support evidence %s/%s: %w", tenantID, connectorID, err)
	}
	defer rows.Close()
	items := []connectors.SupportEvidenceBundle{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan expired channel support evidence: %w", err)
		}
		var item connectors.SupportEvidenceBundle
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode expired channel support evidence: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) listChannelOutcomeDocuments(ctx context.Context, tableName, tenantID, connectorID string, now time.Time) (*sql.Rows, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var orderBy string
	switch tableName {
	case "channel_routing_decisions":
		orderBy = "occurred_at DESC, routing_decision_id DESC"
	case "channel_reply_outcomes":
		orderBy = "occurred_at DESC, reply_outcome_id DESC"
	case "channel_delivery_outcomes":
		orderBy = "occurred_at DESC, delivery_outcome_id DESC"
	default:
		return nil, fmt.Errorf("unsupported channel outcome table %s", tableName)
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT document_json
		FROM %s
		WHERE tenant_id = ? AND connector_id = ? AND retention_expires_at > ?
		ORDER BY %s
		LIMIT 50
	`, tableName, orderBy), tenantID, connectorID, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, fmt.Errorf("list %s %s/%s: %w", tableName, tenantID, connectorID, err)
	}
	return rows, nil
}
