# Data Model: Slack Channel Connector

## Slack Hosted Setup

Represents tenant-owned Slack setup progress and connector readiness for one connector
bound to one workspace.

**Fields**:

- `tenantId`: owning tenant.
- `connectorId`: stable Slack connector identifier.
- `connectorKind`: always `slack`.
- `displayName`: redacted tenant-visible connector label.
- `workspaceBindingId`: active one-workspace binding for the connector.
- `status`: shared connector lifecycle status such as `configured`, `healthy`,
  `degraded`, `failed`, `permission_blocked`, `rate_limited`, or
  `unsupported_capability`.
- `terminalState`: hosted setup terminal state: `ready`, `degraded`, `unavailable`,
  `cancelled`, or `action-required`.
- `oauthState`: `not_started`, `started`, `callback_received`, `grant_valid`,
  `grant_missing`, `scope_missing`, `approval_required`, `revoked`, or
  `redaction_suppressed`.
- `routePolicyState`: `none`, `partial`, `valid`, or `stale`.
- `deliveryEligible`: whether the connector can be selected for background delivery.
- `createdAt`, `updatedAt`, `validatedAt`: setup lifecycle timestamps.
- `retentionExpiresAt`: normal inspection expiry for retained setup evidence.
- `redactionStatus`: `redacted`, `suppressed`, or `redaction_failed`.

**Validation rules**:

- OAuth tokens, signing secrets, authorization payloads, and credential-bearing provider
  payloads are never stored in this entity.
- `ready` requires a valid Slack workspace binding, valid OAuth installation evidence,
  required scopes, passing conformance gates, and at least one valid selected channel or
  explicit DM user/user-group allowment.
- Submitted raw Slack bot tokens, signing secrets, and local-only credentials are
  unsupported setup inputs and cannot transition setup to `ready`.
- `degraded`, `unavailable`, and `action-required` must include remediation-bearing
  diagnostic linkage.
- `cancelled` preserves redacted audit evidence and must not delete unrelated connector
  state.

**State transitions**:

```text
oauth-started -> callback-received
callback-received -> ready
callback-received -> degraded
callback-received -> unavailable
callback-received -> action-required
oauth-started -> cancelled
ready -> degraded
ready -> unavailable
ready -> action-required
degraded -> ready
degraded -> action-required
unavailable -> ready
action-required -> ready
ready -> cancelled
```

## Slack Workspace Binding

Represents the tenant-scoped relationship between one Slack connector and one Slack
workspace installation.

**Fields**:

- `tenantId`
- `connectorId`
- `workspaceBindingId`
- `workspaceId`: redacted provider workspace identifier or stable hash where exposed.
- `workspaceLabel`: redacted workspace name or label.
- `installationId`: redacted installation identifier or stable hash where exposed.
- `oauthGrantState`: `valid`, `missing`, `revoked`, `scope_missing`,
  `approval_required`, `workspace_mismatch`, `provider_unavailable`,
  `network_failed`, or `unknown`.
- `requiredScopeState`: `valid`, `missing`, `stale`, or `unknown`.
- `validatedAt`
- `redactionStatus`
- `safeEvidence`: non-secret workspace validation summary.

**Relationships**:

- One Slack Hosted Setup has one active Slack Workspace Binding.
- One tenant may have many Slack Connectors, each with one Slack Workspace Binding.
- One Slack Workspace Binding has one connector-specific Slack Route Policy.
- Inbound messages, diagnostics, conformance evidence, reply outcomes, and delivery
  outcomes reference the workspace binding by tenant, connector, and workspace identity.

**Validation rules**:

- A connector cannot bind more than one workspace.
- A workspace binding cannot be used for another tenant route.
- Workspace mismatch or ambiguous ownership blocks readiness and ingress.

## Slack Route Policy

Represents selected channels and explicit DM sender authorization for a single Slack
connector.

**Fields**:

- `tenantId`
- `connectorId`
- `workspaceBindingId`
- `selectedChannels`: redacted channel identities and validation states.
- `allowedDMUsers`: redacted Slack user identities or stable hashes.
- `allowedDMUserGroups`: redacted Slack user-group identities or stable hashes.
- `mentionGate`: `agent_mention_required` for channel messages in phase 51.
- `threadReplyMode`: `channel_mentions_thread_rooted`.
- `validationState`: `valid`, `partial`, `stale`, `blocked`, or `missing_permission`.
- `reasonCode`: shared diagnostic reason when invalid.
- `createdAt`, `updatedAt`, `validatedAt`
- `redactionStatus`
- `safeEvidence`: non-secret route validation summary.

**Validation rules**:

- Direct-message routing requires an enabled `allowedDMUsers` entry or membership in an
  enabled `allowedDMUserGroups` entry.
- Channel routing requires an enabled selected channel plus an agent mention or another
  explicitly supported invocation signal.
- Missing selected channel or DM allowment fails closed as `blocked_route` or
  `action-required` depending on setup context.
- Route evidence must suppress raw provider payloads when redaction confidence is
  insufficient.

## Slack Conversation

Represents a direct-message or channel context under a Slack workspace binding.

**Fields**:

- `tenantId`
- `connectorId`
- `workspaceBindingId`
- `conversationId`: redacted Slack conversation identity or stable hash where exposed.
- `conversationType`: `direct_message` or `channel`.
- `selectedChannelState`: `selected`, `not_selected`, `stale`, `archived`,
  `missing_membership`, or `not_applicable`.
- `threadRootPolicy`: `required_for_channel_mentions` or `not_applicable`.
- `redactionStatus`
- `safeEvidence`

**Validation rules**:

- Channel conversations must be selected before accepted ingress or background delivery
  eligibility can use them.
- Direct-message conversations require explicit user or user-group allowment.
- Archived, inaccessible, or missing-membership channels fail closed until repaired.

## Slack Inbound Message

Represents a normalized inbound Slack event handled by the IM loop.

**Fields**:

- `tenantId`
- `connectorId`
- `workspaceBindingId`
- `workspaceId`: redacted or hashed workspace identity where exposed.
- `conversationId`: redacted or hashed channel or DM identity where exposed.
- `messageId`: provider message identity within the Slack conversation.
- `eventId`: retained as redacted provider delivery evidence.
- `senderId`: redacted or hashed Slack sender identity where exposed.
- `senderAllowmentType`: `explicit_user`, `user_group_member`, `channel_mention`, or
  `none`.
- `conversationType`: `direct_message` or `channel`.
- `textKind`: `dm_text`, `channel_mention`, `unsupported`, or `unknown`.
- `threadRootId`: required for accepted channel mentions.
- `routingOutcome`: `accepted`, `ignored`, `blocked`, `duplicate`, `unsupported`, or
  `failed`.
- `reasonCode`: `blocked_route`, `mention_required`, `duplicate_inbound`,
  `unsupported_capability`, or other shared route reason.
- `receivedAt`
- `redactionStatus`

**Validation rules**:

- Durable dedupe identity is tenant, connector, workspace, conversation, and message ID.
- Slack event ID is retained as delivery evidence but is not the canonical dedupe key.
- Unsupported Slack surfaces produce `unsupported` and cannot create runs.
- Missing tenant/workspace/conversation/message identity fails closed unless an explicit
  equivalent durable identity rule is documented and conformance-tested.
- Agent mention artifacts are normalized before assistant handling when a channel message
  is accepted.

## Slack Reply Outcome

Represents the foreground Slack reply result for an accepted inbound message.

**Fields**:

- `tenantId`
- `connectorId`
- `inboundMessageIdentity`
- `assistantExecutionOutcome`: `succeeded`, `failed`, or `cancelled`.
- `slackReplyOutcome`: `sent`, `failed`, or `not_attempted`.
- `replyProgressionLevel`: `final_only` for phase 51, or `unsupported` for optional
  progression surfaces.
- `replyContext`: `direct_message` or `channel_thread`.
- `threadRootId`: required for `channel_thread`.
- `failureReasonCode`: optional diagnostic reason.
- `attemptedAt`, `completedAt`
- `redactionStatus`
- `safeEvidence`

**Validation rules**:

