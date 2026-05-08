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

type TelegramHostedSetupRecord struct {
	TenantID           string                          `json:"tenantId,omitempty"`
	ConnectorID        string                          `json:"connectorId"`
	ConnectorKind      string                          `json:"connectorKind"`
	DisplayName        string                          `json:"displayName"`
	Status             string                          `json:"status"`
	TerminalState      string                          `json:"terminalState"`
	HostedReady        bool                            `json:"hostedReady"`
	CredentialState    string                          `json:"credentialState"`
	AllowmentState     string                          `json:"allowmentState"`
	GroupBehavior      string                          `json:"groupBehavior"`
	DeliveryEligible   bool                            `json:"deliveryEligible"`
	ReasonCode         string                          `json:"reasonCode,omitempty"`
	RedactionStatus    string                          `json:"redactionStatus"`
	CreatedAt          time.Time                       `json:"createdAt"`
	UpdatedAt          time.Time                       `json:"updatedAt"`
	ValidatedAt        time.Time                       `json:"validatedAt,omitempty"`
	RetentionExpiresAt time.Time                       `json:"retentionExpiresAt"`
	AccountBinding     *ConnectorAccountBindingSummary `json:"accountBinding,omitempty"`
	Allowments         []TelegramAllowmentRecord       `json:"allowments,omitempty"`
}

type ConnectorAccountBindingSummary struct {
	TenantID            string    `json:"tenantId,omitempty"`
	ConnectorID         string    `json:"connectorId"`
	ConnectorAccountID  string    `json:"connectorAccountId"`
	DisplayName         string    `json:"displayName,omitempty"`
	ProviderAccountHint string    `json:"providerAccountHint,omitempty"`
	RedactionStatus     string    `json:"redactionStatus"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type TelegramAllowmentRecord struct {
	TenantID        string            `json:"tenantId,omitempty"`
	ConnectorID     string            `json:"connectorId"`
	AllowmentID     string            `json:"allowmentId"`
	ScopeType       string            `json:"telegramScopeType"`
	ScopeID         string            `json:"telegramScopeId"`
	ProviderLabel   string            `json:"providerLabel,omitempty"`
	Enabled         bool              `json:"enabled"`
	GroupGate       string            `json:"groupGate"`
	ValidationState string            `json:"validationState"`
	ReasonCode      string            `json:"reasonCode,omitempty"`
	ValidatedAt     time.Time         `json:"validatedAt"`
	RedactionStatus string            `json:"redactionStatus"`
	SafeEvidence    map[string]string `json:"safeEvidence,omitempty"`
}

type TelegramSmokeEvidenceRecord struct {
	SmokeEvidenceID    string            `json:"smokeEvidenceId"`
	TenantID           string            `json:"tenantId"`
	ConnectorID        string            `json:"connectorId"`
	Status             string            `json:"status"`
	CredentialMode     string            `json:"credentialMode"`
	Owner              string            `json:"owner"`
	Reason             string            `json:"reason"`
	RemainingRisk      string            `json:"remainingRisk,omitempty"`
	ValidatedAt        time.Time         `json:"validatedAt"`
	RetentionExpiresAt time.Time         `json:"retentionExpiresAt"`
	RedactionStatus    string            `json:"redactionStatus"`
	SafeEvidence       map[string]string `json:"safeEvidence,omitempty"`
}

type TelegramUpdateEvidenceRecord struct {
	TenantID           string            `json:"tenantId"`
	ConnectorID        string            `json:"connectorId"`
	ChatID             string            `json:"telegramChatId"`
	MessageID          string            `json:"telegramMessageId"`
	UpdateID           string            `json:"telegramUpdateId"`
	RouteOutcome       string            `json:"routeOutcome"`
	ReasonCode         string            `json:"reasonCode,omitempty"`
	ReceivedAt         time.Time         `json:"receivedAt"`
	RetentionExpiresAt time.Time         `json:"retentionExpiresAt"`
	RedactionStatus    string            `json:"redactionStatus"`
	SafeEvidence       map[string]string `json:"safeEvidence,omitempty"`
}

func (s *SQLiteStore) SaveTelegramHostedSetup(ctx context.Context, record TelegramHostedSetupRecord) error {
	if s == nil {
		return nil
	}
	record = normalizeTelegramHostedSetupRecord(ctx, s, record)
	document, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal telegram hosted setup: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO telegram_hosted_setups (
			tenant_id, connector_id, connector_kind, display_name, status, terminal_state,
			credential_state, allowment_state, group_behavior, delivery_eligible, reason_code,
			redaction_status, created_at, updated_at, validated_at, retention_expires_at,
			document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, connector_id) DO UPDATE SET
			connector_kind = excluded.connector_kind,
			display_name = excluded.display_name,
			status = excluded.status,
			terminal_state = excluded.terminal_state,
			credential_state = excluded.credential_state,
			allowment_state = excluded.allowment_state,
			group_behavior = excluded.group_behavior,
			delivery_eligible = excluded.delivery_eligible,
			reason_code = excluded.reason_code,
			redaction_status = excluded.redaction_status,
			updated_at = excluded.updated_at,
			validated_at = excluded.validated_at,
			retention_expires_at = excluded.retention_expires_at,
			document_json = excluded.document_json
	`, record.TenantID, record.ConnectorID, record.ConnectorKind, record.DisplayName,
		record.Status, record.TerminalState, record.CredentialState, record.AllowmentState,
		record.GroupBehavior, boolToInt(record.DeliveryEligible), nullString(record.ReasonCode),
		record.RedactionStatus, record.CreatedAt.UTC().Format(time.RFC3339Nano),
		record.UpdatedAt.UTC().Format(time.RFC3339Nano), nullableTimeValue(record.ValidatedAt),
		record.RetentionExpiresAt.UTC().Format(time.RFC3339Nano), string(document))
	if err != nil {
		return fmt.Errorf("save telegram hosted setup %s: %w", record.ConnectorID, err)
	}
	return nil
}

