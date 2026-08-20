# Implementation Plan: Non-Knowledge Multi-Turn Continuity

**Branch**: `040-multi-turn-continuity` | **Date**: 2026-05-11 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/040-multi-turn-continuity/spec.md`

## Summary

Close Roadmap 55 by adding bounded, inspectable recent-turn continuity on top of the
Roadmap 54 daemon-owned thread/session lifecycle. The implementation persists explicit
user and assistant turns for active thread segments, assembles at most the 12 most recent
eligible prior turns within the 30-day active-continuity window, records deterministic
inclusion/exclusion evidence for each response, and exposes operator-visible continuity
previews without adding memory recall, semantic retrieval, summaries, knowledge graph
behavior, context packing, or long-term personalization.

The approach extends the existing `daemon/internal/threads` domain, SQLite thread
lifecycle store, chat query routes, connector ingress path, API schemas, SDK, Web, TUI,
events, and contract tests. It is additive and reversible: disabling continuity
assembly restores existing single-turn behavior while retaining already-written
continuity evidence for authorized inspection until retention expiry.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; TypeScript 5.7 SDK and client
tests; React 19/Vite 7 web surface; JSON Schema contracts under `schemas/`; markdown
runtime/channel/operator docs.
**Primary Dependencies**: Existing `daemon/internal/chat`, `daemon/internal/threads`,
`daemon/internal/store`, `daemon/internal/store/tenancy`, `daemon/internal/api`,
`daemon/internal/events`, `daemon/internal/connectors`, `daemon/internal/im`,
`daemon/internal/llm`, `schemas/api`, `schemas/events`, `sdk/ts/src`, `web/src`,
`tui/src`, and Roadmap 54 thread lifecycle surfaces.
**Storage**: Existing SQLite daemon store remains authoritative. Additive persistence is
required for continuity turns, preview/decision evidence, preview items, daemon
acceptance sequence, safe artifact excerpt metadata, redaction status, retention expiry,
and restart recovery. Safe artifact excerpts are persisted as redacted value objects in
`thread_continuity_turns.document_json` and as `artifact_excerpt` preview item evidence
in `thread_continuity_preview_items`; v51 does not introduce a separate artifact excerpt
table. Existing `threads`, `thread_session_segments`, `thread_source_links`,
`thread_runtime_projections`, `sessions`, `llm_dispatches`, connector messages, runs,
workflows, approvals, and delivery records remain compatible and are not destructively
rewritten.
**Testing**: Targeted Go tests under `daemon/internal/threads`, `daemon/internal/chat`,
`daemon/internal/api`, `daemon/internal/store`, `daemon/internal/store/tenancy`,
`daemon/internal/events`, `daemon/internal/connectors`, `daemon/internal/im`, and
`daemon/internal/contracts`; schema/fixture validation via `make daemon-contract-test`;
SDK/Web/TUI coverage via `pnpm test:clients`; client build via `pnpm build`; full daemon
coverage via `go test ./...` from `daemon/`; `go mod tidy` from `daemon/` after
implementation.
**Target Platform**: Local-first daemon and hosted daemon behavior, with API, TypeScript
SDK, Web, TUI/operator shell, and connector ingress support. Default local verification
uses `~/.kura-test` and `127.0.0.1:19192`.
**Project Type**: Multi-surface daemon product feature spanning API, persistence,
contracts, chat dispatch input assembly, connector ingress, SDK, Web, TUI/operator shell,
events/audit, retention, redaction, restart recovery, and docs.
**Performance Goals**: Continuity assembly for the default window completes under
500 ms p95 in the verification environment. Authorized operators can explain why a
representative response did or did not use continuity within 5 minutes using product
evidence.
**Constraints**: Include no more than 12 eligible prior turns, exclude turns older than
30 active-continuity days unless authorized tenant policy changes the active window, use
daemon acceptance sequence for ordering, require `credentials.inspect` for preview
inspection, require `connectors.manage` for thread-level reset, preserve Roadmap 54
90-day default inspection retention, suppress unsafe evidence, and never use memory,
semantic retrieval, summaries, knowledge graph behavior, autonomous context packing,
provider-retained context, client-local history, or cross-thread personalization.
**Scale/Scope**: One whole roadmap slice, Phase 55. Required surfaces are store, domain
model, chat API, streaming API, SDK, Web, TUI/operator shell, connector ingress, runtime
artifact references, restart recovery, retention/redaction, docs, schema/event contracts,
and tests proving bounded behavior.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** - PASS. This plan closes Roadmap 55 as a complete bounded
  non-knowledge continuity slice: persisted recent turns, deterministic inclusion policy,
  reset-aware behavior, operator-visible previews, chat and supported channel paths,
  restart-safe evidence, contracts, docs, and verification.
- **Production-grade, minimal, reversible change** - PASS. The design extends Roadmap 54
  thread lifecycle and existing chat/connector paths additively. Rollback disables
  continuity assembly and hides new preview entry points while preserving existing
  single-turn chat and already-recorded evidence.
- **Contracts and auditability** - PASS. API/schema/event/SDK/operator behavior is
  captured in [contracts/thread-continuity.md](./contracts/thread-continuity.md).
  Every response that evaluates continuity records included and excluded references with
  stable reason classifications.
- **Verification and observability** - PASS. Verification covers bounded inclusion,
  reset exclusion, artifact excerpt limits, source identity, duplicate/replay handling,
  daemon acceptance ordering, restart recovery, permission denial, redaction, retention,
  p95 latency, and non-memory guarantees.
- **Environment and secrets** - PASS. Local work defaults to `~/.kura-test` with fake or
  seeded connector/runtime evidence. Live connectors and production tenants are not
  required. Preview evidence must not expose secrets, raw provider payloads, disallowed
  message bodies, or cross-tenant identifiers.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/040-multi-turn-continuity/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── thread-continuity.md
├── checklists/
│   └── requirements.md
└── tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── threads/
│   │   ├── continuity.go              # continuity domain types and policy
│   │   ├── continuity_test.go
│   │   ├── lifecycle.go               # reset/archive/reopen boundary reuse
│   │   ├── projection.go              # thread detail gains continuity previews
│   │   └── redaction.go               # safe preview/excerpt helpers
│   ├── chat/
│   │   ├── service.go                 # thread-aware bounded dispatch assembly
│   │   └── service_test.go
│   ├── api/
│   │   ├── server.go                  # chat request/response additive fields
│   │   ├── thread_lifecycle.go        # preview route/detail projection
│   │   └── thread_continuity_test.go
│   ├── store/
│   │   ├── thread_continuity.go       # SQLite turn/preview persistence
│   │   ├── thread_continuity_test.go
│   │   ├── thread_continuity_restart_test.go
│   │   ├── perf_thread_continuity_test.go
│   │   └── store.go                   # additive schema migration v51
│   ├── store/tenancy/
│   │   ├── threads.go                 # tenant-safe continuity access
│   │   └── threads_test.go
│   ├── events/
│   │   ├── thread_continuity.go       # continuity evidence event constructors
│   │   └── thread_continuity_test.go
│   ├── im/
│   │   └── loop.go                    # connector-origin turns attach to thread truth
│   ├── connectors/
│   │   └── *_test.go                  # replay/source identity regressions
│   └── contracts/
│       └── thread_continuity_contracts_test.go

schemas/
├── api/
│   ├── chat-query.request.schema.json
│   ├── chat-query.response.schema.json
│   ├── chat-query-stream-started.event.schema.json
│   ├── thread-detail.response.schema.json
│   ├── thread-continuity-preview.schema.json
│   ├── thread-continuity-preview-item.schema.json
│   └── thread-continuity-preview.response.schema.json
└── events/
    ├── thread-continuity-turn-recorded.event.schema.json
    └── thread-continuity-preview-recorded.event.schema.json

sdk/ts/src/
├── index.ts
└── thread-continuity.test.ts

web/src/
├── features/thread-lifecycle.tsx
├── features/thread-lifecycle.test.tsx
├── app/App.tsx
└── app/App.test.tsx

tui/src/
├── cli.ts
└── cli.test.ts

docs/
├── runtime/thread-session-lifecycle.md
├── runtime/minimal-chat-clients.md
└── channels/channel-connector-conformance.md
```

