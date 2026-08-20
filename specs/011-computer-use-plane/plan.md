# Implementation Plan: Computer-Use Capability Plane

**Branch**: `012-computer-use-plane` | **Date**: 2026-04-22 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/011-computer-use-plane/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add a first-class daemon-owned browser-first computer-use plane that lets operators open
run-scoped browser sessions, inspect and approve high-risk actions, capture durable page
evidence, and run computer-use steps inside the existing runtime and workflow surfaces.
Roadmap 26 is closed by additive session, action, and artifact resources; risk-based
approval tied to concrete actions; explicit target-mismatch and interruption semantics;
single-page session boundaries; durable environment-scoped persistence; contract coverage;
and one manual browser-based `KURA_ENV=test` verification path.

## Technical Context

**Language/Version**: Go 1.24.0; Markdown docs; JSON Schema contracts
**Primary Dependencies**: `daemon/internal/api`, `daemon/internal/app`, new `daemon/internal/computeruse`, `daemon/internal/artifacts`, `daemon/internal/runtime`, `daemon/internal/orchestration`, `daemon/internal/policy`, `daemon/internal/events`, `daemon/internal/store`, `daemon/internal/contracts`, existing auth wiring, and a replaceable browser driver boundary that can later target `capabilities/browser`
**Storage**: SQLite daemon state with additive `computer_use_sessions`, `computer_use_actions`, and `computer_use_artifacts` persistence plus additive computer-use linkage on runtime tool-call truth; file-backed artifact content managed under the daemon-owned artifacts service
**Testing**: `go test ./internal/computeruse ./internal/api ./internal/store ./internal/app ./internal/contracts`, `make daemon-contract-test`, `go test ./...`, plus one manual browser-based `KURA_ENV=test` verification path covering session creation, approval gating, evidence capture, and target mismatch
**Target Platform**: macOS/Linux local daemon in `KURA_ENV=test` by default, using the existing localhost HTTP API and daemon-owned SQLite store
**Project Type**: Go daemon and harness control-plane service with schema-backed HTTP and event contracts
**Performance Goals**: create or inspect a computer-use session from persisted local state in `<=500 ms`; record action status and linked evidence metadata in `<=1 s` after a single-page browser action completes on local test hardware; return artifact metadata lookups in `<=500 ms` for recent artifacts in a single-user test environment
**Constraints**: browser-first only; one active page per session; sessions may be reused only within one run or workflow and never across runs or schedule dispatches; high-risk actions require action-scoped approval; target mismatch fails immediately and requires renewed inspection; restart must mark in-flight work interrupted rather than silently resuming; screenshots, snapshots, and downloads remain environment-scoped durable evidence; phase 26 excludes generalized desktop automation, multi-tab browsing, and hidden local-tool fallbacks
**Scale/Scope**: one operator-managed daemon handling low tens of active computer-use sessions, one active page per session, low hundreds of actions and artifacts per day, and mixed workflows that combine browser steps with other runtime capabilities

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan closes Roadmap 26 only: first-class browser-first
  computer use with session and action truth, risk-based approvals, artifact-backed
  evidence, workflow integration, and restart-safe interruption semantics. Generalized
  desktop automation, multi-tab browsing, and later integration domains remain out of
  scope.
- Production-grade change control: PASS. The design adds a new daemon-owned
  `computeruse` control-plane package plus additive API, store, artifact, and event
  surfaces while reusing the existing run/workflow/tool-call execution boundary. Rollback
  is a single change-set revert of computer-use-specific resources, linkage fields, and
  docs while preserving already-recorded runtime and artifact history as historical truth.
- Contracts and auditability: PASS. The plan names additive session/action/artifact
  routes, schema files, event families, persistence tables, runtime linkage fields, and
  docs that must change together so operators can inspect approvals, actions, target
  mismatch, evidence, and interruptions without raw logs.
- Verification and observability: PASS. The design requires targeted computer-use, API,
  store, app restart, and contract coverage plus one manual `KURA_ENV=test` browser path.
  Operator-visible resources and events replace implicit browser-driver logs as the source
  of truth for approval, target mismatch, navigation failure, unavailable consumer, and
  interruption outcomes.
- Environment and secrets: PASS. Local work stays in `KURA_ENV=test`, no production
  connectors are required, browser evidence remains environment-scoped, and any captured
  content follows existing secret-handling and redaction rules.

Post-design re-check:

- PASS. The design remains roadmap-closed: it delivers browser-first computer-use
  capability work without drifting into generalized desktop automation or future domain
  integrations.
- PASS. Concrete browser actions still execute through normal runtime step and tool-call
  truth; the new plane owns inspection, persistence, and driver coordination, not a second
  hidden executor.
- PASS. API, schema, event, persistence, and artifact surfaces stay additive and
  auditable, with explicit rollback and backward-compatibility posture.

