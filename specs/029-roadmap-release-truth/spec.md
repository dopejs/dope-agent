# Feature Specification: Roadmap Authority And Release Truth Reconciliation

**Feature Branch**: `029-roadmap-release-truth`  
**Created**: 2026-05-06  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/029-roadmap-authority-and-release-truth-reconciliation.md 完成 phase 44 的工作"

## Clarifications

### Session 2026-05-06

- Q: What final status should Roadmap 42 use for reconciliation? → A: Implementation and local verification complete; stable-host or real-account release evidence remains pending.
- Q: Where should the release-truth checklist live? → A: Create a standalone reusable release-truth checklist and link it from roadmap/spec materials.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Inspect Accurate Roadmap Closure State (Priority: P1)

As a release owner, I can inspect the roadmap authority materials and see the current closure state for Roadmaps 42, 43, and 44 without reconciling stale labels by hand.

**Why this priority**: Future release and parity planning depends on knowing whether a roadmap is blocked by implementation, local verification, stable-host evidence, full hosted soak, or real-account smoke.

**Independent Test**: Can be fully tested by reviewing the roadmap index, upstream spec index, branch-local quickstart evidence, and harness architecture notes for Roadmaps 42 and 43 and confirming they report compatible closure states and residual evidence gaps.

**Acceptance Scenarios**:

1. **Given** Roadmap 42 is implementation and local verification complete with remaining stable-host or real-account release evidence gaps, **When** the release owner reviews the roadmap materials, **Then** the status distinguishes completed implementation scope from remaining release evidence work.
2. **Given** Roadmap 43 has local implementation evidence, stable-host dry-run evidence, and a remaining full-duration hosted soak gap, **When** the release owner reviews the roadmap materials, **Then** the status identifies each of those evidence states without implying public readiness.
3. **Given** Roadmap 44 is the reconciliation slice, **When** the release owner reviews the roadmap sequence, **Then** Roadmap 44 is visible as the current planning-truth closure item before Roadmap 45 begins.

---

### User Story 2 - Identify The Actual Blocker For Future Work (Priority: P1)

As an engineer, I can determine whether a future roadmap is blocked by missing code, missing local verification, missing stable-host dry-run evidence, missing full hosted soak, unavailable safe real-account credentials, or only documentation status drift.

**Why this priority**: Engineers should not reopen completed implementation scope or schedule public-readiness work against stale "planned" or "pending" labels.

**Independent Test**: Can be fully tested by following the links from roadmap summaries to upstream specs and branch-local quickstarts and confirming that each residual item is labeled with its blocker class.

**Acceptance Scenarios**:

1. **Given** a roadmap has implementation and local verification evidence, **When** an engineer reviews its closure notes, **Then** any remaining hosted or real-account evidence gap is not described as an implementation gap.
2. **Given** safe real-account credentials are unavailable or a live smoke is intentionally skipped, **When** an engineer reviews release evidence notes, **Then** the gap is recorded as credential or approval-bound residual work with no secret material.
3. **Given** historical evidence exists for a completed roadmap, **When** an engineer follows a release-evidence link, **Then** the linked evidence is classified rather than rewritten or silently superseded.

---

### User Story 3 - Start The Next Roadmap Without Status Archaeology (Priority: P2)

As a planner, I can start Roadmap 45 from the upstream spec index and current roadmap authority without rediscovering which Roadmaps 42 through 44 are complete, partially evidenced, or pending release validation.

**Why this priority**: The non-knowledge parity program should continue from one shared planning truth rather than re-litigating closed scope.

**Independent Test**: Can be fully tested by opening the roadmap and upstream spec indexes and confirming that Roadmap 44 is the active reconciliation step, Roadmap 45 is next, and future planning guidance preserves the standard task budget.

**Acceptance Scenarios**:

1. **Given** the planner opens the upstream spec index, **When** they locate Roadmaps 44 and 45, **Then** the index reflects Roadmap 44 as the reconciliation step and Roadmap 45 as the next product-scope spec.
2. **Given** a future roadmap scope naturally grows beyond the standard task budget, **When** planning guidance is reviewed, **Then** the guidance requires splitting the upstream spec before implementation planning.
3. **Given** completed scope from Roadmaps 42 or 43 is referenced by a future spec, **When** the planner reviews dependencies, **Then** those dependencies link to the appropriate quickstart, runbook, or release-evidence material.

