# Tasks: Channel Connector Conformance

**Input**: Design documents from `/specs/033-channel-connector-conformance/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/channel-connector-conformance.md, quickstart.md

**Tests**: Required. This production change touches connector contracts, persistence, API/schema/event surfaces, diagnostics, redaction, and delivery boundaries.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish feature scaffolding and align documentation references before behavior changes.

- [X] T001 Create the channel conformance operator reference from the planning contract in docs/channels/channel-connector-conformance.md
- [X] T002 [P] Add connector conformance schema fixture README in daemon/internal/contracts/testdata/connector_conformance/README.md
- [X] T003 [P] Add connector conformance package skeleton and compile-ready type stubs in daemon/internal/connectors/conformance.go
- [X] T004 [P] Add connector diagnostics package skeleton and compile-ready type stubs in daemon/internal/connectors/diagnostics.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Add shared contract vocabulary, schemas, event families, and persistence shape that all user stories depend on.

**Critical**: No user story work can begin until this phase is complete.

- [X] T005 [P] Add connector capability profile schema in schemas/api/connector-capability-profile.schema.json
- [X] T006 [P] Add connector conformance result schema with redaction status and retention expiry fields in schemas/api/connector-conformance-result.schema.json
- [X] T007 [P] Add connector diagnostic state schema in schemas/api/connector-diagnostic-state.schema.json
- [X] T008 [P] Add connector account binding summary schema in schemas/api/connector-account-binding-summary.schema.json
- [X] T009 [P] Update connector resource schema references for capability, diagnostic, conformance, and account-binding projections in schemas/api/connector-resource.schema.json
- [X] T010 [P] Add connector conformance result event schema in schemas/events/connector-conformance-result-recorded.event.schema.json
- [X] T011 [P] Add connector diagnostic state changed event schema in schemas/events/connector-diagnostic-state-changed.event.schema.json
- [X] T012 [P] Add connector diagnostic redaction failed event schema in schemas/events/connector-diagnostic-redaction-failed.event.schema.json
- [X] T013 [P] Add connector inbound duplicate detected event schema in schemas/events/connector-inbound-duplicate-detected.event.schema.json
- [X] T014 [P] Add connector route blocked or unsupported event schema in schemas/events/connector-route-outcome-recorded.event.schema.json
- [X] T015 [P] Add connector foreground reply failed event schema in schemas/events/connector-foreground-reply-failed.event.schema.json
- [X] T016 [P] Add connector delivery separation evidence event schema in schemas/events/connector-delivery-separation-recorded.event.schema.json
- [X] T017 Add schema contract and redaction fixture tests for new connector API/event schemas, logs, replay fixtures, evaluation artifacts, and support output in daemon/internal/contracts/connector_conformance_contracts_test.go
- [X] T018 Add additive SQLite schema and indexes for connector conformance, diagnostic evidence, retention expiry, and redaction-failure outcomes in daemon/internal/store/store.go
- [X] T019 Add migration fixture coverage for connector conformance and diagnostic evidence in daemon/internal/store/migrationfixture/r48_connector_conformance.go
- [X] T020 Add migration fixture tests for connector conformance and diagnostic evidence in daemon/internal/store/migrationfixture/r48_connector_conformance_test.go

**Checkpoint**: Contract vocabulary, schema validation, event families, and persistence foundations are ready.

---

## Phase 3: User Story 1 - Prove Any Hosted Connector Against One Contract (Priority: P1)

**Goal**: Engineers can run one conformance matrix against fake connectors and Discord to determine hosted readiness.

**Independent Test**: Run the fake connector conformance matrix with pass, fail, limited, unsupported, redaction-failure, and retention-expired cases, then run Discord regression as the only real connector baseline.

### Tests for User Story 1

- [X] T021 [P] [US1] Add fake connector conformance matrix tests for core invariant pass/fail, equivalent durable identity, and unsafe incremental update degradation cases in daemon/internal/connectors/conformance_test.go
- [X] T022 [P] [US1] Add contract tests for connector capability profile, conformance result, and account binding summary resources in daemon/internal/contracts/connector_conformance_contracts_test.go
- [X] T023 [P] [US1] Add conformance result redaction, redaction-failure suppression, and 90-day retention tests in daemon/internal/store/connector_conformance_test.go
- [X] T024 [P] [US1] Add Discord conformance regression tests for core invariants and explicit limited/unsupported provider surfaces in daemon/internal/connectors/discord/runtime_test.go

### Implementation for User Story 1

- [X] T025 [US1] Implement core invariant and provider surface result types in daemon/internal/connectors/conformance.go
- [X] T026 [US1] Implement fake connector conformance matrix runner in daemon/internal/connectors/conformance.go
- [X] T027 [US1] Implement connector capability profile projection and validation, including documented equivalent durable identity rules, in daemon/internal/connectors/conformance.go
- [X] T028 [US1] Persist conformance results, redaction status, redaction-failure outcomes, and retention expiry through store accessors in daemon/internal/store/store.go
- [X] T029 [US1] Project connector capability profiles and account binding summaries through API connector resources in daemon/internal/api/server.go
- [X] T030 [US1] Map Discord runtime capabilities into the shared conformance profile in daemon/internal/connectors/discord/runtime.go
- [X] T031 [US1] Document future connector provider-spec handoff rules in docs/channels/channel-connector-conformance.md

**Checkpoint**: User Story 1 is independently testable with fake connector conformance plus Discord regression.

---

## Phase 4: User Story 2 - Receive Predictable Channel Behavior (Priority: P1)

**Goal**: Users see consistent routing, duplicate handling, blocked-channel behavior, mention normalization, and reply progression across hosted connector providers.

**Independent Test**: Replay direct, group, mention, blocked room/thread, duplicate, and retry scenarios through fake connectors and confirm stable outcomes, normalized user intent, and at most one assistant reply.

### Tests for User Story 2

- [X] T032 [P] [US2] Add store tests for standard and equivalent durable inbound message identity dedupe in daemon/internal/store/store_test.go
- [X] T033 [P] [US2] Add IM loop tests for duplicate, blocked, ignored, unsupported, and failed routing outcomes in daemon/internal/im/loop_test.go
- [X] T034 [P] [US2] Add Discord mention and addressing artifact normalization tests in daemon/internal/connectors/discord/transport_test.go
- [X] T035 [P] [US2] Add connector ingress API tests for tenant-scoped identity and blocked-route outcomes in daemon/internal/api/server_test.go

### Implementation for User Story 2

- [X] T036 [US2] Extend inbound message and message record identity fields with equivalent durable identity rule metadata in daemon/internal/imtypes/messages.go
- [X] T037 [US2] Update connector message persistence and dedupe lookups for tenant, connector account, channel or conversation, provider message ID, and equivalent durable identity rules in daemon/internal/store/store.go
- [X] T038 [US2] Add tenant-safe connector message identity accessors in daemon/internal/store/tenancy/harness.go
- [X] T039 [US2] Implement stable routing decision outcomes in daemon/internal/im/loop.go
- [X] T040 [US2] Update connector ingress request/response types for identity and routing outcomes in daemon/internal/api/types.go
- [X] T041 [US2] Update connector ingress API handling for identity validation, blocked outcomes, and duplicate outcomes in daemon/internal/api/server.go
- [X] T042 [US2] Update connector ingress request schema for standard identity fields and equivalent durable identity rule metadata in schemas/api/connector-ingress-message.request.schema.json
- [X] T043 [US2] Update connector ingress response schema for stable routing outcomes in schemas/api/connector-ingress-message.response.schema.json
- [X] T044 [US2] Update Discord inbound normalization to populate standard identity fields and strip connector-specific mention/addressing artifacts in daemon/internal/connectors/discord/transport.go
- [X] T045 [US2] Publish connector inbound duplicate and blocked/unsupported route outcome events in daemon/internal/events/connector_ingress.go

**Checkpoint**: User Story 2 is independently testable by replaying routing, mention-normalization, and duplicate fixtures without relying on diagnostics or delivery changes.

---

## Phase 5: User Story 3 - Diagnose Connector Readiness And Failures (Priority: P2)

**Goal**: Operators can inspect stable, redacted connector diagnostics with freshness, remediation, retry-safety, and retention semantics.

**Independent Test**: Seed auth missing, permission missing, rate-limited, provider unavailable, network failed, unsupported capability, duplicate inbound, blocked route, reply failed, unknown failure, stale, redaction-failure, and retention-expired cases for one tenant.

### Tests for User Story 3

- [X] T046 [P] [US3] Add connector diagnostic classification tests in daemon/internal/connectors/diagnostics_test.go
- [X] T047 [P] [US3] Add connector diagnostic API and permission-denial tests in daemon/internal/api/server_test.go
- [X] T048 [P] [US3] Add connector diagnostic retention tests in daemon/internal/store/connector_diagnostics_test.go
- [X] T049 [P] [US3] Add connector diagnostic event contract tests in daemon/internal/contracts/connector_conformance_contracts_test.go

### Implementation for User Story 3

- [X] T050 [US3] Implement connector diagnostic reason codes and remediation metadata in daemon/internal/connectors/diagnostics.go
- [X] T051 [US3] Implement connector diagnostic freshness and current-truth helpers in daemon/internal/connectors/diagnostics.go
- [X] T052 [US3] Implement connector diagnostic redaction fail-closed behavior in daemon/internal/connectors/diagnostics.go
- [X] T053 [US3] Persist connector diagnostic state, redaction failures, and retention expiry in daemon/internal/store/store.go
- [X] T054 [US3] Expose connector diagnostic projections and permission gates in daemon/internal/api/server.go
- [X] T055 [US3] Publish connector diagnostic state and redaction-failure events in daemon/internal/events/connector_diagnostics.go
- [X] T056 [US3] Update operator readiness connector projection to include diagnostic freshness and remediation in daemon/internal/api/operator_projection.go
- [X] T057 [US3] Document connector diagnostic reason codes, freshness, redaction, and retention in docs/runtime/connector-diagnostics.md

**Checkpoint**: User Story 3 is independently testable through seeded diagnostics and operator/API inspection without new delivery behavior.

---

## Phase 6: User Story 4 - Preserve Reply And Delivery Boundaries (Priority: P2)

**Goal**: Platform engineers can prove foreground replies and background delivery remain separate truths even when connector transport mechanics are reused.

**Independent Test**: Run foreground reply and connector-backed background delivery scenarios through the same fake transport and confirm separate outcomes, attempts, events, and failure states.

### Tests for User Story 4

- [X] T058 [P] [US4] Add connector-backed delivery separation tests in daemon/internal/delivery/connector_adapter_test.go
- [X] T059 [P] [US4] Add IM foreground reply outcome tests for success, partial, failure states, and foreground-reply-failed events in daemon/internal/im/loop_test.go
- [X] T060 [P] [US4] Add conformance matrix tests for foreground/background separation and unsafe incremental update degradation in daemon/internal/connectors/conformance_test.go
- [X] T061 [P] [US4] Add event contract tests for foreground reply failure and delivery separation evidence in daemon/internal/contracts/connector_conformance_contracts_test.go

### Implementation for User Story 4

- [X] T062 [US4] Add foreground reply outcome projection helpers in daemon/internal/im/loop.go
- [X] T063 [US4] Add connector-backed delivery linkage evidence to send results in daemon/internal/delivery/connector_adapter.go
- [X] T064 [US4] Persist separate foreground reply and background delivery linkage fields in daemon/internal/store/store.go
- [X] T065 [US4] Add event emission for foreground reply failure and delivery-boundary evidence in daemon/internal/events/connector_delivery.go
- [X] T066 [US4] Update delivery outcome schema linkage for connector-backed targets in schemas/api/delivery-outcome-resource.schema.json
- [X] T067 [US4] Document foreground reply versus background delivery rollback and debugging rules in docs/runtime/connector-diagnostics.md

**Checkpoint**: User Story 4 is independently testable by comparing foreground reply records with background delivery records for the same connector transport.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final validation, docs alignment, and optional client surface updates.

- [X] T068 [P] Update connector event schema index and documentation in schemas/events/README.md
- [X] T069 [P] Update API schema index and connector schema documentation in schemas/api/README.md
- [X] T070 [P] Update channel roadmap references for Discord, Telegram, Slack, and future connectors in docs/specs/035-telegram-channel-connector.md
- [X] T071 [P] Update channel roadmap references for Slack and future connectors in docs/specs/036-slack-channel-connector.md
- [X] T072 [P] Update channel roadmap references for WhatsApp or Matrix follow-on work in docs/specs/037-whatsapp-or-matrix-channel-connector.md
- [X] T073 Run targeted daemon tests from quickstart in daemon/
- [X] T074 Run full daemon tests with go test ./... in daemon/
- [X] T075 Run make daemon-contract-test from repository root
- [X] T076 Run pnpm test:clients from repository root if sdk/ts, web, or tui files changed (not required; no sdk/ts, web, or tui files changed)
- [X] T077 Run pnpm build from repository root if sdk/ts, web, or tui files changed (not required; no sdk/ts, web, or tui files changed)
- [X] T078 Run go mod tidy from daemon/ and confirm daemon/go.mod and daemon/go.sum are unchanged unless dependency changes were intentional
- [X] T079 Update implementation notes and residual risk summary in specs/033-channel-connector-conformance/quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies.
- **Foundational (Phase 2)**: Depends on Setup and blocks all user stories.
- **US1 and US2 (Phase 3 and Phase 4)**: Both P1 and can start after Foundational. US1 is the MVP for conformance gating; US2 is required for user-visible routing/dedupe closure.
- **US3 (Phase 5)**: Depends on Foundational and can proceed after diagnostic schemas exist. It can run in parallel with US2 once shared identity fields are stable.
- **US4 (Phase 6)**: Depends on Foundational and benefits from US1 conformance vocabulary. It can run in parallel with US3.
- **Polish (Phase 7)**: Depends on desired user stories being complete.

### User Story Dependencies

- **US1 (P1)**: Starts after Foundational. No dependency on other stories.
- **US2 (P1)**: Starts after Foundational. No dependency on US1 implementation, but uses shared vocabulary from Phase 2.
- **US3 (P2)**: Starts after Foundational. Diagnostic API/event work may depend on schema tasks T007, T011, and T012.
- **US4 (P2)**: Starts after Foundational. Delivery-boundary conformance tasks may depend on US1 conformance runner task T026 and event schemas T015 and T016.

### Within Each User Story

- Write tests first and confirm they fail before implementation.
- Implement shared types before persistence and API projections.
- Update schemas before contract tests are expected to pass.
- Complete story-specific checkpoint before moving to lower-priority work.

---

## Parallel Opportunities

- Setup tasks T002, T003, and T004 can run in parallel.
- Foundational schema and event tasks T005 through T016 can run in parallel before T017 validates them.
- US1 tests T021, T022, T023, and T024 can run in parallel.
- US2 tests T032, T033, T034, and T035 can run in parallel.
- US3 tests T046, T047, T048, and T049 can run in parallel.
- US4 tests T058, T059, T060, and T061 can run in parallel.
- Documentation polish tasks T068 through T072 can run in parallel after implementation stabilizes.

## Parallel Example: User Story 1

```bash
Task: "T021 [P] [US1] Add fake connector conformance matrix tests for core invariant pass/fail, equivalent durable identity, and unsafe incremental update degradation cases in daemon/internal/connectors/conformance_test.go"
Task: "T022 [P] [US1] Add contract tests for connector capability profile, conformance result, and account binding summary resources in daemon/internal/contracts/connector_conformance_contracts_test.go"
Task: "T023 [P] [US1] Add conformance result redaction, redaction-failure suppression, and 90-day retention tests in daemon/internal/store/connector_conformance_test.go"
Task: "T024 [P] [US1] Add Discord conformance regression tests for core invariants and explicit limited/unsupported provider surfaces in daemon/internal/connectors/discord/runtime_test.go"
```

## Parallel Example: User Story 2

```bash
Task: "T032 [P] [US2] Add store tests for standard and equivalent durable inbound message identity dedupe in daemon/internal/store/store_test.go"
Task: "T033 [P] [US2] Add IM loop tests for duplicate, blocked, ignored, unsupported, and failed routing outcomes in daemon/internal/im/loop_test.go"
Task: "T034 [P] [US2] Add Discord mention and addressing artifact normalization tests in daemon/internal/connectors/discord/transport_test.go"
Task: "T035 [P] [US2] Add connector ingress API tests for tenant-scoped identity and blocked-route outcomes in daemon/internal/api/server_test.go"
```

## Parallel Example: User Story 3

```bash
Task: "T046 [P] [US3] Add connector diagnostic classification tests in daemon/internal/connectors/diagnostics_test.go"
Task: "T047 [P] [US3] Add connector diagnostic API and permission-denial tests in daemon/internal/api/server_test.go"
Task: "T048 [P] [US3] Add connector diagnostic retention tests in daemon/internal/store/connector_diagnostics_test.go"
Task: "T049 [P] [US3] Add connector diagnostic event contract tests in daemon/internal/contracts/connector_conformance_contracts_test.go"
```

## Parallel Example: User Story 4

```bash
Task: "T058 [P] [US4] Add connector-backed delivery separation tests in daemon/internal/delivery/connector_adapter_test.go"
Task: "T059 [P] [US4] Add IM foreground reply outcome tests for success, partial, failure states, and foreground-reply-failed events in daemon/internal/im/loop_test.go"
Task: "T060 [P] [US4] Add conformance matrix tests for foreground/background separation and unsafe incremental update degradation in daemon/internal/connectors/conformance_test.go"
Task: "T061 [P] [US4] Add event contract tests for foreground reply failure and delivery separation evidence in daemon/internal/contracts/connector_conformance_contracts_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1 setup.
2. Complete Phase 2 foundational schemas, event families, and persistence vocabulary.
3. Complete Phase 3 User Story 1.
4. Validate fake connector conformance and Discord regression independently.
5. Stop before claiming Phase 48 closure; US2, US3, and US4 are required for the full roadmap definition of done.

