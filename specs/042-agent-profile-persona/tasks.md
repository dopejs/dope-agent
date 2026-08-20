# Tasks: Agent Profile And Persona Configuration

**Input**: Design documents from `/specs/042-agent-profile-persona/`
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/agent-profile-persona.md](./contracts/agent-profile-persona.md), [quickstart.md](./quickstart.md)

**Tests**: Required by the project constitution for all production behavior changes. Write the relevant tests first and confirm they fail before implementing each story.

**Organization**: Tasks are grouped by user story so each story can be implemented and tested independently after the shared foundation is complete.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the Roadmap 57 implementation surfaces without changing behavior yet.

- [X] T001 Create profile domain package scaffolding in `daemon/internal/profiles/profile.go`, `daemon/internal/profiles/policy.go`, `daemon/internal/profiles/projection.go`, and `daemon/internal/profiles/redaction.go`
- [X] T002 [P] Create profile store scaffolding in `daemon/internal/store/profile_store.go` and `daemon/internal/store/profile_projection.go`
- [X] T003 [P] Create tenant-safe profile access scaffolding in `daemon/internal/store/tenancy/profiles.go`
- [X] T004 [P] Create profile API route scaffolding in `daemon/internal/api/agent_profiles.go`
- [X] T005 [P] Create profile event constructor scaffolding in `daemon/internal/events/agent_profiles.go`
- [X] T006 [P] Create Roadmap 57 SDK test placeholder in `sdk/ts/src/agent-profile-persona.test.ts`
- [X] T007 [P] Create Web profile feature directory and component placeholders in `web/src/features/agent-profiles/AgentProfileEditor.tsx` and `web/src/features/agent-profiles/AgentProfileHistory.tsx`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Add shared permissions, persistence, contracts, schemas, and redaction primitives required by every user story.

**CRITICAL**: No user story work can begin until this phase is complete.

- [X] T008 [P] Add permission tests for `profiles.inspect` and `profiles.manage` role grants and denials in `daemon/internal/identity/permissions_test.go`
- [X] T009 Add `profiles.inspect` and `profiles.manage` permission constants and role-derived grants in `daemon/internal/identity/types.go` and `daemon/internal/identity/permissions.go`
- [X] T010 [P] Add schema v53 migration tests for profile tables and indexes in `daemon/internal/store/profile_store_test.go`
- [X] T011 Add additive schema v53 registration for `agent_profiles`, `agent_profile_versions`, `agent_profile_active_selections`, `agent_profile_overlay_references`, and `agent_profile_runtime_projections` in `daemon/internal/store/store.go`
- [X] T012 [P] Add profile domain model tests for statuses, version change kinds, rollback eligibility, overlay validation states, and runtime projection resource kinds in `daemon/internal/profiles/profile_test.go`
- [X] T013 Define profile, version, active selection, overlay reference, runtime projection, audit event, status, reason code, validation state, and redaction types in `daemon/internal/profiles/profile.go`
- [X] T014 [P] Add profile validation and redaction tests for unsafe persona, provider, safety, overlay, and audit summaries in `daemon/internal/profiles/redaction_test.go`
- [X] T015 Implement profile validation and redaction helpers in `daemon/internal/profiles/policy.go` and `daemon/internal/profiles/redaction.go`
- [X] T016 [P] Add API JSON schemas for profile resources, versions, overlays, runtime projection, list response, create request, update request, activation request, and rollback request in `schemas/api/agent-profile-resource.schema.json`, `schemas/api/agent-profile-version-resource.schema.json`, `schemas/api/agent-profile-overlay-reference.schema.json`, `schemas/api/agent-profile-runtime-projection.schema.json`, `schemas/api/agent-profile-list.response.schema.json`, `schemas/api/create-agent-profile.request.schema.json`, `schemas/api/update-agent-profile.request.schema.json`, `schemas/api/agent-profile-activation.request.schema.json`, and `schemas/api/agent-profile-rollback.request.schema.json`
- [X] T017 [P] Add event JSON schemas for profile lifecycle, version creation, and runtime projection in `schemas/events/agent-profile-lifecycle.event.schema.json`, `schemas/events/agent-profile-version-created.event.schema.json`, and `schemas/events/agent-profile-runtime-projected.event.schema.json`
- [X] T018 Update tenant permission, thread detail, and run resource schemas for profile permissions and optional runtime profile projection in `schemas/api/tenant-permission-resource.schema.json`, `schemas/api/thread-detail.response.schema.json`, and `schemas/api/run-resource.schema.json`
- [X] T019 [P] Add Roadmap 57 contract validation tests in `daemon/internal/contracts/agent_profile_contracts_test.go`
- [X] T020 [P] Add base TypeScript SDK profile and runtime projection types in `sdk/ts/src/index.ts`
- [X] T021 [P] Add archive/disable retirement request and response schema coverage in `schemas/api/agent-profile-retirement.request.schema.json`, `schemas/api/agent-profile-retirement.response.schema.json`, and `schemas/api/agent-profile-resource.schema.json`
- [X] T022 [P] Add lifecycle event schema and fixture coverage for validation failure, permission denial, archive, disable, and safe-default fallback outcomes in `schemas/events/agent-profile-lifecycle.event.schema.json` and `schemas/events/README.md`

