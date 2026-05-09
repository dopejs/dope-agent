package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type SlackHostedSetupRecord struct {
	TenantID           string                  `json:"tenantId,omitempty"`
	ConnectorID        string                  `json:"connectorId"`
	ConnectorKind      string                  `json:"connectorKind"`
	DisplayName        string                  `json:"displayName"`
	Status             string                  `json:"status"`
	TerminalState      string                  `json:"terminalState"`
	OAuthState         string                  `json:"oauthState"`
	RoutePolicyState   string                  `json:"routePolicyState"`
	DeliveryEligible   bool                    `json:"deliveryEligible"`
	WorkspaceBindingID string                  `json:"workspaceBindingId"`
	ReasonCode         string                  `json:"reasonCode,omitempty"`
	RedactionStatus    string                  `json:"redactionStatus"`
	CreatedAt          time.Time               `json:"createdAt"`
	UpdatedAt          time.Time               `json:"updatedAt"`
	ValidatedAt        time.Time               `json:"validatedAt,omitempty"`
	RetentionExpiresAt time.Time               `json:"retentionExpiresAt"`
	WorkspaceBinding   *SlackWorkspaceBinding  `json:"workspaceBinding,omitempty"`
	RoutePolicy        *SlackRoutePolicyRecord `json:"routePolicy,omitempty"`
}

type SlackWorkspaceBinding struct {
	TenantID           string            `json:"tenantId,omitempty"`
	ConnectorID        string            `json:"connectorId"`
	WorkspaceBindingID string            `json:"workspaceBindingId"`
	WorkspaceID        string            `json:"workspaceId"`
	WorkspaceLabel     string            `json:"workspaceLabel,omitempty"`
	InstallationID     string            `json:"installationId"`
	OAuthGrantState    string            `json:"oauthGrantState"`
	RequiredScopeState string            `json:"requiredScopeState"`
	ValidatedAt        time.Time         `json:"validatedAt"`
	RedactionStatus    string            `json:"redactionStatus"`
	SafeEvidence       map[string]string `json:"safeEvidence,omitempty"`
}

type SlackRoutePolicyRecord struct {
	TenantID            string                         `json:"tenantId,omitempty"`
	ConnectorID         string                         `json:"connectorId"`
	WorkspaceBindingID  string                         `json:"workspaceBindingId"`
	SelectedChannels    []SlackConversationRouteRecord `json:"selectedChannels"`
	AllowedDMUsers      []string                       `json:"allowedDMUsers"`
	AllowedDMUserGroups []string                       `json:"allowedDMUserGroups"`
	MentionGate         string                         `json:"mentionGate"`
	ThreadReplyMode     string                         `json:"threadReplyMode"`
	ValidationState     string                         `json:"validationState"`
	ReasonCode          string                         `json:"reasonCode,omitempty"`
	ValidatedAt         time.Time                      `json:"validatedAt"`
	RedactionStatus     string                         `json:"redactionStatus"`
	SafeEvidence        map[string]string              `json:"safeEvidence,omitempty"`
}

type SlackConversationRouteRecord struct {
	ConversationID       string            `json:"conversationId"`
	ConversationType     string            `json:"conversationType"`
	SelectedChannelState string            `json:"selectedChannelState"`
	ValidationState      string            `json:"validationState"`
	ReasonCode           string            `json:"reasonCode,omitempty"`
	RedactionStatus      string            `json:"redactionStatus,omitempty"`
	SafeEvidence         map[string]string `json:"safeEvidence,omitempty"`
}

type SlackSmokeEvidenceRecord struct {
	SmokeEvidenceID    string            `json:"smokeEvidenceId"`
	TenantID           string            `json:"tenantId"`
	ConnectorID        string            `json:"connectorId"`
	WorkspaceBindingID string            `json:"workspaceBindingId"`
	Status             string            `json:"status"`
	AuthorizationMode  string            `json:"authorizationMode"`
	Owner              string            `json:"owner"`
	Reason             string            `json:"reason"`
	RemainingRisk      string            `json:"remainingRisk,omitempty"`
	ValidatedAt        time.Time         `json:"validatedAt"`
	RetentionExpiresAt time.Time         `json:"retentionExpiresAt"`
	RedactionStatus    string            `json:"redactionStatus"`
	SafeEvidence       map[string]string `json:"safeEvidence,omitempty"`
}

