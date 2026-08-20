# Quickstart: Personal Integrations Platform

## Goal

Verify in `KURA_ENV=test` that the daemon can:

- register and inspect daemon-owned integration resources
- project explicit `not_configured`, `auth_pending`, `healthy`, `degraded`, and
  `unavailable` readiness truth
- enforce one canonical default per domain/account/environment group
- run one fake integration probe through the normal runtime and approval surfaces
- expose redacted integration provenance on linked operator-visible resources

## Prerequisites

- local test daemon only; do not use `~/.kura`
- authenticated local pairing or an existing bearer token
- no production connectors or live personal-system credentials are required
- the repo-owned fake integration backend is enabled in the test environment

## Suggested Verification Flow

1. Start the daemon in the test environment.

```bash
make daemon-run-test
```

2. Create a fake integration record for a calendar-like personal account.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "integrationId": "calendar-fake-a",
    "domainKind": "calendar",
    "displayName": "Calendar Fake A",
    "backendKind": "fake_local",
    "backendRefId": "fake-calendar-a",
    "backendDisplayName": "Fake Calendar A",
    "accountBinding": {
      "accountKey": "alice@example.com",
      "accountLabel": "Alice Calendar"
    }
  }' \
  http://127.0.0.1:19192/v1/integrations
```

Expected outcome after implementation:

- the response returns an integration resource
- the resource is environment-scoped
- the initial readiness is explicit rather than implied

3. Move the integration through `auth_pending` and then `healthy`.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "readinessStatus": "auth_pending",
    "authState": "pending",
    "requiredOperatorAction": "complete fake auth"
  }' \
  http://127.0.0.1:19192/v1/integrations/calendar-fake-a/readiness
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
  http://127.0.0.1:19192/v1/integrations/calendar-fake-a/readiness
```

Expected outcome after implementation:

- the integration transitions through `auth_pending` and `healthy`
- account binding, readiness reason, and redacted provenance remain inspectable

4. Create a second integration for the same account and promote it as canonical default.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "integrationId": "calendar-fake-b",
    "domainKind": "calendar",
    "displayName": "Calendar Fake B",
    "backendKind": "fake_local",
    "backendRefId": "fake-calendar-b",
    "backendDisplayName": "Fake Calendar B",
    "accountBinding": {
      "accountKey": "alice@example.com",
      "accountLabel": "Alice Calendar"
    }
  }' \
  http://127.0.0.1:19192/v1/integrations
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}' \
  http://127.0.0.1:19192/v1/integrations/calendar-fake-b/default
```

Expected outcome after implementation:

- both integration records remain visible
- exactly one of them is the canonical default for `calendar/alice@example.com/test`

5. Mark the canonical default `degraded`, then `unavailable`, and inspect the difference.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "readinessStatus": "degraded",
    "authState": "authorized",
    "healthState": "degraded",
    "reason": "partial test degradation"
  }' \
  http://127.0.0.1:19192/v1/integrations/calendar-fake-b/readiness
```

Expected outcome after implementation:

- `degraded` remains explicitly inspectable and does not silently become `healthy` or
  `unavailable`

6. Create a normal run and execute a read-only fake probe through the runtime plane.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "entrypoint": "operator",
    "goal": "Verify shared personal integration probe behavior."
  }' \
  http://127.0.0.1:19192/v1/runs
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "probeKind": "inspect"
  }' \
  http://127.0.0.1:19192/v1/runs/$RUN_ID/integrations/calendar-fake-b/probes
```

Expected outcome after implementation:

- the probe creates normal runtime step and tool-call truth
- the linked tool-call resource exposes `integrationBindings`
- the binding summary captures readiness and canonical-default truth at invocation time

7. Execute an approval-gated mutation probe.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "probeKind": "mutate"
  }' \
  http://127.0.0.1:19192/v1/runs/$RUN_ID/integrations/calendar-fake-b/probes
```

Expected outcome after implementation:

- the response includes or links to a pending approval
- the approval resource exposes the same `integrationBindings`
- resolving the approval allows the probe to complete through the normal runtime plane

8. Mark the integration `unavailable` and confirm execution is blocked.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "readinessStatus": "unavailable",
    "authState": "expired",
    "healthState": "unavailable",
    "reason": "fake backend unavailable"
  }' \
  http://127.0.0.1:19192/v1/integrations/calendar-fake-b/readiness
```

Expected outcome after implementation:

- the integration shows `unavailable`
- a later probe request fails with explicit blocked or unavailable truth rather than a
  silent generic runtime error

## Automated Verification

Run targeted suites plus contract coverage:

```bash
cd daemon && go test ./internal/integrations ./internal/api ./internal/store ./internal/app ./internal/runtime ./internal/orchestration ./internal/policy ./internal/contracts
make daemon-contract-test
cd daemon && go test ./internal/api -run 'TestIntegrationRoutesProjectReadinessAndCanonicalDefault|TestIntegrationProbeRoutesLinkRuntimeApprovalAndProvenance|TestIntegrationRestartRestoresResourcesAndDoesNotInventHealthyState' -count=1
```

Expected automated coverage after implementation:

- integration create, list, inspect, readiness update, and canonical-default replacement
- domain/account/environment grouping semantics
- degraded versus unavailable execution behavior
- probe execution linkage to runs, tool calls, approvals, and event history
- restart-safe restoration of persisted integration truth

## Observed Verification Notes

Observed on 2026-04-22 in this branch:

- `cd daemon && go test ./internal/integrations ./internal/runtime ./internal/policy ./internal/store ./internal/api ./internal/app ./internal/contracts ./internal/orchestration -count=1` passed
- `make daemon-contract-test` passed
- the implementation branch emits and validates `integration.registered`, `integration.updated`,
  `integration.readiness_changed`, and `integration.default_changed` events, and exposes
  additive `integrationBindings` on tool calls, workflow steps, and approvals
- a live current-branch daemon walkthrough passed on `127.0.0.1:19193` with
  `KURA_ENV=test` and isolated `KURA_DATA_DIR=/tmp/kura-integrations-manual`
- the isolated daemon started from an empty `/v1/integrations` list, accepted
  local pairing auth, and created `calendar-fake-a` as explicit `not_configured`
  test-scoped integration truth
- `calendar-fake-a` transitioned through `auth_pending` and `healthy`; a second
  record `calendar-fake-b` became the canonical default for the same
  `calendar/alice@example.com/test` account group and remained inspectable as
  `degraded`
- the read-only probe returned `completed` with linked run step, tool call, and
  degraded `integrationBindings`; the mutation probe first returned a pending
  approval with the same binding snapshot, then completed after approval
  resolution
- after marking `calendar-fake-b` `unavailable`, a later inspect probe was
  blocked with HTTP `409 Conflict`; run event history showed
  `run.created`, `tool_call.requested`, and `tool_call.completed`

Manual daemon walkthrough status for this session:

- the shared `make daemon-run-test` environment was not suitable for branch-local
  verification because it exposed stale route state, so the final walkthrough used
  an isolated current-branch daemon on `127.0.0.1:19193`
- workflow-step `integrationBindings` remain covered by the automated
  workflow-hosted regression path; the manual quickstart flow itself validates the
  run, tool-call, approval, and event-history surfaces

## Notes

- Keep all verification in `KURA_ENV=test`.
- Prefer the repo-owned fake integration backend over any live personal-system connector.
- Calendar is only the example grouping used in this walkthrough; roadmap 27 remains
  domain-agnostic.
