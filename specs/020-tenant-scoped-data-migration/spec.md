# Feature Specification: Tenant-Scoped Data Migration

**Feature Branch**: `020-claude`
**Created**: 2026-04-25
**Status**: Draft
**Input**: User description: "结合 docs/specs/020-tenant-scoped-data-migration.md 完成 phase 35 的工作"

**Upstream authority**: `docs/specs/020-tenant-scoped-data-migration.md` is the authoritative
design document for this work (Roadmap 35). This specification translates that design into
testable scenarios, requirements, and success criteria. Where the upstream document and this
spec disagree, the upstream document wins and this spec must be updated.

## Clarifications

### Session 2026-04-25

- Q: When the migration is interrupted (crash or kill) partway through, what is the recovery model? → A: Resume-safe per-step migration with progress tracking; the next start picks up where it left off, completes the migration, and never exposes partial-tenant state to APIs in between starts. "Refuse to start" applies only when resume detects an unsafe state.
- Q: What is the supported rollback path? → A: Backup-restore only. No down-migration is shipped. Operator documentation MUST require taking a database backup before upgrade and define backup-restore as the sole supported rollback procedure.
- Q: How are denied cross-tenant access attempts surfaced for operators? → A: Greppable structured log AND a typed audit event on the daemon's existing event surface. The audit event MUST include acting tenant, route or store helper identifier, resource kind, and timestamp; it MUST NOT include the target tenant id or any target row data.
- Q: Does Roadmap 35 deliver any admin cross-tenant projection (read-only or otherwise)? → A: No. All cross-tenant admin projections are deferred to a later roadmap. Roadmap 35 ships strict tenant scoping on every read, write, event, replay, and operator projection surface. There is no admin escape hatch in this delivery.
- Q: What is the performance budget for tenant-scoped list paths after migration? → A: Relative no-regression budget against a pre-migration baseline on a representative fixture (post-migration p95 list latency ≤ 1.2× pre-migration p95 on the same fixture) PLUS a query-plan assertion that the tenant-aware index is selected for every in-scope list path.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Hosted user sees only their tenant's data (Priority: P1)

A hosted user is a member of one or more tenants and selects an active tenant for their
session. Every list, get, create, update, delete, event stream, replay surface, and operator
projection that returns daemon-owned runtime, product, or harness records returns only
records owned by that active tenant. Records owned by another tenant are not returned, not
mutated, and not observable through any normal API, event, or replay path.

**Why this priority**: Without this guarantee, multi-tenant hosting is unsafe to ship at
all. A single cross-tenant leak is a production-blocking correctness bug, not a UX issue.

**Independent Test**: Provision two tenants with same-shaped fixtures (e.g. a run, a
schedule, an integration, an evaluation, a delivery) in each tenant. Authenticate as a
member of tenant A. Confirm that every in-scope list, get, event-replay, and operator
projection returns only tenant A's records and that direct-by-id access to a tenant B record
fails. Repeat from tenant B's perspective.

**Acceptance Scenarios**:

1. **Given** tenants A and B each have a run with the same identifier shape, **When** a
   member of tenant A lists or fetches runs, **Then** only tenant A's run is returned and
   tenant B's run is neither listed nor reachable by id.
2. **Given** tenant A and tenant B each have an active schedule, integration, delivery,
   calendar event, mail thread, reminder, computer-use record, and evaluation, **When** a
   member of tenant A reads, mutates, or deletes any of those resources, **Then** only
   tenant A's resource is affected and tenant B's resource is unchanged.
3. **Given** events were appended for both tenants, **When** a member of tenant A subscribes
   to the live event stream or replays history, **Then** only tenant A's events are
   delivered and tenant B's events never appear.
4. **Given** a request reaches the store layer without a resolved tenant context, **When**
   that request targets a tenant-owned record, **Then** the access is rejected rather than
   silently returning all rows.

---

