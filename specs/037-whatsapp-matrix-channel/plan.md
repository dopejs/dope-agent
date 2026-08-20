# Implementation Plan: Matrix Channel Connector

**Branch**: `037-whatsapp-matrix-channel` | **Date**: 2026-05-09 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/037-whatsapp-matrix-channel/spec.md`

## Summary

Close Roadmap 52 by adding Matrix as the fourth hosted-ready real channel connector. The
connector consumes the Roadmap 48 shared channel conformance contract, hosted setup
terminal-state behavior, shared connector diagnostics, IM routing/dedupe, delivery
separation, persistence, API/schema/event, live-validation, and operator documentation
boundaries already used by Discord, Telegram, and Slack.

The implementation is additive: introduce a provider-specific Matrix connector package
for tenant-provided Matrix bot accounts on tenant-selected homeservers, validate
homeserver/account/room readiness without operating a Kura Matrix homeserver,
support unencrypted text DMs and allowed rooms with bot mention or configured command,
dedupe by homeserver plus room/direct conversation plus Matrix event ID while retaining
sync or transaction identity as evidence, send final foreground replies, expose Matrix as
a connector-backed background delivery target, and publish redacted setup, route,
diagnostic, conformance, and smoke evidence. Phase 52 does not include WhatsApp,
Kura-hosted homeserver provisioning, encrypted rooms, E2EE key/session management,
voice/calls, media-rich workflows, bridge automation, or memory-based personalization.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; JSON Schema contracts under
`schemas/`; TypeScript SDK, web, and TUI updates only if Matrix setup, diagnostic,
connector, delivery, or live-smoke projections become client-facing; channel/operator
documentation under `docs/`.
**Primary Dependencies**: Existing `daemon/internal/connectors`,
`daemon/internal/connectors/telegram`, `daemon/internal/connectors/slack`,
`daemon/internal/im`, `daemon/internal/imtypes`, `daemon/internal/store`,
`daemon/internal/store/tenancy`, `daemon/internal/setupwizard`,
`daemon/internal/api`, `daemon/internal/events`, `daemon/internal/delivery`,
`daemon/internal/livevalidation`, `daemon/internal/contracts`, `schemas/api`,
`schemas/events`, `docs/channels`, Roadmap 48 connector conformance vocabulary,
hosted setup terminal-state vocabulary, Roadmap 42 diagnostic reason model, and
Roadmap 28 delivery separation behavior. Matrix provider mechanics use the Matrix
Client-Server API concepts for authenticated client access, `/sync`, room membership,
message events, event IDs, transaction IDs, rate limits, and room power levels.
**Storage**: Existing SQLite daemon metadata store remains authoritative. Additive
changes may be needed for Matrix setup attempts, tenant-selected homeserver summaries,
tenant-provided bot account bindings, room/direct route policy, inbound homeserver/
room-or-direct/event dedupe evidence with retained sync or transaction identity, reply
outcomes, background delivery linkage, diagnostics, conformance evidence, live smoke
records, redaction status, and retention expiry. Existing connector, delivery, setup,
Discord, Telegram, and Slack data must remain readable.
**Testing**: Targeted Go tests for Matrix setup terminal states, bot credential redaction,
homeserver discovery/reachability handling, account binding, room validation, allowed
room mention/command gating, unencrypted text-only routing, encrypted/undecryptable
unsupported outcomes, duplicate event suppression with retained sync/transaction
evidence, final foreground replies, background delivery separation, diagnostic
classification/freshness/retention, homeserver/federation/rate-limit/network failures,
tenant isolation, schema/contract fixtures, and live smoke structured skip; `go test
./...` from `daemon/`; `make daemon-contract-test`; `pnpm test:clients` and `pnpm build`
only if SDK/web/TUI surfaces change; `go mod tidy` from `daemon/` after implementation.
**Target Platform**: Local-first daemon and hosted daemon behavior. Local verification
defaults to the isolated test environment (`~/.kura-test`, `127.0.0.1:19192`). Live
Matrix bot credentials and production tenants are not required for automated acceptance.
Safe live Matrix validation, when explicitly used, is scoped to a test tenant,
tenant-provided bot account, tenant-selected homeserver, unencrypted test rooms, and
redacted evidence approved by an operator.
**Project Type**: Multi-surface daemon connector feature spanning Matrix provider
mechanics, hosted setup, connector conformance, IM routing/dedupe, diagnostics,
persistence, API/schema/event contracts, delivery adapter behavior, live validation, and
operator documentation.
**Performance Goals**: Matrix fake-transport and conformance tests run as normal daemon
tests without live provider dependency. Duplicate Matrix deliveries produce at most one
agent run and one foreground reply. Authorized support inspection can identify latest
Matrix health, diagnostic reason, remediation, freshness, homeserver/bot binding,
selected routes, duplicate evidence, and delivery eligibility within 2 minutes. Setup
completes or returns an actionable terminal-state diagnostic within 5 minutes. Cached
connector diagnostics become stale after 15 minutes, and retained connector evidence
expires from normal inspection after 90 days unless an authorized longer retention policy
applies.
**Constraints**: Matrix setup uses tenant-provided bot accounts on tenant-selected
homeservers only. Kura does not operate a shared Matrix homeserver or provision
Matrix accounts in phase 52. Direct messages are accepted only from eligible senders and
tenant-allowed routes. Room messages are accepted only for tenant-allowed unencrypted
rooms and only when the message includes a bot mention or configured command. Encrypted
rooms, undecryptable events, E2EE key/session management, voice, calls, media-rich
workflows, bridge-specific automation, thinking visibility, and incremental visible
updates are unsupported. Matrix access tokens, raw provider payloads, event content that
cannot be safely redacted, credential-bearing payloads, and cross-tenant data must never
appear in setup results, diagnostics, events, fixtures, logs, support output,
conformance evidence, or smoke evidence.
**Scale/Scope**: One whole roadmap slice, Phase 52. Required coverage is Matrix only,
using fake transport tests and provider-specific regression against the Roadmap 48 shared
conformance contract. WhatsApp, future channel management UX, Kura-hosted Matrix
homeserver provisioning, encrypted room support, broad media, calls/voice, bridge
automation, and additional provider domains remain out of scope.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** - PASS. This plan closes Roadmap 52 as one production connector
  slice: Matrix provider decision evidence, tenant-provided bot account setup,
  tenant-selected homeserver validation, room/direct route policy, allowed-room
  mention/command gating, unencrypted text-only routing, event-ID dedupe, foreground
  replies, background delivery reuse, diagnostics, conformance proof, live smoke or
  structured skip, docs, and verification boundaries.
- **Production-grade, minimal, reversible change** - PASS. The change is additive around
  existing connector supervision, setup wizard, IM loop, store, diagnostics, delivery,
  API/schema/event, live validation, and docs surfaces. Rollback can disable Matrix
  setup, ingress, and delivery eligibility while preserving existing channels and
  retained redacted Matrix evidence.
- **Contracts and auditability** - PASS. Public contract implications are captured in
  [contracts/matrix-channel-connector.md](./contracts/matrix-channel-connector.md). Any
  API, schema, event, config, persistence, or docs changes must update fixtures and
  compatibility notes together.
- **Verification and observability** - PASS. Required verification covers setup terminal
  states, bot credential redaction, homeserver validation, room policy, route outcomes,
  duplicate suppression, unsupported encrypted/media/call surfaces, foreground/background
  delivery separation, diagnostics, freshness, retention, redaction, rate-limit and
  homeserver/federation/network behavior, live smoke skip, schema contracts, and
  operator docs.
- **Environment and secrets** - PASS. Local work defaults to `~/.kura-test` with fake
  Matrix transport and fake bot evidence. Real Matrix credentials, live connectors, and
  production tenants are used only in an explicit live validation path that can
  structured-skip when safe Matrix authorization is unavailable.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/037-whatsapp-matrix-channel/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── matrix-channel-connector.md
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
│   │   └── matrix/
│   │       ├── runtime.go           # setup/readiness projection, route outcomes,
│   │       │                        # capability declaration, diagnostic publication
│   │       ├── transport.go         # fake transport plus Matrix client-server boundary
│   │       ├── routes.go            # direct/room allowment and mention/command gates
│   │       ├── diagnostics.go       # Matrix condition to shared reason mapping
│   │       ├── smoke.go             # safe live smoke or structured skip evidence
│   │       └── *_test.go            # fake transport and Matrix regression coverage
│   ├── setupwizard/                 # tenant-provided Matrix bot account setup lifecycle
│   ├── im/ and imtypes/             # inbound homeserver/room/event identity, dedupe,
│   │                                # final reply and delivery evidence linkage
│   ├── delivery/                    # connector-backed Matrix delivery adapter
│   ├── store/ and store/tenancy/     # SQLite setup, homeserver/bot binding, route
│   │                                # policy, dedupe, diagnostic, retention accessors
│   ├── api/                         # connector/setup/diagnostic/live-smoke projections
│   ├── events/                      # setup, health, route, reply, delivery, smoke events
│   ├── livevalidation/              # safe live Matrix smoke or structured skip
│   └── contracts/                   # schema and fixture contract tests
├── go.mod
└── go.sum

schemas/
├── api/                             # Matrix setup, route policy, smoke, capability,
│                                    # diagnostic, and delivery resources if exposed
└── events/                          # setup, diagnostic, route, reply, delivery,
                                     # conformance, and smoke events if exposed

sdk/ts/                              # update only if public API shape changes for clients
web/ and tui/                        # update only if operator setup/repair UI changes

docs/
├── channels/
│   ├── channel-connector-conformance.md
│   └── matrix-channel-loop.md
└── runtime/                         # update if operator diagnostics, setup, or smoke
                                     # guidance changes
```

