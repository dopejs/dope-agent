# Feature Specification: Evaluation Product Expansion

**Feature Branch**: `026-evaluation-product-expansion`  
**Created**: 2026-04-29  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/026-evaluation-product-expansion.md 完成 phase 41 的工作"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Discover Replay Candidates (Priority: P1)

As an operator, I can review tenant-scoped replay candidates automatically suggested from historical runs and workflows, including why each candidate was suggested and where it came from.

**Why this priority**: Automatic, explainable discovery removes the manual fixture-curation bottleneck and is the foundation for fixture editing, campaigns, and evaluation dashboards.

**Independent Test**: Can be fully tested by seeding representative historical runs for multiple tenants, triggering discovery within configured bounds, and verifying that only eligible candidates for the selected tenant appear with explanations and provenance.

**Acceptance Scenarios**:

1. **Given** a tenant has eligible historical runs within the configured discovery window, **When** an operator reviews suggested candidates, **Then** the operator sees tenant-scoped candidates with explanation, score, source provenance, and discovery-window context.
2. **Given** historical runs contain secrets, credentials, raw tokens, or configured sensitive fields, **When** discovery presents evidence or materializes a candidate into a fixture, **Then** sensitive material is excluded or redacted before it is visible or saved.
3. **Given** an operator excludes a run, workflow, candidate, or fixture, **When** future discovery or campaign selection occurs, **Then** the excluded item is not suggested or selected unless the exclusion is explicitly removed by an authorized operator.

---

### User Story 2 - Edit Product Fixtures With Provenance (Priority: P1)

As an engineer or authorized operator, I can create, edit, review, and inspect product-managed fixtures without losing source provenance or silently changing repo-managed fixtures.

**Why this priority**: Product fixture editing turns discovery into a usable evaluation workflow while preserving trust in existing fixture sources and audit trails.

**Independent Test**: Can be fully tested by converting an eligible candidate into a product-managed fixture, editing it with an authorized role, attempting the same action with an unauthorized role, and verifying provenance and audit history.

**Acceptance Scenarios**:

1. **Given** an authorized user opens a discovered candidate, **When** the user creates a fixture, **Then** the fixture records its candidate source, source run or workflow reference, creation actor, creation time, and review state.
2. **Given** an authorized user edits a product-managed fixture, **When** the edit is saved, **Then** the fixture preserves prior provenance, records the edited fields, and emits an audit trail entry.
3. **Given** a repo-managed fixture exists, **When** a user attempts product-side editing, **Then** the system does not silently overwrite the repo-managed fixture and instead creates or updates only product-side fixture state.
4. **Given** a user lacks fixture-edit permission, **When** the user attempts to create, edit, or approve a product-managed fixture, **Then** the action is denied and the denial is visible in the audit trail.

---

### User Story 3 - Run Replay Campaigns (Priority: P2)

As a tenant admin, I can group selected candidates and fixtures into a replay campaign, track campaign lifecycle, and inspect aggregate confidence signals across attempts, comparisons, and live-validation outcomes.

**Why this priority**: Campaigns provide product-level confidence for integration, policy, and orchestration changes instead of requiring users to reason about individual replay attempts.

**Independent Test**: Can be fully tested by creating a campaign from approved fixtures and candidates, running it through completion, and verifying grouped attempts, comparison summaries, live-validation linkage, and lifecycle status.

**Acceptance Scenarios**:

1. **Given** a tenant admin selects eligible fixtures or candidates, **When** the admin starts a campaign, **Then** the campaign records tenant ownership, selected immutable sources, lifecycle status, start actor, and selected evaluation scope.
2. **Given** a campaign completes with mixed replay outcomes, **When** the admin reviews campaign results, **Then** the campaign shows grouped attempts, comparison summaries, live-validation outcomes where available, and items requiring operator action.
3. **Given** a selected fixture or candidate is later edited, deleted, expired, or suppressed, **When** an existing campaign is reviewed, **Then** the campaign continues to show immutable source references and clearly distinguishes historical campaign evidence from current selectable content.

---

### User Story 4 - Monitor Evaluation Dashboards (Priority: P3)

