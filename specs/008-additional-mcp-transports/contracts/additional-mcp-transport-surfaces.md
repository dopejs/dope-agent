# Contract Surfaces: Additional MCP Transports

## Goal

Add a third MCP transport family, `websocket`, while keeping one daemon-owned MCP
registry, lifecycle, authorization, and runtime invocation plane.

## HTTP API Surfaces

### New Transport Capability Route

- `GET /v1/mcp/transports`

Response shape:

- list of transport capability records
- each item includes:
  - `transportKind`
  - `availabilityStatus`
  - `healthStatus`
  - `reason`
  - `prerequisites`
  - `environmentScope`
  - `supportedAuthKinds`
  - `daemonManagedReconnect`
  - `recoverySummary`

Schema surfaces:

- add `schemas/api/mcp-transport-capability.schema.json`
- add `schemas/api/mcp-transport-capability-list.response.schema.json`

### Existing Config Projection Extended

- `GET /v1/config`

Additive MCP config projection:

- `mcp.transports[]` mirrors the transport capability record shape from
  `GET /v1/mcp/transports`

Schema surfaces:

- update `schemas/api/config.response.schema.json`

### Existing MCP Server Routes Extended

- `POST /v1/mcp/servers`
- `PATCH /v1/mcp/servers/{serverId}`
- `GET /v1/mcp/servers`
- `GET /v1/mcp/servers/{serverId}`
- existing lifecycle routes under `/v1/mcp/servers/{serverId}/...`

Additive websocket support requirements:

- `transportKind` enum expands to include `websocket`
- websocket server definitions use:
  - `endpoint`
  - `websocketConfig.subprotocols[]`
  - `websocketConfig.auth`
- websocket auth config supports:
  - `mode`
  - `headerName`
  - `scheme`
  - `secretRef`
  - phase 23 concretely supports `bearer_header` and named `header`
- operator-visible resource projection exposes:
  - redacted auth summary
  - transport config summary
  - latest reconnect or recovery truth

Truthfulness rules:

- endpoints that require auth but have no configured websocket auth must surface explicit
  blocked or unavailable truth
- inline secret values are not allowed in websocket server definitions or operator-visible
  projections
- `stdio` and `streamable-http` request bodies remain valid

Schema surfaces:

- update `schemas/api/mcp-server-create.request.schema.json`
- update `schemas/api/mcp-server-update.request.schema.json`
- update `schemas/api/mcp-server-resource.schema.json`

### Runtime Tool-Call Surfaces Extended

- existing `/v1/runs/{runId}/steps/{stepId}/tool-calls`
- existing tool-call detail or history routes

Additive requirements:

- `mcpTransportKind` enum expands to include `websocket`
- websocket-originated tool calls retain the same provenance and approval model used by
  existing MCP tool calls

Schema surfaces:

- update `schemas/api/tool-call-resource.schema.json`

## Event And History Surfaces

Existing event families updated:

- `mcp.server_registered`
- `mcp.server_updated`
- `mcp.server_started`
- `tool_call.requested`
- `tool_call.completed`
- `tool_call.failed`

New additive websocket recovery event families:

- `mcp.server_reconnect_scheduled`
- `mcp.server_reconnect_completed`
- `mcp.server_reconnect_failed`
- `mcp.server_restore_completed`
- `mcp.server_restore_failed`

Event payload requirements:

- `serverId`
- `transportKind`
- `sessionId` when present
- `attempt` for reconnect events
- `failureClass` or terminal reason when recovery fails
- environment-scoped, redacted auth summary when useful for debugging

Schema surfaces:

- update `schemas/events/mcp-server-registered.event.schema.json`
- update `schemas/events/mcp-server-updated.event.schema.json`
- update `schemas/events/mcp-server-started.event.schema.json`
- update `schemas/events/tool-call-requested.event.schema.json`
- update `schemas/events/tool-call-completed.event.schema.json`
- update `schemas/events/tool-call-failed.event.schema.json`
- add `schemas/events/mcp-server-reconnect-scheduled.event.schema.json`
- add `schemas/events/mcp-server-reconnect-completed.event.schema.json`
- add `schemas/events/mcp-server-reconnect-failed.event.schema.json`

## Persistence Surfaces

Persistence remains additive to existing MCP store documents and event history:

- persisted MCP server document gains websocket config and latest recovery snapshot fields
- no new standalone websocket session registry or second transport table is required
- event history is the durable record for reconnect attempts and restore outcomes
- websocket reconnect remains bounded to a fixed daemon-managed retry budget; no
  transport-specific background scheduler or second state store is introduced

## Truthfulness Constraints

- transport-family readiness must be distinguishable from one server’s endpoint or auth
  failure
- websocket auth uses existing MCP secret refs and must remain redacted in all API, event,
  and history projections
- bounded reconnect behavior must remain reconstructable from daemon-visible events and
  server state
- restore behavior after daemon restart must be distinguishable from a normal manual start
  attempt in both MCP server state and event history
- the daemon must not create a websocket-specific invoke path outside the existing MCP
  runtime tool-call surface
- manual verification must be able to prove auth-blocked truth and one successful
  websocket tool invocation entirely through the daemon-owned MCP surfaces
