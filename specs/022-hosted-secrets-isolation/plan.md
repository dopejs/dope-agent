# Implementation Plan: Hosted Secrets, Integrations, And Connector Isolation

**Branch**: `022-hosted-secrets-isolation` | **Date**: 2026-04-27 | **Spec**: [`spec.md`](./spec.md)
**Input**: Feature specification from `specs/022-hosted-secrets-isolation/spec.md`

## Summary

Close Roadmap 37 by making hosted credential material, integration account bindings,
provider auth state, connectors, MCP installs, MCP exposure, and sandbox secret policy
tenant-owned, permission-gated, and redacted. The implementation introduces a daemon-owned
tenant secret service for local/test secret storage and versioned rotation, moves the R37
boundary resources from global or partially tenant-owned behavior into explicit active
tenant semantics, and reuses the existing tenant context, permission, event, audit, and
contract infrastructure from Roadmaps 34 and 35.

This roadmap is the credential and external-account isolation layer. It does not add
external enterprise secret manager integrations, cross-tenant shared service accounts,
marketplace distribution, billing enforcement, or broad cross-tenant administration.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; JSON Schema contracts under
`schemas/`; TypeScript SDK resources must be updated because R37 exposes tenant secret
administration API shapes.
**Primary Dependencies**: `daemon/internal/identity`, `daemon/internal/tenantctx`,
`daemon/internal/store`, `daemon/internal/store/tenancy`, `daemon/internal/integrations`,
`daemon/internal/providers`, `daemon/internal/managedproviders`, `daemon/internal/mcp`,
`daemon/internal/connectors`, `daemon/internal/sandbox`, `daemon/internal/skills`,
`daemon/internal/audit`, `daemon/internal/events`, `daemon/internal/contracts`,
`schemas/api`, and `schemas/events`. New credential logic belongs in
`daemon/internal/secrets` rather than being embedded in individual domains.
**Storage**: SQLite remains the durable metadata store. Add tenant-owned secret metadata
and version metadata to the daemon store, and use a daemon-owned local secret value backend
under the active data directory for test/local values with file permissions restricted to
the operator. Existing `mcp-secrets.json`, `skill-secrets.json`, integration bindings,
provider auth state, connector, and MCP rows bridge into the default personal tenant
without printing values.
**Testing**: `go test ./...` in `daemon/`, targeted package tests for secrets/providers/
integrations/MCP/connectors/sandbox/skills/store/API, `make daemon-contract-test` for
schema and event contracts, `make daemon-run-test` plus the manual fake integration
smoke, and `go mod tidy` from `daemon/` after implementation.
**Target Platform**: Local-first daemon and hosted daemon behavior using the default test
environment for local verification (`~/.kura-test`, `127.0.0.1:19192`).
**Project Type**: Multi-domain daemon platform change: persistence migration, daemon API,
runtime credential resolution, provider/integration lifecycle, MCP/connector
administration, sandbox secret policy, audit/event contracts, and operator documentation.
**Performance Goals**: Credential resolution adds at most one bounded tenant-scoped lookup
per secret reference per credential-bearing run, connector invocation, MCP invocation, or
sandbox preparation. Repeated internal resolutions within the same work item reuse the
resolved secret version snapshot and emit one audit event per work item.
**Constraints**: Secret values are never returned by read APIs, events, logs, replay
fixtures, evaluation artifacts, diagnostics, or contract fixtures. Secret rotation is
versioned; new work uses the active version and already-started work keeps the version
resolved at start. Disconnect disables dependent connector/MCP uses until reconnect.
Operators inspect only tenants where they have `credentials.inspect`; viewers do not get
credential-bearing inspection through `read_only.inspect`. All local work defaults to test
state and fake credentials.
**Scale/Scope**: One daemon may host multiple tenants with small-to-moderate numbers of
credential-bearing integrations, connectors, MCP installs, provider auth states, and
sandbox policies per tenant. The plan optimizes for correctness, auditability, and
bounded lookup work rather than high-volume secret-manager throughput.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** — PASS. The plan closes Roadmap 37 end-to-end: tenant secrets,
  secret references, versioned rotation, integration/provider auth lifecycle,
  connector/MCP/sandbox administration permissions, runtime credential resolution, audit
  events, redaction contracts, local bridge behavior, and cross-tenant misuse tests.
