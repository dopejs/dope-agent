# Contract Surfaces: Operator Shell And Onboarding

## Goal

Add a web-first operator shell that projects onboarding, readiness, approvals, recent
activity, and diagnostics from daemon-owned truth without duplicating existing domain
contracts.

## New Operator Projection Routes

### `GET /v1/operator/onboarding`

- Purpose: Return the daemon-owned onboarding projection for the active environment.
- Response requirements:
  - `environmentScope`
  - onboarding `status`
  - `currentStepId`
  - ordered `readinessItems`
  - `blockingItemIds`
  - `optionalFollowUpItemIds`
  - bounded `firstUsefulActions`
  - one `recommendedActionId`
  - `lastEvaluatedAt`
- Truthfulness rules:
  - onboarding progress is derived from daemon-owned auth, config, connector,
    integration, capability, and provider truth
  - onboarding completion is based on the minimum readiness set for the selected first
    useful action, not all possible integrations

Schema surfaces:

- add `schemas/api/operator-onboarding.response.schema.json`
- add `schemas/api/operator-readiness-item.schema.json`
- add `schemas/api/operator-first-use-action.schema.json`

### `GET /v1/operator/activity`

- Purpose: Return recent cross-domain operator activity without forcing the client to
  reconstruct it from raw event records.
- Query parameters:
  - `limit`
  - `sourceKind`
  - `attentionOnly`
- Response requirements:
  - ordered `items`
  - each item includes `activityId`, `sourceKind`, `sourceId`, `title`, `status`,
    `summary`, `attentionLevel`, `occurredAt`, `detailRoute`, and linked resource
    references
- Truthfulness rules:
  - activity items are derived from persisted events and authoritative current resource
    linkage
  - items remain environment-scoped and never mix test and live activity

Schema surfaces:

- add `schemas/api/operator-activity-record.schema.json`
- add `schemas/api/operator-activity-list.response.schema.json`

### `GET /v1/operator/diagnostics`

- Purpose: Return actionable diagnostic findings across readiness, approval, execution,
  and delivery planes.
- Query parameters:
  - `sourceKind`
  - `plane`
  - `severity`
  - `unresolvedOnly`
- Response requirements:
  - ordered `items`
  - each item includes `findingId`, `sourceKind`, `sourceId`, `plane`, `severity`,
    `status`, `reason`, `recommendedAction`, `detailRoute`, related resource references,
    and `capturedAt`
- Truthfulness rules:
  - diagnostics are derived from explicit daemon status and reason fields rather than
    client heuristics
  - findings preserve separate readiness, approval, execution, and delivery truth

Schema surfaces:

- add `schemas/api/operator-diagnostic-finding.schema.json`
- add `schemas/api/operator-diagnostic-list.response.schema.json`

## Reused Existing Routes

### Shell Entry And Auth Context

- `GET /v1/system/info`
- `GET /v1/auth/me`
- `GET /v1/config`

These routes remain the source of truth for environment identity, auth state, and
redacted configuration context visible from the shell.

### Approval Inbox And Resolution

- `GET /v1/policy/approvals`
- `GET /v1/policy/approvals/{approvalId}`
- `POST /v1/policy/approvals/{approvalId}/resolve`

Requirements:

- the web shell must use these routes directly for approval action handling
- the operator projection layer may summarize pending approvals, but approval state and
  resolution remain owned by the policy plane

### Readiness And Health Drill-Down

- `GET /v1/integrations`
- `GET /v1/integrations/{integrationId}`
- `GET /v1/connectors`
- `GET /v1/connectors/{connectorId}`
- `GET /v1/capabilities`
- `GET /v1/capabilities/{capabilityId}`
- `GET /v1/providers`
- `GET /v1/providers/{providerId}`

Requirements:

- shell readiness and diagnostics must link back to these authoritative detail routes
- no operator-shell-specific duplicate resource model should replace these domain routes

### Background Work And Outcome Drill-Down

- `GET /v1/runs`
- `GET /v1/runs/{runId}`
- `GET /v1/runs/{runId}/workflows`
- `GET /v1/runs/{runId}/workflows/{workflowId}`
- `GET /v1/schedules`
- `GET /v1/schedules/{scheduleId}`
- `GET /v1/deliveries`
- `GET /v1/deliveries/{deliveryId}`
- `GET /v1/runs/{runId}/computer-use/sessions`
- `GET /v1/runs/{runId}/computer-use/sessions/{computerUseSessionId}`
- `GET /v1/runs/{runId}/computer-use/sessions/{computerUseSessionId}/actions`
- `GET /v1/runs/{runId}/computer-use/sessions/{computerUseSessionId}/actions/{computerUseActionId}`

Requirements:

- the operator shell may project recent activity and findings across these resources, but
  detailed inspection remains backed by the existing domain contracts rather than a
  duplicated operator-only resource model
- activity and diagnostics items must expose route linkage metadata, but the shell must
  render required basic inspection inside shell-resident detail panels or sheets instead of
  requiring navigation to raw daemon endpoints

### Freshness And Bounded First Useful Action

- `GET /v1/events`
- `GET /v1/events/stream`
- `POST /v1/chat/query`
- `POST /v1/chat/query/stream`
- `POST /v1/runs`

Requirements:

- event stream is used for shell freshness hints and targeted refetch, not as a client-side
  shadow source of truth
- the bounded first useful action must reuse existing chat-query or run creation routes
  rather than introducing a shell-only execution boundary

## Persistence And Event Model

- no new operator-shell-specific persistence tables are required in phase 32
- operator projections are derived from existing persisted daemon resources and events:
  - auth pairing and access-token truth
  - config response
  - integrations
  - connectors
  - capabilities
  - approvals and decisions
  - runs, workflows, schedules, deliveries
  - computer-use resources
  - persisted events
- no new shell-specific event family is required if projection routes can be rebuilt from
  existing daemon truth and current event history

## SDK Surface

`sdk/ts/src/index.ts` must add typed client methods for:

- `getOperatorOnboarding()`
- `listOperatorActivity()`
- `listOperatorDiagnostics()`
- `listApprovals()` and `resolveApproval()`
- any reused detail routes the web shell needs for shell-resident inspection panels,
  including run-scoped computer-use detail fetches, when the SDK does not already expose
  them

The SDK remains the only browser-facing API wrapper used by `web/`.

## Documentation Surfaces

Docs updated by implementation:

- `docs/runtime/daemon-roadmaps.md`
- `docs/runtime/daemon-api-and-event-model.md`
- `docs/harness/harness-architecture.md`
- `docs/specs/017-operator-shell-and-onboarding.md`

## Truthfulness Constraints

- operator projections are daemon-owned summaries, not browser-owned derivations
- the shell must display the current environment explicitly and never merge test and live
  truth
- approval resolution stays on the policy plane
- detailed inspection of schedules, workflows, deliveries, integrations, connectors,
  capabilities, and computer-use records stays on their authoritative routes, but phase 32
  must surface the required inspection path inside the primary web shell rather than
  sending operators to raw route URLs
- onboarding completion depends only on the minimum readiness set for the selected bounded
  first useful action
- other client surfaces may remain partial; phase 32 completion is measured against the
  primary web shell only