### User Story 2 - Existing local user's data lands in their personal tenant (Priority: P1)

An operator who has been running the daemon as a single local user upgrades to a build that
introduces tenant ownership. After the upgrade, all of their existing runs, schedules,
integrations, deliveries, calendar entries, mail threads, reminders, computer-use records,
and evaluation records appear under their default personal tenant, with no data loss and no
manual reassignment required.

**Why this priority**: Existing operators are the current install base. If migration drops,
duplicates, or strands their data, the upgrade is unshippable regardless of how clean the
multi-tenant story is.

**Independent Test**: Start from a pre-tenant fixture database, run the upgrade migration,
authenticate as the original local user, and confirm every record they had before the
upgrade is visible in their personal tenant and that record counts before and after match.

**Acceptance Scenarios**:

1. **Given** a database created before tenant ownership existed, **When** the daemon starts
   and runs the migration, **Then** the migration completes without error, every existing
   row is assigned to the default personal tenant, and the original local user can read
   every pre-existing record after authenticating.
2. **Given** the daemon is restarted after the migration completed once, **When** the
   migration runs again, **Then** it is a no-op and existing tenant assignments are not
   altered.
3. **Given** an operator has a backup taken before migration, **When** they restore that
   backup and run the documented rollback path, **Then** the database returns to its
   pre-migration state without partial-tenant artifacts.

---

### User Story 3 - Engineer can prove cross-tenant isolation in every migrated domain (Priority: P2)

An engineer working on any in-scope domain (runtime, product, harness, integrations,
delivery, calendar, mail, reminders, computer-use, evaluation) can run a regression suite
that creates same-shaped fixtures in two tenants and proves that no API, store helper,
event stream, or replay surface in that domain leaks across tenants. A schema inventory test
proves that every persisted table and event-bearing record is classified.

**Why this priority**: Cross-tenant isolation is easy to break silently as new domains are
added. Without explicit isolation regressions and an inventory completeness test, future
domains will reintroduce leaks.

**Independent Test**: Run the cross-tenant isolation suite and the inventory completeness
test. Both must pass on a clean checkout, and either must fail if a new tenant-owned table,
event source, or operator projection is added without classification or without isolation
coverage.

**Acceptance Scenarios**:

1. **Given** a domain in scope, **When** isolation tests create the same-shaped resource in
   two tenants and exercise its API, store helpers, event stream, and replay surface,
   **Then** every assertion confirms tenant A cannot read or mutate tenant B's resource.
2. **Given** the schema inventory checked into the repository, **When** the inventory
   completeness test runs, **Then** every persisted SQLite table and every event-bearing
   record source is present in the inventory with a classification of `tenant_owned`,
   `global`, or `derived`.
3. **Given** a new persisted table is added without an inventory entry, **When** CI runs
   the inventory completeness test, **Then** the test fails and blocks merge.
4. **Given** a tenant-owned table previously had a globally unique natural key (for example
   a name), **When** two tenants each create a record with the same natural key, **Then**
   both creations succeed and tenant-aware uniqueness is enforced per tenant.

---

### Edge Cases

- **Migration interrupted mid-run**: If the migration crashes or the daemon is killed
  partway through, the next start MUST resume from durable per-step progress and complete
  the migration; no partial-tenant state may be exposed to APIs in between starts. The
  daemon refuses to start only when resume detects an unsafe state (for example, schema
  drift or a corrupted progress record), in which case it surfaces a clear operator error
  rather than continuing.
- **Tenantless write attempt after migration**: Any code path that attempts to insert or
  update a tenant-owned record without a resolved tenant context must be rejected at the
  store layer, not silently succeed with a null or default value.
- **Cross-tenant id collision**: When two tenants independently create resources whose
  natural keys would have collided under the pre-tenant uniqueness constraints, both
  creations must succeed and remain isolated.
