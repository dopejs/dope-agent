# Contract Surfaces: Calendar Integration

## Goal

Add a daemon-owned calendar domain that reuses shared integration readiness and delivery
truth while exposing inspectable calendar account projection, event inspection,
busy/free lookup, timed single-event mutation, and background workflow linkage.

## HTTP API Surfaces

### Reused Integration Dependency

- existing phase 27 routes remain the source of truth for calendar integration readiness
  and canonical-default selection:
  - `GET /v1/integrations`
  - `GET /v1/integrations/{integrationId}`
  - `POST /v1/integrations/{integrationId}/readiness`
  - `POST /v1/integrations/{integrationId}/default`
- calendar routes must reference those resources rather than duplicating readiness or
  canonical-default mutation semantics

### New Calendar Account Projection Routes

- `GET /v1/calendar/accounts`
- `GET /v1/calendar/accounts/{integrationId}`

Request and response requirements:

- list responses support filtering by:
  - `integrationId`
  - `readinessStatus`
  - `canonicalDefault`
- account projection responses return:
  - integration linkage (`integrationId`, `accountKey`, `accountLabel`)
  - `environmentScope`
  - readiness summary copied from the linked integration
  - `primaryCalendarRef`
  - `primaryCalendarLabel`
  - `primaryTimezone`
  - capability flags for inspection, busy/free, and timed-event mutation
  - timestamps for last sync or projection refresh

Schema surfaces:

- add `schemas/api/calendar-account-resource.schema.json`
- add `schemas/api/calendar-account-list.response.schema.json`

### New Calendar Inspection And Mutation Routes

- `GET /v1/calendar/events`
- `GET /v1/calendar/events/{eventId}`
- `POST /v1/calendar/availability/queries`
- `GET /v1/calendar/availability/queries/{queryId}`
- `POST /v1/calendar/events`
- `POST /v1/calendar/events/{eventId}/update`
- `POST /v1/calendar/events/{eventId}/cancel`

Request and response requirements:

- all calendar read and write routes accept an optional request-scoped `integrationId`
  selector; when omitted, the server resolves the canonical default account projection
  and returns the chosen `integrationId` in the resulting operation or resource
- event list and detail requests accept:
  - optional `integrationId`
  - optional time-window filters for list inspection
- availability query requests accept:
  - optional `integrationId`
  - `windowStart`
  - `windowEnd`
  - optional `timezone`
- create requests accept only phase-29-supported fields:
  - optional `integrationId`
  - `title`
  - `startsAt`
  - `endsAt`
  - optional `description`
  - optional `location`
  - optional `timezone`
- update requests accept:
  - optional `integrationId`
  - the same timed single-event field set as create
- update requests MUST reject:
  - recurring-event mutation
  - all-day-event mutation
  - attendee invitation or RSVP semantics
  - alternate-calendar selection
- cancel requests accept:
  - optional `integrationId`
  - optional `reason`
  - optional source linkage metadata if initiated from workflow or schedule context
- mutation responses return:
  - authoritative `operationId`
  - selected `integrationId`
  - `calendarRef`
  - `timezoneUsed`
  - resulting event artifact or truthful failure status

Schema surfaces:

- add `schemas/api/calendar-event-resource.schema.json`
- add `schemas/api/calendar-event-list.response.schema.json`
- add `schemas/api/create-calendar-availability-query.request.schema.json`
- add `schemas/api/calendar-availability-query-resource.schema.json`
- add `schemas/api/create-calendar-event.request.schema.json`
- add `schemas/api/update-calendar-event.request.schema.json`
- add `schemas/api/cancel-calendar-event.request.schema.json`
- add `schemas/api/calendar-operation-resource.schema.json`

### New Calendar Operation Inspection Routes

- `GET /v1/calendar/operations`
- `GET /v1/calendar/operations/{operationId}`

Request and response requirements:

- operation list responses support filtering by:
  - `integrationId`
  - `runId`
  - `workflowId`
  - `scheduleId`
  - `operationClass`
  - `status`
  - `externalEventId`
- operation detail returns:
  - account selection and calendar reference
  - timezone used
  - source linkage to run, workflow, schedule, and delivery truth when present
  - linked event artifacts or availability query result
  - failure class and reason when the operation did not complete

