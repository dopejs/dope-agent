# Tasks: Channel Management And Repair UX

**Input**: Design documents from `/specs/038-channel-management-repair/`
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/channel-management-repair.md](./contracts/channel-management-repair.md), [quickstart.md](./quickstart.md)

**Tests**: Required. This roadmap changes API, schemas, persistence, connector runtime behavior, delivery eligibility, SDK methods, and web product flows. Write the targeted tests in each story before implementation and keep contract verification in sync with `schemas/`.

**Organization**: Tasks are grouped by user story so each story can be implemented and tested as an independently valuable increment after the shared foundation is complete.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel because it touches different files and has no dependency on another incomplete task in the same phase
- **[Story]**: User story label for story phases only
- Every task includes the exact file path it changes or verifies

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish the feature files and implementation anchors used by all later work.

- [X] T001 Record test-environment seed assumptions and two-connector walkthrough notes in specs/038-channel-management-repair/quickstart.md
- [X] T002 [P] Create the operator documentation stub for this roadmap in docs/channels/channel-management-repair.md
- [X] T003 [P] Add the channel management schema inventory entry in schemas/README.md
- [X] T004 [P] Create the reusable web feature module shell in web/src/features/channel-management.tsx
- [X] T005 [P] Create the daemon channel management orchestration shell in daemon/internal/connectors/management.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared contracts, types, routing, permissions, and persistence that all user stories depend on.

**Critical**: No user story work should begin until this phase is complete.

- [X] T006 Create the channel management connector resource schema in schemas/api/channel-management-connector-resource.schema.json
- [X] T007 [P] Create the channel management connector list response schema in schemas/api/channel-management-connector-list.response.schema.json
- [X] T008 [P] Create the channel management action request and response schemas in schemas/api/channel-management-action.schema.json
- [X] T009 [P] Create the channel management support evidence schema in schemas/api/channel-management-support-evidence.schema.json
- [X] T010 [P] Create the connector management event schema in schemas/events/connector-management.event.schema.json
- [X] T011 Define shared management domain types, capability enums, state ordering, and redaction-safe projections in daemon/internal/connectors/management.go
- [X] T012 Implement additive channel management store accessors and migration hooks in daemon/internal/store/channel_management.go
- [X] T013 Implement tenant permission helpers for `credentials.inspect`, `integrations.diagnostics.read`, `connectors.manage`, and `secrets.manage` in daemon/internal/api/channel_management_auth.go
- [X] T014 Register channel management API route scaffolding under `/v1/channel-management/connectors` in daemon/internal/api/server.go
- [X] T015 Add channel management SDK type exports and method placeholders in sdk/ts/src/index.ts
- [X] T016 Add channel management schema and fixture validator coverage in daemon/internal/contracts/channel_management_contracts_test.go
- [X] T017 Wire the web feature entry point behind the product app shell in web/src/app/App.tsx

**Checkpoint**: Contract, permission, storage, SDK, and web anchors exist; user story work can proceed.

---

## Phase 3: User Story 1 - Inspect Channel Fleet Health (Priority: P1) MVP

**Goal**: Authorized users can list and inspect every tenant connector with status, setup state, diagnostics freshness, routing summary, capabilities, and next action.

**Independent Test**: Seed a tenant with ready, disabled, degraded, action-required, and unavailable connectors, then verify paginated list and detail responses plus web views show redacted tenant-scoped evidence with deterministic ordering and stable authorization denials.

### Tests for User Story 1

- [X] T018 [P] [US1] Add API list, detail, diagnostics, pagination, ordering, and permission-denial tests in daemon/internal/api/channel_management_list_test.go
- [X] T019 [P] [US1] Add store projection pagination and deterministic ordering tests in daemon/internal/store/channel_management_list_test.go
- [X] T020 [P] [US1] Add schema contract tests for list, detail, and diagnostics resources in daemon/internal/contracts/channel_management_list_contracts_test.go
- [X] T021 [P] [US1] Add SDK list, detail, and diagnostics client tests in sdk/ts/src/channel-management-list.test.ts
- [X] T022 [P] [US1] Add web list, detail, diagnostics, empty state, and unsupported capability tests in web/src/features/channel-management-list.test.tsx

### Implementation for User Story 1