func (s *SQLiteStore) GetTelegramHostedSetup(ctx context.Context, tenantID, connectorID string) (TelegramHostedSetupRecord, bool, error) {
	if s == nil {
		return TelegramHostedSetupRecord{}, false, nil
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM telegram_hosted_setups
		WHERE tenant_id = ? AND connector_id = ?
	`, strings.TrimSpace(tenantID), strings.TrimSpace(connectorID)).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TelegramHostedSetupRecord{}, false, nil
		}
		return TelegramHostedSetupRecord{}, false, fmt.Errorf("get telegram hosted setup %s: %w", connectorID, err)
	}
	var record TelegramHostedSetupRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return TelegramHostedSetupRecord{}, false, fmt.Errorf("decode telegram hosted setup %s: %w", connectorID, err)
	}
	allowments, err := s.ListTelegramAllowments(ctx, tenantID, connectorID)
	if err != nil {
		return TelegramHostedSetupRecord{}, false, err
	}
	record.Allowments = allowments
	return record, true, nil
}

func (s *SQLiteStore) SaveTelegramAllowment(ctx context.Context, record TelegramAllowmentRecord) error {
	if s == nil {
		return nil
	}
	record = normalizeTelegramAllowmentRecord(ctx, s, record)
	document, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal telegram allowment: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO telegram_allowments (
			tenant_id, connector_id, allowment_id, scope_type, scope_id, provider_label,
			enabled, group_gate, validation_state, reason_code, validated_at, redaction_status,
			document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, connector_id, allowment_id) DO UPDATE SET
			scope_type = excluded.scope_type,
			scope_id = excluded.scope_id,
			provider_label = excluded.provider_label,
			enabled = excluded.enabled,
			group_gate = excluded.group_gate,
			validation_state = excluded.validation_state,
			reason_code = excluded.reason_code,
			validated_at = excluded.validated_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, record.TenantID, record.ConnectorID, record.AllowmentID, record.ScopeType, record.ScopeID,
		nullString(record.ProviderLabel), boolToInt(record.Enabled), record.GroupGate,
		record.ValidationState, nullString(record.ReasonCode), record.ValidatedAt.UTC().Format(time.RFC3339Nano),
		record.RedactionStatus, string(document))
	if err != nil {
		return fmt.Errorf("save telegram allowment %s: %w", record.AllowmentID, err)
	}
	return nil
}

func (s *SQLiteStore) ListTelegramAllowments(ctx context.Context, tenantID, connectorID string) ([]TelegramAllowmentRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM telegram_allowments
		WHERE tenant_id = ? AND connector_id = ?
		ORDER BY scope_type ASC, allowment_id ASC
	`, strings.TrimSpace(tenantID), strings.TrimSpace(connectorID))
	if err != nil {
		return nil, fmt.Errorf("list telegram allowments: %w", err)
	}
	defer rows.Close()
	items := make([]TelegramAllowmentRecord, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item TelegramAllowmentRecord
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode telegram allowment: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) SaveTelegramSmokeEvidence(ctx context.Context, record TelegramSmokeEvidenceRecord) error {
	if s == nil {
		return nil
	}
	record = normalizeTelegramSmokeEvidenceRecord(ctx, s, record)
	document, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal telegram smoke evidence: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO telegram_smoke_evidence (
			smoke_evidence_id, tenant_id, connector_id, status, credential_mode, owner,
			reason, remaining_risk, validated_at, retention_expires_at, redaction_status,
			document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(smoke_evidence_id) DO UPDATE SET
			tenant_id = excluded.tenant_id,
			connector_id = excluded.connector_id,
			status = excluded.status,
			credential_mode = excluded.credential_mode,
			owner = excluded.owner,
			reason = excluded.reason,
			remaining_risk = excluded.remaining_risk,
			validated_at = excluded.validated_at,
			retention_expires_at = excluded.retention_expires_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, record.SmokeEvidenceID, record.TenantID, record.ConnectorID, record.Status,
		record.CredentialMode, record.Owner, record.Reason, nullString(record.RemainingRisk),
		record.ValidatedAt.UTC().Format(time.RFC3339Nano), record.RetentionExpiresAt.UTC().Format(time.RFC3339Nano),
		record.RedactionStatus, string(document))
	if err != nil {
		return fmt.Errorf("save telegram smoke evidence %s: %w", record.SmokeEvidenceID, err)
	}
	return nil
}

func (s *SQLiteStore) LatestTelegramSmokeEvidence(ctx context.Context, tenantID, connectorID string, now time.Time) (TelegramSmokeEvidenceRecord, bool, error) {
	if s == nil {
		return TelegramSmokeEvidenceRecord{}, false, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM telegram_smoke_evidence
		WHERE tenant_id = ? AND connector_id = ? AND retention_expires_at > ?
		ORDER BY validated_at DESC, smoke_evidence_id DESC
		LIMIT 1
	`, strings.TrimSpace(tenantID), strings.TrimSpace(connectorID), now.UTC().Format(time.RFC3339Nano)).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TelegramSmokeEvidenceRecord{}, false, nil
		}
		return TelegramSmokeEvidenceRecord{}, false, fmt.Errorf("latest telegram smoke evidence %s: %w", connectorID, err)
	}
	var record TelegramSmokeEvidenceRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return TelegramSmokeEvidenceRecord{}, false, fmt.Errorf("decode telegram smoke evidence: %w", err)
	}
	return record, true, nil
}

func (s *SQLiteStore) SaveTelegramUpdateEvidence(ctx context.Context, record TelegramUpdateEvidenceRecord) error {
	if s == nil {
		return nil
	}
	record = normalizeTelegramUpdateEvidenceRecord(ctx, s, record)
	document, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal telegram update evidence: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO telegram_update_evidence (
			tenant_id, connector_id, chat_id, message_id, update_id, route_outcome,
			reason_code, received_at, retention_expires_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, connector_id, chat_id, message_id, update_id) DO UPDATE SET
			route_outcome = excluded.route_outcome,
			reason_code = excluded.reason_code,
			received_at = excluded.received_at,
			retention_expires_at = excluded.retention_expires_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, record.TenantID, record.ConnectorID, record.ChatID, record.MessageID, record.UpdateID,
		record.RouteOutcome, nullString(record.ReasonCode), record.ReceivedAt.UTC().Format(time.RFC3339Nano),
		record.RetentionExpiresAt.UTC().Format(time.RFC3339Nano), record.RedactionStatus, string(document))
	if err != nil {
		return fmt.Errorf("save telegram update evidence %s/%s/%s: %w", record.ChatID, record.MessageID, record.UpdateID, err)
	}
	return nil
}

func (s *SQLiteStore) ListTelegramUpdateEvidence(ctx context.Context, tenantID, connectorID string, now time.Time, limit int) ([]TelegramUpdateEvidenceRecord, error) {
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
		FROM telegram_update_evidence
		WHERE tenant_id = ? AND connector_id = ? AND retention_expires_at > ?
		ORDER BY received_at DESC, update_id DESC
		LIMIT ?
	`, strings.TrimSpace(tenantID), strings.TrimSpace(connectorID), now.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, fmt.Errorf("list telegram update evidence: %w", err)
	}
	defer rows.Close()
	items := make([]TelegramUpdateEvidenceRecord, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item TelegramUpdateEvidenceRecord
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode telegram update evidence: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func normalizeTelegramHostedSetupRecord(ctx context.Context, s *SQLiteStore, record TelegramHostedSetupRecord) TelegramHostedSetupRecord {
	now := time.Now().UTC()
	record.TenantID = coalesceString(record.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)))
	record.ConnectorKind = coalesceString(record.ConnectorKind, "telegram")
	record.Status = coalesceString(record.Status, "degraded")
	record.TerminalState = coalesceString(record.TerminalState, "action-required")
	record.CredentialState = coalesceString(record.CredentialState, "missing")
	record.AllowmentState = coalesceString(record.AllowmentState, "none")
	record.GroupBehavior = coalesceString(record.GroupBehavior, "disabled")
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
	if record.AccountBinding != nil {
		record.AccountBinding.TenantID = coalesceString(record.AccountBinding.TenantID, record.TenantID)
		record.AccountBinding.ConnectorID = coalesceString(record.AccountBinding.ConnectorID, record.ConnectorID)
		record.AccountBinding.RedactionStatus = coalesceString(record.AccountBinding.RedactionStatus, "redacted")
		if record.AccountBinding.UpdatedAt.IsZero() {
			record.AccountBinding.UpdatedAt = record.UpdatedAt
		}
	}
	record.HostedReady = record.TerminalState == "ready"
	return record
}

func normalizeTelegramAllowmentRecord(ctx context.Context, s *SQLiteStore, record TelegramAllowmentRecord) TelegramAllowmentRecord {
	record.TenantID = coalesceString(record.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)))
	record.ScopeType = coalesceString(record.ScopeType, "direct_chat")
	record.GroupGate = coalesceString(record.GroupGate, "not_applicable")
	record.ValidationState = coalesceString(record.ValidationState, "invalid")
	record.RedactionStatus = coalesceString(record.RedactionStatus, "redacted")
	if record.AllowmentID == "" {
		record.AllowmentID = newStoreID("telegram_allowment")
	}
	if record.ValidatedAt.IsZero() {
		record.ValidatedAt = time.Now().UTC()
	}
	return record
}

func normalizeTelegramSmokeEvidenceRecord(ctx context.Context, s *SQLiteStore, record TelegramSmokeEvidenceRecord) TelegramSmokeEvidenceRecord {
	now := time.Now().UTC()
	record.TenantID = coalesceString(record.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)))
	record.Status = coalesceString(record.Status, "skipped")
	record.CredentialMode = coalesceString(record.CredentialMode, "unavailable")
	record.Owner = coalesceString(record.Owner, "operator")
	record.Reason = coalesceString(record.Reason, "safe_credentials_unavailable")
	record.RedactionStatus = coalesceString(record.RedactionStatus, "redacted")
	if record.SmokeEvidenceID == "" {
		record.SmokeEvidenceID = newStoreID("telegram_smoke")
	}
	if record.ValidatedAt.IsZero() {
		record.ValidatedAt = now
	}
	if record.RetentionExpiresAt.IsZero() {
		record.RetentionExpiresAt = record.ValidatedAt.Add(90 * 24 * time.Hour)
	}
	return record
}

func normalizeTelegramUpdateEvidenceRecord(ctx context.Context, s *SQLiteStore, record TelegramUpdateEvidenceRecord) TelegramUpdateEvidenceRecord {
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