type SlackEventEvidenceRecord struct {
	TenantID           string            `json:"tenantId"`
	ConnectorID        string            `json:"connectorId"`
	WorkspaceID        string            `json:"workspaceId"`
	ConversationID     string            `json:"conversationId"`
	MessageID          string            `json:"messageId"`
	EventID            string            `json:"eventId"`
	RouteOutcome       string            `json:"routeOutcome"`
	ReasonCode         string            `json:"reasonCode,omitempty"`
	ReceivedAt         time.Time         `json:"receivedAt"`
	RetentionExpiresAt time.Time         `json:"retentionExpiresAt"`
	RedactionStatus    string            `json:"redactionStatus"`
	SafeEvidence       map[string]string `json:"safeEvidence,omitempty"`
}

func (s *SQLiteStore) SaveSlackHostedSetup(ctx context.Context, record SlackHostedSetupRecord) error {
	if s == nil {
		return nil
	}
	record = normalizeSlackHostedSetupRecord(ctx, s, record)
	document, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal slack hosted setup: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO slack_hosted_setups (
			tenant_id, connector_id, connector_kind, display_name, status, terminal_state,
			oauth_state, route_policy_state, delivery_eligible, workspace_binding_id,
			reason_code, redaction_status, created_at, updated_at, validated_at,
			retention_expires_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, connector_id) DO UPDATE SET
			connector_kind = excluded.connector_kind,
			display_name = excluded.display_name,
			status = excluded.status,
			terminal_state = excluded.terminal_state,
			oauth_state = excluded.oauth_state,
			route_policy_state = excluded.route_policy_state,
			delivery_eligible = excluded.delivery_eligible,
			workspace_binding_id = excluded.workspace_binding_id,
			reason_code = excluded.reason_code,
			redaction_status = excluded.redaction_status,
			updated_at = excluded.updated_at,
			validated_at = excluded.validated_at,
			retention_expires_at = excluded.retention_expires_at,
			document_json = excluded.document_json
	`, record.TenantID, record.ConnectorID, record.ConnectorKind, record.DisplayName,
		record.Status, record.TerminalState, record.OAuthState, record.RoutePolicyState,
		boolToInt(record.DeliveryEligible), record.WorkspaceBindingID, nullString(record.ReasonCode),
		record.RedactionStatus, record.CreatedAt.UTC().Format(time.RFC3339Nano),
		record.UpdatedAt.UTC().Format(time.RFC3339Nano), nullableTimeValue(record.ValidatedAt),
		record.RetentionExpiresAt.UTC().Format(time.RFC3339Nano), string(document))
	if err != nil {
		return fmt.Errorf("save slack hosted setup %s: %w", record.ConnectorID, err)
	}
	return nil
}

func (s *SQLiteStore) GetSlackHostedSetup(ctx context.Context, tenantID, connectorID string) (SlackHostedSetupRecord, bool, error) {
	if s == nil {
		return SlackHostedSetupRecord{}, false, nil
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM slack_hosted_setups
		WHERE tenant_id = ? AND connector_id = ?
	`, strings.TrimSpace(tenantID), strings.TrimSpace(connectorID)).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SlackHostedSetupRecord{}, false, nil
		}
		return SlackHostedSetupRecord{}, false, fmt.Errorf("get slack hosted setup %s: %w", connectorID, err)
	}
	var record SlackHostedSetupRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return SlackHostedSetupRecord{}, false, fmt.Errorf("decode slack hosted setup %s: %w", connectorID, err)
	}
	policy, ok, err := s.GetSlackRoutePolicy(ctx, tenantID, connectorID)
	if err != nil {
		return SlackHostedSetupRecord{}, false, err
	}
	if ok {
		record.RoutePolicy = &policy
	}
	return record, true, nil
}

func (s *SQLiteStore) SaveSlackRoutePolicy(ctx context.Context, record SlackRoutePolicyRecord) error {
	if s == nil {
		return nil
	}
	record = normalizeSlackRoutePolicyRecord(ctx, s, record)
	document, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal slack route policy: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO slack_route_policies (
			tenant_id, connector_id, workspace_binding_id, validation_state, reason_code,
			validated_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, connector_id) DO UPDATE SET
			workspace_binding_id = excluded.workspace_binding_id,
			validation_state = excluded.validation_state,
			reason_code = excluded.reason_code,
			validated_at = excluded.validated_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, record.TenantID, record.ConnectorID, record.WorkspaceBindingID, record.ValidationState,
		nullString(record.ReasonCode), record.ValidatedAt.UTC().Format(time.RFC3339Nano),
		record.RedactionStatus, string(document))
	if err != nil {
		return fmt.Errorf("save slack route policy %s: %w", record.ConnectorID, err)
	}
	return nil
}

