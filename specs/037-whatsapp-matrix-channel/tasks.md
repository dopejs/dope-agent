# Tasks: Matrix Channel Connector

**Input**: Design documents from `/specs/037-whatsapp-matrix-channel/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/matrix-channel-connector.md, quickstart.md

**Tests**: Required by the project constitution and quickstart. Write focused tests before implementation for changed daemon, contract, schema, persistence, routing, delivery, diagnostics, and documentation behavior.

**Organization**: Tasks are grouped by user story so each story is independently implementable and testable after the shared foundation is complete.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel with other [P] tasks in the same phase after phase prerequisites are complete
- **[Story]**: User story label for story phases only
- Every task includes exact file paths

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish Matrix planning and provider scaffolding without changing runtime behavior.

- [X] T001 Create Matrix provider package scaffold and package documentation in daemon/internal/connectors/matrix/doc.go
- [X] T002 [P] Create Matrix connector planning fixtures README in daemon/internal/connectors/matrix/testdata/README.md
- [X] T003 [P] Add Matrix channel operator documentation scaffold in docs/channels/matrix-channel-loop.md
- [X] T004 [P] Add Matrix contract fixture scaffold in daemon/internal/contracts/testdata/matrix-channel-connector/README.md
- [X] T005 [P] Add Matrix schema fixture scaffold in schemas/api/matrix-hosted-setup-resource.schema.json
- [X] T006 [P] Add Matrix route-policy schema scaffold in schemas/api/matrix-route-policy-resource.schema.json
- [X] T007 [P] Add Matrix smoke-evidence schema scaffold in schemas/api/matrix-smoke-evidence-resource.schema.json

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Add shared Matrix vocabulary, data shapes, persistence, and fake transport boundaries required by all user stories.

**CRITICAL**: No user story work can begin until this phase is complete.

- [X] T008 Add Matrix connector kind, capability surface names, and durable identity constants in daemon/internal/connectors/conformance.go
- [X] T009 [P] Add Matrix diagnostic condition constants and mapping helpers in daemon/internal/connectors/matrix/diagnostics.go
- [X] T010 [P] Add Matrix setup, homeserver binding, route policy, inbound event, reply, delivery, diagnostic, capability, and smoke types in daemon/internal/connectors/matrix/types.go
- [X] T011 [P] Add Matrix fake transport interfaces and fake implementation in daemon/internal/connectors/matrix/transport.go
- [X] T012 Add Matrix storage records and migrations for setup, homeserver binding, route policy, event evidence, and smoke evidence in daemon/internal/store/matrix.go
- [X] T013 Add Matrix storage migration registration for additive SQLite tables in daemon/internal/store/migrations.go
- [X] T014 [P] Add Matrix event constructor names for setup, route, duplicate, reply, delivery, diagnostic, and smoke evidence in daemon/internal/events/connectors.go
- [X] T015 [P] Add Matrix connector config shape and redaction-safe config projection in daemon/internal/config/config.go
- [X] T016 [P] Add Matrix schema references to connector list/config schema projections in schemas/api/connector-resource.schema.json
- [X] T017 Add foundational tests for Matrix conformance constants and capability profile validation in daemon/internal/connectors/matrix/conformance_test.go
- [X] T018 Add foundational tests for Matrix storage migrations and tenant isolation accessors in daemon/internal/store/matrix_test.go
- [X] T019 Add foundational tests for Matrix schema fixture loading in daemon/internal/contracts/matrix_schema_test.go

**Checkpoint**: Matrix vocabulary, fake transport, storage, config, events, and schemas exist; user story implementation can start.

---

## Phase 3: User Story 1 - Record Matrix As The Fourth Channel (Priority: P1) MVP

**Goal**: Matrix is the only selected phase 52 provider, WhatsApp is explicitly rejected, and provider-risk/conformance evidence blocks unsafe implementation paths.

**Independent Test**: Confirm the provider selection record names Matrix as chosen, WhatsApp as rejected, includes owner/date/risk/hosted viability/unsupported boundaries, and blocks WhatsApp or unsafe Matrix alternatives.

### Tests for User Story 1

- [X] T020 [P] [US1] Add provider selection decision tests for Matrix chosen and WhatsApp rejected in daemon/internal/connectors/matrix/provider_decision_test.go
- [X] T021 [P] [US1] Add capability declaration tests for unsupported WhatsApp, hosted homeserver provisioning, encrypted rooms, media, calls, bridge automation, thinking, and incremental updates in daemon/internal/connectors/matrix/conformance_test.go
- [X] T022 [P] [US1] Add contract test for Matrix provider decision and readiness gates in daemon/internal/contracts/matrix_channel_connector_test.go
- [X] T023 [P] [US1] Add documentation assertion for Matrix phase 52 handoff in docs/channels/channel-connector-conformance.md

### Implementation for User Story 1

- [X] T024 [US1] Implement Matrix provider decision record and validation helpers in daemon/internal/connectors/matrix/provider_decision.go
- [X] T025 [US1] Implement Matrix capability profile generation with supported and unsupported surfaces in daemon/internal/connectors/matrix/runtime.go
- [X] T026 [US1] Update shared channel conformance handoff for Matrix phase 52 in docs/channels/channel-connector-conformance.md
- [X] T027 [US1] Verify Matrix planning contract rows for provider decision and unsupported alternatives remain aligned in specs/037-whatsapp-matrix-channel/contracts/matrix-channel-connector.md
- [X] T028 [US1] Wire Matrix connector kind into connector creation and listing validation in daemon/internal/api/connectors.go

**Checkpoint**: User Story 1 is independently testable with provider decision, capability profile, conformance handoff, and contract tests.

---

## Phase 4: User Story 2 - Connect Matrix For Hosted Messaging (Priority: P2)

**Goal**: A tenant can configure a tenant-provided Matrix bot account on a tenant-selected homeserver and receive redacted readiness or actionable terminal-state diagnostics.

**Independent Test**: Complete Matrix setup with fake tenant-provided bot evidence and verify `ready`; invalid credentials, unsupported homeserver, missing room/direct route policy, cancellation, and revoked access return redacted terminal states.

### Tests for User Story 2

- [X] T029 [P] [US2] Add Matrix hosted setup terminal-state and under-5-minute completion or diagnostic-bound tests in daemon/internal/connectors/matrix/setup_test.go
- [X] T030 [P] [US2] Add Matrix homeserver and bot binding validation tests in daemon/internal/connectors/matrix/readiness_test.go
- [X] T031 [P] [US2] Add Matrix setup redaction tests for bot access tokens and raw provider payloads in daemon/internal/connectors/matrix/redaction_test.go
- [X] T032 [P] [US2] Add Matrix setup API/schema contract tests in daemon/internal/contracts/matrix_setup_contract_test.go
- [X] T033 [P] [US2] Add Matrix setup persistence lifecycle tests in daemon/internal/store/matrix_setup_test.go

### Implementation for User Story 2

- [X] T034 [US2] Implement Matrix setup evaluation, terminal-state transitions, and under-5-minute completion or diagnostic-bound enforcement in daemon/internal/connectors/matrix/setup.go
- [X] T035 [US2] Implement Matrix homeserver and tenant-provided bot readiness validation in daemon/internal/connectors/matrix/readiness.go
- [X] T036 [US2] Implement Matrix redaction helpers for credentials, homeserver, room, user, and event evidence in daemon/internal/connectors/matrix/redaction.go
- [X] T037 [US2] Implement Matrix setup persistence accessors in daemon/internal/store/matrix.go
- [X] T038 [US2] Implement Matrix setup API projection in daemon/internal/api/matrix_setup.go
- [X] T039 [US2] Finalize Matrix hosted setup schema in schemas/api/matrix-hosted-setup-resource.schema.json
- [X] T040 [US2] Publish Matrix setup validated event with redacted evidence in daemon/internal/events/connectors.go
- [X] T041 [US2] Document Matrix setup, repair, unsupported homeserver, and rollback behavior in docs/channels/matrix-channel-loop.md

**Checkpoint**: User Story 2 is independently testable with fake setup, storage, API projection, schema, events, and operator docs.

---

## Phase 5: User Story 3 - Route Messages And Send Replies Through Matrix (Priority: P3)

**Goal**: Accepted Matrix unencrypted direct and room messages create exactly one agent run and final reply, while blocked, unsupported, duplicate, and delivery-failure paths remain explicit and redacted.

**Independent Test**: Use fake Matrix events for direct messages, allowed-room mentions, allowed-room commands, missing invocation gates, encrypted/undecryptable events, wrong homeserver/account, duplicate sync replay/transaction retry, foreground replies, and background delivery.

### Tests for User Story 3

- [X] T042 [P] [US3] Add Matrix route policy tests for direct allowment and room mention/command gates in daemon/internal/connectors/matrix/routes_test.go
- [X] T043 [P] [US3] Add Matrix runtime normalization and IM loop tests in daemon/internal/connectors/matrix/runtime_test.go
- [X] T044 [P] [US3] Add Matrix duplicate suppression tests for homeserver, room/direct conversation, and event ID in daemon/internal/connectors/matrix/dedupe_test.go
- [X] T045 [P] [US3] Add Matrix unsupported encrypted, undecryptable, media, call, voice, reaction, and bridge metadata tests in daemon/internal/connectors/matrix/unsupported_test.go
- [X] T046 [P] [US3] Add Matrix foreground reply and reply-failure tests in daemon/internal/connectors/matrix/reply_test.go
- [X] T047 [P] [US3] Add Matrix connector-backed delivery adapter tests in daemon/internal/delivery/matrix_adapter_test.go
- [X] T048 [P] [US3] Add Matrix route and delivery schema/event contract tests in daemon/internal/contracts/matrix_route_delivery_contract_test.go

### Implementation for User Story 3

- [X] T049 [US3] Implement Matrix route policy normalization and validation in daemon/internal/connectors/matrix/routes.go
- [X] T050 [US3] Implement Matrix inbound event normalization and routing decisions in daemon/internal/connectors/matrix/runtime.go
- [X] T051 [US3] Implement Matrix durable dedupe key and retained sync/transaction evidence in daemon/internal/connectors/matrix/dedupe.go
- [X] T052 [US3] Implement Matrix unsupported event classification for encrypted rooms, undecryptable events, media, calls, voice, reactions, and bridge metadata in daemon/internal/connectors/matrix/unsupported.go
- [X] T053 [US3] Integrate Matrix accepted inbound events with the IM message loop in daemon/internal/im/loop.go
- [X] T054 [US3] Implement Matrix final-only foreground replies in daemon/internal/connectors/matrix/reply.go
- [X] T055 [US3] Implement Matrix connector-backed background delivery adapter in daemon/internal/delivery/matrix_adapter.go
- [X] T056 [US3] Finalize Matrix route policy schema in schemas/api/matrix-route-policy-resource.schema.json
- [X] T057 [US3] Finalize Matrix smoke evidence schema for route and delivery evidence in schemas/api/matrix-smoke-evidence-resource.schema.json
- [X] T058 [US3] Publish Matrix route, duplicate, foreground reply, and delivery events in daemon/internal/events/connectors.go
- [X] T059 [US3] Document Matrix routing, unsupported surfaces, dedupe, replies, and delivery behavior in docs/channels/matrix-channel-loop.md

**Checkpoint**: User Story 3 is independently testable with fake Matrix ingress, dedupe, reply, delivery, unsupported surfaces, schemas, events, and docs.

---

## Phase 6: User Story 4 - Diagnose Provider-Specific Failures (Priority: P4)

**Goal**: Authorized operators can inspect Matrix health, diagnostics, duplicate evidence, setup/route status, reply/delivery outcomes, live smoke or skip evidence, freshness, retention, and redaction status.

**Independent Test**: Simulate Matrix auth, permission, rate-limit, provider, homeserver, federation, network, duplicate, blocked-route, unsupported, reply, delivery, and unauthorized inspection failures and verify redacted diagnostics.

### Tests for User Story 4

- [X] T060 [P] [US4] Add Matrix diagnostic mapping tests in daemon/internal/connectors/matrix/diagnostics_test.go
- [X] T061 [P] [US4] Add Matrix diagnostic freshness and current-truth tests in daemon/internal/connectors/matrix/diagnostics_freshness_test.go
- [X] T062 [P] [US4] Add Matrix smoke pass and structured skip tests in daemon/internal/connectors/matrix/smoke_test.go
- [X] T063 [P] [US4] Add Matrix live-validation smoke tests in daemon/internal/livevalidation/matrix_connector_smoke_test.go
- [X] T064 [P] [US4] Add Matrix API authorization, permission-gated inspection, and within-2-minute support inspection tests in daemon/internal/api/matrix_diagnostics_test.go
- [X] T065 [P] [US4] Add Matrix retention and redaction-failure tests in daemon/internal/store/matrix_retention_test.go

### Implementation for User Story 4

- [X] T066 [US4] Implement Matrix diagnostic condition-to-shared-reason mapping in daemon/internal/connectors/matrix/diagnostics.go
- [X] T067 [US4] Implement Matrix diagnostic freshness, retention, and redaction suppression integration in daemon/internal/connectors/matrix/diagnostics.go
- [X] T068 [US4] Implement Matrix diagnostic and support inspection API projections with within-2-minute latest-state inspection evidence in daemon/internal/api/matrix_diagnostics.go
- [X] T069 [US4] Implement Matrix smoke evidence pass/fail/skip model in daemon/internal/connectors/matrix/smoke.go
- [X] T070 [US4] Implement Matrix live-validation smoke or structured skip path in daemon/internal/livevalidation/matrix_connector_smoke.go
- [X] T071 [US4] Implement Matrix retention accessors and expiry behavior in daemon/internal/store/matrix.go
- [X] T072 [US4] Document Matrix diagnostics, live smoke policy, retention, redaction, and support inspection in docs/runtime/connector-diagnostics.md
- [X] T073 [US4] Document Matrix live smoke and structured skip operation in docs/channels/matrix-channel-loop.md

**Checkpoint**: User Story 4 is independently testable with diagnostics, support inspection, smoke/skip, retention, redaction, and authorization tests.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final contract alignment, docs, compatibility checks, and full verification.

- [X] T074 [P] Update Matrix references in docs/channels/README.md
- [X] T075 [P] Update Matrix setup and rollback notes in docs/runtime/hosted-credential-setup.md
- [X] T076 [P] Update daemon API and event model documentation for Matrix surfaces in docs/runtime/daemon-api-and-event-model.md
- [X] T077 [P] Add Matrix examples or fixture notes to schemas/README.md
- [X] T078 [P] Add SDK Matrix client methods only if public API surfaces changed in sdk/ts/src/client.ts
- [X] T079 [P] Add web Matrix setup or diagnostics projections only if public UI surfaces changed in web/src/features/integration-diagnostics.tsx and web/src/features/integration-diagnostics.test.tsx
- [X] T080 [P] Add TUI Matrix setup or diagnostics projections only if public UI surfaces changed in tui/src/cli.ts and tui/src/cli.test.ts
- [X] T081 Run focused Matrix test suite documented in specs/037-whatsapp-matrix-channel/quickstart.md from daemon with go test ./internal/connectors ./internal/connectors/matrix ./internal/setupwizard ./internal/im ./internal/delivery ./internal/store ./internal/api ./internal/livevalidation
- [X] T082 Run full daemon test suite documented in specs/037-whatsapp-matrix-channel/quickstart.md from daemon with go test ./...
- [X] T083 Run daemon contract tests documented in specs/037-whatsapp-matrix-channel/quickstart.md from repository root with make daemon-contract-test
- [X] T084 Run client tests documented in specs/037-whatsapp-matrix-channel/quickstart.md from repository root with pnpm test:clients if SDK/web/TUI surfaces changed
- [X] T085 Run client build documented in specs/037-whatsapp-matrix-channel/quickstart.md from repository root with pnpm build if SDK/web/TUI surfaces changed
- [X] T086 Run go mod tidy from daemon and confirm daemon/go.mod and daemon/go.sum changes are intentional
- [X] T087 Verify rollback path by disabling Matrix setup, ingress, and delivery eligibility in daemon/internal/connectors/matrix/runtime_test.go
- [X] T088 Verify no Matrix token, raw provider payload, event body, room content, or cross-tenant data appears in logs, events, schemas, fixtures, support output, or smoke evidence using daemon/internal/connectors/matrix/redaction_test.go

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately.
- **Foundational (Phase 2)**: Depends on Phase 1; blocks all user stories.
- **US1 (Phase 3)**: Depends on Phase 2; MVP provider decision and conformance declaration.
- **US2 (Phase 4)**: Depends on Phase 2 and benefits from US1 capability constants; can be implemented after US1 or in parallel with careful coordination.
- **US3 (Phase 5)**: Depends on Phase 2 and the setup/readiness contracts from US2.
- **US4 (Phase 6)**: Depends on Phase 2 and integrates with US2/US3 evidence; can start diagnostic mappings after foundational types exist.
- **Polish (Phase 7)**: Depends on all desired user stories.

### User Story Dependencies

- **User Story 1 (P1)**: Starts after Foundational; no dependency on US2-US4.
- **User Story 2 (P2)**: Starts after Foundational; setup can be validated independently with fake Matrix transport.
- **User Story 3 (P3)**: Starts after Foundational; full readiness path requires US2 setup integration, but routing units can be tested with fake ready setup.
- **User Story 4 (P4)**: Starts after Foundational; diagnostic mapping can be built independently, while smoke/support projections integrate with US2 and US3 evidence.

### Within Each User Story

- Write tests first and verify they fail before implementation when changing behavior.
- Define data/contracts before runtime services.
- Implement provider mechanics before API projections.
- Keep foreground reply truth separate from assistant execution and background delivery truth.
- Complete each story checkpoint before relying on it in later phases.

## Parallel Opportunities

- Phase 1 [P] docs/schema/fixture scaffolding tasks can run in parallel.
- Phase 2 [P] Matrix diagnostics, types, transport, events, config, and schemas can run in parallel after T008 path naming is agreed.
- US1 tests T020-T023 can run in parallel.
- US2 tests T029-T033 can run in parallel before setup implementation.
- US3 tests T042-T048 can run in parallel before routing/reply/delivery implementation.
- US4 tests T060-T065 can run in parallel before diagnostics/smoke implementation.
- Polish docs, SDK/client conditional updates, and schema README tasks T074-T080 can run in parallel.

## Parallel Example: User Story 2

```bash
Task: "Add Matrix hosted setup terminal-state and under-5-minute completion or diagnostic-bound tests in daemon/internal/connectors/matrix/setup_test.go"
Task: "Add Matrix homeserver and bot binding validation tests in daemon/internal/connectors/matrix/readiness_test.go"
Task: "Add Matrix setup redaction tests for bot access tokens and raw provider payloads in daemon/internal/connectors/matrix/redaction_test.go"
Task: "Add Matrix setup API/schema contract tests in daemon/internal/contracts/matrix_setup_contract_test.go"
Task: "Add Matrix setup persistence lifecycle tests in daemon/internal/store/matrix_setup_test.go"
```

## Parallel Example: User Story 3

```bash
Task: "Add Matrix route policy tests for direct allowment and room mention/command gates in daemon/internal/connectors/matrix/routes_test.go"
Task: "Add Matrix runtime normalization and IM loop tests in daemon/internal/connectors/matrix/runtime_test.go"
Task: "Add Matrix duplicate suppression tests for homeserver, room/direct conversation, and event ID in daemon/internal/connectors/matrix/dedupe_test.go"
Task: "Add Matrix unsupported encrypted, undecryptable, media, call, voice, reaction, and bridge metadata tests in daemon/internal/connectors/matrix/unsupported_test.go"
Task: "Add Matrix foreground reply and reply-failure tests in daemon/internal/connectors/matrix/reply_test.go"
Task: "Add Matrix connector-backed delivery adapter tests in daemon/internal/delivery/matrix_adapter_test.go"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1 setup.
2. Complete Phase 2 foundation.
3. Complete Phase 3 User Story 1.
4. Validate provider decision, Matrix capability declaration, WhatsApp rejection, and contract gates.

