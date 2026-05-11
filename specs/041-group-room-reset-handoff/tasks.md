# Tasks: Group Room Reset Handoff

**Input**: Design documents from `/specs/041-group-room-reset-handoff/`
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/group-room-reset-handoff.md](./contracts/group-room-reset-handoff.md), [quickstart.md](./quickstart.md)

**Tests**: Required by the project constitution for all production behavior changes. Write the relevant tests first and confirm they fail before implementing each story.

**Organization**: Tasks are grouped by user story so each story can be implemented and tested independently after the shared foundation is complete.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the Roadmap 56 implementation surfaces without changing behavior yet.

- [X] T001 Create group/room domain scaffolding in `daemon/internal/threads/group_room.go`
- [X] T002 [P] Create handoff domain scaffolding in `daemon/internal/threads/handoff.go`
- [X] T003 [P] Create group/room store scaffolding in `daemon/internal/store/thread_group_room.go`
- [X] T004 [P] Create handoff store scaffolding in `daemon/internal/store/thread_handoff.go`
- [X] T005 [P] Create API route scaffolding for handoff in `daemon/internal/api/thread_handoff.go`
- [X] T006 [P] Create event constructor scaffolding in `daemon/internal/events/thread_group_room.go`
- [X] T007 [P] Create Roadmap 56 SDK test placeholder in `sdk/ts/src/group-room-reset-handoff.test.ts`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Add shared persistence, contracts, permissions, and redaction primitives required by every user story.

**CRITICAL**: No user story work can begin until this phase is complete.

- [X] T008 Add additive migration v52 registration for group/room/handoff tables in `daemon/internal/store/store.go`
- [X] T009 Implement conversation shape, room identity, participation decision, reset event, handoff link, and handoff source reference persistence in `daemon/internal/store/thread_group_room.go` and `daemon/internal/store/thread_handoff.go`
- [X] T010 [P] Add tenant-safe group/room/handoff accessors and permission guards in `daemon/internal/store/tenancy/threads.go`
- [X] T011 [P] Define shared group/room/handoff reason codes and redaction statuses in `daemon/internal/threads/group_room.go`
- [X] T012 [P] Define handoff status, source reference status, and first-response eligibility policy in `daemon/internal/threads/handoff.go`
- [X] T013 Add metadata-only redaction helpers for participation and handoff summaries in `daemon/internal/threads/redaction.go`
- [X] T014 [P] Add JSON schemas for conversation shape, participation decision, handoff link, handoff request, and handoff response in `schemas/api/thread-conversation-shape.schema.json`, `schemas/api/thread-participation-decision.schema.json`, `schemas/api/thread-handoff-link.schema.json`, `schemas/api/thread-handoff.request.schema.json`, and `schemas/api/thread-handoff.response.schema.json`
- [X] T015 [P] Add event schemas for participation, scoped reset, and handoff evidence in `schemas/events/thread-participation-decision.event.schema.json`, `schemas/events/thread-reset-scoped.event.schema.json`, and `schemas/events/thread-handoff-linked.event.schema.json`
- [X] T016 Update thread detail schema to include conversation shape, participation decisions, reset events, and handoff links in `schemas/api/thread-detail.response.schema.json`
- [X] T017 [P] Add Roadmap 56 contract validation tests in `daemon/internal/contracts/group_room_reset_handoff_contracts_test.go`
- [X] T018 [P] Add store migration and restart safety tests in `daemon/internal/store/thread_group_room_test.go` and `daemon/internal/store/thread_handoff_restart_test.go`

**Checkpoint**: Foundation ready. User story implementation can proceed in priority order or in parallel where files do not conflict.

---

## Phase 3: User Story 1 - Distinguish Direct, Group, Room, and Web Threads (Priority: P1) Validation Slice

**Goal**: Every accepted conversation carries explicit conversation shape and stable source isolation.

**Independent Test**: Create direct-message, group, room, and web-originated conversations and verify each accepted thread exposes the correct shape, source identity, session segment, and isolation boundary to authorized users.

