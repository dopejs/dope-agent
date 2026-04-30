package contracts_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostedProfileCommandsProvisionAndGenerateEvidenceIndex(t *testing.T) {
	root := contractRepoRoot(t)
	dataDir := filepath.Join(t.TempDir(), ".dope-test")
	artifactRoot := filepath.Join(t.TempDir(), "artifacts")
	reportRoot := filepath.Join(t.TempDir(), "reports")
	env := append(os.Environ(),
		"DOPE_ENV=test",
		"DOPE_DATA_DIR="+dataDir,
		"DOPE_HOSTED_ARTIFACT_DIR="+artifactRoot,
		"DOPE_HOSTED_REPORT_DIR="+reportRoot,
		"DOPE_HOSTED_RUN_ID=hosted_contract_run",
		"DOPE_HOSTED_COMMIT=028-hosted-operational-profile",
		"DOPE_HOSTED_DRY_RUN=1",
	)

	provision := exec.Command("bash", filepath.Join(root, "scripts/production/hosted-profile.sh"), "provision")
	provision.Dir = root
	provision.Env = env
	output, err := provision.CombinedOutput()
	if err != nil {
		t.Fatalf("provision hosted profile: %v\n%s", err, output)
	}
	for _, dir := range []string{dataDir, filepath.Join(dataDir, "logs"), artifactRoot, reportRoot, filepath.Join(dataDir, "backups"), filepath.Join(dataDir, "tmp")} {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("expected provisioned directory %s: %v\n%s", dir, err, output)
		}
	}
	if body := string(output); !strings.Contains(body, "run_id=hosted_contract_run") || strings.Contains(strings.ToLower(body), "access_token") {
		t.Fatalf("unexpected provision output: %s", body)
	}

	index := exec.Command("bash", filepath.Join(root, "scripts/production/hosted-profile.sh"), "evidence-index")
	index.Dir = root
	index.Env = env
	output, err = index.CombinedOutput()
	if err != nil {
		t.Fatalf("generate hosted evidence index: %v\n%s", err, output)
	}
	indexPath := filepath.Join(reportRoot, "hosted_contract_run", "release-evidence-index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read generated release index: %v\n%s", err, output)
	}
	for _, needle := range []string{`"releaseIndexId"`, `"runId": "hosted_contract_run"`, `"decision"`, `"deployment_manifest"`, `"retention_metadata"`} {
		if !strings.Contains(string(data), needle) {
			t.Fatalf("generated release index missing %s: %s", needle, data)
		}
	}
}

func TestHostedProfileSupervisorControlsProcessAndStatus(t *testing.T) {
	root := contractRepoRoot(t)
	dataDir := filepath.Join(t.TempDir(), ".dope-test")
	artifactRoot := filepath.Join(t.TempDir(), "artifacts")
	reportRoot := filepath.Join(t.TempDir(), "reports")
	env := append(os.Environ(),
		"DOPE_ENV=test",
		"DOPE_DATA_DIR="+dataDir,
		"DOPE_HOSTED_ARTIFACT_DIR="+artifactRoot,
		"DOPE_HOSTED_REPORT_DIR="+reportRoot,
		"DOPE_HOSTED_RUN_ID=hosted_supervisor_contract",
		"DOPE_HOSTED_COMMIT=028-hosted-operational-profile",
		"DOPE_HOSTED_DAEMON_COMMAND=sh -c 'while :; do sleep 1; done'",
		"DOPE_HOSTED_HEALTH_COMMAND=true",
	)

	start := exec.Command("bash", filepath.Join(root, "scripts/production/hosted-profile.sh"), "start")
	start.Dir = root
	start.Env = env
	output, err := start.CombinedOutput()
	if err != nil {
		t.Fatalf("start hosted supervisor: %v\n%s", err, output)
	}
	t.Cleanup(func() {
		stop := exec.Command("bash", filepath.Join(root, "scripts/production/hosted-profile.sh"), "stop")
		stop.Dir = root
		stop.Env = env
		_ = stop.Run()
	})

	pidPath := filepath.Join(artifactRoot, "hosted_supervisor_contract", "supervisor.pid")
	pidBytes, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("expected supervisor pid file: %v\n%s", err, output)
	}
	pid := strings.TrimSpace(string(pidBytes))
	if pid == "" {
		t.Fatalf("expected non-empty supervisor pid")
	}

	status := exec.Command("bash", filepath.Join(root, "scripts/production/hosted-profile.sh"), "status")
	status.Dir = root
	status.Env = env
	output, err = status.CombinedOutput()
	if err != nil {
		t.Fatalf("status hosted supervisor: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "process_status=running") || !strings.Contains(string(output), "health=pass") {
		t.Fatalf("expected running healthy status, got:\n%s", output)
	}

	stop := exec.Command("bash", filepath.Join(root, "scripts/production/hosted-profile.sh"), "stop")
	stop.Dir = root
	stop.Env = env
	output, err = stop.CombinedOutput()
	if err != nil {
		t.Fatalf("stop hosted supervisor: %v\n%s", err, output)
	}
	check := exec.Command("kill", "-0", pid)
	if err := check.Run(); err == nil {
		t.Fatalf("expected stopped supervisor process %s", pid)
	}
}

