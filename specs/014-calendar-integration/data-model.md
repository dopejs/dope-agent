# Data Model: Calendar Integration

## Entities

### Calendar Account Projection

- Purpose: Calendar-domain view of the selected integration-backed account, including the
  writable primary calendar and primary timezone used by phase 29.
- Fields:
  - `calendarAccountId`
  - `integrationId`
  - `domainKind`: always `calendar`
  - `environmentScope`
  - `accountKey`
  - `accountLabel`
  - `readinessStatus`
  - `canonicalDefault`
  - `primaryCalendarRef`
  - `primaryCalendarLabel`
  - `primaryTimezone`
  - `supportsEventInspection`
  - `supportsBusyFree`
  - `supportsTimedMutation`
  - `lastSyncedAt`
  - `updatedAt`
- Validation rules:
  - every calendar account projection belongs to exactly one integration resource and one
    environment scope
  - the projection may exist only for calendar-domain integrations
  - the projection must surface the bound account's primary calendar and primary timezone
    explicitly
  - timed-event mutation is allowed only when the projection reports writable timed
    mutation support

### Calendar Operation Record

- Purpose: Operator-visible record for one calendar-domain action, including account
  selection, operation class, event identity when present, source linkage, and terminal
  truth.
- Fields:
  - `operationId`
  - `operationClass`: `list_events`, `get_event`, `busy_free`, `create_event`,
    `update_event`, or `cancel_event`
  - `integrationId`
  - `calendarAccountId`
  - `environmentScope`
  - `calendarRef`: phase 29 expects `primary`
  - `timezoneUsed`
  - `requestSummary`
  - `status`: `requested`, `completed`, `failed`, `blocked`, or `cancelled`
  - `failureClass`
  - `failureReason`
  - `runId`
  - `stepId`
  - `workflowId`
  - `workflowStepId`
  - `scheduleId`
  - `scheduleAttemptId`
  - `deliveryId`
  - `eventArtifactIds`
  - `createdAt`
  - `completedAt`
  - `updatedAt`
- Validation rules:
  - every record captures exactly one operation class
  - `busy_free` operations must not carry mutation success states or mutated event counts
  - `list_events` and `get_event` operations record event artifacts only when backend
    event state was actually observed
  - create, update, and cancel operations must record an event artifact when a durable
    event identity exists
  - operations blocked before any backend state is observed may remain artifact-free
  - operations remain valid even when downstream delivery fails later

### Calendar Event Artifact

- Purpose: Structured snapshot of a calendar event as observed or mutated by one calendar
  operation.
- Fields:
  - `artifactId`
  - `operationId`
  - `integrationId`
  - `environmentScope`
  - `externalEventId`
  - `calendarRef`
  - `title`
  - `startsAt`
  - `endsAt`
  - `timezone`
  - `allDay`
  - `recurrenceSummary`
  - `mutationEligibleInPhase`
  - `lifecycleState`: `active`, `cancelled`, `stale_snapshot`, or `unavailable`
  - `createdAt`
- Validation rules:
  - `mutationEligibleInPhase` is true only for timed single events on the primary
    calendar
  - all-day and recurring events may be represented as artifacts but are not mutation
    eligible in phase 29
  - artifact identity must remain stable enough to correlate create, update, and cancel
    operations on the same external event

### Availability Query Record

- Purpose: Structured result for busy/free inspection without implying any event
  mutation.
- Fields:
  - `queryId`
  - `operationId`
  - `integrationId`
  - `calendarAccountId`
  - `windowStart`
  - `windowEnd`
  - `timezone`
  - `busyIntervals`
  - `conflictCount`
  - `resultSummary`
  - `createdAt`
- Validation rules:
  - one availability query belongs to exactly one `busy_free` operation
  - query results must not create or mutate a calendar event
  - the query timezone defaults to the bound account's primary timezone when not provided

### Calendar Operation Summary

- Purpose: Additive lightweight projection attached to runtime tool calls or workflow
  steps so operators can locate calendar-domain truth from execution records.
- Fields:
  - `operationId`
  - `operationClass`
  - `integrationId`
  - `externalEventId`
  - `status`
  - `timezoneUsed`
  - `capturedAt`
- Validation rules:
  - summaries are immutable snapshots once attached to a tool call or workflow step
  - summaries must never replace the authoritative calendar operation record

## State Transitions

### Calendar Operation Lifecycle

- `requested` -> `completed` when the domain action finishes successfully and records its
  resulting artifacts or availability summary
- `requested` -> `blocked` when integration readiness or policy prevents execution before
  the backend action runs
- `requested` -> `failed` when the backend returns stale-state, conflict, unavailable, or
  other terminal failure truth
- `requested` -> `cancelled` when the enclosing workflow or run is cancelled before the
  action finishes

### Calendar Event Artifact Lifecycle

- `active` remains the default for inspected, created, or updated timed events
- `active` -> `cancelled` when a successful cancel operation is recorded
- any state -> `stale_snapshot` when the domain captures historical event truth that no
  longer matches current backend state
- any state -> `unavailable` when event detail cannot be refreshed because account
  readiness or backend access is lost

## Relationships

- one integration resource may back one current calendar account projection per
  environment and many calendar operations over time
- one calendar account projection may be referenced by many calendar operations
- one calendar operation may own zero or more calendar event artifacts
- one `busy_free` calendar operation owns exactly one availability query record
- one run, tool call, workflow step, or schedule attempt may reference many calendar
  operations over time through additive summary projections

## Derived Views

- calendar account views combine integration readiness with primary calendar and primary
  timezone truth
- calendar operation list views can filter by `integrationId`, `runId`, `workflowId`,
  `scheduleId`, `operationClass`, `status`, or `externalEventId`
- workflow or tool-call detail views can project calendar-operation summaries beside
  existing `integrationBindings`
