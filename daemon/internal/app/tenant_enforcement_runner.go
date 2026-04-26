package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

// Roadmap 35 (US2 / T077a + T077b) — step (c) enforcement driver.
//
// Runs after every backfill (T067 → T077) completes. For each
// runtime-spine table the driver invokes EnforceTenantNotNull which
// rebuilds the table with `tenant_id NOT NULL CHECK (tenant_id GLOB
// 'ten_*')`. For events it invokes EnforceEventsPartialIndexes to
// add the inventory's selectivity indexes (idx_events_tenant_owned
// + idx_events_global) without changing column nullability.
//
// Each step gates on the matching backfill `completed`; if any
// backfill is still running or pending the enforcement step refuses
// (returns ErrEnforcementBackfillIncomplete), the daemon logs a
// clear operator error, and startup fails.
//
// SCOPE: this batch covers the runtime spine + events. The remaining
// ~30 tenant_owned tables (schedules, workflows, calendar/mail/
// reminders, computer-use, evaluation, harness, connector_messages)
// follow the same pattern; landing them is mechanical and can be
// added by appending EnforcementSpec rows in the store package.
func runEnforcementSteps(ctx context.Context, sqliteStore *store.SQLiteStore, bus *events.Bus, logger *telemetry.Logger) error {
	if sqliteStore == nil {
		return errors.New("runEnforcementSteps: nil store")
	}
	// Roadmap 35 Pass B: runtime-spine NOT NULL + CHECK enforcement.
	// All legacy Upsert* helpers now auto-bind tenant_id by delegating
	// to the tenant-aware safe variants once the personal tenant is
	// bootstrapped (see store/default_tenant.go), so it is safe to
	// recreate each runtime spine table with NOT NULL CHECK.
	for _, spec := range store.RuntimeEnforcementSpecs() {
		err := sqliteStore.EnforceTenantNotNullSpec(ctx, spec)
		if err := finalizeEnforcement(ctx, sqliteStore, spec.StepName, err, bus, logger); err != nil {
			return err
		}
	}
	// T019–T026 + T076b extended enforcement (schedules, workflows,
	// integrations + delivery, calendar, mail, reminders, computer-use,
	// evaluation, connector_messages, etc.). Activated after every legacy
	// Upsert helper for these tables was patched to auto-bind tenant_id
	// via ResolveDefaultTenantBinding (the COALESCE-on-conflict pattern
	// keeps existing tenant assignment intact). The shadow-table swap
	// here recreates each table with `tenant_id NOT NULL CHECK
	// (tenant_id GLOB 'ten_*')` and adds the per-tenant UNIQUE/index
	// commitments documented in `schema-inventory.md`.
	for _, spec := range store.ExtendedEnforcementSpecs() {
		err := sqliteStore.EnforceTenantNotNullSpec(ctx, spec)
		if err := finalizeEnforcement(ctx, sqliteStore, spec.StepName, err, bus, logger); err != nil {
			return err
		}
	}
	if err := finalizeEnforcement(ctx, sqliteStore, store.EventsEnforceCheckStepName,
		sqliteStore.EnforceEventsPartialIndexes(ctx, store.EventsEnforceCheckStepName), bus, logger); err != nil {
		return err
	}
	return nil
}

func finalizeEnforcement(ctx context.Context, sqliteStore *store.SQLiteStore, stepName string, err error, bus *events.Bus, logger *telemetry.Logger) error {
	if err == nil {
		if logger != nil {
			logger.Slog().Info("tenant migration enforcement completed", "step", stepName)
		}
		return nil
	}
	if errors.Is(err, store.ErrEnforcementBackfillIncomplete) {
		// Don't mark step `failed` — the gate is operator-actionable
		// and the backfill driver will resolve it on a future boot.
		// Surface the precondition error so daemon startup aborts.
		return fmt.Errorf("enforcement %s blocked: %w", stepName, err)
	}
	if failErr := sqliteStore.FailMigrationStep(ctx, stepName, err); failErr != nil {
		return fmt.Errorf("enforcement %s: %w (also failed to mark step failed: %v)", stepName, err, failErr)
	}
	return fmt.Errorf("enforcement %s: %w", stepName, err)
}
