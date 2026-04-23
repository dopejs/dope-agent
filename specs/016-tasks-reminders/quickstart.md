# Quickstart: Tasks And Reminders

## Goal

Verify in `DOPE_ENV=test` that the daemon can:

- create one-time and recurring reminder resources distinct from raw schedules
- surface explicit reminder occurrence lifecycle truth
- route reminder notifications through the shared delivery plane
- preserve `overdue` versus `missed` truth across restart and recurring rollover
- launch background workflows from reminders without conflating reminder state with
  workflow state
- keep lightweight follow-up linkage to existing personal-domain work

## Prerequisites

- local test daemon only; do not use `~/.dope`
- authenticated local pairing or an existing bearer token
- no production connectors or live personal integrations are required
- a `test_sink` delivery target exists if you want to validate background reminder
  delivery
- examples below use `jq` plus BSD/macOS `date -v`; on Linux, substitute the equivalent
  `date -d` commands
- for the workflow-launch path, reuse a deterministic local entrypoint that already
  exists in the test environment; the roadmap 31 walkthrough used executable capability
  `exec-manual-r31`
- recurring reminders use scheduler-native `trigger.kind: "cron"` rather than a separate
  `"recurring"` API value

## Suggested Verification Flow

1. Start the daemon in the test environment.

```bash
make daemon-run-test
```

2. Configure a `test_sink` delivery target and preference if one does not already exist.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "targetId": "reminder-test-sink",
    "displayName": "Reminder Test Sink",
    "targetKind": "test_sink",
    "addressSummary": "local://reminder-test-sink"
  }' \
  http://127.0.0.1:19192/v1/delivery/targets
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "preferenceId": "reminder-default-pref",
    "scopeKind": "user_default",
    "preferredTargetsByClass": {
      "routine_success": "reminder-test-sink",
      "urgent": "reminder-test-sink",
      "failure": "reminder-test-sink"
    },
    "summaryPolicy": {
      "routineSuccessMode": "digest",
      "windowMinutes": 15
    },
    "active": true
  }' \
  http://127.0.0.1:19192/v1/delivery/preferences
```

3. Pick timestamps safely in the future before creating one-time reminders.

```bash
ONE_TIME_FIRE_AT=$(date -u -v+2M '+%Y-%m-%dT%H:%M:%SZ')
SNOOZE_UNTIL=$(date -u -v+5M '+%Y-%m-%dT%H:%M:%SZ')
RESCHEDULED_FIRE_AT=$(date -u -v+8M '+%Y-%m-%dT%H:%M:%SZ')
WORKFLOW_FIRE_AT=$(date -u -v+3M '+%Y-%m-%dT%H:%M:%SZ')
```

4. Create one one-time notification-only reminder.

```bash
CREATE_RESPONSE=$(curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Pay utility bill",
    "details": "Before 6pm today.",
    "behaviorMode": "notify_only",
    "trigger": {
      "kind": "once",
      "fireAt": "'"$ONE_TIME_FIRE_AT"'"
    }
  }' \
  http://127.0.0.1:19192/v1/reminders)
