# Implementation Plan: Daemon-Owned Thread And Session Lifecycle

**Branch**: `039-thread-session-lifecycle` | **Date**: 2026-05-11 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/039-thread-session-lifecycle/spec.md`

## Summary

Close Roadmap 54 by making thread and session lifecycle first-class daemon-owned product
truth. The work extends the existing session router, SQLite runtime store, connector
message records, runtime projections, event/audit stream, API schemas, TypeScript SDK,
operator shell, and web surfaces so authorized users can list, inspect, reset, archive,
and reopen conversations while operators can trace channel messages to sessions, runs,
workflows, approvals, replies, and deliveries.

The implementation is additive and migration-safe. Reset preserves thread identity and
creates a new active session segment. Archive blocks future continuation without
cancelling already accepted runtime work. Channel continuation is keyed by tenant,
connector, source account, and source conversation, with at most one current thread for
that key. The feature explicitly does not add memory recall, context packing, semantic
summaries, autonomous pruning, or memory-driven routing.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; TypeScript 5.7 SDK and web
client; React 19/Vite 7 web surface; JSON Schema contracts under `schemas/`; markdown
operator/runtime/channel docs.
**Primary Dependencies**: Existing `daemon/internal/router`, `daemon/internal/im`,
`daemon/internal/imtypes`, `daemon/internal/runtime`, `daemon/internal/store`,
`daemon/internal/store/tenancy`, `daemon/internal/api`, `daemon/internal/events`,
`daemon/internal/connectors`, `daemon/internal/delivery`, `daemon/internal/policy`,
`schemas/api`, `schemas/events`, `sdk/ts/src`, `web/src`, `tui/`, and runtime/channel
docs under `docs/runtime` and `docs/channels`.
**Storage**: Existing SQLite daemon store remains authoritative. Additive persistence is
required for thread resources, session segments, lifecycle transition audit, source
linkage, current-thread uniqueness by tenant/connector/source account/source
conversation, runtime projection references, redaction/retention metadata, and legacy
session projection state. Existing `sessions`, `runs`, connector message, workflow,
approval, delivery, and event tables remain readable and are not rewritten destructively.
**Testing**: Targeted Go tests under `daemon/internal/router`, `daemon/internal/im`,
`daemon/internal/api`, `daemon/internal/store`, `daemon/internal/store/tenancy`,
`daemon/internal/events`, `daemon/internal/connectors`, `daemon/internal/delivery`, and
`daemon/internal/contracts`; schema/fixture validation via `make daemon-contract-test`;
SDK/web/TUI client coverage via `pnpm test:clients`; client build via `pnpm build`; full
daemon coverage via `go test ./...` from `daemon/`; `go mod tidy` from `daemon/` after
implementation.
**Target Platform**: Local-first daemon and hosted daemon behavior, with API, TypeScript
SDK, web, TUI/operator shell, and connector ingress behavior. Default local verification
uses `~/.kura-test` and `127.0.0.1:19192`.
**Project Type**: Multi-surface daemon product feature spanning API, persistence,
contracts, connector ingress, runtime/session routing, SDK, web, TUI/operator shell,
events/audit, retention, redaction, and docs.
**Performance Goals**: Authorized users can find and open any recent tenant thread in
the verification dataset within 2 minutes. Authorized operators can trace a
representative channel incident from source message to thread, session, run or workflow,
approval, reply, and delivery facts within 5 minutes. Paginated thread lists return
deterministic, non-duplicated page sequences.
**Constraints**: Redacted inspection requires `credentials.inspect`; reset, archive, and
reopen require `connectors.manage`. Lifecycle mutations must fail closed if required
audit evidence cannot be recorded. Reset preserves thread ID and creates a new active
session segment. Archive blocks future continuation and does not cancel already accepted
runtime work. Lifecycle, source, and runtime projection evidence expires from normal
inspection after 90 days unless an authorized tenant policy requires longer retention.
The feature must not add memory recall, semantic summaries, context packing, autonomous
pruning, or memory-driven routing.
**Scale/Scope**: One whole roadmap slice, Phase 54. Required surfaces are store, API,
schema, events, SDK, operator shell/TUI, web, connector ingress, runtime projections,
restart recovery, migration/backfill support for legacy sessions, docs, and test
environment walkthrough.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** - PASS. This plan closes Roadmap 54 as a complete thread/session
  lifecycle productization slice: list/detail, reset/archive/reopen, current-thread
  source mapping, runtime projections, restart-safe state, API/SDK/web/operator-shell
  views, contract/schema/events, migration/backfill, retention/redaction, docs, and
  verification.
- **Production-grade, minimal, reversible change** - PASS. The design is additive over
  existing sessions, connector messages, runs, workflows, approvals, delivery outcomes,
  events, and channel management behavior. Rollback can hide new lifecycle actions and
  projections while preserving existing `/v1/sessions`, connector ingress, and runtime
  behavior.
- **Contracts and auditability** - PASS. Public API/schema/event/SDK/operator behavior
  is captured in [contracts/thread-session-lifecycle.md](./contracts/thread-session-lifecycle.md).
  Lifecycle mutations are tenant-scoped, permission-gated, auditable, and fail closed
  on required audit-write failure.
- **Verification and observability** - PASS. Verification covers API, SDK, web, TUI,
  store, router, connector ingress, source uniqueness, reset, archive, reopen,
  permission denials, audit fail-closed behavior, restart recovery, legacy projection,
  runtime evidence linkage, redaction, retention, and non-memory guarantees.
- **Environment and secrets** - PASS. Local work defaults to `~/.kura-test` with fake or
  seeded connector/runtime evidence. Live connector credentials and production tenants
  are not required for automated acceptance. Thread support evidence is metadata-only
  and must not display secrets, raw provider payloads, or disallowed message bodies.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/039-thread-session-lifecycle/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── thread-session-lifecycle.md
├── checklists/
│   └── requirements.md
└── tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── threads/
│   │   ├── lifecycle.go           # thread/session lifecycle state and transitions
│   │   ├── projection.go          # list/detail/runtime evidence projection
│   │   ├── source.go              # source conversation identity and current-thread key
│   │   ├── redaction.go           # metadata-only evidence redaction helpers
│   │   └── *_test.go              # lifecycle, projection, source, redaction unit tests
│   ├── router/
│   │   ├── router.go              # session routing integration with thread/session segments
│   │   └── router_test.go         # reset segment and routing compatibility coverage
│   ├── im/
│   │   ├── loop.go                # connector ingress attaches messages to thread truth
│   │   └── loop_test.go           # source continuation and archived-thread ingress tests
│   ├── imtypes/
│   │   └── messages.go            # source linkage fields if current message record is insufficient
│   ├── api/
│   │   ├── thread_lifecycle.go    # thread list/detail/reset/archive/reopen routes
│   │   ├── server.go              # route registration and protected tenant guards
│   │   └── thread_lifecycle_test.go
│   ├── store/
│   │   ├── thread_lifecycle.go    # SQLite persistence/accessors/retention
│   │   ├── thread_lifecycle_test.go
│   │   ├── thread_lifecycle_restart_test.go
│   │   └── migrationfixture/      # fixture proving legacy session projection
│   ├── store/tenancy/
│   │   ├── threads.go             # tenant-safe thread/session lifecycle access
│   │   └── threads_test.go
│   ├── events/
│   │   ├── thread_lifecycle.go    # lifecycle/audit/redaction/retention event constructors
│   │   └── thread_lifecycle_test.go
│   ├── connectors/
│   │   └── *_test.go              # connector regression for daemon-owned thread truth
│   ├── delivery/
│   │   └── *_test.go              # delivery projection linkage from thread detail
│   └── contracts/
│       └── thread_lifecycle_contracts_test.go

schemas/
├── api/
│   ├── thread-resource.schema.json
│   ├── thread-list.response.schema.json
│   ├── thread-detail.response.schema.json
│   ├── thread-lifecycle-action.request.schema.json
│   ├── thread-lifecycle-action.response.schema.json
│   ├── thread-source-linkage.schema.json
│   └── thread-runtime-projection.schema.json
└── events/
    ├── thread-lifecycle.event.schema.json
    ├── thread-source-linked.event.schema.json
    └── thread-retention-applied.event.schema.json

sdk/ts/src/
├── index.ts                       # thread lifecycle types and client methods
└── thread-lifecycle.test.ts       # SDK endpoint/typing/tenant-header coverage

web/src/
├── features/thread-lifecycle.tsx
├── features/thread-lifecycle.test.tsx
├── app/App.tsx                    # product surface integration
└── app/App.test.tsx

tui/
├── src/cli.ts                    # operator shell commands and output formatting
├── src/cli.test.ts               # TUI list/detail/action tests
└── README.md                     # operator usage notes when commands change

docs/
├── runtime/thread-session-lifecycle.md
└── channels/channel-connector-conformance.md
```

