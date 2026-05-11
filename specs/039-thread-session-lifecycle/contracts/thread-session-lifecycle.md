# Contract: Daemon-Owned Thread And Session Lifecycle

## Scope

This contract defines the Phase 54 product surfaces for daemon-owned thread/session
lifecycle. Existing session routing, connector setup, connector diagnostics, run,
workflow, approval, delivery, and channel conformance contracts remain authoritative for
their own domains.

Required surfaces:

- API routes and schemas
- TypeScript SDK types and methods
- Web product flows
- Operator shell/TUI views
- Event/audit evidence
- Metadata-only lifecycle/source/runtime projection evidence

Out of scope:

- Memory recall
- Context packing
- Semantic summaries
- Autonomous conversation pruning
- Runtime cancellation as a side effect of archive
- Raw provider payload display
- Disallowed message-body display

## Permissions

| Action | Required Permission |
|--------|---------------------|
| List threads | `credentials.inspect` |
| View thread detail | `credentials.inspect` |
| View source linkage metadata | `credentials.inspect` |
| View runtime projection metadata | `credentials.inspect` |
| Reset thread | `connectors.manage` |
| Archive thread | `connectors.manage` |
| Reopen thread | `connectors.manage` |

Unauthorized requests MUST return stable denials without exposing inaccessible thread
existence, source identity, session state, or runtime evidence.

## List Contract

### API

`GET /v1/threads`

Query parameters:

- `limit`: Optional positive integer. Default is `20`.
- `cursor`: Optional opaque cursor returned by the previous page.
- `state`: Optional lifecycle state filter: `active`, `reset`, `archived`, or
  `reopened`.
- `sourceKind`: Optional source kind filter: `chat`, `channel`, `workflow`, `schedule`,
  `shell`, or `legacy`.

Response shape:

```json
{
  "tenantId": "ten_123",
  "page": {
    "limit": 20,
    "nextCursor": "opaque_cursor",
    "order": "active_recent_archived_id"
  },
  "items": [
    {
      "threadId": "thr_123",
      "tenantId": "ten_123",
      "lifecycleState": "active",
      "sourceKind": "channel",
      "sourceSummary": "Slack Main / #support",
      "currentSessionSegmentId": "seg_123",
      "currentSessionId": "sess_123",
      "lastActivityAt": "2026-05-11T10:00:00Z",
      "availableActions": ["reset", "archive"],
      "redactionStatus": "redacted",
      "retentionExpiresAt": "2026-08-09T10:00:00Z",
      "updatedAt": "2026-05-11T10:00:00Z"
    }
  ]
}
```

Ordering:

1. Active, reset, and reopened threads before archived threads by default.
2. Descending `lastActivityAt`.
3. Stable `threadId`.

Acceptance:

- Tenants with more than one page of threads can reach all threads across pages.
- Paginated traversal has no missing or duplicated thread entries.
- Page items are metadata-only and redacted.

## Detail Contract

### API

`GET /v1/threads/{threadId}`

Response shape:

```json
{
  "thread": {
    "threadId": "thr_123",
    "tenantId": "ten_123",
    "lifecycleState": "active",
    "sourceKind": "channel",
    "sourceSummary": "Slack Main / #support",
    "currentSessionSegmentId": "seg_123",
    "currentSessionId": "sess_123",
    "lastActivityAt": "2026-05-11T10:00:00Z",
    "availableActions": ["reset", "archive"],
    "redactionStatus": "redacted",
    "retentionExpiresAt": "2026-08-09T10:00:00Z",
    "updatedAt": "2026-05-11T10:00:00Z"
  },
  "sessionSegments": [
    {
      "sessionSegmentId": "seg_123",
      "sessionId": "sess_123",
      "state": "active",
      "generation": 2,
      "startedAt": "2026-05-11T10:00:00Z",
      "lastActiveAt": "2026-05-11T10:05:00Z",
      "partialEvidence": false
    }
  ],
  "sourceLinkages": [
    {
      "sourceLinkageId": "src_123",
      "sourceKind": "channel",
      "connectorId": "slack-main",
      "connectorKind": "slack",
      "sourceAccountId": "workspace_redacted",
      "sourceConversationId": "conversation_redacted",
      "sourceMessageId": "message_redacted",
      "routingOutcome": "accepted",
      "current": true,
      "redactionStatus": "redacted"
    }
  ],
  "runtimeProjections": [
    {
      "runtimeProjectionId": "rtp_123",
      "resourceKind": "run",
      "resourceId": "run_123",
      "status": "completed",
      "occurredAt": "2026-05-11T10:02:00Z",
      "safeSummary": "Assistant run completed",
      "redactionStatus": "redacted"
    }
  ],
  "lifecycleActions": [
    {
      "lifecycleActionId": "act_123",
      "actionKind": "reset",
      "priorState": "active",
      "resultingState": "reset",
      "priorSessionSegmentId": "seg_old",
      "resultingSessionSegmentId": "seg_123",
      "status": "succeeded",
      "auditEventId": "audit_123",
      "completedAt": "2026-05-11T10:00:00Z"
    }
  ]
}
```

Rules:

- The response MUST NOT include raw provider payloads, secrets, or disallowed message
  bodies.
- Runtime projections MUST remain separate facts for sessions, runs, workflows,
  approvals, foreground replies, background deliveries, and connector messages.
- Legacy or incomplete sessions MUST be labeled with `partialEvidence`.

