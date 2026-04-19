# Research: Skill And Local Tool Sandbox Execution

## Decision 1: Keep the delivery unit exactly on Roadmap 19

- **Decision**: Close only executable-skill manifests, sandbox-backed execution for
  executable skills plus the current high-risk local tool path (`exec`, `shell`,
  `browser`), runtime provenance linkage, and operator verification. Do not fold in graph
  orchestration, broader capability migration, or stronger backends.
- **Rationale**: `docs/runtime/daemon-roadmaps.md` and
  `docs/harness/sandbox-execution-plane.md` explicitly sequence Roadmap 19 after MCP and
  before stronger backend work. Pulling in additional capability families or orchestration
  would blur the roadmap boundary and increase blast radius.
- **Alternatives considered**:
  - Migrate all local capability and tool paths now: rejected because the current spec
    narrows scope to executable skills plus the existing high-risk tool path.
  - Implement only executable-skill manifests and postpone real execution migration:
    rejected because Roadmap 19 is about removing the remaining ad hoc subprocess path, not
    just adding metadata.

## Decision 2: Reuse the existing tool-call runtime surface instead of creating a second execution resource

- **Decision**: Represent executable-skill launches on the existing runtime tool-call
  path, with additive fields that identify skill-targeted execution and link the tool call
  to sandbox execution and policy records.
- **Rationale**: The daemon already has run, step, tool-call, approval, and sandbox
  surfaces. Reusing them keeps execution history in one place, satisfies the roadmap
  requirement that tool-call history link to sandbox truth, and avoids introducing a new
  parallel runtime execution plane.
- **Alternatives considered**:
  - Add a new runtime `skill-call` resource family: rejected because it would duplicate
    tool-call history and fracture operator inspection.
  - Launch executable skills only through direct sandbox routes: rejected because runtime
    provenance would then bypass run/step truth and make operator debugging harder.

## Decision 3: Extend the current skill registry with executable manifests and availability state

- **Decision**: Keep skill discovery in `daemon/internal/skills`, add executable-manifest
  parsing and projection there, and surface invalid executable skills as `unavailable`
  with an explicit reason rather than hiding them.
- **Rationale**: The repository already has first-class skill loading, precedence, and
  inspection behavior. Extending that registry is the smallest reversible change and keeps
  manifest validation colocated with the source of skill truth.
- **Alternatives considered**:
  - Build a separate executable-skill registry: rejected because it would duplicate current
    skill discovery and drift from existing inspection routes.
  - Drop invalid executable skills silently: rejected because the clarification outcome
    requires them to remain operator-visible for debugging and audit.

## Decision 4: Route executable skills and the high-risk local tool path through sandbox subprocess execution

- **Decision**: In-scope launches use `sandbox.ExecutionRequest` as the only execution
  boundary, with consumer declarations and policy records attached to each executable-skill
  or local-tool attempt.
- **Rationale**: The sandbox manager already owns approval semantics, execution audit
  trails, subprocess lifecycle, and restart cancellation behavior. Reusing that path closes
  the remaining unmanaged local subprocess gap without inventing a second executor.
- **Alternatives considered**:
  - Keep the current high-risk tool path as approval-only preflight with no sandbox launch:
    rejected because Roadmap 19 explicitly requires real subprocess execution to move onto
    sandbox.
  - Introduce a skill-specific executor outside sandbox and sync records later: rejected
    because it recreates the hidden side path the roadmap is supposed to remove.

## Decision 5: Default undeclared executable-skill approval posture to `ask`

- **Decision**: If an executable skill does not explicitly declare its approval posture,
  the daemon treats it as `ask`.
- **Rationale**: This aligns executable skills with the existing operator trust boundary
  for high-risk local execution and reduces the chance that newly executable skills are
  accidentally allowed with insufficient review.
- **Alternatives considered**:
  - Default to `allow`: rejected because it broadens authority for newly executable skills
    with no operator review.
  - Default to `deny`: rejected because it would force every executable skill to add an
    explicit allow/ask declaration before any local verification can proceed.

## Decision 6: Recover interrupted in-flight executions as `cancelled` after daemon restart

- **Decision**: Any in-flight executable-skill or high-risk tool execution interrupted by
  daemon restart is recovered as `cancelled`, while runtime and sandbox history remain
  durable and linked.
- **Rationale**: This matches existing sandbox behavior and keeps operator-visible state
  truthful; the daemon should not imply work kept running or can be resumed automatically
  when it cannot guarantee either.
- **Alternatives considered**:
  - Attempt automatic resume after restart: rejected because the current sandbox backend
    does not provide restart-safe continuation semantics.
  - Leave executions as `unknown` or `interrupted`: rejected because that weakens audit
    clarity and complicates test expectations.

## Decision 7: Resolve executable-skill secrets from the active daemon data dir

- **Decision**: Executable-skill secret refs resolve from the active environment data dir
  (`~/.dope-test` or `~/.dope`) via `skill-secrets.json` instead of reading ambient daemon
  process environment variables directly.
- **Rationale**: This keeps `test` and `prod` execution separated by daemon instance and
  avoids accidental cross-environment secret injection when both environments share a shell
  session.
- **Alternatives considered**:
  - Keep reading raw process env: rejected because it cannot enforce the test/prod
    separation required by the roadmap.
  - Introduce a second standalone secret-management plane in this slice: rejected because
    Roadmap 19 only needs one environment-scoped source for executable-skill launch.

## Decision 8: Keep contract changes additive across skill, runtime, approval, and sandbox surfaces

- **Decision**: Extend the existing skill inspection, tool-call, approval, sandbox,
  config, and event surfaces with additive executable-skill and execution-provenance
  fields rather than introducing new top-level public planes.
- **Rationale**: Operators already inspect the current daemon through those routes. Adding
  fields there keeps compatibility risk low and preserves one coherent execution story
  across registry, policy, runtime, and audit views.
- **Alternatives considered**:
  - Add a new top-level execution-management API for Roadmap 19: rejected because existing
    routes can carry the required truth additively.
  - Keep changes internal-only with no contract expansion: rejected because the
    constitution requires operator-auditable surfaces, not just internal state.
