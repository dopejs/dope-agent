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

type MatrixHostedSetupRecord struct {
	TenantID            string                         `json:"tenantId,omitempty"`
	ConnectorID         string                         `json:"connectorId"`
	ConnectorKind       string                         `json:"connectorKind"`
	DisplayName         string                         `json:"displayName"`
	Status              string                         `json:"status"`
	TerminalState       string                         `json:"terminalState"`
	BotCredentialState  string                         `json:"botCredentialState"`
	HomeserverState     string                         `json:"homeserverState"`
	RoutePolicyState    string                         `json:"routePolicyState"`
	DeliveryEligible    bool                           `json:"deliveryEligible"`
	HomeserverBindingID string                         `json:"homeserverBindingId"`
	ReasonCode          string                         `json:"reasonCode,omitempty"`
	RedactionStatus     string                         `json:"redactionStatus"`
	CreatedAt           time.Time                      `json:"createdAt"`
	UpdatedAt           time.Time                      `json:"updatedAt"`
	ValidatedAt         time.Time                      `json:"validatedAt,omitempty"`
	RetentionExpiresAt  time.Time                      `json:"retentionExpiresAt"`
	HomeserverBinding   *MatrixHomeserverBindingRecord `json:"homeserverBinding,omitempty"`
	RoutePolicy         *MatrixRoutePolicyRecord       `json:"routePolicy,omitempty"`
}

type MatrixHomeserverBindingRecord struct {
	TenantID                  string            `json:"tenantId,omitempty"`
	ConnectorID               string            `json:"connectorId,omitempty"`
	HomeserverBindingID       string            `json:"homeserverBindingId,omitempty"`
	HomeserverURL             string            `json:"homeserverUrl"`
	HomeserverName            string            `json:"homeserverName,omitempty"`
	BotUserID                 string            `json:"botUserId"`
	BotDeviceID               string            `json:"botDeviceId,omitempty"`
	AuthorizationState        string            `json:"authorizationState"`
	HomeserverCapabilityState string            `json:"homeserverCapabilityState"`
	ValidatedAt               time.Time         `json:"validatedAt"`
	RedactionStatus           string            `json:"redactionStatus"`
	SafeEvidence              map[string]string `json:"safeEvidence,omitempty"`
}

type MatrixRoutePolicyRecord struct {
	TenantID            string                          `json:"tenantId,omitempty"`
	ConnectorID         string                          `json:"connectorId"`
	HomeserverBindingID string                          `json:"homeserverBindingId"`
	SelectedRooms       []MatrixConversationRouteRecord `json:"selectedRooms"`
	AllowedDirectUsers  []string                        `json:"allowedDirectUsers"`
	RoomInvocationGate  string                          `json:"roomInvocationGate"`
	ConfiguredCommands  []string                        `json:"configuredCommands"`
	EncryptedRoomPolicy string                          `json:"encryptedRoomPolicy"`
	ValidationState     string                          `json:"validationState"`
	ReasonCode          string                          `json:"reasonCode,omitempty"`
	ValidatedAt         time.Time                       `json:"validatedAt"`
	RedactionStatus     string                          `json:"redactionStatus"`
	SafeEvidence        map[string]string               `json:"safeEvidence,omitempty"`
}

type MatrixConversationRouteRecord struct {
	ConversationID     string            `json:"conversationId"`
	ConversationType   string            `json:"conversationType"`
	RoomSelectionState string            `json:"roomSelectionState"`
	ValidationState    string            `json:"validationState"`
	ReasonCode         string            `json:"reasonCode,omitempty"`
	RedactionStatus    string            `json:"redactionStatus,omitempty"`
	SafeEvidence       map[string]string `json:"safeEvidence,omitempty"`
}

type MatrixSmokeEvidenceRecord struct {
	SmokeEvidenceID     string            `json:"smokeEvidenceId"`
	TenantID            string            `json:"tenantId"`
	ConnectorID         string            `json:"connectorId"`
	HomeserverBindingID string            `json:"homeserverBindingId"`
	Status              string            `json:"status"`
	AuthorizationMode   string            `json:"authorizationMode"`
	Owner               string            `json:"owner"`
	Reason              string            `json:"reason"`
	RemainingRisk       string            `json:"remainingRisk,omitempty"`
	ValidatedAt         time.Time         `json:"validatedAt"`
	RetentionExpiresAt  time.Time         `json:"retentionExpiresAt"`
	RedactionStatus     string            `json:"redactionStatus"`
	SafeEvidence        map[string]string `json:"safeEvidence,omitempty"`
}