printf '%s\n' "$CREATE_RESPONSE"
REMINDER_ID=$(printf '%s' "$CREATE_RESPONSE" | jq -r '.reminderId')
```

Expected outcome after implementation:

- the response returns a reminder resource, not a raw schedule
- the reminder shows one configured trigger and no resolved occurrence yet

5. Inspect the reminder list and detail views.

```bash
curl -sS \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/reminders
```

```bash
curl -sS \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/reminders/$REMINDER_ID
```

Expected outcome after implementation:

- the reminder list shows `nextDueAt`, `behaviorMode`, and current reminder state
- the detail view shows trigger intent, active occurrence summary, and action history

6. Let the reminder become due and inspect occurrence and delivery truth.

```bash
OCCURRENCES_RESPONSE=$(curl -sS \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  "http://127.0.0.1:19192/v1/reminders/occurrences?reminderId=$REMINDER_ID")
printf '%s\n' "$OCCURRENCES_RESPONSE"
ACTIVE_OCCURRENCE_ID=$(printf '%s' "$OCCURRENCES_RESPONSE" | jq -r '.items[0].occurrenceId')
```

Expected outcome after implementation:

- one due occurrence exists for the reminder
- the occurrence shows separate latest-delivery linkage rather than treating delivery as
  reminder completion

7. Exercise acknowledge, snooze, complete, dismiss, and reschedule flows.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}' \
  http://127.0.0.1:19192/v1/reminders/$REMINDER_ID/acknowledge
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "snoozedUntil": "'"$SNOOZE_UNTIL"'"
  }' \
  http://127.0.0.1:19192/v1/reminders/$REMINDER_ID/snooze
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"occurrenceId":"'$ACTIVE_OCCURRENCE_ID'"}' \
  http://127.0.0.1:19192/v1/reminders/$REMINDER_ID/complete
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"occurrenceId":"'$ACTIVE_OCCURRENCE_ID'"}' \
  http://127.0.0.1:19192/v1/reminders/$REMINDER_ID/dismiss
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "trigger": {
      "kind": "once",
      "fireAt": "'"$RESCHEDULED_FIRE_AT"'"
    },
    "reason": "later today"
  }' \
  http://127.0.0.1:19192/v1/reminders/$REMINDER_ID/reschedule
```

Expected outcome after implementation:

- each command records a distinct reminder action record
- occurrence history remains inspectable after each transition
- `complete` and `dismiss` are terminal transitions, so use separate due reminders if
  you want to exercise both in one walkthrough

8. Create one recurring reminder and validate rollover semantics.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Review inbox triage",
    "behaviorMode": "notify_only",
    "trigger": {
      "kind": "cron",
      "cronExpr": "0 * * * *",
      "timezone": "UTC"
    }
  }' \
  http://127.0.0.1:19192/v1/reminders
```

Expected outcome after implementation:

- if one due occurrence remains unresolved when the next recurrence arrives, the prior
  occurrence becomes `missed` and a new active due occurrence is created
- if the prior occurrence was acknowledged, it remains acknowledged history and the new
  due occurrence is still created

9. Create one workflow-linked reminder and validate reminder/workflow separation.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Send morning summary",
    "behaviorMode": "launch_workflow",
    "trigger": {
      "kind": "once",
      "fireAt": "'"$WORKFLOW_FIRE_AT"'"
    },
    "workflowLaunchConfig": {
      "entrypoint": "exec-manual-r31",
      "runGoal": "Generate the morning summary",
      "workflowGoal": "Run the local summary workflow"
    }
  }' \
  http://127.0.0.1:19192/v1/reminders
```

Expected outcome after implementation:

- when workflow launch succeeds, the due occurrence auto-transitions to
  `acknowledged`
- the reminder does not auto-complete solely because the workflow started
- linked `runId` and `workflowId` remain inspectable from the occurrence
- if your test environment does not already expose `exec-manual-r31`, substitute a local
  executable capability or skill entrypoint that actually exists there

10. Validate workflow-launch failure truth.

Expected outcome after implementation:

- if the reminder-linked workflow fails to start, the occurrence remains `due` or later
  `overdue`
- workflow-launch failure remains operator-visible separately from reminder lifecycle
  truth and delivery truth

11. Create one lightweight follow-up reminder linked to existing calendar work.

Expected outcome after implementation:

- the reminder references existing calendar truth by stable source ID
- if the source later disappears, the reminder remains inspectable and marks the
  follow-up link stale rather than silently dropping it

## Automated Verification Results

Executed on 2026-04-23:

- `cd daemon && go mod tidy`
  - no module fallout; `go.mod` and `go.sum` were unchanged
- `cd daemon && go test ./internal/reminders ./internal/api ./internal/store ./internal/app ./internal/runtime ./internal/orchestration ./internal/scheduler ./internal/delivery ./internal/contracts ./internal/calendar ./internal/mail`
  - passed
- `cd daemon && go test ./internal/api -run 'TestReminderRoutesCreateInspectOccurrencesAndActions|TestReminderRoutesReuseDigestDeliveryPreference|TestReminderLifecycleRoutesAndWorkflowLinkage|TestScheduleWorkflowLauncherPersistsReminderLinkageOnRunsAndWorkflows|TestReminderRoutePerformanceSmoke' -count=1`
  - passed
