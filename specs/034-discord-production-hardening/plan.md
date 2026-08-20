# Implementation Plan: Discord Production Hardening

**Branch**: `034-discord-production-hardening` | **Date**: 2026-05-07 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/034-discord-production-hardening/spec.md`

## Summary

Close Roadmap 49 by hardening the existing Discord connector into a hosted-production-ready
channel connector that consumes the Roadmap 48 shared channel conformance contract. The
implementation stays inside the existing daemon connector, IM loop, setup wizard,
diagnostic, live validation, API/schema, store, and docs boundaries: Discord setup gains
tenant-owned validation and repairable degraded state, Discord capability and diagnostic
evidence map to the shared contract, routing/dedupe/reply behavior is proven with fake
transport and Discord regression cases, and live hosted smoke either runs with explicit
safe credentials or records a structured skip.

The plan preserves existing local Discord test usage while making hosted readiness stricter:
partially invalid or missing explicit hosted destinations save as degraded/needs repair,
hosted-ready status is blocked until selected destinations validate, diagnostic freshness
and retention inherit phase 48 rules, and unsupported optional Discord surfaces remain
explicit rather than hidden.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; JSON Schema contracts under
`schemas/`; TypeScript SDK, web, and TUI updates only if Discord hosted setup or
diagnostic projections become client-facing; operator/channel documentation under `docs/`.
**Primary Dependencies**: Existing `daemon/internal/connectors`,
`daemon/internal/connectors/discord`, `daemon/internal/im`, `daemon/internal/imtypes`,
`daemon/internal/store`, `daemon/internal/store/tenancy`, `daemon/internal/api`,
`daemon/internal/events`, `daemon/internal/contracts`, `schemas/api`, `schemas/events`,
`docs/channels`, Roadmap 48 connector conformance vocabulary, Roadmap 46 hosted setup
wizard surfaces, and Roadmap 42 diagnostic freshness/redaction/retention behavior.
**Storage**: Existing SQLite daemon metadata store remains authoritative. Additive
changes may be needed for Discord setup validation evidence, account binding summaries,
degraded/needs-repair setup state, destination validation results, Discord diagnostic
state, Discord conformance evidence, live smoke records, and retention expiry. Existing
Discord config and connector message history must remain readable.
**Testing**: Targeted Go tests for Discord setup validation, degraded setup state,
destination allowlist behavior, route outcomes, duplicate inbound suppression, reply
failure evidence, diagnostic classification/freshness/retention, redaction fail-closed
behavior, gateway disconnect/reconnect, rate-limit handling, existing local config
compatibility, schema/contract fixtures, and live smoke structured skip; `go test ./...`
from `daemon/`; `make daemon-contract-test`; `pnpm test:clients` and `pnpm build` only
if SDK/web/TUI surfaces change; `go mod tidy` from `daemon/` after implementation.
**Target Platform**: Local-first daemon and hosted daemon behavior. Local verification
defaults to the isolated test environment (`~/.kura-test`, `127.0.0.1:19192`). Live
Discord credentials and production tenants are not required for automated acceptance.
Safe live credentials, when explicitly used, are non-production credentials scoped to a
test tenant, approved by an operator for validation, redacted in all evidence, and isolated
from normal production tenants.
**Project Type**: Multi-surface daemon hardening feature spanning Discord provider
mechanics, hosted setup, connector conformance, IM routing/dedupe, diagnostics,
persistence, API/schema/event contracts, live validation, and operator documentation.
**Performance Goals**: Discord fake-transport and conformance regression tests run as
normal daemon tests without live provider dependency. Cached Discord diagnostics are
marked stale after 15 minutes, failed connector actions produce current diagnostic truth
before remediation is shown, and retained evidence expires from normal inspection after
90 days unless an authorized longer retention policy applies.
**Constraints**: Hosted setup is fail-closed for missing explicit guild/channel
destinations and for partially invalid selected destinations. Tokens, authorization
headers, credential-bearing payloads, and cross-tenant data must never appear in setup
results, diagnostics, events, fixtures, logs, support output, conformance evidence, or
smoke evidence. Discord voice, broad rich media, marketplace listing, memory-based thread
recall, and broad multi-channel abstractions are out of scope.
**Scale/Scope**: One whole roadmap slice, Phase 49. Required coverage is the existing
Discord connector only, using fake transport tests and Discord regression against the
Roadmap 48 shared conformance contract. Telegram, Slack, and new channel connector work
remain out of scope.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** - PASS. This plan closes Roadmap 49 as one production hardening
  slice: tenant-owned Discord setup, degraded repair state, guild/channel allowlist
  validation, mention/DM behavior, diagnostics, rate-limit/reconnect evidence, reply
  progression declaration, conformance proof, live smoke or structured skip, docs, and
  verification boundaries.
- **Production-grade, minimal, reversible change** - PASS. The change is additive around
  existing Discord runtime, setup, diagnostics, store, API/schema/event, live validation,
  and docs surfaces. Rollback can disable hosted-ready gating and new Discord
  projections while preserving the current local Discord message loop and retained
  redacted evidence.
- **Contracts and auditability** - PASS. Public contract implications are captured in
  [contracts/discord-production-hardening.md](./contracts/discord-production-hardening.md).
  Any API, schema, event, config, persistence, or docs changes must update fixtures and
  compatibility notes together.
- **Verification and observability** - PASS. Required verification covers setup
  validation, degraded state, destination repair evidence, route outcomes, dedupe, reply
  failures, foreground/background delivery separation, diagnostics, freshness, retention,
  redaction, reconnect/rate-limit behavior, live smoke skip, schema contracts, and
  operator docs.
- **Environment and secrets** - PASS. Local work defaults to `~/.kura-test` with fake
  credentials and fake transports. Real Discord credentials, live connectors, and
  production tenants are used only in an explicit live validation path that can
  structured-skip when safe credentials are unavailable.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/034-discord-production-hardening/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── discord-production-hardening.md
├── checklists/
│   └── requirements.md
└── tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── connectors/
│   │   ├── conformance.go           # shared phase 48 conformance vocabulary consumed
│   │   ├── diagnostics.go           # freshness, retention, redaction, reason mapping
│   │   └── discord/
│   │       ├── runtime.go           # setup/readiness projection, route outcomes,
│   │       │                        # capability declaration, diagnostic publication
│   │       ├── transport.go         # gateway auth, rate limit, send/edit/reconnect
│   │       └── *_test.go            # fake transport and Discord regression coverage
│   ├── im/ and imtypes/             # inbound identity, dedupe, reply progression,
│   │                                # reply failure separation
│   ├── store/ and store/tenancy/     # SQLite evidence, setup state, tenant-safe reads,
│   │                                # retention expiry and migration fixtures
│   ├── api/                         # connector/setup/diagnostic/live-smoke projections
│   ├── events/                      # connector setup, health, route, reply, smoke events
│   └── contracts/                   # schema and fixture contract tests
├── go.mod
└── go.sum

schemas/
├── api/                             # connector account binding, capability, diagnostic,
│                                    # setup, live validation, and smoke schemas
└── events/                          # connector health, diagnostic, route, reply,
                                     # conformance, and smoke events if exposed

sdk/ts/                              # update only if public API shape changes for clients
web/ and tui/                        # update only if operator setup/repair UI changes

docs/
├── channels/discord-channel-loop.md
├── channels/channel-connector-conformance.md
└── runtime/                         # update if operator diagnostics or live validation
                                     # guidance changes
```

