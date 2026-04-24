# Implementation Plan: Evaluation And Replay Harness

**Branch**: `018-evaluation-replay-harness` | **Date**: 2026-04-24 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/018-evaluation-replay-harness/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Close roadmap 33 by adding a daemon-owned evaluation and replay substrate for curated
representative work and engineer-managed fixtures. The design introduces additive
evaluation records for replay candidates, replay attempts, comparison results, drift
findings, and fixture manifests; exposes schema-backed `/v1/evaluation/*` routes for
candidate inspection, non-live replay launch, attempt inspection, and plane-level
comparison; extends the TypeScript SDK and web operator shell so operators can launch and
inspect replay/comparison outcomes without raw route use; and keeps replay defaulted to
non-live evidence-preserving behavior unless the operator explicitly chooses live
validation. Verification defaults to `DOPE_ENV=test` and includes contract, persistence,
restart, fixture, SDK, web-shell, and manual before/after coverage.

## Technical Context

**Language/Version**: Go 1.26.0; TypeScript 5.7; React 19; Vite 7; Markdown docs; JSON
Schema contracts  
**Primary Dependencies**: `daemon/internal/api`, `daemon/internal/store`,
`daemon/internal/runtime`, `daemon/internal/orchestration`, `daemon/internal/scheduler`,
`daemon/internal/integrations`, `daemon/internal/computeruse`, `daemon/internal/delivery`,
`daemon/internal/policy`, `daemon/internal/events`, new `daemon/internal/evaluation`,
`sdk/ts/src`, `web/src/app`, `schemas/api`, `schemas/events`, `docs/harness`, and
`docs/runtime`  
**Storage**: existing SQLite daemon state with additive evaluation tables for candidates,
attempts, comparisons, drift findings, and fixture metadata; repo-managed fixture
definitions and captured fixture evidence where appropriate  
**Testing**: `cd daemon && go test ./internal/evaluation ./internal/api ./internal/store ./internal/contracts ./internal/app`; `make daemon-contract-test`; `pnpm test:sdk`; `pnpm test:web`; `pnpm build:web`; optional `pnpm test:clients` once implementation reaches client parity; one manual `DOPE_ENV=test` before/after replay walkthrough in the web operator shell  
**Target Platform**: local daemon plus browser-based web operator shell in `DOPE_ENV=test`
by default, with explicit environment scoping for replay evidence and no accidental
test/live mixing  
**Project Type**: Go daemon control-plane service plus TypeScript SDK and React web
operator shell  
**Performance Goals**: candidate and attempt list routes return in `<=1 s` on local test
hardware for low-hundreds evaluation records; replay launch returns an operator-visible
accepted or blocked status in `<=2 s` excluding downstream execution latency; comparison
summary generation completes in `<=5 s` for supported deterministic fixtures; web shell
surfaces terminal replay/comparison status within `<=10 min` for the manual acceptance
flow  
**Constraints**: phase 33 is curated-only for candidate eligibility; default replay is
non-live and must not execute real side effects; live validation requires explicit
operator scope; fixture authoring is engineer-owned and repo-managed; web operator shell
support is required; comparison detail is plane-level, not full artifact equality;
knowledge-plane self-improvement, model training, and autonomous optimization remain out
of scope  
**Scale/Scope**: one operator, one active environment view at a time, low hundreds of
replay candidates and attempts in local test state, at least one reusable fixture for
schedules, integrations, and computer-use paths, and one primary web shell sufficient to
close roadmap 33

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan closes roadmap 33 only: curated replay candidates,
  non-live replay launch, plane-level comparison, engineer-managed regression fixtures,
  web-shell operator visibility, restart-safe evaluation history, and one manual
  before/after verification flow. Knowledge-plane improvement loops, model training,
  automatic optimization, broad historical replay eligibility, and in-product fixture
  authoring remain out of scope.
