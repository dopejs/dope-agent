# Data Model: Non-Knowledge Multi-Turn Continuity

## Continuity Turn

Represents one accepted user or assistant turn that may be eligible for bounded
recent-thread continuity.

### Fields

- `continuityTurnId`: Stable turn identity.
- `tenantId`, `threadId`, `sessionSegmentId`: Roadmap 54 ownership and reset boundary.
- `acceptanceSequence`: Daemon-assigned per-thread sequence used for continuity order.
- `role`: `user` or `assistant`.
- `sourceKind`: `chat`, `channel`, `workflow`, `schedule`, `shell`, or `legacy`.
- `sourceLinkageId`: Optional source linkage record for channel-origin turns.
- `sourceMessageId`: Optional source message identity for duplicate/replay evidence.
- `sourceTimestamp`: Optional provider/source timestamp retained as evidence only.
- `dispatchId`: LLM dispatch associated with the assistant response where applicable.
- `responseToTurnId`: Prior user turn answered by an assistant turn where applicable.
- `safeContent`: Redacted user-visible text eligible for bounded inclusion.
- `contentRedactionStatus`: `redacted`, `suppressed`, or `redaction_failed`.
- `artifactExcerptRefs`: References to user-visible, redacted artifact excerpts tied to
  this turn.
- `recordedAt`: Daemon acceptance time.
- `retentionExpiresAt`: Normal inspection expiry.
- `documentJson`: Full metadata document matching schema and redaction constraints.

### Relationships

- Belongs to one `Thread` and one `Session Segment`.
- May link to one `Source Linkage`.
- May link to one LLM dispatch, run, workflow, connector message, reply, delivery, or
  artifact reference through Roadmap 54 runtime projections.
- May appear in many `Continuity Preview Item` rows as included or excluded evidence.

### Validation Rules

- `tenantId`, `threadId`, `sessionSegmentId`, `acceptanceSequence`, `role`, and
  `recordedAt` are required.
- `(tenantId, threadId, acceptanceSequence)` must be unique.
- Turns before the current reset boundary are excluded from future continuity.
- Turns for archived threads are not eligible until lifecycle rules reopen or replace
  the thread.
- Unsafe or unredactable content must be suppressed and may only appear as safe
  classification metadata.

## Continuity Window

Represents the deterministic policy used to choose prior turns for one response.

### Fields

- `windowPolicyId`: Policy identifier, such as `default_recent_12_30d`.
- `maxPriorTurns`: Default `12`.
- `activeWindowDays`: Default `30`.
- `threadId`, `tenantId`, `sessionSegmentId`.
- `resetBoundarySegmentId`: Current session segment boundary.
- `orderedBy`: `daemon_acceptance_sequence`.
- `includeArtifactExcerpts`: Boolean; true only for safe user-visible excerpts tied to
  included turns.

### Validation Rules

- Excludes the current user input.
- Includes only the most recent eligible prior turns after the current reset boundary.
- Preserves oldest-to-newest daemon acceptance sequence order in dispatch input.
- Applies tenant policy only when it explicitly narrows or extends active-continuity
  eligibility.

## Continuity Preview

Represents response-specific evidence explaining continuity assembly.

### Fields

- `continuityPreviewId`: Stable preview identity.
- `tenantId`, `threadId`, `sessionSegmentId`.
- `dispatchId`: LLM dispatch associated with the response.
- `requestTurnId`: Current user turn that triggered assembly.
- `windowPolicyId`, `maxPriorTurns`, `activeWindowDays`.
- `includedCount`, `excludedCount`.
- `continuityApplied`: Boolean.
- `status`: `applied`, `empty`, `disabled`, `blocked`, `partial`, or `failed`.
- `failureClass`: Optional stable failure classification.
- `assemblyStartedAt`, `assemblyCompletedAt`.
- `assemblyDurationMs`.
- `retentionExpiresAt`.
- `redactionStatus`.
- `documentJson`: Full metadata document.

### Relationships

- Belongs to one `Thread` and one current `Session Segment`.
- Has many `Continuity Preview Item` records.
- Links to one request turn and one dispatch where available.

### Validation Rules

- A chat response that evaluates continuity must have exactly one preview, even when no
  prior turns are eligible.
- Preview reads require `credentials.inspect`.
- Preview metadata must survive daemon restart.
- `assemblyDurationMs` participates in p95 validation for the default window.

## Continuity Preview Item

Represents one included or excluded turn or artifact excerpt decision within a preview.

