# Tasks: Daemon-Owned Thread And Session Lifecycle

**Input**: Design documents from `/specs/039-thread-session-lifecycle/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/thread-session-lifecycle.md, quickstart.md

**Tests**: Required by constitution because this feature changes persistence, API,
schema, event, connector ingress, SDK, web, TUI, restart, retention, and redaction
surfaces. Write failing tests before implementation for each affected boundary.

**Organization**: Tasks are grouped by user story so each story can be implemented and
verified independently after the shared foundation is complete.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create feature-owned files and contract placeholders without changing behavior.

- [X] T001 Create thread lifecycle package skeleton in daemon/internal/threads/lifecycle.go, daemon/internal/threads/source.go, daemon/internal/threads/projection.go, and daemon/internal/threads/redaction.go
- [X] T002 [P] Create thread lifecycle API schema placeholders in schemas/api/thread-resource.schema.json, schemas/api/thread-list.response.schema.json, schemas/api/thread-detail.response.schema.json, schemas/api/thread-lifecycle-action.request.schema.json, schemas/api/thread-lifecycle-action.response.schema.json, schemas/api/thread-source-linkage.schema.json, and schemas/api/thread-runtime-projection.schema.json
- [X] T003 [P] Create thread lifecycle event schema placeholders in schemas/events/thread-lifecycle.event.schema.json, schemas/events/thread-source-linked.event.schema.json, and schemas/events/thread-retention-applied.event.schema.json
- [X] T004 [P] Create thread lifecycle API handler skeleton in daemon/internal/api/thread_lifecycle.go
- [X] T005 [P] Create thread lifecycle store skeleton in daemon/internal/store/thread_lifecycle.go and daemon/internal/store/tenancy/threads.go
- [X] T006 [P] Create thread lifecycle UI and client skeletons in web/src/features/thread-lifecycle.tsx, web/src/features/thread-lifecycle.test.tsx, sdk/ts/src/thread-lifecycle.test.ts, and tui/src/cli.test.ts

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Define shared domain, storage, schemas, event contracts, tenant-safe access, and rollback-safe persistence used by every user story.

**Critical**: No user story work should begin until this phase is complete.

### Tests and Contracts

- [X] T007 [P] Add domain lifecycle transition tests in daemon/internal/threads/lifecycle_test.go
- [X] T008 [P] Add source identity and current-thread uniqueness tests in daemon/internal/threads/source_test.go
- [X] T009 [P] Add projection and redaction unit tests in daemon/internal/threads/projection_test.go and daemon/internal/threads/redaction_test.go
- [X] T010 [P] Add SQLite migration, tenant-safe persistence, and tenant retention-policy override tests in daemon/internal/store/thread_lifecycle_test.go and daemon/internal/store/tenancy/threads_test.go
- [X] T011 [P] Add lifecycle event constructor tests in daemon/internal/events/thread_lifecycle_test.go
- [X] T012 [P] Add API/event schema contract tests and existing /v1/sessions compatibility fixture checks in daemon/internal/contracts/thread_lifecycle_contracts_test.go

### Implementation

- [X] T013 Implement Thread, SessionSegment, LifecycleAction, SourceLinkage, RuntimeProjection, LifecycleAuditRecord, and LegacySessionEvidence domain types in daemon/internal/threads/lifecycle.go
- [X] T014 Implement lifecycle transition rules for reset, archive, reopen, audit fail-closed, and active segment constraints in daemon/internal/threads/lifecycle.go
- [X] T015 Implement source continuation key normalization and uniqueness helpers in daemon/internal/threads/source.go
- [X] T016 Implement metadata-only projection builders for thread list/detail in daemon/internal/threads/projection.go
- [X] T017 Implement redaction status, safe summary, and unsafe evidence suppression helpers in daemon/internal/threads/redaction.go
- [X] T018 Implement additive SQLite tables, indexes, tenant retention-policy references, and migration wiring for threads, thread_session_segments, thread_source_links, thread_lifecycle_events, and thread_runtime_projections in daemon/internal/store/thread_lifecycle.go and daemon/internal/store/store.go
- [X] T019 Implement tenant-safe thread store accessors and cross-tenant denial mapping in daemon/internal/store/tenancy/threads.go
- [X] T020 Implement thread lifecycle event constructors for lifecycle, source-linked, runtime-projection, retention, redaction, and audit-failed-closed events in daemon/internal/events/thread_lifecycle.go
- [X] T021 Implement API and event JSON schemas for thread resources, lifecycle actions, source linkage, runtime projections, lifecycle events, source-linked events, and retention events in schemas/api/thread-resource.schema.json, schemas/api/thread-list.response.schema.json, schemas/api/thread-detail.response.schema.json, schemas/api/thread-lifecycle-action.request.schema.json, schemas/api/thread-lifecycle-action.response.schema.json, schemas/api/thread-source-linkage.schema.json, schemas/api/thread-runtime-projection.schema.json, schemas/events/thread-lifecycle.event.schema.json, schemas/events/thread-source-linked.event.schema.json, and schemas/events/thread-retention-applied.event.schema.json

**Checkpoint**: Shared lifecycle domain, persistence, schemas, events, and tenant-safe store behavior are ready for user story implementation.

---

## Phase 3: User Story 1 - Inspect And Find Conversations (Priority: P1) - MVP

**Goal**: Authorized users can list and inspect tenant threads with lifecycle state, source summary, current session, last activity, and available actions.

**Independent Test**: Seed conversations from chat, channel, workflow, schedule, shell, and legacy sources, then verify authorized users can list and inspect only their tenant's thread lifecycle metadata without logs or connector-local state.

### Tests for User Story 1

- [X] T022 [P] [US1] Add API list/detail, pagination, deterministic ordering, empty-state, credentials.inspect denial, and permission-revocation reauthorization tests in daemon/internal/api/thread_lifecycle_test.go
- [X] T023 [P] [US1] Add store projection tests for active/reset/reopened/archived ordering and partial legacy evidence in daemon/internal/store/thread_lifecycle_test.go
- [X] T024 [P] [US1] Add SDK list/get thread tests with tenant headers and denial handling in sdk/ts/src/thread-lifecycle.test.ts
- [X] T025 [P] [US1] Add web thread list/detail render tests for loading, empty, error, denied, stale-permission refresh, pagination, and source summary states in web/src/features/thread-lifecycle.test.tsx
- [X] T026 [P] [US1] Add TUI list/detail command tests for thread inspection output and reauthorization after permission changes in tui/src/cli.test.ts

### Implementation for User Story 1

- [X] T027 [US1] Implement GET /v1/threads and GET /v1/threads/{threadId} handlers with credentials.inspect enforcement in daemon/internal/api/thread_lifecycle.go
- [X] T028 [US1] Register protected /v1/threads routes and by-ID tenant guards in daemon/internal/api/server.go
- [X] T029 [US1] Implement thread list pagination, filters, default ordering, and detail retrieval accessors in daemon/internal/store/thread_lifecycle.go
- [X] T030 [US1] Implement legacy session projection for incomplete historical sessions in daemon/internal/store/thread_lifecycle.go and daemon/internal/store/migrationfixture/seeds.go
- [X] T031 [US1] Add ThreadResource, ThreadListResponse, ThreadDetailResponse, ThreadSourceLinkage, and ThreadRuntimeProjection SDK types and listThreads/getThread methods in sdk/ts/src/index.ts
- [X] T032 [US1] Implement ThreadLifecycleView list/detail UI with empty/loading/error/denied states in web/src/features/thread-lifecycle.tsx
- [X] T033 [US1] Integrate ThreadLifecycleView into the app navigation or main product surface in web/src/app/App.tsx and web/src/app/App.test.tsx
- [X] T034 [US1] Add TUI list/detail thread commands and output formatting in tui/src/cli.ts

**Checkpoint**: User Story 1 is independently functional and testable as the MVP.

---

## Phase 4: User Story 2 - Reset, Archive, And Reopen Threads (Priority: P1)

**Goal**: Authorized users can reset, archive, and reopen threads with durable audit evidence, fail-closed mutation behavior, and correct runtime side-effect boundaries.

**Independent Test**: Reset, archive, and reopen representative threads, verify future work follows the new lifecycle state, prior evidence remains inspectable, active runtime work is not cancelled by archive, and mutations fail closed when audit cannot be recorded.

### Tests for User Story 2

- [X] T035 [P] [US2] Add lifecycle mutation domain tests for reset segment creation, archive side effects, reopen eligibility, concurrency, and audit fail-closed behavior in daemon/internal/threads/lifecycle_test.go
- [X] T036 [P] [US2] Add API reset/archive/reopen permission, audit failure, and response contract tests in daemon/internal/api/thread_lifecycle_test.go
- [X] T037 [P] [US2] Add store mutation serialization and lifecycle action persistence tests in daemon/internal/store/thread_lifecycle_test.go
- [X] T038 [P] [US2] Add SDK resetThread, archiveThread, and reopenThread tests in sdk/ts/src/thread-lifecycle.test.ts
- [X] T039 [P] [US2] Add web lifecycle action control tests for reset/archive/reopen, unavailable actions, and denied mutations in web/src/features/thread-lifecycle.test.tsx
- [X] T040 [P] [US2] Add TUI reset/archive/reopen command tests with permission and side-effect text in tui/src/cli.test.ts

### Implementation for User Story 2

- [X] T041 [US2] Implement reset lifecycle mutation to preserve threadId and create a new active session segment in daemon/internal/threads/lifecycle.go and daemon/internal/store/thread_lifecycle.go
- [X] T042 [US2] Implement archive lifecycle mutation to block future continuation without cancelling accepted runtime work in daemon/internal/threads/lifecycle.go and daemon/internal/store/thread_lifecycle.go
- [X] T043 [US2] Implement reopen lifecycle mutation with tenant/source/connector/session eligibility checks in daemon/internal/threads/lifecycle.go and daemon/internal/store/thread_lifecycle.go
- [X] T044 [US2] Implement required audit-before-commit and failed-closed mutation behavior in daemon/internal/store/thread_lifecycle.go and daemon/internal/events/thread_lifecycle.go
- [X] T045 [US2] Implement POST /v1/threads/{threadId}/reset, /archive, and /reopen handlers with connectors.manage enforcement in daemon/internal/api/thread_lifecycle.go
- [X] T046 [US2] Add ThreadLifecycleActionInput and ThreadLifecycleActionResponse schemas in schemas/api/thread-lifecycle-action.request.schema.json and schemas/api/thread-lifecycle-action.response.schema.json
- [X] T047 [US2] Add resetThread, archiveThread, and reopenThread SDK methods in sdk/ts/src/index.ts
- [X] T048 [US2] Add reset/archive/reopen controls and lifecycle action history rendering in web/src/features/thread-lifecycle.tsx
- [X] T049 [US2] Add reset/archive/reopen TUI commands and confirmation output in tui/src/cli.ts

**Checkpoint**: User Story 2 works independently after selecting any seeded thread and exercising lifecycle mutations.

---

## Phase 5: User Story 3 - Trace Channel Messages To Runtime Evidence (Priority: P1)

**Goal**: Operators can trace inbound channel messages to daemon-owned thread/session truth and to the run, workflow, approval, foreground reply, and background delivery records they caused.

**Independent Test**: Send accepted, ignored, blocked, duplicate, disabled, unsupported, and failed connector messages, then verify authorized operators can reconstruct source-to-runtime linkage without raw provider payloads or misleading assistant work evidence.

### Tests for User Story 3

- [X] T050 [P] [US3] Add connector ingress source-continuation tests for accepted, duplicate, blocked, disabled, unsupported, failed, unknown-source, stale-source, and inaccessible-tenant-binding messages in daemon/internal/im/loop_test.go
- [X] T051 [P] [US3] Add runtime projection tests for sessions, runs, workflows, approvals, foreground replies, background deliveries, and connector messages in daemon/internal/threads/projection_test.go
- [X] T052 [P] [US3] Add API trace/detail tests for source linkage and separate runtime facts in daemon/internal/api/thread_lifecycle_test.go
- [X] T053 [P] [US3] Add delivery projection linkage tests from thread detail in daemon/internal/delivery/thread_lifecycle_test.go
- [X] T054 [P] [US3] Add redaction tests for source identity, provider payloads, and disallowed message bodies in daemon/internal/threads/redaction_test.go
- [X] T055 [P] [US3] Add web and TUI operator trace view tests in web/src/features/thread-lifecycle.test.tsx and tui/src/cli.test.ts

### Implementation for User Story 3

- [X] T056 [US3] Extend connector message evidence to carry threadId and sessionSegmentId in daemon/internal/imtypes/messages.go and daemon/internal/store/thread_lifecycle.go
- [X] T057 [US3] Integrate im.MessageLoop accepted-message routing with current thread source linkage in daemon/internal/im/loop.go
- [X] T058 [US3] Record source linkage evidence for ignored, blocked, duplicate, disabled, unsupported, failed, unknown-source, stale-source, and inaccessible-tenant-binding inbound messages in daemon/internal/im/loop.go and daemon/internal/events/thread_lifecycle.go
- [X] T059 [US3] Build runtime projection summaries from sessions, runs, workflows, approvals, foreground replies, background deliveries, and connector messages in daemon/internal/threads/projection.go
- [X] T060 [US3] Persist runtime projection references and source-linked events in daemon/internal/store/thread_lifecycle.go and daemon/internal/events/thread_lifecycle.go
- [X] T061 [US3] Include sourceLinkages and runtimeProjections in thread detail API responses in daemon/internal/api/thread_lifecycle.go
- [X] T062 [US3] Render operator trace sections for source, session, run or workflow, approval, reply, and delivery facts in web/src/features/thread-lifecycle.tsx
- [X] T063 [US3] Add TUI trace command output for source-to-runtime evidence in tui/src/cli.ts

**Checkpoint**: User Story 3 works independently by tracing connector message evidence to runtime facts.

---

## Phase 6: User Story 4 - Survive Restarts And Connector Handoffs (Priority: P2)

**Goal**: Thread lifecycle state, source linkage, session ownership, and runtime projections survive daemon restart and connector event replay.

**Independent Test**: Create active, reset, archived, and reopened threads, restart the daemon/store/router, replay connector events, and verify the same lifecycle state and source continuation rules are applied after restart.

### Tests for User Story 4

- [X] T064 [P] [US4] Add restart persistence tests for active, reset, archived, and reopened threads in daemon/internal/store/thread_lifecycle_restart_test.go
- [X] T065 [P] [US4] Add router restore plus existing /v1/sessions API, schema, event, and SDK compatibility regression tests in daemon/internal/router/router_test.go, daemon/internal/api/server_test.go, daemon/internal/api/legacy_client_compat_test.go, and sdk/ts/src/index.test.ts
- [X] T066 [P] [US4] Add connector replay after restart regression tests in daemon/internal/connectors/conformance_test.go
- [X] T067 [P] [US4] Add API restart recovery visibility tests in daemon/internal/api/thread_lifecycle_test.go

### Implementation for User Story 4

- [X] T068 [US4] Implement restart-safe load and restore helpers for current thread mappings and session segments in daemon/internal/store/thread_lifecycle.go
- [X] T069 [US4] Integrate thread/session segment restore behavior with existing router restore flow while preserving existing /v1/sessions semantics, schemas, events, and client behavior in daemon/internal/router/router.go and daemon/internal/api/server.go
- [X] T070 [US4] Ensure connector replay and duplicate handling use daemon-owned source linkage after restart in daemon/internal/im/loop.go
- [X] T071 [US4] Emit and persist restart recovery audit evidence for ambiguous or partial lifecycle state in daemon/internal/events/thread_lifecycle.go and daemon/internal/store/thread_lifecycle.go

**Checkpoint**: User Story 4 works independently by surviving daemon restart and connector replay.

---

## Phase 7: User Story 5 - Inspect Conversation History Without Memory Recall (Priority: P3)

**Goal**: Users and operators can inspect lifecycle metadata and runtime evidence while the product clearly avoids memory recall, semantic summaries, context packing, and autonomous pruning behavior.

**Independent Test**: Create historical conversations and lifecycle actions, then verify all API, SDK, web, TUI, docs, and tests expose only lifecycle/evidence metadata and never inject historical evidence as assistant memory or summary input.

### Tests for User Story 5

- [X] T072 [P] [US5] Add non-memory API and projection tests that reject semantic summary, recalled memory, context packing, and autonomous pruning fields in daemon/internal/api/thread_lifecycle_test.go and daemon/internal/threads/projection_test.go
- [X] T073 [P] [US5] Add SDK/web/TUI non-memory labeling tests in sdk/ts/src/thread-lifecycle.test.ts, web/src/features/thread-lifecycle.test.tsx, and tui/src/cli.test.ts
- [X] T074 [P] [US5] Add retention, tenant longer-retention policy override, and redaction expiry tests for lifecycle/source/runtime projection evidence in daemon/internal/store/thread_lifecycle_test.go and daemon/internal/threads/redaction_test.go

### Implementation for User Story 5

- [X] T075 [US5] Remove or prevent memory recall, semantic summary, context packing, and pruning fields from thread schemas and projections in schemas/api/thread-detail.response.schema.json and daemon/internal/threads/projection.go
- [X] T076 [US5] Implement 90-day retention application and authorized tenant longer-retention policy override for lifecycle, source, and runtime projection evidence in daemon/internal/store/thread_lifecycle.go and daemon/internal/events/thread_lifecycle.go
- [X] T077 [US5] Add non-memory labeling and retention/redaction metadata to web thread detail in web/src/features/thread-lifecycle.tsx
- [X] T078 [US5] Add non-memory labeling and retention/redaction metadata to TUI thread output in tui/src/cli.ts

**Checkpoint**: User Story 5 works independently by proving lifecycle inspection remains separate from memory behavior.

---

## Final Phase: Polish & Cross-Cutting Concerns

**Purpose**: Complete roadmap closure, docs, contract validation, operator guidance, and final verification.

- [X] T079 [P] Update runtime operator documentation in docs/runtime/thread-session-lifecycle.md
- [X] T080 [P] Update channel conformance documentation for daemon-owned thread routing in docs/channels/channel-connector-conformance.md
- [X] T081 [P] Update TUI usage documentation for thread lifecycle commands in tui/README.md
- [X] T082 [P] Update SDK exported type documentation or README references in sdk/ts/src/index.ts
- [X] T083 [P] Add or update contract fixtures referenced by daemon/internal/contracts/thread_lifecycle_contracts_test.go
- [X] T084 Run focused daemon tests and record command results plus measured list/detail and trace timing results in specs/039-thread-session-lifecycle/quickstart.md
- [X] T085 Run make daemon-contract-test and record command results in specs/039-thread-session-lifecycle/quickstart.md
- [X] T086 Run pnpm test:clients and pnpm build and record command results in specs/039-thread-session-lifecycle/quickstart.md
- [X] T087 Run cd daemon && go test ./... and record command results in specs/039-thread-session-lifecycle/quickstart.md
- [X] T088 Run cd daemon && go mod tidy and confirm no daemon/go.mod or daemon/go.sum drift in specs/039-thread-session-lifecycle/quickstart.md
- [X] T089 Complete the test-environment walkthrough, permission-change checks, measured timing notes, tenant retention-policy override checks, and skipped-live-validation notes in specs/039-thread-session-lifecycle/quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 Setup**: No dependencies.
- **Phase 2 Foundational**: Depends on Phase 1 and blocks all user stories.
- **Phase 3 US1**: Depends on Phase 2 and is the MVP.
- **Phase 4 US2**: Depends on Phase 2; can run after or alongside US1, but product rollout should validate US1 first.
- **Phase 5 US3**: Depends on Phase 2; can run alongside US1/US2 after shared source-link foundations exist.
- **Phase 6 US4**: Depends on US1, US2, and US3 behavior because restart recovery must preserve those states and links.
- **Phase 7 US5**: Depends on US1 and shared projection/redaction foundations; can run after US3 for full runtime projection coverage.
- **Final Phase**: Depends on all selected user stories.

### User Story Dependencies

- **US1 Inspect And Find Conversations**: No dependency on other user stories after foundation.
- **US2 Reset, Archive, And Reopen Threads**: No dependency on US1 implementation, but uses the same foundational lifecycle domain.
- **US3 Trace Channel Messages To Runtime Evidence**: No dependency on US1 UI, but depends on foundational source linkage and store persistence.
- **US4 Survive Restarts And Connector Handoffs**: Depends on lifecycle mutations and source tracing being implemented.
- **US5 Inspect Conversation History Without Memory Recall**: Depends on projection surfaces existing; reinforces US1/US2/US3 outputs.

### Within Each User Story

- Tests are listed before implementation and must fail before the corresponding implementation task.
- Domain/store work precedes API handlers.
- API and schemas precede SDK methods.
- SDK methods precede web and TUI integration.
- Story checkpoint must pass before claiming that story complete.

---

## Parallel Opportunities

- Setup tasks T002-T006 can run in parallel after T001.
- Foundational tests T007-T012 can run in parallel.
- Foundational implementation T015-T017 and T020-T021 can run in parallel after T013-T014 interfaces are agreed.
- Within US1, tests T022-T026 can run in parallel; implementation T031-T034 can run in parallel after T027-T030.
- Within US2, tests T035-T040 can run in parallel; SDK/web/TUI tasks T047-T049 can run in parallel after T045-T046.
- Within US3, tests T050-T055 can run in parallel; web/TUI tasks T062-T063 can run in parallel after T061.
- Within US4, tests T064-T067 can run in parallel before implementation.
- Within US5, tests T072-T074 can run in parallel; UI/TUI labeling tasks T077-T078 can run in parallel after T075-T076.
- Final docs T079-T083 can run in parallel before final verification tasks T084-T089.

---

## Parallel Examples

### User Story 1

```bash
# Parallel test tasks
T022 daemon/internal/api/thread_lifecycle_test.go
T023 daemon/internal/store/thread_lifecycle_test.go
T024 sdk/ts/src/thread-lifecycle.test.ts
T025 web/src/features/thread-lifecycle.test.tsx
T026 tui/src/cli.test.ts