### Incremental Delivery

1. Complete Setup + Foundational.
2. Add US1 provider decision/conformance declaration and validate independently.
3. Add US2 hosted Matrix setup and validate independently with fake bot/homeserver evidence.
4. Add US3 routing/reply/delivery and validate independently with fake Matrix events.
5. Add US4 diagnostics/smoke/support inspection and validate independently.
6. Complete cross-cutting docs, schemas, contract tests, and full verification.

### Parallel Team Strategy

1. One engineer owns foundational Matrix package/types/storage.
2. One engineer owns setup/readiness once foundational types are available.
3. One engineer owns routing/dedupe/reply/delivery once fake transport and route types exist.
4. One engineer owns diagnostics/smoke/support inspection once diagnostic types exist.
5. Coordinate shared files explicitly: daemon/internal/events/connectors.go, daemon/internal/store/matrix.go, daemon/internal/api/*, schemas/api/*, and docs/channels/matrix-channel-loop.md.

## Notes

- [P] tasks use different files or can be completed without depending on another incomplete task in the same phase.
- Story labels map to user stories in spec.md.
- Do not implement WhatsApp, Kura-hosted Matrix homeserver provisioning, encrypted rooms, E2EE key/session management, voice/calls, media-rich workflows, bridge automation, or memory-based personalization in this phase.
- Default verification uses fake Matrix transport and `~/.kura-test`; live Matrix credentials require explicit operator approval or a structured skip.