- **Replay window spanning the migration boundary**: Replay or event history requests that
  span time before and after the migration must return events scoped to the requesting
  tenant only, with pre-migration events attributed to the default personal tenant.
- **Operator projection or admin surface**: This roadmap does not introduce any
  cross-tenant admin projection. Every operator-facing projection MUST be tenant-scoped.
  Any future cross-tenant admin path is deferred to a later roadmap and MUST be designed
  with its own explicit access-control model at that time; this delivery MUST NOT add a
  tenantless store helper or admin route to support such a future path.
- **Backup restore after partial migration**: Restoring a backup taken before migration
  must put the database back in a fully pre-migration state, not a mixed state.
- **High-volume list path performance**: Tenant-scoped list queries on tables that grew
  large in single-tenant use MUST stay within a relative no-regression budget against a
  pre-migration baseline measured on the same representative fixture: post-migration p95
  list latency MUST be at most 1.2× the pre-migration p95 on that fixture. A query-plan
  assertion MUST also confirm that the tenant-aware index is selected on every in-scope
  list path.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST add tenant ownership to every persisted record in scope so
  that the owning tenant of any in-scope row can be determined unambiguously after
  migration.
- **FR-002**: The system MUST classify every persisted table and every event-bearing
  record source as `tenant_owned`, `global`, or `derived`, and the implementation plan
  MUST reject any unclassified table or event source.
- **FR-003**: The system MUST migrate all pre-existing rows of tenant-owned tables into
  the default personal tenant during the upgrade, preserving all other field values and
  relationships.
- **FR-004**: The system MUST scope every in-scope list, get, create, update, and delete
  API to the resolved tenant context of the caller and MUST NOT return or mutate records
  owned by a different tenant through normal API paths.
- **FR-005**: The system MUST scope every in-scope event stream and SSE replay surface to
  the resolved tenant context of the subscriber and MUST NOT deliver another tenant's
  events.
- **FR-006**: The system MUST require tenant context at the store layer for tenant-owned
  records and MUST reject or make unavailable any tenantless store helper that would read
  or mutate a tenant-owned table.
- **FR-007**: The system MUST reject inserts and updates on tenant-owned tables that lack
  a resolved tenant value, surfacing a deterministic error rather than a silent default.
- **FR-008**: The system MUST add tenant-aware indexes for common list and get paths on
  tenant-owned tables. The system MUST include automated query-plan assertions proving
  that the tenant-aware index is selected for every in-scope tenant-scoped list path on
  the local database test environment.
- **FR-009**: The system MUST enforce uniqueness on names or other natural keys per
  tenant where the pre-migration design treated those keys as globally unique, allowing
  two tenants to use the same natural key independently.
- **FR-010**: The migration MUST be additive and restart-safe. It MUST persist per-step
  (or per-table) progress durably so that an interrupted run resumes from the last
  completed step on the next start and reaches a fully-migrated state without operator
  intervention. Running the migration after it has already completed MUST produce the
  same end state as running it once. The daemon MUST refuse to start only when resume
  detects an unsafe state (such as schema drift or a corrupted progress record), and in
  that case MUST surface a clear operator error rather than continuing.
- **FR-011**: The system MUST document backup-restore as the sole supported rollback path
  and MUST require operators to take a pre-migration database backup before upgrading. No
  down-migration is shipped. Restoring a pre-migration backup MUST return the database to
  a fully pre-migration state with no tenant-only artifacts.
- **FR-012**: The system MUST include cross-tenant isolation tests that intentionally
  create same-shaped resources in two tenants for every in-scope domain and MUST prove
  that API, store, event, and replay surfaces in that domain do not leak across tenants.
- **FR-013**: The system MUST include an inventory completeness test that fails when the
  checked-in schema inventory does not match the actual persisted SQLite schema or the
  set of persisted event sources, so that newly added tables or event sources cannot
  bypass classification.