# Parallel client implementation after API/store are ready
T031 sdk/ts/src/index.ts
T032 web/src/features/thread-lifecycle.tsx
T034 tui/src/cli.ts
```

### User Story 2

```bash
# Parallel test tasks
T035 daemon/internal/threads/lifecycle_test.go
T036 daemon/internal/api/thread_lifecycle_test.go
T037 daemon/internal/store/thread_lifecycle_test.go
T038 sdk/ts/src/thread-lifecycle.test.ts
T039 web/src/features/thread-lifecycle.test.tsx
T040 tui/src/cli.test.ts
```

### User Story 3

```bash
# Parallel test tasks
T050 daemon/internal/im/loop_test.go
T051 daemon/internal/threads/projection_test.go
T052 daemon/internal/api/thread_lifecycle_test.go
T053 daemon/internal/delivery/thread_lifecycle_test.go
T054 daemon/internal/threads/redaction_test.go
T055 web/src/features/thread-lifecycle.test.tsx tui/src/cli.test.ts
```

---

## Implementation Strategy

### MVP First

1. Complete Phase 1 setup.
2. Complete Phase 2 foundational domain, store, schemas, events, and tenant-safe access.
3. Complete Phase 3 US1 inspect/list/detail.
4. Stop and validate US1 independently with focused daemon, SDK, web, and TUI tests.

### Incremental Delivery

1. US1 delivers inspectable daemon-owned thread truth.
2. US2 adds lifecycle mutations with audit and fail-closed behavior.
3. US3 adds connector-message-to-runtime traceability.
4. US4 proves restart safety and connector replay behavior.
5. US5 proves lifecycle metadata remains separate from memory behavior.
6. Final phase closes docs, contracts, quickstart, and full verification.

### Production Closure

Do not describe Roadmap 54 as complete until T084-T089 are recorded in quickstart.md and
any skipped live connector validation is documented with owner, reason, remaining risk,
timestamp, retention expiry, and redaction status.