### Tests for User Story 1

- [X] T019 [P] [US1] Add domain tests for conversation shape classification and room identity isolation in `daemon/internal/threads/group_room_test.go`
- [X] T020 [P] [US1] Add store tests for shape persistence, room identity uniqueness, and unsupported source shape in `daemon/internal/store/thread_group_room_test.go`
- [X] T021 [P] [US1] Add API tests for thread detail conversation shape projection in `daemon/internal/api/thread_group_room_test.go`
- [X] T022 [P] [US1] Add connector ingress tests for direct-message, group, room, and web-originated source mapping in `daemon/internal/im/loop_test.go`

### Implementation for User Story 1

- [X] T023 [US1] Implement conversation shape and room identity domain policy in `daemon/internal/threads/group_room.go`
- [X] T024 [US1] Implement conversation shape and room identity persistence queries in `daemon/internal/store/thread_group_room.go`
- [X] T025 [US1] Integrate conversation shape resolution into connector ingress in `daemon/internal/im/loop.go`
- [X] T026 [US1] Project conversation shape into thread detail responses in `daemon/internal/api/thread_lifecycle.go`
- [X] T027 [US1] Add conversation shape event construction for accepted and unsupported sources in `daemon/internal/events/thread_group_room.go`
- [X] T028 [US1] Update SDK thread detail types for conversation shape in `sdk/ts/src/index.ts`
- [X] T029 [US1] Update Web thread detail rendering for conversation shape in `web/src/features/thread-lifecycle.tsx`
- [X] T030 [US1] Update TUI thread detail output for conversation shape in `tui/src/cli.ts`

**Checkpoint**: User Story 1 is functional and independently testable as the first validation slice. It is not a shippable Roadmap 56 completion boundary.

---

## Phase 4: User Story 2 - Honor Group Participation Policy (Priority: P1)

**Goal**: Group and room routing follows the default allowlist-plus-qualifying-mention policy with operator-visible outcomes.

**Independent Test**: Send group and room messages with allowed mentions, missing mentions, not-allowlisted rooms, unsupported source identities, permission changes, and duplicate source events; verify accepted, ignored, blocked, denied, duplicate, unsupported, or failed outcomes.

### Tests for User Story 2

- [X] T031 [P] [US2] Add participation policy domain tests for allowlist-plus-mention behavior in `daemon/internal/threads/group_room_test.go`
- [X] T032 [P] [US2] Add store tests for participation decision persistence and duplicate source-event suppression in `daemon/internal/store/thread_group_room_test.go`
- [X] T033 [P] [US2] Add connector conformance tests for mention, allowlist, unsupported, duplicate, edited, and deleted message evidence in `daemon/internal/connectors/conformance_test.go`
- [X] T034 [P] [US2] Add API projection tests for participation decisions and safe denial evidence in `daemon/internal/api/thread_group_room_test.go`

### Implementation for User Story 2

- [X] T035 [US2] Implement default participation policy evaluation in `daemon/internal/threads/group_room.go`
- [X] T036 [US2] Implement participation decision persistence and duplicate source-event keys in `daemon/internal/store/thread_group_room.go`
- [X] T037 [US2] Apply group/room participation policy before assistant work creation in `daemon/internal/im/loop.go`
- [X] T038 [US2] Extend connector conformance capability fields for group/room mention, allowlist, and duplicate evidence in `daemon/internal/connectors/conformance.go`
- [X] T039 [US2] Emit participation decision events in `daemon/internal/events/thread_group_room.go`
- [X] T040 [US2] Project participation decisions into thread detail in `daemon/internal/api/thread_lifecycle.go`
- [X] T041 [US2] Update SDK types for participation decisions in `sdk/ts/src/index.ts`
- [X] T042 [US2] Update Web thread detail participation evidence view in `web/src/features/thread-lifecycle.tsx`
- [X] T043 [US2] Update TUI participation evidence output in `tui/src/cli.ts`

**Checkpoint**: User Story 2 is independently testable without handoff implementation.

---