- **Production-grade, minimal, reversible change** — PASS. The change is additive and
  staged: introduce shared secret resolution first, migrate or bridge metadata into
  default personal tenant, then gate domain surfaces through the shared layer. Rollback is
  backup-restore for storage migration plus disabling the tenant credential APIs and
  returning local secret files/provider state to the prior local configuration.
- **Contracts and auditability** — PASS. API, event, schema, persistence, and operator
  docs changes are named below. `contracts/r37-handoff-table.md` is required before tasks
  so every shared resource has owner, permission, redaction, and misuse-test coverage.
- **Verification and observability** — PASS. Required tests include redaction contract
  coverage, cross-tenant isolation across secrets/integrations/provider auth/connectors/
  MCP/sandbox, permission denial for viewer/operator/admin roles, audit granularity tests,
  bridge-disabled-state tests, and the test-environment smoke.
- **Environment and secrets** — PASS. Verification uses `~/.kura-test` and fake
  credentials. No live connector, production secret, or external secret-manager access is
  required. Any live validation path must be explicit and outside this plan.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/022-hosted-secrets-isolation/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── hosted-credential-surfaces.md
│   ├── redaction-audit-contract.md
│   └── r37-handoff-table.md
├── checklists/
│   └── requirements.md
└── tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── secrets/                         # NEW: tenant secret metadata, versioned
│   │                                    # resolution, redaction, bridge helpers
│   ├── store/
│   │   ├── store.go                     # schema, migrations, tenant secret tables,
│   │   │                                # tenant columns for R37 boundary resources
│   │   ├── tenancy/                     # tenant-aware helpers and R37 boundary tests
│   │   └── migrationfixture/            # pre-R37 local credential fixtures
│   ├── api/
│   │   ├── integrations.go              # tenant-scoped integration/admin behavior
│   │   ├── mcp_server_test.go           # MCP admin and secret resolution coverage
│   │   ├── server.go                    # secret APIs and runtime bridge points
│   │   └── tenant_guard.go              # permission and stable denial reuse
│   ├── integrations/                    # account binding, disconnect, readiness
│   ├── providers/                       # provider auth tenant lifecycle
│   ├── managedproviders/                # local provider auth bridge behavior
│   ├── mcp/                             # tenant-owned install/exposure and secret use
│   ├── connectors/                      # tenant-owned connector config/status
│   ├── sandbox/                         # tenant secret scope resolution
│   ├── skills/                          # executable skill secret resolution
│   ├── audit/ and identity/             # tenant audit event writing and listing
│   ├── events/                          # event envelope filtering and schemas
│   └── contracts/                       # schema contract tests
├── go.mod
└── go.sum

schemas/
├── api/                                 # tenant secret, redacted credential,
│                                        # integration/provider/MCP/connector schemas
└── events/                              # credential audit and lifecycle schemas

docs/
├── runtime/
├── providers/
└── harness/                             # rotation, bridge, smoke, and redaction docs

