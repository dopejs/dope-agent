# Implementation Plan: MCP Execution Plane

**Branch**: `003-mcp-execution-plane` | **Date**: 2026-04-19 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-mcp-execution-plane/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Close Roadmap 18 by introducing a daemon-managed MCP subsystem that registers MCP servers
as first-class resources, manages stdio transport and cancellation-aware lifecycle through
the existing sandbox subprocess backend, isolates credentials through the established
declaration and secret-scope model, and exposes tool availability through explicit
per-tool, per-runtime-surface policy instead of implicit server-wide defaults.

## Technical Context

**Language/Version**: Go 1.24.0; Markdown docs; JSON Schema contracts  
**Primary Dependencies**: `daemon/internal/api`, `daemon/internal/app`, `daemon/internal/sandbox`, `daemon/internal/store`, `daemon/internal/policy`, `daemon/internal/events`, `daemon/internal/contracts`, new `daemon/internal/mcp` registry and stdio transport layer, `modernc.org/sqlite`  
**Storage**: SQLite daemon state; sandbox declaration, secret-scope, and policy-record persistence; environment-scoped config and secret refs under `~/.dope-test` or `~/.dope`  
**Testing**: `go test ./internal/mcp ./internal/sandbox ./internal/api ./internal/app ./internal/store ./internal/policy ./internal/contracts`, `make daemon-contract-test`, `go test ./...`  
**Target Platform**: macOS/Linux local daemon in the default test environment  
**Project Type**: Go daemon and harness control-plane service with schema-backed HTTP and event contracts  
**Performance Goals**: MCP registration and inspection stay effectively immediate for operator use; lifecycle preflight, secret resolution, and exposure evaluation add no more than `<=100 ms` daemon-side overhead per start, restart, or cancellation attempt, excluding subprocess startup time and MCP server readiness latency, and this target must be verified by dedicated regression coverage plus quickstart recording  
**Constraints**: Sandbox backend remains `subprocess` only; enabled servers auto-restart after daemon restart when current policy and config remain valid; MCP tool exposure is explicit allowlist by tool and runtime surface; approval applies at tool exposure, not routine lifecycle; credential scope is per MCP server instance; start, stop, restart, and cancel actions all require schema-backed and auditable route closure; stronger-than-backend guarantees MUST fail as denied or unsupported; all new APIs, schemas, events, and persistence remain additive; local verification defaults to `DOPE_ENV=test`  
**Scale/Scope**: One daemon-managed MCP inventory for current runtime surfaces, expected to cover a small operator-managed fleet (single digits to low dozens of servers and hundreds of exposed tools), plus restart-safe lifecycle, audit, and exposure policy for that fleet

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan closes one whole roadmap slice: MCP registry,
  sandbox-backed lifecycle, credential isolation, tool exposure policy, and operator
  verification. It does not absorb Roadmap 19 generic tool orchestration or stronger
  backends.
- Production-grade change control: PASS. The design keeps the blast radius inside daemon
  control-plane packages, persistence, schemas, and docs. The rollout is reversible by
  reverting MCP-specific resource and lifecycle changes while preserving the existing
  sandbox substrate.
- Contracts and auditability: PASS. The plan names the API, schema, event, config,
  persistence, and sandbox surfaces that must evolve together so MCP does not create a
  hidden process-management path.
- Verification and observability: PASS. The plan requires targeted package tests,
  contract validation, restart-aware recovery coverage, and operator-visible lifecycle,
  credential, and tool-exposure evidence.
- Environment and secrets: PASS. The plan keeps local work in `DOPE_ENV=test`, preserves
  `~/.dope-test` / `~/.dope` separation, and extends the existing declaration-backed
  secret-scope model to MCP server instances.

Post-design re-check:

- PASS. Research keeps the delivery unit on Roadmap 18 and explicitly defers generic tool
  execution migration and additional backends.
- PASS. The design adds a dedicated daemon-managed MCP subsystem instead of overloading
  managed providers, capabilities, or connector supervisors with MCP-specific semantics.
- PASS. Contracts remain additive and auditable across new MCP routes plus existing
  sandbox, config, approval, and event surfaces.

## Project Structure

### Documentation (this feature)

```text
specs/003-mcp-execution-plane/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── mcp-sandbox-surfaces.md
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
│   ├── mcp/
│   ├── policy/
│   ├── runtime/
│   ├── sandbox/
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

**Structure Decision**: Add a dedicated `daemon/internal/mcp` package with a registry in
`daemon/internal/mcp/manager.go` and an explicit stdio transport component in
`daemon/internal/mcp/transport.go` (for handshake, tool discovery, health tracking, and
cancellation-aware session handling). Keep sandbox policy evaluation in
`daemon/internal/sandbox`, wire startup and recovery through `daemon/internal/app`,
expose resources and lifecycle actions in `daemon/internal/api`, persist registry and
lifecycle state in `daemon/internal/store`, and update schema-backed contracts under
`schemas/` plus operator docs under `docs/harness` and `docs/runtime`.

## Complexity Tracking

No constitution violations remain after the roadmap boundary is kept to Roadmap 18.
Generic tool orchestration, generic executable-skill routing, and stronger sandbox
backends remain explicit follow-on work rather than partial scope hidden inside this plan.
