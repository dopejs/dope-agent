# Feature Specification: Sandbox Stronger Backends

**Feature Branch**: `005-sandbox-stronger-backends`  
**Created**: 2026-04-19  
**Status**: Draft  
**Input**: User description: "安装上面讨论的结果，继续推荐sandbox的工作"

## Clarifications

### Session 2026-04-19

- Q: Which stronger isolation-capable backend should this slice implement first? → A: `docker`
- Q: Which consumer should be the first stronger-backend verification target? → A: executable skills
- Q: What should happen when `docker` is required but unavailable on the host? → A: Return `unsupported`
- Q: Which executable skills should migrate to `docker` in this slice? → A: Only explicitly declared `docker` skills

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Operators Can Inspect Backend Guarantees (Priority: P1)

As an operator deciding whether a local execution path is safe enough, I need sandbox
inspection and documentation to make backend guarantees, gaps, and degradation behavior
explicit so I do not have to rely on tribal knowledge when choosing or approving a backend.

**Why this priority**: The next sandbox step fails if the team cannot explain what
subprocess does, what a stronger backend adds, and when subprocess is insufficient.
Without that contract, later migration work becomes guesswork.

**Independent Test**: Inspect sandbox profiles, explain results, and operator guidance for
representative workloads, then verify the operator can determine whether the current
backend is sufficient, insufficient, or unavailable without reading implementation code.

**Acceptance Scenarios**:

1. **Given** a workload requires stronger isolation than the baseline backend can provide,
   **When** an operator inspects the requirement and explain output, **Then** the system
   states clearly that the request requires stronger guarantees and does not imply that the
   baseline backend can satisfy them.
2. **Given** multiple sandbox backends exist with different enforcement properties,
   **When** an operator inspects backend guidance, **Then** the operator can see the
   guarantees, prerequisites, and tradeoffs of each backend in one consistent place.

---

### User Story 2 - Higher-Risk Consumers Can Require Stronger Isolation (Priority: P2)

As an operator onboarding a higher-risk local consumer, I need at least one stronger
isolation-capable backend beyond subprocess so I can require stronger filesystem or
network enforcement without redesigning the sandbox control plane.

**Why this priority**: Sandbox is useful today, but the current subprocess backend is not
the end state for higher-risk execution. The next roadmap must prove that stronger
isolation can be added without breaking the existing control-plane contract.

**Independent Test**: Configure a representative executable skill to use or require the
`docker` backend, then verify successful runs, denials, and backend-unavailable paths all
surface through the existing sandbox boundary with truthful operator-visible outcomes.

**Acceptance Scenarios**:

1. **Given** a consumer is marked as requiring stronger isolation, **When** the stronger
   backend is available and the request is launched, **Then** the execution runs through
   the existing sandbox control plane and records the applied backend and outcome.
2. **Given** a consumer requires stronger isolation but the stronger backend is unavailable
   or cannot satisfy the request, **When** execution is attempted, **Then** the request
   fails explicitly as `unsupported` rather than silently falling back to a weaker
   backend.
3. **Given** an executable skill does not explicitly declare `docker`, **When** this slice
   is deployed, **Then** that skill remains on the baseline backend unless a later change
   updates its declared backend requirement.

---

### User Story 3 - Teams Can Continue Sandbox Migration Without Losing Context (Priority: P3)

As a team continuing sandbox work later, I need the remaining migration inventory,
backend capability matrix, degradation rules, and operator prerequisites captured in
durable artifacts so future implementation does not depend on remembering prior design
conversations.

**Why this priority**: The main operational risk is not just backend implementation. It is
losing the reasoning and migration boundaries needed to continue safely after this round.

**Independent Test**: Review the roadmap artifacts and operator guidance after the feature
is specified, then verify a new engineer can identify which consumers are already
sandbox-backed, which remain out of scope, what stronger backend work remains, and what
host prerequisites must be satisfied.

**Acceptance Scenarios**:

1. **Given** a future engineer needs to continue sandbox work, **When** they inspect the
   roadmap artifacts, **Then** they can identify the remaining consumers, the next backend
   goals, and the migration sequence without reconstructing decisions from commit history.
2. **Given** a consumer remains out of scope for this slice, **When** the operator reviews
   roadmap and migration guidance, **Then** the deferred path is stated explicitly instead
   of being implied to be sandbox-hardened already.

### Edge Cases

- What happens when a stronger backend is required by policy but is not installed or not
  healthy on the host where the daemon runs?
- How does the system behave when a profile requires stronger guarantees for filesystem or
  network control than any currently available backend can provide?
- What happens when a stronger backend can start work but cannot satisfy one declared
  access rule at launch time?
- How does restart recovery work when an execution using a stronger backend is interrupted
  during daemon shutdown or crash recovery?
- How does the operator distinguish “policy denied”, “backend unavailable”, “capability
  mismatch”, and “consumer runtime failure” without reading logs only?
- What happens when a consumer is marked as a migration candidate but its host
  prerequisites are not yet satisfied in the current environment?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST define explicit operator-visible capability metadata for each
  supported sandbox backend, including the isolation guarantees and limits relevant to
  filesystem, network, environment injection, approval, and recovery behavior.
- **FR-002**: The system MUST allow sandbox profiles or consumer requirements to express
  when baseline subprocess execution is insufficient and stronger isolation is required for
  the request to proceed truthfully in this slice.
- **FR-002a**: Executable skills MUST only use `docker` in this slice when their declared
  requirement explicitly selects or requires `docker`; unmodified executable skills MUST
  remain on their existing baseline backend behavior.
- **FR-003**: Explain and inspection surfaces MUST state clearly when a request can run on
  the selected backend, when it requires a stronger backend, and when no available backend
  can satisfy the declared guarantees.
