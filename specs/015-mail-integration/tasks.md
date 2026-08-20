# Tasks: Mail Integration

**Input**: Design documents from `/specs/015-mail-integration/`  
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/mail-domain-surfaces.md](./contracts/mail-domain-surfaces.md), [quickstart.md](./quickstart.md)

**Tests**: Constitution rules apply. This roadmap changes API, schema, event, persistence, runtime, workflow, schedule, and delivery linkage surfaces, so targeted unit, integration, contract, and restart verification is required.

**Organization**: Tasks are grouped by user story so each story can be implemented and verified as an independently testable increment.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the feature scaffolding and shared file layout for the daemon-owned mail domain.

- [x] T001 Create mail package scaffolding in `daemon/internal/mail/types.go`, `daemon/internal/mail/manager.go`, `daemon/internal/mail/backend.go`, `daemon/internal/mail/fake_backend.go`, and `daemon/internal/mail/artifacts.go`
- [x] T002 [P] Create mail API handler scaffolding in `daemon/internal/api/mail.go`, `daemon/internal/api/mail_execution.go`, and route registration stubs in `daemon/internal/api/server.go`
- [x] T003 [P] Create mail API schema placeholders in `schemas/api/mail-account-resource.schema.json`, `schemas/api/mail-account-list.response.schema.json`, `schemas/api/mail-thread-resource.schema.json`, `schemas/api/mail-thread-list.response.schema.json`, `schemas/api/mail-message-resource.schema.json`, `schemas/api/mail-draft-resource.schema.json`, `schemas/api/mail-draft-list.response.schema.json`, `schemas/api/create-mail-draft.request.schema.json`, `schemas/api/update-mail-draft.request.schema.json`, `schemas/api/send-mail-message.request.schema.json`, `schemas/api/send-mail-draft.request.schema.json`, `schemas/api/reply-mail-message.request.schema.json`, `schemas/api/forward-mail-message.request.schema.json`, `schemas/api/mail-operation-resource.schema.json`, `schemas/api/mail-operation-list.response.schema.json`, `schemas/api/mail-operation-summary.schema.json`, and `schemas/api/mail-workflow-action.schema.json`
- [x] T004 [P] Create mail event schema placeholders in `schemas/events/mail-account-projected.event.schema.json`, `schemas/events/mail-operation-requested.event.schema.json`, `schemas/events/mail-operation-completed.event.schema.json`, `schemas/events/mail-operation-failed.event.schema.json`, and `schemas/events/mail-artifact-recorded.event.schema.json`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Land the shared persistence, fake-backend abstraction, manager wiring, and contract foundations that block all mail stories.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [x] T005 Add SQLite migration and store record types for `mail_accounts`, `mail_operations`, and `mail_artifacts` plus linkage indexes in `daemon/internal/store/store.go`
- [x] T006 [P] Add shared mail account, operation, thread snapshot, message snapshot, draft snapshot, attachment reference, and operation-summary structs in `daemon/internal/mail/types.go` and `daemon/internal/api/types.go`
- [x] T007 [P] Implement store helpers for mail account projection, mail operation, and artifact CRUD/list/get in `daemon/internal/store/store.go`
- [x] T008 [P] Wire mail manager startup, dependency injection, and restore hooks into `daemon/internal/app/app.go` and `daemon/internal/api/server.go`
- [x] T009 [P] Make `fake_local` domain support explicit across the shared integration backend contract and the dedicated mail backend interfaces for deterministic mail-domain support in `daemon/internal/integrations/backend.go`, `daemon/internal/integrations/fake_backend.go`, `daemon/internal/mail/backend.go`, and `daemon/internal/mail/fake_backend.go`
- [x] T010 [P] Add mail contract validation scaffolding in `daemon/internal/contracts/validator.go` and `daemon/internal/contracts/contracts_test.go`
- [x] T011 [P] Add shared `mailOperationSummaries` projection types for runtime, workflow, schedule, and delivery surfaces in `daemon/internal/mail/types.go`, `daemon/internal/api/types.go`, `schemas/api/tool-call-resource.schema.json`, `schemas/api/workflow-step-resource.schema.json`, `schemas/api/schedule-attempt-resource.schema.json`, and `schemas/api/delivery-outcome-resource.schema.json`
- [x] T012 [P] Add foundational restart and environment-scope regressions for persisted mail records in `daemon/internal/app/app_test.go` and `daemon/internal/store/store_test.go`

**Checkpoint**: Mail persistence, manager wiring, fake-backend abstraction, and contract scaffolding are ready; user story work can now proceed.

