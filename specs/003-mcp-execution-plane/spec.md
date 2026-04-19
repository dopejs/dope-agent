# Feature Specification: MCP Execution Plane

**Feature Branch**: `003-mcp-execution-plane`  
**Created**: 2026-04-19  
**Status**: Draft  
**Input**: User description: "结合 docs/harness/sandbox-execution-plane.md 和 docs/harness/sandbox-backend-comparison.md 完成 phase 18 的工作"

## Clarifications

### Session 2026-04-19

- Q: What should the daemon do with previously managed MCP servers after restart? → A: Auto-restart previously enabled MCP servers after daemon restart, subject to current policy and config validity.
- Q: When should MCP tools be exposed to runtime surfaces? → A: Expose MCP tools only while the owning server is enabled, policy-allowed, and healthy.
- Q: Where should approval gating apply for MCP? → A: MCP tools may be approval-gated per tool exposure policy, while server lifecycle remains daemon-managed.
- Q: How should MCP tool exposure be granted? → A: Tools must be explicitly allowlisted per tool and runtime surface before they are exposed.
- Q: How should MCP credential access be authorized? → A: Credential access is authorized per MCP server instance, with optional reusable defaults that never widen access beyond that server.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - MCP servers are registered with explicit sandbox policy (Priority: P1)

As an operator preparing the harness for MCP, I need MCP servers to exist as first-class
daemon-managed resources with explicit sandbox profile and requirement bindings so MCP does
not introduce a hidden execution path outside the control plane.

**Why this priority**: Roadmap 18 starts with registry and policy binding. Without an
explicit MCP resource model, lifecycle, credentials, and tool exposure would inherit
ad hoc behavior that the sandbox work was meant to eliminate.

**Independent Test**: Register and inspect representative MCP servers, then verify each
server exposes an explicit identity, enabled state, sandbox profile, requirement
declaration, and current policy or failure state without relying on implicit process
configuration.

**Acceptance Scenarios**:

1. **Given** an MCP server has been defined for daemon management, **When** an operator
   inspects that server, **Then** the server shows its identity, sandbox profile,
   requirement declaration, and effective execution policy as first-class daemon truth.
2. **Given** an MCP server configuration is invalid or incomplete, **When** the daemon
   evaluates or inspects it, **Then** the daemon reports the server as invalid or not
   launchable with an operator-visible reason instead of falling back to unmanaged launch.

---

### User Story 2 - MCP lifecycle runs through sandbox-managed execution (Priority: P2)

As an operator running MCP servers, I need startup, restart, shutdown, and recovery to run
through sandbox-managed execution so lifecycle behavior, denials, and failures are
classifiable and inspectable like the rest of the harness.

**Why this priority**: Roadmap 18 is not closed by registration alone. The daemon must
own transport lifecycle rather than allowing MCP to become a second process-management
plane.

**Independent Test**: Start, restart, cancel, and stop representative MCP servers and
simulate failure cases, then verify the daemon records sandbox-backed lifecycle outcomes,
distinguishes denial from launch or runtime failure, and preserves explicit state across
restart.

**Acceptance Scenarios**:

1. **Given** a registered MCP server whose declared requirements are satisfied, **When**
   the daemon starts or restarts it, **Then** the lifecycle action runs through the
   sandbox boundary and records the applied policy, backend, and terminal outcome.
2. **Given** an MCP server is denied by policy or fails during launch or transport
   operation, **When** an operator inspects its lifecycle state, **Then** denial, launch
   failure, runtime failure, timeout, and cancellation are distinguishable without log-only
   reconstruction.
3. **Given** the daemon restarts while MCP servers are managed, **When** recovery occurs,
   **Then** previously enabled servers are automatically restarted when current policy and
   configuration remain valid, and any server that cannot be restarted reports an explicit
   operator-visible reason instead of remaining ambiguous.

---

### User Story 3 - MCP credentials and tool exposure stay policy-driven (Priority: P3)

As an operator exposing MCP tools to runtime surfaces, I need MCP credentials and tool
availability to be controlled by explicit sandbox and exposure policy so no server inherits
ambient secrets or exposes tools without an inspectable decision.

