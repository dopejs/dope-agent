# Implementation Plan: Agent Profile And Persona Configuration

**Branch**: `042-agent-profile-persona` | **Date**: 2026-05-12 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/042-agent-profile-persona/spec.md`

## Summary

Close Roadmap 57 by making agent identity, persona, provider defaults, safety defaults,
and editable overlays tenant-owned structured product state. The implementation adds
tenant-default active profile selection, durable profile versions, explicit overlay
references, profile lifecycle audit/events, and runtime profile projections on threads,
runs, workflows, and handoff destinations.

The design is additive and reversible. Existing local prompt/config behavior is bridged
into one default profile plus explicit overlay references where safe. Profile management
uses dedicated `profiles.inspect` and `profiles.manage` permissions. Phase 57 does not
introduce memory, learned preferences, channel/workspace/account binding, capability
binding, hard profile deletion, or agent-generated profile mutation.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; TypeScript 5.7 SDK and client
tests; React 19/Vite 7 web surface; JSON Schema contracts under `schemas/`; markdown
runtime/operator docs.
**Primary Dependencies**: Existing tenant identity and audit packages in
`daemon/internal/identity`; SQLite store and tenant-safe access patterns in
`daemon/internal/store` and `daemon/internal/store/tenancy`; thread lifecycle and runtime
projection models in `daemon/internal/threads`; chat/run/workflow startup paths in
`daemon/internal/api`, `daemon/internal/chat`, and runtime orchestration; provider manager
default model behavior in `daemon/internal/providers`; event bus in
`daemon/internal/events`; JSON schemas in `schemas/api` and `schemas/events`; TypeScript
SDK in `sdk/ts/src`; web shell in `web/src`; TUI/operator surface in `tui/src`.
**Storage**: Existing SQLite daemon store remains authoritative. Add schema migration v53
for profile records, profile versions, tenant-default active profile selection, overlay
references, profile audit/event documents, runtime profile projections, and compatibility
metadata for default profile seeding. No destructive rewrite of existing provider,
thread, session, run, workflow, handoff, or prompt/config evidence is allowed.
**Testing**: Targeted Go tests under `daemon/internal/identity`, `daemon/internal/profiles`
or equivalent domain package, `daemon/internal/store`, `daemon/internal/store/tenancy`,
`daemon/internal/api`, `daemon/internal/chat`, `daemon/internal/events`,
`daemon/internal/threads`, and `daemon/internal/contracts`; schema/fixture validation via
`make daemon-contract-test`; SDK/Web/TUI coverage via `pnpm test:clients`; client build
via `pnpm build`; full daemon coverage via `go test ./...` from `daemon/`; `go mod tidy`
from `daemon/` after implementation.
**Target Platform**: Local-first daemon and hosted daemon behavior, with API, TypeScript
SDK, Web, TUI/operator shell, runtime projection, and test-environment verification.
Default local verification uses `~/.kura-test` and `127.0.0.1:19192`.
**Project Type**: Multi-surface daemon product feature spanning API, persistence,
contracts, identity permissions, profile domain policy, provider default projection,
thread/session/run/workflow/handoff runtime evidence, SDK, Web, TUI/operator shell, events/audit,
migration, redaction, restart recovery, and docs.
**Performance Goals**: Authorized operators can identify which profile version influenced
a representative behavior change within 5 minutes using product evidence. Profile list,
detail, active selection, and runtime projection reads should fit existing operator-shell
expectations and must not add user-visible latency to starting new work beyond a single
tenant-scoped profile resolution.
**Constraints**: Profile reads/runtime inspection require `profiles.inspect`; profile
create/update/activation/archive/disable/overlay mutation/rollback require
`profiles.manage`. Each tenant has one tenant-default active profile in Phase 57.
Profile versions are retained while the profile exists, and rollback revalidates current
policy. Archive/disable is the only profile retirement behavior; hard delete is out of
scope while runtime evidence may reference the profile. Overlay files are explicit
references, not hidden primary truth. The feature must not add memory retrieval, learned
preferences, channel/workspace/account binding, capability binding, skill generation,
semantic knowledge retrieval, long-term personalization, or autonomous multi-agent
collaboration.
**Scale/Scope**: One whole roadmap slice, Phase 57. Required surfaces are store, profile
domain model, identity permissions, API, schema, events, SDK, Web, TUI/operator shell,
thread/session/run/workflow/handoff runtime profile projection, default profile migration,
overlay reference validation, restart recovery, redaction, docs, and tests proving CRUD,
tenant isolation, version retention, rollback, active profile linkage, historical
evidence preservation, and non-memory guarantees.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** - PASS. This plan closes Roadmap 57 as a complete structured agent
  profile/persona configuration slice: profile CRUD, tenant-default activation, version
  history, rollback, explicit overlays, provider/safety defaults, runtime projection,
  SDK/Web/TUI surfaces, contracts, docs, and verification.
- **Production-grade, minimal, reversible change** - PASS. The design is additive to
  tenant identity, provider defaults, and runtime evidence. Rollback disables profile
  editing and activation entry points while preserving already-recorded profile,
  version, audit, and runtime evidence for authorized inspection.
- **Contracts and auditability** - PASS. API/schema/event/SDK/operator behavior is
  captured in [contracts/agent-profile-persona.md](./contracts/agent-profile-persona.md).
  Profile changes, denials, validations, rollback, archive/disable, and runtime
  projection produce stable audit/event evidence.
- **Verification and observability** - PASS. Verification covers permissions, CRUD,
  tenant isolation, validation failures, version retention, rollback, archive/disable,
  active profile projection on threads/sessions/runs/workflows/handoffs, restart recovery,
  redaction, SDK/client contracts, web/TUI coverage, and explicit non-use of memory.
- **Environment and secrets** - PASS. Local work defaults to `~/.kura-test` with fake or
  seeded tenant/provider/thread evidence. Live connectors and production tenants are not
  required. Profile, overlay, provider, and audit output must not expose secrets, raw
  provider payloads, unsafe overlay content, disallowed message bodies, or cross-tenant
  identifiers.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/042-agent-profile-persona/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── agent-profile-persona.md
├── checklists/
│   └── requirements.md
└── tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── identity/
│   │   ├── types.go                 # add profiles.inspect/profiles.manage
│   │   ├── permissions.go           # role-derived permission policy
│   │   └── permissions_test.go
│   ├── profiles/
│   │   ├── profile.go               # profile, version, overlay, validation types
│   │   ├── policy.go                # activation, rollback, archive/disable rules
│   │   ├── projection.go            # runtime projection/domain summaries
│   │   ├── redaction.go             # safe profile/overlay summaries
│   │   └── *_test.go
│   ├── store/
│   │   ├── profile_store.go         # SQLite profile lifecycle persistence
│   │   ├── profile_projection.go    # runtime projection persistence/queries
│   │   ├── profile_restart_test.go
│   │   ├── profile_store_test.go
│   │   └── store.go                 # additive schema migration v53
│   ├── store/tenancy/
│   │   ├── profiles.go              # tenant-safe profile access guards
│   │   └── profiles_test.go
│   ├── api/
│   │   ├── agent_profiles.go        # /v1/profiles routes
│   │   ├── agent_profiles_test.go
│   │   ├── thread_lifecycle.go      # additive profile projection in detail
│   │   └── server.go                # route registration
│   ├── chat/
│   │   ├── service.go               # resolve tenant-default profile at work start
│   │   └── profile_projection_test.go
│   ├── events/
│   │   ├── agent_profiles.go        # profile event constructors
│   │   └── agent_profiles_test.go
│   └── contracts/
│       └── agent_profile_contracts_test.go

schemas/
├── api/
│   ├── agent-profile-resource.schema.json
│   ├── agent-profile-version-resource.schema.json
│   ├── agent-profile-overlay-reference.schema.json
│   ├── agent-profile-runtime-projection.schema.json
│   ├── agent-profile-list.response.schema.json
│   ├── create-agent-profile.request.schema.json
│   ├── update-agent-profile.request.schema.json
│   ├── agent-profile-activation.request.schema.json
│   ├── agent-profile-rollback.request.schema.json
│   ├── thread-detail.response.schema.json
│   ├── run-resource.schema.json
│   └── tenant-permission-resource.schema.json
└── events/
    ├── agent-profile-lifecycle.event.schema.json
    ├── agent-profile-version-created.event.schema.json
    ├── agent-profile-runtime-projected.event.schema.json
    └── tenant-permission-denied.event.schema.json

sdk/ts/src/
├── index.ts
└── agent-profile-persona.test.ts

web/src/
├── features/agent-profiles/
│   ├── AgentProfileEditor.tsx
│   ├── AgentProfileHistory.tsx
│   └── agent-profile-editor.test.tsx
├── features/thread-lifecycle.tsx
└── app/App.tsx

tui/src/
├── cli.ts
└── cli.test.ts

docs/
├── runtime/agent-profile-persona.md
├── runtime/thread-session-lifecycle.md
└── providers/provider-identity-and-profiles.md
```

