# Contract: MCP Sandbox Surfaces

## Scope

This contract defines the operator-visible surfaces for the MCP execution-plane roadmap.
The contract is intentionally additive and must remain backward-compatible.

## New MCP Resource Routes

Planned route family:

- `GET /v1/mcp/servers`
- `POST /v1/mcp/servers`
- `GET /v1/mcp/servers/{serverId}`
- `PATCH /v1/mcp/servers/{serverId}`
- `POST /v1/mcp/servers/{serverId}/start`
- `POST /v1/mcp/servers/{serverId}/stop`
- `POST /v1/mcp/servers/{serverId}/restart`
- `POST /v1/mcp/servers/{serverId}/cancel`
- `GET /v1/mcp/servers/{serverId}/tools`
- `PATCH /v1/mcp/servers/{serverId}/tools/{toolName}`
- `POST /v1/mcp/servers/{serverId}/tools/{toolName}/authorize`

Planned contract rules:

- MCP servers are first-class daemon resources with stable ids and additive fields.
- Server resources expose sandbox profile, declaration id, enabled state, lifecycle state,
  last failure or block reason, and redacted credential-scope summary.
- Tool resources expose discovery state, per-runtime-surface allowlist decisions,
  effective availability, and approval requirements.
- Tool-authorization routes must create or validate approval state when an MCP tool is
  marked `approval_required` for the addressed runtime surface.
- The route family does not create a second execution plane; lifecycle launches must still
  resolve through sandbox-backed execution.
- Lifecycle action routes and tool exposure update routes must have explicit schema-backed
  request or response contracts; they must not rely on undocumented implicit payloads.

## Affected Existing Sandbox Surfaces

- `GET /v1/sandboxes/executions`
- `GET /v1/sandboxes/executions/{executionId}`
- `POST /v1/sandboxes/explain`
- `sandbox.execution_*`
- `sandbox.decision_recorded`

Planned contract rules:

- Existing sandbox routes remain in place.
- Additive provenance fields may identify `consumerKind=mcp_server`, `consumerId`,
  declaration id, policy-record id, and linked MCP server id.
- Sandbox decision and execution payloads must distinguish policy denial, approval
  requirement, launch failure, transport/runtime failure, timeout, and cancellation.
- Explain responses must remain truthful when lifecycle or tool exposure is blocked because
  the current subprocess backend cannot satisfy a stronger declared guarantee.

## Affected Existing Approval And Runtime Surfaces

- `GET /v1/policy/approvals`
- `POST /v1/policy/approvals`
- `POST /v1/policy/approvals/{approvalId}/resolve`
- existing tool-call resources and runtime events when MCP tool exposure becomes relevant

Planned contract rules:

- Approval gating for this slice attaches to MCP tool exposure, not routine server
  lifecycle actions.
- Approval and decision payloads may carry MCP server id, tool name, runtime surface, and
  linked tool exposure rule metadata additively.
- Existing approval ids and event names remain stable.

## Affected Existing Config And Event History Surfaces

- `GET /v1/config`
- `GET /v1/events`

New event families expected:

- `mcp.server_registered`
- `mcp.server_updated`
- `mcp.server_started`
- `mcp.server_stopped`
- `mcp.server_failed`
- `mcp.server_health_changed`
- `mcp.tool_exposure_updated`

Planned contract rules:

- Config inspection must surface MCP server definitions and tool exposure policy in a
  redacted, operator-readable form.
- Event history must be sufficient to reconstruct server registration, lifecycle changes,
  restart blocks, credential resolution outcomes, and tool exposure decisions.
- New event types must be schema-backed and remain consistent with any lifecycle state
  projected through MCP resource routes.

## Credential And Secret-Scope Contract

Operator-visible surfaces must clearly distinguish:

- declared secret refs for each MCP server
- server-instance authorization
- environment eligibility (`test`, `prod`, or `both`)
- resolution outcome (`resolved`, `denied`, `unavailable`)
- whether a reusable default contributed to the decision
- redacted presentation of secret-bearing data

Compatibility rule:

- No route, event, or config projection may emit plain-text secret values or raw
  secret-derived material.

## Tool Exposure Contract

Operator-visible surfaces must be able to answer:

- which MCP server owns the tool
- whether the tool is discovered, stale, or currently unavailable
- which runtime surfaces are blocked, allowed, or approval-gated
- whether the tool is effectively exposed right now
- why a tool that was allowlisted is not currently usable

Compatibility rule:

- Tools without an explicit allowlist rule for a runtime surface stay unexposed.
- Tool exposure must never imply the owning server is healthy when lifecycle state says
  otherwise.

## Enforcement Strength Contract

- The current backend remains `subprocess`.
- Declarations that require stronger guarantees than current subprocess support must fail as
  `unsupported` or `denied`.
- No payload, doc, or operator-visible string introduced by this slice may imply container,
  VM, or hardened network isolation that does not exist.

## Non-Goals

- No generic arbitrary tool orchestration planner
- No server-wide implicit exposure of all discovered tools
- No second sandbox backend family
- No unmanaged MCP process-launch path beside sandbox-backed execution
