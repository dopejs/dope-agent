# Implementation Plan: Delivery And Notifications

**Branch**: `013-delivery-notifications` | **Date**: 2026-04-22 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/013-delivery-notifications/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add a first-class daemon-owned delivery plane that can route terminal background results
from schedules, workflows, and integration-backed work to durable delivery targets
without an active foreground session. The roadmap is closed by environment-scoped
delivery targets and preferences, single-target routing with user defaults plus optional
integration overrides, per-attempt delivery history, explicit suppression and retry
truth, routine-success digest scaffolding, additive linkage from source execution truth to
delivery outcomes, and one local `KURA_ENV=test` verification path that does not require
live connectors.

## Technical Context

**Language/Version**: Go 1.24.0; Markdown docs; JSON Schema contracts
**Primary Dependencies**: `daemon/internal/api`, new `daemon/internal/delivery`,
`daemon/internal/app`, `daemon/internal/store`, `daemon/internal/events`,
`daemon/internal/runtime`, `daemon/internal/orchestration`,
`daemon/internal/scheduler`, `daemon/internal/integrations`,
`daemon/internal/policy`, `daemon/internal/connectors`, existing auth wiring, and
existing connector-message persistence reused as transport-specific evidence for
connector-backed targets
**Storage**: SQLite daemon state with additive persistence for delivery targets,
preferences, outcomes, attempts, and summary windows; additive linkage or lookup support
from existing `runs`, `workflows`, and schedule-attempt truth; existing
`connector_messages` remain transport-specific outbound evidence rather than the primary
delivery ledger
**Testing**: `go test ./internal/delivery ./internal/api ./internal/store ./internal/app ./internal/runtime ./internal/orchestration ./internal/scheduler ./internal/integrations ./internal/policy ./internal/contracts`,
`make daemon-contract-test`, targeted delivery-routing and retry regressions, and one
manual `KURA_ENV=test` notification or digest flow
**Target Platform**: macOS/Linux local daemon in `KURA_ENV=test` by default, using the
existing localhost HTTP API, SQLite store, and operator-authenticated `/v1/*`
control-plane routes
**Project Type**: Go daemon and harness control-plane service with schema-backed HTTP and
event contracts
**Performance Goals**: create or inspect a delivery target or outcome from persisted
local state in `<=500 ms`; enqueue a delivery outcome within `<=1 s` of terminal
run/workflow completion on local test hardware; complete a single delivery attempt and
persist its outcome in `<=5 s` excluding external connector latency; close a routine
summary window and persist the emitted digest outcome in `<=2 s` once the window becomes
eligible
**Constraints**: phase 28 MUST keep delivery truth separate from execution truth and
integration readiness truth; each routed result binds to exactly one preferred target in
its environment; exhausted retries end in terminal failure on the chosen target with no
automatic failover; failures and urgent results bypass digest windows and deliver
immediately; routine-success results may be summarized; existing foreground chat reply
flows stay valid and backward compatible; live connectors are optional for initial
verification; mobile push, social-feed behavior, and marketing messaging remain out of
scope
**Scale/Scope**: one operator-managed daemon, low tens of delivery targets and
preferences, low hundreds of delivery outcomes per day, small retry budgets per outcome,
and one repo-owned local verification sink plus one connector-backed transport adapter
sufficient to close roadmap 28

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan closes roadmap 28 only: first-class delivery targets
  and preferences, background result routing, per-attempt retry and suppression truth,
  digest scaffolding, and additive source-to-delivery linkage. Calendar, mail, reminders,
  mobile push, and marketing messaging remain out of scope.
- Production-grade change control: PASS. The design adds a dedicated
  `daemon/internal/delivery` plane and additive contracts around existing runtime,
  scheduler, orchestration, and connector transport surfaces instead of broad refactors.
  Rollback is a single change-set revert of delivery-specific routes, persistence,
  dispatcher wiring, and linkage projections while preserving already-recorded delivery
  history as audit truth.
- Contracts and auditability: PASS. The plan names additive HTTP routes, API schemas,
  event families, persistence tables, source-linkage surfaces, and documentation updates
  so operators can inspect target selection, retry history, suppression, digest grouping,
  and delivery-versus-execution separation without raw transport logs.
- Verification and observability: PASS. The design requires targeted package, contract,
  restart, and manual verification plus operator-visible resources and events for each
  delivery attempt and final outcome. Residual risks are called out explicitly below.
- Environment and secrets: PASS. Local verification stays in `KURA_ENV=test`, the plan
  provides a repo-owned local sink for deterministic checks, live connectors are optional,
  and any target credentials remain operator-owned, environment-scoped, and redacted in
  operator-visible history.

Post-design re-check:

- PASS. The design remains roadmap-closed: it delivers the shared delivery plane without
  drifting into domain-specific calendar, mail, reminder, or mobile-product behavior.
- PASS. Delivery stays a separate truth domain attached additively to terminal
  run/workflow outcomes rather than rewriting run, schedule, workflow, or integration
  status semantics.
- PASS. Existing connector reply persistence is reused only as transport evidence for
  connector-backed targets; it is not misrepresented as the full delivery resource model.

## Project Structure

### Documentation (this feature)

```text
specs/013-delivery-notifications/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── delivery-plane-surfaces.md
└── tasks.md
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── api/
│   ├── app/
│   ├── connectors/
│   ├── contracts/
│   ├── delivery/
│   ├── events/
│   ├── integrations/
│   ├── orchestration/
│   ├── policy/
│   ├── runtime/
│   ├── scheduler/
│   └── store/
└── go.mod

schemas/
├── api/
└── events/

docs/
├── channels/
├── harness/
└── runtime/

AGENTS.md
```

