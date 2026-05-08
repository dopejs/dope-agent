package migrationfixture

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestSeedR50TelegramChannelConnectorRowsCoversTablesAndTenants(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	fixture, err := SeedR50TelegramChannelConnectorRows(ctx, sqliteStore)
	if err != nil {
		t.Fatalf("SeedR50TelegramChannelConnectorRows: %v", err)
	}
	if len(fixture.TenantIDs) != 2 {
		t.Fatalf("expected two tenants, got %v", fixture.TenantIDs)
	}
	counts, err := CountR50TelegramChannelConnectorRows(ctx, sqliteStore)
	if err != nil {
		t.Fatalf("CountR50TelegramChannelConnectorRows: %v", err)
	}
	for table, want := range fixture.ExpectedRowCount {
		if got := counts[table]; got != want {
			t.Fatalf("table %s row count=%d, want %d", table, got, want)
		}
	}
}