sdk/ts/                                  # update tenant secret API resources and clients
web/                                     # update only if operator shell uses new APIs
```

**Structure Decision**: Centralize credential semantics in a new `daemon/internal/secrets`
package and keep domain packages as consumers. Store helpers enforce tenant ownership for
the R37 boundary resources; API and runtime paths resolve credentials through tenant
context rather than reading `mcp-secrets.json`, process environment, or provider state
directly. Existing domain schemas stay additive and redacted.

## Roadmap 37 Handoff Table

The implementation plan MUST keep this table complete and update
[`contracts/r37-handoff-table.md`](./contracts/r37-handoff-table.md) as the detailed
contract. This table is the planning gate for `/speckit.tasks`.

| Resource / table | R35 ownership status | R37 behavior | Permissions | Redaction expectations | Cross-tenant misuse test |
|------------------|----------------------|--------------|-------------|------------------------|--------------------------|
| `tenant_secrets` / secret value backend | New in R37 | Tenant-owned secret metadata and versioned value resolution | `secrets.manage` for mutate/rotate/disable; `credentials.inspect` for tenant-scoped operator redacted status | Never return raw values; redact value, version payload, and derived material in all surfaces | Tenant B cannot resolve, rotate, inspect, or infer tenant A secret value |
| `secret_scope_bindings` | Tenant-owned by R35; credential semantics deferred | Resolve only through active tenant; bind consumer to tenant secret reference | `secrets.manage` for mutation; invoke permission depends on consumer | API/events/logs/fixtures show reference and resolution only | Tenant B consumer cannot use tenant A binding with same `secret_ref` |
| `integrations` | Tenant-owned by R35 | Account bindings and readiness use tenant-local credential state | `integrations.manage` connect/disconnect/mutate; `credentials.inspect` for redacted operator inspection | Account metadata only; provider tokens and secret material redacted | Same human connecting same external account in A does not authorize B |
| `provider_auth_states` | Global/R37 boundary in R35 | Add tenant ownership and tenant-local connect/expiry/revoke/refresh/disconnect/rotation | `integrations.manage` or provider-specific admin permission; `credentials.inspect` for redacted operator inspection; invoke requires active tenant | No tokens, OAuth codes, refresh tokens, or CLI-local secrets in output | Tenant B cannot use tenant A provider auth state or fallback token |
| `connectors` | Global/R37 boundary in R35 | Tenant-owned connector config/status and disabled dependent uses | `connectors.manage` mutate/invoke; `credentials.inspect` for redacted operator inspection | Redacted secret refs/status only | Tenant B cannot inspect, mutate, or invoke tenant A connector |
| `mcp_servers` | Global/R37 boundary in R35 | Tenant-owned MCP install/admin state | `mcp.manage` read/mutate/start/stop/invoke as applicable; `credentials.inspect` for redacted operator inspection | Secret refs and auth summaries redacted | Tenant B cannot list or invoke tenant A MCP server |
| `mcp_server_states` | Global/R37 boundary in R35 | Tenant-owned lifecycle/status tied to owning server | `mcp.manage` mutate; `credentials.inspect` for redacted operator inspection | Health/status only; no secret-derived payloads | Tenant B cannot observe tenant A MCP health or restart state |
| `mcp_tools` | Global/R37 boundary in R35 | Tenant-owned discovered tool state tied to owning server | `mcp.manage` exposure/invoke; `credentials.inspect` for redacted operator inspection | Tool schema allowed; secret-backed auth redacted | Tenant B cannot invoke tenant A tool via same server/tool name |
| `mcp_tool_exposure_rules` | Tenant-owned by R35; credential portion deferred | Exposure mutation permission and credential-bearing runtime surface enforcement | `mcp.manage` mutate; invoke permission by runtime surface | No raw secret refs beyond redacted reference/status | Tenant B exposure rule cannot authorize tenant A secret-backed MCP use |
| Sandbox policies/profiles with secrets | Tenant-owned related rows from R35; runtime semantics deferred | Secret scope resolves through active tenant and fails closed on missing/mismatched tenant | Sandbox execution permission plus `secrets.manage` for policy mutation; `credentials.inspect` for redacted operator inspection | Policy views show redacted secret refs/resolution | Missing or mismatched tenant context cannot prepare sandbox secrets |
| Existing `mcp-secrets.json` / `skill-secrets.json` / integration bindings / local provider auth | Pre-R37 local files/state | Bridge into default personal tenant; unsafe state starts disabled with redacted metadata | Default personal tenant admin remediation | Never print bridged values in logs/docs/errors | Bridged tenant A value or binding cannot be used by tenant B or global fallback |

## Post-Design Constitution Check

- **Roadmap closure** — PASS. `research.md`, `data-model.md`, and the contracts cover all
  Roadmap 37 surfaces: tenant secret records, versioning, resolution, provider auth,
  integration disconnect, connector/MCP ownership, sandbox policy, bridge behavior,
  redaction, audit, and misuse verification.
- **Production-grade, minimal, reversible change** — PASS. Design artifacts keep the
  value backend local/test by default, centralize resolution in one package, define
  disabled bridge states for unsafe legacy material, and use backup-restore plus feature
  disabling as rollback.
- **Contracts and auditability** — PASS. Contract artifacts define API shapes, event
  granularity, redaction invariants, and the required handoff table. Schema changes must
  update `schemas/api`, `schemas/events`, and contract tests together.
- **Verification and observability** — PASS. `quickstart.md` names package tests,
  contract checks, full daemon tests, test daemon smoke, redaction scans, and required
  audit evidence.
- **Environment and secrets** — PASS. The design defaults to `~/.kura-test`, fake
  credentials, local file permissions, and explicit live-connector exclusion.

No post-design violations require justification.

## Complexity Tracking

> Filled only when Constitution Check has unjustified violations. None for this plan.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    |            |                                     |