---

## Phase 3: User Story 1 - Inspect Mailbox State Truthfully (Priority: P1) 🎯 MVP

**Goal**: Users and operators can inspect the selected mailbox account projection, list and inspect threads and messages, and inspect drafts without creating send side effects.

**Independent Test**: Register healthy fake mail integrations, inspect `/v1/mail/accounts`, issue one read with an explicit `integrationId` and one without it, then list threads, inspect one message, and inspect drafts while confirming the chosen mailbox projection, canonical-default fallback, and message or draft truth are returned without any send side effects.

### Tests for User Story 1

- [x] T013 [P] [US1] Add contract tests for mail account projection, explicit-`integrationId` route selection, canonical-default fallback, thread list/detail, message detail, and draft inspection routes plus `mail.account_projected` coverage in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [x] T014 [P] [US1] Add manager and store regressions for explicit-`integrationId` selection, canonical-default fallback, mailbox projection, thread inspection, message inspection, and draft inspection non-mutation behavior in `daemon/internal/mail/manager_test.go` and `daemon/internal/store/store_test.go`

### Implementation for User Story 1

- [x] T015 [P] [US1] Implement mail account projection, explicit-`integrationId` read selection, canonical-default fallback, and fake backend read models in `daemon/internal/mail/backend.go`, `daemon/internal/mail/fake_backend.go`, and `daemon/internal/mail/manager.go`
- [x] T016 [P] [US1] Implement thread list/detail, message detail, and draft list/detail handling in `daemon/internal/mail/manager.go` and `daemon/internal/mail/types.go`
- [x] T017 [US1] Implement mail account, thread, message, and draft inspection route handlers with optional request-scoped `integrationId` selection in `daemon/internal/api/mail.go` and `daemon/internal/api/server.go`
- [x] T018 [US1] Persist mail account projections, read operations, and structured thread, message, and draft artifacts for inspection routes in `daemon/internal/store/store.go` and `daemon/internal/mail/artifacts.go`
- [x] T019 [US1] Finalize mail account, thread, message, and draft inspection API schemas including optional `integrationId` selection surfaces in `schemas/api/mail-account-resource.schema.json`, `schemas/api/mail-account-list.response.schema.json`, `schemas/api/mail-thread-resource.schema.json`, `schemas/api/mail-thread-list.response.schema.json`, `schemas/api/mail-message-resource.schema.json`, `schemas/api/mail-draft-resource.schema.json`, and `schemas/api/mail-draft-list.response.schema.json`
- [x] T020 [US1] Publish mailbox projection, mail read operation, and artifact events in `daemon/internal/mail/manager.go`, `schemas/events/mail-account-projected.event.schema.json`, `schemas/events/mail-operation-requested.event.schema.json`, `schemas/events/mail-operation-completed.event.schema.json`, `schemas/events/mail-operation-failed.event.schema.json`, and `schemas/events/mail-artifact-recorded.event.schema.json`

**Checkpoint**: User Story 1 is complete when mailbox account projection, thread inspection, message inspection, and draft inspection are operator-visible and independently testable without any send side effects.

---

## Phase 4: User Story 2 - Draft And Send With Explicit Side-Effect Truth (Priority: P2)

**Goal**: Users can create and update drafts, directly send new messages, send existing drafts, reply, and forward while preserving truthful distinction between draft-only and sent outcomes, explicit-recipient requirements, and blocked-send attachment failure truth.

**Independent Test**: Create one draft, update it, send one direct message, send one existing draft, reply to one message, and forward one message through the fake mail backend while confirming send path truth, stable draft and thread linkage, explicit-recipient enforcement for new outbound mail, and blocked final send when attachment references are unresolved.

### Tests for User Story 2

- [x] T021 [P] [US2] Add contract and API regressions for draft create/update, direct send, send-existing-draft, reply, forward, and mail operation inspection routes in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [x] T022 [P] [US2] Add manager and store regressions for stable draft identity, send-path truth, explicit-recipient enforcement, blocked-send behavior for unresolved attachments, and reply or forward linkage in `daemon/internal/mail/manager_test.go` and `daemon/internal/store/store_test.go`
- [x] T023 [P] [US2] Add regression coverage for attachment metadata-only scope, draft-only versus sent result separation, and stale-state or unavailable outbound failure truth in `daemon/internal/mail/manager_test.go` and `daemon/internal/api/server_test.go`

### Implementation for User Story 2

