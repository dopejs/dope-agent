# Implementation Plan: Slack Channel Connector

**Branch**: `036-slack-channel-connector` | **Date**: 2026-05-08 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/036-slack-channel-connector/spec.md`

## Summary

Close Roadmap 51 by adding Slack as a hosted-ready work-channel connector that consumes
the Roadmap 48 shared channel conformance contract, the Roadmap 46 hosted setup wizard,
and the Roadmap 42 diagnostic model. The implementation is additive: introduce a
tenant-owned Slack connector resource where each connector binds exactly one Slack
workspace through hosted Slack app installation/OAuth setup, allows multiple Slack
connectors per tenant, validates selected channels and explicitly allowed direct-message
users or user groups, routes DMs and mention-gated channel messages through the existing
IM loop, dedupes inbound messages by workspace/conversation/message identity while
retaining event identity as evidence, sends final foreground replies with channel
mentions rooted in Slack threads, reuses Slack as a connector-backed background delivery
target, and publishes redacted diagnostics, conformance, and smoke evidence.

The plan keeps Slack provider mechanics inside a new provider-specific connector package
while reusing shared connector supervision, OAuth setup, IM identity/dedupe, delivery,
diagnostics, persistence, schema/event, and contract-test boundaries. Phase 51 does not
add Slack marketplace publication, enterprise grid administration, voice huddles,
memory-based team context, raw-token setup, or broad media and interactive Slack surfaces.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; JSON Schema contracts under
`schemas/`; TypeScript SDK, web, and TUI updates only if Slack setup, diagnostic,
connector, delivery, or live-smoke projections become client-facing; channel/operator
documentation under `docs/`.
**Primary Dependencies**: Existing `daemon/internal/connectors`,
`daemon/internal/connectors/telegram` and `daemon/internal/connectors/discord` patterns,
`daemon/internal/im`, `daemon/internal/imtypes`, `daemon/internal/store`,
`daemon/internal/store/tenancy`, `daemon/internal/setupwizard`,
`daemon/internal/setupwizard/oauth.go`, `daemon/internal/api`, `daemon/internal/events`,
`daemon/internal/delivery`, `daemon/internal/livevalidation`,
`daemon/internal/contracts`, `schemas/api`, `schemas/events`, `docs/channels`, Roadmap
48 connector conformance vocabulary, Roadmap 46 hosted setup terminal-state vocabulary,
Roadmap 42 diagnostic reason model, and Roadmap 28 delivery separation behavior.
**Storage**: Existing SQLite daemon metadata store remains authoritative. Additive
changes may be needed for Slack setup attempts, OAuth installation summaries, workspace
bindings, connector-specific selected channel policy, explicit DM user or user-group
allowment, inbound workspace/conversation/message dedupe evidence with retained event
identity, thread reply outcomes, background delivery linkage, diagnostics, conformance
evidence, live smoke records, redaction status, and retention expiry. Existing connector,
delivery, setup, Discord, and Telegram data must remain readable.
**Testing**: Targeted Go tests for hosted Slack app installation/OAuth terminal states,
workspace binding cardinality, OAuth grant and scope redaction, selected channel
allowment, explicit DM user or user-group allowment, mention gating, unsupported setup
modes, unsupported Slack surfaces, workspace/conversation/message dedupe with retained
event evidence, thread-rooted channel replies, normal DM replies, foreground/background
delivery separation, diagnostic classification/freshness/retention, provider/network/
rate-limit/event-delivery failures, tenant isolation, schema/contract fixtures, and live
smoke structured skip; `go test ./...` from `daemon/`; `make daemon-contract-test`;
`pnpm test:clients` and `pnpm build` only if SDK/web/TUI surfaces change; `go mod tidy`
from `daemon/` after implementation.
**Target Platform**: Local-first daemon and hosted daemon behavior. Local verification
defaults to the isolated test environment (`~/.dope-test`, `127.0.0.1:19192`). Live
Slack workspace authorization and production tenants are not required for automated
acceptance. Safe live Slack authorization, when explicitly used, is non-production or
test-workspace authorization scoped to a test tenant and selected test channels/users,
approved by an operator for validation, redacted in all evidence, and isolated from
normal production tenants.
**Project Type**: Multi-surface daemon connector feature spanning Slack provider
mechanics, hosted OAuth setup, connector conformance, IM routing/dedupe, diagnostics,
persistence, API/schema/event contracts, delivery adapter behavior, live validation, and
operator documentation.
**Performance Goals**: Slack fake-transport and conformance tests run as normal daemon
tests without live provider dependency. Duplicate provider deliveries produce at most one
agent run and one foreground reply. Authorized support inspection can identify latest
Slack health, diagnostic reason, remediation, freshness, workspace binding, selected
routes, and event-delivery status within 2 minutes. Setup completes or returns an
actionable terminal-state diagnostic within 5 minutes. Cached connector diagnostics
become stale after 15 minutes, and retained connector evidence expires from normal
inspection after 90 days unless an authorized longer retention policy applies.
**Constraints**: Slack setup uses hosted Slack app installation/OAuth only. Submitted raw
Slack bot tokens, signing secrets, and local-only credentials are unsupported setup
inputs. Each Slack connector binds exactly one workspace, and a tenant may create
multiple Slack connectors. Direct messages create runs only for explicitly allowed Slack
users or user groups. Channel messages create runs only for selected channels and agent
mentions or another explicitly supported invocation signal. Accepted channel mentions
reply in a thread rooted at the triggering message; DMs reply in the DM conversation.
Slack OAuth tokens, installation grants, authorization payloads, raw provider payloads,
credential-bearing payloads, and cross-tenant data must never appear in setup results,
diagnostics, events, fixtures, logs, support output, conformance evidence, or smoke
evidence.
**Scale/Scope**: One whole roadmap slice, Phase 51. Required coverage is Slack only,
using fake transport tests and provider-specific regression against the Roadmap 48 shared
conformance contract. WhatsApp/Matrix, future channel management UX, enterprise grid
administration, marketplace publication, voice huddles, memory-based team context, broad
media, and additional provider domains remain out of scope.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** - PASS. This plan closes Roadmap 51 as one production connector
  slice: hosted Slack app installation/OAuth setup, one-workspace-per-connector binding,
  multiple connectors per tenant, selected channel allowment, explicit DM user or
  user-group allowment, channel mention gating, workspace/conversation/message dedupe
  with event evidence, required channel thread replies, normal DM replies, background
  delivery target reuse, diagnostics, conformance proof, live smoke or structured skip,
  docs, and verification boundaries.
- **Production-grade, minimal, reversible change** - PASS. The change is additive around
  existing connector supervision, setup wizard OAuth, IM loop, store, diagnostics,
  delivery, API/schema/event, live validation, and docs surfaces. Rollback can disable
  Slack setup, ingress, and delivery eligibility while preserving existing channels and
  retained redacted Slack evidence.
- **Contracts and auditability** - PASS. Public contract implications are captured in
  [contracts/slack-channel-connector.md](./contracts/slack-channel-connector.md). Any
  API, schema, event, config, persistence, or docs changes must update fixtures and
  compatibility notes together.
- **Verification and observability** - PASS. Required verification covers setup terminal
  states, OAuth redaction, workspace cardinality, explicit route allowment, route
  outcomes, duplicate suppression, required thread replies, foreground/background
  delivery separation, diagnostics, freshness, retention, redaction, rate-limit and
  event-delivery behavior, live smoke skip, schema contracts, and operator docs.
- **Environment and secrets** - PASS. Local work defaults to `~/.dope-test` with fake
  OAuth installation evidence and fake transports. Real Slack authorization, live
  connectors, and production tenants are used only in an explicit live validation path
  that can structured-skip when safe workspace authorization is unavailable.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/036-slack-channel-connector/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── slack-channel-connector.md
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
│   │   └── slack/
│   │       ├── runtime.go           # setup/readiness projection, route outcomes,
│   │       │                        # capability declaration, diagnostic publication
│   │       ├── transport.go         # OAuth installation validation, event ingestion,
│   │       │                        # send/retry/rate-limit behavior
│   │       ├── destinations.go      # selected channel and DM allowment validation
│   │       ├── diagnostics.go       # Slack condition to shared reason mapping
│   │       ├── smoke.go             # safe live smoke or structured skip evidence
│   │       └── *_test.go            # fake transport and Slack regression coverage
│   ├── setupwizard/                 # hosted Slack app installation/OAuth lifecycle
│   ├── im/ and imtypes/             # inbound workspace/conversation/message identity,
│   │                                # dedupe, final reply, thread-root reply linkage
│   ├── delivery/                    # connector-backed Slack delivery adapter
│   ├── store/ and store/tenancy/     # SQLite setup, workspace binding, route policy,
│   │                                # dedupe, diagnostic, retention, tenant accessors
│   ├── api/                         # connector/setup/diagnostic/live-smoke projections
│   ├── events/                      # setup, health, route, reply, delivery, smoke events
│   ├── livevalidation/              # safe live Slack smoke or structured skip
│   └── contracts/                   # schema and fixture contract tests
├── go.mod
└── go.sum

schemas/
├── api/                             # Slack setup, workspace binding, route policy,
│                                    # capability, diagnostic, and smoke resources
└── events/                          # setup, diagnostic, route, reply, delivery,
                                     # conformance, and smoke events if exposed

sdk/ts/                              # update only if public API shape changes for clients
web/ and tui/                        # update only if operator setup/repair UI changes

docs/
├── channels/
│   ├── channel-connector-conformance.md
│   └── slack-channel-loop.md
└── runtime/                         # update if operator diagnostics, setup, or smoke
                                     # guidance changes
```

