# Contract: Non-Knowledge Multi-Turn Continuity

## Scope

This contract defines the Phase 55 product surfaces for bounded, inspectable recent-turn
continuity. Roadmap 54 thread/session lifecycle remains authoritative for thread
identity, lifecycle mutations, source linkage, permissions, and reset/archive/reopen
state.

Required surfaces:

- Chat query and streaming query request/response additions
- Thread detail continuity preview summaries
- Dedicated continuity preview inspection route
- TypeScript SDK types and methods
- Web thread detail and TUI/operator inspection views
- Event/audit evidence
- JSON schemas and contract fixtures

Out of scope:

- Memory writes or memory recall
- Semantic retrieval or knowledge graph behavior
- Summarization
- Autonomous context packing
- Long-term preference learning
- Cross-thread personalization
- Raw provider payload display
- Disallowed message-body display

## Permissions

| Action | Required Permission |
|--------|---------------------|
| Use continuity for a valid active thread | Existing chat/channel authorization plus thread ownership |
| View continuity preview summary | `credentials.inspect` |
| View continuity preview detail | `credentials.inspect` |
| Reset thread continuity | `connectors.manage` through Roadmap 54 reset |

Unauthorized inspection MUST return stable denial without exposing inaccessible thread
existence, source identity, included content, excluded content, artifact excerpts, or
runtime evidence.

## Chat Query Contract

### Request

`POST /v1/chat/query`

Additive request fields:

```json
{
  "query": "What about the second option?",
  "provider": "echo",
  "model": "echo-v1",
  "threadId": "thr_123",
  "continuity": {
    "mode": "auto"
  }
}
```

Rules:

- `threadId` is optional. Requests without `threadId` keep existing single-turn
  behavior.
- `continuity.mode` is optional and defaults to `auto`.
- Supported values: `auto`, `disabled`.
- `auto` uses bounded continuity only when the thread is active/reopened/reset-eligible,
  tenant-scoped, and has a current session segment.
- `disabled` records a preview with `continuityApplied: false` when `threadId` is
  present, and assembles no prior turns.
- The server MUST NOT infer continuity from tenant ID, user identity, message text,
  client-local history, provider context, or channel-local state.

### Response

Additive response fields:

```json
{
  "dispatchId": "dispatch_123",
  "threadId": "thr_123",
  "sessionSegmentId": "seg_123",
  "requestTurnId": "turn_user_123",
  "responseTurnId": "turn_assistant_123",
  "continuityPreviewId": "contprev_123",
  "continuityApplied": true,
  "continuityStatus": "applied",
  "continuityIncludedCount": 4,
  "continuityExcludedCount": 2
}
```

Rules:

- Existing required response fields remain unchanged.
- New fields are omitted when no thread identity was supplied or resolved.
- `continuityStatus` values: `applied`, `empty`, `disabled`, `blocked`, `partial`,
  `failed`.
- A response that evaluates continuity for a thread MUST persist a preview before the
  terminal response is returned.

## Streaming Chat Contract

`POST /v1/chat/query/stream`

Rules:

- Request fields match non-stream chat.
- `chat.query.started` includes additive continuity metadata when available:
  `threadId`, `sessionSegmentId`, `requestTurnId`, `continuityPreviewId`,
  `continuityApplied`, and `continuityStatus`.
- `chat.query.completed`, `chat.query.failed`, `chat.query.cancelled`, and
  `chat.query.partial_failed` use the same terminal response shape as non-stream chat.
- Delta events do not repeat preview item details.

## Continuity Preview Summary In Thread Detail

`GET /v1/threads/{threadId}`

Add `continuityPreviews` to thread detail:

```json
{
  "continuityPreviews": [
    {
      "continuityPreviewId": "contprev_123",
      "dispatchId": "dispatch_123",
      "requestTurnId": "turn_user_123",
      "responseTurnId": "turn_assistant_123",
      "sessionSegmentId": "seg_123",
      "continuityApplied": true,
      "status": "applied",
      "includedCount": 4,
      "excludedCount": 2,
      "windowPolicyId": "default_recent_12_30d",
      "assemblyDurationMs": 37,
      "createdAt": "2026-05-11T10:00:00Z",
      "retentionExpiresAt": "2026-08-09T10:00:00Z",
      "redactionStatus": "redacted"
    }
  ]
}
```

Rules:

- Thread detail may return only recent preview summaries to keep the response bounded.
- Full included/excluded item detail is served by the dedicated preview route.
- Preview summaries are metadata-only and require `credentials.inspect`.

## Continuity Preview Detail Route

`GET /v1/threads/{threadId}/continuity-previews/{previewId}`

Response:

```json
{
  "preview": {
    "continuityPreviewId": "contprev_123",
    "tenantId": "ten_123",
    "threadId": "thr_123",
    "sessionSegmentId": "seg_123",
    "dispatchId": "dispatch_123",
    "requestTurnId": "turn_user_123",
    "windowPolicyId": "default_recent_12_30d",
    "maxPriorTurns": 12,
    "activeWindowDays": 30,
    "orderedBy": "daemon_acceptance_sequence",
    "continuityApplied": true,
    "status": "applied",
    "includedCount": 2,
    "excludedCount": 1,
    "assemblyDurationMs": 37,
    "redactionStatus": "redacted"
  },
  "items": [
    {
      "previewItemId": "contitem_1",
      "itemKind": "turn",
      "decision": "included",
      "reasonCode": "included_recent",
      "continuityTurnId": "turn_1",
      "role": "user",
      "acceptanceSequence": 8,
      "safeSummary": "User asked about option A",
      "redactionStatus": "redacted"
    },
    {
      "previewItemId": "contitem_2",
      "itemKind": "artifact_excerpt",
      "decision": "included",
      "reasonCode": "included_recent",
      "continuityTurnId": "turn_2",
      "artifactRef": "artifact_123",
      "safeSummary": "Visible excerpt from prior result",
      "redactionStatus": "redacted"
    },
    {
      "previewItemId": "contitem_3",
      "itemKind": "turn",
      "decision": "excluded",
      "reasonCode": "over_limit",
      "continuityTurnId": "turn_old",
      "acceptanceSequence": 1,
      "safeSummary": "Excluded by continuity limit",
      "redactionStatus": "redacted"
    }
  ]
}
```

