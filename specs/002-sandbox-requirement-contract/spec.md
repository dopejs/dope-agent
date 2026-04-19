# Feature Specification: Sandbox Requirement Declaration Contract

**Feature Branch**: `002-sandbox-requirement-contract`  
**Created**: 2026-04-19  
**Status**: Draft  
**Input**: User description: "结合 docs/harness/sandbox-execution-plane.md 和 docs/harness/sandbox-backend-comparison.md 完成 phase 17 的后续工作"

## Clarifications

### Session 2026-04-19

- Q: What implementation boundary should this phase enforce for existing consumer families? → A: All current consumer families already present in the repository must adopt the shared contract on their real current paths in this phase; future or not-yet-delivered consumer families remain out of scope.
- Q: Which current consumer families must be mandatory adopters in this phase? → A: Managed providers, current skill registry and explicit skill-selection surfaces, and the existing daemon-owned high-risk tool-call path (`exec`, `shell`, `browser`) must adopt the shared contract in this phase; MCP remains contract-compatible but its runtime lifecycle stays out of scope.
- Q: How should the system handle declarations that require stronger guarantees than the current backend can provide? → A: The system must reject those requests as unsupported or denied; it must not silently degrade them onto the current backend.
- Q: At what granularity should secret scope be authorized? → A: Secret scope should be authorized per consumer instance, with reusable default rules allowed within the same consumer kind.
- Q: Should denied or preflight-only policy decisions be stored durably even when no process starts? → A: Yes. All meaningful policy decisions, including denied and preflight-only paths, must produce durable provenance records even when no process is launched.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Shared execution requirements become explicit (Priority: P1)

As an operator preparing the harness for MCP and future local consumers, I need every
sandbox-consuming surface to describe its execution requirements through one shared
declaration contract so policy, approval, and audit behavior no longer depend on
consumer-specific conventions.

**Why this priority**: This is the direct prerequisite called out by Roadmap 17. Without a
shared requirement contract, future consumers would keep adding one-off execution and
access rules.

**Independent Test**: Inspect each mandatory adopter in this phase, then verify managed
providers, current skill registry and explicit skill-selection surfaces, and the existing
daemon-owned high-risk tool-call path all use the shared declaration contract to express
required execution mode, backend intent, filesystem scope, network scope, and secret needs
while producing consistent policy explanations through their real current paths.

**Acceptance Scenarios**:

1. **Given** each mandatory adopter in this phase, **When** an operator inspects its
   declared execution requirements, **Then** managed providers, current skill registry and
   explicit skill-selection surfaces, and the existing daemon-owned high-risk tool-call
   path use the same core fields and vocabulary across consumer kinds and attach that
   declaration to the real current path for that consumer.
2. **Given** a consumer requests execution or local access outside its declared
   requirements, **When** the daemon evaluates the request, **Then** the outcome is
   surfaced as a policy or contract failure instead of consumer-specific hidden behavior.

---

### User Story 2 - Secret scope and redaction stay explicit and safe (Priority: P2)

As an operator managing credentials, I need secret scope and redaction rules to be explicit
by consumer and environment so only permitted secret material reaches a consumer and
operator-visible surfaces never expose raw sensitive values.

**Why this priority**: Roadmap 17 stays blocked for MCP until secret exposure rules are
explicit, explainable, and separated by environment.

**Independent Test**: Exercise secret-backed declarations across test and production-like
contexts, then verify allowed secret references resolve only in permitted scopes while
configuration, explanation, event, and history surfaces keep values redacted.

**Acceptance Scenarios**:

1. **Given** a consumer declaration that references secret material, **When** the daemon
   resolves the execution context, **Then** only secrets permitted for that consumer
   instance and environment are made available, while reusable defaults may be inherited
   from the consumer kind.
2. **Given** an operator inspects configuration, explanation output, or event history for
   a secret-backed action, **When** secret-bearing data is present, **Then** secret values
   and secret-derived material remain redacted while the requested scope and resolution
   outcome remain understandable.

---

### User Story 3 - Consumer provenance is durable and queryable (Priority: P3)

As an operator debugging sandbox activity, I need each execution and policy decision to
identify the requesting consumer so I can trace outcomes back to the right skill, provider
bridge, MCP server, or tool instance even after restart.

**Why this priority**: Provenance is required to debug a shared execution plane once more
than one consumer family uses it.

**Independent Test**: Run representative sandbox-backed operations, restart the daemon, and
verify inspection surfaces still show consumer kind, consumer identity, effective
requirement declaration, backend intent, approval state, and terminal outcome.

**Acceptance Scenarios**:

1. **Given** multiple consumer types share the sandbox plane, **When** an operator inspects
   execution history or explanation output, **Then** each record identifies consumer kind
   and consumer instance alongside the applied requirement declaration and backend intent.
2. **Given** sandbox activity has been recorded before a daemon restart, **When** an
   operator queries history afterward, **Then** provenance fields remain durable and
   queryable without reconstructing them from logs, including denied and preflight-only
   decisions that never launched a process.

### Edge Cases

- What happens when a consumer declares no secrets but ambient environment inheritance would
  otherwise expose sensitive values?
- How does the system behave when multiple consumers share the same sandbox profile but have
  different secret scopes or provenance identities?
- What happens when a declaration requests a backend intent or execution mode that is not
  yet implemented by the current system or requires stronger guarantees than the current
  backend can provide?
- How does the system report a required secret that is valid in production but unavailable
  in the test environment?
- What happens when a request is denied during policy or preflight evaluation and no process
  is launched?
- How does the system redact output that contains values derived from a secret rather than
  the literal secret itself?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST define one shared execution requirement declaration contract
  that can be used by skills, provider bridges, MCP servers, and future local tools.
- **FR-002**: The shared declaration contract MUST identify consumer kind and consumer
  instance independently from the sandbox profile or backend intent.
- **FR-003**: The shared declaration contract MUST express required execution mode, backend
  intent, filesystem scope, network scope, and secret requirements.
- **FR-004**: The system MUST use the shared declaration contract to explain why a request
  is allowed, approval-gated, denied, or unsupported, rather than relying on
  consumer-specific heuristics.
- **FR-005**: Every current consumer family already present in the repository and covered
  by this slice MUST adopt the shared contract on its real runtime path and MUST NOT rely
  on undeclared execution, filesystem, network, or secret access.
- **FR-005a**: Mandatory adopters for this slice MUST include managed providers,
  current skill registry and explicit skill-selection surfaces, and the existing
  daemon-owned high-risk tool-call path (`exec`, `shell`, `browser`).
- **FR-006**: The system MUST keep the currently implemented backend as the only required
  execution backend for this slice while preserving the contract shape needed for stronger
  future backends.
- **FR-007**: The system MUST make clear when a declaration asks for stronger guarantees
  than the current backend can deliver, MUST reject the request as unsupported or denied,
  and MUST NOT imply those guarantees were provided.
- **FR-008**: The system MUST define secret scope by consumer and environment so operators
  can determine whether a secret may be exposed to a given consumer instance in test,
  production, or both.
- **FR-009**: The system MUST support secret access by explicit reference or equivalent
  declared secret identity rather than uncontrolled environment inheritance.
- **FR-009a**: The system MUST allow reusable secret-scope defaults within the same
  consumer kind without widening authorization beyond the addressed consumer instance.
- **FR-010**: The system MUST redact secrets and secret-derived values from operator-visible
  configuration, explanation, event, and history surfaces.
- **FR-011**: The system MUST preserve enough operator-visible metadata for users to
  understand which secret scopes were requested and whether they were resolved, denied, or
  unavailable without revealing secret values.
- **FR-011a**: Operator-visible secret-scope metadata MUST identify the addressed consumer
  instance and whether any default rule from its consumer kind contributed to the decision.
- **FR-012**: The system MUST record execution provenance that identifies the requesting
  consumer kind, consumer instance, and effective requirement declaration for each
  meaningful sandbox decision or execution outcome.
- **FR-013**: Provenance records for this slice MUST remain durable across restart and
  queryable through operator-visible inspection surfaces.
- **FR-014**: The system MUST support meaningful provenance and policy outcomes even when a
  request is denied or satisfied through preflight evaluation without launching a process,
  and denied and preflight-only decisions MUST create durable provenance records even when
  no process is launched.
- **FR-015**: The system MUST keep new requirement-contract, secret-scope, and provenance
  details additive to existing operator-visible surfaces so current integrations remain
  backward-compatible.
- **FR-016**: The system MUST document which current consumer families already present in
  the repository are brought into full conformance by this slice and which future or
  not-yet-delivered consumer families remain explicitly out of scope.
- **FR-016a**: MCP server runtime lifecycle MUST remain explicitly out of scope for this
  slice even though its future contract shape must stay compatible with the shared model.
- **FR-017**: This slice MUST establish the prerequisite requirement, secret-scope, and
  provenance contract across all current consumer families already present in the
  repository, while leaving future MCP lifecycle or tool exposure management out of scope
  for the same change.
- **FR-018**: The system MUST preserve test-versus-production separation when evaluating
  declarations, resolving secret scope, and presenting provenance.
- **FR-019**: The system MUST keep sandbox as the single daemon-owned execution boundary
  for consumers covered by this slice and MUST NOT introduce a second unmanaged
  consumer-specific execution plane.