type MatrixEventEvidenceRecord struct {
	TenantID           string            `json:"tenantId"`
	ConnectorID        string            `json:"connectorId"`
	HomeserverID       string            `json:"homeserverId"`
	ConversationID     string            `json:"conversationId"`
	MatrixEventID      string            `json:"matrixEventId"`
	SyncBatchID        string            `json:"syncBatchId,omitempty"`
	TransactionID      string            `json:"transactionId,omitempty"`
	RouteOutcome       string            `json:"routeOutcome"`
	ReasonCode         string            `json:"reasonCode,omitempty"`
	ReceivedAt         time.Time         `json:"receivedAt"`
	RetentionExpiresAt time.Time         `json:"retentionExpiresAt"`
	RedactionStatus    string            `json:"redactionStatus"`
	SafeEvidence       map[string]string `json:"safeEvidence,omitempty"`
}

func (s *SQLiteStore) SaveMatrixHostedSetup(ctx context.Context, record MatrixHostedSetupRecord) error {
	if s == nil {
		return nil
	}
	record = normalizeMatrixHostedSetupRecord(ctx, s, record)
	document, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal matrix hosted setup: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO matrix_hosted_setups (
			tenant_id, connector_id, connector_kind, display_name, status, terminal_state,
			bot_credential_state, homeserver_state, route_policy_state, delivery_eligible,
			homeserver_binding_id, reason_code, redaction_status, created_at, updated_at,
			validated_at, retention_expires_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, connector_id) DO UPDATE SET
			connector_kind = excluded.connector_kind,
			display_name = excluded.display_name,
			status = excluded.status,
			terminal_state = excluded.terminal_state,
			bot_credential_state = excluded.bot_credential_state,
			homeserver_state = excluded.homeserver_state,
			route_policy_state = excluded.route_policy_state,
			delivery_eligible = excluded.delivery_eligible,
			homeserver_binding_id = excluded.homeserver_binding_id,
			reason_code = excluded.reason_code,
			redaction_status = excluded.redaction_status,
			updated_at = excluded.updated_at,
			validated_at = excluded.validated_at,
			retention_expires_at = excluded.retention_expires_at,
			document_json = excluded.document_json
	`, record.TenantID, record.ConnectorID, record.ConnectorKind, record.DisplayName,
		record.Status, record.TerminalState, record.BotCredentialState, record.HomeserverState,
		record.RoutePolicyState, boolToInt(record.DeliveryEligible), record.HomeserverBindingID,
		nullString(record.ReasonCode), record.RedactionStatus, record.CreatedAt.UTC().Format(time.RFC3339Nano),
		record.UpdatedAt.UTC().Format(time.RFC3339Nano), nullableTimeValue(record.ValidatedAt),
		record.RetentionExpiresAt.UTC().Format(time.RFC3339Nano), string(document))
	if err != nil {
		return fmt.Errorf("save matrix hosted setup %s: %w", record.ConnectorID, err)
	}
	return nil
}

func (s *SQLiteStore) GetMatrixHostedSetup(ctx context.Context, tenantID, connectorID string) (MatrixHostedSetupRecord, bool, error) {
	if s == nil {
		return MatrixHostedSetupRecord{}, false, nil
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM matrix_hosted_setups
		WHERE tenant_id = ? AND connector_id = ?
	`, strings.TrimSpace(tenantID), strings.TrimSpace(connectorID)).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MatrixHostedSetupRecord{}, false, nil
		}
		return MatrixHostedSetupRecord{}, false, fmt.Errorf("get matrix hosted setup %s: %w", connectorID, err)
	}
	var record MatrixHostedSetupRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return MatrixHostedSetupRecord{}, false, fmt.Errorf("decode matrix hosted setup %s: %w", connectorID, err)
	}
	policy, ok, err := s.GetMatrixRoutePolicy(ctx, tenantID, connectorID)
	if err != nil {
		return MatrixHostedSetupRecord{}, false, err
	}
	if ok {
		record.RoutePolicy = &policy
	}
	return record, true, nil
}

func (s *SQLiteStore) SaveMatrixRoutePolicy(ctx context.Context, record MatrixRoutePolicyRecord) error {
	if s == nil {
		return nil
	}
	record = normalizeMatrixRoutePolicyRecord(ctx, s, record)
	document, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal matrix route policy: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO matrix_route_policies (
			tenant_id, connector_id, homeserver_binding_id, validation_state, reason_code,
			validated_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, connector_id) DO UPDATE SET
			homeserver_binding_id = excluded.homeserver_binding_id,
			validation_state = excluded.validation_state,
			reason_code = excluded.reason_code,
			validated_at = excluded.validated_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, record.TenantID, record.ConnectorID, record.HomeserverBindingID, record.ValidationState,
		nullString(record.ReasonCode), record.ValidatedAt.UTC().Format(time.RFC3339Nano),
		record.RedactionStatus, string(document))
	if err != nil {
		return fmt.Errorf("save matrix route policy %s: %w", record.ConnectorID, err)
	}
	return nil
}