---

### User Story 4 - Enforce Evidence-Backed Public Readiness (Priority: P2)

As a release reviewer, I can use a standalone reusable release-truth checklist to reject public-readiness claims that lack linked evidence or that confuse local implementation with hosted release validation.

**Why this priority**: Public readiness must be tied to reviewable artifacts, not implied by implementation completion.

**Independent Test**: Can be fully tested by opening the standalone checklist from roadmap or spec links, applying it to Roadmaps 42 and 43, and confirming that missing full hosted soak or real-account smoke evidence remains visible as no-ship or residual release work.

**Acceptance Scenarios**:

1. **Given** a roadmap claims public readiness, **When** the reviewer applies the standalone release-truth checklist, **Then** the claim is accepted only if current release evidence is linked.
2. **Given** a roadmap has local implementation and local verification but lacks hosted soak evidence, **When** the checklist is applied, **Then** the roadmap cannot be classified as publicly ready.
3. **Given** evidence is stale, mismatched, missing, or only a dry-run, **When** the checklist is applied, **Then** the limitation is explicitly recorded.

### Edge Cases

- A roadmap status appears in multiple documents with different wording; reconciliation must either make the meanings equivalent or identify the more authoritative source.
- A quickstart records local verification but no stable-host or real-account evidence; status must not collapse those states into either "done" or "not started."
- A stable-host dry-run exists but no full-duration hosted daemon soak exists; release notes must preserve that distinction.
- Real-account smoke is skipped because safe credentials or approvals are unavailable; the skip must be visible as residual work and must not expose secret material.
- Historical evidence uses older wording; the feature must classify and link it without rewriting the evidence record itself.
- Future roadmap text attempts to exceed the standard task budget; planning guidance must require a split before implementation planning.
- Documentation-only changes are reviewed alongside code-bearing specs; verification expectations must make clear that code tests are not required unless validators or scripts change.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The documentation set MUST use a consistent status vocabulary for proposed, implementation complete, local verification complete, stable-host dry-run complete, full hosted soak pending, real-account smoke pending, and public readiness states.
- **FR-002**: Every implemented roadmap after Roadmap 39 that is referenced by this phase MUST link its upstream spec, branch-local planning artifacts, quickstart or runbook evidence, and any release-readiness evidence known at the time of reconciliation.
- **FR-003**: Roadmap 42 status MUST state that implementation and local verification are complete while distinguishing any remaining stable-host smoke, real-account smoke, credential, approval, or hosted release evidence gaps.
- **FR-004**: Roadmap 43 status MUST distinguish current implementation and local verification evidence, stable-host dry-run evidence, and the remaining full-duration hosted daemon release soak gap.
- **FR-005**: Roadmap 44 MUST be represented as the current roadmap authority and release-truth reconciliation slice that closes before Roadmap 45 starts.
- **FR-006**: No roadmap, harness note, upstream spec, or release-readiness note may claim public readiness unless it links current release evidence sufficient for that claim.
- **FR-007**: The upstream spec index and roadmap sequencing materials MUST agree on the mapping from Roadmap 44 to the roadmap authority and release truth reconciliation work and from Roadmap 45 to the next hosted activation work.
- **FR-008**: Future planning guidance MUST state that standard branch-local implementation specs should remain below 50 tasks and that oversized upstream specs should be split before implementation planning.
- **FR-009**: The release-truth checklist MUST be a standalone reusable artifact linked from roadmap and spec materials.
- **FR-009a**: The release-truth checklist MUST require reviewers to classify implementation state, local verification state, stable-host dry-run state, full hosted soak state, real-account smoke state, evidence freshness, evidence links, residual gaps, and public-readiness eligibility.
- **FR-010**: Residual evidence work MUST be labeled by blocker class, including implementation missing, verification missing, hosted soak pending, stable-host dry-run pending, real-account credentials unavailable, tenant approval unavailable, operator-deferred, or evidence stale.
- **FR-011**: Historical evidence MUST be linked and classified without rewriting the original evidence record or erasing prior verification notes.
- **FR-012**: The reconciliation MUST remain documentation-only unless planning discovers a broken validator or script that is required to verify documentation consistency.

