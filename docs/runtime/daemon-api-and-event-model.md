# P0 Daemon API And Event Model

## Purpose

This document defines the first contract shape for the Dope daemon.

It focuses on:

- control-plane API style
- transport choices
- endpoint categories
- event envelope shape
- event taxonomy for P0

This is not a full route reference yet. It is the contract model that should guide the first daemon API implementation.

## Primary Recommendation

For P0, the daemon should expose:

- HTTP JSON APIs for commands and queries
- SSE for server-to-client event streaming

This means:

- request and response flows are plain HTTP
- client subscriptions are one-way event streams
- no WebSocket dependency is required for P0 operator surfaces

## Why SSE Instead Of WebSocket For P0

SSE is the better default for the first milestone because:

- the operator clients mostly need server-to-client updates
- it is simpler to debug than a bidirectional socket protocol
- it works well for dashboards, TUI subscriptions, and automation watchers
- it keeps the daemon control plane easier to reason about

WebSocket can still be added later if:

- we need richer live collaborative control
- we need browser-driven duplex workflows
- we want a single long-lived transport for advanced operator tooling

For P0, that extra protocol complexity is not justified yet.

## API Design Principles

- the daemon is the system of record
- all mutable state changes go through daemon APIs
- clients never mutate runtime truth locally
- events are append-oriented, not synthetic UI patches
- contracts should be schema-defined and versionable

## API Surface Categories

P0 should define the daemon API in six categories.

## 1. System API

Purpose:

- daemon liveness and metadata
- version
- health
- configuration summary

Examples:

- `GET /healthz`
- `GET /version`
- `GET /v1/system/info`

## 2. Session API

Purpose:

- inspect session state
- inspect routing and channel identity
- create or reset sessions when needed

Examples:

- `GET /v1/sessions`
- `GET /v1/sessions/{sessionId}`
- `POST /v1/sessions/{sessionId}/reset`

## 3. Run API

Purpose:

- create and inspect runs
- drive the runtime
- cancel and resume work
- inspect and control nested workflow plans that execute on the existing runtime plane

Examples:

- `POST /v1/runs`
- `GET /v1/runs`
- `GET /v1/runs/{runId}`
- `POST /v1/runs/{runId}/cancel`
- `POST /v1/runs/{runId}/resume`
- `POST /v1/runs/{runId}/workflows`
- `GET /v1/runs/{runId}/workflows`
- `GET /v1/runs/{runId}/workflows/{workflowId}`
- `POST /v1/runs/{runId}/workflows/{workflowId}/start`
- `POST /v1/runs/{runId}/workflows/{workflowId}/cancel`

## 4. Connector, Capability, And Integration API

Purpose:

- inspect connector state
- inspect capability state
- inspect daemon-owned integration state
- start, stop, or reload managed units when policy allows

Examples:

- `GET /v1/connectors`
- `GET /v1/connectors/{connectorId}`
- `POST /v1/connectors/{connectorId}/restart`
- `GET /v1/capabilities`
- `GET /v1/capabilities/{capabilityId}`
- `GET /v1/integrations`
- `GET /v1/integrations/{integrationId}`
- `POST /v1/integrations/{integrationId}/readiness`
- `POST /v1/integrations/{integrationId}/default`
- `POST /v1/runs/{runId}/integrations/{integrationId}/probes`

## 5. Delivery API

Purpose:

- inspect and configure daemon-owned delivery targets and preferences
- inspect routed delivery outcomes, attempt history, and summary windows
- project latest delivery linkage onto existing run, workflow, and schedule-attempt views

Examples:

- `POST /v1/delivery/targets`
- `GET /v1/delivery/targets`
- `POST /v1/delivery/preferences`
- `GET /v1/deliveries`
- `GET /v1/delivery/windows`

The delivery plane is additive:

- it does not replace foreground connector replies
- it does not redefine execution truth for runs, workflows, or schedules
- `latestDeliveryId`, `latestDeliveryStatus`, and `latestDeliveryTargetId` are lookup
  hints on source resources, not the authoritative delivery ledger

## 5.5 Calendar Domain API

Purpose:

- inspect daemon-owned calendar account projections
- inspect event detail and busy/free truth without collapsing them into mutation
- inspect truthful calendar operation and artifact history
- project additive calendar-operation linkage onto workflow, schedule, and delivery
  surfaces

Examples:

- `GET /v1/calendar/accounts`
- `GET /v1/calendar/events`
- `GET /v1/calendar/events/{eventId}`
- `POST /v1/calendar/availability/queries`
- `POST /v1/calendar/events`
- `POST /v1/calendar/events/{eventId}/update`
- `POST /v1/calendar/events/{eventId}/cancel`
- `GET /v1/calendar/operations`