func (s *SQLiteStore) GetSlackRoutePolicy(ctx context.Context, tenantID, connectorID string) (SlackRoutePolicyRecord, bool, error) {
	if s == nil {
		return SlackRoutePolicyRecord{}, false, nil
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM slack_route_policies
		WHERE tenant_id = ? AND connector_id = ?
	`, strings.TrimSpace(tenantID), strings.TrimSpace(connectorID)).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SlackRoutePolicyRecord{}, false, nil
		}
		return SlackRoutePolicyRecord{}, false, fmt.Errorf("get slack route policy %s: %w", connectorID, err)
	}
	var record SlackRoutePolicyRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return SlackRoutePolicyRecord{}, false, fmt.Errorf("decode slack route policy %s: %w", connectorID, err)
	}
	return record, true, nil
}

func (s *SQLiteStore) SaveSlackSmokeEvidence(ctx context.Context, record SlackSmokeEvidenceRecord) error {
	if s == nil {
		return nil
	}
	record = normalizeSlackSmokeEvidenceRecord(ctx, s, record)
	document, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal slack smoke evidence: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO slack_smoke_evidence (
			smoke_evidence_id, tenant_id, connector_id, workspace_binding_id, status,
			authorization_mode, owner, reason, remaining_risk, validated_at,
			retention_expires_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(smoke_evidence_id) DO UPDATE SET
			tenant_id = excluded.tenant_id,
			connector_id = excluded.connector_id,
			workspace_binding_id = excluded.workspace_binding_id,
			status = excluded.status,
			authorization_mode = excluded.authorization_mode,
			owner = excluded.owner,
			reason = excluded.reason,
			remaining_risk = excluded.remaining_risk,
			validated_at = excluded.validated_at,
			retention_expires_at = excluded.retention_expires_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, record.SmokeEvidenceID, record.TenantID, record.ConnectorID, record.WorkspaceBindingID,
		record.Status, record.AuthorizationMode, record.Owner, record.Reason,
		nullString(record.RemainingRisk), record.ValidatedAt.UTC().Format(time.RFC3339Nano),
		record.RetentionExpiresAt.UTC().Format(time.RFC3339Nano), record.RedactionStatus, string(document))
	if err != nil {
		return fmt.Errorf("save slack smoke evidence %s: %w", record.SmokeEvidenceID, err)
	}
	return nil
}

func (s *SQLiteStore) LatestSlackSmokeEvidence(ctx context.Context, tenantID, connectorID string, now time.Time) (SlackSmokeEvidenceRecord, bool, error) {
	if s == nil {
		return SlackSmokeEvidenceRecord{}, false, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM slack_smoke_evidence
		WHERE tenant_id = ? AND connector_id = ? AND retention_expires_at > ?
		ORDER BY validated_at DESC, smoke_evidence_id DESC
		LIMIT 1
	`, strings.TrimSpace(tenantID), strings.TrimSpace(connectorID), now.UTC().Format(time.RFC3339Nano)).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SlackSmokeEvidenceRecord{}, false, nil
		}
		return SlackSmokeEvidenceRecord{}, false, fmt.Errorf("latest slack smoke evidence %s: %w", connectorID, err)
	}
	var record SlackSmokeEvidenceRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return SlackSmokeEvidenceRecord{}, false, fmt.Errorf("decode slack smoke evidence: %w", err)
	}
	return record, true, nil
}

func (s *SQLiteStore) SaveSlackEventEvidence(ctx context.Context, record SlackEventEvidenceRecord) error {
	if s == nil {
		return nil
	}
	record = normalizeSlackEventEvidenceRecord(ctx, s, record)
	document, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal slack event evidence: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO slack_event_evidence (
			tenant_id, connector_id, workspace_id, conversation_id, message_id, event_id,
			route_outcome, reason_code, received_at, retention_expires_at,
			redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, connector_id, workspace_id, conversation_id, message_id, event_id) DO UPDATE SET
			route_outcome = excluded.route_outcome,
			reason_code = excluded.reason_code,
			received_at = excluded.received_at,
			retention_expires_at = excluded.retention_expires_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, record.TenantID, record.ConnectorID, record.WorkspaceID, record.ConversationID,
		record.MessageID, record.EventID, record.RouteOutcome, nullString(record.ReasonCode),
		record.ReceivedAt.UTC().Format(time.RFC3339Nano), record.RetentionExpiresAt.UTC().Format(time.RFC3339Nano),
		record.RedactionStatus, string(document))
	if err != nil {
		return fmt.Errorf("save slack event evidence %s: %w", record.EventID, err)
	}
	return nil
}

