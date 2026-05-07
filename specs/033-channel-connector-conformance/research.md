# Phase 0 Research: Channel Connector Conformance

## Decision: Treat Phase 48 As A Shared Contract Plus Evidence, Not A New Connector

**Rationale**: The upstream Roadmap 48 document explicitly scopes the phase to a
connector conformance suite and excludes new channel connector implementation. Existing
Discord behavior is the compatibility baseline, while Telegram, Slack, and future
connectors consume the contract later.

**Alternatives considered**:

- Add one non-Discord connector stub now. Rejected because clarification fixed Phase 48
  coverage to fake connector conformance plus Discord regression only.
- Require Telegram and Slack regressions now. Rejected because that would convert the
  shared contract phase into provider implementation work.

## Decision: Separate Core Invariants From Provider-Specific Surfaces

**Rationale**: Hosted safety depends on tenant ownership, permission gating, redaction,
active-tenant account binding, inbound identity, durable dedupe, stable routing
decisions, minimum final-only foreground reply delivery, required diagnostics, and
foreground/background delivery separation. Provider-specific channel surfaces such as
rooms, threads, rich media, or incremental update mechanics vary across providers and can
be supported, limited, or unsupported when explicit.

**Alternatives considered**:

- Treat every listed contract area as mandatory. Rejected because it would block valid
  final-only or room-less providers without improving core hosted safety.
- Allow any area to be unsupported if documented. Rejected because it would weaken the
  safety contract and permit inconsistent tenant, dedupe, and redaction behavior.

## Decision: Standard Inbound Identity Is Tenant + Connector Account + Channel Or Conversation + Provider Message ID

**Rationale**: Current storage dedupes primarily by connector, direction, and external
message ID. Phase 48 needs provider-independent duplicate suppression that cannot
cross tenants, accounts, or conversations. The standard key matches the clarification
answer and leaves a narrow equivalent durable identity rule for providers that cannot
expose the exact fields.

**Alternatives considered**:

- Provider message ID only. Rejected because provider IDs may be scoped to a channel,
  account, or workspace and could collide across tenants.
- Connector-defined identity only. Rejected because it makes conformance too weak and
  hard to compare across providers.
- Synthetic identity when provider message ID is missing. Rejected as a default because
  synthetic keys are usually unstable across provider retries and restart replay.

## Decision: Reuse Roadmap 42 Diagnostic Freshness, Redaction, And Retention Rules

**Rationale**: Connector diagnostics are operator-facing readiness and failure truth like
integration diagnostics. Reusing the 15-minute stale threshold, current diagnostic truth
on action failure, redaction fail-closed behavior, and 90-day default retention avoids
parallel operator mental models and lets implementation reuse existing diagnostic
patterns.

**Alternatives considered**:

- Always-current diagnostics. Rejected because operator inspection can safely display
  cached state when marked stale and fresh checks may require external provider calls.
- Connector-defined freshness and retention. Rejected because inconsistent provider
  policies would undermine hosted operator readiness evidence.

## Decision: Keep Foreground Reply Outcomes Separate From Background Delivery Outcomes

**Rationale**: Roadmap 28 established delivery as distinct from foreground connector
chat replies. Connector-backed delivery may reuse channel transport mechanics, but it
must record chosen target, attempts, retry/suppression state, and terminal outcome
separately from active conversation reply state.

**Alternatives considered**:

- Treat a successful foreground reply as delivery success. Rejected because background
  work may use delivery targets without an active conversation.
- Reuse delivery outcomes for foreground replies. Rejected because foreground reply
  progression is tied to session/run/step truth and channel-specific visible progress.

## Decision: Use Fake Connector Matrix Tests As The Conformance Authority

**Rationale**: Fake connectors can deterministically cover pass, fail, duplicate,
blocked-route, limited, unsupported, redaction, retention, freshness, and delivery
boundary cases without live providers or secrets. Discord regression then proves the
existing real connector conforms to the same contract.

**Alternatives considered**:

- Rely on Discord-only regression. Rejected because it would leave future connectors
  without provider-neutral proof.
- Rely on documentation only. Rejected because Phase 48 completion requires proving
  behavior, not just describing it.