**Why this priority**: MCP combines external credentials with tool surfacing. If either is
implicit, the sandbox control plane becomes incomplete at the point where new remote
capability enters the system.

**Independent Test**: Configure secret-backed MCP servers and multiple tool exposure
states, then verify credentials are injected only through declared policy, sensitive values
stay redacted, and operators can explain why each tool is available, blocked, or
approval-gated for each runtime surface, with exposure granted only through explicit
per-tool and per-surface allowlisting.

**Acceptance Scenarios**:

1. **Given** an MCP server requires credentials, **When** the daemon prepares its runtime
   environment, **Then** credentials are provided only through declared sandbox secret and
   environment policy rather than uncontrolled process inheritance, and credential access
   is authorized for the addressed MCP server instance rather than broadly shared across
   unrelated servers.
2. **Given** an MCP tool is not universally available, **When** an operator inspects its
   exposure state, **Then** the system explains whether that tool is available, blocked, or
   approval-gated for each relevant runtime surface, and tools are exposed only while the
   owning server is enabled, policy-allowed, and healthy.
3. **Given** an MCP tool is configured as approval-gated, **When** a runtime surface tries
   to use it, **Then** the daemon requires approval for that tool use without turning
   normal server startup, restart, or shutdown into a manual approval workflow.
4. **Given** a newly registered MCP server or a server whose tool set changes, **When**
   the operator has not explicitly allowlisted a tool for a runtime surface, **Then** that
   tool remains unexposed by default.
5. **Given** operator-visible configuration, event, or history surfaces include
   credential-backed MCP activity, **When** those surfaces are rendered, **Then** secret
   values and secret-derived material remain redacted while the requested scope and
   exposure decision remain understandable.

---

### User Story 4 - Operators can verify MCP and sandbox stay aligned (Priority: P4)

As an operator validating a release, I need MCP-specific docs, audit surfaces, and
verification coverage to prove MCP executes through sandbox rather than through a side path
that only appears safe in design documents.

**Why this priority**: A daemon-managed MCP subsystem is only credible if the observable
contracts, docs, and verification evidence all show the same execution boundary.

**Independent Test**: Run the documented MCP verification flow, then confirm the daemon's
operator-visible surfaces, emitted records, and documentation all describe the same
registry, policy, lifecycle, credential, and tool exposure behavior.

**Acceptance Scenarios**:

1. **Given** MCP server configuration and lifecycle activity exist, **When** an operator
   follows the verification workflow, **Then** documentation and operator-visible surfaces
   agree on profile binding, credential handling, lifecycle state, and failure visibility.
2. **Given** an MCP-backed tool becomes available or blocked, **When** its related records
   are inspected, **Then** the operator can trace the decision back to the owning MCP
   server, its policy binding, and the relevant lifecycle or policy outcome.

### Edge Cases

- What happens when an MCP server definition references a sandbox profile or requirement
  declaration that no longer exists or is inactive?
- How does the system behave when a server is configured correctly but the current backend
  cannot provide a stronger isolation guarantee the declaration requires?
- What happens when credential references are valid in production but unavailable in the
  test environment?
- How does the daemon report an MCP server that is registered but intentionally disabled,
  unhealthy, or waiting on approval?
- What happens when a previously enabled MCP server cannot be auto-restarted after daemon
  restart because policy, credentials, or configuration are no longer valid?
- What happens when a server starts successfully but its transport becomes unhealthy after
  launch?
- How does the system prevent a tool from remaining exposed after the owning MCP server is
  disabled, denied, or no longer healthy?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST represent MCP servers as first-class daemon-managed
  resources that can be registered, inspected, configured, enabled, disabled, and queried
  without relying on unmanaged local process state.
- **FR-002**: Each MCP server MUST bind to an explicit sandbox profile and execution
  requirement declaration before it is eligible to launch.
- **FR-003**: Operator-visible inspection for each MCP server MUST expose its identity,
  configured intent, effective sandbox policy, enabled state, and current lifecycle or
  failure state.
- **FR-004**: MCP server startup, restart, shutdown, and cancellation MUST run through
  sandbox-managed execution paths and MUST NOT introduce a parallel unmanaged process model.