**Structure Decision**: Implement agent profile/persona configuration as a new daemon
profile domain package with additive store, API, schema, and client surfaces. Keep
profile configuration separate from memory, workspace/capability binding, and provider
auth state. Runtime profile identity is projected into existing thread/session/run/workflow/handoff
evidence rather than replacing those resources.

## Roadmap 57 Planning Contracts

The implementation plan MUST keep this artifact complete before `/speckit.tasks`:

- [contracts/agent-profile-persona.md](./contracts/agent-profile-persona.md)
  - Profile resource and version shapes, tenant-default active selection, permissions,
    overlay reference validation, rollback/archive/disable behavior, runtime projection
    on threads/sessions/runs/workflows/handoffs, API routes, SDK/Web/TUI expectations, event
    evidence, JSON schemas, redaction, compatibility, migration, and rollback.

This artifact is a planning gate. Implementation is incomplete if profile configuration
can affect runtime behavior, operator inspection, provider defaults, overlays, or
historical evidence without a contract row and proving test.

## Migration And Rollback Plan

1. Add additive schema migration v53 with tables for `agent_profiles`,
   `agent_profile_versions`, `agent_profile_active_selections`,
   `agent_profile_overlay_references`, and `agent_profile_runtime_projections`. Include
   tenant, profile, version, status, actor, audit/event references, validation status,
   redaction status, created/updated/archived/disabled timestamps, and `document_json`.
