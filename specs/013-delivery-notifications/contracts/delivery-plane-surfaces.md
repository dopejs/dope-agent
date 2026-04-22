# Contract Surfaces: Delivery And Notifications

## Goal

Add a daemon-owned delivery plane for background results, alerts, and summaries without
collapsing delivery truth into connector transport records or execution status.

## HTTP API Surfaces

### New Delivery Target Routes

- `POST /v1/delivery/targets`
- `GET /v1/delivery/targets`
- `GET /v1/delivery/targets/{targetId}`
- `POST /v1/delivery/targets/{targetId}/activate`
- `POST /v1/delivery/targets/{targetId}/disable`

Request and response requirements:

- delivery target creation accepts:
  - `targetId`
  - `displayName`
  - `targetKind`
  - environment-scoped binding information:
    - connector route identity for connector-backed targets, or
    - repo-owned local sink configuration for `test_sink`
  - capability summary such as immediate or digest support
- target resource responses return:
  - `targetId`
  - `displayName`
  - `environmentScope`
  - `targetKind`
  - activation or health status
  - connector or local binding summary
  - capability flags
  - validation timestamps
- `activate` and `disable` are explicit command routes so target availability remains
  operator-visible and auditable

Schema surfaces:

- add `schemas/api/create-delivery-target.request.schema.json`
- add `schemas/api/delivery-target-resource.schema.json`
- add `schemas/api/delivery-target-list.response.schema.json`
- add `schemas/api/update-delivery-target-status.request.schema.json` if activate and
  disable share one action schema

### New Delivery Preference Routes

- `POST /v1/delivery/preferences`
- `GET /v1/delivery/preferences`
- `GET /v1/delivery/preferences/{preferenceId}`

Request and response requirements:

- preference creation or update accepts:
  - `preferenceId`
  - `environmentScope`
  - `scopeKind`: `user_default` or `integration_override`
  - optional `integrationId`
  - preferred target selection for `routine_success`, `urgent`, and `failure`
  - digest policy for routine-success outcomes
  - suppression settings if any
- preference resources return:
  - scope metadata
  - resolved preferred targets by result class
  - digest policy
  - suppression policy
  - active flag and timestamps

Schema surfaces:

- add `schemas/api/upsert-delivery-preference.request.schema.json`
- add `schemas/api/delivery-preference-resource.schema.json`
- add `schemas/api/delivery-preference-list.response.schema.json`

### New Delivery Outcome Inspection Routes

- `GET /v1/deliveries`
- `GET /v1/deliveries/{deliveryId}`
- `GET /v1/delivery/windows`
- `GET /v1/delivery/windows/{summaryWindowId}`

Request and response requirements:

- delivery outcome list responses support filtering by:
  - `sourceKind`
  - `sourceId`
  - `runId`
  - `workflowId`
  - `scheduleId`
  - `integrationId`
  - `status`
  - `targetId`
- delivery outcome detail returns:
  - source linkage
  - chosen target and preference identifiers
  - result class and mode
  - payload preview
  - suppression reason when applicable
  - per-attempt history including transport receipts
  - final outcome timestamps
- summary window resources return:
  - target and preference linkage
  - window boundaries
  - member count
  - emitted digest outcome linkage when present

Schema surfaces:

- add `schemas/api/delivery-outcome-resource.schema.json`
- add `schemas/api/delivery-outcome-list.response.schema.json`
- add `schemas/api/delivery-attempt-resource.schema.json`
- add `schemas/api/delivery-summary-window.resource.schema.json`
- add `schemas/api/delivery-summary-window-list.response.schema.json`

### Existing Source Surfaces Extended

- `GET /v1/runs`
- `GET /v1/runs/{runId}`
- `GET /v1/runs/{runId}/workflows/{workflowId}`
- `GET /v1/schedules/{scheduleId}`

Additive requirements:

- run resources gain latest delivery summary fields:
  - `latestDeliveryId`
  - `latestDeliveryStatus`
  - `latestDeliveryTargetId`
- workflow resources gain the same latest delivery summary when the workflow emits a
  routed result
- schedule-attempt resources gain latest delivery summary when the dispatched background
  result creates a delivery outcome
- these summary fields are additive lookup aids only and MUST NOT replace authoritative
  delivery outcome resources

