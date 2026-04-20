# Contract Surfaces: MCP Catalog Management

## Goal

Keep MCP catalog maintenance on the existing daemon-owned MCP registry plane. Installed
catalog-managed servers remain the source of truth; maintenance adds explicit actions,
provenance, drift, and revalidation surfaces without creating a second package-state
model.

## HTTP API Surfaces

### Existing Installed MCP Server Routes Extended

- `GET /v1/mcp/servers`
- `GET /v1/mcp/servers/{serverId}`

Existing server resources gain additive catalog-management projection:

- existing compatibility fields:
  - `originKind`
  - `catalogEntryId`
  - `installMethod`
  - `environmentScope`
  - `operatorModified`
- new additive fields:
  - `catalogManagement.sourceKind`
  - `catalogManagement.installedRevision`
  - `catalogManagement.currentRevision`
  - `catalogManagement.driftStatus`
  - `catalogManagement.driftReason`
  - `catalogManagement.installedAt`
  - `catalogManagement.lastAction`
  - `catalogManagement.lastActionAt`
  - `catalogManagement.lastActionStatus`
  - `catalogManagement.lastActionFailureClass`
  - `catalogManagement.lastActionReason`
  - `catalogManagement.installInputSnapshot`
  - `catalogManagement.lastRevalidation`

Schema surfaces:

- update `schemas/api/mcp-server-resource.schema.json`
- update any collection envelope schemas that reference MCP server resources

### New Catalog Lifecycle Action Routes

- `POST /v1/mcp/servers/{serverId}/refresh`
- `POST /v1/mcp/servers/{serverId}/reinstall`
- `POST /v1/mcp/servers/{serverId}/uninstall`

Contract expectations:

- actions apply only to `originKind == catalog`
- actions fail closed with explicit `busy` or `conflict` while lifecycle or tool
  invocation is active
- `refresh` and `reinstall` classify a missing bundled source as `missing_entry`
- refresh and reinstall fail closed on `operatorModified` or equivalent conflicting state
- uninstall removes the resource from the active registry and returns a result body rather
  than `204`, because audit truth is part of the contract
- successful uninstall returns `removed=true` and no active `server` payload
- blocked or failed actions return HTTP `409`; missing installed resources return `404`

Shared response shape:

- `actionId`
- `action`
- `status`
- `serverId`
- `catalogEntryId`
- `failureClass`
- `reason`
- `auditEventIds`
- `removed`
- `server` for refresh and reinstall results

Schema surface:

- add `schemas/api/mcp-catalog-lifecycle-result.schema.json`

### New Revalidation Route

- `POST /v1/mcp/servers/{serverId}/revalidate`

Contract expectations:

- explicit operator-triggered action only in this phase
- no daemon startup or background revalidation path
- result distinguishes:
  - current availability status
  - primary classification
  - ordered issue list
  - current drift state
  - runtime health versus prerequisite loss

Response shape:

- `actionId`
- `action: "revalidate"`
- `serverId`
- `catalogEntryId`
- `status`
- `classification`
- `reason`
- `issues`
- `auditEventIds`
- `server`

Schema surface:

- add `schemas/api/mcp-catalog-revalidation-result.schema.json`

## Event And History Surfaces

Additive event families are required so operator-visible history can distinguish lifecycle
maintenance from runtime lifecycle:

- `mcp.catalog_lifecycle_requested`
- `mcp.catalog_lifecycle_completed`
- `mcp.catalog_lifecycle_failed`
- `mcp.catalog_revalidation_completed`

Event payload requirements:

- `actionId`
- `action`
- `serverId`
- `catalogEntryId`
- `status`
- `failureClass` or `classification`
- `reason`
- `environment`
- redacted catalog-management summary when useful for operator debugging

Observed implementation notes:

- maintenance request, completion, and failure audit events are published even when the
  target server is later removed by uninstall
- environment scope is derived from the active daemon data directory; no cross-environment
  maintenance lookup or mutation path is allowed

Schema surfaces:

- add `schemas/events/mcp-catalog-lifecycle-requested.event.schema.json`
- add `schemas/events/mcp-catalog-lifecycle-completed.event.schema.json`
- add `schemas/events/mcp-catalog-lifecycle-failed.event.schema.json`
- add `schemas/events/mcp-catalog-revalidation-completed.event.schema.json`
- update any event fixture snapshots used by `make daemon-contract-test`

## Persistence Surfaces

Persistence remains additive to the existing MCP store:

- extend the persisted `mcp_servers.document_json` shape with:
  - install snapshot
  - installed/current catalog revision
  - drift summary
  - last maintenance action
  - last revalidation snapshot
- no new standalone catalog install-state or inactive-resource table is introduced
- uninstall relies on existing cascade deletes for state, tools, and exposure rows when
  the server document is removed

## Truthfulness Constraints

- source, revision, drift, and revalidation surfaces must remain environment-scoped and
  secret-redacted
- multiple revalidation failures must remain explicit through `issues[]`
- `busy` or `conflict` maintenance failures must be operator-visible and restart-safe
- missing bundled catalog entries must classify as `missing_entry` instead of silently
  degrading to a generic manual MCP resource
