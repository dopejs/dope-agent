# Data Model: Group Room Reset Handoff

## Conversation Shape

Represents the explicit product classification for a thread/source conversation.

### Fields

- `conversationShapeId`: Stable shape record identity.
- `tenantId`, `threadId`, `sessionSegmentId`
- `shape`: `direct_message`, `group`, `room`, `web`, `unknown`, or `unsupported`.
- `sourceKind`: `chat`, `channel`, `shell`, `workflow`, `schedule`, or `legacy`.
- `connectorId`, `connectorKind`
- `sourceAccountId`
- `sourceConversationId`
- `sourceConversationDisplay`: Redacted display label.
- `participantSummary`: Redacted participant metadata summary where allowed.
- `shapeEvidenceStatus`: `proven`, `partial`, `unsupported`, or `failed`.
- `recordedAt`, `updatedAt`
- `retentionExpiresAt`
- `redactionStatus`
- `documentJson`: Full metadata document.

### Relationships

- Belongs to one Roadmap 54 `Thread` and current `Session Segment`.
- Links to one or more Roadmap 54 `Source Linkage` records.
- May be referenced by participation decisions, reset events, and handoff links.

### Validation Rules

- Supported group, room, direct-message, and web behavior requires a proven shape.
- `unknown` or `unsupported` shape must not receive implicit group participation or
  handoff behavior.
- Stable source identity, not display name or participant overlap, determines room
  isolation.
- Unsafe source or participant detail must be suppressed to safe metadata.

## Room Identity

Represents stable source evidence that distinguishes one shared conversation space from
another.

### Fields

- `roomIdentityId`
- `tenantId`, `connectorId`, `connectorKind`
- `sourceAccountId`
- `sourceConversationId`
- `roomKind`: `group`, `room`, `shared_channel`, `direct_message`, or `unknown`.
- `displayNameSafeSummary`
- `identityStatus`: `proven`, `partial`, `renamed`, `deleted`, `recreated`,
  `unsupported`, or `failed`.
- `firstSeenAt`, `lastSeenAt`
- `retentionExpiresAt`
- `redactionStatus`

### Relationships

- May map to the current thread for a tenant/connector/source account/source
  conversation key.
- May have many participation decisions and handoff links over time.

### Validation Rules

- Room rename, archive, delete, or recreate events must not change identity unless the
  connector proves a new source conversation identity.
- Participant overlap must not merge room identities.

## Participation Policy

Represents tenant and source-specific rules for whether the assistant may participate in
a group or room.

### Fields

- `participationPolicyId`
- `tenantId`
- `connectorId`, `connectorKind`
- `sourceAccountId`
- `sourceConversationId`
- `shape`: `group` or `room`
- `allowlistEligible`: Boolean.
- `qualifyingMentionRequired`: Boolean; default `true`.
- `policyStatus`: `enabled`, `disabled`, `unsupported`, or `failed`.
- `configuredByPrincipalId`
- `configuredAt`, `updatedAt`
- `retentionExpiresAt`
- `redactionStatus`

### Relationships

- Applies to participation decisions for matching group or room source identity.
- May be projected into connector conformance/support evidence.

### Validation Rules

- Default group/room participation requires both `allowlistEligible=true` and a
  qualifying mention.
- Broader room-level participation requires a future explicit policy decision and is not
  implied by this phase.
- Missing, unsupported, or failed policy evaluation must fail closed.

## Participation Decision

Represents the routing outcome for a group or room message.

### Fields

- `participationDecisionId`
- `tenantId`, `threadId`, `sessionSegmentId`
- `connectorId`, `connectorKind`
- `sourceAccountId`, `sourceConversationId`, `sourceMessageId`
- `conversationShape`
- `policyId`
- `mentionStatus`: `qualified`, `missing`, `ambiguous`, `unsupported`, or `failed`.
- `allowlistStatus`: `eligible`, `not_allowlisted`, `unsupported`, or `failed`.
- `decision`: `accepted`, `ignored`, `blocked`, `denied`, `duplicate`, `unsupported`,
  or `failed`.
- `reasonCode`: Stable reason classification.
- `createdAssistantWork`: Boolean.
- `occurredAt`
- `retentionExpiresAt`
- `redactionStatus`
- `safeSummary`
- `documentJson`

### Relationships

- Belongs to one thread when accepted or when safe source mapping is known.
- Links to connector message evidence and source linkage records.
- May link to a runtime projection when assistant work is created.

### Validation Rules

- Decisions that are not `accepted` must not create assistant turns, handoff links, or
  misleading continuity evidence.
- Duplicate, replayed, edited, deleted, ignored, blocked, disabled, unsupported, or
  failed source messages must not create duplicate accepted decisions.
- Unauthorized inspection must not reveal inaccessible source identity or message
  content.

## Reset Event

Represents a lifecycle reset scoped to a specific conversation shape and source thread
boundary.

### Fields

- `resetEventId`
- `tenantId`, `threadId`
- `conversationShape`
- `sourceConversationId`
- `actorPrincipalId`
- `permissionGate`: `connectors.manage`.
- `priorSessionSegmentId`
- `resultingSessionSegmentId`
- `status`: `succeeded`, `denied`, `failed_closed`, or `unsupported`.
- `reasonCode`
- `requestedAt`, `completedAt`
- `auditEventId`
- `retentionExpiresAt`
- `redactionStatus`

### Relationships

- Extends or references Roadmap 54 lifecycle action evidence.
- May be visible in thread detail and handoff evidence.

### Validation Rules