func (s *SQLiteStore) ListSlackEventEvidence(ctx context.Context, tenantID, connectorID string, now time.Time, limit int) ([]SlackEventEvidenceRecord, error) {
	if s == nil {
		return nil, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM slack_event_evidence
		WHERE tenant_id = ? AND connector_id = ? AND retention_expires_at > ?
		ORDER BY received_at DESC, event_id DESC
		LIMIT ?
	`, strings.TrimSpace(tenantID), strings.TrimSpace(connectorID), now.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, fmt.Errorf("list slack event evidence: %w", err)
	}
	defer rows.Close()
	items := make([]SlackEventEvidenceRecord, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item SlackEventEvidenceRecord
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode slack event evidence: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func normalizeSlackHostedSetupRecord(ctx context.Context, s *SQLiteStore, record SlackHostedSetupRecord) SlackHostedSetupRecord {
	now := time.Now().UTC()
	record.TenantID = coalesceString(record.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)))
	record.ConnectorKind = coalesceString(record.ConnectorKind, "slack")
	record.Status = coalesceString(record.Status, "degraded")
	record.TerminalState = coalesceString(record.TerminalState, "action-required")
	record.OAuthState = coalesceString(record.OAuthState, "grant_missing")
	record.RoutePolicyState = coalesceString(record.RoutePolicyState, "none")
	record.RedactionStatus = coalesceString(record.RedactionStatus, "redacted")
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = now
	}
	if record.RetentionExpiresAt.IsZero() {
		record.RetentionExpiresAt = record.UpdatedAt.Add(90 * 24 * time.Hour)
	}
	return record
}

func normalizeSlackRoutePolicyRecord(ctx context.Context, s *SQLiteStore, record SlackRoutePolicyRecord) SlackRoutePolicyRecord {
	record.TenantID = coalesceString(record.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)))
	record.MentionGate = coalesceString(record.MentionGate, "agent_mention_required")
	record.ThreadReplyMode = coalesceString(record.ThreadReplyMode, "channel_mentions_thread_rooted")
	record.ValidationState = coalesceString(record.ValidationState, "blocked")
	record.RedactionStatus = coalesceString(record.RedactionStatus, "redacted")
	if record.ValidatedAt.IsZero() {
		record.ValidatedAt = time.Now().UTC()
	}
	return record
}

func normalizeSlackSmokeEvidenceRecord(ctx context.Context, s *SQLiteStore, record SlackSmokeEvidenceRecord) SlackSmokeEvidenceRecord {
	now := time.Now().UTC()
	record.TenantID = coalesceString(record.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)))
	record.Status = coalesceString(record.Status, "skipped")
	record.AuthorizationMode = coalesceString(record.AuthorizationMode, "unavailable")
	record.Owner = coalesceString(record.Owner, "operator")
	record.Reason = coalesceString(record.Reason, "safe_slack_authorization_unavailable")
	record.RedactionStatus = coalesceString(record.RedactionStatus, "redacted")
	if record.SmokeEvidenceID == "" {
		record.SmokeEvidenceID = newStoreID("slack_smoke")
	}
	if record.ValidatedAt.IsZero() {
		record.ValidatedAt = now
	}
	if record.RetentionExpiresAt.IsZero() {
		record.RetentionExpiresAt = record.ValidatedAt.Add(90 * 24 * time.Hour)
	}
	return record
}

func normalizeSlackEventEvidenceRecord(ctx context.Context, s *SQLiteStore, record SlackEventEvidenceRecord) SlackEventEvidenceRecord {
	now := time.Now().UTC()
	record.TenantID = coalesceString(record.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)))
	record.RouteOutcome = coalesceString(record.RouteOutcome, "accepted")
	record.RedactionStatus = coalesceString(record.RedactionStatus, "redacted")
	if record.ReceivedAt.IsZero() {
		record.ReceivedAt = now
	}
	if record.RetentionExpiresAt.IsZero() {
		record.RetentionExpiresAt = record.ReceivedAt.Add(90 * 24 * time.Hour)
	}
	return record
}