- [x] T024 [P] [US2] Implement draft lifecycle plus direct-send and send-existing-draft logic with explicit send-path truth in `daemon/internal/mail/manager.go` and `daemon/internal/mail/types.go`
- [x] T025 [P] [US2] Implement explicit-recipient enforcement for new outbound mail and attachment-reference validation that blocks final send when required references are unresolved in `daemon/internal/mail/manager.go` and `daemon/internal/api/mail.go`
- [x] T026 [P] [US2] Implement reply and forward behavior with preserved source message and thread linkage in `daemon/internal/mail/manager.go`, `daemon/internal/mail/types.go`, and `daemon/internal/mail/artifacts.go`
- [x] T027 [US2] Implement draft, send, reply, forward, and mail operation inspection route handlers in `daemon/internal/api/mail.go`, `daemon/internal/api/mail_execution.go`, and `daemon/internal/api/server.go`
- [x] T028 [US2] Persist mutation operation records and structured message, draft, and attachment metadata artifacts with truthful result modes in `daemon/internal/store/store.go` and `daemon/internal/mail/artifacts.go`
- [x] T029 [US2] Finalize create/update draft and outbound request schemas, operation schemas, and mail operation summary schema in `schemas/api/create-mail-draft.request.schema.json`, `schemas/api/update-mail-draft.request.schema.json`, `schemas/api/send-mail-message.request.schema.json`, `schemas/api/send-mail-draft.request.schema.json`, `schemas/api/reply-mail-message.request.schema.json`, `schemas/api/forward-mail-message.request.schema.json`, `schemas/api/mail-operation-resource.schema.json`, `schemas/api/mail-operation-list.response.schema.json`, and `schemas/api/mail-operation-summary.schema.json`
- [x] T030 [US2] Expose blocked-send, draft-only, sent, stale-state, and attachment-failure truth in `daemon/internal/api/types.go`, `daemon/internal/api/mail.go`, and `schemas/api/mail-operation-resource.schema.json`
- [x] T031 [US2] Publish outbound-operation, blocked-send, and artifact lifecycle events in `daemon/internal/mail/manager.go` and `schemas/events/mail-operation-completed.event.schema.json`, `schemas/events/mail-operation-failed.event.schema.json`, and `schemas/events/mail-artifact-recorded.event.schema.json`

**Checkpoint**: User Story 2 is complete when draft and outbound mail behavior is truthful, send-path-distinguishable, explicit-recipient-safe, and independently testable with blocked-send attachment cases.

---

## Phase 5: User Story 3 - Run Mail Work Through Shared Delivery (Priority: P3)

**Goal**: Scheduled and workflow-driven mail operations run through the normal runtime plane and route their terminal results through the shared delivery plane without collapsing mail, readiness, and delivery truth together.

**Independent Test**: Execute one scheduled or workflow-driven mail inspection or draft action, one background send attempt without explicit send permission, and one background send attempt with explicit send permission against the fake backend, then confirm runtime/workflow surfaces expose `mailOperationSummaries` and delivery outcomes link back to the correct mail operation truth.

### Tests for User Story 3

- [x] T032 [P] [US3] Add API and contract regressions for scheduled and workflow-driven mail actions, `allowSendSideEffects` gating, and delivery linkage in `daemon/internal/api/workflows_test.go`, `daemon/internal/api/schedules_test.go`, `daemon/internal/api/server_test.go`, and `daemon/internal/contracts/contracts_test.go`
- [x] T033 [P] [US3] Add restart and projection regressions for `mailOperationSummaries` on tool calls, workflow steps, schedule attempts, and delivery outcomes in `daemon/internal/app/app_test.go`, `daemon/internal/runtime/runtime_test.go`, `daemon/internal/store/store_test.go`, and `daemon/internal/api/mail_projection_test.go`

### Implementation for User Story 3

