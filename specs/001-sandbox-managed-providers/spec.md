# Feature Specification: Sandbox Managed Provider Convergence

**Feature Branch**: `001-sandbox-managed-providers`  
**Created**: 2026-04-19  
**Status**: Draft  
**Input**: User description: "sandbox 继续落地到 managed providers"

## Clarifications

### Session 2026-04-19

- Q: Which managed-provider workflows are in scope for this slice? → A: Current core CLI workflows: auth status, logout, prompt execution, and the provider-owned local state they depend on.
- Q: How should provider-owned local state be brought under sandbox control? → A: Bring it under the sandbox requirement, policy, and audit model without making every local read or write a standalone sandbox execution.
- Q: What should happen if an in-scope workflow needs access outside its declared requirements? → A: Fail closed with a sandbox denial and no legacy fallback.
- Q: How should managed-provider credential material be handled in this slice? → A: Treat credential-bearing provider local state as sandbox-scoped sensitive local state with explicit declaration and redaction, without introducing the full generic secret-ref substrate yet.
- Q: What approval behavior should apply to declared in-scope managed-provider access? → A: Allow declared baseline access by default and deny access outside declared requirements instead of asking interactively for each operation.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Managed provider actions stay inside sandbox policy (Priority: P1)

As an operator relying on managed providers, I need every supported managed-provider action
that invokes provider tooling or touches provider-owned local state to pass through declared
sandbox requirements so the daemon no longer depends on hidden execution or filesystem side
paths.

**Why this priority**: This closes the most important remaining gap between the sandbox
control plane and a real consumer that already serves production-facing provider workflows.

**Independent Test**: Trigger a supported managed-provider action that requires provider
tooling or local provider state and verify it is either allowed, denied, or approval-gated
through sandbox-visible policy and failure surfaces without any hidden fallback path.

**Acceptance Scenarios**:

1. **Given** a supported managed-provider action with declared filesystem, network, and
   secret needs, **When** the daemon prepares that action, **Then** the action is evaluated
   through sandbox policy before any provider-side effect occurs.
2. **Given** a managed-provider action whose required access is denied or missing,
   **When** the action is attempted, **Then** the daemon stops the action through the
   sandbox boundary and returns a classified, operator-visible denial instead of falling
   back to ad hoc local access.

---

### User Story 2 - Operators can explain provider failures and provenance (Priority: P2)

As an operator debugging managed-provider issues, I need sandbox-visible provenance and
failure classification for managed-provider actions so I can distinguish policy denial,
missing local state, provider authentication problems, and provider process failure without
guessing from logs alone.

**Why this priority**: Converging execution onto sandbox without equivalent provenance
would make failures harder to debug even if the control plane became more correct.

**Independent Test**: Run one successful action and at least one blocked or failing action,
then confirm operator-visible surfaces identify the provider, action type, requirement
profile, and terminal failure class for each attempt.

**Acceptance Scenarios**:

1. **Given** a managed-provider action completes or fails, **When** an operator inspects
   the resulting daemon-visible records, **Then** they can identify which provider and
   action ran, which sandbox requirement profile applied, and how the action terminated.
2. **Given** two managed-provider failures with different causes, **When** an operator
   compares their recorded outcomes, **Then** policy denial, missing local state, provider
   auth failure, and provider process failure are distinguishable.

---

### User Story 3 - Supported workflows remain usable across environments (Priority: P3)

As a user of managed providers, I need existing supported managed-provider workflows to
keep working after sandbox convergence so the rollout improves safety and inspectability
without breaking routine provider use.

**Why this priority**: A correct control plane rollout still fails if it regresses the
core managed-provider workflows people already depend on.

**Independent Test**: Verify supported managed-provider workflows in the test environment
still complete successfully when their declared requirements are satisfied and still keep
test and production state separate.

**Acceptance Scenarios**:

1. **Given** a supported managed-provider workflow in the test environment, **When** its
   declared requirements are satisfied, **Then** it completes successfully without reading
   or mutating production-only state.
2. **Given** an operator-visible failure or audit record from a managed-provider action,
   **When** secrets or credential-derived values are involved, **Then** sensitive material
   is redacted from operator-visible output.

### Edge Cases

- What happens when a managed-provider action needs provider-owned local state that is
  missing, stale, or unreadable?
- How does the system behave when sandbox policy allows the action but provider
  authentication is expired or revoked?
- How does the system report a managed-provider action whose declared network access is
  acceptable at policy level but cannot be hard-isolated beyond the current local backend's
  enforcement strength?
- What happens when the same managed-provider workflow is valid in the test environment but
  would require different local state in production?
- How does the system report an action whose requirements are declared but whose required
  secret cannot be resolved safely?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST declare sandbox requirements for every supported
  managed-provider action that invokes provider tooling or accesses provider-owned local
  state.
- **FR-002**: The system MUST evaluate those managed-provider requirements through the
  sandbox policy boundary before the action can proceed.
- **FR-003**: The system MUST NOT leave any in-scope managed-provider workflow dependent on
  consumer-owned or undisclosed direct local execution, filesystem access, or secret
  inheritance outside the sandbox declaration, policy, and audit path.
- **FR-004**: The system MUST preserve the existing sandbox control-plane model where
  policy, approval, backend selection, and audit visibility are daemon-owned rather than
  consumer-specific behavior.
- **FR-005**: The system MUST extend the already-selected first local sandbox backend for
  managed providers in this slice and MUST NOT require a new backend to close the feature.
- **FR-006**: The system MUST classify managed-provider action outcomes at least well
  enough to distinguish policy denial, approval gating or rejection, missing local state,
  provider authentication failure, provider process failure, timeout, and cancellation.
- **FR-007**: The system MUST record operator-visible provenance for managed-provider
  actions, including provider identity, action type, and the sandbox requirement profile or
  equivalent requirement declaration that was applied.
- **FR-008**: The system MUST preserve currently supported managed-provider workflows when
  their declared requirements are satisfied.
- **FR-009**: The system MUST preserve separation between test and production state when
  resolving managed-provider local data and execution context.
- **FR-010**: The system MUST redact secrets and secret-derived values from operator-visible
  logs, execution history, failure records, and other audit surfaces.
- **FR-011**: The system MUST provide operator-visible explanation for why an in-scope
  managed-provider action is allowed, denied, or approval-gated.
- **FR-012**: The system MUST make the effective enforcement strength visible enough that
  operators can tell the difference between declared policy, preflight checks, and stronger
  isolation guarantees that are not yet provided by this slice.
- **FR-013**: The system MUST document which managed-provider workflows are in scope for
  this convergence slice and which remain explicitly out of scope, including stronger
  backends, MCP, and non-managed tool consumers.
- **FR-014**: The in-scope managed-provider workflows for this slice MUST be limited to
  auth-status inspection, logout, prompt execution, and the provider-owned local state
  those workflows require.
- **FR-015**: Provider-owned local state access in scope for this slice MUST be evaluated,
  explained, and audited through the sandbox requirement model even when the access is not
  represented as a standalone sandbox execution record.
- **FR-016**: If an in-scope managed-provider workflow requires local state, filesystem
  access, secret scope, or other access outside its declared sandbox requirements, the
  system MUST fail closed with a sandbox denial and MUST NOT fall back to legacy direct
  access.
- **FR-017**: Credential-bearing provider local state in scope for this slice MUST be
  modeled as sandbox-scoped sensitive state with explicit requirement declaration,
  operator-visible explanation, and redaction in audit surfaces, without requiring this
  slice to introduce the full generic secret-ref substrate.
