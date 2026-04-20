# Implementation Plan: Complete MCP Runtime And Catalog

**Branch**: `006-mcp-runtime-and-catalog` | **Date**: 2026-04-20 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/006-mcp-runtime-and-catalog/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Recut the remaining post-Roadmap-18 MCP work into one roadmap-closed slice that makes MCP complete from the daemon product surface: invoke MCP tools through the existing runtime tool-call plane, add `streamable-http` as the first remote transport beside `stdio`, ship an installable starter catalog, and provide both daemon API and repo-script install paths that converge on the same installed MCP server resource and audit truth.

## Technical Context

**Language/Version**: Go 1.24.0; Bash for repo install scripts; Markdown docs; JSON Schema contracts  
**Primary Dependencies**: `daemon/internal/api`, `daemon/internal/app`, `daemon/internal/contracts`, `daemon/internal/events`, `daemon/internal/mcp`, `daemon/internal/runtime`, `daemon/internal/sandbox`, `daemon/internal/store`, `modernc.org/sqlite`  
**Storage**: SQLite daemon state for MCP server registry, lifecycle, install provenance, and tool-call history; environment-scoped config and secret material under `~/.dope-test` or `~/.dope`; committed starter catalog definitions and repo install helpers in the repository  
**Testing**: `go test ./internal/mcp ./internal/api ./internal/runtime ./internal/app ./internal/store ./internal/contracts`, `make daemon-contract-test`, `go test ./...`  
**Target Platform**: macOS/Linux local daemon in `DOPE_ENV=test`, with remote MCP access over `streamable-http` when configured  
**Project Type**: Go daemon and harness control-plane service with schema-backed HTTP and event contracts plus repo operator scripts  
**Performance Goals**: Catalog list/inspect, install preflight, and MCP tool invocation preflight add no more than `<=100 ms` daemon-side overhead per request, excluding remote MCP round-trip latency, subprocess startup, external package manager time, and remote service readiness  
**Constraints**: MCP tool invocation MUST reuse `/v1/runs/.../tool-calls` as the canonical execution plane; no second standalone MCP invoke plane; first remote transport is `streamable-http`; both daemon API install and repo script install MUST converge on the same installed resource shape; bundled entries MUST report truthful unavailable or blocked reasons for missing binaries, endpoints, credentials, or host prerequisites; secret values and secret-derived material remain redacted; all changes remain additive to existing MCP registry and sandbox surfaces; local verification defaults to `DOPE_ENV=test`  
**Scale/Scope**: One daemon-managed MCP inventory for a small operator-managed fleet (single digits to low dozens of installed servers, hundreds of exposed tools, and a curated starter catalog with immediate-use plus template entries)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan explicitly recuts the remaining MCP product-surface work after Roadmap 18 into one closed slice: runtime invocation, remote transport, starter catalog, install flows, operator docs, and verification. It does not absorb broader plugin marketplace or non-MCP connector work.
- Production-grade change control: PASS. The design extends the existing daemon-managed MCP subsystem instead of creating a second control plane. Rollback is a single change-set revert of MCP invocation, catalog, and transport additions while preserving the already-shipped MCP registry and sandbox substrate.
- Contracts and auditability: PASS. The plan names the API, schema, event, script, persistence, and docs surfaces that must change together so catalog install, remote transport, and invocation truth remain explicit.
- Verification and observability: PASS. The plan requires targeted package tests, contract coverage, unavailable-path verification, at least one real end-to-end installed-catalog invocation in `DOPE_ENV=test`, and operator-visible audit trails for install and invocation.
- Environment and secrets: PASS. Local work remains in `DOPE_ENV=test`, starter entries are environment-scoped, and secret resolution and redaction stay daemon-owned with no raw secret projection into operator-visible routes.

Post-design re-check:

- PASS. Research and design keep the slice roadmap-closed by centering only MCP completion work that was intentionally deferred after Roadmap 18.
- PASS. The design keeps one daemon-owned execution story by reusing runtime tool calls for MCP invocation and one daemon-owned registry story for manual and catalog-installed servers.
- PASS. Contracts stay additive and auditable across API routes, schemas, events, docs, and repo install helpers; no hidden side path is introduced.

## Project Structure

### Documentation (this feature)

```text
specs/006-mcp-runtime-and-catalog/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── mcp-runtime-and-catalog-surfaces.md
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
│   ├── runtime/
│   ├── sandbox/
│   └── store/
└── go.mod

schemas/
├── api/
└── events/

scripts/
├── run-daemon.sh
└── install-mcp-catalog-entry.sh

docs/
├── harness/
└── runtime/

AGENTS.md
```

**Structure Decision**: Extend the existing `daemon/internal/mcp` subsystem instead of creating a new harness package. Keep catalog definitions, install orchestration, and remote transport handling in `daemon/internal/mcp`; expose catalog/install/invocation state through `daemon/internal/api`; persist install provenance and transport state in `daemon/internal/store`; reuse `daemon/internal/runtime` for MCP tool-call execution records; update schema-backed API and event contracts under `schemas/`; and add one repo-supported install helper under `scripts/` that writes through the same daemon-owned MCP server resource model.

## Complexity Tracking

No constitution violations remain after explicitly recutting the remaining MCP user-surface work into this closed slice. Marketplace-style discovery, connector ecosystems beyond the curated starter catalog, and additional MCP transports remain follow-on work rather than hidden scope.