**Structure Decision**: Add Slack as a provider-specific connector in place rather than
creating a separate execution path. `connectors/slack` owns Slack provider mechanics,
workspace readiness, route policy, OAuth installation validation, event delivery, and
reply mechanics; shared `connectors` owns conformance and diagnostic vocabulary;
`setupwizard` owns hosted OAuth lifecycle; `im` owns message identity, dedupe, and
foreground reply separation; `delivery` owns connector-backed background delivery;
`store` owns tenant-scoped evidence and retention; `api`, `schemas`, and `events`
expose additive operator-visible state only as needed; docs record setup, routing,
diagnostics, smoke, and rollback behavior.

## Roadmap 51 Planning Contracts

The implementation plan MUST keep this artifact complete before `/speckit.tasks`:

- [contracts/slack-channel-connector.md](./contracts/slack-channel-connector.md)
  - Slack hosted app installation/OAuth setup terminal states, one-workspace-per-
    connector binding, multiple connectors per tenant, selected channel policy, explicit
    DM user or user-group allowment, channel mention gating, required channel thread
    replies, workspace/conversation/message dedupe with retained event identity,
    unsupported setup modes, unsupported Slack surfaces, diagnostic mappings, redaction
    and retention, conformance gates, live smoke or structured skip, compatibility and
    rollback expectations.

