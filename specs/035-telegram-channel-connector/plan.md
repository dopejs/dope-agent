# Implementation Plan: Telegram Channel Connector

**Branch**: `035-telegram-channel-connector` | **Date**: 2026-05-07 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/035-telegram-channel-connector/spec.md`

## Summary

Close Roadmap 50 by adding Telegram as the second hosted-ready real channel connector
after Discord, consuming the Roadmap 48 shared channel conformance contract and the
Roadmap 46 hosted setup wizard. The implementation is additive: introduce a
tenant-owned Telegram connector resource and bot-token setup path, validate and redact
credentials, bind allowed Telegram users/chats/groups, route text-only direct messages
and explicitly gated group messages through the existing IM loop, dedupe inbound messages
by Telegram chat/message identity, send at least final-only foreground replies, reuse
Telegram for connector-backed background delivery, and publish redacted diagnostics and
conformance evidence.

The plan keeps Telegram provider mechanics inside a new provider-specific connector
package while reusing shared connector supervision, setup wizard, IM identity/dedupe,
delivery, diagnostics, persistence, schema/event, and contract-test boundaries. Phase 50
does not add voice, attachments, payments, mini apps, broad media transfer, memory, or a
new channel execution path.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; JSON Schema contracts under
`schemas/`; TypeScript SDK, web, and TUI updates only if Telegram setup, diagnostic,
connector, or delivery projections become client-facing; channel/operator documentation
under `docs/`.
**Primary Dependencies**: Existing `daemon/internal/connectors`,
`daemon/internal/im`, `daemon/internal/imtypes`, `daemon/internal/store`,
`daemon/internal/store/tenancy`, `daemon/internal/setupwizard`, `daemon/internal/api`,
`daemon/internal/events`, `daemon/internal/delivery`, `daemon/internal/contracts`,
`schemas/api`, `schemas/events`, `docs/channels`, Roadmap 48 connector conformance
vocabulary, Roadmap 46 hosted setup wizard terminal-state vocabulary, and Roadmap 28
delivery separation behavior.
**Storage**: Existing SQLite daemon metadata store remains authoritative. Additive
changes may be needed for Telegram setup attempts, account binding summaries, explicit
allowed sender/chat/group bindings, inbound chat/message dedupe evidence with retained
update identity, reply outcomes, background delivery linkage, diagnostics, conformance
evidence, live smoke records, redaction status, and retention expiry. Existing connector,
delivery, and setup data must remain readable.
**Testing**: Targeted Go tests for Telegram setup validation, terminal states,
credential redaction, explicit allowment, group mention/command gating, text-only
unsupported surfaces, chat/message dedupe with retained update evidence, foreground
reply outcomes, background delivery separation, diagnostic classification/freshness/
retention, provider/network/rate-limit failures, reconnect/retry behavior, tenant
isolation, schema/contract fixtures, and live smoke structured skip; `go test ./...`
from `daemon/`; `make daemon-contract-test`; `pnpm test:clients` and `pnpm build` only
if SDK/web/TUI surfaces change; `go mod tidy` from `daemon/` after implementation.
**Target Platform**: Local-first daemon and hosted daemon behavior. Local verification
defaults to the isolated test environment (`~/.kura-test`, `127.0.0.1:19192`). Live
Telegram credentials and production tenants are not required for automated acceptance.
Safe live Telegram credentials, when explicitly used, are non-production bot credentials
scoped to a test tenant and test chats/groups, approved by an operator for validation,
redacted in all evidence, and isolated from normal production tenants.
**Project Type**: Multi-surface daemon connector feature spanning Telegram provider
mechanics, hosted setup, connector conformance, IM routing/dedupe, diagnostics,
persistence, API/schema/event contracts, delivery adapter behavior, live validation, and
operator documentation.
**Performance Goals**: Telegram fake-transport and conformance tests run as normal
daemon tests without live provider dependency. Duplicate provider deliveries produce at
most one agent run and one foreground reply. Authorized support inspection can identify
latest Telegram health, diagnostic reason, remediation, and freshness within 2 minutes.
Setup completes or returns an actionable terminal-state diagnostic within 5 minutes.
Cached connector diagnostics become stale after 15 minutes, and retained connector
evidence expires from normal inspection after 90 days unless an authorized longer
retention policy applies.
**Constraints**: Telegram is text-only for phase 50. Attachments, voice, payments, mini
apps, media transfer, memory behavior, and all other out-of-scope Telegram surfaces must
produce explicit unsupported outcomes. Only explicitly allowed Telegram users or chats
may create runs. Group messages require an explicitly allowed group plus a bot mention
or command. Tokens, authorization headers, credential-bearing payloads, raw provider
payloads, and cross-tenant data must never appear in setup results, diagnostics, events,
fixtures, logs, support output, conformance evidence, or smoke evidence.
**Scale/Scope**: One whole roadmap slice, Phase 50. Required coverage is Telegram only,
using fake transport tests and provider-specific regression against the Roadmap 48
shared conformance contract. Slack, WhatsApp/Matrix, memory, voice, attachments, and
additional provider domains remain out of scope.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** - PASS. This plan closes Roadmap 50 as one production connector
  slice: hosted setup, bot-token validation/redaction, explicit DM/user/chat allowment,
  group mention/command gating, text-only routing, chat/message dedupe with update
  evidence, final-only replies, background delivery target reuse, diagnostics,
  conformance proof, live smoke or structured skip, docs, and verification boundaries.
- **Production-grade, minimal, reversible change** - PASS. The change is additive around
  existing connector supervision, setup wizard, IM loop, store, diagnostics, delivery,
  API/schema/event, live validation, and docs surfaces. Rollback can disable Telegram
  setup, ingress, and delivery eligibility while preserving existing channels and
  retained redacted Telegram evidence.
- **Contracts and auditability** - PASS. Public contract implications are captured in
  [contracts/telegram-channel-connector.md](./contracts/telegram-channel-connector.md).
  Any API, schema, event, config, persistence, or docs changes must update fixtures and
  compatibility notes together.
- **Verification and observability** - PASS. Required verification covers setup terminal
  states, credential redaction, explicit allowment, group gating, route outcomes,
  duplicate suppression, reply failures, foreground/background delivery separation,
  diagnostics, freshness, retention, redaction, reconnect/rate-limit behavior, live
  smoke skip, schema contracts, and operator docs.
- **Environment and secrets** - PASS. Local work defaults to `~/.kura-test` with fake
  credentials and fake transports. Real Telegram credentials, live connectors, and
  production tenants are used only in an explicit live validation path that can
  structured-skip when safe credentials are unavailable.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/035-telegram-channel-connector/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── telegram-channel-connector.md
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
│   │   └── telegram/
│   │       ├── runtime.go           # setup/readiness projection, route outcomes,
│   │       │                        # capability declaration, diagnostic publication
│   │       ├── transport.go         # bot validation, update polling/webhook boundary,
│   │       │                        # send/retry/rate-limit behavior
│   │       └── *_test.go            # fake transport and Telegram regression coverage
│   ├── setupwizard/                 # submitted-secret flow and terminal-state mapping
│   ├── im/ and imtypes/             # inbound chat/message identity, dedupe, final reply
│   ├── delivery/                    # connector-backed Telegram delivery adapter
│   ├── store/ and store/tenancy/     # SQLite setup, allowment, dedupe, diagnostic,
│   │                                # retention, and tenant-safe accessors
│   ├── api/                         # connector/setup/diagnostic/live-smoke projections
│   ├── events/                      # setup, health, route, reply, delivery, smoke events
│   └── contracts/                   # schema and fixture contract tests
├── go.mod
└── go.sum

schemas/
├── api/                             # Telegram setup, allowment, account binding,
│                                    # capability, diagnostic, and smoke resources
└── events/                          # setup, diagnostic, route, reply, delivery,
                                     # conformance, and smoke events if exposed

sdk/ts/                              # update only if public API shape changes for clients
web/ and tui/                        # update only if operator setup/repair UI changes

docs/
├── channels/
│   ├── channel-connector-conformance.md
│   └── telegram-channel-loop.md
└── runtime/                         # update if operator diagnostics, setup, or smoke
                                     # guidance changes
```

