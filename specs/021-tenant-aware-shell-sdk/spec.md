# Feature Specification: Tenant-Aware Operator Shell And SDK

**Feature Branch**: `022-tenant-aware-shell-sdk`  
**Created**: 2026-04-26  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/021-tenant-aware-operator-shell-and-sdk.md 完成 phase 36 的工作"

**Upstream authority**: `docs/specs/021-tenant-aware-operator-shell-and-sdk.md` is the authoritative design document for this work (Roadmap 36). This specification translates that design into testable scenarios, requirements, and success criteria. Where the upstream document and this spec disagree, the upstream document wins and this spec must be updated.

## Clarifications

### Session 2026-04-26

- Q: What should happen when the active tenant access is revoked during an operator shell session? → A: Show a stable denied state, clear tenant-scoped views, and require user action to choose another allowed tenant.
- Q: How should tenant selection behave across shell reloads or new shell sessions? → A: Restore the last selected tenant only if it is still allowed; otherwise show the default allowed tenant or a denied selection state.
- Q: Should membership management allow role changes or removals that leave an organization tenant with no active owner? → A: Prevent role changes or removals that would leave an organization tenant with no active owner.
- Q: How should in-flight work behave when the user switches tenants? → A: In-flight work remains attributed to the tenant it started under; stale responses from the previous tenant are ignored after switch.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Operate With A Visible Active Tenant (Priority: P1)

As a user who can belong to one or more tenants, I need the operator shell to show the active tenant and the tenants I am allowed to use so I can launch runs, inspect diagnostics, resolve approvals, and review evaluations under the intended tenant.

**Why this priority**: Tenant isolation is unsafe to operate if users cannot tell which tenant they are acting in. The shell must make tenant context visible before users perform tenant-scoped work.

**Independent Test**: Sign in as a user with exactly one personal tenant, then as a user with a personal tenant and an organization tenant. Confirm the shell shows the correct active tenant, lists only allowed tenants, and lets the multi-tenant user switch to an allowed tenant.

**Acceptance Scenarios**:

1. **Given** a user has one personal tenant and no organization tenants, **When** the operator shell first loads, **Then** the personal tenant is shown as active and no organization tenant options are shown.
2. **Given** a user has a personal tenant and one or more organization tenants, **When** the operator shell first loads, **Then** the active tenant and all allowed tenant choices are visible.
3. **Given** a user selects an allowed organization tenant, **When** the switch completes, **Then** subsequent shell actions are associated with the selected tenant and the active tenant display updates.
4. **Given** a user previously selected a tenant, **When** the shell reloads, **Then** the previous selection is restored only if the tenant is still allowed.
5. **Given** a user is not allowed to access a tenant, **When** the user attempts to select or use that tenant, **Then** the shell shows a stable authorization state without falling back to global data or previously visible tenant data.

---

### User Story 2 - Refresh Tenant-Scoped Operator Views Safely (Priority: P1)

As an operator using activity, diagnostics, approvals, evaluation, or onboarding views, I need each view to refresh under the newly selected tenant and hide stale details during the switch so I never act on rows from another tenant by mistake.

**Why this priority**: The shell can show sensitive operational state. Stale detail panes or rows from a previous tenant would create cross-tenant confusion and possible data leakage.

**Independent Test**: Open each tenant-scoped view with tenant A selected, switch to tenant B, and verify that rows and detail panes are cleared, marked stale, or refreshed before tenant B data is shown. Confirm tenant A rows never remain visible after the tenant switch completes.

**Acceptance Scenarios**:

1. **Given** tenant A has visible activity rows and tenant B has different activity rows, **When** the user switches from tenant A to tenant B, **Then** tenant A rows are cleared or marked stale before tenant B rows appear.
2. **Given** a detail pane is open for a tenant A approval, diagnostic item, onboarding state, or evaluation record, **When** the user switches to tenant B, **Then** the detail pane is cleared, closed, or marked stale before any tenant B details are loaded.
3. **Given** a tenant switch is in progress, **When** any tenant-scoped projection is still refreshing, **Then** the shell does not show previous-tenant rows as current data.
4. **Given** a request or action started under tenant A, **When** the user switches to tenant B before it finishes, **Then** the work remains attributed to tenant A and any stale tenant A response is not shown as current tenant B data.
5. **Given** the selected tenant returns an authorization denial for a tenant-scoped view, **When** the view loads, **Then** the user sees the denial state and no previous-tenant or global projection data is shown.

