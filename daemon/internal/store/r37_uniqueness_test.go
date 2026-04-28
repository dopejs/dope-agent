package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/secrets"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/store/tenancy"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestR37SecretRefUniquenessIsTenantScoped(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	backend, err := secrets.NewLocalBackend(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalBackend: %v", err)
	}
	manager := secrets.NewManager(sqliteStore, backend)
	ctx := context.Background()

	if _, err := manager.Create(ctx, secrets.CreateInput{TenantID: "ten_r37_a", SecretRef: "shared/api-key", Value: "tenant-a-value"}); err != nil {
		t.Fatalf("create tenant A secret: %v", err)
	}
	if _, err := manager.Create(ctx, secrets.CreateInput{TenantID: "ten_r37_b", SecretRef: "shared/api-key", Value: "tenant-b-value"}); err != nil {
		t.Fatalf("same secretRef must be allowed in tenant B: %v", err)
	}
	if _, err := manager.Create(ctx, secrets.CreateInput{TenantID: "ten_r37_a", SecretRef: "shared/api-key", Value: "duplicate"}); err == nil {
		t.Fatal("duplicate secretRef in the same tenant succeeded")
	}
}

func TestR37BoundaryResourceSameIDCrossTenantDoesNotMutateOwner(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	resources := tenancy.NewR37Resources(sqliteStore, nil)
	ctxA := r37StoreTenantContext("ten_r37_a")
	ctxB := r37StoreTenantContext("ten_r37_b")
	now := time.Now().UTC()

	if err := resources.UpsertConnectorForTenant(ctxA, connectors.Connector{
		ConnectorID: "shared-connector", Kind: "discord", DisplayName: "tenant-a",
		Status: connectors.StatusRegistered, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed connector A: %v", err)
	}
	err = resources.UpsertConnectorForTenant(ctxB, connectors.Connector{
		ConnectorID: "shared-connector", Kind: "discord", DisplayName: "tenant-b",
		Status: connectors.StatusRegistered, CreatedAt: now, UpdatedAt: now,
	})
	if !errors.Is(err, tenancy.ErrCrossTenantWrite) {
		t.Fatalf("cross-tenant connector same id err=%v, want ErrCrossTenantWrite", err)
	}
	connectorsList, err := sqliteStore.ListConnectors(context.Background())
	if err != nil {
		t.Fatalf("list connectors: %v", err)
	}
	if len(connectorsList) != 1 || connectorsList[0].DisplayName != "tenant-a" {
		t.Fatalf("cross-tenant connector collision mutated owner: %#v", connectorsList)
	}

	if err := resources.UpsertProviderAuthStateForTenant(ctxA, providers.AuthState{
		ProviderID: "shared-provider", Family: providers.FamilyOpenAICompatible,
		AuthMode: providers.AuthModeAPIKey, Status: providers.AuthStatusAuthenticated,
		CLIAvailable: true, LastCheckedAt: now,
	}); err != nil {
		t.Fatalf("seed provider auth A: %v", err)
	}
	err = resources.UpsertProviderAuthStateForTenant(ctxB, providers.AuthState{
		ProviderID: "shared-provider", Family: providers.FamilyOpenAICompatible,
		AuthMode: providers.AuthModeAPIKey, Status: providers.AuthStatusRevoked,
		CLIAvailable: true, LastCheckedAt: now,
	})
	if err != nil {
		t.Fatalf("same provider auth id must be allowed in tenant B: %v", err)
	}
	states, err := sqliteStore.ListProviderAuthStates(context.Background())
	if err != nil {
		t.Fatalf("list provider auth states: %v", err)
	}
	statusByTenant := map[string]providers.AuthStatus{}
	for _, state := range states {
		statusByTenant[state.TenantID] = state.Status
	}
	if len(states) != 2 ||
		statusByTenant["ten_r37_a"] != providers.AuthStatusAuthenticated ||
		statusByTenant["ten_r37_b"] != providers.AuthStatusRevoked {
		t.Fatalf("cross-tenant provider auth states were not isolated: %#v", states)
	}

	if err := resources.UpsertMCPServerForTenant(ctxA, store.MCPServerRecord{
		ServerID: "shared-mcp", Enabled: true, UpdatedAt: now, Document: []byte(`{"name":"tenant-a"}`),
	}); err != nil {
		t.Fatalf("seed mcp server A: %v", err)
	}
	err = resources.UpsertMCPServerForTenant(ctxB, store.MCPServerRecord{
		ServerID: "shared-mcp", Enabled: false, UpdatedAt: now, Document: []byte(`{"name":"tenant-b"}`),
	})
	if !errors.Is(err, tenancy.ErrCrossTenantWrite) {
		t.Fatalf("cross-tenant mcp server same id err=%v, want ErrCrossTenantWrite", err)
	}
	servers, err := sqliteStore.ListMCPServers(context.Background())
	if err != nil {
		t.Fatalf("list mcp servers: %v", err)
	}
	if len(servers) != 1 || !servers[0].Enabled {
		t.Fatalf("cross-tenant mcp server collision mutated owner: %#v", servers)
	}
}

func r37StoreTenantContext(tenantID string) context.Context {
	return tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:     tenantID,
		PrincipalID:  "prn_" + tenantID,
		TenantSource: "test",
		Role:         identity.RoleAdmin,
		Permissions:  identity.PermissionsForRole(identity.RoleAdmin, identity.StatusActive),
		ResolvedAt:   time.Now().UTC(),
	})
}
