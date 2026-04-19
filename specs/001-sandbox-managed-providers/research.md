# Research: Sandbox Managed Provider Convergence

## Decision 1: Converge managed-provider workflows around explicit requirement declarations and logical operations

- **Decision**: Model the slice around managed-provider action requirements plus a logical
  managed-provider operation record, instead of forcing every provider-owned local file read
  or write to become its own standalone sandbox execution.
- **Rationale**: Claude and Codex do not behave the same way. Prompt execution and logout
  use CLI subprocesses, while Codex auth-state inspection reads local files directly. A
  logical operation model lets the daemon audit both patterns without over-rotating this
  slice into generic filesystem virtualization.
- **Alternatives considered**:
  - Treat only subprocesses as sandboxed and leave local-state reads outside sandbox:
    rejected because it preserves the exact hidden access path this slice is supposed to
    close.
  - Turn every local-state access into a standalone sandbox execution: rejected because it
    is significantly heavier, expands the sandbox contract too early, and is not required
    by the clarified scope.

## Decision 2: Keep the current subprocess backend as the only backend in scope

- **Decision**: Extend the existing subprocess-backed sandbox profiles and policy model for
  managed providers rather than adding Docker, SSH, or remote execution.
- **Rationale**: The sandbox backend comparison and sandbox execution plane docs both make
  this explicit: the first backend exists to validate control-plane truth, not to close
  stronger isolation. Managed-provider convergence is the next consumer-onboarding slice,
  not a backend-expansion slice.
- **Alternatives considered**:
  - Add Docker now for stronger isolation: rejected because it introduces operator burden
    and shifts this plan away from managed-provider convergence into backend delivery.
  - Add SSH or remote execution: rejected because those imply much larger transport,
    credential, and lifecycle work that belongs to later roadmaps.

## Decision 3: Treat credential-bearing provider local state as sandbox-scoped sensitive state

- **Decision**: Model credential-bearing files and derived values as sandbox-scoped
  sensitive local state with explicit declaration, explanation, and redaction, but do not
  introduce the full generic secret-ref substrate in this slice.
- **Rationale**: The spec clarification narrowed the goal to managed providers. This slice
  must stop uncontrolled secret inheritance and unredacted provider-state exposure, but it
  does not need to design the general secret system that later MCP and tool consumers will
  require.
- **Alternatives considered**:
  - Introduce the full generic secret-ref system now: rejected because it broadens the
    scope beyond the managed-provider convergence slice.
  - Leave credential-bearing files under legacy file handling: rejected because it fails
    the explicit secret-scope and redaction requirement for this feature.

## Decision 4: Use baseline allow plus fail-closed enforcement, not interactive approval or fallback

- **Decision**: Managed-provider access that is explicitly declared and within the
  baseline requirement profile is allowed by default. Access outside declared requirements
  is denied immediately, with no legacy fallback and no per-operation approval prompts.
- **Rationale**: The in-scope workflows are recurring daemon-owned operations. Converting
  them into interactive approval prompts would create operator friction without improving
  correctness, and retaining fallback paths would keep audit and provenance unreliable.
- **Alternatives considered**:
  - Ask for approval on each sensitive access: rejected because these are baseline daemon
    behaviors, not operator-driven exceptional actions.
  - Permit a one-time or legacy fallback: rejected because the feature explicitly requires
    hidden access to fail closed.

## Decision 5: Keep external contract changes additive and centered on existing provider and sandbox surfaces

- **Decision**: Preserve existing provider auth routes and sandbox routes, and add any
  required provenance, failure classification, and enforcement-strength detail through
  additive schema-backed fields and event payloads.
- **Rationale**: The daemon already has external surfaces for managed-provider auth state,
  model inspection, and sandbox execution inspection. This slice should make those surfaces
  more truthful rather than inventing a parallel provider-specific control plane.
- **Alternatives considered**:
  - Introduce new top-level routes for managed-provider sandbox operations: rejected
    because the same truth can be expressed through existing provider and sandbox resources.
  - Keep the new information only in logs: rejected because the constitution and roadmap
    require schema-backed, operator-visible auditability.

## Decision 6: Split operator visibility by workflow type while preserving one control plane

- **Decision**: Auth-state-style workflows may surface requirement and failure details
  through provider auth state metadata and auth events, while subprocess-backed workflows
  also surface through sandbox execution resources and sandbox events.
- **Rationale**: Some managed-provider workflows are not natural subprocess executions, but
  they still need control-plane visibility. Using provider auth state metadata for those
  flows keeps the design minimal while preserving one daemon-owned policy model.
- **Alternatives considered**:
  - Force all workflows into sandbox execution resources only: rejected because not every
    in-scope workflow launches a process today.
  - Keep auth-state workflows opaque and only make prompt execution inspectable: rejected
    because it fails the operator-debugging requirements in the spec.