Rules:

- Included items are ordered oldest-to-newest by daemon acceptance sequence.
- Excluded items expose stable reasons and safe summaries only.
- Redaction failures suppress unsafe detail and use reason `redaction_failed`.
- Expired previews return not found or a stable expired response according to existing
  retention conventions; they must not be used for normal inspection.

## Inclusion Policy

Default policy: `default_recent_12_30d`

Rules:

1. Consider only the current tenant, thread, and session segment after the most recent
   reset boundary.
2. Exclude the current user input.
3. Exclude archived lifecycle blocks unless reopened according to Roadmap 54 rules.
4. Exclude turns older than 30 active-continuity days unless authorized tenant policy
   changes the active window.
5. Exclude turns or excerpts with unsafe redaction state.
6. Exclude duplicate or replayed source events.
7. Select no more than the 12 most recent eligible prior turns.
8. Assemble selected turns oldest-to-newest by daemon acceptance sequence.
9. Include artifact content only as user-visible, safely redacted excerpts tied to
   included turns.

## Event Contract

Add event schemas for:

- `thread.continuity_turn_recorded`
- `thread.continuity_preview_recorded`

Common payload fields:

```json
{
  "tenantId": "ten_123",
  "threadId": "thr_123",
  "sessionSegmentId": "seg_123",
  "continuityTurnId": "turn_123",
  "continuityPreviewId": "contprev_123",
  "dispatchId": "dispatch_123",
  "action": "preview_recorded",
  "outcome": "applied",
  "reasonCode": "included_recent",
  "redactionStatus": "redacted"
}
```

Rules:

- Events are metadata-only.
- Event payloads must not include raw prompt text, raw provider payloads, secrets,
  disallowed message bodies, or cross-tenant identifiers.
- Contract tests must cover schema sync between Go event constructors, `schemas/events`,
  and fixtures.

## SDK Contract

Additive TypeScript types:

- `ChatQueryInput.threadId?: string`
- `ChatQueryInput.continuity?: { mode?: "auto" | "disabled" }`
- `ChatQueryResponse.threadId?: string`
- `ChatQueryResponse.sessionSegmentId?: string`
- `ChatQueryResponse.requestTurnId?: string`
- `ChatQueryResponse.responseTurnId?: string`
- `ChatQueryResponse.continuityPreviewId?: string`
- `ChatQueryResponse.continuityApplied?: boolean`
- `ChatQueryResponse.continuityStatus?: "applied" | "empty" | "disabled" | "blocked" | "partial" | "failed"`
- `getThreadContinuityPreview(threadId, previewId, tenantOptions?)`

Existing SDK chat calls remain source-compatible.

## Web And TUI Contract

Web:

- Thread detail shows continuity preview summaries.
- A preview detail view shows included/excluded references and reason codes.
- Chat input can continue a selected thread when thread identity is available.
- UI labels continuity as recent-thread evidence, not memory.

TUI/operator shell:

- Existing thread detail output includes recent preview IDs and statuses.
- New inspection command can fetch one preview detail.
- Chat command can pass an explicit thread ID for bounded continuity.
- Denials must not print stale preview content.

## Compatibility

- Existing chat requests without `threadId` remain single-turn.
- Existing thread lifecycle routes remain compatible when no continuity rows exist.
- Existing Roadmap 54 reset/archive/reopen behavior remains authoritative.
- Existing clients may ignore additive response fields.
- Schema changes must be additive and covered by contract fixtures.

## Failure Classes

Stable classifications:

- `continuity_unavailable`
- `no_eligible_turns`
- `lifecycle_blocked`
- `reset_boundary`
- `source_mismatch`
- `permission_denied`
- `redaction_failed`
- `retention_expired`
- `duplicate_source_event`
- `incomplete_evidence`
- `unsupported_source`
- `continuity_disabled`

## Verification Requirements

- Go tests prove bounded inclusion, reset exclusion, archive/reopen blocking, artifact
  excerpt limits, duplicate/replay suppression, daemon acceptance ordering, restart
  recovery, redaction failure, retention expiry, and p95 latency.
- Contract tests validate all API and event schemas.
- SDK/Web/TUI tests cover additive fields, preview inspection, denials, and single-turn
  compatibility.
- Redaction validation finds zero secrets, raw provider payloads, disallowed message
  bodies, unsafe channel metadata, or cross-tenant identifiers.

## Fixture Inventory

Contract fixtures required for implementation:

- `daemon/internal/contracts/testdata/thread_continuity/chat-query-continuity.json`
  validates additive chat request/response continuity fields.
- `daemon/internal/contracts/testdata/thread_continuity/thread-continuity-preview.json`
  validates continuity preview summary/detail payloads and safe item evidence.
- `daemon/internal/contracts/testdata/thread_continuity/thread-continuity-events.json`
  validates metadata-only continuity turn and preview event payloads.
