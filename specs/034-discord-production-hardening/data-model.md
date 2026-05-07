# Data Model: Discord Production Hardening

## Discord Hosted Setup

Represents tenant-owned Discord setup progress and hosted readiness.

**Fields**:

- `tenantId`: owning tenant.
- `connectorId`: stable Discord connector identifier.
- `connectorKind`: always `discord`.
- `displayName`: redacted tenant-visible connector label.
- `status`: `configured`, `healthy`, `degraded`, `failed`, `permission_blocked`,
  `rate_limited`, or `unsupported_capability`.
- `readinessState`: `hosted_ready`, `degraded_needs_repair`, `failed`, or `disabled`.
- `credentialState`: `missing`, `submitted`, `valid`, `invalid`, `revoked`, or
  `redaction_suppressed`.
- `respondInDM`: whether direct messages are accepted.
- `requireMention`: whether guild messages require bot mention.
- `deliveryMode`: `gateway` for phase 49.
- `createdAt`, `updatedAt`, `validatedAt`: setup lifecycle timestamps.
- `redactionStatus`: `redacted`, `suppressed`, or `redaction_failed`.

**Validation rules**:

- Token values and credential-bearing payloads are never stored in this entity.
- Hosted readiness requires valid credentials and explicit validated destinations.
- Authentic credentials with partially invalid or missing explicit destinations save as
  `degraded_needs_repair` and cannot become `hosted_ready`.
- Existing local config projection may remain usable even when hosted readiness is not
  satisfied.

**State transitions**:

```text
disabled -> configured -> degraded_needs_repair -> hosted_ready
configured -> failed
hosted_ready -> degraded_needs_repair
hosted_ready -> failed
degraded_needs_repair -> hosted_ready
failed -> configured
```

## Discord Destination Validation

Represents validation of a selected Discord guild, channel, or direct-message behavior.

**Fields**:

- `tenantId`
- `connectorId`
- `destinationId`: redacted provider destination identifier or stable hash.
- `destinationType`: `guild`, `channel`, or `direct_message`.
- `providerLabel`: optional redacted display label.
- `selected`: whether tenant selected this destination for hosted setup.
- `validationState`: `valid`, `invalid`, `missing_permission`,
  `message_content_missing`, `bot_not_member`, `not_found`, `dm_restricted`, or `stale`.
- `reasonCode`: shared diagnostic reason when invalid.
- `validatedAt`
- `redactionStatus`
- `safeEvidence`: non-secret provider context such as redacted guild/channel label,
  missing permission class, or last validation timestamp.

**Validation rules**:

- Hosted setup with no explicit guild or channel destination selection is
  `degraded_needs_repair`.
- Each selected destination must validate before hosted-ready status.
- Evidence must suppress raw provider payloads when redaction confidence is insufficient.

## Discord Account Binding

Represents the tenant-scoped relationship between the tenant and Discord bot account.

**Fields**:

- `tenantId`
- `connectorId`
- `connectorAccountId`: bot account identifier, redacted or hashed where exposed.
- `providerAccountLabel`: redacted bot label.
- `permissionState`: `valid`, `missing_permission`, `message_content_missing`,
  `rate_limited`, `provider_unavailable`, or `unknown`.
- `redactionStatus`
- `validatedAt`

**Relationships**:

- One Discord Hosted Setup has one active Discord Account Binding.
- One Discord Account Binding has many Destination Validation records.
- Inbound messages, diagnostics, conformance evidence, and smoke evidence reference the
  account binding by tenant and connector account identity.

## Discord Inbound Message

Represents a normalized inbound Discord message handled by the IM loop.

**Fields**:

- `tenantId`
- `connectorId`
- `connectorAccountId`
- `channelOrConversationId`
- `providerMessageId`
- `externalMessageId`
- `guildId`
- `channelId`
- `authorId`
- `direct`
- `mentioned`
- `routingOutcome`: `accepted`, `ignored`, `blocked`, `duplicate`, `unsupported`, or
  `failed`.
- `reasonCode`: `direct_message_disabled`, `blocked_guild`, `blocked_channel`,
  `mention_required`, `duplicate_inbound`, or other shared route reason.
- `receivedAt`
- `redactionStatus`

**Validation rules**:

- Durable dedupe identity is tenant, connector account, channel/conversation, and provider
  message ID.
- Missing tenant/account/channel/provider identity fails closed unless an explicit
  equivalent durable identity rule is documented and conformance-tested.
- Discord mention artifacts are removed before assistant handling.

## Discord Reply Outcome

Represents the foreground Discord reply result for an accepted inbound message.

**Fields**:

- `tenantId`
- `connectorId`
- `inboundMessageIdentity`
- `assistantExecutionOutcome`: `succeeded`, `failed`, or `cancelled`.
- `discordDeliveryOutcome`: `sent`, `edited`, `failed`, `degraded_to_final_only`, or
  `not_attempted`.
- `replyProgressionLevel`: `final_only`, `thinking_plus_final`,
  `thinking_plus_incremental`, or `unsupported`.
- `failureReasonCode`: optional diagnostic reason.
- `attemptedAt`, `completedAt`
- `redactionStatus`

**Validation rules**:

- Assistant execution outcome and Discord delivery outcome remain separate.
- Unsafe or rate-limited progression must degrade before it produces excessive edits.
- Reply failure produces operator-visible diagnostic evidence.

## Discord Diagnostic State

Represents supportable, redacted Discord setup or runtime diagnostic evidence.

**Fields**:

- `diagnosticStateId`
- `tenantId`
- `connectorId`
- `connectorAccountId`
- `status`
- `reasonCode`: `auth_missing`, `permission_missing`, `rate_limited`,
  `provider_unavailable`, `network_failed`, `unsupported_capability`, `blocked_route`,
  `duplicate_inbound`, `reply_failed`, or `unknown_connector_failure`, with Discord
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
- Failed Discord actions produce current diagnostic truth before remediation is shown.
- Evidence expires from normal inspection after 90 days unless a longer authorized
  retention policy applies.

## Discord Capability Profile

Represents Discord's conformance declaration.

**Fields**:

- `profileId`
- `tenantId`
- `connectorId`
- `connectorKind`: `discord`
- `coreInvariantResults`: pass/fail results for phase 48 core invariants.
- `providerSurfaceResults`: supported/limited/unsupported results for Discord direct
  messages, group channels, mention gating, threads, rooms, rich media, thinking,
  incremental visible updates, final-only replies, and connector-backed delivery.
- `declaredAt`

**Validation rules**:

- Hosted-ready requires all core invariants to pass.
- Unsupported optional surfaces must not weaken core invariants.
- Discord voice, broad rich media, app marketplace listing, memory-based thread recall,
  and broad multi-channel abstractions remain out of scope.

## Discord Smoke Evidence

Represents live hosted/test validation when safe credentials exist, or the structured skip
when they do not.

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
- Skip evidence includes owner, reason, date, and remaining risk.
- Smoke evidence follows the same 90-day default retention and redaction rules.
