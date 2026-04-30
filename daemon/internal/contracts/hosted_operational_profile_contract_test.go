package contracts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostedOperationalProfilePlanningContractsMapToImplementationEvidence(t *testing.T) {
	root := contractRepoRoot(t)
	required := map[string][]string{
		"specs/028-hosted-operational-profile/contracts/hosted-profile-commands.md": {
			"provision", "evidence-index", "~/.dope-test", "repo-owned foreground supervisor",
		},
		"specs/028-hosted-operational-profile/contracts/deployment-supervisor-evidence.md": {
			"reboot_recovery", "5 minutes", "redaction", "repo_foreground",
		},
		"specs/028-hosted-operational-profile/contracts/recovery-evidence.md": {
			"at least three tenants", "alternate", "raw credential", "rollback",
		},
		"specs/028-hosted-operational-profile/contracts/observability-report.md": {
			"failure-owner", "unsupported marker", "queue or backlog", "monotonic resource growth",
		},
		"specs/028-hosted-operational-profile/contracts/release-evidence-index.md": {
			"30-minute", "run identity", "90-day", "no_ship",
		},
	}
	for rel, needles := range required {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		body := strings.ToLower(string(data))
		for _, needle := range needles {
			if !strings.Contains(body, strings.ToLower(needle)) {
				t.Fatalf("%s missing %q", rel, needle)
			}
		}
	}
}
