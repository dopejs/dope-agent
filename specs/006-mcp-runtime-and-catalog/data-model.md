# Data Model: Complete MCP Runtime And Catalog

## Entities

### MCP Catalog Entry

- Purpose: Represents a curated, installable MCP starter definition exposed through daemon inspection and repo-supported install flows.
- Fields:
  - `id`: stable catalog identifier such as `filesystem` or `context7`
  - `displayName`: operator-facing name
  - `description`: short summary of the integration
  - `transportKind`: `stdio` or `streamable-http`
  - `sourceKind`: `bundled`
  - `tags`: classification such as `local`, `remote`, `docs`, `credentials`, `database`
  - `immediateUse`: boolean flag for entries intended to work in `KURA_ENV=test` without extra credentials or host infrastructure
  - `prerequisites`: host or dependency requirements, each with machine-readable kind plus operator text
  - `secretRequirements`: declared optional or required secret refs, never raw secret values
  - `environmentEligibility`: allowed environments such as `test` and `prod`
  - `availabilityStatus`: `ready`, `blocked`, `unavailable`, or `unsupported`
  - `availabilityReason`: explicit operator-facing explanation when status is not `ready`
  - `defaultInstallSpec`: daemon-installable server definition template including transport config defaults, sandbox requirement declaration, and exposure defaults
  - `scriptInstallSupport`: repo helper arguments and caveats for script-driven install
- Validation rules:
  - `id` must be unique and stable across releases
  - `transportKind` must map to a supported daemon MCP transport or resolve to explicit `unsupported`
  - `availabilityStatus` must be derived from real prerequisite and secret checks, not static marketing text
  - `defaultInstallSpec` must not include raw secrets

### Installed MCP Server

- Purpose: Represents the first-class daemon-managed MCP server resource created manually or from a catalog entry.
- Fields:
  - existing MCP server identity and lifecycle fields from Roadmap 18
  - `originKind`: `manual` or `catalog`
  - `catalogEntryId`: optional stable link back to the bundled entry
  - `installMethod`: `api` or `script`
  - `installTimestamp`: when the server resource was created from catalog
  - `environmentScope`: `test` or `prod`
  - `transportKind`: `stdio` or `streamable-http`
  - `transportConfigSummary`: redacted command or endpoint projection
  - `availabilityStatus`: installed-server readiness projection
  - `availabilityReason`: installed-server readiness explanation
  - `toolInventoryVersion`: snapshot marker for discovered tools
  - `operatorModified`: whether local edits have diverged from the bundled default
- Relationships:
  - may originate from one `MCP Catalog Entry`
  - owns zero or one active `MCP Transport Session`
  - owns many `MCP Tool Exposure` records already modeled in the existing MCP subsystem
- Validation rules:
  - catalog installs must preserve the same server resource shape as manual installs
  - secret scope must remain environment-scoped and redacted
  - script installs and API installs must populate the same origin and provenance fields

### MCP Transport Session

- Purpose: Represents the daemon-owned client connection used for MCP discovery and invocation.
- Fields:
  - `sessionId`
  - `serverId`
  - `transportKind`
  - `state`: `starting`, `healthy`, `unhealthy`, `backing_off`, `stopped`
  - `capabilitySummary`: handshake and protocol summary safe for operator inspection
  - `endpointSummary`: redacted endpoint or command projection
  - `lastHealthyAt`
  - `lastFailureClass`
  - `lastFailureReason`
  - `restartGeneration`
- Validation rules:
  - `transportKind` must match the owning installed server
  - remote transport failure must stay distinguishable from local subprocess failure
  - session state must remain operator-visible and restart-safe

### MCP Tool Invocation Record

- Purpose: Extends the existing runtime tool-call record to represent a daemon-mediated MCP tool call.
- Fields:
  - existing tool-call fields
  - `invocationKind`: `mcp_tool`
  - `mcpServerId`
  - `mcpServerName`
  - `mcpToolName`
  - `mcpTransportKind`
  - `mcpSessionId`
  - `authorizationResult`
  - `failureClass`: `blocked`, `approval_required`, `approval_rejected`, `server_unhealthy`, `transport_failed`, `timeout`, `remote_tool_error`
  - `resultSummary`: redacted operator-visible outcome
- Relationships:
  - belongs to one `Installed MCP Server`
  - may reference one `MCP Transport Session`
- Validation rules:
  - must be created only through the existing runtime tool-call plane
  - must preserve server identity, transport, and policy provenance
  - result projections must redact secrets and secret-derived material

### MCP Catalog Install Record

- Purpose: Tracks a catalog install attempt for audit and convergence between API and script installs.
- Fields:
  - `installId`
  - `catalogEntryId`
  - `method`: `api` or `script`
  - `environmentScope`
  - `requestedBy`
  - `status`: `pending`, `installed`, `blocked`, `failed`
  - `resultServerId`
  - `failureReason`
  - `auditEventIds`
- Validation rules:
  - every successful install must resolve to one `Installed MCP Server`
  - blocked installs must preserve explicit prerequisite or secret reasons
  - repeated installs must not silently overwrite operator-modified resources

## State Transitions

### Catalog Availability

- `ready` -> `blocked`: credentials or optional infrastructure become absent
- `ready` -> `unavailable`: required local dependency or endpoint support disappears
- `unavailable` -> `ready`: host prerequisites become satisfied
- `blocked` -> `ready`: required secret or external access becomes satisfied
- `unsupported`: transport or host capability is outside current daemon support and cannot be installed truthfully

### Installed Server Lifecycle

- `configured` -> `enabled`
- `enabled` -> `starting`
- `starting` -> `healthy`
- `starting` -> `unhealthy`
- `healthy` -> `backing_off`
- `healthy` -> `stopped`
- `unhealthy` -> `starting`
- `backing_off` -> `starting`

### Tool Invocation Outcome

- `requested` -> `approval_required`
- `requested` -> `blocked`
- `requested` -> `running`
- `running` -> `completed`
- `running` -> `transport_failed`
- `running` -> `remote_tool_error`
- `running` -> `timeout`
- `running` -> `cancelled`

## Derived Views

- Catalog inspection derives `availabilityStatus` and `availabilityReason` from current host, secret, transport, and environment checks.
- Installed server inspection derives `originKind`, `catalogEntryId`, `installMethod`, and `operatorModified` from persisted install provenance.
- Runtime tool-call inspection derives MCP provenance from the owning installed server and active transport session.
- MCP lifecycle inspection also derives bounded bootstrap truth: an unresponsive stdio or
  remote server transitions to explicit `failed` / `unavailable` state instead of leaving
  restore or operator start actions hung indefinitely.
