# Data Model: Mail Integration

## Entities

### Mail Account Projection

- Purpose: Mail-domain view of the selected integration-backed mailbox, including the
  mailbox identity and capability flags used by phase 30.
- Fields:
  - `mailAccountId`
  - `integrationId`
  - `domainKind`: always `mail`
  - `environmentScope`
  - `accountKey`
  - `accountLabel`
  - `readinessStatus`
  - `canonicalDefault`
  - `selectionMode`
  - `mailboxAddress`
  - `mailboxLabel`
  - `supportsThreadInspection`
  - `supportsDrafts`
  - `supportsDirectSend`
  - `supportsReply`
  - `supportsForward`
  - `lastSyncedAt`
  - `updatedAt`
- Validation rules:
  - every mail account projection belongs to exactly one integration resource and one
    environment scope
  - the projection may exist only for mail-domain integrations
  - the projection must surface the selected mailbox identity explicitly
  - outbound actions are allowed only when the projection reports the corresponding
    capability

### Mail Operation Record

- Purpose: Operator-visible record for one mail-domain action, including mailbox
  selection, operation class, send path when present, related thread or message or draft
  identity, source linkage, and terminal truth.
- Fields:
  - `operationId`
  - `operationClass`: `list_threads`, `get_thread`, `get_message`, `list_drafts`,
    `get_draft`, `create_draft`, `update_draft`, `send_message`, `send_draft`,
    `reply_message`, or `forward_message`
  - `integrationId`
  - `mailAccountId`
  - `environmentScope`
  - `selectionMode`
  - `resultMode`: `inspection`, `draft_only`, `sent`, `blocked`, or `failed`
  - `sendPath`: `direct`, `draft`, or empty when not applicable
  - `threadId`
  - `messageId`
  - `draftId`
  - `requestSummary`
  - `failureClass`
  - `failureReason`
  - `backgroundSendPermitted`
  - `runId`
  - `stepId`
  - `toolCallID`
  - `workflowId`
  - `workflowStepId`
  - `scheduleId`
  - `scheduleAttemptId`
  - `deliveryId`
  - `artifactIds`
  - `createdAt`
  - `completedAt`
  - `updatedAt`
- Validation rules:
  - every record captures exactly one operation class
  - `send_message` and `send_draft` are distinct operation classes and must surface
    `sendPath`
  - `reply_message` and `forward_message` may end in `draft_only` or `sent` but must
    preserve source message and thread linkage
  - background-send operations may reach `sent` only when `backgroundSendPermitted` is
    true
  - operations blocked before any backend state is observed may remain artifact-free
  - operations remain valid even when downstream delivery fails later

### Mail Thread Snapshot

- Purpose: Structured snapshot of a mailbox conversation as observed by one mail
  operation.
- Fields:
  - `threadId`
  - `operationId`
  - `integrationId`
  - `mailAccountId`
  - `subject`
  - `participantSummary`
  - `messageIds`
  - `draftIds`
  - `latestMessageAt`
  - `messageCount`
  - `draftCount`
  - `createdAt`
- Validation rules:
  - one thread snapshot belongs to exactly one mail operation
  - thread snapshots remain inspectable even when later sends or draft updates occur
  - thread identity must remain stable enough to correlate inspection, reply, and
    forward actions

### Mail Message Snapshot

- Purpose: Structured snapshot of one inbound or outbound message as observed or created
  by a mail operation.
- Fields:
  - `messageId`
  - `threadId`
  - `operationId`
  - `integrationId`
  - `mailAccountId`
  - `direction`: `inbound` or `outbound`
  - `senderSummary`
  - `recipientSummary`
  - `subject`
  - `bodyPreview`
  - `replyToMessageId`
  - `forwardedFromMessageId`
  - `deliveryState`: `received`, `sent`, `blocked`, or `historical`
  - `attachmentRefIds`
  - `sentAt`
  - `receivedAt`
  - `createdAt`
- Validation rules:
  - `deliveryState` is `sent` only when the corresponding operation result is `sent`
  - new outbound messages that are not replies or forwards must show explicit recipient
    summaries taken from the current request
  - blocked-send snapshots must not claim successful delivery