func (s *SQLiteStore) GetMatrixRoutePolicy(ctx context.Context, tenantID, connectorID string) (MatrixRoutePolicyRecord, bool, error) {
	if s == nil {
		return MatrixRoutePolicyRecord{}, false, nil
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM matrix_route_policies
		WHERE tenant_id = ? AND connector_id = ?
	`, strings.TrimSpace(tenantID), strings.TrimSpace(connectorID)).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MatrixRoutePolicyRecord{}, false, nil
		}
		return MatrixRoutePolicyRecord{}, false, fmt.Errorf("get matrix route policy %s: %w", connectorID, err)
	}
	var record MatrixRoutePolicyRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return MatrixRoutePolicyRecord{}, false, fmt.Errorf("decode matrix route policy %s: %w", connectorID, err)
	}
	return record, true, nil
}

func (s *SQLiteStore) SaveMatrixSmokeEvidence(ctx context.Context, record MatrixSmokeEvidenceRecord) error {
	if s == nil {
		return nil
	}
	record = normalizeMatrixSmokeEvidenceRecord(ctx, s, record)
	document, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal matrix smoke evidence: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO matrix_smoke_evidence (
			smoke_evidence_id, tenant_id, connector_id, homeserver_binding_id, status,
			authorization_mode, owner, reason, remaining_risk, validated_at,
			retention_expires_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(smoke_evidence_id) DO UPDATE SET
			status = excluded.status,
			authorization_mode = excluded.authorization_mode,
			owner = excluded.owner,
			reason = excluded.reason,
			remaining_risk = excluded.remaining_risk,
			validated_at = excluded.validated_at,
			retention_expires_at = excluded.retention_expires_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, record.SmokeEvidenceID, record.TenantID, record.ConnectorID, record.HomeserverBindingID,
		record.Status, record.AuthorizationMode, record.Owner, record.Reason, nullString(record.RemainingRisk),
		record.ValidatedAt.UTC().Format(time.RFC3339Nano), record.RetentionExpiresAt.UTC().Format(time.RFC3339Nano),
		record.RedactionStatus, string(document))
	if err != nil {
		return fmt.Errorf("save matrix smoke evidence %s: %w", record.SmokeEvidenceID, err)
	}
	return nil
}

func (s *SQLiteStore) LatestMatrixSmokeEvidence(ctx context.Context, tenantID, connectorID string, now time.Time) (MatrixSmokeEvidenceRecord, bool, error) {
	if s == nil {
		return MatrixSmokeEvidenceRecord{}, false, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM matrix_smoke_evidence
		WHERE tenant_id = ? AND connector_id = ? AND retention_expires_at > ?
		ORDER BY validated_at DESC, smoke_evidence_id DESC
		LIMIT 1
	`, strings.TrimSpace(tenantID), strings.TrimSpace(connectorID), now.UTC().Format(time.RFC3339Nano)).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MatrixSmokeEvidenceRecord{}, false, nil
		}
		return MatrixSmokeEvidenceRecord{}, false, fmt.Errorf("latest matrix smoke evidence %s: %w", connectorID, err)
	}
	var record MatrixSmokeEvidenceRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return MatrixSmokeEvidenceRecord{}, false, fmt.Errorf("decode matrix smoke evidence: %w", err)
	}
	return record, true, nil
}

func (s *SQLiteStore) SaveMatrixEventEvidence(ctx context.Context, record MatrixEventEvidenceRecord) error {
	if s == nil {
		return nil
	}
	record = normalizeMatrixEventEvidenceRecord(ctx, s, record)
	document, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal matrix event evidence: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO matrix_event_evidence (
			tenant_id, connector_id, homeserver_id, conversation_id, matrix_event_id,
			sync_batch_id, transaction_id, route_outcome, reason_code, received_at,
			retention_expires_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, connector_id, homeserver_id, conversation_id, matrix_event_id) DO UPDATE SET
			sync_batch_id = excluded.sync_batch_id,
			transaction_id = excluded.transaction_id,
			route_outcome = excluded.route_outcome,
			reason_code = excluded.reason_code,
			received_at = excluded.received_at,
			retention_expires_at = excluded.retention_expires_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, record.TenantID, record.ConnectorID, record.HomeserverID, record.ConversationID, record.MatrixEventID,
		nullString(record.SyncBatchID), nullString(record.TransactionID), record.RouteOutcome, nullString(record.ReasonCode),
		record.ReceivedAt.UTC().Format(time.RFC3339Nano), record.RetentionExpiresAt.UTC().Format(time.RFC3339Nano),
		record.RedactionStatus, string(document))
	if err != nil {
		return fmt.Errorf("save matrix event evidence %s: %w", record.MatrixEventID, err)
	}
	return nil
}

