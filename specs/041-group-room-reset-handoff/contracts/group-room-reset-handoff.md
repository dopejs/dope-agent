# Contract: Group Room Reset Handoff

## Scope

This contract defines the Phase 56 product surfaces for group, room, reset, and
cross-surface handoff semantics. Roadmap 54 remains authoritative for daemon-owned
thread/session lifecycle, permissions, reset mechanics, and thread inspection. Roadmap 55
remains authoritative for bounded recent-turn continuity and continuity preview evidence.

Required surfaces:

- Conversation shape and room identity in thread/source evidence
- Group/room participation policy and participation decisions
- Source-specific reset evidence for direct-message, group, room, and web-originated
  threads
- Handoff creation between supported channel and web shell surfaces
- Thread detail projection of participation, reset, and handoff evidence
- First-destination-response handoff source-turn references
- TypeScript SDK types and methods
- Web thread detail/handoff controls and TUI/operator inspection views
- Event/audit evidence
- JSON schemas and contract fixtures
- Connector conformance additions

Out of scope:

- Group memory
- Team knowledge base behavior
- Semantic retrieval or cross-room recall
- Summaries
- Autonomous agent-to-agent delegation
- Long-term personalization
- Raw provider payload display
- Disallowed message-body display

## Permissions

| Action | Required Permission |
|--------|---------------------|
| View conversation shape and safe participation evidence | Roadmap 54 thread inspection permission |
| View handoff link from an accessible side | Roadmap 54 thread inspection permission for that side |
| Reset direct-message, group, room, or web-originated thread | `connectors.manage` |
| Create handoff | `connectors.manage` plus source and destination eligibility |
| Use first-response handoff source references | Successful handoff plus source and destination permissions |

Unauthorized requests MUST return stable denial without exposing inaccessible thread
existence, source identity, participant identity, message content, reset history, handoff
destination, source-turn references, or runtime evidence.

## Conversation Shape Contract

Thread detail and source linkage projections add conversation shape metadata:

```json
{
  "conversationShape": {
    "shape": "room",
    "sourceKind": "channel",
    "connectorId": "slack-main",
    "connectorKind": "slack",
    "sourceAccountId": "workspace_redacted",
    "sourceConversationId": "channel_redacted",
    "sourceConversationSummary": "Slack Main / #support",
    "shapeEvidenceStatus": "proven",
    "redactionStatus": "redacted"
  }
}
```

Allowed `shape` values:

- `direct_message`
- `group`
- `room`
- `web`
- `unknown`
- `unsupported`

Rules:

- Supported group, room, direct-message, and web behavior requires `shapeEvidenceStatus:
  proven`.
- `unknown` and `unsupported` shapes must not receive implicit group participation,
  source-specific reset, or handoff behavior.
- Room isolation is based on stable source identity, not display name or participant
  overlap.

## Participation Policy And Decision Contract

Default group/room policy:

1. Source must be allowlist-eligible.
2. Source message must contain a qualifying mention.
3. Tenant, connector, and participant permission must allow routing.
4. Duplicate/replayed/edited/deleted/unsupported/failed source messages must not create
   accepted participation.

Thread detail may include recent participation decisions:

```json
{
  "participationDecisions": [
    {
      "participationDecisionId": "part_123",
      "threadId": "thr_123",
      "sessionSegmentId": "seg_123",
      "conversationShape": "room",
      "decision": "ignored",
      "reasonCode": "missing_qualifying_mention",
      "mentionStatus": "missing",
      "allowlistStatus": "eligible",
      "createdAssistantWork": false,
      "safeSummary": "Room message ignored by participation policy",
      "occurredAt": "2026-05-11T10:00:00Z",
      "redactionStatus": "redacted"
    }
  ]
}
```

Decision values:

- `accepted`
- `ignored`
- `blocked`
- `denied`
- `duplicate`
- `unsupported`
- `failed`

Reason code examples:

- `accepted_qualifying_mention`
- `missing_qualifying_mention`
- `not_allowlisted`
- `permission_denied`
- `duplicate_source_event`
- `unsupported_conversation_shape`
- `redaction_failed`
- `incomplete_source_identity`
- `connector_disabled`
- `connector_failed`

Rules:

- Non-accepted decisions must not create assistant turns, handoff links, or misleading
  continuity evidence.
- Decisions expose safe summaries and classifications only.
- Reason code additions require schema, SDK, fixture, and contract updates.

## Reset Contract

Roadmap 54 reset route remains authoritative:

`POST /v1/threads/{threadId}/reset`

Additive behavior:

- Reset evidence includes conversation shape and source scope where available.
- Reset requires `connectors.manage`.
- Reset of one source or conversation shape must not reset unrelated threads.
- Reset excludes pre-reset source turns from future continuity and handoff source
  references while preserving historical evidence where allowed.

Thread detail reset evidence example:

```json
{
  "resetEvents": [
    {
      "resetEventId": "reset_123",
      "threadId": "thr_group",
      "conversationShape": "group",
      "priorSessionSegmentId": "seg_old",
      "resultingSessionSegmentId": "seg_new",
      "status": "succeeded",
      "permissionGate": "connectors.manage",
      "changedAt": "2026-05-11T10:00:00Z",
      "redactionStatus": "redacted"
    }
  ]
}
```

## Handoff Creation Contract

`POST /v1/threads/{threadId}/handoffs`

Request:

```json
{
  "destination": {
    "surface": "web",
    "connectorId": null,
    "sourceAccountId": null,
    "sourceConversationId": null
  },
  "reasonCode": "user_requested_handoff"
}
```

Channel destination request:

```json
{
  "destination": {
    "surface": "channel",
    "connectorId": "slack-main",
    "sourceAccountId": "workspace_redacted",
    "sourceConversationId": "channel_redacted",
    "conversationShape": "room"
  },
  "reasonCode": "user_requested_handoff"
}
```

