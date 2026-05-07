# Tasks: Discord Production Hardening

**Input**: Design documents from `/specs/034-discord-production-hardening/`
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/discord-production-hardening.md](./contracts/discord-production-hardening.md), [quickstart.md](./quickstart.md)

**Tests**: Required. This is production channel behavior touching setup, diagnostics, routing, persistence, contracts, and operator evidence. Write the targeted tests before implementation and confirm they fail for the missing behavior.

**Organization**: Tasks are grouped by user story so each story can be implemented and tested independently after the foundational phase.

## Phase 1: Setup

**Purpose**: Establish the shared contract and schema scaffolding for Discord hosted hardening.

- [X] T001 [P] Add Discord hosted setup schema contract in schemas/api/discord-hosted-setup-resource.schema.json
- [X] T002 [P] Add Discord destination validation schema contract in schemas/api/discord-destination-validation-resource.schema.json
- [X] T003 [P] Add Discord live smoke evidence schema contract in schemas/api/discord-smoke-evidence-resource.schema.json
- [X] T004 [P] Add Discord setup validation event schema in schemas/events/connector-discord-setup-validated.event.schema.json
- [X] T005 [P] Add Discord hardening schema fixture coverage in daemon/internal/contracts/discord_hardening_contract_test.go
- [X] T006 [P] Update Discord operator behavior notes in docs/channels/discord-channel-loop.md

---

## Phase 2: Foundational

**Purpose**: Blocking model, persistence, compatibility, and contract primitives used by every user story.

**Critical**: No user story implementation should begin until these tasks are complete.

- [X] T007 Add Discord setup/readiness domain types in daemon/internal/connectors/discord/readiness.go
- [X] T008 Add Discord destination validation domain types in daemon/internal/connectors/discord/destinations.go
- [X] T009 Add Discord diagnostic mapping helpers in daemon/internal/connectors/discord/diagnostics.go
- [X] T010 Add Discord live smoke evidence domain types in daemon/internal/connectors/discord/smoke.go
- [X] T011 Add tenant-safe Discord setup persistence accessors in daemon/internal/store/discord_setup.go
- [X] T012 Add Discord setup migration and retention fields in daemon/internal/store/migrations.go
- [X] T013 [P] Add local Discord config compatibility tests in daemon/internal/config/config_test.go
- [X] T014 Preserve local Discord config projection for hosted readiness in daemon/internal/config/config.go
- [X] T015 [P] Add Discord setup migration fixture in daemon/internal/store/migrationfixture/r49_discord_hardening.go
- [X] T016 [P] Add Discord setup tenant boundary helpers in daemon/internal/store/tenancy/integrations.go
- [X] T017 Add Discord hosted setup API projection structs in daemon/internal/api/types.go
- [X] T018 Add Discord hosted setup API handlers in daemon/internal/api/integrations.go
- [X] T019 Add Discord event construction helpers in daemon/internal/events/connector_discord.go
- [X] T020 [P] Add foundational schema contract tests in daemon/internal/contracts/discord_hardening_contract_test.go
- [X] T021 [P] Add foundational store migration tests in daemon/internal/store/discord_setup_test.go

**Checkpoint**: Foundational setup, destination, diagnostic, smoke, schema, compatibility, and persistence primitives exist.

---

## Phase 3: User Story 1 - Connect Discord With Tenant-Owned Credentials (Priority: P1) - MVP

**Goal**: A tenant can submit Discord credentials, configure DM/mention/guild/channel behavior, receive redacted validation results, and only reach hosted-ready when explicit selected destinations validate.

**Independent Test**: Complete setup with fake valid credentials and explicit valid destinations, then attempt invalid credentials, missing destinations, partially invalid destinations, and legacy local config projection; verify redacted setup state, compatibility, and hosted-ready gating.

### Tests for User Story 1

- [X] T022 [P] [US1] Add setup validation tests for valid credentials and explicit destinations in daemon/internal/connectors/discord/readiness_test.go
- [X] T023 [P] [US1] Add setup validation tests for missing explicit destinations and partial destination failures in daemon/internal/connectors/discord/readiness_test.go
- [X] T024 [P] [US1] Add credential redaction tests for setup evidence in daemon/internal/connectors/discord/diagnostics_test.go
- [X] T025 [P] [US1] Add Discord setup persistence tests for degraded/needs-repair state in daemon/internal/store/discord_setup_test.go
- [X] T026 [P] [US1] Add Discord setup API projection tests in daemon/internal/api/setupwizard_test.go

