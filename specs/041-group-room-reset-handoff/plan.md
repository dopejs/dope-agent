# Implementation Plan: Group Room Reset Handoff

**Branch**: `041-group-room-reset-handoff` | **Date**: 2026-05-11 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/041-group-room-reset-handoff/spec.md`

## Summary

Close Roadmap 56 by adding explicit group, room, reset, and cross-surface handoff
semantics on top of Roadmap 54 daemon-owned thread lifecycle and Roadmap 55 bounded
recent-turn continuity. The implementation makes conversation shape first-class product
state, applies default group/room participation policy requiring both allowlist
eligibility and a qualifying mention, records reset and participation decisions as
operator-visible evidence, and creates separate destination threads for handoff linked to
source threads by auditable handoff records.

The design is additive and reversible. Handoff never merges source and destination thread
identity, never copies source turns into destination history, and may reference eligible
current-segment source turns only for the first destination response after an authorized
handoff. Reset and handoff creation both use the existing `connectors.manage` lifecycle
mutation boundary. Unsupported connectors remain compatible by producing safe
unsupported evidence rather than receiving implicit group or handoff behavior.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; TypeScript 5.7 SDK and client
tests; React 19/Vite 7 web surface; JSON Schema contracts under `schemas/`; markdown
runtime/channel/operator docs.
**Primary Dependencies**: Existing `daemon/internal/threads`,
`daemon/internal/store`, `daemon/internal/store/tenancy`, `daemon/internal/api`,
`daemon/internal/chat`, `daemon/internal/events`, `daemon/internal/connectors`,
`daemon/internal/im`, `daemon/internal/contracts`, `schemas/api`, `schemas/events`,
`sdk/ts/src`, `web/src`, `tui/src`, Roadmap 54 lifecycle surfaces, Roadmap 55
continuity surfaces, and channel connector conformance docs.
**Storage**: Existing SQLite daemon store remains authoritative. Additive persistence is
required for conversation shape/source-room metadata, participation policy projections,
participation decisions, reset events scoped by conversation shape/source, handoff links,
handoff source-turn reference eligibility, permission/redaction/retention metadata, and
restart recovery. Existing thread, session segment, source linkage, lifecycle action,
continuity turn, continuity preview, connector message, run, workflow, approval, and
delivery records remain compatible and are not destructively rewritten.
**Testing**: Targeted Go tests under `daemon/internal/threads`, `daemon/internal/store`,
`daemon/internal/store/tenancy`, `daemon/internal/api`, `daemon/internal/chat`,
`daemon/internal/events`, `daemon/internal/connectors`, `daemon/internal/im`, and
`daemon/internal/contracts`; schema/fixture validation via `make daemon-contract-test`;
SDK/Web/TUI coverage via `pnpm test:clients`; client build via `pnpm build`; full daemon
coverage via `go test ./...` from `daemon/`; `go mod tidy` from `daemon/` after
implementation.
**Target Platform**: Local-first daemon and hosted daemon behavior, with API, TypeScript
SDK, Web, TUI/operator shell, connector ingress, and supported channel handoff behavior.
Default local verification uses `~/.kura-test` and `127.0.0.1:19192`.
**Project Type**: Multi-surface daemon product feature spanning API, persistence,
contracts, thread lifecycle domain policy, chat handoff context bridge, connector
ingress/conformance, SDK, Web, TUI/operator shell, events/audit, retention, redaction,
restart recovery, and docs.
**Performance Goals**: Authorized operators can determine why a representative group
message participated, did not participate, reset, or handed off within 5 minutes using
product evidence. Handoff source-turn reference assembly for the first destination
response must remain within Roadmap 55 default continuity assembly target of p95 under
500 ms in the verification environment.
**Constraints**: Default group/room participation requires both allowlist eligibility and
a qualifying mention. Reset and handoff creation require `connectors.manage`. Inspection
requires Roadmap 54 thread detail/inspection permission. Handoff creates or selects a
separate destination thread; source-turn references are eligible only for the first
destination response; source turns are never copied into destination history. The feature
must not add group memory, team knowledge base behavior, semantic retrieval, summaries,
autonomous delegation, long-term personalization, cross-room recall, or raw provider
payload display.
**Scale/Scope**: One whole roadmap slice, Phase 56. Required surfaces are store, thread
domain model, API, schema, events, SDK, Web, TUI/operator shell, connector conformance,
supported connector ingress, chat handoff context bridge, restart recovery,
retention/redaction, docs, and tests proving group/room isolation, reset scoping,
handoff traceability, and non-memory guarantees.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** - PASS. This plan closes Roadmap 56 as a complete group/room/reset
  and handoff semantics slice: conversation shape, participation policy, reset scoping,
  separate-thread handoff links, first-response source-turn references, operator
  evidence, connector conformance, contracts, docs, and verification.
- **Production-grade, minimal, reversible change** - PASS. The design extends Roadmap 54
  thread lifecycle and Roadmap 55 continuity additively. Rollback disables new
  participation, reset, and handoff entry points while preserving existing chat,
  connector, thread lifecycle, and already-recorded metadata evidence.
- **Contracts and auditability** - PASS. API/schema/event/SDK/operator behavior is
  captured in [contracts/group-room-reset-handoff.md](./contracts/group-room-reset-handoff.md).
  Participation, reset, and handoff decisions record stable reason classifications and
  permission outcomes.
- **Verification and observability** - PASS. Verification covers conversation shape,
  allowlist-plus-mention policy, direct/group/room reset, source-specific reset scoping,
  channel-to-web and web-to-channel handoff, denied handoff, duplicate/replay handling,
  restart recovery, redaction, retention, connector conformance, and non-memory scope.
- **Environment and secrets** - PASS. Local work defaults to `~/.kura-test` with fake or
  seeded connector/runtime evidence. Live connectors and production tenants are not
  required. Evidence must not expose secrets, raw provider payloads, disallowed message
  bodies, unsafe connector metadata, or cross-tenant identifiers.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/041-group-room-reset-handoff/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── group-room-reset-handoff.md
├── checklists/
│   └── requirements.md
└── tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── threads/
│   │   ├── group_room.go              # conversation shape, room identity, policy types
│   │   ├── group_room_test.go
│   │   ├── handoff.go                 # handoff domain policy and source references
│   │   ├── handoff_test.go
│   │   ├── lifecycle.go               # reset scope reuse and shape-aware reset hooks
│   │   ├── projection.go              # thread detail gains participation/handoff evidence
│   │   └── redaction.go               # safe summaries for group/handoff evidence
│   ├── store/
│   │   ├── thread_group_room.go       # SQLite policy/decision/source-shape persistence
│   │   ├── thread_handoff.go          # SQLite handoff links and source refs
│   │   ├── thread_group_room_test.go
│   │   ├── thread_handoff_test.go
│   │   ├── thread_handoff_restart_test.go
│   │   └── store.go                   # additive schema migration v52
│   ├── store/tenancy/
│   │   ├── threads.go                 # tenant-safe policy/handoff/reset reads/mutations
│   │   └── threads_test.go
│   ├── api/
│   │   ├── thread_lifecycle.go        # reset/detail additive evidence reuse
│   │   ├── thread_handoff.go          # handoff routes and detail projection
│   │   ├── thread_group_room_test.go
│   │   └── thread_handoff_test.go
│   ├── chat/
│   │   ├── service.go                 # first-response handoff source reference bridge
│   │   └── handoff_context_test.go
│   ├── events/
│   │   ├── thread_group_room.go       # participation/reset/handoff event constructors
│   │   └── thread_group_room_test.go
│   ├── im/
│   │   ├── loop.go                    # connector ingress applies conversation shape/policy
│   │   └── loop_test.go
│   ├── connectors/
│   │   ├── conformance.go             # group/room/handoff conformance capability fields
│   │   └── *_test.go                  # connector regressions for mention/allowlist/dedupe
│   └── contracts/
│       └── group_room_reset_handoff_contracts_test.go

schemas/
├── api/
│   ├── thread-detail.response.schema.json
│   ├── thread-conversation-shape.schema.json
│   ├── thread-participation-decision.schema.json
│   ├── thread-handoff-link.schema.json
│   ├── thread-handoff.request.schema.json
│   └── thread-handoff.response.schema.json
└── events/
    ├── thread-participation-decision.event.schema.json
    ├── thread-reset-scoped.event.schema.json
    └── thread-handoff-linked.event.schema.json

sdk/ts/src/
├── index.ts
└── group-room-reset-handoff.test.ts

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

**Structure Decision**: Implement group, room, reset, and handoff semantics as an
extension of daemon-owned thread lifecycle and bounded continuity, not as a memory or
knowledge subsystem. Keep conversation-shape and handoff policy in
`daemon/internal/threads`, store persistence in `daemon/internal/store`, connector
ingress decisions in `daemon/internal/im` and `daemon/internal/connectors`, and product
inspection through additive thread detail/handoff contracts.

## Roadmap 56 Planning Contracts

The implementation plan MUST keep this artifact complete before `/speckit.tasks`:

- [contracts/group-room-reset-handoff.md](./contracts/group-room-reset-handoff.md)
  - Conversation shape values, group/room participation policy, participation decision
    evidence, reset scoping, handoff route, separate destination thread identity,
    first-response source-turn reference rules, thread detail projection, event evidence,
    connector conformance additions, SDK/Web/TUI expectations, permission gates,
    redaction, retention, compatibility, migration, and rollback.

This artifact is a planning gate. Implementation is incomplete if group or handoff
behavior can affect routing, reset scope, destination context, or operator inspection
without a contract row and proving test.

## Migration And Rollback Plan

1. Add additive schema migration v52 with tables or columns for conversation shape,
   room identity metadata, participation decisions, participation policy projections,
   scoped reset evidence where Roadmap 54 lifecycle actions need shape/source evidence,
   handoff links, and handoff source-turn reference records. Include tenant, thread,
   session segment, connector, source account, source conversation, conversation shape,
   reason code, redaction status, retention expiry, and `document_json`.
2. Preserve existing Roadmap 54 `threads`, `thread_session_segments`,
   `thread_source_links`, lifecycle action, and runtime projection records. Preserve
   Roadmap 55 continuity turn and preview records. Do not rewrite existing source or
   continuity history destructively.
3. Backfill is metadata-only and conservative: legacy or partial conversations may be
   labeled `unknown` or `unsupported` for conversation shape and remain inspectable where
   allowed, but they must not receive implicit group/room participation or handoff
   semantics until source identity and redaction eligibility are provable.
4. Roll out in stages: record conversation shape and participation decisions in
   read-only/shadow mode, enable default allowlist-plus-mention policy for verified
   connector sources, enable source-specific reset evidence, then enable handoff routes
   for verified source/destination pairs.
5. On rollback, disable group participation acceptance, handoff creation, and additive
   client entry points while preserving already-recorded metadata evidence until
   retention expiry. Existing direct-message chat, single-turn behavior, Roadmap 54
   lifecycle mutations, Roadmap 55 continuity, and connector ingress remain compatible.
6. Irreversible behavior is limited to recording metadata-only evidence rows. No
   destructive backfill, source-history merge, thread-identity merge, or copy of source
   turns into destination history is allowed.

## Post-Design Constitution Re-check

- **Roadmap closure** - PASS. `research.md`, `data-model.md`, contract, and quickstart
  cover the full Roadmap 56 surface across store, thread domain policy, API, SDK, Web,
  TUI/operator shell, connector ingress/conformance, reset, handoff, restart recovery,
  retention, redaction, and verification.
- **Production-grade, minimal, reversible change** - PASS. Design artifacts preserve
  existing thread lifecycle, continuity, direct-message, web, and connector behavior and
  add group/room/handoff semantics only when source support and permissions are
  provable.
- **Contracts and auditability** - PASS. The contract defines route shapes, thread detail
  projection, event evidence, permission gates, source identity, first-response handoff
  references, redaction, retention, compatibility, migration, and rollback.
- **Verification and observability** - PASS. The quickstart and contract identify
  targeted Go, contract, SDK, Web, TUI, restart, redaction, retention, and
  non-memory-scope checks.
- **Environment and secrets** - PASS. Verification uses test-environment seeded/fake
  evidence by default, with no production tenants or live connector credentials required.

No post-design violations require justification.

## Complexity Tracking

No constitution violations or complexity exceptions.
