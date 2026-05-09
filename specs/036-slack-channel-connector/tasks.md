# Tasks: Slack Channel Connector

**Input**: Design documents from `/specs/036-slack-channel-connector/`
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/slack-channel-connector.md](./contracts/slack-channel-connector.md), [quickstart.md](./quickstart.md)

**Tests**: Required. This is production channel behavior touching OAuth setup, diagnostics, routing, persistence, contracts, delivery, and operator evidence. Write targeted tests before implementation and confirm they fail for the missing behavior.

**Organization**: Tasks are grouped by user story so each story can be implemented and tested independently after the foundational phase.

## Phase 1: Setup

**Purpose**: Establish Slack-specific contract, schema, event, package, and documentation scaffolding.

- [X] T001 [P] Add Slack hosted setup schema contract in schemas/api/slack-hosted-setup-resource.schema.json
- [X] T002 [P] Add Slack route policy schema contract in schemas/api/slack-route-policy-resource.schema.json
- [X] T003 [P] Add Slack smoke evidence schema contract in schemas/api/slack-smoke-evidence-resource.schema.json
- [X] T004 [P] Add Slack setup validation event schema in schemas/events/connector-slack-setup-validated.event.schema.json
- [X] T005 [P] Add Slack channel connector contract tests in daemon/internal/contracts/slack_connector_contract_test.go
- [X] T006 [P] Add Slack operator channel documentation in docs/channels/slack-channel-loop.md
- [X] T007 [P] Add Slack connector package documentation in daemon/internal/connectors/slack/doc.go
- [X] T008 [P] Add Slack connector API schema references in schemas/api/connector-resource.schema.json
- [X] T009 [P] Add Slack connector docs entry in docs/channels/README.md

---

## Phase 2: Foundational

**Purpose**: Blocking domain, migration, contract, and tenant-safety primitives used by every user story.

**Critical**: No user story implementation should begin until this phase is complete.

- [X] T010 Add Slack setup/readiness domain types in daemon/internal/connectors/slack/readiness.go
- [X] T011 Add Slack route policy domain types in daemon/internal/connectors/slack/destinations.go
- [X] T012 Add Slack diagnostic mapping helpers in daemon/internal/connectors/slack/diagnostics.go
- [X] T013 Add Slack smoke evidence domain types in daemon/internal/connectors/slack/smoke.go
- [X] T014 Add Slack transport interfaces and fake transport boundary in daemon/internal/connectors/slack/transport.go
- [X] T015 Add Slack runtime skeleton and capability declarations in daemon/internal/connectors/slack/runtime.go
- [X] T016 Add tenant-safe Slack setup persistence accessors in daemon/internal/store/slack_setup.go
- [X] T017 Add Slack setup, workspace binding, route policy, smoke, and retained event evidence migrations in daemon/internal/store/store.go
- [X] T018 [P] Add Slack migration fixture in daemon/internal/store/migrationfixture/r51_slack_channel_connector.go
- [X] T019 [P] Add Slack setup migration tests in daemon/internal/store/slack_setup_test.go
- [X] T020 [P] Add Slack setup tenant boundary helpers in daemon/internal/store/tenancy/slack_setup.go
- [X] T021 Add Slack setup API projection types in daemon/internal/api/setupwizard.go
- [X] T022 Add Slack event construction helpers in daemon/internal/events/connector_slack.go
- [X] T023 [P] Add foundational Slack schema fixture coverage in daemon/internal/contracts/slack_connector_contract_test.go
- [X] T024 [P] Add Slack connector conformance fixture coverage in daemon/internal/contracts/connector_conformance_contracts_test.go
- [X] T025 [P] Add Slack setup redaction test helpers in daemon/internal/connectors/slack/diagnostics_test.go
- [X] T026 Add Slack connector registration placeholder in daemon/internal/connectors/supervisor.go
- [X] T027 Add Slack setup target registration for hosted OAuth in daemon/internal/setupwizard/targets.go
- [X] T028 Add Slack OAuth setup probe placeholder in daemon/internal/setupwizard/probe.go

