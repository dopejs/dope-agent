# Quickstart: Calendar Integration

## Goal

Verify in `KURA_ENV=test` that the daemon can:

- project a calendar account from the shared integration substrate
- expose primary calendar and primary timezone truth
- inspect event lists, event detail, and busy/free windows without mutation side effects
- create, update, and cancel timed single events on the primary calendar
- reject recurring-event, all-day-event, attendee, and alternate-calendar mutation
  requests truthfully
- run calendar work through the existing workflow and delivery planes

## Prerequisites

- local test daemon only; do not use `~/.kura`
- authenticated local pairing or an existing bearer token
- no production connectors or live calendar credentials are required
- the repo-owned fake calendar backend is enabled through the fake integration path
- a `test_sink` delivery target exists if you want to validate background delivery reuse
- for a fully local background walkthrough, a workspace-local executable skill is enough;
  no external LLM provider is required

## Suggested Verification Flow

1. Start the daemon in the test environment.

```bash
make daemon-run-test
```

2. Register a fake calendar integration and mark it healthy.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "integrationId": "calendar-fake-primary",
    "domainKind": "calendar",
    "displayName": "Calendar Fake Primary",
    "backendKind": "fake_local",
    "backendRefId": "fake-calendar-primary",
    "backendDisplayName": "Fake Calendar Primary",
    "accountBinding": {
      "accountKey": "alice@example.com",
      "accountLabel": "Alice Calendar"
    },
    "canonicalDefault": true
  }' \
  http://127.0.0.1:19192/v1/integrations
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "readinessStatus": "healthy",
    "authState": "authorized",
    "healthState": "healthy",
    "secretResolution": "resolved"
  }' \
  http://127.0.0.1:19192/v1/integrations/calendar-fake-primary/readiness
```

Expected outcome after implementation:

- the integration is the canonical default for the fake calendar account
- readiness remains visible through `/v1/integrations`

3. Inspect the calendar account projection.

```bash
curl -sS \
  -H "Authorization: Bearer $KURA_TOKEN" \
  http://127.0.0.1:19192/v1/calendar/accounts
```

Expected outcome after implementation:

- the response includes `calendar-fake-primary`
- the account projection shows the primary calendar reference and primary timezone
- capability flags indicate timed single-event mutation support

4. Inspect event list and detail without mutating state.

```bash
curl -sS \
  -H "Authorization: Bearer $KURA_TOKEN" \
  "http://127.0.0.1:19192/v1/calendar/events?integrationId=calendar-fake-primary"
```

```bash
curl -sS \
  -H "Authorization: Bearer $KURA_TOKEN" \
  "http://127.0.0.1:19192/v1/calendar/events/$EVENT_ID?integrationId=calendar-fake-primary"
```

Expected outcome after implementation:

- both responses return calendar event resources and linked operation truth
- no create, update, or cancel mutation is recorded as a side effect

5. Run a busy/free lookup.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "integrationId": "calendar-fake-primary",
    "windowStart": "2026-04-23T09:00:00-07:00",
    "windowEnd": "2026-04-23T11:00:00-07:00"
  }' \
  http://127.0.0.1:19192/v1/calendar/availability/queries
```

Expected outcome after implementation:

- the response returns a distinct availability query resource
- the query uses the account's primary timezone by default when the request omits one
- no event mutation is recorded

6. Create, update, and cancel one timed event on the primary calendar.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "integrationId": "calendar-fake-primary",
    "title": "Phase 29 test event",
    "startsAt": "2026-04-23T13:00:00-07:00",
    "endsAt": "2026-04-23T13:30:00-07:00",
    "location": "Desk"
  }' \
  http://127.0.0.1:19192/v1/calendar/events
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Phase 29 moved event",
    "startsAt": "2026-04-23T14:00:00-07:00",
    "endsAt": "2026-04-23T14:30:00-07:00"
  }' \
  http://127.0.0.1:19192/v1/calendar/events/$EVENT_ID/update
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}' \
  http://127.0.0.1:19192/v1/calendar/events/$EVENT_ID/cancel
```

Expected outcome after implementation:

- all three responses share the same external event identity
- each response records a calendar operation and linked structured event artifact
- the write path is explicitly tied to the primary calendar and primary timezone

7. Confirm unsupported mutation paths fail truthfully.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "All day event",
    "startsAt": "2026-04-24",
    "endsAt": "2026-04-25",
    "allDay": true
  }' \
  http://127.0.0.1:19192/v1/calendar/events
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attendees": [{"email": "bob@example.com"}]
  }' \
  http://127.0.0.1:19192/v1/calendar/events/$EVENT_ID/update
```

Expected outcome after implementation:

- the system rejects all-day-event mutation
- the system rejects attendee semantics rather than silently dropping them
- analogous recurring-event and alternate-calendar mutation attempts should fail with the
  same explicit out-of-scope truth

8. Configure a test delivery target and preference if one does not already exist.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "targetId": "calendar-test-sink",
    "displayName": "Calendar Test Sink",
    "targetKind": "test_sink",
    "addressSummary": "local://calendar-test-sink"
  }' \
  http://127.0.0.1:19192/v1/delivery/targets
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "preferenceId": "calendar-default-pref",
    "scopeKind": "user_default",
    "preferredTargetsByClass": {
      "routine_success": "calendar-test-sink",
      "urgent": "calendar-test-sink",
      "failure": "calendar-test-sink"
    }
  }' \
  http://127.0.0.1:19192/v1/delivery/preferences
