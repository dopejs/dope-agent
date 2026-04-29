package contracts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionInstallUpgradeRunbooksNameElapsedEvidenceAndScope(t *testing.T) {
	root := contractRepoRoot(t)
	required := map[string][]string{
		"docs/runtime/production-install.md": {
			"60 minutes or less", "tenant-scoped single-node", "~/.dope-test", "multi-node managed service rollout is out of scope",
		},
		"docs/runtime/production-upgrade.md": {
			"90 minutes or less", "preflight", "postflight", "restore from backup",
		},
	}
	for rel, needles := range required {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		body := normalizeContractWhitespace(strings.ToLower(string(data)))
		for _, needle := range needles {
			if !strings.Contains(body, normalizeContractWhitespace(strings.ToLower(needle))) {
				t.Fatalf("%s missing %q", rel, needle)
			}
		}
	}
}

func normalizeContractWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
