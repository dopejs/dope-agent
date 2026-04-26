package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Roadmap 35 (US2 / T077a) — step (c) NOT NULL + CHECK enforcement
// regression tests.

func TestEnforceTenantNotNull_BackfillNotCompleted_Refuses(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	// Backfill step is `pending` (registered at store init but never
	// completed). Enforcement MUST refuse with a clear gate error
	// rather than try to swap the table half-bound.
	err := s.EnforceTenantNotNull(ctx, EnforceNotNullSessionsStepName, "sessions", RuntimeBackfillSessionsStepName)
	if !errors.Is(err, ErrEnforcementBackfillIncomplete) {
		t.Fatalf("expected ErrEnforcementBackfillIncomplete, got %v", err)
	}
}

func TestEnforceTenantNotNull_AfterBackfill_AddsConstraint(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	// Seed sessions with bound tenant_id (post-backfill state).
	seedNullTenantRow(t, s, "sessions", "session_id", "sess_a", map[string]any{
		"kind": "chat", "status": "active", "channel": "test",
		"peer_id": "peer", "routing_key": "rk_a", "generation": 1,
		"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
		"last_active_at": "2025-01-01T00:00:00Z",
	})
	if _, err := s.db.ExecContext(ctx, `UPDATE sessions SET tenant_id = 'ten_owner' WHERE session_id = 'sess_a'`); err != nil {
		t.Fatalf("bind: %v", err)
	}
	// Mark backfill completed.
	if err := s.RegisterMigrationStep(ctx, RuntimeBackfillSessionsStepName); err != nil {
		t.Fatalf("register backfill: %v", err)
	}
	if err := s.CompleteMigrationStep(ctx, RuntimeBackfillSessionsStepName); err != nil {
		t.Fatalf("complete backfill: %v", err)
	}
	if err := s.RegisterMigrationStep(ctx, EnforceNotNullSessionsStepName); err != nil {
		t.Fatalf("register enforcement: %v", err)
	}

	if err := s.EnforceTenantNotNull(ctx, EnforceNotNullSessionsStepName, "sessions", RuntimeBackfillSessionsStepName); err != nil {
		t.Fatalf("EnforceTenantNotNull: %v", err)
	}

	// Verify the existing row survived intact.
	var got string
	if err := s.db.QueryRowContext(ctx, `SELECT tenant_id FROM sessions WHERE session_id = 'sess_a'`).Scan(&got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != "ten_owner" {
		t.Fatalf("post-swap tenant=%q, want ten_owner", got)
	}

	// Verify the new constraint blocks NULL-tenant inserts.
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions (session_id, kind, status, channel, peer_id, routing_key, generation, created_at, updated_at, last_active_at)
		VALUES ('sess_b','chat','active','t','p','rk_b',1,'2025-01-01T00:00:00Z','2025-01-01T00:00:00Z','2025-01-01T00:00:00Z')`)
	if err == nil {
		t.Fatalf("INSERT with NULL tenant_id MUST fail post-enforcement")
	}
	if !strings.Contains(err.Error(), "NOT NULL") && !strings.Contains(err.Error(), "constraint") {
		t.Fatalf("unexpected error class: %v", err)
	}

	// Verify the CHECK constraint blocks invalid tenant_id format.
	_, err = s.db.ExecContext(ctx, `INSERT INTO sessions (session_id, kind, status, channel, peer_id, routing_key, generation, created_at, updated_at, last_active_at, tenant_id)
		VALUES ('sess_c','chat','active','t','p','rk_c',1,'2025-01-01T00:00:00Z','2025-01-01T00:00:00Z','2025-01-01T00:00:00Z','not_a_tenant_id')`)
	if err == nil {
		t.Fatalf("INSERT with malformed tenant_id MUST fail CHECK")
	}

	// Idempotence: running again is a no-op.
	if err := s.EnforceTenantNotNull(ctx, EnforceNotNullSessionsStepName, "sessions", RuntimeBackfillSessionsStepName); err != nil {
		t.Fatalf("idempotent re-run: %v", err)
	}
}

// T018 step (c) regression: post-enforcement, two sessions with the
// same routing_key MUST coexist when they belong to different tenants
// (per-tenant UNIQUE replaces the previous global UNIQUE).
func TestEnforceTenantNotNull_AppliesPerTenantUniqueIndex(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	if err := s.RegisterMigrationStep(ctx, RuntimeBackfillSessionsStepName); err != nil {
		t.Fatalf("register backfill: %v", err)
	}
	if err := s.CompleteMigrationStep(ctx, RuntimeBackfillSessionsStepName); err != nil {
		t.Fatalf("complete backfill: %v", err)
	}
	if err := s.RegisterMigrationStep(ctx, EnforceNotNullSessionsStepName); err != nil {
		t.Fatalf("register enforcement: %v", err)
	}

	specs := RuntimeEnforcementSpecs()
	var sessionsSpec EnforcementSpec
	for _, sp := range specs {
		if sp.Table == "sessions" {
			sessionsSpec = sp
			break
		}
	}
	if err := s.EnforceTenantNotNullSpec(ctx, sessionsSpec); err != nil {
		t.Fatalf("EnforceTenantNotNullSpec: %v", err)
	}

	// Two tenants writing the same routing_key MUST both succeed.
	for i, tenant := range []string{"ten_aaaa", "ten_bbbb"} {
		_, err := s.db.ExecContext(ctx, `INSERT INTO sessions (session_id, kind, status, channel, peer_id, routing_key, generation, created_at, updated_at, last_active_at, tenant_id)
			VALUES (?,'chat','active','t','p','SHARED_KEY',1,'2025-01-01T00:00:00Z','2025-01-01T00:00:00Z','2025-01-01T00:00:00Z',?)`,
			fmt.Sprintf("sess_%d", i), tenant)
		if err != nil {
			t.Fatalf("tenant %s session insert (per-tenant UNIQUE): %v", tenant, err)
		}
	}
	// Same tenant + same routing_key MUST collide.
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions (session_id, kind, status, channel, peer_id, routing_key, generation, created_at, updated_at, last_active_at, tenant_id)
		VALUES ('sess_dup','chat','active','t','p','SHARED_KEY',1,'2025-01-01T00:00:00Z','2025-01-01T00:00:00Z','2025-01-01T00:00:00Z','ten_aaaa')`)
	if err == nil {
		t.Fatalf("same-tenant routing_key collision MUST fail per-tenant UNIQUE")
	}
}

