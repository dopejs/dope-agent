# Quickstart: Hosted Secrets, Integrations, And Connector Isolation

## Preconditions

- Work from branch `022-hosted-secrets-isolation`.
- Use the default test daemon environment: `~/.kura-test` and `127.0.0.1:19192`.
- Use fake credentials only. Do not touch `~/.kura`, production secrets, or live
  connectors for this roadmap.
- Review the R37 boundary test before editing credential-bearing packages:
  `daemon/internal/store/tenancy/r37_boundary_test.go`.

## Implementation Order

1. Add `daemon/internal/secrets` with tenant secret metadata, versioned resolution,
   redaction helpers, local/test value backend, and bridge helpers.
2. Add SQLite metadata tables, tenant-aware indexes, and restart-safe migration/bridge
   behavior for tenant secrets and R37 boundary resources.
3. Update `contracts/r37-handoff-table.md` verification so every shared resource has
   owner, permission, redaction, and misuse-test coverage.
4. Route existing MCP, executable skill, sandbox, integration binding, provider, and
   connector secret reference resolution through the new tenant secret service.
5. Make `provider_auth_states`, `connectors`, `mcp_servers`, `mcp_server_states`, and
   `mcp_tools` tenant-owned at store/API/runtime boundaries.
6. Add tenant secret administration APIs and update integration/provider disconnect and
   rotation behavior.
7. Add audit events and redaction contract tests for successful use, admin changes, and
   denied cross-tenant attempts.
8. Add bridge tests for existing `mcp-secrets.json`, `skill-secrets.json`, integration
   bindings, provider auth, connector config, and MCP installs into the default personal
   tenant.
9. Update docs for secret rotation, disconnect/reconnect, unsafe bridge remediation, and
   test-environment smoke.

## Verification Commands

From the repository root:

```bash
make daemon-contract-test
make daemon-test-status
```

From `daemon/`:

```bash
go test ./internal/secrets ./internal/store ./internal/store/migrationfixture ./internal/store/tenancy ./internal/app ./internal/api ./internal/integrations ./internal/providers ./internal/mcp ./internal/connectors ./internal/sandbox ./internal/audit ./internal/contracts
go test ./...
go mod tidy
```

R37 exposes hosted credential API resources, so update and verify client contracts:

```bash
pnpm test:clients
pnpm build
```

Commands observed during implementation:

```bash
GOCACHE=/tmp/kura-go-cache go test ./internal/secrets ./internal/store ./internal/store/migrationfixture ./internal/store/tenancy ./internal/app ./internal/api ./internal/integrations ./internal/providers ./internal/mcp ./internal/connectors ./internal/sandbox ./internal/audit ./internal/contracts
GOCACHE=/tmp/kura-go-cache go test ./internal/...
GOCACHE=/tmp/kura-go-cache go mod tidy
```

## Manual Test-Environment Smoke

1. Start the test daemon in the default test environment:

   ```bash
   make daemon-run-test
   ```

2. Start a timer for the smoke run; the end-to-end path must complete in under
   15 minutes.
3. Check daemon health:

   ```bash
   make daemon-test-status
   ```

4. Seed two test tenants with fake credential references that use the same `secretRef`
   name but different fake values. Use only test tenants and fake values.
   Hosted tenant secret APIs are:

   ```text
   GET  /v1/tenant-secrets
   POST /v1/tenant-secrets
   GET  /v1/tenant-secrets/{secretRef}
   PATCH /v1/tenant-secrets/{secretRef}
   POST /v1/tenant-secrets/{secretRef}/rotate
   POST /v1/tenant-secrets/{secretRef}/disable
   ```

5. Create or bridge a fake integration binding, connector, MCP server, and sandbox profile for
   tenant A.
6. Repeat with same-shaped resources for tenant B.
7. As tenant A admin, rotate tenant A's fake secret and confirm new work uses the new
   active version while already-started work keeps the prior resolved version.
8. Disconnect tenant A's fake integration and confirm dependent connector/MCP uses become
   disabled, keep redacted config, and cannot invoke until reconnect.
9. As a tenant-scoped operator with tenant A `credentials.inspect`, inspect redacted
   connector/MCP/integration state and confirm tenant B resources are not visible.
10. As a viewer or tenant B principal, attempt to inspect, mutate, rotate, disconnect, or
   invoke tenant A resources and confirm stable denials with no raw secret material.
11. Query tenant audit output and confirm one successful-use audit record per
   credential-bearing run, connector invocation, MCP invocation, or sandbox preparation.
12. Generate replay/evaluation artifacts from the fake credential-backed work and confirm
    fake secret values do not appear.
13. Stop the timer and record the elapsed time with the implementation notes; exceeding
    15 minutes is a smoke failure unless the spec is revised.

## Redaction Check

Use fake unique values such as `R37_FAKE_SECRET_TENANT_A_DO_NOT_LEAK` and
`R37_FAKE_TOKEN_TENANT_B_DO_NOT_LEAK` in tests. Any occurrence in API output, logs, event
payloads, replay fixtures, evaluation artifacts, diagnostics, contract fixtures, or test
failure output is a blocking failure.

## Phase 7 Smoke Note

Local smoke on 2026-04-28 against the test daemon completed in under 1 minute.
Verified pairing bootstrap, two-tenant tenant secret create/list/rotate, fake
integration creation, connector redacted projection, cross-tenant connector denial,
MCP redacted projection, sandbox explain redaction, and tenant audit redaction with
fake sentinel values.

Supplemental review verification on 2026-04-28 also added startup bridge coverage for
legacy `mcp-secrets.json`, `skill-secrets.json`, integration binding, provider auth,
connector config, MCP server/state/tool/exposure metadata, unsafe disabled-resource
handling, and bridge idempotency. Replay and evaluation artifact redaction remain
covered by the targeted R37 and full daemon automated test commands above.

## Rollback

Rollback for storage changes is backup-restore. Operators must take a pre-upgrade backup
before enabling this roadmap.

Operational rollback steps:

- stop the daemon
- restore the pre-R37 test or production backup as appropriate
- restore prior local credential files/provider auth state from operator backup
- restart without the hosted credential isolation changes enabled

Do not use logs, events, fixtures, or diagnostics as a source for restoring secret values;
those surfaces must never contain raw credential material.