### Incremental Delivery

1. Setup + Foundational: establish contract vocabulary, schemas, event families, and persistence compatibility.
2. US1: prove conformance matrix and Discord baseline.
3. US2: close predictable user-facing routing, mention normalization, and durable dedupe.
4. US3: add operator diagnostics, freshness, redaction, and retention.
5. US4: prove foreground/background delivery separation.
6. Polish: update docs, indexes, and run full verification.

### Parallel Team Strategy

1. Complete Setup + Foundational together.
2. After Foundational:
   - Developer A: US1 conformance matrix, retention/redaction evidence, and Discord regression.
   - Developer B: US2 identity, dedupe, mention normalization, routing outcomes, and ingress events.
   - Developer C: US3 diagnostics, retention, and connector-specific diagnostic events.
   - Developer D: US4 delivery separation and connector delivery events.
3. Coordinate on shared files `daemon/internal/store/store.go`, `daemon/internal/api/server.go`, and `daemon/internal/im/loop.go` to avoid write conflicts.

## Notes

- Every task uses the required checkbox, task ID, optional `[P]`, optional story label, and exact file path format.
- Tasks marked `[P]` touch different files or are safe to start before dependent implementation tasks.
- Tests must fail before implementation tasks satisfy them.
- Public schema or event changes require `make daemon-contract-test`.
- Default verification uses `~/.kura-test`; live connector credentials and production tenants are out of scope.