**Checkpoint**: Foundational Slack setup, workspace binding, route policy, diagnostic, smoke, schema, migration, and package primitives exist.

---

## Phase 3: User Story 1 - Connect A Slack Workspace (Priority: P1) - MVP

**Goal**: A hosted tenant user can complete hosted Slack app installation/OAuth setup, bind exactly one workspace per connector, configure selected routes, inspect redacted setup evidence, retry/replace/cancel/disable setup, and avoid leaking secrets.

**Independent Test**: Complete the hosted Slack app installation/OAuth setup flow for a fake safe Slack workspace, select at least one allowed channel, verify `ready`, then test missing installation, missing OAuth grant, missing scope, workspace approval, wrong workspace, missing route policy, cancellation, retry, replacement, disablement, timeout, and redaction cases.

### Tests for User Story 1

- [X] T029 [P] [US1] Add Slack OAuth setup validation tests for valid workspace installation in daemon/internal/connectors/slack/readiness_test.go
- [X] T030 [US1] Add Slack setup terminal-state tests for missing OAuth grant, revoked installation, missing scope, approval required, unavailable, cancelled, and action-required states in daemon/internal/connectors/slack/readiness_test.go
- [X] T031 [US1] Add Slack readiness gate tests proving valid OAuth installation without selected channel or DM allowment remains action-required in daemon/internal/connectors/slack/readiness_test.go
- [X] T032 [US1] Add Slack one-workspace-per-connector and multiple-connectors-per-tenant tests in daemon/internal/connectors/slack/readiness_test.go
- [X] T033 [US1] Add Slack setup timing and timeout tests for the 5-minute setup outcome bound in daemon/internal/connectors/slack/readiness_test.go
- [X] T034 [P] [US1] Add Slack OAuth redaction tests for setup evidence in daemon/internal/connectors/slack/diagnostics_test.go
- [X] T035 [P] [US1] Add Slack setup persistence tests for retry, replacement, cancellation, disablement, workspace binding, route policy, and retention in daemon/internal/store/slack_setup_test.go
- [X] T036 [P] [US1] Add Slack setup API projection tests in daemon/internal/api/setupwizard_test.go
- [X] T037 [P] [US1] Add Slack hosted OAuth setup wizard tests in daemon/internal/setupwizard/service_test.go
- [X] T038 [P] [US1] Add Slack setup event schema tests in daemon/internal/contracts/slack_connector_contract_test.go

### Implementation for User Story 1

- [X] T039 [US1] Implement Slack OAuth installation validation result handling in daemon/internal/connectors/slack/readiness.go
- [X] T040 [US1] Implement Slack workspace binding summaries with exactly one workspace per connector in daemon/internal/connectors/slack/readiness.go
- [X] T041 [US1] Implement setup terminal-state mapping for ready, degraded, unavailable, cancelled, and action-required in daemon/internal/connectors/slack/readiness.go
- [X] T042 [US1] Enforce selected channel or explicit DM allowment before Slack setup can report ready in daemon/internal/connectors/slack/readiness.go
- [X] T043 [US1] Reject submitted raw Slack bot tokens, signing secrets, and local-only credentials in daemon/internal/setupwizard/service.go
- [X] T044 [US1] Enforce setup validation timeout and actionable terminal-state diagnostics in daemon/internal/connectors/slack/readiness.go
- [X] T045 [US1] Persist Slack hosted setup, OAuth installation, workspace binding, and route policy evidence in daemon/internal/store/slack_setup.go
- [X] T046 [US1] Implement retry, replacement, cancellation, disablement, and retention accessors in daemon/internal/store/slack_setup.go
- [X] T047 [US1] Project Slack setup state through setup wizard APIs in daemon/internal/api/setupwizard.go
- [X] T048 [US1] Wire Slack hosted OAuth setup target and probe behavior in daemon/internal/setupwizard/targets.go
- [X] T049 [US1] Emit redacted Slack setup validation events in daemon/internal/connectors/slack/runtime.go
- [X] T050 [US1] Implement Slack setup event payload construction in daemon/internal/events/connector_slack.go
- [X] T051 [US1] Update Slack setup schema fixtures in schemas/api/slack-hosted-setup-resource.schema.json
- [X] T052 [US1] Document Slack hosted setup, repair, cancellation, OAuth redaction, workspace binding, and unsupported setup modes in docs/channels/slack-channel-loop.md