As an operator or product team member, I can use evaluation dashboards to understand drift, failures, unsupported replay, live-validation signals, and campaign trends for the tenants I am allowed to access.

**Why this priority**: Dashboards make the evaluation product usable for repeated operational decisions and release-readiness checks after campaigns exist.

**Independent Test**: Can be fully tested by loading dashboard projections for tenants with known campaign and replay outcomes and verifying scoped aggregate values, pagination behavior, and trend summaries.

**Acceptance Scenarios**:

1. **Given** multiple campaigns have completed for a tenant, **When** an authorized user opens the evaluation dashboard, **Then** the dashboard shows drift, failure, unsupported replay, and operator-action-needed summaries for that tenant only.
2. **Given** dashboard result sets exceed a single page, **When** the user pages through campaign or replay evidence, **Then** the user can navigate deterministically without duplicate, missing, or cross-tenant results.
3. **Given** a release-readiness review includes Roadmap 40 live validation and Roadmap 41 evaluation workflows, **When** the dashboard is used during the review, **Then** it exposes enough campaign and validation evidence to support the final hosted-productization readiness decision.

---

### User Story 5 - Inspect Tool-Call Replay Evidence (Priority: P3)

As an engineer debugging evaluation results, I can compare original behavior, non-live replay behavior, and live-validation evidence for tool calls where that evidence is available.

**Why this priority**: Tool-call inspection closes the debugging loop by showing why replay, drift, unsupported behavior, or side-effect validation changed confidence in a campaign result.

**Independent Test**: Can be fully tested by running replays with original, non-live, unsupported, and live-validation evidence and verifying that the inspection view presents aligned evidence without replacing runtime truth.

**Acceptance Scenarios**:

1. **Given** a replayed tool call has original and non-live replay evidence, **When** an engineer opens the inspection view, **Then** the engineer can compare inputs, observed outputs, classifications, and differences with sensitive data redacted.
2. **Given** live-validation evidence exists for a tool call, **When** the engineer inspects the replay result, **Then** the view links the validation outcome to the side-effect record and clearly distinguishes validation evidence from the underlying runtime record.
3. **Given** a tool call is unsupported for replay or live validation, **When** the engineer inspects campaign results, **Then** the unsupported classification and reason are explicit and contribute to campaign and dashboard summaries.

### Edge Cases

