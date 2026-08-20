# Feature Specification: Tasks And Reminders

**Feature Branch**: `[016-tasks-reminders]`  
**Created**: 2026-04-23  
**Status**: Draft  
**Input**: User description: "结合 docs/specs/016-tasks-and-reminders.md 完成 phase 31 的工作"

## Clarifications

### Session 2026-04-23

- Q: `overdue` 和 `missed` 是否要分开建模？ → A: 是。`overdue` 表示提醒已过期但仍待处理，`missed` 表示该次提醒已被判定为错过并转入历史。
- Q: reminder-linked workflow 成功启动后，该次 reminder occurrence 应如何状态迁移？ → A: 自动记为 `acknowledged`，但不自动 `completed`。
- Q: recurring reminder 到下一次 recurrence 时，如果上一 occurrence 仍未处理，应如何处理？ → A: 将上一条未处理 occurrence 自动转为 `missed`，然后创建新的 `due` occurrence。
- Q: reminder-linked workflow 在 due 时启动失败后，该 occurrence 应如何处理？ → A: 保持 `due`，若继续未处理可再进入 `overdue`，不自动转为 `missed`。
- Q: 对 recurring reminder 来说，`acknowledged` 是否仍算未解决积压？ → A: 否。`acknowledged` 保留为历史，不阻止下一次 recurrence，也不会在 rollover 时自动转为 `missed`。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create And Receive Personal Reminders (Priority: P1)

As a user, I need to create one-time or recurring reminders that resurface at the right
time and notify me through the agent's existing background-delivery behavior so the
assistant can reliably remember personal work for me.

**Why this priority**: Phase 31 is not complete unless reminders become a first-class
user-facing domain rather than a thin wrapper around raw schedules. The minimum usable
slice is being able to create a reminder, let it become due, and receive truthful
notification.

**Independent Test**: Create one one-time reminder and one recurring reminder, inspect
their stored state before either is due, allow them to become due, and confirm each
reminder surfaces as a reminder-domain outcome with routed notification through the
shared delivery plane.

**Acceptance Scenarios**:

1. **Given** a user creates a one-time reminder for a future time, **When** the reminder
   is inspected before it is due, **Then** the system shows a reminder resource with its
   content, due time, current state, and no reminder outcome yet recorded.
2. **Given** a user creates a recurring reminder, **When** the reminder is inspected
   after creation, **Then** the system shows the reminder's recurrence intent, next due
   time, and current state as reminder truth rather than only as a low-level schedule.
3. **Given** a reminder becomes due and is configured for notification-only behavior,
   **When** the due time arrives, **Then** the system records a reminder-domain outcome
   and routes notification through the existing delivery targets and preferences instead
   of introducing a reminder-only delivery path.
4. **Given** reminders exist in one environment, **When** the user inspects another
   environment, **Then** the system does not imply cross-environment reminder state,
   history, or delivery reuse.

---

### User Story 2 - Manage Explicit Reminder Lifecycle (Priority: P2)

As a user or operator, I need reminders to move through explicit states such as due,
acknowledged, snoozed, completed, dismissed, cancelled, overdue, and missed so I can
trust what happened to each reminder without reconstructing it from logs.

**Why this priority**: The upstream roadmap explicitly requires acknowledgement, snooze,
completion, and missed reminders to be durable state truth. Without explicit lifecycle
state, reminder behavior remains ambiguous and operationally weak.

**Independent Test**: Let representative reminders become due, then snooze one,
acknowledge one, complete one, dismiss one, reschedule one, and force separate reminder
occurrences to become overdue and missed, confirming each outcome remains inspectable
after restart.

**Acceptance Scenarios**:

1. **Given** a reminder is currently due, **When** the user snoozes it, **Then** the
   system preserves the original due occurrence in reminder history, records the snooze
   action explicitly, and exposes the next reminder time.
2. **Given** a reminder is currently due, **When** the user acknowledges it without
   completing it, **Then** the system records acknowledgement explicitly and keeps the
   reminder inspectable for later follow-up or completion.
