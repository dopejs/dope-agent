# Evaluation Product

Roadmap 41 turns the replay harness into a tenant-scoped product workflow for
candidate discovery, product-managed fixtures, replay campaigns, dashboard
projections, and tool-call inspection.

## Operator Flow

1. Select an explicit tenant in the operator shell.
2. Review the active discovery policy before starting discovery.
3. Start a bounded discovery run over tenant-owned historical runs, workflows,
   tool calls, replay evidence, and Roadmap 40 live-validation evidence.
4. Inspect suggested candidates with scores, explanations, source provenance,
   scan bounds, redaction status, retention state, and suppression state.
5. Suppress runs, workflows, candidates, fixtures, or source families that must
   not appear in future discovery or campaign selection.
6. Materialize eligible candidates into product-managed fixtures only through
   permission-gated fixture routes.
7. Review fixture revisions and approve only content that preserves provenance
   and redacted evidence.
8. Start replay campaigns from eligible candidates, product-managed fixtures,
   repo-managed fixtures, or existing replay candidates.
9. Inspect campaign summaries, dashboard projections, and tool-call evidence
   without replacing underlying runtime truth.

## Safety Rules

- Product reads never trigger unbounded historical scans.
- Discovery evidence is redacted before persistence, display,
  materialization, campaign selection, export, or audit.
- Repo-managed fixtures remain immutable from product fixture editing paths.
- Campaign items keep immutable source snapshots.
- Retention/deletion removes selectable product payloads while preserving
  audit records, campaign snapshots, repo-managed fixtures, and runtime truth.
- Redaction failures fail closed and emit audit evidence instead of exposing
  raw payloads.

## Candidate Discovery

Discovery policies are tenant resources. Each policy records the enabled state,
source kinds, time window, inspected-record limit, emitted-candidate limit,
cost budget, sensitive-field rules, retention reference, creator, and update
timestamps. Operators should keep bounds narrow enough that page loads only read
stored product projections; historical scanning happens only through explicit
discovery-run starts or resumes.

Discovery runs persist the exact policy bounds, cursor, idempotency key, source
kinds, inspected count, emitted count, status, and partial reason. Reaching the
inspection, emitted-candidate, or cost boundary must produce a partial result
instead of silently continuing. Repeating the same tenant and idempotency key
returns the same run identity.

Suggested candidates include source provenance, score band, explanation fields,
readiness, redaction status, retention state, and suppression state. Scoring is
advisory and uses operational signals such as recurrence, drift, relevant tool
classes, live-validation outcomes, workflow coverage, recency, and operator
relevance. Fixture materialization and campaign selection still require explicit
authorized product actions.

Candidate evidence is built from redacted payloads only. Default redaction
removes secrets, credentials, raw tokens, bearer/session tokens, and configured
tenant sensitive fields from nested objects and arrays before persistence. If
redaction cannot complete safely, evidence materialization is blocked and the
failure is represented by audit/event records rather than raw payloads.

Suppressions can target a discovered candidate, a concrete product resource, or
a source family such as `run:run_1`. Active suppressions are tenant-scoped,
expire when their expiry time is reached, and are ignored after revocation.
Future discovery review and campaign selection must filter suppressed sources
without mutating repo-managed fixtures or runtime truth.

Retention/deletion applies only to Roadmap 41 product records such as discovered
candidates, candidate evidence, product fixtures, campaign projections, and
inspection records. Runtime runs, workflows, replay records, live-validation
ledger entries, repo fixtures, and immutable audit records remain separate
authoritative evidence.

## Product Fixtures

Product-managed fixtures are created from eligible discovered candidates and
their redacted candidate evidence. Creation records the source candidate,
source references, first immutable revision, creator, tenant, review state, and
retention/suppression state. Source evidence must be materialization-safe; a
redaction failure or suppressed/expired candidate blocks fixture creation.

Each edit creates a new immutable fixture revision. The fixture head points at
the current revision, while older revisions remain audit evidence and rollback
references. Rollback is performed by creating a new revision from a prior
approved payload rather than mutating an existing revision record.

Review changes fixture eligibility. Draft and rejected fixtures remain visible
for authorized users but are not selectable for ordinary campaigns. Approved
fixtures are selectable only while active, unsuppressed, and not deleted or
expired. Review, suppress, archive, delete, and redaction-failed outcomes emit
tenant-scoped audit/event records with fixture and revision references.

Repo-managed fixtures remain read-only from product editing paths. Product
routes must reject attempts to edit repo fixture ids directly; operators can
create a separate product-managed fixture copy only through explicit
materialization or revision workflows that preserve source provenance.

## Replay Campaigns

Replay campaigns are tenant-owned product records built from eligible
discovered candidates and approved product-managed fixtures. Campaign creation
rechecks tenant ownership, suppression state, retention state, and fixture
review state before writing any campaign item. Each campaign item stores an
immutable source snapshot so later fixture edits, suppression, expiry, or
deletion do not rewrite historical campaign evidence.

Campaign lifecycle transitions are explicit: draft or queued campaigns can
start, running campaigns can complete or fail, completed campaigns can publish
results, and draft/queued/running campaigns can be cancelled. Invalid
transitions are rejected instead of silently rewriting state. Campaign worker
hooks produce non-live replay launch plans and campaign attempt groups that
carry replay attempt ids, comparison ids, Roadmap 40 live-validation ledger
ids, drift counts, failure counts, unsupported counts, and
operator-action-needed counts.

Rollback disables new campaign creation and lifecycle mutation while leaving
existing campaigns, source snapshots, grouped result links, and audit events
readable for authorized tenants.

## Dashboards

Dashboard projections are stored tenant-scoped summaries, not live historical
scans. Projections aggregate campaign status, candidate retention/suppression,
fixture review/retention, drift, failures, unsupported replay, live-validation
linkage, and operator-action-needed totals from persisted Roadmap 41 product
records. Dashboard reads are permission-gated and use deterministic cursor
pagination over stored projections.

Projection data is evidence for release-readiness review, but it does not
replace runtime truth. Operators should inspect linked campaign groups, replay
attempts, comparisons, and live-validation ledger entries when a dashboard
signal affects a ship/no-ship decision.

## Tool-Call Inspection

Tool-call inspection records align original evidence, non-live replay evidence,
and Roadmap 40 live-validation ledger references for a campaign item. The
inspection classification explicitly distinguishes matched, drifted, failed,
unsupported, missing original evidence, missing replay evidence, denied live
validation, aborted live validation, failed live validation,
operator-action-needed validation, and completed live validation states.

Diff summaries are generated only from redacted evidence. Default and
tenant-configured sensitive fields are excluded before a diff summary is saved
or shown. The inspection record links back to runtime and validation evidence;
it is a debugging projection and must not be treated as the authoritative
runtime record.

## Rollback

Roadmap 41 rollback disables new discovery starts, product fixture edits,
campaign starts, and dashboard publication. Existing product records remain
readable to authorized users for diagnosis. Roadmap 33 replay and Roadmap 40
live-validation records remain available through their existing routes.
