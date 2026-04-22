# Quickstart: Scheduled Tasks Wakeups

## Goal

Verify in `DOPE_ENV=test` that the daemon can:

- create and inspect a one-time schedule before it fires
- launch normal run or workflow truth when the schedule becomes due
- create a recurring schedule, pause it, resume it, and inspect history
- preserve visible dispatch, retry, skipped, and missed outcomes without raw-log access

## Prerequisites

- local test daemon only; do not use `~/.dope`
- authenticated local pairing or an existing bearer token
- no production connectors or production secrets are required
- one deterministic run or workflow target that is safe in `DOPE_ENV=test`

## Suggested Verification Flow

1. Start the daemon in the test environment.

```bash
make daemon-run-test
```

2. Create a one-time schedule that launches a normal run.

```bash
FIRE_AT=$(date -u -v+1M +"%Y-%m-%dT%H:%M:%SZ")

curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"trigger\": {
      \"kind\": \"once\",
      \"fireAt\": \"$FIRE_AT\"
    },
    \"target\": {
      \"kind\": \"run\",
      \"run\": {
        \"entrypoint\": \"operator\",
        \"goal\": \"Record one deterministic schedule-dispatched run in test.\"
      }
    },
    \"retryPolicy\": {
      \"maxRetries\": 2,
      \"backoffKind\": \"fixed\",
      \"baseDelaySeconds\": 5,
      \"maxDelaySeconds\": 5
    }
  }" \
  http://127.0.0.1:19192/v1/schedules
```

Expected outcome after implementation:

- the response is a persisted schedule resource with `status` `scheduled`
- `nextDueAt` matches the one-time trigger
- the schedule includes a stable `targetRefId`
- no run or workflow exists yet

3. Inspect the schedule before its due time.

```bash
curl -sS -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/schedules/$SCHEDULE_ID
```

Expected outcome after implementation:

- status remains pending/scheduled
- attempt history is empty
- no downstream `runId` or `workflowId` is linked yet

4. Wait until the due time passes, then inspect the schedule again.

```bash
curl -sS -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/schedules/$SCHEDULE_ID
```

Expected outcome after implementation:

- the schedule shows one dispatch attempt with `dispatchStatus` `dispatched`
- the attempt links to a normal `runId`
- one-time schedule status becomes terminal rather than remaining pending

5. Inspect the downstream run created by the schedule.

```bash
curl -sS -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/runs/$RUN_ID
```

Expected outcome after implementation:

- the run exists through the normal run route
- the run includes additive `scheduleId` and `scheduleAttemptId` linkage
- downstream status changes remain normal run/workflow truth, not schedule-only state

6. Create a recurring schedule with explicit timezone semantics.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "trigger": {
      "kind": "cron",
      "cronExpr": "*/1 * * * *",
      "timezone": "UTC"
    },
    "target": {
      "kind": "run",
      "run": {
        "entrypoint": "operator",
        "goal": "Emit one recurring test run."
      }
    },
    "retryPolicy": {
      "maxRetries": 1,
      "backoffKind": "fixed",
      "baseDelaySeconds": 10,
      "maxDelaySeconds": 10
    }
  }' \
  http://127.0.0.1:19192/v1/schedules
```

Expected outcome after implementation:

- the recurring schedule is `active`
- the response shows `timezone` `UTC`
- `nextDueAt` is derived from the cron rule and stored timezone

7. Pause the recurring schedule before the next due time, then confirm no dispatch occurs.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/schedules/$RECURRING_ID/pause
```

Expected outcome after implementation:

- schedule status becomes `paused`
- any due-time evaluation while paused records a visible skipped/paused attempt instead of
  launching work

8. Resume the recurring schedule and confirm the next due time is recalculated.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/schedules/$RECURRING_ID/resume
```

Expected outcome after implementation:

- the recurring schedule returns to `active`
- `nextDueAt` advances from the stored timezone-aware cron calculation
- prior attempt history remains intact

9. Verify restart catch-up with automated regression coverage.

Expected outcome after implementation:

- persisted future schedules remain inspectable after daemon restart
- only the most recent overdue trigger is eligible for catch-up dispatch
- older overdue intervals are recorded as visible missed/skipped history

## Automated Verification

Run targeted suites plus contract coverage:

```bash
cd daemon && go test ./internal/scheduler ./internal/api ./internal/store ./internal/app ./internal/contracts
make daemon-contract-test
cd daemon && go test ./...
```

Expected automated coverage after implementation:

- one-time schedule creation and single dispatch truth
- recurring pause/resume with zero unintended dispatches while paused
- bounded retry/backoff for dispatch-side failures
- non-reentrant overlap behavior with visible skipped/blocked attempts
- restart recovery and bounded overdue catch-up behavior
- additive schedule linkage on run/workflow resources and event fixtures

## Observed Results

Validated on `2026-04-22` in `DOPE_ENV=test`.

Manual API smoke:

- Started the test daemon with `make daemon-run-test` and confirmed health with `make daemon-test-status`.
- Completed local pairing and used the issued bearer token for all calls.
- Created one-time schedule `sched_0e1a4f632a6dcf1b` with `fireAt` `2026-04-22T03:38:39Z`.
- Verified pre-fire inspection returned `status` `scheduled`, `nextDueAt` `2026-04-22T03:38:39Z`, and no attempts.
- Verified post-fire inspection returned `status` `completed` with one `dispatched` attempt linked to run `run_c613d1a83e877c47`.
- Verified `GET /v1/runs/run_c613d1a83e877c47` returned additive `scheduleId` and `scheduleAttemptId` linkage.
- Created recurring schedule `sched_98da1456844567c4`, verified initial `status` `active` and timezone-derived `nextDueAt`.
- Verified `pause` moved the recurring schedule to `paused` and `resume` returned it to `active` without losing the stored next due time.

Automated verification completed:

```bash
cd daemon && go test ./internal/api ./internal/scheduler ./internal/store ./internal/app ./internal/contracts
make daemon-contract-test
cd daemon && go test ./...
```

Observed automated coverage:

- one-time create, inspect, cancel, and exact-once dispatch
- one-time workflow-target dispatch with linked workflow completion and runtime step/tool-call truth
- recurring pause/resume, non-reentrant overlap skip, and bounded retry/exhausted truth
- environment-scoped schedule visibility and schedule-scoped event persistence
- restart catch-up that records missed intervals and dispatches only the latest overdue trigger
- timing assertions for create/detail latency, due detection, and catch-up evaluation

Residual notes:

- manual validation covered one-time run-target dispatch and recurring pause/resume commands; restart catch-up, retry exhaustion, and workflow-target dispatch were validated through automated suites.
- the current `~/.dope-test` skill set did not include an executable workflow consumer, so live manual smoke stayed on run targets while workflow-target execution remained covered by deterministic automated regression.

## Notes

- Keep all verification in `DOPE_ENV=test`.
- Use deterministic run/workflow targets that do not require live connectors.
- Manual verification should prove inspect-before-fire behavior, not only successful
  dispatch after the due time.