2. Add `profiles.inspect` and `profiles.manage` to tenant permissions and role-derived
   permission sets. Owners/admins receive both by default; other role grants must be
   explicit during implementation if needed by product policy.
3. Seed one default profile per eligible tenant from existing provider defaults and safe
   local prompt/config references. Backfill must be metadata-only and conservative:
   unproven overlays become partial/invalid references with safe reason codes rather than
   hidden runtime truth.
4. Roll out in stages: schema and permissions, read-only default profile projection,
   profile list/detail/history, profile create/update/activation, runtime projection on
   new work, overlay validation, then rollback/archive/disable actions.
5. On rollback, disable profile create/update/activation/rollback/archive entry points
   and ignore new profile selection changes while preserving already-recorded profile,
   version, overlay, audit, and runtime projection evidence for authorized inspection.
   Existing chat, provider, thread, session, run, workflow, handoff, connector, and prompt-file behavior
   remains compatible.
6. Irreversible behavior is limited to recording metadata-only evidence rows and seeding
   default profile records. No destructive prompt/config rewrite, historical run rewrite,
   hard profile delete, memory backfill, or channel/workspace/capability binding is
   allowed in Phase 57.

## Post-Design Constitution Re-check

- **Roadmap closure** - PASS. `research.md`, `data-model.md`, contract, and quickstart
  cover the full Roadmap 57 surface across identity, store, profile domain policy, API,
  SDK, Web, TUI/operator shell, runtime projection, migration, restart recovery,
  redaction, and verification.
- **Production-grade, minimal, reversible change** - PASS. Design artifacts preserve
  existing provider, prompt/config, thread, session, run, workflow, handoff, connector, and lifecycle
  behavior while adding structured profile truth only through explicit tenant-default
  selection and runtime projections.
- **Contracts and auditability** - PASS. The contract defines route shapes, profile and
  version resources, event evidence, permission gates, overlay validation, profile
  runtime projection, redaction, compatibility, migration, and rollback.
- **Verification and observability** - PASS. The quickstart and contract identify
  targeted Go, contract, SDK, Web, TUI, restart, redaction, migration, and
  non-memory-scope checks.
- **Environment and secrets** - PASS. Verification uses test-environment seeded/fake
  evidence by default, with no production tenants or live connector credentials required.

No post-design violations require justification.

## Complexity Tracking

No constitution violations or complexity exceptions.