## Phase 5: User Story 3 - Reset Direct, Group, and Room Threads Without Deleting Evidence (Priority: P1)

**Goal**: Authorized users can reset direct-message, group, and room threads by source scope while historical evidence remains inspectable.

**Independent Test**: Reset direct-message, group, and room threads with prior turns, then verify future continuity excludes pre-reset content, unrelated threads are unaffected, and unauthorized reset attempts are safely denied.

### Tests for User Story 3

- [X] T044 [P] [US3] Add reset scope domain tests for direct-message, group, room, and wrong-source reset behavior in `daemon/internal/threads/group_room_test.go`
- [X] T045 [P] [US3] Add store tests for reset event persistence and scoped reset evidence in `daemon/internal/store/thread_group_room_test.go`
- [X] T046 [P] [US3] Add API tests for `connectors.manage` reset denial and scoped reset thread detail evidence in `daemon/internal/api/thread_group_room_test.go`
- [X] T047 [P] [US3] Add continuity regression tests proving pre-reset turns cannot become handoff source references in `daemon/internal/chat/handoff_context_test.go`

### Implementation for User Story 3

- [X] T048 [US3] Extend Roadmap 54 reset flow with conversation shape and source scope evidence in `daemon/internal/threads/lifecycle.go`
- [X] T049 [US3] Persist scoped reset events and permission outcomes in `daemon/internal/store/thread_group_room.go`
- [X] T050 [US3] Enforce `connectors.manage` and safe denial behavior for scoped reset projections in `daemon/internal/api/thread_lifecycle.go`
- [X] T051 [US3] Exclude pre-reset scoped turns from handoff source eligibility in `daemon/internal/threads/handoff.go`
- [X] T052 [US3] Emit scoped reset events in `daemon/internal/events/thread_group_room.go`
- [X] T053 [US3] Update SDK reset event types in `sdk/ts/src/index.ts`
- [X] T054 [US3] Update Web reset event display in `web/src/features/thread-lifecycle.tsx`
- [X] T055 [US3] Update TUI reset event output in `tui/src/cli.ts`

**Checkpoint**: User Story 3 is independently testable with Roadmap 54 reset and Roadmap 55 continuity.

---

## Phase 6: User Story 4 - Hand Off Conversations Across Surfaces With Traceability (Priority: P1)

**Goal**: Authorized users can hand off channel conversations to the web shell or web-originated threads to supported channels using separate destination threads and traceable source/destination links.

**Independent Test**: Handoff channel-to-web and web-to-channel conversations, verify separate destination thread identity, source/destination references, permission enforcement, first-response source-turn references, and no source-turn copying into destination history.

### Tests for User Story 4

- [X] T056 [P] [US4] Add handoff domain tests for separate destination thread identity and source/destination link validation in `daemon/internal/threads/handoff_test.go`
- [X] T057 [P] [US4] Add store tests for handoff link and handoff source reference persistence in `daemon/internal/store/thread_handoff_test.go`
- [X] T058 [P] [US4] Add API tests for handoff creation success, denied handoff, unsupported handoff, and no silent destination creation in `daemon/internal/api/thread_handoff_test.go`
- [X] T059 [P] [US4] Add chat tests for first-destination-response source references and no later source-reference reuse in `daemon/internal/chat/handoff_context_test.go`
- [X] T060 [P] [US4] Add connector conformance tests for handoff source and destination support declarations in `daemon/internal/connectors/conformance_test.go`

### Implementation for User Story 4

