# Data Model: Matrix Channel Connector

## Matrix Hosted Setup

Represents tenant-owned Matrix setup progress and connector readiness for one connector
using a tenant-provided bot account on a tenant-selected homeserver.

**Fields**:

- `tenantId`: owning tenant.
- `connectorId`: stable Matrix connector identifier.
- `connectorKind`: always `matrix`.
- `displayName`: redacted tenant-visible connector label.
- `homeserverBindingId`: active homeserver/bot binding for the connector.
- `status`: shared connector lifecycle status such as `configured`, `healthy`,
  `degraded`, `failed`, `permission_blocked`, `rate_limited`, or
  `unsupported_capability`.
- `terminalState`: hosted setup terminal state: `ready`, `degraded`, `unavailable`,
  `cancelled`, or `action-required`.
- `botCredentialState`: `not_started`, `submitted`, `valid`, `invalid`, `revoked`,
  `permission_missing`, `redaction_suppressed`, or `unknown`.
- `homeserverState`: `reachable`, `unreachable`, `unsupported`, `rate_limited`,
  `federation_failed`, `network_failed`, or `unknown`.
- `routePolicyState`: `none`, `partial`, `valid`, `stale`, or `blocked`.
- `deliveryEligible`: whether the connector can be selected for background delivery.
- `createdAt`, `updatedAt`, `validatedAt`: setup lifecycle timestamps.
- `retentionExpiresAt`: normal inspection expiry for retained setup evidence.
- `redactionStatus`: `redacted`, `suppressed`, or `redaction_failed`.

**Validation rules**:

- Matrix access tokens, credential-bearing payloads, and raw provider payloads are never
  stored in this entity.
- `ready` requires valid bot authorization, reachable tenant-selected homeserver,
  validated account binding, passing conformance gates, and at least one valid direct or
  room route policy.
- DopeAgent-hosted homeserver provisioning and Matrix account provisioning cannot
  transition setup to `ready` in phase 52.
- `degraded`, `unavailable`, and `action-required` must include remediation-bearing
  diagnostic linkage.
- `cancelled` preserves redacted audit evidence and must not delete unrelated connector
  state.

**State transitions**:

```text
not-started -> credential-submitted
credential-submitted -> ready
credential-submitted -> degraded
credential-submitted -> unavailable
credential-submitted -> action-required
credential-submitted -> cancelled
ready -> degraded
ready -> unavailable
ready -> action-required
degraded -> ready
degraded -> action-required
unavailable -> ready
action-required -> ready
ready -> cancelled
```

## Matrix Homeserver Binding

Represents the tenant-scoped relationship between one Matrix connector, one
tenant-selected homeserver, and one tenant-provided Matrix bot account.

**Fields**:

- `tenantId`
- `connectorId`
- `homeserverBindingId`
- `homeserverUrl`: redacted or normalized homeserver origin where exposed.
- `homeserverName`: redacted homeserver label or stable hash where exposed.
- `botUserId`: redacted Matrix user ID or stable hash where exposed.
- `botDeviceId`: redacted device identifier or stable hash where exposed when available.
- `authorizationState`: `valid`, `missing`, `revoked`, `permission_missing`,
  `ownership_mismatch`, `provider_unavailable`, `network_failed`, or `unknown`.
- `homeserverCapabilityState`: `valid`, `unsupported`, `stale`, `rate_limited`, or
  `unknown`.
- `validatedAt`
- `redactionStatus`
- `safeEvidence`: non-secret homeserver and bot validation summary.

**Relationships**:

- One Matrix Hosted Setup has one active Matrix Homeserver Binding.
- One tenant may have many Matrix connectors, each with one active homeserver/bot
  binding.
- One Matrix Homeserver Binding has one connector-specific Matrix Route Policy.
- Inbound events, diagnostics, conformance evidence, reply outcomes, and delivery
  outcomes reference the binding by tenant, connector, homeserver, and bot account
  identity.

**Validation rules**:

- A connector cannot bind more than one active tenant-selected homeserver/bot account.
- A homeserver/bot binding cannot be used for another tenant route.
- Homeserver mismatch, ambiguous account ownership, invalid bot authorization, or
  unsupported homeserver behavior blocks readiness and ingress.

## Matrix Route Policy

Represents direct and room routing authorization for a single Matrix connector.

**Fields**:

- `tenantId`
- `connectorId`
- `homeserverBindingId`
- `allowedDirectUsers`: redacted Matrix user identities or stable hashes.
- `selectedRooms`: redacted room identities, aliases, labels, and validation states.
- `roomInvocationGate`: `bot_mention_or_command_required` for room messages in phase 52.
- `configuredCommands`: tenant-visible command labels or redacted command identifiers.
- `encryptedRoomPolicy`: `unsupported`.
- `validationState`: `valid`, `partial`, `stale`, `blocked`, or `missing_permission`.
- `reasonCode`: shared diagnostic reason when invalid.
- `createdAt`, `updatedAt`, `validatedAt`
- `redactionStatus`
- `safeEvidence`: non-secret route validation summary.

**Validation rules**:

- Direct routing requires an enabled direct-user route or another explicit tenant-owned
  direct-conversation allowment.
- Room routing requires an enabled selected room plus bot mention or configured command.
- Encrypted rooms and undecryptable events are unsupported and cannot create runs.
- Missing direct allowment or room allowment fails closed as `blocked_route` or
  `action-required` depending on setup context.
- Route evidence must suppress raw event content and provider payloads when redaction
  confidence is insufficient.

## Matrix Conversation

Represents an unencrypted direct or unencrypted room context under a Matrix homeserver
binding.

**Fields**:

- `tenantId`
- `connectorId`
- `homeserverBindingId`
- `homeserverId`: redacted homeserver identity or stable hash where exposed.
- `conversationId`: redacted direct conversation or room identity where exposed.
- `conversationType`: `direct_message` or `room`.
- `roomSelectionState`: `selected`, `not_selected`, `stale`, `left`, `banned`,
  `missing_membership`, `encrypted_unsupported`, or `not_applicable`.
- `invocationGate`: `not_applicable`, `bot_mention_required`,
  `configured_command_required`, or `bot_mention_or_command_required`.
- `redactionStatus`
- `safeEvidence`

**Validation rules**:

- Room conversations must be selected before accepted ingress or background delivery
  eligibility can use them.
- Direct conversations require explicit allowment.
- Left, banned, inaccessible, encrypted, or missing-membership rooms fail closed until
  repaired or produce unsupported outcomes.

## Matrix Inbound Event

Represents a normalized inbound Matrix event handled by the IM loop.

**Fields**:

- `tenantId`
- `connectorId`
- `homeserverBindingId`
- `homeserverId`: redacted or hashed homeserver identity where exposed.
- `conversationId`: redacted or hashed room or direct conversation identity where
  exposed.
- `matrixEventId`: provider event identity.
- `syncBatchId`: retained as redacted provider delivery evidence when available.
- `transactionId`: retained as redacted provider delivery evidence when available.
- `senderId`: redacted or hashed Matrix sender identity where exposed.
- `senderAllowmentType`: `explicit_user`, `allowed_room_mention`,
  `allowed_room_command`, or `none`.
- `conversationType`: `direct_message` or `room`.
- `messageKind`: `unencrypted_text`, `encrypted_unsupported`,
  `undecryptable_unsupported`, `unsupported`, or `unknown`.
- `invocationKind`: `direct`, `bot_mention`, `configured_command`, `none`, or
  `unsupported`.
- `routingOutcome`: `accepted`, `ignored`, `blocked`, `duplicate`, `unsupported`, or
  `failed`.
- `reasonCode`: `blocked_route`, `mention_required`, `duplicate_inbound`,
  `unsupported_capability`, or other shared route reason.
- `receivedAt`
- `redactionStatus`

**Validation rules**:

- Durable dedupe identity is tenant, connector, tenant-selected homeserver, room or
  direct conversation, and Matrix event ID.
- Sync batch and transaction identity are retained as delivery evidence but are not the
  canonical dedupe key.
- Encrypted rooms, undecryptable events, media, voice, calls, reactions, bridge-specific
  metadata, and unsupported Matrix surfaces produce `unsupported` and cannot create runs.
- Missing tenant/homeserver/conversation/event identity fails closed unless an explicit
  equivalent durable identity rule is documented and conformance-tested.
- Bot mention or command artifacts are normalized before assistant handling when a room
  message is accepted.

## Matrix Reply Outcome

Represents the foreground Matrix reply result for an accepted inbound event.

**Fields**:

- `tenantId`
- `connectorId`
- `inboundEventIdentity`
- `assistantExecutionOutcome`: `succeeded`, `failed`, or `cancelled`.
- `matrixReplyOutcome`: `sent`, `failed`, or `not_attempted`.
- `replyProgressionLevel`: `final_only` for phase 52, or `unsupported` for optional
  progression surfaces.
- `replyContext`: `direct_message` or `room`.
- `failureReasonCode`: optional diagnostic reason.
- `attemptedAt`, `completedAt`
- `redactionStatus`
- `safeEvidence`

