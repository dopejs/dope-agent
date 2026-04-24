# Quickstart: Operator Shell And Onboarding

## Goal

Verify in `DOPE_ENV=test` that the primary web shell can:

- show the active environment and current onboarding status
- project daemon-owned readiness items and optional follow-up setup
- handle approvals directly from the shell
- show recent operator activity and diagnostics from daemon truth
- run one bounded first useful action and show immediate result or status feedback

## Prerequisites

- local test daemon only; do not use `~/.dope`
- authenticated local pairing or an existing bearer token in `DOPE_TOKEN`
- Node dependencies installed for the repo workspace
- no live connectors or production secrets are required
- the shell uses `web/` as the primary operator surface in this roadmap
- the seeding `curl` commands below are for developer test setup only; the operator path
  under validation is the web shell

## Suggested Verification Flow

1. Start the daemon in the test environment.

```bash
make daemon-run-test
```

2. Pair or reuse a local bearer token.

```bash
make daemon-test-status
```

3. Seed one degraded integration so onboarding and diagnostics have non-empty readiness
   truth.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "integrationId": "calendar-shell",
    "domainKind": "calendar",
    "displayName": "Shell Calendar",
    "backendKind": "fake_local",
    "accountBinding": {
      "accountKey": "acct_shell"
    },
    "canonicalDefault": true
  }' \
  http://127.0.0.1:19192/v1/integrations
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "readinessStatus": "degraded",
    "authState": "authorized",
    "healthState": "degraded",
    "reason": "probe failed",
    "requiredOperatorAction": "Reconnect calendar"
  }' \
  http://127.0.0.1:19192/v1/integrations/calendar-shell/readiness
```

4. Seed one pending approval for the shell inbox.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "action": "shell.test.approval",
    "resourceKind": "operator_shell",
    "resourceId": "phase32",
    "reason": "Seed approval inbox for phase 32 verification",
    "requestedBy": "quickstart"
  }' \
  http://127.0.0.1:19192/v1/policy/approvals
```

5. Seed representative background activity for schedule and workflow inspection.

```bash
RUN_RESPONSE=$(curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "entrypoint": "operator.shell.quickstart",
    "goal": "Seed operator activity"
  }' \
  http://127.0.0.1:19192/v1/runs)
printf '%s\n' "$RUN_RESPONSE"
RUN_ID=$(printf '%s' "$RUN_RESPONSE" | jq -r '.runId')
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "goal": "Seed operator workflow"
  }' \
  http://127.0.0.1:19192/v1/runs/$RUN_ID/workflows
```

```bash
SCHEDULE_FIRE_AT=$(date -u -v+5M '+%Y-%m-%dT%H:%M:%SZ')
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "trigger": {
      "kind": "once",
      "fireAt": "'"$SCHEDULE_FIRE_AT"'"
    },
    "target": {
      "kind": "run",
      "run": {
        "entrypoint": "operator.shell.schedule",
        "goal": "Seed scheduled operator activity"
      }
    }
  }' \
  http://127.0.0.1:19192/v1/schedules
```

6. Inspect the new operator projection routes directly before opening the shell.

```bash
curl -sS \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/operator/onboarding | jq
```

```bash
curl -sS \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/operator/activity | jq
```

```bash
curl -sS \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/operator/diagnostics | jq
```

Expected outcome after implementation:

- onboarding response shows the explicit environment, readiness items, the minimum
  readiness set for the recommended first useful action, and optional follow-up setup
- activity response shows recent schedule, workflow, approval, or first-action records
- diagnostics response shows at least the seeded degraded integration as an actionable
  finding

7. Start the primary web shell.

```bash
pnpm dev:web
```

Expected outcome after implementation:

- the shell loads with the active environment shown prominently
- the current onboarding view matches `/v1/operator/onboarding`
- the approval inbox shows the seeded pending approval
- recent activity and diagnostics match the operator projection routes

8. Resolve the seeded approval from the shell.

Expected outcome after implementation:

- approving or rejecting the request happens inside the shell
- the approval row updates without requiring a raw route handoff
- the related activity or blocked-work summary refreshes accordingly

9. Run the first useful action from the shell.

Expected outcome after implementation:

- if a test query path is ready, the shell can submit a bounded query and show immediate
  result and status feedback
- if the environment lacks a ready provider for a test query, the shell can surface a
  bounded test-run path instead and show immediate run status feedback
- completing the recommended bounded action can move onboarding to `completed` even if
  unrelated optional readiness items remain visible

10. Exercise diagnostic drill-down.

Expected outcome after implementation:

- selecting the degraded integration diagnostic opens authoritative readiness detail
- selecting activity tied to schedules, workflows, or deliveries links to authoritative
  existing detail surfaces
- the shell never mixes test and live data in one view

## Automated Checks To Run Before Manual Validation

```bash
pnpm test:web
pnpm test:sdk
pnpm build:web
cd daemon && go test ./internal/api ./internal/contracts ./internal/app
make daemon-contract-test
```

## Recorded Verification Notes

Observed on 2026-04-24 in `DOPE_ENV=test`:

- seeded `calendar-shell` integration appeared in `/v1/operator/onboarding` and
  `/v1/operator/diagnostics` as degraded readiness with a required operator action
- a local pairing-generated bearer token loaded operator projections successfully after a
  daemon restart, confirming token durability for the walkthrough
- browser loading from `http://127.0.0.1:4173/` succeeded after local-origin CORS was
  verified for `http://127.0.0.1:4173 -> http://127.0.0.1:19192`
- the operator shell showed explicit `test` environment scope, `completed` onboarding,
  approval inbox content, event-backed recent activity, diagnostics, and same-shell
  authoritative detail for `/v1/auth/me`
- seeded approval `approval_6331e20225f637a9` resolved to `approved` inside the shell and
  remained `approved` after daemon restart when fetched from
  `/v1/policy/approvals/{approvalId}`
- `/v1/operator/activity` remained available after daemon restart and continued to show
  persisted event-backed `computer_use_action` records alongside workflow and run truth
- launching the recommended first useful action from the shell created
  `run_18e08dd707ade7a3` and surfaced immediate bounded run status feedback
- spot timing checks on the seeded local daemon stayed well inside the plan targets:
  - `/v1/operator/onboarding`: `0.038592 s`
  - `/v1/operator/activity?attentionOnly=true&limit=20`: `0.569744 s`
  - `/v1/operator/diagnostics`: `0.513144 s`
