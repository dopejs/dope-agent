# Phase 0 Research: Tenant-Scoped Data Migration

This document resolves the open design questions raised during planning so that
data-model.md, contracts/, and quickstart.md can be written without `NEEDS CLARIFICATION`
markers. The five product-level clarifications are already recorded in `spec.md` under
`## Clarifications`. This file covers the remaining technical-design questions.

## R1. Tenant id column representation and indexing

**Decision**: Add a `tenant_id TEXT NOT NULL` column to every `tenant_owned` table.
Tenant-aware composite indexes lead with `tenant_id`, e.g. `(tenant_id, created_at DESC,
<row_id>)`. Per-tenant uniqueness becomes `UNIQUE (tenant_id, <natural_key>)`.

**Rationale**:
- Matches the existing `TenantID = string` representation in
  `daemon/internal/identity/types.go` (no new typedef churn).
- `NOT NULL` enforces FR-007 (reject tenantless writes) at the storage layer rather than
  relying on application code.
- Composite indexes leading with `tenant_id` give SQLite a deterministic plan for
  list-by-tenant queries (verified in Q5 with a query-plan assertion). Also ensures the
  indexes are useful for `WHERE tenant_id = ? AND id = ?` get paths.
- Per-tenant `UNIQUE` keeps natural keys (e.g. integration name) from colliding across
  tenants, satisfying FR-009.

**Alternatives considered**:
- Typed alias `type TenantID string` package-wide: deferred. Pure refactor with no
  behavior change; would expand the diff and slow review without preventing any class of
  bug. Can be added later without a schema change.
- Per-tenant tables (e.g. `runs_<tenant>`): rejected, conflicts with the upstream design
  doc's "shared tables with strong tenant scoping" decision and would block per-tenant
  physical DB future work without a much larger rewrite.
- Foreign key `REFERENCES tenants(tenant_id)`: deferred. SQLite foreign keys are off by
  default in this codebase; enabling them broadly is out of scope. The combination of
  `NOT NULL` + tenant existence check at the API/middleware layer is sufficient for the
  isolation invariant.

## R2. Migration progress durability and per-step granularity

**Decision**: Add a new table `tenant_migration_progress` (schema version 22) containing
`(step_name TEXT PRIMARY KEY, status TEXT NOT NULL, started_at INTEGER, completed_at
INTEGER, last_processed_key TEXT, error TEXT)`. The migration framework writes a row per
step (one per in-scope table or backfill phase). Backfills page through rows ordered by
their primary key, persist `last_processed_key` after each chunk commit, and resume from
that key on restart.

**Rationale**:
- The existing `schemaMigrations` array (in `daemon/internal/store/store.go`) tracks
  whole-version application but does not survive mid-step failures — a long backfill
  that crashes today would be re-run from scratch.
- A separate progress table is simpler than splitting backfills into many micro-versions
  (which would explode `CurrentSchemaVersion`).
- Chunked, key-ordered backfill keeps individual transactions small (bounded write
  amplification) and gives the resume path a deterministic restart point.
- `status` codes: `pending` / `running` / `completed` / `failed`. Daemon refuses to
  start when any row is `failed` or when schema state and progress disagree (Q1's
  "unsafe state").

**Alternatives considered**:
- Reuse `schema_migrations` table with extra columns: rejected. Migration framework
  semantics are "version applied" — stretching them to a partial-completion model would
  break the invariant other code relies on.
- In-memory progress with restart-from-zero: rejected by Q1.
- Separate progress file outside SQLite: rejected — splits the durable boundary across
  two stores and complicates atomic rollback.

## R3. Tenant-aware store helper API shape

**Decision**: Introduce `daemon/internal/store/tenancy` exposing two contracts:

1. `RequireTenant(ctx) (TenantID, error)` — pulls the resolved tenant from the request
   context (already populated by the existing `protected()` middleware) and returns an
   error sentinel when missing. All tenant-owned store helpers call this on entry.
