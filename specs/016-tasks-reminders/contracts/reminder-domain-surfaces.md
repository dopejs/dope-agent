# Contract Surfaces: Tasks And Reminders

## Goal

Add a daemon-owned reminders domain that reuses shared trigger, workflow, and delivery
truth while exposing inspectable reminder resources, occurrence lifecycle, lightweight
follow-up links, and reminder-triggered workflow linkage.

## HTTP API Surfaces

### Reused Delivery Dependency

- existing phase 28 routes remain the source of truth for reminder-result routing and
  delivery history:
  - `GET /v1/delivery/targets`
  - `POST /v1/delivery/targets`
  - `GET /v1/delivery/preferences`
  - `POST /v1/delivery/preferences`
  - `GET /v1/deliveries`
  - `GET /v1/deliveries/{deliveryId}`
- reminder routes must reference those resources rather than duplicating delivery target,
  preference, suppression, or digest semantics

### New Reminder Resource Routes

- `POST /v1/reminders`
- `GET /v1/reminders`
- `GET /v1/reminders/{reminderId}`
- `POST /v1/reminders/{reminderId}/acknowledge`
- `POST /v1/reminders/{reminderId}/snooze`
- `POST /v1/reminders/{reminderId}/complete`
- `POST /v1/reminders/{reminderId}/dismiss`
- `POST /v1/reminders/{reminderId}/reschedule`
- `POST /v1/reminders/{reminderId}/cancel`

Request and response requirements:

- reminder creation accepts:
  - `title`
  - optional `details`
  - `behaviorMode`: `notify_only` or `launch_workflow`
  - trigger definition:
    - one-time `fireAt`, or
    - recurring `cronExpr` plus explicit IANA `timezone`
  - optional `workflowLaunchConfig`
  - optional `followUpLink`
- reminder list responses support filtering by:
  - `behaviorMode`
  - `triggerKind`
  - `currentState`
  - `dueBefore`
  - `dueAfter`
- reminder detail responses return:
  - resource identity and environment scope
  - trigger intent and `nextDueAt`
  - `activeOccurrenceId`
  - current reminder-state projection
  - workflow-launch configuration summary when present
  - follow-up-link summary when present
  - recent occurrence and action-history summaries
- lifecycle command routes accept:
  - optional `occurrenceId`; when omitted, the active occurrence is targeted
  - command-specific fields such as `snoozedUntil` or updated trigger fields for
    reschedule
- command responses return:
  - authoritative reminder resource
  - the targeted occurrence after transition
  - additive action-history summary for the accepted transition

Schema surfaces:

- add `schemas/api/create-reminder.request.schema.json`
- add `schemas/api/reminder-resource.schema.json`
- add `schemas/api/reminder-list.response.schema.json`
- add `schemas/api/reminder-trigger-resource.schema.json`
- add `schemas/api/reminder-workflow-launch.schema.json`
- add `schemas/api/reminder-follow-up-link.schema.json`
- add `schemas/api/acknowledge-reminder.request.schema.json`
- add `schemas/api/snooze-reminder.request.schema.json`
- add `schemas/api/complete-reminder.request.schema.json`
- add `schemas/api/dismiss-reminder.request.schema.json`
- add `schemas/api/reschedule-reminder.request.schema.json`
- add `schemas/api/cancel-reminder.request.schema.json`

### New Reminder Occurrence And History Routes

- `GET /v1/reminders/occurrences`
- `GET /v1/reminders/occurrences/{occurrenceId}`
- `GET /v1/reminders/{reminderId}/actions`

Request and response requirements:

- occurrence list responses support filtering by:
  - `reminderId`
  - `state`
  - `scheduledBefore`
  - `scheduledAfter`
  - `runId`
  - `workflowId`
  - `deliveryId`
- occurrence detail responses return:
  - `occurrenceId`
  - `reminderId`
  - scheduled time and due-time metadata
  - lifecycle state and timestamps
  - workflow linkage (`runId`, `workflowId`) when present
  - latest delivery linkage when present
  - follow-up-link summary inherited from the parent reminder when relevant
- reminder action-history routes return:
  - ordered action records with previous/new state, actor kind, reason, and related
    run/workflow/delivery linkage when present

Schema surfaces:

- add `schemas/api/reminder-occurrence-resource.schema.json`
- add `schemas/api/reminder-occurrence-list.response.schema.json`
- add `schemas/api/reminder-action-resource.schema.json`
- add `schemas/api/reminder-action-list.response.schema.json`

