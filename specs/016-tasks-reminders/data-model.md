# Data Model: Tasks And Reminders

## Entities

### Reminder Resource

- Purpose: User-facing reminder configuration and current reminder-domain projection,
  including content, trigger intent, workflow-launch mode when present, and the latest
  active-state summary.
- Fields:
  - `reminderId`
  - `environmentScope`
  - `title`
  - `details`
  - `behaviorMode`: `notify_only` or `launch_workflow`
  - `triggerKind`: `once` or `recurring`
  - `fireAt`
  - `cronExpr`
  - `timezone`
  - `nextDueAt`
  - `activeOccurrenceId`
  - `currentState`
  - `workflowLaunchConfig`
  - `followUpLink`
  - `createdAt`
  - `updatedAt`
  - `cancelledAt`
- Validation rules:
  - each reminder belongs to exactly one environment scope
  - one-time reminders require `fireAt`
  - recurring reminders require both `cronExpr` and explicit IANA `timezone`
  - `launch_workflow` reminders require valid workflow-launch configuration
  - reminder resources remain distinct from schedule resources even when they reuse
    trigger semantics

### Workflow Launch Config

- Purpose: Reminder-owned configuration describing the downstream workflow or run target
  to launch when a due occurrence should start background work.
- Fields:
  - `entrypoint`
  - `runGoal`
  - `workflowGoal`
  - `allowBackgroundLaunch`
  - `launchSummary`
- Validation rules:
  - config is present only when `behaviorMode` is `launch_workflow`
  - successful launch does not complete the reminder automatically
  - launch config references the existing workflow/runtime plane rather than a
    reminder-only executor

### Reminder Occurrence

- Purpose: One due instance of a reminder with explicit lifecycle, delivery linkage, and
  optional workflow-launch linkage.
- Fields:
  - `occurrenceId`
  - `reminderId`
  - `sequenceNumber`
  - `scheduledFor`
  - `state`: `due`, `acknowledged`, `snoozed`, `completed`, `dismissed`, `cancelled`,
    `overdue`, or `missed`
  - `stateReason`
  - `becameDueAt`
  - `snoozedUntil`
  - `acknowledgedAt`
  - `completedAt`
  - `dismissedAt`
  - `cancelledAt`
  - `overdueAt`
  - `missedAt`
  - `autoAcknowledgedByWorkflow`
  - `runId`
  - `workflowId`
  - `latestDeliveryId`
  - `latestDeliveryStatus`
  - `createdAt`
  - `updatedAt`
- Validation rules:
  - each occurrence belongs to exactly one reminder
  - recurring reminders keep at most one active unresolved occurrence at a time
  - unresolved rollover converts the prior unresolved occurrence to `missed` before a
    new due occurrence is created
  - acknowledged occurrences remain historical and do not become `missed` solely because
    later recurrences occur
  - successful workflow launch sets `autoAcknowledgedByWorkflow=true` and moves the
    occurrence to `acknowledged`
  - workflow-launch failure leaves the occurrence `due` or later `overdue`

### Reminder Action Record

- Purpose: Operator-visible audit trail describing why a reminder or occurrence changed
  state.
- Fields:
  - `actionId`
  - `reminderId`
  - `occurrenceId`
  - `actionKind`: `created`, `due_marked`, `acknowledged`, `snoozed`, `completed`,
    `dismissed`, `cancelled`, `rescheduled`, `overdue_marked`, `missed_marked`,
    `workflow_launch_started`, `workflow_launch_failed`, `delivery_linked`,
    `follow_up_stale`
  - `actorKind`: `user` or `system`
  - `previousState`
  - `newState`
  - `reason`
  - `relatedRunId`
  - `relatedWorkflowId`
  - `relatedDeliveryId`
  - `createdAt`
- Validation rules:
  - each accepted lifecycle change records exactly one action record
  - action history remains append-only
  - action records preserve workflow-launch and delivery linkage separately from
    lifecycle state

### Follow-Up Link

- Purpose: Lightweight typed reference from a reminder to existing personal-domain work
  without duplicating the source domain model.
- Fields:
  - `linkKind`: `calendar_operation`, `calendar_artifact`, `mail_operation`,
    `workflow`, `run`, or future typed source
  - `sourceId`
  - `integrationId`
  - `displaySummary`
  - `stale`
  - `staleReason`
  - `linkedAt`
  - `updatedAt`
- Validation rules:
  - follow-up links are optional
  - calendar-linked follow-up references existing phase-29 calendar truth rather than a
    duplicate calendar state model
  - stale or missing source truth must remain explicit without deleting the reminder

### Reminder Summary Projection

- Purpose: Additive lightweight linkage projected onto runs, workflows, and delivery
  outcomes so operators can locate reminder-domain truth from background execution and
  delivery records.
- Fields:
  - `reminderId`
  - `occurrenceId`
  - `state`
  - `behaviorMode`
  - `capturedAt`
- Validation rules:
  - summary projections are immutable snapshots once attached to downstream resources
  - projections never replace the authoritative reminder occurrence record

## State Transitions

### Reminder Occurrence Lifecycle

- `due` -> `acknowledged` when the user acknowledges the occurrence or when workflow
  launch starts successfully
- `due` -> `snoozed` when the user postpones the occurrence to a later actionable time
- `due` -> `completed` when the user marks the reminder work complete
- `due` -> `dismissed` when the user intentionally clears the occurrence without
  completion
- `due` -> `overdue` when the occurrence remains actionable after its intended surface
  time
- `due` or `overdue` -> `missed` when the occurrence is no longer actionable or when a
  recurring rollover replaces an unresolved occurrence with the next due occurrence
- `due`, `acknowledged`, `snoozed`, or `overdue` -> `cancelled` when the reminder is
  cancelled before terminal resolution

### Recurring Rollover

- unresolved prior occurrence + next recurrence -> prior occurrence `missed` and one new
  occurrence `due`
- acknowledged prior occurrence + next recurrence -> prior occurrence remains
  `acknowledged` history and one new occurrence `due`
- completed, dismissed, or cancelled prior occurrence + next recurrence -> one new
  occurrence `due`

## Relationships

- one reminder resource may own many occurrences over time
- one occurrence may own many action records
- one reminder may reference zero or one follow-up link at a time in phase 31
- one occurrence may link to zero or one run, zero or one workflow, and zero or one
  latest delivery outcome
- one run, workflow, or delivery outcome may reference reminder summary projections for
  reminder-originated background work

## Derived Views

- reminder list views can filter by `behaviorMode`, `triggerKind`, `currentState`,
  `nextDueAt`, or follow-up-link kind
- occurrence list views can filter by `reminderId`, `state`, `scheduledFor`, `runId`,
  `workflowId`, or `latestDeliveryId`
- run, workflow, and delivery detail views can project reminder linkage beside existing
  schedule, calendar, and mail linkage without replacing those domains' authoritative
  records
