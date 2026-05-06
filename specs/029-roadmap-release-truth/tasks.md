# Tasks: Roadmap Authority And Release Truth Reconciliation

**Input**: Design documents from `specs/029-roadmap-release-truth/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`

**Tests**: This roadmap is documentation-only. Required verification is text-search, link
review, checklist application, and Markdown review. Run Go/client/schema tests only if an
implementation task changes scripts, validators, schemas, generated artifacts, daemon
code, SDK, web, or TUI surfaces.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish the reusable release-truth artifact and baseline source review.

- [X] T001 Create the standalone checklist file scaffold from the release-truth checklist contract in docs/runtime/release-truth-checklist.md
- [X] T002 [P] Review Roadmap 42, 43, and 44 status occurrences in docs/runtime/daemon-roadmaps.md against specs/029-roadmap-release-truth/contracts/status-vocabulary.md
- [X] T003 [P] Review sequencing and planning-boundary occurrences in docs/harness/harness-architecture.md against specs/029-roadmap-release-truth/contracts/status-vocabulary.md
- [X] T004 [P] Review upstream spec mapping and 50-task guidance in docs/specs/README.md against specs/029-roadmap-release-truth/contracts/roadmap-evidence-linkage.md
- [X] T005 [P] Review release-readiness evidence references in docs/runtime/release-readiness.md against specs/029-roadmap-release-truth/contracts/release-truth-checklist.md

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Define the shared vocabulary and checklist rules that all user stories depend on.

**CRITICAL**: No user story work can begin until this phase is complete.

- [X] T006 Add canonical status vocabulary and prohibited wording sections to docs/runtime/release-truth-checklist.md
- [X] T007 Add residual blocker classes and evidence-gap definitions to docs/runtime/release-truth-checklist.md
- [X] T008 Add no-ship, residual-work, and pass outcome rules to docs/runtime/release-truth-checklist.md
- [X] T009 Add reviewer workflow and verification command sections to docs/runtime/release-truth-checklist.md
- [X] T010 Link the standalone release-truth checklist from docs/runtime/release-readiness.md

**Checkpoint**: Shared release-truth vocabulary and reviewer checklist are ready for roadmap-specific reconciliation.

---

## Phase 3: User Story 1 - Inspect Accurate Roadmap Closure State (Priority: P1) MVP

**Goal**: Release owners can inspect the roadmap authority materials and see accurate closure states for Roadmaps 42, 43, and 44.

**Independent Test**: Review roadmap, upstream spec index, branch-local quickstart evidence, and harness architecture notes for Roadmaps 42 and 43 and confirm compatible closure states and residual evidence gaps.

### Implementation for User Story 1

- [X] T011 [US1] Update Roadmap 42 status wording in docs/runtime/daemon-roadmaps.md to state implementation and local verification complete with stable-host or real-account release evidence pending
- [X] T012 [US1] Update Roadmap 43 status wording in docs/runtime/daemon-roadmaps.md to preserve local implementation, stable-host dry-run evidence, and full-duration hosted daemon release soak pending
- [X] T013 [US1] Update Roadmap 44 status and evidence-link wording in docs/runtime/daemon-roadmaps.md so it is the current reconciliation slice before Roadmap 45
- [X] T014 [P] [US1] Update Roadmap 42 and 43 sequencing language in docs/harness/harness-architecture.md with the canonical evidence-state vocabulary
- [X] T015 [P] [US1] Update Roadmap 44 mapping and planning-truth wording in docs/specs/README.md
- [X] T016 [P] [US1] Update Roadmap 44 upstream status wording in docs/specs/029-roadmap-authority-and-release-truth-reconciliation.md to match the clarified Roadmap 42 and 43 evidence states

### Verification for User Story 1

- [X] T017 [US1] Run the Roadmap 42/43/44 status contradiction search from specs/029-roadmap-release-truth/quickstart.md and record findings in specs/029-roadmap-release-truth/quickstart.md

**Checkpoint**: Roadmap 42, 43, and 44 closure states are visible and testable without chat history.

---

## Phase 4: User Story 2 - Identify The Actual Blocker For Future Work (Priority: P1)

**Goal**: Engineers can tell whether remaining work is implementation, local verification, stable-host dry-run, full hosted soak, real-account credentials, tenant approval, operator deferral, evidence staleness, or documentation drift.

**Independent Test**: Follow links from roadmap summaries to upstream specs and branch-local quickstarts and confirm every residual item is labeled with its blocker class.

### Implementation for User Story 2