- **FR-014**: Where additive tenant fields are exposed on existing API or event payloads,
  the system MUST preserve backward-compatible serialization for clients that do not yet
  read those fields.
- **FR-014a**: The system MUST emit a typed audit event on the daemon's existing event
  surface for every denied cross-tenant access attempt, in addition to a greppable
  structured log line. The audit event MUST include the acting tenant, the route or
  store helper identifier, the resource kind, and a timestamp. The audit event MUST NOT
  include the target tenant id, the target resource id, or any target row data.
- **FR-015**: The system MUST keep secret value storage, OAuth or provider auth lifecycle,
  connector and MCP and sandbox policy administration, secret reference resolution, and
  redaction policy out of scope for this work, deferring those concerns to Roadmap 37; for
  any table touched by both, this work MUST make tenant ownership and tenant filtering
  explicit and safe without redefining credential semantics.
- **FR-016**: The system MUST keep per-tenant physical databases, billing usage counters,
  tenant switcher UI, and live side-effect replay out of scope for this work.
- **FR-017**: The system MUST NOT introduce any cross-tenant admin projection, admin
  route, or admin store helper in this delivery. Every operator-facing projection MUST be
  tenant-scoped. Cross-tenant admin surfaces are deferred to a later roadmap that will
  define their own access-control model.

### Key Entities

- **Tenant**: The unit of ownership and isolation; every in-scope persisted record either
  belongs to exactly one tenant (`tenant_owned`), belongs to no tenant (`global`), or is
  derived from tenant-owned records at read time (`derived`).
- **Default Personal Tenant**: The tenant created automatically for an existing local user
  during the upgrade; receives ownership of all pre-existing tenant-owned rows.
- **Tenant-Scoped Resource**: Any in-scope persisted record (run, schedule, workflow,
  delivery, integration, calendar entry, mail thread, reminder, computer-use record,
  evaluation record, or other in-scope domain record) that is owned by exactly one tenant
  after migration.
- **Schema Inventory**: A checked-in artifact that lists every persisted table and event
  source with its classification, tenant id source, migration action, affected APIs and
  events, store access requirements, indexes and uniqueness changes, isolation test
  expectations, and rollback note; the implementation plan MUST fail without it.
- **Tenant-Aware Store Access**: The set of store helpers and queries that require a
  resolved tenant context and that replace, restrict, or remove pre-existing tenantless
  helpers for tenant-owned tables.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Schema and storage surfaces change for every tenant-owned
  table (new tenant column, tenant-aware indexes, tenant-aware uniqueness). API payloads
  may grow additive tenant fields; existing fields and routes remain. Event and SSE
  replay payloads may grow additive tenant fields and gain server-side tenant filtering.
  No breaking removal of existing fields is in scope.
- **Migration / Rollback**: Additive, restart-safe forward migration that adds tenant
  ownership and backfills existing rows into the default personal tenant. Rollback is
  defined exclusively as restoring a pre-migration database backup; no down-migration is
  shipped. Operator-facing upgrade documentation MUST require taking a backup before the
  upgrade and MUST name backup-restore as the only supported rollback procedure.
- **Verification Strategy**: Migration tests from pre-tenant fixtures; cross-tenant API
  and store isolation regressions for every in-scope domain; schema inventory
  classification test; inventory completeness test that compares the inventory against
  the actual SQLite schema and event sources; store-layer tests proving tenantless access
  to tenant-owned tables fails or is unavailable; query-plan assertions confirming
  tenant-aware index selection for every in-scope list path; relative no-regression
  latency check (post-migration p95 ≤ 1.2× pre-migration p95 on a representative fixture)
  for high-volume tenant-owned list paths where the local database test environment
  supports it; contract tests for additive tenant fields where exposed; full daemon test
  suite after migration.