**Structure Decision**: Add Matrix as a provider-specific connector in place rather than
creating a separate execution path. `connectors/matrix` owns Matrix provider mechanics,
homeserver/bot readiness, route policy, Matrix sync/event delivery, and reply mechanics;
shared `connectors` owns conformance and diagnostic vocabulary; `setupwizard` owns the
tenant-provided bot account lifecycle; `im` owns message identity, dedupe, and foreground
reply separation; `delivery` owns connector-backed background delivery; `store` owns
tenant-scoped evidence and retention; `api`, `schemas`, and `events` expose additive
operator-visible state only as needed; docs record setup, routing, diagnostics, smoke,
and rollback behavior.

## Roadmap 52 Planning Contracts

The implementation plan MUST keep this artifact complete before `/speckit.tasks`:

- [contracts/matrix-channel-connector.md](./contracts/matrix-channel-connector.md)
  - Matrix tenant-provided bot setup terminal states, tenant-selected homeserver and bot
    binding, route policy, direct allowment, allowed-room mention/command gating,
    unencrypted text-only scope, encrypted/undecryptable unsupported outcomes, durable
    event identity, retained sync or transaction delivery evidence, diagnostic mappings,
    redaction and retention, conformance gates, live smoke or structured skip,
    compatibility, and rollback expectations.

