---
description: "Task list for Roadmap 36 — Tenant-Aware Operator Shell And SDK"
---

# Tasks: Tenant-Aware Operator Shell And SDK

**Input**: Design documents from `specs/021-tenant-aware-shell-sdk/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`

**Tests**: Required by the project constitution and the feature spec. Every behavior
change below includes targeted SDK, web, daemon, contract, build, or manual verification
tasks before the implementation is considered complete.

**Organization**: Tasks are grouped by user story so each story can be implemented and
verified as an independently testable increment. US1 and US2 are both P1 and together
form the operator-shell MVP. US3 closes the SDK tenant contract. US4 closes minimal
membership management.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: parallelizable with other marked tasks in the same phase because it touches
  different files and has no dependency on incomplete tasks
- **[Story]**: maps to user stories from `spec.md`
- Every task includes concrete repository paths

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Confirm the existing surfaces and prepare shared test fixtures used by
multiple story phases.

- [X] T001 Verify active feature context and record any mismatch in `specs/021-tenant-aware-shell-sdk/quickstart.md`: current branch must be `022-tenant-aware-shell-sdk`, `.specify/feature.json` must point to `specs/021-tenant-aware-shell-sdk`, and `AGENTS.md` must point to `specs/021-tenant-aware-shell-sdk/plan.md`.
- [X] T002 [P] Add tenant, auth/me, membership, and denial fixture builders to `sdk/ts/src/index.test.ts` for reuse by SDK tenant contract tests.
- [X] T003 [P] Add tenant, membership, projection, and delayed-response fixture builders to `web/src/app/App.test.tsx` for reuse by shell tenant-switch and membership tests.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Land failing tests for shared SDK/shell primitives first, then add the shared
SDK tenant plumbing and shell state slots required by all user stories.

**CRITICAL**: No user story phase should begin until this phase is complete.

### Foundational Tests

- [X] T004 [P] Add SDK test for default tenant, per-request override, omitted tenant, and stream tenant header resolution in `sdk/ts/src/index.test.ts`.
- [X] T005 [P] Add SDK test for tenant resource exports, tenant helper routes, and stable tenant denial metadata in `sdk/ts/src/index.test.ts`.
- [X] T006 [P] Add web test harness coverage for unresolved, active, stale, and denied active-tenant state transitions in `web/src/app/App.test.tsx`.
- [X] T007 [P] Add web test harness coverage proving action controls stay unavailable while active tenant resolution is pending in `web/src/app/App.test.tsx`.

### Foundational Implementation

- [X] T008 Add `TenantRequestOptions`, tenant resource exports, membership resource exports, principal resource exports, permission exports, token grant exports, `defaultTenantId`, and internal tenant resolution helpers to `sdk/ts/src/index.ts`.
- [X] T009 Update `requestJSON`, `streamEvents`, `streamChatQuery`, and `buildHeaders` in `sdk/ts/src/index.ts` so tenant intent is propagated through `X-Kura-Tenant-ID` only when a per-request override or client default is present.
- [X] T010 Add SDK wrappers for `getMe`, `listTenants`, and `getTenant` in `sdk/ts/src/index.ts` so the web shell can resolve identity and allowed tenants without raw fetch calls.
- [X] T011 Add active tenant, allowed tenant list, projection generation, denied state, and tenant selection preference state types to `web/src/app/App.tsx`.
- [X] T012 Add tenant switcher, stale projection, denied state, and membership panel style hooks to `web/src/styles.css` without changing existing non-tenant shell layout behavior.

**Checkpoint**: SDK can carry tenant intent and the shell has typed state slots for tenant-aware behavior.

---

## Phase 3: User Story 1 - Operate With A Visible Active Tenant (Priority: P1) MVP

**Goal**: The operator shell shows the active tenant, lists only allowed tenants, restores
the last selected tenant only after revalidation, and lets a multi-tenant user switch to
an allowed tenant.

**Independent Test**: Sign in with one personal tenant and then with personal plus
organization tenants. Confirm active tenant display, allowed tenant choices, revalidated
selection restore, allowed tenant switch, denied selection behavior, and blocked actions
before active tenant resolution.

### Tests for User Story 1

