# Implementation Plan: Channel Connector Conformance

**Branch**: `033-channel-connector-conformance` | **Date**: 2026-05-07 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/033-channel-connector-conformance/spec.md`

## Summary

Close Roadmap 48 by turning hosted channel connector behavior into one shared, testable
conformance contract. The implementation extends the existing daemon connector, IM loop,
Discord runtime, delivery connector adapter, tenant store, API/schema, event, and docs
surfaces additively: fake connector conformance tests prove the core invariants, Discord
is the only required real connector regression baseline, and future Telegram, Slack, and
other provider specs consume the contract without re-defining routing, dedupe, reply,
diagnostic, redaction, or delivery-boundary rules.

The plan preserves current foreground Discord behavior while making missing contract
truth explicit: core invariants must pass, provider-specific surfaces may be supported,
limited, or unsupported, connector diagnostics follow the Roadmap 42 freshness and
retention rules, and inbound message identity becomes tenant + connector account +
channel/conversation + provider message ID with an explicit equivalent-rule escape hatch.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; JSON Schema contracts under
`schemas/`; TypeScript SDK and web/TUI clients update only if connector contract
projections become public client surface; operator/channel documentation under `docs/`.
**Primary Dependencies**: Existing `daemon/internal/connectors`, `daemon/internal/im`,
`daemon/internal/imtypes`, `daemon/internal/connectors/discord`,
`daemon/internal/delivery`, `daemon/internal/store`, `daemon/internal/store/tenancy`,
`daemon/internal/api`, `daemon/internal/events`, `daemon/internal/contracts`,
`schemas/api`, `schemas/events`, `docs/channels`, Roadmap 28 delivery truth, Roadmap 37
hosted credential isolation, and Roadmap 42 diagnostics freshness/redaction/retention.
**Storage**: Existing SQLite daemon metadata store remains authoritative. Additive
changes are expected for connector capability profiles, connector diagnostic/conformance
evidence, retention expiry, redaction-failure markers, and standard inbound message
identity fields. Existing `connector_messages` rows remain compatible while new rows
carry enough identity to dedupe by tenant, connector account, channel/conversation, and
provider message ID.
**Testing**: Targeted Go tests for fake connector conformance, Discord regression,
connector identity/dedupe, tenant isolation, permission denial, diagnostics freshness,
retention, redaction fail-closed behavior, foreground/background delivery separation,
API/schema contract tests, and docs/contract fixture checks; `go test ./...` from
`daemon/`; `make daemon-contract-test`; `pnpm test:clients` and `pnpm build` only if SDK,
web, or TUI surfaces change; `go mod tidy` from `daemon/` after implementation.
**Target Platform**: Local-first daemon and hosted daemon behavior, verified by default
in the isolated test environment (`~/.kura-test`, `127.0.0.1:19192`). Live connector
credentials and production tenants are not required for acceptance.
**Project Type**: Multi-surface daemon contract feature spanning connector runtime
contracts, IM routing and dedupe, Discord regression, delivery-boundary evidence,
persistence, API/schema contracts, event contracts, and operator documentation.
**Performance Goals**: Conformance fixtures run as normal daemon tests without requiring
live external providers. Connector diagnostic inspection can show cached state, marks it
stale after 15 minutes, and connector action failures produce current diagnostic truth
before remediation is shown.
**Constraints**: Core invariants are non-negotiable: tenant ownership, permission
gating, redaction, active-tenant account binding, inbound identity, durable dedupe,
stable routing decisions, minimum final-only foreground reply delivery for accepted
messages, required diagnostic classifications, and foreground/background delivery
separation. Provider-specific surfaces may be unsupported or limited only when explicit
and conformance-tested. Connector conformance and diagnostic evidence use 90-day default
retention unless an authorized longer retention policy applies.
**Scale/Scope**: One whole roadmap slice, Phase 48. Required coverage is shared fake
connector conformance plus Discord regression only; Telegram, Slack, and other
non-Discord connector implementations, stubs, or regressions are out of scope.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** - PASS. This plan closes Roadmap 48 as one shared hosted connector
  conformance contract: lifecycle states, account binding, inbound routing, dedupe,
  reply progression, diagnostics, redaction, delivery separation, fake connector matrix,
  Discord regression baseline, schemas/contracts, docs, and verification boundaries.
- **Production-grade, minimal, reversible change** - PASS. The change is additive around
  existing connector, IM, delivery, store, API, schema, and docs surfaces. Rollback can
  disable conformance gating/projections and leave current Discord foreground behavior
  and existing connector message history intact.
- **Contracts and auditability** - PASS. API/schema/event/config/persistence boundary
  changes are named in [contracts/channel-connector-conformance.md](./contracts/channel-connector-conformance.md).
  Contract tests and docs must change with any public shape, event, diagnostic, or
  persistence rule.
- **Verification and observability** - PASS. Required verification covers fake
  connector matrix cases, Discord regression, tenant isolation, durable dedupe,
  foreground/background separation, diagnostics classification, freshness, retention,
  redaction, contract fixtures, and operator docs.
- **Environment and secrets** - PASS. Local verification defaults to `~/.kura-test`.
  Fake connectors and fake credentials are the default. Live connectors, production
  tenants, real channel credentials, rich media, voice, and mobile push are out of
  acceptance scope unless explicitly chosen outside this spec.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/033-channel-connector-conformance/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── channel-connector-conformance.md
├── checklists/
│   └── requirements.md
└── tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── connectors/                  # shared conformance model, fake connector matrix,
│   │   └── discord/                  # Discord real connector regression baseline
│   ├── im/ and imtypes/              # inbound identity, routing, dedupe,
│   │                                 # reply progression degradation vocabulary
│   ├── delivery/                     # connector-backed background delivery separation
│   ├── store/ and store/tenancy/      # SQLite schema/accessors, tenant-safe evidence,
│   │                                 # retention expiry and migration fixtures
│   ├── api/                          # connector resource, diagnostic, conformance
│   │                                 # and ingress projections
│   ├── events/                       # connector conformance, diagnostic, ingress,
│   │                                 # foreground-reply, and delivery-boundary events
│   └── contracts/                    # schema and fixture contract tests
├── go.mod
└── go.sum

schemas/
├── api/                              # connector resources, capability profiles,
│                                      # diagnostic/conformance/account-binding resources
└── events/                           # connector conformance, diagnostic, ingress,
                                       # foreground-reply, and delivery-boundary events

sdk/ts/                               # update only if public connector contract
                                        # projections become client-facing
web/ and tui/                         # update only if operator UI surfaces change

docs/
├── channels/                         # connector conformance contract and Discord notes
└── runtime/                          # operator diagnostics, rollback, retention,
                                       # contract pipeline notes if schema changes
```