This artifact is a planning gate. Implementation is incomplete if Matrix can become
ready, emit setup/diagnostic/conformance/smoke evidence, accept an inbound event, send a
foreground reply, or deliver a background notification without a contract row and proving
test.

## Migration And Rollback Plan

1. Add or extend contract/schema vocabulary for Matrix connector kind, tenant-provided
   bot setup, tenant-selected homeserver binding, route policy, route outcomes,
   diagnostic reasons, capability declarations, smoke evidence, and unsupported surfaces
   without changing existing Discord, Telegram, Slack, or generic connector behavior.
2. Add tenant-safe persistence/accessors for Matrix setup attempts, homeserver summaries,
   bot account bindings, room/direct route policy, durable homeserver/room-or-direct/
   event dedupe identity, retained sync or transaction evidence, reply outcomes,
   delivery linkage, diagnostics, conformance, smoke, redaction status, and retention
   expiry.
3. Add Matrix setup through the hosted setup wizard for tenant-provided bot accounts on
   tenant-selected homeservers. Persist repairable terminal-state evidence (`ready`,
   `degraded`, `unavailable`, `cancelled`, `action-required`) without exposing access
   tokens, raw provider payloads, event content that cannot be safely redacted, or
   cross-tenant data.
4. Implement Matrix runtime behind the shared connector supervisor and IM loop. Accept
   unencrypted direct text only from eligible senders and tenant-allowed routes, accept
   unencrypted room text only from tenant-allowed rooms with bot mention or configured
   command, record all other route decisions, and dedupe by tenant-selected homeserver,
   room/direct conversation, and Matrix event ID.
5. Map Matrix provider failures into shared diagnostic states, freshness, redaction, and
   retention rules. Distinguish invalid or revoked bot authorization, missing room
   permissions, ownership mismatch, unsupported homeserver behavior, homeserver or
   federation failure, rate limits, provider unavailable, local network failure, blocked
   route, duplicate inbound, unsupported encrypted/media/call surfaces, reply failure,
   and unknown failure where evidence permits.
6. Add final-only foreground replies for accepted Matrix messages. Add connector-backed
   Matrix background delivery adapter evidence while preserving foreground/background
   delivery separation.
7. Extend fake transport and Matrix regression tests for setup, route outcomes, duplicate
   inbound, sync replay, transaction retry, rate limits, homeserver/federation failures,
   reply failures, unsupported setup modes, encrypted/undecryptable events, unsupported
   Matrix surfaces, delivery separation, and live smoke structured skip.
8. Project Matrix setup, repair, diagnostic, conformance, and smoke evidence through API,
   schemas, events, SDK/client surfaces, and docs only where existing product surfaces
   require it.

