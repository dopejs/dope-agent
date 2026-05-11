# Tasks: Non-Knowledge Multi-Turn Continuity

**Input**: Design documents from `/specs/040-multi-turn-continuity/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/thread-continuity.md, quickstart.md

**Tests**: Required. This production change modifies persistence, API schemas, event schemas, chat behavior, SDK types, Web, TUI, and operator-visible evidence. Write targeted tests before implementation for each affected boundary.

**Organization**: Tasks are grouped by user story so each story can be independently implemented and tested after shared foundations are complete.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel because it touches different files and does not depend on incomplete tasks
- **[Story]**: User story label for story phases only
- Every task includes an exact file path

## Phase 1: Setup

**Purpose**: Align planning artifacts, contract inventory, and shared test harness entry points before implementation.

- [X] T001 Review Roadmap 55 scope and preserve the current active plan reference in `AGENTS.md`
- [X] T002 [P] Add continuity contract fixture inventory notes to `specs/040-multi-turn-continuity/contracts/thread-continuity.md`
- [X] T003 [P] Add quickstart verification placeholders for measured latency and preview inspection timing in `specs/040-multi-turn-continuity/quickstart.md`
- [X] T004 [P] Add schema inventory entries for planned continuity API and event schemas in `schemas/api/README.md`
- [X] T005 [P] Add event schema inventory entries for continuity evidence events in `schemas/events/README.md`
- [X] T006 [P] Add SDK task coverage notes for continuity tests in `sdk/ts/src/thread-continuity.test.ts`
- [X] T007 [P] Add Web task coverage notes for continuity preview behavior in `web/src/features/thread-lifecycle.test.tsx`
- [X] T008 [P] Add TUI task coverage notes for continuity preview commands in `tui/src/cli.test.ts`

---

## Phase 2: Foundational

**Purpose**: Shared persistence, domain, schema, and event foundations that block all user stories.

**Critical**: No user story implementation begins until this phase is complete.

- [X] T009 Add schema migration v51 tables and indexes for `thread_continuity_turns`, `thread_continuity_previews`, and `thread_continuity_preview_items` in `daemon/internal/store/store.go`
- [X] T010 [P] Add migration/schema tests for continuity tables, unique acceptance sequence, indexes, and backward compatibility in `daemon/internal/store/thread_continuity_test.go`
- [X] T011 [P] Add migration fixture coverage for pre-v51 databases with existing Roadmap 54 thread rows in `daemon/internal/store/migrationfixture/r55_thread_continuity_test.go`
- [X] T012 [P] Define continuity domain types, statuses, mode values, exclusion reasons, and policy constants in `daemon/internal/threads/continuity.go`
- [X] T013 [P] Add continuity policy unit tests for default window, age limit, source status, and exclusion reason values in `daemon/internal/threads/continuity_test.go`
- [X] T014 Add tenant-safe store interfaces for continuity turn and preview persistence in `daemon/internal/store/thread_continuity.go`
- [X] T015 Add tenant boundary tests for continuity store access, denial indistinguishability, and retention filtering in `daemon/internal/store/tenancy/threads_test.go`
- [X] T016 Add tenant-safe continuity helper methods to `daemon/internal/store/tenancy/threads.go`
- [X] T017 [P] Add continuity preview API schema in `schemas/api/thread-continuity-preview.schema.json`
- [X] T018 [P] Add continuity preview item API schema in `schemas/api/thread-continuity-preview-item.schema.json`
- [X] T019 [P] Add continuity preview detail response API schema in `schemas/api/thread-continuity-preview.response.schema.json`
- [X] T020 [P] Add continuity turn recorded event schema in `schemas/events/thread-continuity-turn-recorded.event.schema.json`
- [X] T021 [P] Add continuity preview recorded event schema in `schemas/events/thread-continuity-preview-recorded.event.schema.json`
- [X] T022 [P] Add Go event constructors for continuity turn and preview evidence in `daemon/internal/events/thread_continuity.go`
- [X] T023 [P] Add event constructor tests for metadata-only continuity evidence in `daemon/internal/events/thread_continuity_test.go`
- [X] T024 Add contract test scaffolding for continuity API and event schemas in `daemon/internal/contracts/thread_continuity_contracts_test.go`
- [X] T025 Add persistence rollback and mixed-version notes to `docs/runtime/thread-session-lifecycle.md`

**Checkpoint**: Foundation ready. User story implementation can begin.

---

## Phase 3: User Story 1 - Continue Recent Thread Context (Priority: P1)

**Goal**: Users can ask follow-ups in an active thread and receive responses using only bounded recent turns from the current thread segment.

**Independent Test**: Start an active thread, ask an initial question, ask a follow-up that depends on the prior exchange, and verify only eligible recent turns from the current segment are included.

### Tests for User Story 1

- [X] T026 [P] [US1] Add chat service tests for empty, within-limit, over-limit, age-limited, and current-segment continuity assembly in `daemon/internal/chat/service_test.go`
- [X] T027 [P] [US1] Add domain tests for daemon acceptance sequence ordering and 12-turn window selection in `daemon/internal/threads/continuity_test.go`
- [X] T028 [P] [US1] Add store tests for turn insertion, assistant response linkage, and bounded recent-turn lookup in `daemon/internal/store/thread_continuity_test.go`
- [X] T029 [P] [US1] Add API tests for optional `threadId`, single-turn compatibility, and continuity response fields in `daemon/internal/api/thread_continuity_test.go`
- [X] T030 [P] [US1] Add contract tests for chat request/response additive continuity fields in `daemon/internal/contracts/thread_continuity_contracts_test.go`

### Implementation for User Story 1

- [X] T031 [P] [US1] Implement continuity turn builders and safe content validation in `daemon/internal/threads/continuity.go`
- [X] T032 [US1] Implement transactional daemon acceptance sequence allocation in `daemon/internal/store/thread_continuity.go`
- [X] T033 [US1] Implement continuity turn persistence and bounded lookup by tenant, thread, and session segment in `daemon/internal/store/thread_continuity.go`
- [X] T034 [US1] Extend chat query input and result types with thread and continuity metadata in `daemon/internal/chat/service.go`
- [X] T035 [US1] Implement bounded recent-turn assembly before LLM dispatch in `daemon/internal/chat/service.go`
- [X] T036 [US1] Persist request and assistant response continuity turns around non-stream chat dispatch in `daemon/internal/chat/service.go`
- [X] T037 [US1] Persist request and assistant response continuity turns around streaming chat dispatch in `daemon/internal/chat/service.go`
- [X] T038 [US1] Extend chat request parsing and response serialization with optional continuity fields in `daemon/internal/api/server.go`
- [X] T039 [US1] Update chat query request schema for `threadId` and continuity mode in `schemas/api/chat-query.request.schema.json`
- [X] T040 [US1] Update chat query response schema for continuity metadata fields in `schemas/api/chat-query.response.schema.json`
- [X] T041 [US1] Update stream started event schema for additive continuity metadata in `schemas/api/chat-query-stream-started.event.schema.json`
- [X] T042 [US1] Add SDK chat input and response continuity types in `sdk/ts/src/index.ts`
- [X] T043 [US1] Add SDK tests for thread-aware chat and single-turn compatibility in `sdk/ts/src/thread-continuity.test.ts`

**Checkpoint**: User Story 1 is independently functional and testable.

---

## Phase 4: User Story 2 - Reset Continuity Explicitly (Priority: P1)

**Goal**: Reset starts a fresh continuity boundary on the same thread and prevents pre-reset turns from affecting future responses.

**Independent Test**: Create a thread with prior turns, reset it, ask a follow-up that would require pre-reset context, and verify pre-reset turns are excluded while historical evidence remains inspectable.

### Tests for User Story 2

- [X] T044 [P] [US2] Add reset-boundary domain tests for excluding pre-reset turns in `daemon/internal/threads/continuity_test.go`
- [X] T045 [P] [US2] Add store tests for continuity lookup after reset and historical preview retention in `daemon/internal/store/thread_continuity_test.go`
- [X] T046 [P] [US2] Add API tests proving reset requires `connectors.manage` and future continuity excludes pre-reset turns in `daemon/internal/api/thread_continuity_test.go`
- [X] T047 [P] [US2] Add restart tests proving active, reset, archived, and reopened thread continuity turns, daemon acceptance sequence, inclusion decisions, reset boundaries, and preview evidence survive daemon restart in `daemon/internal/store/thread_continuity_restart_test.go`

### Implementation for User Story 2

- [X] T048 [US2] Integrate Roadmap 54 reset segment boundary into continuity lookup filters in `daemon/internal/store/thread_continuity.go`
- [X] T049 [US2] Add reset-boundary exclusion reason generation to continuity preview assembly in `daemon/internal/threads/continuity.go`
- [X] T050 [US2] Persist reset-aware preview items for excluded pre-reset turns in `daemon/internal/store/thread_continuity.go`
- [X] T051 [US2] Ensure thread reset actions keep using `connectors.manage` for continuity reset behavior in `daemon/internal/api/thread_lifecycle.go`
- [X] T052 [US2] Add reset-boundary continuity preview projection to thread detail responses in `daemon/internal/threads/projection.go`
- [X] T053 [US2] Add reset-boundary continuity evidence to Web thread detail rendering in `web/src/features/thread-lifecycle.tsx`
- [X] T054 [US2] Add reset-boundary continuity output to TUI thread detail rendering in `tui/src/cli.ts`
- [X] T055 [US2] Add SDK reset-boundary tests for continuity metadata in `sdk/ts/src/thread-continuity.test.ts`

**Checkpoint**: User Story 2 is independently functional and testable.

---

## Phase 5: User Story 3 - Inspect Included Continuity Evidence (Priority: P1)

**Goal**: Operators can inspect included and excluded recent-turn evidence, policy limits, artifact excerpts, redaction state, and reset boundaries.

**Independent Test**: Produce responses with no prior turns, within-limit turns, over-limit turns, reset-excluded turns, redacted turns, and artifact-linked turns, then inspect preview evidence through product APIs.

### Tests for User Story 3

- [X] T056 [P] [US3] Add preview detail API tests for included and excluded item evidence in `daemon/internal/api/thread_continuity_test.go`
- [X] T057 [P] [US3] Add store tests for preview, preview item, artifact excerpt, retention, and redaction persistence in `daemon/internal/store/thread_continuity_test.go`
- [X] T058 [P] [US3] Add contract tests for preview schemas and continuity event schemas in `daemon/internal/contracts/thread_continuity_contracts_test.go`
- [X] T059 [P] [US3] Add redaction tests for unsafe turn content and artifact excerpt suppression in `daemon/internal/threads/redaction_test.go`
- [X] T060 [P] [US3] Add SDK tests for `getThreadContinuityPreview` and permission denial behavior in `sdk/ts/src/thread-continuity.test.ts`
- [X] T061 [P] [US3] Add Web preview summary and detail tests, including reset-boundary evidence rendered after US2 reset behavior, in `web/src/features/thread-lifecycle.test.tsx`
- [X] T062 [P] [US3] Add TUI preview inspection command tests, including reset-boundary evidence rendered after US2 reset behavior, in `tui/src/cli.test.ts`

### Implementation for User Story 3

- [X] T063 [US3] Implement continuity preview and preview item builders in `daemon/internal/threads/continuity.go`
- [X] T064 [US3] Implement preview and preview item persistence plus retention filtering in `daemon/internal/store/thread_continuity.go`
- [X] T065 [US3] Implement user-visible artifact excerpt eligibility and reference-only artifact handling in `daemon/internal/threads/continuity.go`
- [X] T066 [US3] Implement safe summary and redaction failure handling for continuity previews in `daemon/internal/threads/redaction.go`
- [X] T067 [US3] Add continuity preview summaries to thread detail model in `daemon/internal/threads/projection.go`
- [X] T068 [US3] Add continuity preview summary to thread detail response schema in `schemas/api/thread-detail.response.schema.json`
- [X] T069 [US3] Add `GET /v1/threads/{threadId}/continuity-previews/{previewId}` route in `daemon/internal/api/thread_lifecycle.go`
- [X] T070 [US3] Publish continuity turn and preview evidence events from store or chat paths in `daemon/internal/events/thread_continuity.go`
- [X] T071 [US3] Add SDK preview types and `getThreadContinuityPreview` client method in `sdk/ts/src/index.ts`
- [X] T072 [US3] Add Web continuity preview summary and detail UI in `web/src/features/thread-lifecycle.tsx`
- [X] T073 [US3] Add TUI continuity preview inspection output in `tui/src/cli.ts`

**Checkpoint**: User Story 3 is independently functional and testable.

---

## Phase 6: User Story 4 - Preserve Continuity Across Supported Surfaces (Priority: P2)

**Goal**: Web, TUI, and supported channel paths apply the same continuity behavior when they carry daemon-owned thread identity.

**Independent Test**: Create equivalent active threads through Web, TUI, and channel paths, ask follow-ups, and verify the same inclusion limits, reset boundaries, and evidence classifications.

### Tests for User Story 4

- [X] T074 [P] [US4] Add Web thread-aware chat flow tests in `web/src/app/App.test.tsx`
- [X] T075 [P] [US4] Add TUI thread-aware chat argument tests in `tui/src/cli.test.ts`
- [X] T076 [P] [US4] Add connector ingress continuity tests for accepted, duplicate, and unsupported source messages in `daemon/internal/im/loop_test.go`
- [X] T077 [P] [US4] Add connector conformance regression tests for source identity and archived lifecycle blocking in `daemon/internal/connectors/conformance_test.go`
- [X] T078 [P] [US4] Add API streaming continuity parity tests in `daemon/internal/api/thread_continuity_test.go`

### Implementation for User Story 4

- [X] T079 [US4] Add explicit thread ID support to Web chat request flow in `web/src/app/App.tsx`
- [X] T080 [US4] Add explicit thread ID and preview ID display support to Web lifecycle feature in `web/src/features/thread-lifecycle.tsx`
- [X] T081 [US4] Add TUI `--thread-id` chat option and continuity response output in `tui/src/cli.ts`
- [X] T082 [US4] Attach accepted connector messages to continuity turns when thread identity is valid in `daemon/internal/im/loop.go`
- [X] T083 [US4] Prevent unsupported or missing source identity from inferring continuity in `daemon/internal/im/loop.go`
- [X] T084 [US4] Suppress duplicate/replayed connector source events from continuity turn creation in `daemon/internal/store/thread_continuity.go`
- [X] T085 [US4] Ensure archived lifecycle state blocks channel continuity until reopened in `daemon/internal/threads/continuity.go`
- [X] T086 [US4] Add streaming started and terminal continuity metadata serialization in `daemon/internal/api/server.go`
- [X] T087 [US4] Update minimal chat client docs for thread-aware continuity behavior in `docs/runtime/minimal-chat-clients.md`
- [X] T088 [US4] Update channel conformance docs for continuity source identity and duplicate suppression in `docs/channels/channel-connector-conformance.md`

**Checkpoint**: User Story 4 is independently functional and testable.

---

## Phase 7: User Story 5 - Keep Continuity Separate From Knowledge (Priority: P2)

**Goal**: The feature remains bounded recent-thread continuity only, with no memory, semantic retrieval, summaries, knowledge graph behavior, or cross-thread personalization.

**Independent Test**: Configure related content outside eligible recent turns, then verify responses, evidence, reset behavior, retention, and docs show no memory or knowledge-plane behavior.

### Tests for User Story 5

- [X] T089 [P] [US5] Add negative chat tests proving unrelated threads, old turns, and provider-retained context are not used in `daemon/internal/chat/service_test.go`
- [X] T090 [P] [US5] Add domain tests proving legacy or partial evidence is excluded unless eligibility is proven in `daemon/internal/threads/continuity_test.go`
- [X] T091 [P] [US5] Add retention tests for 30-day active window and 90-day inspection expiry in `daemon/internal/store/thread_continuity_test.go`
- [X] T092 [P] [US5] Add contract fixture tests proving no raw prompt, memory, summary, or knowledge fields are emitted in `daemon/internal/contracts/thread_continuity_contracts_test.go`
- [X] T093 [P] [US5] Add Web tests for non-memory continuity labeling in `web/src/features/thread-lifecycle.test.tsx`

### Implementation for User Story 5

- [X] T094 [US5] Enforce same-thread and current-session-segment eligibility in `daemon/internal/threads/continuity.go`
- [X] T095 [US5] Enforce legacy or partial evidence exclusion in `daemon/internal/store/thread_continuity.go`
- [X] T096 [US5] Enforce active 30-day continuity window separately from 90-day inspection retention in `daemon/internal/store/thread_continuity.go`
- [X] T097 [US5] Add retention application helpers for continuity turns and previews in `daemon/internal/store/thread_continuity.go`
- [X] T098 [US5] Ensure chat dispatch assembly never calls memory, summary, semantic retrieval, or knowledge-plane code in `daemon/internal/chat/service.go`
- [X] T099 [US5] Add non-memory continuity labels and descriptions to Web thread lifecycle UI in `web/src/features/thread-lifecycle.tsx`
- [X] T100 [US5] Add non-memory continuity labels to TUI preview output in `tui/src/cli.ts`
- [X] T101 [US5] Update runtime docs with non-knowledge continuity boundaries in `docs/runtime/thread-session-lifecycle.md`

**Checkpoint**: User Story 5 is independently functional and testable.

---

## Phase 8: Polish & Cross-Cutting

**Purpose**: Final verification, documentation alignment, and release-readiness checks across all stories.

- [X] T102 [P] Update contract documentation with final route, schema, event, SDK, Web, and TUI behavior in `specs/040-multi-turn-continuity/contracts/thread-continuity.md`
- [X] T103 [P] Add default-window p95 continuity assembly performance validation for the 12-turn window in `daemon/internal/store/perf_thread_continuity_test.go`
- [X] T104 [P] Update quickstart with actual measured p95 continuity assembly latency, operator preview inspection timing, and any structured live-connector skips in `specs/040-multi-turn-continuity/quickstart.md`
- [X] T105 [P] Add chat continuity request and response contract fixtures in `daemon/internal/contracts/testdata/thread_continuity/chat-query-continuity.json`
- [X] T106 [P] Add continuity preview detail contract fixtures in `daemon/internal/contracts/testdata/thread_continuity/thread-continuity-preview.json`
- [X] T107 [P] Add continuity turn and preview event payload fixtures in `daemon/internal/contracts/testdata/thread_continuity/thread-continuity-events.json`
- [X] T108 [P] Add or update API schema fixture index for continuity request, response, preview, and event payloads in `daemon/internal/contracts/testdata/thread_continuity/README.md`
- [X] T109 [P] Run schema and contract validation and record any residual gaps in `specs/040-multi-turn-continuity/quickstart.md`
- [X] T110 [P] Run focused daemon tests, including restart and p95 performance validation, and record command results in `specs/040-multi-turn-continuity/quickstart.md`
- [X] T111 [P] Run SDK, Web, and TUI client tests and record command results in `specs/040-multi-turn-continuity/quickstart.md`
- [X] T112 Run full daemon tests and record command results in `specs/040-multi-turn-continuity/quickstart.md`
- [X] T113 Run `go mod tidy` from `daemon/` and ensure module files remain consistent in `daemon/go.mod`
- [X] T114 Run client build and record command results in `specs/040-multi-turn-continuity/quickstart.md`
- [X] T115 Perform final redaction and secret-output audit for continuity schemas, fixtures, logs, Web, TUI, and docs in `specs/040-multi-turn-continuity/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies.
- **Foundational (Phase 2)**: Depends on Setup and blocks all user stories.
- **User Story 1 (Phase 3)**: Depends on Foundational. This is the MVP continuity behavior.
- **User Story 2 (Phase 4)**: Depends on Foundational and Roadmap 54 reset behavior. Can run after US1 tests exist, but final validation needs US1 assembly.
- **User Story 3 (Phase 5)**: Depends on Foundational. Can build preview inspection in parallel with US1/US2, but final evidence checks need generated previews.
- **User Story 4 (Phase 6)**: Depends on Foundational and the US1 chat continuity contract.
- **User Story 5 (Phase 7)**: Depends on Foundational and should be validated against all implemented stories.
- **Polish (Phase 8)**: Depends on all desired user stories.

