# Data Model: Additional MCP Transports

## Entities

### Transport Capability Record

- Purpose: Operator-visible transport-family readiness and prerequisite summary that can
  be inspected before a specific MCP server start attempt.
- Fields:
  - `transportKind`: `stdio`, `streamable-http`, or `websocket`
  - `availabilityStatus`: `ready`, `blocked`, `unavailable`, or `unsupported`
  - `healthStatus`: `healthy` or `degraded`
  - `reason`: short operator-facing summary
  - `prerequisites[]`: ordered list of host or configuration prerequisites
  - `environmentScope`
  - `supportedAuthKinds[]`: empty for transports that do not use explicit auth in this
    phase; `websocket` includes `bearer_header` and `header`
  - `daemonManagedReconnect`: boolean
  - `recoverySummary`: short explanation of restart or reconnect behavior
- Validation rules:
  - transport capability must be environment-scoped
  - host capability truth must not leak resolved secret values
  - capability truth must distinguish transport-family mismatch from one server’s runtime
    failure

### Websocket Transport Config

- Purpose: Additive websocket-specific configuration attached to an MCP server resource.
- Fields:
  - `endpoint`: `ws://` or `wss://` URL
  - `subprotocols[]`: optional ordered list of requested subprotocols
  - `auth`: optional websocket auth object
- Validation rules:
  - `endpoint` is required when `transportKind == websocket`
  - `endpoint` must not be blank and must use websocket URL semantics
  - inline secret material in URL userinfo or query parameters is forbidden

### Websocket Auth Config

- Purpose: Explicit operator-owned auth declaration for websocket transports.
- Fields:
  - `mode`: `bearer_header` or `header`
  - `headerName`: optional for `header`; defaults to `Authorization` for
    `bearer_header`
  - `scheme`: optional for `bearer_header`; defaults to `Bearer`
  - `secretRef`
- Validation rules:
  - auth config is optional only when the endpoint does not require authentication
  - every configured auth value must resolve through the existing MCP secret model
  - operator-visible projections may mention `secretRef`, `mode`, and `headerName` but
    never the resolved secret value

### Transport-Managed MCP Resource

- Purpose: The existing MCP server resource, extended to model a third transport family
  and transport-aware recovery truth.
- Existing base:
  - `serverId`
  - `transportKind`
  - lifecycle state
  - tool inventory and exposure
  - `availabilityStatus`
  - `availabilityReason`
  - tool-call provenance
- New additive fields:
  - `websocketConfig` when `transportKind == websocket`
  - `transportConfigSummary` updated to summarize websocket endpoint and auth mode without
    exposing secrets
  - `websocketAuthSummary`: redacted auth mode, header name, and secret-ref presence
  - `state.lastSessionId`: most recent transport session identifier
  - `state.lastRecoveryAt`
  - `state.lastRecoveryClass`: `reconnect_scheduled`, `reconnect_succeeded`,
    `reconnect_failed`, `restore_requested`, `restore_succeeded`, `restore_failed`, or
    empty
  - `state.reconnectAttemptCount`
  - `state.nextReconnectAt`
- Validation rules:
  - existing stdio and `streamable-http` fields remain backward compatible
  - `websocket` servers still project `transportKind`, `availabilityStatus`, and tool
    exposure through the same MCP resource contract

### Transport Recovery Record

- Purpose: Operator-visible history for websocket reconnect, retry, restart, cancel, and
  restore outcomes.
- Fields:
  - `recoveryId`
  - `serverId`
  - `transportKind`
  - `sessionId`
  - `action`: `reconnect`, `restore`, `cancel`, or `restart`
  - `status`: `scheduled`, `succeeded`, `failed`, or `abandoned`
  - `attempt`
  - `reason`
  - `failureClass`
  - `occurredAt`
- Validation rules:
  - recovery records must be reconstructable from daemon-visible events and current
    server state
  - transport recovery must remain distinct from tool invocation failure
  - bounded reconnect attempts must stop at the fixed daemon policy and surface terminal
    truth
  - phase 23 uses a fixed maximum of three reconnect attempts before terminal failure
  - successful reconnect resets the attempt counter so the next disconnect gets a fresh
    bounded budget

## State Transitions

### Transport Capability

- `unsupported` -> `ready` when a transport implementation and its host prerequisites are
  satisfied in the current environment
- `ready` -> `degraded` when the daemon can still use the transport family but runtime
  recovery or dependency checks indicate reduced health
- `ready` or `degraded` -> `blocked` when required auth or secret material is missing for
  declared transport usage

### Websocket Lifecycle

- `stopped` -> `starting` -> `healthy`
- `starting` -> `failed` when dial or initialization fails
- `healthy` -> `degraded` when the transport session is lost but reconnect remains
  eligible
- `degraded` -> `healthy` after successful bounded reconnect
- `degraded` -> `failed` after reconnect budget is exhausted
- `healthy` or `degraded` -> `stopping` -> `stopped`
- `healthy` or `degraded` -> `unsupported` when restore or reconnect concludes the host
  or config can no longer satisfy the transport

### Restore

- `enabled websocket server at daemon shutdown` -> `restore_requested`
- `restore_requested` -> `healthy` after successful session restoration
- `restore_requested` -> `degraded` while bounded reconnect is still in progress
- `restore_requested` -> `failed` or `unsupported` after explicit terminal restore truth

## Derived Views

- `GET /v1/mcp/transports` and `/v1/config` project transport capability records for each
  supported transport family.
- `GET /v1/mcp/servers/{serverId}` projects transport-specific config summary,
  availability, redacted auth summary, and latest recovery truth on the existing MCP
  server resource.
- tool-call resources and events keep `mcpTransportKind` so runtime history can attribute
  tool invocation to `stdio`, `streamable-http`, or `websocket`.
- recovery events plus the current server resource reconstruct websocket reconnect and
  restore behavior without requiring raw daemon logs.
- explicit restore-completed and restore-failed events distinguish daemon restart recovery
  from normal manual lifecycle start attempts.
