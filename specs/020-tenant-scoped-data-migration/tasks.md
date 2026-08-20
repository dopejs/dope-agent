---
description: "Task list for Roadmap 35 — Tenant-Scoped Data Migration"
---

# Tasks: Tenant-Scoped Data Migration

**Input**: Design documents from `specs/020-tenant-scoped-data-migration/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`

**Tests**: Per the constitution and Verification Expectations in spec.md, every change set
below carries targeted verification tasks: contract tests for additive schema/event
fields, store-layer tests for tenant-aware helpers, API isolation regressions,
inventory-completeness tests, migration tests from a pre-tenant fixture, resume-safety
tests, and query-plan + relative-latency assertions.

**Organization**: Tasks are grouped by user story so each story can be implemented and
verified as a complete increment. US1 and US2 are both P1 and together form the MVP for
this roadmap (isolation invariant + migration of existing data). US3 is the
correctness-gate suite that prevents regressions on future roadmaps.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: parallelizable (different files, no dependencies on incomplete tasks)
- **[Story]**: required on US1/US2/US3 phase tasks only
- File paths are absolute under the worktree root
  `/Users/John/Code/kura-agent/.worktrees/020-claude/`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Confirm developer environment and stub the new packages so subsequent phases
have somewhere to land code. Inventory artifact and audit event schema were authored by
/speckit.plan and live under `specs/020-tenant-scoped-data-migration/contracts/`.