## Project Structure

### Documentation (this feature)

```text
specs/011-computer-use-plane/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── computer-use-surfaces.md
└── tasks.md
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── api/
│   ├── app/
│   ├── artifacts/
│   ├── capabilities/
│   ├── computeruse/
│   ├── contracts/
│   ├── events/
│   ├── orchestration/
│   ├── policy/
│   ├── runtime/
│   └── store/
└── go.mod

schemas/
├── api/
└── events/

docs/
├── harness/
└── runtime/

AGENTS.md
```

**Structure Decision**: Keep the browser control plane isolated in
`daemon/internal/computeruse`, where session lifecycle, action orchestration, target-match
evaluation, and driver abstraction live. `daemon/internal/api` exposes additive
run-scoped computer-use routes and artifact retrieval surfaces. `daemon/internal/artifacts`
owns evidence metadata and file-backed content. `daemon/internal/runtime` and
`daemon/internal/orchestration` remain the only execution owners after a computer-use
action is requested, with additive linkage fields on tool-call truth. `daemon/internal/app`
restores persisted computer-use state and marks interrupted sessions or actions after
restart. `daemon/internal/store`, `schemas/`, and `docs/` carry the new session/action/
artifact contracts and operator guidance. `AGENTS.md` points to this plan for later task
generation.

## Complexity Tracking

No constitution violations remain. The design avoids a second execution plane, avoids
cross-run session reuse, avoids multi-tab browser state in phase 26, and avoids making a
dedicated isolated browser worker a prerequisite for closing the roadmap.

## Implementation Notes

- Add daemon-owned computer-use persistence for sessions, actions, and artifacts rather
  than overloading generic session or capability tables.
- Extend runtime tool-call truth with additive computer-use linkage and expose derived
  workflow visibility through existing runtime/workflow relations.
- Use action-scoped approval requests so high-risk browser operations retain inspectable
  page context and operator-visible approval truth.
- Keep artifact metadata queryable from persisted store state while storing content through
  the artifacts service for restart-safe retrieval.

## Automated Verification

- `cd daemon && go test ./internal/app ./internal/contracts ./internal/api ./internal/computeruse ./internal/store ./internal/orchestration`
- `make daemon-contract-test`
- `cd daemon && go test ./internal/api -run 'TestScheduleRoutesDispatchWorkflowTargetAndLinkWorkflowTruth|TestScheduleWorkflowComputerUseDoesNotReuseOperatorRunSession|TestWorkflowStartExecutesComputerUseStepAndProjectsEvidence|TestComputerUseSessionAndApprovalRoutes|TestComputerUseRoutesFilterArtifactsByEnvironmentAndExposeTargetMismatch' -count=1`

These commands cover:

- API, workflow, schedule, store, contract, and restart-interruption regressions
- explicit failure classes for policy denial, navigation failure, unavailable consumer,
  target mismatch, and interrupted restart recovery
- latency bounds for session create, action completion, and artifact metadata lookup in
  the local test environment

## Manual Verification

- `make daemon-run-test`
- `make daemon-test-status`
- completed local pairing, then created run `run_dfa00005d9412f59`
- created session `cusess_825539a53aa874bc`
- completed snapshot action `cuact_35c37fbbfcff9066` with artifact
  `cuart_ed4b8f703cdb73fe`
- approved input action `cuact_ad217f269395fd67` via `approval_ebe6411f27423da3` and
  verified artifact `cuart_3e98aec5468885a4`
- approved click action `cuact_fc086c4ec4a53aeb` via `approval_1602e87b7cfe0fd7` and
  verified terminal `target_mismatch` with artifact `cuart_e652dd1db93e7da2`
- confirmed `/v1/computer-use/artifacts/{artifactId}` and `/content` retrieval, plus
  event history containing `computer_use.artifact_recorded` and
  `computer_use.action_target_mismatch`

## Residual Risks

- The current phase 26 surface remains intentionally browser-first and single-page only;
  callers that expect tab management, uploads, or generalized desktop automation still
  receive explicit rejection rather than degraded support.
- Latency checks are local test-environment assertions, not a production benchmark; slower
  hosts may need broader operating thresholds before the limits become release gates.
- The manual verification path uses the deterministic in-process driver and API routes; it
  does not validate a future external browser backend.

## Rollback Notes

- Rollback is a single change-set revert of computer-use API routes, store tables,
  artifact persistence wiring, runtime linkage fields, and event/schema/docs updates.
- Already-recorded runs, tool calls, approvals, and artifacts remain valid historical
  truth even if computer-use routes are later disabled.
- If a rollback is needed after enabling these routes, stop issuing new computer-use
  actions first, then revert the daemon change set and preserve the existing SQLite rows
  plus artifact files as read-only historical evidence.
