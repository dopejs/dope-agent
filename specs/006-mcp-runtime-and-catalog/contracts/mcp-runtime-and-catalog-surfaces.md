# Contract Surfaces: Complete MCP Runtime And Catalog

## Goal

Keep MCP completion on one daemon-owned control plane. Catalog install, transport truth,
and tool invocation must be additive to the existing MCP registry and runtime tool-call
surfaces.

## HTTP API Surfaces

### Bundled MCP Catalog

- `GET /v1/mcp/catalog`
  - returns the curated starter catalog
  - schema-backed response envelope: `schemas/api/mcp-catalog-list.response.schema.json`
  - each entry projects:
    - `id`
    - `displayName`
    - `description`
    - `transportKind`
    - `tags`
    - `immediateUse`
    - `prerequisites`
    - `secretRequirements`
    - `environmentEligibility`
    - `availabilityStatus`
    - `availabilityReason`
    - `installSupport`
- `GET /v1/mcp/catalog/{entryId}`
  - returns the same catalog entry with richer install and availability detail
  - schema-backed response shape: `schemas/api/mcp-catalog-detail.response.schema.json`
- `POST /v1/mcp/catalog/{entryId}/install`
  - daemon-managed install path
  - request carries environment-safe install arguments only
  - response returns:
    - `installId`
    - `status`
    - `catalogEntryId`
    - `serverId` when created
    - `availabilityStatus`
    - `availabilityReason`
    - `auditEventIds`

### Existing MCP Server Surfaces Extended

- `GET /v1/mcp/servers`
- `GET /v1/mcp/servers/{serverId}`
- `POST /v1/mcp/servers`

These existing MCP routes remain the installed resource source of truth and gain additive
fields:

- `originKind`
- `catalogEntryId`
- `installMethod`
- `transportKind`
- `transportConfigSummary`
- `availabilityStatus`
- `availabilityReason`
- `operatorModified`

### Runtime Tool-Call Surface

No new standalone MCP invoke plane is introduced. MCP tool invocation remains on the
existing runtime tool-call route family:

- `/v1/runs/{runId}/steps/{stepId}/tool-calls`

MCP-originated tool calls add provenance fields:

- `invocationKind: "mcp_tool"`
- `mcpServerId`
- `mcpServerName`
- `mcpToolName`
- `mcpTransportKind`
- `mcpSessionId`
- `authorizationResult`
- `failureClass`

## Event And History Surfaces

Additive event projections are required for:

- catalog install requested
- catalog install completed
- catalog install blocked or failed
- MCP server transport state changes for `streamable-http`
- runtime tool-call records that reference MCP provenance

The event family should converge on existing daemon event conventions rather than create a
parallel MCP-only history subsystem.

## Schema Surfaces

Expected additive schema work:

- `schemas/api/mcp-catalog-entry.schema.json`
- `schemas/api/mcp-catalog-list.response.schema.json`
- `schemas/api/mcp-catalog-detail.response.schema.json`
- `schemas/api/mcp-catalog-install-request.schema.json`
- `schemas/api/mcp-catalog-install-result.schema.json`
- existing MCP server schemas updated for catalog origin and transport truth
- existing tool-call schemas updated for MCP invocation provenance
- corresponding event schemas for catalog install and remote transport state

## Repo Script Contract

Repo-supported install remains a first-class operator workflow but not a second resource
model.

- proposed script: `scripts/install-mcp-catalog-entry.sh`
- required behavior:
  - defaults to `KURA_ENV=test`
  - accepts a stable catalog entry id
  - calls daemon-owned install behavior or writes through the same server resource shape
  - prints the resulting `serverId`, `catalogEntryId`, and status
  - never writes raw secrets to stdout
  - may perform local pairing bootstrap to obtain a bearer token when the operator has not
    supplied one explicitly

## Truthfulness Constraints

- Missing binaries, remote endpoints, credentials, or unsupported transport capabilities
  must project `blocked`, `unavailable`, or `unsupported` explicitly.
- `streamable-http` transport state must stay distinguishable from `stdio` subprocess
  lifecycle state.
- MCP session bootstrap must be bounded so start and restore operations cannot hang
  indefinitely on an unresponsive stdio or remote server.
- MCP invocation outputs, stored tool-call history, and operator-visible event payloads
  must redact secret values and secret-derived material.
- API install and script install must be indistinguishable once the installed server
  resource is persisted, aside from audit provenance.