**Structure Decision**: Implement continuity as an extension of daemon-owned thread
lifecycle, not as a memory subsystem. Keep domain policy in `daemon/internal/threads`,
store access in `daemon/internal/store`, assembly in `daemon/internal/chat`, and product
inspection through additive thread detail/preview contracts.

## Roadmap 55 Planning Contracts

The implementation plan MUST keep this artifact complete before `/speckit.tasks`:

- [contracts/thread-continuity.md](./contracts/thread-continuity.md)
  - Chat request/response continuity fields, stream behavior, thread detail preview
    exposure, dedicated preview inspection route, turn/preview schemas, event evidence,
    SDK/Web/TUI expectations, permission gates, source identity, reset boundaries,
    artifact excerpt limits, retention, redaction, compatibility, migration, and
    rollback.

This artifact is a planning gate. Implementation is incomplete if continuity can affect
assistant behavior without a preview record and contract coverage.

## Migration And Rollback Plan

1. Add schema migration v51 with additive tables for `thread_continuity_turns`,
   `thread_continuity_previews`, and `thread_continuity_preview_items`. Include tenant,
   thread, session segment, daemon acceptance sequence, role, source linkage, dispatch
   linkage, source timestamp, source message identity, artifact excerpt metadata,
   included/excluded status, exclusion reason, redaction status, retention expiry, and
   `document_json`. Artifact excerpts are embedded redacted metadata/value objects on
   continuity turns and preview items, not a standalone v51 table.
