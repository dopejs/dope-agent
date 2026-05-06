# Implementation Plan: Hosted Signup And Tenant Activation

**Branch**: `030-hosted-tenant-activation` | **Date**: 2026-05-06 | **Spec**: [`spec.md`](./spec.md)
**Input**: Feature specification from `specs/030-hosted-tenant-activation/spec.md`

## Summary

Close Roadmap 45 by adding a product-owned hosted activation layer that creates or
resolves each authenticated hosted user's personal tenant, projects activation readiness
and quota baseline, exposes the state to SDK and web shell clients, and requires a v1
test chat action before activation is complete. The implementation is additive over the
existing tenant identity, token grant, billing quota, operator onboarding, and chat query
surfaces. It must remain tenant-scoped, idempotent under repeated or concurrent signup,
durable across daemon restart, diagnosable through stable reason codes, and metadata-only
for test chat audit and diagnostics.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; TypeScript 5.7 SDK; React 19 web
shell; JSON Schema API contracts under `schemas/api/`; Markdown docs and Spec Kit
artifacts.
**Primary Dependencies**: Existing tenant identity and resolver behavior in
`daemon/internal/identity`, tenant context and protected route behavior in
`daemon/internal/api`, SQLite store and migrations in `daemon/internal/store`, billing
projection and quota-state behavior in `daemon/internal/billing`, tenant audit events,
operator onboarding projection in `daemon/internal/api/operator_projection.go`, chat query
support in `daemon/internal/chat` and `/v1/chat/query`, SDK tenant options in
`sdk/ts/src/index.ts`, web shell first-run UI in `web/src/app/App.tsx`, and schema
contract validation under `daemon/internal/contracts`.
**Storage**: Existing SQLite daemon state remains authoritative. Add a durable,
tenant-scoped activation record and metadata-only activation audit evidence. Activation
records are uniquely scoped by principal and personal tenant so repeated and concurrent
activation attempts converge on one state. Test chat transcripts and message content are
not persisted in activation records, diagnostics, audit events, fixtures, or logs.
**Testing**: Targeted Go unit/integration tests for personal tenant resolution,
activation idempotency, concurrent activation, eligibility denials, quota-baseline
blocking, test chat completion, metadata-only audit, tenant isolation, and restart
durability. Contract tests via `make daemon-contract-test` for new activation schemas and
fixtures. SDK/web tests through `pnpm test:clients`; `pnpm build` if client bundles
change. Run `go test ./...` in `daemon/`, `make daemon-run-test`, `make
daemon-test-status`, and `go mod tidy` from `daemon/` after implementation.
**Target Platform**: Hosted/test daemon and browser shell using the default test
environment (`DOPE_ENV=test`, `~/.dope-test`, `127.0.0.1:19192`) for local validation.
Hosted production behavior uses the same protected daemon contracts, but live connectors,
payment checkout, enterprise SSO, production secrets, and organization administration are
out of scope.
**Project Type**: Multi-surface product change: Go daemon API and persistence, JSON
Schema contracts, TypeScript SDK library, React operator shell, and operator-facing docs.
**Performance Goals**: A first-run shell can show active tenant, environment, quota
baseline, readiness state, and next action within 30 seconds. Operators can identify a
failing activation stage and remediation class in 10 minutes or less. Activation
projection should reuse existing identity and billing reads and avoid extra sequential
round trips where the SDK/web shell can load projections concurrently.
**Constraints**: Preserve existing tenant, token, billing, onboarding, and chat contracts
unless adding optional fields or new endpoints. Activation completion is blocked while
quota baseline is unavailable. Any authenticated hosted user is eligible unless disabled
or denied. Organization onboarding is additive and cannot block personal activation. The
v1 safe first action is test chat. Activation diagnostics and audit must not retain test
chat transcripts or message content. Default validation must not touch `~/.dope`, live
connectors, production secrets, payment credentials, enterprise identity credentials, or
privileged organization setup.
**Scale/Scope**: One personal tenant activation per authenticated hosted user, plus
optional organization tenant context. Concurrency coverage must prove repeated or
simultaneous activation attempts for the same principal converge to one personal tenant
and one activation state. The first release covers one required v1 action (`test_chat`)
and a bounded reason-code set for activation failures.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** - PASS. The plan closes Roadmap 45 end-to-end: hosted signup or
  invite acceptance projection, personal tenant creation/resolution, activation state,
  quota baseline readiness, test chat completion, SDK/web support, stable failures,
  tenant-scoped audit, restart durability, and manual `DOPE_ENV=test` walkthrough.
- **Production-grade, minimal, reversible change** - PASS. The design is additive:
  introduce activation state/contracts and reuse existing identity, billing, onboarding,
  SDK tenant options, and chat behavior. Rollback disables activation routes and shell
  affordances while preserving existing tenant identity, billing, chat, and audit records.
- **Contracts and auditability** - PASS. New API, SDK, schema, event/audit, and
  persistence surfaces are explicit in planning contracts. Schema and fixture updates must
  ship with contract tests. Activation audit records are tenant-scoped and metadata-only
  for test chat.
- **Verification and observability** - PASS. The plan names daemon, contract, SDK, web,
  restart, tenant isolation, concurrency, redaction, and manual test-environment
  walkthrough verification.
- **Environment and secrets** - PASS. Development and automated verification default to
  `DOPE_ENV=test` and must not require live connectors or production secrets. Diagnostics,
  audit, logs, fixtures, and activation records must suppress raw secrets and test chat
  message content.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/030-hosted-tenant-activation/