### Implementation for User Story 1

- [X] T027 [US1] Implement Discord credential validation result handling in daemon/internal/connectors/discord/readiness.go
- [X] T028 [US1] Implement explicit guild/channel destination validation in daemon/internal/connectors/discord/destinations.go
- [X] T029 [US1] Implement degraded/needs-repair hosted readiness gating in daemon/internal/connectors/discord/readiness.go
- [X] T030 [US1] Persist Discord hosted setup and destination validation evidence in daemon/internal/store/discord_setup.go
- [X] T031 [US1] Project Discord setup state through setup wizard APIs in daemon/internal/api/setupwizard.go
- [X] T032 [US1] Emit redacted Discord setup validation events in daemon/internal/connectors/discord/runtime.go
- [X] T033 [US1] Update Discord setup schemas and fixtures in schemas/api/discord-hosted-setup-resource.schema.json
- [X] T034 [US1] Update Discord destination schemas and fixtures in schemas/api/discord-destination-validation-resource.schema.json
- [X] T035 [US1] Document hosted setup and degraded repair behavior in docs/channels/discord-channel-loop.md

**Checkpoint**: User Story 1 is independently functional and testable as the MVP.

---

## Phase 4: User Story 2 - Repair Discord Readiness Failures (Priority: P1)

**Goal**: Operators can inspect stable, redacted Discord diagnostics for auth, permission, message content, rate limit, gateway, network, provider, duplicate, route, reply, unsupported, and unknown failures.

**Independent Test**: Replay each diagnostic family with fake transport and store/API fixtures; verify stable reason codes, remediation owner, freshness, retention, redaction, and tenant-safe access denial.

### Tests for User Story 2

- [X] T036 [P] [US2] Add Discord diagnostic mapping tests in daemon/internal/connectors/discord/diagnostics_test.go
- [X] T037 [P] [US2] Add diagnostic freshness and 90-day retention tests in daemon/internal/connectors/diagnostics_test.go
- [X] T038 [P] [US2] Add redaction suppression tests for unsafe Discord evidence in daemon/internal/connectors/discord/diagnostics_test.go
- [X] T039 [P] [US2] Add tenant authorization tests for Discord diagnostics in daemon/internal/api/integration_diagnostics_test.go
- [X] T040 [P] [US2] Add diagnostic persistence and retention tests in daemon/internal/store/connector_diagnostics_test.go

### Implementation for User Story 2

- [X] T041 [US2] Map Discord provider errors to shared diagnostic reason codes in daemon/internal/connectors/discord/diagnostics.go
- [X] T042 [US2] Replace coarse Discord start failure evidence with redacted diagnostic states in daemon/internal/connectors/discord/runtime.go
- [X] T043 [US2] Persist Discord diagnostics with freshness and retention metadata in daemon/internal/store/connector_diagnostics.go
- [X] T044 [US2] Project tenant-safe Discord diagnostics through API handlers in daemon/internal/api/integration_diagnostics.go
- [X] T045 [US2] Emit Discord diagnostic events with redacted evidence in daemon/internal/events/connector_discord.go
- [X] T046 [US2] Update connector diagnostic schemas for Discord reason mappings in schemas/api/connector-diagnostic-state.schema.json
- [X] T047 [US2] Document Discord repair reason codes and operator remediation in docs/channels/discord-channel-loop.md

**Checkpoint**: User Story 2 diagnostics are independently functional and tenant-safe.

---

## Phase 5: User Story 3 - Receive Predictable Discord Replies (Priority: P1)

**Goal**: Discord users receive replies only where tenant configuration allows them, duplicates do not double-reply, mention artifacts are stripped, reply failures remain separate from assistant execution, and foreground replies remain separate from connector-backed background delivery.

**Independent Test**: Send fake DM, guild mention, non-mention, blocked guild/channel, duplicate replay, progression degradation, reply failure, and connector-backed background delivery cases through the Discord runtime, IM loop, and delivery adapter; verify outcomes and evidence.

### Tests for User Story 3