**Checkpoint**: Foundation ready. User story implementation can proceed in priority order or in parallel where files do not conflict.

---

## Phase 3: User Story 1 - Create, Edit, Archive, And Disable Agent Profiles (Priority: P1) MVP

**Goal**: Users can create, edit, archive, and disable tenant-owned structured profiles with identity, persona, provider defaults, safety defaults, version creation, no hard delete, and safe fallback when retiring the current tenant default.

**Independent Test**: Create a profile, edit it, archive or disable it, verify version/history and retirement evidence is created, verify the current tenant default falls back safely when retired, and verify unauthorized or cross-tenant users cannot see or mutate it.

### Tests for User Story 1

- [X] T023 [P] [US1] Add store tests for profile create, list, detail, update, version creation, tenant isolation, and permission-safe absence in `daemon/internal/store/profile_store_test.go`
- [X] T024 [P] [US1] Add API tests for `POST /v1/profiles`, `GET /v1/profiles`, `GET /v1/profiles/{profileId}`, and `PATCH /v1/profiles/{profileId}` in `daemon/internal/api/agent_profiles_test.go`
- [X] T025 [P] [US1] Add SDK tests for profile list, detail, create, update, and denial decoding in `sdk/ts/src/agent-profile-persona.test.ts`
- [X] T026 [P] [US1] Add Web tests for profile list, editor validation, save success, and permission denial states in `web/src/features/agent-profiles/agent-profile-editor.test.tsx`
- [X] T027 [P] [US1] Add TUI tests for profile list/detail output and denied profile inspection in `tui/src/cli.test.ts`
- [X] T028 [P] [US1] Add store and API tests for `POST /v1/profiles/{profileId}/archive`, `POST /v1/profiles/{profileId}/disable`, no hard delete, and retiring the current tenant-default profile in `daemon/internal/store/profile_store_test.go` and `daemon/internal/api/agent_profiles_test.go`
- [X] T029 [P] [US1] Add SDK, Web, and TUI tests for archive/disable actions, retired-profile inspection, denied retirement, and safe-default fallback display in `sdk/ts/src/agent-profile-persona.test.ts`, `web/src/features/agent-profiles/agent-profile-editor.test.tsx`, and `tui/src/cli.test.ts`
- [X] T030 [P] [US1] Add lifecycle event tests for validation failure, permission denial, archive, disable, retirement denial, and safe-default fallback outcomes in `daemon/internal/events/agent_profiles_test.go`

### Implementation for User Story 1