- [X] T061 [US4] Implement handoff domain validation and separate destination thread policy in `daemon/internal/threads/handoff.go`
- [X] T062 [US4] Implement handoff link and source reference persistence in `daemon/internal/store/thread_handoff.go`
- [X] T063 [US4] Implement tenant-safe handoff creation accessors in `daemon/internal/store/tenancy/threads.go`
- [X] T064 [US4] Implement `POST /v1/threads/{threadId}/handoffs` route in `daemon/internal/api/thread_handoff.go`
- [X] T065 [US4] Integrate first-response handoff source reference assembly into chat dispatch in `daemon/internal/chat/service.go`
- [X] T066 [US4] Mark handoff source references consumed after the first destination response in `daemon/internal/store/thread_handoff.go`
- [X] T067 [US4] Extend connector conformance for handoff source and destination support in `daemon/internal/connectors/conformance.go`
- [X] T068 [US4] Emit handoff linked events in `daemon/internal/events/thread_group_room.go`
- [X] T069 [US4] Add SDK handoff creation method and types in `sdk/ts/src/index.ts`
- [X] T070 [US4] Add Web handoff controls and source/destination link display in `web/src/features/thread-lifecycle.tsx`
- [X] T071 [US4] Add TUI handoff command and output formatting in `tui/src/cli.ts`

**Checkpoint**: User Story 4 is independently testable with separate destination threads and one-response source references.

---

## Phase 7: User Story 5 - Inspect Reset and Handoff Events (Priority: P2)

**Goal**: Operators can inspect participation decisions, reset events, handoff links, and reason classifications without relying on logs or unsafe payloads.

**Independent Test**: Produce accepted, ignored, reset, denied-reset, successful-handoff, denied-handoff, and unsupported-source cases; verify authorized inspection shows stable event references, safe summaries, actor information, policy outcome, and reason classifications.

### Tests for User Story 5

- [X] T072 [P] [US5] Add API tests for bounded thread detail projection of participation, reset, and handoff evidence in `daemon/internal/api/thread_group_room_test.go`
- [X] T073 [P] [US5] Add event tests for participation, reset, handoff, denial, and redaction-limited events in `daemon/internal/events/thread_group_room_test.go`
- [X] T074 [P] [US5] Add SDK tests for inspection shapes and permission-denial behavior in `sdk/ts/src/group-room-reset-handoff.test.ts`
- [X] T075 [P] [US5] Add Web tests for inspection display, inaccessible-side suppression, and redaction states in `web/src/features/thread-lifecycle.test.tsx`
- [X] T076 [P] [US5] Add TUI tests for participation, reset, and handoff evidence output in `tui/src/cli.test.ts`

### Implementation for User Story 5

- [X] T077 [US5] Implement combined inspection projection for conversation shape, participation decisions, reset events, and handoff links in `daemon/internal/threads/projection.go`
- [X] T078 [US5] Implement API thread detail projection wiring and inaccessible-side suppression in `daemon/internal/api/thread_lifecycle.go`
- [X] T079 [US5] Implement redaction-limited event construction for unsafe evidence in `daemon/internal/events/thread_group_room.go`
- [X] T080 [US5] Update SDK inspection types and decoding in `sdk/ts/src/index.ts`
- [X] T081 [US5] Implement Web inspection states for denied, unsupported, redaction-failed, and consumed handoff references in `web/src/features/thread-lifecycle.tsx`
- [X] T082 [US5] Implement TUI inspection states for denied, unsupported, redaction-failed, and consumed handoff references in `tui/src/cli.ts`

**Checkpoint**: User Story 5 is independently testable as an operator inspection increment.

---

## Phase 8: User Story 6 - Preserve Non-Memory Scope (Priority: P2)

**Goal**: Group, room, reset, and handoff behavior cannot create group memory, team knowledge behavior, semantic recall, autonomous delegation, long-term personalization, or cross-room recall.

**Independent Test**: Configure related older conversations, unrelated groups, repeated preferences, reset-before-handoff cases, and related content in another room; verify only eligible current-segment source references are used for the first destination response and no memory-like behavior appears.

### Tests for User Story 6

- [X] T083 [P] [US6] Add non-memory scope domain tests for cross-room and old-history exclusion in `daemon/internal/threads/handoff_test.go`
- [X] T084 [P] [US6] Add chat tests proving handoff source references do not invoke memory, semantic retrieval, summaries, or cross-room recall in `daemon/internal/chat/handoff_context_test.go`
- [X] T085 [P] [US6] Add connector tests proving unrelated room content and repeated preferences do not affect routing or handoff context in `daemon/internal/connectors/conformance_test.go`
- [X] T086 [P] [US6] Add Web/TUI copy tests that label handoff as traceable continuation rather than memory in `web/src/features/thread-lifecycle.test.tsx` and `tui/src/cli.test.ts`

