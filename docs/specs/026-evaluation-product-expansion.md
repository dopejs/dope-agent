# Evaluation Product Expansion

Status: complete

Release-evidence closure is tracked by
`docs/harness/roadmap41-soak-acceptance-runbook.md` and T153 in
`specs/026-evaluation-product-expansion/tasks.md`. The combined Roadmap 39/40/41
24-hour rerun report exists and passed the runbook criteria on 2026-05-01
Asia/Shanghai for commit `5ad95ba`.

Authority: This document is the authoritative upstream spec for Roadmap 41, the evaluation
product expansion that completes the remaining Roadmap 33 out-of-scope product gaps.

Primary source documents:
- `docs/product/hosted-productization-roadmap-split.md`
- `docs/specs/018-evaluation-and-replay-harness.md`
- `docs/specs/024-production-install-upgrade-backup-and-soak.md`
- `docs/specs/025-live-validation-and-side-effect-replay.md`

## Background

Roadmap 33 created the replay harness substrate and Roadmap 40 adds live validation. The
product still needs automatic candidate discovery, in-product fixture editing, campaign
management, and richer tool-call replay inspection so evaluation becomes an everyday
operator workflow rather than an engineer-managed fixture list.

## Goal

Expand evaluation into a tenant-aware product surface for automatic historical run
candidate discovery, fixture editing, replay campaigns, dashboards, and tool-call replay
inspection.

## Fixed Decisions

- Automatic candidate discovery must be explainable and tenant-scoped.
- Product fixture editing is permission-gated and preserves provenance.
- Campaigns group replay attempts, comparisons, and live validation outcomes without
  replacing underlying runtime truth.
- Tool-call replay inspection consumes the Roadmap 40 side-effect ledger for live paths.
- Candidate discovery must have explicit scan bounds, retention policy, and sensitive-data
  exclusion rules.
- Operators can manually exclude runs, workflows, or fixtures from discovery and campaigns.

## Dependencies On Completed Phases

- Roadmap 33: Evaluation And Replay Harness
- Roadmap 35: Tenant-Scoped Data Migration
- Roadmap 36: Tenant-Aware Operator Shell And SDK
- Roadmap 40: Live Validation And Side-Effect Replay

## In Scope

- automatic historical run and workflow candidate discovery
- candidate scoring, explanation, and operator override
- configurable discovery windows by time, count, and tenant
- sensitive-data filtering for candidate evidence and fixture materialization
- retention and deletion behavior for discovered candidates and product-edited fixtures
- product UI for fixture creation, editing, review, and provenance inspection
- replay campaign records and dashboard projections
- tool-call replay inspection and diff views
- tenant-aware evaluation permissions and audit events

## Required Discovery Design Artifact

Implementation planning MUST define the candidate discovery contract before coding. The
artifact MUST include:

- candidate source tables and APIs
- tenant context and permission required for discovery
- scan bounds by time window, maximum inspected records, maximum candidates emitted, and
  per-tenant cost budget
- incremental cursor or background job strategy
- scoring inputs and explanation fields
- sensitive-field exclusion and redaction rules before evidence is presented or
  materialized into fixtures
- retention period for discovered candidates and generated evidence
- deletion and manual suppression behavior
- behavior for repo-managed fixtures versus product-edited fixtures
- audit events for discovery, suppression, fixture creation, fixture edit, campaign start,
  and campaign result publication

## Required Campaign And Dashboard Contract

The implementation plan MUST define campaign and dashboard resource shapes with:

- campaign identity, tenant ownership, status, and lifecycle transitions
- selected fixtures or candidates and immutable source references
- replay attempt grouping and comparison summary fields
- live validation linkage to Roadmap 40 side-effect ledger entries
- drift, failure, unsupported replay, and operator-action-needed aggregate fields
- pagination and retention behavior for dashboard queries
- SDK and web projection contracts

## Out Of Scope

- model training infrastructure
- autonomous self-improvement
- memory promotion
- unreviewed fixture mutation by the agent

## Operator Or User Problems To Solve

- Operators should not manually curate every useful replay candidate.
- Engineers and operators need to edit fixtures from the product without losing provenance.
- Product teams need campaign-level confidence when changing integrations, policies, or
  tool orchestration.

## User Stories

- As an operator, I can see automatically suggested replay candidates from historical runs.
- As an engineer, I can edit a fixture in-product and keep an audit trail.
- As a tenant admin, I can run a replay campaign and inspect drift trends.

## Functional Requirements

- The system MUST discover historical run and workflow candidates by tenant.
- Candidate suggestions MUST include explanation and source provenance.
- Candidate discovery MUST be bounded by configured time window and maximum inspected
  record count per tenant.
- Candidate discovery MUST exclude or redact secrets, credentials, raw tokens, and
  configured sensitive fields before presenting candidate evidence or creating fixtures.
- The system MUST support manual exclusion of specific runs, workflows, candidates, and
  fixtures from future discovery and campaigns.
- The system MUST define retention and deletion behavior for discovered candidates,
  campaign results, and product-edited fixtures.
- Fixture editing MUST be permission-gated and auditable.
- Campaigns MUST group replay attempts, comparisons, and live-validation outcomes.
- Dashboards MUST expose drift, failure, and unsupported replay summaries.
- Tool-call replay inspection MUST show original, non-live replay, and live validation
  evidence where available.

## Compatibility And Operational Notes

- Repo-managed fixtures remain supported.
- Product-edited fixtures must not overwrite repo-managed fixtures silently.
- Candidate discovery must be bounded so it does not scan unbounded history on every page
  load.
- Candidate discovery should run as an incremental or background process with explicit
  per-tenant cost limits rather than synchronous full-history scans.
- Deletion or exclusion requests must not mutate immutable repo-managed fixtures; they
  should create product-side suppression records instead.

## Verification Expectations

- Candidate discovery tests over representative historical run data.
- Candidate discovery tests for scan bounds, per-tenant cost limits, sensitive-field
  redaction, manual exclusion, and retention/deletion behavior.
- Fixture editing API and UI tests.
- Campaign aggregation tests.
- Dashboard and SDK tests for tenant-scoped evaluation projections.
- Regression proving repo-managed fixtures remain immutable through product editing paths.
- Discovery bounds test proving page loads or dashboard refreshes cannot trigger
  unbounded full-history scans.
- Campaign contract tests proving live validation outcomes link to side-effect ledger
  records without replacing underlying runtime truth.
- Phase 8 evidence is recorded in
  `specs/026-evaluation-product-expansion/quickstart.md`, including targeted Go tests,
  full daemon verification, contract tests, client tests/build, local daemon smoke,
  Roadmap 41 product smoke, and the accepted Roadmap 39/40/41 24-hour soak rerun.

## Definition Of Done

- Evaluation is usable as a tenant-aware product workflow with candidate discovery, fixture
  editing, campaigns, and tool-call replay inspection.
- Automatic discovery is bounded, privacy-aware, and operator-controllable.
- Final hosted-productization release readiness has rerun the Roadmap 39 soak harness with
  Roadmap 40 live validation and Roadmap 41 evaluation-product workflows included in the
  workload, fault drills, cross-tenant leakage checks, and resource-growth report.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/026-evaluation-product-expansion.md 完成 phase 41 的工作`