Rollback disables Matrix setup, runtime ingress, and delivery-target eligibility while
retaining redacted Matrix setup, homeserver/bot binding, route policy, diagnostic, route,
reply, delivery, conformance, and smoke evidence for authorized inspection until
retention expiry. Additive persistence columns or tables remain inert rather than being
destructively removed. Existing Discord, Telegram, Slack, and shared connector behavior
must remain unaffected.

## Phase 0 Research

Research decisions are recorded in [research.md](./research.md). No unresolved
clarifications remain.

## Phase 1 Design

Design artifacts:

- [data-model.md](./data-model.md)
- [contracts/matrix-channel-connector.md](./contracts/matrix-channel-connector.md)
- [quickstart.md](./quickstart.md)

### Contract Risk Review

Contract risks, ordered by severity:

1. Matrix homeserver and account ambiguity can create cross-tenant routing if the bot
   account, room, homeserver, or direct conversation is not bound to exactly one active
   tenant route. The design requires tenant-selected homeserver, tenant-provided bot
   account, route policy validation, and fail-closed tenant mapping.
2. Encrypted Matrix rooms can leak or mishandle secrets if the connector implies E2EE
   support without key/session management. The design makes encrypted rooms,
   undecryptable events, and Matrix key/session management explicit unsupported outcomes.
3. Room traffic can trigger accidental agent runs if allowed-room membership alone is
   treated as consent. The design requires tenant-allowed rooms plus bot mention or
   configured command.
4. Duplicate suppression can be incorrect if sync batches or transaction IDs are used as
   the canonical identity. The design dedupes by homeserver, room/direct conversation,
   and Matrix event ID, while retaining sync or transaction identity as redacted delivery
   evidence.
5. Matrix diagnostics can leak access tokens, event bodies, room/user IDs, homeserver
   details, or cross-tenant content. The design redacts all evidence by default and
   suppresses detail when redaction confidence is insufficient.
6. Kura-hosted homeserver provisioning would introduce account lifecycle,
   federation, abuse, moderation, and operational responsibilities outside this roadmap.
   The design explicitly excludes hosted homeserver operation and Matrix account
   provisioning.

Compatibility assessment: additive. Existing connector list/resource schemas, Discord,
Telegram, Slack, delivery resources, and IM loop behavior remain backward compatible.
Matrix introduces new provider-specific resources or additive fields as needed. Old
clients continue to read existing connector resources and ignore Matrix-specific
projections they do not understand.

Required validation cases: Matrix setup terminal states, invalid/revoked bot credential,
unsupported homeserver, homeserver unreachable, missing room permission, ownership
mismatch, tenant-selected homeserver binding, tenant-provided bot account binding,
allowed room validation, explicit direct allowment, allowed-room mention accepted,
allowed-room command accepted, room without mention/command ignored, unallowed room
blocked, wrong homeserver/account blocked, encrypted room unsupported, undecryptable event
unsupported, unsupported media/calls/reactions/bridge metadata, durable dedupe after sync
replay/transaction retry/restart, retained sync/transaction evidence, final foreground
reply success/failure, background delivery success/failure/suppression, diagnostic
freshness/current-truth behavior, 90-day retention, redaction fail-closed behavior,
tenant isolation, schema contract tests, docs updates, live smoke success or structured
skip, and `go mod tidy`.

## Post-Design Constitution Check

- **Roadmap closure** - PASS. Research, data model, quickstart, and contract artifacts
  cover all Roadmap 52 gates: Matrix provider decision evidence, tenant-provided bot
  setup, homeserver binding, route policy, direct and room routing, mention/command
  gating, unencrypted text-only scope, dedupe, foreground replies, background delivery
  reuse, diagnostics, conformance proof, smoke or structured skip, compatibility, docs,
  and verification.
- **Production-grade, minimal, reversible change** - PASS. The design is additive and
  staged: contract vocabulary first, persistence/projections, Matrix setup/runtime,
  diagnostic mapping, delivery adapter, fake transport evidence, public projections, and
  docs. Rollback disables Matrix setup/runtime/delivery eligibility while preserving
  existing channels and retained evidence.
- **Contracts and auditability** - PASS. The contract defines setup terminal states,
  homeserver/bot binding, route policy, durable identity, diagnostic mappings,
  capability declarations, retention, redaction, live smoke evidence, compatibility, and
  verification gates. Schemas, docs, fixtures, and tests must change together.
- **Verification and observability** - PASS. Quickstart names targeted package tests,
  full daemon tests, schema contract tests, delivery separation checks, live-smoke skip
  evidence, redaction checks, retention checks, and optional client verification when
  public client surfaces change.
- **Environment and secrets** - PASS. Test environment and fake Matrix evidence remain
  the default. Live Matrix bot credentials and production tenants are excluded from
  automated acceptance unless an operator explicitly chooses live validation, and all
  evidence is redacted.