**Checkpoint**: User Story 1 is independently functional and testable as the MVP.

---

## Phase 4: User Story 2 - Route Slack DMs And Channel Mentions (Priority: P2)

**Goal**: Slack messages create runs only for explicitly allowed DM users or user-group members and selected channel mentions, wrong-workspace and unselected traffic fails closed, duplicate workspace/conversation/message deliveries are suppressed, and Slack event identity is retained as redacted evidence.

**Independent Test**: Feed fake Slack events for allowed DMs, user-group DMs, blocked DMs, selected channel mentions, selected channel messages without mention, unselected channels, wrong workspaces, disabled connector state, duplicate replay, missing identity, and unsupported Slack surfaces; verify routing decisions and no duplicate runs or replies.

### Tests for User Story 2

- [X] T053 [P] [US2] Add Slack route policy validation tests in daemon/internal/connectors/slack/destinations_test.go
- [X] T054 [P] [US2] Add Slack DM allow/block routing tests in daemon/internal/connectors/slack/runtime_test.go
- [X] T055 [US2] Add Slack user-group DM allowment routing tests in daemon/internal/connectors/slack/runtime_test.go
- [X] T056 [US2] Add Slack selected channel, mention, no-mention, unselected channel, and wrong-workspace routing tests in daemon/internal/connectors/slack/runtime_test.go
- [X] T057 [US2] Add Slack unsupported file, voice clip, huddle, canvas, workflow button, interactive block, rich media, thinking, and incremental update tests in daemon/internal/connectors/slack/runtime_test.go
- [X] T058 [P] [US2] Add Slack workspace/conversation/message dedupe replay tests in daemon/internal/im/loop_test.go
- [X] T059 [P] [US2] Add retained Slack event evidence tests in daemon/internal/store/slack_setup_test.go
- [X] T060 [P] [US2] Add Slack tenant authorization tests for route policy and routing evidence in daemon/internal/api/setupwizard_test.go
- [X] T061 [P] [US2] Add Slack route outcome event tests in daemon/internal/events/connector_slack_test.go

### Implementation for User Story 2

- [X] T062 [US2] Implement explicit Slack selected channel, DM user, and DM user-group validation in daemon/internal/connectors/slack/destinations.go
- [X] T063 [US2] Persist Slack route policy records and validation evidence in daemon/internal/store/slack_setup.go
- [X] T064 [US2] Enforce direct-message sender allowment in daemon/internal/connectors/slack/runtime.go
- [X] T065 [US2] Enforce Slack user-group membership allowment in daemon/internal/connectors/slack/runtime.go
- [X] T066 [US2] Enforce selected channel plus agent mention routing in daemon/internal/connectors/slack/runtime.go
- [X] T067 [US2] Normalize Slack agent mention artifacts before assistant handling in daemon/internal/connectors/slack/transport.go
- [X] T068 [US2] Record accepted, ignored, blocked, duplicate, unsupported, and failed route outcomes in daemon/internal/connectors/slack/runtime.go
- [X] T069 [US2] Implement unsupported outcomes for raw unsupported Slack message surfaces in daemon/internal/connectors/slack/transport.go
- [X] T070 [US2] Apply Slack workspace/conversation/message dedupe identity in daemon/internal/connectors/slack/runtime.go
- [X] T071 [US2] Retain Slack event identity as redacted delivery evidence in daemon/internal/store/slack_setup.go
- [X] T072 [US2] Project Slack route policy and route evidence through API handlers in daemon/internal/api/setupwizard.go
- [X] T073 [US2] Update Slack route policy schema fixtures in schemas/api/slack-route-policy-resource.schema.json
- [X] T074 [US2] Document Slack routing, DM allowment, user-group allowment, channel mention gating, dedupe, and unsupported surfaces in docs/channels/slack-channel-loop.md