- [x] T034 [P] [US3] Add `mailAction` request and schema shapes plus explicit `allowSendSideEffects` workflow gating fields in `daemon/internal/api/types.go`, `schemas/api/mail-workflow-action.schema.json`, `schemas/api/create-workflow.request.schema.json`, and `schemas/api/create-schedule.request.schema.json`
- [x] T035 [P] [US3] Attach immutable `mailOperationSummaries` to tool calls, workflow steps, and schedule attempts in `daemon/internal/runtime/runtime.go`, `daemon/internal/api/types.go`, `schemas/api/tool-call-resource.schema.json`, `schemas/api/workflow-step-resource.schema.json`, and `schemas/api/schedule-attempt-resource.schema.json`
- [x] T036 [P] [US3] Implement background mail operation emission and send-permission gating from workflow and schedule execution paths in `daemon/internal/api/workflows.go`, `daemon/internal/api/schedule_workflow_launcher.go`, `daemon/internal/scheduler/scheduler.go`, and `daemon/internal/api/mail_execution.go`
- [x] T037 [US3] Link background mail outcomes to shared delivery outcomes without collapsing truth planes in `daemon/internal/api/mail.go`, `daemon/internal/delivery/linkage.go`, `daemon/internal/api/workflows.go`, and `schemas/api/delivery-outcome-resource.schema.json`
- [x] T038 [US3] Implement mail operation list/detail filtering by run, workflow, schedule, and delivery linkage in `daemon/internal/api/mail.go` and `daemon/internal/store/store.go`
- [x] T039 [US3] Extend fake-local mail support with deterministic background inspection, draft-only, allowed-send, and blocked-send scenarios in `daemon/internal/mail/fake_backend.go`, `daemon/internal/integrations/backend.go`, and `daemon/internal/integrations/fake_backend.go`

**Checkpoint**: User Story 3 is complete when background mail work is independently testable through workflow and schedule paths and its delivery linkage remains additive and inspectable.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Finish docs, schema fixtures, recorded verification, and rollback guidance for the full mail domain roadmap.

- [x] T040 [P] Update mail schema fixtures and validator coverage in `daemon/internal/contracts/contracts_test.go`, `daemon/internal/contracts/validator.go`, and all mail-facing schema files under `schemas/api/` and `schemas/events/`
- [x] T041 [P] Document the mail domain, send-path truth, background send gating, and truth-plane separation in `docs/runtime/daemon-roadmaps.md`, `docs/runtime/daemon-api-and-event-model.md`, `docs/runtime/operator-trust-model.md`, and `docs/harness/harness-architecture.md`
- [x] T042 [P] Update the upstream mail roadmap spec plus the feature-local contract and quickstart references to the concrete mail contract in `docs/specs/015-mail-integration.md`, `specs/015-mail-integration/contracts/mail-domain-surfaces.md`, and `specs/015-mail-integration/quickstart.md`
- [x] T043 [P] Run the manual `KURA_ENV=test` mail walkthrough with one explicit-`integrationId` read, one canonical-default read, one draft flow, one direct-send flow, one send-draft flow, one blocked-send attachment case, and one delivery-linked background run, then record observed results in `specs/015-mail-integration/quickstart.md`
- [x] T044 Record automated verification commands, residual risks, rollback notes, and the phase-30 verification procedure in `specs/015-mail-integration/plan.md` and `specs/015-mail-integration/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1: Setup**: No dependencies; start immediately.
- **Phase 2: Foundational**: Depends on Phase 1; blocks all story work.
- **Phase 3: US1**: Depends on Phase 2; establishes the MVP mailbox inspection surface.
- **Phase 4: US2**: Depends on Phase 2 and builds on the account projection and operation model from US1 to add truthful draft and outbound behavior.
- **Phase 5: US3**: Depends on Phase 2 and is easiest to close after US1 and US2 define stable mail operation, artifact, and send-path truth.
- **Phase 6: Polish**: Depends on all desired user stories being complete.

### User Story Dependencies

- **US1 (P1)**: Starts after Foundational; no dependency on other user stories.
- **US2 (P2)**: Builds on US1 mailbox projection and inspection truth but remains independently testable once draft and outbound operation logic exists.
- **US3 (P3)**: Builds on US1/US2 operation truth to project background mail execution onto workflow, schedule, and delivery surfaces.

### Within Each User Story

- Tests and contract coverage land before or alongside implementation and must fail before the story is considered complete.
- Store and type changes precede API projection work that depends on persisted truth.
- Mail manager behavior precedes route handlers and background execution wiring.
- Story-specific docs and recorded validation happen only after the corresponding behavior is functional.

### Parallel Opportunities

- Setup tasks marked `[P]` can run together.
- In Foundational, shared typing, store helpers, app wiring, fake backend extension, validator work, and projection-shape work can proceed in parallel.
- For each user story, API/contract tests and manager/store regressions can be written in parallel.
- Within each story, persistence and schema work can proceed in parallel with route wiring once the manager behavior is stable.

---

## Parallel Example: User Story 1

```bash
# Tests in parallel
Task: "T013 [US1] Add contract tests for mail account projection, explicit-integrationId route selection, canonical-default fallback, thread list/detail, message detail, and draft inspection routes plus mail.account_projected coverage in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
Task: "T014 [US1] Add manager and store regressions for explicit-integrationId selection, canonical-default fallback, mailbox projection, thread inspection, message inspection, and draft inspection non-mutation behavior in daemon/internal/mail/manager_test.go and daemon/internal/store/store_test.go"

