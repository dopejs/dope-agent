# Research: Sandbox Stronger Backends

## Implementation Outcome

- Roadmap 20 was implemented with `docker_default` as the first stronger backend.
- Backend capability truth is now projected through config, profile, explain, execution,
  runtime, and event surfaces.
- The fail-closed `docker required -> unsupported` rule is the implemented behavior.

## Decision 1: Keep the delivery unit exactly on Roadmap 20

- **Decision**: Close only the stronger-backend slice: backend capability contracts,
  `docker` as the first stronger isolation-capable backend, truthful unsupported semantics,
  operator-facing capability comparison, and one initial stronger-backend consumer target.
- **Rationale**: `docs/runtime/daemon-roadmaps.md` and
  `docs/harness/sandbox-execution-plane.md` explicitly sequence stronger backends after the
  current subprocess-backed control-plane and migration groundwork. Folding in broader
  local capability migration or remote execution would recut the roadmap without saying so.
- **Alternatives considered**:
  - Migrate all remaining local execution families now: rejected because it combines
    backend work with broader consumer migration and raises rollback risk.
  - Implement only documentation and capability metadata with no second backend: rejected
    because Roadmap 20 requires one real stronger backend, not just design notes.

## Decision 2: Use `docker` as the first stronger isolation-capable backend

- **Decision**: Implement `docker` as the first non-subprocess backend.
- **Rationale**: The existing backend comparison doc already identifies `docker` as the
  most practical next step after subprocess. It offers materially stronger filesystem and
  network control without dragging in remote-host, queueing, or identity-lifecycle work.
- **Alternatives considered**:
  - `ssh`: rejected because it adds remote-host identity and lifecycle coupling too early.
  - Remote managed backend: rejected because it implies a new execution control plane.
  - Stay subprocess-only: rejected because it cannot close the stronger-isolation roadmap.

## Decision 3: Preserve one sandbox control plane and one execution contract

- **Decision**: Keep `docker` inside the existing sandbox profile, explain, execution,
  runtime, and provenance contract instead of creating backend-specific APIs.
- **Rationale**: The repository already has operator-visible sandbox profiles, explain,
  executions, approvals, runtime tool-call linkage, and events. Reusing those surfaces
  keeps auditability intact and avoids backend-specific side paths.
- **Alternatives considered**:
  - Add a `docker`-specific execution API: rejected because it would fracture operator
    truth and violate the existing control-plane model.
  - Hide backend selection entirely behind internal heuristics: rejected because operators
    need explicit guarantee and degradation visibility.

## Decision 4: Make `docker` migration opt-in for executable skills only

- **Decision**: In this slice, executable skills move to `docker` only when their declared
  requirement explicitly selects or requires `docker`.
- **Rationale**: Roadmap 20 should prove the stronger backend path without silently
  changing the runtime semantics of all existing executable skills. Opt-in migration is the
  smallest reversible change and protects current workflows.
- **Alternatives considered**:
  - Default all executable skills to `docker`: rejected because it changes behavior too
    broadly and makes rollback harder.
  - Auto-select `docker` based on heuristics: rejected because it hides policy and makes
    acceptance tests ambiguous.

## Decision 5: Use executable skills as the first stronger-backend verification target

- **Decision**: The first end-to-end stronger-backend consumer is executable skills.
- **Rationale**: Executable skills already have explicit manifests, requirement
  declarations, and sandbox-backed execution paths from Roadmap 19. They are the cleanest
  place to verify backend capability negotiation without mixing in MCP lifecycle or
  provider-specific special cases.
- **Alternatives considered**:
  - High-risk local tools first: rejected because they are broader and more operator-facing
    than necessary for the first stronger-backend validation.
  - MCP servers first: rejected because lifecycle and transport concerns would blur the
    backend validation slice.
  - Managed providers first: rejected because provider-specific auth and config flows are
    not the simplest stronger-backend validation target.

## Decision 6: Fail `docker`-required requests as `unsupported` when host prerequisites are absent

- **Decision**: If a request requires `docker` but the host cannot provide it, the
  canonical outcome is `unsupported`.
- **Rationale**: This is the most truthful operator-facing result and prevents degraded
  subprocess execution from being mistaken for stronger isolation. It also cleanly separates
  host capability gaps from runtime or policy failure.
- **Alternatives considered**:
  - Fall back to `subprocess`: rejected because the spec explicitly forbids silent
    downgrade to a weaker backend.
  - Leave the request pending: rejected because host capability absence is not an approval
    workflow.
  - Report normal runtime failure: rejected because it hides the real mismatch.

## Decision 7: Record migration inventory and backend capability matrix as first-class planning artifacts

- **Decision**: The roadmap must commit a backend capability matrix, remaining consumer
  inventory, degradation rules, and host prerequisites as durable design artifacts.
- **Rationale**: The current user concern is future continuity. Capturing these items in
  roadmap artifacts is the lowest-cost way to avoid future work depending on memory or
  commit archaeology.
- **Alternatives considered**:
  - Leave migration inventory implicit in source and docs: rejected because it does not
    reduce future discovery risk.
  - Defer inventory to implementation notes only: rejected because planning itself needs
    those boundaries to stay auditably scoped.

## Decision 8: Keep verification centered on targeted daemon tests plus one real consumer flow

- **Decision**: Verification should combine package-level daemon tests, contract checks,
  restart coverage, and one end-to-end executable-skill flow on `docker`.
- **Rationale**: This matches the repository’s existing verification style and is strong
  enough to prove backend capability semantics, unsupported behavior, and operator-visible
  provenance without inventing a separate external validation harness.
- **Alternatives considered**:
  - Rely on unit tests inside `daemon/internal/sandbox` only: rejected because Roadmap 20
    also changes inspection and consumer-facing behavior.
  - Require broad manual validation only: rejected because it is too easy to miss contract
    drift or restart regressions.
