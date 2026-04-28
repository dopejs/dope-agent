package contracts_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/secrets"
)

const (
	r37FakeSecretTenantA = "R37_FAKE_SECRET_TENANT_A_DO_NOT_LEAK"
	r37FakeSecretTenantB = "R37_FAKE_SECRET_TENANT_B_DO_NOT_LEAK"
	r37FakeTokenTenantA  = "R37_FAKE_TOKEN_TENANT_A_DO_NOT_LEAK"
	r37FakeTokenTenantB  = "R37_FAKE_TOKEN_TENANT_B_DO_NOT_LEAK"
)

func r37CredentialLeakSentinels() []string {
	return []string{
		r37FakeSecretTenantA,
		r37FakeSecretTenantB,
		r37FakeTokenTenantA,
		r37FakeTokenTenantB,
	}
}

func assertR37NoCredentialLeak(t *testing.T, label string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s for R37 leak scan: %v", label, err)
	}
	assertR37NoCredentialLeakString(t, label, string(data))
}

func assertR37NoCredentialLeakString(t *testing.T, label, value string) {
	t.Helper()
	for _, sentinel := range r37CredentialLeakSentinels() {
		if strings.Contains(value, sentinel) {
			t.Fatalf("%s leaked R37 credential sentinel %q", label, sentinel)
		}
	}
}

func TestR37RedactionContractRejectsSentinelsAcrossArtifacts(t *testing.T) {
	redactedRefs := secrets.RedactSecretRefs([]string{
		"mcp/filesystem/token",
		"provider/oauth/refresh",
	})
	payloads := map[string]any{
		"api response": map[string]any{
			"secretRefs": redactedRefs,
			"value":      secrets.RedactSecretValue(r37FakeTokenTenantA),
		},
		"event payload": map[string]any{
			"resolution": "denied",
			"secretRefs": redactedRefs,
		},
		"replay fixture": map[string]any{
			"credentialSummary": redactedRefs,
		},
		"evaluation artifact": map[string]any{
			"credentialSummary": redactedRefs,
		},
		"diagnostic": map[string]any{
			"status": "unavailable",
			"value":  secrets.RedactedValue,
		},
	}
	for label, payload := range payloads {
		assertR37NoCredentialLeak(t, label, payload)
	}
}