- **FR-005**: The system MUST classify MCP lifecycle outcomes distinctly enough for
  operators to tell policy denial, approval gating or rejection, invalid configuration,
  launch failure, runtime transport failure, timeout, cancellation, and recovery state
  apart.
- **FR-006**: The daemon MUST make recovery behavior for previously managed MCP servers
  explicit and operator-visible after restart.
- **FR-006a**: After daemon restart, the system MUST automatically attempt to restart MCP
  servers that were enabled before restart, and any server that is not restarted MUST
  expose the blocking policy, credential, or configuration reason through operator-visible
  state.
- **FR-007**: MCP credentials MUST be injected only through explicit sandbox secret and
  environment policy rather than uncontrolled process inheritance.
- **FR-007a**: Credential access for MCP in this slice MUST be authorized per MCP server
  instance, with reusable defaults allowed only when they do not widen access beyond the
  addressed server instance.
- **FR-008**: The system MUST preserve test-versus-production separation when resolving MCP
  credentials, filesystem scope, and execution context.
- **FR-009**: The system MUST define a tool exposure policy that identifies which MCP tools
  are available to which runtime surfaces and whether approval is required.
- **FR-009a**: MCP tool exposure in this slice MUST require explicit allowlisting by tool
  and runtime surface; tools that lack an explicit allowlist decision MUST remain
  unexposed by default.
- **FR-010**: The system MUST provide operator-visible explanation for why an MCP tool is
  available, blocked, approval-gated, or unavailable.
- **FR-010a**: Approval gating for MCP in this slice MUST apply at the individual tool
  exposure policy layer, while normal server lifecycle actions remain daemon-managed and
  do not require per-action operator approval by default.
- **FR-011**: The system MUST ensure MCP tools are not exposed when the owning server is
  disabled, denied, misconfigured, or not in a state that can safely serve that tool.
- **FR-011a**: MCP tools MUST be exposed to runtime surfaces only while the owning server
  is enabled, policy-allowed, and healthy enough to serve requests.
- **FR-012**: Operator-visible configuration, event, and history surfaces MUST redact
  secret values and secret-derived material while still identifying requested secret scope
  and credential resolution outcome.
- **FR-012a**: Operator-visible credential-scope metadata MUST identify the addressed MCP
  server instance and whether any reusable default contributed to the decision.
- **FR-013**: The system MUST record durable provenance linking MCP server identity,
  sandbox profile, requirement declaration, lifecycle outcome, and tool exposure decisions
  so operators can debug without reconstructing state from logs only.
- **FR-014**: Tool exposure and lifecycle inspection MUST let operators trace an exposed or
  blocked MCP tool back to the owning server and the relevant policy or lifecycle record.
- **FR-015**: Changes introduced by this slice MUST remain additive to existing
  operator-visible surfaces so current integrations do not require a breaking migration.
- **FR-016**: This slice MUST use the current sandbox backend as the only required backend
  while preserving the contract shape needed for future stronger or remote backends.
- **FR-017**: If an MCP server declaration requires stronger guarantees than the current
  backend can provide, the system MUST reject that request as unsupported or denied and
  MUST NOT imply the stronger guarantees were provided.
- **FR-018**: The system MUST document the in-scope MCP lifecycle, credential, exposure,
  and audit behavior for this slice and MUST identify remaining out-of-scope items
  explicitly.
- **FR-019**: Multi-backend placement beyond the current backend, full orchestration across
  multiple tools, and stronger browser or desktop isolation MUST remain out of scope for
  this slice.
- **FR-020**: Verification for this slice MUST prove that managed MCP lifecycle activity
  executes through sandbox-backed paths rather than through consumer-specific launch logic.

### Key Entities *(include if feature involves data)*

- **MCP Server Resource**: A daemon-managed definition of one MCP server, including its
  identity, enabled state, sandbox profile binding, execution requirement declaration,
  credential scope, and lifecycle status.
- **MCP Lifecycle Record**: The durable operator-visible record for one MCP lifecycle
  action or recovery outcome, including the applied sandbox policy, backend, failure class,
  and terminal state.
- **MCP Tool Exposure Policy**: The rules and current decision state that determine whether
  a tool from a given MCP server is available, blocked, or approval-gated for a runtime
  surface.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Existing operator-visible APIs, events, and inspection surfaces
  may gain additive MCP resource, lifecycle, credential, and tool exposure fields. No
  intentional breaking change or unmanaged second execution plane is in scope.
