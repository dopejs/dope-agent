package tenancy_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestR37HandoffTableCoversRequiredResources(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	path := filepath.Join(repoRoot, "specs", "022-hosted-secrets-isolation", "contracts", "r37-handoff-table.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read R37 handoff table: %v", err)
	}
	text := string(body)
	for _, required := range []string{
		"tenant_secrets",
		"tenant_secret_versions",
		"secret_scope_bindings",
		"integrations",
		"provider_auth_states",
		"connectors",
		"mcp_servers",
		"mcp_server_states",
		"mcp_tools",
		"mcp_tool_exposure_rules",
		"Sandbox policies/profiles with secrets",
		"Existing integration bindings",
		"Existing `mcp-secrets.json`",
		"Existing `skill-secrets.json`",
		"Existing provider local auth",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("R37 handoff table missing %q", required)
		}
	}
}