func (s *SQLiteStore) ListMatrixEventEvidence(ctx context.Context, tenantID, connectorID string, now time.Time, limit int) ([]MatrixEventEvidenceRecord, error) {
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
		FROM matrix_event_evidence
		WHERE tenant_id = ? AND connector_id = ? AND retention_expires_at > ?
		ORDER BY received_at DESC, matrix_event_id DESC
		LIMIT ?
	`, strings.TrimSpace(tenantID), strings.TrimSpace(connectorID), now.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, fmt.Errorf("list matrix event evidence: %w", err)
	}
	defer rows.Close()
	items := make([]MatrixEventEvidenceRecord, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item MatrixEventEvidenceRecord
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode matrix event evidence: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func normalizeMatrixHostedSetupRecord(ctx context.Context, s *SQLiteStore, record MatrixHostedSetupRecord) MatrixHostedSetupRecord {
	now := time.Now().UTC()
	record.TenantID = coalesceString(record.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)))
	record.ConnectorKind = coalesceString(record.ConnectorKind, "matrix")
	record.Status = coalesceString(record.Status, "degraded")
	record.TerminalState = coalesceString(record.TerminalState, "action-required")
	record.BotCredentialState = coalesceString(record.BotCredentialState, "unknown")
	record.HomeserverState = coalesceString(record.HomeserverState, "unknown")
	record.RoutePolicyState = coalesceString(record.RoutePolicyState, "none")
	record.RedactionStatus = coalesceString(record.RedactionStatus, "redacted")
	if record.HomeserverBindingID == "" && record.ConnectorID != "" {
		record.HomeserverBindingID = "matrix_homeserver_" + record.ConnectorID
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = now
	}
	if record.RetentionExpiresAt.IsZero() {
		record.RetentionExpiresAt = now.Add(90 * 24 * time.Hour)
	}
	return record
}

func normalizeMatrixRoutePolicyRecord(ctx context.Context, s *SQLiteStore, record MatrixRoutePolicyRecord) MatrixRoutePolicyRecord {
	now := time.Now().UTC()
	record.TenantID = coalesceString(record.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)))
	record.RoomInvocationGate = coalesceString(record.RoomInvocationGate, "bot_mention_or_command_required")
	record.EncryptedRoomPolicy = coalesceString(record.EncryptedRoomPolicy, "unsupported")
	record.ValidationState = coalesceString(record.ValidationState, "valid")
	record.RedactionStatus = coalesceString(record.RedactionStatus, "redacted")
	if record.ValidatedAt.IsZero() {
		record.ValidatedAt = now
	}
	for i := range record.SelectedRooms {
		record.SelectedRooms[i].ConversationType = coalesceString(record.SelectedRooms[i].ConversationType, "room")
		record.SelectedRooms[i].RoomSelectionState = coalesceString(record.SelectedRooms[i].RoomSelectionState, "selected")
		record.SelectedRooms[i].ValidationState = coalesceString(record.SelectedRooms[i].ValidationState, "valid")
		record.SelectedRooms[i].RedactionStatus = coalesceString(record.SelectedRooms[i].RedactionStatus, "redacted")
	}
	return record
}

func normalizeMatrixSmokeEvidenceRecord(ctx context.Context, s *SQLiteStore, record MatrixSmokeEvidenceRecord) MatrixSmokeEvidenceRecord {
	now := time.Now().UTC()
	record.TenantID = coalesceString(record.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)))
	record.Status = coalesceString(record.Status, "skipped")
	record.AuthorizationMode = coalesceString(record.AuthorizationMode, "unavailable")
	record.Owner = coalesceString(record.Owner, "operator")
	record.Reason = coalesceString(record.Reason, "safe_matrix_authorization_unavailable")
	record.RedactionStatus = coalesceString(record.RedactionStatus, "redacted")
	if record.SmokeEvidenceID == "" {
		record.SmokeEvidenceID = newStoreID("matrix_smoke")
	}
	if record.ValidatedAt.IsZero() {
		record.ValidatedAt = now
	}
	if record.RetentionExpiresAt.IsZero() {
		record.RetentionExpiresAt = record.ValidatedAt.Add(90 * 24 * time.Hour)
	}
	return record
}

func normalizeMatrixEventEvidenceRecord(ctx context.Context, s *SQLiteStore, record MatrixEventEvidenceRecord) MatrixEventEvidenceRecord {
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