---

### User Story 3 - Use Tenant Intent Through The SDK (Priority: P2)

As an SDK caller, I need to set a default tenant and override the tenant for one request so automated clients can express tenant intent consistently without hand-building tenant transport details for every call.

**Why this priority**: Hosted automation depends on a stable client contract. If callers construct tenant selection inconsistently, tenant bugs become easy to introduce and hard to audit.

**Independent Test**: Configure a client with no tenant, with a default tenant, and with a one-request override. Verify the default tenant is used consistently, the override affects only the intended request, and later requests keep the original default.

**Acceptance Scenarios**:

1. **Given** an SDK caller configures a default tenant, **When** the caller makes tenant-scoped requests without per-request overrides, **Then** each request carries the configured tenant intent.
2. **Given** an SDK caller configures a default tenant, **When** the caller provides a one-request tenant override, **Then** only that request uses the override and subsequent requests return to the default tenant.
3. **Given** an SDK caller does not configure a tenant, **When** the caller makes a request that the server can resolve to a default tenant, **Then** the request succeeds under the server-resolved default tenant.
4. **Given** an SDK caller requests a tenant the principal is not allowed to access, **When** the request is made, **Then** the caller receives a stable tenant authorization denial that can be handled consistently.

---

### User Story 4 - Inspect And Manage Tenant Memberships (Priority: P3)

As an organization owner or authorized administrator, I need a minimal production-shaped membership surface so I can inspect members and correct role mistakes without using ad hoc operational procedures.

**Why this priority**: Organization tenants need basic membership correction to stay usable. This phase should not become a full administration suite, but it must expose enough control to operate safely.

**Independent Test**: Sign in as a viewer, operator, admin, and owner. Confirm only users with tenant management permission can inspect management controls and update roles, and that a successful role change is reflected in the member list and audit-visible state.

**Acceptance Scenarios**:

1. **Given** a user lacks tenant management permission, **When** the membership area is shown, **Then** membership management controls are hidden or disabled and role changes cannot be submitted.
2. **Given** an owner or authorized administrator opens membership management, **When** the member list loads, **Then** current members, roles, and membership states are visible for the active tenant.
3. **Given** an owner or authorized administrator changes a member role, **When** the change succeeds, **Then** the updated role is visible in the membership list and the resulting state is audit-visible.
4. **Given** a role change or removal would leave an organization tenant with no active owner, **When** an authorized user attempts the change, **Then** the change is prevented and the tenant keeps at least one active owner.
5. **Given** a role change is denied or fails, **When** the membership area refreshes, **Then** the previous role remains visible and the user receives a stable error state.

### Edge Cases