# Implementation in parallel
Task: "T015 [US1] Implement mail account projection, explicit-integrationId read selection, canonical-default fallback, and fake backend read models in daemon/internal/mail/backend.go, daemon/internal/mail/fake_backend.go, and daemon/internal/mail/manager.go"
Task: "T016 [US1] Implement thread list/detail, message detail, and draft list/detail handling in daemon/internal/mail/manager.go and daemon/internal/mail/types.go"
```

## Parallel Example: User Story 2

```bash
# Tests in parallel
Task: "T021 [US2] Add contract and API regressions for draft create/update, direct send, send-existing-draft, reply, forward, and mail operation inspection routes in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
Task: "T022 [US2] Add manager and store regressions for stable draft identity, send-path truth, explicit-recipient enforcement, blocked-send behavior for unresolved attachments, and reply or forward linkage in daemon/internal/mail/manager_test.go and daemon/internal/store/store_test.go"

# Implementation in parallel
Task: "T024 [US2] Implement draft lifecycle plus direct-send and send-existing-draft logic with explicit send-path truth in daemon/internal/mail/manager.go and daemon/internal/mail/types.go"
Task: "T025 [US2] Implement explicit-recipient enforcement for new outbound mail and attachment-reference validation that blocks final send when required references are unresolved in daemon/internal/mail/manager.go and daemon/internal/api/mail.go"
```

## Parallel Example: User Story 3

```bash
# Tests in parallel
Task: "T032 [US3] Add API and contract regressions for scheduled and workflow-driven mail actions, allowSendSideEffects gating, and delivery linkage in daemon/internal/api/workflows_test.go, daemon/internal/api/schedules_test.go, daemon/internal/api/server_test.go, and daemon/internal/contracts/contracts_test.go"
Task: "T033 [US3] Add restart and projection regressions for mailOperationSummaries on tool calls, workflow steps, schedule attempts, and delivery outcomes in daemon/internal/app/app_test.go, daemon/internal/runtime/runtime_test.go, daemon/internal/store/store_test.go, and daemon/internal/api/mail_projection_test.go"

# Implementation in parallel
Task: "T034 [US3] Add mailAction request and schema shapes plus explicit allowSendSideEffects workflow gating fields in daemon/internal/api/types.go, schemas/api/mail-workflow-action.schema.json, schemas/api/create-workflow.request.schema.json, and schemas/api/create-schedule.request.schema.json"
Task: "T035 [US3] Attach immutable mailOperationSummaries to tool calls, workflow steps, and schedule attempts in daemon/internal/runtime/runtime.go, daemon/internal/api/types.go, schemas/api/tool-call-resource.schema.json, schemas/api/workflow-step-resource.schema.json, and schemas/api/schedule-attempt-resource.schema.json"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1.
4. Validate mailbox account projection, thread inspection, message inspection, and draft inspection independently in `KURA_ENV=test`.
5. Treat this as the first executable checkpoint only; roadmap 30 is not closed until US2, US3, and Phase 6 complete.

### Incremental Delivery

1. Land Setup + Foundational to establish the mail package, persistence, fake backend support, and contract scaffolding.
2. Deliver US1 for mailbox projection, thread inspection, message inspection, and draft inspection truth.
3. Deliver US2 for draft lifecycle, direct send, send-existing-draft, reply, forward, and blocked-send attachment truth.
4. Deliver US3 for background workflow or schedule execution and shared delivery linkage.
5. Finish with docs, schema fixtures, recorded manual verification, and rollback notes.

### Parallel Team Strategy

1. One engineer lands store, app wiring, validator scaffolding, and fake backend extension in Setup + Foundational.
2. After Foundational is complete:
   - Engineer A takes US1 mailbox projection and inspection routes.
   - Engineer B takes US2 draft, outbound send-path truth, and blocked-send rules.
   - Engineer C takes US3 workflow/schedule/delivery linkage once the shared operation model is stable.

## Notes

- `[P]` means the task can run in parallel because it targets different files or only depends on completed foundational work.
- Every user story has explicit tests because roadmap 30 changes operator-visible behavior and contract-backed surfaces.
- Existing integrations, runtime, workflow, schedule, delivery, and calendar behavior must remain backward compatible throughout implementation.
- Manual quickstart validation complements but does not replace API, store, runtime, contract, and restart coverage.
