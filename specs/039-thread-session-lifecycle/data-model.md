# Data Model: Daemon-Owned Thread And Session Lifecycle

## Thread

Represents one tenant-owned conversation/workstream with daemon-owned lifecycle truth.

### Fields

- `threadId`: Stable thread identity.
- `tenantId`: Tenant that owns the thread.
- `lifecycleState`: `active`, `reset`, `archived`, or `reopened`.
- `currentSessionSegmentId`: Active session segment used for future continuation.
- `sourceKind`: `chat`, `channel`, `workflow`, `schedule`, `shell`, or `legacy`.
- `sourceSummary`: Redacted source label for user/operator display.
- `lastActivityAt`: Latest lifecycle, source, session, or runtime activity.
- `createdAt`, `updatedAt`: Thread timestamps.
- `retentionExpiresAt`: Expiry for normal lifecycle/source/runtime projection inspection.
- `redactionStatus`: `redacted`, `suppressed`, or `redaction_failed`.

### Relationships

- Has one or more `Session Segment` records.
- Has zero or more `Source Linkage` records.
- Has zero or more `Lifecycle Action` records.
- Has zero or more `Runtime Projection` records.
- Has zero or more `Connector Message Evidence` records for channel-origin threads.
- Has zero or more `Lifecycle Audit Record` records.

### Validation Rules

- Reads require `credentials.inspect`.
- Mutations require `connectors.manage`.
- Archived threads remain inspectable but are not eligible for unintended continuation.
- Reset preserves `threadId` and creates a new active session segment.
- Thread evidence expires from normal inspection after 90 days unless tenant policy
  requires longer retention.

## Session Segment

Represents a bounded unit of conversation continuity within a thread.

### Fields

- `sessionSegmentId`: Stable segment identity.
- `threadId`, `tenantId`
- `sessionId`: Existing router/session identity when the segment maps to a legacy or
  current `router.Session`.
- `generation`: Session generation or segment sequence, minimum `1`.
- `state`: `active`, `closed`, `reset`, `archived`, or `partial`.
- `startedAt`, `endedAt`
- `lastActiveAt`
- `resetFromSessionSegmentId`: Prior segment when created by reset.
- `partialEvidence`: Boolean marker for legacy sessions with incomplete linkage.

### Relationships

- Belongs to one `Thread`.
- Links to zero or more runs, workflows, approvals, replies, and deliveries through
  `Runtime Projection`.

### State Transitions

- `active` -> `reset`: Reset closes the previous segment and creates a new `active`
  segment on the same thread.
- `active` -> `archived`: Archive blocks future continuation but does not cancel already
  accepted runtime work.
- `archived` -> `active`: Reopen creates or marks an eligible active segment when source
  and session rules still allow continuation.
- Any mutation -> unchanged when permission, tenant, source, or audit checks fail.

### Validation Rules

- A thread has at most one active segment at a time.
- Segment creation is tenant-scoped and audit-linked.
- Existing `/v1/sessions` semantics remain compatible.

## Thread Lifecycle State

Represents current lifecycle state plus transition history.

### Fields

- `threadId`, `tenantId`
- `state`: `active`, `reset`, `archived`, or `reopened`
- `previousState`
- `changedByPrincipalId`
- `changedAt`
- `reasonCode`
- `auditEventId`
- `redactionStatus`

### Validation Rules

- `reset`, `archive`, and `reopen` require `connectors.manage`.
- Required audit evidence must be recorded before state changes.
- Audit-write failure leaves state unchanged.

## Lifecycle Action

Represents a user-requested reset, archive, or reopen mutation.

### Fields

- `lifecycleActionId`
- `threadId`, `tenantId`
- `actionKind`: `reset`, `archive`, or `reopen`
- `actorPrincipalId`
- `priorState`
- `resultingState`
- `priorSessionSegmentId`
- `resultingSessionSegmentId`
- `reasonCode`
- `requestedAt`, `completedAt`
- `status`: `succeeded`, `denied`, `failed_closed`, or `unsupported`
- `auditEventId`
- `redactionStatus`

### Validation Rules

- Reset preserves thread identity and creates a new active session segment.
- Archive blocks future continuation and must not cancel active/pending runtime work.
- Reopen is allowed only when tenant, source, connector, and session rules still allow
  continuation.