- [X] T023 [US1] Implement connector list and detail projection composition in daemon/internal/connectors/management.go
- [X] T024 [US1] Implement paginated tenant-scoped projection reads in daemon/internal/store/channel_management.go
- [X] T025 [US1] Implement `GET /v1/channel-management/connectors`, `GET /v1/channel-management/connectors/{connectorId}`, and `GET /v1/channel-management/connectors/{connectorId}/diagnostics` handlers in daemon/internal/api/channel_management.go
- [X] T026 [US1] Register the list and detail handlers in daemon/internal/api/server.go
- [X] T027 [US1] Implement `listChannelConnectors`, `getChannelConnector`, and `getChannelConnectorDiagnostics` SDK methods in sdk/ts/src/index.ts
- [X] T028 [US1] Implement the paginated fleet list, detail panel, diagnostics panel, empty state, status ordering, and next-action rendering in web/src/features/channel-management.tsx
- [X] T029 [US1] Integrate the channel management screen into the app state and navigation in web/src/app/App.tsx
- [X] T030 [US1] Document fleet inspection behavior, diagnostics permissions, pagination, and deterministic ordering in docs/channels/channel-management-repair.md

**Checkpoint**: User Story 1 is independently usable as the MVP management read surface.

---

## Phase 4: User Story 2 - Disable, Re-enable, And Preserve History (Priority: P1)

**Goal**: Authorized users can disable and re-enable connectors while preserving prior evidence, blocking new inbound work, blocking new background deliveries, serializing mutations, and failing closed when audit evidence cannot be recorded.

**Independent Test**: Disable a ready connector, simulate inbound and background delivery attempts, verify no new agent work or delivery eligibility is allowed, re-enable only after current validation, and confirm prior evidence remains inspectable.

### Tests for User Story 2

- [X] T031 [P] [US2] Add API disable, re-enable, permission, serialization, and audit fail-closed tests in daemon/internal/api/channel_management_enablement_test.go
- [X] T032 [P] [US2] Add enablement persistence and restart recovery tests in daemon/internal/store/channel_management_enablement_test.go
- [X] T033 [P] [US2] Add disabled background delivery eligibility tests in daemon/internal/delivery/channel_management_enablement_test.go
- [X] T034 [P] [US2] Add disabled inbound suppression and conformance regression tests in daemon/internal/connectors/management_enablement_test.go
- [X] T035 [P] [US2] Add SDK disable and re-enable tests in sdk/ts/src/channel-management-enablement.test.ts
- [X] T036 [P] [US2] Add web disable, re-enable, history preservation, and denial-state tests in web/src/features/channel-management-enablement.test.tsx

### Implementation for User Story 2

- [X] T037 [US2] Implement per-connector serialized disable and re-enable orchestration in daemon/internal/connectors/management.go
- [X] T038 [US2] Persist enablement transitions, mutation locks, and fail-closed audit evidence in daemon/internal/store/channel_management.go
- [X] T039 [US2] Enforce disabled inbound suppression before agent-run creation in daemon/internal/connectors/supervisor.go
- [X] T040 [US2] Enforce disabled connector background delivery blocking in daemon/internal/delivery/manager.go
- [X] T041 [US2] Implement `POST /disable` and `POST /re-enable` channel management handlers in daemon/internal/api/channel_management.go
- [X] T042 [US2] Implement `disableChannelConnector` and `reEnableChannelConnector` SDK methods in sdk/ts/src/index.ts
- [X] T043 [US2] Implement disable and re-enable controls, validation feedback, and preserved-history indicators in web/src/features/channel-management.tsx
- [X] T044 [US2] Document disable, re-enable, audit fail-closed, and rollback behavior in docs/channels/channel-management-repair.md

**Checkpoint**: User Story 2 safely controls connector enablement without losing history.

---

## Phase 5: User Story 3 - Repair Or Reconnect A Broken Connector (Priority: P2)

**Goal**: Authorized users can start repair from diagnostic next steps, reconnect authorization, rotate supported credentials, refresh stale diagnostics, and land in a clear terminal state without implicitly re-enabling disabled connectors.

**Independent Test**: Induce setup, permission, authorization, route, rate-limit, network, and unsupported failures, then verify each repair path carries diagnostic context, permission gates, redacted evidence, setup linkage, and terminal repair state.

