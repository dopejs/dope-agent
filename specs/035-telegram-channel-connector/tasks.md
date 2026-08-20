# Tasks: Telegram Channel Connector

**Input**: Design documents from `/specs/035-telegram-channel-connector/`
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/telegram-channel-connector.md](./contracts/telegram-channel-connector.md), [quickstart.md](./quickstart.md)

**Tests**: Required. This is production channel behavior touching setup, diagnostics, routing, persistence, contracts, delivery, and operator evidence. Write targeted tests before implementation and confirm they fail for the missing behavior.

**Organization**: Tasks are grouped by user story so each story can be implemented and tested independently after the foundational phase.

## Phase 1: Setup

**Purpose**: Establish Telegram-specific contract, schema, event, package, and documentation scaffolding.

- [x] T001 [P] Add Telegram hosted setup schema contract in schemas/api/telegram-hosted-setup-resource.schema.json
- [x] T002 [P] Add Telegram allowment schema contract in schemas/api/telegram-allowment-resource.schema.json
- [x] T003 [P] Add Telegram smoke evidence schema contract in schemas/api/telegram-smoke-evidence-resource.schema.json
- [x] T004 [P] Add Telegram setup validation event schema in schemas/events/connector-telegram-setup-validated.event.schema.json
- [x] T005 [P] Add Telegram channel connector contract tests in daemon/internal/contracts/telegram_connector_contract_test.go
- [x] T006 [P] Add Telegram operator channel documentation in docs/channels/telegram-channel-loop.md
- [x] T007 [P] Add Telegram connector package documentation in daemon/internal/connectors/telegram/doc.go

---

## Phase 2: Foundational

**Purpose**: Blocking domain, migration, contract, and tenant-safety primitives used by every user story.

**Critical**: No user story implementation should begin until this phase is complete.

- [x] T008 Add Telegram setup/readiness domain types in daemon/internal/connectors/telegram/readiness.go
- [x] T009 Add Telegram allowment domain types in daemon/internal/connectors/telegram/allowment.go
- [x] T010 Add Telegram diagnostic mapping helpers in daemon/internal/connectors/telegram/diagnostics.go
- [x] T011 Add Telegram smoke evidence domain types in daemon/internal/connectors/telegram/smoke.go
- [x] T012 Add Telegram transport interfaces and fake transport boundary in daemon/internal/connectors/telegram/transport.go
- [x] T013 Add tenant-safe Telegram setup persistence accessors in daemon/internal/store/telegram_setup.go
- [x] T014 Add Telegram setup, allowment, smoke, and retained update evidence migrations in daemon/internal/store/migrations.go
- [x] T015 [P] Add Telegram migration fixture in daemon/internal/store/migrationfixture/r50_telegram_channel_connector.go
- [x] T016 [P] Add Telegram setup migration tests in daemon/internal/store/telegram_setup_test.go
- [x] T017 [P] Add Telegram setup tenant boundary helpers in daemon/internal/store/tenancy/telegram_setup.go
- [x] T018 Add Telegram setup API projection types in daemon/internal/api/setupwizard.go
- [x] T019 Add Telegram event construction helpers in daemon/internal/events/connector_telegram.go
- [x] T020 [P] Add foundational Telegram schema fixture coverage in daemon/internal/contracts/telegram_connector_contract_test.go
- [x] T021 [P] Add Telegram connector conformance fixture coverage in daemon/internal/contracts/connector_conformance_contracts_test.go
- [x] T022 [P] Add Telegram setup redaction test helpers in daemon/internal/connectors/telegram/diagnostics_test.go
- [x] T023 Add Telegram connector registration placeholder in daemon/internal/connectors/supervisor.go
- [x] T024 Add Telegram connector resource projection wiring in schemas/api/connector-resource.schema.json

**Checkpoint**: Foundational Telegram setup, allowment, diagnostic, smoke, schema, migration, and package primitives exist.

---

## Phase 3: User Story 1 - Connect Telegram For Hosted Messaging (Priority: P1) - MVP

**Goal**: A hosted tenant user can submit a Telegram bot token, validate bot ownership, receive a standard terminal state, inspect redacted setup evidence, retry/replace/cancel/disable setup, and avoid leaking secrets.

