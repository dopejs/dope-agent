# Feature Specification: Skill And Local Tool Sandbox Execution

**Feature Branch**: `004-skill-tool-sandbox`  
**Created**: 2026-04-19  
**Status**: Draft  
**Input**: User description: "结合 docs/harness/sandbox-execution-plane.md 和 docs/harness/sandbox-backend-comparison.md ，完成 phase 19 的工作。"

## Clarifications

### Session 2026-04-19

- Q: Which local tool paths are in scope for Roadmap 19 sandbox migration? → A: Only executable skills and the current high-risk local tool path (`exec` / `shell` / `browser`).
- Q: What should happen to in-flight tool or skill executions after daemon restart? → A: Any interrupted in-flight execution is recovered as `cancelled`.
- Q: What is the default approval posture for executable skills when not explicitly declared? → A: Default to `ask`.
- Q: How should invalid executable skills appear in operator-visible skill inspection? → A: Keep them visible as `unavailable` with an explicit reason.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Executable skills declare real execution requirements (Priority: P1)

As an operator exposing executable skills, I need each skill that can launch local work to
declare its sandbox requirements explicitly so skill execution does not reintroduce hidden
filesystem, network, or secret access outside the control plane.

**Why this priority**: Roadmap 19 starts by turning executable skills from descriptive
assets into sandbox-governed execution consumers. Without an explicit execution manifest,
skill execution would bypass the requirement and provenance model that earlier sandbox work
established.

**Independent Test**: Inspect representative executable skills, attempt to load invalid or
unsafe ones, and verify operator-visible skill surfaces show required execution policy for
valid skills while rejecting invalid declarations without falling back to ad hoc execution.

**Acceptance Scenarios**:

1. **Given** a skill is intended to launch local executable work, **When** an operator
   inspects that skill, **Then** the skill exposes an explicit execution requirement
   declaration including required sandbox profile, filesystem scope, network intent, secret
   needs, and approval posture, with undeclared approval posture defaulting to `ask`.
2. **Given** an executable skill declares invalid, unsupported, or unsafe requirements,
   **When** the daemon loads or evaluates that skill, **Then** the skill remains visible
   as `unavailable` with an operator-visible reason instead of silently widening access,
   bypassing sandbox policy, or disappearing from inspection surfaces.

---

### User Story 2 - Local tool and skill execution use one sandbox path (Priority: P2)

As a user invoking executable skills or the current high-risk local tools, I need in-scope
execution to run through sandbox-backed requests so approval, timeout, cancellation,
stdout or stderr capture, and failure behavior are consistent across the harness.

**Why this priority**: The core Roadmap 19 closure is removing the remaining ad hoc local
subprocess path. Registration or metadata alone does not help if actual execution still
escapes the sandbox boundary.

**Independent Test**: Execute representative executable skills and high-risk local tools
(`exec`, `shell`, `browser`) through their real runtime paths, then verify successful
runs, denials, approval-gated runs, timeouts, and cancellations all produce sandbox-backed
operator-visible outcomes.

**Acceptance Scenarios**:

1. **Given** an executable skill or one of the current high-risk local tools has
   satisfied requirements, **When** runtime execution is requested, **Then** the work is
   launched through the sandbox boundary and records the applied policy, backend family,
   and terminal outcome.
2. **Given** an in-scope execution is denied, waits on approval, times out, is cancelled,
   or fails after launch, **When** an operator inspects the result, **Then** those outcome
   classes are distinguishable without reconstructing the path from logs only.
3. **Given** an execution request requires stronger guarantees than the current backend can
   provide, **When** the daemon evaluates the request, **Then** the request is rejected as
   unsupported or denied and the system does not imply stronger isolation was provided.

---

### User Story 3 - Runtime history and sandbox provenance stay linked (Priority: P3)

As an operator debugging a run, I need runtime tool history to link back to sandbox truth
so I can trace which skill or tool launched, which policy applied, and how execution ended
even after restart.

**Why this priority**: Moving execution onto sandbox is only useful if the runtime still
has coherent execution truth. Otherwise Roadmap 19 would improve policy but regress
debuggability.

**Independent Test**: Execute in-scope skill and local tool actions, inspect run and step
history plus sandbox records, restart the daemon around recorded activity, and verify the
execution path remains reconstructable through daemon-visible surfaces.

**Acceptance Scenarios**:

1. **Given** a run step invokes a local tool or executable skill, **When** an operator
   inspects runtime and sandbox history, **Then** the operator can trace the action across
   run, step, tool call, sandbox execution, policy decision, and consumer provenance.
