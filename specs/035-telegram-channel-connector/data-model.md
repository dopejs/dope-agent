# Data Model: Telegram Channel Connector

## Telegram Hosted Setup

Represents tenant-owned Telegram setup progress and connector readiness.

**Fields**:

- `tenantId`: owning tenant.
- `connectorId`: stable Telegram connector identifier.
- `connectorKind`: always `telegram`.
- `displayName`: redacted tenant-visible connector label.
- `status`: shared connector lifecycle status such as `configured`, `healthy`,
  `degraded`, `failed`, `permission_blocked`, `rate_limited`, or
  `unsupported_capability`.
- `terminalState`: hosted setup terminal state: `ready`, `degraded`, `unavailable`,
  `cancelled`, or `action-required`.
- `credentialState`: `missing`, `submitted`, `valid`, `invalid`, `revoked`, or
  `redaction_suppressed`.
- `allowmentState`: `none`, `partial`, `valid`, or `stale`.
- `groupBehavior`: `disabled` or `mention_or_command_required`.
- `deliveryEligible`: whether the connector can be selected for background delivery.
- `createdAt`, `updatedAt`, `validatedAt`: setup lifecycle timestamps.
- `retentionExpiresAt`: normal inspection expiry for retained setup evidence.
- `redactionStatus`: `redacted`, `suppressed`, or `redaction_failed`.

**Validation rules**:

- Token values and credential-bearing payloads are never stored in this entity.
- `ready` requires a valid bot account binding and at least one explicit valid Telegram
  user/chat/group allowment that matches the enabled behavior.
- `degraded`, `unavailable`, and `action-required` must include remediation-bearing
  diagnostic linkage.
- `cancelled` preserves redacted audit evidence and must not delete unrelated connector
  state.

**State transitions**:

```text
submitted -> ready
submitted -> degraded
submitted -> unavailable
submitted -> action-required
submitted -> cancelled
ready -> degraded
ready -> unavailable
degraded -> ready
degraded -> action-required
unavailable -> ready
action-required -> ready
ready -> cancelled
```

## Telegram Account Binding

Represents the tenant-scoped relationship between the tenant and Telegram bot identity.

**Fields**:

- `tenantId`
- `connectorId`
- `connectorAccountId`: bot account identifier, redacted or hashed where exposed.
- `providerAccountLabel`: redacted bot username or label.
- `permissionState`: `valid`, `missing_permission`, `rate_limited`,
  `provider_unavailable`, `network_failed`, or `unknown`.
- `validatedAt`
- `redactionStatus`
- `safeEvidence`: non-secret account validation summary.

**Relationships**:

- One Telegram Hosted Setup has one active Telegram Account Binding.
- One Telegram Account Binding has many Telegram Allowment records.
- Inbound messages, diagnostics, conformance evidence, reply outcomes, and delivery
  outcomes reference the account binding by tenant and connector account identity.

## Telegram Allowment

Represents an explicitly allowed Telegram user, direct chat, or group context.

**Fields**:

- `tenantId`
- `connectorId`
- `allowmentId`
- `telegramScopeType`: `user`, `direct_chat`, or `group`.
- `telegramScopeId`: redacted provider identifier or stable hash.
- `providerLabel`: optional redacted username, chat label, or group title.
- `enabled`: whether the allowment can accept ingress.
- `groupGate`: `not_applicable` for users/direct chats or `mention_or_command_required`
  for groups.
- `validationState`: `valid`, `invalid`, `blocked`, `stale`,
  `missing_permission`, or `not_found`.
- `reasonCode`: shared diagnostic reason when invalid.
- `createdAt`, `updatedAt`, `validatedAt`
- `redactionStatus`
- `safeEvidence`: non-secret provider context such as redacted label, scope type, or
  validation timestamp.

**Validation rules**:

- Direct-message routing requires an enabled `user` or `direct_chat` allowment.
- Group routing requires an enabled `group` allowment plus a bot mention or command in
  the inbound text.
- Missing allowment fails closed as `blocked_route`.
- Allowment evidence must suppress raw provider payloads when redaction confidence is
  insufficient.

## Telegram Inbound Message

Represents a normalized inbound Telegram update handled by the IM loop.

**Fields**:

- `tenantId`
- `connectorId`
- `connectorAccountId`
- `telegramChatId`: redacted or hashed chat identity where exposed.
- `telegramMessageId`: provider message identity within the chat.
- `telegramUpdateId`: retained as redacted provider delivery evidence.
- `senderId`: redacted or hashed sender identity where exposed.
- `conversationType`: `direct` or `group`.
- `textKind`: `text`, `command`, `mention_text`, or `unsupported`.
- `routingOutcome`: `accepted`, `ignored`, `blocked`, `duplicate`, `unsupported`, or
  `failed`.
- `reasonCode`: `blocked_route`, `mention_required`, `duplicate_inbound`,
  `unsupported_capability`, or other shared route reason.
- `receivedAt`
- `redactionStatus`

**Validation rules**:

- Durable dedupe identity is tenant, connector account, Telegram chat ID, and Telegram
  message ID.