- **Migration / Rollback**: No standalone data migration is assumed. Rollout should allow
  MCP management to be introduced behind explicit server registration and policy binding.
  Rollback remains possible by reverting MCP-specific control-plane changes while
  preserving the existing sandbox substrate and pre-MCP requirement-contract work.
- **Verification Strategy**: Validate registration and inspection behavior, sandbox-backed
  lifecycle transitions, credential redaction, tool exposure policy, and restart-aware
  recovery behavior; confirm operator-visible surfaces and documentation stay aligned and
  prove there is no side-path MCP launch flow.
- **Observability Impact**: Operator-visible surfaces must identify MCP server identity,
  sandbox profile, requirement declaration, lifecycle state, failure classification,
  credential resolution outcome, and tool exposure decision clearly enough for production
  debugging without log-only investigation.
- **Environment & Secrets**: Local verification defaults to the test environment and must
  preserve separation from production state. MCP credentials remain operator-owned material
  and must never appear in plain text through updated configuration, history, or event
  surfaces. The current sandbox backend remains the only required execution backend for
  this slice.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can register and inspect 100% of in-scope MCP servers with an
  explicit sandbox profile, requirement declaration, lifecycle state, and credential scope
  using daemon-visible surfaces alone.
- **SC-002**: In verification, 100% of MCP start, restart, shutdown, and cancellation
  actions execute through sandbox-backed lifecycle paths with zero documented unmanaged
  launch exceptions in scope.
- **SC-003**: For every representative denial or failure exercised during verification,
  operators can identify the MCP server, failure class, and applied policy from
  operator-visible surfaces within 2 minutes without reading raw logs.
- **SC-004**: Verification finds zero plain-text MCP credential exposure in updated
  configuration, event, or history surfaces.
- **SC-004a**: Verification finds zero cases where one MCP server instance can inherit or
  use credential scope that was authorized only for a different server instance.
- **SC-005**: Review of exposed and blocked MCP tools finds 100% have an inspectable
  server owner, exposure decision, and approval state.
- **SC-005a**: Verification finds zero cases where a tool remains exposed after its owning
  MCP server becomes disabled, denied, unhealthy, or otherwise unable to serve requests.
- **SC-005b**: Verification finds zero cases where routine MCP server lifecycle actions
  are blocked solely because tool-level approval is required.
- **SC-005c**: Verification finds zero cases where a newly registered or newly discovered
  MCP tool becomes exposed to any runtime surface without an explicit allowlist decision.
- **SC-006**: Release-readiness review finds no operator-visible wording or behavior that
  suggests stronger backend guarantees than the current sandbox backend actually provides.
- **SC-007**: In restart verification, 100% of previously enabled MCP servers either
  resume automatically or report an explicit operator-visible reason why restart was
  blocked.

## Assumptions

- This slice applies to daemon-managed MCP servers launched and supervised by the harness,
  not to externally managed remote MCP services that the daemon does not control.
- The shared sandbox requirement declaration, secret-scope, and provenance work from the
  prerequisite roadmap remains the contract foundation for MCP in this slice.
- The current sandbox backend remains the only backend required to implement or verify this
  roadmap slice, but MCP-facing contracts must stay compatible with future stronger or
  remote backends.
- Credential authorization is expected to be evaluated per MCP server instance even when
  reusable defaults exist for the same server kind or policy family.
- Tool exposure policy applies to the runtime surfaces already present in the repository;
  broader orchestration and planner behavior remain later-roadmap work.
- Tool availability is expected to track the current enabled, policy, and health state of
  the owning MCP server rather than registry presence alone.
- Exposure defaults are expected to be deny-by-default until a tool is explicitly
  allowlisted for a runtime surface.
- Approval requirements are expected to attach to specific MCP tool exposure decisions
  rather than to routine server lifecycle management.
- If stronger isolation is required than the current backend can deliver, fail-closed
  behavior is acceptable for this slice and silent degradation is not.
- Previously enabled MCP servers are expected to auto-restart after daemon restart when
  their current policy, credentials, and configuration remain valid.