Schema surfaces:

- add `schemas/api/calendar-operation-list.response.schema.json`
- add `schemas/api/calendar-event-artifact.schema.json`

### Existing Runtime, Workflow, Schedule, And Delivery Surfaces Extended

- `GET /v1/runs/{runId}/tool-calls`
- `GET /v1/runs/{runId}/workflows/{workflowId}`
- `GET /v1/schedules/{scheduleId}`
- `GET /v1/deliveries`
- `GET /v1/deliveries/{deliveryId}`

Additive requirements:

- tool-call resources gain `calendarOperationSummaries`
- workflow-step resources gain `calendarOperationSummaries`
- schedule-attempt resources may expose latest calendar-operation summary for background
  calendar runs
- delivery outcome detail may include linked `calendarOperationIds` when the background
  result originated from calendar-domain work

Schema surfaces:

- update `schemas/api/tool-call-resource.schema.json`
- update `schemas/api/workflow-step-resource.schema.json`
- update `schemas/api/schedule-attempt-resource.schema.json`
- update `schemas/api/delivery-outcome-resource.schema.json`

## Event And History Surfaces

New calendar event families:

- `calendar.account_projected`
- `calendar.operation_requested`
- `calendar.operation_completed`
- `calendar.operation_failed`
- `calendar.artifact_recorded`

Event payload requirements:

- account projection truth:
  - `integrationId`
  - `accountKey`
  - `primaryCalendarRef`
  - `primaryTimezone`
  - `readinessStatus`
  - `canonicalDefault`
- operation truth:
  - `operationId`
  - `operationClass`
  - `integrationId`
  - `runId`
  - `workflowId`
  - `scheduleId`
  - `status`
  - `timezoneUsed`
  - `externalEventId` when present
  - `failureClass`
- artifact truth:
  - `artifactId`
  - `operationId`
  - `externalEventId`
  - `calendarRef`
  - `lifecycleState`

Schema surfaces:

- add `schemas/events/calendar-account-projected.event.schema.json`
- add `schemas/events/calendar-operation-requested.event.schema.json`
- add `schemas/events/calendar-operation-completed.event.schema.json`
- add `schemas/events/calendar-operation-failed.event.schema.json`
- add `schemas/events/calendar-artifact-recorded.event.schema.json`

## Persistence Surfaces

Persistence remains additive to the daemon-owned SQLite store:

- add a `calendar_accounts` table for account projection and primary-calendar metadata
- add a `calendar_operations` table for domain action truth and source linkage
- add a `calendar_artifacts` table for structured event snapshots and availability
  artifacts
- extend runtime tool-call and workflow-step documents with `calendarOperationSummaries`
- extend delivery-outcome documents or linkage indexes to reference originating calendar
  operations when present

Persistence rules:

- account projection, operation truth, and artifact snapshots are environment-scoped and
  durable across daemon restart
- `list_events`, `get_event`, `create_event`, `update_event`, and `cancel_event` store
  structured event artifacts only when backend event state was actually observed
- availability query results persist as operation-linked artifacts rather than mutating
  event state
- stored artifacts must not depend on live backend re-fetch to remain inspectable
- secret-bearing backend details remain outside calendar-domain documents and stay owned
  by the integrations plane

## Documentation Surfaces

Docs updated by implementation:

- `docs/runtime/daemon-roadmaps.md`
- `docs/runtime/daemon-api-and-event-model.md`
- `docs/runtime/operator-trust-model.md`
- `docs/harness/harness-architecture.md`
- downstream roadmap specs that depend on calendar semantics:
  - `docs/specs/014-calendar-integration.md`
  - `docs/specs/015-mail-integration.md`
  - `docs/specs/016-tasks-and-reminders.md`

## Truthfulness Constraints

- integration readiness, calendar execution, and delivery outcomes remain separate planes
- busy/free lookup must remain distinct from create, update, and cancel mutation truth
- phase 29 mutation applies only to timed single events on the primary calendar
- recurring events and all-day events may be inspected but not mutated
- attendee invitation, RSVP, and external notification semantics are out of scope
- timed-event writes use the bound calendar account's primary timezone by default