### Mail Draft Snapshot

- Purpose: Structured snapshot of a not-yet-sent or previously sent draft as observed or
  changed by a mail operation.
- Fields:
  - `draftId`
  - `threadId`
  - `operationId`
  - `integrationId`
  - `mailAccountId`
  - `composeMode`: `new_message`, `reply`, or `forward`
  - `sourceMessageId`
  - `recipientSummary`
  - `subject`
  - `bodyPreview`
  - `attachmentRefIds`
  - `draftStatus`: `draft`, `updated`, `send_blocked`, `sent_from_draft`,
    `stale_snapshot`, or `unavailable`
  - `createdAt`
  - `updatedAt`
- Validation rules:
  - one draft snapshot belongs to exactly one operation record
  - `sent_from_draft` is valid only after a successful `send_draft` operation
  - `send_blocked` is valid when unresolved attachment references or missing explicit
    recipients prevent final send
  - draft identity must remain stable across create, update, and send-from-draft flows

### Attachment Reference

- Purpose: Operator-visible description of an attachment associated with a draft or
  message action, including metadata and attachment-specific failure truth.
- Fields:
  - `attachmentRefId`
  - `parentKind`: `draft` or `message`
  - `parentId`
  - `displayName`
  - `mediaType`
  - `sizeBytes`
  - `resolutionStatus`: `resolved`, `unresolved`, or `failed`
  - `failureReason`
  - `createdAt`
- Validation rules:
  - attachment references stay metadata-only in phase 30 and do not require persisted
    binary content
  - a final-send request that depends on `unresolved` or `failed` attachment references
    must not reach a `sent` result

### Mail Operation Summary

- Purpose: Additive lightweight projection attached to runtime tool calls, workflow
  steps, schedule attempts, and delivery outcomes so operators can locate mail-domain
  truth from execution records.
- Fields:
  - `operationId`
  - `operationClass`
  - `integrationId`
  - `threadId`
  - `messageId`
  - `draftId`
  - `resultMode`
  - `sendPath`
  - `status`
  - `capturedAt`
- Validation rules:
  - summaries are immutable snapshots once attached to a tool call, workflow step,
    schedule attempt, or delivery outcome
  - summaries must never replace the authoritative mail operation record

## State Transitions

### Mail Operation Lifecycle

- `requested` -> `completed` when the mail-domain action finishes successfully and
  records its resulting artifacts
- `requested` -> `blocked` when readiness, explicit-recipient requirements, attachment
  resolution, or workflow send-permission rules prevent execution before final send
- `requested` -> `failed` when the backend returns stale-state, unavailable, or other
  terminal failure truth
- `requested` -> `cancelled` when the enclosing workflow or run is cancelled before the
  action finishes

### Mail Draft Lifecycle

- `draft` -> `updated` when an update operation changes draft content
- `draft` or `updated` -> `sent_from_draft` when a successful `send_draft` operation is
  recorded
- `draft` or `updated` -> `send_blocked` when the attempted final send is prevented by
  missing explicit recipients, unresolved attachments, or background-send policy
- any state -> `stale_snapshot` when historical draft truth no longer matches the
  current backend draft state
- any state -> `unavailable` when draft detail cannot be refreshed because mailbox
  readiness or backend access is lost

## Relationships

- one integration resource may back one current mail account projection per environment
  and many mail operations over time
- one mail account projection may be referenced by many mail operations
- one mail operation may own zero or more thread, message, draft, and attachment
  artifacts
- one thread snapshot may reference many message snapshots and draft snapshots
- one run, tool call, workflow step, schedule attempt, or delivery outcome may
  reference many mail operations over time through additive summary projections

## Derived Views

- mail account views combine integration readiness with mailbox identity and capability
  truth
- mail operation list views can filter by `integrationId`, `runId`, `workflowId`,
  `scheduleId`, `deliveryId`, `operationClass`, `resultMode`, `threadId`, `messageId`,
  or `draftId`
- workflow, tool-call, schedule, and delivery detail views can project
  `mailOperationSummaries` beside existing `integrationBindings` and
  `calendarOperationSummaries`
