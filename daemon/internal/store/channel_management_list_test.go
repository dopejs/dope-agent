package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func TestChannelManagementProjectionOrderingIsDeterministic(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	page := connectors.BuildConnectorPage(connectors.ProjectionInput{
		TenantID: "ten_channels",
		Connectors: []connectors.Connector{
			{TenantID: "ten_channels", ConnectorID: "ready", Kind: "discord", DisplayName: "Ready", Status: connectors.StatusHealthy, UpdatedAt: now},
			{TenantID: "ten_channels", ConnectorID: "disabled", Kind: "telegram", DisplayName: "Disabled", Status: connectors.StatusDisabled, UpdatedAt: now},
			{TenantID: "ten_channels", ConnectorID: "broken", Kind: "slack", DisplayName: "Broken", Status: connectors.StatusRegistered, UpdatedAt: now},
			{TenantID: "ten_other", ConnectorID: "other", Kind: "matrix", DisplayName: "Other", Status: connectors.StatusFailed, UpdatedAt: now},
		},
		Diagnostics: map[string][]connectors.ConnectorDiagnosticState{
			"broken": {{
				DiagnosticStateID:  "diag_broken",
				ConnectorID:        "broken",
				Status:             connectors.LifecycleStatePermissionBlocked,
				ReasonCode:         connectors.DiagnosticPermissionMissing,
				EvidenceTimestamp:  now,
				RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
				RedactionStatus:    connectors.RedactionStatusRedacted,
			}},
		},
		Now:   now,
		Limit: 2,
	})

	if len(page.Items) != 2 || page.Items[0].ConnectorID != "broken" || page.Items[1].ConnectorID != "disabled" || page.Page.NextCursor == "" {
		t.Fatalf("unexpected deterministic projection page: %+v", page)
	}
}

func TestChannelManagementStoreProjectionPageIsTenantScopedAndPaginated(t *testing.T) {
	t.Parallel()

	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	for _, connector := range []connectors.Connector{
		{TenantID: "ten_channels", ConnectorID: "ready", Kind: "discord", DisplayName: "Ready", Status: connectors.StatusHealthy, CreatedAt: now.Add(-3 * time.Minute), UpdatedAt: now},
		{TenantID: "ten_channels", ConnectorID: "disabled", Kind: "telegram", DisplayName: "Disabled", Status: connectors.StatusDisabled, CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now},
		{TenantID: "ten_channels", ConnectorID: "broken", Kind: "slack", DisplayName: "Broken", Status: connectors.StatusRegistered, CreatedAt: now.Add(-1 * time.Minute), UpdatedAt: now},
		{TenantID: "ten_other", ConnectorID: "other", Kind: "matrix", DisplayName: "Other", Status: connectors.StatusFailed, CreatedAt: now, UpdatedAt: now},
	} {
		if err := sqliteStore.UpsertConnector(ctx, connector); err != nil {
			t.Fatalf("UpsertConnector(%s): %v", connector.ConnectorID, err)
		}
	}
	diagnostic, err := connectors.ClassifyDiagnostic(connectors.DiagnosticInput{
		DiagnosticStateID: "diag_broken",
		TenantID:          "ten_channels",
		ConnectorID:       "broken",
		ReasonCode:        connectors.DiagnosticPermissionMissing,
		EvidenceTimestamp: now,
		RedactionReliable: true,
	})
	if err != nil {
		t.Fatalf("ClassifyDiagnostic: %v", err)
	}
	if err := sqliteStore.SaveConnectorDiagnosticState(ctx, diagnostic); err != nil {
		t.Fatalf("SaveConnectorDiagnosticState: %v", err)
	}

	page, err := sqliteStore.ListChannelConnectorProjectionPage(ctx, ChannelConnectorProjectionQuery{
		TenantID: "ten_channels",
		Limit:    2,
		Now:      now,
	})
	if err != nil {
		t.Fatalf("ListChannelConnectorProjectionPage: %v", err)
	}
	if len(page.Items) != 2 || page.Items[0].ConnectorID != "broken" || page.Items[1].ConnectorID != "disabled" {
		t.Fatalf("unexpected first page: %+v", page)
	}
	if page.Page.NextCursor == "" {
		t.Fatalf("expected next cursor: %+v", page.Page)
	}

	next, err := sqliteStore.ListChannelConnectorProjectionPage(ctx, ChannelConnectorProjectionQuery{
		TenantID: "ten_channels",
		Limit:    2,
		Cursor:   page.Page.NextCursor,
		Now:      now,
	})
	if err != nil {
		t.Fatalf("ListChannelConnectorProjectionPage next: %v", err)
	}
	if len(next.Items) != 1 || next.Items[0].ConnectorID != "ready" {
		t.Fatalf("unexpected second page: %+v", next)
	}
}