|-- plan.md
|-- research.md
|-- data-model.md
|-- quickstart.md
|-- contracts/
|   |-- activation-api-contract.md
|   |-- sdk-web-shell-activation-contract.md
|   `-- activation-audit-diagnostics-contract.md
|-- checklists/
|   `-- requirements.md
`-- tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
daemon/
|-- internal/
|   |-- activation/                 # activation service, state machine, reason codes,
|   |                                # metadata-only test chat completion helpers
|   |-- api/                        # protected /v1/activation routes, onboarding
|   |                                # projection integration, activation tests
|   |-- store/                      # SQLite activation persistence and migration
|   |-- identity/                   # reuse principal, membership, token grant, audit
|   |-- billing/                    # reuse quota baseline and quota-state unavailable
|   |-- chat/                       # reuse bounded chat execution; no activation
|   |                                # transcript persistence
|   `-- contracts/                  # API schema fixture validation
|-- go.mod
`-- go.sum

schemas/
|-- api/                            # activation state/request/response/error schemas
`-- events/                         # update only if activation emits versioned events

sdk/ts/
`-- src/
    |-- index.ts                    # activation resource types and client methods
    `-- index.test.ts               # SDK activation contract tests

web/
`-- src/
    |-- app/
    |   |-- App.tsx                 # hosted activation panel/state/action wiring
    |   `-- App.test.tsx            # activation UX, quota block, metadata-only cases
    `-- styles.css                  # activation surface styling if needed

docs/
|-- runtime/                        # hosted activation/operator notes if needed
`-- specs/                          # upstream roadmap doc remains authority
```

**Structure Decision**: Add a small `daemon/internal/activation` package for activation
state and reason-code logic rather than spreading state-machine behavior across handlers,
SDK, and UI. Keep tenant resolution in `identity`, quota projection in `billing`, chat
execution in `chat`, and first-run rendering in the existing operator shell. Add new API
schemas and SDK methods instead of changing existing tenant or chat response semantics.

## Roadmap 45 Planning Contracts

The implementation plan MUST keep these artifacts complete before `/speckit.tasks`:

- [`contracts/activation-api-contract.md`](./contracts/activation-api-contract.md) -
  protected activation endpoints, activation state shape, idempotency, eligibility,
  quota readiness, test chat action, and stable reason-code behavior.
- [`contracts/sdk-web-shell-activation-contract.md`](./contracts/sdk-web-shell-activation-contract.md) -
  SDK activation and diagnostic methods, shell first-run rendering, tenant switch behavior,
  disabled states, and client-side expectations.
- [`contracts/activation-audit-diagnostics-contract.md`](./contracts/activation-audit-diagnostics-contract.md) -
  activation audit events, diagnostics projection, metadata-only test chat evidence,
  redaction requirements, and operator troubleshooting fields.

These artifacts are planning gates. Implementation is incomplete if activation can be
marked complete without quota baseline readiness, without the required test chat action,
  without tenant-scoped audit evidence, without durable restart behavior before and after
  the first action, or while retaining test chat transcripts or message content in
  activation records or diagnostics.

## Migration And Rollback Plan

1. Add activation schemas, contracts, and daemon tests before exposing web shell behavior.
2. Add SQLite activation persistence with a unique principal/personal-tenant activation
   identity and idempotent upsert semantics. Existing tenants are resolved and reused; no
   existing tenant records are rewritten.
3. Add protected activation projection and activation start/test-chat routes. Keep
   existing tenant, billing, onboarding, and chat routes compatible.
4. Add SDK methods and web shell activation UI behind the same active-tenant context used
   by existing shell views.
5. Add operator diagnostics and audit metadata only after activation state transitions are
   durable and contract-tested.

Rollback disables activation routes and hides the web shell activation surface while
leaving existing tenant identity, token grants, billing projections, chat endpoints, and
already-written activation audit records readable. If a migration is added, rollback must
preserve the table or provide a reversible migration that does not delete tenant identity
or audit history.

## Post-Design Constitution Check

- **Roadmap closure** - PASS. `research.md`, `data-model.md`, `quickstart.md`, and the
  three contracts cover the full Roadmap 45 activation slice and explicitly keep
  enterprise SSO, payment checkout, organization administration, live connectors,
  production secrets, memory, context recall, and personalized knowledge behavior out of
  scope.
- **Production-grade, minimal, reversible change** - PASS. Design is staged and additive:
  contracts/schemas, activation state persistence, protected daemon routes, SDK wrappers,
  web shell activation view, diagnostics, and verification. Rollback leaves existing
  identity, billing, chat, and onboarding behavior intact.
- **Contracts and auditability** - PASS. Contracts define endpoint shape, SDK methods,
  schema changes, failure reason codes, audit metadata, redaction, and idempotency.
  Additive schema evolution is required for all client-facing changes.
- **Verification and observability** - PASS. Quickstart requires Go tests, contract tests,
  SDK/web tests, restart durability, tenant isolation, concurrency/idempotency, redaction,
  and manual test-environment walkthrough evidence.
- **Environment and secrets** - PASS. Design defaults to `DOPE_ENV=test`, avoids live
  connectors and production secrets, and forbids raw secrets and test chat message content
  in activation diagnostics and audit records.

No post-design violations require justification.

## Complexity Tracking

> Filled only when Constitution Check has unjustified violations. None for this plan.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    |            |                                     |
