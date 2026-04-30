# Implementation Plan: Integration Health And Permission Diagnostics

**Branch**: `027-integration-diagnostics` | **Date**: 2026-04-30 | **Spec**: [`spec.md`](./spec.md)
**Input**: Feature specification from `specs/027-integration-diagnostics/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Close Roadmap 42 by making integration health, authorization, permission failures,
provider availability, retry safety, and real-account smoke evidence first-class
tenant-scoped product behavior. Feishu/Lark is the full proof domain. Other supported
integration domains must expose either limited structured diagnostic state or an explicit
unsupported-diagnostic classification.

The change is additive around the existing integration, live-validation, ops-readiness,
tenant store, audit/event, API, SDK, and web/operator-shell boundaries. Existing
integration execution remains compatible, but failures gain stable reason codes,
remediation owners, redacted evidence, 15-minute freshness semantics, 90-day default
retention, and safe real-account smoke evidence.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; JSON Schema contracts under
`schemas/`; TypeScript SDK resources in `sdk/ts`; React web operator shell in `web`;
operator documentation under `docs/providers`, `docs/harness`, and `docs/runtime`.  
**Primary Dependencies**: `daemon/internal/integrations`, `daemon/internal/calendar`,
`daemon/internal/mail`, `daemon/internal/reminders`, `daemon/internal/delivery`,
`daemon/internal/livevalidation`, `daemon/internal/opsreadiness`,
`daemon/internal/store`, `daemon/internal/store/tenancy`, `daemon/internal/api`,
`daemon/internal/audit`, `daemon/internal/events`, `daemon/internal/identity`,
`daemon/internal/tenantctx`, `daemon/internal/contracts`, `schemas/api`,
`schemas/events`, `sdk/ts`, `web`, Roadmap 27 integration resources, Roadmap 37 hosted
credentials and connector isolation, Roadmap 39 real-account smoke/readiness evidence,
Roadmap 40 side-effect ledger and retry-safety evidence, and Roadmap 41 evaluation
product/release evidence.  
**Storage**: Existing SQLite daemon metadata store remains authoritative. Add durable
tenant-scoped diagnostic tables for latest diagnostic state, diagnostic runs, provider
classification evidence, smoke matrix reports, smoke probe outcomes, diagnostic audit
references, retention expiry, and redaction-failure markers. Store complete redacted
resource JSON plus indexed columns for tenant, integration account, provider, domain,
status, reason code, freshness timestamp, retention expiry, and pagination.  
**Testing**: Targeted Go tests for Feishu/Lark provider classification, diagnostic state
freshness, permission denials, tenant isolation, redaction fail-closed behavior, smoke
approval gates, retention expiry, audit/events, API routes, store migrations, and
release-readiness linkage; `go test ./...` in `daemon/`; `make daemon-contract-test`;
`pnpm test:clients`; `pnpm build`; `make daemon-run-test`; `make daemon-test-status`;
`go mod tidy` from `daemon/` after implementation.  
**Target Platform**: Local-first daemon and hosted daemon behavior, verified by default
in the isolated test environment (`~/.dope-test`, `127.0.0.1:19192`). Live connector and
real-account smoke use only explicit safe credentials and tenant approval.  
**Project Type**: Multi-surface daemon product change spanning provider classification,
integration health resources, user remediation projection, persistence, API contracts,
tenant permissions, audit events, SDK/web projections, ops-readiness smoke evidence, and
operator documentation.  
**Performance Goals**: Operator inspection returns cached or current diagnostic state
within the product target of 2 minutes. Cached diagnostic state is marked stale after
15 minutes. Real-account Feishu/Lark smoke completes within 10 minutes when safe
credentials and tenant approval exist, or records a structured blocked/skipped outcome.  
**Constraints**: Diagnostics are tenant-scoped, permission-gated, redacted before
display/export, and fail closed when evidence cannot be confidently redacted.
Non-idempotent or externally visible smoke probes require both tenant administrator and
authorized operator approval. Diagnostic runs and smoke evidence expire from normal
inspection after 90 days unless an authorized longer retention policy applies.  
**Scale/Scope**: One daemon may host multiple tenants with multiple integration
accounts, repeated diagnostic runs, real-account smoke matrices, and release-readiness
evidence. Correct tenant isolation, stable classifications, bounded external probes,
auditable failure visibility, and rollback safety take priority over high-volume
diagnostic execution.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** - PASS. The plan closes Roadmap 42 end-to-end: diagnostic reason
  codes, Feishu/Lark full proof-domain coverage, limited/unsupported classifications for
  other domains, user remediation messages, provider failure classification,
  retry-safety classification, real-account smoke reports, redaction fail-closed
  behavior, tenant isolation, audit evidence, retention, SDK/web projections, and
  release-readiness linkage.
- **Production-grade, minimal, reversible change** - PASS. The change is additive around
  existing integration, live-validation, ops-readiness, store, API, SDK, and web
  boundaries. Rollback disables diagnostic runs, hides new diagnostic projections, and
  stops smoke publication while preserving existing integration execution and previously
  written audit evidence.
- **Contracts and auditability** - PASS. Required API, schema, event, SDK, persistence,
  smoke, audit, retention, and redaction surfaces are named below. Stable reason-code and
  smoke contracts are planning gates before implementation.
- **Verification and observability** - PASS. Required verification covers provider
  classification fixtures, Feishu/Lark readiness, unsupported-domain projection,
  freshness, redaction, retention, permission denial, cross-tenant leakage, audit/events,
  smoke report fixtures, SDK/web projections, and release-readiness evidence.
- **Environment and secrets** - PASS. Default execution uses `~/.dope-test`; live
  connectors or real-account smoke require explicit safe credentials, tenant approval,
  and dual approval for non-idempotent or externally visible probes. Secrets, tokens,
  authorization headers, and credential-bearing payloads are never exposed.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/027-integration-diagnostics/
|-- plan.md
|-- research.md
|-- data-model.md
|-- quickstart.md
|-- contracts/
|   |-- diagnostic-surfaces.md
|   |-- provider-classification.md
|   |-- real-account-smoke.md
|   `-- audit-redaction-retention.md
|-- checklists/
|   `-- requirements.md
`-- tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
daemon/
|-- internal/
|   |-- integrations/                    # diagnostic domain model,
|   |                                    # Feishu/Lark classifier boundary,
|   |                                    # limited/unsupported domain projection
|   |-- calendar/ mail/ reminders/       # domain hooks for limited diagnostic
|   |   delivery/                        # summaries and operation failures
|   |-- livevalidation/                  # retry-safety and ambiguous commit evidence
|   |-- opsreadiness/                    # real-account smoke matrix and readiness links
|   |-- store/                           # SQLite schema, migrations, retention,
|   |   |-- migrationfixture/            # seeded migration coverage
|   |   `-- tenancy/                     # tenant-safe diagnostic accessors
|   |-- api/                             # diagnostic and smoke routes/handlers
|   |-- audit/ and events/               # diagnostic run, redaction failure,
|   |                                    # smoke publication, permission-denial events
|   |-- identity/                        # diagnostic and live-smoke permissions
|   `-- contracts/                       # schema/fixture contract tests
|-- go.mod
`-- go.sum

schemas/
|-- api/                                 # diagnostic resources, smoke reports,
|                                        # reason-code catalogs, remediation payloads
`-- events/                              # diagnostic and smoke audit/event payloads

