# Implementation Plan: Scheduled Tasks Wakeups

**Branch**: `011-scheduled-tasks-wakeups` | **Date**: 2026-04-22 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/010-scheduled-tasks-wakeups/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add a first-class daemon-owned schedule trigger plane that lets operators create
one-time and recurring wakeups, inspect future and past dispatch truth, and launch normal
runs or workflows through the existing runtime and orchestration surfaces. The roadmap is
closed by durable environment-scoped schedule resources, operator-visible dispatch
history, bounded restart catch-up, non-reentrant recurring execution, explicit
retry/backoff semantics, IANA-timezone recurring evaluation, additive contracts and
events, and restart-safe recovery in `DOPE_ENV=test`.

## Technical Context

**Language/Version**: Go 1.24.0; Markdown docs; JSON Schema contracts  
**Primary Dependencies**: `daemon/internal/api`, `daemon/internal/app`, `daemon/internal/scheduler`, `daemon/internal/store`, `daemon/internal/events`, `daemon/internal/runtime`, `daemon/internal/orchestration`, `daemon/internal/contracts`, `daemon/internal/config`, and existing auth/policy wiring for operator-facing routes  
**Storage**: SQLite daemon state and event history with additive schedule persistence for schedules, schedule-owned launch targets, and dispatch-attempt history; existing `runs`, `workflows`, `steps`, and `tool_calls` remain authoritative for downstream execution truth after dispatch  
**Testing**: `go test ./internal/scheduler ./internal/api ./internal/store ./internal/app ./internal/contracts`, `make daemon-contract-test`, `go test ./...`, plus one manual `DOPE_ENV=test` verification covering one-time dispatch and recurring pause/resume behavior  
**Target Platform**: macOS/Linux local daemon in `DOPE_ENV=test` by default, using the existing localhost HTTP API and daemon-owned SQLite store  
**Project Type**: Go daemon and harness control-plane service with schema-backed HTTP and event contracts  
**Performance Goals**: create or inspect a schedule from persisted local state in `<=500 ms`; detect and record a due schedule within `<=1 s` of its due time on local test hardware for up to 100 active schedules; complete restart catch-up evaluation for up to 100 persisted schedules in `<=2 s` excluding downstream run/workflow execution time  
**Constraints**: schedule dispatch MUST create normal runs and optional workflows rather than hidden background execution; schedule state, history, and events MUST remain environment-scoped; restart catch-up is bounded to only the most recent overdue trigger; recurring schedules are non-reentrant; dispatch-side retries are bounded and operator-visible; recurring triggers use explicit IANA timezone semantics; launch targets are resolved from stable references at dispatch time; phase 25 excludes webhook triggers, notification delivery, natural-language routine planning, and mobile push infrastructure  
**Scale/Scope**: one operator-managed daemon handling low hundreds of schedules, low tens of due triggers at once, one active execution per schedule, and low single-digit retry budgets per dispatch attempt

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan closes Roadmap 25 only: first-class schedules, restart-safe persistence, bounded overdue catch-up, recurring pause or resume control, retry/backoff truth, additive schedule contracts, and verification. Delivery, notifications, and external trigger sources remain out of scope.
- Production-grade change control: PASS. The design expands the currently empty `daemon/internal/scheduler` package into the daemon-owned trigger loop while reusing existing run and workflow execution boundaries. Rollback is a single change-set revert of schedule resources, scheduler startup wiring, additive store tables, and schedule contracts while preserving already-created runs and workflows as historical truth.
- Contracts and auditability: PASS. The plan identifies additive HTTP routes, API schemas, schedule events, persistence tables, linkage fields on downstream execution resources, and docs that must change together so operators can inspect schedule creation, pause/resume, dispatch, retry, skipped or missed intervals, and linked runtime truth.
- Verification and observability: PASS. The design requires targeted scheduler, store, API, app recovery, and contract coverage plus one manual `DOPE_ENV=test` run. Operator-visible events and resource history replace raw-log reconstruction for dispatch failure, downstream failure, overlap skip, retry exhaustion, and restart catch-up.
- Environment and secrets: PASS. Local work stays in `DOPE_ENV=test`, schedule dispatch reuses existing run/workflow approval and policy semantics, and the roadmap adds no new secret-bearing execution path.

Post-design re-check:

- PASS. The design remains roadmap-closed: it delivers daemon-owned schedules and wakeups without expanding into delivery, external webhook triggers, or knowledge-plane planning.
- PASS. Concrete work still happens only through existing run and workflow execution truth; the scheduler is a trigger plane, not a second executor.
- PASS. API, event, schema, persistence, and operator docs stay additive and auditable, with explicit rollback and backward-compatibility posture.

## Project Structure

### Documentation (this feature)

```text
specs/010-scheduled-tasks-wakeups/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── schedule-surfaces.md
└── tasks.md
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── api/
│   ├── app/
│   ├── contracts/
│   ├── events/
│   ├── orchestration/
│   ├── runtime/
│   ├── scheduler/
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

**Structure Decision**: Keep the trigger plane isolated in `daemon/internal/scheduler`,
where due-time evaluation, non-reentrant dispatch gating, retry/backoff bookkeeping, and
restart catch-up policy live. `daemon/internal/api` exposes additive schedule routes and
serializes schedule resources. `daemon/internal/app` owns startup restore and launches the
scheduler loop in the selected environment. `daemon/internal/store` persists schedules,
schedule targets, and dispatch history while adding only additive linkage on downstream
run/workflow truth. `daemon/internal/runtime` and `daemon/internal/orchestration` remain
the only execution owners after dispatch. `schemas/` and `docs/` are updated with the new
schedule contracts and operator guidance. `AGENTS.md` points to this plan for later task
generation.

## Complexity Tracking

No constitution violations remain. The design avoids OS-level cron dependencies, avoids a
second hidden execution boundary, avoids unbounded restart replay, and avoids concurrent
re-entry for the same schedule in phase 25.

## Implementation Notes

- Implemented additive schedule persistence in SQLite schema version `12`, including
  `schedules`, `schedule_targets`, `schedule_dispatch_attempts`, and additive schedule
  linkage on `runs`, `workflows`, and persisted event scope.
- The daemon now exposes top-level `/v1/schedules` routes, starts a daemon-owned
  scheduler loop, and performs bounded catch-up on startup.
- Workflow targets now dispatch through a reusable schedule workflow launcher that creates
  a normal run, persists first-class workflow planning truth, starts execution through the
  existing workflow plane, and preserves schedule linkage on the resulting workflow and
  runtime step/tool-call truth.

## Verification Record

- `cd daemon && go test ./internal/api ./internal/scheduler ./internal/store ./internal/app ./internal/contracts`
- `make daemon-contract-test`
- `cd daemon && go test ./...`
- manual `DOPE_ENV=test` one-time schedule creation, pre-fire inspection, due-time
  dispatch, run linkage inspection, and recurring pause/resume command validation
- automated workflow-target dispatch regression proving linked workflow completion and
  runtime step/tool-call linkage

## Rollback Notes

- Rollback is a single change-set revert of schedule API routes, scheduler startup
  wiring, scheduler package behavior, additive schema version `12`, and schedule schemas.
- Already-created scheduled runs and workflows remain valid historical runtime truth even
  if schedule routes are later disabled.
