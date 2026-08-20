# Implementation Plan: Sandbox Requirement Declaration Contract

**Branch**: `002-sandbox-requirement-contract` | **Date**: 2026-04-19 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-sandbox-requirement-contract/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Close the remaining pre-MCP sandbox prerequisite work by introducing a shared
consumer-owned requirement declaration contract, per-consumer-instance secret scope, and
durable consumer provenance across the current adopter surfaces already present in the
repository. The plan explicitly recuts the roadmap boundary so the existing daemon-owned
high-risk tool path converges onto sandbox truth now, while generic MCP lifecycle and
generic executable-skill subprocess support stay outside this slice.

## Technical Context

**Language/Version**: Go 1.24.0; Markdown docs; JSON Schema contracts  
**Primary Dependencies**: `daemon/internal/sandbox`, `daemon/internal/managedproviders`, `daemon/internal/runtime`, `daemon/internal/policy`, `daemon/internal/skills`, `daemon/internal/api`, `daemon/internal/store`, `daemon/internal/events`, `modernc.org/sqlite`  
**Storage**: SQLite daemon state; skill and overlay files under `<dataDir>/skills`, `<dataDir>/AGENTS.md`, and `~/.agents`; config and environment-backed secret material; managed-provider local state under environment-specific homes  
**Testing**: `go test ./internal/sandbox ./internal/managedproviders ./internal/api ./internal/store ./internal/policy ./internal/runtime ./internal/skills ./internal/app ./internal/contracts`, `make daemon-contract-test`, `go test ./...`  
**Target Platform**: macOS/Linux local daemon in the default test environment  
**Project Type**: Go daemon and harness control-plane service with HTTP API, schema-backed contracts, and subprocess sandbox backend  
**Performance Goals**: Requirement evaluation, secret-scope resolution, and provenance persistence should add no more than `<=100 ms` daemon-side overhead to current managed-provider and high-risk tool preflight paths, excluding upstream provider latency or operator approval wait time  
**Constraints**: Current backend remains `subprocess` only; requests that require stronger guarantees MUST fail as unsupported or denied; contract and schema changes must stay additive; secret authorization is per consumer instance with reusable consumer-kind defaults; denied and preflight-only decisions must be durable; test and prod separation must remain intact; no new unmanaged execution plane may be introduced  
**Scale/Scope**: 2 managed providers (`claude_managed`, `codex_managed`), current skill registry and explicit skill-selection surfaces, and the existing daemon-owned high-risk tool path (`exec`, `shell`, `browser`) as the current local-tool runtime family; MCP runtime lifecycle and generic bundled-script skill execution remain out of scope

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS with explicit recut. The clarified feature now spans the unfinished
  Roadmap 17 prerequisite work plus the already-existing daemon-owned high-risk tool path
  that would otherwise remain a consumer-specific exception outside sandbox truth. This
  plan does not claim Roadmap 19 closure: generic executable-skill manifests, generic
  local-tool routing, and full operator docs for arbitrary tool execution remain open.
- Production-grade change control: PASS. The design keeps the blast radius inside daemon
  control-plane packages, persistence, schema/event contracts, and harness docs. It avoids
  a new backend, keeps contract changes additive, and preserves rollback to the current
  per-surface behavior if regressions appear.
- Contracts and auditability: PASS. The plan names the API, schema, event, config, and
  persistence surfaces that must change together so consumer declarations, secret scope,
  and provenance do not end up split across undocumented side paths.
- Verification and observability: PASS. The plan requires targeted package tests, contract
  validation, restart-aware provenance checks, and operator-visible explanation updates for
  denied and preflight-only paths.
- Environment and secrets: PASS. The plan keeps local work in `KURA_ENV=test`, preserves
  `~/.kura-test` / `~/.kura` separation, and treats secret scope, redaction, and env
  injection as first-class operator-facing behavior rather than incidental config handling.

Post-design re-check:

- PASS. Research decisions keep the slice on the existing subprocess backend and reject
  stronger-guarantee declarations instead of silently degrading them.
- PASS. The design introduces a consumer-owned declaration and provenance model that can be
  projected onto current managed-provider, skill, and local-tool surfaces without claiming
  MCP lifecycle or generic bundled-script execution already exists.
- PASS. Contract changes remain additive and auditable across existing API routes, event
  payloads, config inspection, and runtime/tool-call resources.

## Project Structure

### Documentation (this feature)

```text
specs/002-sandbox-requirement-contract/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── consumer-sandbox-surfaces.md
└── tasks.md
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── api/
│   ├── app/
│   ├── contracts/
│   ├── managedproviders/
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

**Structure Decision**: This feature remains entirely inside the daemon control plane,
schema/event boundary, and harness/runtime documentation. The implementation focus is on
the shared sandbox contract model, current consumer adoption points, durability in SQLite,
and additive operator-visible surfaces; no client, web, TUI, or SDK-specific work is
required.

## Complexity Tracking

No constitution violations remain after the roadmap recut is documented in the plan. This
slice is intentionally narrow about backend scope and intentionally honest that generic MCP
lifecycle and generic executable-skill subprocess behavior remain later-roadmap work.