**Structure Decision**: Keep target registration, preference resolution, dispatch
selection, retry bookkeeping, suppression, digest-window ownership, and transport adapter
coordination in a new `daemon/internal/delivery` package. `daemon/internal/api` exposes
additive delivery routes and source-resource projections. `daemon/internal/app` wires the
delivery dispatcher beside existing scheduler, runtime, orchestration, and connector
surfaces. `daemon/internal/store` persists delivery resources and lookup indexes while
keeping `connector_messages` as connector transport evidence only. Existing runtime,
scheduler, orchestration, integrations, and policy packages remain the owners of
execution truth, approval truth, and integration readiness; they only emit additive
delivery-eligible completion facts or expose additive delivery linkage. `schemas/` and
`docs/` carry the new delivery contracts and operator guidance. `AGENTS.md` points to
this plan for later task generation.

## Complexity Tracking

No constitution violations remain. The design avoids treating connector reply records as
the delivery plane, avoids broadcast or automatic failover semantics in phase 28, avoids
rewriting execution or readiness truth with delivery state, and avoids requiring live
connectors for local verification.

## Implementation Notes

- Add daemon-owned delivery resources rather than deriving operator truth from ad hoc
  connector sends or source-specific completion handlers.
- Emit delivery requests only from terminal background result boundaries: completed or
  failed scheduled runs, completed or failed workflows, and integration-backed work that
  resolves to a terminal user-facing result. Intermediate step or tool-call events do not
  independently create user delivery outcomes in phase 28.
- Reuse the existing single-operator environment assumption from earlier roadmaps:
  preferences are environment-scoped, with user defaults as the baseline and optional
  integration-specific overrides for delivery-sensitive sources.
- Keep summary behavior narrow: routine successes may accumulate inside a summary window,
  while failures and urgent results bypass the digest path and deliver immediately.
- Support one connector-backed transport adapter for operator-realistic delivery and one
  repo-owned `test_sink` adapter for deterministic `KURA_ENV=test` verification. Both
  adapters converge on the same delivery target, attempt, and outcome model.
- Preserve additive source linkage from runs, workflows, and schedule attempts to latest
  delivery outcome summaries so operators can inspect execution success beside delivery
  failure without conflating the two.

## Automated Verification

- `cd daemon && go test ./internal/delivery ./internal/api ./internal/store ./internal/app ./internal/runtime ./internal/orchestration ./internal/scheduler ./internal/integrations ./internal/policy ./internal/contracts`
- `make daemon-contract-test`
- `cd daemon && go test ./internal/api -run 'TestDeliveryRoutesExposeTargetsPreferencesSuppressionAndEvents|TestRunRoutesProjectLatestDeliverySummaryWithoutForegroundRegression|TestRunDeliveryUsesIntegrationOverrideTarget|TestWorkflowRoutesProjectLatestDeliverySummary|TestWorkflowDeliveryUsesIntegrationOverrideTarget|TestScheduleRoutesDispatchWorkflowTargetAndLinkWorkflowTruth|TestScheduleRoutesProjectLatestDeliverySummaryOntoAttempts' -count=1`
- `cd daemon && go test ./internal/delivery -count=1`
- `cd daemon && go test ./internal/app ./internal/store -run 'TestAppRunRestoresPendingDeliveryLifecycle|TestSQLiteStoreDeliveryResourcesRemainEnvironmentScoped' -count=1`

These commands are expected to cover:

- delivery target create, list, inspect, activate, and disable behavior
- user-default preference resolution plus integration-specific override selection for both run and workflow outcomes
- single-target routing, per-attempt retry history, suppression, and terminal failure
  without automatic failover
- additive linkage from source run, workflow, scheduled workflow, and schedule-attempt
  truth to delivery outcomes
- routine-success digest grouping with urgent and failed results bypassing the summary
  path
- restart-safe restoration of pending retries, open summary windows, and outcome history

Observed on 2026-04-22:

- all commands above passed on branch `013-delivery-notifications`
- manual `KURA_ENV=test` walkthrough produced queued source delivery
  `delivery_9e3c576d1c3c1de3`, summary window `summary_window_584fc46ff405b9ec`, and
  emitted digest delivery `delivery_daae990f9020dcbc`

## Residual Risks

- The first connector-backed transport adapter may expose transport-specific quirks that
  the generic delivery plane must not leak into shared delivery semantics.
- Digest grouping policy is intentionally narrow in phase 28; later reminder or memory
  work may still need additive summary controls.
- Background result emission depends on correctly identifying terminal user-facing
  outcomes. If source boundaries are chosen too broadly or too narrowly, operators may
  see duplicate notifications or missing expected results.
- The local `test_sink` can verify routing and audit truth without live connectors, but
  it does not validate third-party connector rate limits or channel-side formatting
  constraints.

## Rollback Notes

- Rollback is a single change-set revert of delivery routes, dispatcher wiring,
  persistence, schema updates, and source-linkage projections.
- Existing runs, workflows, schedules, integration resources, and connector messages
  remain valid historical truth if roadmap 28 is reverted because the delivery design is
  additive.
- If rollback occurs after delivery resources were created, preserve recorded outcomes and
  attempts as read-only audit truth even if routing is later disabled.