**Independent Test**: Start Telegram setup with fake safe credentials and explicit validated allowment, complete validation, verify `ready`; then test valid credentials without allowment, malformed, invalid, revoked, unavailable, cancelled, retry, replacement, setup timeout, and redaction cases without exposing raw token material.

### Tests for User Story 1

- [x] T025 [P] [US1] Add Telegram setup validation tests for valid bot credentials in daemon/internal/connectors/telegram/readiness_test.go
- [x] T026 [P] [US1] Add Telegram setup terminal-state tests for invalid, revoked, unavailable, cancelled, and action-required states in daemon/internal/connectors/telegram/readiness_test.go
- [x] T027 [P] [US1] Add Telegram readiness gate tests proving valid bot credentials without explicit allowment remain action-required in daemon/internal/connectors/telegram/readiness_test.go
- [x] T028 [P] [US1] Add Telegram setup timing and timeout tests for the 5-minute setup outcome bound in daemon/internal/connectors/telegram/readiness_test.go
- [x] T029 [P] [US1] Add Telegram credential redaction tests for setup evidence in daemon/internal/connectors/telegram/diagnostics_test.go
- [x] T030 [P] [US1] Add Telegram setup persistence tests for retry, replacement, cancellation, and retention in daemon/internal/store/telegram_setup_test.go
- [x] T031 [P] [US1] Add Telegram setup API projection tests in daemon/internal/api/setupwizard_test.go
- [x] T032 [P] [US1] Add Telegram setup event schema tests in daemon/internal/contracts/telegram_connector_contract_test.go

### Implementation for User Story 1

- [x] T033 [US1] Implement Telegram bot credential validation result handling in daemon/internal/connectors/telegram/readiness.go
- [x] T034 [US1] Implement Telegram account binding summaries in daemon/internal/connectors/telegram/readiness.go
- [x] T035 [US1] Implement setup terminal-state mapping for ready, degraded, unavailable, cancelled, and action-required in daemon/internal/connectors/telegram/readiness.go
- [x] T036 [US1] Enforce explicit allowment before Telegram setup can report ready in daemon/internal/connectors/telegram/readiness.go
- [x] T037 [US1] Enforce setup validation timeout and actionable terminal-state diagnostics in daemon/internal/connectors/telegram/readiness.go
- [x] T038 [US1] Persist Telegram hosted setup and account binding evidence in daemon/internal/store/telegram_setup.go
- [x] T039 [US1] Implement retry, replacement, cancellation, disablement, and retention accessors in daemon/internal/store/telegram_setup.go
- [x] T040 [US1] Project Telegram setup state through setup wizard APIs in daemon/internal/api/setupwizard.go
- [x] T041 [US1] Emit redacted Telegram setup validation events in daemon/internal/connectors/telegram/runtime.go
- [x] T042 [US1] Implement Telegram setup event payload construction in daemon/internal/events/connector_telegram.go
- [x] T043 [US1] Update Telegram setup schema fixtures in schemas/api/telegram-hosted-setup-resource.schema.json
- [x] T044 [US1] Document Telegram hosted setup, repair, cancellation, and redaction behavior in docs/channels/telegram-channel-loop.md

**Checkpoint**: User Story 1 is independently functional and testable as the MVP.

---

## Phase 4: User Story 2 - Route Telegram Messages To The Agent (Priority: P2)

**Goal**: Telegram text messages create runs only for explicitly allowed users/chats/groups, group messages require mention or command gating, duplicates are suppressed by chat/message identity, update identity is retained as redacted evidence, and unsupported surfaces fail explicitly.

**Independent Test**: Feed fake Telegram updates for allowed DMs, blocked DMs, disabled groups, blocked groups, allowed groups without mention/command, allowed groups with mention/command, duplicate replay, missing identity, and unsupported attachments/media/voice/payment/mini-app inputs; verify routing decisions and no duplicate runs or replies.

### Tests for User Story 2