- [X] T031 [US1] Implement profile CRUD and version persistence in `daemon/internal/store/profile_store.go`
- [X] T032 [US1] Implement tenant-safe profile list, detail, create, and update accessors with `profiles.inspect` and `profiles.manage` gates in `daemon/internal/store/tenancy/profiles.go`
- [X] T033 [US1] Implement profile create, list, detail, and update handlers in `daemon/internal/api/agent_profiles.go`
- [X] T034 [US1] Register `/v1/profiles` and `/v1/profiles/` routes in `daemon/internal/api/server.go`
- [X] T035 [US1] Emit profile created and profile updated events in `daemon/internal/events/agent_profiles.go`
- [X] T036 [US1] Add profile SDK methods for list, detail, create, and update in `sdk/ts/src/index.ts`
- [X] T037 [US1] Implement Web profile list and editor flows in `web/src/features/agent-profiles/AgentProfileEditor.tsx` and `web/src/app/App.tsx`
- [X] T038 [US1] Implement TUI profile list and detail commands in `tui/src/cli.ts`
- [X] T039 [US1] Implement archive/disable state transitions, no-hard-delete enforcement, and current default retirement fallback in `daemon/internal/store/profile_store.go` and `daemon/internal/profiles/policy.go`
- [X] T040 [US1] Implement `POST /v1/profiles/{profileId}/archive` and `POST /v1/profiles/{profileId}/disable` handlers and route registration in `daemon/internal/api/agent_profiles.go` and `daemon/internal/api/server.go`
- [X] T041 [US1] Emit profile archived, profile disabled, retirement denied, and safe-default fallback events in `daemon/internal/events/agent_profiles.go`
- [X] T042 [US1] Add `archiveProfile` and `disableProfile` SDK methods plus Web/TUI retirement actions in `sdk/ts/src/index.ts`, `web/src/features/agent-profiles/AgentProfileEditor.tsx`, and `tui/src/cli.ts`
- [X] T043 [US1] Emit validation failure and permission denial lifecycle events from profile create, update, inspect, archive, disable, activation, and rollback paths in `daemon/internal/api/agent_profiles.go` and `daemon/internal/events/agent_profiles.go`

**Checkpoint**: User Story 1 is functional and independently testable as the MVP profile lifecycle slice.

---

## Phase 4: User Story 2 - Apply Active Profiles To Runtime Evidence (Priority: P1)

**Goal**: New work records the tenant-default active profile identity/version on threads, sessions, runs, workflows, and handoff destinations without rewriting historical evidence.

**Independent Test**: Activate a tenant-default profile, start representative work, change the active profile, and verify old runtime evidence keeps the original profile projection while new work uses the current tenant default.

### Tests for User Story 2

- [X] T044 [P] [US2] Add store tests for tenant-default active selection and runtime profile projection persistence in `daemon/internal/store/profile_projection_test.go`
- [X] T045 [P] [US2] Add restart recovery tests for active profile selection and runtime projection evidence in `daemon/internal/store/profile_restart_test.go`
- [X] T046 [P] [US2] Add API tests for profile activation and runtime profile projection in thread and run evidence in `daemon/internal/api/agent_profiles_test.go` and `daemon/internal/api/thread_lifecycle_test.go`
- [X] T047 [P] [US2] Add chat tests proving new work resolves the tenant-default active profile once at work start in `daemon/internal/chat/profile_projection_test.go`
- [X] T048 [P] [US2] Add SDK tests for activate profile and active profile projection types in `sdk/ts/src/agent-profile-persona.test.ts`
- [X] T049 [P] [US2] Add Web tests for tenant-default active selection and runtime profile evidence display in `web/src/features/agent-profiles/agent-profile-editor.test.tsx` and `web/src/features/thread-lifecycle.test.tsx`
- [X] T050 [P] [US2] Add runtime context assembly tests proving persona, provider defaults, and safety defaults are applied once at work start in `daemon/internal/chat/profile_projection_test.go` and `daemon/internal/providers/manager_test.go`
- [X] T051 [P] [US2] Add API tests for session, workflow-start, and handoff-destination profile projection creation and non-rewrite behavior in `daemon/internal/api/thread_lifecycle_test.go`, `daemon/internal/api/workflows_test.go`, and `daemon/internal/api/thread_handoff_test.go`

### Implementation for User Story 2