- Reset preserves thread identity and starts a new active segment.
- Reset of one conversation shape or source must not reset unrelated threads.
- Denied reset must not expose inaccessible historical detail.
- Audit evidence must be recorded before successful mutation commits.

## Handoff Link

Represents a traceable relationship between a source thread and a separate destination
thread.

### Fields

- `handoffLinkId`
- `tenantId`
- `sourceThreadId`, `sourceSessionSegmentId`
- `destinationThreadId`, `destinationSessionSegmentId`
- `sourceConversationShape`, `destinationConversationShape`
- `sourceKind`, `destinationKind`
- `sourceConnectorId`, `destinationConnectorId`
- `sourceConversationId`, `destinationConversationId`
- `actorPrincipalId`
- `permissionGate`: `connectors.manage`.
- `status`: `succeeded`, `denied`, `failed_closed`, `unsupported`, or `expired`.
- `reasonCode`
- `firstDestinationResponseId`: Optional response/dispatch identity that consumes the
  source-turn reference bridge.
- `sourceReferenceStatus`: `available`, `consumed`, `blocked`, `expired`, or `none`.
- `createdAt`, `consumedAt`
- `retentionExpiresAt`
- `redactionStatus`
- `documentJson`

### Relationships

- Links exactly one source thread to one separate destination thread.
- Has zero or more `Handoff Source Reference` records.
- Appears in thread detail for either side when the inspector can see that side.

### Validation Rules

- `sourceThreadId` and `destinationThreadId` must be different.
- Handoff creation requires `connectors.manage`.
- Source and destination identity, lifecycle eligibility, tenant permission, connector
  permission, and participant permission must be proven before success.
- Denied handoff must not silently create a destination conversation.

## Handoff Source Reference

Represents an eligible source turn or excerpt made available by reference to the first
destination response after handoff.

### Fields

- `handoffSourceReferenceId`
- `handoffLinkId`
- `tenantId`
- `sourceThreadId`, `sourceSessionSegmentId`
- `destinationThreadId`, `destinationSessionSegmentId`
- `continuityTurnId`
- `artifactExcerptRef`: Optional safe excerpt reference.
- `eligibilityStatus`: `eligible`, `permission_denied`, `redaction_failed`,
  `retention_expired`, `reset_boundary`, `incomplete_evidence`, or `unsupported`.
- `decision`: `referenced`, `excluded`, or `consumed`.
- `safeSummary`
- `redactionStatus`
- `createdAt`, `consumedAt`
- `retentionExpiresAt`

### Relationships

- Belongs to one handoff link.
- References Roadmap 55 continuity turns and preview evidence by identity.
- May be represented in the first destination continuity preview as handoff source
  evidence.

### Validation Rules

- References are eligible only for current-segment source turns after the latest reset
  boundary.
- References are available only for the first destination response after handoff.
- Source turns must never be copied into destination thread history as new destination
  turns.
- After the first destination response, destination continuity uses destination-thread
  turns only unless another authorized handoff occurs.

## Inspection Evidence

Represents operator-visible summaries for group participation, reset, and handoff
behavior.

### Fields

- `inspectionEvidenceId`
- `tenantId`, `threadId`, `sessionSegmentId`
- `evidenceKind`: `conversation_shape`, `participation_decision`, `reset_event`,
  `handoff_link`, or `handoff_source_reference`.
- `resourceId`
- `status`
- `reasonCode`
- `safeSummary`
- `redactionStatus`
- `createdAt`
- `retentionExpiresAt`

### Validation Rules

- Reads require the thread inspection permission from Roadmap 54.
- Users who can inspect only one side of a handoff may see only safe, permission-allowed
  detail for that side.
- Evidence must never expose secrets, raw provider payloads, disallowed message bodies,
  unsafe connector metadata, or cross-tenant identifiers.

## State Transitions

```text
connector message received
  -> conversation shape resolved from tenant/connector/source identity
  -> participation policy evaluated for group/room sources
  -> participation decision recorded
  -> accepted decisions attach to current thread segment or create eligible thread state
  -> non-accepted decisions record evidence without assistant work

thread reset requested
  -> connectors.manage checked
  -> source/conversation shape scope resolved
  -> Roadmap 54 lifecycle reset creates new active segment on the same thread
  -> reset event records scoped source evidence

handoff requested
  -> connectors.manage checked
  -> source and destination identity, lifecycle, participation, and permissions checked
  -> separate destination thread created or selected
  -> handoff link recorded
  -> eligible source-turn references created for first destination response

first destination response after handoff
  -> Roadmap 55 continuity evaluates destination thread
  -> eligible handoff source references may be included by reference
  -> source references marked consumed
  -> source turns are not copied into destination history

later destination responses
  -> use destination-thread continuity only
  -> source references are unavailable unless another authorized handoff occurs
```

## Storage And Indexing Notes

- Conversation shape records should index `(tenant_id, thread_id, session_segment_id)`.
- Room identity and participation policy should index `(tenant_id, connector_id,
  source_account_id, source_conversation_id)`.
- Participation decisions should include a unique source-event key where available to
  suppress duplicate/replayed connector messages.
- Handoff links should index `(tenant_id, source_thread_id, created_at DESC)` and
  `(tenant_id, destination_thread_id, created_at DESC)` for thread detail projection.
- Handoff source references should index `handoff_link_id` and
  `(tenant_id, destination_thread_id, destination_session_segment_id)`.
- All additive tables include `document_json` to match existing store conventions and
  allow schema-compatible extension.