- [x] T045 [P] [US2] Add Telegram allowment validation tests in daemon/internal/connectors/telegram/allowment_test.go
- [x] T046 [P] [US2] Add Telegram DM allow/block routing tests in daemon/internal/connectors/telegram/runtime_test.go
- [x] T047 [P] [US2] Add Telegram group disabled, blocked, mention, and command routing tests in daemon/internal/connectors/telegram/runtime_test.go
- [x] T048 [P] [US2] Add Telegram unsupported attachment, media, voice, payment, and mini-app tests in daemon/internal/connectors/telegram/runtime_test.go
- [x] T049 [P] [US2] Add Telegram chat/message dedupe replay tests in daemon/internal/im/loop_test.go
- [x] T050 [P] [US2] Add retained Telegram update evidence tests in daemon/internal/store/telegram_setup_test.go
- [x] T051 [P] [US2] Add Telegram tenant authorization tests for allowment and routing evidence in daemon/internal/api/setupwizard_test.go

### Implementation for User Story 2

- [x] T052 [US2] Implement explicit Telegram user, direct chat, and group allowment validation in daemon/internal/connectors/telegram/allowment.go
- [x] T053 [US2] Persist Telegram allowment records and validation evidence in daemon/internal/store/telegram_setup.go
- [x] T054 [US2] Enforce direct-message sender and chat allowment in daemon/internal/connectors/telegram/runtime.go
- [x] T055 [US2] Enforce group allowment plus bot mention or command gating in daemon/internal/connectors/telegram/runtime.go
- [x] T056 [US2] Normalize Telegram bot mention and command artifacts before assistant handling in daemon/internal/connectors/telegram/transport.go
- [x] T057 [US2] Record accepted, ignored, blocked, duplicate, unsupported, and failed route outcomes in daemon/internal/connectors/telegram/runtime.go
- [x] T058 [US2] Implement text-only filtering and explicit unsupported outcomes in daemon/internal/connectors/telegram/transport.go
- [x] T059 [US2] Apply Telegram chat/message dedupe identity in daemon/internal/connectors/telegram/runtime.go
- [x] T060 [US2] Retain Telegram update identity as redacted delivery evidence in daemon/internal/store/telegram_setup.go
- [x] T061 [US2] Project Telegram allowment and route evidence through API handlers in daemon/internal/api/setupwizard.go
- [x] T062 [US2] Update Telegram allowment schema fixtures in schemas/api/telegram-allowment-resource.schema.json
- [x] T063 [US2] Document Telegram routing, allowment, group gating, dedupe, and unsupported surfaces in docs/channels/telegram-channel-loop.md

**Checkpoint**: User Story 2 routing behavior is independently functional and conformance-aligned.

---

## Phase 5: User Story 3 - Reply And Deliver Through Telegram With Diagnostics (Priority: P3)

**Goal**: Accepted Telegram messages receive final-only foreground replies, Telegram can be selected as a background delivery target, reply and delivery outcomes stay separate from assistant execution, and operators can inspect redacted diagnostics for auth, permission, rate-limit, provider, network, duplicate, blocked route, reply, unsupported, and unknown failures.

**Independent Test**: Simulate accepted messages, reply success/failure, background delivery success/retry/suppression/failure, rate limits, provider outages, network failures, unsupported surfaces, stale diagnostics, retention expiry, redaction suppression, live smoke pass, and structured skip with fake Telegram transport.

### Tests for User Story 3

- [x] T064 [P] [US3] Add Telegram final-only foreground reply tests in daemon/internal/connectors/telegram/runtime_test.go
- [x] T065 [P] [US3] Add Telegram reply failure separation tests in daemon/internal/im/loop_test.go
- [x] T066 [P] [US3] Add Telegram connector-backed delivery adapter tests in daemon/internal/delivery/connector_adapter_test.go
- [x] T067 [P] [US3] Add Telegram delivery separation event tests in daemon/internal/events/connector_delivery_test.go
- [x] T068 [P] [US3] Add Telegram diagnostic mapping tests in daemon/internal/connectors/telegram/diagnostics_test.go
- [x] T069 [P] [US3] Add Telegram support diagnostic retrieval tests for the 2-minute inspection bound in daemon/internal/api/integration_diagnostics_test.go
- [x] T070 [P] [US3] Add Telegram diagnostic freshness and 90-day retention tests in daemon/internal/connectors/diagnostics_test.go
- [x] T071 [P] [US3] Add Telegram redaction suppression tests for unsafe evidence in daemon/internal/connectors/telegram/diagnostics_test.go
- [x] T072 [P] [US3] Add Telegram live smoke structured skip tests in daemon/internal/connectors/telegram/smoke_test.go
- [x] T073 [P] [US3] Add Telegram fake safe-live smoke pass tests in daemon/internal/connectors/telegram/smoke_test.go
- [x] T074 [P] [US3] Add Telegram capability and conformance profile tests in daemon/internal/connectors/telegram/runtime_test.go
- [x] T075 [P] [US3] Add Telegram smoke schema contract tests in daemon/internal/contracts/telegram_connector_contract_test.go