- `cd daemon && go test ./internal/reminders -run 'TestManagerTickCreatesDueOccurrenceAndLinksDeliveryOutcome|TestManagerRecurringRemindersMarkMissedAndPreserveAcknowledgedHistory|TestManagerWorkflowLinkedReminderAcknowledgesOnSuccessAndStaysDueOnFailure|TestManagerRefreshesFollowUpLinkStaleness|TestManagerPerformanceSmoke' -count=1`
  - passed
- `make daemon-contract-test`
  - passed

Performance smoke observations:

- manager inspect latency for 100 reminders: `434.417µs`
- manager due detection tick for 100 reminders: `129.352875ms`
- manager occurrence projection: `16.042µs`
- manager acknowledge transition persistence: `329.917µs`
- reminder create route: `821.584µs`
- reminder list route: `63.333µs`
- reminder occurrence route: `25.334µs`

## Manual Verification Results

Primary daemon on `127.0.0.1:19192`:

- created delivery target `manual-target-r31-b` and preference `manual-pref-r31-b`
- created one-time notification reminder `rem_af1eaed57523b78a`
- after due time, occurrence `rem_occ_3906f2bc55225546` surfaced separately from
  delivery truth with linked delivery `delivery_e4272a3804c248ec`
- `GET /v1/deliveries?sourceKind=reminder_occurrence&sourceId=rem_occ_3906f2bc55225546`
  showed digest reuse with `mode: "digest"`, `preferenceId: "manual-pref-r31-b"`, and
  `summaryWindowId: "summary_window_7a6aa135c1b3695a"`
- `POST /v1/reminders/rem_af1eaed57523b78a/acknowledge` moved reminder and occurrence
  truth to `acknowledged` while preserving separate delivery linkage
- `POST /v1/reminders/rem_04fee8ef6c40f2e7/snooze` recorded a `snoozed` action and
  updated `nextDueAt`
- `POST /v1/reminders/rem_d10ee8aba45de1bf/reschedule` kept the reminder `pending` and
  moved `nextDueAt` from `2026-04-23T12:47:00Z` to `2026-04-23T12:49:00Z`
- `POST /v1/reminders/rem_5fa046ee00aaa910/complete` transitioned overdue occurrence
  `rem_occ_6d69abe193f953c1` to `completed`
- `POST /v1/reminders/rem_cf7f0ff8fb69397f/dismiss` transitioned overdue occurrence
  `rem_occ_b51815bf6203f6fd` to `dismissed`
- recurring reminder `rem_90b4044e64fa83a9` rolled prior unresolved occurrence
  `rem_occ_fbb22ba4c79f54f7` to `missed` and created new active occurrence
  `rem_occ_2bd1f59b7c5cd2be`
- recurring reminder `rem_40c3f4f853615e72` preserved acknowledged occurrence
  `rem_occ_8e26ee4b46cb4950` as history when the next recurrence created active overdue
  occurrence `rem_occ_3b2f5e29d922f37f`
- workflow-linked reminder `rem_e8022032ea7588ac` auto-acknowledged on launch and linked
  `run_2d58cb07ab888985` plus `wf_c9676f6f430f`
- run-linked follow-up reminder `rem_3e9b2e69543bd428` stayed inspectable with its typed
  run source reference
- calendar-linked follow-up reminder `rem_bcff1ce4a5e978c3` stayed inspectable and
  projected `stale: true` for the missing calendar operation

Isolated failure daemon on `127.0.0.1:19193` with empty home and data roots:

- reminder `rem_0b8eb3856e4e7ac2` became `due` and then `overdue`
- occurrence `rem_occ_6a409233ed40bc07` did not auto-acknowledge
- action history recorded `workflow_start_failed` with reason
  `workflow planning failed to start`

Manual notes:

- use `fireAt` values safely in the future during live testing; if the timestamp is too
  close to creation time, you may race reminder creation against due evaluation
- the normal test environment may contain executable home or data-root skills, which can
  make reminder-triggered workflow launch succeed more often than a clean-room daemon