- Telegram update ID is retained as delivery evidence but is not the sole dedupe key.
- Attachments, voice, payments, mini apps, media transfer, and unsupported provider
  surfaces produce `unsupported` and cannot create runs.
- Missing tenant/account/chat/message identity fails closed unless an explicit equivalent
  durable identity rule is documented and conformance-tested.
- Bot mention or command artifacts are normalized before assistant handling when a group
  message is accepted.

## Telegram Reply Outcome

Represents the foreground Telegram reply result for an accepted inbound message.

**Fields**:

- `tenantId`
- `connectorId`
- `inboundMessageIdentity`
- `assistantExecutionOutcome`: `succeeded`, `failed`, or `cancelled`.
- `telegramReplyOutcome`: `sent`, `failed`, or `not_attempted`.
- `replyProgressionLevel`: `final_only` for phase 50, or `unsupported` for optional
  progression surfaces.
- `failureReasonCode`: optional diagnostic reason.
- `attemptedAt`, `completedAt`
- `redactionStatus`
- `safeEvidence`

**Validation rules**:

- Assistant execution outcome and Telegram reply outcome remain separate.
- Accepted foreground messages require at least final-only reply delivery attempt.
- Reply failure produces operator-visible diagnostic evidence without raw provider
  payloads.

## Telegram Delivery Outcome

Represents the background notification result for scheduled or workflow-originated work
delivered through Telegram.

**Fields**:

- `tenantId`
- `deliveryTargetId`
- `connectorId`
- `connectorAccountId`
- `telegramScopeType`: `direct_chat`, `user`, or `group`.
- `telegramScopeId`: redacted provider identifier or stable hash.
- `deliveryOutcome`: `sent`, `retrying`, `suppressed`, `failed`, or `not_attempted`.
- `failureReasonCode`: optional diagnostic reason.
- `attemptedAt`, `completedAt`
- `redactionStatus`
- `safeEvidence`

**Validation rules**:

- Background delivery outcome is tracked independently from foreground reply outcome and
  assistant execution outcome.
- Delivery eligibility requires a valid Telegram setup and an enabled destination
  allowment.
- Failed delivery produces diagnostic evidence without changing the execution result of
  the scheduled or workflow-originated work.

## Telegram Diagnostic State

Represents supportable, redacted Telegram setup or runtime diagnostic evidence.

**Fields**:

- `diagnosticStateId`
- `tenantId`
- `connectorId`
- `connectorAccountId`
- `status`
- `reasonCode`: `auth_missing`, `permission_missing`, `rate_limited`,
  `provider_unavailable`, `network_failed`, `unsupported_capability`, `blocked_route`,
  `duplicate_inbound`, `reply_failed`, or `unknown_connector_failure`, with Telegram
  provider-specific subreasons in safe evidence when redaction permits.
- `remediationOwner`: `product_user`, `tenant_admin`, `operator`, `provider`, or
  `none_required`.
- `userVisibleSeverity`: `info`, `warning`, or `error`.
- `retrySafety`: `no_action_needed`, `retryable`, `retry_after`, `blocked`, or `unsafe`.
- `evidenceTimestamp`
- `freshnessState`: `fresh` or `stale`.
- `retentionExpiresAt`
- `redactionStatus`
- `safeEvidence`
- `redactionFailureId`

**Validation rules**:

- Cached diagnostics older than 15 minutes are stale.
- Failed Telegram actions produce current diagnostic truth before remediation is shown.
- Evidence expires from normal inspection after 90 days unless a longer authorized
  retention policy applies.

## Telegram Capability Profile

Represents Telegram's conformance declaration.

**Fields**:

- `profileId`
- `tenantId`
- `connectorId`
- `connectorKind`: `telegram`
- `coreInvariantResults`: pass/fail results for phase 48 core invariants.
- `providerSurfaceResults`: supported/limited/unsupported results for Telegram direct
  messages, group messages, mention gating, command gating, final-only replies,
  connector-backed delivery, attachments, voice, payments, mini apps, media transfer,
  thinking, and incremental visible updates.
- `declaredAt`

**Validation rules**:

- Hosted-ready requires all core invariants to pass.
- Unsupported optional surfaces must not weaken core invariants.
- Attachments, voice, payments, mini apps, media transfer, memory behavior, thinking, and
  incremental visible updates remain unsupported for phase 50 unless explicitly recut.

## Telegram Smoke Evidence

Represents live hosted/test validation when safe credentials exist, or the structured
skip when they do not.

**Fields**:

- `smokeEvidenceId`
- `tenantId`
- `connectorId`
- `status`: `passed`, `failed`, or `skipped`.
- `credentialMode`: `safe_live`, `fake`, or `unavailable`.
- `owner`
- `reason`
- `remainingRisk`
- `validatedAt`
- `retentionExpiresAt`
- `redactionStatus`
- `safeEvidence`

**Validation rules**:

- Live smoke never runs implicitly against production tenants or unapproved credentials.
- Skip evidence includes owner, reason, date, remaining risk, and redaction status.
- Smoke evidence follows the same 90-day default retention and redaction rules.