- [X] T001 Verify worktree environment: branch is `020-claude`, `make daemon-build` and `make daemon-test` pass on the existing `main` checkpoint, `pnpm install` and `cd daemon && go mod download` already done; record the pre-roadmap baseline `make daemon-test` output for comparison in `specs/020-tenant-scoped-data-migration/quickstart.md` (no edits to source).
- [X] T002 [P] Create empty Go package `daemon/internal/inventory/` with `daemon/internal/inventory/doc.go` summarizing purpose ("loads schema-inventory.md and verifies completeness against live SQLite schema and event registry").
- [X] T003 [P] Create empty Go package `daemon/internal/store/tenancy/` with `daemon/internal/store/tenancy/doc.go` summarizing purpose ("centralizes tenant context resolution and tenant-aware store access for tenant-owned tables").

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Land the migration framework, tenant-aware store primitive, audit emitter,
event-bus tenant filter, and daemon startup gate. None of the user-story phases can begin
until this phase completes — every per-domain task in US1/US2 imports from these packages
or relies on the daemon refusing to serve traffic mid-migration.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T004 Add schema migration v22 `tenant_migration_progress` to `daemon/internal/store/store.go` (`schemaMigrations` array): `CREATE TABLE tenant_migration_progress (step_name TEXT PRIMARY KEY, status TEXT NOT NULL, started_at INTEGER, completed_at INTEGER, last_processed_key TEXT, error TEXT)`; bump `CurrentSchemaVersion` to 22.
- [X] T005 Implement `daemon/internal/store/migration_progress.go`: `RegisterStep(name)`, `BeginStep(name)`, `RecordChunk(name, lastKey)`, `CompleteStep(name)`, `FailStep(name, err)`, `LoadAll()` returning the state-machine view from contracts/migration-progress.md. All writes through SQLite transactions.
- [X] T006 Add `daemon/internal/store/migration_progress_test.go` covering: state transitions (pending→running→completed, running→failed→pending after operator reset), idempotent CompleteStep, RecordChunk persists `last_processed_key` per commit, concurrent-step isolation by `step_name`.
- [X] T006a Refactor the tenant context carrier into a new shared package `daemon/internal/tenantctx/` so both `daemon/internal/api/` and `daemon/internal/store/tenancy/` (and any future caller) can import it. Currently `tenantContextKey`, `withTenantContext`, and `tenantContextFromContext` live unexported inside `daemon/internal/api/server.go` (server.go:4657–4658, types.go:804–810); `daemon/internal/store/tenancy` cannot legally depend on the api package. The new package exposes: type alias for `identity.TenantContext`, `WithContext(ctx, identity.TenantContext) context.Context`, `FromContext(ctx) (identity.TenantContext, bool)`, and a single sentinel `ErrTenantContextRequired error`. `daemon/internal/api/server.go` and `types.go` are updated so `protected()` calls `tenantctx.WithContext(...)` and the existing api-private `tenantContextFromContext` becomes a thin one-line wrapper (or is removed entirely and call sites switch to `tenantctx.FromContext`). No API or persistence-surface change. (depends on Roadmap 34 identity package; blocks T007)
- [X] T007 Implement `daemon/internal/store/tenancy/tenancy.go`: re-export `tenantctx.ErrTenantContextRequired` (single source of truth); implement `RequireTenant(ctx) (identity.TenantID, error)` that calls `tenantctx.FromContext` and returns the sentinel when missing; implement `MustTenant(ctx)` for places where missing context is a programmer error. The store/tenancy package depends ONLY on `daemon/internal/tenantctx` and `daemon/internal/identity` for tenant-context carrier semantics — never on `daemon/internal/api`. As a fallback for callers that legitimately have a `TenantID` value but no context (e.g. background jobs that resolve tenancy from a stored row before opening a request context), every tenant-owned helper signature also accepts `tenantID identity.TenantID` as an explicit parameter; the `ForTenant` variants extract it via `RequireTenant(ctx)` first and pass it through. Store-layer code never imports api-private symbols. (depends on T006a)
- [X] T008 Add `daemon/internal/store/tenancy/tenancy_test.go` proving: missing tenant returns `ErrTenantContextRequired`; resolved tenant is returned verbatim; concurrent contexts are isolated; the tenancy package does not import `daemon/internal/api` (enforced by `go list -deps` assertion in the same test file). (depends on T007)
- [X] T009 Implement `daemon/internal/inventory/inventory.go`: parser for `specs/020-tenant-scoped-data-migration/contracts/schema-inventory.md` into `[]Entry{Name, Classification, TenantIDSource, MigrationAction, AffectedAPIs, AffectedEvents, StoreAccess, IndexesAndUniqueness, IsolationTests, Rollback}`; rejects unknown classification or migrationAction values.
- [X] T010 [P] Add `daemon/internal/inventory/inventory_test.go` covering: parser round-trips a known fixture; rejects malformed rows; loads the real `schema-inventory.md` and asserts every required column is non-empty.
- [X] T011 Add `schemas/events/audit-cross-tenant-access-denied.event.schema.json` by copying `specs/020-tenant-scoped-data-migration/contracts/tenant-audit-event.json` verbatim into the flat `schemas/events/` directory (kebab-case, no subdirectories — matches every other event schema). The repository does not maintain a separate event registry file (verified). The new event is named distinctly from R34's existing `tenant-access-denied.event.schema.json` (which covers identity-boundary denials); this one covers cross-tenant data-plane denials with `category=audit`, `name=audit.cross_tenant_access_denied`.
- [X] T012 Implement `daemon/internal/audit/tenant_breach.go`: `Emit(ctx, surface, resourceKind)` constructs the payload from the resolved tenant context, never accepts a target tenant id or row data argument; publishes via the existing event bus with envelope category `audit` and name `audit.cross_tenant_access_denied` and resource `{ kind: "tenant", id: <actingTenantId> }`; also writes a structured log line with stable code `audit.cross_tenant_access_denied`. Schema reference: `schemas/events/audit-cross-tenant-access-denied.event.schema.json` (T011).
- [X] T013 [P] Add `daemon/internal/audit/tenant_breach_test.go`: emitter rejects calls without tenant context; envelope has `category=audit`, `name=audit.cross_tenant_access_denied`, `resource.kind=tenant`, `resource.id=<actingTenantId>`; payload contains exactly `actingTenantId, principalId, surface, resourceKind` (no target tenant id, no row data); the full event validates against `schemas/events/audit-cross-tenant-access-denied.event.schema.json`; structured log line with code `audit.cross_tenant_access_denied` is emitted alongside the event.
- [X] T014 Update `daemon/internal/events/bus.go`: add `TenantID identity.TenantID` field to `Filter`; enforce on `Publish` (tenant-owned categories require tenant context on the publisher side) and on subscriber fan-out (subscribers receive only events where the event's tenant matches their filter).
- [X] T015 [P] Add `daemon/internal/events/bus_test.go` cases: tenantless publish on a tenant-owned category returns error; subscribers with `Filter.TenantID = A` never receive `Filter.TenantID = B` events; legacy filter without `TenantID` rejects tenant-owned categories.
- [X] T016 Add schema migration v23 to `daemon/internal/store/store.go`: `ALTER TABLE events ADD COLUMN tenant_id TEXT` (NULLABLE — step (a) only, per the three-stage pattern); add indexes `idx_events_tenant_time (tenant_id, occurred_at DESC)` and `idx_events_tenant_category_time (tenant_id, category, name, occurred_at DESC)`; register the backfill step `tenant_migration:backfill:events` and the step (c) enforcement step `tenant_migration:enforce_check:events` (the canonical step name used everywhere; events is a mixed table, so step (c) is a CHECK + partial-index pair, not a NOT NULL swap — see T077b) in `tenant_migration_progress` as `pending` (no rows are written here). T016 explicitly does NOT perform the events backfill — that is owned by T077 in US2 and runs only after every parent table's tenant_id has been added (v24..v32) and every parent backfill step has reached `completed`. T077b then runs step (c) for `events` after T077a finishes the NOT NULL swaps for the other tenant-owned tables.
- [X] T017 Update `daemon/internal/app/app.go` startup sequence: after schema migrations apply, call `migration_progress.LoadAll()`; if any row is `failed` or schema and progress disagree, refuse to start and emit `daemon.migration.failed`; if any row is `running`, resume the step; while migration is in progress, return HTTP 503 with stable error code on tenant-owned routes; emit `daemon.migration.started`, `daemon.migration.step_started`, `daemon.migration.step_completed`, `daemon.migration.completed` events as documented in `contracts/migration-progress.md`.

**Checkpoint**: Foundation ready — per-domain user-story work can now begin.

---

## Phase 3: User Story 1 — Hosted user sees only their tenant's data (Priority: P1) 🎯 MVP

**Goal**: Tenant-owned reads, writes, events, and SSE replay return only the acting
tenant's records across every in-scope domain. Cross-tenant access is rejected at the
store layer and surfaced as both a structured log line and a typed audit event.

**Independent Test**: Provision tenants A and B with same-shaped fixtures; perform every
in-scope list / get / create / update / delete / event / replay operation as A; assert
B's records are neither returned nor mutated. Repeat as B. Removing the tenant filter
from any in-scope helper or route causes at least one isolation test to fail
deterministically.

### Per-domain schema migrations (US1)

These add `tenant_id`, tenant-aware indexes, and tenant-aware uniqueness per the
inventory's `indexesAndUniqueness` column. Each is a separate `schemaMigrations` entry to
keep diffs small and resumable. NOT NULL is enforced via the SQLite shadow-table swap
where the inventory requires it; otherwise via `CHECK (tenant_id IS NOT NULL)` once the
backfill (US2) completes.

All DDL below uses the real columns from `daemon/internal/store/store.go` schema v21 head; UNIQUE constraints are added only where a real SQL column carries the natural key (display names that live in `document_json` are NOT promoted to SQL constraints). Each task is a separate `schemaMigrations` entry to keep diffs small.

**Three-stage per-domain pattern (applies to every T018–T026 task)**: each domain's
schema migration ships in three sequenced sub-steps to avoid the
"NOT NULL before backfill" deadlock. (a) Add `tenant_id TEXT` (NULLABLE) plus the
tenant-aware indexes — both new and replacement; (b) the domain's US2 backfill
populates `tenant_id` for every existing row; (c) a follow-up migration (paired with
the domain's backfill task in US2 — see T067a, T068a, …) enforces non-null via SQLite
shadow-table swap (recreate the table with `tenant_id NOT NULL` and the new
constraints, copy rows, drop, rename). Step (c) MUST run only after the matching US2
backfill step writes `completed` to `tenant_migration_progress`. Tasks T018–T026
deliver only steps (a) for their domains; the step (c) tasks live in US2 and are
listed below the backfill tasks. UNIQUE constraints declared in T018–T026 are added
in step (c), not step (a), since a UNIQUE on a partially-NULL column would behave
inconsistently across SQLite versions.

- [X] T018 [P] [US1] Schema v24 — runtime domain: add tenant ownership to `sessions`, `runs`, `steps`, `tool_calls`, `llm_dispatches`, `checkpoints` in `daemon/internal/store/store.go`. Indexes per inventory: `(tenant_id, created_at DESC)` on `sessions`; `(tenant_id, created_at DESC, run_id DESC)` on `runs`; `(tenant_id, run_id, created_at DESC)` on `steps`; `(tenant_id, step_id)` and `(tenant_id, status, created_at DESC)` on `tool_calls`; `(tenant_id, created_at DESC)` and `(tenant_id, provider, status, created_at DESC)` on `llm_dispatches` (the table has no run_id/step_id FK; tenant comes from the authenticated context at dispatch time and per-run filtering is done via the events table); `(tenant_id, run_id, captured_at DESC)` on `checkpoints`. Uniqueness: replace existing global UNIQUE on `sessions.routing_key` with UNIQUE `(tenant_id, routing_key)` via shadow-table swap. (`runs.run_id` is the global PK so per-tenant UNIQUE is implicit and not added.) [Step (a) complete in v24; UNIQUE shadow-swap deferred to T077a per the three-stage pattern.]
- [X] T019 [P] [US1] Schema v25 — schedules domain: add tenant ownership to `schedules`, `schedule_targets`, `schedule_dispatch_attempts` (the real table is `schedule_dispatch_attempts`). Indexes: `(tenant_id, status, next_due_at, schedule_id)` on `schedules` (the real column is `next_due_at`, not `next_fire_at`); `(tenant_id, schedule_id, target_ref_id)` on `schedule_targets`; `(tenant_id, schedule_id, due_at DESC, attempt_id DESC)` on `schedule_dispatch_attempts` (real column is `due_at`). Uniqueness: UNIQUE `(tenant_id, kind, target_ref_id)` on `schedules` (mirrors application-level dedupe; schedule display name lives in `document_json`).
- [X] T020 [P] [US1] Schema v26 — workflows domain: add tenant ownership to `workflows`, `workflow_steps`, `workflow_dependencies`, `workflow_handoffs` (there is no `workflow_runs` table). `workflows.tenant_id` is derived from `runs.tenant_id` via the existing `workflows.run_id` FK during backfill. Indexes: `(tenant_id, updated_at DESC, workflow_id DESC)` on `workflows`; `(tenant_id, workflow_id, position)` on `workflow_steps` (real column is `position`); `(tenant_id, workflow_id)` on `workflow_dependencies` and `workflow_handoffs`. No SQL UNIQUE on workflow display name (lives in `document_json`); `workflow_id` global PK is sufficient.
- [X] T021 [P] [US1] Schema v27 — integrations + delivery domain: add tenant ownership to `integrations`, `delivery_targets`, `delivery_preferences`, `delivery_outcomes`, `delivery_attempts`, `delivery_summary_windows` (no single `deliveries` table). Indexes: `(tenant_id, domain_kind, account_key, canonical_default)` and `(tenant_id, readiness_status, updated_at DESC, integration_id DESC)` on `integrations` (real columns are `domain_kind`, `account_key`, `readiness_status` — there is no `kind`, `status`, or `name` column); `(tenant_id, status, updated_at DESC, target_id DESC)` on `delivery_targets`; `(tenant_id, scope_kind, integration_id, active, updated_at DESC, preference_id DESC)` on `delivery_preferences`; `(tenant_id, updated_at DESC, delivery_id DESC)` on `delivery_outcomes` (PK is `delivery_id`); `(tenant_id, delivery_id, attempt_number ASC, attempt_id ASC)` on `delivery_attempts` (FK is `delivery_id`, not `outcome_id`); `(tenant_id, status, window_ends_at ASC, summary_window_id ASC)` on `delivery_summary_windows` (real columns `summary_window_id`, `window_ends_at`). Uniqueness (added in step (c) per the three-stage pattern): UNIQUE `(tenant_id, domain_kind, account_key)` on `integrations` filtered to non-NULL `account_key` (integration display name lives in `document_json`).
- [X] T022 [P] [US1] Schema v28 — calendar domain: add tenant ownership to `calendar_accounts`, `calendar_operations`, `calendar_artifacts`. Indexes: `(tenant_id, readiness_status, updated_at DESC, calendar_account_id DESC)` on `calendar_accounts` (PK is `calendar_account_id`; `integration_id` already has a global UNIQUE — preserve it); `(tenant_id, calendar_account_id, updated_at DESC, operation_id DESC)` on `calendar_operations` (FK is `calendar_account_id`); `(tenant_id, operation_id, created_at ASC, artifact_id ASC)` on `calendar_artifacts`. Uniqueness: UNIQUE `(tenant_id, account_key)` on `calendar_accounts` filtered to non-NULL `account_key` (partial index).
- [X] T023 [P] [US1] Schema v29 — mail domain: add tenant ownership to `mail_accounts`, `mail_operations`, `mail_artifacts`. Indexes: `(tenant_id, readiness_status, updated_at DESC, mail_account_id DESC)` on `mail_accounts`; `(tenant_id, mail_account_id, updated_at DESC, operation_id DESC)` on `mail_operations` (FK is `mail_account_id`); `(tenant_id, operation_id, created_at ASC, artifact_id ASC)` on `mail_artifacts`. Uniqueness: UNIQUE `(tenant_id, account_key)` on `mail_accounts` filtered to non-NULL `account_key`.
- [X] T024 [P] [US1] Schema v30 — reminders domain: add tenant ownership to `reminders`, `reminder_occurrences`, `reminder_actions`. Indexes: `(tenant_id, current_state, updated_at DESC, reminder_id DESC)` and `(tenant_id, next_due_at, updated_at DESC, reminder_id DESC)` on `reminders` (real columns `current_state`, `next_due_at` — there is no `status` or `due_at`); `(tenant_id, reminder_id, scheduled_for DESC, occurrence_id DESC)` on `reminder_occurrences`; `(tenant_id, reminder_id, created_at DESC, action_id DESC)` on `reminder_actions` (real column `created_at`). No SQL UNIQUE on reminder display name (lives in `document_json`).
- [X] T025 [P] [US1] Schema v31 — computer-use domain: add tenant ownership to `computer_use_sessions`, `computer_use_actions`, `computer_use_artifacts`. Indexes: `(tenant_id, run_id, updated_at DESC, computer_use_session_id DESC)` on `computer_use_sessions` (PK is `computer_use_session_id`; `run_id` FK is NOT NULL so `computer_use_sessions.tenant_id` is derived from `runs.tenant_id` at backfill time); `(tenant_id, computer_use_session_id, requested_at ASC, computer_use_action_id ASC)` on `computer_use_actions` (real columns are `computer_use_session_id` and `requested_at` — there is no `session_id` or `occurred_at`); `(tenant_id, computer_use_action_id, created_at ASC, artifact_id ASC)` on `computer_use_artifacts` (PK is `artifact_id`; FKs `computer_use_session_id` and `computer_use_action_id`).
- [X] T026 [P] [US1] Schema v32 — evaluation + harness domain: add tenant ownership to `evaluation_replay_candidates`, `evaluation_replay_attempts`, `evaluation_comparisons`, `evaluation_regression_fixtures` (real table is `evaluation_regression_fixtures`); also add tenant ownership in the same migration to `approvals`, `decisions`, `connector_messages`, `sandbox_executions`, `consumer_policy_records`, `provider_preferences`, `mcp_tool_exposure_rules`, `secret_scope_bindings` (secret_scope_bindings credential semantics remain owned by Roadmap 37). Indexes per inventory. For `mcp_tool_exposure_rules`: existing PK is `(server_id, tool_name, runtime_surface)` (no `tool_id`); per-tenant uniqueness extends the PK to `(tenant_id, server_id, tool_name, runtime_surface)` via shadow swap in step (c). Evaluation uniqueness (step (c)): UNIQUE `(tenant_id, manifest_path)` on `evaluation_regression_fixtures` (real natural key — `name` lives in `document_json`).
- [X] T027 [US1] Schema v33: verified-only migration for Roadmap 34 tenant tables. Confirmed in v21 that `memberships`, `tenant_invitations`, `token_tenant_grants`, and `tenant_audit_events` carry `tenant_id` and have tenant-aware list indexes (`idx_memberships_tenant_status`, `idx_invitations_tenant_status`, `idx_token_grants_tenant_status`, `idx_tenant_audit_events_tenant_created`). Add the missing per-tenant uniqueness constraints `CREATE UNIQUE INDEX IF NOT EXISTS uq_memberships_tenant_principal ON memberships(tenant_id, principal_id)` and `CREATE UNIQUE INDEX IF NOT EXISTS uq_token_grants_tenant_token ON token_tenant_grants(tenant_id, token_id)`. No column adds; no backfill (rows already have tenant_id). Document the verification in the migration step's comment and reference the inventory rows.

### Per-domain tenancy helpers (US1) — staged rollout

To bound blast radius, T028–T039 land in TWO sub-passes per domain rather than one:

- **Pass A (additive, fail-closed)**: introduce the new `tenancy.<X>ForTenant` helpers
  on the same migration commit as the schema column adds. The new helpers are
  fail-closed: missing tenant context → `tenancy.ErrTenantContextRequired`; tenant
  mismatch → not-found + audit emit. Existing `Store.List<X>` / `Store.Get<X>` helpers
  REMAIN (still tenantless) so that callers compile and the runtime keeps working
  end-to-end during the per-domain backfill window.
- **Pass B (compile-time gate, in the same domain task ID)**: only after the matching
  per-domain backfill (Phase 4 US2) for that domain has landed AND all callers in the
  daemon have been migrated to the new helpers (verified by `go build ./...`),
  DELETE the tenantless helpers. The deletion is a separate commit at the end of the
  domain task; it MUST NOT be merged before the corresponding US2 backfill is committed.

This means each T028–T039 (and T038a) task has two commits per domain (helper add +
helper remove) but is still a single task in this list, since the work is one
end-to-end migration per domain.

**Audit emission rule (applies to every Pass A helper)**: every helper that detects a
tenant mismatch (caller's resolved tenant differs from the target row's `tenant_id`,
including by-id lookups whose row exists in another tenant) MUST call
`audit.Emit(ctx, "store:<HelperName>", "<resourceKind>")` before returning the
not-found / forbidden result. Helpers that reject due to missing tenant context return
`tenancy.ErrTenantContextRequired` and do NOT emit (no acting tenant exists).

- [X] T028 [US1] Add `daemon/internal/store/tenancy/runtime.go` covering the runtime family: `runs` (ListRunsForTenant, GetRunForTenant, UpsertRunForTenant, DeleteRunForTenant), `sessions`, `steps`, `tool_calls`, `llm_dispatches`, `checkpoints`. Delete `Store.ListRuns`, `Store.GetRun`, and the analogous tenantless helpers for the other tables in this group; update internal callers under `daemon/internal/runtime/` and `daemon/internal/llm/` to pass `ctx`. (depends on T018, T007) [Pass A complete: tenancy.Runtime accessor with fail-closed reads, write-then-bind upserts, audit on cross-tenant lookup; existing tenantless helpers retained per the staged-rollout contract. Pass B (caller switch + tenantless deletion) gated on T067 backfill landing.]
- [X] T029 [US1] Add `daemon/internal/store/tenancy/approvals.go` covering `approvals` and `decisions`; delete tenantless variants; update internal callers in the policy package. (depends on T026, T007)
- [X] T030 [US1] Add `daemon/internal/store/tenancy/schedules.go` covering `schedules`, `schedule_targets`, and `schedule_dispatch_attempts`; delete tenantless variants; update callers in `daemon/internal/runtime/` and the scheduler package. (depends on T019, T007)
- [X] T031 [US1] Add `daemon/internal/store/tenancy/workflows.go` covering `workflows`, `workflow_steps`, `workflow_dependencies`, `workflow_handoffs`. (depends on T020, T007)
- [X] T032 [US1] Add `daemon/internal/store/tenancy/integrations.go` for `integrations`; restrict `storeAccess` per inventory; the helper exposes ONLY ownership wiring and query isolation — it MUST NOT add, remove, or alter any function that participates in credential storage, OAuth state, secret-reference resolution, or readiness probing. Those surfaces remain owned by Roadmap 37 and are guarded by T089c's signature snapshot. (depends on T021, T007)
- [X] T033 [US1] Add `daemon/internal/store/tenancy/delivery.go` covering `delivery_targets`, `delivery_preferences`, `delivery_outcomes`, `delivery_attempts`, `delivery_summary_windows`. (depends on T021, T007)
- [X] T034 [US1] Add `daemon/internal/store/tenancy/calendar.go` for `calendar_accounts`, `calendar_operations`, `calendar_artifacts`. (depends on T022, T007)
- [X] T035 [US1] Add `daemon/internal/store/tenancy/mail.go` for `mail_accounts`, `mail_operations`, `mail_artifacts`. (depends on T023, T007)
- [X] T036 [US1] Add `daemon/internal/store/tenancy/reminders.go` covering `reminders`, `reminder_occurrences`, `reminder_actions`. (depends on T024, T007)
- [X] T037 [US1] Add `daemon/internal/store/tenancy/computer_use.go` for `computer_use_sessions`, `computer_use_actions`, `computer_use_artifacts`. (depends on T025, T007)
- [X] T038 [US1] Add `daemon/internal/store/tenancy/evaluation.go` covering `evaluation_replay_candidates`, `evaluation_replay_attempts`, `evaluation_comparisons`, `evaluation_regression_fixtures`. (depends on T026, T007)
- [X] T038a [US1] Add `daemon/internal/store/tenancy/harness.go` covering `connector_messages`, `sandbox_executions`, `consumer_policy_records`, `provider_preferences`, `mcp_tool_exposure_rules`, and `secret_scope_bindings`. **Roadmap 37 boundary (binding constraint, enforced by T089c)**: the helpers added here do ONLY (a) tenant ownership wiring (set/read `tenant_id`), (b) tenant-scoped list/get/delete query isolation, and (c) the per-tenant indexes named in the inventory. They MUST NOT add, modify, or replace ANY function or behavior that participates in: secret value storage or retrieval, OAuth/provider auth lifecycle, secret reference resolution at execution time, connector/MCP/sandbox policy administration, or credential redaction. Existing functions in `daemon/internal/secrets`, `daemon/internal/providers`, `daemon/internal/managedproviders`, `daemon/internal/mcp`, and `daemon/internal/connectors` are read-only consumers from this task's perspective; their public signatures are snapshotted by T089c. (depends on T026, T007)
- [X] T039 [US1] Add `daemon/internal/store/tenancy/events.go` for tenant-owned event categories: `AppendEventForTenant`, `ListEventsForTenant`; delete the tenantless `Store.AppendEvent` paths for tenant-owned categories (keep one explicit `appendGlobalEvent` for global categories per inventory: `mcp-*`, `provider-*`, `system-*`, `daemon.migration.*`). (depends on T016, T007)

### Per-domain API handler updates (US1)

Each task wires the resolved tenant context (already populated by `protected()` in
`daemon/internal/api/server.go`) into the new tenancy helpers. On any cross-tenant intent
(get-by-id of another tenant's resource, mutating another tenant's resource via path
parameter), the handler emits the audit event via `daemon/internal/audit.Emit(ctx,
"api:<METHOD route>", "<resourceKind>")` and returns 404 (to avoid leaking existence)
per existing API conventions. Note: when the underlying tenancy helper already emitted
the audit event (per the rule above), the handler MUST NOT emit a duplicate — once per
denial, attributed to the first surface that detected it.

- [X] T040 [US1] Update runtime-family route handlers in `daemon/internal/api/server.go` (runs, sessions, steps, tool_calls, llm_dispatches, checkpoints) to call the `tenancy.*ForTenant` helpers from T028; emit `audit.Emit(ctx, "api:<METHOD route>", "<resourceKind>")` on cross-tenant denial; return 404 not 403 to avoid existence leak. (depends on T028, T012)
- [X] T041 [US1] Update approvals + decisions route handlers analogously to use T029. (depends on T029, T012)
- [X] T042 [US1] Update schedule, schedule-target, and schedule-dispatch-attempt route handlers analogously. (depends on T030, T012)
- [X] T043 [US1] Update workflow + workflow-step + workflow-handoff route handlers analogously. (depends on T031, T012)
- [X] T044 [US1] Update integration route handlers analogously; do NOT touch credential routes (Roadmap 37 boundary). The handler diff is limited to (a) calling the new tenancy helpers, (b) emitting the audit event on cross-tenant denial, (c) returning 404 on cross-tenant id paths. Existing integration probe / readiness routes that carry credential semantics are out of scope for any behavior change in this task; their handler bodies must be byte-identical except for the tenancy-helper substitution (verified by T089c). (depends on T032, T012)
- [X] T045 [US1] Update delivery_target / delivery_preference / delivery_outcome / delivery_attempt / delivery_summary_window route handlers analogously. (depends on T033, T012)
- [X] T046 [US1] Update calendar route handlers analogously. (depends on T034, T012)
- [X] T047 [US1] Update mail route handlers analogously. (depends on T035, T012)
- [X] T048 [US1] Update reminder, reminder_occurrence, reminder_action route handlers analogously. (depends on T036, T012)
- [X] T049 [US1] Update computer-use route handlers analogously. (depends on T037, T012)
- [X] T050 [US1] Update evaluation route handlers (replay candidates, replay attempts, comparisons, regression fixtures) analogously. (depends on T038, T012)
- [X] T050a [US1] Update harness-domain route handlers (sandbox, connector ingress/reply, consumer policy, provider preferences, mcp tool exposure) to use the T038a helpers; do NOT touch credential semantics (Roadmap 37 boundary). (depends on T038a, T012)
- [X] T051 [US1] Update SSE replay handler in `daemon/internal/api/server.go` (`/v1/events` and `/v1/events/stream`): construct event-bus filter with `TenantID` from resolved context; reject any client filter that names a different tenant; emit audit event on rejection. (depends on T014, T039, T012)

### Additive tenantId on schemas (US1, parallelizable)

`schemas/api/` and `schemas/events/` are FLAT directories (no per-domain subdirectory);
file names are kebab-case (e.g. `run-resource.schema.json`,
`run-created.event.schema.json`). Each task adds an additive `tenantId` field (string,
optional) to every file matching the named glob pattern under those directories, then
runs `make daemon-contract-test`. No removals; no field renames.

- [X] T052 [P] [US1] Add additive `tenantId` to runtime-domain schemas: `schemas/api/run-*.schema.json`, `schemas/api/create-run.request.schema.json`, `schemas/api/run-provider-check.request.schema.json`, `schemas/api/step-*.schema.json`, `schemas/api/tool-call-*.schema.json`, `schemas/api/llm-dispatch-*.schema.json`; events `schemas/events/run-*.event.schema.json`, `schemas/events/step-*.event.schema.json`, `schemas/events/tool-call-*.event.schema.json`, `schemas/events/llm-dispatch-*.event.schema.json`, `schemas/events/session-*.event.schema.json`, `schemas/events/chat-query-stream-*.event.schema.json`, plus the shared envelope `schemas/events/runtime-event.schema.json` (additive optional `tenantId` at envelope level).
- [X] T053 [P] [US1] Add additive `tenantId` to schedule-domain schemas: `schemas/api/schedule-*.schema.json`; events `schemas/events/schedule-*.event.schema.json`.
- [X] T054 [P] [US1] Add additive `tenantId` to workflow-domain schemas: `schemas/api/workflow-*.schema.json`; events `schemas/events/workflow-*.event.schema.json`.
- [X] T055 [P] [US1] Add additive `tenantId` to integration-domain schemas: `schemas/api/integration-*.schema.json`; events `schemas/events/integration-*.event.schema.json`.
- [X] T056 [P] [US1] Add additive `tenantId` to delivery-domain schemas: `schemas/api/delivery-*.schema.json`; events `schemas/events/delivery-*.event.schema.json`.
- [X] T057 [P] [US1] Add additive `tenantId` to calendar-domain schemas: `schemas/api/calendar-*.schema.json`, `schemas/api/cancel-calendar-event.request.schema.json`, `schemas/api/create-calendar-event.request.schema.json`, `schemas/api/create-calendar-availability-query.request.schema.json`; events `schemas/events/calendar-*.event.schema.json`.
- [X] T058 [P] [US1] Add additive `tenantId` to mail-domain schemas (`schemas/api/mail-*.schema.json` if present plus any send/reply request shapes; the daemon currently exposes mail through events more than direct API resources); events `schemas/events/mail-*.event.schema.json`.
- [X] T059 [P] [US1] Add additive `tenantId` to reminder-domain schemas: `schemas/api/reminder-*.schema.json`, `schemas/api/acknowledge-reminder.request.schema.json`, `schemas/api/cancel-reminder.request.schema.json`, `schemas/api/complete-reminder.request.schema.json`; events `schemas/events/reminder-*.event.schema.json`.
- [X] T060 [P] [US1] Add additive `tenantId` to computer-use-domain schemas: `schemas/api/computer-use-*.schema.json`, `schemas/api/create-computer-use-action.request.schema.json`, `schemas/api/create-computer-use-session.request.schema.json`; events `schemas/events/computer-use-*.event.schema.json`.
- [X] T061 [P] [US1] Add additive `tenantId` to evaluation/harness schemas where exposed: events `schemas/events/evaluation-*.event.schema.json`; api shapes for evaluation are largely internal but if any resource files exist (`schemas/api/evaluation-*.schema.json`), include them.
- [X] T062 [P] [US1] Add additive `tenantId` to harness/policy schemas: `schemas/api/approval-*.schema.json`, `schemas/api/decision-resource.schema.json`, `schemas/api/sandbox-*.schema.json`, `schemas/api/connector-*.schema.json`, `schemas/api/mcp-tool-authorization.*.schema.json`, `schemas/api/mcp-tool-exposure-update.request.schema.json`; events `schemas/events/policy-*.event.schema.json`, `schemas/events/sandbox-*.event.schema.json`, `schemas/events/connector-*.event.schema.json`, `schemas/events/mcp-tool-exposure-updated.event.schema.json`.

### Cross-tenant isolation regressions (US1)

Table-driven tests so adding a new domain to the inventory automatically requires an
isolation row.

- [X] T063 [US1] Add `daemon/internal/store/isolation_test.go`: table-driven test that, for each `tenant_owned` row in the parsed inventory, provisions same-shaped fixtures in tenants A and B and asserts every `tenancy.*ForTenant` helper returns only the calling tenant's row, never B's, and rejects writes that mix tenant context. (depends on T028–T039, T009)
- [X] T064 [US1] Add `daemon/internal/api/isolation_test.go`: table-driven test exercising every in-scope API route per inventory `affectedAPIs`; asserts cross-tenant get returns 404, cross-tenant list omits other tenant's rows, and audit event fires on every denial. (depends on T040–T051, T012)
- [X] T065 [US1] Add `daemon/internal/events/isolation_test.go` (live event isolation only): subscribes as A and as B, publishes events as both, asserts A receives only A's events and B receives only B's events; SSE replay over the in-test event window scoped to A returns no B events; tenantless publish on a tenant-owned category is rejected. Backfilled-events isolation is covered by T081a in US2 (after T077). (depends on T014, T016)
- [X] T065a [US1] Add `daemon/internal/store/uniqueness_test.go`: load the parsed inventory (via `daemon/internal/inventory`) and iterate every row whose `indexesAndUniqueness` column declares a `UNIQUE (tenant_id, ...)` constraint introduced by Roadmap 35 — DO NOT hard-code a natural-key list, since display names that live in `document_json` are explicitly NOT promoted to SQL constraints and any hard-coded list will drift. For each declared UNIQUE entry, provision tenants A and B via the matching `tenancy` helper, create rows whose tenant-aware natural key collides across tenants, and assert: (a) both creations succeed, (b) the rows are isolated, (c) creating a duplicate within the same tenant fails with a SQLite UNIQUE violation, and (d) the constraint identifier present in `sqlite_master` matches the one declared in the inventory. memberships.(tenant_id, principal_id) and token_tenant_grants.(tenant_id, token_id) are exercised in a sibling test file `daemon/internal/identity/uniqueness_test.go` because they go through R34 identity helpers, not the tenancy package; add or extend that file in the same task with the equivalent two-tenant assertion. Verifies SC-006. (depends on T009 inventory loader, T018–T027, T028–T038a, T077a)

**Checkpoint**: User Story 1 fully functional — multi-tenant isolation invariant holds
across every in-scope domain. Removing any tenant filter causes a deterministic test
failure.

---

## Phase 4: User Story 2 — Existing local user's data lands in their personal tenant (Priority: P1)

**Goal**: Operators upgrading from a pre-tenant database see every existing in-scope
record under their default personal tenant after migration. Migration is restart-safe,
idempotent, and resumes from durable per-step progress on crash. Backup-restore is the
sole supported rollback.

**Independent Test**: Start from a checked-in pre-tenant (schema v21) SQLite snapshot;
run migrations; assert post-migration record counts equal pre-migration counts and that
every row is owned by the default personal tenant. Re-run migrations and assert zero row
changes. Interrupt a backfill mid-step and assert the resume completes the step exactly
once.

### Pre-tenant fixture and per-domain backfill (US2)

**Parent-child backfill ordering rule (applies to T069, T070, T072, T073, T075, T077)**:
when a child table derives `tenant_id` from a parent row's `tenant_id`, the parent
backfill step MUST complete before the child backfill begins for the same key range.
Each child backfill MUST resolve `tenant_id` from the parent via the existing FK
column (e.g. `schedule_dispatch_attempts.schedule_id → schedules.tenant_id`). Falling back to
the default personal tenant for a child row whose parent could not be resolved is a
hard error: emit a `daemon.migration.orphan_detected` event carrying the table name
and parent FK value (no other row data) and mark the step `failed` so the daemon
refuses to start until an operator resolves the parent. The default personal tenant
is the destination for top-level rows only.

- [X] T066 [US2] Capture `daemon/internal/store/testdata/pre_tenant_v21.sqlite`: a SQLite database initialized at schema version 21 (the pre-roadmap-35 head) seeded with at least 5 rows in every in-scope `tenant_owned` table, including parent + child rows for each parent-child pair (schedules+attempts, workflows+runs, calendar/mail/computer-use parents+children); document the seed script in `daemon/internal/store/testdata/README.md` so the fixture is reproducible.
- [X] T067 [US2] Implement six runtime-domain backfill steps in order: (1) `tenant_migration:backfill:sessions` → default personal tenant; (2) `tenant_migration:backfill:runs` → default personal tenant; (3) `tenant_migration:backfill:steps` → parent `runs.tenant_id` via `steps.run_id` FK; (4) `tenant_migration:backfill:tool_calls` → parent `steps.tenant_id` via `tool_calls.step_id` FK; (5) `tenant_migration:backfill:llm_dispatches` → default personal tenant (the table has NO run_id/step_id FK; pre-roadmap-35 rows have no recoverable per-tenant attribution and go to the default personal tenant — new rows after migration receive their tenant from the authenticated context at dispatch time via the tenancy helper); (6) `tenant_migration:backfill:checkpoints` → parent `runs.tenant_id` via `checkpoints.run_id` FK. Each step iterates primary keys in chunks of 500 rows per transaction, persists `last_processed_key` via `migration_progress.RecordChunk`. Apply the parent-child orphan rule for steps, tool_calls, checkpoints; llm_dispatches has no parent so the orphan rule does not apply. (depends on T005, T018, T066)
- [X] T068 [US2] Implement two approvals-domain backfill steps in order: (1) approvals → default personal tenant; (2) decisions → default personal tenant (decisions reference an approval but the foreign key is optional and the row itself stands alone — backfill as top-level). (depends on T005, T026)
- [X] T069 [US2] Implement three schedules-domain backfill steps in order: (1) `tenant_migration:backfill:schedules` → default personal tenant; (2) `tenant_migration:backfill:schedule_targets` → parent `schedules.tenant_id`; (3) `tenant_migration:backfill:schedule_dispatch_attempts` → parent `schedules.tenant_id`. Apply the orphan rule. (depends on T005, T019)
- [X] T070 [US2] Implement four workflows-domain backfill steps in order: (1) workflows → default personal tenant; (2) workflow_steps → parent `workflows.tenant_id`; (3) workflow_dependencies → parent `workflows.tenant_id`; (4) workflow_handoffs → parent `workflows.tenant_id`. Apply the orphan rule. (depends on T005, T020)
- [X] T071 [US2] Implement six integrations + delivery backfill steps: (1) integrations → default personal tenant; (2) delivery_targets → default personal tenant; (3) delivery_preferences → default personal tenant (or parent target where joined); (4) delivery_outcomes → default personal tenant; (5) delivery_attempts → parent `delivery_outcomes.tenant_id`; (6) delivery_summary_windows → default personal tenant. Apply the orphan rule for delivery_attempts. (depends on T005, T021)
- [X] T072 [US2] Implement three calendar-domain backfill steps in order: (1) `calendar_accounts` → default personal tenant; (2) `calendar_operations` → parent `calendar_accounts.tenant_id`; (3) `calendar_artifacts` → parent `calendar_operations.tenant_id`. Apply the orphan rule. (depends on T005, T022)
- [X] T073 [US2] Implement three mail-domain backfill steps in order (accounts → operations → artifacts), same shape as T072. Apply the orphan rule. (depends on T005, T023)
- [X] T074 [US2] Implement three reminders-domain backfill steps in order: (1) reminders → default personal tenant; (2) reminder_occurrences → parent `reminders.tenant_id`; (3) reminder_actions → parent `reminders.tenant_id`. Apply the orphan rule. (depends on T005, T024)
- [X] T075 [US2] Implement three computer-use backfill steps in order: (1) `computer_use_sessions` → default personal tenant; (2) `computer_use_actions` → parent session's tenant_id; (3) `computer_use_artifacts` → parent session's tenant_id. Apply the orphan rule. (depends on T005, T025)
- [X] T076 [US2] Implement four evaluation-domain backfill steps in order: (1) `evaluation_replay_candidates` → default personal tenant; (2) `evaluation_replay_attempts` → parent `evaluation_replay_candidates.tenant_id`; (3) `evaluation_comparisons` → default personal tenant; (4) `evaluation_regression_fixtures` → default personal tenant. Apply the orphan rule for replay attempts. (depends on T005, T026)
- [X] T076a [US2] Implement five top-level harness-domain backfill steps (any order): consumer_policy_records → default personal tenant; provider_preferences → default personal tenant; mcp_tool_exposure_rules → default personal tenant; secret_scope_bindings → default personal tenant; sandbox_executions → resolve via `run_id → runs.tenant_id` when `run_id` is non-NULL (apply parent-child orphan rule), otherwise default personal tenant. connector_messages is NOT in this batch — it is parent-dependent and lives in T076b. (depends on T005, T026)
- [X] T076b [US2] Implement parent-dependent backfill step `tenant_migration:backfill:connector_messages`: resolve `tenant_id` in priority order per inventory — (1) `session_id → sessions.tenant_id` when non-NULL; (2) else `run_id → runs.tenant_id` when non-NULL; (3) else default personal tenant. Runs ONLY after both `tenant_migration:backfill:sessions` (in T067) and `tenant_migration:backfill:runs` (in T067) reach `completed`. Apply the parent-child orphan rule for cases where session_id or run_id is non-NULL but the parent row no longer exists. (depends on T005, T026, T067)
- [X] T077 [US2] Implement backfill step `tenant_migration:backfill:events`. **Schema-version precondition**: the events table reaches its final pre-R35 column shape only after schema v12 (which adds `workflow_id`, `workflow_step_id`, `schedule_id`, `schedule_attempt_id`); the daemon at this point is past v12 (current head is v21) and past R35's v23 (which adds `events.tenant_id`). The backfill therefore reads from the union of all event FK columns present at v21 + v23. Resolve each event's `tenant_id` in this priority order from columns that ARE present: `run_id → runs.tenant_id`, `session_id → sessions.tenant_id`, `step_id → steps.tenant_id`, `workflow_id → workflows.tenant_id`, `workflow_step_id → workflow_steps.tenant_id`, `schedule_id → schedules.tenant_id`, `schedule_attempt_id → schedule_dispatch_attempts.tenant_id`. **Connector events**: `events.connector_id` is the connector identifier (NOT a foreign key into `connector_messages` — `connector_messages.connector_id` is non-unique, and `connector_messages` PK is `delivery_id`); resolve connector events instead via `events.resource_kind = "connector_message"` AND `events.resource_id = <delivery_id>` joined to `connector_messages.delivery_id → connector_messages.tenant_id`. Connector events that do not carry such a `resource_kind/resource_id` pair (legacy emissions where the resource pointer is missing) are reclassified into the global `connector-*` category and their `tenant_id` is left NULL. `capability_id` resolves to no tenant (capabilities are global) — events whose only parent is `capability_id` are reclassified into a global event category at backfill time and their `tenant_id` is left NULL. Runs only after all parent backfill steps (T067, T068, T069, T070, T071, T072, T073, T074, T075, T076, T076a, T076b) complete. Apply the orphan rule for events whose parent FK does resolve to a row that is NOT tenant_owned: emit `daemon.migration.orphan_detected` with `table=events, parent_kind=<kind>, parent_id=<id>` and fail the step. (depends on T005, T016, T067, T068, T069, T070, T071, T072, T073, T074, T075, T076, T076a, T076b)
- [X] T077a [US2] Step (c) NOT NULL enforcement and UNIQUE finalization. After all per-domain backfills (T067–T077) complete, register schema migrations v34..v42 (one per US1 schema migration v24..v32) that, for every tenant_owned table EXCEPT `events`, recreate the table via SQLite shadow-table swap with `tenant_id TEXT NOT NULL`, the per-tenant UNIQUE constraints declared in T018–T026, and a CHECK that `tenant_id` matches the existing tenant id format `^ten_[A-Za-z0-9]+$`. Each step is registered in `tenant_migration_progress` with name `tenant_migration:enforce_not_null:<table>`. Each step asserts at start that the matching backfill step's status is `completed`; if not, the step refuses to run and surfaces a clear operator error. After all step (c) migrations succeed, the daemon resumes serving tenant-owned API and SSE traffic. **`events` is a mixed table** — it carries both tenant_owned and `global`-classified events (per the inventory's "Event Sources" section: `mcp-*`, `provider-*`, `system-*`, `daemon.migration.*`, plus reclassified legacy connector/capability events) — so step (c) for `events` is handled by T077b (not by this task), keeping the column NULLABLE and using a CHECK + partial index pair instead of a blanket NOT NULL. (depends on T067, T068, T069, T070, T071, T072, T073, T074, T075, T076, T076a, T077)
- [X] T077b [US2] Step (c) for the mixed `events` table. Register schema migration v43 that adds: (a) a CHECK constraint `tenant_id IS NOT NULL OR category IN ('mcp', 'provider', 'system', 'daemon.migration', 'connector_global', 'capability_global')` so that any tenant-owned event category MUST carry a `tenant_id`, and global event categories MAY carry NULL; (b) a partial UNIQUE / list index `idx_events_tenant_owned (tenant_id, occurred_at DESC, event_id DESC) WHERE tenant_id IS NOT NULL` so the tenant-aware list path stays selective; (c) a partial index `idx_events_global (category, occurred_at DESC, event_id DESC) WHERE tenant_id IS NULL` for the global-event read path. The two reclassification labels (`connector_global`, `capability_global`) are introduced by T077 when an orphan parent FK forces a global remap; both labels MUST appear in the inventory's `Event Sources` section as `global`. The tenancy event filter in T039 enforces the symmetric query-policy rule: tenant-owned subscribers MUST add `WHERE tenant_id = ?` and MUST NOT receive NULL-tenant rows; global subscribers MAY ask for `WHERE tenant_id IS NULL AND category IN (...)`. Registered as `tenant_migration:enforce_check:events`. (depends on T077, T077a)

### Migration test suite (US2)

- [X] T078 [US2] Add `daemon/internal/store/migration_fixture_test.go`: copies `testdata/pre_tenant_v21.sqlite` to a tempdir, runs all schema migrations + backfills (T067–T077, T076a, T076b) and the step (c) enforcement (T077a + T077b), and asserts: (a) for every in-scope tenant_owned table EXCEPT `events`, every row's `tenant_id` is the default personal tenant id (the fixture is single-operator, single-tenant); (b) for parent-derived child rows (steps, tool_calls, schedule_targets, schedule_dispatch_attempts, workflow_steps, workflow_dependencies, workflow_handoffs, calendar_operations, calendar_artifacts, mail_operations, mail_artifacts, reminder_occurrences, reminder_actions, computer_use_sessions, computer_use_actions, computer_use_artifacts, evaluation_replay_attempts, evaluation_comparisons, delivery_attempts, connector_messages where session_id/run_id is non-NULL, sandbox_executions where run_id is non-NULL), each row's `tenant_id` equals the parent row's `tenant_id`; (c) for the mixed `events` table, every row whose `category` is tenant-owned (per the inventory's Event Sources section: every category except `mcp`, `provider`, `system`, `daemon.migration`, `connector_global`, `capability_global`) has a non-NULL `tenant_id` equal to the resolved parent's tenant, AND every row whose `category` is one of the global labels has `tenant_id IS NULL`; (d) pre/post row counts are equal per table. (depends on T066–T077, T076a, T076b, T077a, T077b)
- [X] T079 [US2] Add re-run idempotence test in the same file: run migrations twice on the fixture, assert second run produces zero row changes (compare row hashes). (depends on T078)
- [X] T080 [US2] Add resume-safety test: run the `runs` backfill until `last_processed_key` is past row 250 of 500, force-abort the transaction, restart migration, assert the resume completes the remaining rows exactly once and `tenant_migration_progress.runs.status == completed`. (depends on T067)
- [X] T081 [US2] Add unsafe-state startup refusal test in `daemon/internal/app/app_test.go` (or equivalent): inject a `failed` row in `tenant_migration_progress`, start the daemon, assert it refuses to serve traffic, emits `daemon.migration.failed` event, and exits non-zero. (depends on T017)
- [X] T081a [US2] Add backfilled-event isolation test in `daemon/internal/events/backfilled_isolation_test.go`: copy the pre-tenant fixture, run all migrations including T077, then subscribe as the default personal tenant and assert pre-roadmap-35 events appear in the stream with the correct `tenant_id` derived from each event's parent; subscribe as a different tenant and assert none of the pre-existing events appear. (depends on T077)
- [X] T081b [US2] Add automated backup-restore test in `daemon/internal/store/rollback_test.go` verifying SC-007: copy the pre-tenant fixture to a tempdir, compute SHA-256 of the file, run all migrations to completion, close the SQLite connection, copy the original fixture back over the daemon DB file, reopen the SQLite connection (do NOT reuse the migrated handle — schema metadata is cached per-connection), recompute SHA-256, assert it matches the pre-migration hash AND that introspecting `sqlite_master` on the reopened connection shows no `tenant_migration_progress` table and no `tenant_id` columns on tenant-owned tables. (depends on T066, T067–T077)