- Production-grade change control: PASS. The change set is additive and reversible:
  evaluation records, `/v1/evaluation/*` routes, schemas, SDK methods, and web-shell
  panels are added without removing existing run, workflow, schedule, integration,
  delivery, computer-use, approval, or operator projection routes.
- Contracts and auditability: PASS. The design identifies API resources, request/response
  schemas, event additions, SDK surfaces, fixture manifests, docs, and restart persistence
  required to keep replay and comparison auditable.
- Verification and observability: PASS. The plan names targeted daemon, contract, store,
  SDK, web, restart, and manual `DOPE_ENV=test` verification, plus operator-visible
  replay status, readiness limitations, comparison drift findings, and evidence links.
- Environment and secrets: PASS. Local planning and later verification default to
  `DOPE_ENV=test`; replay records are environment-scoped; secret-bearing evidence remains
  redacted; real side effects require explicit live validation scope.

If any gate fails, stop and resolve the gap before Phase 0 research proceeds.

Post-design re-check:

- PASS. The design remains roadmap-closed to phase 33 and does not drift into memory,
  self-improvement, broad automatic replay of all work, or operator-authored fixtures.
- PASS. Evaluation adds a focused `daemon/internal/evaluation` domain for records and
  comparison logic while replay execution still reuses existing runtime, workflow,
  schedule, integration, delivery, policy, and computer-use truth.
- PASS. API, schema, SDK, fixture, docs, and web-shell contracts are explicitly scoped and
  additive, with rollback available by disabling/removing evaluation routes and shell
  panels while preserving existing execution history.

## Project Structure

### Documentation (this feature)

```text
specs/018-evaluation-replay-harness/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── evaluation-replay-surfaces.md
└── tasks.md
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── api/
│   ├── app/
│   ├── computeruse/
│   ├── delivery/
│   ├── evaluation/
│   ├── events/
│   ├── integrations/
│   ├── orchestration/
│   ├── policy/
│   ├── runtime/
│   ├── scheduler/
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
├── runtime/
└── specs/

AGENTS.md
```

**Structure Decision**: Add `daemon/internal/evaluation` for replay candidate,
attempt, comparison, drift, fixture-manifest, and plane-level comparison behavior because
phase 33 introduces durable evaluation state and domain rules that do not belong inside
the operator projection builder. Keep API exposure in `daemon/internal/api`, persistence
in `daemon/internal/store`, and execution linkage against existing `runtime`,
`orchestration`, `scheduler`, `integrations`, `delivery`, `computeruse`, `policy`, and
`events` packages. Extend `sdk/ts/src/index.ts` with typed evaluation methods and update
`web/src/app` so the primary web operator shell can select curated replay candidates,
launch non-live replay attempts, and inspect comparison results. Keep `schemas/` and
`docs/` aligned with every new API or event surface. Update `AGENTS.md` to point to this
plan for downstream task generation.

## Complexity Tracking

No constitution violations remain. A new `daemon/internal/evaluation` package is justified
because replay candidates, attempts, fixtures, and comparisons are durable domain objects
with lifecycle and comparison rules; placing this inside `api` or `runtime` would mix
cross-domain evaluation policy with transport handlers or normal execution state. The
design avoids a second executor, avoids browser-owned comparison truth, avoids broad
automatic replay eligibility, and avoids live side effects by default.

## Implementation Notes

- Add a focused evaluation domain:
  - `daemon/internal/evaluation/types.go`
  - `daemon/internal/evaluation/manager.go`
  - `daemon/internal/evaluation/fixtures.go`
  - `daemon/internal/evaluation/comparison.go`
- Add durable store support for replay candidates, replay attempts, comparison results,
  drift findings, and fixture metadata in `daemon/internal/store`.
- Keep replay candidates curated:
  - engineer-managed fixture manifests are loaded from repo-owned definitions or test
    fixture directories
  - ordinary completed work is not automatically replay-candidate eligible
  - later broad eligibility can be added after the evidence model is stable
