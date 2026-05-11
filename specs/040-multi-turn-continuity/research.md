# Research: Non-Knowledge Multi-Turn Continuity

## Decision: Extend Thread Lifecycle Instead Of Creating Memory

**Decision**: Implement recent-turn continuity as an additive extension of Roadmap 54
thread/session lifecycle. Store continuity turns and previews under daemon-owned thread
truth, and assemble dispatch input only from eligible turns in the current active thread
segment.

**Rationale**: The upstream spec requires bounded recent-thread continuity while
explicitly excluding memory, semantic retrieval, summaries, long-term personalization,
and hidden context engines. Roadmap 54 already owns thread identity, reset boundaries,
source linkage, permissions, and operator inspection, so the least risky path is to add
turn persistence and inclusion evidence there.

**Alternatives considered**:
- Add a memory or knowledge adapter: rejected because this phase is explicitly
  non-knowledge and must not write or recall memory.
- Keep continuity in Web/TUI client state: rejected because channel paths and restart
  recovery need daemon-owned evidence.
- Use provider-side retained context: rejected because it is hidden, non-inspectable,
  and cannot honor reset or evidence requirements.

## Decision: Store Explicit Turns Plus Preview Decisions

**Decision**: Persist explicit continuity turns separately from per-response preview
decisions. A turn records accepted user/assistant content metadata and safe text; a
preview records which prior turns and excerpts were included or excluded for one
response.

**Rationale**: Turns are durable inputs; previews are response-specific evidence. Keeping
them separate avoids duplicating turn content in every response while preserving a stable
operator explanation for behavior questions, redaction failures, over-limit exclusions,
and reset boundaries.

**Alternatives considered**:
- Store only final prompt messages: rejected because operators need inclusion and
  exclusion reasons, not an opaque prompt snapshot.
- Store only turns and recompute previews on inspection: rejected because later reset,
  retention, or redaction changes could make historical response evidence unstable.
- Store only preview JSON: rejected because follow-up assembly needs indexed recent-turn
  queries and duplicate suppression.

## Decision: Use Daemon Acceptance Sequence For Ordering

**Decision**: Assign a daemon acceptance sequence per tenant/thread and use it as the
canonical continuity order. Source timestamps remain evidence only.

**Rationale**: Connector clocks, provider event timestamps, replays, and concurrent
messages can arrive out of order. A daemon acceptance sequence gives deterministic
assembly, stable over-limit behavior, and restart-safe tests while preserving source
timestamp metadata for debugging.

**Alternatives considered**:
- Order by source timestamp: rejected because provider clocks and replay timing can
  reorder continuity unexpectedly.
- Order by daemon receive timestamp: rejected because concurrent writes still need a
  strict sequence.
- Order by response completion time: rejected because it would place slow assistant
  responses after later accepted user turns and blur causality.

## Decision: Add Optional Thread Fields To Chat Contracts

**Decision**: Add optional `threadId` to chat query input and additive thread/continuity
fields to non-stream and stream responses. Existing requests without `threadId` remain
single-turn and compatible.

**Rationale**: Web and TUI need a way to continue an explicit daemon thread, while
existing clients must keep working. Additive optional fields preserve wire
compatibility and let clients adopt continuity when they are thread-aware.

**Alternatives considered**:
- Create a separate `/v1/threads/{threadId}/chat` route: rejected for this phase because
  it would duplicate chat streaming/non-stream logic and SDK behavior.
- Make `threadId` required for chat: rejected because the minimal chat clients and
  existing automation rely on single-turn behavior.
- Infer thread from token or tenant: rejected because it would create hidden
  cross-thread context and violate source identity requirements.

## Decision: Include Only Safe User-Visible Artifact Excerpts

**Decision**: Allow runtime artifact content to contribute to continuity only as
user-visible, safely redacted excerpts tied to included turns. Other artifacts remain
reference-only evidence.

**Rationale**: Tool and runtime artifacts can be necessary for natural follow-ups, but
full artifact content can contain secrets, raw provider payloads, or data outside the
conversation boundary. Excerpt-only inclusion keeps behavior useful and auditable without
becoming broad context packing.

**Alternatives considered**:
- Include all directly linked artifact content: rejected because it increases privacy,
  latency, and hidden-context risk.
- Exclude all artifacts from continuity and evidence: rejected because operators need
  to debug artifact-linked responses.
- Keep artifact references only: rejected because some follow-ups would fail despite
  user-visible artifact output in the prior turn.

## Decision: Add A Dedicated Preview Inspection Route And Thread Detail Projection

**Decision**: Expose recent continuity preview summaries through thread detail and full
preview detail through `GET /v1/threads/{threadId}/continuity-previews/{previewId}`.

**Rationale**: Thread detail needs enough information for operators to find relevant
responses quickly, but full included/excluded item lists can grow and should be
inspectable on demand. A dedicated route keeps the list/detail contract bounded while
preserving precise evidence.

**Alternatives considered**:
- Put all preview item details into thread detail: rejected because thread detail would
  become too large and harder to paginate.
- Expose previews only in logs: rejected because the spec requires product evidence.
- Expose preview only on chat response: rejected because support inspection often occurs
  after the response.

## Decision: Stage Migration With Shadow Evidence Before Inclusion

**Decision**: Plan rollout as additive schema, shadow turn/preview evidence, then
enabled continuity assembly after tests and latency validation pass.

**Rationale**: Mixed-version and rollback safety matter because chat and channel paths
are user-facing. Shadow evidence lets the team validate ordering, redaction, reset
boundaries, and p95 assembly behavior before behavior changes.

**Alternatives considered**:
- Enable assembly immediately after schema migration: rejected because preview evidence
  and latency need validation first.
- Backfill old sessions into active continuity: rejected because legacy ordering and
  source linkage may be incomplete.
- Require a destructive migration: rejected because rollback would be expensive and
  unnecessary for a bounded recent-turn feature.

## Decision: Keep Retention And Permissions Aligned With Roadmap 54

**Decision**: Use `credentials.inspect` for continuity preview inspection,
`connectors.manage` for reset, Roadmap 54's 90-day default inspection retention, and the
phase 55 30-day active-continuity inclusion window.

**Rationale**: This keeps authorization and retention consistent with existing thread
lifecycle evidence while separating active inclusion from longer inspection retention.
Operators can inspect recent evidence longer than it is eligible for active continuity,
unless tenant policy changes the active window.

**Alternatives considered**:
- Add a new continuity permission: rejected as unnecessary identity surface expansion
  for this roadmap.
- Use 90 days for active inclusion: rejected because that would make continuity too
  close to long-term memory.
- Retain previews indefinitely: rejected because it increases privacy and operational
  burden.
