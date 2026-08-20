# Research: Additional MCP Transports

## Decisions

### Decision: Transport capability truth should be exposed as a first-class MCP surface, not inferred only from server failures

- Rationale: Roadmap 23 requires operators to determine whether a transport is ready,
  blocked, degraded, unsupported, or unavailable before start attempts. The daemon should
  therefore publish additive transport capability records under MCP inspection surfaces
  and also project them through `/v1/config`, rather than expecting operators to infer
  transport support from one failed server lifecycle.
- Alternatives considered:
  - Reuse only per-server `availabilityStatus` on `GET /v1/mcp/servers`.
    - Rejected because transport-family host readiness and server-specific endpoint or
      auth problems are different questions.
  - Add no dedicated transport inspection and rely on docs plus runtime errors.
    - Rejected because it fails the operator-truth and under-5-minute inspection goals.

### Decision: `websocket` should be implemented as a third `TransportKind` inside the existing MCP transport mux

- Rationale: The current MCP design already centralizes stdio and `streamable-http`
  session creation behind `Transport.Open(...)`. Adding `websocket` as another
  `TransportKind` keeps one MCP manager, one lifecycle state machine, one tool discovery
  path, and one runtime tool-call plane.
- Alternatives considered:
  - Create a separate websocket MCP manager or service.
    - Rejected because it would split lifecycle, recovery, and audit semantics.
  - Generalize the remote transport path first into a more abstract remote session
    framework before adding websocket.
    - Rejected because phase 23 only needs one additional transport family and should stay
      minimal and reversible.

### Decision: Use `github.com/gorilla/websocket` for the websocket transport client and helper server

- Rationale: The repository already carries `github.com/gorilla/websocket` in `go.mod`
  and `go.sum`, making it the lowest-friction production-ready choice for a bounded
  websocket transport implementation and a deterministic test helper.
- Alternatives considered:
  - Pull in a new websocket library such as `nhooyr.io/websocket`.
    - Rejected because it adds dependency churn without a clear phase-23 benefit.
  - Implement websocket framing directly with lower-level primitives.
    - Rejected because it increases code risk and review cost for no operator-visible gain.

### Decision: Websocket auth should be header-based and secret-ref-backed in this phase

- Rationale: The spec fixes explicit auth, no anonymous fallback, and reuse of MCP secret
  refs. A small `websocketConfig.auth` object that points at a `secretRef` and renders an
  `Authorization` or named header during dial satisfies that contract while preserving
  redaction and environment separation.
- Alternatives considered:
  - Allow inline bearer tokens or arbitrary inline headers in server definitions.
    - Rejected because it violates secret-discipline and operator-redaction rules.
  - Support every possible auth shape now, including cookies and query parameters.
    - Rejected because it expands the phase beyond the first safe websocket slice.

### Decision: Bounded reconnect should be daemon-managed and persisted as explicit recovery history

- Rationale: `websocket` introduces long-lived session failure modes that do not fit the
  current stdio or request-response remote model. The daemon should own reconnect
  attempts so recovery remains bounded, restart-safe, and visible in events and server
  state. Phase 23 uses a fixed bounded policy rather than user-tunable reconnect knobs.
- Alternatives considered:
  - No automatic reconnect; require operators to restart every broken websocket session.
    - Rejected because it weakens the “explicit but operable” goal for long-lived remote
      transports.
  - Unbounded background reconnect until success.
    - Rejected because it creates an opaque background loop and complicates operator trust.

Implementation closure:

- Phase 23 implemented this as a fixed three-attempt reconnect budget with persisted
  attempt count, next retry time, and last recovery classification on the MCP server
  resource plus additive reconnect event families.
- The reconnect budget is per disconnect episode rather than cumulative across the entire
  server lifetime; successful recovery resets the attempt counter before the next incident.

### Decision: Real verification should use a repo-owned websocket MCP helper server in `KURA_ENV=test`

- Rationale: Phase 23 needs one real end-to-end websocket MCP server, but this repository
  does not yet have a bundled websocket catalog entry and should not rely on an external
  service for deterministic verification. A small repo-owned helper command gives stable
  local manual and automated verification without creating a new public product surface.
- Alternatives considered:
  - Depend on a third-party remote websocket MCP server for manual validation.
    - Rejected because network availability and upstream behavior would make the roadmap
      closure fragile.
  - Skip manual verification and rely on in-process tests only.
    - Rejected because the roadmap explicitly requires a real end-to-end transport run.

Implementation closure:

- The repository now includes `daemon/cmd/mcp-websocket-helper`, and manual verification on
  2026-04-21 used it over localhost to prove capability inspection, auth-blocked truth,
  websocket start, tool discovery, and runtime tool invocation through the daemon-owned
  MCP plane.

## Implementation Notes

- Transport capability truth should distinguish:
  - transport-family support on this host
  - server-specific endpoint and auth readiness
  - lifecycle degradation versus invocation failure
- `stdio` and `streamable-http` should stay source-compatible. Phase 23 adds `websocket`
  paths and capability records without moving existing route names or replacing current
  fields.
- Websocket lifecycle and recovery should remain visible through the same MCP server
  resources, tool-call projection, and event history already used by existing transports.
- The implemented websocket auth slice is intentionally narrow: `bearer_header` and named
  `header` modes sourced from MCP secret refs, with redacted operator-visible summaries.
- Websocket endpoints now reject inline credential material such as URL userinfo or query
  parameters so the only supported secret-bearing auth path remains `websocketConfig.auth`
  via MCP secret refs.