- Implement default non-live replay:
  - no real side effects execute unless live validation is explicitly requested
  - approval-gated steps become blocked or evidence-only in default mode
  - live validation scope is persisted on the replay attempt when selected
- Expose schema-backed evaluation routes:
  - `GET /v1/evaluation/replay-candidates`
  - `GET /v1/evaluation/replay-candidates/{candidateId}`
  - `POST /v1/evaluation/replay-candidates/{candidateId}/attempts`
  - `GET /v1/evaluation/replay-attempts`
  - `GET /v1/evaluation/replay-attempts/{attemptId}`
  - `POST /v1/evaluation/replay-attempts/{attemptId}/compare`
  - `GET /v1/evaluation/comparisons`
  - `GET /v1/evaluation/comparisons/{comparisonId}`
  - `GET /v1/evaluation/fixtures`
- Publish additive events for replay launch, replay completion, replay blocking,
  unreplayable replay terminal state, replay processing failure, and comparison
  completion so operator activity and diagnostics can refresh without
  browser-side event replay.
- Extend contracts in `schemas/api` and `schemas/events`; add canonical fixtures in
  `daemon/internal/contracts`.
- Extend the SDK with typed evaluation resources, query types, and methods.
- Extend the web operator shell with an evaluation/replay view that supports:
  - candidate list and readiness state
  - non-live replay launch
  - attempt status and source linkage
  - plane-level comparison summary
  - drift finding inspection with links back to authoritative run, workflow, schedule,
    integration, delivery, approval, and computer-use details
- Update operator activity/diagnostic projections only as needed to surface evaluation
  outcomes; do not make the web client reconstruct replay status from raw events.
- Update docs:
  - `docs/harness/harness-architecture.md`
  - `docs/runtime/daemon-api-and-event-model.md`
  - `docs/runtime/daemon-roadmaps.md`
  - `docs/specs/018-evaluation-and-replay-harness.md`

## Automated Verification

- `cd daemon && go test ./internal/evaluation ./internal/api ./internal/store ./internal/contracts ./internal/app`
- `make daemon-contract-test`
- `pnpm test:sdk`
- `pnpm test:web`
- `pnpm build:web`
- `pnpm test:clients` once SDK and web changes stabilize
- `cd daemon && go mod tidy`

These commands are expected to cover:

- fixture manifest loading and provenance validation
- curated replay-candidate eligibility
- default non-live replay behavior for side-effecting and approval-gated steps
- replay readiness states: fully replayable, partially replayable, blocked, and
  unreplayable
- replay attempt persistence and restart restoration
- distinct replay attempt execution status versus comparison terminal status
- plane-level comparison classification for runtime, policy, integration, delivery, and
  evidence-summary drift
- API and event schema conformance for all new evaluation surfaces
- SDK typing and web-shell rendering/action behavior
- environment scoping and secret redaction in evaluation records

## Manual Verification

- `make daemon-run-test`
- `make daemon-test-status`
- pair or reuse a local bearer token
- seed or load the repo-managed schedule, integration, and computer-use replay fixtures
- open the web operator shell
- confirm the Evaluation/Replay surface:
  - shows only curated replay candidates and engineer-managed fixtures
  - does not present ordinary completed work as candidate-eligible
  - displays replay readiness and missing evidence limitations
  - launches a default non-live replay without executing real side effects
  - marks approval-gated or side-effecting steps as blocked or evidence-only unless live
    validation is explicitly selected
  - shows replay attempt terminal status and source linkage
  - creates a plane-level comparison summary with terminal status plus runtime, policy,
    integration, delivery, and evidence-summary differences where available
  - survives daemon restart with candidate, attempt, comparison, and drift history still
    inspectable
  - keeps environment scope explicit and never mixes test and live evidence
  - meets the documented timing targets on normal local test data

## Implementation Results

Completed during `/speckit.implement`.