## Reset Contract

`POST /v1/threads/{threadId}/reset`

Request:

```json
{
  "reasonCode": "user_requested_reset",
  "note": "optional metadata-only operator note"
}
```

Response:

```json
{
  "threadId": "thr_123",
  "lifecycleState": "reset",
  "previousSessionSegmentId": "seg_old",
  "currentSessionSegmentId": "seg_new",
  "auditEventId": "audit_123",
  "changedAt": "2026-05-11T10:00:00Z"
}
```

Rules:

- Requires `connectors.manage`.
- Preserves the same thread ID.
- Creates a new active session segment for future work.
- Does not delete, rewrite, or hide historical messages, sessions, runs, workflows,
  approvals, replies, deliveries, or audit evidence.
- Required audit evidence must be recorded before state changes.
- If audit write fails, return a failure and leave lifecycle state unchanged.

## Archive Contract

`POST /v1/threads/{threadId}/archive`

Rules:

- Requires `connectors.manage`.
- Blocks future continuation from users, channel connectors, scheduled work, workflows,
  or retries unless the thread is reopened or a new thread is created.
- Does not cancel active or pending runs, approvals, workflows, foreground replies, or
  background deliveries already accepted before archive.
- Archived threads remain inspectable by authorized users.
- Required audit evidence must be recorded before state changes.

## Reopen Contract

`POST /v1/threads/{threadId}/reopen`

Rules:

- Requires `connectors.manage`.
- Allowed only when tenant, source, connector, and session rules still allow
  continuation.
- Preserves archive and reopen evidence.
- Does not erase or collapse prior lifecycle history.
- Required audit evidence must be recorded before state changes.

## Source Continuation Contract

For inbound channel connectors, current thread identity is keyed by:

```text
tenantId + connectorId + sourceAccountId + sourceConversationId
```

Rules:

- At most one current thread may exist for this key.
- Later accepted inbound messages for the same key attach to the same current thread
  unless lifecycle state blocks continuation or a lifecycle action creates a new eligible
  current thread.
- Source message ID is event evidence for the inbound message and duplicate detection,
  not the thread identity.
- Duplicate, ignored, blocked, disabled, unsupported, and failed inbound messages must
  produce routing evidence without creating misleading assistant work evidence.

## Event Contract

Add event schemas for:

- `thread.lifecycle_reset`
- `thread.lifecycle_archived`
- `thread.lifecycle_reopened`
- `thread.source_linked`
- `thread.runtime_projection_recorded`
- `thread.retention_applied`
- `thread.redaction_failed`
- `thread.audit_failed_closed`

Common payload fields:

```json
{
  "tenantId": "ten_123",
  "threadId": "thr_123",
  "sessionSegmentId": "seg_123",
  "action": "reset",
  "outcome": "succeeded",
  "auditEventId": "audit_123",
  "reasonCode": "user_requested_reset",
  "redactionStatus": "redacted"
}
```

Event rules:

- Mutation events require audit linkage.
- Redaction failures suppress unsafe detail and emit a safe reason class.
- Retention application events remain metadata-only.

## SDK Contract

Add TypeScript SDK types and methods:

- `ThreadResource`
- `ThreadDetailResponse`
- `ThreadListResponse`
- `ThreadLifecycleActionInput`
- `ThreadLifecycleActionResponse`
- `ThreadSourceLinkage`
- `ThreadRuntimeProjection`
- `listThreads(query, tenantOptions)`
- `getThread(threadId, tenantOptions)`
- `resetThread(threadId, input, tenantOptions)`
- `archiveThread(threadId, input, tenantOptions)`
- `reopenThread(threadId, input, tenantOptions)`

SDK methods MUST preserve tenant headers and return stable denial errors without leaking
resource existence.

## Web And Operator Shell/TUI Contract

Required views:

- Thread list with lifecycle state, source kind, source summary, current session, last
  activity, available actions, empty state, loading state, error state, and pagination.
- Thread detail with session segments, source linkage, runtime projections, lifecycle
  action history, retention/redaction metadata, and metadata-only support evidence.
- Reset/archive/reopen controls gated by available actions and denial states.
- Operator trace view from source message to thread, session, run or workflow, approval,
  reply, and delivery facts.

Views MUST make lifecycle metadata distinct from memory recall. They MUST NOT imply that
historical evidence becomes assistant memory, semantic summary, or context-packing input.

## Compatibility And Rollback

- Existing `/v1/sessions`, session schemas, and session events remain compatible.
- Existing connector ingress, duplicate detection, run creation, workflow records,
  approval records, and delivery records remain authoritative.
- Rollback disables new `/v1/threads` routes and client entry points while retaining
  already-written additive evidence until retention expiry.
- Additive thread tables remain inert on rollback and must not require destructive
  removal.

## Required Tests

- Store migration and restart tests for lifecycle/source/runtime projection state.
- Tenant-safe store tests for cross-tenant denial and partial legacy sessions.
- API tests for list/detail/reset/archive/reopen, permission denials, pagination,
  redaction, retention, and audit fail-closed behavior.
- Connector ingress regression proving accepted messages attach to daemon-owned current
  thread truth.
- Runtime projection tests linking source messages to sessions, runs, workflows,
  approvals, foreground replies, and background deliveries.
- Contract tests for API and event schemas.
- SDK, web, and TUI/operator shell tests for list, detail, lifecycle actions, denials,
  and non-memory labeling.
