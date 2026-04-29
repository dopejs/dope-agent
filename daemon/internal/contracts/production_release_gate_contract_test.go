package contracts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionReleaseGateDocsRequireRoadmapRerun(t *testing.T) {
	root := contractRepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "docs/runtime/release-readiness.md"))
	if err != nil {
		t.Fatalf("read release readiness doc: %v", err)
	}
	body := strings.ToLower(string(data))
	for _, needle := range []string{"30 minutes or less", "roadmap 40", "roadmap 41", "24-hour", "no-ship"} {
		if !strings.Contains(body, strings.ToLower(needle)) {
			t.Fatalf("release readiness doc missing %q", needle)
		}
	}
}