// T026 step (c) regression: mcp_tool_exposure_rules has a composite PK
// (server_id, tool_name, runtime_surface). After enforcement the PK
// MUST be extended to (tenant_id, server_id, tool_name, runtime_surface),
// allowing two tenants to define overlapping (server, tool, surface)
// rules without colliding on the global PK.
func TestEnforceTenantNotNull_ExtendsCompositePK_MCPToolExposureRules(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	// Seed the FK parent rows so subsequent inserts pass FK validation.
	if _, err := s.db.ExecContext(ctx, `INSERT INTO mcp_servers (server_id, enabled, updated_at, document_json)
		VALUES ('srv_a',1,'2025-01-01T00:00:00Z','{}')`); err != nil {
		t.Fatalf("seed mcp_servers: %v", err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO mcp_tools (server_id, tool_name, discovery_status, updated_at, document_json)
		VALUES ('srv_a','tool_x','ready','2025-01-01T00:00:00Z','{}')`); err != nil {
		t.Fatalf("seed mcp_tools: %v", err)
	}
	// Seed one already-bound exposure rule so post-swap it survives.
	if _, err := s.db.ExecContext(ctx, `INSERT INTO mcp_tool_exposure_rules (tenant_id, server_id, tool_name, runtime_surface, exposure_mode, active, document_json, updated_at)
		VALUES ('ten_aaaa','srv_a','tool_x','chat','allow',1,'{}','2025-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("seed exposure rule: %v", err)
	}

	// Backfill gate.
	if err := s.RegisterMigrationStep(ctx, MCPToolExposureRulesBackfillStepName); err != nil {
		t.Fatalf("register backfill: %v", err)
	}
	if err := s.CompleteMigrationStep(ctx, MCPToolExposureRulesBackfillStepName); err != nil {
		t.Fatalf("complete backfill: %v", err)
	}
	if err := s.RegisterMigrationStep(ctx, EnforceNotNullMCPToolExposureRulesStepName); err != nil {
		t.Fatalf("register enforcement: %v", err)
	}

	var spec EnforcementSpec
	for _, sp := range ExtendedEnforcementSpecs() {
		if sp.Table == "mcp_tool_exposure_rules" {
			spec = sp
			break
		}
	}
	if spec.Table == "" {
		t.Fatalf("mcp_tool_exposure_rules spec missing from ExtendedEnforcementSpecs()")
	}
	if err := s.EnforceTenantNotNullSpec(ctx, spec); err != nil {
		t.Fatalf("EnforceTenantNotNullSpec: %v", err)
	}

	// Verify the seeded row survived.
	var got string
	if err := s.db.QueryRowContext(ctx, `SELECT exposure_mode FROM mcp_tool_exposure_rules WHERE tenant_id='ten_aaaa' AND server_id='srv_a' AND tool_name='tool_x' AND runtime_surface='chat'`).Scan(&got); err != nil {
		t.Fatalf("read seeded row: %v", err)
	}
	if got != "allow" {
		t.Fatalf("post-swap exposure_mode=%q, want allow", got)
	}

	// PK extension assertion: a *different* tenant MUST be able to
	// define the same (server, tool, surface) tuple without colliding.
	if _, err := s.db.ExecContext(ctx, `INSERT INTO mcp_tool_exposure_rules (tenant_id, server_id, tool_name, runtime_surface, exposure_mode, active, document_json, updated_at)
		VALUES ('ten_bbbb','srv_a','tool_x','chat','deny',1,'{}','2025-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("second tenant insert MUST succeed under per-tenant PK: %v", err)
	}

	// Same tenant duplicate MUST still collide on the extended PK.
	_, err := s.db.ExecContext(ctx, `INSERT INTO mcp_tool_exposure_rules (tenant_id, server_id, tool_name, runtime_surface, exposure_mode, active, document_json, updated_at)
		VALUES ('ten_aaaa','srv_a','tool_x','chat','deny',1,'{}','2025-01-01T00:00:00Z')`)
	if err == nil {
		t.Fatalf("same-tenant PK duplicate MUST collide on extended PK")
	}

	// PRAGMA assertion: the live PK is now 4-wide.
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info("mcp_tool_exposure_rules")`)
	if err != nil {
		t.Fatalf("table_info: %v", err)
	}
	defer rows.Close()
	pkCols := map[string]int{}
	for rows.Next() {
		var (
			cid       int
			name, typ string
			notnull   int
			dflt      sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if pk > 0 {
			pkCols[name] = pk
		}
	}
	want := []string{"tenant_id", "server_id", "tool_name", "runtime_surface"}
	for _, c := range want {
		if _, ok := pkCols[c]; !ok {
			t.Fatalf("PK missing column %s; got %v", c, pkCols)
		}
	}
	if len(pkCols) != len(want) {
		t.Fatalf("PK width = %d, want %d (%v)", len(pkCols), len(want), pkCols)
	}

	// Idempotence: re-running MUST NOT prepend tenant_id twice.
	if err := s.EnforceTenantNotNullSpec(ctx, spec); err != nil {
		t.Fatalf("idempotent re-run: %v", err)
	}
}

// T077b regression: after EnforceEventsPartialIndexes the events table
// MUST reject malformed tenant_id values and MUST reject NULL tenant_id
// rows whose category is not in the global allow-list. Global-category
// rows (mcp/provider/system/...) MUST still be accepted with NULL.
func TestEnforceEventsPartialIndexes_AddsCheckConstraint(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	if err := s.RegisterMigrationStep(ctx, EventsBackfillStepName); err != nil {
		t.Fatalf("register backfill: %v", err)
	}
	if err := s.CompleteMigrationStep(ctx, EventsBackfillStepName); err != nil {
		t.Fatalf("complete backfill: %v", err)
	}
	if err := s.RegisterMigrationStep(ctx, EventsEnforceCheckStepName); err != nil {
		t.Fatalf("register enforcement: %v", err)
	}

	// Pre-existing rows: one tenant-bound, one global-category NULL row.
	if _, err := s.db.ExecContext(ctx, `INSERT INTO events (event_id, category, name, occurred_at, resource_kind, resource_id, payload_json, tenant_id)
		VALUES ('evt_a','runtime','run.started','2025-01-01T00:00:00Z','run','r1','{}','ten_aaaa')`); err != nil {
		t.Fatalf("seed tenant event: %v", err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO events (event_id, category, name, occurred_at, resource_kind, resource_id, payload_json, tenant_id)
		VALUES ('evt_b','mcp','tool.update','2025-01-01T00:00:00Z','tool','t1','{}',NULL)`); err != nil {
		t.Fatalf("seed global event: %v", err)
	}

	if err := s.EnforceEventsPartialIndexes(ctx, EventsEnforceCheckStepName); err != nil {
		t.Fatalf("EnforceEventsPartialIndexes: %v", err)
	}

	// Pre-existing rows survive intact.
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE event_id IN ('evt_a','evt_b')`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Fatalf("post-swap rows = %d, want 2", n)
	}

	// CHECK rejects malformed tenant_id format.
	_, err := s.db.ExecContext(ctx, `INSERT INTO events (event_id, category, name, occurred_at, resource_kind, resource_id, payload_json, tenant_id)
		VALUES ('evt_bad','runtime','run.started','2025-01-01T00:00:00Z','run','r1','{}','not_a_tenant')`)
	if err == nil {
		t.Fatalf("CHECK MUST reject malformed tenant_id")
	}

	// CHECK rejects NULL tenant_id on a tenant-owned category.
	_, err = s.db.ExecContext(ctx, `INSERT INTO events (event_id, category, name, occurred_at, resource_kind, resource_id, payload_json, tenant_id)
		VALUES ('evt_orphan','runtime','run.started','2025-01-01T00:00:00Z','run','r1','{}',NULL)`)
	if err == nil {
		t.Fatalf("CHECK MUST reject NULL tenant_id on non-global category")
	}

	// CHECK accepts NULL tenant_id when category is in the global set.
	for _, cat := range []string{"mcp", "provider", "system", "daemon.migration", "connector_global", "capability_global"} {
		id := "evt_g_" + cat
		if _, err := s.db.ExecContext(ctx, `INSERT INTO events (event_id, category, name, occurred_at, resource_kind, resource_id, payload_json, tenant_id)
			VALUES (?, ?, 'x', '2025-01-01T00:00:00Z','x','x','{}',NULL)`, id, cat); err != nil {
			t.Fatalf("CHECK MUST accept NULL tenant_id for global category %q: %v", cat, err)
		}
	}

	// CHECK accepts a properly-formatted tenant_id on any category.
	if _, err := s.db.ExecContext(ctx, `INSERT INTO events (event_id, category, name, occurred_at, resource_kind, resource_id, payload_json, tenant_id)
		VALUES ('evt_good','runtime','run.started','2025-01-01T00:00:00Z','run','r1','{}','ten_bbbb')`); err != nil {
		t.Fatalf("CHECK MUST accept valid tenant_id: %v", err)
	}

	// Idempotence: re-running MUST be a no-op (no error, no double swap).
	if err := s.EnforceEventsPartialIndexes(ctx, EventsEnforceCheckStepName); err != nil {
		t.Fatalf("idempotent re-run: %v", err)
	}
}

func TestBuildShadowDDL_RewritesTenantIDColumn(t *testing.T) {
	cols := []columnInfo{
		{Name: "id", Type: "TEXT", PK: 1},
		{Name: "tenant_id", Type: "TEXT"},
		{Name: "body", Type: "TEXT", NotNull: true},
	}
	got := buildShadowDDL("__new_foo", cols, nil, []string{"id"})
	if !strings.Contains(got, "tenant_id TEXT NOT NULL CHECK (tenant_id GLOB 'ten_*')") {
		t.Fatalf("missing tenant_id constraint: %q", got)
	}
	if !strings.Contains(got, "id TEXT PRIMARY KEY") {
		t.Fatalf("missing inline PK: %q", got)
	}
}

func TestBuildShadowDDL_CompositePKAndFK(t *testing.T) {
	cols := []columnInfo{
		{Name: "server_id", Type: "TEXT", NotNull: true, PK: 1},
		{Name: "tool_name", Type: "TEXT", NotNull: true, PK: 2},
		{Name: "tenant_id", Type: "TEXT"},
	}
	fks := []fkInfo{
		{ID: 0, Seq: 0, Table: "mcp_servers", From: "server_id", To: "server_id", OnDelete: "CASCADE"},
	}
	got := buildShadowDDL("__new_x", cols, fks, []string{"server_id", "tool_name"})
	if !strings.Contains(got, "PRIMARY KEY (server_id, tool_name)") {
		t.Fatalf("missing composite PK: %q", got)
	}
	if !strings.Contains(got, "FOREIGN KEY (server_id) REFERENCES mcp_servers (server_id) ON DELETE CASCADE") {
		t.Fatalf("missing FK: %q", got)
	}
}