The calendar domain is additive:

- readiness truth still belongs to `/v1/integrations`
- calendar operation truth does not replace workflow, schedule, or delivery truth
- workflow and schedule resources project `calendarOperationSummaries` as lookup aids,
  not as a second execution ledger
- delivery outcomes project `calendarOperationIds` and summaries additively; they do not
  redefine whether the underlying calendar action succeeded

## 5.6 Mail Domain API

Purpose:

- inspect daemon-owned mail account projections
- inspect truthful thread, message, and draft state without collapsing inspection into
  mutation
- execute draft create or update, direct send, send draft, reply, and forward while
  keeping outcome truth explicit
- project additive mail-operation linkage onto workflow, schedule, and delivery surfaces

Examples:

- `GET /v1/mail/accounts`
- `GET /v1/mail/threads`
- `GET /v1/mail/messages/{messageId}`
- `GET /v1/mail/drafts`
- `POST /v1/mail/drafts`
- `POST /v1/mail/messages/send`
- `GET /v1/mail/operations`

The mail domain is additive:

- readiness truth still belongs to `/v1/integrations`
- `resultMode` and `sendPath` distinguish draft-only, blocked, failed, and sent outcomes
- workflow and schedule resources project `mailOperationSummaries` as lookup aids, not as
  a second execution ledger
- delivery outcomes project additive `mailOperationIds` and summaries; they do not
  redefine whether the underlying mail action succeeded

## 5.7 Reminder Domain API

Purpose:

- inspect daemon-owned reminder resources distinct from raw schedules
- inspect explicit reminder occurrence lifecycle and append-only action history
- keep reminder lifecycle truth, workflow truth, and delivery truth separately visible
- project lightweight follow-up linkage to calendar, mail, run, or workflow source truth

Examples:

- `POST /v1/reminders`
- `GET /v1/reminders`
- `GET /v1/reminders/{reminderId}`
- `POST /v1/reminders/{reminderId}/acknowledge`
- `POST /v1/reminders/{reminderId}/snooze`
- `POST /v1/reminders/{reminderId}/complete`
- `POST /v1/reminders/{reminderId}/dismiss`
- `POST /v1/reminders/{reminderId}/reschedule`
- `POST /v1/reminders/{reminderId}/cancel`
- `GET /v1/reminders/{reminderId}/actions`
- `GET /v1/reminders/occurrences`
- `GET /v1/reminders/occurrences/{occurrenceId}`

The reminder domain is additive:

- recurring reminders reuse scheduler trigger semantics, but the reminder resource stays
  distinct from raw schedule resources
- recurring API requests currently use scheduler-native trigger kinds: `once` and `cron`
- successful reminder-triggered workflow launch auto-acknowledges the occurrence, but it
  does not auto-complete the reminder
- reminder occurrences project additive latest-delivery linkage, but `/v1/deliveries`
  remains the authoritative delivery ledger
- reminder follow-up links project source references and stale-source state; they do not
  copy calendar, mail, run, or workflow truth into the reminder resource

## 6. Config And Policy API

Purpose:

- inspect active config
- inspect policy decisions and pending approvals
- apply narrow config changes later

Examples:

- `GET /v1/config`
- `GET /v1/policy/approvals`
- `POST /v1/policy/approvals/{approvalId}/resolve`

## 7. Event Stream API

Purpose:

- stream daemon events to operator clients and automation

Examples:

- `GET /v1/events/stream`
- `GET /v1/runs/{runId}/events`
- `GET /v1/sessions/{sessionId}/events`

## API Shape Recommendation

Command endpoints should use explicit action routes when the action changes state meaningfully.

Examples:

- `POST /v1/runs/{runId}/cancel`
- `POST /v1/sessions/{sessionId}/reset`
- `POST /v1/connectors/{connectorId}/restart`

This is preferable to pretending these are plain resource updates when they are operational commands.

## Event Model Principles

- events should describe system facts, not UI rendering hints
- each event should be attributable to a system area
- event streams should support filtering by scope
- events should be safe to persist and replay

## Event Envelope

Every event should follow a common envelope shape.

Suggested fields:

- `eventId`
- `category`
- `name`
- `occurredAt`
- `scope`
- `resource`
- `payload`

Suggested example:

```json
{
  "eventId": "evt_01",
  "category": "run",
  "name": "run.created",
  "occurredAt": "2026-04-17T12:00:00Z",
  "scope": {
    "runId": "run_01",
    "sessionId": "sess_01"
  },
  "resource": {
    "kind": "run",
    "id": "run_01"
  },
  "payload": {
    "entrypoint": "chat",
    "goal": "help the user ship a task"
  }
}
```

