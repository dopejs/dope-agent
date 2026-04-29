# Phase 0 Research: Billing, Quotas, And Usage Accounting

## R1. Billing Service Boundary

**Decision**: Add a shared `daemon/internal/billing` package that owns tenant plans,
quota definitions, effective quota projection, usage lifecycle, stable denial decisions,
audit inputs, and recovery rules. Guarded domains call this package rather than
embedding quota decisions in route handlers or managers.

**Rationale**: Roadmap 38 spans runs, workflows, runtime tool calls, live-validation gate
readiness, integration operations, storage/artifacts, and replay/evaluation attempts. A
shared package is the smallest maintainable way to keep reservation, commit, refund,
idempotency, concurrency, and fail-closed semantics consistent across every entry point.

**Alternatives considered**:

- Add quota checks directly to each route: rejected because retry, restart, and
  concurrency behavior would drift across domains.
- Treat quotas as an API-only concern: rejected because tool calls, artifacts, replay,
  and background launches also need daemon-owned enforcement before work starts.
- Add external billing infrastructure now: rejected because the upstream spec excludes
  payment-provider checkout and revenue workflows for this phase.

## R2. Durable Accounting Model

**Decision**: Store tenant plans, quota definitions, quota overrides, quota periods,
usage counters, reservations, usage events, denials, and manual adjustments durably in
SQLite. Reservation, commit, refund, denial, and adjustment operations are recorded by
stable operation identity and update the effective counter in the same durable
transaction.

**Rationale**: The daemon already uses SQLite for tenant-owned control-plane metadata.
Combining lifecycle records with counter updates allows auditability and efficient
projection while preserving idempotency across retries and restarts.

**Alternatives considered**:

- Derive all usage by scanning runtime/event history: rejected because reservations,
  refunds, manual adjustments, and pending recovery decisions need explicit state.
- Keep only counters without lifecycle records: rejected because operators could not
  explain denials, refunds, or adjustments.
- Introduce a separate metering service: rejected as unnecessary blast radius for the
  first internal-plan roadmap.

## R3. Initial Quota Catalog

**Decision**: Define the first quota catalog in `contracts/quota-catalog.md` before
coding. Initial categories cover run launches, workflow launches, runtime tool calls,
live validation attempts, integration operations, artifact/storage bytes where
measurable, and replay/evaluation attempts where measurable. Roadmap 38 defines the
live-validation quota category and reusable preflight gate contract, but does not build
the Roadmap 40 live-validation executor.

**Rationale**: The upstream spec makes catalog completeness a planning requirement.
Codifying every category before implementation prevents a narrow launch-only slice from
being mistaken for roadmap completion.

**Alternatives considered**:

- Start with run launches only: rejected because the roadmap definition of done requires
  all listed quota dimensions.
- Implement the Roadmap 40 live-validation executor in this phase: rejected because this
  roadmap only needs quota readiness and any existing entry-point wiring; side-effect
  execution semantics belong to Roadmap 40.
- Make categories free-form config only: rejected because stable denial shapes, tests,
  and SDK contracts need known initial identifiers.

## R4. UTC Period Boundaries

**Decision**: Anchor all quota period reset boundaries in UTC.

**Rationale**: UTC removes daylight-saving and tenant-local timezone ambiguity from
period reset tests, carryover math, retry recovery, and audit explanations.

**Alternatives considered**:

- Tenant-local timezone resets: rejected because tenant timezone changes and daylight
  saving transitions complicate deterministic accounting.
- Plan-configured timezone resets: rejected for v1 because it increases test and
  operator complexity without a Roadmap 38 requirement.

## R5. Reservation, Commit, Refund, And Idempotency

**Decision**: Every guarded operation uses a stable operation identity and records a
reservation before work starts. Successful consumption commits actual usage. Denial,
failure-before-consumption, cancellation, and retry release or refund according to the
quota category. Repeated lifecycle calls with the same operation identity return the
existing outcome instead of double-counting.

**Rationale**: Reservation/commit/refund semantics are required by the upstream spec and
are the only defensible way to handle retries, concurrent launch pressure, and daemon
restart without overcounting or bypassing limits.

**Alternatives considered**:

- Count only after successful completion: rejected because over-limit work could consume
  resources or external systems before denial.
