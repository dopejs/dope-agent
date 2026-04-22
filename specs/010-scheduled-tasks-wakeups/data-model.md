# Data Model: Scheduled Tasks Wakeups

## Entities

### Schedule Resource

- Purpose: Durable operator-managed trigger resource that defines when future work should
  be launched and what current schedule state is visible.
- Fields:
  - `scheduleId`
  - `environmentScope`
  - `kind`: `one_time` or `recurring`
  - `status`: `scheduled`, `active`, `paused`, `cancelled`, `completed`, or
    `dispatch_failed`
  - `targetRefId`
  - `trigger`
  - `timezone`
  - `nextDueAt`
  - `lastAttemptAt`
  - `lastOutcome`
  - `retryPolicy`
  - `createdAt`
  - `updatedAt`
  - `pausedAt`
  - `cancelledAt`
  - `completedAt`
  - `attempts[]`
- Validation rules:
  - every schedule belongs to exactly one daemon environment scope
  - one-time schedules use an absolute `fireAt` timestamp and transition to a terminal
    state after one dispatch success or terminal dispatch failure
  - recurring schedules require an explicit IANA timezone and a recurrence rule
  - `nextDueAt` is empty only for terminal or paused schedules
  - a schedule never launches overlapping concurrent executions for the same schedule in
    phase 25

### Schedule Trigger

- Purpose: Timing definition that determines when a schedule is due.
- Fields:
  - `triggerKind`: `once` or `cron`
  - `fireAt` for one-time schedules
  - `cronExpr` for recurring schedules
  - `timezone` for recurring schedules
  - `computedNextDueAt`
- Validation rules:
  - `fireAt` must be RFC3339 UTC or include an explicit offset
  - `cronExpr` must be valid for the chosen cron parser used by the daemon
  - recurring trigger evaluation always uses the stored IANA timezone, not host local time

### Schedule Target Reference

- Purpose: Stable reference that the scheduler resolves at dispatch time to create a run
  or workflow.
- Fields:
  - `targetRefId`
  - `scheduleId`
  - `targetKind`: `run` or `workflow`
  - `revision`
  - `active`
  - `document`
  - `updatedAt`
- Validation rules:
  - the reference is stable for the life of the schedule, while the referenced document
    may advance in revision
  - each dispatch attempt records which `revision` was resolved
  - if the current referenced target is inactive or invalid at dispatch time, dispatch
    fails without creating downstream execution truth

### Schedule Dispatch Attempt

- Purpose: Durable history record for a due-time evaluation, retry, skip, miss, or launch
  attempt.
- Fields:
  - `attemptId`
  - `scheduleId`
  - `dueAt`
  - `triggerSource`: `normal`, `catch_up`, or `retry`
  - `dispatchStatus`: `pending`, `dispatching`, `dispatched`, `failed`, `missed`,
    `skipped_paused`, `skipped_overlap`, `skipped_cancelled`, or `exhausted`
  - `failureClass`
  - `failureReason`
  - `retryCount`
  - `retryBudget`
  - `nextRetryAt`
  - `resolvedTargetRevision`
  - `runId`
  - `workflowId`
  - `downstreamStatus`: `none`, `running`, `completed`, `failed`, `cancelled`, or
    `interrupted`
  - `createdAt`
  - `updatedAt`
- Validation rules:
  - every due-time evaluation creates or updates exactly one attempt record
  - a dispatch attempt can link to a run and optionally a workflow only after dispatch
    succeeds
  - downstream failure is recorded separately from dispatch failure on the same attempt
  - exhausted retry state is terminal for that due interval, not necessarily for the whole
    recurring schedule

### Retry Policy

- Purpose: Operator-visible bounded retry configuration for dispatch-side failures.
- Fields:
  - `maxRetries`
  - `backoffKind`: `fixed` or `exponential`
  - `baseDelaySeconds`
  - `maxDelaySeconds`
- Validation rules:
  - retry policy applies only to dispatch-side failures
  - `maxRetries` must be bounded and non-negative
  - the next retry time must remain visible once a retry is scheduled

## State Transitions

### Schedule Lifecycle

- one-time `scheduled` -> `paused` when the operator pauses before dispatch
- recurring `active` -> `paused` when the operator pauses before the next due time
- `paused` -> `active` or `scheduled` when the operator resumes and the daemon computes a
  new `nextDueAt`
- one-time `scheduled` -> `completed` when dispatch succeeds
- one-time `scheduled` -> `dispatch_failed` when dispatch-side retries exhaust without a
  launch
- recurring `active` remains `active` across successful dispatches, skipped overlaps, and
  exhausted per-interval retries while `nextDueAt` advances
- `scheduled`, `active`, or `paused` -> `cancelled` when the operator cancels the schedule

### Dispatch Attempt Lifecycle

- `pending` -> `dispatching` when the scheduler claims a due interval for evaluation
- `dispatching` -> `dispatched` when a run or workflow is created successfully
- `dispatching` -> `failed` when launch target resolution or launch creation fails
- `failed` -> `pending` on a future retry attempt while retry budget remains
- `failed` -> `exhausted` when retry budget is consumed for that due interval
- `pending` -> `missed` when a due interval is older than the bounded restart catch-up
  window
- `pending` -> `skipped_paused`, `skipped_overlap`, or `skipped_cancelled` when the
  schedule is not eligible to launch at that due moment
- `dispatched` updates `downstreamStatus` from `running` to `completed`, `failed`,
  `cancelled`, or `interrupted` as linked execution truth changes later

## Relationships

- one schedule owns one trigger definition and one stable target reference
- one schedule has many dispatch attempts over time
- one dispatch attempt may create one run and optionally one workflow
- one run or workflow may point back to exactly one originating schedule attempt in this
  roadmap slice

## Derived Views

- `GET /v1/schedules` lists top-level schedule state, next due time, target summary, and
  most recent outcome for operator triage
- `GET /v1/schedules/{scheduleId}` expands trigger definition, retry policy, current
  target reference metadata, and recent dispatch attempts
- linked run/workflow resources expose additive `scheduleId` and `scheduleAttemptId`
  fields so operators can pivot from downstream execution truth back to the originating
  schedule
- schedule events plus persisted attempt history reconstruct missed intervals, overlap
  skips, retry exhaustion, dispatch failure, and downstream failure without raw logs
