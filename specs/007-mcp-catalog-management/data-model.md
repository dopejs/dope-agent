# Data Model: MCP Catalog Management

## Entities

### Catalog-Managed MCP Resource

- Purpose: The existing installed MCP server resource, extended with catalog-management
  provenance and maintenance truth.
- Existing base:
  - `serverId`
  - lifecycle state and health fields
  - transport config
  - tool inventory and exposure
  - `originKind`
  - `catalogEntryId`
  - `installMethod`
  - `environmentScope`
  - `operatorModified`
- New additive fields:
  - `catalogManagement.sourceKind`: currently `bundled`
  - `catalogManagement.installedRevision`: canonical fingerprint of the catalog
    definition and effective install snapshot used by the last successful install,
    refresh, or reinstall
  - `catalogManagement.currentRevision`: canonical fingerprint of the current bundled
    catalog definition for the same entry, if still present
  - `catalogManagement.driftStatus`: `in_sync`, `catalog_updated`, `locally_modified`,
    `missing_entry`, or `conflicting`
  - `catalogManagement.driftReason`: short operator-facing explanation
  - `catalogManagement.installedAt`: first successful catalog install timestamp
  - `catalogManagement.lastAction`: `install`, `refresh`, `reinstall`, `revalidate`, or
    `uninstall`
  - `catalogManagement.lastActionAt`: timestamp of the most recent maintenance or
    revalidation action attempt
  - `catalogManagement.lastActionStatus`: `completed`, `blocked`, or `failed`
  - `catalogManagement.lastActionFailureClass`: optional blocked or failed classification
  - `catalogManagement.lastActionReason`: optional operator-facing explanation for the
    latest blocked or failed action
  - `catalogManagement.installInputSnapshot`: persisted install-time overrides safe to
    replay without raw secret values; includes fields such as `serverId`, `displayName`,
    `enabled`, `sandboxProfileId`, `command`, `args`, `endpoint`, `workingDir`, and
    `secretRefs`
  - `catalogManagement.lastRevalidation`: latest revalidation snapshot
- Relationships:
  - belongs to one catalog entry when `originKind == catalog`
  - owns many tool exposure records and tool inventory records
  - is the target of catalog lifecycle actions and revalidation
- Validation rules:
  - `installInputSnapshot` must never contain resolved secret values
  - `catalogManagement` fields must remain environment-scoped
  - `operatorModified == true` forces `driftStatus` to at least `locally_modified`
  - uninstall removes the resource from the active registry; no inactive resource is
    persisted after successful uninstall
  - successful revalidation updates `lastAction="revalidate"` and stores the most recent
    `lastRevalidation` snapshot without mutating install provenance

### Catalog Lifecycle Action Record

- Purpose: Operator-visible result for uninstall, refresh, and reinstall.
- Fields:
  - `actionId`
  - `action`: `uninstall`, `refresh`, or `reinstall`
  - `serverId`
  - `catalogEntryId`
  - `status`: `completed`, `blocked`, or `failed`
  - `failureClass`: `busy`, `conflict`, `missing_entry`, `not_catalog_managed`,
    `unavailable`, or `failed`
  - `reason`
  - `auditEventIds`
  - `server`: optional updated server resource for `refresh` and `reinstall`
  - `removed`: boolean for successful uninstall
- Validation rules:
  - `busy` or `conflict` must be returned when active lifecycle or tool invocation exists
  - refresh and reinstall must fail closed when the target has operator-owned or
    conflicting state
  - refresh and reinstall must classify a missing bundled source as `missing_entry`
  - uninstall must not return an updated active server resource

### Revalidation Snapshot

- Purpose: Durable summary of the latest explicit prerequisite revalidation for a
  catalog-managed server.
- Fields:
  - `checkedAt`
  - `status`: `ready`, `blocked`, `unavailable`, or `unsupported`
  - `classification`: `healthy`, `prerequisite_lost`, `catalog_drift`,
    `locally_modified`, `runtime_unhealthy`, or `missing_entry`
  - `reason`
  - `issues`: ordered list of issue objects
- Issue object fields:
  - `kind`: `secret`, `binary`, `endpoint`, `catalog`, `runtime`, or `configuration`
  - `name`
  - `status`: `blocked`, `unavailable`, `unsupported`, or `warning`
  - `reason`
  - `environmentScope`
- Validation rules:
  - revalidation is updated only by explicit operator-triggered action in this slice
  - the top-level `classification` must be derivable from the highest-severity issue plus
    current drift/runtime state
  - secret-related issues may mention secret refs but never resolved values

### Catalog Drift Assessment

- Purpose: Derived comparison between the current bundled catalog definition and the
  installed server resource.
- Inputs:
  - current bundled catalog entry
  - persisted `installInputSnapshot`
  - current installed server document
  - `operatorModified`
- Derived outputs:
  - `installedRevision`
  - `currentRevision`
  - `driftStatus`
  - `driftReason`
  - `effectiveSpecFingerprint`
- Validation rules:
  - if the entry no longer exists, `driftStatus = missing_entry`
  - if `operatorModified` is true, `driftStatus = locally_modified` even when the current
    catalog revision also changed
  - catalog revision changes without local modification must classify as
    `catalog_updated`, not `locally_modified`

## State Transitions

### Catalog Lifecycle

- `installed` -> `refresh_requested` -> `completed`
- `installed` -> `refresh_requested` -> `blocked(conflict|busy)`
- `installed` -> `refresh_requested` -> `failed`
- `installed` -> `reinstall_requested` -> `completed`
- `installed` -> `reinstall_requested` -> `blocked(conflict|busy)`
- `installed` -> `reinstall_requested` -> `failed`
- `installed` -> `uninstall_requested` -> `completed(removed)`
- `installed` -> `uninstall_requested` -> `blocked(conflict|busy)`
- `installed` -> `uninstall_requested` -> `failed`

### Drift Classification

- `in_sync` -> `catalog_updated` when bundled entry fingerprint changes and no local
  operator modifications exist
- `in_sync` -> `locally_modified` when operator patch updates the installed resource
- `catalog_updated` -> `in_sync` after successful refresh or reinstall
- `locally_modified` -> `in_sync` only after explicit operator cleanup plus refresh or
  reinstall
- any state -> `missing_entry` when the bundled catalog entry disappears

### Revalidation

- `unknown` -> `healthy`
- `healthy` -> `prerequisite_lost`
- `healthy` -> `catalog_drift`
- `healthy` -> `runtime_unhealthy`
- `catalog_drift` -> `healthy` after successful refresh or reinstall
- `prerequisite_lost` -> `healthy` after a subsequent successful explicit revalidation

## Derived Views

- `GET /v1/mcp/servers/{serverId}` projects current catalog source, installed revision,
  current revision, drift state, last maintenance action, and last revalidation snapshot
  from the persisted server document.
- lifecycle action responses and events project the same catalog provenance fields instead
  of flattening catalog-managed resources into generic manual MCP resources.
- event and history inspection can reconstruct a maintenance timeline from install,
  refresh, reinstall, uninstall, and revalidation action records plus the current server
  resource.
- after successful uninstall, the active server resource is gone but the maintenance
  timeline remains reconstructable from audit events and lifecycle result resources.