**Checkpoint**: User Story 2 routing behavior is independently functional and conformance-aligned.

---

## Phase 5: User Story 3 - Reply, Thread, And Diagnose Slack Outcomes (Priority: P3)

**Goal**: Accepted Slack DMs receive final-only replies in the DM conversation, accepted channel mentions receive final-only replies in a thread rooted at the triggering message, Slack can be selected as a background delivery target, reply and delivery outcomes stay separate from assistant execution, and operators can inspect redacted diagnostics for scope, installation, approval, route, duplicate, rate-limit, provider, network, event-delivery, reply, unsupported, and unknown failures.

**Independent Test**: Simulate accepted DMs, accepted channel mentions, thread reply success/failure, background delivery success/retry/suppression/failure, missing scopes, missing installation, rate limits, provider outages, network failures, event-delivery failures, unsupported surfaces, stale diagnostics, retention expiry, redaction suppression, live smoke pass, and structured skip with fake Slack transport.

### Tests for User Story 3

- [X] T075 [P] [US3] Add Slack direct-message final-only foreground reply tests in daemon/internal/connectors/slack/runtime_test.go
- [X] T076 [US3] Add Slack channel mention thread-rooted reply tests in daemon/internal/connectors/slack/runtime_test.go
- [X] T077 [US3] Add Slack thread reply failure separation tests in daemon/internal/connectors/slack/runtime_test.go
- [X] T078 [P] [US3] Add Slack reply failure separation tests in daemon/internal/im/loop_test.go
- [X] T079 [P] [US3] Add Slack connector-backed delivery adapter tests in daemon/internal/delivery/connector_adapter_test.go
- [X] T080 [P] [US3] Add Slack delivery separation event tests in daemon/internal/events/connector_delivery_test.go
- [X] T081 [P] [US3] Add Slack diagnostic mapping tests in daemon/internal/connectors/slack/diagnostics_test.go
- [X] T082 [P] [US3] Add Slack support diagnostic retrieval tests for the 2-minute inspection bound in daemon/internal/api/integration_diagnostics_test.go
- [X] T083 [P] [US3] Add Slack diagnostic freshness and 90-day retention tests in daemon/internal/connectors/diagnostics_test.go
- [X] T084 [US3] Add Slack redaction suppression tests for unsafe evidence in daemon/internal/connectors/slack/diagnostics_test.go
- [X] T085 [P] [US3] Add Slack live smoke structured skip tests in daemon/internal/connectors/slack/smoke_test.go
- [X] T086 [US3] Add Slack fake safe-live smoke pass tests in daemon/internal/connectors/slack/smoke_test.go
- [X] T087 [US3] Add Slack capability and conformance profile tests for marketplace publication, enterprise grid administration, memory-based team context, and unsupported surfaces in daemon/internal/connectors/slack/runtime_test.go
- [X] T088 [P] [US3] Add Slack smoke schema contract tests in daemon/internal/contracts/slack_connector_contract_test.go

### Implementation for User Story 3

