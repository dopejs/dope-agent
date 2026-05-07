# Implementation Plan: Public Quota UX

**Branch**: `032-public-quota-ux` | **Date**: 2026-05-07 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/032-public-quota-ux/spec.md`

## Summary

Expose tenant-scoped quota, usage, abuse restriction, denial detail, and support evidence
surfaces for the public hosted product. The implementation extends the existing Roadmap 38
billing/quota control plane additively: existing billing tables, enforcement behavior, and
stable denial semantics remain authoritative while daemon API projections, JSON schemas,
the TypeScript SDK, and web shell views gain user-readable status, grouped quota
summaries, denial detail, and structured redacted JSON support evidence export.

## Technical Context

**Language/Version**: Go 1.24 for daemon control-plane code; TypeScript with React/Vite for SDK and web shell surfaces  
**Primary Dependencies**: Existing daemon `billing`, `identity`, `api`, `store`, `audit`, and `events` packages; JSON Schema contract fixtures under `schemas/`; TypeScript SDK under `sdk/ts`; React web shell under `web/`  
**Storage**: Existing SQLite `billing_*` tables remain the source of truth; any new state must be additive projection/evidence metadata, not a replacement ledger  
**Testing**: `go test ./...` from `daemon/`; `make daemon-contract-test`; `pnpm test:clients`; `pnpm build`; targeted web and SDK tests  
**Target Platform**: Local daemon and hosted daemon API, TypeScript SDK consumers, and web shell users operating tenant-scoped hosted accounts  
**Project Type**: Multi-surface web-service/client feature spanning daemon API, schemas/contracts, TypeScript SDK, and React web shell  
**Performance Goals**: Authorized users can identify plan, limit, current/previous-period usage, recovery action, and abuse restriction state within 30 seconds; support evidence export completes within 2 minutes in a test walkthrough  
**Constraints**: Denials must remain fail-closed before expensive or side-effecting work; no payment checkout; no new accounting ledger; no exposure of abuse detection signals or enforcement thresholds; no cross-tenant leakage; all public contract changes must be additive  
**Scale/Scope**: One whole roadmap slice, Phase 47, covering all enforced quota categories from the Roadmap 38 catalog and active tenant projections across daemon, SDK, and web shell

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan closes Roadmap 47 as one product-visible quota and
  billing UX slice: dashboard projection, denial details, abuse restriction messaging,
  override visibility, support evidence, SDK updates, and web shell updates.
- Production-grade change control: PASS. The change set is additive and reversible:
  disable or hide the new projections/routes/web views while retaining Roadmap 38
  enforcement and audit evidence. Existing clients keep their current fields and methods.
- Contracts and auditability: PASS. API/schema/SDK contracts are called out in
  [contracts/public-quota-ux.md](./contracts/public-quota-ux.md); schema fixtures and
  contract tests are required for every response shape added or extended.
- Verification and observability: PASS. Verification includes daemon unit/API tests,
  schema contract tests, SDK tests, web tests, fail-closed regression tests, and a manual
  `DOPE_ENV=test` walkthrough for exhausted quota, abuse restriction, tenant switch, and
  evidence export.
- Environment and secrets: PASS. All local verification uses `~/.dope-test` and test
  tenants. Live connectors, production tenants, secrets, payment checkout, invoices, and
  taxes are out of scope.

## Project Structure

### Documentation (this feature)

```text
specs/032-public-quota-ux/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── public-quota-ux.md
└── tasks.md
```

### Source Code (repository root)

```text
daemon/
├── internal/api/
│   ├── hosted_billing.go
│   └── hosted_billing_test.go
├── internal/billing/
│   ├── denial.go
│   ├── denial_test.go
│   ├── projection.go
│   ├── projection_test.go
│   └── types.go
├── internal/contracts/
│   └── billing_contracts_test.go
├── internal/identity/
│   ├── permissions.go
│   └── permissions_test.go
└── internal/store/
    ├── billing.go
    └── billing_test.go

schemas/
├── api/
│   ├── billing-usage.response.schema.json
│   ├── billing-quota-resource.schema.json
│   ├── billing-denial-resource.schema.json
│   ├── billing-quota-dashboard.response.schema.json
│   ├── billing-denial-detail.response.schema.json
│   ├── billing-evidence-export.response.schema.json
│   └── tenant-permission-resource.schema.json
└── events/

sdk/ts/
├── src/index.ts
├── src/index.test.ts
├── dist/index.d.ts
└── dist/index.js

web/
└── src/app/
    ├── App.tsx
    └── App.test.tsx

docs/
└── runtime/hosted-billing-quotas.md
```

**Structure Decision**: Extend the existing Roadmap 38 billing surface in place. Daemon
projection and permission behavior stays under `daemon/internal/billing` and
`daemon/internal/api`; public contract changes stay in `schemas/`; client typing and
methods stay in `sdk/ts`; the web shell adds quota UX in the existing app surface rather
than creating a separate billing application.

## Complexity Tracking

No constitution violations require justification. The design is additive and follows the
existing billing, schema, SDK, and web-shell structure.

## Phase 0 Research

Research decisions are recorded in [research.md](./research.md). No unresolved
clarifications remain.

## Phase 1 Design

Design artifacts:

- [data-model.md](./data-model.md)
- [contracts/public-quota-ux.md](./contracts/public-quota-ux.md)
- [quickstart.md](./quickstart.md)

### Contract Risk Review

Contract risks, ordered by severity:

1. Evidence export can leak secrets or cross-tenant data if it reuses raw audit payloads.
   The contract requires structured redacted JSON with explicit redaction status and
   excludes connector payloads, secrets, unrelated run content, and other tenants.
2. Abuse restriction details can expose detection signals. The contract exposes status,
   affected category, duration when available, and recovery action only.
3. Dashboard projections can become semantically incompatible if existing billing response
   fields are repurposed. The contract uses additive fields/resources and keeps existing
   `tenantId`, `category`, `reasonCode`, and amount fields stable.
4. Tenant switch can show stale data. The web contract requires clearing or hiding previous
   tenant quota and denial details before rendering the new tenant.

Compatibility assessment: additive. Existing Roadmap 38 routes remain valid; existing SDK
methods continue to work. New dashboard/detail/export methods and nullable fields are added
without changing current required response fields.

Required validation cases: schema conformance, stable error semantics, `billing.evidence_export`
permission gates, tenant scoping, stale-data clearing, redaction, explicit abuse restriction
record projection, abuse threshold withholding, previous-period usage, category-defined
typical-operation near-limit warning thresholds, and fail-closed denials before side effects.

## Post-Design Constitution Check

- Roadmap closure: PASS. Design covers all in-scope Roadmap 47 surfaces and excludes
  checkout, invoices, taxes, cross-tenant pooled quota, and marketplace pricing.
- Production-grade change control: PASS. Rollback is feature-surface disablement while
  preserving Roadmap 38 enforcement. No destructive migration is planned.
- Contracts and auditability: PASS. Contract artifacts define new additive daemon, schema,
  SDK, and web behavior; support evidence remains structured and redacted.
- Verification and observability: PASS. The quickstart and contract document define tests,
  manual walkthroughs, and operator-visible support evidence.
- Environment and secrets: PASS. Test environment remains the default. Evidence export
  explicitly redacts secrets and connector payloads.