3. **Given** a reminder is currently due, **When** the user completes or dismisses it,
   **Then** the system records which terminal reminder action occurred and does not leave
   the reminder appearing pending or due.
4. **Given** a recurring reminder has a due occurrence, **When** the user acknowledges,
   completes, or dismisses that occurrence, **Then** the system preserves future
   recurrence unless the user explicitly cancels or reschedules the series.
5. **Given** a reminder is rescheduled before or after it becomes due, **When** the user
   inspects it afterward, **Then** the system shows the updated next due time and keeps
   prior reminder history visible.
6. **Given** a reminder was due but remains actionable after its intended surface time,
   **When** the reminder is later inspected, **Then** the system marks that occurrence
   as overdue explicitly rather than presenting it as on-time or silently discarding it.
7. **Given** a reminder occurrence is no longer actionable after it was not surfaced or
   handled in time, **When** the reminder history is later inspected, **Then** the
   system records that occurrence as missed explicitly rather than collapsing it into
   generic late delivery.
8. **Given** a recurring reminder reaches its next recurrence while the previous
   occurrence is still unresolved, **When** the new recurrence is created, **Then** the
   previous occurrence is recorded as missed and the new occurrence becomes the active
   due occurrence.
9. **Given** a recurring reminder has an acknowledged prior occurrence, **When** the
   next recurrence arrives, **Then** the acknowledged occurrence remains as history and
   a new due occurrence is created without converting the acknowledged one to missed.

---

### User Story 3 - Track Lightweight Follow-Up And Workflow-Linked Reminders (Priority: P3)

As a user, I need reminders to support lightweight follow-up tracking and optional
workflow launch so the agent can resurface personal work or continue a small piece of
background automation without becoming a full project-management product.

**Why this priority**: The roadmap scope is broader than simple notification alarms. It
includes lightweight follow-up tracking and reminder-linked workflows, but it must keep
those behaviors truthful and narrowly scoped.

**Independent Test**: Create one follow-up reminder linked to an existing piece of work
and one reminder configured to launch a workflow, let them become due, and confirm the
system preserves source linkage, workflow linkage, and delivery truth as separate
operator-visible outcomes.

**Acceptance Scenarios**:

1. **Given** a reminder is configured as notification-only, **When** it becomes due,
   **Then** the system records due reminder truth and any later acknowledgement or
   completion separately without inventing workflow execution that never happened.
2. **Given** a reminder is configured to launch a workflow when due, **When** the due
   time arrives, **Then** the system records reminder-trigger truth, automatically marks
   the reminder occurrence as acknowledged, and records linked workflow execution truth
   separately so operators can distinguish reminder acknowledgement from downstream
   workflow success or failure.
3. **Given** a reminder is configured to launch a workflow when due, **When** workflow
   startup fails at due time, **Then** the reminder occurrence remains due and may later
   become overdue if still unhandled, while workflow failure is recorded separately.
4. **Given** a lightweight follow-up reminder references prior calendar work, **When**
   the user inspects that reminder, **Then** the system reuses the existing
   calendar-domain linkage and outcome truth instead of redefining calendar identity or
   delivery semantics inside the reminder domain.
5. **Given** a follow-up reminder references prior work whose source is no longer
   available, **When** the reminder becomes due or is inspected, **Then** the system
   preserves the follow-up reminder itself and reports the missing or stale source truth
   explicitly rather than silently dropping the reminder.

### Edge Cases

- If a reminder becomes due while no active delivery target is available, the reminder
  remains inspectable as reminder truth and delivery is recorded separately as delayed,
  suppressed, unsent, or failed rather than marking the reminder completed.
- If a recurring reminder reaches a new due time while a previous occurrence remains
  unacknowledged or otherwise unresolved, the prior occurrence is recorded as missed and
  a new due occurrence is created, while prior occurrence history remains inspectable.