- [X] T089 [US3] Implement final-only Slack direct-message foreground replies in daemon/internal/connectors/slack/runtime.go
- [X] T090 [US3] Implement final-only Slack channel replies rooted at the triggering message thread in daemon/internal/connectors/slack/runtime.go
- [X] T091 [US3] Separate assistant execution outcome from Slack reply outcome in daemon/internal/im/loop.go
- [X] T092 [US3] Emit Slack reply sent and reply failed evidence in daemon/internal/connectors/slack/runtime.go
- [X] T093 [US3] Implement Slack connector-backed delivery adapter behavior in daemon/internal/delivery/connector_adapter.go
- [X] T094 [US3] Preserve Slack foreground/background delivery separation in daemon/internal/delivery/connector_adapter.go
- [X] T095 [US3] Map Slack provider failures to shared diagnostic reason codes in daemon/internal/connectors/slack/diagnostics.go
- [X] T096 [US3] Persist Slack diagnostic freshness, retention, and redaction evidence in daemon/internal/store/connector_diagnostics.go
- [X] T097 [US3] Project tenant-safe Slack diagnostics through API handlers in daemon/internal/api/integration_diagnostics.go
- [X] T098 [US3] Emit Slack diagnostic, route, reply, and delivery events with redacted evidence in daemon/internal/events/connector_slack.go
- [X] T099 [US3] Declare Slack capability profile with marketplace publication, enterprise grid administration, memory-based team context, and unsupported surfaces in daemon/internal/connectors/slack/runtime.go
- [X] T100 [US3] Implement Slack live smoke pass or structured skip evidence in daemon/internal/connectors/slack/smoke.go
- [X] T101 [US3] Persist Slack smoke evidence with redaction and retention in daemon/internal/store/slack_setup.go
- [X] T102 [US3] Project Slack smoke evidence through live validation APIs in daemon/internal/api/live_validation.go
- [X] T103 [US3] Update Slack smoke schema fixtures in schemas/api/slack-smoke-evidence-resource.schema.json
- [X] T104 [US3] Document Slack diagnostics, thread replies, delivery, unsupported marketplace publication, unsupported enterprise grid administration, unsupported memory-based team context, live smoke, and structured skip behavior in docs/channels/slack-channel-loop.md

**Checkpoint**: User Story 3 reply, delivery, diagnostic, and readiness evidence is independently functional and release-reviewable.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final integration, compatibility, documentation, client surfaces, and verification across the whole roadmap.

- [X] T105 [P] Update phase 51 planning contract notes with implemented surface names in specs/036-slack-channel-connector/contracts/slack-channel-connector.md
- [X] T106 [P] Update channel conformance handoff notes for Slack in docs/channels/channel-connector-conformance.md
- [X] T107 [P] Update SDK client Slack setup, smoke, conformance, and config types in sdk/ts/src/index.ts
- [X] T108 [P] Add SDK tests for Slack setup, smoke, conformance, and config routes in sdk/ts/src/index.test.ts
- [X] T109 [P] Update web setup and diagnostics projections for Slack OAuth targets in web/src/app/App.tsx
- [X] T110 [P] Add web tests for Slack OAuth setup target handling in web/src/app/App.test.tsx
- [X] T111 [P] Update TUI operator projections for Slack setup or diagnostic surfaces in tui/src/index.ts
- [X] T112 [P] Add TUI tests for Slack setup or diagnostic command output in tui/src/cli.test.ts
- [X] T113 Run focused daemon package tests from quickstart in daemon/
- [X] T114 Run full daemon tests with go test ./... in daemon/
- [X] T115 Run daemon contract verification with make daemon-contract-test in /Users/John/Code/dope-agent/
- [X] T116 Run client verification with pnpm test:clients in /Users/John/Code/dope-agent/
- [X] T117 Run client build with pnpm build in /Users/John/Code/dope-agent/
- [X] T118 Run go mod tidy in daemon/
- [X] T119 Record final rollback and residual live-smoke risk notes in specs/036-slack-channel-connector/quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 Setup**: No dependencies.
- **Phase 2 Foundational**: Depends on Phase 1; blocks all user stories.
- **Phase 3 US1**: Depends on Phase 2. This is the MVP.
- **Phase 4 US2**: Depends on Phase 2 and can develop route policy and blocked/unsupported fixtures independently. Full accepted-message validation uses a ready setup from US1.
- **Phase 5 US3**: Depends on Phase 2 and can develop diagnostics/delivery fixtures independently. Full foreground reply validation composes accepted-message evidence from US2 and setup readiness from US1.
- **Phase 6 Polish**: Depends on the desired user stories being complete.