- [X] T048 [P] [US3] Add Discord DM and mention routing tests in daemon/internal/connectors/discord/runtime_test.go
- [X] T049 [P] [US3] Add blocked guild/channel route outcome tests in daemon/internal/connectors/discord/runtime_test.go
- [X] T050 [P] [US3] Add mention normalization tests in daemon/internal/connectors/discord/transport_test.go
- [X] T051 [P] [US3] Add duplicate inbound replay tests in daemon/internal/im/loop_test.go
- [X] T052 [P] [US3] Add reply failure separation tests in daemon/internal/im/loop_test.go
- [X] T053 [P] [US3] Add connector-backed delivery separation tests in daemon/internal/delivery/connector_adapter_test.go
- [X] T054 [P] [US3] Add reply progression degradation tests in daemon/internal/connectors/discord/runtime_test.go

### Implementation for User Story 3

- [X] T055 [US3] Enforce fail-closed tenant/account binding for Discord inbound messages in daemon/internal/connectors/discord/runtime.go
- [X] T056 [US3] Tighten Discord route outcomes for DM, mention, guild, and channel decisions in daemon/internal/connectors/discord/runtime.go
- [X] T057 [US3] Persist blocked, ignored, duplicate, and failed route evidence in daemon/internal/store/discord_setup.go
- [X] T058 [US3] Ensure Discord inbound identity uses tenant, connector account, channel/conversation, and provider message ID in daemon/internal/connectors/discord/transport.go
- [X] T059 [US3] Preserve mention normalization before assistant handling in daemon/internal/connectors/discord/transport.go
- [X] T060 [US3] Separate assistant execution outcome from Discord reply delivery outcome in daemon/internal/im/loop.go
- [X] T061 [US3] Preserve foreground/background connector delivery separation in daemon/internal/delivery/connector_adapter.go
- [X] T062 [US3] Degrade Discord reply progression when send/edit behavior is unsafe in daemon/internal/connectors/discord/runtime.go
- [X] T063 [US3] Emit reply failure and duplicate inbound diagnostics in daemon/internal/connectors/discord/runtime.go
- [X] T064 [US3] Update Discord route and reply event schemas in schemas/events/connector-route-outcome-recorded.event.schema.json and schemas/events/connector-reply-failed.event.schema.json

**Checkpoint**: User Story 3 reply behavior is independently functional and conformance-aligned.

---

## Phase 6: User Story 4 - Prove Hosted Production Readiness (Priority: P2)

**Goal**: Release reviewers can inspect Discord conformance, reconnect, rate-limit, repair, and live-smoke evidence, including a structured skip when safe credentials are unavailable.

**Independent Test**: Run Discord conformance regression, fake reconnect/rate-limit cases, capability declaration checks, and live smoke pass or structured skip validation. The story is independently testable with fixture evidence; final hosted-ready release review also consumes completed setup, diagnostic, routing, and delivery-separation evidence from US1-US3.

### Tests for User Story 4

- [X] T065 [P] [US4] Add Discord conformance profile tests in daemon/internal/connectors/discord/runtime_test.go
- [X] T066 [P] [US4] Add gateway reconnect evidence tests in daemon/internal/connectors/discord/transport_test.go
- [X] T067 [P] [US4] Add Discord rate-limit evidence tests in daemon/internal/connectors/discord/transport_test.go
- [X] T068 [P] [US4] Add live smoke structured skip tests in daemon/internal/connectors/live_validation_fake_test.go
- [X] T069 [P] [US4] Add Discord smoke API contract tests in daemon/internal/contracts/discord_hardening_contract_test.go

### Implementation for User Story 4

- [X] T070 [US4] Update Discord capability profile to pass required core invariants in daemon/internal/connectors/discord/runtime.go
- [X] T071 [US4] Declare unsupported or limited Discord provider surfaces explicitly in daemon/internal/connectors/discord/runtime.go
- [X] T072 [US4] Record gateway reconnect and disconnect evidence in daemon/internal/connectors/discord/transport.go
- [X] T073 [US4] Record Discord rate-limit evidence for send/edit/gateway operations in daemon/internal/connectors/discord/transport.go
- [X] T074 [US4] Implement Discord live smoke pass or structured skip evidence in daemon/internal/connectors/discord/smoke.go
- [X] T075 [US4] Persist Discord smoke evidence with redaction and retention in daemon/internal/store/discord_setup.go
- [X] T076 [US4] Project Discord conformance and smoke evidence through API handlers in daemon/internal/api/live_validation.go
- [X] T077 [US4] Update Discord smoke evidence schema and fixtures in schemas/api/discord-smoke-evidence-resource.schema.json
- [X] T078 [US4] Document live smoke, structured skip, and residual risk rules in docs/channels/discord-channel-loop.md