- First load for a single-tenant personal user must not require organization setup or show empty organization management affordances as required steps.
- Reloading the shell after a prior tenant selection must revalidate that selection against the current allowed tenant list; if it is no longer allowed, the shell must show the default allowed tenant or a denied selection state.
- A user with multiple allowed tenants may switch repeatedly; each switch must leave every tenant-scoped view associated with exactly one active tenant state.
- Tenant switch failure must leave the shell in a clear prior or denied state rather than a mixed state with rows from different tenants.
- In-flight work that started before a tenant switch must remain attributed to the tenant it started under, and stale responses from the previous tenant must be ignored after the switch.
- If the active tenant access is revoked during a shell session, the shell must show a stable denied state, clear tenant-scoped views, and require the user to choose another allowed tenant before resuming tenant-scoped work.
- Tenant-scoped activity, diagnostics, approvals, onboarding, and evaluation views must not show previous-tenant rows after the switch completes.
- A hidden or disabled membership control must not be the only protection for unauthorized role changes; denied membership changes must produce a stable authorization state.
- Membership management for an active tenant with no organization members beyond the owner must show a useful empty state rather than implying a loading or error condition.
- Membership management must prevent role changes or removals that would leave an organization tenant with no active owner.
- SDK per-request tenant override must not mutate the configured default tenant used by later requests.
- SDK calls without a configured tenant must preserve existing behavior by allowing server-side default tenant resolution where the server supports it.
- Tenant authorization denials must be stable enough for UI and SDK callers to handle without relying on raw error text.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The operator shell MUST display the active tenant wherever users perform tenant-scoped operational work.
- **FR-002**: The operator shell MUST list only tenants the current user is allowed to use and MUST support switching to an allowed tenant.
- **FR-003**: First load with one personal tenant and no organization tenants MUST present the personal tenant as active without requiring organization setup.
- **FR-003a**: The operator shell MUST restore the user's last selected tenant on reload only when that tenant is still in the current allowed tenant list; otherwise it MUST show the default allowed tenant or a denied selection state.
- **FR-004**: Tenant switch behavior MUST cause activity, diagnostics, approvals, onboarding, and evaluation views to refresh under the new active tenant.
- **FR-005**: Tenant switch behavior MUST clear, close, or visibly mark stale tenant-scoped detail panes before refreshed detail data for the new tenant is shown.
- **FR-006**: Tenant-scoped views MUST NOT show previous-tenant rows as current data after a tenant switch completes.
- **FR-006a**: Work or requests already started before a tenant switch MUST remain attributed to the tenant they started under, and stale responses from the previous tenant MUST NOT be rendered as current data after the switch.
- **FR-007**: Tenant authorization denials in the shell MUST show a stable denial state and MUST NOT fall back to global data, tenantless data, or previous-tenant data.
- **FR-007a**: If active tenant access is revoked during an operator shell session, the shell MUST clear tenant-scoped views, show a stable denied state, and require explicit user action to choose another allowed tenant.
- **FR-008**: Membership inspection MUST show active-tenant members, roles, and membership state to users with sufficient permission.
- **FR-009**: Membership management controls MUST be hidden or disabled for users without tenant management permission, and any submitted unauthorized membership change MUST be denied.
- **FR-010**: Authorized owners or administrators MUST be able to update member roles for the active tenant and see the resulting role state after the change.
- **FR-011**: Successful membership role changes MUST produce audit-visible state sufficient for operators to confirm who changed which member role, in which tenant, and when.
- **FR-011a**: Membership management MUST prevent role changes or removals that would leave an organization tenant with no active owner.
- **FR-012**: The SDK MUST support a client-level default tenant setting for tenant-scoped requests.
- **FR-013**: The SDK MUST support a per-request tenant override that applies only to the specified request and does not mutate the client-level default tenant.
- **FR-014**: SDK usage without a configured tenant override MUST continue to allow server-resolved default tenant behavior where available.
- **FR-015**: SDK callers MUST be able to express tenant intent without manually constructing tenant transport details for every request.
- **FR-016**: SDK tenant authorization denials MUST map to stable caller-visible error information that can be handled without parsing raw error text.
- **FR-017**: Shared client-facing representations MUST cover tenant, membership, principal, permission, denial, and token grant resources introduced by the tenant foundation.
- **FR-018**: The implementation plan MUST include acceptance coverage for tenant switch behavior, scoped projection refresh, denied tenant access, membership permission gating, role update results, SDK default tenant behavior, and SDK per-request override behavior.
- **FR-019**: The feature MUST preserve existing SDK usage that relies on server-resolved default tenant behavior unless a caller explicitly configures or overrides tenant intent.
- **FR-020**: Billing UI, payment checkout, full organization administration, and native mobile tenant switching MUST remain out of scope for this phase.

### Key Entities