### Tests for User Story 3

- [X] T045 [P] [US3] Add API repair, reconnect, credential-rotation, stale diagnostic, terminal-state, and audit-evidence tests in daemon/internal/api/channel_management_repair_test.go
- [X] T046 [P] [US3] Add setup-session linkage and cancellation tests for repair actions in daemon/internal/setupwizard/channel_management_repair_test.go
- [X] T047 [P] [US3] Add repair action persistence, retry-state, repair-completion audit, reconnect audit, and credential-rotation audit tests in daemon/internal/store/channel_management_repair_test.go
- [X] T048 [P] [US3] Add SDK repair, reconnect, and credential-rotation tests in sdk/ts/src/channel-management-repair.test.ts
- [X] T049 [P] [US3] Add web repair, reconnect, rotation, unsupported-action, and terminal-state tests in web/src/features/channel-management-repair.test.tsx

### Implementation for User Story 3

- [X] T050 [US3] Implement repair action orchestration and terminal-state mapping in daemon/internal/connectors/management_repair.go
- [X] T051 [US3] Persist repair action attempts, status transitions, diagnostic references, and audit IDs in daemon/internal/store/channel_management.go
- [X] T052 [US3] Link repair, reconnect, and supported credential rotation actions to setup sessions in daemon/internal/setupwizard/channel_management_repair.go
- [X] T053 [US3] Implement `POST /repair-actions` channel management handler with repair, reconnect, credential-rotation, completion, and denial audit evidence in daemon/internal/api/channel_management.go
- [X] T054 [US3] Implement 15-minute diagnostic staleness detection and user-initiated refresh hooks in daemon/internal/connectors/diagnostics.go
- [X] T055 [US3] Implement `startChannelConnectorRepair`, `reconnectChannelConnector`, and `rotateChannelConnectorCredentials` SDK methods in sdk/ts/src/index.ts
- [X] T056 [US3] Implement repair, reconnect, supported rotation, stale diagnostic refresh, and terminal-state UI flows in web/src/features/channel-management.tsx

**Checkpoint**: User Story 3 provides coherent repair flows for broken connectors.

---

## Phase 6: User Story 4 - Manage Routes, Allowlists, And Delivery Visibility (Priority: P2)

**Goal**: Authorized users can inspect and update supported route policy, see blocked route decisions, and inspect foreground replies separately from background deliveries.

**Independent Test**: Change route policy for at least two connector kinds, verify future accepted and blocked routing behavior, and confirm foreground reply outcomes, agent execution status, and background delivery outcomes stay separately inspectable.

### Tests for User Story 4

- [X] T057 [P] [US4] Add API route policy, route-update audit, reply outcome, delivery outcome, and permission tests in daemon/internal/api/channel_management_routes_test.go
- [X] T058 [P] [US4] Add connector route-policy projection, future-only routing-decision, blocked-decision audit, and permission-denial audit tests in daemon/internal/connectors/management_routes_test.go
- [X] T059 [P] [US4] Add foreground reply and background delivery separation tests in daemon/internal/delivery/channel_management_status_test.go
- [X] T060 [P] [US4] Add SDK route policy, reply outcome, and delivery outcome tests in sdk/ts/src/channel-management-routes.test.ts
- [X] T061 [P] [US4] Add web route policy, reply status, delivery status, and unsupported-action tests in web/src/features/channel-management-routes.test.tsx

### Implementation for User Story 4

- [X] T062 [US4] Implement route policy projection, validation, and update orchestration in daemon/internal/connectors/management_routes.go
- [X] T063 [US4] Persist route policy snapshots, routing decisions, reply outcome links, and delivery outcome links in daemon/internal/store/channel_management.go
- [X] T064 [US4] Implement route policy, route-update audit, reply outcome, and delivery outcome API handlers in daemon/internal/api/channel_management.go
- [X] T065 [US4] Wire background delivery eligibility and outcome projection into channel management in daemon/internal/delivery/manager.go
- [X] T066 [US4] Implement `getChannelRoutePolicy`, `updateChannelRoutePolicy`, `listChannelReplyOutcomes`, and `listChannelDeliveryOutcomes` SDK methods in sdk/ts/src/index.ts
- [X] T067 [US4] Implement route policy editing, routing decisions, foreground reply, and background delivery sections in web/src/features/channel-management.tsx