- [X] T052 [US2] Implement tenant-default active profile selection persistence in `daemon/internal/store/profile_store.go`
- [X] T053 [US2] Implement runtime profile projection persistence and lookup in `daemon/internal/store/profile_projection.go`
- [X] T054 [US2] Implement activation domain policy and scoped-binding rejection in `daemon/internal/profiles/policy.go`
- [X] T055 [US2] Implement `POST /v1/profiles/{profileId}/activate` in `daemon/internal/api/agent_profiles.go`
- [X] T056 [US2] Resolve and record tenant-default active profile projection when chat work starts in `daemon/internal/chat/service.go`
- [X] T057 [US2] Project active profile evidence into thread detail responses in `daemon/internal/threads/projection.go` and `daemon/internal/api/thread_lifecycle.go`
- [X] T058 [US2] Project active profile evidence into run and workflow response resources in `daemon/internal/api/server.go` and `daemon/internal/api/types.go`
- [X] T059 [US2] Project active profile evidence for handoff destinations in `daemon/internal/api/thread_handoff.go`
- [X] T060 [US2] Emit profile activated and runtime projected events in `daemon/internal/events/agent_profiles.go`
- [X] T061 [US2] Add activate profile SDK method and runtime projection types to `sdk/ts/src/index.ts`
- [X] T062 [US2] Implement Web tenant-default active profile selection and runtime evidence display in `web/src/features/agent-profiles/AgentProfileEditor.tsx` and `web/src/features/thread-lifecycle.tsx`
- [X] T063 [US2] Implement TUI active profile projection output for thread and run inspection in `tui/src/cli.ts`
- [X] T064 [US2] Apply resolved profile persona, provider defaults, and safety defaults to runtime context/provider resolution without mutating profile state in `daemon/internal/chat/service.go` and `daemon/internal/providers/manager.go`
- [X] T065 [US2] Persist runtime profile projection rows for sessions, workflow starts, and handoff destinations in `daemon/internal/store/profile_projection.go` and `daemon/internal/api/workflows.go`
- [X] T066 [US2] Expose session, workflow-start, and handoff profile projection evidence through authorized API responses in `daemon/internal/api/thread_lifecycle.go`, `daemon/internal/api/workflows.go`, and `daemon/internal/api/thread_handoff.go`

**Checkpoint**: User Story 2 is independently testable with active profile evidence on new runtime work.

---

## Phase 5: User Story 3 - Inspect Profile History And Roll Back Changes (Priority: P1)

**Goal**: Users and operators can inspect retained profile versions and roll back to an eligible prior version with audit evidence and fail-closed behavior.

**Independent Test**: Make multiple changes, inspect version history, roll back to an eligible version, and verify invalid rollback or audit-write failure leaves active profile state unchanged.

### Tests for User Story 3

- [X] T067 [P] [US3] Add domain tests for rollback eligibility, invalid provider, invalid overlay, archived profile, disabled profile, and fail-closed outcomes in `daemon/internal/profiles/policy_test.go`
- [X] T068 [P] [US3] Add store tests for retained versions, rollback-created versions, and immutable history in `daemon/internal/store/profile_store_test.go`
- [X] T069 [P] [US3] Add API tests for `GET /v1/profiles/{profileId}/versions` and `POST /v1/profiles/{profileId}/rollback` in `daemon/internal/api/agent_profiles_test.go`
- [X] T070 [P] [US3] Add event tests for rollback requested, rollback succeeded, rollback denied, and audit failed closed in `daemon/internal/events/agent_profiles_test.go`
- [X] T071 [P] [US3] Add Web and SDK tests for profile history and rollback states in `web/src/features/agent-profiles/agent-profile-editor.test.tsx` and `sdk/ts/src/agent-profile-persona.test.ts`

### Implementation for User Story 3

- [X] T072 [US3] Implement version history queries and rollback version creation in `daemon/internal/store/profile_store.go`
- [X] T073 [US3] Implement rollback validation and fail-closed state transition rules in `daemon/internal/profiles/policy.go`
- [X] T074 [US3] Implement version history and rollback handlers in `daemon/internal/api/agent_profiles.go`
- [X] T075 [US3] Emit profile rollback requested, succeeded, denied, and audit failed closed events in `daemon/internal/events/agent_profiles.go`
- [X] T076 [US3] Add profile history and rollback SDK methods in `sdk/ts/src/index.ts`
- [X] T077 [US3] Implement Web profile history and rollback UI in `web/src/features/agent-profiles/AgentProfileHistory.tsx`
- [X] T078 [US3] Implement TUI profile history and rollback command output in `tui/src/cli.ts`

**Checkpoint**: User Story 3 is independently testable as the rollback and behavior-forensics slice.

---

## Phase 6: User Story 4 - Manage Overlay References Explicitly (Priority: P2)

**Goal**: Users can attach prompt/workspace/config overlays as explicit validated references while structured profile configuration remains the primary truth.

**Independent Test**: Attach, update, remove, and inspect overlay references, then verify missing, inaccessible, unsafe, oversized, or legacy partial overlays are visible and cannot silently affect new work.

### Tests for User Story 4