- If a recurring reminder reaches a new due time after the previous occurrence was
  acknowledged, the acknowledged occurrence remains in history and does not become
  missed solely because a new recurrence was created.
- If a user snoozes one occurrence of a recurring reminder, the snooze changes only that
  occurrence unless the user explicitly changes the recurring reminder itself.
- If a reminder is cancelled before it becomes due, the system does not emit a due
  notification later and preserves the cancellation reason in reminder history.
- If the daemon restarts while reminders are due or overdue, reminder resources and
  prior outcomes remain durable and later resurfacing makes overdue or missed status
  explicit instead of fabricating an on-time outcome.
- If multiple reminders become due at nearly the same time, each reminder remains
  independently inspectable and does not collapse into one undifferentiated reminder
  event.
- If a reminder-triggered workflow is blocked, interrupted, or fails after the reminder
  becomes due, operators can still distinguish reminder-trigger truth from downstream
  workflow truth and from reminder delivery truth.
- If a reminder-linked workflow fails to start when the reminder becomes due, that
  occurrence remains due and may later become overdue rather than being reclassified as
  missed or completed.
- If a reminder-triggered workflow starts successfully, that occurrence becomes
  acknowledged automatically, but it does not become completed unless a later explicit
  reminder action resolves it.
- If a follow-up reminder references existing calendar work, the reminder reuses the
  established calendar-domain contract instead of projecting a second calendar state
  model inside the reminder domain.
- If a user attempts to use phase 31 as a team assignment board, a full project plan, or
  memory-based habit coaching system, the system reports those behaviors as out of scope
  rather than presenting partial reminder support as full task-management capability.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST expose first-class reminder resources that are distinct
  from low-level schedules and are inspectable as reminder-domain truth.
- **FR-002**: Users MUST be able to create one-time reminders with user-visible content,
  a due time, and an explicit reminder behavior of notification-only or reminder-linked
  workflow launch.
- **FR-003**: Users MUST be able to create recurring reminders whose next due time and
  recurrence intent remain inspectable as reminder-domain truth.
- **FR-004**: Reminder resources MUST support explicit operator-visible lifecycle truth
  for at least pending, due, acknowledged, snoozed, completed, dismissed, cancelled,
  overdue, and missed reminder states.
- **FR-005**: Users or operators MUST be able to acknowledge, snooze, complete,
  dismiss, cancel, and reschedule reminders, and each action MUST persist explicit
  reminder history rather than overwriting prior reminder truth.
- **FR-005a**: For recurring reminders, acknowledge, snooze, complete, or dismiss
  actions on a due occurrence MUST affect that occurrence without implicitly cancelling
  the series unless the user explicitly cancels or rewrites the recurring reminder
  itself.
- **FR-005c**: For recurring reminders, an acknowledged occurrence MUST remain as
  acknowledged history and MUST NOT be reclassified as missed solely because a later
  recurrence occurs.
- **FR-005b**: If a reminder cannot be surfaced on time because the daemon, trigger
  path, or delivery path was unavailable, the system MUST preserve explicit overdue or
  missed reminder truth according to whether the occurrence remains actionable, and MUST
  NOT present the reminder later as an on-time success.
- **FR-006**: Reminder history MUST survive daemon restart within the same environment
  and remain inspectable without requiring raw logs or live re-fetch.
- **FR-007**: Reminder behavior MUST remain environment-scoped so test and later live
  environments do not share reminder resources, occurrence history, or delivery truth.
- **FR-008**: Reminder notifications MUST reuse the shared delivery targets,
  preferences, routing, and digest behavior established in phase 28 instead of
  introducing a reminder-only delivery model.
- **FR-009**: Reminder lifecycle truth MUST remain distinct from reminder delivery truth
  so a delivered, delayed, suppressed, or failed notification does not overwrite the
  reminder's own state.
- **FR-010**: Reminder-triggered workflow execution MUST remain distinct from
  notification-only reminders, and operator-visible history MUST show whether a due
  reminder only notified the user or also launched downstream work.
