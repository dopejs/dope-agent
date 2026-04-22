# Contract Surfaces: Scheduled Tasks Wakeups

## Goal

Add a daemon-owned trigger plane for one-time and recurring schedules without creating a
second execution boundary.

## HTTP API Surfaces

### New Schedule Resource Routes

- `POST /v1/schedules`
- `GET /v1/schedules`
- `GET /v1/schedules/{scheduleId}`
- `POST /v1/schedules/{scheduleId}/pause`
- `POST /v1/schedules/{scheduleId}/resume`
- `POST /v1/schedules/{scheduleId}/cancel`

Request and response requirements:

- schedule creation accepts:
  - trigger definition:
    - one-time RFC3339 timestamp, or
    - recurring cron expression plus explicit IANA timezone
  - launch target definition that the daemon persists as a stable target reference
  - target kind: `run` or `workflow`
  - bounded retry policy for dispatch-side failures
- `GET /v1/schedules` returns summary fields:
  - `scheduleId`
  - `kind`
  - `status`
  - `nextDueAt`
  - target summary
  - most recent outcome
  - timezone for recurring schedules
- `GET /v1/schedules/{scheduleId}` expands:
  - full trigger definition
  - stable target reference metadata
  - retry and backoff state
  - dispatch-attempt history
  - linked run/workflow IDs when dispatch succeeded
- `pause`, `resume`, and `cancel` are explicit command routes rather than hidden state
  mutations

Schema surfaces:

- add `schemas/api/create-schedule.request.schema.json`
- add `schemas/api/schedule-resource.schema.json`
- add `schemas/api/schedule-list.response.schema.json`
- add `schemas/api/schedule-trigger-resource.schema.json`
- add `schemas/api/schedule-attempt-resource.schema.json`

### Existing Run And Workflow Surfaces Extended

- `GET /v1/runs`
- `GET /v1/runs/{runId}`
- `GET /v1/runs/{runId}/workflows/{workflowId}`

Additive requirements:

- run resources gain additive schedule linkage:
  - `scheduleId`
  - `scheduleAttemptId`
- workflow resources gain additive schedule linkage when the workflow was launched by a
  schedule:
  - `scheduleId`
  - `scheduleAttemptId`
- existing direct run and workflow creation remain valid without schedule creation

Schema surfaces:

- update `schemas/api/run-resource.schema.json`
- update `schemas/api/workflow-resource.schema.json`
- update `schemas/api/run-list.response.schema.json` if the shared resource reference
  changes

## Event And History Surfaces

New schedule event families:

- `schedule.created`
- `schedule.status_changed`
- `schedule.dispatch_attempted`
- `schedule.dispatch_recorded`
- `schedule.retry_scheduled`

Event payload requirements:

- `scheduleId`
- `scheduleAttemptId` when applicable
- `status` or `dispatchStatus`
- trigger metadata:
  - `dueAt`
  - `triggerSource`
  - `timezone` when recurring
- target metadata:
  - `targetKind`
  - `targetRefId`
  - `resolvedTargetRevision` when dispatch was attempted
- linkage metadata when dispatch succeeded:
  - `runId`
  - `workflowId`
- failure metadata when relevant:
  - `failureClass`
  - `failureReason`
  - `retryCount`
  - `nextRetryAt`
- skip or miss metadata when relevant:
  - `skippedReason`
  - `missedCount`

Schema surfaces:

- add `schemas/events/schedule-created.event.schema.json`
- add `schemas/events/schedule-status-changed.event.schema.json`
- add `schemas/events/schedule-dispatch-attempted.event.schema.json`
- add `schemas/events/schedule-dispatch-recorded.event.schema.json`
- add `schemas/events/schedule-retry-scheduled.event.schema.json`

Truthfulness rules:

- dispatch failure must be distinguishable from downstream run/workflow failure
- restart catch-up must record missed intervals explicitly instead of silently replaying
  them
- overlap skips and paused skips must be distinguishable
- retry exhaustion must be explicit and operator-visible

## Persistence Surfaces

Persistence remains additive to the existing daemon store:

- add a `schedules` table for top-level schedule resources
- add a `schedule_targets` table for stable launch target references and current target
  definition
- add a `schedule_dispatch_attempts` table for due-time, skip, miss, retry, and launch
  history
- add nullable schedule linkage columns to `runs` and `workflows` for reverse inspection

Persistence rules:

- schedule rows are environment-scoped inside the daemon-owned SQLite store
- due-time progress, retry scheduling, and missed intervals must be durable across daemon
  restart
- restart catch-up evaluates persisted schedules using stored `nextDueAt` and emits
  explicit missed/dispatch records rather than reconstructing from logs
- schedule persistence must not become a second execution ledger for steps or tool calls

## Documentation Surfaces

Docs updated by implementation:

- `docs/harness/harness-architecture.md`
- `docs/runtime/daemon-roadmaps.md`
- operator-facing runtime/harness docs covering schedule lifecycle, retry/backoff,
  timezone semantics, restart catch-up, and linkage to downstream execution truth

## Truthfulness Constraints

- schedule dispatch must create normal run/workflow truth rather than hidden background
  work
- recurring schedules are non-reentrant in phase 25
- only the most recent overdue trigger is eligible for restart catch-up dispatch
- recurring evaluation always uses explicit IANA timezone semantics
- launch targets resolve from stable references at dispatch time rather than stale
  embedded snapshots
- existing non-scheduled run and workflow flows remain backward compatible
