# Implementation Plan: Operator Shell And Onboarding

**Branch**: `017-operator-shell-onboarding` | **Date**: 2026-04-23 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/017-operator-shell-onboarding/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Close roadmap 32 by replacing the current chat-only `web` surface with a web-first
operator shell that projects onboarding, readiness, approvals, recent activity, and
diagnostics from daemon-owned truth. The design keeps existing domain routes
authoritative, adds only a minimal set of operator projection routes in
`daemon/internal/api`, extends the TypeScript SDK to cover those projections and reused
approval actions plus authoritative detail routes consumed behind shell-resident
inspection panels, and embeds one bounded first useful action that reuses existing
chat-query or run-creation execution paths. Verification remains defaulted to
`DOPE_ENV=test` and focuses on truthful environment scoping, approval handling inside
the shell, and server-side projections that do not drift from daemon state.

## Technical Context

**Language/Version**: Go 1.26.0; TypeScript 5.7; React 19; Vite 7; Markdown docs; JSON
Schema contracts  
**Primary Dependencies**: `daemon/internal/api`, `daemon/internal/app`,
`daemon/internal/store`, `daemon/internal/events`, `daemon/internal/policy`,
`daemon/internal/integrations`, `daemon/internal/connectors`,
`daemon/internal/capabilities`, `daemon/internal/runtime`,
`daemon/internal/orchestration`, `daemon/internal/delivery`,
`daemon/internal/computeruse`, `web/src/app`, `sdk/ts/src`, and existing auth/config
routes  
**Storage**: existing SQLite daemon state and persisted events only; phase 32 adds
derived operator projections and schemas, not new operator-shell-owned persistence tables  
**Testing**: `cd daemon && go test ./internal/api ./internal/contracts ./internal/app`,
`make daemon-contract-test`, `pnpm test:sdk`, `pnpm test:web`, `pnpm build:web`,
optional `pnpm test:clients` once implementation reaches repo-wide client parity, plus
one manual `DOPE_ENV=test` onboarding acceptance walkthrough in the browser  
**Target Platform**: local daemon plus browser-based web client in `DOPE_ENV=test` by
default, with the current environment made explicit but not switchable inside the shell  
**Project Type**: Go daemon control-plane service plus TypeScript/React web client and
shared TypeScript SDK  
**Performance Goals**: operator projection routes should return on local test hardware in
`<=1 s` for normal low-hundreds record counts; first useful action status or result
feedback should appear in the shell in `<=2 s` after route completion excluding external
provider latency; approval resolution feedback should refresh the shell in `<=2 s` after
the decision is accepted  
**Constraints**: phase 32 MUST stay web-first and must not require TUI parity; onboarding
completes when the minimum readiness set for the selected bounded first useful action is
satisfied; approvals must be actionable inside the shell; environment scoping must stay
strict and explicit; operator projections must be daemon-owned summaries rather than
client-derived truth; detailed inspection must keep existing domain routes authoritative  
**Scale/Scope**: one operator, one active environment view at a time, low hundreds of
integrations, approvals, schedules, workflows, and deliveries in local test state, and
one primary web shell sufficient to close roadmap 32 without adding multi-user admin or
remote-access features

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan closes roadmap 32 only: first-run onboarding,
  readiness views, approval inbox and resolution, operator activity inspection, health
  and diagnostics, and one bounded first useful action in a primary web shell. TUI
  parity, remote access, multi-user admin, and unrelated design-system expansion remain
  out of scope.
- Production-grade change control: PASS. The change set is additive and reversible:
  operator projection routes in `daemon/internal/api`, SDK client coverage, and a
  replacement of the current chat-only web surface. Existing raw daemon routes remain in
  place as the authoritative detail and action paths, but required operator inspection
  stays inside the shell.
- Contracts and auditability: PASS. The plan calls out new operator projection schemas,
  reused approval and authoritative detail routes, SDK contract updates, and
  documentation updates needed to keep onboarding, activity, and diagnostics auditable.