## Event Categories

P0 should define at least these event categories.

## 1. System Events

Examples:

- `system.started`
- `system.stopping`
- `system.config_reloaded`
- `system.health_changed`

## Additional Integration Events

Examples:

- `integration.registered`
- `integration.updated`
- `integration.readiness_changed`
- `integration.default_changed`

## 2. Session Events

Examples:

- `session.created`
- `session.routed`
- `session.reset`
- `session.archived`

## 3. Run Events

Examples:

- `run.created`
- `run.started`
- `run.cancel_requested`
- `run.cancelled`
- `run.completed`
- `run.failed`

## 4. Step Events

Examples:

- `step.created`
- `step.status_changed`
- `step.blocked`
- `step.completed`
- `step.failed`

## 5. Delivery Events

Examples:

- `delivery.target_registered`
- `delivery.target_status_changed`
- `delivery.preference_updated`
- `delivery.outcome_created`
- `delivery.attempt_recorded`
- `delivery.outcome_status_changed`
- `delivery.summary_emitted`

Delivery events carry delivery truth only:

- retry, suppression, and terminal failure remain explicit facts
- connector transport receipts are linked as evidence, not promoted into the primary
  delivery resource model
- summary emission is separate from the original source outcome so operators can inspect
  digest membership and digest delivery independently

## Reminder Events

Examples:

- `reminder.created`
- `reminder.updated`
- `reminder.occurrence_created`
- `reminder.occurrence_transitioned`
- `reminder.workflow_launch_started`
- `reminder.workflow_launch_failed`
- `reminder.delivery_linked`

Reminder events carry reminder-domain truth only:

- overdue and missed remain explicit occurrence facts rather than delivery or workflow
  inferences
- successful workflow launch and workflow-start failure stay distinct from downstream
  workflow execution status
- delivery linkage events attach delivery evidence additively without redefining reminder
  lifecycle state

## 5. Policy Events

Examples:

- `policy.decision_recorded`
- `policy.approval_requested`
- `policy.approval_resolved`

## 6. Connector Events

Examples:

- `connector.started`
- `connector.disconnected`
- `connector.restart_scheduled`
- `connector.failed`

## 7. Capability Events

Examples:

- `capability.registered`
- `capability.unhealthy`
- `capability.restarted`

## 8. Artifact Events

Examples:

- `artifact.created`
- `artifact.updated`
- `artifact.attached`

## 9. Workflow Events

Examples:

- `workflow.planned`
- `workflow.started`
- `workflow.status_changed`
- `workflow.step_status_changed`

Workflow events stay additive to run and step history:

- workflow planning remains inspectable before execution begins
- workflow execution still creates normal runtime step and tool-call records
- workflow-step events can project `workflowStepId`, `runtimeStepId`, `toolCallId`, and
  `attempt` without creating a second execution ledger

## Event Stream Filtering

The event stream should support simple server-side filters such as:

- `category`
- `runId`
- `sessionId`
- `resourceKind`

This avoids forcing every client to subscribe to the full daemon event firehose.

## Transport Recommendation For P0

The recommended event transport is:

- `text/event-stream`

Suggested event format:

```text
event: run.created
data: {"eventId":"evt_01","category":"run","name":"run.created", ...}
```

This is enough for:

- Web UI live dashboards
- TUI tailing
- simple local automation

## Auth Recommendation For P0

P0 auth should remain simple and local-first.

Recommended starting point:

- local loopback bind by default
- token or pairing-based operator access
- no public-by-default exposure

This keeps the daemon easier to reason about while the API is still evolving.

## What We Are Not Doing Yet

P0 should not attempt:

- a large RPC framework
- GraphQL
- bidirectional WebSocket control as the primary protocol
- public remote multi-tenant API design
- connector-specific custom operator APIs outside the shared control surface

## How This Connects To The Repo

This document should directly guide:

- `schemas/api/`
- `schemas/events/`
- `daemon/internal/api/`
- `daemon/internal/events/`

It should also constrain:

- Web client API usage
- future TUI client contract design
- connector and capability supervision UX

## Immediate Next Steps

1. define the first API schemas for system and run routes
2. define the first event schemas for system, run, and step events
3. update the Go daemon server scaffold to reflect `/v1/...` route groups
4. define the run resource and step resource response shapes

## Short Version

P0 should use:

- HTTP JSON for commands and queries
- SSE for event streaming

The daemon API should be resource-oriented where possible and command-oriented where necessary.

The event model should be scoped, replayable, and stable enough for Web UI, TUI, and automation clients to share.