- [X] T013 [P] [US1] Add web test for first load with one personal tenant and no organization tenants in `web/src/app/App.test.tsx`.
- [X] T014 [P] [US1] Add web test for multi-tenant first load and personal-to-organization tenant switch in `web/src/app/App.test.tsx`.
- [X] T015 [P] [US1] Add web test for restoring last selected tenant only when it appears in the current allowed tenant list in `web/src/app/App.test.tsx`.
- [X] T016 [P] [US1] Add web test for denied tenant selection clearing shell state without falling back to global or previous-tenant data in `web/src/app/App.test.tsx`.
- [X] T017 [P] [US1] Add web test proving tenant-scoped action controls and detail controls are disabled or absent until active tenant resolution completes in `web/src/app/App.test.tsx`.
- [X] T018 [P] [US1] Add web test proving bootstrap loads identity and allowed tenants before any tenant-scoped projection request in `web/src/app/App.test.tsx`.

### Implementation for User Story 1

- [X] T019 [US1] Implement shell bootstrap order in `web/src/app/App.tsx`: authenticate, call SDK identity/tenant helpers, resolve active tenant, then load tenant-scoped projections.
- [X] T020 [US1] Implement tenant switcher rendering and allowed tenant selection in `web/src/app/App.tsx`.
- [X] T021 [US1] Implement tenant selection preference read/write with daemon URL and principal scoping in `web/src/app/App.tsx`.
- [X] T022 [US1] Implement denied tenant selection state and active tenant display text in `web/src/app/App.tsx`.
- [X] T023 [US1] Finalize tenant switcher, denied state, single-tenant, and action-disabled visual styling in `web/src/styles.css`.
- [X] T024 [US1] Update shell usage of `createKuraClient` in `web/src/app/App.tsx` so all tenant-scoped shell requests use the resolved active tenant through SDK options.

**Checkpoint**: User Story 1 is independently functional and testable through `pnpm --filter @kura/web test`.

---

## Phase 4: User Story 2 - Refresh Tenant-Scoped Operator Views Safely (Priority: P1)

**Goal**: Activity, diagnostics, approvals, onboarding, and evaluation projections reload
under the selected tenant while stale rows, detail panes, streams, and late responses from
the previous tenant are hidden or ignored.

**Independent Test**: Open shell views under tenant A, switch to tenant B, and verify all
views and detail panes clear or mark stale before tenant B data is shown. Delayed tenant A
responses must never render as current tenant B data, and projection refreshes should use
one concurrent active-tenant refresh batch after tenant resolution.

### Tests for User Story 2

- [X] T025 [P] [US2] Add web test proving activity, diagnostics, approvals, onboarding, and evaluation projections clear or mark stale during tenant switch in `web/src/app/App.test.tsx`.
- [X] T026 [P] [US2] Add web test proving an open detail pane is cleared, closed, or marked stale before new-tenant detail data is displayed in `web/src/app/App.test.tsx`.
- [X] T027 [P] [US2] Add web test using delayed promises to prove previous-tenant responses are ignored after tenant switch in `web/src/app/App.test.tsx`.
- [X] T028 [P] [US2] Add web test proving event stream subscriptions close and reopen under the new active tenant in `web/src/app/App.test.tsx`.
- [X] T029 [P] [US2] Add web test for active tenant access revocation: views clear, stable denied state appears, and explicit user selection is required in `web/src/app/App.test.tsx`.
- [X] T030 [P] [US2] Add web test proving tenant-scoped projections are dispatched as one concurrent refresh batch after active tenant resolution in `web/src/app/App.test.tsx`.

### Implementation for User Story 2

- [X] T031 [US2] Implement monotonic tenant refresh generation and stale response guards in `web/src/app/App.tsx`.
- [X] T032 [US2] Update `refreshShell` in `web/src/app/App.tsx` to tag every projection result with active tenant and generation before committing state.
- [X] T033 [US2] Update detail inspection, approval resolution, first useful actions, replay launch, and replay comparison handlers in `web/src/app/App.tsx` so in-flight work remains attributed to its starting tenant and stale responses are ignored.
- [X] T034 [US2] Update event stream lifecycle in `web/src/app/App.tsx` so streams are opened with active tenant intent and closed/reopened on tenant switch.
- [X] T035 [US2] Implement stable denied projection state and active-tenant revocation handling in `web/src/app/App.tsx`.
- [X] T036 [US2] Add stale, loading, denied, and cleared-detail visual states to `web/src/styles.css`.