2. **Given** in-scope execution activity exists before daemon restart, **When** the daemon
   recovers and the operator inspects history afterward, **Then** prior execution records
   remain durable and any interrupted in-flight work is recovered as `cancelled` rather
   than left in an indeterminate running state.

---

### User Story 4 - Operators can verify no supported tool path bypasses sandbox (Priority: P4)

As an operator preparing a release, I need documentation and verification evidence that the
supported executable-skill and local-tool paths no longer bypass sandbox so the harness has
one credible local execution boundary before richer orchestration is added.

**Why this priority**: Roadmap 19 is a release boundary, not just an implementation task.
The slice is only credible when docs, contracts, and verification all show the same
execution model.

**Independent Test**: Follow the documented verification flow for executable skills and
local tools, then confirm operator-visible routes, events, runtime history, and sandbox
records describe the same requirement, approval, provenance, and failure behavior.

**Acceptance Scenarios**:

1. **Given** supported executable-skill and local-tool paths are exercised, **When** an
   operator follows the verification workflow, **Then** documentation and daemon-visible
   surfaces agree on requirement declaration, approval behavior, audit linkage, and failure
   classification.
2. **Given** a tool path remains out of scope for this slice, **When** the operator
   reviews the roadmap and operator guidance, **Then** the unsupported or deferred path is
   stated explicitly rather than implied to be sandbox-backed.

### Edge Cases

- What happens when an executable skill manifest references a sandbox profile, secret, or
  filesystem scope that is valid in one environment but unavailable in the current one?
- How does the system behave when a skill or local tool asks for stronger isolation than
  the current backend can truthfully provide?
- What happens when approval is requested for an execution, but the run is cancelled or the
  step is otherwise closed before approval is resolved?
- How does the daemon report a long-running tool or skill process when the daemon restarts
  before that execution finishes, and how is the resulting `cancelled` recovery outcome
  surfaced?
- What happens when captured output or audit-visible metadata contains secret-derived
  material rather than a literal secret value?
- How does the system prevent a supported local tool path from silently falling back to a
  legacy direct subprocess launch if sandbox launch is denied or unavailable?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST define an explicit execution requirement manifest for any
  skill that can launch local executable work.
- **FR-002**: Skill inspection surfaces MUST expose executable-skill requirement
  declarations clearly enough for operators to understand required sandbox profile,
  filesystem scope, network intent, secret needs, and approval posture before execution.
- **FR-002a**: When an executable skill does not explicitly declare its approval posture,
  the system MUST treat the skill as `ask` by default.
- **FR-003**: The system MUST reject or disable executable skills whose declared
  requirements are invalid, unsupported, unsafe, or incomplete, and MUST NOT widen access
  silently to make those skills run.
- **FR-003a**: Invalid, unsupported, unsafe, or incomplete executable skills MUST remain
  visible in operator-facing skill inspection as `unavailable` with an explicit reason.
- **FR-004**: Executable-skill subprocess execution and the current high-risk local tool
  path (`exec`, `shell`, `browser`) MUST run through sandbox-backed execution requests and
  MUST NOT retain a parallel unmanaged local subprocess path.
- **FR-005**: The system MUST preserve one consistent execution outcome model across
  sandbox-backed local tools and executable skills, including success, policy denial,
  approval required, approval rejected, invalid declaration, unsupported backend strength,
  launch failure, process failure, timeout, cancellation, and output-capture failure.
- **FR-006**: Required approval for in-scope local execution MUST be explicit and
  operator-visible, and execution MUST NOT start until the required approval state is
  satisfied.
- **FR-007**: The system MUST preserve explicit stdout, stderr, timeout, cancellation, and
  terminal-status visibility for sandbox-backed tool and skill execution.
- **FR-008**: Runtime tool history MUST link in-scope local tool and executable-skill
  actions back to the related sandbox execution, policy decision, and requesting consumer.
- **FR-009**: Operator-visible records for in-scope execution MUST identify the requesting
  skill or tool, the applied requirement declaration, the backend family used, and the
  terminal outcome without requiring log-only reconstruction.
- **FR-010**: Execution records and recovery behavior for this slice MUST remain durable
  across daemon restart, including recovering any interrupted in-flight tool or skill
  execution as `cancelled` with an explicit operator-visible recovery outcome.
- **FR-011**: Secret values and secret-derived material MUST remain redacted in
  operator-visible output, audit, and history surfaces for sandbox-backed tool and skill
  execution.
- **FR-012**: This slice MUST use the current sandbox backend as the only required backend
  while preserving the contract shape needed for stronger future backends.
- **FR-013**: If an executable skill or local tool declaration requests stronger
  guarantees than the current backend can provide, the system MUST reject the request as
  unsupported or denied and MUST NOT imply that the stronger guarantees were delivered.
