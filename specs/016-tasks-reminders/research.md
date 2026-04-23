# Research: Tasks And Reminders

## Decisions

### Decision: Introduce a dedicated daemon-owned reminders domain instead of exposing raw schedules as reminder truth

- Rationale: Phase 25 already owns low-level wakeup and dispatch semantics, but roadmap
  31 requires user-facing reminder resources, explicit lifecycle state, follow-up
  linkage, and operator-visible reminder history. A dedicated reminder domain keeps
  reminder truth inspectable without turning schedule resources into an accidental task
  API.
- Alternatives considered:
  - Reuse `daemon/internal/scheduler` resources directly as reminders.
    - Rejected because schedule status, target references, and dispatch attempts do not
      model acknowledgement, snooze, overdue, missed, or reminder-specific follow-up
      truth cleanly.
  - Represent reminders only as generic workflows with delayed starts.
    - Rejected because operators need durable reminder resources even when no workflow is
      configured or launched.

### Decision: Reuse scheduler trigger semantics, but persist reminder resources and occurrence history separately

- Rationale: The repository already has tested one-time and cron-based due-time logic,
  timezone handling, and restart-safe trigger behavior in the scheduler plane. Phase 31
  should reuse those semantics for recurrence evaluation rather than cloning cron logic,
  while still keeping reminder resources distinct from schedule resources.
- Alternatives considered:
  - Build a second independent recurrence engine inside the reminder domain.
    - Rejected because it would fork timezone and due-time semantics from roadmap 25 and
      increase long-term audit risk.
  - Internally create hidden schedule resources for every reminder.
    - Rejected because it would couple reminder persistence and operator reasoning to
      low-level schedule targets and dispatch history.

### Decision: Model reminder lifecycle around first-class reminder occurrences plus explicit action history

- Rationale: The clarified spec makes `due`, `acknowledged`, `snoozed`, `completed`,
  `dismissed`, `cancelled`, `overdue`, and `missed` materially different states, and
  recurring reminders need occurrence-level rollover rules. A separate occurrence record
  plus action history preserves truthful auditability without overloading the reminder
  resource itself.
- Alternatives considered:
  - Store only one mutable current-state field on the reminder resource.
    - Rejected because recurring reminders would lose prior occurrence truth and operators
      could not distinguish overdue from missed history.
  - Record only audit events and derive occurrence state on the fly.
    - Rejected because operator inspection and API filtering would become fragile and
      expensive under recurring rollover.

### Decision: Keep at most one active unresolved occurrence per recurring reminder

- Rationale: Clarification established that unresolved recurring occurrences roll to
  `missed` when the next recurrence arrives, while acknowledged occurrences remain
  historical. This rule keeps reminder state manageable and matches the intended
  semantics of lightweight personal follow-up rather than a queue of parallel tasks.
- Alternatives considered:
  - Allow multiple active unresolved occurrences to accumulate.
    - Rejected because it would widen the domain toward project-management backlog
      semantics and complicate delivery, filtering, and completion logic.
  - Block creation of the next recurrence until the prior occurrence is completed.
    - Rejected because it would make recurring reminders silently stop resurfacing.

### Decision: Treat reminder-triggered workflow launch as reminder-owned configuration that reuses the normal background workflow launcher

- Rationale: Phase 31 requires reminder-triggered workflow execution but also requires
  reminder lifecycle truth to stay distinct from downstream workflow truth. The reminder
  domain should therefore own the reminder-side decision and linkage while delegating the
  actual run/workflow execution to the same launcher path used by schedules.
- Alternatives considered:
  - Add a reminder-only executor inside the reminder manager.
    - Rejected because it would create a second execution boundary and violate the
      existing workflow/runtime architecture.
  - Auto-complete reminders when workflow launch starts.
    - Rejected because clarification established that successful launch only
      auto-acknowledges the occurrence; completion still requires later reminder truth.

### Decision: Preserve workflow-launch failure as reminder lifecycle truth separate from reminder miss semantics

- Rationale: Clarification established that workflow-launch failure does not mean the
  reminder was missed. The occurrence remains `due` and may later become `overdue`,
  while workflow failure is recorded separately. This keeps “the reminder was not
  handled” distinct from “the reminder was never actionable again.”
- Alternatives considered:
  - Convert workflow-launch failure directly into `missed`.
    - Rejected because it would conflate delivery or execution failure with reminder
      lifecycle semantics.
  - Add a new `failed` reminder state in phase 31.
    - Rejected because the current roadmap can represent the truth with reminder state
      plus separate workflow-launch failure linkage.

### Decision: Reuse the shared delivery plane with additive reminder linkage instead of a reminder-only notification model

- Rationale: The upstream roadmap explicitly requires reminder notifications to reuse
  roadmap 28 delivery targets, preferences, and digest behavior. Reminder occurrences
  should therefore emit normal delivery outcomes with reminder-owned source linkage while
  keeping reminder lifecycle truth authoritative on the reminder side.
- Alternatives considered:
  - Add a reminder-only notification inbox or separate reminder dispatcher.
    - Rejected because it would fork routing, suppression, and digest semantics from the
      shared delivery plane.
  - Skip delivery linkage and rely on reminder state alone.
    - Rejected because operators need to distinguish “due reminder exists” from “due
      reminder was routed successfully.”

### Decision: Store follow-up links as typed references to existing domain truth rather than embedding copied source-domain state

- Rationale: Lightweight follow-up is in scope, but the spec explicitly says reminder or
  follow-up flows that reference calendar execution should reuse the existing calendar
  contract. Typed references let reminders point at source-domain truth while preserving
  stale-source handling and avoiding duplicate ownership of calendar or mail state.
- Alternatives considered:
  - Copy calendar or mail snapshots into reminder documents.
    - Rejected because it would create shadow domain models and stale data ownership.
  - Restrict follow-up reminders to opaque free-text notes only.
    - Rejected because the roadmap explicitly includes lightweight follow-up tracking
      tied to prior personal work.

## Implementation Notes

- Extract or reuse a generic background workflow-launch helper so both schedules and
  reminders can launch normal runs/workflows without diverging execution semantics.
- Use reminder occurrence linkage, not delivery status, as the source of truth for
  whether a reminder is still actionable.
- Keep local verification deterministic: one due notification-only reminder, one snooze,
  one recurring rollover, one workflow-launch success, one workflow-launch failure, and
  one stale follow-up-link case are enough to close roadmap 31.