**Checkpoint**: User Story 2 is independently functional and testable through `pnpm --filter @kura/web test`.

---

## Phase 5: User Story 3 - Use Tenant Intent Through The SDK (Priority: P2)

**Goal**: SDK callers can configure a default tenant, override tenant intent for exactly
one request, preserve server-default behavior when no tenant is configured, and handle
tenant authorization denials without parsing raw error text.

**Independent Test**: Instantiate SDK clients with no tenant, default tenant, and
per-request override. Verify header propagation, override isolation, stream behavior,
stable denial mapping, and exported tenant resource types.

### Tests for User Story 3

- [X] T037 [P] [US3] Add SDK test for default tenant header propagation on representative tenant-scoped requests in `sdk/ts/src/index.test.ts`.
- [X] T038 [P] [US3] Add SDK test proving per-request tenant override affects exactly one request and does not mutate the client default in `sdk/ts/src/index.test.ts`.
- [X] T039 [P] [US3] Add SDK test proving omitted tenant configuration preserves server-resolved default tenant behavior by omitting `X-Kura-Tenant-ID` in `sdk/ts/src/index.test.ts`.
- [X] T040 [P] [US3] Add SDK test proving `streamEvents` and `streamChatQuery` use the same tenant header resolution rules in `sdk/ts/src/index.test.ts`.
- [X] T041 [P] [US3] Add SDK test proving tenant authorization denials map to stable `KuraClientError` metadata without raw message parsing in `sdk/ts/src/index.test.ts`.
- [X] T042 [P] [US3] Add SDK test coverage for tenant resource, membership resource, principal resource, permission, denial, and token grant exports in `sdk/ts/src/index.test.ts`.

### Implementation for User Story 3

- [X] T043 [US3] Extend every tenant-scoped public SDK method in `sdk/ts/src/index.ts` with an optional trailing `TenantRequestOptions` argument while preserving existing call signatures.
- [X] T044 [US3] Implement stable tenant denial metadata on `KuraClientError` and `toClientError` in `sdk/ts/src/index.ts`.
- [X] T045 [US3] Implement tenant API query helpers and membership query helper URL construction in `sdk/ts/src/index.ts` according to `specs/021-tenant-aware-shell-sdk/contracts/sdk-tenant-contract.md`.
- [X] T046 [US3] Update generated SDK outputs in `sdk/ts/dist/index.js` and `sdk/ts/dist/index.d.ts` by running the package build after `sdk/ts/src/index.ts` changes.
- [X] T047 [US3] Update SDK README tenant examples in `sdk/README.md` to document default tenant, per-request override, and omitted-tenant default behavior.

**Checkpoint**: User Story 3 is independently functional and testable through `pnpm --filter @kura/client test`.

---

## Phase 6: User Story 4 - Inspect And Manage Tenant Memberships (Priority: P3)

**Goal**: Authorized users can inspect active-tenant memberships and update roles, while
unauthorized users see hidden or disabled controls. Last-owner changes are prevented,
failed role updates leave visible state unchanged, and successful changes leave
audit-visible state.

**Independent Test**: Sign in as viewer/operator/admin/owner. Confirm only users with
`tenant.manage` can update roles, successful updates show daemon-confirmed state and
audit-visible role-change details, and last-owner downgrade/removal is prevented.

### Tests for User Story 4

- [X] T048 [P] [US4] Add daemon API regression test for membership role-change audit-visible state including actor, active tenant, target member, old role, new role, and timestamp in `daemon/internal/api/tenant_identity_test.go`.
- [X] T049 [P] [US4] Add daemon API regression test for preventing last-owner role downgrade and membership removal in `daemon/internal/api/tenant_identity_test.go`.
- [X] T050 [P] [US4] Add SDK tests for `listMemberships`, `updateMembershipRole`, and optional `removeMembership` helper routes in `sdk/ts/src/index.test.ts`.
- [X] T051 [P] [US4] Add web test proving membership controls are hidden or disabled without `tenant.manage` in `web/src/app/App.test.tsx`.
- [X] T052 [P] [US4] Add web test proving authorized role update refreshes the visible membership row with daemon-confirmed state in `web/src/app/App.test.tsx`.
- [X] T053 [P] [US4] Add web test proving denied or failed role update preserves the previous visible role and shows a stable error state in `web/src/app/App.test.tsx`.
- [X] T054 [P] [US4] Add web test proving owner-only or empty organization membership state is distinct from loading and error states in `web/src/app/App.test.tsx`.

