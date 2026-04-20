# Contract: Sandbox Stronger Backend Surfaces

## Implemented Surface Summary

- `GET /v1/config` now projects backend capability and readiness summaries.
- Sandbox profile, explain, execution, runtime tool-call, and event surfaces preserve
  backend identity and unsupported mismatch truth.
- The first real stronger-backend consumer family is executable skills that opt into
  `docker_default`.

## Scope

This contract defines the operator-visible surfaces that may change for Roadmap 20. The
contract remains additive and must not introduce a backend-specific side plane.

## In-Scope Backend Families

- existing `subprocess`
- new stronger backend `docker`

## In-Scope Consumer Families For This Slice

- executable skills that explicitly declare `docker`

Explicitly deferred in this slice:

- automatic migration of all executable skills
- high-risk local tools (`exec`, `shell`, `browser`)
- MCP servers
- managed providers

## Affected Existing Sandbox Profile Surfaces

- `GET /v1/sandboxes/profiles`
- `GET /v1/sandboxes/profiles/{profileId}`
- `POST /v1/sandboxes/profiles/reload`

Planned contract rules:

- Existing route names remain in place.
- Profile inspection may gain additive backend-capability and host-prerequisite fields.
- Operators must be able to distinguish truthful hard-enforcement differences between
  `subprocess` and `docker`.
- Profile inspection must not imply `docker` availability when the host cannot satisfy its
  prerequisites.

## Affected Existing Sandbox Explain Surfaces

- `POST /v1/sandboxes/explain`
- `sandbox.decision_recorded`

Planned contract rules:

- Explain responses remain the canonical operator surface for backend selection and
  capability mismatch truth.
- When `docker` is required but unavailable, explain must return an explicit
  `unsupported`-style result rather than a degraded subprocess plan.
- Explain responses must name the effective backend when execution is possible.

## Affected Existing Sandbox Execution Surfaces

- `GET /v1/sandboxes/executions`
- `GET /v1/sandboxes/executions/{executionId}`
- `POST /v1/sandboxes/executions`
- `POST /v1/sandboxes/executions/{executionId}/cancel`
- `sandbox.execution_*`

Planned contract rules:

- Existing execution routes remain in place.
- Execution resources and events may gain additive backend-capability and host-prerequisite
  context.
- `docker` executions must remain queryable through the same execution resource family as
  subprocess.
- Unsupported stronger-backend requests must remain operator-visible even when no
  container launch occurs.

## Affected Existing Skill Inspection Surfaces

- `GET /v1/skills`
- `GET /v1/skills/{skillId}`
- `POST /v1/skills/reload`

Planned contract rules:

- Executable-skill inspection may gain additive backend-selection fields.
- Only explicitly declared `docker` executable skills change backend posture in this
  slice.
- Skills that do not declare `docker` must not be misrepresented as stronger-backend
  consumers.

## Affected Existing Runtime Tool-Call Surfaces

- `POST /v1/runs/{runId}/steps/{stepId}/tool-calls`
- `GET /v1/runs/{runId}/steps/{stepId}/tool-calls`
- `GET /v1/runs/{runId}/steps/{stepId}/tool-calls/{toolCallId}`
- tool-call resources and related runtime events

Planned contract rules:

- Existing tool-call route families remain the runtime execution surface.
- Skill-backed tool calls may gain additive backend identity and mismatch classification.
- Runtime history must preserve whether the skill used `subprocess`, `docker`, or failed
  as `unsupported` before launch.

## Affected Existing Config And Event History Surfaces

- `GET /v1/config`
- `GET /v1/events`

Planned contract rules:

- Config inspection may gain additive host-capability or backend-readiness summaries if
  they remain operator-safe.
- Event history must be sufficient to reconstruct backend selection, unsupported mismatch,
  launch outcome, and restart recovery truth.

## Backend Capability Contract

Operator-visible surfaces must be able to answer:

- which backends are supported by the daemon
- which backend was required, selected, or unavailable for a request
- what stronger guarantees `docker` adds relative to `subprocess`
- which host prerequisites block `docker` usage
- whether an execution actually ran on `docker` or failed before launch

Compatibility rule:

- Existing ids, route names, and event names stay stable.
- New backend capability truth is additive and schema-backed.

## Stronger-Backend Mismatch Contract

When a request explicitly requires `docker`:

- no silent fallback to `subprocess` is allowed
- the canonical host-prerequisite failure outcome is `unsupported`
- the outcome must be distinguishable from policy denial and consumer runtime failure

## Provenance Contract

Operator-visible surfaces must preserve:

- consumer kind and consumer id
- selected or required backend
- runtime tool-call linkage where applicable
- restart-recovery classification for stronger-backend executions

## Documentation And Migration Contract

Roadmap 20 artifacts and operator docs must provide:

- a backend capability matrix for `subprocess` and `docker`
- a remaining consumer inventory across current sandbox consumer families
- host prerequisite and degradation guidance
- explicit statement of which families are deferred beyond this slice

## Non-Goals

- No new backend-specific execution API
- No automatic migration of every executable skill to `docker`
- No migration of high-risk local tools, MCP, or managed providers in this roadmap
- No SSH, remote managed backend, or VM-grade isolation backend in this slice