### User Story Dependencies

- **US1 Continue Recent Thread Context**: MVP. No dependency on other stories after Foundational.
- **US2 Reset Continuity Explicitly**: Depends on the shared continuity lookup model and Roadmap 54 reset boundaries.
- **US3 Inspect Included Continuity Evidence**: Depends on preview persistence foundations; can be developed in parallel with US1 service assembly.
- **US4 Preserve Continuity Across Supported Surfaces**: Depends on US1 chat continuity metadata and shared preview contract.
- **US5 Keep Continuity Separate From Knowledge**: Cross-checks US1 through US4 and hardens scope boundaries.

### Within Each User Story

- Tests come before implementation.
- Domain and store changes come before API or client changes.
- API/schema changes come before SDK/Web/TUI consumption.
- Story is complete only when its independent test passes without relying on later stories.

## Parallel Opportunities

- T002 through T008 can run in parallel after T001.
- T010 through T013 and T017 through T024 can run in parallel once T009 is scoped.
- Tests within each user story marked [P] can run in parallel.
- US2 and US3 can start after Foundational while US1 implementation is underway, if they coordinate on `daemon/internal/threads/continuity.go` and `daemon/internal/store/thread_continuity.go`.
- Web, TUI, SDK, schema, and docs tasks can run in parallel after the API contract for each story is stable.