**Checkpoint**: User Story 4 exposes routing and delivery controls without rewriting history.

---

## Phase 7: User Story 5 - Provide Support Evidence For Incidents (Priority: P3)

**Goal**: Authorized support users can reconstruct connector incidents from metadata-only redacted evidence while unauthorized users learn nothing about inaccessible connectors and unsafe details are suppressed.

**Independent Test**: Generate setup, routing, disablement, repair, reply, and delivery incidents, then verify authorized support can inspect metadata-only evidence, retention expiry removes normal inspection after 90 days, and redaction failures emit audit evidence without exposing unsafe details.

### Tests for User Story 5

- [X] T068 [P] [US5] Add API support evidence permission, denial, metadata-only, and redaction-failure tests in daemon/internal/api/channel_management_support_test.go
- [X] T069 [P] [US5] Add support evidence retention and 90-day expiry tests in daemon/internal/store/channel_management_retention_test.go
- [X] T070 [P] [US5] Add contract fixture tests that reject raw payloads, message bodies, tokens, and authorization grants in daemon/internal/contracts/channel_management_support_contracts_test.go
- [X] T071 [P] [US5] Add SDK support evidence and permission-denial tests in sdk/ts/src/channel-management-support.test.ts
- [X] T072 [P] [US5] Add web support evidence, redaction failure, retention, and unauthorized-state tests in web/src/features/channel-management-support.test.tsx

### Implementation for User Story 5

- [X] T073 [US5] Implement metadata-only support evidence bundle assembly in daemon/internal/connectors/management_support.go
- [X] T074 [US5] Implement support evidence persistence, redaction status, and retention expiry accessors in daemon/internal/store/channel_management.go
- [X] T075 [US5] Implement `GET /support-evidence` channel management handler in daemon/internal/api/channel_management.go
- [X] T076 [US5] Emit connector management redaction failure, support evidence generated, and retention applied events in daemon/internal/events/channel_management.go
- [X] T077 [US5] Implement `getChannelConnectorSupportEvidence` SDK method in sdk/ts/src/index.ts
- [X] T078 [US5] Implement support evidence, redaction failure, and retention UI states in web/src/features/channel-management.tsx
- [X] T079 [US5] Document metadata-only incident evidence handling and support access constraints in docs/channels/channel-management-repair.md

**Checkpoint**: User Story 5 gives support safe incident evidence without exposing raw channel content.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Final validation, documentation, formatting, and operational readiness across all stories.

- [X] T080 [P] Add representative redacted channel management contract JSON fixtures in daemon/internal/contracts/testdata/channel-management/list-detail.redacted.json, daemon/internal/contracts/testdata/channel-management/mutation-denied.audit-failed.json, and daemon/internal/contracts/testdata/channel-management/support-evidence.retention-redacted.json
- [X] T081 [P] Link the new channel management guide from docs/channels/README.md
- [X] T082 Format Go changes with `gofmt` for daemon/internal/connectors/management.go, daemon/internal/connectors/management_repair.go, daemon/internal/connectors/management_routes.go, daemon/internal/connectors/management_support.go, daemon/internal/connectors/supervisor.go, daemon/internal/connectors/diagnostics.go, daemon/internal/api/channel_management.go, daemon/internal/api/channel_management_auth.go, daemon/internal/store/channel_management.go, daemon/internal/setupwizard/channel_management_repair.go, daemon/internal/events/channel_management.go, and daemon/internal/delivery/manager.go
- [X] T083 Run focused daemon tests and record results in specs/038-channel-management-repair/quickstart.md
- [X] T084 Run `make daemon-contract-test` and record schema/fixture validation results in specs/038-channel-management-repair/quickstart.md
- [X] T085 Run `pnpm test:clients` and `pnpm build` and record SDK/web results in specs/038-channel-management-repair/quickstart.md
- [X] T086 Run `cd daemon && go test ./...` and record full daemon test results in specs/038-channel-management-repair/quickstart.md
- [X] T087 Run `cd daemon && go mod tidy` and review daemon/go.mod and daemon/go.sum
- [X] T088 Update rollback, observability, and skipped-live-validation notes in docs/channels/channel-management-repair.md
- [X] T089 Execute the two-connector manual test-environment walkthrough with 2-minute fleet inspection timing and 5-minute support reconstruction timing recorded in specs/038-channel-management-repair/quickstart.md
- [X] T090 Perform final cross-artifact consistency review against specs/038-channel-management-repair/spec.md, specs/038-channel-management-repair/plan.md, specs/038-channel-management-repair/contracts/channel-management-repair.md, and specs/038-channel-management-repair/tasks.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately.
- **Foundational (Phase 2)**: Depends on Setup completion; blocks all user stories.
- **User Story 1 (Phase 3)**: Depends on Foundational; recommended MVP.
- **User Story 2 (Phase 4)**: Depends on Foundational; can run after or alongside US1 if shared files are coordinated, but it is also P1 and should be completed before P2 release scope.
- **User Story 3 (Phase 5)**: Depends on Foundational and benefits from US1 detail projection; repair must not implicitly re-enable disabled connectors from US2.
- **User Story 4 (Phase 6)**: Depends on Foundational and benefits from US1 detail projection; route edits integrate with US2 delivery eligibility.
- **User Story 5 (Phase 7)**: Depends on Foundational and aggregates evidence from US1-US4.
- **Polish (Phase 8)**: Depends on the selected story scope being complete.