```

9. Run a scheduled workflow with an inline calendar action, then inspect both operation
and delivery truth.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "trigger": {
      "kind": "once",
      "fireAt": "$FIRE_AT_RFC3339"
    },
    "target": {
      "kind": "workflow",
      "workflow": {
        "entrypoint": "operator",
        "runGoal": "calendar manual background run",
        "workflowGoal": "Create a background calendar event.",
        "calendarAction": {
          "operationClass": "create_event",
          "integrationId": "calendar-fake-primary",
          "title": "Background linked calendar event",
          "startsAt": "2026-04-23T15:00:00-07:00",
          "endsAt": "2026-04-23T15:30:00-07:00"
        }
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

After the schedule completes, read the emitted `runId`, `workflowId`,
`scheduleAttemptId`, and `latestDeliveryId` from:

```bash
curl -sS \
  -H "Authorization: Bearer $KURA_TOKEN" \
  http://127.0.0.1:19192/v1/schedules/$SCHEDULE_ID
```

Expected outcome after implementation:

- background workflow steps record `calendarOperationSummaries`
- the resulting delivery outcome links back to the calendar operation
- operators can distinguish successful calendar execution from failed or delayed delivery

## Observed Results: 2026-04-23

Environment used for this run:

- `KURA_ENV=test`
- `KURA_DATA_DIR=/Users/John/Code/kura-agent/.tmp/calendar-manual`
- fake calendar integration `calendar-fake-primary`
- delivery target `calendar-test-sink`
- deterministic local skill `exec-skill`

Observed behavior:

- `/v1/calendar/accounts` returned one projection for `calendar-fake-primary` with
  `primaryCalendarRef=primary`, `primaryTimezone=America/Los_Angeles`, and all three
  phase-29 capability flags enabled.
- `GET /v1/calendar/events?integrationId=calendar-fake-primary` returned
  `selectionMode=explicit` and one seeded event `fake_event_seed`.
- `GET /v1/calendar/events` fell back to `selectionMode=canonical_default` against the
  same fake integration.
- `POST /v1/calendar/availability/queries` returned one `busy_free` operation with
  timezone defaulted to `America/Los_Angeles` and one busy interval matching the seeded
  event.
- create, update, and cancel all preserved the same external event ID
  `evt_calendar_fake_primary_1776909698013135000`.
- unsupported writes failed truthfully with HTTP `400`:
  `all-day-event mutation is out of scope for phase 29` and
  `attendee mutation semantics are out of scope for phase 29`.
- one scheduled workflow completed through the shared delivery plane with:
  `scheduleId=sched_f4a49888d88dea3f`,
  `workflowId=wf_d0a0858fa563`,
  `scheduleAttemptId=sched_attempt_2c6ab2ddac2bdf43`, and
  `deliveryId=delivery_0aac6d5063a852df`.
- the scheduled workflow's inline `calendarAction` created one calendar operation and the
  daemon projected:
  `calendarOperationSummaries` onto the workflow step and schedule attempt, and
  `calendarOperationIds` plus `calendarOperationSummaries` onto the delivery outcome.

Observed latency on local test hardware:

| Operation | Observed `time_total` |
|-----------|------------------------|
| `GET /v1/calendar/accounts` | `0.007726 s` |
| explicit event list | `0.008721 s` |
| canonical-default event list | `0.007580 s` |
| availability query | `0.006969 s` |
| create event | `0.005400 s` |
| update event | `0.003941 s` |
| cancel event | `0.004471 s` |
| background-linked create event | `0.007379 s` |
| workflow detail with projected summary | `0.007118 s` |
| schedule detail with projected summary | `0.006799 s` |
| delivery detail with projected linkage | `0.005656 s` |

Interpretation:

- the observed explicit read, canonical-default read, and busy/free calls stayed well
  below the `<=500 ms` local target
- event mutation and projection stayed well below the `<=1 s` local target
- background workflow plus shared-delivery projection stayed well below the `<=2 s`
  local target with the calendar write executing inside the scheduled workflow itself
- this manual background exercise validated the real workflow/schedule calendar execution
  path rather than a post-hoc linkage workaround; the automated Go regressions remain
  the authoritative end-to-end check for projection and delivery-linkage correctness

## Automated Verification

Run targeted suites plus contract coverage:

```bash
cd daemon && go test ./internal/calendar ./internal/api ./internal/store ./internal/app ./internal/runtime ./internal/orchestration ./internal/scheduler ./internal/integrations ./internal/delivery ./internal/policy ./internal/contracts
make daemon-contract-test
cd daemon && go test ./internal/api ./internal/store ./internal/contracts
```

Expected automated coverage after implementation:

- account projection and primary timezone exposure
- event list/detail inspection and busy/free separation
- timed single-event create/update/cancel truth with stable event identity
- truthful rejection of out-of-scope mutation requests
- background workflow and delivery linkage for calendar-domain work
