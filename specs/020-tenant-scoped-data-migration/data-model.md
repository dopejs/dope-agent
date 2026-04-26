# Phase 1 Data Model: Tenant-Scoped Data Migration

This document captures the entities, schema deltas, and state transitions introduced by
Roadmap 35. The authoritative classification per persisted table and event source lives
in [`contracts/schema-inventory.md`](./contracts/schema-inventory.md); this file
describes the shape and rules.

## 1. Entities

### 1.1 Tenant (existing — Roadmap 34)

- `tenant_id TEXT PRIMARY KEY` — `ten_<random>` per
  `daemon/internal/identity/manager.go`.
- `kind TEXT NOT NULL` — `personal` or `organization`.
- Other fields owned by Roadmap 34 are unchanged.

This roadmap consumes Tenant; it does not redefine it.

### 1.2 Default Personal Tenant (existing — Roadmap 34)

The tenant created by `BootstrapLocal()` for the local operator. Roadmap 35 uses its
`tenant_id` as the backfill destination for every pre-existing row of every
`tenant_owned` table.

### 1.3 Tenant-Owned Resource (extended)

Every in-scope persisted record gains the same shape:

- `tenant_id TEXT NOT NULL` — added by this roadmap.
- Tenant-aware indexes leading with `tenant_id`.
- Tenant-aware uniqueness on natural keys that were previously global (per row in the
  inventory).

**Exception — `events` is a mixed table.** It carries both tenant-owned and
`global`-classified events (per the inventory's "Event Sources" section: `mcp-*`,
`provider-*`, `system-*`, `daemon.migration.*`, plus the reclassified
`connector_global` and `capability_global` labels for legacy emissions whose parent FK
cannot be resolved). For `events` the column stays `tenant_id TEXT` (NULLABLE), and
the invariant is enforced via a CHECK constraint — `tenant_id IS NOT NULL OR category
IN (<global-categories>)` — plus a partial UNIQUE/list index on the
`tenant_id IS NOT NULL` half and a partial index on the `tenant_id IS NULL` half.
T077b owns this enforcement step; tenancy event filters in T039 enforce the symmetric
query-policy rule (tenant-owned subscribers must add `WHERE tenant_id = ?` and never
receive NULL-tenant rows).

The set of in-scope tables is enumerated authoritatively in the schema inventory.
Examples (full list in inventory): `sessions`, `runs`, `steps`, `tool_calls`,
`llm_dispatches`, `checkpoints`, `events`, `approvals`, `decisions`, `schedules`,
`schedule_targets`, `schedule_dispatch_attempts`, `workflows`, `workflow_steps`,
`workflow_dependencies`, `workflow_handoffs`, `integrations`, `delivery_targets`,
`delivery_preferences`, `delivery_outcomes`, `delivery_attempts`,
`delivery_summary_windows`, `calendar_accounts`, `calendar_operations`,
`calendar_artifacts`, `mail_accounts`, `mail_operations`, `mail_artifacts`,
`reminders`, `reminder_occurrences`, `reminder_actions`, `computer_use_sessions`,
`computer_use_actions`, `computer_use_artifacts`, `evaluation_replay_candidates`,
`evaluation_replay_attempts`, `evaluation_comparisons`,
`evaluation_regression_fixtures`, plus harness rows
(`connector_messages`, `sandbox_executions`, `consumer_policy_records`,
`provider_preferences`, `mcp_tool_exposure_rules`, and `secret_scope_bindings` —
the last as a column-only Roadmap 37 boundary touch).

### 1.4 Schema Inventory Entry (new artifact, not a table)

A row in `contracts/schema-inventory.md` with the fields mandated by the spec:

| Field                  | Description                                                |
|------------------------|------------------------------------------------------------|
| `name`                 | Table or event source name                                 |
| `classification`       | `tenant_owned` \| `global` \| `derived`                    |
| `tenantIdSource`       | column / parent / context / `not_applicable`               |
| `migrationAction`      | `add_column_backfill` \| `index_rebuild` \| `leave_global` \| `derive_at_read` \| `remove_tenantless_access` |
| `affectedAPIs`         | List of API routes that must read tenant context           |
| `affectedEvents`       | Event names + SSE filters that must include tenant scope   |
| `storeAccess`          | Required tenant-aware helper(s); tenantless helpers to remove |
| `indexesAndUniqueness` | Tenant-aware indexes + uniqueness changes                  |
| `isolationTests`       | Same-shaped cross-tenant fixtures required                 |
| `rollback`             | `backup_restore` (constant for this delivery)              |

The Markdown table is the canonical source. The Go inventory loader parses it.

### 1.5 Migration Progress Record (new table)

Schema version 22 introduces `tenant_migration_progress`:

```sql
CREATE TABLE tenant_migration_progress (
  step_name           TEXT PRIMARY KEY,
  status              TEXT NOT NULL,    -- pending | running | completed | failed
  started_at          INTEGER,
  completed_at        INTEGER,
  last_processed_key  TEXT,
  error               TEXT
);
```

