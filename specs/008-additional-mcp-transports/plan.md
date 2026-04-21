# Implementation Plan: Additional MCP Transports

**Branch**: `009-additional-mcp-transports` | **Date**: 2026-04-20 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/008-additional-mcp-transports/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Extend the existing daemon-owned MCP manager with a third transport family, `websocket`,
without creating a transport-specific control path. The design adds explicit transport
capability inspection, additive `websocket` transport configuration and secret-backed auth
contracts, bounded reconnect and restore truth, and end-to-end verification on a real
`websocket` MCP server running in `DOPE_ENV=test`.

## Technical Context

**Language/Version**: Go 1.24.0; Markdown docs; JSON Schema contracts  
**Primary Dependencies**: `daemon/internal/mcp`, `daemon/internal/api`, `daemon/internal/app`, `daemon/internal/contracts`, `daemon/internal/events`, `daemon/internal/runtime`, `daemon/internal/store`, `modernc.org/sqlite`, `github.com/gorilla/websocket`  
**Storage**: SQLite daemon state for MCP server documents, lifecycle state, tool exposure, tool-call history, and event history; additive transport capability and recovery truth persisted inside existing MCP server documents and event records; environment-scoped config and secret material under `~/.dope-test` or `~/.dope`  
**Testing**: `go test ./internal/mcp ./internal/api ./internal/app ./internal/store ./internal/contracts`, `make daemon-contract-test`, `go test ./...`, plus one manual `DOPE_ENV=test` websocket verification workflow  
**Target Platform**: macOS/Linux local daemon in `DOPE_ENV=test` by default, with a repo-owned websocket MCP helper server reachable over localhost for deterministic verification  
**Project Type**: Go daemon and harness control-plane service with schema-backed HTTP and event contracts  
**Performance Goals**: transport capability inspection plus websocket start and reconnect preflight add no more than `<=100 ms` daemon-side overhead per request, excluding actual websocket dial time, remote server latency, and reconnect wait windows  
**Constraints**: MCP transport expansion MUST stay on the existing daemon-owned registry, lifecycle, authorization, and runtime tool-call plane; `websocket` auth MUST use MCP secret refs and redacted operator-visible projection, not inline secrets; reconnect MUST be daemon-managed and bounded; readiness, mismatch, and recovery truth MUST stay environment-scoped; existing `stdio` and `streamable-http` behavior MUST remain backward compatible and additive  
**Scale/Scope**: one operator-managed daemon with single digits to low dozens of MCP servers, more than one supported transport family, and bounded reconnect history for long-lived websocket sessions

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan closes Roadmap 23 only: transport capability truth, one additional transport family, bounded reconnect and restore semantics, additive contracts, docs, and verification. Catalog management, additional future transports beyond `websocket`, and orchestration remain in later roadmaps.
- Production-grade change control: PASS. The design extends the existing MCP manager, transport mux, server resource model, config projection, tool-call provenance, and event history rather than introducing a second transport-specific manager or standalone invoke API. Rollback is a single change-set revert of websocket transport support and additive inspection or recovery fields while preserving current stdio and streamable-http behavior.
- Contracts and auditability: PASS. The plan identifies the API routes, schemas, event families, persistence fields, docs, and verification artifacts that must change together so transport capability, websocket auth, and reconnect truth remain operator-visible and restart-safe.
- Verification and observability: PASS. The plan requires targeted daemon tests, contract coverage, reconnect and restore regressions, and a manual `DOPE_ENV=test` websocket workflow. Recovery and mismatch truth are explicit outputs, not hidden implementation details.
- Environment and secrets: PASS. Planning assumes `DOPE_ENV=test` and localhost verification. Websocket auth stays bound to existing MCP secret refs with redacted projection and no process-environment-only secret path.

Post-design re-check:

- PASS. The design remains roadmap-closed: one new transport family, explicit capability truth, recovery semantics, docs, and end-to-end verification are all inside the same slice.
- PASS. The plan preserves one daemon-owned MCP control plane by implementing websocket inside the current transport mux and manager lifecycle rather than adding a second transport service.
- PASS. Contracts remain additive and auditable across config, MCP server resources, tool-call projection, schemas, events, and docs. No hidden or transport-specific operator shortcut is introduced.

## Project Structure

### Documentation (this feature)

```text
specs/008-additional-mcp-transports/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── additional-mcp-transport-surfaces.md
└── tasks.md
```

### Source Code (repository root)

```text
daemon/
├── cmd/
│   └── mcp-websocket-helper/
├── internal/
│   ├── api/
│   ├── app/
│   ├── contracts/
│   ├── events/
│   ├── mcp/
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

**Structure Decision**: Keep all phase 23 behavior inside the existing MCP subsystem. `daemon/internal/mcp` owns transport capability records, websocket session handling, reconnect policy, and restore behavior; `daemon/internal/api` exposes additive capability inspection and websocket-compatible server routes; `daemon/internal/store` and `daemon/internal/events` continue to persist server documents and transport recovery truth; `schemas/` extends existing MCP request, resource, tool-call, and event contracts; `daemon/cmd/mcp-websocket-helper` provides a deterministic real server for manual and targeted verification; `docs/` explain transport selection, host prerequisites, and recovery behavior; `AGENTS.md` points at this plan for later task generation and implementation.

## Complexity Tracking

No constitution violations remain. The plan avoids a second transport registry, avoids inline websocket credentials, and keeps reconnect policy bounded and explicit instead of creating an unbounded background recovery subsystem.