### Implementation for User Story 3

- [x] T076 [US3] Implement final-only Telegram foreground replies in daemon/internal/connectors/telegram/runtime.go
- [x] T077 [US3] Separate assistant execution outcome from Telegram reply outcome in daemon/internal/im/loop.go
- [x] T078 [US3] Emit Telegram reply sent and reply failed evidence in daemon/internal/connectors/telegram/runtime.go
- [x] T079 [US3] Implement Telegram connector-backed delivery adapter behavior in daemon/internal/delivery/connector_adapter.go
- [x] T080 [US3] Preserve Telegram foreground/background delivery separation in daemon/internal/delivery/connector_adapter.go
- [x] T081 [US3] Map Telegram provider failures to shared diagnostic reason codes in daemon/internal/connectors/telegram/diagnostics.go
- [x] T082 [US3] Persist Telegram diagnostic freshness, retention, and redaction evidence in daemon/internal/store/connector_diagnostics.go
- [x] T083 [US3] Project tenant-safe Telegram diagnostics through API handlers in daemon/internal/api/integration_diagnostics.go
- [x] T084 [US3] Emit Telegram diagnostic, route, reply, and delivery events with redacted evidence in daemon/internal/events/connector_telegram.go
- [x] T085 [US3] Declare Telegram capability profile and unsupported surfaces in daemon/internal/connectors/telegram/runtime.go
- [x] T086 [US3] Implement Telegram live smoke pass or structured skip evidence in daemon/internal/connectors/telegram/smoke.go
- [x] T087 [US3] Persist Telegram smoke evidence with redaction and retention in daemon/internal/store/telegram_setup.go
- [x] T088 [US3] Project Telegram smoke evidence through live validation APIs in daemon/internal/api/live_validation.go
- [x] T089 [US3] Update Telegram smoke schema fixtures in schemas/api/telegram-smoke-evidence-resource.schema.json
- [x] T090 [US3] Document Telegram diagnostics, delivery, live smoke, and structured skip behavior in docs/channels/telegram-channel-loop.md

**Checkpoint**: User Story 3 reply, delivery, diagnostic, and readiness evidence is independently functional and release-reviewable.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final integration, compatibility, documentation, and verification across the whole roadmap.

- [x] T091 [P] Update phase 50 planning contract notes with implemented surface names in specs/035-telegram-channel-connector/contracts/telegram-channel-connector.md
- [x] T092 [P] Update channel conformance handoff notes for Telegram in docs/channels/channel-connector-conformance.md
- [x] T093 [P] Update SDK client types if public Telegram setup or diagnostic shapes changed in sdk/ts/src/index.ts
- [x] T094 [P] Update web operator projections if public Telegram setup or repair surfaces changed in web/src/features/integration-diagnostics.tsx
- [x] T095 [P] Update TUI operator projections if public Telegram setup or repair surfaces changed in tui/src/index.ts
- [x] T096 Run focused daemon package tests from quickstart in daemon/
- [x] T097 Run full daemon tests with go test ./... in daemon/
- [x] T098 Run daemon contract verification with make daemon-contract-test in /Users/John/Code/kura-agent
- [x] T099 Run client verification with pnpm test:clients in /Users/John/Code/kura-agent if SDK/web/TUI surfaces changed
- [x] T100 Run client build with pnpm build in /Users/John/Code/kura-agent if SDK/web/TUI surfaces changed
- [x] T101 Run go mod tidy in daemon/
- [x] T102 Record final rollback and residual live-smoke risk notes in specs/035-telegram-channel-connector/quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 Setup**: No dependencies.
- **Phase 2 Foundational**: Depends on Phase 1; blocks all user stories.
- **Phase 3 US1**: Depends on Phase 2. This is the MVP.
- **Phase 4 US2**: Depends on Phase 2 and can run after the shared Telegram allowment, store, runtime, and route evidence primitives exist. Full accepted-message validation uses a ready setup from US1, but blocked/unsupported route fixtures remain independently testable.
- **Phase 5 US3**: Depends on Phase 2 and can develop diagnostics/delivery fixtures independently. Full reply and delivery validation composes accepted-message evidence from US2 and setup readiness from US1.
- **Phase 6 Polish**: Depends on the desired user stories being complete.

