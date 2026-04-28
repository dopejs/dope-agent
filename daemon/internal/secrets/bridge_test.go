package secrets_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/secrets"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestLocalCredentialBridgeCreatesTenantSecretsIdempotently(t *testing.T) {
	manager, sqliteStore, _ := r37Manager(t)
	dataDir := t.TempDir()
	writeJSONFile(t, filepath.Join(dataDir, "mcp-secrets.json"), map[string]string{
		"MCP_TOKEN":    "mcp-secret",
		"SHARED_TOKEN": "shared-secret",
	})
	writeJSONFile(t, filepath.Join(dataDir, "skill-secrets.json"), map[string]string{
		"SKILL_TOKEN":  "skill-secret",
		"SHARED_TOKEN": "shared-secret",
	})

	result, err := secrets.BridgeLocalCredentialFiles(context.Background(), secrets.LocalCredentialBridgeInput{
		DataDir:       dataDir,
		TenantID:      "ten_bridge",
		StepName:      store.HostedCredentialBridgeStepName,
		Manager:       manager,
		ProgressStore: sqliteStore,
	})
	if err != nil {
		t.Fatalf("BridgeLocalCredentialFiles returned error: %v", err)
	}
	if len(result.Created) != 3 || result.SecretRefCount != 3 {
		t.Fatalf("expected 3 bridged secrets, got result %+v", result)
	}
	for _, ref := range []string{"MCP_TOKEN", "SKILL_TOKEN", "SHARED_TOKEN"} {
		resolved, err := manager.Resolve(context.Background(), secrets.ResolveInput{TenantID: "ten_bridge", SecretRef: ref})
		if err != nil {
			t.Fatalf("resolve bridged secret %s: %v", ref, err)
		}
		if resolved.Value == "" || resolved.Value == secrets.RedactedValue {
			t.Fatalf("unexpected bridged value for %s", ref)
		}
	}

	second, err := secrets.BridgeLocalCredentialFiles(context.Background(), secrets.LocalCredentialBridgeInput{
		DataDir:       dataDir,
		TenantID:      "ten_bridge",
		StepName:      store.HostedCredentialBridgeStepName,
		Manager:       manager,
		ProgressStore: sqliteStore,
	})
	if err != nil {
		t.Fatalf("second BridgeLocalCredentialFiles returned error: %v", err)
	}
	if !second.AlreadyCompleted || len(second.Created) != 0 || len(second.SkippedExisting) != 0 {
		t.Fatalf("expected completed progress step to skip second bridge, got %+v", second)
	}
}

func TestLocalCredentialBridgeReportsConflictsWithoutCreatingAmbiguousSecret(t *testing.T) {
	manager, _, _ := r37Manager(t)
	dataDir := t.TempDir()
	writeJSONFile(t, filepath.Join(dataDir, "mcp-secrets.json"), map[string]string{"DUP_TOKEN": "one"})
	writeJSONFile(t, filepath.Join(dataDir, "skill-secrets.json"), map[string]string{"DUP_TOKEN": "two"})

	result, err := secrets.BridgeLocalCredentialFiles(context.Background(), secrets.LocalCredentialBridgeInput{
		DataDir:  dataDir,
		TenantID: "ten_bridge",
		Manager:  manager,
	})
	if err != nil {
		t.Fatalf("BridgeLocalCredentialFiles returned error: %v", err)
	}
	if len(result.Disabled) != 1 || result.Disabled[0].Status != secrets.SecretStatusPendingRemediation {
		t.Fatalf("expected disabled remediation resource, got %+v", result.Disabled)
	}
	secret, err := manager.Get(context.Background(), "ten_bridge", "DUP_TOKEN")
	if err != nil {
		t.Fatalf("get conflicting bridged metadata: %v", err)
	}
	if secret.Status != secrets.SecretStatusPendingRemediation || secret.DisabledReason == "" || secret.RemediationReason == "" {
		t.Fatalf("expected disabled remediation secret metadata, got %+v", secret)
	}
	if _, err := manager.Resolve(context.Background(), secrets.ResolveInput{TenantID: "ten_bridge", SecretRef: "DUP_TOKEN"}); err == nil {
		t.Fatal("conflicting legacy secret should not resolve")
	}
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}