- **FR-014**: Changes introduced by this slice MUST remain additive to existing runtime,
  approval, event, and sandbox inspection surfaces so current integrations do not require a
  breaking migration.
- **FR-015**: The system MUST document that this slice covers executable skills and the
  current high-risk local tool path (`exec`, `shell`, `browser`), and MUST identify all
  other local-tool or capability paths that remain explicitly out of scope.
- **FR-016**: The system MUST preserve test-versus-production separation when resolving
  executable-skill requirements, secret access, filesystem scope, and local execution
  context.
- **FR-017**: The system MUST ensure supported local execution paths do not silently fall
  back to direct unmanaged subprocess launch when sandbox policy, backend availability, or
  declaration validation prevents launch.
- **FR-018**: Operator guidance and verification material MUST demonstrate that supported
  executable-skill and local-tool paths use the same sandbox execution boundary and
  surface the same approval, provenance, and failure truth.
- **FR-019**: This slice MUST establish one credible local execution boundary for
  executable skills and in-scope local tools while keeping graph orchestration, additional
  backends, remote execution, memory work, and self-improvement behavior out of scope.

### Key Entities *(include if feature involves data)*

- **Executable Skill Manifest**: The operator-visible declaration attached to a skill that
  can launch local work, describing its required sandbox profile, filesystem scope,
  network intent, secret needs, approval posture, and current validity or `unavailable`
  reason.
- **Sandbox-Backed Tool Execution**: One in-scope local tool or executable-skill run that
  passes through sandbox policy, launch, output capture, cancellation, and terminal
  outcome handling.
- **Execution Provenance Record**: The durable record that links runtime tool history to
  the requesting skill or tool, the applied requirement declaration, the related sandbox
  decision or execution, the backend family, and the classified terminal result.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Runtime tool-call history, sandbox inspection, event, approval,
  and skill-inspection surfaces may gain additive requirement, provenance, and execution
  linkage fields. No intentional breaking change or second local execution plane is in
  scope.
- **Migration / Rollback**: No standalone data migration is expected. Rollout should move
  supported executable-skill and local-tool paths onto sandbox without changing the closed
  sandbox and MCP roadmap boundaries. Rollback remains possible by reverting the local
  execution migration and additive audit linkage while preserving existing sandbox-backed
  provider and MCP behavior.
- **Verification Strategy**: Validate executable-skill manifest parsing and rejection,
  sandbox-backed local tool and skill execution success and failure paths, approval
  behavior, output redaction, runtime-to-sandbox provenance linkage, restart and recovery
  semantics, and contract alignment across API, event, and operator documentation
  surfaces.
- **Observability Impact**: Operator-visible routes, events, runtime history, and sandbox
  records must show enough execution truth to answer what tried to run, who requested it,
  which policy applied, whether approval was required, which backend handled it, how it
  ended, and whether any recovery occurred.
- **Environment & Secrets**: Local verification defaults to the test environment and must
  not rely on production state. Secret access remains explicit and redacted. The current
  backend remains the only required backend and its weaker enforcement posture must stay
  visible to operators.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of supported executable-skill launches and in-scope local tool
  subprocess launches exercised in verification run through sandbox-backed execution, with
  zero documented legacy bypass paths remaining in scope.
- **SC-002**: For every in-scope execution scenario covered in verification, an operator
  can identify the requesting skill or tool, the applied policy outcome, the backend
  family, and the terminal result from daemon-visible surfaces alone within 2 minutes.
- **SC-003**: Supported in-scope local tool and executable-skill actions continue to
  complete successfully in the test environment when their declared requirements and
  required approvals are satisfied.
- **SC-004**: Verification and audit review find no plain-text secret or secret-derived
  value exposure in operator-visible outputs introduced by this slice.
- **SC-005**: Review of operator-visible behavior finds no case where this slice suggests
  stronger filesystem or network isolation than the current backend actually provides.

## Assumptions

- In-scope local execution for this slice is limited to supported executable skills and the
  current high-risk daemon-owned local tool path (`exec`, `shell`, `browser`); MCP server
  lifecycle, broader local capability migration, graph orchestration, and additional
  backends remain separate roadmap work.
- Earlier sandbox roadmap slices already established shared requirement declarations,
  secret-scope handling, provenance vocabulary, sandbox control-plane APIs, and MCP as a
  sandbox-backed subsystem.
- Skill discovery, registry loading, and explicit skill selection already exist; this slice
  adds real execution boundary closure for executable skills rather than a new skill
  discovery model.
- The current backend remains subprocess-based for this slice, so stronger isolation is a
  later roadmap rather than an implied part of this feature.