- Verification and observability: PASS. The plan names targeted Go API and contract
  tests, web and SDK tests, manual `DOPE_ENV=test` onboarding acceptance, and operator
  signals visible through the projection routes and shell-resident authoritative detail
  surfaces.
- Environment and secrets: PASS. Local planning and later verification stay in
  `DOPE_ENV=test`; the active environment is explicit in all operator projections; the
  shell explains readiness and failures without exposing secret material.

If any gate fails, stop and resolve the gap before Phase 0 research proceeds.

Post-design re-check:

- PASS. The design remains roadmap-closed to the operator shell and does not drift into
  TUI parity, remote access, or team-admin features.
- PASS. Operator summaries are daemon-owned projections over existing resources and
  events, not browser-owned shadow state.
- PASS. Approval resolution, execution detail, and domain-specific inspection remain on
  existing authoritative routes while the shell reuses those contracts through in-shell
  detail panels, preserving auditability and rollback safety.

## Project Structure

### Documentation (this feature)

```text
specs/017-operator-shell-onboarding/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── operator-shell-surfaces.md
└── tasks.md
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── api/
│   ├── app/
│   ├── capabilities/
│   ├── connectors/
│   ├── computeruse/
│   ├── delivery/
│   ├── events/
│   ├── integrations/
│   ├── policy/
│   ├── runtime/
│   ├── orchestration/
│   └── store/
└── go.mod

web/
├── src/
│   ├── app/
│   └── styles.css
└── package.json

sdk/ts/
├── src/
│   └── index.ts
└── package.json

schemas/
├── api/
└── events/

docs/
├── harness/
└── runtime/

AGENTS.md
```

**Structure Decision**: Keep all operator-shell server projections in
`daemon/internal/api` so the daemon remains the owner of onboarding, activity, and
diagnostic truth. Reuse existing managers and store-backed resources from
`integrations`, `connectors`, `capabilities`, `policy`, `runtime`, `orchestration`,
`delivery`, `computeruse`, and persisted events rather than adding a new operator-only
domain package. Extend `sdk/ts/src/index.ts` with operator projection methods and
reused approval/detail methods required by the shell. Replace `web/src/app/App.tsx` and
its tests with a web-first operator shell that consumes the SDK. Keep `schemas/` and
`docs/` aligned with any new projection contracts. Update `AGENTS.md` to point to this
plan for downstream task generation.

## Complexity Tracking

No constitution violations remain. The design avoids a second approval subsystem,
avoids browser-owned onboarding or diagnostics truth, avoids new operator-only
persistence, and avoids TUI parity work inside the roadmap 32 closure.

## Implementation Notes

- Add three daemon-owned operator projection routes:
  - `GET /v1/operator/onboarding`
  - `GET /v1/operator/activity`
  - `GET /v1/operator/diagnostics`
- Keep existing policy, integration, connector, capability, run, workflow, schedule,
  delivery, computer-use, config, auth, and event routes authoritative for detail
  data and mutation, but consume required detail inside shell-resident inspection panels
  instead of navigating operators to raw endpoints.
- Extend `daemon/internal/api/types.go` and `schemas/api/` with operator projection
  resource shapes; do not add new shell-specific persistence tables or event families
  unless implementation proves they are unavoidable.
- Follow the repository's existing projection pattern by building operator summaries in
  `daemon/internal/api` helper files rather than embedding cross-domain assembly logic
  directly in handlers or the web client.
- Keep the primary shell in `web/`; phase 32 does not require equivalent `tui/`
  behavior.
- Extend `sdk/ts` so `web` continues to consume a typed shared client instead of raw
  `fetch` calls.