### User Story Dependencies

- **US1 Connect Telegram For Hosted Messaging**: No dependency on other stories after Phase 2; establishes the MVP hosted setup and redacted credential path.
- **US2 Route Telegram Messages To The Agent**: No dependency on US3 after Phase 2; uses foundational allowment and runtime primitives, and uses US1 ready setup for end-to-end accepted-message verification.
- **US3 Reply And Deliver Through Telegram With Diagnostics**: No dependency on US2 for diagnostic and delivery fixtures after Phase 2; final foreground reply validation uses accepted-message routing from US2.

### Within Each User Story

- Write tests first and confirm they fail for missing behavior.
- Implement domain logic before persistence projections.
- Implement persistence before API/event projection.
- Update schemas and docs with the behavior they expose.
- Validate each story at its checkpoint before moving to lower-priority work.

## Parallel Opportunities

- Setup tasks T001-T007 can run in parallel because they touch distinct schema, contract, event, package-doc, and documentation files.
- Foundational tasks T015, T016, T017, T020, T021, and T022 can run in parallel after the domain/storage ownership is assigned because they touch different fixture/test/helper files.
- US1 test tasks T025-T032 can run in parallel before implementation.
- US2 test tasks T045-T051 can run in parallel before implementation.
- US3 test tasks T064-T075 can run in parallel before implementation.
- Documentation/client polish tasks T091-T095 can run in parallel after public surface decisions are known.

## Parallel Example: User Story 1

```text
Task: "T025 [P] [US1] Add Telegram setup validation tests for valid bot credentials in daemon/internal/connectors/telegram/readiness_test.go"
Task: "T026 [P] [US1] Add Telegram setup terminal-state tests for invalid, revoked, unavailable, cancelled, and action-required states in daemon/internal/connectors/telegram/readiness_test.go"
Task: "T027 [P] [US1] Add Telegram readiness gate tests proving valid bot credentials without explicit allowment remain action-required in daemon/internal/connectors/telegram/readiness_test.go"
Task: "T028 [P] [US1] Add Telegram setup timing and timeout tests for the 5-minute setup outcome bound in daemon/internal/connectors/telegram/readiness_test.go"
Task: "T029 [P] [US1] Add Telegram credential redaction tests for setup evidence in daemon/internal/connectors/telegram/diagnostics_test.go"
Task: "T030 [P] [US1] Add Telegram setup persistence tests for retry, replacement, cancellation, and retention in daemon/internal/store/telegram_setup_test.go"
Task: "T031 [P] [US1] Add Telegram setup API projection tests in daemon/internal/api/setupwizard_test.go"
Task: "T032 [P] [US1] Add Telegram setup event schema tests in daemon/internal/contracts/telegram_connector_contract_test.go"
```

## Parallel Example: User Story 2

```text
Task: "T045 [P] [US2] Add Telegram allowment validation tests in daemon/internal/connectors/telegram/allowment_test.go"
Task: "T046 [P] [US2] Add Telegram DM allow/block routing tests in daemon/internal/connectors/telegram/runtime_test.go"
Task: "T047 [P] [US2] Add Telegram group disabled, blocked, mention, and command routing tests in daemon/internal/connectors/telegram/runtime_test.go"
Task: "T048 [P] [US2] Add Telegram unsupported attachment, media, voice, payment, and mini-app tests in daemon/internal/connectors/telegram/runtime_test.go"
Task: "T049 [P] [US2] Add Telegram chat/message dedupe replay tests in daemon/internal/im/loop_test.go"
Task: "T050 [P] [US2] Add retained Telegram update evidence tests in daemon/internal/store/telegram_setup_test.go"
Task: "T051 [P] [US2] Add Telegram tenant authorization tests for allowment and routing evidence in daemon/internal/api/setupwizard_test.go"
```