- Discovery finds no eligible candidates within the configured bounds; the operator sees an empty state with the applied tenant, time window, scan limit, and exclusion context.
- A tenant has more historical data than the configured inspection or cost budget permits; discovery stops at the configured bounds and reports that the result set is partial.
- Candidate evidence includes nested sensitive material or tenant-specific excluded fields; evidence is redacted before display, fixture creation, campaign selection, or export.
- A source run, workflow, or product-managed fixture is deleted, expired, or manually suppressed after a campaign starts; existing campaign evidence remains auditable while future discovery and selection honor deletion or suppression rules.
- Repo-managed and product-managed fixtures refer to the same underlying behavior; product edits do not mutate repo-managed fixtures and campaign selection makes the selected source explicit.
- A user has permission to view evaluation dashboards but not edit fixtures or start campaigns; restricted actions are unavailable or denied while read-only evidence remains scoped to permitted tenants.
- Live-validation evidence is missing, expired, denied, aborted, or unsupported; campaign and inspection views classify the absence or limitation explicitly instead of treating it as success.
- Dashboard refreshes or page loads occur repeatedly for large tenants; they do not trigger unbounded historical discovery work.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST discover replay candidates from historical runs and workflows within a single tenant scope.
- **FR-002**: Candidate discovery MUST be bounded by tenant, time window, maximum inspected record count, maximum emitted candidate count, and per-tenant cost budget.
- **FR-003**: Candidate discovery MUST record and show candidate score, explanation, source provenance, discovery bounds, and discovery time for each suggested candidate.
- **FR-004**: Candidate discovery MUST exclude or redact secrets, credentials, raw tokens, and configured sensitive fields before evidence is shown, saved, selected, or materialized into fixtures.
- **FR-005**: Authorized operators MUST be able to manually suppress specific runs, workflows, candidates, and fixtures from future discovery and campaign selection.
- **FR-006**: The system MUST define and enforce retention, deletion, and suppression behavior for discovered candidates, candidate evidence, campaign results, and product-managed fixtures.
- **FR-007**: Product-managed fixture creation and editing MUST be permission-gated by tenant-aware evaluation permissions.
- **FR-008**: Product-managed fixture creation and editing MUST preserve source provenance, edit history, review state, actor identity, and timestamps.
- **FR-009**: Product-side fixture editing MUST NOT silently overwrite repo-managed fixtures.
- **FR-010**: The system MUST emit audit events for discovery, suppression, fixture creation, fixture edit, campaign start, campaign status change, campaign result publication, dashboard projection generation, tool-call inspection generation, redaction failure, and retention/deletion application.
- **FR-011**: Tenant admins MUST be able to create replay campaigns from eligible fixtures and candidates with immutable source references.
- **FR-012**: Campaigns MUST track tenant ownership, lifecycle status, selected evaluation scope, start actor, start time, completion time, and retention state.
- **FR-013**: Campaign results MUST group replay attempts, comparison summaries, live-validation outcomes where available, unsupported replay classifications, drift summaries, failures, and operator-action-needed items.
- **FR-014**: Campaign and dashboard views MUST preserve links to the underlying runtime truth when aggregating or projecting replay, comparison, and live-validation evidence.
- **FR-015**: Evaluation dashboards MUST provide tenant-scoped summaries for drift, failures, unsupported replay, campaign status, live-validation linkage, and operator-action-needed counts.
- **FR-016**: Dashboard result sets MUST support deterministic paging and retention-aware filtering for campaign and replay evidence.
- **FR-017**: Tool-call replay inspection MUST compare original behavior, non-live replay behavior, and live-validation evidence when available.
- **FR-018**: Tool-call replay inspection MUST explicitly classify missing, expired, denied, aborted, failed, unsupported, and completed validation evidence.
- **FR-019**: Discovery, fixture editing, campaign management, dashboard access, and tool-call inspection MUST enforce tenant isolation for reads and writes.
- **FR-020**: The product workflow MUST support repo-managed fixtures and product-managed fixtures side by side while making source type and mutability clear to users.
- **FR-021**: Planning for this feature MUST produce an approved discovery policy that covers source data, permission model, scan bounds, incremental discovery behavior, scoring explanations, redaction rules, retention, deletion, suppression, repo-managed fixture behavior, and audit events.
- **FR-022**: Planning for this feature MUST produce an approved campaign and dashboard contract that covers campaign identity, ownership, lifecycle, selected sources, grouped attempts, comparison summaries, live-validation linkage, aggregate fields, pagination, retention, and user-facing projections.

### Key Entities