- **Tenant**: An ownership boundary a user can operate in. A tenant may be personal or organization-scoped and appears as a selectable context only when the user is allowed to access it.
- **Active Tenant**: The tenant currently selected in the shell or expressed by an SDK request. Tenant-scoped views and actions use this context.
- **Allowed Tenant List**: The set of tenants available to the current user, including the user's personal tenant and any organization tenants granted by membership and token authority.
- **Tenant-Scoped Projection**: Any operator-facing activity, diagnostics, approvals, onboarding, or evaluation view whose rows and details must be filtered to the active tenant.
- **Membership**: A user's relationship to an organization tenant, including role and current membership state.
- **Role**: The permission-bearing assignment shown and changed through the minimal membership management surface.
- **Tenant Authorization Denial**: A stable denied-access outcome shown to shell users and SDK callers when tenant access or tenant management is not allowed.
- **SDK Tenant Configuration**: The caller-visible tenant intent used by automated clients, including a client-level default tenant and a one-request override.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Adds visible tenant-selection and membership surfaces to the operator shell and tenant intent support to the SDK. Existing SDK usage without explicit tenant configuration continues to rely on server-resolved default tenant behavior. Client-facing representations for tenant, membership, principal, permission, denial, and token grant resources may be added or reused from the tenant foundation.
- **Migration / Rollback**: No data migration is introduced by this phase. Rollback should remove or disable the tenant switcher, membership management surface, and SDK tenant configuration behavior while preserving existing server-resolved default tenant operation.
- **Verification Strategy**: Requires shell tests for first load, tenant switching, scoped projection refresh, stale detail clearing, denied tenant states, membership permission gating, and role update results. Requires SDK tests for default tenant configuration, per-request override behavior, no-tenant default resolution, stable denial mapping, and type coverage for tenant-related resources.
- **Observability Impact**: Membership role changes must be audit-visible. Tenant switch denials and membership denials must remain diagnosable through stable denial states without exposing inaccessible tenant data. Shell refresh behavior should make stale or denied states observable to users rather than silently retaining old rows.
- **Environment & Secrets**: Development and verification must use the test environment by default. This phase does not require live connectors, payment provider access, billing secrets, or native mobile credentials.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of tested tenant-scoped shell views display the active tenant context before a user can act on visible rows or detail panes.
- **SC-002**: In tenant switch verification, 100% of activity, diagnostics, approvals, onboarding, and evaluation views refresh under the selected tenant and show zero previous-tenant rows after the switch completes.
- **SC-003**: In stale-detail regression coverage, 100% of open tenant-scoped detail panes are cleared, closed, or marked stale before data for the newly selected tenant is displayed.
- **SC-003a**: In-flight tenant switch tests confirm that 100% of stale responses from a previous tenant are ignored after the switch and are never shown as current data for the new tenant.
- **SC-004**: A first-load test for a user with one personal tenant and no organization tenants completes without requiring organization setup and shows the personal tenant as active.
- **SC-004a**: Tenant selection persistence tests confirm that 100% of restored tenant selections are revalidated against the current allowed tenant list before tenant-scoped data is shown.
- **SC-005**: Unauthorized tenant selection and unauthorized membership management attempts produce stable denial states in 100% of covered cases and do not fall back to global or previous-tenant data.
- **SC-005a**: Revoking the active tenant during a shell session clears tenant-scoped views and requires explicit user selection of another allowed tenant in 100% of covered revocation scenarios.
- **SC-006**: Authorized owner or administrator role changes are reflected in the active tenant membership list and audit-visible state in every covered role-change scenario.
- **SC-006a**: Last-owner protection tests confirm that 100% of attempted role changes or removals that would leave an organization tenant without an active owner are prevented.
- **SC-007**: SDK default tenant tests show that 100% of tenant-scoped requests use the configured default tenant when no per-request override is supplied.
- **SC-008**: SDK override tests show that a per-request tenant override affects exactly one request and does not change the default tenant used by subsequent requests.
- **SC-009**: SDK compatibility tests show that callers without explicit tenant configuration can still use server-resolved default tenant behavior.
- **SC-010**: Tenant-related client-facing representations cover tenant, membership, principal, permission, denial, and token grant resources in contract or type verification.

## Assumptions

- Roadmap 36 consumes the tenant identity and access foundation from Roadmap 34 and the tenant-scoped data behavior from Roadmap 35.
- Roadmap 36 consumes the existing operator shell and onboarding surface from Roadmap 32 rather than creating a separate administration application.
- "Operator projections" in this phase means activity, diagnostics, approvals, onboarding, and evaluation surfaces named by the upstream document.
- Membership management is intentionally minimal: inspect members, see roles and state, and change roles for authorized users.
- Billing UI is limited to links or read-only quota placeholders if present elsewhere; payment checkout and full billing administration remain out of scope.
- Native mobile tenant switching and a full organization administration suite remain out of scope.
- Existing tenant, membership, principal, permission, denial, and token grant concepts from the tenant foundation are reused rather than redefined by this phase.
