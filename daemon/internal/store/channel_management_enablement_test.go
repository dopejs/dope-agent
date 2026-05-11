package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func TestChannelManagementEnablementPersistenceSurvivesRestart(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	sqliteStore, err := NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	ctx := context.Background()
	changedAt := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)
	if err := sqliteStore.SaveChannelConnectorEnablementState(ctx, connectors.EnablementState{
		TenantID:             "ten_channels",
		ConnectorID:          "discord-main",
		State:                "disabled",
		ReasonCode:           "maintenance",
		ChangedByPrincipalID: "prn_channels",
		ChangedAt:            changedAt,
		AuditEventID:         "audit_disable",
	}); err != nil {
		t.Fatalf("SaveChannelConnectorEnablementState: %v", err)
	}
	if err := sqliteStore.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	state, ok, err := reopened.GetChannelConnectorEnablementState(ctx, "ten_channels", "discord-main")
	if err != nil || !ok {
		t.Fatalf("GetChannelConnectorEnablementState ok=%v err=%v", ok, err)
	}
	if state.State != "disabled" || state.ReasonCode != "maintenance" || state.AuditEventID != "audit_disable" {
		t.Fatalf("unexpected state after restart: %+v", state)
	}
}
