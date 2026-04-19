# Research: Sandbox Requirement Declaration Contract

## Decision 1: Recut the delivery boundary around current consumers already present in the repository

- **Decision**: Treat this slice as the remaining Roadmap 17 prerequisite contract work
  plus convergence of the already-existing daemon-owned high-risk tool path, instead of
  keeping the boundary limited to managed providers only or trying to close all of Roadmap
  19.
- **Rationale**: The codebase already has three relevant consumer surfaces today:
  managed-provider sandbox usage, skill registry and explicit skill-selection surfaces, and
  runtime high-risk tool calls guarded through the policy plane. Leaving the tool path out
  would keep a real consumer-specific exception outside sandbox truth, while trying to
  absorb generic executable-skill and local-tool runtime work would overrun the intended
  slice.
- **Alternatives considered**:
  - Keep the slice limited to Roadmap 17 docs and declarations only: rejected because the
    runtime high-risk tool path would remain a hidden exception.
  - Pull in full Roadmap 19 execution migration: rejected because generic executable-skill
    subprocess execution and broader local-tool routing are materially larger than the
    current prerequisite slice.

## Decision 2: Model declarations as consumer-owned contract objects, not just ad hoc sandbox requests

- **Decision**: Introduce a shared consumer requirement declaration model that sits above
  concrete `sandbox.ExecutionRequest` and `sandbox.AccessRequest` values, then project that
  model onto current consumer families.
- **Rationale**: Managed providers already have provider-specific declaration data, but the
  runtime tool path and skill surfaces need the same core contract vocabulary without being
  forced into one family-specific shape. A consumer-owned declaration separates "what this
  consumer needs" from "what subprocess execution request was launched, if any."
- **Alternatives considered**:
  - Reuse `sandbox.ExecutionRequest` directly as the declaration contract: rejected because
    access-only and declaration-only paths do not always launch a process and should not be
    forced to look like one.
  - Keep separate declaration structures per consumer family: rejected because that
    preserves the ambiguity Roadmap 17 is supposed to remove.

## Decision 3: Treat skill adoption in this slice as declaration and provenance support on existing skill surfaces

- **Decision**: For the current skill family, adopt the shared contract on the existing
  registry, overlay, and explicit skill-selection surfaces, while leaving generic
  bundled-script or subprocess-backed skill execution to later roadmap work.
- **Rationale**: Roadmap 15 explicitly made skill loading and prompt support first-class,
  but also explicitly kept bundled script execution out of scope. The repository therefore
  has a real skill family already, but not a generic executable-skill runner. This slice
  should converge the real skill surfaces that exist rather than invent a larger runtime.
- **Alternatives considered**:
  - Treat skills as out of scope until generic execution exists: rejected because the
    clarified feature requires current consumer families to adopt the shared contract.
  - Introduce bundled-script execution now: rejected because it would effectively start
    Roadmap 19 inside this prerequisite slice.

## Decision 4: Authorize secret scope per consumer instance with reusable consumer-kind defaults

- **Decision**: Secret access is evaluated at consumer-instance granularity, while allowing
  reusable default rules within a consumer kind. Secret exposure is resolved through
  explicit secret references and declared injection behavior instead of uncontrolled
  inheritance.
- **Rationale**: This preserves least privilege while avoiding duplication for families
  with repeated patterns. It also matches the clarified spec requirement that operators be
  able to understand exactly which consumer instance received or was denied a secret.
- **Alternatives considered**:
  - Authorize only by consumer kind: rejected because two different skills or tool
    instances should not automatically share the same secret scope.
  - Keep ambient environment inheritance and redact later: rejected because that hides
    secret scope instead of declaring it.

## Decision 5: Reject stronger-than-backend declarations instead of silently degrading them

- **Decision**: If a declaration asks for stronger isolation or backend guarantees than the
  current subprocess backend can provide, fail as `unsupported` or `denied` rather than
  executing in a degraded mode.
- **Rationale**: Both sandbox design docs insist that operator-visible behavior must remain
  truthful about enforcement strength. Silent degradation would undermine the very purpose
  of the new declaration contract.
- **Alternatives considered**:
  - Execute with a degraded flag: rejected because it still performs work under guarantees
    the declaration explicitly required and the backend does not provide.
  - Allow per-consumer exceptions: rejected because that recreates consumer-specific policy
    behavior outside the shared contract.

## Decision 6: Persist durable consumer policy records for denied and preflight-only paths

- **Decision**: Represent durable cross-consumer provenance through a logical consumer
  policy record that can link to sandbox execution, tool-call, provider-auth, or skill
  surfaces as appropriate, including cases where no process launched.
- **Rationale**: Current sandbox execution history already covers launched subprocess work,
  but access-only and denied paths would otherwise fall back to logs or transient explain
  output. A logical record keeps provenance durable without overloading every non-process
  path into a fake subprocess execution.
- **Alternatives considered**:
  - Persist only launched subprocess executions: rejected because denied and preflight-only
    decisions would not survive restart as first-class records.
  - Store provenance only in logs or events: rejected because operator queryability and
    replay would remain incomplete.
  - Force every access-only path into `sandbox_execution`: rejected because it blurs the
    difference between declared policy evaluation and actual process launch.

## Decision 7: Keep external contract changes additive and centered on existing operator-visible surfaces

- **Decision**: Extend existing sandbox, provider auth, tool-call, skill, config, and
  event surfaces with additive declaration, secret-scope, and provenance fields rather than
  introducing a second public control plane.
- **Rationale**: The repository already has operator-facing surfaces for the relevant
  consumers. Adding a parallel public API for the same truth would increase confusion and
  compatibility risk at the exact moment the project is trying to consolidate execution
  boundaries.
- **Alternatives considered**:
  - Introduce a brand-new top-level consumer-operations API immediately: rejected because
    existing surfaces can carry the needed truth additively in this slice.
  - Limit new truth to internal persistence only: rejected because the constitution
    requires operator-auditable surfaces, not just internal state.
