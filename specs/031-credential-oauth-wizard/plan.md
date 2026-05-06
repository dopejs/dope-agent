# Implementation Plan: Hosted Credential And OAuth Setup Wizard

**Branch**: `031-credential-oauth-wizard` | **Date**: 2026-05-06 | **Spec**: [`spec.md`](./spec.md)
**Input**: Feature specification from `specs/031-credential-oauth-wizard/spec.md`

## Summary

Close Roadmap 46 by adding a hosted setup-session layer that guides activated tenant
users through credential and OAuth setup without raw resource-model knowledge. The v1
proof targets are OpenAI-compatible provider credential setup for the submitted-secret
path and Feishu/Lark OAuth setup for the OAuth path. The design is additive over existing
tenant secrets, provider auth state, integration diagnostics, operator diagnostics,
tenant audit, SDK, and web shell surfaces.

Setup sessions orchestrate existing authoritative resources rather than replacing them.
They persist tenant-scoped progress, state, redacted evidence, diagnostics linkage,
retry/replace/cancel/disable transitions, and audit metadata. Raw credential material,
OAuth tokens, authorization codes, callback payloads, provider secrets, and
authorization headers never appear in setup sessions, diagnostics, audit, fixtures, logs,
or client-visible output.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; TypeScript 5.7 SDK; React 19 web
shell; JSON Schema API contracts under `schemas/api/`; Markdown docs and Spec Kit
artifacts.  
**Primary Dependencies**: Existing tenant identity and permissions in
`daemon/internal/identity`, protected tenant context handling in `daemon/internal/api`,
tenant secret management in `daemon/internal/secrets`, provider profiles/auth/checks in
`daemon/internal/providers`, integration readiness and diagnostics in
`daemon/internal/integrations`, SQLite persistence and migrations in
`daemon/internal/store`, tenant audit/events in `daemon/internal/audit` and
`daemon/internal/events`, operator diagnostics in
`daemon/internal/api/operator_projection.go`, SDK tenant request options in
`sdk/ts/src/index.ts`, web shell provider/secret/integration surfaces in
`web/src/app/App.tsx`, and contract validation in `daemon/internal/contracts`.  
**Storage**: Existing SQLite daemon state remains authoritative for tenant secrets,
provider auth, integration readiness, and diagnostics. Add durable tenant-scoped
setup-session records and setup audit references with indexed tenant, actor, target,
state, style, current diagnostic result, and updated timestamp. Setup records store only
redacted evidence and references to underlying secret/provider/integration resources.  
**Testing**: Targeted Go tests for setup-session lifecycle, permissions, state
transitions, OpenAI-compatible submitted-secret setup, Feishu/Lark OAuth setup, retry,
replace, cancel, disable, restart durability, concurrent retries, diagnostic linkage,
dependent-use gating, redaction fail-closed behavior, and tenant isolation. Contract
tests via `make daemon-contract-test`. SDK/web tests via `pnpm test:clients`; `pnpm
build` if client bundles change. Run `go test ./...` and `go mod tidy` from `daemon/`
after implementation. Use `make daemon-run-test` and `make daemon-test-status` for
manual test-environment walkthrough.  
**Target Platform**: Hosted/test daemon and browser shell using the default test
environment (`DOPE_ENV=test`, `~/.dope-test`, `127.0.0.1:19192`) for local validation.
Production behavior uses the same protected tenant contracts but live provider
credentials and real OAuth approval are not required for default verification.  
**Project Type**: Multi-surface product change spanning daemon API and persistence,
secret/provider/integration orchestration, diagnostics, audit, JSON schemas, TypeScript
SDK, React web shell, and operator docs.  
**Performance Goals**: Hosted users can identify setup state and remediation within 30
seconds. Operators can identify setup stage, stable reason code, retry safety,
remediation owner, and tenant scope in 10 minutes or less. Setup projection should reuse
existing tenant, secret, provider, and diagnostic reads without requiring users to perform
manual technical setup.  
**Constraints**: Preserve existing secret, provider auth, integration diagnostic,
activation, tenant identity, audit, SDK, and web contracts unless adding optional fields
or new endpoints. Mutating setup requires both `secrets.manage` and
`integrations.manage`; redacted inspection requires `credentials.inspect`. Setup terminal
states are `ready`, `degraded`, `unavailable`, `cancelled`, `disabled`, and `action-required`;
recoverable failures are represented through `action-required` or `unavailable` with
stable reason codes. `ready` permits normal dependent credential-bearing use; `degraded`
permits only explicitly marked limited safe use; `action-required`, `unavailable`,
`cancelled`, and `disabled` block dependent use.  
**Scale/Scope**: One or more setup sessions per tenant and target, with one current
setup state per tenant/target/style. V1 closes OpenAI-compatible submitted-secret setup
and Feishu/Lark OAuth setup end to end. Other existing domains may expose unsupported or
action-required classifications until selected for full wizard coverage.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** - PASS. The plan closes Roadmap 46 end-to-end: setup-session
  resource, submitted-secret setup, OAuth start/callback/completion state, diagnostic
  probe linkage, retry/replace/cancel/disable flows, web wizard, SDK methods, tenant
  audit, redaction, restart recovery, and operator-visible diagnostic truth.