### Fields

- `previewItemId`
- `continuityPreviewId`
- `tenantId`, `threadId`
- `itemKind`: `turn` or `artifact_excerpt`.
- `continuityTurnId`: Required for turn items.
- `artifactRef`: Optional runtime artifact reference.
- `artifactExcerptId`: Optional safe excerpt reference.
- `decision`: `included` or `excluded`.
- `reasonCode`: Stable reason such as `included_recent`, `over_limit`, `too_old`,
  `reset_boundary`, `lifecycle_blocked`, `source_mismatch`, `permission_denied`,
  `redaction_failed`, `retention_expired`, `duplicate_source_event`,
  `incomplete_evidence`, `unsupported_source`, or `artifact_reference_only`.
- `acceptanceSequence`: Copied from the turn when available.
- `sourceTimestamp`: Evidence only.
- `safeSummary`: Redacted metadata for operator display.
- `redactionStatus`.

### Validation Rules

- Included turn items must be ordered by daemon acceptance sequence.
- Excluded items must never leak content beyond safe summaries.
- Artifact excerpts can be included only when tied to an included turn, user-visible, and
  safely redacted.

## Runtime Artifact Excerpt

Represents safe, user-visible artifact content that can be included with an eligible
turn. This is a persisted value object, not an independently queryable table. Excerpt
metadata is stored inside the owning `thread_continuity_turns.document_json` entry and
is copied into `thread_continuity_preview_items` rows when an excerpt is included or
excluded as preview evidence.

### Fields

- `artifactExcerptId`
- `tenantId`, `threadId`, `sessionSegmentId`
- `continuityTurnId`
- `resourceKind`: `tool_result`, `file`, `approval`, `workflow`, `reply`,
  `delivery`, or `other`
- `resourceId`
- `excerptText`: Redacted excerpt.
- `excerptSource`: User-visible surface that produced the excerpt.
- `createdAt`
- `retentionExpiresAt`
- `redactionStatus`

### Validation Rules

- Must be directly tied to an included turn.
- Must be user-visible before it can be used as continuity input.
- Must be excluded when redaction is unreliable or the artifact is outside the
  inspection boundary.
- Must not be fetched by `artifactExcerptId` without tenant, thread, session segment,
  and preview/turn ownership context.

## Continuity Exclusion Reason

Represents stable classifications for operator evidence and tests.

### Values

- `no_eligible_turns`
- `over_limit`
- `too_old`
- `reset_boundary`
- `lifecycle_blocked`
- `source_mismatch`
- `permission_denied`
- `redaction_failed`
- `retention_expired`
- `duplicate_source_event`
- `incomplete_evidence`
- `unsupported_source`
- `artifact_reference_only`
- `continuity_disabled`

### Validation Rules

- Exclusion reasons are contract values and require schema/SDK/test updates when
  extended.
- Reasons must be visible in preview evidence without exposing unsafe content.

## State Transitions

```text
turn accepted
  -> persisted as Continuity Turn with daemon acceptance sequence
  -> eligible only if current segment, unexpired, redacted, supported source

chat or channel response requested with valid thread identity
  -> Continuity Preview created
  -> prior turns scanned by daemon acceptance sequence
  -> preview items record included/excluded decisions
  -> dispatch input assembled oldest-to-newest from included items
  -> assistant turn persisted after response

thread reset
  -> Roadmap 54 creates new session segment
  -> future windows exclude prior segment turns with reset_boundary reason

thread archived
  -> future windows are blocked until reopen or replacement thread

retention expiry
  -> expired turns/previews disappear from normal inclusion and inspection
```

## Storage And Indexing Notes

- `thread_continuity_turns` should index `(tenant_id, thread_id, session_segment_id,
  acceptance_sequence DESC)` for bounded-window lookup.
- `thread_continuity_turns` should carry a unique source-event key where source identity
  is available to suppress duplicate/replayed connector messages.
- `thread_continuity_previews` should index `(tenant_id, thread_id, assembly_completed_at
  DESC, continuity_preview_id DESC)` for thread detail.
- `thread_continuity_preview_items` should index `continuity_preview_id` and preserve
  item order for included turns.
- Runtime artifact excerpts are stored as redacted value objects in
  `thread_continuity_turns.document_json` and represented as `artifact_excerpt` rows in
  `thread_continuity_preview_items`; no separate artifact excerpt table is added in v51.
- All tables include `document_json` to match existing store conventions and allow
  additive schema evolution.
