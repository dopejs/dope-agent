# Research: Complete MCP Runtime And Catalog

## External References

- HermesAgent MCP user guide:
  https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp/
- OpenClaw MCP CLI guide:
  https://docs.openclaw.ai/cli/mcp

## Decisions

### Decision: Recut the remaining MCP work as a new roadmap-closed slice

- Rationale: Roadmap 18 closed MCP registry, sandbox lifecycle, credential isolation, and tool exposure, but it intentionally stopped short of the full operator surface expected from a complete MCP client. This slice finishes the missing user-facing closure instead of retroactively stretching Roadmap 18.
- Alternatives considered:
  - Treat the work as a grab bag of follow-up tasks under Roadmap 18.
    - Rejected because the constitution forbids using tasks as justification for partial roadmap closure.
  - Fold the work into a broader plugin or connector marketplace roadmap.
    - Rejected because MCP completion is a coherent vertical slice that can be closed without dragging unrelated integration work into scope.

### Decision: MCP tool invocation stays on the existing runtime tool-call plane

- Rationale: The daemon already has one operator-visible execution story with approvals, tool-call history, runtime provenance, and sandbox linkage. Reusing that plane keeps MCP invocation auditable and avoids a second unmanaged path.
- Alternatives considered:
  - Add a dedicated standalone MCP invoke API as the primary call path.
    - Rejected because it would split approvals, history, and operator expectations.
  - Keep MCP as registry and lifecycle only.
    - Rejected because that leaves MCP operationally incomplete and below the external reference bar.

### Decision: `streamable-http` is the first remote MCP transport

- Rationale: It gives the repository one modern remote transport family without committing this slice to multiple legacy transport variants. It is enough to support remote docs/context catalog entries while keeping the implementation bounded.
- Alternatives considered:
  - Legacy HTTP plus SSE as the first remote transport.
    - Rejected because it adds legacy-specific work without improving the roadmap closure goal.
  - Support both `streamable-http` and legacy HTTP plus SSE now.
    - Rejected because it broadens transport work before the base remote story is proven.
  - Stay `stdio`-only.
    - Rejected because that fails the stated parity target for representative remote MCP usage.

### Decision: Ship a curated starter catalog with truthful availability rather than only ready-to-run entries

- Rationale: External references reduce friction by shipping saved definitions, not just documentation. For this repository, a useful starter set includes immediately usable local entries plus credential-backed or infrastructure-backed templates that stay visible but explicitly unavailable until prerequisites are satisfied.
- Alternatives considered:
  - Ship only entries that are immediately runnable in the default test environment.
    - Rejected because it would omit representative MCP families such as GitHub, Postgres, and Slack.
  - Ship a static docs list without installable catalog metadata.
    - Rejected because that does not close the operator surface gap.

### Decision: Support both daemon API install and repo-script install, but converge on one installed server model

- Rationale: The daemon API is the canonical control plane, while a repo script provides a practical operator shortcut for test-environment setup. Convergence on one installed server resource preserves audit truth and keeps state inspection simple.
- Alternatives considered:
  - API-only install.
    - Rejected because test-environment bootstrap is easier and more reproducible with a repo-supported helper.
  - Script-only install.
    - Rejected because it would weaken the daemon-owned operator surface and make install provenance less explicit.
  - Separate resource models for API installs and script installs.
    - Rejected because that would create drift in inspection, rollback, and audit behavior.

### Decision: The initial bundled catalog should be broad enough to cover local, remote, and credential-backed starters

- Rationale: `filesystem`, `Context7`, `GitHub`, `Postgres`, and `Slack` create a representative first catalog: a local filesystem-oriented template, a remote docs/context entry, and multiple credential-backed or infrastructure-backed templates. This matches the stated user goal better than a minimal two-entry catalog.
- Alternatives considered:
  - Minimal catalog with only `filesystem` and one remote entry.
    - Rejected because it does not prove the truthfulness and install ergonomics of blocked/template entries.
  - Very large ecosystem catalog.
    - Rejected because it would increase support surface before the install and invocation contracts are stable.

## Implemented Notes

- Real verification against the Context7 endpoint forced two protocol-compatibility fixes
  beyond the original plan:
  - `streamable-http` requests must advertise `Accept: application/json, text/event-stream`
  - `notifications/initialized` must be sent as a true notification without an RPC `id`
- Real verification against installed stdio starters also forced an MCP session bootstrap
  timeout so daemon startup and restore cannot hang indefinitely on a non-responsive local
  server process.