- **FR-018**: In-scope managed-provider access that is explicitly declared and within the
  approved baseline requirement profile MUST be allowed by default, while access outside
  declared requirements MUST be denied rather than converted into per-operation interactive
  approval prompts.

### Key Entities *(include if feature involves data)*

- **Managed Provider Action**: A daemon-mediated operation against a managed provider, such
  as auth-status inspection, logout, or prompt execution, with declared access
  requirements and an auditable terminal outcome.
- **Managed Provider Requirement Declaration**: The declared filesystem, network, secret,
  environment, and approval needs that the sandbox uses to evaluate a managed-provider
  action.
- **Managed Provider Execution Record**: The operator-visible provenance and terminal
  outcome for one managed-provider action, including provider identity, action type,
  applied requirement profile, and failure classification.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: No intentional user-facing breaking change is planned. Any
  API, event, schema, or execution-history changes introduced by this feature must remain
  backward-compatible while adding the provenance and failure visibility needed for managed
  providers. The feature must preserve the multi-backend contract shape while remaining
  scoped to the already-selected first local backend.
- **Migration / Rollback**: No standalone data migration is expected. Rollback must be
  possible by reverting the managed-provider convergence change set and restoring the prior
  provider path if severe regressions appear.
- **Verification Strategy**: Validate supported managed-provider workflows with targeted
  provider and sandbox tests; add contract checks if API, schema, or event surfaces change;
  run full daemon verification before closing the slice.
- **Observability Impact**: Operator-visible audit records, failure explanations,
  documentation, and any relevant execution history must identify managed-provider
  provenance, applied requirement policy, effective backend family, and failure class
  clearly enough for debugging without log-only investigation.
- **Environment & Secrets**: Local verification defaults to the test environment.
  Production access remains explicit. Provider credentials and other sensitive material must
  stay scoped and redacted across all operator-visible outputs. In this slice,
  credential-bearing provider local state is treated as sandbox-scoped sensitive state
  rather than being migrated to a full generic secret-ref system.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of supported managed-provider actions that invoke provider tooling or
  access provider-owned local state are mediated by declared sandbox requirements, with no
  hidden fallback path remaining in scope.
- **SC-002**: For every managed-provider action exercised in verification, an operator can
  identify the provider, action type, applied sandbox requirement, and terminal failure
  class using daemon-visible surfaces alone within 2 minutes.
- **SC-003**: Supported managed-provider workflows continue to pass regression verification
  in the test environment when their declared requirements are satisfied.
- **SC-004**: Verification and audit review find no plain-text provider secret exposure in
  operator-visible outputs introduced by this feature.
- **SC-005**: Review of operator-visible outputs shows no case where this feature implies
  stronger isolation guarantees than the current local sandbox backend actually provides.

## Assumptions

- This slice applies to the currently supported managed providers already present in the
  daemon, rather than introducing a new provider family.
- In-scope workflow coverage is limited to auth status, logout, prompt execution, and the
  provider-owned local state those workflows require.
- Provider-owned local state is brought under sandbox requirement, policy, and audit
  control in this slice without requiring every individual file access to become a
  standalone sandbox execution.
- In-scope workflows fail closed when they need access outside declared sandbox
  requirements; this slice does not preserve a legacy fallback path for those cases.
- Credential-bearing provider local state is explicitly declared and redacted in this slice
  without requiring the full generic sandbox secret-ref substrate to be introduced here.
- Declared in-scope baseline access is allowed by default in this slice; access outside the
  declared boundary is denied rather than converted into per-operation approval friction.
- The existing sandbox control plane remains the required execution boundary for this work;
  this feature does not introduce a second execution plane.
- This slice extends the already-selected first local sandbox backend and does not add
  docker, ssh, remote, browser, or VM-grade isolation behavior.
- This slice is limited to managed-provider convergence and does not attempt to close the
  broader MCP, skills, or stronger-backend roadmaps.
- The repository's test-versus-production separation remains the default operating model
  for verification and rollout decisions.
