# Contract: Consumer Sandbox Surfaces

## Scope

This contract defines the operator-visible surfaces that may change for the shared
consumer requirement declaration slice. The contract is intentionally additive and must
remain backward-compatible.

## Current Consumer Families In Scope

- managed providers
- skills on the existing registry and explicit skill-selection surfaces
- daemon-owned local tools on the current high-risk tool-call path

## Affected Existing Surfaces

### Sandbox Control-Plane Routes

- `GET /v1/sandboxes/executions`
- `GET /v1/sandboxes/executions/{executionId}`
- `POST /v1/sandboxes/explain`

Planned contract rules:

- Existing routes remain in place; this slice adds no new parallel sandbox control plane.
- Additive fields may identify consumer kind, consumer instance, declaration id,
  policy-record id, secret-scope summary, and truthful enforcement expectation.
- Launched subprocess work continues to surface through sandbox execution resources and
  lifecycle events.
- Explain responses must remain truthful when a request is unsupported because the current
  backend cannot satisfy the declared guarantee.

### Provider Auth Routes And Events

- `GET /v1/providers/{providerId}/auth`
- `POST /v1/providers/{providerId}/auth/start`
- `POST /v1/providers/{providerId}/auth/complete`
- `POST /v1/providers/{providerId}/auth/refresh`
- `POST /v1/providers/{providerId}/auth/revoke`
- `provider.auth_started`
- `provider.auth_completed`
- `provider.auth_refreshed`
- `provider.auth_revoked`

Planned contract rules:

- Managed-provider auth surfaces stay additive and schema-backed.
- Access-only or auth-state-style operations may expose declaration id, policy-record id,
  secret-scope summary, and failure classification through auth metadata and auth events.
- Sensitive provider state remains redacted; only class-based summaries and secret-scope
  outcomes may be exposed.

### Tool-Call, Approval, And Runtime Surfaces

- `POST /v1/runs/{runId}/steps/{stepId}/tool-calls`
- `POST /v1/runs/{runId}/steps/{stepId}/tool-calls/{toolCallId}/complete`
- `POST /v1/runs/{runId}/steps/{stepId}/tool-calls/{toolCallId}/fail`
- `GET /v1/policy/approvals`
- `POST /v1/policy/approvals`
- `POST /v1/policy/approvals/{approvalId}/resolve`
- `tool-call` resources and related runtime events
- `policy.approval_requested`
- `policy.approval_resolved`
- `policy.decision_recorded`

Planned contract rules:

- The current daemon-owned high-risk tool path must no longer rely on approval truth alone;
  it must also expose the declaration, consumer identity, and sandbox-facing provenance
  needed to explain why a tool call was allowed, denied, or blocked.
- Approval and decision surfaces remain additive and keep existing resource ids stable.
- If a tool declaration requires stronger guarantees than the current backend supports, the
  surfaced result must be `unsupported` or `denied`, never a silent degraded execution.

### Skill Registry And Explicit Skill-Selection Surfaces

- `GET /v1/skills`
- `GET /v1/skills/{skillId}`
- `POST /v1/skills/reload`
- `POST /v1/chat/query`
- `chat.query.started`
- `chat.query.completed`

Planned contract rules:

- Skill surfaces may expose declaration-bearing metadata for the current skill family,
  including consumer identity and secret-scope summary when relevant.
- This slice must not imply that bundled skill scripts or arbitrary skill subprocesses are
  already daemon-managed execution paths.
- Explicit skill-selection behavior stays stable; new fields are additive only.

### Config Inspection And Event History

- `GET /v1/config`
- `GET /v1/events`
- `sandbox.execution_*`
- `sandbox.decision_recorded`

Planned contract rules:

- Config inspection must continue to redact secret material while exposing enough metadata
  to explain configured secret refs or secret-backed surfaces.
- Event history must remain sufficient to reconstruct declaration, consumer identity,
  secret-scope outcome, and terminal result for launched, denied, unsupported, and
  preflight-only paths.

## Provenance Contract

Operator-visible surfaces must be able to answer:

- which consumer kind initiated the work
- which consumer instance initiated the work
- which declaration applied
- whether a subprocess launched
- whether approval was required, granted, rejected, or not applicable
- whether secret scope resolved, was denied, or was unavailable

Compatibility rule:

- Existing ids, route names, and event names stay stable.
- New provenance is additive and must be schema-backed.

## Secret Scope And Redaction Contract

Operator-visible surfaces must clearly distinguish:

- requested secret refs
- consumer-instance authorization
- environment eligibility (`test`, `prod`, or both)
- resolution outcome (`resolved`, `denied`, `unavailable`)
- redacted presentation of secret-bearing material

Compatibility rule:

- No API, event, or config surface may emit plain-text secret values or raw secret-derived
  material introduced by this slice.

## Enforcement Strength Contract

- The current backend remains `subprocess`.
- Declarations that require stronger guarantees than current subprocess support must fail as
  `unsupported` or `denied`.
- No payload, doc, or operator-visible string introduced by this slice may imply container,
  VM, or hardened network isolation that does not exist.

## Non-Goals

- No MCP server registry or lifecycle routes
- No generic bundled-script or executable-skill subprocess runner
- No second sandbox backend family
- No new parallel public execution plane beside existing operator-visible resources
