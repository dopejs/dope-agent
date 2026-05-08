# Phase 0 Research: Telegram Channel Connector

## Decision: Add Telegram as a provider-specific connector that consumes shared channel contracts

**Rationale**: Roadmap 50 is a provider-specific channel slice after the shared phase 48
connector contract and phase 49 Discord hardening. Reusing connector supervision, hosted
setup, IM loop routing/dedupe, diagnostics, delivery, persistence, API/schema/event, and
contract-test boundaries keeps the change minimal while proving the second real channel
against the same invariants.

**Alternatives considered**:

- Build a separate Telegram execution service: rejected because it duplicates connector
  runtime, routing, diagnostics, delivery, and rollback boundaries.
- Generalize all future channels first: rejected because the shared conformance contract
  already provides the cross-channel abstraction needed for this phase.

## Decision: Use submitted bot-token setup with standard hosted setup terminal states

**Rationale**: Telegram Bot API behavior is sufficient for the first slice, and bot
tokens fit the hosted setup wizard's submitted-secret path. Reusing `ready`, `degraded`,
`unavailable`, `cancelled`, and `action-required` avoids a Telegram-specific setup
vocabulary and keeps UI, support, diagnostics, and retry behavior aligned with phase 46.

**Alternatives considered**:

- Define Telegram-specific terminal states: rejected because support and clients would
  need provider-specific readiness rules.
- Treat invalid setup as a generic failed state only: rejected because degraded,
  unavailable, action-required, and cancelled carry different remediation and retry
  behavior.

## Decision: Require explicit Telegram user/chat/group allowment before creating runs

**Rationale**: Possession of a Telegram bot link must not grant access to a tenant-owned
agent. Explicit allowment is the safest production default, preserves tenant boundaries,
and gives operators an auditable routing decision for blocked direct-message senders and
groups.

**Alternatives considered**:

- Accept any direct-message sender after setup: rejected because it silently broadens
  agent access and makes bot-link sharing a security boundary.
- Bind the first sender automatically: rejected because setup ownership and first-message
  timing can diverge and create hard-to-debug account binding mistakes.

## Decision: Require allowed group plus bot mention or command for group routing

**Rationale**: Telegram group traffic is noisy and visible to multiple participants.
Requiring both explicit group allowment and intentional mention/command gating prevents
accidental public agent activation while still supporting purposeful group workflows.

**Alternatives considered**:

- Allowed group alone accepts every text message: rejected because ordinary group chatter
  could create agent runs and replies.
- Command-only gating: rejected because bot mentions are a common, expected group
  interaction pattern and can remain testable under the same explicit gate.

## Decision: Dedupe by Telegram chat/message identity and retain update identity as evidence

**Rationale**: Chat/message identity represents the user-visible Telegram message that
must produce at most one agent run and one foreground reply. Telegram update identity is
still valuable as redacted provider delivery evidence for retries, reconnects, and
diagnostics, but it should not be the sole dedupe key for user-visible message handling.

**Alternatives considered**:

- Dedupe by update identity only: rejected because provider redelivery mechanics can make
  update identity too transport-oriented for user-visible duplicate suppression.
- Drop update identity after dedupe: rejected because support loses useful reconnect and
  provider-delivery evidence.

## Decision: Keep phase 50 text-only and make attachments/media explicit unsupported outcomes

**Rationale**: Attachments introduce storage, scanning, redaction, retention, and
delivery semantics that are not necessary to close the first Telegram connector slice.
Making attachments, voice, payments, mini apps, broad media transfer, and memory behavior
explicit unsupported outcomes keeps scope production-grade and testable.

**Alternatives considered**:

- Support basic attachment metadata only: rejected because users may infer media handling
  exists while actual download/redaction behavior remains incomplete.
- Support safe attachment download in phase 50: rejected because it expands the roadmap
  into media handling and retention work beyond the channel connector slice.

## Decision: Reuse shared connector diagnostic freshness, retention, and redaction rules

**Rationale**: Telegram should not create provider-specific policy for stale diagnostics
or evidence retention. Cached connector diagnostics become stale after 15 minutes,
failed actions must produce current diagnostic truth before remediation is shown, and
diagnostic/conformance/smoke/redaction-failure evidence expires from normal inspection
after 90 days by default.

**Alternatives considered**:

- Define Telegram-specific freshness and retention rules: rejected because it would
  diverge from phase 48 and make support behavior inconsistent across channels.
- Display raw Telegram provider errors for support: rejected because raw payloads can
  contain secret, tenant, chat, or user data and are not a stable public contract.

## Decision: Live Telegram smoke is optional but must produce explicit evidence

**Rationale**: Automated acceptance must not require real Telegram credentials. When safe
credentials exist, a live hosted/test smoke can prove end-to-end setup, routing, reply,
and delivery behavior. When they do not, the result must be a structured skip with owner,
reason, date, remaining risk, and redaction status so release review does not mistake
missing live validation for success.

**Alternatives considered**:

- Require live Telegram smoke for all acceptance: rejected because it would force real
  secrets into normal local verification.
- Omit live smoke entirely: rejected because public hosted readiness benefits from
  explicit live validation or a reviewable skip.

## Decision: Start with final-only foreground replies and separate background delivery outcomes

**Rationale**: Final-only foreground replies satisfy the shared channel contract and keep
Telegram rate-limit behavior simple for the first slice. Connector-backed background
delivery can reuse Telegram transport, but delivery truth must remain separate from
foreground reply truth and assistant execution truth.

**Alternatives considered**:

- Add thinking or incremental visible updates in phase 50: rejected because rate-limit
  and edit/update behavior would increase surface area without being required for the
  first Telegram connector.
- Treat foreground replies and background delivery as the same result: rejected because
  phase 28 and phase 48 require separate product truth.