func TestHostedEvidenceIndexMarksMissingArtifactsNoShip(t *testing.T) {
	root := contractRepoRoot(t)
	dataDir := filepath.Join(t.TempDir(), ".dope-test")
	artifactRoot := filepath.Join(t.TempDir(), "artifacts")
	reportRoot := filepath.Join(t.TempDir(), "reports")
	env := append(os.Environ(),
		"DOPE_ENV=test",
		"DOPE_DATA_DIR="+dataDir,
		"DOPE_HOSTED_ARTIFACT_DIR="+artifactRoot,
		"DOPE_HOSTED_REPORT_DIR="+reportRoot,
		"DOPE_HOSTED_RUN_ID=hosted_missing_evidence",
		"DOPE_HOSTED_COMMIT=028-hosted-operational-profile",
		"DOPE_HOSTED_HEALTH_COMMAND=false",
	)

	index := exec.Command("bash", filepath.Join(root, "scripts/production/hosted-profile.sh"), "evidence-index")
	index.Dir = root
	index.Env = env
	output, err := index.CombinedOutput()
	if err != nil {
		t.Fatalf("generate missing-evidence index: %v\n%s", err, output)
	}

	indexPath := filepath.Join(reportRoot, "hosted_missing_evidence", "release-evidence-index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read generated release index: %v\n%s", err, output)
	}
	var parsed struct {
		Decision      string `json:"decision"`
		EvidenceLinks []struct {
			EvidenceType     string   `json:"evidenceType"`
			Status           string   `json:"status"`
			BlockingFindings []string `json:"blockingFindings"`
		} `json:"evidenceLinks"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("decode release index: %v\n%s", err, data)
	}
	if parsed.Decision != "no_ship" {
		t.Fatalf("expected no_ship decision for missing evidence, got %q", parsed.Decision)
	}
	for _, link := range parsed.EvidenceLinks {
		if link.EvidenceType == "restore_evidence" && link.Status != "fail" {
			t.Fatalf("expected missing restore evidence to fail, got %+v", link)
		}
		if link.EvidenceType == "health_checks" && link.Status != "fail" {
			t.Fatalf("expected failed health check to fail, got %+v", link)
		}
	}
}

func TestHostedEvidenceIndexRunsGoValidatorForShipReadyArtifacts(t *testing.T) {
	root := contractRepoRoot(t)
	dataDir := filepath.Join(t.TempDir(), ".dope-test")
	artifactRoot := filepath.Join(t.TempDir(), "artifacts")
	reportRoot := filepath.Join(t.TempDir(), "reports")
	runID := "hosted_ship_ready_evidence"
	runArtifactDir := filepath.Join(artifactRoot, runID)
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("create log dir: %v", err)
	}
	if err := os.MkdirAll(runArtifactDir, 0o755); err != nil {
		t.Fatalf("create artifact dir: %v", err)
	}
	for _, name := range []string{
		"deployment-manifest.json",
		"soak-report.json",
		"backup-evidence.json",
		"restore-evidence.json",
		"upgrade-preflight.json",
		"upgrade-postflight.json",
		"rollback-decision.json",
		"integration-diagnostics.json",
		"observability-report.json",
	} {
		body := []byte(`{"result":"passed","finalResult":"passed","daemonHealth":"pass","redactionStatus":"passed","blockingFindings":[]}`)
		if err := os.WriteFile(filepath.Join(runArtifactDir, name), body, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	env := append(os.Environ(),
		"DOPE_ENV=test",
		"DOPE_DATA_DIR="+dataDir,
		"DOPE_HOSTED_ARTIFACT_DIR="+artifactRoot,
		"DOPE_HOSTED_REPORT_DIR="+reportRoot,
		"DOPE_HOSTED_RUN_ID="+runID,
		"DOPE_HOSTED_COMMIT=028-hosted-operational-profile",
		"DOPE_HOSTED_HEALTH_COMMAND=true",
	)
	index := exec.Command("bash", filepath.Join(root, "scripts/production/hosted-profile.sh"), "evidence-index")
	index.Dir = root
	index.Env = env
	output, err := index.CombinedOutput()
	if err != nil {
		t.Fatalf("generate ship-ready hosted evidence index: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "release_evidence_validation=") {
		t.Fatalf("expected hosted-profile evidence-index to run Go validator, got:\n%s", output)
	}

	indexPath := filepath.Join(reportRoot, runID, "release-evidence-index.json")
	validate := exec.Command("go", "run", "./cmd/hosted-evidence-validate", indexPath)
	validate.Dir = filepath.Join(root, "daemon")
	output, err = validate.CombinedOutput()
	if err != nil {
		t.Fatalf("validate ship-ready hosted evidence index: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "hosted_evidence_validation=pass") {
		t.Fatalf("expected validator pass output, got:\n%s", output)
	}
}
