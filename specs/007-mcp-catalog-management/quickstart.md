# Quickstart: MCP Catalog Management

## Goal

Verify that a catalog-managed MCP server can be inspected, revalidated, and safely
maintained through daemon-owned workflows in `DOPE_ENV=test`.

## Prerequisites

- local test daemon only; do not use `~/.dope`
- authenticated local pairing or an existing bearer token
- one installed catalog-managed MCP server
- recommended starter for manual verification: `context7`, because it is already the most
  reliable immediate-use bundled entry from Roadmap 21

## Suggested Verification Flow

1. Start the daemon in the test environment.

```bash
make daemon-run-test
```

2. Install a catalog entry if you do not already have one. The canonical phase 22 path is
   the daemon-owned install API.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"serverId":"context7-phase22-manual"}' \
  http://127.0.0.1:19192/v1/mcp/catalog/context7/install
```

3. Inspect the installed server and verify additive provenance is present:

```bash
curl -sS -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/mcp/servers/context7-phase22-manual
```

Expected inspection points after implementation:

- `originKind == "catalog"`
- `catalogEntryId == "context7"`
- `installMethod` reflects `api` or `script`
- `catalogManagement.installedRevision` is present
- `catalogManagement.driftStatus` is explicit
- `catalogManagement.lastRevalidation` is absent or reflects the latest explicit run

4. Trigger explicit revalidation:

```bash
curl -sS -X POST -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/mcp/servers/context7-phase22-manual/revalidate
```

Expected outcome after implementation:

- action result includes `classification`, `status`, and `issues`
- no daemon startup or background revalidation is required
- operator-visible history records the action

5. Verify fail-closed maintenance on operator-modified state:

```bash
curl -sS -X PATCH \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  http://127.0.0.1:19192/v1/mcp/servers/context7-phase22-manual \
  -d '{"displayName":"Context7 Manual Modified"}'

curl -sS -X POST -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/mcp/servers/context7-phase22-manual/refresh
```

Expected outcome after implementation:

- refresh returns a blocked result with `failureClass` or `reason` equivalent to
  `conflict`
- the server is not silently overwritten

6. Verify one safe maintenance cycle. This phase only requires one full
   `install -> remove` or `install -> refresh` cycle. The lower-risk manual path is
   install -> remove:

```bash
curl -sS -X POST -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/mcp/servers/context7-phase22-manual/uninstall
```

Expected outcome after implementation:

- action result marks uninstall as completed
- the server disappears from `GET /v1/mcp/servers`
- audit and event history still preserve uninstall truth

## Automated Verification

Run the targeted suites plus contract coverage:

```bash
cd daemon && go test ./internal/mcp ./internal/api ./internal/app ./internal/store ./internal/contracts
make daemon-contract-test
cd daemon && go test ./...
```

Recorded on 2026-04-20:

- `cd daemon && go test ./internal/mcp ./internal/api ./internal/app ./internal/store ./internal/contracts`
  passed
- `make daemon-contract-test` passed
- `cd daemon && go test ./...` passed
- targeted regressions added in this pass also proved:
  - healthy running catalog-managed server revalidation returns `200` with
    `status="ready"` and `classification="healthy"`
  - operator-visible `catalogManagement.installInputSnapshot` no longer echoes raw
    transport override fields such as `command`, `args`, `endpoint`, or `workingDir`

## Recorded Manual Verification

Recorded on 2026-04-20 against `DOPE_ENV=test` with a test daemon on
`http://127.0.0.1:19192`.

### Fresh install, inspect, and revalidate

- `POST /v1/mcp/catalog/context7/install` with `{"serverId":"context7-phase22-manual"}`
  returned `status=installed`, `availabilityStatus=ready`
- `GET /v1/mcp/servers/context7-phase22-manual` showed:
  - `originKind="catalog"`
  - `catalogEntryId="context7"`
  - `installMethod="api"`
  - `catalogManagement.installedRevision`
  - `catalogManagement.currentRevision`
  - `catalogManagement.driftStatus="in_sync"`
  - `catalogManagement.lastAction="install"`
  - `catalogManagement.installInputSnapshot` remained secret-safe and did not project raw
    transport override fields
- `POST /v1/mcp/servers/context7-phase22-manual/revalidate` returned:
  - `status="ready"`
  - `classification="healthy"`
  - `catalogManagement.lastAction="revalidate"` on subsequent inspection

### Healthy running revalidation remains allowed

- a clean catalog-managed filesystem helper server was installed and started through the
  daemon-owned API in the test environment
- `POST /v1/mcp/servers/filesystem-revalidate/revalidate` returned HTTP `200` while the
  server remained running and healthy
- the result reported:
  - `status="ready"`
  - `classification="healthy"`
  - no synthetic `busy` failure class from the active transport session

### Fail-closed `conflict` on idle locally modified resource

- `PATCH /v1/mcp/servers/context7-phase22-manual` changed `displayName` to
  `Context7 Manual Modified`
- subsequent inspection showed:
  - `operatorModified=true`
  - `catalogManagement.driftStatus="locally_modified"`
  - `catalogManagement.driftReason="server has local operator modifications"`
- `POST /v1/mcp/servers/context7-phase22-manual/refresh` returned HTTP `409` with:
  - `status="blocked"`
  - `failureClass="conflict"`
  - `reason="server has local operator modifications"`

### Successful uninstall after idle

- `POST /v1/mcp/servers/context7-phase22-manual/uninstall` returned HTTP `200` with:
  - `status="completed"`
  - `removed=true`
- subsequent `GET /v1/mcp/servers/context7-phase22-manual` returned HTTP `404`

### Fail-closed `busy` on active resource

- an already healthy `context7` server in the same test environment returned HTTP `409`
  for both:
  - `POST /v1/mcp/servers/context7/refresh`
  - `POST /v1/mcp/servers/context7/uninstall`
- both responses reported:
  - `status="blocked"`
  - `failureClass="busy"`
  - `reason="server has an active lifecycle or transport session"`

## Notes

- This quickstart intentionally keeps verification in `DOPE_ENV=test`.
- `context7` remained the highest-signal bundled starter for this slice because it is
  both catalog-managed and immediately usable over `streamable-http`.
- The phase 21 helper script can still bootstrap a catalog install, but daemon API routes
  are the canonical maintenance surface for phase 22.
