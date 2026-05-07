package migrationfixture

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestSeedR48ConnectorConformanceRowsCoversTablesAndTenants(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	fixture, err := SeedR48ConnectorConformanceRows(ctx, sqliteStore)
	if err != nil {
		t.Fatalf("SeedR48ConnectorConformanceRows: %v", err)
	}
	if len(fixture.TenantIDs) < 2 {
		t.Fatalf("expected at least two tenants, got %v", fixture.TenantIDs)
	}
	counts, err := CountR48ConnectorConformanceRows(ctx, sqliteStore)
	if err != nil {
		t.Fatalf("CountR48ConnectorConformanceRows: %v", err)
	}
	for table, want := range fixture.ExpectedRowCount {
		if got := counts[table]; got != want {
			t.Fatalf("table %s row count=%d, want %d", table, got, want)
		}
	}
}