### Key Entities *(include if feature involves data)*

- **Execution Requirement Declaration**: A shared description of one consumer's execution
  needs, including consumer identity, execution mode, backend intent, filesystem scope,
  network scope, secret requirements, and approval expectations.
- **Secret Scope Binding**: The rule set that determines whether a declared secret may be
  exposed to a specific consumer instance and environment, optionally through reusable
  defaults within the same consumer kind, plus how that access is explained and redacted in
  operator-visible surfaces.
- **Consumer Provenance Record**: The durable operator-visible record that ties a sandbox
  decision or execution outcome back to the requesting consumer kind, consumer instance,
  effective requirement declaration, backend intent, approval state, and terminal result,
  including denied and preflight-only outcomes with no launched process.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Existing operator-visible inspection, explanation, event, and
  history surfaces across all current consumer families already present in the repository
  may gain additive requirement-contract, secret-scope, and provenance fields. No
  intentional breaking change or new parallel execution plane is in scope.
- **Migration / Rollback**: No standalone data migration is expected. Rollout should allow
  consumers to adopt the shared contract incrementally. Rollback remains possible by
  reverting new contract-enforcement and additive visibility changes while preserving the
  already-completed sandbox control plane and managed-provider convergence slice.
- **Verification Strategy**: Validate that managed providers, current skill registry and
  explicit skill-selection surfaces, and the existing daemon-owned high-risk tool-call path
  adopt the shared declaration shape on their real current paths; add environment-scoped
  secret-scope and redaction regression coverage; add restart and durability verification
  for provenance; confirm operator-visible explanations remain truthful about current
  backend guarantees.
- **Observability Impact**: Operator-visible surfaces must identify consumer kind,
  consumer instance, effective requirement declaration, backend intent, approval outcome,
  secret-scope resolution, and terminal result clearly enough to debug cross-consumer
  behavior without log-only investigation, even for denied or preflight-only paths.
- **Environment & Secrets**: Local verification defaults to the test environment and must
  preserve test-versus-production separation. Secret material remains operator-owned and
  must never be exposed in plain text through updated surfaces. The current local backend
  remains the only required execution backend for this slice.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can represent 100% of required execution, filesystem, network, and
  secret intent for managed providers, current skill registry and explicit skill-selection
  surfaces, and the existing daemon-owned high-risk tool-call path using the shared
  declaration contract without consumer-specific side channels.
- **SC-002**: 100% of representative secret-backed evaluations show explicit
  allow, deny, or unavailable outcomes by consumer and environment, with zero plain-text
  secret exposure in updated operator-visible surfaces during verification.
- **SC-003**: For every verification exercise, operators can identify consumer kind,
  consumer instance, effective requirement declaration, backend intent, and terminal
  outcome from daemon-visible surfaces alone within 2 minutes, including after restart and
  including decisions where no process was launched.
- **SC-004**: Verification finds no operator-visible wording or behavior that suggests
  stronger isolation guarantees than the current local backend actually provides.
- **SC-004a**: Verification finds zero cases where a request that explicitly requires
  stronger guarantees than the current backend can provide is silently executed in a
  degraded mode.
- **SC-005**: Readiness review finds no missing requirement-declaration, secret-scope, or
  provenance field that would block planning the first MCP onboarding slice.

## Assumptions

- The managed-provider convergence slice in `specs/001-sandbox-managed-providers/` remains
  the reference consumer slice already completed and is not reopened except as needed to
  align with the shared contract.
- This slice defines cross-consumer contract and visibility foundations by moving all
  mandatory adopters in the repository onto the shared contract before full future MCP
  lifecycle expansion.
- Managed providers, current skill registry and explicit skill-selection surfaces, and the
  existing daemon-owned high-risk tool-call path are treated as the mandatory adopters for
  this phase; MCP remains a future consumer family for runtime lifecycle work.
- The current local backend remains the only backend required to execute or verify this
  slice, but the contract must stay compatible with stronger future backends.
- Declarations that explicitly require stronger guarantees than the current backend can
  provide fail as unsupported or denied in this slice rather than degrading silently.
- Meaningful policy decisions are durable records in this slice even when evaluation ends
  before a process is launched.
- Existing operator-visible inspection, explanation, and history surfaces remain the
  preferred place to expose new truth rather than introducing a second control plane.
- Operator-owned secret material remains the source of truth; this slice only governs how
  consumers declare access to it and how outputs explain and redact that access.
- Test-versus-production separation remains the default operating model for verification,
  rollout, and secret-scope decisions.
- Secret authorization is evaluated at consumer-instance granularity, even when multiple
  instances of the same consumer kind share reusable default policy.
