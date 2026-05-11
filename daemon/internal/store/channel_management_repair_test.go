package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func TestChannelManagementRepairActionsListNewestFirst(t *testing.T) {
	t.Parallel()

	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	base := connectors.RepairAction{
		TenantID:        "ten_channels",
		ConnectorID:     "discord-main",
		ConnectorKind:   "discord",
		ActionKind:      connectors.ManagementActionRepair,
		Status:          connectors.ManagementTerminalActionRequired,
		RetrySafety:     connectors.RetrySafetyRetryable,
		AuditEventID:    "audit_repair",
		RedactionStatus: connectors.RedactionStatusRedacted,
	}
	older := base
	older.RepairActionID = "repair_older"
	older.StartedAt = time.Date(2026, 5, 10, 8, 0, 0, 0, time.UTC)
	newer := base
	newer.RepairActionID = "repair_newer"
	newer.StartedAt = older.StartedAt.Add(time.Minute)
	if _, err := sqliteStore.SaveChannelRepairAction(ctx, older); err != nil {
		t.Fatalf("SaveChannelRepairAction older: %v", err)
	}
	if _, err := sqliteStore.SaveChannelRepairAction(ctx, newer); err != nil {
		t.Fatalf("SaveChannelRepairAction newer: %v", err)
	}

	items, err := sqliteStore.ListChannelRepairActions(ctx, "ten_channels", "discord-main")
	if err != nil {
		t.Fatalf("ListChannelRepairActions: %v", err)
	}
	if len(items) != 2 || items[0].RepairActionID != "repair_newer" || items[1].RepairActionID != "repair_older" {
		t.Fatalf("unexpected repair order: %+v", items)
	}
}
