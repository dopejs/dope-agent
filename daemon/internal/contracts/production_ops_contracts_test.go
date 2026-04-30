package contracts_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionOpsPlanningContractsRemainComplete(t *testing.T) {
	root := contractRepoRoot(t)
	required := map[string][]string{
		"specs/024-production-ops-soak/contracts/backup-restore-evidence.md": {
			"raw secret values", "at least three tenants", "restore from verified backup",
		},
		"specs/024-production-ops-soak/contracts/soak-harness-report.md": {
			"at least 24 hours", "Minimum restarts: 3", "queue backlog persists for more than 30 minutes",
		},
		"specs/024-production-ops-soak/contracts/release-readiness-gate.md": {
			"Roadmaps 40/41 rerun requirement", "ship_with_recorded_skips", "Missing safe credentials do not block",
		},
	}
	for rel, needles := range required {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		body := string(data)
		for _, needle := range needles {
			if !strings.Contains(body, needle) {
				t.Fatalf("%s missing %q", rel, needle)
			}
		}
	}
}

func TestProductionSoakRunnerGeneratesReportArtifact(t *testing.T) {
	root := contractRepoRoot(t)
	dataDir := t.TempDir()
	dbBytes := []byte("sqlite-size-sample")
	logBytes := []byte("log-size-sample")
	if err := os.WriteFile(filepath.Join(dataDir, "daemon.sqlite"), dbBytes, 0o644); err != nil {
		t.Fatalf("write db size fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "daemon.log"), logBytes, 0o644); err != nil {
		t.Fatalf("write log size fixture: %v", err)
	}
	reportPath := filepath.Join(t.TempDir(), "soak-report.json")
	cmd := exec.Command("bash", filepath.Join(root, "scripts/production/run-soak.sh"))
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"DOPE_DATA_DIR="+dataDir,
		"DOPE_SOAK_DURATION=targeted-validation",
		"DOPE_SOAK_REPORT="+reportPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run soak script: %v\n%s", err, output)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read generated report: %v\n%s", err, output)
	}
	body := string(data)
	for _, needle := range []string{"\"reportId\"", "\"elapsedSeconds\"", "\"workloadCoverage\"", "\"faultDrills\"", "\"resourceObservations\"", "\"followUpFullRerun\": true", "\"finalResult\""} {
		if !strings.Contains(body, needle) {
			t.Fatalf("generated soak report missing %s: %s", needle, body)
		}
	}
	for _, needle := range []string{fmt.Sprintf("\"maxLogBytes\": %d", len(logBytes)), fmt.Sprintf("\"maxStoredDataBytes\": %d", len(dbBytes))} {
		if !strings.Contains(body, needle) {
			t.Fatalf("generated soak report missing resource sample %s: %s", needle, body)
		}
	}
}

func contractRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repo root not found from %s", dir)
		}
		dir = parent
	}
}
