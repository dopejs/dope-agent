package migrationfixture

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

const R37FakeSecretTenantA = "R37_FAKE_SECRET_TENANT_A_DO_NOT_LEAK"

type R37CredentialFixture struct {
	MCPSecretRefs   []string
	SkillSecretRefs []string
	ConflictRef     string
	ProviderID      string
	IntegrationID   string
	ConnectorID     string
	MCPServerID     string
	MCPToolName     string
}

func SeedR37LocalCredentialFiles(dataDir string) (R37CredentialFixture, error) {
	fixture := R37CredentialFixture{
		MCPSecretRefs:   []string{"R37_MCP_TOKEN", "R37_SHARED_TOKEN", "R37_CONFLICT_TOKEN"},
		SkillSecretRefs: []string{"R37_SKILL_TOKEN", "R37_SHARED_TOKEN", "R37_CONFLICT_TOKEN"},
		ConflictRef:     "R37_CONFLICT_TOKEN",
		ProviderID:      "r37_legacy_provider",
		IntegrationID:   "r37_legacy_integration",
		ConnectorID:     "r37_legacy_connector",
		MCPServerID:     "r37_legacy_mcp",
		MCPToolName:     "lookup",
	}
	if err := writeR37CredentialJSON(filepath.Join(dataDir, "mcp-secrets.json"), map[string]string{
		"R37_MCP_TOKEN":      R37FakeSecretTenantA,
		"R37_SHARED_TOKEN":   "shared-r37-value",
		"R37_CONFLICT_TOKEN": "mcp-side",
	}); err != nil {
		return fixture, err
	}
	if err := writeR37CredentialJSON(filepath.Join(dataDir, "skill-secrets.json"), map[string]string{
		"R37_SKILL_TOKEN":    "skill-r37-value",
		"R37_SHARED_TOKEN":   "shared-r37-value",
		"R37_CONFLICT_TOKEN": "skill-side",
	}); err != nil {
		return fixture, err
	}
	return fixture, nil
}

func SeedR37LocalCredentialState(ctx context.Context, sqliteStore *store.SQLiteStore, dataDir string) (R37CredentialFixture, error) {
	fixture, err := SeedR37LocalCredentialFiles(dataDir)
	if err != nil {
		return fixture, err
	}
	now := time.Now().UTC()
	if err := sqliteStore.UpsertProviderAuthState(ctx, providers.AuthState{
		ProviderID:    fixture.ProviderID,
		Family:        providers.FamilyOpenAICompatible,
		AuthMode:      providers.AuthModeAPIKey,
		Status:        providers.AuthStatusAuthenticated,
		CLIAvailable:  true,
		AccountLabel:  "R37 legacy provider",
		LastCheckedAt: now,
		Metadata:      map[string]string{"source": "r37_migration_fixture"},
	}); err != nil {
		return fixture, fmt.Errorf("seed provider auth: %w", err)
	}
	if err := sqliteStore.UpsertIntegration(ctx, integrations.Resource{
		IntegrationID:    fixture.IntegrationID,
		DomainKind:       "calendar",
		DisplayName:      "R37 Legacy Integration",
		EnvironmentScope: "test",
		ReadinessStatus:  integrations.ReadinessStatusHealthy,
		AuthState:        integrations.AuthStateAuthorized,
		HealthState:      integrations.HealthStateHealthy,
		AccountBinding:   integrations.AccountBinding{AccountKey: "r37-legacy@example.com"},
		BackendBinding:   integrations.BackendBinding{BackendKind: integrations.BackendKindManagedProvider, BackendRefID: fixture.ProviderID},
		CreatedAt:        now,
		UpdatedAt:        now,
		LastTransitionAt: now,
	}); err != nil {
		return fixture, fmt.Errorf("seed integration: %w", err)
	}
	if err := sqliteStore.UpsertConnector(ctx, connectors.Connector{
		ConnectorID:    fixture.ConnectorID,
		Kind:           "discord",
		DisplayName:    "R37 Legacy Connector",
		Status:         connectors.StatusHealthy,
		SecretRefs:     []string{fixture.ConflictRef},
		BackoffSeconds: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		return fixture, fmt.Errorf("seed connector: %w", err)
	}
	if err := sqliteStore.UpsertMCPServer(ctx, store.MCPServerRecord{
		ServerID:  fixture.MCPServerID,
		Enabled:   true,
		UpdatedAt: now,
		Document: mustR37JSON(map[string]any{
			"serverId":      fixture.MCPServerID,
			"displayName":   "R37 Legacy MCP",
			"enabled":       true,
			"transportKind": "stdio",
			"command":       "fake",
			"secretRefs":    []string{fixture.ConflictRef},
		}),
	}); err != nil {
		return fixture, fmt.Errorf("seed mcp server: %w", err)
	}
	if err := sqliteStore.UpsertMCPServerState(ctx, store.MCPServerStateRecord{
		ServerID:  fixture.MCPServerID,
		Status:    "healthy",
		UpdatedAt: now,
		Document:  mustR37JSON(map[string]any{"serverId": fixture.MCPServerID, "status": "healthy"}),
	}); err != nil {
		return fixture, fmt.Errorf("seed mcp server state: %w", err)
	}
	if err := sqliteStore.UpsertMCPTool(ctx, store.MCPToolRecord{
		ServerID:        fixture.MCPServerID,
		ToolName:        fixture.MCPToolName,
		DiscoveryStatus: "discovered",
		UpdatedAt:       now,
		Document:        mustR37JSON(map[string]any{"name": fixture.MCPToolName}),
	}); err != nil {
		return fixture, fmt.Errorf("seed mcp tool: %w", err)
	}
	if err := sqliteStore.UpsertMCPToolExposureRule(ctx, store.MCPToolExposureRuleRecord{
		ServerID:       fixture.MCPServerID,
		ToolName:       fixture.MCPToolName,
		RuntimeSurface: "chat",
		ExposureMode:   "allow",
		Active:         true,
		UpdatedAt:      now,
		Document:       mustR37JSON(map[string]any{"toolName": fixture.MCPToolName, "runtimeSurface": "chat"}),
	}); err != nil {
		return fixture, fmt.Errorf("seed mcp exposure: %w", err)
	}
	return fixture, nil
}

func writeR37CredentialJSON(path string, value map[string]string) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

func mustR37JSON(value any) []byte {
	payload, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return payload
}
