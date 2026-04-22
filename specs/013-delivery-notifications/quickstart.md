# Quickstart: Delivery And Notifications

## Goal

Verify in `DOPE_ENV=test` that the daemon can:

- register and inspect delivery targets that are separate from the immediate request
  channel
- resolve user-default delivery preferences and optional integration-specific overrides
- emit a background result through the shared delivery plane without an active foreground
  session
- preserve per-attempt delivery history, suppression truth, and terminal failure on the
  chosen target without automatic failover
- batch routine-success results into one digest while urgent or failed results still
  deliver immediately

## Prerequisites

- local test daemon only; do not use `~/.dope`
- authenticated local pairing or an existing bearer token
- no production connectors are required
- the repo-owned `test_sink` delivery target kind is enabled in the test environment
- if you want to verify an integration-specific override, first create or restore the
  fake integration `calendar-fake-b` using
  `specs/012-personal-integrations-platform/quickstart.md` so the override has a real
  integration binding to target

## Suggested Verification Flow

1. Start the daemon in the test environment.

```bash
make daemon-run-test
```

2. Create one default local delivery target for general background results.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "targetId": "test-sink-default",
    "displayName": "Default Test Sink",
    "targetKind": "test_sink"
  }' \
  http://127.0.0.1:19192/v1/delivery/targets
```

Expected outcome after implementation:

- the response returns an active delivery target resource
- the target is explicitly environment-scoped
- the target remains inspectable even without an active chat session

3. Configure user-default delivery preferences.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "preferenceId": "user-default-test",
    "environmentScope": "test",
    "scopeKind": "user_default",
    "preferredTargetsByClass": {
      "routine_success": "test-sink-default",
      "urgent": "test-sink-default",
      "failure": "test-sink-default"
    },
    "summaryPolicy": {
      "routineSuccessMode": "immediate"
    }
  }' \
  http://127.0.0.1:19192/v1/delivery/preferences
```

Expected outcome after implementation:

- the user-default preference set becomes active
- each result class resolves to exactly one target
- no broadcast or multi-target routing is implied

4. Create a one-time schedule that launches a normal background run.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "trigger": {
      "kind": "one_time",
      "at": "2026-04-22T12:00:00Z"
    },
    "target": {
      "kind": "run",
      "run": {
        "entrypoint": "operator",
        "goal": "Produce a background result for delivery verification."
      }
    },
    "retryPolicy": {
      "maxRetries": 0,
      "backoffKind": "fixed",
      "baseDelaySeconds": 5,
      "maxDelaySeconds": 5
    }
  }' \
  http://127.0.0.1:19192/v1/schedules
```

Expected outcome after implementation:

- the schedule is created through the existing phase 25 trigger plane
- once the background run reaches a terminal result, the delivery plane creates a linked
  delivery outcome without requiring an active foreground request

5. Inspect delivery outcomes after the scheduled work completes.

```bash
curl -sS \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/deliveries
```

```bash
curl -sS \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/deliveries/$DELIVERY_ID
```

Expected outcome after implementation:

- the outcome shows `sourceKind`, source linkage, chosen target, mode, and final status
- successful delivery records per-attempt history even when only one attempt occurred
- source execution truth remains separate and unchanged on the linked run or workflow

6. Register a second target and use an integration-specific override.

If `calendar-fake-b` does not already exist in your test environment, create it first
using `specs/012-personal-integrations-platform/quickstart.md` before applying the
override below.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "targetId": "test-sink-integration",
    "displayName": "Integration Override Sink",
    "targetKind": "test_sink"
  }' \
  http://127.0.0.1:19192/v1/delivery/targets
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "preferenceId": "calendar-override-test",
    "environmentScope": "test",
    "scopeKind": "integration_override",
    "integrationId": "calendar-fake-b",
    "preferredTargetsByClass": {
      "routine_success": "test-sink-integration",
      "urgent": "test-sink-integration",
      "failure": "test-sink-integration"
    },
    "summaryPolicy": {
      "routineSuccessMode": "immediate"
    }
  }' \
  http://127.0.0.1:19192/v1/delivery/preferences
```