This artifact is a planning gate. Implementation is incomplete if Slack can become
ready, emit setup/diagnostic/conformance/smoke evidence, accept an inbound message, send
a foreground reply, or deliver a background notification without a contract row and
proving test.

## Migration And Rollback Plan

1. Add or extend contract/schema vocabulary for Slack connector kind, hosted OAuth setup
   terminal states, workspace binding, route policy, explicit DM allowment, route
   outcomes, diagnostic reasons, capability declarations, smoke evidence, and
   unsupported surfaces without changing existing Discord, Telegram, or generic
   connector behavior.
2. Add tenant-safe persistence/accessors for Slack setup attempts, OAuth installation
   evidence, one-workspace-per-connector binding, selected channels, explicit DM users
   or user groups, workspace/conversation/message dedupe identity, retained event
   evidence, thread reply outcomes, delivery linkage, diagnostics, conformance, smoke,
   redaction status, and retention expiry.
3. Add Slack hosted app installation/OAuth setup through the hosted setup wizard. Persist
   repairable terminal-state evidence (`ready`, `degraded`, `unavailable`, `cancelled`,
   `action-required`) without exposing OAuth tokens, installation grants, authorization
   payloads, or raw provider payloads.
4. Implement Slack runtime behind the shared connector supervisor and IM loop. Accept
   direct messages only from explicitly allowed Slack users or user groups, accept
   channel messages only from selected channels and agent mentions or explicitly
   supported invocation signals, record all other route decisions, and dedupe by
   workspace/conversation/message identity.
5. Map Slack provider failures into shared diagnostic states, freshness, redaction, and
   retention rules. Distinguish missing OAuth grant, missing or revoked installation,
   missing scopes, workspace approval required, channel access missing, rate limits,
   event-delivery failures, provider unavailable, network failure, blocked route,
   duplicate inbound, unsupported capability, reply failure, and unknown failure where
   evidence permits.
6. Add final-only foreground replies where channel mentions reply in a Slack thread
   rooted at the triggering message and DMs reply normally. Add connector-backed Slack
   background delivery adapter evidence while preserving foreground/background delivery
   separation.
7. Extend fake transport and Slack regression tests for setup, route outcomes, duplicate
   inbound, rate limits, event delivery, required thread replies, reply failures,
   unsupported setup modes, unsupported Slack surfaces, delivery separation, and live
   smoke structured skip.
8. Project Slack setup, repair, diagnostic, conformance, and smoke evidence through API,
   schemas, events, SDK/client surfaces, and docs only where existing product surfaces
   require it.

Rollback disables Slack setup, runtime ingress, and delivery-target eligibility while
retaining redacted Slack setup, workspace binding, route policy, diagnostic, route,
reply, delivery, conformance, and smoke evidence for authorized inspection until
retention expiry. Additive persistence columns or tables remain inert rather than being
destructively removed. Existing Discord, Telegram, and shared connector behavior must
remain unaffected.

