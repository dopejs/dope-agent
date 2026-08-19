# Phase 0 Research: Matrix Channel Connector

## Decision: Add Matrix as a provider-specific connector that consumes shared channel contracts

**Rationale**: Roadmap 52 is the fourth real channel slice after Discord, Telegram, and
Slack. Reusing connector supervision, setup, IM loop routing/dedupe, diagnostics,
delivery, persistence, API/schema/event, and contract-test boundaries keeps the change
minimal while proving a materially different open-room provider against the same
invariants.

**Alternatives considered**:

- Build a separate Matrix execution service: rejected because it duplicates connector
  runtime, routing, diagnostics, delivery, and rollback boundaries.
- Generalize every federated room provider first: rejected because the shared
  conformance contract already provides the cross-channel abstraction needed for this
  phase.

## Decision: Choose Matrix over WhatsApp for phase 52

**Rationale**: Matrix has lower hosted-provider policy risk for this slice because it is
an open protocol with explicit client-server, room, membership, federation, and event
semantics. WhatsApp offers stronger personal mobile reach, but the safe hosted path
depends on business-account approval, phone-number ownership, conversation/template
policy, and provider commercial constraints that are less suitable for the fourth
connector slice.

**Alternatives considered**:

- WhatsApp Business Platform connector: rejected for phase 52 because provider approval,
  phone-number ownership, conversation windows, templates, and commercial policy add more
  hosted-risk coupling than Matrix for this roadmap.
- Implement both WhatsApp and Matrix: rejected because the upstream roadmap explicitly
  requires one channel, not both.

## Decision: Use tenant-provided Matrix bot accounts on tenant-selected homeservers

**Rationale**: Tenant-provided bot accounts keep account ownership, homeserver choice,
room membership, rollback, and credential responsibility in the tenant boundary while
allowing Kura to validate readiness and route policy. Kura-hosted homeserver
operation would add account lifecycle, federation, moderation, abuse, backup, and
operational responsibilities outside the roadmap.

**Alternatives considered**:

- Kura operates a shared Matrix homeserver and provisions accounts: rejected because
  it expands the roadmap into hosted Matrix infrastructure and moderation.
- Support both tenant-provided and Kura-hosted accounts: rejected because it doubles
  setup, diagnostics, rollback, and credential lifecycle paths before the simpler hosted
  connector path is proven.

## Decision: Treat Matrix Client-Server API concepts as the provider boundary

**Rationale**: Matrix client-server communication is based on authenticated JSON HTTP
APIs and supports room membership, `/sync`, message sending, event IDs, transaction IDs,
rate limits, and room power levels. Planning against these protocol concepts keeps the
connector compatible with tenant-selected homeservers without committing to one
homeserver implementation.

**Alternatives considered**:

- Depend on a single homeserver implementation: rejected because tenants may select
  different Matrix homeservers and the connector should classify unsupported behavior
  rather than assume one server.
- Use bridge-specific automation or non-standard bot behavior: rejected because it would
  weaken hosted viability and conformance evidence.

Primary references:

- Matrix Client-Server API latest specification: https://spec.matrix.org/latest/client-server-api/index.html
- Matrix message event, event ID, transaction ID, membership, rate limit, power level,
  and E2EE sections in the official Matrix specification.

## Decision: Require unencrypted text-only Matrix ingress in phase 52

**Rationale**: Text-only unencrypted messages are enough to prove the fourth connector
slice while keeping key management, device trust, undecryptable-event recovery, encrypted
attachments, and secret storage out of scope. Encrypted rooms and undecryptable events
must produce explicit unsupported outcomes so users and operators do not assume E2EE
support exists.

**Alternatives considered**:

- Support encrypted rooms when the bot can decrypt: rejected because partial E2EE support
  still introduces key/session lifecycle, device trust, and redaction risks.
- Full encrypted-room support: rejected because it is a separate roadmap-scale feature.
- Accept rich media metadata only: rejected because users may infer media handling exists
  while download, retention, and redaction remain incomplete.

## Decision: Require allowed-room bot mention or configured command for Matrix room routing

**Rationale**: Room membership alone is too broad to represent consent to create agent
runs. Requiring both tenant-allowed room policy and a bot mention or configured command
prevents accidental activation in shared rooms and matches the safety pattern from
Telegram group routing and Slack channel routing.

**Alternatives considered**:

- Accept every message in an allowed Matrix room: rejected because ordinary room chatter
  could create agent runs and public replies.
- Disable Matrix room routing entirely: rejected because open federated rooms are a key
  reason to choose Matrix.
- Mention-only without tenant-selected room allowment: rejected because it weakens route
  control and tenant ownership.

## Decision: Dedupe by homeserver, room/direct conversation, and Matrix event ID

**Rationale**: Matrix event ID is the stable event identity for user-visible messages,
but pairing it with tenant-selected homeserver and room/direct conversation keeps tenant
and route boundaries explicit. Sync batch and transaction identities are useful delivery
evidence for replay, retry, and support inspection, but they are not the canonical
duplicate suppression key.

**Alternatives considered**:

- Dedupe by event ID only: rejected because it weakens explicit tenant, homeserver, and
  room route boundaries.
- Dedupe by sync batch or transaction identity: rejected because transport delivery
  mechanics can diverge from the user-visible event identity.
- Dedupe by sender and timestamp window: rejected because it risks suppressing legitimate
  messages or accepting duplicate event replays.

## Decision: Keep final-only foreground replies and connector-backed background delivery

**Rationale**: Final-only replies satisfy the shared connector contract and keep Matrix
send/retry/rate-limit behavior focused for the first slice. Connector-backed background
delivery is in scope only when destination eligibility is explicit and delivery truth
remains separate from foreground replies and assistant execution.

**Alternatives considered**:

- Add incremental Matrix edits or reactions: rejected because edit semantics,
  rate-limit behavior, and user-visible progression require separate acceptance and
  rollback decisions.
- Support broad rich replies, reactions, or media: rejected because those surfaces add
  storage, redaction, and capability complexity outside phase 52.

## Decision: Reuse shared connector diagnostic freshness, retention, and redaction rules

**Rationale**: Matrix should not create provider-specific policy for stale diagnostics or
evidence retention. Cached connector diagnostics become stale after 15 minutes, failed
actions must produce current diagnostic truth before remediation is shown, and
diagnostic/conformance/smoke/redaction-failure evidence expires from normal inspection
after 90 days by default.

**Alternatives considered**:

- Define Matrix-specific freshness and retention rules: rejected because it would diverge
  from phase 48 and make support behavior inconsistent across channels.
- Display raw Matrix provider errors for support: rejected because raw payloads can
  contain access tokens, homeserver, room, user, event, message, or tenant data and are
  not a stable public contract.

## Decision: Live Matrix smoke is optional but must produce explicit evidence

**Rationale**: Automated acceptance must not require real Matrix credentials. When safe
credentials exist, a live test smoke can prove setup, routing, reply, delivery, and
diagnostic behavior. When they do not, the result must be a structured skip with owner,
reason, date, remaining risk, and redaction status so release review does not mistake
missing live validation for success.

**Alternatives considered**:

- Require live Matrix smoke for all acceptance: rejected because it would force real
  external authorization into normal local verification.
- Omit live smoke entirely: rejected because public hosted readiness benefits from
  explicit live validation or a reviewable skip.
