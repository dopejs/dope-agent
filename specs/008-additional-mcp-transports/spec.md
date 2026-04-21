# Feature Specification: Additional MCP Transports

**Feature Branch**: `[009-additional-mcp-transports]`  
**Created**: 2026-04-20  
**Status**: Draft  
**Input**: User description: "docs/harness/harness-architecture.md docs/specs/008-additional-mcp-transports.md"

## Clarifications

### Session 2026-04-20

- Q: phase 23 应该先实现哪一种新增 MCP transport？ → A: 首个新增 transport 选 `websocket`。
- Q: `websocket` transport 的认证边界应该是什么？ → A: `websocket` 需要显式配置的认证材料；不做 anonymous-by-default 提升。
- Q: `websocket` 断线后的 reconnect 语义应该是什么？ → A: daemon 负责有界自动重连，并把 retry 与最终失败历史显式暴露给 operator。
- Q: `websocket` 的认证材料应该如何存储与解析？ → A: 认证材料必须走现有 MCP secret refs 与 redacted resolution，不允许内联泄露到 server 定义。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Inspect Transport Capability Truth (Priority: P1)

As an operator, I need each MCP transport family to expose explicit readiness,
prerequisite, and mismatch truth so I can decide whether a transport is usable on the
current host before I attempt to run a server with it.

**Why this priority**: Transport expansion is unsafe if operators cannot distinguish a
supported transport from one that is blocked, degraded, or unavailable on the current
host.

**Independent Test**: Register or inspect MCP server definitions across the supported
transport families and verify the daemon surfaces explicit readiness, prerequisite, and
unsupported truth without requiring raw logs or code inspection.

**Acceptance Scenarios**:

1. **Given** an MCP transport family is supported on the current host, **When** the
   operator inspects daemon transport surfaces, **Then** the daemon reports that transport
   as ready with explicit prerequisite truth.
2. **Given** an MCP transport family is not usable on the current host, **When** the
   operator inspects daemon transport surfaces, **Then** the daemon reports explicit
   unsupported, blocked, or unavailable truth instead of a generic runtime failure.
3. **Given** a transport family is usable but degraded, **When** the operator inspects it,
   **Then** the daemon distinguishes degraded transport state from invocation failure
   inside an otherwise healthy session.

---

### User Story 2 - Run A Real Server On A New Transport (Priority: P2)

As an operator, I need `websocket` to become the first additional MCP transport family
beyond `stdio` and `streamable-http`, running through the existing daemon-owned MCP
manager so transport expansion does not create a second control path.

**Why this priority**: The roadmap is not complete unless a new transport family actually
uses the same registry, lifecycle, authorization, and invocation surfaces as existing MCP
servers.

**Independent Test**: Register one MCP server on `websocket`, start it,
discover tools, invoke one tool through the existing runtime tool-call plane, and verify
the daemon does not fork into a transport-specific side path.

**Acceptance Scenarios**:

1. **Given** a server uses `websocket`, **When** the operator
   starts it, **Then** the daemon initializes and tracks it through the existing MCP
   registry and lifecycle surfaces.
2. **Given** a running server uses `websocket`, **When** the
   operator discovers or invokes its tools, **Then** the daemon routes discovery and
   invocation through the same authorization, provenance, and audit path used by existing
   MCP servers.
3. **Given** `websocket` encounters a transport-specific failure, **When**
   the daemon reports the outcome, **Then** the operator receives explicit transport truth
   rather than a collapsed generic failure.
4. **Given** a `websocket` endpoint expects authentication, **When** the operator has not
   configured the required auth material, **Then** the daemon reports explicit blocked or
   unavailable transport truth instead of silently attempting anonymous access.
5. **Given** a `websocket` endpoint requires secret-backed authentication, **When** the
   operator inspects the server resource or history, **Then** the daemon exposes only
   redacted secret-resolution truth rather than inline secret material.

---

### User Story 3 - Preserve Recovery And Audit Truth (Priority: P3)

As an operator, I need restart, reconnect, retry, cancellation, and restore behavior for
every supported transport family to remain explicit and auditable so transport growth does
not hide recovery behavior in raw logs.

**Why this priority**: Additional transport families often introduce more session and
recovery complexity; if those semantics are not explicit, operators lose control during
failure handling.

**Independent Test**: Trigger representative recovery scenarios for the new transport
family and confirm the daemon surfaces restart, reconnect, cancellation, and restore truth
through existing event, history, and inspection surfaces.

**Acceptance Scenarios**:

1. **Given** a server on the new transport family loses connectivity or session health,
   **When** the daemon attempts recovery, **Then** the operator can inspect whether the
   daemon reconnected, retried, failed, or stopped.
2. **Given** a transport session is cancelled or interrupted, **When** the operator
   reviews daemon history, **Then** the audit trail distinguishes transport recovery from
   invocation failure and from normal server stop behavior.
3. **Given** the daemon restarts while servers on multiple transport families are enabled,
   **When** restore runs, **Then** transport-specific recovery semantics remain bounded,
   explicit, and visible to operators.

### Edge Cases

- What happens when a transport family is declared in a server definition but the current
  host cannot satisfy its prerequisites?
- How does the daemon classify a transport session that is healthy enough to stay
  connected but temporarily degraded for invocation or discovery work?
