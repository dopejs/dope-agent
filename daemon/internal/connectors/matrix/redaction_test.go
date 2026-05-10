package matrix

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRedactEvidenceSuppressesSecretsAndRawPayloads(t *testing.T) {
	t.Parallel()

	got := RedactEvidence(map[string]string{
		"accessToken":        "secret-token",
		"rawProviderPayload": "{\"body\":\"hello\"}",
		"homeserver":         "matrix.example.org",
		"room":               "!room:example.org",
	})
	if got.Status != "suppressed" {
		t.Fatalf("Status = %s, want suppressed", got.Status)
	}
	if _, exists := got.SafeEvidence["accessToken"]; exists {
		t.Fatalf("secret key leaked in safe evidence: %+v", got.SafeEvidence)
	}
	if got.SafeEvidence["homeserver"] == "" || got.SafeEvidence["room"] == "" {
		t.Fatalf("expected non-secret evidence to remain: %+v", got.SafeEvidence)
	}
}

func TestMatrixArtifactsDoNotLeakSensitiveEvidenceMarkers(t *testing.T) {
	t.Parallel()

	root := matrixRepoRoot(t)
	paths := []string{
		"schemas/api/matrix-hosted-setup-resource.schema.json",
		"schemas/api/matrix-route-policy-resource.schema.json",
		"schemas/api/matrix-smoke-evidence-resource.schema.json",
		"schemas/events/connector-matrix-setup-validated.event.schema.json",
		"daemon/internal/contracts/testdata/matrix-channel-connector/README.md",
		"docs/channels/matrix-channel-loop.md",
		"docs/runtime/connector-diagnostics.md",
	}
	markers := []string{
		"accessToken",
		"botAccessToken",
		"Authorization: Bearer",
		"rawProviderPayload",
		"raw_provider_payload",
		"eventBody",
		"roomContent",
		"matrix-secret",
		"mxc://secret",
	}
	for _, rel := range paths {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		text := string(body)
		for _, marker := range markers {
			if strings.Contains(text, marker) {
				t.Fatalf("%s leaked sensitive Matrix marker %q", rel, marker)
			}
		}
	}
}

func matrixRepoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller returned no file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
}