### User Story Dependencies

- **US1 Inspect Channel Fleet Health (P1)**: Can start after Foundation; no dependency on other stories.
- **US2 Disable, Re-enable, And Preserve History (P1)**: Can start after Foundation; integrates with US1 projection but remains independently testable through API and connector/delivery tests.
- **US3 Repair Or Reconnect A Broken Connector (P2)**: Can start after Foundation; must respect US2 disabled-state precedence.
- **US4 Manage Routes, Allowlists, And Delivery Visibility (P2)**: Can start after Foundation; must respect US2 disabled delivery eligibility and US1 detail projection vocabulary.
- **US5 Provide Support Evidence For Incidents (P3)**: Best after US1-US4 because it aggregates their evidence and retention behavior.

### Within Each User Story

- Write tests first and confirm they fail before implementation.
- Add or update schemas before API handlers that emit those resources.
- Add store/domain behavior before API handlers.
- Add API behavior before SDK methods.
- Add SDK and API behavior before web product flows.
- Finish story-specific docs after behavior and verification are clear.

### Parallel Opportunities

- T002, T003, T004, and T005 can run in parallel after T001 is understood.
- T007, T008, T009, and T010 can run in parallel with schema ownership coordination.
- Tests within each user story can run in parallel because they use story-specific files.
- US1 and US2 can proceed in parallel after Foundation if shared edits to `daemon/internal/connectors/management.go`, `daemon/internal/store/channel_management.go`, `daemon/internal/api/channel_management.go`, `sdk/ts/src/index.ts`, and `web/src/features/channel-management.tsx` are coordinated.
- US3 and US4 can proceed in parallel after Foundation and the US1 projection contract is stable.
- US5 implementation should wait for the evidence-producing stories it aggregates, but its tests and contract fixtures can be drafted earlier.

---

## Parallel Example: User Story 1

```text
Task: "T018 [US1] Add API list, detail, diagnostics, pagination, ordering, and permission-denial tests in daemon/internal/api/channel_management_list_test.go"
Task: "T019 [US1] Add store projection pagination and deterministic ordering tests in daemon/internal/store/channel_management_list_test.go"
Task: "T020 [US1] Add schema contract tests for list, detail, and diagnostics resources in daemon/internal/contracts/channel_management_list_contracts_test.go"
Task: "T021 [US1] Add SDK list, detail, and diagnostics client tests in sdk/ts/src/channel-management-list.test.ts"
Task: "T022 [US1] Add web list, detail, diagnostics, empty state, and unsupported capability tests in web/src/features/channel-management-list.test.tsx"
```

## Parallel Example: User Story 2