### Key Entities

- **Roadmap Closure Record**: A documented statement of a roadmap's implementation state, verification state, evidence links, and residual gaps.
- **Status Vocabulary**: The shared set of labels used to distinguish proposed work, implemented work, locally verified work, stable-host dry-run evidence, full hosted soak evidence, real-account smoke evidence, and public readiness.
- **Release Evidence Link**: A reference from roadmap or planning material to quickstart, runbook, smoke, soak, or release-review evidence.
- **Residual Evidence Gap**: A documented remaining validation requirement that does not imply missing implementation unless explicitly classified that way.
- **Release-Truth Checklist**: Standalone reusable reviewer-facing criteria used to decide whether roadmap closure and public-readiness claims are evidence-backed.
- **Planning Boundary**: The documented limit that future standard branch-local specs should stay below 50 tasks or be split before planning.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: No compatibility-impacting API, schema, event, config, or storage surface changes are required. The feature changes planning documents, roadmap status wording, evidence indexes, and reviewer guidance.
- **Migration / Rollback**: No data migration or backfill is required. Rollback is reverting the documentation and checklist changes while preserving all historical evidence artifacts and prior verification records.
- **Verification Strategy**: Required validation includes text searches for stale or contradictory Roadmap 42 and Roadmap 43 status claims, manual link review from roadmap summaries to branch-local quickstart or release evidence, and checklist review that no public-readiness claim lacks linked evidence. Code tests are required only if scripts, validators, schemas, or generated artifacts change.
- **Observability Impact**: The feature improves operator-visible planning observability by adding explicit status vocabulary, evidence links, residual gap labels, and release-truth checklist criteria. It does not add runtime logs, metrics, events, or audit trails.
- **Environment & Secrets**: The feature is documentation-only and must not use production data, live connectors, real-account credentials, or privileged secrets. Any references to real-account smoke must describe evidence or residual gaps without exposing credential material.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of Roadmap 42 and Roadmap 43 status statements in roadmap, harness, upstream spec, and branch-local planning materials either agree or explicitly identify the authoritative interpretation.
- **SC-002**: A release owner can identify the implementation state, local verification state, and remaining release-evidence gaps for Roadmaps 42 and 43 in 10 minutes or less without reading chat history.
- **SC-003**: 100% of implemented roadmaps after Roadmap 39 that are referenced by this reconciliation link to their quickstart, runbook, or release evidence where such evidence exists.
- **SC-004**: 100% of public-readiness or ship-readiness claims in the reconciled materials include linked evidence or are downgraded to residual release work.
- **SC-005**: 100% of Roadmap 42 remaining stable-host or real-account smoke gaps and Roadmap 43 remaining full hosted soak gaps are labeled as release evidence gaps rather than implementation gaps.
- **SC-006**: The standalone release-truth checklist can be reached from roadmap and spec materials, applied to Roadmaps 42 and 43, and produce a clear pass, residual-work, or no-ship classification for each release-readiness claim.
- **SC-007**: Future planning guidance states the under-50-task standard for branch-local specs exactly once in each authoritative planning area where future roadmap sizing is discussed.
- **SC-008**: Manual link review finds zero broken links among Roadmap 44's authoritative upstream spec, roadmap mapping, harness sequencing note, and branch-local spec artifacts.

## Assumptions

- `docs/specs/029-roadmap-authority-and-release-truth-reconciliation.md` is the authoritative upstream input for this feature.
- Roadmaps 39, 41, 42, and 43 provide the relevant evidence baseline for this reconciliation.
- Roadmap 42 is treated as implementation and local verification complete, with remaining stable-host or real-account release evidence classification to be made explicit from existing artifacts.
- Roadmap 43 is treated as locally implemented with stable-host dry-run evidence and a remaining full-duration hosted daemon release soak gap.
- This phase does not change runtime behavior, connector behavior, provider behavior, context engineering, memory, knowledge-plane design, or production release gates.
- Documentation should classify existing evidence and residual gaps; it should not invent new release evidence or rewrite historical evidence artifacts.