sdk/ts/                                  # typed diagnostic, smoke, and remediation methods
web/                                     # operator shell integration diagnostic views
tui/                                     # update only if diagnostic projection is exposed there

docs/
|-- providers/                           # Feishu/Lark diagnostic guidance
|-- harness/                             # real-account smoke matrix docs
`-- runtime/                             # release readiness, rollback, retention
```

**Structure Decision**: Add a diagnostic subdomain under the existing integration and
ops-readiness boundaries instead of creating a separate service. `integrations` owns
diagnostic state and provider classification, domain packages contribute limited
diagnostic summaries, `livevalidation` remains the source of retry-safety and ambiguous
commit evidence, `opsreadiness` owns real-account smoke/readiness publication, and
`store` owns tenant-scoped persistence and retention.

## Roadmap 42 Planning Contracts

The implementation plan MUST keep these artifacts complete before `/speckit.tasks`:

- [`contracts/diagnostic-surfaces.md`](./contracts/diagnostic-surfaces.md) - diagnostic
  resources, routes, freshness, permissions, SDK/web projections, limited/unsupported
  classifications, and user remediation payloads.
- [`contracts/provider-classification.md`](./contracts/provider-classification.md) -
  stable reason codes, retry-safety categories, remediation owners, provider evidence,
  Feishu/Lark fixture coverage, and ambiguous evidence behavior.