2. A scoped query helper that prepends `WHERE tenant_id = ?` to every list/get on
   tenant-owned tables and rejects writes that lack a tenant value.

The existing tenantless helpers on tenant-owned tables (e.g. `Store.ListRuns`) are
either (a) deleted and replaced with `Store.ListRunsForTenant(ctx, …)` or (b) made
internal/private and given a `tenantID` parameter. The choice per helper is recorded in
the schema inventory's `storeAccess` field.

**Rationale**:
- Centralizing the rule prevents each domain from re-implementing tenant filtering and
  drifting.
- A tenantless helper that still exists on a tenant-owned table is a foot-gun even if
  every current call site is correct — eliminating the helper makes new misuse a compile
  error, satisfying FR-006.
- Returning an explicit error from `RequireTenant` keeps the failure mode deterministic
  (FR-007, SC-005).

**Alternatives considered**:
- Reflection-based interception of all queries: rejected, too magic and hides intent.
- Apply tenant filter only at the API layer: rejected — leaves background tasks and
  internal callers free to bypass it. Spec FR-006 explicitly forbids this.

## R4. Inventory artifact format and completeness test

**Decision**: The schema inventory is checked in as Markdown at
`specs/020-tenant-scoped-data-migration/contracts/schema-inventory.md` with a strict,
machine-parseable table format (one row per table or event source). A small Go parser in
`daemon/internal/inventory` reads the Markdown into a map and a unit test compares the
parsed set against (a) the live SQLite schema introspected from a freshly-initialized
test DB and (b) the registered event source list. Drift fails CI.

**Rationale**:
- Markdown is human-reviewable; engineers reviewing a tenant-related PR will read the
  inventory diff naturally.
- A strict table grammar is parseable enough for the completeness test without taking
  on a YAML/JSON authoring step.
- Locating the inventory under `specs/.../contracts/` keeps it next to the spec it
  derives from; the consuming Go test imports the file with a relative path, not a
  build-time copy.

**Alternatives considered**:
- JSON inventory: rejected — review diffs are noisier and the file gains less from being
  machine-only.
- Inline inventory inside `spec.md`: rejected — makes the spec massive and conflates
  product-level scope with implementation-level table list.
- Generated-from-code inventory: rejected — defeats the purpose of an explicit
  classification artifact, since the test would just compare code to itself.

## R5. Audit event placement on the existing event surface

**Decision**: Emit a new event `audit.cross_tenant_access_denied` on the existing
in-process event bus (`daemon/internal/events`) and persist it via the existing
`events` table under the standard `runtime-event.schema.json` envelope. A new JSON
Schema lives at `schemas/events/audit-cross-tenant-access-denied.event.schema.json`
(flat kebab-case, matching every other event file). The R34 event
`tenant.access_denied` (file `tenant-access-denied.event.schema.json`) already exists
and covers identity-boundary denials at tenant context resolution; the R35 event is
intentionally distinct and covers cross-tenant data-plane denials. Envelope: `category
= audit`, `name = audit.cross_tenant_access_denied`, `resource = { kind: "tenant", id:
<actingTenantId> }`. Payload includes only: `actingTenantId`, `principalId`, `surface`
(e.g. `"api:GET /v1/runs/{id}"` or `"store:ListSchedulesForTenant"`), `resourceKind`.
The target tenant id, target resource id, and any row data are explicitly excluded.
`occurredAt` lives on the runtime-event envelope (not in payload).

**Rationale**:
- Reusing the existing event surface avoids inventing a parallel observability pipeline.
- A typed schema lets clients and operator dashboards subscribe via the same event
  filter mechanism used elsewhere.
- The omission list is enforced by the schema (no `targetTenantId` field) and by the
  audit emitter (no row data ever passed in).

**Alternatives considered**:
- Log-only: rejected by Q3.
- Counter only: deferred — the typed event is the durable signal; counters can be
  derived from it later without changing this delivery.
- Storing the event in a separate `audit_events` table: rejected — fragments operator
  observability and forces a second SSE subscription.

