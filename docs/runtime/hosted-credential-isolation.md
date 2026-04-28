# Hosted Credential Isolation

Hosted credential state is tenant-owned. Secret metadata, integration bindings,
provider auth state, connector configuration, MCP installs, tool state, and sandbox
secret scope must resolve through the active tenant.

## Permissions

- `secrets.manage`: create, update metadata, rotate, and disable tenant secrets.
- `integrations.manage`: connect, disconnect, refresh, or revoke integration and
  provider auth state.
- `connectors.manage`: mutate connector configuration and lifecycle state.
- `mcp.manage`: mutate MCP server, tool, exposure, and lifecycle state.
- `credentials.inspect`: inspect redacted credential-bearing metadata for the
  active tenant only.

`read_only.inspect` does not grant credential inspection. Viewers receive stable
denials without secret details.

## Tenant Secrets

Tenant secret API responses expose metadata only:

- `secretId`, `tenantId`, `secretRef`
- status and disabled/remediation reason
- active version id
- timestamps
- redacted document metadata

Raw values are accepted only on create or rotate requests and are never returned.
Rotation creates a new active version. New credential resolutions use the new
version; work that already resolved a previous version continues with its captured
value.

## Disconnect And Reconnect

Disconnecting an integration or provider auth state disables dependent
credential-bearing use instead of deleting operator context. Connectors, MCP
servers, and integration bindings keep redacted ownership/status metadata so an
operator can see what needs remediation.

Reconnect by creating or rotating the active tenant secret, then reconnecting or
refreshing the integration/provider auth state in the same tenant. Cross-tenant
resources with the same external account key or secret ref do not satisfy each
other.

## Redaction Rules

The following surfaces must never contain raw credential material:

- API responses
- durable events and tenant audit records
- daemon logs
- replay fixtures and evaluation artifacts
- schema contract fixtures
- smoke-test output

Operator-facing summaries use secret refs, status, resolution, delivery kind, and
redaction rule names only. Successful runtime use emits one credential audit record
per credential-bearing run, connector invocation, MCP invocation, or sandbox
preparation.

## Test Environment Smoke

Use the test daemon only:

```bash
make daemon-run-test
make daemon-test-status
```

Use fake values such as `R37_FAKE_SECRET_TENANT_A_DO_NOT_LEAK`. Any appearance of
that value in output, events, audit records, replay/evaluation artifacts, or logs is
a blocking failure. The full smoke flow is maintained in
`specs/022-hosted-secrets-isolation/quickstart.md`.