- [`contracts/real-account-smoke.md`](./contracts/real-account-smoke.md) - smoke matrix
  reports, probe outcomes, pass/fail/blocked/skipped semantics, safe credential handling,
  dual approval for risky probes, and release-readiness linkage.
- [`contracts/audit-redaction-retention.md`](./contracts/audit-redaction-retention.md) -
  audit event families, redaction fail-closed behavior, retention expiry, denial shapes,
  and cross-tenant non-disclosure.

These artifacts are planning gates. Implementation is incomplete if a diagnostic state,
reason code, remediation message, smoke outcome, redaction behavior, retention rule,
SDK method, web projection, or audit event can exist without a contract row and proving
test.

## Migration And Rollback Plan

1. Add schema tables and indexes for diagnostic runs, latest diagnostic state,
   classification evidence, smoke matrix reports, smoke probe outcomes, retention expiry,
   and audit references with no diagnostic runners enabled by default.
2. Add tenant-safe store accessors and migration fixtures; existing integration,
   credential, live-validation, and ops-readiness records remain backward compatible.
3. Add reason-code, remediation, and smoke API/schema/SDK contracts behind explicit
   diagnostic routes while keeping existing integration execution and Roadmap 39/40/41
   routes compatible.
4. Add provider classification adapters starting with Feishu/Lark fixtures; other
   domains expose limited structured state or deliberate unsupported classification.
5. Enable diagnostic runs and user-facing remediation only after redaction,
   tenant-isolation, audit, and freshness tests pass.
6. Enable real-account smoke publication after safe-probe defaults, dual approval, skip
   reasons, retention, and release-readiness linkage pass contract tests.

Rollback disables diagnostic run creation, hides new diagnostic projections, stops smoke
matrix publication, and leaves existing integration actions unchanged. Previously written
diagnostic audit events and non-expired smoke evidence remain readable to authorized
operators until retention expiry.

## Post-Design Constitution Check

- **Roadmap closure** - PASS. `research.md`, `data-model.md`, `quickstart.md`, and the
  four contracts cover all Roadmap 42 gates, including Feishu/Lark coverage,
  limited/unsupported domain classifications, freshness, remediation, smoke evidence,
  dual approval, redaction, retention, audit, SDK/web, and release-readiness evidence.
- **Production-grade, minimal, reversible change** - PASS. Design is additive and staged:
  schema first, read-only contracts, classification fixtures, diagnostic runs, product
  projections, user remediation, smoke publication, and release-readiness linkage.
  Rollback preserves existing integration behavior and disables new diagnostic actions.
- **Contracts and auditability** - PASS. Contracts define request/response shapes,
  reason-code catalog, retry safety, remediation owners, denial payloads, audit event
  names, retention behavior, redaction fail-closed rules, and compatibility rules.
  Schemas, SDK, docs, and contract tests must change together.
- **Verification and observability** - PASS. Quickstart names targeted package tests,
  full daemon tests, schema contract tests, client tests, daemon smoke, redaction checks,
  cross-tenant leakage checks, retention checks, and final readiness evidence.
- **Environment and secrets** - PASS. Design defaults to `~/.dope-test`, requires
  explicit safe credentials for live smoke, blocks risky probes without dual approval,
  and fails closed when redaction cannot be proven.

No post-design violations require justification.

## Complexity Tracking

> Filled only when Constitution Check has unjustified violations. None for this plan.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    |            |                                     |