- [X] T018 [US2] Add Roadmap 42 evidence links and residual blocker labels to docs/runtime/daemon-roadmaps.md
- [X] T019 [US2] Add Roadmap 43 evidence links and residual blocker labels to docs/runtime/daemon-roadmaps.md
- [X] T020 [P] [US2] Add Roadmap 42 evidence-state notes to docs/specs/027-integration-health-and-permission-diagnostics.md
- [X] T021 [P] [US2] Add Roadmap 43 evidence-state notes to docs/specs/028-hosted-operational-profile-and-recovery.md
- [X] T022 [P] [US2] Add Roadmap 42 local verification and release-evidence residual notes to specs/027-integration-diagnostics/quickstart.md
- [X] T023 [P] [US2] Add Roadmap 43 full-duration hosted soak residual note to specs/028-hosted-operational-profile/quickstart.md
- [X] T024 [US2] Link the release-truth checklist from Roadmap 42, Roadmap 43, and Roadmap 44 sections in docs/runtime/daemon-roadmaps.md
- [X] T025 [US2] Verify Roadmap 41 evidence links remain explicit in docs/runtime/daemon-roadmaps.md and docs/runtime/release-readiness.md without reopening Roadmap 41 implementation scope

### Verification for User Story 2

- [X] T026 [US2] Manually follow updated Roadmap 41, 42, and 43 evidence links and record broken-link or residual-gap findings in specs/029-roadmap-release-truth/quickstart.md

**Checkpoint**: Remaining blockers are classified as release evidence gaps unless implementation is explicitly missing.

---

## Phase 5: User Story 3 - Start The Next Roadmap Without Status Archaeology (Priority: P2)

**Goal**: Planners can start Roadmap 45 from one accurate roadmap/spec authority and the standard task budget rule.

**Independent Test**: Open roadmap and upstream spec indexes and confirm Roadmap 44 is the active reconciliation step, Roadmap 45 is next, and future planning guidance preserves the standard task budget.

### Implementation for User Story 3

- [X] T027 [US3] Update non-knowledge parity sequencing and under-50-task guidance around Roadmaps 44 and 45 in docs/runtime/daemon-roadmaps.md
- [X] T028 [P] [US3] Update future-spec sizing guidance in docs/harness/harness-architecture.md to use the canonical under-50-task planning boundary
- [X] T029 [P] [US3] Update future-spec sizing guidance in docs/specs/README.md to require splitting oversized upstream specs before implementation planning
- [X] T030 [P] [US3] Update planning-boundary wording in docs/specs/029-roadmap-authority-and-release-truth-reconciliation.md

### Verification for User Story 3

- [X] T031 [US3] Run the planning-boundary search from specs/029-roadmap-release-truth/quickstart.md and record whether wording appears only in authoritative planning areas in specs/029-roadmap-release-truth/quickstart.md

**Checkpoint**: Roadmap 45 can start from current authority without reopening Roadmaps 42 through 44.

---

## Phase 6: User Story 4 - Enforce Evidence-Backed Public Readiness (Priority: P2)

**Goal**: Release reviewers can use the standalone checklist to reject public-readiness claims without linked evidence.

**Independent Test**: Open the standalone checklist from roadmap/spec links, apply it to Roadmaps 42 and 43, and confirm missing full hosted soak or real-account smoke evidence remains visible as no-ship or residual release work.

### Implementation for User Story 4

- [X] T032 [US4] Add Roadmap 42 checklist application example to docs/runtime/release-truth-checklist.md
- [X] T033 [US4] Add Roadmap 43 checklist application example to docs/runtime/release-truth-checklist.md
- [X] T034 [US4] Add public-readiness no-ship examples for missing, stale, mismatched, dry-run-only, and secret-exposing evidence to docs/runtime/release-truth-checklist.md
- [X] T035 [US4] Link docs/runtime/release-truth-checklist.md from docs/specs/029-roadmap-authority-and-release-truth-reconciliation.md after T030 updates the same file
- [X] T036 [US4] Link docs/runtime/release-truth-checklist.md from docs/harness/harness-architecture.md after T028 updates the same file

### Verification for User Story 4

- [X] T037 [US4] Apply docs/runtime/release-truth-checklist.md to Roadmaps 42 and 43 and record pass, residual-work, or no-ship outcomes in specs/029-roadmap-release-truth/quickstart.md

**Checkpoint**: Public-readiness claims are checklist-backed and reject missing or mismatched release evidence.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final consistency checks, documentation polish, and explicit test boundary recording.