## Parallel Example: User Story 1

```text
Task: "T026 [P] [US1] Add chat service tests for empty, within-limit, over-limit, age-limited, and current-segment continuity assembly in daemon/internal/chat/service_test.go"
Task: "T027 [P] [US1] Add domain tests for daemon acceptance sequence ordering and 12-turn window selection in daemon/internal/threads/continuity_test.go"
Task: "T028 [P] [US1] Add store tests for turn insertion, assistant response linkage, and bounded recent-turn lookup in daemon/internal/store/thread_continuity_test.go"
Task: "T029 [P] [US1] Add API tests for optional threadId, single-turn compatibility, and continuity response fields in daemon/internal/api/thread_continuity_test.go"
Task: "T030 [P] [US1] Add contract tests for chat request/response additive continuity fields in daemon/internal/contracts/thread_continuity_contracts_test.go"
```

## Parallel Example: User Story 3

```text
Task: "T056 [P] [US3] Add preview detail API tests for included and excluded item evidence in daemon/internal/api/thread_continuity_test.go"
Task: "T057 [P] [US3] Add store tests for preview, preview item, artifact excerpt, retention, and redaction persistence in daemon/internal/store/thread_continuity_test.go"
Task: "T058 [P] [US3] Add contract tests for preview schemas and continuity event schemas in daemon/internal/contracts/thread_continuity_contracts_test.go"
Task: "T060 [P] [US3] Add SDK tests for getThreadContinuityPreview and permission denial behavior in sdk/ts/src/thread-continuity.test.ts"
Task: "T061 [P] [US3] Add Web preview summary and detail tests, including reset-boundary evidence rendered after US2 reset behavior, in web/src/features/thread-lifecycle.test.tsx"
Task: "T062 [P] [US3] Add TUI preview inspection command tests, including reset-boundary evidence rendered after US2 reset behavior, in tui/src/cli.test.ts"
```