```text
Task: "T031 [US2] Add API disable, re-enable, permission, serialization, and audit fail-closed tests in daemon/internal/api/channel_management_enablement_test.go"
Task: "T032 [US2] Add enablement persistence and restart recovery tests in daemon/internal/store/channel_management_enablement_test.go"
Task: "T033 [US2] Add disabled background delivery eligibility tests in daemon/internal/delivery/channel_management_enablement_test.go"
Task: "T034 [US2] Add disabled inbound suppression and conformance regression tests in daemon/internal/connectors/management_enablement_test.go"
Task: "T035 [US2] Add SDK disable and re-enable tests in sdk/ts/src/channel-management-enablement.test.ts"
Task: "T036 [US2] Add web disable, re-enable, history preservation, and denial-state tests in web/src/features/channel-management-enablement.test.tsx"
```

## Parallel Example: User Story 3

```text
Task: "T045 [US3] Add API repair, reconnect, credential-rotation, stale diagnostic, terminal-state, and audit-evidence tests in daemon/internal/api/channel_management_repair_test.go"
Task: "T046 [US3] Add setup-session linkage and cancellation tests for repair actions in daemon/internal/setupwizard/channel_management_repair_test.go"
Task: "T047 [US3] Add repair action persistence, retry-state, repair-completion audit, reconnect audit, and credential-rotation audit tests in daemon/internal/store/channel_management_repair_test.go"
Task: "T048 [US3] Add SDK repair, reconnect, and credential-rotation tests in sdk/ts/src/channel-management-repair.test.ts"
Task: "T049 [US3] Add web repair, reconnect, rotation, unsupported-action, and terminal-state tests in web/src/features/channel-management-repair.test.tsx"
```

## Parallel Example: User Story 4

```text
Task: "T057 [US4] Add API route policy, route-update audit, reply outcome, delivery outcome, and permission tests in daemon/internal/api/channel_management_routes_test.go"
Task: "T058 [US4] Add connector route-policy projection, future-only routing-decision, blocked-decision audit, and permission-denial audit tests in daemon/internal/connectors/management_routes_test.go"
Task: "T059 [US4] Add foreground reply and background delivery separation tests in daemon/internal/delivery/channel_management_status_test.go"
Task: "T060 [US4] Add SDK route policy, reply outcome, and delivery outcome tests in sdk/ts/src/channel-management-routes.test.ts"
Task: "T061 [US4] Add web route policy, reply status, delivery status, and unsupported-action tests in web/src/features/channel-management-routes.test.tsx"
```

## Parallel Example: User Story 5

```text
Task: "T068 [US5] Add API support evidence permission, denial, metadata-only, and redaction-failure tests in daemon/internal/api/channel_management_support_test.go"
Task: "T069 [US5] Add support evidence retention and 90-day expiry tests in daemon/internal/store/channel_management_retention_test.go"
Task: "T070 [US5] Add contract fixture tests that reject raw payloads, message bodies, tokens, and authorization grants in daemon/internal/contracts/channel_management_support_contracts_test.go"
Task: "T071 [US5] Add SDK support evidence and permission-denial tests in sdk/ts/src/channel-management-support.test.ts"
Task: "T072 [US5] Add web support evidence, redaction failure, retention, and unauthorized-state tests in web/src/features/channel-management-support.test.tsx"
```

---

## Implementation Strategy

### MVP First (US1 Only)

1. Complete Phase 1 Setup.
2. Complete Phase 2 Foundation.
3. Complete Phase 3 US1.
4. Validate US1 independently with API, store, contract, SDK, and web tests.
5. Use US1 as the read-only management surface before enabling mutation flows.

### Production P1 Scope

1. Complete Setup and Foundation.
2. Complete US1 fleet inspection.
3. Complete US2 disable/re-enable with disabled inbound and delivery blocking.
4. Run focused daemon, contract, SDK, and web tests before moving to P2 repair and route controls.

### Incremental Delivery

1. Add US3 repair/reconnect/rotation after P1 state and permission semantics are stable.
2. Add US4 route policy and delivery visibility after US1 detail projection and US2 delivery eligibility are stable.
3. Add US5 support evidence after evidence-producing flows are present.
4. Finish Phase 8 verification before describing the roadmap as complete.

### Rollback Discipline

- Keep all API, SDK, and web additions additive.
- Preserve existing connector runtime, setup, diagnostics, ingress, delivery, and conformance behavior if channel management routes are hidden.
- Never delete evidence on rollback; retain already-written audit and support evidence until retention expiry.