**Structure Decision**: Implement Roadmap 54 as a daemon-owned thread lifecycle layer
that composes existing session routing, connector message evidence, runtime records, and
delivery/workflow/approval projections. Use a new focused `daemon/internal/threads`
package for lifecycle domain behavior so router, API, store, connector ingress, SDK,
and UI surfaces do not each reinvent state transitions. Keep existing `/v1/sessions`
compatible and expose richer lifecycle behavior through additive thread resources.

## Roadmap 54 Planning Contracts

The implementation plan MUST keep this artifact complete before `/speckit.tasks`:

- [contracts/thread-session-lifecycle.md](./contracts/thread-session-lifecycle.md)
  - Tenant-scoped thread list/detail resources, pagination and ordering, reset/archive/
    reopen permission gates, current-thread source identity, runtime projection links,
    legacy session projection, retention, redaction, events, SDK/web/TUI expectations,
    compatibility, migration, and rollback.

This artifact is a planning gate. Implementation is incomplete if thread/session
lifecycle can list, mutate, route connector messages, expose runtime projections, or
emit lifecycle events without a contract row and proving test.

## Migration And Rollback Plan

1. Add additive SQLite tables or columns for `threads`, `thread_session_segments`,
   `thread_source_links`, `thread_lifecycle_events`, and `thread_runtime_projections`.
   Include tenant ID, current-thread uniqueness by tenant/connector/source account/
   source conversation, retention expiry, redaction status, and audit references.
