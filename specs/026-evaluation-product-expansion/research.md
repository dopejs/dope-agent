# Research: Evaluation Product Expansion

## Decision: Keep Roadmap 41 Additive To The Existing Evaluation Boundary

**Decision**: Extend `daemon/internal/evaluation` for discovery, product fixtures,
campaigns, dashboards, and inspection instead of creating a parallel evaluation product
package.

**Rationale**: Existing replay candidates, attempts, comparisons, and repo fixtures are
already owned by `evaluation`. Keeping product state in the same domain boundary reduces
compatibility risk and makes it easier to prove repo-managed and product-managed fixtures
share replay semantics without hidden side paths.

**Alternatives considered**:

- New `daemon/internal/evaluationproduct` package. Rejected because it would duplicate
  domain ownership and increase contract drift.
- Embed all product behavior directly in API handlers. Rejected because discovery,
  retention, scoring, redaction, and campaign aggregation need testable domain logic.

## Decision: Discovery Runs Are Explicit, Bounded, And Cursor-Based

**Decision**: Discovery is started or resumed through explicit tenant-scoped discovery
runs with durable cursors, idempotency keys, scan bounds, emitted-candidate limits, and
partial-result status.

**Rationale**: The upstream spec forbids unbounded discovery on page load. Cursor-based
runs make repeated work idempotent, restart-safe, auditable, and bounded by tenant cost
budgets.

**Alternatives considered**:

- Synchronous discovery during dashboard load. Rejected because it can scan unbounded
  history and make user reads operationally expensive.
- One global background sweep. Rejected because it weakens tenant isolation and makes
  per-tenant budget enforcement harder.

## Decision: Use Separate Discovered Candidate Resources Before Materialization

**Decision**: Store automatically suggested candidates as `DiscoveredCandidate` resources
separate from existing `ReplayCandidateResource`. A discovered candidate can be selected
for a campaign or materialized into product-managed fixture/replay state only through
explicit product actions.

**Rationale**: Existing replay candidate schemas have stable `candidateKind` values for
repo fixtures and curated work. A separate product resource avoids surprising existing
clients and allows discovery evidence, score, redaction status, suppression state, and
retention to evolve independently.

**Alternatives considered**:

- Add a new `candidateKind` enum value to existing replay candidates immediately. Rejected
  for initial rollout because TypeScript clients may treat current union values as
  exhaustive.
- Store discoveries only as logs. Rejected because operators need review, suppression,
  retention, and campaign selection state.

## Decision: Redact Before Persisting Candidate Evidence

**Decision**: Candidate evidence is redacted before it is persisted, displayed, used to
create product fixtures, selected for campaigns, exported, or audited.

**Rationale**: The feature mines historical runs, workflows, and tool calls, which may
contain secrets, credentials, raw tokens, and tenant-configured sensitive fields.
Persisting raw evidence and redacting only at display time would create unnecessary data
exposure and deletion complexity.

**Alternatives considered**:

- Store raw evidence and redact on read. Rejected because it increases secret exposure
  and audit risk.
- Drop all evidence and keep only scores. Rejected because discovery must be explainable
  and reviewable.

## Decision: Product Fixtures Use Immutable Revisions

**Decision**: Product-managed fixtures have a mutable head plus immutable revision
history. Each creation, edit, review transition, and suppression records actor, time,
source provenance, redaction status, and audit evidence.

**Rationale**: Product editing is useful only if users can inspect what changed and why.
Immutable revisions preserve provenance, support rollback of fixture content, and prevent
silent mutation of repo-managed fixtures.

**Alternatives considered**:

- Update fixture content in place. Rejected because it loses provenance and makes audit
  reconstruction hard.
- Convert repo-managed fixtures into product fixtures automatically. Rejected because it
  violates the fixed decision that repo-managed fixtures must not be silently overwritten.

## Decision: Campaign Sources Are Immutable Snapshots

**Decision**: Campaign items store immutable source references and source snapshots for
the selected discovered candidate, product fixture, repo fixture, or replay candidate at
campaign start.

**Rationale**: Campaign results must remain explainable even when the underlying product
fixture is edited, a discovery candidate expires, or a suppression record is added later.
Immutable snapshots also make dashboard aggregation reproducible.

**Alternatives considered**:

- Resolve campaign sources live on every view. Rejected because historical results would
  drift as source objects change.
- Copy full runtime truth into campaigns. Rejected because campaigns must link to
  underlying runtime truth rather than replacing it.

## Decision: Dashboard Projections Read From Product And Replay Evidence

**Decision**: Dashboard projections aggregate campaign records, replay attempts,
comparison results, unsupported classifications, product fixture state, discovered
candidate state, and Roadmap 40 live-validation ledger links.

**Rationale**: Roadmap 41 dashboards must answer product questions across discovery,
fixture readiness, campaign confidence, drift, failures, unsupported replay, and
operator-action-needed states. Aggregating from durable product state avoids recomputing
from raw history during page loads.

**Alternatives considered**:

- Dashboard reads directly from raw runs and tool calls. Rejected because it reintroduces
  unbounded scans and duplicates discovery logic.
- Dashboard shows only campaign records. Rejected because operators also need discovery
  and fixture workflow status.

## Decision: Retention Deletes Selectable Product Evidence But Preserves Audit Trail

**Decision**: Retention and deletion remove or expire selectable product-side evidence,
discovered candidates, product fixtures, campaign result projections, and inspection
payloads according to policy, while preserving minimal audit records and immutable
runtime/repo-managed fixture references.

**Rationale**: Operators need deletion and suppression behavior without corrupting audit
history or immutable repo/runtime truth. Product-side tombstones and suppression records
make future discovery and campaign selection deterministic.

**Alternatives considered**:

- Hard-delete every related row. Rejected because campaigns, audit trails, and runtime
  evidence would become inconsistent.
- Never delete product evidence. Rejected because the upstream spec requires retention
  and deletion behavior.

## Decision: Stable Contracts Require Idempotency, Pagination, And Denial Shapes

**Decision**: All mutating product routes accept client request identifiers where retry
is plausible, all list routes define deterministic ordering and cursor pagination, and
permission/quota/redaction/suppression denials use stable error or resource states.

**Rationale**: Discovery jobs and campaign starts are asynchronous and retry-prone. SDK
and web clients need stable semantics for retries, pagination, and denied operations.

**Alternatives considered**:

- Limit/offset pagination only. Rejected because concurrent writes can duplicate or skip
  records under active discovery and campaign updates.
- Free-form error strings. Rejected because clients and tests need stable handling.
