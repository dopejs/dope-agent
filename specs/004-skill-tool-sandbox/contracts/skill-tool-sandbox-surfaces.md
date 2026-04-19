# Contract: Skill And Local Tool Sandbox Surfaces

## Scope

This contract defines the operator-visible surfaces that may change for Roadmap 19. The
contract is intentionally additive and must remain backward-compatible.

## In-Scope Execution Families

- executable skills surfaced through the existing skill registry
- the current high-risk local tool path (`exec`, `shell`, `browser`)

## Affected Existing Skill Surfaces

- `GET /v1/skills`
- `GET /v1/skills/{skillId}`
- `POST /v1/skills/reload`

Planned contract rules:

- Existing route names remain in place.
- Skill responses may gain additive executable-manifest fields, availability state, and
  operator-visible `unavailable` reasons.
- Non-executable skills remain valid and must not be misrepresented as sandbox-launched
  execution targets.
- Invalid executable skills stay visible in skill inspection as `unavailable` with an
  explicit reason.

## Affected Existing Runtime Tool-Call Surfaces

- `POST /v1/runs/{runId}/steps/{stepId}/tool-calls`
- `GET /v1/runs/{runId}/steps/{stepId}/tool-calls`
- `GET /v1/runs/{runId}/steps/{stepId}/tool-calls/{toolCallId}`
- `POST /v1/runs/{runId}/steps/{stepId}/tool-calls/{toolCallId}/complete`
- `POST /v1/runs/{runId}/steps/{stepId}/tool-calls/{toolCallId}/fail`
- tool-call resources and related runtime events

Planned contract rules:

- The existing tool-call route family remains the primary runtime execution surface for
  Roadmap 19; no second public execution resource is introduced.
- Create and resource payloads may gain additive fields that distinguish local-tool versus
  skill-targeted execution and link the tool call to approval, consumer-policy, and
  sandbox-execution truth.
- In-scope requests blocked before launch must still produce operator-visible denial or
  approval-pending truth with durable provenance.
- Restart recovery for interrupted in-flight executions must surface as `cancelled`, not
  `running` or `unknown`.

## Affected Existing Approval Surfaces

- `GET /v1/policy/approvals`
- `POST /v1/policy/approvals`
- `GET /v1/policy/approvals/{approvalId}`
- `POST /v1/policy/approvals/{approvalId}/resolve`
- `policy.approval_requested`
- `policy.approval_resolved`
- `policy.decision_recorded`

Planned contract rules:

- Approval ids and event names remain stable.
- Executable skills may appear as approval-gated consumers with additive skill identity,
  declaration, tool-call linkage, and sandbox provenance.
- High-risk local tools keep their current approval boundary, but approval truth must now
  align with the launched sandbox execution when work proceeds beyond preflight.
- Approval lookups must preserve the originally applied declaration and secret-scope view;
  they must not degrade to a generic placeholder declaration after persistence or restart.
- Default executable-skill approval posture is `ask` when not explicitly declared.

## Affected Existing Sandbox Surfaces

- `GET /v1/sandboxes/executions`
- `GET /v1/sandboxes/executions/{executionId}`
- `POST /v1/sandboxes/executions`
- `POST /v1/sandboxes/executions/{executionId}/cancel`
- `POST /v1/sandboxes/explain`
- `sandbox.execution_*`
- `sandbox.decision_recorded`

Planned contract rules:

- Existing sandbox routes remain in place; in-scope execution uses them as the only launch
  boundary.
- Additive provenance fields may identify `consumerKind=skill` or `consumerKind=local_tool`,
  the addressed skill or capability id, the linked tool call, and the effective policy
  record.
- Sandbox responses and events must distinguish approval-required, approval-rejected,
  denied, unsupported, launch-failed, process-failed, timeout, cancelled, and
  output-capture-failed outcomes.
- Explain responses must remain truthful when a request is unsupported because the current
  subprocess backend cannot satisfy the declared guarantee.

## Affected Existing Config And Event History Surfaces

- `GET /v1/config`
- `GET /v1/events`

Planned contract rules:

- Config inspection may gain additive executable-skill or local-execution policy summaries
  only where they are already operator-visible and can remain redacted.
- Event history must be sufficient to reconstruct manifest availability, approval outcome,
  runtime tool-call state, sandbox execution linkage, and restart recovery classification.

## Provenance Contract

Operator-visible surfaces must be able to answer:

- whether the execution target was a skill or a high-risk local tool
- which skill id or capability id requested the work
- which declaration applied
- whether approval was required, granted, rejected, or not applicable
- whether a subprocess launched
- which sandbox execution and policy record correspond to the runtime tool call
- whether restart recovery changed the terminal state to `cancelled`

Compatibility rule:

- Existing ids, route names, and event names stay stable.
- New provenance is additive and must be schema-backed.

## Secret Scope And Redaction Contract

Operator-visible surfaces must clearly distinguish:

- declared secret refs
- consumer-instance authorization
- environment eligibility (`test`, `prod`, or both)
- resolution outcome (`resolved`, `denied`, `unavailable`)
- environment-scoped secret source rooted in the active daemon data dir
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

- No new top-level runtime `skill-call` resource family
- No broader migration of all capability or local-tool paths
- No automatic skill triggering or orchestration planner
- No second sandbox backend family
- No unmanaged subprocess launch path for supported executable skills or the current
  high-risk local tools
