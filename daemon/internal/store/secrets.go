package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/secrets"
)

func (s *SQLiteStore) CreateSecret(ctx context.Context, secret secrets.TenantSecret, version secrets.SecretVersion) error {
	if s == nil {
		return nil
	}
	documentJSON, err := marshalJSON(secret.Document)
	if err != nil {
		return fmt.Errorf("marshal tenant secret document: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create tenant secret transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := insertTenantSecret(ctx, tx, secret, documentJSON); err != nil {
		return err
	}
	if err := insertTenantSecretVersion(ctx, tx, version); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create tenant secret transaction: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateSecretMetadata(ctx context.Context, secret secrets.TenantSecret) error {
	if s == nil {
		return nil
	}
	documentJSON, err := marshalJSON(secret.Document)
	if err != nil {
		return fmt.Errorf("marshal tenant secret document: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE tenant_secrets
		SET display_name = ?, disabled_reason = ?, remediation_reason = ?, updated_at = ?, document_json = ?
		WHERE tenant_id = ? AND secret_ref = ?
	`, nullString(secret.DisplayName), nullString(secret.DisabledReason), nullString(secret.RemediationReason), secret.UpdatedAt.UTC().Format(time.RFC3339Nano), documentJSON, secret.TenantID, secret.SecretRef)
	if err != nil {
		return fmt.Errorf("update tenant secret metadata: %w", err)
	}
	return nil
}

func (s *SQLiteStore) RotateSecret(ctx context.Context, secret secrets.TenantSecret, previousVersionID string, version secrets.SecretVersion) error {
	if s == nil {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin rotate tenant secret transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var nextVersion int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_number), 0) + 1 FROM tenant_secret_versions WHERE tenant_id = ? AND secret_id = ?`, secret.TenantID, secret.SecretID).Scan(&nextVersion); err != nil {
		return fmt.Errorf("select next tenant secret version: %w", err)
	}
	version.VersionNumber = nextVersion
	now := secret.UpdatedAt
	if previousVersionID != "" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE tenant_secret_versions
			SET status = ?, superseded_at = ?
			WHERE tenant_id = ? AND secret_version_id = ?
		`, string(secrets.SecretVersionStatusSuperseded), now.UTC().Format(time.RFC3339Nano), secret.TenantID, previousVersionID); err != nil {
			return fmt.Errorf("supersede tenant secret version: %w", err)
		}
	}
	if err := insertTenantSecretVersion(ctx, tx, version); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE tenant_secrets
		SET active_version_id = ?, rotated_at = ?, updated_at = ?
		WHERE tenant_id = ? AND secret_ref = ?
	`, secret.ActiveVersionID, nullableTimeString(secret.RotatedAt), secret.UpdatedAt.UTC().Format(time.RFC3339Nano), secret.TenantID, secret.SecretRef); err != nil {
		return fmt.Errorf("update rotated tenant secret: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit rotate tenant secret transaction: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DisableSecret(ctx context.Context, secret secrets.TenantSecret) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE tenant_secrets
		SET status = ?, disabled_reason = ?, disabled_at = ?, updated_at = ?
		WHERE tenant_id = ? AND secret_ref = ?
	`, string(secret.Status), nullString(secret.DisabledReason), nullableTimeString(secret.DisabledAt), secret.UpdatedAt.UTC().Format(time.RFC3339Nano), secret.TenantID, secret.SecretRef)
	if err != nil {
		return fmt.Errorf("disable tenant secret: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetSecretByRef(ctx context.Context, tenantID, secretRef string) (secrets.TenantSecret, bool, error) {
	if s == nil {
		return secrets.TenantSecret{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT secret_id, tenant_id, secret_ref, display_name, status, active_version_id, disabled_reason, remediation_reason, created_at, updated_at, rotated_at, disabled_at, document_json
		FROM tenant_secrets
		WHERE tenant_id = ? AND secret_ref = ?
	`, tenantID, secretRef)
	item, err := scanTenantSecret(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return secrets.TenantSecret{}, false, nil
		}
		return secrets.TenantSecret{}, false, err
	}
	return item, true, nil
}

func (s *SQLiteStore) GetSecretVersion(ctx context.Context, tenantID, secretVersionID string) (secrets.SecretVersion, bool, error) {
	if s == nil {
		return secrets.SecretVersion{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT secret_version_id, secret_id, tenant_id, secret_ref, version_number, status, value_backend_ref, created_at, activated_at, superseded_at
		FROM tenant_secret_versions
		WHERE tenant_id = ? AND secret_version_id = ?
	`, tenantID, secretVersionID)
	item, err := scanTenantSecretVersion(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return secrets.SecretVersion{}, false, nil
		}
		return secrets.SecretVersion{}, false, err
	}
	return item, true, nil
}

func (s *SQLiteStore) ListSecrets(ctx context.Context, tenantID string) ([]secrets.TenantSecret, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT secret_id, tenant_id, secret_ref, display_name, status, active_version_id, disabled_reason, remediation_reason, created_at, updated_at, rotated_at, disabled_at, document_json
		FROM tenant_secrets
		WHERE tenant_id = ?
		ORDER BY updated_at DESC, secret_id DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list tenant secrets: %w", err)
	}
	defer rows.Close()
	items := make([]secrets.TenantSecret, 0)
	for rows.Next() {
		item, err := scanTenantSecret(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func insertTenantSecret(ctx context.Context, tx *sql.Tx, secret secrets.TenantSecret, documentJSON sql.NullString) error {
	if !documentJSON.Valid {
		documentJSON = sql.NullString{String: "{}", Valid: true}
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO tenant_secrets (
			secret_id, tenant_id, secret_ref, display_name, status, active_version_id,
			disabled_reason, remediation_reason, created_at, updated_at, rotated_at, disabled_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, secret.SecretID, secret.TenantID, secret.SecretRef, nullString(secret.DisplayName), string(secret.Status), nullString(secret.ActiveVersionID), nullString(secret.DisabledReason), nullString(secret.RemediationReason), secret.CreatedAt.UTC().Format(time.RFC3339Nano), secret.UpdatedAt.UTC().Format(time.RFC3339Nano), nullableTimeString(secret.RotatedAt), nullableTimeString(secret.DisabledAt), documentJSON)
	if err != nil {
		return fmt.Errorf("insert tenant secret: %w", err)
	}
	return nil
}

func insertTenantSecretVersion(ctx context.Context, tx *sql.Tx, version secrets.SecretVersion) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO tenant_secret_versions (
			secret_version_id, secret_id, tenant_id, secret_ref, version_number, status,
			value_backend_ref, created_at, activated_at, superseded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, version.SecretVersionID, version.SecretID, version.TenantID, version.SecretRef, version.VersionNumber, string(version.Status), version.ValueBackendRef, version.CreatedAt.UTC().Format(time.RFC3339Nano), nullableTimeString(version.ActivatedAt), nullableTimeString(version.SupersededAt))
	if err != nil {
		return fmt.Errorf("insert tenant secret version: %w", err)
	}
	return nil
}

type tenantSecretScanner interface {
	Scan(dest ...any) error
}

func scanTenantSecret(scanner tenantSecretScanner) (secrets.TenantSecret, error) {
	var item secrets.TenantSecret
	var displayName, activeVersionID, disabledReason, remediationReason, rotatedAt, disabledAt, documentJSON sql.NullString
	var status string
	var createdAt, updatedAt string
	if err := scanner.Scan(&item.SecretID, &item.TenantID, &item.SecretRef, &displayName, &status, &activeVersionID, &disabledReason, &remediationReason, &createdAt, &updatedAt, &rotatedAt, &disabledAt, &documentJSON); err != nil {
		return secrets.TenantSecret{}, err
	}
	item.DisplayName = displayName.String
	item.Status = secrets.SecretStatus(status)
	item.ActiveVersionID = activeVersionID.String
	item.DisabledReason = disabledReason.String
	item.RemediationReason = remediationReason.String
	var err error
	item.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return secrets.TenantSecret{}, fmt.Errorf("parse tenant secret created_at: %w", err)
	}
	item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return secrets.TenantSecret{}, fmt.Errorf("parse tenant secret updated_at: %w", err)
	}
	err = assignOptionalTime(&item.RotatedAt, rotatedAt)
	if err != nil {
		return secrets.TenantSecret{}, fmt.Errorf("parse tenant secret rotated_at: %w", err)
	}
	err = assignOptionalTime(&item.DisabledAt, disabledAt)
	if err != nil {
		return secrets.TenantSecret{}, fmt.Errorf("parse tenant secret disabled_at: %w", err)
	}
	if err := unmarshalNullableJSON(documentJSON, &item.Document); err != nil {
		return secrets.TenantSecret{}, fmt.Errorf("decode tenant secret document: %w", err)
	}
	return item, nil
}

func scanTenantSecretVersion(scanner tenantSecretScanner) (secrets.SecretVersion, error) {
	var item secrets.SecretVersion
	var status string
	var createdAt string
	var activatedAt, supersededAt sql.NullString
	if err := scanner.Scan(&item.SecretVersionID, &item.SecretID, &item.TenantID, &item.SecretRef, &item.VersionNumber, &status, &item.ValueBackendRef, &createdAt, &activatedAt, &supersededAt); err != nil {
		return secrets.SecretVersion{}, err
	}
	item.Status = secrets.SecretVersionStatus(status)
	var err error
	item.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return secrets.SecretVersion{}, fmt.Errorf("parse tenant secret version created_at: %w", err)
	}
	err = assignOptionalTime(&item.ActivatedAt, activatedAt)
	if err != nil {
		return secrets.SecretVersion{}, fmt.Errorf("parse tenant secret version activated_at: %w", err)
	}
	err = assignOptionalTime(&item.SupersededAt, supersededAt)
	if err != nil {
		return secrets.SecretVersion{}, fmt.Errorf("parse tenant secret version superseded_at: %w", err)
	}
	return item, nil
}