**Structure Decision**: Add Telegram as a provider-specific connector in place rather
than creating a separate execution path. `connectors/telegram` owns Telegram provider
mechanics and readiness projection; shared `connectors` owns conformance and diagnostic
vocabulary; `setupwizard` owns submitted-token lifecycle; `im` owns message identity,
dedupe, and foreground reply separation; `delivery` owns connector-backed background
delivery; `store` owns tenant-scoped evidence and retention; `api`, `schemas`, and
`events` expose additive operator-visible state only as needed; docs record setup,
routing, diagnostics, smoke, and rollback behavior.

## Roadmap 50 Planning Contracts

The implementation plan MUST keep this artifact complete before `/speckit.tasks`:

- [contracts/telegram-channel-connector.md](./contracts/telegram-channel-connector.md)
  - Telegram setup terminal states, credential validation, explicit sender/chat/group
  allowment, group mention/command gating, text-only unsupported-surface behavior,
  durable chat/message dedupe with retained update identity, reply and delivery
  separation, diagnostic mappings, redaction and retention, conformance gates, live smoke
  or structured skip, compatibility and rollback expectations.

This artifact is a planning gate. Implementation is incomplete if Telegram can become
ready, emit setup/diagnostic/conformance/smoke evidence, accept an inbound message, send
a foreground reply, or deliver a background notification without a contract row and
proving test.

## Migration And Rollback Plan

1. Add or extend contract/schema vocabulary for Telegram connector kind, setup terminal
   states, account binding, explicit allowment, route outcomes, diagnostic reasons,
   capability declarations, smoke evidence, and unsupported surfaces without changing
   existing Discord or generic connector behavior.
2. Add tenant-safe persistence/accessors for Telegram setup attempts, bot account
   binding, allowed Telegram users/chats/groups, chat/message dedupe identity, retained
   update evidence, reply outcomes, delivery linkage, diagnostics, conformance, smoke,
   redaction status, and retention expiry.
