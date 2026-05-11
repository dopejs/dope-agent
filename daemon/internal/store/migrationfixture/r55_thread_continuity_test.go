package migrationfixture

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestR55ThreadContinuityMigratesPreV51ThreadRows(t *testing.T) {
	ctx := context.Background()
	sqliteStore, err := store.NewSQLiteStoreAtVersion(t.TempDir(), 50)
	if err != nil {
		t.Fatalf("NewSQLiteStoreAtVersion: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	if err := sqliteStore.ApplyHeadMigrations(ctx); err != nil {
		t.Fatalf("ApplyHeadMigrations: %v", err)
	}
	for _, table := range []string{"thread_continuity_turns", "thread_continuity_previews", "thread_continuity_preview_items"} {
		var name string
		if err := sqliteStore.DB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
			t.Fatalf("expected continuity table %s: %v", table, err)
		}
	}
}