Response:

```json
{
  "handoffLinkId": "handoff_123",
  "sourceThreadId": "thr_source",
  "destinationThreadId": "thr_destination",
  "sourceSessionSegmentId": "seg_source",
  "destinationSessionSegmentId": "seg_destination",
  "sourceConversationShape": "room",
  "destinationConversationShape": "web",
  "status": "succeeded",
  "sourceReferenceStatus": "available",
  "permissionGate": "connectors.manage",
  "createdAt": "2026-05-11T10:00:00Z",
  "redactionStatus": "redacted"
}
```

Rules:

- Requires `connectors.manage`.
- `destinationThreadId` must differ from `sourceThreadId`.
- Handoff creates or selects a separate destination thread.
- Source and destination tenant, connector, participant, lifecycle, and destination
  surface eligibility must be proven before success.
- Denied handoff must not silently create a destination conversation.
- Handoff does not merge thread identities and does not copy source turns into
  destination history.

## Handoff Source Reference Contract

Successful handoff may create source-turn references for the first destination response.

```json
{
  "handoffSourceReferences": [
    {
      "handoffSourceReferenceId": "href_123",
      "handoffLinkId": "handoff_123",
      "sourceThreadId": "thr_source",
      "destinationThreadId": "thr_destination",
      "continuityTurnId": "turn_source_123",
      "decision": "referenced",
      "eligibilityStatus": "eligible",
      "safeSummary": "Source user asked about deployment options",
      "redactionStatus": "redacted"
    }
  ]
}
```

Rules:

- References are by identity and safe summary; source turn content is not copied into
  destination thread history.
- References are eligible only for source turns in the current segment after the latest
  reset boundary.
- References are subject to source permission, destination permission, redaction,
  retention, duplicate detection, and Roadmap 55 continuity eligibility.
- References are available only for the first destination response after handoff.
- After the first destination response, destination continuity uses only destination
  thread turns unless another authorized handoff occurs.
- First-response continuity preview evidence must classify included and excluded handoff
  references.

## Thread Detail Projection

`GET /v1/threads/{threadId}`

Additive thread detail fields:

```json
{
  "conversationShape": {
    "shape": "room",
    "shapeEvidenceStatus": "proven",
    "redactionStatus": "redacted"
  },
  "participationDecisions": [],
  "resetEvents": [],
  "handoffLinks": [
    {
      "handoffLinkId": "handoff_123",
      "direction": "outbound",
      "sourceThreadId": "thr_source",
      "destinationThreadId": "thr_destination",
      "status": "succeeded",
      "sourceReferenceStatus": "consumed",
      "createdAt": "2026-05-11T10:00:00Z",
      "redactionStatus": "redacted"
    }
  ]
}
```

Rules:

- Thread detail may return bounded recent evidence summaries.
- Full handoff/source-reference detail may be served by a dedicated handoff detail route
  if needed by implementation.
- Users who can inspect only one side of a handoff receive safe detail for that side and
  suppression for inaccessible side fields.

## Event Contract

Add event schemas for:

- `thread.participation_decision_recorded`
- `thread.reset_scoped`
- `thread.handoff_linked`

Common payload fields:

```json
{
  "tenantId": "ten_123",
  "threadId": "thr_123",
  "sessionSegmentId": "seg_123",
  "eventKind": "thread.handoff_linked",
  "resourceId": "handoff_123",
  "status": "succeeded",
  "reasonCode": "user_requested_handoff",
  "permissionGate": "connectors.manage",
  "redactionStatus": "redacted",
  "occurredAt": "2026-05-11T10:00:00Z"
}
```

Rules:

- Events are metadata-only and tenant-scoped.
- Events must not expose raw provider payloads, unsafe message bodies, secrets, or
  cross-tenant identifiers.
- Restart recovery must preserve enough event/evidence state to explain final status.

## Connector Conformance Contract

Connectors claiming group/room/reset/handoff support must provide conformance evidence
for:

- Conversation shape classification.
- Stable source account and source conversation identity.
- Room rename/delete/recreate behavior where available.
- Qualifying mention detection.
- Allowlist policy evaluation.
- Duplicate/replay/edit/delete handling.
- Safe source summaries.
- Reset support or explicit unsupported classification.
- Handoff source support or explicit unsupported classification.
- Handoff destination support or explicit unsupported classification.

Unsupported connectors must continue existing routing behavior and expose safe
unsupported evidence without implicit group or handoff semantics.

## SDK, Web, And TUI Expectations

TypeScript SDK:

- Exposes additive conversation shape, participation decision, reset event, handoff link,
  and handoff creation types.
- Preserves existing thread lifecycle and continuity method compatibility.
- Preserves tenant headers and stable permission-denial behavior.

Web:

- Thread detail shows conversation shape, participation decision summaries, reset events,
  and handoff links.
- Handoff controls are available only when the current user has mutation permission and
  the source/destination are eligible.
- UI copy describes handoff as traceable continuation, not memory.

TUI/operator shell:

- Thread detail output includes safe group/room participation, reset, and handoff
  evidence.
- Handoff creation command or equivalent route invocation requires explicit destination
  and reports denied/unsupported outcomes safely.

## Compatibility And Rollback

- Existing chat and connector calls without group/room/handoff support remain compatible.
- Existing Roadmap 54 reset/archive/reopen behavior remains authoritative.
- Existing Roadmap 55 continuity remains unchanged except for first-response handoff
  references after authorized handoff.
- Rollback disables handoff creation, group participation acceptance, and additive
  client controls while preserving metadata evidence until retention expiry.
- No source turns are copied, no thread identities are merged, and no destructive
  migration is required.
