# Implementation Plan: Skill And Local Tool Sandbox Execution

**Branch**: `004-skill-tool-sandbox` | **Date**: 2026-04-19 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/004-skill-tool-sandbox/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Close Roadmap 19 by giving executable skills a first-class execution manifest, routing
their subprocess launches and the existing high-risk local tool path (`exec`, `shell`,
`browser`) through the sandbox execution plane, and linking runtime tool-call truth back
to sandbox execution, policy, approval, and restart-recovery records without introducing a
second local execution path.

## Technical Context

**Language/Version**: Go 1.24.0; Markdown docs; JSON Schema contracts  
**Primary Dependencies**: `daemon/internal/api`, `daemon/internal/runtime`, `daemon/internal/skills`, `daemon/internal/sandbox`, `daemon/internal/store`, `daemon/internal/policy`, `daemon/internal/events`, `daemon/internal/contracts`, `modernc.org/sqlite`  
**Storage**: SQLite daemon state for runtime tool calls, approvals, sandbox executions, and consumer policy records; skill files under `~/.agents/skills` and `<dataDir>/skills`; environment-scoped config and secret refs under `~/.dope-test` or `~/.dope`  
**Testing**: `go test ./internal/skills ./internal/api ./internal/runtime ./internal/sandbox ./internal/store ./internal/policy ./internal/app ./internal/contracts`, `make daemon-contract-test`, `go test ./...`  
**Target Platform**: macOS/Linux local daemon in the default test environment  
**Project Type**: Go daemon and harness control-plane service with schema-backed HTTP and event contracts  
**Performance Goals**: Executable-skill manifest validation and in-scope local execution preflight should add no more than `<=100 ms` daemon-side overhead per request, excluding subprocess runtime, stdout/stderr volume, and operator approval wait time  
**Constraints**: Sandbox backend remains `subprocess` only; scope is limited to executable skills plus the current high-risk local tool path (`exec`, `shell`, `browser`); undeclared executable-skill approval posture defaults to `ask`; invalid executable skills stay visible as `unavailable`; in-flight executions interrupted by daemon restart recover as `cancelled`; stronger-than-backend guarantees fail as `unsupported` or `denied`; all APIs, schemas, events, and persistence changes remain additive; local verification defaults to `DOPE_ENV=test`  
**Scale/Scope**: One operator-managed skill inventory (tens to low hundreds of skills) and the current interactive local tool-call flow on a single daemon host, with low-concurrency, human-driven executions and operator-visible audit requirements for every in-scope run

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan closes one whole roadmap slice: executable-skill
  manifests, sandbox-backed execution for executable skills plus the existing high-risk
  local tool path, runtime-to-sandbox provenance, and operator verification. It does not
  absorb orchestration, broader local capability migration, or stronger backends.
- Production-grade change control: PASS. The design keeps the blast radius inside existing
  daemon packages and additive contracts. The rollout is reversible by reverting the
  executable-skill and high-risk tool migration while preserving the already-closed
  sandbox prerequisite and MCP slices.
- Contracts and auditability: PASS. The plan identifies the existing skill, runtime,
  approval, sandbox, schema, event, and documentation surfaces that must evolve together
  so no supported path bypasses the daemon-owned execution plane.
- Verification and observability: PASS. The plan requires targeted package tests, contract
  verification, restart-aware recovery coverage, and operator-visible linkage across tool
  history, approvals, and sandbox execution records.
- Environment and secrets: PASS. Local work stays in `DOPE_ENV=test`, preserves
  `~/.dope-test` / `~/.dope` separation, and keeps secret scope explicit and redacted
  across skill and tool execution surfaces.

Post-design re-check:

- PASS. Research and data modeling keep the delivery boundary on Roadmap 19 and explicitly
  defer graph orchestration, generic capability migration, remote execution, and stronger
  backends.
- PASS. The design reuses the existing tool-call, approval, and sandbox surfaces rather
  than creating a second runtime execution plane.
- PASS. Contracts remain additive and auditable across skill inspection, runtime tool
  history, sandbox execution, approval, config, and event surfaces.

## Project Structure

### Documentation (this feature)

```text
specs/004-skill-tool-sandbox/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── skill-tool-sandbox-surfaces.md
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
│   ├── policy/
│   ├── runtime/
│   ├── sandbox/
│   ├── skills/
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

**Structure Decision**: Keep the design inside existing daemon modules. Extend
`daemon/internal/skills` to parse and expose executable-skill manifests plus availability,
use `daemon/internal/api` and `daemon/internal/runtime` to represent skill launches on the
existing tool-call path, use `daemon/internal/sandbox` for actual subprocess execution and
consumer provenance, persist additive linkage in `daemon/internal/store`, and update
schema-backed contracts under `schemas/` plus operator docs under `docs/harness` and
`docs/runtime`.

## Complexity Tracking

No constitution violations remain after keeping Roadmap 19 scoped to executable skills and
the current high-risk local tool path. Broader local capability migration, graph
orchestration, and stronger backends remain explicit follow-on work rather than hidden
inside this slice.
