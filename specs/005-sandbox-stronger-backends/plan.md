# Implementation Plan: Sandbox Stronger Backends

**Branch**: `005-sandbox-stronger-backends` | **Date**: 2026-04-19 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/005-sandbox-stronger-backends/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Close Roadmap 20 by adding `docker` as the first stronger isolation-capable sandbox
backend, making backend capability differences explicit in inspection and explain
surfaces, and proving that executable skills can opt into `docker` without introducing a
second execution plane, silent fallback, or loss of operator-visible provenance.

## Technical Context

**Language/Version**: Go 1.24.0; Markdown docs; JSON Schema contracts
**Primary Dependencies**: `daemon/internal/sandbox`, `daemon/internal/api`, `daemon/internal/skills`, `daemon/internal/runtime`, `daemon/internal/store`, `daemon/internal/contracts`, `daemon/internal/events`, `daemon/internal/app`, `modernc.org/sqlite`
**Storage**: SQLite daemon state for sandbox executions, runtime tool calls, approvals, and event history; skill files under `~/.agents/skills` and `<dataDir>/skills`; sandbox profiles and host capability state projected from daemon-owned config/runtime inspection
**Testing**: `go test ./internal/sandbox ./internal/api ./internal/skills ./internal/runtime ./internal/store ./internal/app ./internal/contracts`, `make daemon-contract-test`, `go test ./...`
**Target Platform**: macOS/Linux local daemon in the default test environment, with Roadmap 20 verification requiring hosts where `docker` is either explicitly available or explicitly absent for negative-path coverage
**Project Type**: Go daemon and harness control-plane service with schema-backed HTTP and event contracts
**Performance Goals**: Backend selection, capability evaluation, and explain/preflight work should remain low-latency operator interactions, adding no more than `<=100 ms` daemon-side decision overhead per request, excluding container image pull/startup time and runtime execution duration
**Constraints**: `docker` is the first stronger backend; only explicitly declared `docker` executable skills migrate in this slice; `docker`-required requests fail as `unsupported` when the host cannot satisfy prerequisites; no silent fallback to `subprocess`; APIs, schemas, events, and persistence changes remain additive; local verification defaults to `KURA_ENV=test`; broader migration of high-risk local tools, MCP, and managed providers remains out of scope
**Scale/Scope**: Single-daemon-host execution with tens to low hundreds of skills, one initial stronger-backend migration target family (executable skills), operator-visible capability comparison between `subprocess` and `docker`, and one host-level capability matrix suitable for future migration work

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan closes one whole roadmap slice: backend capability
  contracts, one stronger backend (`docker`), truthful unsupported semantics, and one
  real executable-skill migration target. It does not absorb broad capability migration,
  orchestration, remote execution, or VM-grade isolation.
- Production-grade change control: PASS. The design keeps the blast radius centered in
  existing sandbox, skill, runtime, API, contract, and documentation packages. Rollback
  removes `docker` backend selection and opt-in migration while preserving the already
  closed subprocess-backed sandbox plane.
- Contracts and auditability: PASS. The plan identifies the profile, explain, execution,
  skill-inspection, runtime, schema, event, and operator-doc surfaces that must evolve
  together so backend guarantees and mismatch outcomes remain explicit.
- Verification and observability: PASS. The plan requires targeted sandbox/runtime/API
  tests, contract verification, restart coverage, and end-to-end stronger-backend
  validation for one real consumer plus negative-path unsupported coverage.
- Environment and secrets: PASS. Local work stays in `KURA_ENV=test`, keeps
  `~/.kura-test` / `~/.kura` separation intact, and does not weaken existing secret-scope
  or redaction guarantees when `docker` is used.

Post-design re-check:

- PASS. Research and data modeling keep the delivery boundary on Roadmap 20 and explicitly
  defer broader local-tool migration, SSH/remote backends, and VM-grade isolation.
- PASS. The design keeps one sandbox control plane and one runtime execution story rather
  than creating a backend-specific parallel API or executor.
- PASS. Contracts remain additive and auditable across sandbox inspection, executable-skill
  declaration, runtime history, event surfaces, and operator guidance.

## Project Structure

### Documentation (this feature)

```text
specs/005-sandbox-stronger-backends/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── sandbox-stronger-backend-surfaces.md
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

**Structure Decision**: Keep the roadmap inside existing daemon modules. Extend
`daemon/internal/sandbox` with backend capability metadata, `docker` host checks, and
launch behavior; use `daemon/internal/api` to project backend capability and mismatch
truth; update `daemon/internal/skills` so executable skills can opt into `docker` without
changing current subprocess defaults; preserve additive provenance through
`daemon/internal/runtime` and `daemon/internal/store`; and update schema-backed contracts
under `schemas/` plus operator docs under `docs/harness` and `docs/runtime`.

## Complexity Tracking

No constitution violations remain after keeping Roadmap 20 scoped to one stronger backend
(`docker`), one first migration target (executable skills), and explicit operator-facing
capability/mismatch truth. Broader migration, remote execution, and stronger isolation
classes beyond `docker` remain explicit follow-on work rather than hidden inside this
slice.
