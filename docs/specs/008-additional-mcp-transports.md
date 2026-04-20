# Spec 008: Additional MCP Transports

## Status

Planned

## Authority

This document is the authoritative upstream spec for Roadmap 23 before a branch-local
speckit feature spec is created.

Primary source documents:

- `docs/runtime/daemon-roadmaps.md` Roadmap 23
- `docs/harness/harness-architecture.md` MCP follow-on sequencing

## Background

The daemon currently supports:

- `stdio`
- `streamable-http`

This is enough to close the first complete MCP slice, but not enough to treat transport as
an extensible MCP capability family. Future MCP work needs a stable way to add more
transports without:

- splitting the control plane
- creating transport-specific side paths
- weakening lifecycle, authorization, recovery, or operator visibility

## Goal

Expand MCP transport coverage beyond `stdio` and `streamable-http` while preserving one
daemon-owned MCP registry, lifecycle, authorization, and invocation plane.

## Fixed Decisions

- New transport work must extend the existing daemon-owned MCP manager rather than creating
  a separate MCP control path.
- Transport support remains subordinate to the current runtime, approval, and audit model.
- Catalog/package management is not solved in this phase except where transport truth
  depends on it.
- Marketplace-style transport discovery remains out of scope.

## Dependencies

- Roadmap 18: MCP Execution Plane
- Roadmap 21: Complete MCP Runtime And Catalog

## Scope

### In Scope

- transport capability contract
- prerequisite and host-readiness truth per transport
- at least one additional MCP transport family
- transport-specific lifecycle, retry, restart, and recovery semantics
- operator-visible transport selection and failure truth
- verification on one real server using the new transport

### Out Of Scope

- marketplace catalog work
- non-MCP remote execution control planes
- multi-tool orchestration
- catalog package management beyond what is needed for transport truth

## Operator Problems To Solve

1. Operators need MCP integrations that do not fit cleanly into only `stdio` or
   `streamable-http`.
2. Future transport additions should not fork lifecycle semantics or bypass existing
   runtime, policy, and audit surfaces.
3. Operators need to know whether a transport is truly supported, degraded, blocked, or
   unavailable on the current host.

## User Stories

### Story 1: Inspect Transport Capability Truth

As an operator, I want each MCP transport family to expose explicit readiness and
prerequisite truth so I can understand why a transport is or is not usable on this host.

Acceptance expectations:

- transport capability and prerequisites are visible through daemon inspection
- unsupported, blocked, degraded, and ready states are distinguished explicitly
- transport mismatch is not reported as a generic runtime failure

### Story 2: Run A Real Server On A New Transport

As an operator, I want at least one additional MCP transport to run through the same
daemon-owned MCP manager so transport expansion does not create a second control path.

Acceptance expectations:

- initialize, discovery, and invocation all remain on the existing MCP manager path
- transport-specific failures remain explicit
- restart and restore stay bounded

### Story 3: Preserve Recovery And Audit Truth

As an operator, I want transport recovery behavior to remain visible and auditable so
remote or session-oriented transports do not hide reconnect or retry behavior.

Acceptance expectations:

- restart, reconnect, and cancellation semantics are explicit
- history and events distinguish lifecycle recovery from invocation failure
- audit truth does not depend on reading raw logs

## Functional Requirements

- Every supported MCP transport family must have explicit capability, prerequisite, and
  readiness metadata.
- The daemon must support at least one additional MCP transport beyond `stdio` and
  `streamable-http`.
- New transports must initialize, discover tools, and invoke through the existing daemon
  MCP manager.
- Transport support must not create a second registry, authorization path, or runtime
  invocation plane.
- Operator-visible surfaces must distinguish:
  - transport unsupported
  - transport blocked by prerequisites
  - transport degraded
  - transport runtime failure
  - invocation failure inside an otherwise healthy transport session
- Restart, reconnect, retry, cancellation, and restore semantics must be explicit for every
  supported transport family.

## Compatibility And Operational Notes

- This phase adds transport families, not a new control plane.
- Transport-specific implementation detail may grow underneath the MCP subsystem, but
  daemon-visible resource, policy, and audit surfaces must remain coherent.
- Host prerequisites should be treated as operator-facing contract, not hidden setup.

## Verification

- targeted daemon tests for capability truth, lifecycle, invocation, and recovery behavior
- contract coverage for transport capability and health surfaces
- at least one real end-to-end server on the newly added transport

## Definition Of Done

- MCP supports more than two real transport families without splitting into multiple
  control planes
- transport readiness, failure, and recovery semantics remain inspectable and auditable
- remote MCP support can expand without weakening the current runtime or policy model

## Recommended Speckit Input

```text
$speckit-specify 结合 docs/specs/008-additional-mcp-transports.md 完成 phase 23 的工作
```
