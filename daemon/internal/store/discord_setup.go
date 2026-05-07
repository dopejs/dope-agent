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

type DiscordHostedSetupRecord struct {
	TenantID           string                               `json:"tenantId,omitempty"`
	ConnectorID        string                               `json:"connectorId"`
	ConnectorKind      string                               `json:"connectorKind"`
	DisplayName        string                               `json:"displayName"`
	Status             string                               `json:"status"`
	ReadinessState     string                               `json:"readinessState"`
	HostedReady        bool                                 `json:"hostedReady"`
	CredentialState    string                               `json:"credentialState"`
	RespondInDM        bool                                 `json:"respondInDM"`
	RequireMention     bool                                 `json:"requireMention"`
	DeliveryMode       string                               `json:"deliveryMode"`
	ReasonCode         string                               `json:"reasonCode,omitempty"`
	RedactionStatus    string                               `json:"redactionStatus"`
	CreatedAt          time.Time                            `json:"createdAt"`
	UpdatedAt          time.Time                            `json:"updatedAt"`
	ValidatedAt        time.Time                            `json:"validatedAt,omitempty"`
	RetentionExpiresAt time.Time                            `json:"retentionExpiresAt"`
	Destinations       []DiscordDestinationValidationRecord `json:"destinations,omitempty"`
}

type DiscordDestinationValidationRecord struct {
	TenantID        string            `json:"tenantId,omitempty"`
	ConnectorID     string            `json:"connectorId"`
	DestinationID   string            `json:"destinationId"`
	DestinationType string            `json:"destinationType"`
	ProviderLabel   string            `json:"providerLabel,omitempty"`
	Selected        bool              `json:"selected"`
	ValidationState string            `json:"validationState"`
	ReasonCode      string            `json:"reasonCode,omitempty"`
	ValidatedAt     time.Time         `json:"validatedAt"`
	RedactionStatus string            `json:"redactionStatus"`
	SafeEvidence    map[string]string `json:"safeEvidence,omitempty"`
}

type DiscordSmokeEvidenceRecord struct {
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

func (s *SQLiteStore) SaveDiscordHostedSetup(ctx context.Context, record DiscordHostedSetupRecord) error {
	if s == nil {
		return nil
	}
	record = normalizeDiscordHostedSetupRecord(ctx, s, record)
	document, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal discord hosted setup: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO discord_hosted_setups (
			tenant_id, connector_id, connector_kind, display_name, status, readiness_state,
			credential_state, respond_in_dm, require_mention, delivery_mode, reason_code,
			redaction_status, created_at, updated_at, validated_at, retention_expires_at,
			document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, connector_id) DO UPDATE SET
			connector_kind = excluded.connector_kind,
			display_name = excluded.display_name,
			status = excluded.status,
			readiness_state = excluded.readiness_state,
			credential_state = excluded.credential_state,
			respond_in_dm = excluded.respond_in_dm,
			require_mention = excluded.require_mention,
			delivery_mode = excluded.delivery_mode,
			reason_code = excluded.reason_code,
			redaction_status = excluded.redaction_status,
			updated_at = excluded.updated_at,
			validated_at = excluded.validated_at,
			retention_expires_at = excluded.retention_expires_at,
			document_json = excluded.document_json
	`, record.TenantID, record.ConnectorID, record.ConnectorKind, record.DisplayName,
		record.Status, record.ReadinessState, record.CredentialState, boolToInt(record.RespondInDM),
		boolToInt(record.RequireMention), record.DeliveryMode, nullString(record.ReasonCode),
		record.RedactionStatus, record.CreatedAt.UTC().Format(time.RFC3339Nano),
		record.UpdatedAt.UTC().Format(time.RFC3339Nano), nullableTimeValue(record.ValidatedAt),
		record.RetentionExpiresAt.UTC().Format(time.RFC3339Nano), string(document))
	if err != nil {
		return fmt.Errorf("save discord hosted setup %s: %w", record.ConnectorID, err)
	}
	return nil
}

func (s *SQLiteStore) GetDiscordHostedSetup(ctx context.Context, tenantID, connectorID string) (DiscordHostedSetupRecord, bool, error) {
	if s == nil {
		return DiscordHostedSetupRecord{}, false, nil
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM discord_hosted_setups
		WHERE tenant_id = ? AND connector_id = ?
	`, strings.TrimSpace(tenantID), strings.TrimSpace(connectorID)).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DiscordHostedSetupRecord{}, false, nil
		}
		return DiscordHostedSetupRecord{}, false, fmt.Errorf("get discord hosted setup %s: %w", connectorID, err)
	}
	var record DiscordHostedSetupRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return DiscordHostedSetupRecord{}, false, fmt.Errorf("decode discord hosted setup %s: %w", connectorID, err)
	}
	destinations, err := s.ListDiscordDestinationValidations(ctx, tenantID, connectorID)
	if err != nil {
		return DiscordHostedSetupRecord{}, false, err
	}
	record.Destinations = destinations
	return record, true, nil
}