## R6. Pre-tenant fixture construction

**Decision**: Check in a small SQLite snapshot at `daemon/internal/store/testdata/`
captured at the current schema version (21, immediately before this roadmap's first
migration). The fixture contains a representative set of pre-tenant rows across every
in-scope domain. Migration tests open this snapshot, run the new migrations, and assert
that every row is owned by the default personal tenant after the upgrade.

**Rationale**:
- A real binary fixture exercises the actual migration path against a real schema-21
  shape, which is more honest than synthesizing the same shape inside Go code.
- Keeps the migration test independent of future schema versions: the snapshot is
  immutable; new migrations always replay against it.

**Alternatives considered**:
- Programmatically build a v21 DB at test time: rejected — duplicates the schema as Go
  literals and risks drift.
- Use an existing operator's DB: rejected — privacy and reproducibility concerns.

## R7. Cross-tenant isolation regression shape

**Decision**: Each in-scope domain (runs, schedules, integrations, deliveries, calendar,
mail, reminders, computer-use, evaluation, workflows) gets a table-driven test in both
`daemon/internal/store/isolation_test.go` (store layer) and
`daemon/internal/api/isolation_test.go` (API + SSE). The pattern: provision tenants A
and B with same-shaped resources, perform every list/get/create/update/delete/event
operation as A, assert that B's records are neither returned nor mutated, and assert
the inverse from B.

**Rationale**:
- Table-driven tests force every domain to declare its isolation expectations in one
  place and make missing-domain coverage visible at code review.
- Splitting store and API tests catches both "store helper accepts tenantless context"
  and "route handler forgets to pass tenant context".

**Alternatives considered**:
- One end-to-end test per domain: rejected — slower and harder to localize a regression.
- Property-based testing: deferred — overkill for the first delivery.

## R8. Performance baseline and latency assertion

**Decision**: For the small number of tables expected to grow large in single-tenant use
(initially `runs`, `events`, and `mail_artifacts`), the test suite includes a fixture
generator that seeds N rows (N=10000 by default for local CI; configurable for nightly
runs), measures p95 list latency before applying tenant migrations, applies migrations,
re-measures, and asserts post ≤ 1.2× pre on the same fixture. The query-plan assertion
runs `EXPLAIN QUERY PLAN` and verifies the named tenant-aware index is used.

**Rationale**:
- A relative budget is hardware-neutral and survives CI runner variance better than an
  absolute latency target.
- Pinning the assertion to the same fixture in the same process eliminates noise from
  cold caches or other concurrent tests.

**Alternatives considered**:
- Absolute latency target: rejected by Q5.
- Query-plan assertion only: rejected by Q5 — catches structural regressions but not
  behavioral ones.

## R9. Backward-compatible client serialization

**Decision**: Tenant fields are added to API/event JSON payloads as optional fields in
the schema and as JSON-omittable fields on the Go side (no `omitempty` removal of
existing fields). The TypeScript SDK consumer regenerates its types but does not change
runtime behavior in this roadmap.

**Rationale**:
- Existing clients that don't know about `tenantId` continue to deserialize correctly.
- The web/TUI tenant-aware behavior change is owned by Roadmap 36
  (Tenant-aware Operator Shell And SDK), not this roadmap.

## R10. Domains and tables held in common with Roadmap 37

**Decision**: Tables touched by both this roadmap and Roadmap 37 (notably `integrations`,
`computer_use_sessions`, future MCP/connector tables) get the additive `tenant_id`
column and tenant-aware index/uniqueness from this roadmap. This roadmap MUST NOT touch
secret value columns, OAuth refresh semantics, redaction rules, or connector
administration permissions on those tables — those remain owned by Roadmap 37.

**Rationale**:
- Establishes the isolation invariant without reaching into credential semantics.
- Keeps the diff small and the ownership boundary explicit, matching FR-015.

---

All Phase 0 questions resolved. No `NEEDS CLARIFICATION` markers remain. Proceed to
Phase 1.
