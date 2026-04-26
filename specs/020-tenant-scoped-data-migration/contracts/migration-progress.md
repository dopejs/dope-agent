# Contract: Tenant Migration Progress

This contract pins the externally-observable behavior of the resume-safe per-step
migration introduced by Roadmap 35. It defines the operator-visible state model and the
guarantees the daemon makes on restart.

## Persistence

A new SQLite table `tenant_migration_progress` (added in schema version 22):

```sql
CREATE TABLE tenant_migration_progress (
  step_name           TEXT PRIMARY KEY,
  status              TEXT NOT NULL,    -- pending | running | completed | failed
  started_at          INTEGER,          -- unix seconds
  completed_at        INTEGER,          -- unix seconds
  last_processed_key  TEXT,             -- domain-specific resume cursor
  error               TEXT
);
```

`status` is the only field that drives daemon startup behavior. The other fields exist
for operator visibility and resume cursors.

## Step Identity

A "step" is the smallest unit of resumable work. Each in-scope migration consists of
one or more steps. Step names are stable strings so that operators and tests can refer
to them. Examples:

- `tenant_migration:add_column:runs`
- `tenant_migration:backfill:runs`
- `tenant_migration:rebuild_indexes:runs`
- `tenant_migration:shadow_swap_not_null:runs`

Steps within a single in-scope table run in the order above. Steps across tables may
run in parallel only when both targets are independent.

## State Machine

```
            register
              │
              ▼
   ┌──────► pending
   │          │ start
   │          ▼
   │       running ──────► completed
   │          │  (last_processed_key updated per chunk commit)
   │          │
   │          │ unrecoverable error
   │          ▼
   │       failed
   │          │ operator action
   └──────────┘  (consult runbook; either backup-restore or manual fix → reset to pending)
```

- `pending → running`: set on entry into the step.
- `running → running`: each chunk commit updates `last_processed_key` and refreshes the
  row in the same transaction as the chunk.
- `running → completed`: set when the step finishes its work. Re-running a `completed`
  step is a no-op.
- `running → failed`: set when the step encounters an unrecoverable error. `error`
  contains the operator-actionable message.

## Daemon Startup Behavior

On every daemon start, before serving any API or event traffic:

1. Apply pending schema-version migrations (existing behavior).
2. Scan `tenant_migration_progress`:
   - If any row is `failed` → daemon refuses to start. Emit `daemon.migration.failed`
     event and a structured log line naming the failed `step_name` and `error`. Exit
     non-zero.
   - If any row is `running` → resume from `last_processed_key` for that step. This is
     the normal recovery path after a crash or kill.
   - If all rows are `completed` and the in-scope table set matches the inventory →
     proceed to serve traffic.
3. While migration is in progress, the daemon MUST NOT serve any tenant-owned API or
   SSE traffic (returns HTTP 503 with a stable error code), and MUST NOT publish
   tenant-owned events. Global routes (health check, identity bootstrap admin) MAY
   serve.
4. On successful completion of all in-scope steps, emit `daemon.migration.completed`
   and proceed to serve normally.

## Idempotence Guarantees

- Re-running a `completed` step MUST NOT modify any tenant-owned row.
- Re-running the entire migration after it has completed MUST result in zero row
  changes (verified by SC-004).
- `last_processed_key` resume MUST process every row exactly once across the combined
  pre-crash and post-crash passes.

## Operator Surfaces

- `daemon.migration.started`, `daemon.migration.step_started`,
  `daemon.migration.step_completed`, `daemon.migration.step_failed`,
  `daemon.migration.completed`, `daemon.migration.failed` events on the existing event
  bus, persisted via the events table. These are `global`-classified per the inventory
  (no tenant scoping, since they predate any tenant being established for migrated
  rows).
- Structured log lines with stable codes (e.g. `migration.step_completed`,
  `migration.step_failed`) for grep-based monitoring.
- Healthcheck endpoint reports `migration_status` with values `not_started`,
  `in_progress`, `completed`, or `failed:<step_name>`.

## Rollback

Per Q2 / FR-011, the only supported rollback is restoring a pre-migration database
backup. There is no down-migration step. The runbook
(`docs/runtime/tenant-migration-rollback.md`) documents:

1. Stop the daemon.
2. Restore the pre-migration backup over the daemon's data directory.
3. Restart the daemon. (It will see schema version 21 and the absence of
   `tenant_migration_progress` and serve as before.)

If an operator did not take a pre-migration backup, rollback is unavailable. The
upgrade documentation MUST require taking a backup as a hard prerequisite.

## Test Surfaces

- A migration test seeds a checked-in pre-tenant SQLite snapshot, runs the migration,
  and asserts every in-scope row is owned by the default personal tenant.
- A resume-safety test interrupts a backfill mid-step (by aborting the running
  transaction at a chosen `last_processed_key`), restarts the daemon, and asserts the
  resumed run completes the same step exactly once.
- An unsafe-state test injects a `failed` row in `tenant_migration_progress` and
  asserts the daemon refuses to start.
- A re-run idempotence test runs the full migration twice on a clean fixture and
  asserts the second run produces zero row changes.