- **FR-010a**: When a reminder-linked workflow starts successfully for a due
  occurrence, the system MUST transition that occurrence to acknowledged automatically,
  but MUST NOT transition it to completed solely because downstream work started.
- **FR-010b**: When a reminder-linked workflow fails to start for a due occurrence, the
  system MUST preserve the occurrence as due, allow it to become overdue if it remains
  unhandled, and record the workflow-start failure separately from reminder lifecycle
  truth.
- **FR-011**: Reminder-triggered workflows MUST reuse the normal run and workflow plane
  instead of a parallel reminder-only execution path.
- **FR-012**: The system MUST support lightweight follow-up tracking as reminder-domain
  records that can reference prior personal work without requiring a full
  project-management suite.
- **FR-013**: When a reminder or lightweight follow-up references an existing
  integration-backed or domain-backed source item, the reminder projection MUST preserve
  source identity and environment scope without redefining the source domain's readiness,
  execution, or delivery truth.
- **FR-013a**: When a reminder or follow-up references calendar execution or calendar
  state, the system MUST reuse the concrete calendar-domain contract from phase 29
  instead of redefining calendar linkage, account identity, or delivery semantics.
- **FR-014**: Operator-visible history MUST distinguish reminder state transitions,
  reminder-triggered workflow execution outcomes, and delivery outcomes so operators can
  troubleshoot each plane independently.
- **FR-015**: Operators MUST be able to inspect pending, due, acknowledged, snoozed,
  completed, dismissed, cancelled, overdue, and missed reminders through reminder-domain
  surfaces rather than reconstructing state from scheduler internals.
- **FR-016**: Recurring reminders MUST preserve occurrence-level history so a later due,
  snooze, completion, dismissal, overdue, or missed occurrence does not erase prior
  occurrence truth.
- **FR-016a**: When a recurring reminder reaches a new scheduled occurrence while the
  prior occurrence remains unresolved, the system MUST mark the prior occurrence as
  missed and create a new due occurrence instead of keeping multiple active unresolved
  occurrences for the same reminder.
- **FR-016b**: For recurrence rollover, acknowledged occurrences are not considered
  unresolved active occurrences; the system MUST create the next due occurrence while
  preserving the acknowledged occurrence as historical reminder truth.
- **FR-017**: Phase 31 MUST stay scoped to personal reminders and lightweight follow-up.
  It MUST NOT require team assignment workflows, full project planning, or memory-based
  habit coaching to claim completion.

### Key Entities *(include if feature involves data)*

- **Reminder**: A durable user-facing record for personal work that needs to resurface at
  a later time, including its content, due or recurrence intent, current reminder state,
  and whether it is notification-only or linked to downstream work.
- **Reminder Occurrence**: One due instance of a reminder, especially for recurring or
  rescheduled reminders, including when it became due, what reminder state transition
  happened next, whether it became automatically acknowledged by workflow launch, and
  whether it became overdue, missed, or otherwise resolved.
- **Follow-Up Link**: The reminder-domain reference to an existing piece of personal
  work, such as prior calendar execution or another personal-domain outcome, while
  preserving the source domain's own identity and truth model.
- **Reminder Action Record**: The operator-visible history describing reminder creation,
  acknowledgement, snooze, completion, dismissal, cancellation, reschedule, due,
  overdue, or missed transition, and any linked workflow launch or delivery outcome
  references.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Additive API, schema, event, config, and storage surface
  changes are expected for reminder resources, occurrence history, follow-up linkage,
  and reminder-linked workflow or delivery projections. Existing schedule, workflow, and
  delivery behavior remains backward compatible.
- **Migration / Rollback**: Additive reminder-resource persistence and reminder history
  are required. Rollback is a revert of reminder-specific routes, projections, and
  persistence while preserving already-recorded reminder, workflow, and delivery history
  as read-only audit truth where needed.