**Checkpoint**: User Story 2 fully functional — pre-tenant operators upgrade losslessly,
migration resumes from crashes, re-running is a no-op, and unsafe state surfaces a
clear operator error.

---

## Phase 5: User Story 3 — Engineer can prove cross-tenant isolation in every migrated domain (Priority: P2)

**Goal**: An engineer can run a single regression suite that proves no domain leaks
across tenants and that the schema inventory matches reality. Newly added tables or
event sources cannot bypass classification.

**Independent Test**: Run the inventory completeness test on a clean checkout — passes.
Add a new persisted table without an inventory entry — test fails. Run the audit event
suite — every denied cross-tenant access emits a typed audit event whose payload omits
target tenant id and row data. Run the latency suite on the seeded fixture —
post-migration p95 ≤ 1.2× pre-migration p95 and the tenant-aware index is selected.

### Inventory completeness (US3)

- [X] T082 [US3] Add `daemon/internal/inventory/completeness_table_test.go`: introspect a freshly-initialized SQLite test DB at the head schema version via `sqlite_master`; assert every persisted table appears as a row in the parsed inventory and every inventory `name` for tables exists in the schema; FAIL on drift.
- [X] T083 [US3] Add `daemon/internal/inventory/completeness_events_test.go`: collect the registered event categories at daemon startup (or from `schemas/events/index.json`); assert every event source is present in the inventory and every inventory event entry exists at runtime.
- [X] T084 [US3] Add `daemon/internal/inventory/completeness_classification_test.go`: assert no inventory row has classification `tenant_owned` with `tenantIdSource == not_applicable`; assert every `tenant_owned` row has at least one entry in `indexesAndUniqueness`, `isolationTests`, and `storeAccess`; assert every row has `rollback == backup_restore`.