**Checkpoint**: User Story 4 readiness evidence is independently functional and release-reviewable.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final integration, compatibility, documentation, and verification across the whole roadmap.

- [X] T079 [P] Update phase 49 contract notes with implemented surface names in specs/034-discord-production-hardening/contracts/discord-production-hardening.md
- [X] T080 [P] Update channel conformance handoff notes for Discord in docs/channels/channel-connector-conformance.md
- [X] T081 [P] Add or update SDK client types if public Discord setup or diagnostic shapes changed in sdk/ts/src/index.ts
- [X] T082 [P] Add or update web operator projections if public Discord repair surfaces changed in web/src/features/integration-diagnostics.tsx
- [X] T083 [P] Add or update TUI operator projections if public Discord repair surfaces changed in tui/src/index.ts
- [X] T084 Run focused daemon package tests from quickstart in daemon/
- [X] T085 Run full daemon tests with go test ./... in daemon/
- [X] T086 Run daemon contract verification with make daemon-contract-test in /Users/John/Code/dope-agent
- [X] T087 Run client verification with pnpm test:clients in /Users/John/Code/dope-agent if SDK/web/TUI surfaces changed
- [X] T088 Run client build with pnpm build in /Users/John/Code/dope-agent if SDK/web/TUI surfaces changed
- [X] T089 Run go mod tidy in daemon/
- [X] T090 Record final rollback and residual live-smoke risk notes in specs/034-discord-production-hardening/quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 Setup**: No dependencies.
- **Phase 2 Foundational**: Depends on Phase 1; blocks all user stories.
- **Phase 3 US1**: Depends on Phase 2. This is the MVP.
- **Phase 4 US2**: Depends on Phase 2; can run in parallel with US1 after shared persistence and schemas exist.
- **Phase 5 US3**: Depends on Phase 2; can run in parallel with US1/US2 after shared diagnostics and route evidence primitives exist.
- **Phase 6 US4**: Depends on Phase 2 for fixture-level conformance and smoke scaffolding. Final release-readiness review composes validated evidence from US1, US2, and US3.
- **Phase 7 Polish**: Depends on the desired user stories being complete.

### User Story Dependencies

- **US1 Connect Discord With Tenant-Owned Credentials**: No dependency on other stories after Phase 2; establishes MVP hosted setup.
- **US2 Repair Discord Readiness Failures**: No dependency on US1 after Phase 2, but should reuse US1 setup evidence when both are complete.
- **US3 Receive Predictable Discord Replies**: No dependency on US1 or US2 after Phase 2, but final route diagnostics should reuse US2 reason mapping.
- **US4 Prove Hosted Production Readiness**: Fixture-level conformance, reconnect/rate-limit, and smoke evidence can start after Phase 2. Final hosted-ready proof requires completed evidence from US1, US2, and US3.

### Within Each User Story

- Write tests first and confirm they fail for missing behavior.
- Implement domain logic before persistence projections.
- Implement persistence before API/event projection.
- Update schemas and docs with the behavior they expose.
- Validate each story at its checkpoint before moving to lower-priority work.

## Parallel Opportunities

- Setup tasks T001-T006 can run in parallel because they touch distinct schema, contract, and documentation files.
- Foundational tasks T013, T015, T016, T020, and T021 can run in parallel after T007-T012 are assigned because they touch different fixture/test/helper files.
- US1 test tasks T022-T026 can run in parallel before implementation.
- US2 test tasks T036-T040 can run in parallel before implementation.
- US3 test tasks T048-T054 can run in parallel before implementation.
- US4 test tasks T065-T069 can run in parallel before implementation.
- Documentation/client polish tasks T079-T083 can run in parallel after public surface decisions are known.

## Parallel Example: User Story 1

```text
Task: "T022 [P] [US1] Add setup validation tests for valid credentials and explicit destinations in daemon/internal/connectors/discord/readiness_test.go"
Task: "T023 [P] [US1] Add setup validation tests for missing explicit destinations and partial destination failures in daemon/internal/connectors/discord/readiness_test.go"
Task: "T024 [P] [US1] Add credential redaction tests for setup evidence in daemon/internal/connectors/discord/diagnostics_test.go"
Task: "T025 [P] [US1] Add Discord setup persistence tests for degraded/needs-repair state in daemon/internal/store/discord_setup_test.go"
Task: "T026 [P] [US1] Add Discord setup API projection tests in daemon/internal/api/setupwizard_test.go"
```