## Implementation Strategy

### MVP First

1. Complete Phase 1 and Phase 2.
2. Complete Phase 3, User Story 1.
3. Stop and validate bounded follow-up behavior, single-turn compatibility, and chat contract coverage.
4. Do not claim Roadmap 55 complete until US2 through US5 and Polish are complete.

### Incremental Delivery

1. Foundation: persistence, domain constants, schemas, events, and contract scaffolding.
2. US1: active-thread bounded continuity.
3. US2: reset-aware exclusion and permission alignment.
4. US3: operator preview inspection and artifact excerpt boundaries.
5. US4: Web, TUI, and channel parity.
6. US5: non-knowledge and retention hardening.
7. Polish: full verification and documentation.

### Parallel Team Strategy

1. One engineer owns `daemon/internal/threads` and `daemon/internal/store` sequencing.
2. One engineer owns API/schema/contracts after foundational domain types stabilize.
3. One engineer owns SDK/Web/TUI client surfaces after API contracts stabilize.
4. One engineer owns connector ingress and replay tests after store APIs stabilize.

## Completion Criteria

- Every user story has passing targeted tests.
- `make daemon-contract-test` passes.
- `cd daemon && go test ./...` passes.
- `cd daemon && go mod tidy` produces no unexpected drift.
- `pnpm test:clients` passes.
- `pnpm build` passes.
- Quickstart verification notes include p95 continuity assembly latency, preview inspection timing, redaction audit result, and any structured live-connector skip.
