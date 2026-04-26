package app

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

// Roadmap 35 (US2 / T077) — events backfill driver.
//
// Wraps store.RunEventsBackfill, classifies orphan errors, and emits
// `daemon.migration.orphan_detected` consistent with the per-domain
// drivers. The events step MUST be the last backfill in the suite —
// earlier per-domain backfills must have populated parent.tenant_id
// for the priority cascade to resolve correctly.
func runEventsBackfill(ctx context.Context, sqliteStore *store.SQLiteStore, defaultTenantID string, bus *events.Bus, logger *telemetry.Logger) error {
	if sqliteStore == nil {
		return errors.New("runEventsBackfill: nil store")
	}
	if defaultTenantID == "" {
		return errors.New("runEventsBackfill: defaultTenantID required")
	}
	err := sqliteStore.RunEventsBackfill(ctx, store.EventsBackfillStepName, defaultTenantID)
	return finalizeStep(ctx, sqliteStore, store.EventsBackfillStepName, err, bus, logger)
}