- Deny based on cached remaining allowance only: rejected because concurrent operations
  could double-spend the last remaining quota.

## R6. Hosted Fail-Closed And Local Unlimited Behavior

**Decision**: Hosted tenants fail closed when quota state cannot be safely determined.
Local-first installations use an explicit development or unlimited plan that allows
guarded work by default and reports the plan as unlimited rather than silently skipping
quota logic.

**Rationale**: Hosted enforcement must not allow resource consumption when accounting is
unavailable. Local-first compatibility must avoid surprising denials in existing
operator workflows.

**Alternatives considered**:

- Fail open for hosted tenants: rejected because it creates quota bypass during storage
  or accounting failures.
- Skip plan records for local installs: rejected because inspection would be ambiguous
  and tests could not prove local behavior intentionally differs from hosted behavior.

## R7. Storage And Artifact Byte Accounting

**Decision**: Storage/artifact byte quotas reserve a defensible byte estimate before
write, commit actual bytes after write, and refund or adjust the difference. If the
actual byte count exceeds the reserved estimate and places the tenant over quota after
the write, keep the actual count committed with audit-visible over-limit evidence and
deny future quota-consuming work until effective usage is within limit.

**Rationale**: Actual bytes are often known only after serialization or write. Estimate
reservation preserves pre-consumption enforcement while actual-byte reconciliation keeps
usage accurate.

**Alternatives considered**:

- Count only after write: rejected because large writes could exceed limits before
  enforcement.
- Delete or hide already-written artifacts solely to repair quota state: rejected because
  it risks data loss and weakens auditability; quota correction should be explicit.
- Enforce only through periodic reconciliation: rejected because users need immediate and
  explainable quota behavior.

## R8. Restart Recovery

**Decision**: Pending reservations after restart recover to committed, released, or
operator-action-needed. If recovery cannot prove whether the operation consumed the
resource or is refundable, mark the reservation operator-action-needed and deny duplicate
quota-consuming work for the same operation until resolved.

**Rationale**: Ambiguous restart recovery should not guess. Operator-action-needed
preserves evidence and avoids both accidental overuse from automatic refunds and
accidental overcounting from automatic commits.

**Alternatives considered**:

- Automatically refund ambiguous reservations: rejected because consumed work could be
  made free and duplicated.
- Automatically commit ambiguous reservations: rejected because unconsumed work could
  strand tenant quota.
- Timeout then refund: rejected because timeout does not prove whether consumption
  happened.

## R9. Audit Retention

**Decision**: Retain billing and usage audit records indefinitely unless an operator
later applies an explicit retention policy.

**Rationale**: Quota-impacting records explain entitlement changes, denials,
adjustments, refunds, and recovery decisions. Shortening retention would weaken operator
support and future billing defensibility.

**Alternatives considered**:

- 90-day retention: rejected as too short for support and accounting disputes.
- 13-month retention: reasonable later, but rejected for v1 because no operator
  retention policy exists yet.
- Use generic audit retention: rejected because the spec requires billing/usage records
  to remain defensible by default.

## R10. Contract And SDK Surface

**Decision**: Expose tenant plan, effective quota, usage, denial, reservation lifecycle
summary, and manual adjustment shapes through daemon API contracts, event schemas, and
TypeScript SDK resources. Use stable denial reason codes rather than text parsing.

**Rationale**: Tenant owners, operators, SDK clients, and future UI surfaces need the
same source of truth. Contract tests prevent route handlers, schemas, and client models
from diverging.

**Alternatives considered**:

- Keep inspection operator-only through logs: rejected because tenant owners need plan
  and usage visibility.
- Return free-form denial messages only: rejected because SDK and UI behavior needs
  stable machine-readable outcomes.

## R11. Billing Administration Permission

**Decision**: Add `billing.manage` as the canonical plan, quota, manual-adjustment, and
reservation-resolution administration permission. Owner and admin roles receive it;
operator and viewer roles do not.

**Rationale**: Existing identity permissions include `billing.view`, which is appropriate
for inspection but too broad or ambiguous for entitlement mutation. A stable manage
permission keeps API contracts, SDK behavior, audit explanations, and role tests aligned.

**Alternatives considered**:

- Reuse `billing.view`: rejected because view permission must not imply mutation.
- Reuse `tenant.manage`: rejected because billing changes need separately auditable
  entitlement authority.