- **Production-grade, minimal, reversible change** - PASS. The design is additive:
  setup sessions orchestrate existing secret, provider auth, integration, and diagnostic
  records. Rollback hides the wizard and disables setup-session mutation while leaving
  existing credentials, provider auth, integrations, diagnostics, and audit readable.
- **Contracts and auditability** - PASS. New API, SDK, schema, persistence, diagnostics,
  audit, redaction, proof-target, and UI behavior are captured in planning contracts.
  Contract tests and schema fixtures must ship with client-facing changes.
- **Verification and observability** - PASS. Verification covers daemon unit/integration
  tests, contract tests, SDK/web tests, restart durability, tenant isolation,
  redaction, dependent-use gating, operator diagnostics, and manual test-environment
  walkthrough evidence.
- **Environment and secrets** - PASS. Default work uses `~/.dope-test` and fake
  credential/OAuth fixtures. Live credentials, production secrets, enterprise SSO,
  external managed secret managers, and new provider domains are out of scope.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/031-credential-oauth-wizard/
|-- plan.md
|-- research.md
|-- data-model.md
|-- quickstart.md
|-- contracts/
|   |-- setup-api-sdk-contract.md
|   |-- wizard-ui-contract.md
|   |-- diagnostics-audit-redaction-contract.md
|   `-- proof-targets-and-rollback-contract.md
|-- checklists/
|   `-- requirements.md
`-- tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
daemon/
|-- internal/
|   |-- setupwizard/                 # setup-session types, state machine, redaction,
|   |                                # permission decisions, proof-target orchestration
|   |-- api/                         # protected setup wizard routes, provider/secret
|   |                                # wiring, operator diagnostics integration
|   |-- store/                       # SQLite setup-session persistence and migrations
|   |-- secrets/                     # reuse create/rotate/disable metadata behavior
|   |-- providers/                   # reuse OpenAI-compatible profile/auth/check state
|   |-- integrations/                # reuse Feishu/Lark diagnostics and reason codes
|   |-- identity/                    # reuse setup mutation and inspection permissions
|   |-- audit/ and events/           # setup audit event publication if evented
|   `-- contracts/                   # schema/fixture contract tests
|-- go.mod
`-- go.sum

schemas/
|-- api/                             # setup-session resources, requests, responses,
|                                    # diagnostics and error schemas
`-- events/                          # update only if setup emits versioned events

sdk/ts/
`-- src/
    |-- index.ts                     # setup wizard resource types and methods
    `-- index.test.ts                # SDK setup contract tests