- What happens when reconnect or retry succeeds after one or more operator-visible
  failures?
- How does restore behave when a transport family cannot resume or reconnect after daemon
  restart?
- What happens when the new transport family can initialize a session but tool discovery
  or invocation fails afterward?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST expose explicit capability, prerequisite, and readiness
  truth for every supported MCP transport family.
- **FR-002**: The system MUST support `websocket` as the first MCP transport family
  beyond `stdio` and `streamable-http`.
- **FR-003**: New transport families MUST initialize, discover tools, and invoke tools
  through the existing daemon-owned MCP manager.
- **FR-004**: Transport expansion MUST NOT create a second MCP registry, authorization
  path, or runtime invocation plane.
- **FR-004a**: `websocket` transport support MUST require explicit configured
  authentication material whenever the endpoint expects authentication and MUST NOT
  silently elevate anonymous-by-default access.
- **FR-004b**: `websocket` authentication material MUST be stored and resolved through the
  existing MCP secret reference model rather than inline in MCP server definitions.
- **FR-005**: Operator-visible surfaces MUST distinguish transport unsupported, blocked,
  unavailable, degraded, runtime session failure, and invocation failure inside an
  otherwise healthy transport session.
- **FR-006**: Restart, reconnect, retry, cancellation, and restore semantics MUST be
  explicit for every supported transport family.
- **FR-006a**: `websocket` MUST use bounded daemon-managed automatic reconnect rather than
  unbounded retry or purely manual restart semantics.
- **FR-007**: Operator-visible API, event, and history surfaces MUST preserve transport
  identity and transport-specific failure truth for lifecycle, discovery, and invocation
  flows.
- **FR-008**: Transport readiness and failure truth MUST remain environment-scoped so test
  and production MCP resources do not leak state across environments.
- **FR-009**: Transport capability and operator-visible failure surfaces MUST preserve
  existing secret-redaction and audit expectations.
- **FR-010**: `websocket` MUST be verifiable end-to-end on at least one real MCP server
  without bypassing existing daemon lifecycle and runtime surfaces.

### Key Entities *(include if feature involves data)*

- **Transport Capability Record**: Operator-visible transport metadata that explains
  whether a transport family is ready, blocked, unavailable, unsupported, or degraded on
  the current host.
- **Transport-Managed MCP Session**: The daemon-tracked MCP server session for a specific
  transport family, including lifecycle, discovery, invocation, and recovery truth.
- **Transport Recovery Record**: The operator-visible history of reconnect, retry,
  cancellation, restore, or transport failure outcomes for a transport-managed MCP
  session.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: MCP server resources, lifecycle results, capability surfaces,
  events, and operator-visible history gain additive transport capability and recovery
  truth. Existing MCP registry and runtime routes remain additive rather than replaced.
- **Migration / Rollback**: Rollback is a single change-set revert of the new transport
  family and its additive capability and recovery surfaces while preserving the current
  MCP registry, lifecycle, and invocation model. No separate control plane may be
  introduced that would require a parallel rollback path.
- **Verification Strategy**: Run targeted daemon tests for transport capability truth,
  lifecycle, discovery, invocation, recovery, and restore behavior; run contract coverage
  for transport capability and recovery surfaces; record one manual `DOPE_ENV=test`
  end-to-end workflow on the new transport family.
- **Observability Impact**: Operator-visible API resources, events, and history must
  explain transport identity, host readiness, unsupported or blocked truth, degraded or
  failed session state, and reconnect or retry outcomes.
- **Environment & Secrets**: Validation remains in `DOPE_ENV=test` by default. Transport
  capability and lifecycle work must stay environment-scoped and continue to respect
  existing secret-redaction rules on operator-visible surfaces.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can determine whether a transport family is ready, blocked,
  unsupported, degraded, or unavailable in under 5 minutes using committed docs and
  daemon-visible inspection only.
- **SC-002**: At least one real MCP server using `websocket` can be started, inspected,
  and invoked end-to-end in `DOPE_ENV=test` without using any transport-specific control
  path outside the daemon.
- **SC-003**: 100% of validated transport mismatch and prerequisite-loss scenarios surface
  explicit operator-visible classification instead of a generic runtime failure.
- **SC-004**: 100% of validated reconnect, retry, cancellation, and restore scenarios for
  the new transport family remain reconstructable from daemon-visible history and events.
- **SC-005**: 100% of validated `websocket` disconnect scenarios either recover within the
  bounded reconnect policy or surface an explicit terminal failure without requiring raw
  logs.

## Assumptions

- Roadmap 18 and Roadmap 21 MCP registry, lifecycle, install, and invocation behavior are
  already complete and remain the base for this slice.
- Roadmap 22 catalog management is complete and remains additive context for transport
  readiness and operator inspection truth.
- `websocket` is fixed as the first additional transport family for this phase.
- `websocket` reconnect is daemon-managed and bounded; unbounded background recovery is
  out of scope for this slice.
- Transport capability and recovery truth remain subordinate to the current daemon-owned
  runtime, approval, and audit model.
- `websocket` authentication is treated as operator-owned configuration material rather
  than an implicit anonymous fallback.
- `websocket` secret-backed authentication reuses the current MCP secret-resolution and
  redaction model instead of introducing inline credential storage.
- Marketplace transport discovery and non-MCP remote execution control planes remain out
  of scope for this slice.
