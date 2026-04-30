package contracts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostedRestoreScriptRejectsMissingRepresentativeTenants(t *testing.T) {
	root := contractRepoRoot(t)
	workDir := t.TempDir()
	backupPath := filepath.Join(workDir, "daemon.sqlite.bak")
	makeDB := exec.Command("sqlite3", backupPath, "CREATE TABLE unrelated(id TEXT);")
	if output, err := makeDB.CombinedOutput(); err != nil {
		t.Fatalf("create sqlite backup fixture: %v\n%s", err, output)
	}

	restore := exec.Command("bash", filepath.Join(root, "scripts/production/restore-test-state.sh"), backupPath)
	restore.Dir = root
	restore.Env = append(os.Environ(),
		"DOPE_RESTORE_TARGET_DIR="+filepath.Join(workDir, "restore"),
		"DOPE_HOSTED_SOURCE_DATA_DIR="+filepath.Join(workDir, "source"),
		"DOPE_HOSTED_RUN_ID=hosted_restore_contract",
		"DOPE_HOSTED_ARTIFACT_DIR="+filepath.Join(workDir, "artifacts"),
	)
	output, err := restore.CombinedOutput()
	if err == nil {
		t.Fatalf("expected hosted restore to reject missing representative tenants:\n%s", output)
	}
	if !strings.Contains(string(output), "expected at least 3 tenants") {
		t.Fatalf("expected tenant coverage failure, got:\n%s", output)
	}
}

func TestHostedUpgradePreflightRecordsMissingBackupAsBlocking(t *testing.T) {
	root := contractRepoRoot(t)
	workDir := t.TempDir()
	preflight := exec.Command("bash", filepath.Join(root, "scripts/production/upgrade-preflight.sh"))
	preflight.Dir = root
	preflight.Env = append(os.Environ(),
		"DOPE_DATA_DIR="+filepath.Join(workDir, "data"),
		"DOPE_HOSTED_RUN_ID=hosted_preflight_contract",
		"DOPE_HOSTED_ARTIFACT_DIR="+filepath.Join(workDir, "artifacts"),
	)
	output, err := preflight.CombinedOutput()
	if err != nil {
		t.Fatalf("run hosted preflight: %v\n%s", err, output)
	}
	evidencePath := filepath.Join(workDir, "artifacts", "hosted_preflight_contract", "upgrade-preflight.json")
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatalf("read hosted preflight evidence: %v\n%s", err, output)
	}
	body := string(data)
	if !strings.Contains(body, `"requiredBackupState": "fail"`) || !strings.Contains(body, "backup evidence missing") {
		t.Fatalf("expected missing backup to be blocking, got:\n%s", body)
	}
}

