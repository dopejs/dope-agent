package migrationfixture

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// TestBuildPreTenantV21Fixture proves the seed helper builds a v21
// database with at least one row in every in-scope tenant_owned table.
// Phase 4 batch 6 (T078) extends this with the full migration assertion.
func TestBuildPreTenantV21Fixture(t *testing.T) {
	s, err := BuildPreTenantV21Fixture(t.TempDir())
	if err != nil {
		t.Fatalf("BuildPreTenantV21Fixture: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	counts, err := CountSeededRows(context.Background(), s)
	if err != nil {
		t.Fatalf("CountSeededRows: %v", err)
	}
	for table, n := range counts {
		if n == 0 {
			t.Errorf("table %s has zero seeded rows", table)
		}
	}
}

// TestApplyHeadMigrations_FromV21Fixture proves the v21 fixture can
// be migrated up to head + has every migration_progress step
// registered. The actual backfill driver invocation lives in the app
// package; this test only covers the schema migration path.
func TestApplyHeadMigrations_FromV21Fixture(t *testing.T) {
	s, err := BuildPreTenantV21Fixture(t.TempDir())
	if err != nil {
		t.Fatalf("BuildPreTenantV21Fixture: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if err := s.ApplyHeadMigrations(context.Background()); err != nil {
		t.Fatalf("ApplyHeadMigrations: %v", err)
	}
	steps, err := s.LoadMigrationSteps(context.Background())
	if err != nil {
		t.Fatalf("LoadMigrationSteps: %v", err)
	}
	if len(steps) < 30 {
		t.Fatalf("expected ~38 registered steps, got %d", len(steps))
	}
}

func TestSeedR37LocalCredentialFiles(t *testing.T) {
	dataDir := t.TempDir()
	fixture, err := SeedR37LocalCredentialFiles(dataDir)
	if err != nil {
		t.Fatalf("SeedR37LocalCredentialFiles: %v", err)
	}
	if fixture.ConflictRef == "" || len(fixture.MCPSecretRefs) == 0 || len(fixture.SkillSecretRefs) == 0 {
		t.Fatalf("unexpected fixture metadata: %+v", fixture)
	}
	for _, name := range []string{"mcp-secrets.json", "skill-secrets.json"} {
		payload, err := os.ReadFile(filepath.Join(dataDir, name))
		if err != nil {
			t.Fatalf("ReadFile %s: %v", name, err)
		}
		if len(payload) == 0 {
			t.Fatalf("%s is empty", name)
		}
	}
}

func TestSeedR37LocalCredentialState(t *testing.T) {
	dataDir := t.TempDir()
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	fixture, err := SeedR37LocalCredentialState(context.Background(), sqliteStore, dataDir)
	if err != nil {
		t.Fatalf("SeedR37LocalCredentialState: %v", err)
	}
	if fixture.ProviderID == "" || fixture.IntegrationID == "" || fixture.ConnectorID == "" || fixture.MCPServerID == "" || fixture.MCPToolName == "" {
		t.Fatalf("fixture did not expose all legacy resource ids: %+v", fixture)
	}

	assertRowCount := func(table string) {
		t.Helper()
		var count int
		if err := sqliteStore.DB().QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count == 0 {
			t.Fatalf("expected seeded rows in %s", table)
		}
	}
	for _, table := range []string{"provider_auth_states", "integrations", "connectors", "mcp_servers", "mcp_server_states", "mcp_tools", "mcp_tool_exposure_rules"} {
		assertRowCount(table)
	}
}