### Audit event integration (US3)

- [X] T085 [US3] Add `daemon/internal/audit/integration_test.go`: spin up the daemon with two tenants, perform a series of cross-tenant API calls per inventory, assert each one produces exactly one `audit.cross_tenant_access_denied` event (envelope: `category=audit`, `name=audit.cross_tenant_access_denied`, `resource.kind=tenant`, `resource.id=<actingTenantId>`) whose payload validates against `schemas/events/audit-cross-tenant-access-denied.event.schema.json` and contains no target tenant id or row data; assert the structured log line is present with stable code `audit.cross_tenant_access_denied`.

### Performance assertions (US3)

- [X] T086 [US3] Add `daemon/internal/store/queryplan_test.go`: for every in-scope tenant-scoped list path per inventory, run `EXPLAIN QUERY PLAN`, assert the named tenant-aware index is selected (matches the index name from the inventory `indexesAndUniqueness` column); FAIL on table scan.
- [X] T087 [US3] Add `daemon/internal/store/perf_runs_test.go`: seed N=10000 runs across 2 tenants in a v21 fixture, measure p95 list latency pre-migration, run migrations, re-measure post-migration, assert post ≤ 1.2× pre on the same fixture; mark the test as `testing.Short()`-skippable for fast local runs but always-on in CI.
- [X] T088 [US3] Add the same shape of test for `events` in `daemon/internal/store/perf_events_test.go`.
- [X] T089 [US3] Add the same shape of test for `mail_artifacts` in `daemon/internal/store/perf_mail_artifacts_test.go`.
- [X] T089a [US3] Add legacy-client backward-compat contract test in `daemon/internal/api/legacy_client_compat_test.go` verifying FR-014: for each api/event payload that gained an additive `tenantId`, decode the payload through a strict-unknown-field deserializer that strips `tenantId` and confirm the remaining fields parse exactly as they did pre-roadmap-35; this task also creates the pre-roadmap-35 reference payload fixtures under `daemon/internal/api/testdata/legacy_payloads/`. (depends on T052–T062)
- [X] T089b [US3] Add no-admin-cross-tenant-route assertion in `daemon/internal/api/no_admin_route_test.go` verifying FR-017: introspect the registered route table after `NewServer()` and assert that no route is registered without going through the `protected()` middleware that resolves a tenant context, and that no route handler skips tenant scoping (signaled by the forbidden comment marker `// admin: cross-tenant` — the test greps the `daemon/internal/api/` tree and FAILS if the marker appears). The test file MUST include a header comment explaining the convention so future contributors discover it via grep; T090 cross-references the convention in the operator runbook. When Roadmap 36 introduces admin paths, this test moves with the admin design. (depends on T040–T051)
- [X] T089c [US3] Add Roadmap 37 boundary assertion test in `daemon/internal/store/tenancy/r37_boundary_test.go` verifying FR-015. The test enforces TWO disjoint groups distinguished by inventory classification:

  **Group A (this roadmap adds tenant ownership)** — `secret_scope_bindings`, `provider_preferences`, `mcp_tool_exposure_rules`. For each table the test asserts: (i) the only column added is `tenant_id` plus the indexes/constraints declared in the inventory's `indexesAndUniqueness` cell; (ii) the new `tenancy.<X>` helpers expose ONLY ownership wiring (set/read tenant_id) and tenant-scoped query isolation — no credential/OAuth/secret-reference/redaction/authorization function is added or changed; (iii) credential-resolution outputs (secret reference lookup, provider preference read, MCP tool authorization decision) are byte-identical before and after migration on a fixture run.

  **Group B (this roadmap MUST NOT change DDL or storeAccess)** — `provider_auth_states`, `mcp_servers`, `mcp_server_states`, `mcp_tools`, `connectors`. For each table the test asserts: (i) NO column has been added (`tenant_id` MUST NOT appear); (ii) NO new index or constraint declared in any R35 schema migration touches this table; (iii) NO new tenancy helper exists for this table; (iv) the existing exported functions in the table's owning package are unchanged from the goldens.

  Both groups share an exported-signature snapshot under `daemon/internal/store/tenancy/testdata/r37_boundary_signatures.golden` covering `daemon/internal/secrets`, `daemon/internal/providers`, `daemon/internal/managedproviders`, `daemon/internal/mcp`, and `daemon/internal/connectors` (regenerate via `go test -update`; updates MUST be co-reviewed with the Roadmap 37 owner). Failures here block the roadmap from closing. (depends on T026, T038a, T076a)