3. Add Telegram submitted-token setup and validation through the hosted setup wizard.
   Persist repairable terminal-state evidence (`ready`, `degraded`, `unavailable`,
   `cancelled`, `action-required`) without exposing token material.
4. Implement Telegram runtime behind the shared connector supervisor and IM loop. Accept
   text-only DMs from explicitly allowed users/chats, accept group text only when the
   group is explicitly allowed and the message contains a bot mention or command, record
   all other route decisions, and dedupe by chat/message identity.
5. Map Telegram provider failures into shared diagnostic states, freshness, redaction,
   and retention rules. Suppress detailed evidence when redaction confidence is
   insufficient.
6. Add final-only foreground replies and connector-backed background delivery adapter
   evidence while preserving foreground/background delivery separation.
7. Extend fake transport and Telegram regression tests for setup, route outcomes,
   duplicate inbound, rate limits, reconnects/retries, reply failures, unsupported
   surfaces, delivery separation, and live smoke structured skip.
8. Project Telegram setup, repair, diagnostic, conformance, and smoke evidence through
   API, schemas, events, SDK/client surfaces, and docs only where existing product
   surfaces require it.

Rollback disables Telegram setup, runtime ingress, and delivery-target eligibility while
retaining redacted Telegram setup, allowment, diagnostic, route, reply, delivery,
conformance, and smoke evidence for authorized inspection until retention expiry.
Additive persistence columns or tables remain inert rather than being destructively
removed. Existing Discord and shared connector behavior must remain unaffected.

## Phase 0 Research

Research decisions are recorded in [research.md](./research.md). No unresolved
clarifications remain.

## Phase 1 Design

Design artifacts:

- [data-model.md](./data-model.md)
- [contracts/telegram-channel-connector.md](./contracts/telegram-channel-connector.md)
- [quickstart.md](./quickstart.md)

### Contract Risk Review

Contract risks, ordered by severity:

1. Telegram bot links can grant unintended access if direct messages are accepted from
   arbitrary senders. The design requires explicit Telegram user/chat allowment before a
   message can create an agent run.
2. Group traffic can trigger public agent replies accidentally if allowment alone is
   treated as consent. The design requires both explicit group allowment and a bot
   mention or command.
3. Duplicate suppression can be incorrect if Telegram update IDs are used as the only
   identity. The design dedupes by chat/message identity and retains update identity as
   redacted provider delivery evidence.
4. Unsupported attachments or media can leak unreviewed storage/redaction behavior. The
   design is text-only and makes every attachment/media/voice/payment/mini-app input an
   explicit unsupported route outcome.
5. Telegram diagnostics can leak raw bot tokens or provider payloads. The design redacts
   all evidence by default and suppresses detail when redaction confidence is
   insufficient.

Compatibility assessment: additive. Existing connector list/resource schemas, Discord
setup resources, delivery resources, and IM loop behavior remain backward compatible.
Telegram introduces new provider-specific resources or additive fields as needed. Old
clients continue to read existing connector resources and ignore Telegram-specific
projections they do not understand.

Required validation cases: Telegram setup terminal states, invalid/revoked/malformed bot
token redaction, provider unavailable setup, explicit DM sender/chat allowment, blocked
DM sender, group disabled, allowed group without mention/command ignored, allowed group
with mention/command accepted, unsupported attachments/media/voice/payments/mini apps,
chat/message dedupe after retry/reconnect/restart, retained update evidence, final-only
reply success/failure, background delivery success/failure/suppression, diagnostic
freshness/current-truth behavior, 90-day retention, redaction fail-closed behavior,
tenant isolation, schema contract tests, docs updates, live smoke success or structured
skip, and `go mod tidy`.

## Post-Design Constitution Check

- **Roadmap closure** - PASS. Research, data model, quickstart, and contract artifacts
  cover all Roadmap 50 gates: hosted setup, bot-token validation/redaction, explicit
  allowment, group gating, text-only unsupported surfaces, dedupe, final replies,
  background delivery reuse, diagnostics, conformance proof, smoke or structured skip,
  compatibility, docs, and verification.
- **Production-grade, minimal, reversible change** - PASS. The design is additive and
  staged: contract vocabulary first, persistence/projections, Telegram setup/runtime,
  diagnostic mapping, delivery adapter, fake transport evidence, public projections,
  docs. Rollback disables Telegram setup/runtime/delivery eligibility while preserving
  existing channels and retained evidence.
- **Contracts and auditability** - PASS. The contract defines setup terminal states,
  allowment, group gating, durable identity, diagnostic mappings, capability
  declarations, retention, redaction, live smoke evidence, compatibility, and
  verification gates. Schemas, docs, fixtures, and tests must change together.
- **Verification and observability** - PASS. Quickstart names targeted package tests,
  full daemon tests, schema contract tests, delivery separation checks, live-smoke skip
  evidence, redaction checks, retention checks, and optional client verification when
  public client surfaces change.
- **Environment and secrets** - PASS. Test environment and fake credentials remain the
  default. Live Telegram credentials and production tenants are excluded from automated
  acceptance unless an operator explicitly chooses live validation, and all evidence is
  redacted.