**Structure Decision**: Harden Discord in place rather than creating a new connector
service. `connectors/discord` owns Discord provider mechanics and readiness projection;
shared `connectors` owns conformance and diagnostic vocabulary; `im` owns message
identity, dedupe, and reply outcome separation; `store` owns tenant-scoped evidence and
retention; `api`, `schemas`, and `events` expose additive operator-visible state only as
needed; docs record the hosted setup, repair, smoke, and rollback behavior.

## Roadmap 49 Planning Contracts

The implementation plan MUST keep this artifact complete before `/speckit.tasks`:

- [contracts/discord-production-hardening.md](./contracts/discord-production-hardening.md)
  - Discord setup states, destination validation, degraded/needs-repair rules, capability
  declaration, diagnostic mappings, redaction and retention, route outcomes, reply
  progression, conformance gates, live smoke or structured skip, compatibility and
  rollback expectations.

This artifact is a planning gate. Implementation is incomplete if Discord can become
hosted-ready, emit a setup/diagnostic/conformance/smoke result, or handle a routed
message without a contract row and proving test.

## Migration And Rollback Plan

1. Add or extend contract/schema vocabulary for Discord hosted setup state,
   destination validation, degraded/needs-repair status, diagnostic reasons, capability
   declarations, and smoke evidence without changing existing local Discord startup.
2. Add tenant-safe persistence/accessors for Discord setup validation and repair evidence.
   Preserve existing config projection and connector message history; legacy local config
   remains usable even when hosted-ready state is stricter.