**Validation rules**:

- Assistant execution outcome and Matrix reply outcome remain separate.
- Accepted direct messages reply in the direct conversation.
- Accepted room messages reply in the originating room when Matrix delivery permits it.
- Reply failure produces operator-visible diagnostic evidence without raw provider
  payloads.

## Matrix Delivery Outcome

Represents the background notification result for scheduled or workflow-originated work
delivered through Matrix.

**Fields**:

- `tenantId`
- `deliveryTargetId`
- `connectorId`
- `homeserverBindingId`
- `destinationType`: `direct_message` or `room`.
- `destinationId`: redacted provider identifier or stable hash.
- `deliveryOutcome`: `sent`, `retrying`, `suppressed`, `failed`, or `not_attempted`.
- `failureReasonCode`: optional diagnostic reason.
- `attemptedAt`, `completedAt`
- `redactionStatus`
- `safeEvidence`

**Validation rules**:

- Background delivery outcome is tracked independently from foreground reply outcome and
  assistant execution outcome.
- Delivery eligibility requires valid Matrix setup and a validated destination policy.
- Failed delivery produces diagnostic evidence without changing the execution result of
  the scheduled or workflow-originated work.

## Matrix Diagnostic State

Represents supportable, redacted Matrix setup or runtime diagnostic evidence.

**Fields**:

- `diagnosticStateId`
- `tenantId`
- `connectorId`
- `homeserverBindingId`
- `status`
- `reasonCode`: `auth_missing`, `permission_missing`, `rate_limited`,
  `provider_unavailable`, `network_failed`, `unsupported_capability`, `blocked_route`,
  `duplicate_inbound`, `reply_failed`, or `unknown_connector_failure`, with Matrix
  provider-specific subreasons in safe evidence when redaction permits.
- `matrixCondition`: `bot_auth_invalid`, `bot_auth_revoked`, `room_permission_missing`,
  `ownership_mismatch`, `homeserver_unsupported`, `homeserver_unreachable`,
  `federation_failed`, `rate_limited`, `provider_unavailable`, `network_failed`,
  `blocked_route`, `duplicate_event`, `encrypted_room_unsupported`,
  `undecryptable_event`, `unsupported_surface`, or `unknown`.
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
- Failed Matrix actions produce current diagnostic truth before remediation is shown.
- Evidence expires from normal inspection after 90 days unless a longer authorized
  retention policy applies.

## Matrix Capability Profile

Represents Matrix's conformance declaration.

**Fields**:

- `profileId`
- `tenantId`
- `connectorId`
- `connectorKind`: `matrix`
- `coreInvariantResults`: pass/fail results for phase 48 core invariants.
- `providerSurfaceResults`: supported/limited/unsupported results for tenant-provided
  bot setup, DopeAgent-hosted homeserver provisioning, direct messages, allowed room
  mention routing, allowed room command routing, unencrypted text, encrypted rooms,
  undecryptable events, E2EE key/session management, final-only replies,
  connector-backed delivery, media, voice, calls, reactions, bridge automation, thinking,
  and incremental visible updates.
- `equivalentDurableIdentityRuleId`: `matrix_homeserver_conversation_event_id`.
- `equivalentDurableIdentityRule`: tenant, connector, homeserver, room/direct
  conversation, and Matrix event ID.
- `declaredAt`

**Validation rules**:

- Hosted-ready requires all core invariants to pass.
- Unsupported optional surfaces must not weaken core invariants.
- DopeAgent-hosted homeserver provisioning, encrypted rooms, undecryptable events,
  key/session management, broad media, voice, calls, bridge automation, thinking, and
  incremental visible updates remain unsupported for phase 52 unless explicitly recut.

## Matrix Smoke Evidence

Represents live hosted/test validation when safe Matrix credentials exist, or the
structured skip when they do not.

**Fields**:

- `smokeEvidenceId`
- `tenantId`
- `connectorId`
- `homeserverBindingId`
- `status`: `passed`, `failed`, or `skipped`.
- `authorizationMode`: `safe_live`, `fake_matrix`, or `unavailable`.
- `owner`
- `reason`
- `remainingRisk`
- `validatedAt`
- `retentionExpiresAt`
- `redactionStatus`
- `safeEvidence`

**Validation rules**:

- Live smoke never runs implicitly against production tenants, production rooms, or
  unapproved homeserver accounts.
- Skip evidence includes owner, reason, date, remaining risk, and redaction status.
- Smoke evidence follows the same 90-day default retention and redaction rules.