### Implementation for User Story 4

- [X] T055 [US4] Fill any missing membership role-change audit-visible state gap in `daemon/internal/api/tenants.go` or `daemon/internal/identity/` so successful role changes record actor, active tenant, target member, old role, new role, and timestamp.
- [X] T056 [US4] Fill any missing last-owner protection gap in `daemon/internal/api/tenants.go` or `daemon/internal/identity/` so role updates and removals cannot leave an organization tenant without an active owner.
- [X] T057 [US4] Add or update membership request/response schema coverage in `schemas/api/update-membership.request.schema.json`, `schemas/api/membership-resource.schema.json`, and `schemas/api/membership-list.response.schema.json` only if T055 or T056 changes an externally visible shape.
- [X] T058 [US4] Implement SDK membership helper methods in `sdk/ts/src/index.ts` for listing memberships and updating active-tenant member roles.
- [X] T059 [US4] Implement active-tenant membership panel state, loading, empty, denied, and role update flows in `web/src/app/App.tsx`.
- [X] T060 [US4] Gate membership role controls in `web/src/app/App.tsx` using active tenant permissions, especially `tenant.manage`.
- [X] T061 [US4] Ensure successful membership role updates in `web/src/app/App.tsx` display daemon-confirmed state and do not commit optimistic local role changes.
- [X] T062 [US4] Style membership table, disabled controls, empty state, pending update, and error state in `web/src/styles.css`.

**Checkpoint**: User Story 4 is independently functional and testable through daemon, SDK, and web tests.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final compatibility, documentation, generated output, and verification work
across all stories.

- [X] T063 [P] Update `web/README.md` with tenant-aware shell smoke-test notes, including single personal tenant, multi-tenant switch, and membership role update checks.
- [X] T064 [P] Update `specs/021-tenant-aware-shell-sdk/quickstart.md` with any final command or smoke-test changes discovered during implementation.
- [X] T065 [P] Run `make daemon-contract-test` from `/Users/John/Code/kura-agent` and fix any schema or contract drift in `schemas/api/`.
- [X] T066 [P] Run `pnpm test:clients` from `/Users/John/Code/kura-agent` and fix SDK/web/TUI client regressions in `sdk/ts/`, `web/`, or `tui/`.
- [X] T067 Run `pnpm build` from `/Users/John/Code/kura-agent` and fix generated client or web build failures in `sdk/ts/dist/` and `web/dist/`.
- [X] T068 Run `go test ./...` from `/Users/John/Code/kura-agent/daemon` if daemon files changed and fix failures in `daemon/internal/`.
- [X] T069 Run `go mod tidy` from `/Users/John/Code/kura-agent/daemon` after daemon-side changes and commit any legitimate `daemon/go.mod` or `daemon/go.sum` updates.
- [X] T070 Manually validate the smoke flow in `specs/021-tenant-aware-shell-sdk/quickstart.md` against the test daemon and record any unverified paths in the final implementation notes.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 Setup**: No dependencies.
- **Phase 2 Foundational**: Depends on Phase 1 and blocks all user stories.
- **US1 (Phase 3)**: Depends on Phase 2.
- **US2 (Phase 4)**: Depends on Phase 2 and can run after US1 tests are written; final validation expects US1 active tenant state to exist.
- **US3 (Phase 5)**: Depends on Phase 2 and can run in parallel with US1/US2 once SDK shared plumbing exists.
- **US4 (Phase 6)**: Depends on Phase 2 and SDK tenant helpers; can run in parallel with US2/US3 after membership helper tests are written.
- **Phase 7 Polish**: Depends on selected user stories being complete.

### User Story Dependencies