3. Adapt Discord readiness so authentic credentials plus partially invalid or missing
   explicit hosted destinations save as degraded/needs repair and block hosted-ready
   status until selected destinations validate.
4. Map Discord provider failures into shared diagnostic states, freshness, redaction, and
   retention rules. Suppress detailed evidence when redaction confidence is insufficient.
5. Extend fake transport and Discord regression tests for setup, route outcomes, duplicate
   inbound, rate limits, reconnects, reply failures, reply progression degradation, and
   live smoke structured skip.
6. Project hosted setup, repair, diagnostic, conformance, and smoke evidence through API,
   schemas, events, SDK/client surfaces, and docs only where existing product surfaces
   require it.

Rollback disables hosted-ready gating, repair projections, and live-smoke publication for
Discord while preserving the current local gateway message loop and retained redacted
evidence for authorized inspection until retention expiry. Additive persistence columns or
tables remain inert rather than being destructively removed.

## Phase 0 Research

Research decisions are recorded in [research.md](./research.md). No unresolved
clarifications remain.

## Phase 1 Design

Design artifacts:

- [data-model.md](./data-model.md)
- [contracts/discord-production-hardening.md](./contracts/discord-production-hardening.md)
- [quickstart.md](./quickstart.md)

### Contract Risk Review

Contract risks, ordered by severity:

1. Hosted setup can accidentally grant broad Discord access if missing guild/channel
   allowlists are treated as "all destinations." The design makes hosted-ready fail
   closed: setup saves as degraded/needs repair until explicit destinations validate.
2. Partially invalid destinations can create misleading ready state. The design separates
   persisted setup progress from hosted-ready status so tenants can repair without
   advertising a broken connector as production-ready.
3. Discord diagnostics can remain too coarse (`auth_error` / `transport_error`) for
   support. The design maps provider failures into stable shared reason codes, including
   permission, message content access, rate limit, gateway, network, duplicate, blocked
   route, reply failure, and unknown failure.
4. Secret or cross-tenant leakage can enter diagnostic or smoke evidence. The design
   redacts all evidence by default and suppresses detail when redaction confidence is
   insufficient.
5. Live Discord validation can become an implicit production dependency. The design
   defaults to fake transport/test environment and requires live smoke to either run with
   explicit safe credentials or record structured skip with owner, reason, date, and risk.

Compatibility assessment: additive. Existing Discord local config, gateway transport, and
message loop remain compatible. Hosted readiness adds stricter state and evidence; public
schemas/events gain fields or resources additively if needed. Old clients continue to read
existing connector resources.

Required validation cases: Discord setup validation, missing explicit destinations,
partially invalid destinations, credential failure, permission/message-content failure,
allowlist inspection, route accept/block/ignore/duplicate outcomes, mention normalization,
dedupe after replay/reconnect, rate-limit and reconnect evidence, reply progression
degradation, reply failure separation, foreground/background delivery separation,
diagnostic freshness/current-truth behavior, 90-day retention, redaction fail-closed
behavior, local config compatibility, live smoke success or structured skip, schema
contract tests, docs updates, and `go mod tidy`.

## Post-Design Constitution Check

- **Roadmap closure** - PASS. Research, data model, quickstart, and contract artifacts
  cover all Roadmap 49 gates: hosted setup, degraded repair, allowlists, DM/mention
  behavior, diagnostics, reconnect/rate-limit evidence, reply progression declaration,
  conformance proof, smoke or structured skip, compatibility, docs, and verification.
- **Production-grade, minimal, reversible change** - PASS. The design is additive and
  staged: contract vocabulary first, persistence/projections, Discord readiness,
  diagnostic mapping, fake transport evidence, public projections, docs. Rollback disables
  hosted-ready gating and projections while preserving local Discord behavior.
- **Contracts and auditability** - PASS. The contract defines setup state vocabulary,
  destination validation, diagnostic mappings, capability declarations, retention,
  redaction, live smoke evidence, compatibility, and verification gates. Schemas, docs,
  fixtures, and tests must change together.
- **Verification and observability** - PASS. Quickstart names targeted package tests,
  full daemon tests, schema contract tests, delivery separation checks, live-smoke skip
  evidence, redaction checks, retention checks, and optional client verification when
  public client surfaces change.
- **Environment and secrets** - PASS. Test environment and fake credentials remain the
  default. Live Discord credentials and production tenants are excluded from automated
  acceptance unless an operator explicitly chooses live validation, and all evidence is
  redacted.
