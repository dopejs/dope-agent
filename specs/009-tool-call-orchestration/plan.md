# Implementation Plan: Tool-Call Orchestration

**Branch**: `010-tool-call-orchestration` | **Date**: 2026-04-21 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/009-tool-call-orchestration/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add a first-class daemon-owned workflow orchestration layer on top of the existing runtime
tool-call plane. The phase introduces goal-driven workflow planning, operator-visible and
frozen workflow plans, bounded step retry without mid-run replanning, mixed-family
workflow execution across MCP tools, local tools, and executable skills, and additive
workflow schemas, events, persistence, and docs. Concrete execution remains on the
existing `Run` / `Step` / `ToolCall` runtime boundary, and daemon restart preserves
workflow audit truth by marking in-flight workflows as interrupted rather than
auto-resuming them.

## Technical Context

**Language/Version**: Go 1.24.0; Markdown docs; JSON Schema contracts
**Primary Dependencies**: `daemon/internal/api`, `daemon/internal/runtime`, `daemon/internal/store`, `daemon/internal/events`, `daemon/internal/policy`, `daemon/internal/sandbox`, `daemon/internal/mcp`, `daemon/internal/skills`, `daemon/internal/contracts`, plus a new `daemon/internal/orchestration` package
**Storage**: SQLite daemon state and event history; existing `runs`, `steps`, and `tool_calls` persistence remains authoritative for concrete execution, with additive workflow persistence for workflow plans, workflow steps, dependency edges, handoff records, and interrupted-state truth under the same environment-scoped daemon store
**Testing**: `go test ./internal/orchestration ./internal/api ./internal/runtime ./internal/store ./internal/app ./internal/contracts`, `make daemon-contract-test`, `go test ./...`, plus one manual `KURA_ENV=test` mixed-workflow verification using at least two consumer families
**Target Platform**: macOS/Linux local daemon in `KURA_ENV=test` by default, with localhost-accessible verification fixtures for mixed MCP plus local-tool or skill workflows
**Project Type**: Go daemon and harness control-plane service with schema-backed HTTP and SSE event contracts
**Performance Goals**: workflow planning returns a persisted inspectable plan in `<=2 s` for workflows up to 10 planned steps on local test hardware; ready-step scheduling adds no more than `<=100 ms` daemon-side overhead per step transition excluding actual tool latency; workflow inspection remains `<=500 ms` from persisted local state
**Constraints**: orchestration MUST stay on the existing runtime tool-call plane; workflow input is goal-driven and inspectable before execution; approval remains step-scoped with predeclared expectations on the workflow plan; workflow execution is plan-frozen once started; retries are bounded per step; automatic mid-run replanning is out of scope; daemon restart preserves workflow truth but interrupts in-flight execution; existing single-step tool-call routes remain backward compatible and valid
**Scale/Scope**: one operator-managed daemon handling low tens of workflow runs, each workflow typically spanning 2-10 steps, at least two consumer families in the mixed-workflow verification case, and a low single-digit retry budget per retriable step

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan closes Roadmap 24 only: planning and selection truth, workflow execution semantics, mixed-family coordination, additive contracts, docs, and end-to-end verification. Context engineering, memory-driven planning, and autonomous improvement remain explicitly out of scope.
- Production-grade change control: PASS. The design adds a new orchestration subsystem and additive workflow surfaces while reusing the current runtime step and tool-call plane for all concrete execution. Rollback is a single change-set revert of workflow resources, persistence, event families, and orchestration-specific API handlers while preserving existing run, step, and tool-call behavior.
- Contracts and auditability: PASS. The plan identifies additive HTTP routes, schemas, workflow events, persistence surfaces, docs, and run or tool-call linkage fields that must change together so workflow rationale, handoffs, retries, cancellation, and interruption truth remain operator-visible.
- Verification and observability: PASS. The plan requires targeted orchestration package tests, runtime and store regressions, contract coverage, and a manual `KURA_ENV=test` mixed workflow. Workflow planning, execution, retry, blocked, partial-failure, and interruption states all become explicit operator-visible records instead of implicit logs.
- Environment and secrets: PASS. The design keeps local work in `KURA_ENV=test`, reuses existing approval and secret-redaction behavior per concrete step, and avoids any new bypass path that would weaken sandbox or secret-handling rules.

Post-design re-check:

- PASS. The design remains roadmap-closed: it adds goal-driven planning, bounded frozen-plan execution, mixed-family coordination, additive contracts, docs, and verification inside a single workflow slice without expanding into context or memory systems.
- PASS. The plan preserves one execution boundary by compiling workflow steps onto existing runtime steps and tool calls rather than creating a second orchestrator-owned execution engine.
- PASS. Workflow routes, workflow state, runtime linkage, and event families stay additive and auditable across API, schema, store, and docs, with explicit rollback and backward-compatibility posture.

## Project Structure

### Documentation (this feature)

```text
specs/009-tool-call-orchestration/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── tool-call-orchestration-surfaces.md
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
│   ├── sandbox/
│   ├── skills/
│   ├── mcp/
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

**Structure Decision**: Add a dedicated `daemon/internal/orchestration` package to own
workflow planning, frozen plan persistence, dependency resolution, retry accounting, and
workflow-to-runtime linkage. `daemon/internal/api` exposes additive workflow routes under
the existing run hierarchy. `daemon/internal/runtime` remains the only concrete execution
plane for step and tool-call state transitions. `daemon/internal/store` persists workflow
records plus nullable workflow linkage on runtime records where needed for inspection and
history queries. `daemon/internal/events` and `schemas/` gain workflow-specific event and
resource contracts. `docs/` explains lifecycle, limits, failure handling, and mixed-tool
verification. `AGENTS.md` is updated to point at this plan for subsequent task generation.

## Complexity Tracking

No constitution violations remain. The design avoids a second execution boundary, avoids a
workflow-level approval bypass, and avoids mid-run replanning or automatic post-restart
resume in phase 24.
