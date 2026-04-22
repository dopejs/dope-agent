# Research: Scheduled Tasks Wakeups

## Decisions

### Decision: Introduce first-class top-level schedule resources managed by a daemon-owned scheduler loop

- Rationale: Schedules are not subordinate to a single existing run because their primary
  job is to create future work later. A top-level resource makes inspection, pause or
  resume, cancellation, and dispatch history operator-visible before any downstream run or
  workflow exists. The scheduler loop belongs in the daemon so wakeups remain
  environment-scoped and recoverable after restart.
- Alternatives considered:
  - Reuse connector loops or chat sessions as wakeup owners.
    - Rejected because wakeups must exist without an open connector session.
  - Depend on OS cron or external task runners.
    - Rejected because the roadmap requires daemon-owned truth, environment scoping, and
      auditable restart behavior inside the existing control plane.

### Decision: Dispatch by creating normal runs and optional workflows, never by adding a hidden background executor

- Rationale: The spec requires all scheduled work to preserve current run, workflow, step,
  tool-call, approval, and policy semantics. The scheduler therefore stops at creating a
  normal run and, when requested, planning and starting a normal workflow through the
  existing orchestration routes or equivalent internal helpers.
- Alternatives considered:
  - Execute schedule targets directly inside the scheduler package.
    - Rejected because it would create a second execution boundary and drift from current
      audit and approval truth.
  - Add a schedule-specific execution ledger disconnected from run/workflow resources.
    - Rejected because it would force operators to reconstruct outcomes from two systems.

### Decision: Use explicit schedule dispatch-attempt history as the single operator-visible ledger for fired, missed, skipped, retried, and exhausted triggers

- Rationale: The operator needs to tell apart dispatch failure, downstream execution
  failure, paused skip, overlap skip, and bounded restart catch-up misses. A dedicated
  dispatch-attempt ledger keeps these outcomes explicit and testable without overloading
  the top-level schedule status.
- Alternatives considered:
  - Only store the most recent outcome on the schedule resource.
    - Rejected because recurring schedules need durable historical truth.
  - Use raw events as the only historical record.
    - Rejected because list and detail surfaces must remain queryable from persisted
      schedule state even without replaying the full event log.

### Decision: Persist schedule-owned stable launch target references that resolve at dispatch time

- Rationale: The spec clarification fixed launch target semantics to “resolve the current
  visible definition at dispatch time.” Phase 25 therefore needs a stable target
  reference, not a one-off embedded execution snapshot. The schedule stores or references a
  daemon-owned target definition, and each dispatch attempt records the resolved target
  revision used when work is launched.
- Alternatives considered:
  - Embed a full immutable launch snapshot in the schedule and always dispatch from that
    copy.
    - Rejected because it violates the clarified reference semantics and makes target
      updates invisible to future dispatches.
  - Require schedules to point only at pre-existing immutable external workflow artifacts.
    - Rejected because the current daemon does not yet expose that artifact layer and the
      roadmap should stay closed inside schedule work.

### Decision: Evaluate one-time schedules with RFC3339 timestamps and recurring schedules with cron expressions plus explicit IANA timezones

- Rationale: One-time wakeups need an absolute moment; recurring schedules need a compact,
  familiar rule format that works with explicit timezone semantics. Persisting
  `nextDueAt` in UTC while retaining the source timezone and cron rule keeps due-time
  evaluation deterministic and operator-visible.
- Alternatives considered:
  - Use daemon-host local time only.
    - Rejected because the clarified spec forbids implicit host-timezone semantics.
  - Invent a custom recurrence DSL.
    - Rejected because it adds complexity without product value for the first slice.

### Decision: Use a non-reentrant recurring model where overlapping triggers become visible skipped or blocked attempts

- Rationale: Phase 25 should keep one active execution per schedule. If the next due time
  arrives while the prior linked run/workflow is still active, recording a skipped/blocked
  attempt is safer and more explainable than queuing or launching concurrent duplicates.
- Alternatives considered:
  - Queue overlapping triggers to run later.
    - Rejected because it creates implicit backlog semantics and complicates catch-up and
      retry state.
  - Allow concurrent executions for the same schedule.
    - Rejected because it increases operational risk and complicates personal-agent task
      semantics.

### Decision: Apply bounded automatic retry with backoff only to dispatch-side failures, not to downstream run/workflow failures

- Rationale: The schedule plane owns trigger and launch reliability, while downstream
  execution remains owned by run/workflow truth. Automatic retries therefore apply when
  the scheduler cannot launch work, not when an already-created run or workflow later
  fails.
- Alternatives considered:
  - Never retry dispatch-side failures.
    - Rejected because the spec explicitly includes operator-visible retry/backoff.
  - Retry downstream workflow failures at the schedule plane.
    - Rejected because it blurs the execution boundary and may duplicate side effects.

### Decision: Keep scheduler ownership single-daemon and environment-scoped, with idempotent persisted progress rather than distributed leasing

- Rationale: The current daemon is a single local or server process per environment. A
  single-owner scheduler loop with persisted state transitions is enough for roadmap 25
  and avoids unnecessary distributed-coordination machinery.
- Alternatives considered:
  - Add cross-process leader election or row-level leases.
    - Rejected because the current deployment model does not need it.
  - Keep scheduler state only in memory.
    - Rejected because restart-safe catch-up and operator-visible history require durable
      persistence.

## Implementation Notes

- The existing placeholder `daemon/internal/scheduler/scheduler.go` becomes the primary
  owner for due-time scanning, overlap checks, bounded catch-up, retry/backoff, and
  dispatch bookkeeping.
- `daemon/internal/app/app.go` should restore persisted state first, then start the
  scheduler loop in the current environment after the store, runtime, and event bus are
  ready.
- The store should add dedicated schedule tables rather than overload workflow tables,
  while downstream run/workflow resources gain additive schedule linkage fields for
  reverse inspection.
- Restart catch-up should only synthesize one overdue dispatch per recurring schedule and
  record older missed intervals explicitly instead of silently replaying them.