- **US1**: MVP tenant visibility and allowed tenant switching; no dependency on US2/US3/US4 after Phase 2.
- **US2**: Requires US1 active tenant state for full validation but can develop stale generation tests and handlers independently.
- **US3**: SDK contract can be verified independently with fetch mocks after Phase 2.
- **US4**: Membership UI can be verified independently with SDK mocks after Phase 2; daemon audit and last-owner protection are independently testable.

### Within Each User Story

- Write the story's test tasks before implementation tasks.
- SDK types and request helpers before web shell integration.
- Tenant state resolution before projection refresh.
- Membership daemon/API guard before relying on shell controls as user-facing affordance.
- Story checkpoint must pass before claiming that story complete.

---

## Parallel Opportunities

- T002 and T003 can run in parallel.
- Foundational tests T004-T007 can run in parallel.
- US1 tests T013-T018 can run in parallel.
- US2 tests T025-T030 can run in parallel.
- US3 tests T037-T042 can run in parallel.
- US4 tests T048-T054 can run in parallel across daemon, SDK, and web files.
- Polish checks T063-T066 can run in parallel once implementation is complete.

---

## Parallel Example: User Story 2

```text
Task: "Add web test proving projections clear or mark stale during tenant switch in web/src/app/App.test.tsx"
Task: "Add web test proving detail pane clears or marks stale before new-tenant data in web/src/app/App.test.tsx"
Task: "Add web test using delayed promises to prove previous-tenant responses are ignored in web/src/app/App.test.tsx"
Task: "Add web test proving event stream subscriptions close and reopen under new active tenant in web/src/app/App.test.tsx"
Task: "Add web test proving tenant-scoped projections are dispatched as one concurrent refresh batch in web/src/app/App.test.tsx"
```

## Parallel Example: User Story 3

```text
Task: "Add SDK test for default tenant header propagation in sdk/ts/src/index.test.ts"
Task: "Add SDK test proving per-request tenant override affects exactly one request in sdk/ts/src/index.test.ts"
Task: "Add SDK test proving omitted tenant preserves server default behavior in sdk/ts/src/index.test.ts"
Task: "Add SDK test proving stream requests use tenant header resolution in sdk/ts/src/index.test.ts"
Task: "Add SDK test proving tenant authorization denials map to stable metadata in sdk/ts/src/index.test.ts"
```

## Parallel Example: User Story 4

```text
Task: "Add daemon API regression test for membership role-change audit-visible state in daemon/internal/api/tenant_identity_test.go"
Task: "Add daemon API regression test for last-owner protection in daemon/internal/api/tenant_identity_test.go"
Task: "Add SDK tests for membership helper routes in sdk/ts/src/index.test.ts"
Task: "Add web test proving membership controls are hidden or disabled without tenant.manage in web/src/app/App.test.tsx"
Task: "Add web test proving authorized role update refreshes visible membership state in web/src/app/App.test.tsx"
```

---

## Implementation Strategy

### MVP First

1. Complete Phase 1 setup.
2. Complete Phase 2 foundational tests and shared SDK/shell primitives.
3. Complete US1 so users can see and switch active tenants.
4. Complete US2 so tenant-scoped projections are safe against stale or denied data.
5. Validate with web tests before starting membership management.

### Incremental Delivery

1. Add US1 tenant visibility and switcher.
2. Add US2 safe projection refresh and stale response protection.
3. Add US3 complete SDK tenant contract.
4. Add US4 membership inspection, role management, audit-visible state, and last-owner protection.
5. Run full verification and manual smoke flow.

### Parallel Team Strategy

After Phase 2:

- Developer A: US1 tenant switcher and persisted selection.
- Developer B: US2 stale generation, stream lifecycle, and denied states.
- Developer C: US3 SDK contract and generated outputs.
- Developer D: US4 daemon audit/last-owner regression and membership UI.

---

## Notes

- Do not store access tokens in browser persistence for this roadmap.
- Do not add billing checkout, full organization administration, payment provider flows,
  native mobile behavior, or new daemon persistence migrations.
- Web shell must use SDK tenant request options rather than constructing raw tenant
  headers.
- Hidden or disabled membership controls are not authorization; daemon and SDK denials
  must remain stable and test-covered.
- Membership role changes must leave audit-visible state, not only UI-visible state.
- Commit after each completed story or coherent task group.