### User Story Dependencies

- **US1 Connect A Slack Workspace**: No dependency on other stories after Phase 2; establishes the MVP hosted OAuth setup, one-workspace binding, route-policy readiness, and redacted setup path.
- **US2 Route Slack DMs And Channel Mentions**: No dependency on US3 after Phase 2; uses foundational route policy and runtime primitives, and uses US1 ready setup for end-to-end accepted-message verification.
- **US3 Reply, Thread, And Diagnose Slack Outcomes**: No dependency on US2 for diagnostic and delivery fixtures after Phase 2; final foreground reply validation uses accepted-message routing from US2.

### Within Each User Story

- Write tests first and confirm they fail for missing behavior.
- Implement domain logic before persistence projections.
- Implement persistence before API/event projection.
- Update schemas and docs with the behavior they expose.
- Validate each story at its checkpoint before moving to lower-priority work.

## Parallel Opportunities

- Setup tasks T001-T009 can run in parallel because they touch distinct schema, contract, event, package-doc, and documentation files.
- Foundational tasks T018, T019, T020, T023, T024, and T025 can run in parallel after the domain/storage ownership is assigned because they touch different fixture/test/helper files.
- US1 test tasks T029, T034, T035, T036, T037, and T038 can run in parallel before implementation.
- US2 test tasks T053, T054, T058, T059, T060, and T061 can run in parallel before implementation.
- US3 test tasks T075, T078, T079, T080, T081, T082, T083, T085, and T088 can run in parallel before implementation.
- Documentation/client polish tasks T105-T112 can run in parallel after public surface decisions are known.

## Parallel Example: User Story 1

```text
Task: "T029 [P] [US1] Add Slack OAuth setup validation tests for valid workspace installation in daemon/internal/connectors/slack/readiness_test.go"
Task: "T034 [P] [US1] Add Slack OAuth redaction tests for setup evidence in daemon/internal/connectors/slack/diagnostics_test.go"
Task: "T035 [P] [US1] Add Slack setup persistence tests for retry, replacement, cancellation, disablement, workspace binding, route policy, and retention in daemon/internal/store/slack_setup_test.go"
Task: "T036 [P] [US1] Add Slack setup API projection tests in daemon/internal/api/setupwizard_test.go"
Task: "T037 [P] [US1] Add Slack hosted OAuth setup wizard tests in daemon/internal/setupwizard/service_test.go"
Task: "T038 [P] [US1] Add Slack setup event schema tests in daemon/internal/contracts/slack_connector_contract_test.go"
```

## Parallel Example: User Story 2

```text
Task: "T053 [P] [US2] Add Slack route policy validation tests in daemon/internal/connectors/slack/destinations_test.go"
Task: "T054 [P] [US2] Add Slack DM allow/block routing tests in daemon/internal/connectors/slack/runtime_test.go"
Task: "T058 [P] [US2] Add Slack workspace/conversation/message dedupe replay tests in daemon/internal/im/loop_test.go"
Task: "T059 [P] [US2] Add retained Slack event evidence tests in daemon/internal/store/slack_setup_test.go"
Task: "T060 [P] [US2] Add Slack tenant authorization tests for route policy and routing evidence in daemon/internal/api/setupwizard_test.go"
Task: "T061 [P] [US2] Add Slack route outcome event tests in daemon/internal/events/connector_slack_test.go"
```

## Parallel Example: User Story 3