**Checkpoint**: User Story 3 fully functional — drift in schema, events, or
classification fails CI; audit event surface verified end-to-end; latency within budget.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Operator documentation, contract test pass, and roadmap docs update so
Roadmap 35 can be marked complete per the constitution's roadmap-closure rule.

- [X] T090 [P] Add `docs/runtime/tenant-migration-rollback.md` documenting: (a) the hard prerequisite to take a database backup before upgrade, (b) the backup-restore rollback procedure, (c) what to do when the daemon refuses to start with `daemon.migration.failed` or `daemon.migration.orphan_detected`, (d) explicit statement that no down-migration is shipped, (e) the convention that the comment marker `// admin: cross-tenant` is FORBIDDEN inside `daemon/internal/api/` and `daemon/internal/store/` for the duration of Roadmap 35 (enforced by T089b), to be revisited when Roadmap 36 introduces admin surfaces.
- [X] T091 [P] Update `docs/runtime/daemon-roadmaps.md` to mark Roadmap 35 complete and link to `specs/020-tenant-scoped-data-migration/`.
- [X] T092 [P] Update `docs/product/hosted-productization-roadmap-split.md` to reflect Roadmap 35 closure and the explicit Roadmap 37 ownership boundary on credential and connector semantics.
- [X] T093 Run `make daemon-contract-test` and resolve any contract drift introduced by additive `tenantId` fields and the new `audit.cross_tenant_access_denied` schema (file `schemas/events/audit-cross-tenant-access-denied.event.schema.json`).
- [X] T094 Run the full `make daemon-test` from the worktree root with the test data dir reset, then again with the pre-tenant fixture as initial state; both must pass.
- [X] T095 [P] Run `pnpm test:clients` to confirm the SDK still builds and tests after schema regeneration of additive `tenantId` fields. No client logic changes are expected from this roadmap.
- [X] T096 Walk through `specs/020-tenant-scoped-data-migration/quickstart.md` end-to-end on the test daemon: confirm migration completes, isolation regressions pass, audit event fires, latency budget holds, and backup-restore returns the database to a fully pre-migration state.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies; can start immediately.
- **Foundational (Phase 2)**: depends on Setup; BLOCKS every user story.
- **US1 (Phase 3)**: depends on Phase 2; per-domain tasks within US1 follow the
  declared sub-dependencies (schema → tenancy helpers → API → schemas → isolation
  tests).