## Phase 0 Research

Research decisions are recorded in [research.md](./research.md). No unresolved
clarifications remain.

## Phase 1 Design

Design artifacts:

- [data-model.md](./data-model.md)
- [contracts/slack-channel-connector.md](./contracts/slack-channel-connector.md)
- [quickstart.md](./quickstart.md)

### Contract Risk Review

Contract risks, ordered by severity:

1. Slack workspace installation can grant unintended agent access if all workspace
   members can DM the agent. The design requires explicit Slack user or user-group
   allowment before a DM can create an agent run.
2. Cross-tenant leakage can occur if a workspace binding is shared across tenants or a
   connector binds more than one workspace. The design allows multiple Slack connectors
   per tenant while each connector binds exactly one workspace.
3. Channel traffic can trigger public replies accidentally if selected channel membership
   alone is treated as consent. The design requires selected channels plus agent mention
   or another explicitly supported invocation signal, and channel replies are rooted in
   a Slack thread.
4. Duplicate suppression can be incorrect if Slack event IDs are used as the only
   identity. The design dedupes by workspace/conversation/message identity and retains
   event identity as redacted provider delivery evidence.
5. Slack diagnostics can leak OAuth tokens, installation grants, signing material, or raw
   provider payloads. The design uses hosted OAuth setup only, rejects submitted raw
   token setup, redacts all evidence by default, and suppresses detail when redaction
   confidence is insufficient.
6. Unsupported Slack surfaces and capabilities such as marketplace publication,
   enterprise grid administration, memory-based team context, files, voice clips,
   huddles, canvases, workflow buttons, and interactive blocks can imply unreviewed
   storage, admin, or callback behavior. The design makes every out-of-scope Slack
   surface or capability an explicit unsupported outcome.

Compatibility assessment: additive. Existing connector list/resource schemas, Discord
setup resources, Telegram setup resources, delivery resources, and IM loop behavior
remain backward compatible. Slack introduces new provider-specific resources or additive
fields as needed. Old clients continue to read existing connector resources and ignore
Slack-specific projections they do not understand.

Required validation cases: Slack hosted OAuth setup terminal states, missing/revoked
OAuth grant, missing scope, missing installation, workspace approval required, workspace
mismatch, one-workspace-per-connector cardinality, multiple connectors per tenant,
selected channel validation, explicit DM user or user-group allowment, blocked DM sender,
unselected channel, channel without mention ignored, selected channel mention accepted,
required channel thread reply success/failure, normal DM reply success/failure,
unsupported raw-token setup, unsupported marketplace publication, unsupported enterprise
grid administration, unsupported memory-based team context, unsupported files/voice/
huddles/canvases/workflow buttons/interactive blocks, workspace/conversation/message
dedupe after retry/reconnect/restart, retained event evidence, background delivery
success/failure/suppression, diagnostic
freshness/current-truth behavior, 90-day retention, redaction fail-closed behavior,
tenant isolation, schema contract tests, docs updates, live smoke success or structured
skip, and `go mod tidy`.

## Post-Design Constitution Check

- **Roadmap closure** - PASS. Research, data model, quickstart, and contract artifacts
  cover all Roadmap 51 gates: hosted Slack app installation/OAuth setup, workspace
  binding, explicit route policy, DM allowment, channel mention gating, dedupe, required
  thread replies, background delivery reuse, diagnostics, conformance proof, smoke or
  structured skip, compatibility, docs, and verification.
- **Production-grade, minimal, reversible change** - PASS. The design is additive and
  staged: contract vocabulary first, persistence/projections, Slack setup/runtime,
  diagnostic mapping, delivery adapter, fake transport evidence, public projections, and
  docs. Rollback disables Slack setup/runtime/delivery eligibility while preserving
  existing channels and retained evidence.
- **Contracts and auditability** - PASS. The contract defines setup terminal states,
  workspace cardinality, route policy, durable identity, diagnostic mappings, capability
  declarations, retention, redaction, live smoke evidence, compatibility, and
  verification gates. Schemas, docs, fixtures, and tests must change together.
- **Verification and observability** - PASS. Quickstart names targeted package tests,
  full daemon tests, schema contract tests, delivery separation checks, live-smoke skip
  evidence, redaction checks, retention checks, and optional client verification when
  public client surfaces change.
- **Environment and secrets** - PASS. Test environment and fake OAuth installation
  evidence remain the default. Live Slack authorization and production tenants are
  excluded from automated acceptance unless an operator explicitly chooses live
  validation, and all evidence is redacted.
