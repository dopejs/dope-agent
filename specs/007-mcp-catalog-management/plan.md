# Implementation Plan: MCP Catalog Management

**Branch**: `008-mcp-catalog-management` | **Date**: 2026-04-20 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/007-mcp-catalog-management/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Extend the existing daemon-owned MCP catalog and registry so installed catalog-managed
servers can be uninstalled, refreshed, reinstalled, and explicitly revalidated without
hand-editing MCP resources. The design keeps one MCP server resource model, persists
catalog install provenance and revision truth on that resource, and adds operator-visible
drift and revalidation results through additive API, schema, event, and history surfaces.

## Technical Context

**Language/Version**: Go 1.24.0; Bash for existing repo helper usage in verification; Markdown docs; JSON Schema contracts  
**Primary Dependencies**: `daemon/internal/api`, `daemon/internal/app`, `daemon/internal/contracts`, `daemon/internal/events`, `daemon/internal/mcp`, `daemon/internal/runtime`, `daemon/internal/store`, `modernc.org/sqlite`  
**Storage**: SQLite daemon state for MCP server documents, lifecycle state, tool exposure, and event history; additive catalog-management metadata persisted inside the existing `mcp_servers` document model; environment-scoped config and secret material under `~/.dope-test` or `~/.dope`  
**Testing**: `go test ./internal/mcp ./internal/api ./internal/app ./internal/store ./internal/contracts`, `make daemon-contract-test`, `go test ./...`  
**Target Platform**: macOS/Linux local daemon in `DOPE_ENV=test` by default, with MCP catalog entries spanning local stdio and remote `streamable-http` transports already introduced in Roadmap 21  
**Project Type**: Go daemon and harness control-plane service with schema-backed HTTP and event contracts plus repo operator utilities  
**Performance Goals**: Catalog-management inspection plus uninstall, refresh, reinstall, and revalidation preflight add no more than `<=100 ms` daemon-side overhead per request, excluding subprocess lifecycle latency, remote transport latency, and external service readiness  
**Constraints**: Catalog management MUST stay additive to the existing MCP registry; uninstall MUST remove the active resource instead of creating an inactive tombstone model; uninstall, refresh, and reinstall MUST return `conflict` or `busy` while lifecycle or tool invocation is active; refresh and reinstall MUST fail closed on `operatorModified` or equivalent conflicting state; revalidation is operator-triggered only in this phase; source, revision, drift, and revalidation truth MUST stay redacted and environment-scoped  
**Scale/Scope**: One daemon-managed MCP inventory for a small operator-managed environment (single digits to low dozens of installed servers, repeated maintenance actions on bundled catalog entries, and bounded event/history growth per action)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan closes Roadmap 22 only: managed lifecycle after MCP catalog install, source/version/drift truth, explicit revalidation, docs, and verification. It leaves marketplace distribution, additional transports, and orchestration in the already-recut later roadmaps.
- Production-grade change control: PASS. The design extends the existing MCP manager, API routes, store documents, and event model instead of introducing a second package-management subsystem or hidden action path. Rollback is a single change-set revert of catalog-management metadata and action routes while preserving Roadmap 21 install and invocation behavior.
- Contracts and auditability: PASS. The plan names the API routes, schemas, event families, persistence fields, docs, and verification artifacts that must change together so uninstall, refresh, reinstall, drift, and revalidation remain operator-visible.
- Verification and observability: PASS. The plan requires targeted daemon tests, contract coverage, event/history verification, and a manual `DOPE_ENV=test` maintenance walkthrough. Operator-visible action results, drift classification, and revalidation outcomes are part of the design, not optional follow-up.
- Environment and secrets: PASS. Local work remains in `DOPE_ENV=test`; no live environment is needed for planning; secret refs remain environment-scoped and operator-visible surfaces continue to redact secret values and supported derived forms.

Post-design re-check:

- PASS. The design keeps phase 22 roadmap-closed by covering maintenance, provenance, drift, and revalidation without absorbing future catalog distribution or transport expansion work.
- PASS. The plan preserves one MCP server resource model by persisting catalog-management metadata inside existing server documents instead of introducing tombstones or parallel install-state resources.
- PASS. Contracts remain additive and auditable across API routes, schemas, events, docs, and verification. No hidden shortcut bypasses the daemon-owned MCP control plane.

## Project Structure

### Documentation (this feature)

```text
specs/007-mcp-catalog-management/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── mcp-catalog-management-surfaces.md
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
│   └── store/
└── go.mod

schemas/
├── api/
└── events/

scripts/
└── install-mcp-catalog-entry.sh

docs/
├── harness/
└── runtime/

AGENTS.md
```

**Structure Decision**: Keep all phase 22 behavior inside the existing MCP subsystem. `daemon/internal/mcp` owns lifecycle actions, revision/drift computation, and revalidation; `daemon/internal/api` exposes additive action routes on installed MCP servers; `daemon/internal/store` persists metadata through the existing `mcp_servers` document model and event history; `schemas/` gains the additive response and event contracts; `docs/` explains lifecycle semantics and manual verification; `AGENTS.md` points at this plan for subsequent task generation and implementation.

## Complexity Tracking

No constitution violations remain. The plan avoids a second package-management subsystem, an inactive-resource registry model, background revalidation loops, and any new daemon plane outside the current MCP registry and event surfaces.