- **US2 (Phase 4)**: depends on Phase 2 and on the matching per-domain schema
  migrations from US1 (T018–T026); the backfill steps and US1 schema migrations are
  paired but US2 can begin once any single per-domain schema migration lands.
- **US3 (Phase 5)**: depends on Phase 2 and on US1 reaching the point where every
  domain has tenancy helpers and isolation tests in place; US3 tests are correctness
  gates that can only be meaningful once US1 is substantially done.
- **Polish (Phase 6)**: depends on US1, US2, and US3 all complete.

### User Story Dependencies

- **US1 (P1, MVP)**: foundational only.
- **US2 (P1, MVP)**: foundational + per-domain US1 schema migrations (T018–T026 pair
  with T067–T076).
- **US3 (P2)**: foundational + US1 substantially complete.

### Within Each User Story

- Schema migrations (DDL) before tenancy helpers (DML access) before route handlers.
- Tenancy helpers and additive schema files can be authored in parallel for distinct
  domains.
- Isolation tests come after both store and API surfaces are wired.
- Per-tenant uniqueness behavior test (T065a) comes after step (c) NOT NULL + UNIQUE
  enforcement (T077a), since UNIQUE constraints are added in step (c), not step (a).
  T065a's enumeration is loaded dynamically from the parsed inventory — never
  hard-coded — so adding or removing a UNIQUE row in the inventory automatically
  changes the test surface.