- **Discovery Policy**: Tenant-scoped discovery configuration; includes enabled state, source kinds, scan window, inspected-record and emitted-candidate bounds, cost budget, sensitive-field rules, retention policy reference, actor identity, and audit timestamps.
- **Discovered Candidate**: A suggested replay opportunity derived from historical tenant-scoped run or workflow evidence; includes source reference, explanation, score, redacted evidence summary, discovery bounds, retention state, and suppression state.
- **Candidate Evidence**: Redacted information used to justify or materialize a discovered candidate; includes provenance, sensitive-data handling status, and retention status.
- **Suppression Record**: A tenant-scoped operator decision that excludes a run, workflow, candidate, or fixture from future discovery or campaign selection.
- **Product-Managed Fixture**: A fixture created or edited through the product; includes source provenance, review state, current editable content, edit history, audit trail, retention state, and relationship to any source candidate.
- **Repo-Managed Fixture**: A fixture managed outside the product editing workflow; remains selectable where supported but cannot be silently mutated by product-side editing.
- **Replay Campaign**: A tenant-owned grouping of selected fixtures or candidates, replay attempts, comparisons, live-validation outcomes, lifecycle status, aggregate summaries, and retention state.
- **Campaign Attempt Group**: A campaign-scoped grouping of replay runs, comparison evidence, live-validation outcomes, and operator-action-needed signals for a selected source.
- **Tool-Call Replay Inspection**: A user-facing comparison record for original tool-call behavior, non-live replay behavior, live-validation evidence, unsupported classifications, and redacted differences.
- **Evaluation Audit Event**: A tenant-scoped record of user or system actions affecting discovery, suppression, fixtures, campaigns, dashboards, tool-call inspections, result publication, retention/deletion application, or failed-closed redaction behavior.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Evaluation product surfaces change across fixture management, campaign management, dashboard projections, audit events, permissions, retention behavior, and user-facing inspection views. Existing repo-managed fixtures remain supported and must retain their current behavior unless explicitly selected as immutable sources.
- **Migration / Rollback**: Rollout requires introducing product-managed evaluation state while preserving existing replay harness behavior. Rollback must be able to disable candidate discovery, fixture editing, campaign starts, and dashboard publication while leaving historical campaign and audit evidence readable for authorized operators.
- **Verification Strategy**: Required validation includes tenant-scoped discovery tests, scan-bound and cost-budget tests, sensitive-data redaction tests, suppression and retention tests, fixture permission and provenance tests, campaign aggregation tests, dashboard projection tests, tool-call inspection tests, cross-tenant leakage tests, and release-readiness rerun coverage that includes live validation and evaluation product workflows.
- **Observability Impact**: The feature must add operator-visible audit events, discovery-bound reporting, suppression history, campaign lifecycle evidence, result publication evidence, unsupported classification visibility, and dashboard signals for drift, failure, live-validation linkage, and operator-action-needed items.
- **Environment & Secrets**: Development and verification must default to the test environment. Live connectors or live-validation evidence may be referenced only through explicit tenant-scoped validation records, and secrets or sensitive fields must never appear in discovery evidence, fixtures, campaign results, dashboards, audits, or inspection views.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For a tenant with eligible historical evidence, an authorized operator can identify at least 10 ranked replay candidates, or all eligible candidates if fewer than 10 exist, within 2 minutes of starting the discovery review.
- **SC-002**: 100% of presented candidate evidence and product-managed fixture materialization paths exclude configured sensitive fields, raw tokens, credentials, and secrets in representative privacy test data.
- **SC-003**: 100% of discovery jobs tested against large tenant histories stop within configured time, count, emitted-candidate, and cost bounds.
- **SC-004**: 100% of unauthorized fixture creation, fixture edit, campaign start, and cross-tenant access attempts are denied and auditable in permission tests.
- **SC-005**: Authorized users can create or edit a product-managed fixture from a candidate in under 5 minutes while retaining source provenance and edit history.
- **SC-006**: Campaign summaries correctly classify drift, failures, unsupported replay, live-validation linkage, and operator-action-needed items for at least 95% of seeded mixed-outcome campaign fixtures.
- **SC-007**: Dashboard users can page through campaign and replay evidence for large result sets with no duplicate, missing, or cross-tenant records in deterministic pagination tests.
- **SC-008**: Tool-call inspection enables engineers to distinguish original, non-live replay, live-validation, unsupported, and missing-evidence states for 100% of representative tool-call replay cases.
- **SC-009**: Final hosted-productization readiness can rerun the Roadmap 39 soak workload with Roadmap 40 live validation and Roadmap 41 evaluation workflows included, producing fault-drill, cross-tenant leakage, and resource-growth evidence.

## Assumptions

- Roadmap 33 evaluation and replay harness capabilities are available as the baseline replay substrate.
- Roadmap 35 tenant-scoped data foundations are available for evaluation reads, writes, permissions, retention, and deletion behavior.
- Roadmap 36 tenant-aware operator shell and SDK behavior is available for tenant selection and authorized operator workflows.
- Roadmap 40 live validation and side-effect replay evidence is available for tool-call inspection and campaign linkage where live validation was attempted.
- Candidate discovery runs as a bounded product workflow rather than a full-history scan triggered by every dashboard page load.
- Product-managed fixtures are mutable only through authorized product workflows; repo-managed fixtures remain immutable from this feature's editing paths.
- Model training infrastructure, autonomous self-improvement, memory promotion, and unreviewed fixture mutation are outside this feature scope.