- Assistant execution outcome and Slack reply outcome remain separate.
- Accepted direct messages reply in the DM conversation.
- Accepted channel mentions reply in a thread rooted at the triggering channel message.
- Reply failure produces operator-visible diagnostic evidence without raw provider
  payloads.

## Slack Delivery Outcome

Represents the background notification result for scheduled or workflow-originated work
delivered through Slack.

**Fields**:

- `tenantId`
- `deliveryTargetId`
- `connectorId`
- `workspaceBindingId`
- `destinationType`: `direct_message` or `channel`.
- `destinationId`: redacted provider identifier or stable hash.
- `deliveryOutcome`: `sent`, `retrying`, `suppressed`, `failed`, or `not_attempted`.
- `failureReasonCode`: optional diagnostic reason.
- `attemptedAt`, `completedAt`
- `redactionStatus`
- `safeEvidence`

**Validation rules**:

- Background delivery outcome is tracked independently from foreground reply outcome and
  assistant execution outcome.
- Delivery eligibility requires a valid Slack setup and a validated destination policy.
- Failed delivery produces diagnostic evidence without changing the execution result of
  the scheduled or workflow-originated work.

## Slack Diagnostic State

Represents supportable, redacted Slack setup or runtime diagnostic evidence.

**Fields**:

- `diagnosticStateId`
- `tenantId`
- `connectorId`
- `workspaceBindingId`
- `status`
- `reasonCode`: `auth_missing`, `permission_missing`, `rate_limited`,
  `provider_unavailable`, `network_failed`, `unsupported_capability`, `blocked_route`,
  `duplicate_inbound`, `reply_failed`, or `unknown_connector_failure`, with Slack
  provider-specific subreasons in safe evidence when redaction permits.
- `slackCondition`: `missing_oauth_grant`, `missing_scope`, `installation_missing`,
  `workspace_approval_required`, `workspace_mismatch`, `channel_access_missing`,
  `event_delivery_failed`, `rate_limited`, `provider_unavailable`, `network_failed`,
  `blocked_route`, `duplicate_message`, `unsupported_surface`, or `unknown`.
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
- Failed Slack actions produce current diagnostic truth before remediation is shown.
- Evidence expires from normal inspection after 90 days unless a longer authorized
  retention policy applies.

## Slack Capability Profile

Represents Slack's conformance declaration.

**Fields**:

- `profileId`
- `tenantId`
- `connectorId`
- `connectorKind`: `slack`
- `coreInvariantResults`: pass/fail results for phase 48 core invariants.
- `providerSurfaceResults`: supported/limited/unsupported results for Slack direct
  messages, selected channel mentions, required channel thread replies, final-only
  replies, connector-backed delivery, hosted OAuth setup, submitted-token setup,
  marketplace publication, enterprise grid administration, files, voice clips, huddles,
  canvases, workflow buttons, interactive blocks, rich media, memory-based team context,
  thinking, and incremental visible updates.
- `declaredAt`

**Validation rules**:

- Hosted-ready requires all core invariants to pass.
- Unsupported optional surfaces must not weaken core invariants.
- Submitted raw-token setup, marketplace publication, enterprise grid administration,
  voice huddles, memory-based team context, broad media, thinking, and incremental visible
  updates remain unsupported for phase 51 unless explicitly recut.

## Slack Smoke Evidence

Represents live hosted/test validation when safe workspace authorization exists, or the
structured skip when it does not.

**Fields**:

- `smokeEvidenceId`
- `tenantId`
- `connectorId`
- `workspaceBindingId`
- `status`: `passed`, `failed`, or `skipped`.
- `authorizationMode`: `safe_live`, `fake_oauth`, or `unavailable`.
- `owner`
- `reason`
- `remainingRisk`
- `validatedAt`
- `retentionExpiresAt`
- `redactionStatus`
- `safeEvidence`

**Validation rules**:

- Live smoke never runs implicitly against production tenants or unapproved workspaces.
- Skip evidence includes owner, reason, date, remaining risk, and redaction status.
- Smoke evidence follows the same 90-day default retention and redaction rules.