Expected outcome after implementation:

- outcomes linked to `calendar-fake-b` resolve to the integration override
- unrelated background results continue using the user-default target

7. Force retry behavior and confirm no automatic failover.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}' \
  http://127.0.0.1:19192/v1/delivery/targets/test-sink-default/disable
```

Expected outcome after implementation:

- new outcomes targeting `test-sink-default` record retryable and then terminal failure
  attempts when policy allows retries
- the failed outcome remains bound to `test-sink-default`
- the system does not silently reroute the same result to `test-sink-integration`

8. Enable digest behavior for routine-success outcomes and verify summary emission.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "preferenceId": "user-default-test",
    "environmentScope": "test",
    "scopeKind": "user_default",
    "preferredTargetsByClass": {
      "routine_success": "test-sink-default",
      "urgent": "test-sink-default",
      "failure": "test-sink-default"
    },
    "summaryPolicy": {
      "routineSuccessMode": "digest",
      "windowMinutes": 15
    }
  }' \
  http://127.0.0.1:19192/v1/delivery/preferences
```

Expected outcome after implementation:

- multiple routine-success outcomes created inside the configured window join one summary
  window
- `GET /v1/delivery/windows` and `GET /v1/delivery/windows/{summaryWindowId}` expose the
  grouped result count and emitted digest linkage
- a failure or urgent result created during the same period bypasses the digest path and
  still routes immediately

## Automated Verification

Run targeted suites plus contract coverage:

```bash
cd daemon && go test ./internal/delivery -count=1
cd daemon && go test ./internal/api -run 'TestRunRoutesProjectLatestDeliverySummaryWithoutForegroundRegression|TestRunDeliveryUsesIntegrationOverrideTarget|TestWorkflowRoutesProjectLatestDeliverySummary|TestWorkflowDeliveryUsesIntegrationOverrideTarget|TestScheduleRoutesDispatchWorkflowTargetAndLinkWorkflowTruth|TestScheduleRoutesProjectLatestDeliverySummaryOntoAttempts|TestDeliveryRoutesExposeTargetsPreferencesSuppressionAndEvents' -count=1
cd daemon && go test ./internal/app ./internal/store -run 'TestAppRunRestoresPendingDeliveryLifecycle|TestSQLiteStoreDeliveryResourcesRemainEnvironmentScoped' -count=1
cd daemon && go test ./internal/delivery ./internal/api ./internal/store ./internal/app ./internal/runtime ./internal/orchestration ./internal/scheduler ./internal/integrations ./internal/policy ./internal/contracts
make daemon-contract-test
```

Expected automated coverage after implementation:

- target registration, activation, and inspection
- user-default routing plus integration override selection for both run-backed and workflow-backed outcomes
- background result delivery from schedules, scheduled workflows, and workflows
- per-attempt retry history, suppression, and terminal failure without failover
- digest grouping for routine-success outcomes only
- restart-safe restoration of delivery attempts and summary windows

## Recorded Manual Results

Observed on 2026-04-22 in `DOPE_ENV=test` against `http://127.0.0.1:19192`:

- created target `manual-test-sink` and preference `manual-default-pref`
- created one-time schedule `sched_20712ee47fcf87d5`
- schedule dispatch created run `run_0a04aa993a3657d1` and schedule attempt
  `sched_attempt_be1ebd4ebb00e596`
- after the run reached `completed`, the source delivery outcome
  `delivery_9e3c576d1c3c1de3` was created with:
  - `mode=digest`
  - `status=queued`
  - `latestDeliveryStatus=queued` on the run resource
- summary window `summary_window_584fc46ff405b9ec` later reached `delivered`
- emitted digest delivery `delivery_daae990f9020dcbc` reached `delivered` with one
  `test_sink` attempt and receipt summary `stored in repo-owned test sink`

Manual walkthrough conclusions:

- background schedule-triggered results route into the daemon-owned delivery plane
- routine-success outcomes can queue into a summary window without mutating run truth
- digest emission creates a second operator-visible delivery outcome linked back to the
  summary window