### Existing Run, Workflow, And Delivery Surfaces Extended

- `GET /v1/runs`
- `GET /v1/runs/{runId}`
- `GET /v1/runs/{runId}/workflows/{workflowId}`
- `GET /v1/deliveries`
- `GET /v1/deliveries/{deliveryId}`

Additive requirements:

- run resources gain additive reminder linkage:
  - `reminderId`
  - `reminderOccurrenceId`
- workflow resources gain additive reminder linkage when launched from a reminder:
  - `reminderId`
  - `reminderOccurrenceId`
- delivery outcomes must accept reminder-owned source linkage:
  - `sourceKind: "reminder_occurrence"`
  - `sourceId`: the occurrence ID
  - additive reminder summary when present

Schema surfaces:

- update `schemas/api/run-resource.schema.json`
- update `schemas/api/workflow-resource.schema.json`
- update `schemas/api/run-list.response.schema.json` if shared references change
- update `schemas/api/delivery-outcome-resource.schema.json`

## Event And History Surfaces

New reminder event families:

- `reminder.created`
- `reminder.updated`
- `reminder.occurrence_created`
- `reminder.occurrence_transitioned`
- `reminder.workflow_launch_started`
- `reminder.workflow_launch_failed`
- `reminder.delivery_linked`

Event payload requirements:

- reminder resource truth:
  - `reminderId`
  - `behaviorMode`
  - trigger metadata
  - `nextDueAt`
- occurrence truth:
  - `occurrenceId`
  - `reminderId`
  - `state`
  - `scheduledFor`
  - `previousState`
  - `reason`
- workflow linkage truth when present:
  - `runId`
  - `workflowId`
- delivery linkage truth when present:
  - `deliveryId`
  - `deliveryStatus`
- follow-up-link truth when present:
  - `linkKind`
  - `sourceId`
  - `stale`

Schema surfaces:

- add `schemas/events/reminder-created.event.schema.json`
- add `schemas/events/reminder-updated.event.schema.json`
- add `schemas/events/reminder-occurrence-created.event.schema.json`
- add `schemas/events/reminder-occurrence-transitioned.event.schema.json`
- add `schemas/events/reminder-workflow-launch-started.event.schema.json`
- add `schemas/events/reminder-workflow-launch-failed.event.schema.json`
- add `schemas/events/reminder-delivery-linked.event.schema.json`

Truthfulness rules:

- reminder lifecycle truth must remain distinct from workflow execution truth
- reminder lifecycle truth must remain distinct from delivery outcome truth
- `overdue` and `missed` must remain separate occurrence states
- recurring rollover must not silently erase prior occurrence history
- acknowledged occurrences must not be auto-reclassified as missed on later recurrence

## Persistence Surfaces

Persistence remains additive to the existing daemon-owned SQLite store:

- add a `reminders` table for top-level reminder resources
- add a `reminder_occurrences` table for due instances, lifecycle state, workflow
  linkage, and latest delivery linkage
- add a `reminder_actions` table for append-only reminder action history
- add nullable reminder linkage columns to `runs` and `workflows` for reverse inspection

Persistence rules:

- reminder rows, occurrence rows, and action-history rows are environment-scoped and
  durable across daemon restart
- recurrence evaluation must use persisted trigger metadata and current occurrence state
  rather than reconstructing from logs
- reminder persistence must not become a shadow schedule ledger or a second execution
  ledger for workflow steps or tool calls
- follow-up-link persistence stores typed references and stale-source state, not copied
  source-domain payloads

## Documentation Surfaces

Docs updated by implementation:

- `docs/runtime/daemon-roadmaps.md`
- `docs/runtime/daemon-api-and-event-model.md`
- `docs/runtime/operator-trust-model.md`
- `docs/harness/harness-architecture.md`
- downstream roadmap specs that depend on reminder semantics:
  - `docs/specs/016-tasks-and-reminders.md`

## Truthfulness Constraints

- reminder resources are distinct from raw schedule resources even when they reuse
  trigger semantics
- reminder-triggered workflow launch uses the normal runtime/workflow plane rather than a
  reminder-only executor
- successful workflow launch auto-acknowledges but does not auto-complete the reminder
  occurrence
- failed workflow launch leaves the reminder occurrence due or later overdue
- recurring reminders keep at most one active unresolved occurrence at a time
- follow-up links reuse source-domain truth rather than duplicating calendar or mail
  state inside reminders
