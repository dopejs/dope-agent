package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

// ErrTenantMigrationFailed is returned when the daemon refuses to start
// because at least one tenant_migration_progress row carries the `failed`
// status. The operator runbook
// (docs/runtime/tenant-migration-rollback.md) covers recovery.
var ErrTenantMigrationFailed = errors.New("tenant migration is in a failed state; refusing to start")

// MigrationGate exposes the post-startup view of the tenant migration so
// HTTP handlers can refuse tenant-owned traffic with a stable error
// while the backfill is still in flight. The Phase-3 startup contract
// (per Roadmap 35) is:
//
//   - any `failed` row → daemon refuses to start (ErrTenantMigrationFailed);
//   - any `pending` or `running` tenant_owned step → daemon starts but
//     `InProgress()` returns true so route handlers can return HTTP
//     503 with code `tenant_migration_in_progress` on tenant-owned
//     paths;
//   - all rows `completed` → InProgress() returns false; daemon serves
//     tenant-owned traffic normally.
//
// The gate is a small typed value; callers consult it via
// `(*MigrationGate).InProgress()`. The audit emit + 503 behaviour at
// the route layer is wired in api/server.go's `protected()`
// middleware; this type is the single source of truth for the
// in-progress predicate.
type MigrationGate struct {
	store *store.SQLiteStore
}

// InProgress reports whether at least one tenant_owned migration step
// is still pending or running. Live-queries the store so the
// in-flight US2 backfill driver clears the gate as soon as the last
// step transitions to `completed` — without this, the gate captured at
// startup would stay positive even after a successful in-process
// backfill.
func (g *MigrationGate) InProgress() bool {
	if g == nil || g.store == nil {
		return false
	}
	steps, err := g.store.LoadMigrationSteps(context.Background())
	if err != nil {
		// Read failure is opaque here; fail-safe to "in progress" so the
		// route layer keeps tenant traffic on the 503 path until the
		// operator restarts and the daemon's startup gate surfaces the
		// underlying read error.
		return true
	}
	for _, step := range steps {
		switch step.Status {
		case store.MigrationStepPending, store.MigrationStepRunning:
			return true
		}
	}
	return false
}

// PendingSteps returns the list of step names that are still pending
// or running. Live-queried so the response reflects in-flight backfill
// progress accurately.
func (g *MigrationGate) PendingSteps() []string {
	if g == nil || g.store == nil {
		return nil
	}
	steps, err := g.store.LoadMigrationSteps(context.Background())
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(steps))
	for _, step := range steps {
		switch step.Status {
		case store.MigrationStepPending, store.MigrationStepRunning:
			out = append(out, step.Name)
		}
	}
	return out
}

// guardTenantMigrationStartup enforces Roadmap 35's resume-safety contract.
// On every daemon start, after schema migrations have applied, it scans
// `tenant_migration_progress` and:
//
//   - emits a `daemon.migration.started` event noting the registered steps;
//   - if any row is `failed`, emits `daemon.migration.failed` and returns
//     ErrTenantMigrationFailed (the daemon refuses to serve traffic);
//   - if any row is `running`, logs a resume notice — the matching backfill
//     driver will pick the work up from `last_processed_key` when invoked;
//   - if every row is `completed`, emits `daemon.migration.completed`.
//
// At Phase 2 only the two events-table progress rows are registered; the
// per-domain backfill rows land as US1/US2 schema migrations are added.
// The function therefore tolerates a small, growing set of step rows.
func guardTenantMigrationStartup(ctx context.Context, sqliteStore *store.SQLiteStore, logger *telemetry.Logger, bus *events.Bus) (*MigrationGate, error) {
	steps, err := sqliteStore.LoadMigrationSteps(ctx)
	if err != nil {
		return nil, fmt.Errorf("load tenant migration steps: %w", err)
	}

	bus.Publish(events.Event{
		Category: "daemon.migration",
		Name:     "daemon.migration.started",
		Resource: events.Resource{Kind: "tenant_migration", ID: "tenant_scope"},
		Payload: map[string]any{
			"step_count": len(steps),
		},
	})

	var (
		failed    []string
		running   []string
		pending   []string
		completed int
	)
	for _, step := range steps {
		switch step.Status {
		case store.MigrationStepFailed:
			failed = append(failed, fmt.Sprintf("%s: %s", step.Name, step.Error))
		case store.MigrationStepRunning:
			running = append(running, step.Name)
		case store.MigrationStepPending:
			pending = append(pending, step.Name)
		case store.MigrationStepCompleted:
			completed++
		}
	}

	if len(failed) > 0 {
		bus.Publish(events.Event{
			Category: "daemon.migration",
			Name:     "daemon.migration.failed",
			Resource: events.Resource{Kind: "tenant_migration", ID: "tenant_scope"},
			Payload: map[string]any{
				"failed_steps": failed,
			},
		})
		logger.Slog().Error("tenant migration failed; refusing to start",
			"failed_steps", strings.Join(failed, "; "),
		)
		return nil, fmt.Errorf("%w: %s", ErrTenantMigrationFailed, strings.Join(failed, "; "))
	}

	gate := &MigrationGate{store: sqliteStore}

	if len(running) > 0 || len(pending) > 0 {
		bus.Publish(events.Event{
			Category: "daemon.migration",
			Name:     "daemon.migration.in_progress",
			Resource: events.Resource{Kind: "tenant_migration", ID: "tenant_scope"},
			Payload: map[string]any{
				"pending_steps": pending,
				"running_steps": running,
			},
		})
		logger.Slog().Warn("tenant migration not yet complete; tenant-owned routes will return HTTP 503 until backfill finishes",
			"pending_count", len(pending),
			"running_count", len(running),
			"running_steps", strings.Join(running, ", "),
		)
	}

	if completed == len(steps) && len(steps) > 0 {
		bus.Publish(events.Event{
			Category: "daemon.migration",
			Name:     "daemon.migration.completed",
			Resource: events.Resource{Kind: "tenant_migration", ID: "tenant_scope"},
			Payload: map[string]any{
				"step_count": len(steps),
			},
		})
	}

	return gate, nil
}