func (s *SQLiteStore) SaveDiscordDestinationValidation(ctx context.Context, record DiscordDestinationValidationRecord) error {
	if s == nil {
		return nil
	}
	record = normalizeDiscordDestinationValidationRecord(ctx, s, record)
	document, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal discord destination validation: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO discord_destination_validations (
			tenant_id, connector_id, destination_id, destination_type, provider_label,
			selected, validation_state, reason_code, validated_at, redaction_status,
			document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, connector_id, destination_type, destination_id) DO UPDATE SET
			provider_label = excluded.provider_label,
			selected = excluded.selected,
			validation_state = excluded.validation_state,
			reason_code = excluded.reason_code,
			validated_at = excluded.validated_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, record.TenantID, record.ConnectorID, record.DestinationID, record.DestinationType,
		nullString(record.ProviderLabel), boolToInt(record.Selected), record.ValidationState,
		nullString(record.ReasonCode), record.ValidatedAt.UTC().Format(time.RFC3339Nano),
		record.RedactionStatus, string(document))
	if err != nil {
		return fmt.Errorf("save discord destination validation %s: %w", record.DestinationID, err)
	}
	return nil
}

func (s *SQLiteStore) ListDiscordDestinationValidations(ctx context.Context, tenantID, connectorID string) ([]DiscordDestinationValidationRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM discord_destination_validations
		WHERE tenant_id = ? AND connector_id = ?
		ORDER BY destination_type ASC, destination_id ASC
	`, strings.TrimSpace(tenantID), strings.TrimSpace(connectorID))
	if err != nil {
		return nil, fmt.Errorf("list discord destination validations: %w", err)
	}
	defer rows.Close()
	items := make([]DiscordDestinationValidationRecord, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item DiscordDestinationValidationRecord
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode discord destination validation: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) SaveDiscordSmokeEvidence(ctx context.Context, record DiscordSmokeEvidenceRecord) error {
	if s == nil {
		return nil
	}
	record = normalizeDiscordSmokeEvidenceRecord(ctx, s, record)
	document, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal discord smoke evidence: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO discord_smoke_evidence (
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
		return fmt.Errorf("save discord smoke evidence %s: %w", record.SmokeEvidenceID, err)
	}
	return nil
}

func (s *SQLiteStore) LatestDiscordSmokeEvidence(ctx context.Context, tenantID, connectorID string, now time.Time) (DiscordSmokeEvidenceRecord, bool, error) {
	if s == nil {
		return DiscordSmokeEvidenceRecord{}, false, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM discord_smoke_evidence
		WHERE tenant_id = ? AND connector_id = ? AND retention_expires_at > ?
		ORDER BY validated_at DESC, smoke_evidence_id DESC
		LIMIT 1
	`, strings.TrimSpace(tenantID), strings.TrimSpace(connectorID), now.UTC().Format(time.RFC3339Nano)).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DiscordSmokeEvidenceRecord{}, false, nil
		}
		return DiscordSmokeEvidenceRecord{}, false, fmt.Errorf("latest discord smoke evidence %s: %w", connectorID, err)
	}
	var record DiscordSmokeEvidenceRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return DiscordSmokeEvidenceRecord{}, false, fmt.Errorf("decode discord smoke evidence: %w", err)
	}
	return record, true, nil
}

func normalizeDiscordHostedSetupRecord(ctx context.Context, s *SQLiteStore, record DiscordHostedSetupRecord) DiscordHostedSetupRecord {
	now := time.Now().UTC()
	record.TenantID = coalesceString(record.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)))
	record.ConnectorKind = coalesceString(record.ConnectorKind, "discord")
	record.Status = coalesceString(record.Status, "degraded")
	record.ReadinessState = coalesceString(record.ReadinessState, "degraded_needs_repair")
	record.CredentialState = coalesceString(record.CredentialState, "missing")
	record.DeliveryMode = coalesceString(record.DeliveryMode, "gateway")
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
	record.HostedReady = record.ReadinessState == "hosted_ready"
	return record
}

func normalizeDiscordDestinationValidationRecord(ctx context.Context, s *SQLiteStore, record DiscordDestinationValidationRecord) DiscordDestinationValidationRecord {
	record.TenantID = coalesceString(record.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)))
	record.ValidationState = coalesceString(record.ValidationState, "invalid")
	record.RedactionStatus = coalesceString(record.RedactionStatus, "redacted")
	if record.ValidatedAt.IsZero() {
		record.ValidatedAt = time.Now().UTC()
	}
	return record
}

func normalizeDiscordSmokeEvidenceRecord(ctx context.Context, s *SQLiteStore, record DiscordSmokeEvidenceRecord) DiscordSmokeEvidenceRecord {
	now := time.Now().UTC()
	record.TenantID = coalesceString(record.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)))
	record.Status = coalesceString(record.Status, "skipped")
	record.CredentialMode = coalesceString(record.CredentialMode, "unavailable")
	record.Owner = coalesceString(record.Owner, "operator")
	record.Reason = coalesceString(record.Reason, "safe_credentials_unavailable")
	record.RedactionStatus = coalesceString(record.RedactionStatus, "redacted")
	if record.SmokeEvidenceID == "" {
		record.SmokeEvidenceID = newStoreID("discord_smoke")
	}
	if record.ValidatedAt.IsZero() {
		record.ValidatedAt = now
	}
	if record.RetentionExpiresAt.IsZero() {
		record.RetentionExpiresAt = record.ValidatedAt.Add(90 * 24 * time.Hour)
	}
	return record
}

func nullableTimeValue(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: value.UTC().Format(time.RFC3339Nano), Valid: true}
}