2. Keep existing `sessions`, `runs`, connector messages, workflow, approval, delivery,
   and event records intact. Project legacy sessions into thread lifecycle evidence
   lazily or through a bounded backfill that labels incomplete linkage as partial.
3. Add tenant-safe store accessors under `daemon/internal/store/tenancy` before exposing
   API reads or mutations. Cross-tenant rows must remain indistinguishable from missing
   resources and emit existing tenant-denial audit behavior where appropriate.
4. Add lifecycle mutation flow that records required audit evidence before committing
   reset, archive, or reopen state changes. If audit recording fails, leave thread state
   unchanged and return a stable failure.
5. Integrate connector ingress with source-link resolution so accepted inbound messages
   attach to the current thread for tenant/connector/source account/source conversation.
   Archived threads block continuation; duplicate and rejected messages retain routing
   evidence without creating misleading assistant work.
6. Add runtime projection linkage from thread detail to sessions, runs, workflows,
   approvals, foreground replies, and background deliveries using metadata-only
   summaries and existing authoritative records.
7. Expose API schemas/routes, SDK methods/types, web view, and operator shell/TUI
   commands after store and lifecycle behavior are covered by tests.
8. Add retention application for lifecycle/source/runtime projection evidence with the
   90-day default and redaction-failure events for unsafe evidence.
9. Update docs and quickstart for test-environment lifecycle inspection and channel
   tracing over at least two connector kinds.

Rollback hides or disables the new thread lifecycle routes, SDK/web/TUI entry points,
and lifecycle mutations while retaining additive tables and already-written lifecycle
evidence until retention expiry. Connector ingress may fall back to prior compatible
session routing only if doing so does not rewrite existing thread evidence. Existing
`/v1/sessions`, chat, connector runtime, runs, workflows, approvals, and delivery
behavior must continue to work without the new product views.

## Post-Design Constitution Re-check

- **Roadmap closure** - PASS. `research.md`, `data-model.md`, contract, and quickstart
  cover the full Roadmap 54 lifecycle surface across store, API, SDK, web, TUI/operator
  shell, connector ingress, runtime projections, retention, redaction, restart recovery,
  and verification.
- **Production-grade, minimal, reversible change** - PASS. Design artifacts preserve
  existing session and connector runtime behavior and add thread lifecycle state,
  evidence, and controls without replacing provider runtimes or introducing memory.
- **Contracts and auditability** - PASS. The contract defines API/SDK/web/TUI behavior,
  event/audit expectations, permission gates, current-thread uniqueness, retention,
  redaction, and fail-closed lifecycle mutation behavior.
- **Verification and observability** - PASS. The quickstart and contract identify
  daemon, contract, SDK, web, TUI, restart, retention, redaction, and manual walkthrough
  verification.
- **Environment and secrets** - PASS. The quickstart defaults to the test daemon and
  metadata-only evidence; live credentials and production tenants remain out of default
  verification.

## Complexity Tracking

No constitution violations require justification.
