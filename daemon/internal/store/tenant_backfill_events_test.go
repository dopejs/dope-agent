package store

import (
	"context"
	"errors"
	"testing"
)

// Roadmap 35 (T077) — events backfill regression tests.

func TestRunEventsBackfill_PriorityCascade_ReclassificationAndOrphan(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	// Seed a parent run with tenant_id bound (pretend its backfill ran first).
	seedNullTenantRow(t, s, "runs", "run_id", "run_a", map[string]any{
		"entrypoint": "t", "status": "queued", "goal": "g",
		"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
	})
	if _, err := s.db.ExecContext(ctx, `UPDATE runs SET tenant_id = ? WHERE run_id = ?`, "ten_run", "run_a"); err != nil {
		t.Fatalf("bind run: %v", err)
	}
	// Seed a connector_message bound for the proper-resource event below.
	if _, err := s.db.ExecContext(ctx, `INSERT INTO connector_messages (delivery_id, connector_id, direction, channel_id, content, status, created_at, updated_at, tenant_id)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		"cm_a", "discord_a", "out", "ch", "hi", "queued", "2025-01-01T00:00:00Z", "2025-01-01T00:00:00Z", "ten_cm"); err != nil {
		t.Fatalf("seed cm: %v", err)
	}

	// Tenant-owned event with run_id parent.
	if _, err := s.db.ExecContext(ctx, `INSERT INTO events (event_id, category, name, occurred_at, run_id, resource_kind, resource_id, payload_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		"evt_run", "run", "run.created", "2025-01-01T00:00:00Z", "run_a", "run", "run_a", "{}"); err != nil {
		t.Fatalf("seed evt_run: %v", err)
	}
	// Global system event — must stay NULL with category unchanged.
	if _, err := s.db.ExecContext(ctx, `INSERT INTO events (event_id, category, name, occurred_at, resource_kind, resource_id, payload_json)
		VALUES (?,?,?,?,?,?,?)`,
		"evt_sys", "system", "sys.heartbeat", "2025-01-01T00:00:00Z", "system", "hb", "{}"); err != nil {
		t.Fatalf("seed evt_sys: %v", err)
	}
	// Connector event with proper resource pointer — should bind from cm_a.
	if _, err := s.db.ExecContext(ctx, `INSERT INTO events (event_id, category, name, occurred_at, connector_id, resource_kind, resource_id, payload_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		"evt_cm_ok", "connector.message", "cm.delivered", "2025-01-01T00:00:00Z", "discord_a", "connector_message", "cm_a", "{}"); err != nil {
		t.Fatalf("seed evt_cm_ok: %v", err)
	}
	// Legacy connector event missing resource pointer — reclassify to connector_global.
	if _, err := s.db.ExecContext(ctx, `INSERT INTO events (event_id, category, name, occurred_at, connector_id, resource_kind, resource_id, payload_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		"evt_cm_legacy", "connector.legacy", "cm.legacy", "2025-01-01T00:00:00Z", "discord_a", "", "", "{}"); err != nil {
		t.Fatalf("seed evt_cm_legacy: %v", err)
	}
	// Capability-only event — reclassify to capability_global.
	if _, err := s.db.ExecContext(ctx, `INSERT INTO events (event_id, category, name, occurred_at, capability_id, resource_kind, resource_id, payload_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		"evt_cap", "capability.heartbeat", "cap.hb", "2025-01-01T00:00:00Z", "cap_a", "capability", "cap_a", "{}"); err != nil {
		t.Fatalf("seed evt_cap: %v", err)
	}

	if err := s.RegisterMigrationStep(ctx, EventsBackfillStepName); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := s.RunEventsBackfill(ctx, EventsBackfillStepName, "ten_default_fallback"); err != nil {
		t.Fatalf("RunEventsBackfill: %v", err)
	}

	cases := []struct{ id, wantCategory, wantTenant string }{
		{"evt_run", "run", "ten_run"},
		{"evt_sys", "system", ""},
		{"evt_cm_ok", "connector.message", "ten_cm"},
		{"evt_cm_legacy", "connector_global", ""},
		{"evt_cap", "capability_global", ""},
	}
	for _, c := range cases {
		var gotCategory, gotTenantBuf string
		row := s.db.QueryRowContext(ctx, `SELECT category, COALESCE(tenant_id, '') FROM events WHERE event_id = ?`, c.id)
		if err := row.Scan(&gotCategory, &gotTenantBuf); err != nil {
			t.Fatalf("read %s: %v", c.id, err)
		}
		if gotCategory != c.wantCategory {
			t.Errorf("%s category = %q, want %q", c.id, gotCategory, c.wantCategory)
		}
		if gotTenantBuf != c.wantTenant {
			t.Errorf("%s tenant_id = %q, want %q", c.id, gotTenantBuf, c.wantTenant)
		}
	}
}

// Tenant-owned event with an unresolvable parent FK falls back to the
// default personal tenant (lossless backfill, single-operator system).
// The strict orphan rule only fires for events with NO linkage at all
// (no typed FK, no resource pointer) — see TestRunEventsBackfill_OrphanWhenNoLinkage.
func TestRunEventsBackfill_FallbackWhenParentUnbound(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	seedNullTenantRow(t, s, "runs", "run_id", "run_unbound", map[string]any{
		"entrypoint": "t", "status": "queued", "goal": "g",
		"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
	})
	if _, err := s.db.ExecContext(ctx, `INSERT INTO events (event_id, category, name, occurred_at, run_id, resource_kind, resource_id, payload_json)
		VALUES (?,?,?,?,?,?,?,?)`,
		"evt_fallback", "run", "run.created", "2025-01-01T00:00:00Z", "run_unbound", "run", "run_unbound", "{}"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.RegisterMigrationStep(ctx, EventsBackfillStepName); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := s.RunEventsBackfill(ctx, EventsBackfillStepName, "ten_default_fallback"); err != nil {
		t.Fatalf("RunEventsBackfill: %v", err)
	}
	var got string
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(tenant_id,'') FROM events WHERE event_id = ?`, "evt_fallback").Scan(&got); err != nil {
		t.Fatalf("read tenant: %v", err)
	}
	if got != "ten_default_fallback" {
		t.Fatalf("unbound-parent event tenant=%q, want ten_default_fallback", got)
	}
}

// True-orphan case: tenant-owned category, no typed FK, no resource
// pointer. This is the only scenario where the migration fails out so
// the operator can investigate truly malformed data.
func TestRunEventsBackfill_OrphanWhenNoLinkage(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	if _, err := s.db.ExecContext(ctx, `INSERT INTO events (event_id, category, name, occurred_at, resource_kind, resource_id, payload_json)
		VALUES (?,?,?,?,?,?,?)`,
		"evt_no_linkage", "run", "run.created", "2025-01-01T00:00:00Z", "", "", "{}"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.RegisterMigrationStep(ctx, EventsBackfillStepName); err != nil {
		t.Fatalf("register: %v", err)
	}
	err := s.RunEventsBackfill(ctx, EventsBackfillStepName, "ten_default_fallback")
	var orphan *OrphanError
	if !errors.As(err, &orphan) {
		t.Fatalf("expected OrphanError, got %v", err)
	}
	if orphan.Table != "events" || orphan.ParentValue != "evt_no_linkage" {
		t.Fatalf("unexpected orphan: %+v", orphan)
	}
}