- [X] T038 [P] Normalize release-truth terminology across docs/runtime/release-readiness.md and docs/runtime/release-truth-checklist.md
- [X] T039 [P] Verify Markdown links in docs/runtime/daemon-roadmaps.md, docs/harness/harness-architecture.md, docs/specs/README.md, docs/runtime/release-readiness.md, docs/runtime/release-truth-checklist.md, and docs/specs/029-roadmap-authority-and-release-truth-reconciliation.md
- [X] T040 Run the full quickstart validation flow in specs/029-roadmap-release-truth/quickstart.md and record the final validation summary in specs/029-roadmap-release-truth/quickstart.md
- [X] T041 Record the documentation-only Go/client/schema test boundary and daemon/go mod tidy result in specs/029-roadmap-release-truth/quickstart.md
- [X] T042 Final review specs/029-roadmap-release-truth/tasks.md to confirm the completed Roadmap 44 task count remains below 50 tasks

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - blocks all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational - MVP closure-state reconciliation
- **User Story 2 (Phase 4)**: Depends on Foundational; benefits from US1 wording but remains independently verifiable by evidence-link review
- **User Story 3 (Phase 5)**: Depends on Foundational; can proceed after US1 if roadmap sequencing is stable
- **User Story 4 (Phase 6)**: Depends on Foundational; checklist-content tasks can proceed after checklist sections exist, but shared-file link tasks must wait for US3 edits to the same files
- **Polish (Phase 7)**: Depends on desired user stories being complete

### User Story Dependencies

- **US1 (P1)**: MVP. Establishes accurate Roadmap 42, 43, and 44 closure-state wording.
- **US2 (P1)**: Adds evidence links and blocker classes. Can start after Phase 2, but should reconcile with US1 wording before final verification.
- **US3 (P2)**: Updates next-roadmap planning and task-budget guidance. Can start after Phase 2 and US1 roadmap sequencing edits.
- **US4 (P2)**: Makes public-readiness review checklist-backed. Checklist content can start after Phase 2; shared-file links in `docs/harness/harness-architecture.md` and `docs/specs/029-roadmap-authority-and-release-truth-reconciliation.md` wait for US3 updates.

### Parallel Opportunities

- Setup source reviews T002, T003, T004, and T005 can run in parallel.
- US1 tasks T014, T015, and T016 touch different files and can run in parallel after Roadmap status wording is drafted.
- US2 tasks T020, T021, T022, and T023 touch different files and can run in parallel.
- US3 tasks T028, T029, and T030 touch different files and can run in parallel.
- US4 checklist-content tasks T032, T033, and T034 run in sequence within `docs/runtime/release-truth-checklist.md`; link tasks T035 and T036 must wait for T030 and T028 respectively.
- Polish terminology and link verification tasks T038 and T039 can run in parallel.

---

## Parallel Example: User Story 2

```text
Task: "Add Roadmap 42 evidence-state notes to docs/specs/027-integration-health-and-permission-diagnostics.md"
Task: "Add Roadmap 43 evidence-state notes to docs/specs/028-hosted-operational-profile-and-recovery.md"
Task: "Add Roadmap 42 local verification and release-evidence residual notes to specs/027-integration-diagnostics/quickstart.md"
Task: "Add Roadmap 43 full-duration hosted soak residual note to specs/028-hosted-operational-profile/quickstart.md"
```

## Sequenced Example: User Story 4

```text
Task: "Add Roadmap 42 checklist application example to docs/runtime/release-truth-checklist.md"
Task: "Add Roadmap 43 checklist application example to docs/runtime/release-truth-checklist.md"
Task: "Add public-readiness no-ship examples for missing, stale, mismatched, dry-run-only, and secret-exposing evidence to docs/runtime/release-truth-checklist.md"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational checklist vocabulary and rules.
3. Complete Phase 3: User Story 1.
4. Stop and validate Roadmap 42, 43, and 44 closure-state wording with the quickstart search.

### Incremental Delivery

1. Add shared checklist vocabulary and rules.
2. Deliver US1 so release owners can inspect accurate closure state.
3. Deliver US2 so engineers can follow evidence links and blocker classes.
4. Deliver US3 so Roadmap 45 planning starts from reconciled truth.
5. Deliver US4 so public-readiness claims are checklist-backed.
6. Finish cross-cutting link, terminology, and quickstart validation.

### Documentation-Only Test Boundary

Do not run daemon/client/schema tests for pure Markdown changes. If implementation changes
scripts, validators, schemas, generated artifacts, daemon code, SDK, web, or TUI surfaces,
run the relevant repository tests and record the changed test boundary in
`specs/029-roadmap-release-truth/quickstart.md`.

---

## Notes

- Every task is scoped to exact repository paths.
- Story phases are independently reviewable through the quickstart checks.
- Historical evidence must be linked and classified, not rewritten.
- Missing real-account credentials and pending hosted soak are release evidence gaps, not implementation gaps.
- Keep the final task list below 50 tasks for this standard branch-local spec.
