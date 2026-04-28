# Tenant Credential Bridge

The hosted credential bridge imports legacy local credential files and binds
credential-bearing local resource metadata into the default personal tenant
during daemon startup.

## Inputs

- `mcp-secrets.json`
- `skill-secrets.json`
- legacy `provider_auth_states`
- legacy `integrations`
- legacy `connectors`
- legacy `mcp_servers`, `mcp_server_states`, `mcp_tools`, and
  `mcp_tool_exposure_rules`

Both files are read from the daemon data directory. The bridge treats keys as
tenant secret refs and values as secret material. Empty keys and empty values
are ignored. Legacy resource rows are read from SQLite metadata only; provider
tokens and secret values are not loaded into bridge summaries.

## Startup Behavior

After `BootstrapLocal` resolves the default personal tenant, startup runs the
`tenant_migration:bridge:hosted_credentials` progress step. The step is
idempotent: existing tenant secrets are skipped and no duplicate versions are
created on restart.

When the same secret ref appears in both legacy files with different values,
the bridge creates a disabled `pending_remediation` metadata-only tenant secret.
It does not choose or persist either conflicting value. The disabled resource
uses reason `legacy_secret_ref_conflict` and must be rotated by an operator
before use.

Legacy provider auth state, integration bindings, connector configuration, and
MCP install/state/tool/exposure rows with no tenant owner are bound to the
default personal tenant. Document-backed resources are patched with `tenantId`
so restore paths that project from JSON preserve tenant ownership.

Credential-bearing legacy connectors and MCP servers that reference a conflicted
or unavailable bridged secret start disabled. Their redacted metadata remains
available to authorized operators, but invocation is blocked until the operator
rotates or recreates the missing tenant secret.

## Inspection And Redaction

Operator-facing APIs return tenant ownership, status, disabled reason, and
redacted secret summaries only. Raw secret values are never written to startup
events, audit documents, API responses, or bridge progress records.

Callers need `credentials.inspect` or the relevant credential-management
permission to inspect hosted credential metadata. Mutation still requires the
domain management permission such as `secrets.manage`, `connectors.manage`, or
`mcp.manage`.

## Rollback

The bridge does not delete legacy files. To roll back, stop the daemon, disable
or rotate the hosted tenant secrets created by the bridge, and restart with the
legacy files still present. Existing local resolvers continue to use the legacy
files for paths that have not yet moved to hosted tenant secret resolution.