## Parallel Example: User Story 2

```text
Task: "T036 [P] [US2] Add Discord diagnostic mapping tests in daemon/internal/connectors/discord/diagnostics_test.go"
Task: "T037 [P] [US2] Add diagnostic freshness and 90-day retention tests in daemon/internal/connectors/diagnostics_test.go"
Task: "T038 [P] [US2] Add redaction suppression tests for unsafe Discord evidence in daemon/internal/connectors/discord/diagnostics_test.go"
Task: "T039 [P] [US2] Add tenant authorization tests for Discord diagnostics in daemon/internal/api/integration_diagnostics_test.go"
Task: "T040 [P] [US2] Add diagnostic persistence and retention tests in daemon/internal/store/connector_diagnostics_test.go"
```

## Parallel Example: User Story 3

```text
Task: "T048 [P] [US3] Add Discord DM and mention routing tests in daemon/internal/connectors/discord/runtime_test.go"
Task: "T049 [P] [US3] Add blocked guild/channel route outcome tests in daemon/internal/connectors/discord/runtime_test.go"
Task: "T050 [P] [US3] Add mention normalization tests in daemon/internal/connectors/discord/transport_test.go"
Task: "T051 [P] [US3] Add duplicate inbound replay tests in daemon/internal/im/loop_test.go"
Task: "T052 [P] [US3] Add reply failure separation tests in daemon/internal/im/loop_test.go"
Task: "T053 [P] [US3] Add connector-backed delivery separation tests in daemon/internal/delivery/connector_adapter_test.go"
Task: "T054 [P] [US3] Add reply progression degradation tests in daemon/internal/connectors/discord/runtime_test.go"
```

## Parallel Example: User Story 4

```text
Task: "T065 [P] [US4] Add Discord conformance profile tests in daemon/internal/connectors/discord/runtime_test.go"
Task: "T066 [P] [US4] Add gateway reconnect evidence tests in daemon/internal/connectors/discord/transport_test.go"
Task: "T067 [P] [US4] Add Discord rate-limit evidence tests in daemon/internal/connectors/discord/transport_test.go"
Task: "T068 [P] [US4] Add live smoke structured skip tests in daemon/internal/connectors/live_validation_fake_test.go"
Task: "T069 [P] [US4] Add Discord smoke API contract tests in daemon/internal/contracts/discord_hardening_contract_test.go"
```

## Implementation Strategy

### MVP First

1. Complete Phase 1 setup.
2. Complete Phase 2 foundational primitives.
3. Complete Phase 3 US1.
4. Stop and validate Discord setup independently with fake credentials, explicit destination fixtures, and legacy local config projection.

### Incremental Delivery

1. US1 delivers hosted setup and degraded repair gating.
2. US2 delivers operator diagnostics and supportability.
3. US3 delivers predictable Discord reply behavior plus foreground/background delivery separation.
4. US4 delivers production-readiness proof and live-smoke evidence.
5. Phase 7 closes docs, contracts, verification, rollback, and residual risk notes.

### Parallel Team Strategy

After Phase 2, split work by story ownership:

- Engineer A: US1 setup/readiness.
- Engineer B: US2 diagnostics/repair.
- Engineer C: US3 routing/replies/delivery separation.
- Engineer D: US4 conformance/smoke.

Coordinate schema, event, and shared file changes before merging because `runtime.go`, `transport.go`, `discord_setup.go`, `connector_adapter.go`, and schema fixtures are likely integration points.

## Notes

- `[P]` tasks touch distinct files or are test tasks that can be authored independently.
- `[US1]`, `[US2]`, `[US3]`, and `[US4]` labels map directly to the user stories in [spec.md](./spec.md).
- Do not mark Discord hosted-ready until explicit selected destinations validate.
- Do not run live Discord smoke unless credentials are safe: test tenant only, explicitly approved by an operator, non-production, redacted in evidence, and scoped to the validation path.
- Do not log or fixture raw Discord tokens, authorization headers, credential-bearing payloads, raw provider payloads, or cross-tenant data.
- Run `go mod tidy` from `daemon/` before considering implementation complete.
