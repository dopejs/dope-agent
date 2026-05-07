# Data Model: Channel Connector Conformance

## Hosted Channel Connector

Represents a tenant-owned channel integration that can receive inbound messages, send
foreground replies, and optionally act as a background delivery target.

**Fields**:

- `tenantId`: Owning tenant. Required for hosted operation.
- `connectorId`: Stable daemon connector identifier.
- `kind`: Provider kind, such as `discord`, `telegram`, or `slack`.
- `displayName`: Operator-readable label.
- `status`: Lifecycle state.
- `accountBinding`: Redacted connector account binding summary.
- `capabilityProfileId`: Latest declared connector capability profile.
- `diagnosticStateId`: Latest connector diagnostic state, when available.
- `createdAt`, `updatedAt`: Audit timestamps.

**State transitions**:

- `configured` -> `starting` -> `healthy`
- `healthy` -> `degraded` -> `failed`
- `healthy` or `degraded` -> `rate_limited` -> `healthy` or `failed`
- Any active state -> `permission_blocked` when provider or tenant permission blocks use
- Any active state -> `disabled`
- Any state can include `unsupported_capability` evidence for provider-specific surfaces

**Validation rules**:

- Hosted-ready connectors must pass all core invariants.
- Connector inspection must be tenant-scoped and redacted.
- Disabled or permission-blocked connectors cannot accept inbound events or background
  delivery attempts.

## Connector Account Binding

Represents the tenant-scoped relationship between a tenant and an external provider
account, bot, workspace, room, or similar identity.

**Fields**:

- `tenantId`: Owning tenant.
- `connectorId`: Connector that owns this binding.
- `connectorAccountId`: Stable account binding identity used in inbound message identity.
- `providerAccountLabel`: Redacted operator-readable external account label.
- `secretRefs`: Redacted secret reference summaries only.
- `permissionState`: `available`, `missing_permission`, `auth_missing`,
  `permission_blocked`, `disconnected`, or `unknown`.
- `createdAt`, `updatedAt`: Audit timestamps.

**Validation rules**:

- Raw tokens, secret values, authorization headers, and credential-bearing provider
  payloads are never exposed.
- Runtime use resolves only inside the active tenant.

## Connector Capability Profile

Declares a connector's support for core invariants and provider-specific surfaces.

**Fields**:

- `profileId`: Stable profile identity.
- `connectorId`, `tenantId`: Scope.
- `coreInvariantResults`: Result map for tenant ownership, permission gating, redaction,
  active-tenant account binding, inbound identity, durable dedupe, stable routing
  decisions, final-only foreground reply delivery, diagnostics, and delivery separation.
- `providerSurfaceResults`: Result map for direct messages, groups, mentions, rooms,
  threads, thinking, incremental updates, rich media, and other provider-specific
  surfaces.
- `declaredAt`: Time of declaration.

**Validation rules**:

- Every core invariant must pass for hosted readiness.
- Provider-specific surfaces may be `supported`, `limited`, or `unsupported` only when
  explicit and conformance-tested.

## Inbound Channel Event

Normalized event representing a user-originated channel message.

**Fields**:

- `tenantId`: Active tenant.
- `connectorId`: Connector receiving the event.
- `connectorAccountId`: Account binding that received the provider event.
- `channelOrConversationId`: Provider conversation scope used for routing and dedupe.
- `providerMessageId`: Provider message identity.
- `authorId`: Redacted sender identity.
- `routingContext`: Direct, group, mention, room, or thread context.
- `content`: Normalized user intent after connector-specific addressing artifacts are
  removed.
- `receivedAt`: Event timestamp.

**Validation rules**:

- Standard message identity is `tenantId + connectorAccountId + channelOrConversationId
  + providerMessageId`.
- Equivalent durable identity rules must be explicit and conformance-tested.
- Missing or unstable identity fails conformance unless the equivalent rule proves tenant
  scope and duplicate suppression.

## Message Identity

Durable key used to suppress duplicate inbound deliveries and restart replays.

**Fields**:

- `tenantId`
- `connectorAccountId`
- `channelOrConversationId`
- `providerMessageId`
- `equivalentRuleId`, optional, when a provider-specific durable identity rule is used

**Relationships**:

- One identity resolves to at most one inbound connector message record.
- Duplicate deliveries reference the existing inbound record and do not create a second
  assistant reply.

## Routing Decision

Result of evaluating an inbound channel event against tenant, account, channel, and
provider-specific rules.

**Fields**:

- `decisionId`
- `tenantId`
- `connectorId`
- `messageIdentity`
- `outcome`: `accepted`, `ignored`, `blocked`, `duplicate`, `unsupported`, or `failed`
- `reasonCode`: Stable machine-readable reason.
- `conversationDestination`: Session route when accepted.
- `createdAt`

**Validation rules**:

- Accepted messages must route through daemon-owned session/run truth.
- Blocked and unsupported outcomes must not send assistant replies.
- Duplicate outcomes must be operator-inspectable and must not re-run the assistant.

## Reply Progression Level

Visible foreground reply capability for a connector.

**Values**:

- `final_only`
- `thinking_plus_final`
- `thinking_plus_incremental`
- `unsupported`

**Validation rules**:

- Accepted messages require at least `final_only`.
- Thinking and incremental output must degrade safely when a provider cannot support
  them or cannot throttle updates.
- Reply progression remains tied to daemon-owned session/run/step truth.

## Connector Diagnostic State

Tenant-scoped readiness and failure status for connector operation.

**Fields**:

- `diagnosticStateId`
- `tenantId`
- `connectorId`
- `connectorAccountId`
- `status`: `healthy`, `degraded`, `failed`, `auth_missing`,
  `permission_missing`, `rate_limited`, `provider_unavailable`, `network_failed`,
  `unsupported_capability`, `blocked_route`, `duplicate_inbound`, `reply_failed`,
  or `unknown_connector_failure`
- `remediationOwner`: User, tenant administrator, operator, provider, or no action.
- `retrySafety`: Retryable, unsafe to retry, operator-action-needed, or not applicable.
- `evidenceTimestamp`
- `freshnessState`: `fresh` or `stale`
- `redactionStatus`: `redacted`, `suppressed`, or `redaction_failed`
- `retentionExpiresAt`

**Validation rules**:

- Cached state becomes stale after 15 minutes.
- Connector action failures must produce current diagnostic truth before remediation is
  shown.
- Evidence expires from normal inspection after 90 days unless covered by an authorized
  longer retention policy.

## Foreground Reply Outcome

Result of answering an active inbound connector message.

**Fields**:

- `outcomeId`
- `tenantId`
- `connectorId`
- `messageIdentity`
- `sessionId`, `runId`, `stepId`
- `replyProgressionLevel`
- `status`: `sent`, `partial`, `failed`, or `unsupported`
- `replyMessageIds`
- `errorClass`, optional
- `createdAt`, `updatedAt`

**Validation rules**:

- Foreground reply outcome is not a background delivery outcome.
- Reply failures after assistant execution remain inspectable separately from execution
  success.

## Background Delivery Outcome

Result of delivering background work through a connector-backed delivery target.

**Fields**:

- `deliveryId`
- `tenantId`
- `chosenTargetId`
- `connectorId`
- `connectorAccountId`
- `channelOrConversationId`
- `attempts`
- `status`: `delivered`, `retrying`, `suppressed`, or `failed`
- `terminalReason`, optional
- `createdAt`, `updatedAt`

**Validation rules**:

- Background delivery outcome is not a foreground reply outcome.
- Connector transport reuse must still record delivery target, attempts, retry or
  suppression state, and terminal failure separately.

## Conformance Result

Evidence produced by the shared fake connector matrix or Discord regression.

**Fields**:

- `conformanceResultId`
- `tenantId`
- `connectorKind`
- `connectorId`, optional for fake matrix cases
- `scenarioId`
- `area`: Core invariant or provider-specific surface.
- `result`: `pass`, `fail`, `supported`, `limited`, or `unsupported`
- `reasonCode`
- `redactionStatus`
- `evidenceTimestamp`
- `retentionExpiresAt`

**Validation rules**:

- 100% of core invariants must pass for hosted-ready connectors.
- 100% of provider-specific surfaces must be explicit.
- Results expire from normal inspection after 90 days unless covered by an authorized
  longer retention policy.
