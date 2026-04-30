# Contract: Tool-Call Replay Inspection

## Goal

Expose an operator-visible comparison of original tool-call behavior, non-live replay
behavior, live-validation evidence, unsupported replay state, and missing evidence
without leaking sensitive material or replacing runtime truth.

## Resource Shape

### ToolCallInspectionResource

Required fields:

- `inspectionId`
- `tenantId`
- `campaignId`
- `campaignItemId`
- `toolCallRef`
- `classification`
- `redactionStatus`
- `createdAt`
- `updatedAt`

Optional fields:

- `originalEvidenceRef`
- `nonLiveReplayEvidenceRef`
- `liveValidationLedgerRefs`
- `diffSummary`
- `retentionState`

Allowed `classification` values:

- `matched`
- `drifted`
- `failed`
- `unsupported`
- `missing_original_evidence`
- `missing_replay_evidence`
- `live_validation_denied`
- `live_validation_aborted`
- `live_validation_failed`
- `live_validation_operator_action_needed`
- `live_validation_completed`

## API Routes

### `GET /v1/evaluation/campaigns/{campaignId}/tool-call-inspections`

Purpose: List tool-call inspections for a campaign.

Query:

- `cursor`
- `limit`

Ordering: `updatedAt DESC, inspectionId DESC`.

### `GET /v1/evaluation/tool-call-inspections/{inspectionId}`

Purpose: Inspect one redacted tool-call replay comparison.

Response includes:

- source tool-call reference
- original evidence summary when available
- non-live replay evidence summary when available
- live-validation ledger links when available
- unsupported or missing-evidence reason when applicable
- redacted diff summary
- retention state

## Evidence Rules

- Original evidence links to runtime tool-call records or captured replay evidence.
- Non-live replay evidence links to evaluation replay attempts and comparisons.
- Live-validation evidence links to Roadmap 40 validation attempts and ledger entries.
- Inspection records never overwrite runtime records, replay attempts, comparisons, or
  live-validation ledger entries.
- Retention may remove product-side diff payloads but must keep enough metadata to
  explain that evidence expired.

## Redaction Rules

Inspection payloads must apply the same redaction policy as candidate discovery:

- no secrets
- no credentials
- no raw tokens
- no configured sensitive fields
- no unredacted provider payload fragments marked sensitive

If redaction cannot be completed, the inspection is classified as
`missing_replay_evidence` or `unsupported` with a redaction failure reason instead of
displaying raw evidence.

## Audit Events

Required audit/events:

- `evaluation.tool_call_inspection.generated`
- `evaluation.tool_call_inspection.redaction_failed`
- `evaluation.tool_call_inspection.retention_applied`

Events include tenant id, campaign id, campaign item id, tool call ref, classification,
outcome, and evidence references.

Resource schema: `schemas/api/evaluation-tool-call-inspection-resource.schema.json`.
Event schema:
`schemas/events/evaluation-tool-call-inspection-generated.event.schema.json`.

## Required Tests

- Inspection compares original and non-live replay evidence when both exist.
- Inspection links live-validation ledger entries when available.
- Unsupported tool classes are explicit and feed campaign/dashboard aggregates.
- Missing, expired, denied, aborted, failed, operator-action-needed, and completed
  live-validation states are distinct.
- Redaction happens before persistence and display.
- Cross-tenant inspection access is denied and audited.