**Structure Decision**: Extend the existing connector and IM loop boundaries in place
instead of creating a separate conformance service. `connectors` owns capability,
diagnostic, and conformance vocabulary; `im` and `imtypes` own inbound identity, routing,
dedupe, and reply progression behavior; `delivery` proves foreground/background
separation; `store` owns tenant-scoped persistence and retention; `api`, `schemas`, and
`events` expose additive operator-visible projections; Discord remains the real
connector regression baseline.

## Roadmap 48 Planning Contracts

The implementation plan MUST keep this artifact complete before `/speckit.tasks`:

- [contracts/channel-connector-conformance.md](./contracts/channel-connector-conformance.md)
  - connector lifecycle states, core invariant matrix, provider-specific supported /
  limited / unsupported surfaces, inbound identity, routing outcomes, reply progression,
  diagnostics, redaction, retention, event/API/schema expectations, Discord regression
  gates, and delivery-boundary evidence.

This artifact is a planning gate. Implementation is incomplete if a connector state,
capability, diagnostic reason, routing decision, dedupe rule, reply progression level,
foreground/background delivery result, redaction behavior, retention rule, API/schema
shape, event, or Discord regression can exist without a contract row and proving test.

## Migration And Rollback Plan

1. Add contract/schema vocabulary for connector capability profiles, conformance results,
   diagnostics, core invariants, provider-specific surfaces, redaction status, freshness,
   and retention without changing live connector startup behavior.
2. Add tenant-safe persistence for conformance and diagnostic evidence. Add inbound
   identity fields or equivalent accessors so `connector_messages` can dedupe by tenant,
   connector account, channel/conversation, and provider message ID while preserving
   legacy rows and existing Discord message history.