- **FR-004**: The system MUST support at least one stronger isolation-capable sandbox
  backend beyond subprocess while preserving the existing sandbox control-plane model, and
  the first such backend in this slice MUST be `docker`.
- **FR-005**: The common sandbox execution contract MUST remain stable across backends so
  operators and consumers do not need a separate execution plane for stronger isolation.
- **FR-006**: At least one supported backend in this slice MUST provide materially stronger
  filesystem or network enforcement than the baseline subprocess backend.
- **FR-007**: Operator-visible outcomes MUST distinguish policy denial, backend
  unavailability, backend capability mismatch, launch failure, runtime failure, timeout,
  cancellation, and restart-recovery outcomes across supported backends.
- **FR-008**: The system MUST allow at least one real higher-risk consumer to use or
  require the `docker` backend through the existing sandbox execution boundary, and the
  first verification target in this slice MUST be executable skills.
- **FR-009**: The system MUST preserve test-versus-production separation, secret handling,
  approval behavior, and audit visibility when stronger backend execution is used.
- **FR-010**: The system MUST NOT silently fall back from a stronger-required request to a
  weaker backend.
- **FR-010a**: When `docker` is required but unavailable on the current host, the system
  MUST return `unsupported` rather than degrading to subprocess, entering a pending state,
  or reporting the outcome as a normal consumer runtime failure.
- **FR-011**: Operator guidance MUST compare supported backends, including subprocess and
  `docker`, with their guarantees, prerequisites, degradation behavior, and rollout
  tradeoffs.
- **FR-012**: The roadmap artifacts for this slice MUST include an explicit backend
  capability matrix, a remaining consumer inventory, degradation rules, and host
  prerequisites needed for future sandbox work.
- **FR-013**: The roadmap artifacts MUST identify which local consumers are already
  sandbox-backed, which are candidates for stronger-backend migration, and which remain
  explicitly out of scope after this slice.
- **FR-014**: Restart and recovery behavior for stronger-backend executions MUST remain
  explicit and operator-visible through the existing daemon surfaces.
- **FR-015**: Changes introduced by this slice MUST remain additive to current sandbox,
  runtime, approval, and inspection surfaces so current integrations do not require a
  breaking migration.
- **FR-016**: Verification for this slice MUST prove that a stronger backend can carry at
  least one real consumer through the same sandbox control plane without unmanaged bypass
  paths.

### Key Entities *(include if feature involves data)*

- **Backend Capability Matrix**: The operator-visible summary of each supported sandbox
  backend’s guarantees, limits, prerequisites, and degradation semantics.
- **Backend Selection Rule**: The decision data that determines whether a request may use
  subprocess, must use a stronger backend, or must be rejected because no backend can
  satisfy the declared guarantees.
- **Migration Candidate Inventory**: The durable record of which local consumers are
  already converged on sandbox, which need stronger isolation later, and which remain out
  of scope for the current slice.
- **Stronger-Backend Execution Record**: The sandbox execution record for work launched on
  a stronger backend, including backend identity, policy outcome, and recovery state.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Sandbox profile inspection, explain semantics, execution
  records, operator docs, and roadmap artifacts will gain additive backend-capability and
  migration-readiness information. Existing route families remain in place.
- **Migration / Rollback**: Rollout adds a stronger backend and selected consumer usage on
  top of the current sandbox control plane. Rollback reverts stronger-backend selection and
  consumer migration while preserving the existing subprocess-backed sandbox plane.
- **Verification Strategy**: Required validation includes targeted sandbox, runtime,
  contract, and restart coverage, plus end-to-end verification for at least one real
  consumer using the stronger backend and explicit capability-mismatch rejection paths.
- **Observability Impact**: Operator-visible docs, inspection surfaces, and audit records
  must explain backend identity, capability mismatch, stronger-backend prerequisites, and
  migration status for remaining consumers.
- **Environment & Secrets**: Test and production environments must preserve separate
  backend prerequisites, secret resolution, and host capability checks. No stronger-backend
  work may weaken existing secret-scope or environment-separation rules.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can determine the required backend, degradation path, and host
  prerequisite status for a representative workload in under 5 minutes using committed
  documentation and daemon-visible inspection only.
- **SC-002**: At least one higher-risk consumer can be verified end-to-end on a stronger
  backend through the existing sandbox plane with no documented unmanaged bypass path in
  scope.
- **SC-003**: 100% of validated stronger-backend mismatch scenarios surface an explicit
  operator-visible outcome class rather than ambiguous failure or silent fallback.
- **SC-004**: The committed migration inventory covers all current sandbox consumer
  families and identifies which remaining local capability families are deferred beyond
  this slice.
- **SC-005**: Verification demonstrates that interrupted stronger-backend executions
  recover to an explicit terminal state rather than remaining operator-visible as running
  indefinitely.

## Assumptions

- The current subprocess-backed sandbox control plane remains the baseline and is not
  removed by this slice.
- Only one stronger isolation-capable backend is required to close the next roadmap, and
  that backend is `docker`; wider multi-backend expansion can come later.
- The first stronger-backend migration target is executable skills; migration of high-risk
  local tools, MCP servers, and managed providers to `docker` can be evaluated later.
- If a workload requires `docker` and the host cannot provide it, the canonical outcome is
  `unsupported`, not degraded subprocess execution.
- This slice does not auto-promote existing executable skills to `docker`; stronger-backend
  usage is opt-in through explicit declaration.
- Only a limited set of higher-risk consumers need to migrate in this slice; broad
  migration of lower-risk local capability paths remains a later roadmap.
- The project will continue to use the existing sandbox, runtime, approval, and event
  surfaces rather than creating a second execution control plane.
- VM-grade isolation, fleet-scale remote execution, memory systems, and orchestration
  planning remain out of scope for this slice.