- **Verification Strategy**: Required validation includes targeted reminder lifecycle
  coverage for one-time and recurring reminders; coverage for acknowledge, snooze,
  complete, dismiss, cancel, reschedule, and distinct overdue and missed state
  transitions; restart recovery validation for durable reminder history; contract
  coverage for reminder-domain resources or events; workflow-linked reminder coverage;
  and one manual recurring reminder verification path in `KURA_ENV=test`.
- **Observability Impact**: Operators must be able to inspect reminder creation, due or
  overdue transitions, acknowledgement, snooze, completion, dismissal, cancellation,
  reschedule, occurrence history, follow-up source linkage, linked workflow outcomes,
  and delivery suppression or failure as separate operator-visible truths.
- **Environment & Secrets**: Reminder-only validation defaults to `KURA_ENV=test` and
  does not require production connectors or production secrets. When a reminder links to
  calendar or other integration-backed work, it reuses the existing environment-scoped
  bindings and approval boundaries for that source domain and does not introduce new
  secret exposure.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In manual validation, a user can create and inspect a one-time or
  recurring reminder in under 2 minutes without using raw database access or log
  inspection.
- **SC-002**: In manual `KURA_ENV=test` verification, a created reminder becomes due and
  produces either a routed reminder notification or a clearly linked reminder-triggered
  workflow outcome without requiring an active foreground chat session.
- **SC-003**: In automated verification, 100% of exercised reminder outcomes are
  distinguishable as pending, due, acknowledged, snoozed, completed, dismissed,
  cancelled, overdue, or missed without consulting raw logs.
- **SC-004**: After daemon restart in test verification, previously created reminders
  remain inspectable, preserve prior occurrence history, and continue to show overdue or
  missed truth explicitly for reminders that were not surfaced on time.
- **SC-004a**: In automated verification for recurring reminders, when a new recurrence
  arrives before the prior occurrence is resolved, the prior occurrence transitions to
  missed and exactly one new active due occurrence is created.
- **SC-004b**: In automated verification for recurring reminders, when the prior
  occurrence is acknowledged, the next recurrence creates one new active due occurrence
  while preserving the prior occurrence as acknowledged history rather than converting it
  to missed.
- **SC-005**: In reminder verification that includes linked background work, operators
  can distinguish reminder state truth, downstream workflow truth, and delivery truth
  from operator-visible surfaces alone.
- **SC-005a**: In automated verification, when a reminder-linked workflow fails to
  start, the reminder occurrence remains due or later overdue, and operators can inspect
  that lifecycle truth separately from workflow-start failure truth.
- **SC-006**: In manual validation, a lightweight follow-up reminder linked to prior
  calendar work remains inspectable with its source linkage intact and does not require a
  separate calendar-only truth model to understand what it refers to.

## Assumptions

- Phase 31 is a personal single-user domain slice and does not introduce multi-user team
  assignment or collaborative task-management behavior.
- Recurring reminder actions default to occurrence-level semantics: snooze, complete, or
  dismiss act on the currently due occurrence unless the user explicitly changes the
  recurring reminder itself, and acknowledged occurrences remain historical rather than
  blocking later recurrence.
- Recurring reminders keep at most one active unresolved occurrence at a time; when the
  next scheduled recurrence arrives, any prior unresolved occurrence becomes missed.
- If a reminder cannot be surfaced on time, phase 31 distinguishes overdue
  still-actionable occurrences from missed historical occurrences instead of collapsing
  both into one late-state label.
- If a reminder-linked workflow cannot start at due time, phase 31 keeps the reminder
  occurrence actionable as due or overdue rather than treating workflow-start failure as
  an automatic miss.
- Lightweight follow-up tracking may reference prior workflow, calendar, mail, or other
  personal-domain work, but phase 31 does not require full external task-manager sync to
  claim completion.
- Reminder notifications reuse the shared delivery plane, including summary behavior
  where policy allows, while preserving separate reminder lifecycle truth.