## Parallel Example: User Story 3

```text
Task: "T064 [P] [US3] Add Telegram final-only foreground reply tests in daemon/internal/connectors/telegram/runtime_test.go"
Task: "T065 [P] [US3] Add Telegram reply failure separation tests in daemon/internal/im/loop_test.go"
Task: "T066 [P] [US3] Add Telegram connector-backed delivery adapter tests in daemon/internal/delivery/connector_adapter_test.go"
Task: "T067 [P] [US3] Add Telegram delivery separation event tests in daemon/internal/events/connector_delivery_test.go"
Task: "T068 [P] [US3] Add Telegram diagnostic mapping tests in daemon/internal/connectors/telegram/diagnostics_test.go"
Task: "T069 [P] [US3] Add Telegram support diagnostic retrieval tests for the 2-minute inspection bound in daemon/internal/api/integration_diagnostics_test.go"
Task: "T070 [P] [US3] Add Telegram diagnostic freshness and 90-day retention tests in daemon/internal/connectors/diagnostics_test.go"
Task: "T071 [P] [US3] Add Telegram redaction suppression tests for unsafe evidence in daemon/internal/connectors/telegram/diagnostics_test.go"
Task: "T072 [P] [US3] Add Telegram live smoke structured skip tests in daemon/internal/connectors/telegram/smoke_test.go"
Task: "T073 [P] [US3] Add Telegram fake safe-live smoke pass tests in daemon/internal/connectors/telegram/smoke_test.go"
Task: "T074 [P] [US3] Add Telegram capability and conformance profile tests in daemon/internal/connectors/telegram/runtime_test.go"
Task: "T075 [P] [US3] Add Telegram smoke schema contract tests in daemon/internal/contracts/telegram_connector_contract_test.go"
```

## Implementation Strategy

### MVP First

1. Complete Phase 1 setup.
2. Complete Phase 2 foundational primitives.
3. Complete Phase 3 US1.
4. Stop and validate Telegram setup independently with fake credentials, explicit allowment gating, terminal-state fixtures, retry/replacement/cancellation cases, timeout checks, API projection tests, and redaction checks.

### Incremental Delivery

1. US1 delivers hosted setup and redacted credential readiness.
2. US2 delivers explicit allowment, group gating, route outcomes, unsupported surfaces, and dedupe.
3. US3 delivers final-only replies, background delivery reuse, diagnostics, smoke evidence, and release-review evidence.
4. Phase 6 closes docs, contracts, verification, rollback, and residual risk notes.

### Parallel Team Strategy

After Phase 2, split work by story ownership:

- Engineer A: US1 setup/readiness.
- Engineer B: US2 routing/allowment/dedupe.
- Engineer C: US3 replies/delivery/diagnostics/smoke.

Coordinate schema, event, and shared file changes before merging because `runtime.go`, `transport.go`, `telegram_setup.go`, `connector_adapter.go`, and schema fixtures are likely integration points.

## Notes

- `[P]` tasks touch distinct files or are test tasks that can be authored independently.
- `[US1]`, `[US2]`, and `[US3]` labels map directly to the user stories in [spec.md](./spec.md).
- Do not mark Telegram `ready` until bot credential, account binding, conformance gates, and explicit allowment validate.
- Do not accept Telegram messages from senders/chats/groups without explicit tenant allowment.
- Do not support attachments, voice, payments, mini apps, media transfer, thinking visibility, or incremental visible updates in phase 50.
- Do not run live Telegram smoke unless credentials are safe: test tenant only, explicitly approved by an operator, non-production, redacted in evidence, and scoped to the validation path.
- Do not log or fixture raw Telegram bot tokens, authorization headers, credential-bearing payloads, raw provider payloads, or cross-tenant data.
- Run `go mod tidy` from `daemon/` before considering implementation complete.