- Backfilled-event isolation (T081a) and automated backup-restore (T081b) come after
  the per-domain backfills (T067–T077).
- Parent-child backfills (T069, T070, T072, T073, T075, T077) MUST sequence the parent
  step before the child step; orphan rows fail the migration rather than fall back to
  the default personal tenant.
- Tenancy helpers are staged Pass A → Pass B per domain: Pass A (additive,
  fail-closed) lands with the schema migration in US1; Pass B (delete tenantless
  helpers) lands ONLY after the matching US2 backfill for the same domain has been
  committed AND `go build ./...` passes with the deletions applied. Pass B for one
  domain may proceed before all other domains have completed Pass B.

### Parallel Opportunities

- T002, T003 in Setup are parallel.
- T010, T013, T015 in Foundational are parallel after their direct dependencies.
- T018–T026 (per-domain schema migrations in US1) are parallel — different
  `schemaMigrations` entries.
- T028–T039 (per-domain tenancy helpers, including T038a harness) are parallel after
  their schema migration lands.
- T040–T051 (per-domain API handlers, including T050a harness) are parallel after the
  matching tenancy helpers land.
- T052–T062 (additive tenantId on schemas) are fully parallel.
- T067–T077 (per-domain backfill registration, including T076a harness) are parallel
  across domains after T005 and the matching schema migration; within a domain, the
  parent-child ordering rule is strictly sequential.
