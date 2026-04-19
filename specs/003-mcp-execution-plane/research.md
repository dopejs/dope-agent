# Research: MCP Execution Plane

## Decision 1: Keep the delivery unit exactly on Roadmap 18

- **Decision**: Implement MCP as the next full sandbox-backed harness subsystem: registry,
  lifecycle, credential isolation, tool exposure policy, and operator verification. Do
  not fold Roadmap 19 generic local-tool execution migration or stronger backend work into
  this slice.
- **Rationale**: The roadmap order in `docs/runtime/daemon-roadmaps.md` and
  `docs/harness/sandbox-execution-plane.md` explicitly places MCP after the prerequisite
  declaration and provenance work and before generic tool execution migration. Pulling in
  Roadmap 19 would blur the roadmap boundary and increase blast radius.
- **Alternatives considered**:
  - Start only MCP registry and postpone lifecycle: rejected because Roadmap 18 requires
    daemon-managed execution, not just static metadata.
  - Absorb generic tool orchestration now: rejected because it would turn one roadmap into
    two and reopen the explicit sequencing established in the harness docs.

## Decision 2: Add a dedicated `daemon/internal/mcp` subsystem instead of overloading existing managers

- **Decision**: Introduce a new MCP package that owns server registry, lifecycle state,
  tool catalog, and exposure policy while integrating with existing sandbox, store, app,
  and API layers.
- **Rationale**: The current repository has no MCP package and the existing
  `managedproviders`, `capabilities`, and `connectors` packages each encode consumer- or
  transport-specific semantics that do not map cleanly onto MCP server identity, tool
  inventory, and policy-managed exposure. MCP needs first-class daemon truth, not a loose
  extension of another subsystem.
- **Alternatives considered**:
  - Reuse `managedproviders`: rejected because MCP is not an LLM provider bridge.
  - Reuse `capabilities.Supervisor` directly: rejected because capability supervision tracks
    health and backoff but does not model sandbox binding, secret scope, or tool catalog.

## Decision 3: Persist MCP servers and lifecycle state as first-class daemon resources

- **Decision**: Store MCP server definitions, current lifecycle state, and tool exposure
  policy durably in SQLite so registry, restart behavior, and operator inspection survive
  daemon restarts.
- **Rationale**: The spec requires enabled servers to auto-restart after daemon restart and
  requires blocked restart reasons to remain queryable. That needs more than in-memory
  state; it needs persisted server definitions plus restart-relevant state.
- **Alternatives considered**:
  - Keep MCP server definitions only in config: rejected because operator-driven register,
    inspect, enable, and disable flows would not be durable or auditable enough.
  - Persist only sandbox executions: rejected because lifecycle and exposure truth would be
    lost once no process was currently running.

## Decision 4: Run MCP lifecycle through sandbox subprocess execution, but keep lifecycle state separate from execution records

- **Decision**: Every MCP server start, restart, and stop attempt goes through the
  existing sandbox manager and subprocess backend, while MCP keeps its own higher-level
  lifecycle state for registration, health, backoff, and recovery semantics.
- **Rationale**: Sandbox executions already capture backend choice, policy, approvals, and
  process outcome. MCP still needs a daemon-managed server state model that answers
  operator questions like "is this server enabled", "why is it backing off", and "why was
  restart blocked" even when no subprocess is currently active.
- **Alternatives considered**:
  - Model MCP only as raw sandbox executions: rejected because registry and recovery state
    would be too transient.
  - Start MCP servers outside sandbox and only audit later: rejected because it would
    create the unmanaged side path Roadmap 18 explicitly forbids.

## Decision 5: Auto-restart enabled servers after restart, but fail closed when current policy or config blocks restart

- **Decision**: On daemon startup, previously enabled MCP servers are reloaded from
  persistence and automatically restarted if their current sandbox declaration, profile,
  credentials, and config remain valid. Otherwise they stay non-running with an explicit
  operator-visible blocking reason.
- **Rationale**: This matches the clarification outcome and preserves the "daemon-managed"
  behavior promised by the roadmap without hiding when the environment has drifted or
  credentials are no longer valid.
- **Alternatives considered**:
  - Restore records only and require manual restart: rejected because it would make
    restart semantics dependent on manual operator intervention.
  - Force restart even with degraded or missing policy: rejected because the sandbox
    contract must stay truthful and fail closed when guarantees are not met.

## Decision 6: Expose tools by explicit per-tool, per-runtime-surface allowlisting

- **Decision**: MCP tool exposure is deny-by-default. Each tool must have an explicit
  allowlist decision for each runtime surface, and approval gating attaches at that tool
  exposure layer rather than to routine server lifecycle.
- **Rationale**: MCP introduces potentially broad external capability surfaces. Allowing
  all tools from a healthy server by default would turn registration into accidental
  privilege expansion. Per-tool, per-surface policy keeps exposure reviewable and
  compatible with the current operator trust model.
- **Alternatives considered**:
  - Expose all tools from any healthy server: rejected because newly discovered tools would
    appear without explicit policy review.
  - Gate every server start with manual approval: rejected because lifecycle management
    should stay daemon-owned and restart-safe.

## Decision 7: Reuse declaration-backed secret scope and provenance with a new consumer kind for MCP servers

- **Decision**: Extend the existing sandbox declaration, secret-scope binding, and durable
  policy-record model to cover MCP servers as a new consumer kind. Credential scope is
  authorized per MCP server instance, and operator-visible surfaces identify the addressed
  server plus any reusable default rule that contributed to the decision.
- **Rationale**: Roadmap 17 was explicitly closed to make MCP the next consumer of these
  contracts. Reusing them preserves consistency across managed providers, current tool
  paths, and MCP while keeping least privilege at the server-instance level.
- **Alternatives considered**:
  - Create an MCP-only secret model: rejected because it would duplicate the prerequisite
    work and fracture operator understanding.
  - Share credentials by sandbox profile: rejected because two different MCP servers on the
    same profile should not automatically share secret scope.

## Decision 8: Keep external contracts additive across new MCP routes and existing sandbox surfaces

- **Decision**: Add dedicated MCP resource routes and event types, while also extending
  existing sandbox execution, config inspection, event history, and approval surfaces with
  additive MCP provenance where relevant.
- **Rationale**: Operators need a first-class MCP subsystem, but they also need to inspect
  sandbox execution and policy history in one consistent place. A dedicated MCP API plus
  additive sandbox provenance gives both without rewriting existing route families.
- **Alternatives considered**:
  - Use only existing sandbox routes for MCP inspection: rejected because operators need a
    server-centric registry and lifecycle view, not just process execution history.
  - Build a parallel unmanaged operator surface: rejected because it would recreate the
    fragmentation the control plane is intended to prevent.