- [X] T079 [P] [US4] Add overlay validation domain tests for valid, partial, missing, permission denied, out of scope, too large, unsafe content, and redaction failed states in `daemon/internal/profiles/redaction_test.go`
- [X] T080 [P] [US4] Add store tests for overlay reference persistence, version linkage, and partial legacy mapping evidence in `daemon/internal/store/profile_store_test.go`
- [X] T081 [P] [US4] Add API tests for overlay reference create/update/remove through profile create and patch requests in `daemon/internal/api/agent_profiles_test.go`
- [X] T082 [P] [US4] Add default profile seeding tests for legacy prompt/config bridging in `daemon/internal/store/profile_store_test.go`
- [X] T083 [P] [US4] Add Web and SDK tests for overlay validation state display in `web/src/features/agent-profiles/agent-profile-editor.test.tsx` and `sdk/ts/src/agent-profile-persona.test.ts`

### Implementation for User Story 4

- [X] T084 [US4] Implement overlay reference validation, normalization, and safe display labels in `daemon/internal/profiles/redaction.go`
- [X] T085 [US4] Persist overlay references and version linkage in `daemon/internal/store/profile_store.go`
- [X] T086 [US4] Integrate overlay reference mutation into profile create and update handlers in `daemon/internal/api/agent_profiles.go`
- [X] T087 [US4] Implement default profile seeding from provider defaults and safe prompt/config references in `daemon/internal/store/profile_store.go`
- [X] T088 [US4] Add partial legacy prompt/config mapping evidence and reason codes in `daemon/internal/profiles/profile.go`
- [X] T089 [US4] Add overlay reference SDK types and request support in `sdk/ts/src/index.ts`
- [X] T090 [US4] Implement Web overlay reference controls and validation badges in `web/src/features/agent-profiles/AgentProfileEditor.tsx`
- [X] T091 [US4] Implement TUI overlay reference inspection output in `tui/src/cli.ts`

**Checkpoint**: User Story 4 is independently testable with explicit overlay evidence and no hidden prompt-file truth.

---

## Phase 7: User Story 5 - Preserve Non-Memory Scope (Priority: P2)

**Goal**: Profile configuration remains explicit structured state and cannot become learned preference memory, semantic retrieval, agent self-mutation, skill generation, or autonomous collaboration.

**Independent Test**: Repeatedly express preferences in conversation and verify future work uses only explicit profile configuration and overlay references unless an authorized user changes the profile.

### Tests for User Story 5

- [X] T092 [P] [US5] Add domain tests proving conversation text cannot mutate profile state or rollback eligibility in `daemon/internal/profiles/policy_test.go`
- [X] T093 [P] [US5] Add chat tests proving repeated preferences do not create learned profile changes or memory-driven active selection in `daemon/internal/chat/profile_projection_test.go`
- [X] T094 [P] [US5] Add API tests proving no profile mutation occurs during chat/run/thread activity without `profiles.manage` in `daemon/internal/api/agent_profiles_test.go`
- [X] T095 [P] [US5] Add redaction tests proving profile summaries, overlay summaries, events, fixtures, and logs expose no secrets or unsafe overlay content in `daemon/internal/profiles/redaction_test.go`
- [X] T096 [P] [US5] Add Web/TUI label tests distinguishing profile configuration from memory and scoped binding in `web/src/features/agent-profiles/agent-profile-editor.test.tsx` and `tui/src/cli.test.ts`

### Implementation for User Story 5

- [X] T097 [US5] Enforce explicit-actor-only profile mutation and non-memory guardrails in `daemon/internal/profiles/policy.go`
- [X] T098 [US5] Ensure chat and runtime startup paths read profile selection without writing learned preferences in `daemon/internal/chat/service.go`
- [X] T099 [US5] Add non-memory and Roadmap 58 deferred-binding classifications to profile inspection evidence in `daemon/internal/profiles/projection.go`
- [X] T100 [US5] Harden profile, overlay, event, and runtime projection redaction in `daemon/internal/profiles/redaction.go`
- [X] T101 [US5] Update Web copy and labels to avoid memory terminology for profile configuration in `web/src/features/agent-profiles/AgentProfileEditor.tsx` and `web/src/features/agent-profiles/AgentProfileHistory.tsx`
- [X] T102 [US5] Update TUI copy and labels to avoid memory terminology for profile configuration in `tui/src/cli.ts`