- T086, T087, T088, T089 (perf assertions per table) are parallel.
- T090, T091, T092, T095 (Polish docs/test passes) are parallel.

---

## Parallel Example: User Story 1 schema migrations

```bash
# Per-domain schema migration entries can land in parallel commits/PRs:
Task: "T018 [US1] Schema v24 — runtime (sessions, runs, steps, tool_calls, llm_dispatches, checkpoints)"
Task: "T019 [US1] Schema v25 — schedules + schedule_targets + schedule_dispatch_attempts"
Task: "T020 [US1] Schema v26 — workflows + workflow_steps + workflow_dependencies + workflow_handoffs"
Task: "T021 [US1] Schema v27 — integrations + delivery_* (5 tables)"
Task: "T022 [US1] Schema v28 — calendar_accounts + calendar_operations + calendar_artifacts"
Task: "T023 [US1] Schema v29 — mail_accounts + mail_operations + mail_artifacts"
Task: "T024 [US1] Schema v30 — reminders + reminder_occurrences + reminder_actions"
Task: "T025 [US1] Schema v31 — computer_use_sessions + computer_use_actions + computer_use_artifacts"
Task: "T026 [US1] Schema v32 — evaluation_* + harness (approvals, decisions, connector_messages, sandbox_executions, consumer_policy_records, provider_preferences, mcp_tool_exposure_rules, secret_scope_bindings)"
```

```bash
# Additive tenantId on schemas — fully parallel; flat kebab-case file globs:
Task: "T052 [P] [US1] Additive tenantId in run-/step-/tool-call-/llm-dispatch-/session-/chat-query-* schemas"
Task: "T053 [P] [US1] Additive tenantId in schedule-* schemas"
Task: "T054 [P] [US1] Additive tenantId in workflow-* schemas"
Task: "T055 [P] [US1] Additive tenantId in integration-* schemas"
Task: "T056 [P] [US1] Additive tenantId in delivery-* schemas"
Task: "T057 [P] [US1] Additive tenantId in calendar-* schemas"
Task: "T058 [P] [US1] Additive tenantId in mail-* schemas"
Task: "T059 [P] [US1] Additive tenantId in reminder-* schemas"
Task: "T060 [P] [US1] Additive tenantId in computer-use-* schemas"
Task: "T061 [P] [US1] Additive tenantId in evaluation-* schemas"
Task: "T062 [P] [US1] Additive tenantId in approval-/decision-/sandbox-/connector-/policy-/mcp-tool-exposure-* schemas"
```

---

## Implementation Strategy

### MVP First (US1 + US2 — both P1)

Per the constitution's roadmap-closure rule, this roadmap ships as one unit. The natural
internal milestones are:

1. Complete Phase 1 (Setup) and Phase 2 (Foundational).
2. Walk one domain (recommend `runs`) end-to-end through US1 + US2 + the matching US3
   inventory + isolation + perf rows. This proves the recipe.
3. Repeat per-domain in parallel, batching domains by file proximity (calendar +
   mail share patterns; computer-use is independent; evaluation is independent).
4. Land Phase 5 (US3 inventory completeness + perf) once all domains are migrated.
5. Land Phase 6 (Polish) and run `make daemon-test` against both a clean and a
   pre-tenant-fixture data dir.
6. Mark Roadmap 35 complete in `docs/runtime/daemon-roadmaps.md`.

### Incremental Delivery

- After Phase 2 (Foundational) lands, the daemon already enforces tenant context on the
  `events` table and refuses tenantless writes there. That alone is shippable
  observability.
- After the first domain (`runs`) reaches US1 + US2 completion, hosted multi-tenant is
  proven on one domain. Other domains follow the same recipe.
- After all domains land US1, isolation regressions catch any drift.
- After US3 completeness and perf land, the roadmap is closeable.

### Parallel Team Strategy

With multiple engineers:

1. Engineer A owns Phase 2 (Foundational) end-to-end.
2. Once Phase 2 lands:
   - Engineer B (runtime + events): T018, T028, T040, T052, T067, T077, perf tests
     for runs and events.
   - Engineer C (orchestration): T019–T021, T029–T033, T041–T045, T053–T056,
     T068–T071 — schedules, workflows, integrations, delivery, approvals.
   - Engineer D (operator-domain): T022–T026, T034–T038, T046–T050, T057–T061,
     T072–T076 — calendar, mail, reminders, computer-use, evaluation.
   - Engineer E (harness boundary): T026 (harness portion), T038a, T050a, T062, T076a
     — connector_messages, sandbox_executions, consumer_policy_records,
     provider_preferences, mcp_tool_exposure_rules, secret_scope_bindings; coordinate
     with Roadmap 37 owner before merging the secret_scope_bindings touch.
3. Engineer A (or a fourth engineer) owns Phase 5 (US3) and Phase 6 (Polish) in
   parallel with the tail of US1/US2.

Authoritative table list (against `daemon/internal/store/store.go` head schema v21) lives
in `contracts/schema-inventory.md`; in-scope tenant_owned tables that this roadmap migrates
are: sessions, runs, steps, tool_calls, llm_dispatches, checkpoints, events, approvals,
decisions, schedules, schedule_targets, schedule_dispatch_attempts, workflows,
workflow_steps, workflow_dependencies, workflow_handoffs, integrations, delivery_targets,
delivery_preferences, delivery_outcomes, delivery_attempts, delivery_summary_windows,
calendar_accounts, calendar_operations, calendar_artifacts, mail_accounts,
mail_operations, mail_artifacts, reminders, reminder_occurrences, reminder_actions,
computer_use_sessions, computer_use_actions, computer_use_artifacts,
evaluation_replay_candidates, evaluation_replay_attempts, evaluation_comparisons,
evaluation_regression_fixtures, connector_messages, sandbox_executions,
consumer_policy_records, provider_preferences, mcp_tool_exposure_rules, and (R37
boundary, column-only) secret_scope_bindings.

---

## Notes

- [P] tasks operate on different files with no dependencies on incomplete tasks.
- [Story] label maps the task to its user story for traceability and is REQUIRED on
  US1/US2/US3 phase tasks.
- The constitution's roadmap-closure rule means partial domains do not constitute a
  shippable slice; mark Roadmap 35 complete only after every in-scope domain is
  migrated, every isolation regression passes, and the inventory completeness test
  matches reality.
- Avoid: leaving a tenantless helper visible on a tenant-owned table (caller drift),
  letting the inventory and live schema disagree (Phase 5 will fail CI), including
  target tenant id or row data in the audit event payload (schema rejects it),
  introducing a cross-tenant admin path (FR-017 forbids it).
