package migrationfixture

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type R39ProductionOpsFixture struct {
	Tenants              []R39TenantState
	RawCredentialValues  []string
	ExpectedRecordChecks int
}

type R39TenantState struct {
	TenantID             string
	CredentialRefs       []string
	QuotaState           string
	WorkState            string
	ReconnectRequired    bool
	OperatorActionNeeded bool
}

func BuildR39ProductionOpsFixture() R39ProductionOpsFixture {
	return R39ProductionOpsFixture{
		Tenants: []R39TenantState{
			{
				TenantID:       "ten_ops_alpha",
				CredentialRefs: []string{"secretref_calendar_alpha", "secretref_provider_alpha"},
				QuotaState:     "usage_10_of_100",
				WorkState:      "runtime_delivery_completed",
			},
			{
				TenantID:       "ten_ops_beta",
				CredentialRefs: []string{"secretref_mail_beta"},
				QuotaState:     "usage_40_of_100",
				WorkState:      "scheduled_work_pending",
			},
			{
				TenantID:             "ten_ops_gamma",
				CredentialRefs:       []string{"secretref_gamma_reconnect"},
				QuotaState:           "usage_95_of_100",
				WorkState:            "retry_exhausted_operator_action_needed",
				ReconnectRequired:    true,
				OperatorActionNeeded: true,
			},
		},
		RawCredentialValues:  nil,
		ExpectedRecordChecks: 12,
	}
}

func BuildR39ProductionOpsSQLiteFixture(dbPath string) (R39ProductionOpsFixture, error) {
	fixture := BuildR39ProductionOpsFixture()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fixture, fmt.Errorf("create fixture dir: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fixture, fmt.Errorf("open sqlite fixture: %w", err)
	}
	defer db.Close()
	statements := []string{
		`CREATE TABLE r39_tenants (tenant_id TEXT PRIMARY KEY, quota_state TEXT NOT NULL, work_state TEXT NOT NULL, reconnect_required INTEGER NOT NULL, operator_action_needed INTEGER NOT NULL)`,
		`CREATE TABLE r39_secret_refs (tenant_id TEXT NOT NULL, secret_ref TEXT NOT NULL, reconnect_required INTEGER NOT NULL, PRIMARY KEY (tenant_id, secret_ref))`,
		`CREATE TABLE r39_work_items (work_id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, state TEXT NOT NULL)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fixture, fmt.Errorf("create r39 fixture schema: %w", err)
		}
	}
	for _, tenant := range fixture.Tenants {
		if _, err := db.Exec(
			`INSERT INTO r39_tenants (tenant_id, quota_state, work_state, reconnect_required, operator_action_needed) VALUES (?, ?, ?, ?, ?)`,
			tenant.TenantID, tenant.QuotaState, tenant.WorkState, boolInt(tenant.ReconnectRequired), boolInt(tenant.OperatorActionNeeded),
		); err != nil {
			return fixture, fmt.Errorf("insert tenant %s: %w", tenant.TenantID, err)
		}
		for _, ref := range tenant.CredentialRefs {
			if containsR39RawCredential(ref) {
				return fixture, fmt.Errorf("credential ref for %s contains raw material", tenant.TenantID)
			}
			if _, err := db.Exec(
				`INSERT INTO r39_secret_refs (tenant_id, secret_ref, reconnect_required) VALUES (?, ?, ?)`,
				tenant.TenantID, ref, boolInt(tenant.ReconnectRequired),
			); err != nil {
				return fixture, fmt.Errorf("insert secret ref for %s: %w", tenant.TenantID, err)
			}
		}
		if _, err := db.Exec(
			`INSERT INTO r39_work_items (work_id, tenant_id, state) VALUES (?, ?, ?)`,
			"work_"+tenant.TenantID, tenant.TenantID, tenant.WorkState,
		); err != nil {
			return fixture, fmt.Errorf("insert work for %s: %w", tenant.TenantID, err)
		}
	}
	return fixture, nil
}

func CopyR39ProductionOpsSQLiteFixture(sourcePath, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("create restore dir: %w", err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source sqlite fixture: %w", err)
	}
	defer source.Close()
	dest, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create restored sqlite fixture: %w", err)
	}
	defer dest.Close()
	if _, err := io.Copy(dest, source); err != nil {
		return fmt.Errorf("copy sqlite fixture: %w", err)
	}
	return nil
}

func ValidateR39ProductionOpsSQLiteRestore(dbPath string, expected R39ProductionOpsFixture) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open restored sqlite fixture: %w", err)
	}
	defer db.Close()
	var integrity string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return fmt.Errorf("run sqlite integrity_check: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("sqlite integrity_check failed: %s", integrity)
	}
	var tenantCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM r39_tenants`).Scan(&tenantCount); err != nil {
		return fmt.Errorf("count tenants: %w", err)
	}
	if tenantCount != len(expected.Tenants) {
		return fmt.Errorf("tenant count mismatch: got %d want %d", tenantCount, len(expected.Tenants))
	}
	for _, tenant := range expected.Tenants {
		var quotaState, workState string
		var reconnectRequired int
		if err := db.QueryRow(
			`SELECT quota_state, work_state, reconnect_required FROM r39_tenants WHERE tenant_id = ?`,
			tenant.TenantID,
		).Scan(&quotaState, &workState, &reconnectRequired); err != nil {
			return fmt.Errorf("read tenant %s: %w", tenant.TenantID, err)
		}
		if quotaState != tenant.QuotaState || workState != tenant.WorkState {
			return fmt.Errorf("tenant %s state mismatch after restore", tenant.TenantID)
		}
		if boolInt(tenant.ReconnectRequired) != reconnectRequired {
			return fmt.Errorf("tenant %s reconnect state mismatch after restore", tenant.TenantID)
		}
		for _, ref := range tenant.CredentialRefs {
			var exists int
			if err := db.QueryRow(
				`SELECT COUNT(*) FROM r39_secret_refs WHERE tenant_id = ? AND secret_ref = ?`,
				tenant.TenantID, ref,
			).Scan(&exists); err != nil {
				return fmt.Errorf("read secret ref for %s: %w", tenant.TenantID, err)
			}
			if exists != 1 {
				return fmt.Errorf("secret ref %s for tenant %s missing after restore", ref, tenant.TenantID)
			}
		}
	}
	return validateR39NoRawCredentialRows(db)
}

func validateR39NoRawCredentialRows(db *sql.DB) error {
	rows, err := db.Query(`SELECT secret_ref FROM r39_secret_refs`)
	if err != nil {
		return fmt.Errorf("scan secret refs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			return fmt.Errorf("scan secret ref: %w", err)
		}
		if containsR39RawCredential(ref) {
			return fmt.Errorf("restored secret ref contains raw credential material")
		}
	}
	return rows.Err()
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func containsR39RawCredential(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"raw_secret", "access_token", "refresh_token", "oauth_code", "provider_token", "do_not_leak"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