3. Add fake connector conformance fixtures for pass, fail, limited, unsupported,
   duplicate, blocked-route, redaction-failure, stale-diagnostic, retention-expired, and
   delivery-boundary cases.
4. Adapt Discord to the shared contract meanings or record explicit unsupported/limited
   provider-specific surfaces. Keep Discord as the only required real connector
   regression for Phase 48.
5. Project conformance and diagnostic evidence through API/schema/event surfaces only
   after tenant isolation, permission denial, redaction, retention, and contract tests
   pass.
6. Update channel/operator docs so future Telegram, Slack, and other channel specs
   reference the shared contract instead of repeating shared behavior.

Rollback disables conformance gating/projections and diagnostic publication while keeping
existing Discord foreground replies, existing connector routes, and already-written
redacted conformance/diagnostic evidence readable to authorized operators until retention
expiry. Any additive persistence columns or tables remain inert rather than being
destructively removed.

## Phase 0 Research

Research decisions are recorded in [research.md](./research.md). No unresolved
clarifications remain.

## Phase 1 Design

Design artifacts:

- [data-model.md](./data-model.md)
- [contracts/channel-connector-conformance.md](./contracts/channel-connector-conformance.md)
- [quickstart.md](./quickstart.md)

### Contract Risk Review

Contract risks, ordered by severity:

1. Dedupe can double-reply or cross tenants if identity remains only
   `connector_id + direction + external_message_id`. The design requires tenant,
   connector account, channel/conversation, and provider message ID, with an explicit
   equivalent-rule path only when provider mechanics require it.
2. Unsupported capability declarations can hide safety failures. The contract separates
   core invariants, which must pass, from provider-specific surfaces, which may be
   supported, limited, or unsupported only with proving conformance evidence.
3. Foreground reply and background delivery can collapse if connector transport reuse is
   treated as one outcome. The contract requires separate records, events, and
   conformance cases for foreground reply outcomes and background delivery outcomes.
4. Diagnostics can leak provider secrets or stale remediation. The contract reuses the
   Roadmap 42 rules: stale after 15 minutes, current diagnostic truth on action failure,
   redaction fail-closed, and 90-day default retention.
5. Discord can remain a special case. The contract requires Discord regression to either
   pass core invariants or declare explicit unsupported/limited provider-specific
   surfaces, with no Telegram/Slack implementation expected in this phase.

Compatibility assessment: additive. Existing connector routes, Discord gateway behavior,
and connector message history remain valid. Public schemas and event shapes gain fields
or resources additively; old clients continue to read existing connector resources.

Required validation cases: schema conformance, fake connector matrix coverage, Discord
regression, tenant-scoped identity and permission denial, duplicate/retry suppression,
blocked group/room/thread outcomes, reply progression degradation, foreground/background
delivery separation, diagnostics freshness/current-truth behavior, 90-day retention,
redaction fail-closed behavior, and docs references for future connector phases.

## Post-Design Constitution Check

- **Roadmap closure** - PASS. Research, data model, quickstart, and contract artifacts
  cover all Roadmap 48 gates: fake connector conformance, Discord regression,
  tenant/account binding, routing, dedupe identity, reply progression, diagnostics,
  redaction, retention, foreground/background delivery separation, schemas/events, and
  docs for future channels.
- **Production-grade, minimal, reversible change** - PASS. The design is additive and
  staged: contract vocabulary first, persistence/accessors, fake conformance matrix,
  Discord adaptation, public projections, docs. Rollback disables the new projections and
  gating while preserving current Discord behavior and existing records.
- **Contracts and auditability** - PASS. The contract defines state vocabularies,
  request/resource/event expectations, persistence identity rules, redaction,
  freshness, retention, and verification gates. Schemas, docs, and tests must change
  together.
- **Verification and observability** - PASS. Quickstart names targeted package tests,
  full daemon tests, schema contract tests, contract fixtures, redaction checks,
  retention checks, and optional client verification when public client surfaces change.
- **Environment and secrets** - PASS. Test environment and fake connectors remain the
  default. Live connector credentials and production tenants are excluded from
  acceptance, and all evidence remains redacted.

No post-design violations require justification.

## Complexity Tracking

> Filled only when Constitution Check has unjustified violations. None for this plan.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    |            |                                     |
