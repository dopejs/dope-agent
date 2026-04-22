# Data Model: Delivery And Notifications

## Entities

### Delivery Target

- Purpose: Operator-visible destination that can receive routed results, alerts, or
  summaries for the current daemon owner within one environment.
- Fields:
  - `targetId`
  - `displayName`
  - `environmentScope`
  - `targetKind`: `connector_route`, `test_sink`, or future delivery kind
  - `status`: `active`, `disabled`, `unreachable`, or `misconfigured`
  - `connectorBinding`
  - `addressSummary`
  - `supportsImmediate`
  - `supportsDigest`
  - `lastValidatedAt`
  - `createdAt`
  - `updatedAt`
- Validation rules:
  - every target belongs to exactly one environment scope
  - target configuration is operator-visible, but secret-backed transport details remain
    redacted
  - `disabled` targets remain inspectable and may continue to appear in historical
    outcomes
  - connector-backed targets may reference existing connector identity, but they do not
    require an active foreground session

### Delivery Preference

- Purpose: Environment-scoped routing rule set that decides which single target should be
  used for a result class and whether the result is immediate, digest-eligible, or
  suppressed.
- Fields:
  - `preferenceId`
  - `environmentScope`
  - `scopeKind`: `user_default` or `integration_override`
  - `integrationId`
  - `preferredTargetsByClass`
    - `routine_success`
    - `urgent`
    - `failure`
  - `summaryPolicy`
  - `suppressionPolicy`
  - `active`
  - `createdAt`
  - `updatedAt`
- Validation rules:
  - one active user-default preference set exists per environment
  - integration overrides are optional and apply only to outcomes linked to that
    integration
  - each result class resolves to exactly one preferred target when delivery is enabled
  - digest policy may apply only to `routine_success` in phase 28

### Delivery Outcome Record

- Purpose: Durable operator-visible ledger entry for one routed background result after
  target selection and preference resolution.
- Fields:
  - `deliveryId`
  - `environmentScope`
  - `sourceKind`: `run`, `workflow`, `schedule_attempt`, or other future result source
  - `sourceId`
  - `runId`
  - `workflowId`
  - `scheduleId`
  - `scheduleAttemptId`
  - `integrationId`
  - `resultClass`: `routine_success`, `urgent`, or `failure`
  - `mode`: `immediate`, `digest`, or `suppressed`
  - `status`: `pending`, `queued`, `dispatching`, `delivered`, `suppressed`, `failed`
  - `chosenTargetId`
  - `preferenceId`
  - `summaryWindowId`
  - `payloadPreview`
  - `suppressionReason`
  - `createdAt`
  - `updatedAt`
  - `finalizedAt`
- Validation rules:
  - a delivery outcome remains separate from the source execution status and cannot
    overwrite run, workflow, schedule, or integration truth
  - each outcome binds to exactly one chosen target unless it is suppressed before any
    transport attempt
  - exhausted retries produce terminal `failed` status on the bound target
  - source linkage is immutable once the outcome is recorded

### Delivery Attempt

- Purpose: One concrete dispatch try for a delivery outcome against its chosen target.
- Fields:
  - `attemptId`
  - `deliveryId`
  - `attemptNumber`
  - `targetId`
  - `transportKind`
  - `status`: `running`, `delivered`, `retryable_failure`, `terminal_failure`
  - `failureClass`
  - `failureReason`
  - `nextRetryAt`
  - `connectorMessageDeliveryId`
  - `transportReceiptSummary`
  - `startedAt`
  - `completedAt`
- Validation rules:
  - attempts remain ordered and immutable once completed
  - retries stay bound to the original chosen target
  - `connectorMessageDeliveryId` is optional and only present for connector-backed
    attempts
  - terminal attempts must explain whether failure is retry exhaustion, target disablement,
    transport rejection, or another explicit class

### Summary Window

- Purpose: Environment-scoped grouping period that collects routine-success outcomes for
  later digest emission.
- Fields:
  - `summaryWindowId`
  - `environmentScope`
  - `targetId`
  - `preferenceId`
  - `status`: `open`, `ready`, `dispatching`, `delivered`, `failed`, `cancelled`
  - `windowStartedAt`
  - `windowEndsAt`
  - `resultCount`
  - `emittedDeliveryId`
  - `createdAt`
  - `updatedAt`
- Validation rules:
  - only routine-success outcomes may join a summary window in phase 28
  - failures and urgent results bypass summary windows entirely
  - a summary window may emit at most one digest delivery outcome

## State Transitions

### Delivery Outcome Lifecycle

- `pending` -> `suppressed` when policy disables delivery before target dispatch
- `pending` -> `queued` when a target and mode have been resolved
- `queued` -> `dispatching` when the first delivery attempt starts
- `dispatching` -> `delivered` when an attempt succeeds
- `dispatching` -> `queued` when an attempt fails but retry remains eligible
- `dispatching` -> `failed` when the final attempt reaches terminal failure
- `pending` -> `queued` with `mode=digest` when the outcome is assigned to a summary
  window rather than immediate dispatch

### Delivery Attempt Lifecycle

- `running` -> `delivered` on successful transport completion
- `running` -> `retryable_failure` when the chosen target remains eligible for another
  attempt
- `running` -> `terminal_failure` when retry budget is exhausted or the target becomes
  unusable for the selected outcome

### Summary Window Lifecycle

- `open` -> `ready` when the configured window closes with at least one eligible
  routine-success outcome
- `ready` -> `dispatching` when digest emission begins
- `dispatching` -> `delivered` when the digest outcome succeeds
- `dispatching` -> `failed` when digest emission exhausts retries
- `open` -> `cancelled` when all member outcomes are removed or policy disables summary
  delivery before emission

## Relationships

- one delivery target may be referenced by many preferences, outcomes, and attempts
- one delivery preference may drive many outcomes and summary windows
- one delivery outcome may have many attempts but only one chosen target
- one summary window may group many routine-success outcomes and emit one digest outcome
- one run, workflow, or schedule attempt may link to many delivery outcomes over time,
  though phase 28 expects the latest successful or failed user-facing result to dominate
  operator inspection

## Derived Views

- delivery target list views can show activation status, target kind, environment, and
  last validation time
- delivery preference views can show user-default routing beside integration overrides and
  the resolved target for each result class
- delivery outcome detail views can show source linkage, chosen target, mode, payload
  preview, and full attempt history without rereading raw connector logs
- run, workflow, and schedule-attempt views can project a latest-delivery summary while
  preserving execution truth separately
