# Quickstart: Additional MCP Transports

## Goal

Verify that `websocket` becomes the first additional MCP transport beyond `stdio` and
`streamable-http`, with explicit capability truth, bounded reconnect behavior, and normal
runtime tool-call provenance in `DOPE_ENV=test`.

## Prerequisites

- local test daemon only; do not use `~/.dope`
- authenticated local pairing or an existing bearer token
- one local websocket MCP helper server provided by this repository after implementation
- a test secret stored in `~/.dope-test/mcp-secrets.json` for websocket auth verification

## Suggested Verification Flow

1. Start the daemon in the test environment.

```bash
make daemon-run-test
```

2. Start the repo-owned websocket MCP helper server on localhost.

```bash
cd daemon && go run ./cmd/mcp-websocket-helper --listen 127.0.0.1:19234
```

3. Inspect transport capability truth before creating a server.

```bash
curl -sS -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/mcp/transports
```

Expected outcome after implementation:

- `websocket` appears beside `stdio` and `streamable-http`
- the response distinguishes transport availability from server-specific endpoint or auth
  issues
- `/v1/config` mirrors the same MCP transport projection

4. Create and start a websocket MCP server through the normal daemon API.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "serverId":"websocket-phase23",
    "displayName":"Websocket Phase23",
    "enabled":true,
    "sandboxProfileId":"subprocess_default",
    "declarationId":"mcp_server:websocket-phase23:lifecycle.start",
    "transportKind":"websocket",
    "endpoint":"ws://127.0.0.1:19234/mcp",
    "websocketConfig":{
      "auth":{"mode":"bearer_header","secretRef":"MCP_WS_TOKEN"}
    },
    "secretRefs":["MCP_WS_TOKEN"],
    "autoRestart":true
  }' \
  http://127.0.0.1:19192/v1/mcp/servers

curl -sS -X POST -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/mcp/servers/websocket-phase23/start
```

Expected outcome after implementation:

- server registration and lifecycle stay on the existing MCP server routes
- operator-visible inspection shows `transportKind="websocket"`
- auth projection is redacted and references `MCP_WS_TOKEN` without echoing the value

5. Discover tools and invoke one tool through the existing runtime tool-call plane.

```bash
curl -sS -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/mcp/servers/websocket-phase23/tools
```

Expected outcome after implementation:

- tools are discovered through the normal MCP manager path
- runtime tool-call history records `mcpTransportKind="websocket"`

6. Trigger a websocket disconnect and verify bounded reconnect truth.

Expected outcome after implementation:

- reconnect attempts are daemon-managed and bounded
- operator-visible events and server inspection show attempt count, retry timing, and
  final outcome
- if reconnect budget is exhausted, the daemon returns explicit terminal transport truth
  without requiring raw logs

## Automated Verification

Run the targeted suites plus contract coverage:

```bash
cd daemon && go test ./internal/mcp ./internal/api ./internal/app ./internal/runtime ./internal/store ./internal/contracts
make daemon-contract-test
cd daemon && go test ./...
```

Recorded result on 2026-04-21:

- `cd daemon && go test ./internal/mcp ./internal/api ./internal/app ./internal/runtime ./internal/store ./internal/contracts`
  -> passed
- `make daemon-contract-test` -> passed
- `cd daemon && go test ./...` -> passed

Covered by the targeted and full regression runs:

- websocket transport capability inspection returns explicit host readiness truth
- websocket auth missing or unresolved secret refs classify as blocked or unavailable, not
  anonymous fallback
- websocket tool invocation preserves `mcpTransportKind="websocket"` and existing
  approval or provenance semantics
- websocket disconnect triggers bounded reconnect with explicit recovery events and final
  state
- websocket restore after daemon restart emits explicit restore-completed or restore-failed
  truth instead of looking identical to a normal manual start
- websocket endpoints reject inline credential material in URL userinfo or query
  parameters; secret-bearing auth must stay in `websocketConfig.auth`

## Recorded Manual Verification

Recorded on 2026-04-21 in `DOPE_ENV=test` against the local test daemon and the
repo-owned websocket helper server:

1. Started the daemon with `make daemon-run-test`.
2. Started the helper server with:

```bash
cd daemon && go run ./cmd/mcp-websocket-helper --listen 127.0.0.1:19234
```

3. Completed the operator inspection walkthrough with:

```bash
curl -sS -H "Authorization: Bearer $DOPE_TOKEN" http://127.0.0.1:19192/v1/mcp/transports
curl -sS -H "Authorization: Bearer $DOPE_TOKEN" http://127.0.0.1:19192/v1/config
curl -sS -H "Authorization: Bearer $DOPE_TOKEN" http://127.0.0.1:19192/v1/mcp/servers/websocket-phase23
```

Recorded outcome:

- inspection completed in under 5 minutes
- `stdio`, `streamable-http`, and `websocket` all projected through transport inspection
- the `websocket` transport record reported:
  - `availabilityStatus="ready"`
  - `healthStatus="healthy"`
  - `supportedAuthKinds=["bearer_header","header"]`
  - `daemonManagedReconnect=true`

4. Verified auth-blocked truth by creating a websocket server that referenced
   `MCP_WS_TOKEN` before the test secret was present.

Recorded outcome:

- create returned `201`
- the MCP server resource projected `availabilityStatus="blocked"`
- the reason was explicit: `MCP_WS_TOKEN is unavailable in test`

5. Verified successful end-to-end websocket lifecycle and invocation by creating a
   websocket server with a valid test secret, starting it, discovering tools, allowing the
   `lookup` tool on the `chat` runtime surface, and invoking it through
   `/v1/runs/.../tool-calls`.

Recorded outcome:

- server create returned `201`
- lifecycle start returned `200` and projected `state.status="healthy"`
- `GET /v1/mcp/servers/websocket-phase23/tools` returned the `lookup` tool
- the runtime tool-call create returned `201`
- the tool-call resource projected `mcpTransportKind="websocket"` and
  `status="completed"`

6. Recovery truth for disconnect and restore was validated through automated regression
   rather than the manual walkthrough:

- bounded reconnect scheduling, success, and exhaustion are covered in
  `daemon/internal/mcp/manager_test.go`
- successful reconnect resets the attempt counter before the next disconnect episode
- restart restore is covered in `daemon/internal/app/mcp_app_test.go`
- restore-completed and restore-failed events are covered in `daemon/internal/app/mcp_app_test.go`
- reconnect state and history projection are covered in `daemon/internal/api/mcp_server_test.go`

## Notes

- This quickstart intentionally keeps verification in `DOPE_ENV=test`.
- The helper server is part of verification infrastructure, not a second MCP control
  plane.
- `stdio` and `streamable-http` verification remains covered by existing phase 21 and
  phase 22 regressions; this quickstart focuses on the additive websocket slice.