Implemented:

- `daemon/internal/evaluation` domain for curated fixture-backed replay candidates,
  evidence-backed non-live replay attempts, plane-level comparisons, drift findings, and
  fixture loading.
- Completed non-live replay attempts now create an `evaluation.replay` runtime run,
  persist a completed replay workflow envelope, and bind the replay runtime step to that
  workflow step, with `resultRunId` and `resultWorkflowId` returned on the replay
  attempt.
- Replay candidate registration now enforces source provenance (`sourceKind`, `sourceId`,
  and non-empty `sourceRefs`) and keeps API-created fixture candidates blocked so
  fixtures remain repo-managed.
- Replay events now carry result run/workflow scope and payload linkage for audit
  consumers.
- SQLite schema version 20 with restart-safe evaluation candidate, attempt, comparison,
  drift, and fixture records.
- `/v1/evaluation/*` API family, additive `evaluation.*` events, JSON schemas, contract
  fixtures, SDK methods, and web operator-shell Evaluation Replay panel.
- `POST /v1/evaluation/replay-candidates` for explicit curated candidate registration;
  ordinary completed work remains ineligible unless curated.
- Repo-managed schedule, integration, and computer-use fixtures under
  `daemon/internal/evaluation/testdata/fixtures`.
- Fixture replay now reads captured `evidence.json` plane summaries rather than copying
  expected comparison summaries into replay attempts.
- `live_validation` requests are explicitly blocked until a real replay executor and
  approval flow are implemented.

Verification results:

- `cd daemon && GOCACHE=/tmp/dope-go-cache go test ./internal/evaluation ./internal/app ./internal/api ./internal/store`: pass. `api` and `app` need non-sandbox local listener permission because existing tests use `httptest` listeners.
- `cd daemon && GOCACHE=/tmp/dope-go-cache go test ./internal/evaluation ./internal/api ./internal/store ./internal/contracts ./internal/app`: pass. `api` and `app` need non-sandbox local listener permission because existing tests use `httptest` listeners.
- `cd daemon && GOCACHE=/tmp/dope-go-cache go test ./...`: pass. `api` and `app` need non-sandbox local listener permission because existing tests use `httptest` listeners.
- `GOCACHE=/tmp/dope-go-cache make daemon-contract-test`: pass.
- `pnpm build:sdk`: pass.
- `pnpm test:sdk`: pass, 7 tests.
- `pnpm test:web -- --runInBand`: pass, 4 tests.
- `pnpm build:web`: pass.
- `pnpm build:clients`: pass.
- `pnpm test:clients`: pass.
- `cd daemon && GOCACHE=/tmp/dope-go-cache go mod tidy`: pass; `go.mod` and `go.sum` unchanged.

Recorded acceptance evidence:

- Supported fixture classification rate: 3 of 3 required classes loaded and candidate-linked
  (`schedule`, `integration`, `computer_use`).
- Local route timing target: candidate/fixture/replay/compare calls completed within the
  synchronous local request path, satisfying the <=1 s list-route and <=2 s replay-launch
  targets for current fixture scale.
- Replay completion target: default non-live fixture replay completed immediately, well
  under the 10-minute target.
- Drift determination target: comparison returned `matched` immediately after replay,
  well under the 5-minute target.
- Restart check: after restarting the `DOPE_ENV=test` daemon, evaluation history still
  exposed 1 manual attempt, 1 comparison, and 3 fixtures; the manual attempt remained
  `completed` and the comparison remained `matched`.

Rollback path:

- Hide the web Evaluation Replay panel and stop calling SDK evaluation methods.
- Disable or remove `/v1/evaluation/*` route registration while leaving existing runtime,
  workflow, schedule, integration, delivery, policy, and computer-use routes unchanged.
- Stop loading repo-managed fixtures by clearing the fixture directory wiring.
- Preserve evaluation tables for audit unless a deliberate migration rollback is planned.