Schema surfaces:

- update `schemas/api/run-resource.schema.json`
- update `schemas/api/run-list.response.schema.json` if run summaries change
- update `schemas/api/workflow-resource.schema.json`
- update `schemas/api/schedule-attempt-resource.schema.json`

### Existing Connector Message Surfaces Reused As Transport Evidence

- existing connector outbound message persistence remains the transport-specific record
  for connector-backed attempts
- delivery attempt detail may include `connectorMessageDeliveryId` so operators can
  inspect linked connector receipts when relevant
- no connector-only route becomes the primary delivery inspection surface

## Event And History Surfaces

New delivery event families:

- `delivery.target_registered`
- `delivery.target_status_changed`
- `delivery.preference_updated`
- `delivery.outcome_created`
- `delivery.attempt_recorded`
- `delivery.outcome_status_changed`
- `delivery.summary_emitted`

Event payload requirements:

- target identity:
  - `targetId`
  - `targetKind`
  - `environmentScope`
- source linkage when relevant:
  - `sourceKind`
  - `sourceId`
  - `runId`
  - `workflowId`
  - `scheduleId`
  - `integrationId`
- outcome truth:
  - `deliveryId`
  - `resultClass`
  - `mode`
  - `status`
  - `chosenTargetId`
  - `suppressionReason`
- attempt truth:
  - `attemptId`
  - `attemptNumber`
  - `transportKind`
  - `failureClass`
  - `nextRetryAt`
  - `connectorMessageDeliveryId` when applicable
- summary truth:
  - `summaryWindowId`
  - `resultCount`
  - `emittedDeliveryId`

Schema surfaces:

- add `schemas/events/delivery-target-registered.event.schema.json`
- add `schemas/events/delivery-target-status-changed.event.schema.json`
- add `schemas/events/delivery-preference-updated.event.schema.json`
- add `schemas/events/delivery-outcome-created.event.schema.json`
- add `schemas/events/delivery-attempt-recorded.event.schema.json`
- add `schemas/events/delivery-outcome-status-changed.event.schema.json`
- add `schemas/events/delivery-summary-emitted.event.schema.json`

Truthfulness rules:

- delivery failure must remain distinguishable from source execution failure
- retry, suppression, and terminal failure must be explicit operator-visible facts
- per-attempt history must survive restart and must not be reconstructed only from raw
  transport logs
- automatic failover to a secondary target is out of scope for phase 28
- failures and urgent results bypass digest windows even when routine-success summaries
  are enabled

## Persistence Surfaces

Persistence remains additive to the daemon-owned SQLite store:

- add a `delivery_targets` table for target identity, status, and binding summaries
- add a `delivery_preferences` table for user defaults and integration overrides
- add a `delivery_outcomes` table for routed result truth
- add a `delivery_attempts` table for per-attempt history and retry metadata
- add a `delivery_summary_windows` table for digest grouping
- add additive latest-delivery summary fields or lookup support for `runs`, `workflows`,
  and schedule-attempt documents

Persistence rules:

- delivery resources are environment-scoped and durable across daemon restart
- delivery attempts remain ordered under one outcome and do not migrate to a secondary
  target automatically
- connector-backed transport evidence links to existing `connector_messages` rows instead
  of duplicating connector-delivery payloads in a second connector table
- secret-bearing transport configuration remains redacted in operator-visible documents

## Documentation Surfaces

Docs updated by implementation:

- `docs/runtime/daemon-roadmaps.md`
- `docs/runtime/daemon-api-and-event-model.md`
- `docs/runtime/operator-trust-model.md`
- `docs/channels/channel-reply-progression.md`
- connector-specific channel docs if a connector-backed target adapter is implemented
- downstream roadmap specs that depend on the delivery plane:
  - `docs/specs/014-calendar-integration.md`
  - `docs/specs/015-mail-integration.md`
  - `docs/specs/016-tasks-and-reminders.md`

## Truthfulness Constraints

- delivery is a separate daemon-owned plane, not a synonym for connector chat replies
- each routed result binds to exactly one chosen target in phase 28
- retries remain target-local and do not automatically fail over
- delivery state must not redefine run, workflow, schedule, approval, or integration
  readiness truth
- existing foreground reply flows remain backward compatible and additive