- **Observability Impact**: Migration progress, success, and failure must be visible to
  operators through the daemon's existing log and event surfaces with enough context to
  diagnose mid-migration failure. Cross-tenant access rejections at the API and store
  layers must be surfaced as deterministic errors that operators can grep AND as a typed
  audit event on the daemon's existing event surface, carrying acting tenant, route or
  store helper identifier, resource kind, and timestamp; neither surface may leak the
  target tenant's id or row data.
- **Environment & Secrets**: This work changes test and production daemon storage
  surfaces. No new secrets are introduced and no live connector behavior changes are
  required. Secret value storage and credential rotation remain out of scope and belong
  to Roadmap 37; this work must not redefine credential semantics for any table it
  touches in common with Roadmap 37.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Cross-tenant isolation regressions exist and pass for every in-scope
  domain, and removing the tenant filter from any in-scope API or store helper causes at
  least one isolation test to fail deterministically.
- **SC-002**: 100% of persisted tables and event-bearing record sources are classified in
  the checked-in schema inventory, and the inventory completeness test fails when a new
  persisted table or event source is added without a classification entry.
- **SC-003**: After running the upgrade migration on a pre-tenant fixture database, every
  pre-existing in-scope record is readable by the original local user under their default
  personal tenant and the post-migration record count for each in-scope table equals the
  pre-migration count.
- **SC-004**: Re-running the migration after it has already completed does not change any
  tenant assignment and does not produce errors.
- **SC-005**: Any attempt to insert or update a tenant-owned record without a resolved
  tenant context fails with a deterministic error from the store layer in 100% of cases.
- **SC-006**: For tenant-owned tables that previously had globally unique natural keys,
  two tenants can independently create records with the same natural key and both
  creations succeed without affecting each other.
- **SC-007**: Restoring a pre-migration database backup over a post-migration database
  returns the database to a fully pre-migration state with no tenant-only residual
  artifacts in 100% of attempts.
- **SC-008**: The full daemon test suite passes after migration on a database that was
  initialized from a pre-tenant fixture.
- **SC-009**: 100% of denied cross-tenant access attempts in isolation tests produce both
  a greppable structured log line and a typed audit event whose payload contains acting
  tenant, route or store helper identifier, resource kind, and timestamp, and contains
  no target tenant id or target row data.
- **SC-010**: For every in-scope tenant-scoped list path on the representative fixture
  (10 000 rows seeded across two tenants in the local SQLite test environment, applied
  per high-volume table — `runs`, `events`, and `mail_artifacts` for the first
  delivery), the query-plan assertion confirms the tenant-aware index is selected and
  the post-migration p95 latency is at most 1.2× the pre-migration p95 latency measured
  on the same fixture in the same process.

## Assumptions

- The default personal tenant exists as part of Roadmap 34 (Tenant Identity And Access
  Foundation) and is the correct destination for all pre-existing tenant-owned rows.
- Replay and harness infrastructure from Roadmap 33 (Evaluation And Replay Harness) is
  available and can accept tenant-aware filtering without redesign.
- Shared database tables with strong tenant scoping are the storage model for the first
  implementation; per-tenant physical databases are explicitly deferred and the storage
  resolver abstraction introduced here is sufficient for that future work.
- Existing local users are the only pre-migration data shape that must be supported;
  migrating from a multi-tenant snapshot is not a scenario this work needs to handle.
- The set of in-scope domains follows the upstream design document
  (`docs/specs/020-tenant-scoped-data-migration.md`) and includes runtime, product, and
  harness records along with replay, schedule, workflow, run, delivery, integration,
  calendar, mail, reminder, computer-use, and evaluation records.
- Roadmap 37 will own all credential, secret, OAuth/provider auth, connector and MCP and
  sandbox policy administration, secret reference resolution, and redaction concerns; any
  table touched in common is left in a state that is safe for tenant filtering but does
  not have its credential semantics changed by this work.
- Operators take a usable database backup before running the upgrade migration; if they
  do not, rollback is unavailable and this is documented as a hard prerequisite in the
  upgrade instructions.