**Checkpoint**: User Story 5 is independently testable as the non-memory and explicit-configuration safety boundary.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Finish compatibility, documentation, full verification, and release-readiness evidence.

- [X] T103 [P] Update runtime documentation for profile/persona configuration in `docs/runtime/agent-profile-persona.md`
- [X] T104 [P] Update thread lifecycle documentation for active profile projection in `docs/runtime/thread-session-lifecycle.md`
- [X] T105 [P] Update provider documentation for profile-owned provider defaults and existing provider preference compatibility in `docs/providers/provider-identity-and-profiles.md`
- [X] T106 [P] Update quickstart validation notes with final route names and command examples in `specs/042-agent-profile-persona/quickstart.md`
- [X] T107 [P] Add or update schema fixtures for profile API and event resources in `schemas/api/README.md` and `schemas/events/README.md`
- [X] T108 [P] Add an operator diagnostic walkthrough proving profile-version influence can be identified within 5 minutes using product evidence in `specs/042-agent-profile-persona/quickstart.md` and `docs/runtime/agent-profile-persona.md`
- [X] T109 [P] Update verification notes for archive/disable fallback, runtime persona/provider/safety application, session/workflow/handoff projection, and lifecycle event outcomes in `specs/042-agent-profile-persona/quickstart.md`
- [X] T110 Run schema and contract validation with `make daemon-contract-test` and record the result in `specs/042-agent-profile-persona/quickstart.md`
- [X] T111 Run full daemon tests with `go test ./...` from `daemon/` and record the result in `specs/042-agent-profile-persona/quickstart.md`
- [X] T112 Run client verification with `pnpm test:clients` and record the result in `specs/042-agent-profile-persona/quickstart.md`
- [X] T113 Run client build verification with `pnpm build` and record the result in `specs/042-agent-profile-persona/quickstart.md`
- [X] T114 Run `go mod tidy` from `daemon/` and inspect `daemon/go.mod` and `daemon/go.sum`
- [X] T115 Record residual risks, rollback path, and verification results in `specs/042-agent-profile-persona/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately.
- **Foundational (Phase 2)**: Depends on Setup completion; blocks all user stories.
- **User Story 1 (Phase 3)**: Depends on Foundational; recommended MVP.
- **User Story 2 (Phase 4)**: Depends on Foundational and can use US1 profile records; runtime projection remains independently testable with seeded profiles.
- **User Story 3 (Phase 5)**: Depends on Foundational and can use US1 version records; rollback remains independently testable with seeded versions.
- **User Story 4 (Phase 6)**: Depends on Foundational and integrates with US1 create/update flows.
- **User Story 5 (Phase 7)**: Depends on Foundational and can run after any story that creates profile runtime behavior; final validation should run after US1-US4.
- **Polish (Phase 8)**: Depends on completed target stories.

### User Story Dependencies

- **US1 Create, Edit, Archive, And Disable Agent Profiles**: First independently valuable slice after foundation.
- **US2 Apply Active Profiles To Runtime Evidence**: Can start after foundation with seeded profiles, but final integration expects US1 APIs.
- **US3 Inspect Profile History And Roll Back Changes**: Can start after foundation with seeded versions, but final integration expects US1 update flow.
- **US4 Manage Overlay References Explicitly**: Extends US1 profile create/update behavior and should be validated before final non-memory checks.
- **US5 Preserve Non-Memory Scope**: Cross-checks US1-US4 behavior and should be final among story phases.

### Within Each User Story

- Tests must be written and fail before implementation.
- Domain and store behavior before API handlers.
- API handlers before SDK/Web/TUI integration.
- Runtime projection writes before inspection projection.
- Story complete before moving to the next priority unless separate contributors own disjoint files.

## Parallel Opportunities

- Setup scaffolding tasks T002-T007 can run in parallel after T001.
- Foundational tests T008, T010, T012, T014, T016, T017, T019, T020, T021, and T022 touch different files and can run in parallel.
- In each user story, test tasks marked `[P]` can run in parallel before implementation.
- SDK, Web, and TUI tasks can run in parallel after the corresponding API contract is stable.
- US2 and US3 can be developed in parallel after foundation if they coordinate on `daemon/internal/store/profile_store.go`.
- US4 Web/SDK/TUI work can run in parallel with overlay domain/store work after request/response shapes are stable.

## Parallel Example: User Story 1

```text
Task: "T023 [P] [US1] Add store tests for profile create, list, detail, update, version creation, tenant isolation, and permission-safe absence in daemon/internal/store/profile_store_test.go"
Task: "T024 [P] [US1] Add API tests for POST /v1/profiles, GET /v1/profiles, GET /v1/profiles/{profileId}, and PATCH /v1/profiles/{profileId} in daemon/internal/api/agent_profiles_test.go"
Task: "T025 [P] [US1] Add SDK tests for profile list, detail, create, update, and denial decoding in sdk/ts/src/agent-profile-persona.test.ts"
Task: "T026 [P] [US1] Add Web tests for profile list, editor validation, save success, and permission denial states in web/src/features/agent-profiles/agent-profile-editor.test.tsx"
Task: "T027 [P] [US1] Add TUI tests for profile list/detail output and denied profile inspection in tui/src/cli.test.ts"
Task: "T028 [P] [US1] Add store and API tests for POST /v1/profiles/{profileId}/archive, POST /v1/profiles/{profileId}/disable, no hard delete, and retiring the current tenant-default profile in daemon/internal/store/profile_store_test.go and daemon/internal/api/agent_profiles_test.go"
Task: "T030 [P] [US1] Add lifecycle event tests for validation failure, permission denial, archive, disable, retirement denial, and safe-default fallback outcomes in daemon/internal/events/agent_profiles_test.go"
```

## Parallel Example: User Story 2

```text
Task: "T044 [P] [US2] Add store tests for tenant-default active selection and runtime profile projection persistence in daemon/internal/store/profile_projection_test.go"
Task: "T045 [P] [US2] Add restart recovery tests for active profile selection and runtime projection evidence in daemon/internal/store/profile_restart_test.go"
Task: "T047 [P] [US2] Add chat tests proving new work resolves the tenant-default active profile once at work start in daemon/internal/chat/profile_projection_test.go"
Task: "T048 [P] [US2] Add SDK tests for activate profile and active profile projection types in sdk/ts/src/agent-profile-persona.test.ts"
Task: "T050 [P] [US2] Add runtime context assembly tests proving persona, provider defaults, and safety defaults are applied once at work start in daemon/internal/chat/profile_projection_test.go and daemon/internal/providers/manager_test.go"
Task: "T051 [P] [US2] Add API tests for session, workflow-start, and handoff-destination profile projection creation and non-rewrite behavior in daemon/internal/api/thread_lifecycle_test.go, daemon/internal/api/workflows_test.go, and daemon/internal/api/thread_handoff_test.go"
```

## Parallel Example: User Story 4

```text
Task: "T079 [P] [US4] Add overlay validation domain tests for valid, partial, missing, permission denied, out of scope, too large, unsafe content, and redaction failed states in daemon/internal/profiles/redaction_test.go"
Task: "T080 [P] [US4] Add store tests for overlay reference persistence, version linkage, and partial legacy mapping evidence in daemon/internal/store/profile_store_test.go"
Task: "T083 [P] [US4] Add Web and SDK tests for overlay validation state display in web/src/features/agent-profiles/agent-profile-editor.test.tsx and sdk/ts/src/agent-profile-persona.test.ts"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1.
4. Stop and validate profile CRUD, tenant isolation, version creation, and client visibility.

### Roadmap Closure

Roadmap 57 is not complete until all five user stories, contracts, schemas, migration, archive/disable retirement, runtime profile application, runtime projection, redaction, restart recovery, SDK/Web/TUI surfaces, docs, and verification tasks are complete.

### Incremental Delivery

1. US1 creates structured profile truth and supports archive/disable retirement without hard delete.
2. US2 makes profile truth visible in runtime evidence.
3. US3 makes profile history and rollback operable.
4. US4 makes overlays explicit without hidden prompt truth.
5. US5 proves the feature remains explicit configuration and not memory.

## Notes

- `[P]` tasks touch different files or can be executed without waiting on incomplete implementation tasks.
- `[US#]` labels map tasks to user stories in [spec.md](./spec.md).
- Every production behavior task includes or depends on targeted tests.
- Keep changes additive and rollback-safe; do not hard-delete profile evidence or rewrite historical runtime records.
- Default all local validation to `~/.kura-test`; do not use live connectors or production tenants for this roadmap.