### Implementation for User Story 6

- [X] T087 [US6] Enforce no cross-room or old-history source selection in `daemon/internal/threads/handoff.go`
- [X] T088 [US6] Ensure chat handoff context uses only explicit handoff references and not memory or semantic retrieval paths in `daemon/internal/chat/service.go`
- [X] T089 [US6] Add safe non-memory classifications to inspection evidence in `daemon/internal/threads/projection.go`
- [X] T090 [US6] Update Web handoff labels to avoid memory terminology in `web/src/features/thread-lifecycle.tsx`
- [X] T091 [US6] Update TUI handoff labels to avoid memory terminology in `tui/src/cli.ts`

**Checkpoint**: User Story 6 is independently testable as a non-memory safety boundary.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Finish compatibility, documentation, full verification, and release-readiness evidence.

- [X] T092 [P] Update runtime documentation for group/room reset and handoff semantics in `docs/runtime/thread-session-lifecycle.md`
- [X] T093 [P] Update minimal chat client documentation for handoff and first-response source references in `docs/runtime/minimal-chat-clients.md`
- [X] T094 [P] Update channel connector conformance documentation for conversation shape, mention, allowlist, reset, and handoff support in `docs/channels/channel-connector-conformance.md`
- [X] T095 [P] Add or update schema fixtures for Roadmap 56 API and event contracts in `daemon/internal/contracts/testdata/group-room-reset-handoff-thread-detail.json`, `daemon/internal/contracts/testdata/group-room-reset-handoff-response.json`, and `daemon/internal/contracts/testdata/group-room-reset-handoff-events.json`
- [X] T096 Run contract validation and fix any Roadmap 56 schema drift in `daemon/internal/contracts/group_room_reset_handoff_contracts_test.go`
- [X] T097 Run focused daemon package tests and fix Roadmap 56 failures in `daemon/internal/threads/group_room.go`, `daemon/internal/store/thread_group_room.go`, `daemon/internal/api/thread_handoff.go`, `daemon/internal/chat/service.go`, `daemon/internal/events/thread_group_room.go`, `daemon/internal/connectors/conformance.go`, `daemon/internal/im/loop.go`, and `daemon/internal/contracts/group_room_reset_handoff_contracts_test.go`
- [X] T098 Run full daemon tests and address Roadmap 56 regressions in `daemon/internal/threads/group_room.go`, `daemon/internal/threads/handoff.go`, `daemon/internal/store/thread_group_room.go`, `daemon/internal/store/thread_handoff.go`, `daemon/internal/api/thread_lifecycle.go`, and `daemon/internal/api/thread_handoff.go`
- [X] T099 Run `go mod tidy` and keep module files stable in `daemon/go.mod` and `daemon/go.sum`
- [X] T100 Run client tests and build, then fix Roadmap 56 regressions in `sdk/ts/src`, `web/src`, and `tui/src`
- [X] T101 Add and run focused handoff source-reference assembly performance measurement for p95 under 500ms in `daemon/internal/chat/handoff_context_test.go`
- [X] T102 Run and record a Roadmap 56 redaction and secret-output audit covering inspection output, test fixtures, and logs in `specs/041-group-room-reset-handoff/quickstart.md`
- [X] T103 Record quickstart verification notes, skipped live connector validation, T101 latency measurement, T102 redaction audit result, and operator inspection timing in `specs/041-group-room-reset-handoff/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies.
- **Foundational (Phase 2)**: Depends on Setup completion and blocks all user stories.
- **User Stories (Phase 3+)**: Depend on Foundational completion.
- **Polish (Phase 9)**: Depends on all desired user stories being complete.

### User Story Dependencies

- **US1 Distinguish Direct, Group, Room, and Web Threads (P1)**: Start after Foundational. First validation slice and prerequisite for reliable US2-US5 behavior; not a shippable Roadmap 56 completion boundary.
- **US2 Honor Group Participation Policy (P1)**: Start after Foundational; uses US1 shape policy when implemented sequentially.
- **US3 Reset Direct, Group, and Room Threads Without Deleting Evidence (P1)**: Start after Foundational; uses US1 shape scope when implemented sequentially.
- **US4 Hand Off Conversations Across Surfaces With Traceability (P1)**: Start after Foundational; uses US1 shape scope, US3 reset boundaries, and Roadmap 55 continuity.
- **US5 Inspect Reset and Handoff Events (P2)**: Best after US2-US4 produce evidence, but projection tests can be developed in parallel with seeded fixtures.
- **US6 Preserve Non-Memory Scope (P2)**: Best after US4 handoff context exists, but tests can be written in parallel against the contract.

### Within Each User Story

- Tests first; confirm they fail before implementation.
- Domain and store behavior before API and connector integration.
- API/schema/event behavior before SDK/Web/TUI integration.
- Story complete before relying on it in later story phases.

---

## Parallel Opportunities

- Setup tasks T002-T007 can run in parallel after T001 begins because they touch separate files.
- Foundational tasks T010-T018 can run in parallel after migration scope T008-T009 is clear because they touch separate packages or schema files.
- Test tasks within each user story are parallelizable because they target separate packages.
- US2 and US3 can proceed in parallel after US1 shape primitives are available.
- US5 client inspection tasks can proceed in parallel with API projection implementation using schema fixtures.
- Documentation tasks T092-T095 can run in parallel after contracts stabilize.

---

## Parallel Example: User Story 4

```text
Task: "T056 [P] [US4] Add handoff domain tests for separate destination thread identity and source/destination link validation in daemon/internal/threads/handoff_test.go"
Task: "T057 [P] [US4] Add store tests for handoff link and handoff source reference persistence in daemon/internal/store/thread_handoff_test.go"
Task: "T058 [P] [US4] Add API tests for handoff creation success, denied handoff, unsupported handoff, and no silent destination creation in daemon/internal/api/thread_handoff_test.go"
Task: "T059 [P] [US4] Add chat tests for first-destination-response source references and no later source-reference reuse in daemon/internal/chat/handoff_context_test.go"
Task: "T060 [P] [US4] Add connector conformance tests for handoff source and destination support declarations in daemon/internal/connectors/conformance_test.go"
```

---

## Implementation Strategy

### First Validation Slice (User Story 1 Only)

1. Complete Phase 1 setup tasks.
2. Complete Phase 2 foundational persistence, schemas, permissions, redaction, and contract scaffolding.
3. Complete Phase 3 User Story 1.
4. Stop and validate conversation shape and source isolation independently before continuing. Do not treat this checkpoint as shippable Roadmap 56 completion.

### Incremental Delivery

1. US1: Conversation shape and room identity.
2. US2: Group/room allowlist-plus-mention participation policy.
3. US3: Source-scoped reset evidence and reset boundary behavior.
4. US4: Separate-thread handoff with first-response source references.
5. US5: Operator inspection and client surfaces.
6. US6: Non-memory safety boundary.
7. Phase 9: Full verification, docs, contract fixtures, and quickstart evidence.

### Parallel Team Strategy

After Setup and Foundational:

- Developer A: US1 domain/store/API projection.
- Developer B: US2 participation policy and connector conformance.
- Developer C: US3 reset scope and Roadmap 55 boundary regressions.
- Developer D: US4 handoff route, store, and chat source reference bridge.
- Developer E: US5 and US6 client/operator evidence and non-memory verification once fixtures are available.

## Summary

- Total tasks: 103
- Setup tasks: 7
- Foundational tasks: 11
- US1 tasks: 12
- US2 tasks: 13
- US3 tasks: 12
- US4 tasks: 16
- US5 tasks: 11
- US6 tasks: 9
- Polish tasks: 12
- Suggested first validation scope: Phase 1 + Phase 2 + User Story 1
