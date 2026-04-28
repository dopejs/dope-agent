# Contract Surfaces: Hosted Credential Isolation

## Goal

Expose tenant-owned, permission-gated, redacted credential administration and runtime
resolution surfaces while preserving compatibility for existing local configuration
through the default personal tenant bridge.

## HTTP API Surfaces

Concrete route names may follow existing `daemon/internal/api/server.go` routing patterns,
but implementation MUST provide the following capabilities.

### Tenant Secret Administration

Capabilities:

- list tenant secrets as redacted metadata for authorized tenant administrators or
  tenant-scoped operators with `credentials.inspect`
- create a tenant secret
- update non-secret metadata
- rotate a tenant secret, creating a new active version
- disable a tenant secret

Response requirements:

- include `secretId`, `tenantId`, `secretRef`, `displayName`, `status`,
  `activeVersionId`, timestamps, and redacted remediation/status fields
- never include raw secret values or derived credential material
- stable denial response for missing tenant, missing permission, or cross-tenant access

Schema surfaces:

- add `schemas/api/tenant-secret-resource.schema.json`
- add `schemas/api/tenant-secret-list.response.schema.json`
- add `schemas/api/create-tenant-secret.request.schema.json`
- add `schemas/api/rotate-tenant-secret.request.schema.json`
- add `schemas/api/rotate-tenant-secret.response.schema.json`
- add or reuse `schemas/api/error-response.schema.json` for stable denial codes

### Integration And Provider Auth Lifecycle

Existing integration/provider routes must become tenant-owned and redacted:

- integration connect/disconnect/readiness/default behavior
- provider auth state list/read/connect/refresh/revoke/disconnect behavior

Requirements:

- provider auth state is scoped to active tenant
- disconnect revokes active tenant provider auth, disables dependent connector/MCP uses,
  and preserves redacted config for reconnect
- same human user and same external account in multiple tenants remain separate bindings
- no fallback to global provider auth state

Schema surfaces:

- update `schemas/api/integration-resource.schema.json`
- update `schemas/api/integration-list.response.schema.json`
- update `schemas/api/provider-auth-state.response.schema.json`
- add disconnect request/response schema if the existing provider/integration route does
  not already have one

### Connector Administration

Connector list/get/create/update/delete/enable/disable/invoke paths must be tenant-owned.

Requirements:

- tenant admins or principals with `connectors.manage` can mutate
- tenant-scoped operators with `credentials.inspect` can inspect redacted status
- viewers cannot inspect or mutate credential-bearing connector state
- connector resources include tenant ownership, status, disabled reason, and redacted
  secret reference summaries

Schema surfaces:

- update `schemas/api/connector-resource.schema.json`
- update `schemas/api/connector-list.response.schema.json`
- update `schemas/api/create-connector.request.schema.json`

### MCP Administration And Invocation

MCP server install, update, lifecycle, tool list, tool authorization, and tool exposure
surfaces must be tenant-owned.

Requirements:

- MCP install/state/tool uniqueness is tenant-scoped
- websocket auth summaries and secret summaries are redacted
- tool invocation resolves secrets through active tenant only
- disconnected or unsafe bridged dependencies disable invocation until reconnect or
  remediation

Schema surfaces:

- update `schemas/api/mcp-server-resource.schema.json`
- update `schemas/api/mcp-server-list.response.schema.json`
- update `schemas/api/mcp-server-create.request.schema.json`
- update `schemas/api/mcp-tool-resource.schema.json`
- update `schemas/api/mcp-tool-list.response.schema.json`
- update `schemas/api/mcp-tool-authorization.request.schema.json`
- update `schemas/api/mcp-tool-authorization.response.schema.json`
- update `schemas/api/mcp-tool-exposure-update.request.schema.json`

### Sandbox Secret Policy

Sandbox explain/execute/profile surfaces that reference secrets must resolve through the
active tenant.

Requirements:

- missing or mismatched tenant context fails closed
- redacted secret scope outcomes are allowed
- raw values are injected only into the prepared execution environment and never persisted
  in sandbox result artifacts

Schema surfaces:

- update `schemas/api/sandbox-execution.request.schema.json`
- update `schemas/api/sandbox-execution.resource.schema.json`
- update `schemas/api/sandbox-profile.schema.json`
- update `schemas/api/sandbox-consumer-view.schema.json`

## Compatibility

- Existing local `mcp-secrets.json`, `skill-secrets.json`, integration bindings,
  provider auth state, connector config, and MCP installs bridge into the default
  personal tenant.
- Unsafe or ambiguous bridged resources start disabled with redacted metadata preserved.
- API additions must be backward-compatible for clients that do not request new secret
  administration resources, and the TypeScript SDK must expose the new tenant secret
  resources without returning raw values.