```text
Task: "T075 [P] [US3] Add Slack direct-message final-only foreground reply tests in daemon/internal/connectors/slack/runtime_test.go"
Task: "T078 [P] [US3] Add Slack reply failure separation tests in daemon/internal/im/loop_test.go"
Task: "T079 [P] [US3] Add Slack connector-backed delivery adapter tests in daemon/internal/delivery/connector_adapter_test.go"
Task: "T080 [P] [US3] Add Slack delivery separation event tests in daemon/internal/events/connector_delivery_test.go"
Task: "T081 [P] [US3] Add Slack diagnostic mapping tests in daemon/internal/connectors/slack/diagnostics_test.go"
Task: "T082 [P] [US3] Add Slack support diagnostic retrieval tests for the 2-minute inspection bound in daemon/internal/api/integration_diagnostics_test.go"
Task: "T083 [P] [US3] Add Slack diagnostic freshness and 90-day retention tests in daemon/internal/connectors/diagnostics_test.go"
Task: "T085 [P] [US3] Add Slack live smoke structured skip tests in daemon/internal/connectors/slack/smoke_test.go"
Task: "T088 [P] [US3] Add Slack smoke schema contract tests in daemon/internal/contracts/slack_connector_contract_test.go"
```

## Implementation Strategy

### MVP First

1. Complete Phase 1 setup.
2. Complete Phase 2 foundational primitives.
3. Complete Phase 3 US1.
4. Stop and validate Slack setup independently with fake OAuth installation evidence, selected route-policy gating, workspace cardinality fixtures, terminal-state fixtures, retry/replacement/cancellation cases, timeout checks, API projection tests, and redaction checks.

### Incremental Delivery

1. US1 delivers hosted OAuth setup, one-workspace binding, and redacted setup readiness.
2. US2 delivers explicit DM allowment, user-group allowment, selected-channel mention gating, route outcomes, unsupported surfaces, and dedupe.
3. US3 delivers final replies, required channel thread replies, background delivery reuse, diagnostics, smoke evidence, and release-review evidence.
4. Phase 6 closes docs, contracts, client projections, verification, rollback, and residual risk notes.

### Parallel Team Strategy

After Phase 2, split work by story ownership:

- Engineer A: US1 setup/readiness.
- Engineer B: US2 routing/allowment/dedupe.
- Engineer C: US3 replies/delivery/diagnostics/smoke.

Coordinate schema, event, and shared file changes before merging because `runtime.go`, `transport.go`, `slack_setup.go`, `connector_adapter.go`, `setupwizard.go`, and schema fixtures are likely integration points.

## Notes

- `[P]` tasks touch distinct files or are test tasks that can be authored independently.
- `[US1]`, `[US2]`, and `[US3]` labels map directly to the user stories in [spec.md](./spec.md).
- Do not mark Slack `ready` until OAuth installation, workspace binding, conformance gates, and selected route policy validate.
- Do not accept Slack DMs from senders without explicit user or user-group allowment.
- Do not accept Slack channel messages without selected channel policy plus agent mention or an explicitly supported invocation signal.
- Do not dedupe Slack inbound by event identity alone; use workspace/conversation/message identity and retain event identity as redacted evidence.
- Do not support raw-token setup, signing-secret setup, local-only credentials, marketplace publication, enterprise grid administration, memory-based team context, files, voice clips, huddles, canvases, workflow buttons, interactive blocks, rich media, thinking visibility, or incremental visible updates in phase 51.
- Do not run live Slack smoke unless authorization is safe: test tenant only, explicitly approved by an operator, non-production or approved test workspace, redacted in evidence, and scoped to the validation path.
- Do not log or fixture Slack OAuth tokens, installation grants, signing secrets, authorization headers, credential-bearing payloads, raw provider payloads, or cross-tenant data.
- Run `go mod tidy` from `daemon/` before considering implementation complete.