web/
`-- src/
    |-- app/
    |   |-- App.tsx                  # hosted credential/OAuth setup wizard UI
    |   `-- App.test.tsx             # setup UX, permission, retry, redaction tests
    `-- styles.css                   # setup wizard styling if needed

docs/
|-- runtime/                         # hosted setup, rollback, redaction notes
|-- providers/                       # Feishu/Lark and OpenAI-compatible setup guidance
`-- specs/                           # upstream Roadmap 46 authority remains unchanged
```

**Structure Decision**: Add a focused `daemon/internal/setupwizard` package for
setup-session state, stable status/reason semantics, permission checks, target catalog,
redacted evidence, and orchestration boundaries. Keep secret storage in `secrets`,
provider profile/auth/check behavior in `providers`, Feishu/Lark diagnostic
classification in `integrations`, and tenant identity/permissions in `identity`. Add new
API schemas and SDK methods instead of overloading raw secret or provider routes.

## Roadmap 46 Planning Contracts

The implementation plan MUST keep these artifacts complete before `/speckit.tasks`:

- [`contracts/setup-api-sdk-contract.md`](./contracts/setup-api-sdk-contract.md) -
  protected setup endpoints, resource shape, request/response semantics, permissions,
  SDK methods, idempotency, and stable error codes.
- [`contracts/wizard-ui-contract.md`](./contracts/wizard-ui-contract.md) - web shell
  wizard entry points, submitted-secret flow, OAuth flow, retry/replace/cancel/disable
  states, redaction behavior, and tenant switch behavior.
- [`contracts/diagnostics-audit-redaction-contract.md`](./contracts/diagnostics-audit-redaction-contract.md) -
  diagnostic linkage, setup reason codes, audit events, redaction fail-closed behavior,
  operator diagnostics, and forbidden evidence fields.
- [`contracts/proof-targets-and-rollback-contract.md`](./contracts/proof-targets-and-rollback-contract.md) -
  OpenAI-compatible and Feishu/Lark proof target behavior, dependent-use gating,
  unsupported/action-required classifications, migration, rollback, and manual evidence.

These artifacts are planning gates. Implementation is incomplete if a setup target can be
marked ready without diagnostic linkage, if raw credential/OAuth material can appear in
client-visible output or persisted evidence, if setup mutation can bypass required
permissions, if recoverable failures become terminal failed setup states, or if rollback
would delete existing credential/integration/provider state.

## Migration And Rollback Plan

1. Add setup-session schemas, contracts, and daemon tests before exposing web shell
   behavior.
2. Add SQLite setup-session persistence with tenant/target/style uniqueness for current
   setup state and append-only redacted attempt evidence. Existing secret, provider auth,
   integration, and diagnostic records are not rewritten.
3. Add protected setup routes and SDK methods for listing targets, starting sessions,
   submitting secrets, starting/completing OAuth, retrying, replacing, cancelling,
   disabling, and reading diagnostics.
4. Wire OpenAI-compatible provider credential setup through existing tenant secret and
   provider profile/check behavior.
5. Wire Feishu/Lark OAuth setup through existing provider auth/integration diagnostics
   behavior and safe OAuth fixtures for default tests.
6. Add web wizard and operator diagnostics only after setup state transitions,
   permission denials, redaction, and diagnostic linkage are contract-tested.

Rollback disables setup-session mutation routes and hides the wizard while preserving
existing tenant secrets, provider auth state, integration readiness, diagnostics, and
setup audit records already written. Existing dependent credential-bearing use remains
controlled by pre-existing secret/provider/integration state if setup-session gating is
disabled during rollback.

## Post-Design Constitution Check

- **Roadmap closure** - PASS. `research.md`, `data-model.md`, `quickstart.md`, and the
  four contracts cover the full Roadmap 46 setup-session layer, including both proof
  targets, permissions, state semantics, diagnostics, redaction, SDK/web, audit, restart,
  rollback, and manual evidence.
- **Production-grade, minimal, reversible change** - PASS. Design is staged and additive:
  contracts/schemas, setup-session persistence, route shells, orchestration over existing
  secrets/providers/integrations, SDK, web wizard, diagnostics, and docs. Rollback hides
  setup mutation and leaves existing authoritative resources intact.
- **Contracts and auditability** - PASS. Contracts define setup resource shape, state
  machine, reason codes, permission denials, diagnostic linkage, audit metadata,
  redaction, proof targets, and rollback behavior.
- **Verification and observability** - PASS. Quickstart names Go tests, contract tests,
  SDK/web tests, restart durability, tenant isolation, redaction, dependent-use gating,
  daemon health smoke, and operator diagnostic evidence.
- **Environment and secrets** - PASS. Design defaults to `DOPE_ENV=test`, fake secrets,
  and safe OAuth fixtures. Live provider credentials, production secrets, external
  managed secret managers, and enterprise identity are out of scope.

No post-design violations require justification.

## Complexity Tracking

> Filled only when Constitution Check has unjustified violations. None for this plan.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    |            |                                     |
