# Quickstart: Complete MCP Runtime And Catalog

## Purpose

Validate the completed MCP runtime and catalog slice locally in `DOPE_ENV=test`
without touching production state.

## Environment Used For Recorded Verification

- Repository root: `/Users/John/Code/dope-agent`
- Environment: `DOPE_ENV=test`
- Daemon bind address: `127.0.0.1:19192`
- Data dir: `~/.dope-test`
- Remote MCP endpoint used for positive-path verification: `https://mcp.context7.com/mcp`

## Automated Verification

### Targeted daemon packages

```bash
cd /Users/John/Code/dope-agent/daemon
go test ./internal/mcp ./internal/api ./internal/runtime ./internal/app ./internal/store ./internal/contracts
```

Result: passed.

### Contract verification

```bash
cd /Users/John/Code/dope-agent
make daemon-contract-test
```

Result: passed.

### Full daemon regression

```bash
cd /Users/John/Code/dope-agent/daemon
go test ./...
```

Result: passed.

## Manual Operator Walkthrough

Total operator time for the documented path below was under 5 minutes once the daemon was
already running.

### 1. Start the test daemon

```bash
cd /Users/John/Code/dope-agent
make daemon-run-test
```

Health check:

```bash
curl --noproxy '*' -sS http://127.0.0.1:19192/healthz
```

Observed result:

```json
{"ok":true,"service":"dope"}
```

### 2. Verify a truthful unavailable local template

`filesystem` script install:

```bash
cd /Users/John/Code/dope-agent
DOPE_ENV=test ./scripts/install-mcp-catalog-entry.sh filesystem
```

Observed result:

```text
status=blocked
catalogEntryId=filesystem
httpStatus=409
availabilityStatus=unavailable
availabilityReason=default bundled stdio command requires a local command override because sandbox network is denied
```

This verified the repo helper path while preserving truthful local-template status:

- defaults to `DOPE_ENV=test`
- performs local pairing bootstrap when no bearer token is provided
- does not imply the bundled filesystem default is runnable on a host where only `npx`
  is present

### 3. Verify a truthful blocked credential path

`github` install without `GITHUB_TOKEN`:

```bash
cd /Users/John/Code/dope-agent
DOPE_ENV=test DOPE_MCP_INSTALL_RAW_RESPONSE=1 ./scripts/install-mcp-catalog-entry.sh github
```

Observed result:

```text
status=blocked
catalogEntryId=github
httpStatus=409
availabilityStatus=blocked
availabilityReason=GitHub personal access token
```

This is the recorded blocked-path proof for the bundled credential-backed catalog entries.

### 4. Verify remote starter health and tool discovery

`Context7` install:

```bash
cd /Users/John/Code/dope-agent
DOPE_ENV=test DOPE_MCP_INSTALL_RAW_RESPONSE=1 ./scripts/install-mcp-catalog-entry.sh context7
```

Observed result:

```text
status=installed
catalogEntryId=context7
serverId=context7
httpStatus=201
availabilityStatus=ready
```

Start the installed remote server:

```bash
curl --noproxy '*' -sS -X POST \
  -H 'Authorization: Bearer <token>' \
  http://127.0.0.1:19192/v1/mcp/servers/context7/start
```

Observed result:

- `status=healthy`
- `transportKind=streamable-http`
- `toolCount=2`
- discovered tools included `resolve-library-id` and `query-docs`

This manual verification also proved the real remote fixes in this slice:

- `streamable-http` requests advertise `Accept: application/json, text/event-stream`
- `notifications/initialized` is sent as a true notification without an RPC `id`

### 5. Invoke a real MCP tool through `/v1/runs/.../tool-calls`

Allowlist one discovered Context7 tool:

```bash
curl --noproxy '*' -sS -X PATCH \
  -H 'Authorization: Bearer <token>' \
  -H 'Content-Type: application/json' \
  -d '{"runtimeSurface":"chat","exposureMode":"allow","active":true}' \
  http://127.0.0.1:19192/v1/mcp/servers/context7/tools/resolve-library-id
```

Create a run and step, then invoke the MCP tool through the existing runtime plane:

```bash
curl --noproxy '*' -sS -X POST \
  -H 'Authorization: Bearer <token>' \
  -H 'Content-Type: application/json' \
  -d '{"entrypoint":"chat","goal":"manual mcp invoke"}' \
  http://127.0.0.1:19192/v1/runs

curl --noproxy '*' -sS -X POST \
  -H 'Authorization: Bearer <token>' \
  -H 'Content-Type: application/json' \
  -d '{"title":"invoke context7","kind":"tool"}' \
  http://127.0.0.1:19192/v1/runs/<runId>/steps

curl --noproxy '*' -sS -X POST \
  -H 'Authorization: Bearer <token>' \
  -H 'Content-Type: application/json' \
  -d '{"mcpServerId":"context7","toolName":"resolve-library-id","runtimeSurface":"chat","input":{"libraryName":"react","query":"react"}}' \
  http://127.0.0.1:19192/v1/runs/<runId>/steps/<stepId>/tool-calls
```

Observed result:

- `invocationKind=mcp_tool`
- `mcpServerId=context7`
- `mcpTransportKind=streamable-http`
- `authorizationResult=allowed`
- terminal `status=completed`
- output returned real Context7 library matches for React, including `/reactjs/react.dev`

Note: the real Context7 endpoint on this date required both `libraryName` and `query` to
be present for `resolve-library-id`. Single-field attempts truthfully returned
`failureClass=remote_tool_error`, which also verified the daemon’s remote error
classification path.

## Readiness Notes

- The daemon MCP surface is now closed end-to-end: install, inspect, start, expose,
  invoke, audit.
- `filesystem`, `Context7`, `GitHub`, `Postgres`, and `Slack` are all visible in the
  bundled catalog, but only entries whose host prerequisites and credentials are currently
  satisfied should be expected to run immediately.
- `filesystem` is now truthfully treated as a local template until the operator supplies a
  local stdio command override; the daemon no longer projects the bundled default as ready.
- MCP bootstrap timeout is now part of the safety boundary; unresponsive stdio or remote
  servers fail explicitly instead of stalling daemon startup.

## Rollback

- Revert MCP catalog, remote transport, install helper, runtime invocation, and contract
  changes together.
- Preserve the earlier MCP registry and sandbox-lifecycle substrate if only this slice is
  rolled back.