- Replace the current single-turn chat console with an operator shell layout that
  includes:
  - environment and onboarding summary
  - readiness and follow-up setup surfaces
  - approval inbox with approve/reject handling
  - recent activity and diagnostics views
  - shell-resident detail panels or sheets for authoritative inspection of linked runs,
    workflows, schedules, deliveries, approvals, and computer-use records
  - one bounded first useful action panel using chat query or test run reuse
- Use `/v1/events/stream` to trigger bounded refetch of affected operator views after
  approvals, readiness changes, or recent activity changes; do not build a browser-side
  event replay system.

## Automated Verification

- `cd daemon && go test ./internal/api ./internal/contracts ./internal/app`
- `make daemon-contract-test`
- `pnpm test:sdk`
- `pnpm test:web`
- `pnpm build:web`

These commands are expected to cover:

- operator projection route behavior for onboarding, activity, and diagnostics
- approval listing and resolution flow as exercised through the shell contract
- schema contract conformance for new operator projection resources
- shared SDK typing and client behavior for new operator surfaces
- primary web-shell rendering, empty states, approval actions, and first useful action
  flow
- restart durability for onboarding state and recent operator-visible history

## Manual Verification

- `make daemon-run-test`
- `make daemon-test-status`
- pair or reuse a local bearer token
- optionally seed one degraded integration, one pending approval, one run/workflow, and
  one schedule using raw routes as developer-only setup for non-empty operator views
- `pnpm dev:web`
- open the primary web shell and confirm:
  - the active environment is explicit
  - onboarding reflects the minimum readiness set for the recommended bounded first
    useful action
  - optional follow-up setup remains visible without blocking onboarding completion
  - approvals can be approved or rejected directly from the shell
  - recent activity and diagnostics open authoritative detail inside shell-resident
    inspection panels
  - diagnostics distinguish readiness, approval, execution, and delivery blockers
  - the bounded first useful action produces immediate result or status feedback in the
    shell
  - after a daemon restart, completed onboarding state, approval records, and recent
    activity remain inspectable without resetting to a false first-run view
  - local operator projection responses and post-action refreshes stay within the stated
    latency targets on normal test data

## Implementation Results

- `go mod tidy` was run in `daemon/`; no unexpected module fallout remained afterward.
- Automated verification completed on 2026-04-24:
  - `cd daemon && go test ./internal/api ./internal/contracts ./internal/app`
  - `make daemon-contract-test`
  - `pnpm --dir sdk/ts build`
  - `pnpm --dir sdk/ts test`
  - `pnpm --dir web exec tsc --noEmit`
  - `pnpm --dir web test`
  - `pnpm --dir web build`
  - local projection timing spot-checks against a seeded `DOPE_ENV=test` daemon:
    - `/v1/operator/onboarding`: `0.038592 s`
    - `/v1/operator/activity?attentionOnly=true&limit=20`: `0.569744 s`
    - `/v1/operator/diagnostics`: `0.513144 s`
- Manual `DOPE_ENV=test` verification on 2026-04-24 confirmed:
  - local pairing produced a durable bearer token that remained valid after daemon
    restart
  - browser-based operator-shell loading succeeded against `http://127.0.0.1:4173/`
    after verifying local-origin CORS for `http://127.0.0.1:4173 -> 127.0.0.1:19192`
  - the shell showed explicit `test` environment scope, `completed` onboarding state,
    approval inbox content, event-backed recent activity, diagnostics, and same-shell
    authoritative detail for `/v1/auth/me`
  - approving `approval_6331e20225f637a9` from the shell changed the approval to
    `approved` and reduced the pending approval list to zero without leaving the shell
  - launching the recommended first useful action from the shell created
    `run_18e08dd707ade7a3` and surfaced immediate status feedback with `run.created`
    shown in the shell event banner
  - after daemon restart, onboarding still projected `completed` with
    `completedStepIds=["auth-ready","test-run-recorded"]`, the approved approval detail
    remained inspectable, and recent activity still included persisted event-backed
    `computer_use_action` records
