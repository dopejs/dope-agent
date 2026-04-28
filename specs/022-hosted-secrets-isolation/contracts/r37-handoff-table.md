# Roadmap 37 Handoff Table

Every implementation task that touches a shared credential-bearing resource must map back
to this table. The table is intentionally explicit so Roadmap 35 tenant ownership and
Roadmap 37 credential behavior do not drift.

| Resource or table | Roadmap 35 tenant-ownership status | Roadmap 37 credential/admin behavior | Permission required | Redaction expectations | Cross-tenant misuse test |
|-------------------|------------------------------------|--------------------------------------|---------------------|------------------------|--------------------------|
| `tenant_secrets` / local value backend | New in R37 | Tenant-owned secret metadata, value backend reference, versioned rotation, disabled/remediation state | `secrets.manage` mutate/rotate/disable; `credentials.inspect` for tenant-scoped operator inspection | No raw values in API/events/logs/fixtures/replay/evaluation/diagnostics/tests | Tenant B cannot resolve, rotate, inspect, disable, or infer tenant A secret value |
| `tenant_secret_versions` | New in R37 | Version metadata for tenant secrets; active version snapshot for work | `secrets.manage` rotate/disable; runtime resolve through active tenant | Version ids allowed; values and derived material forbidden | Tenant B cannot resolve tenant A version even with same `secretRef` |
| `secret_scope_bindings` | Tenant-owned by R35; credential semantics deferred | Bind tenant secret references to consumers and resolve through active tenant only | `secrets.manage` mutate; invoke permission by consumer kind | Redacted `secretRef`, resolution, delivery kind only | Tenant B consumer cannot use tenant A binding with same `secret_ref` |
| `integrations` | Tenant-owned by R35 | Tenant-local account binding, readiness, connect/disconnect, and provider auth relationship | `integrations.manage` connect/disconnect/mutate; `credentials.inspect` for redacted operator inspection | Account metadata allowed; tokens and secret material forbidden | Same external account connected in tenant A does not authorize tenant B |
| `provider_auth_states` | Global/R37 boundary in R35 | Add tenant ownership; tenant-local login/refresh/revoke/disconnect/rotation lifecycle | `integrations.manage` or provider admin permission for mutate; `credentials.inspect` for redacted operator inspection; runtime resolve requires active tenant | No OAuth codes, access tokens, refresh tokens, provider tokens, or local CLI auth material | Tenant B cannot use tenant A provider auth state or fallback to global auth |
| `connectors` | Global/R37 boundary in R35 | Add tenant ownership; permission-gated config/status; disabled dependent use after disconnect | `connectors.manage` mutate/invoke; `credentials.inspect` for redacted operator inspection | Redacted secret refs/status only | Tenant B cannot list, inspect, mutate, or invoke tenant A connector |
| `mcp_servers` | Global/R37 boundary in R35 | Add tenant ownership for installs and admin lifecycle | `mcp.manage` create/update/delete/start/stop; `credentials.inspect` for redacted operator inspection | Secret refs and websocket auth summaries redacted | Tenant B cannot inspect or invoke tenant A MCP server |
| `mcp_server_states` | Global/R37 boundary in R35 | Add tenant ownership tied to owning server lifecycle | `mcp.manage` lifecycle mutation; `credentials.inspect` for redacted operator inspection | Health/status/reasons only; no secret-derived payload | Tenant B cannot observe tenant A lifecycle or restart state |
| `mcp_tools` | Global/R37 boundary in R35 | Add tenant ownership tied to owning server; invoke through active tenant only | `mcp.manage` exposure/admin; `credentials.inspect` for redacted operator inspection; runtime invoke permission by active tenant | Tool schema allowed; credential auth redacted | Tenant B cannot invoke tenant A tool with same server/tool name |
| `mcp_tool_exposure_rules` | Tenant-owned by R35; credential portion deferred | Permission-gated exposure mutation and credential-bearing runtime enforcement | `mcp.manage` mutate; invoke permission by runtime surface | Redacted exposure reason and secret requirements only | Tenant B exposure rule cannot authorize tenant A secret-backed MCP use |
| Connector/MCP disabled dependent uses | New state in R37 | Disconnect disables dependent uses and preserves redacted config for reconnect | Owning integration permission to disconnect; domain permission to reconnect | Disabled reason and redacted references allowed | Tenant B cannot reconnect or invoke tenant A disabled dependency |
| Sandbox policies/profiles with secrets | Tenant-owned related rows from R35; runtime semantics deferred | Resolve secret scope through active tenant and fail closed on missing/mismatched tenant | Sandbox execution permission; `secrets.manage` for policy mutation; `credentials.inspect` for redacted operator inspection | Redacted secret scope outcomes only | Missing or mismatched tenant cannot prepare sandbox secrets |
| Existing integration bindings | Pre-R37 runtime/policy snapshots, partially tenant-owned through related rows | Bridge or reconcile account binding snapshots into default personal tenant and disable unsafe references | Default personal tenant admin remediation; `credentials.inspect` for redacted operator inspection | Account labels and readiness only; provider tokens and secret material forbidden | Tenant B cannot use, inspect, or replay a bridged tenant A integration binding |
| Existing `mcp-secrets.json` | Local environment file, not tenant-owned | Bridge entries into default personal tenant; unsafe entries disabled | Default personal tenant admin remediation | Never print values; preserve redacted ref/status | Tenant B cannot use bridged tenant A MCP secret |
| Existing `skill-secrets.json` | Local environment file, not tenant-owned | Bridge executable skill secrets into default personal tenant secret records where in scope | Default personal tenant admin remediation | Never print values; preserve redacted ref/status | Tenant B cannot use bridged tenant A skill secret |
| Existing provider local auth | Local/provider-owned state, not tenant-owned | Bridge provider auth metadata into default personal tenant; unsafe state disabled | Default personal tenant admin remediation | No CLI token/session material in output | Tenant B cannot use default tenant local provider auth |
| Replay fixtures and evaluation artifacts | Tenant-owned artifacts from R35 | Redaction proof for credential-bearing state | Read permission by artifact surface | No raw secret or token material | Fixture/evaluation generated from tenant A cannot leak secret material to tenant B |

## Handoff Verification

Implementation must include a test that fails when this table omits any of:

- `tenant_secrets`
- `tenant_secret_versions`
- `secret_scope_bindings`
- `integrations`
- `provider_auth_states`
- `connectors`
- `mcp_servers`
- `mcp_server_states`
- `mcp_tools`
- `mcp_tool_exposure_rules`
- sandbox policies/profiles with secrets
- existing integration bindings
- existing local secret/provider auth bridge sources

The verification should live near the existing R37 boundary tests in
`daemon/internal/store/tenancy` or the inventory package, and must be updated together
with `TestR37BoundarySignaturesGolden` when exported boundary signatures change.