func TestHostedBackupScriptRejectsMissingRepresentativeTenants(t *testing.T) {
	root := contractRepoRoot(t)
	workDir := t.TempDir()
	dataDir := filepath.Join(workDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	dbPath := filepath.Join(dataDir, "daemon.sqlite")
	makeDB := exec.Command("sqlite3", dbPath, "CREATE TABLE unrelated(id TEXT);")
	if output, err := makeDB.CombinedOutput(); err != nil {
		t.Fatalf("create sqlite data fixture: %v\n%s", err, output)
	}

	backup := exec.Command("bash", filepath.Join(root, "scripts/production/backup-test-state.sh"))
	backup.Dir = root
	backup.Env = append(os.Environ(),
		"DOPE_DATA_DIR="+dataDir,
		"DOPE_HOSTED_RUN_ID=hosted_backup_contract",
		"DOPE_HOSTED_ARTIFACT_DIR="+filepath.Join(workDir, "artifacts"),
	)
	output, err := backup.CombinedOutput()
	if err == nil {
		t.Fatalf("expected hosted backup to reject missing representative tenants:\n%s", output)
	}
	if !strings.Contains(string(output), "expected at least 3 tenants") {
		t.Fatalf("expected tenant coverage failure, got:\n%s", output)
	}
}

func TestHostedUpgradePostflightRecordsMissingTenantsAsBlocking(t *testing.T) {
	root := contractRepoRoot(t)
	workDir := t.TempDir()
	dataDir := filepath.Join(workDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	dbPath := filepath.Join(dataDir, "daemon.sqlite")
	makeDB := exec.Command("sqlite3", dbPath, "CREATE TABLE unrelated(id TEXT);")
	if output, err := makeDB.CombinedOutput(); err != nil {
		t.Fatalf("create sqlite data fixture: %v\n%s", err, output)
	}

	postflight := exec.Command("bash", filepath.Join(root, "scripts/production/upgrade-postflight.sh"))
	postflight.Dir = root
	postflight.Env = append(os.Environ(),
		"DOPE_DATA_DIR="+dataDir,
		"DOPE_HOSTED_RUN_ID=hosted_postflight_contract",
		"DOPE_HOSTED_ARTIFACT_DIR="+filepath.Join(workDir, "artifacts"),
		"DOPE_HOSTED_HEALTH_COMMAND=true",
	)
	output, err := postflight.CombinedOutput()
	if err != nil {
		t.Fatalf("run hosted postflight: %v\n%s", err, output)
	}
	evidencePath := filepath.Join(workDir, "artifacts", "hosted_postflight_contract", "upgrade-postflight.json")
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatalf("read hosted postflight evidence: %v\n%s", err, output)
	}
	body := string(data)
	if !strings.Contains(body, `"tenantDataVerification": "fail"`) || !strings.Contains(body, "representative tenants missing") {
		t.Fatalf("expected missing tenants to be blocking, got:\n%s", body)
	}
}

func TestHostedSoakAttributesHealthFailureToDaemon(t *testing.T) {
	root := contractRepoRoot(t)
	workDir := t.TempDir()
	reportPath := filepath.Join(workDir, "soak.json")
	soak := exec.Command("bash", filepath.Join(root, "scripts/production/run-soak.sh"))
	soak.Dir = root
	soak.Env = append(os.Environ(),
		"DOPE_DATA_DIR="+filepath.Join(workDir, "data"),
		"DOPE_SOAK_DURATION=targeted-validation",
		"DOPE_SOAK_REPORT="+reportPath,
		"DOPE_DAEMON_HEALTH_URL=http://127.0.0.1:1/healthz",
		"DOPE_HOSTED_RUN_ID=hosted_soak_contract",
	)
	output, err := soak.CombinedOutput()
	if err != nil {
		t.Fatalf("run hosted soak: %v\n%s", err, output)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read hosted soak report: %v\n%s", err, output)
	}
	body := string(data)
	if !strings.Contains(body, `"finalResult": "fail"`) || !strings.Contains(body, `"failureOwner": "daemon"`) {
		t.Fatalf("expected daemon health failure attribution, got:\n%s", body)
	}
}

func TestHostedSoakMarksUnobservedConnectorMCPAndDiagnosticsUnsupported(t *testing.T) {
	root := contractRepoRoot(t)
	workDir := t.TempDir()
	reportPath := filepath.Join(workDir, "soak.json")
	soak := exec.Command("bash", filepath.Join(root, "scripts/production/run-soak.sh"))
	soak.Dir = root
	soak.Env = append(os.Environ(),
		"DOPE_DATA_DIR="+filepath.Join(workDir, "data"),
		"DOPE_SOAK_DURATION=targeted-validation",
		"DOPE_SOAK_REPORT="+reportPath,
		"DOPE_DAEMON_HEALTH_URL=http://127.0.0.1:1/healthz",
		"DOPE_HOSTED_RUN_ID=hosted_soak_unsupported_contract",
	)
	output, err := soak.CombinedOutput()
	if err != nil {
		t.Fatalf("run hosted soak: %v\n%s", err, output)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read hosted soak report: %v\n%s", err, output)
	}
	body := string(data)
	for _, needle := range []string{`"connectorHealth": "unsupported"`, `"mcpHealth": "unsupported"`, `"integrationDiagnosticState": "unsupported"`, `"connectorHealth"`, `"mcpHealth"`, `"integrationDiagnosticState"`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected hosted soak unsupported marker %s, got:\n%s", needle, body)
		}
	}
}