2. Allocate daemon acceptance sequence transactionally per tenant/thread, with a unique
   `(tenant_id, thread_id, acceptance_sequence)` constraint. Retain source timestamps as
   evidence only.
3. Keep old rows readable in mixed-version operation: existing chat requests without
   `threadId` remain single-turn; existing thread lifecycle list/detail remains valid if
   no continuity rows exist; connectors without valid thread identity do not infer
   continuity.
4. Roll out in stages: persist turns and previews in read-only/shadow mode, validate
   preview evidence and latency, then enable inclusion for verified chat and supported
   channel paths.
5. On rollback, disable continuity assembly and preview routes through configuration or
   route hiding while preserving additive tables and already-written evidence until
   retention expiry. Existing `/v1/chat/query`, `/v1/chat/query/stream`, `/v1/threads`,
   connector ingress, and Roadmap 54 lifecycle mutations continue to work.
6. Irreversible behavior is limited to recording metadata-only evidence rows. No
   destructive backfill or rewrite of existing thread/session/runtime data is required.

## Post-Design Constitution Re-check

- **Roadmap closure** - PASS. `research.md`, `data-model.md`, contract, and quickstart
  cover the full Roadmap 55 continuity surface across store, chat, API, SDK, Web, TUI,
  connector ingress, restart recovery, retention, redaction, latency, and verification.
- **Production-grade, minimal, reversible change** - PASS. Design artifacts preserve
  single-turn behavior and Roadmap 54 lifecycle semantics, add continuity only when a
  valid current thread segment exists, and define a staged rollback path.
- **Contracts and auditability** - PASS. The contract defines request/response shapes,
  preview inspection, event evidence, permission boundaries, source identity, ordering,
  redaction, retention, and fail-safe behavior.
- **Verification and observability** - PASS. The quickstart and contract identify
  targeted Go, contract, SDK, Web, TUI, restart, performance, and redaction checks.
- **Environment and secrets** - PASS. Verification uses test-environment seeded/fake
  evidence by default, with no production tenants or live connector credentials required.

No post-design violations require justification.

## Complexity Tracking

No constitution violations or complexity exceptions.
