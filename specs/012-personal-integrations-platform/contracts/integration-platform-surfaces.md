# Contract Surfaces: Personal Integrations Platform

## Goal

Add a first-class daemon-owned integration resource plane for account-backed personal
systems, plus additive runtime and approval linkage so downstream domains reuse one shared
readiness and provenance contract.

## HTTP API Surfaces

### New Top-Level Integration Resource Routes

- `POST /v1/integrations`
- `GET /v1/integrations`
- `GET /v1/integrations/{integrationId}`
- `POST /v1/integrations/{integrationId}/readiness`
- `POST /v1/integrations/{integrationId}/default`

Request and response requirements:

- integration creation accepts:
  - `integrationId`
  - `domainKind`
  - `displayName`
  - backend summary (`backendKind`, `backendRefId`, `backendDisplayName`)
  - optional initial account-binding summary
  - optional canonical-default intent
- integration resource responses return:
  - identity fields (`integrationId`, `domainKind`, `displayName`)
  - `environmentScope`
  - readiness, auth, and health summaries
  - canonical-default flag
  - account-binding summary when known
  - backend-binding summary
  - redacted secret-provenance summary
  - readiness reason and any required operator action
- readiness update accepts:
  - `readinessStatus`
  - optional `authState`
  - optional `healthState`
  - optional account-binding updates when auth reveals more identity
  - redacted secret-resolution or provenance updates
  - operator-visible reason and required follow-up action
- default-promotion requests promote one record as canonical default for its
  domain/account/environment group and demote siblings atomically

Schema surfaces:

- add `schemas/api/create-integration.request.schema.json`
- add `schemas/api/integration-resource.schema.json`
- add `schemas/api/integration-list.response.schema.json`
- add `schemas/api/report-integration-readiness.request.schema.json`
- add `schemas/api/set-integration-default.request.schema.json`
- add `schemas/api/integration-binding-summary.schema.json`

### New Run-Scoped Fake Integration Probe Route

- `POST /v1/runs/{runId}/integrations/{integrationId}/probes`

Requirements:

- this route exists only to verify the shared runtime, approval, and provenance model in
  `DOPE_ENV=test` using the repo-owned fake integration backend
- probe requests accept:
  - `probeKind`: `inspect` or `mutate`
  - optional `approvalId`
  - optional small structured input for deterministic fake responses
- probe responses return:
  - owning `runId`
  - created or linked `stepId`
  - created `toolCallId`
  - linked `integrationBindings`
  - linked `approval` when the probe is approval-gated
  - final or pending status

Schema surfaces:

- add `schemas/api/create-integration-probe.request.schema.json`
- optionally add `schemas/api/integration-probe.response.schema.json` if the route returns
  a wrapper around the linked runtime resource instead of the tool-call resource directly

### Existing Runtime, Workflow, And Approval Surfaces Extended

- `GET /v1/runs`
- `GET /v1/runs/{runId}`
- `GET /v1/runs/{runId}/tool-calls`
- `GET /v1/runs/{runId}/workflows/{workflowId}`
- `GET /v1/policy/approvals`
- `GET /v1/policy/approvals/{approvalId}`

Additive requirements:

- tool-call resources gain `integrationBindings`
- workflow-step resources gain `integrationBindings`
- approval resources gain `integrationBindings`
- these summaries capture:
  - `integrationId`
  - `domainKind`
  - `displayName`
  - `accountKey`
  - `canonicalDefault`
  - `readinessAtInvocation`
  - `backendKind`
  - redacted `secretResolution`
  - `environmentScope`

Schema surfaces:

- update `schemas/api/tool-call-resource.schema.json`
- update `schemas/api/workflow-step-resource.schema.json`
- update `schemas/api/approval-resource.schema.json`

## Event And History Surfaces

New integration event families:

- `integration.registered`
- `integration.updated`
- `integration.readiness_changed`
- `integration.default_changed`

Event payload requirements:

- resource identity:
  - `integrationId`
  - `domainKind`
  - `displayName`
  - `environmentScope`
- readiness truth:
  - `readinessStatus`
  - `authState`
  - `healthState`
  - `reason`
  - `requiredOperatorAction`
- grouping and provenance:
  - `accountKey`
  - `canonicalDefault`
  - `backendKind`
  - redacted `secretResolution`

Additive runtime truth requirements:

- fake integration probe execution reuses normal `run.*`, `step.*`, `tool_call.*`, and
  `policy.approval_*` events with additive integration-binding summaries where projected
- no second integration-only execution history should be required to understand what work
  happened

Schema surfaces:

- add `schemas/events/integration-registered.event.schema.json`
- add `schemas/events/integration-updated.event.schema.json`
- add `schemas/events/integration-readiness-changed.event.schema.json`
- add `schemas/events/integration-default-changed.event.schema.json`
- update any shared runtime event schema only if integration-binding summaries are
  explicitly projected there

## Persistence Surfaces

Persistence remains additive to the daemon-owned SQLite store:

- add an `integrations` table for resource identity, readiness, canonical-default
  grouping, and persisted document state
- extend persisted runtime tool-call documents with `integrationBindings`
- extend persisted workflow-step documents with `integrationBindings`
- extend approval-enrichment / policy-backed documents with `integrationBindings`

Persistence rules:

- integration resources are environment-scoped and durable across daemon restart
- canonical-default demotion and promotion within one group must be atomic
- tool-call and approval projections store invocation-time binding snapshots rather than
  resolving live integration state on read
- secret values are never persisted in operator-visible documents

## Documentation Surfaces

Docs updated by implementation:

- `docs/runtime/daemon-roadmaps.md`
- `docs/runtime/operator-trust-model.md`
- `docs/harness/harness-architecture.md`
- `docs/runtime/daemon-api-and-event-model.md` if route or event coverage is summarized
  there
- `docs/specs/013-delivery-and-notifications.md` for the delivery-versus-readiness
  boundary
- `docs/specs/014-calendar-integration.md`
- `docs/specs/015-mail-integration.md`

## Truthfulness Constraints

- the platform stays domain-agnostic in roadmap 27
- `unavailable` blocks integration-backed work; `degraded` stays inspectable and delegates
  operation-level gating to the specific action or later domain policy
- exactly one canonical default exists per domain/account/environment group
- integration-backed work remains subordinate to existing runtime, workflow, and approval
  truth
- delivery outcomes remain a separate plane and must not be rewritten by integration
  readiness projection
- secret-backed provenance remains redacted and environment-scoped