State transitions:

```
pending → running → completed
                 ↘ failed → (operator action) → pending
```

- `pending` is the initial row state when a step is registered.
- `running` is set when a step begins; `last_processed_key` is updated after each chunk
  commit.
- `completed` is set when the step finishes its work; idempotent (re-running is a
  no-op).
- `failed` records the error and prevents the daemon from serving traffic on next
  start; an operator must intervene (consult the runbook).

### 1.6 Cross-Tenant Denial Audit Event (new event type)

Emitted on the existing in-process event bus and persisted via the existing `events`
table under the standard `runtime-event.schema.json` envelope. JSON Schema lives at
`schemas/events/audit-cross-tenant-access-denied.event.schema.json` (flat layout, kebab
case — matching every other event in `schemas/events/`). The event is named distinctly
from R34's `tenant.access_denied` (which already exists at
`schemas/events/tenant-access-denied.event.schema.json` and covers identity-boundary
denials at tenant context resolution); the R35 event covers cross-tenant **data-plane**
denials.

Envelope: `category = "audit"`, `name = "audit.cross_tenant_access_denied"`,
`resource = { kind: "tenant", id: <actingTenantId> }`.

Payload fields (the only place tenant-specific data lives):

- `actingTenantId` — the tenant attempting the access (resolved tenant context); mirrors
  `resource.id`.
- `principalId` — the authenticated principal making the request, format `prn_<random>`.
- `surface` — string identifier such as `api:GET /v1/runs/{id}` or
  `store:ListSchedulesForTenant`.
- `resourceKind` — e.g. `run`, `schedule`, `integration`.

Explicitly excluded: target tenant id, target resource id, any row data. The schema
omits these fields and the emitter does not pass them in.

## 2. Schema Deltas (additive)

Each in-scope `tenant_owned` table receives the same additive change set, applied per
table by version 23+ migration steps:

```sql
ALTER TABLE <table> ADD COLUMN tenant_id TEXT;
-- backfill via tenant_migration_progress in chunked transactions
UPDATE <table> SET tenant_id = ? WHERE tenant_id IS NULL AND <pk> > ? LIMIT N;
-- after backfill completes:
CREATE INDEX idx_<table>_tenant_<list_key>
  ON <table>(tenant_id, <list_key>);
-- replace any previously global UNIQUE on natural keys:
DROP INDEX IF EXISTS uq_<table>_<natural_key>;
CREATE UNIQUE INDEX uq_<table>_tenant_<natural_key>
  ON <table>(tenant_id, <natural_key>);
-- enforce NOT NULL once backfill is complete:
-- (SQLite cannot ALTER COLUMN; rebuild via shadow-table swap if NOT NULL is required)
```

Notes:

- SQLite does not support `ALTER COLUMN ... SET NOT NULL`. For tables where `NOT NULL`
  is required (per inventory), the migration uses the standard SQLite shadow-table
  pattern: create a new table with the desired schema, copy rows, drop the original,
  rename. The shadow swap is a single migration step entry in
  `tenant_migration_progress`.
- For tables where the application layer enforces non-null tenant via the `tenancy`
  package and a CHECK constraint suffices, the simpler path is a `CHECK (tenant_id IS
  NOT NULL)` constraint added during the shadow swap, or a deferred rebuild in a
  follow-up version.

The `events` table receives the same additive `tenant_id` column and a tenant-aware
index leading the existing time/category index. Event source classification is recorded
in the inventory.

## 3. Validation Rules

- **Tenant existence**: A row whose `tenant_id` is set MUST reference an existing
  `tenants.tenant_id`. Application-layer enforcement only (no SQLite FK enabled by
  default in this codebase).
- **Tenant context required**: All store helpers on `tenant_owned` tables MUST require
  a non-empty resolved `TenantID` argument. Calls without one return
  `tenancy.ErrTenantContextRequired`.
- **Tenant immutability**: After a row is created, its `tenant_id` MUST NOT change.
  Update helpers reject changes; the schema does not provide a path to mutate it.
- **Per-tenant uniqueness**: Where the inventory specifies, natural keys are unique
  per tenant via `UNIQUE (tenant_id, <natural_key>)`. Two tenants with the same natural
  key both succeed.

## 4. Lifecycle / State Transitions Touched

This roadmap does not change resource lifecycles (run state machine, schedule trigger
states, etc.). It only adds an ownership dimension. Any state transition emitted as an
event continues to fire and now carries `tenantId` per FR-014.

## 5. Out-of-Scope Data Model Changes

Per FR-015, FR-016, and FR-017:

- No secret value columns added or changed.
- No OAuth refresh, scope, or rotation semantics added or changed.
- No connector or MCP administration permission columns added.
- No per-tenant physical database schema.
- No billing usage counter tables.
- No cross-tenant admin projection columns or views.

Tables held in common with Roadmap 37 (e.g. `integrations`, `computer_use_sessions`)
receive the additive `tenant_id` column and tenant-aware index/uniqueness only; their
credential semantics are untouched.