- Denied actions must not expose inaccessible thread existence.

## Source Linkage

Represents how a thread/session maps to its origin.

### Fields

- `sourceLinkageId`
- `threadId`, `tenantId`
- `sourceKind`: `channel`, `chat`, `workflow`, `schedule`, `shell`, or `legacy`
- `connectorId`
- `connectorKind`
- `sourceAccountId`
- `sourceConversationId`
- `sourceMessageId`
- `routingOutcome`: `accepted`, `ignored`, `blocked`, `duplicate`, `disabled`,
  `unsupported`, or `failed`
- `current`: Boolean marker for the current source-conversation-to-thread mapping.
- `linkedAt`
- `retentionExpiresAt`
- `redactionStatus`

### Relationships

- Belongs to one `Thread`.
- For channel sources, links one source message to `Connector Message Evidence`.

### Validation Rules

- For channel continuation, at most one current thread exists for each tenant,
  connector, source account, and source conversation.
- Later accepted inbound messages for the same source key attach to the current thread
  unless lifecycle state blocks continuation or a lifecycle action creates a new eligible
  current thread.
- Source message identity is event evidence, not thread identity.

## Connector Message Evidence

Represents metadata-only evidence for a connector message and its routing result.

### Fields

- `connectorMessageEvidenceId`
- `tenantId`, `threadId`, `sessionSegmentId`
- `connectorId`, `connectorKind`
- `sourceAccountId`
- `sourceConversationId`
- `sourceMessageId`
- `providerMessageId`
- `routingOutcome`
- `runId`
- `workflowId`
- `replyOutcomeId`
- `deliveryOutcomeId`
- `occurredAt`
- `safeEvidence`
- `retentionExpiresAt`
- `redactionStatus`

### Validation Rules

- Accepted messages link to a thread/session segment and to any run or workflow they
  caused.
- Ignored, blocked, duplicate, disabled, unsupported, or failed messages produce routing
  evidence without implying assistant work was created.
- Evidence must not expose secrets, raw provider payloads, or disallowed message bodies.

## Runtime Projection

Represents thread-visible linkage to authoritative runtime records.

### Fields

- `runtimeProjectionId`
- `threadId`, `tenantId`
- `sessionSegmentId`
- `resourceKind`: `session`, `run`, `workflow`, `approval`, `foreground_reply`,
  `background_delivery`, or `connector_message`
- `resourceId`
- `status`
- `reasonCode`
- `occurredAt`
- `route`: Optional product route for authorized detail inspection.
- `safeSummary`
- `retentionExpiresAt`
- `redactionStatus`

### Validation Rules

- Projection records summarize and link to authoritative records; they do not replace
  run, workflow, approval, reply, or delivery ownership.
- Thread detail presents execution, approval, foreground reply, and background delivery
  outcomes as separate facts.
- Projection reads require `credentials.inspect`.

## Lifecycle Audit Record

Represents audit evidence for inspection, mutation, permission denial, redaction failure,
audit-write failure, retention application, and restart recovery.

### Fields

- `auditEventId`
- `tenantId`
- `threadId`
- `principalId`
- `action`
- `permissionGate`
- `outcome`: `succeeded`, `denied`, `failed_closed`, `redacted`, or `retention_applied`
- `reasonCode`
- `createdAt`
- `redactionStatus`

### Validation Rules

- Required audit records must be written before lifecycle mutations commit.
- Audit-write failure for reset/archive/reopen leaves thread state unchanged.
- Permission denials do not reveal inaccessible tenant thread existence.

## Legacy Session Evidence

Represents partial projection of sessions created before first-class thread lifecycle.

### Fields

- `legacyEvidenceId`
- `tenantId`
- `threadId`
- `sessionId`
- `routingKey`
- `channel`
- `accountId`
- `peerId`
- `threadIdFromSession`
- `projectionStatus`: `complete`, `partial`, `unsupported`, or `failed`
- `missingFields`
- `createdAt`
- `redactionStatus`

### Validation Rules

- Legacy sessions must remain inspectable as partial evidence rather than disappearing.
- Projection must not invent source linkage when the historical record lacks enough
  data.
- Partial evidence must be labeled clearly in list/detail projections.
