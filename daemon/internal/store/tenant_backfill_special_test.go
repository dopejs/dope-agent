package store

import (
	"context"
	"errors"
	"testing"
)

// Roadmap 35 (US2 / T076a + T076b) — specialized backfill regression
// tests: composite-PK bulk, multi-parent priority.

func TestRunSimpleBulkBackfill_CompositePKTopLevel(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	// Need parent rows (mcp_servers, mcp_tools) for the FK to satisfy.
	if _, err := s.db.ExecContext(ctx, `INSERT INTO mcp_servers (server_id, enabled, updated_at, document_json) VALUES (?,?,?,?)`,
		"srv_a", 1, "2025-01-01T00:00:00Z", "{}"); err != nil {
		t.Fatalf("seed mcp_servers: %v", err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO mcp_tools (server_id, tool_name, discovery_status, updated_at, document_json) VALUES (?,?,?,?,?)`,
		"srv_a", "tool_a", "ready", "2025-01-01T00:00:00Z", "{}"); err != nil {
		t.Fatalf("seed mcp_tools: %v", err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO mcp_tool_exposure_rules (server_id, tool_name, runtime_surface, exposure_mode, active, document_json, updated_at) VALUES (?,?,?,?,?,?,?)`,
		"srv_a", "tool_a", "chat", "allow", 1, "{}", "2025-01-01T00:00:00Z"); err != nil {
		t.Fatalf("seed mcp_tool_exposure_rules: %v", err)
	}
	if err := s.RegisterMigrationStep(ctx, MCPToolExposureRulesBackfillStepName); err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := s.RunSimpleBulkBackfill(ctx, MCPToolExposureRulesBackfillStepName, "mcp_tool_exposure_rules", "ten_default"); err != nil {
		t.Fatalf("RunSimpleBulkBackfill: %v", err)
	}
	var got string
	if err := s.db.QueryRowContext(ctx, `SELECT tenant_id FROM mcp_tool_exposure_rules WHERE server_id=? AND tool_name=? AND runtime_surface=?`,
		"srv_a", "tool_a", "chat").Scan(&got); err != nil {
		t.Fatalf("read tenant_id: %v", err)
	}
	if got != "ten_default" {
		t.Fatalf("composite-PK row tenant_id=%q, want ten_default", got)
	}
}

func TestRunConnectorMessagesBackfill_PriorityResolution(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	// Three rows: (1) session_id present, (2) only run_id present, (3) neither.
	seedNullTenantRow(t, s, "sessions", "session_id", "sess_a", map[string]any{
		"kind": "chat", "status": "active", "channel": "test",
		"peer_id":        "peer",
		"routing_key":    "rk_sess_a",
		"generation":     1,
		"created_at":     "2025-01-01T00:00:00Z",
		"updated_at":     "2025-01-01T00:00:00Z",
		"last_active_at": "2025-01-01T00:00:00Z",
	})
	if _, err := s.db.ExecContext(ctx, `UPDATE sessions SET tenant_id = ? WHERE session_id = ?`, "ten_sess", "sess_a"); err != nil {
		t.Fatalf("bind sess: %v", err)
	}
	seedNullTenantRow(t, s, "runs", "run_id", "run_a", map[string]any{
		"entrypoint": "t", "status": "queued", "goal": "g",
		"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
	})
	if _, err := s.db.ExecContext(ctx, `UPDATE runs SET tenant_id = ? WHERE run_id = ?`, "ten_run", "run_a"); err != nil {
		t.Fatalf("bind run: %v", err)
	}

	// Three connector_messages rows.
	seedConnectorMessage(t, s, "del_session", "sess_a", "")
	seedConnectorMessage(t, s, "del_run", "", "run_a")
	seedConnectorMessage(t, s, "del_neither", "", "")

	if err := s.RegisterMigrationStep(ctx, ConnectorMessagesBackfillStepName); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := s.RunConnectorMessagesBackfill(ctx, ConnectorMessagesBackfillStepName, "ten_default"); err != nil {
		t.Fatalf("RunConnectorMessagesBackfill: %v", err)
	}
	for _, c := range []struct{ id, want string }{
		{"del_session", "ten_sess"},
		{"del_run", "ten_run"},
		{"del_neither", "ten_default"},
	} {
		if got := tenantOf(t, s, "connector_messages", "delivery_id", c.id); got != c.want {
			t.Fatalf("connector_message %s tenant_id=%q, want %q", c.id, got, c.want)
		}
	}
}

func TestRunConnectorMessagesBackfill_OrphanRunFK(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	// Parent run exists but its tenant_id is still NULL (parent
	// backfill not yet run). Per the orphan rule, the connector
	// backfill MUST refuse and surface OrphanError. (FK constraint
	// prevents a truly dangling run_id, but the "unbound parent" case
	// is the realistic in-flight scenario.)
	seedNullTenantRow(t, s, "runs", "run_id", "run_unbound", map[string]any{
		"entrypoint": "t", "status": "queued", "goal": "g",
		"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
	})
	seedConnectorMessage(t, s, "del_orphan", "", "run_unbound")
	if err := s.RegisterMigrationStep(ctx, ConnectorMessagesBackfillStepName); err != nil {
		t.Fatalf("register: %v", err)
	}
	err := s.RunConnectorMessagesBackfill(ctx, ConnectorMessagesBackfillStepName, "ten_default")
	var orphan *OrphanError
	if !errors.As(err, &orphan) {
		t.Fatalf("expected OrphanError, got %v", err)
	}
	if orphan.ParentTable != "runs" || orphan.ParentValue != "run_unbound" {
		t.Fatalf("unexpected orphan: %+v", orphan)
	}
}

func seedConnectorMessage(t *testing.T, s *SQLiteStore, deliveryID, sessionID, runID string) {
	t.Helper()
	args := []any{deliveryID, "discord_a", "out", "channel_a", "hello", "queued", "2025-01-01T00:00:00Z", "2025-01-01T00:00:00Z"}
	cols := "delivery_id, connector_id, direction, channel_id, content, status, created_at, updated_at"
	placeholders := "?,?,?,?,?,?,?,?"
	if sessionID != "" {
		cols += ", session_id"
		placeholders += ",?"
		args = append(args, sessionID)
	}
	if runID != "" {
		cols += ", run_id"
		placeholders += ",?"
		args = append(args, runID)
	}
	q := "INSERT INTO connector_messages (" + cols + ") VALUES (" + placeholders + ")"
	if _, err := s.db.ExecContext(context.Background(), q, args...); err != nil {
		t.Fatalf("seed connector_messages %s: %v", deliveryID, err)
	}
}
