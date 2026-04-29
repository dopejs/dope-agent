# Feature Specification: Billing, Quotas, And Usage Accounting

**Feature Branch**: `023-billing-quotas-usage`  
**Created**: 2026-04-28  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/023-billing-quotas-and-usage-accounting.md 完成 phase 38 的工作"

**Upstream authority**: `docs/specs/023-billing-quotas-and-usage-accounting.md` is the authoritative design document for this work (Roadmap 38). This specification translates that design into testable scenarios, requirements, and success criteria. Where the upstream document and this spec disagree, the upstream document wins and this spec must be updated.

## Clarifications

### Session 2026-04-28

- Q: What happens when an administrator lowers a tenant quota below current or reserved usage? → A: Lowered quota takes effect immediately; existing usage remains, and new quota-consuming work is denied until usage is within the new limit.
- Q: How should persisted storage and artifact byte quotas be accounted when actual size is only known after write? → A: Reserve a defensible byte estimate before write, commit actual bytes after write, and refund or adjust the difference.
- Q: What timezone anchors quota period reset boundaries? → A: All quota periods reset on UTC boundaries.
- Q: How long are billing and usage audit records retained? → A: Retain billing and usage audit records indefinitely unless an operator applies an explicit retention policy later.
- Q: What should restart recovery do when it cannot prove whether a pending reservation was consumed or refundable? → A: Mark the reservation operator-action-needed and deny duplicate work until resolved.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Tenant Owner Inspects Plan And Usage (Priority: P1)

As a tenant owner, I need to inspect my tenant's active plan, effective quotas, current usage, remaining allowance, period boundaries, and reset behavior so that I can understand whether planned work will be allowed before starting it.

**Why this priority**: Hosted tenants need predictable limits. Without clear plan and usage visibility, quota enforcement becomes surprising and creates support load even when the enforcement decision is correct.

**Independent Test**: Create a hosted tenant with a non-unlimited plan and seeded usage. Confirm a tenant owner can view the active plan, quota categories, usage consumed, reserved usage, remaining allowance, period end, carryover behavior, and any manual adjustments without seeing another tenant's billing or usage state.

**Acceptance Scenarios**:

1. **Given** a tenant has an active plan with multiple quota categories, **When** a tenant owner inspects plan and usage, **Then** the owner sees the effective quota, consumed usage, reserved usage, remaining allowance, period boundary, reset behavior, and carryover rule for each category.
2. **Given** a quota period has reset, **When** the tenant owner inspects usage, **Then** the prior period is not counted against the new period except where the category explicitly allows carryover.
3. **Given** two tenants have similar plan names and usage profiles, **When** a tenant owner from tenant A inspects usage, **Then** only tenant A's plan, quota, usage, reservation, and adjustment information is visible.
4. **Given** a local-first installation uses the default development or unlimited plan, **When** the operator inspects plan and usage, **Then** the unlimited status and any non-enforced categories are explicit rather than silently absent.

---

### User Story 2 - Work Is Denied Before Costly Or Side-Effecting Consumption (Priority: P1)

As a hosted user starting runs, workflows, live validation, integration operations, or other expensive work, I need quota decisions to happen before the work begins so that over-limit work is denied consistently before it consumes resources or touches external systems.

**Why this priority**: Quotas are ineffective if denial happens after resource consumption or external side effects. This is the central safety requirement for hosted operation.

**Independent Test**: Configure a tenant with exhausted quota for each guarded category. Attempt to launch each guarded entry point and confirm the work is denied before resource consumption, external side effects, or durable workflow start, with a stable reason that tenant owners and operators can inspect.

**Acceptance Scenarios**:

1. **Given** a tenant has exhausted run launch quota, **When** a user attempts to start a run, **Then** the run is denied before launch and the response includes a stable quota-denial reason.
2. **Given** a tenant has insufficient live-validation allowance, **When** a user attempts live validation, **Then** validation is denied before external validation begins.
3. **Given** quota state cannot be safely determined for a hosted tenant, **When** a guarded hosted entry point is requested, **Then** the request is denied before expensive or side-effecting work starts.
4. **Given** a local-first development plan is configured as unlimited, **When** a guarded entry point is requested, **Then** the work is allowed without creating a false hosted denial.

---

### User Story 3 - Usage Accounting Survives Retry, Failure, Restart, And Concurrency (Priority: P1)

As an operator responsible for hosted reliability, I need usage reservations, commits, refunds, and adjustments to be idempotent and concurrency-safe so that retries, failures, restarts, and simultaneous launches cannot double-count usage or bypass tenant limits.

**Why this priority**: Billing and quota decisions affect availability and trust. Incorrect accounting under failure or concurrency can either overcharge tenants, strand quota, or allow tenants to exceed limits.

**Independent Test**: Exercise the same quota-affecting operation across retry, failure-before-consumption, cancellation, daemon restart, and concurrent launch scenarios. Confirm the tenant's final usage state contains exactly one correct outcome per operation and cannot exceed the effective quota.

**Acceptance Scenarios**:

1. **Given** a guarded operation is retried with the same operation identity, **When** the retry reaches usage accounting again, **Then** usage is not double-reserved or double-committed.
2. **Given** a guarded operation reserves usage and then fails before consuming the resource, **When** the failure is recorded, **Then** the reservation is released or refunded according to the quota category's rule.
3. **Given** multiple launch requests arrive concurrently for the last remaining quota unit, **When** the requests are evaluated, **Then** at most one request consumes the remaining quota and the others receive stable quota denials.
4. **Given** the daemon restarts while reservations are pending, **When** recovery completes, **Then** every pending reservation is either safely committed, released, or marked for operator action with audit-visible evidence.

---

### User Story 4 - Admin Adjusts Plans And Quotas With Audit Evidence (Priority: P2)

As an administrator, I need to assign plans, override effective quotas, and make manual usage adjustments with an audit trail so that hosted support can resolve entitlement changes and accounting corrections without hidden state changes.

**Why this priority**: Hosted plans and quotas must be operationally manageable. Support and compliance require visible evidence for plan changes, quota overrides, denials, reservations, commits, refunds, and manual adjustments.

**Independent Test**: Change a tenant's plan, apply a quota override, and perform a manual adjustment. Confirm effective quota projection changes as expected, usage decisions reflect the updated entitlement, and audit evidence records actor, tenant, resource category, reason, time, before/after values, and outcome.

**Acceptance Scenarios**:

1. **Given** an administrator changes a tenant's plan, **When** the change is saved, **Then** the tenant's effective quotas are recalculated and the plan-change audit record is visible.
2. **Given** an administrator applies a manual usage adjustment with a reason, **When** usage is inspected, **Then** the adjustment is reflected in the effective usage view and the audit trail preserves the reason and actor.
3. **Given** a user lacks plan or quota administration permission, **When** they attempt to change a tenant plan, quota override, or usage adjustment, **Then** the action is denied and no tenant usage state changes.
4. **Given** quota denial occurs, **When** an operator inspects the tenant's billing or usage evidence, **Then** the denial reason, category, period, tenant, and operation identity are available without parsing free-form logs.

---

### User Story 5 - Planning Covers Every Guarded Quota Category (Priority: P2)

As an implementation planner, I need a complete first quota catalog and enforcement matrix before coding starts so that run launches, workflow launches, runtime tool calls, live validation, integration operations, storage, and replay or evaluation attempts are consistently governed.

**Why this priority**: The upstream Roadmap 38 document requires quota catalog and enforcement matrix completeness. Missing a guarded entry point would create a bypass even if the accounting model is otherwise correct.

**Independent Test**: Review the implementation plan and verify that every required quota category has period, reset, carryover, reservation, commit, refund, operation identity, concurrency guard, denial shape, and test coverage defined, and that every guarded entry point has an enforcement row.

**Acceptance Scenarios**:

1. **Given** planning is complete, **When** the quota catalog is reviewed, **Then** each required quota category has a stable identifier, explicit unit, period, carryover rule, reservation point, commit point, refund point, operation identity shape, concurrency guard, denial shape, and required tests.
2. **Given** planning is complete, **When** the enforcement matrix is reviewed, **Then** each guarded entry point identifies tenant context source, quota categories touched, reservation amount, commit/refund transition, operation identity source, unavailable-state behavior, and tests for allowed, denied, retry, restart, and concurrent launch cases.
3. **Given** a newly identified expensive or side-effecting hosted entry point is added during implementation, **When** the readiness check runs, **Then** the work cannot be considered complete until the enforcement matrix includes that entry point or explicitly justifies why it is out of scope.

### Edge Cases

- Quota state is unavailable for a hosted tenant at the moment a guarded operation is requested.
- A quota period resets while a reservation is pending, committed, refunded, or awaiting operator action.
- Tenant-local timezone or daylight-saving changes occur near a quota reset; quota periods still reset only on UTC boundaries.
- A category allows carryover and the tenant has unused prior-period allowance near the carryover cap.
- A category does not allow carryover and prior-period usage or unused allowance must not affect the new period.
- Multiple quota categories apply to the same operation and one category allows the operation while another denies it.
- A retry repeats a request after the original operation already reserved, committed, refunded, or was denied.
- A daemon restart interrupts an in-flight reservation before the operation's final outcome is known; if recovery cannot prove whether the reservation was consumed or refundable, the reservation is marked operator-action-needed and duplicate work is denied until resolved.
- Concurrent launches attempt to consume the same remaining allowance.
- A failure occurs after reservation but before actual resource consumption.
- A cancellation occurs after partial consumption, requiring a category-specific commit or refund decision.
- An administrator lowers a tenant's quota below current or reserved usage; existing usage remains unchanged, the lowered quota takes effect immediately, and new quota-consuming work is denied until effective usage is within the new limit.
- An administrator raises a tenant's quota while denied work is being retried.
- A manual adjustment would make effective usage negative or otherwise inconsistent with the category unit.
- A local-first unlimited or development plan must remain explicit and must not accidentally use hosted fail-closed behavior.
- Usage for storage or artifacts is only partially measurable at the time a guarded operation begins; the system reserves a defensible byte estimate before write, commits actual bytes after write, and refunds or adjusts the difference. If actual bytes exceed the estimate and place the tenant over quota after the write, the actual bytes remain committed with audit-visible over-limit evidence and new quota-consuming work is denied until effective usage is within limit.
- Replay or evaluation campaign attempts are measurable in some contexts but not in others.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST persist tenant plans, quota definitions, quota periods, reset behavior, carryover rules, effective quota projections, usage counters, reservations, commits, refunds, denials, and manual adjustments for tenant-scoped hosted use.
- **FR-002**: The system MUST provide an explicit default development or unlimited plan for local-first installations so that non-hosted operators do not receive accidental hosted quota denials.
- **FR-003**: Tenant owners and authorized operators MUST be able to inspect a tenant's active plan, effective quotas, current usage, reserved usage, remaining allowance, period boundaries, reset behavior, carryover behavior, denials, and manual adjustments.
- **FR-004**: Plan, quota, usage, reservation, adjustment, and denial inspection MUST be tenant-scoped and MUST NOT expose another tenant's billing, quota, or usage state.
- **FR-005**: Authorized administrators MUST be able to assign tenant plans, change quota overrides, and apply manual usage adjustments with a required reason.
- **FR-005a**: When an administrator lowers a quota below current or reserved usage, the lower quota MUST take effect immediately, existing usage records MUST remain unchanged, and new quota-consuming work MUST be denied until effective usage is within the new limit.
- **FR-006**: Users without the canonical `billing.manage` plan and quota administration permission MUST be denied plan changes, quota overrides, reservation resolutions, and manual usage adjustments without changing tenant usage state.
- **FR-006a**: The identity contract MUST define `billing.manage` as the stable billing administration permission, grant it to owner/admin roles consistently with existing tenant administration policy, and exclude it from operator/viewer roles unless a later spec changes role semantics.
- **FR-007**: Every quota category MUST define a stable category identifier, explicit unit, period, reset behavior, carryover rule, reservation behavior, commit behavior, refund behavior, operation identity shape, concurrency safety requirement, and stable denial reason.
- **FR-008**: The initial quota catalog MUST cover run launches, workflow launches, runtime tool calls, live validation attempts, integration operations, persisted artifact or storage bytes where measurable, and replay or evaluation campaign attempts where measurable.
- **FR-008a**: Persisted artifact or storage byte quota categories MUST reserve a defensible byte estimate before write, commit actual bytes after write, and refund or adjust the difference between estimated and actual usage. When actual bytes exceed the reserved estimate, the system MUST commit the actual byte count, emit audit-visible over-limit evidence when the tenant is now over quota, and deny new quota-consuming work until effective usage is within limit.
- **FR-008b**: Roadmap 38 MUST define the `live_validation_attempts` quota category, operation identity, stable denial shape, reusable preflight gate, and enforcement matrix row without implementing the Roadmap 40 live-validation executor. If a concrete live-validation entry point already exists during implementation, it MUST be wired through the preflight gate before live side effects; if none exists, the matrix and tests MUST record the not-yet-mounted gate contract.
- **FR-009**: The system MUST enforce quotas before starting in-scope expensive or side-effecting hosted work.
- **FR-010**: The system MUST reserve usage before guarded work starts, commit actual usage after consumption, and refund or release reservations when work is denied, cancelled, retried, or fails before consuming the resource.
- **FR-011**: Usage accounting MUST be idempotent by stable operation identity so retries and daemon restarts cannot double-reserve, double-commit, double-refund, or bypass quota limits.
- **FR-012**: Concurrent quota checks MUST prevent multiple operations from consuming the same remaining allowance.
- **FR-013**: Hosted quota enforcement MUST deny guarded work before resource consumption when quota state cannot be safely determined.
- **FR-014**: Multi-category operations MUST be allowed only when every required quota category permits the reservation; if any category denies, the operation MUST NOT consume resources and MUST leave all category accounting consistent.
- **FR-015**: Quota period reset and carryover behavior MUST be deterministic and auditable for every quota category.
- **FR-015a**: All quota period reset boundaries MUST be anchored in UTC, regardless of tenant locale, operator locale, or daylight-saving changes.
- **FR-016**: Pending reservations after restart MUST recover into a safe committed, released, or operator-action-needed state with audit-visible evidence.
- **FR-016a**: If restart recovery cannot prove whether a pending reservation was consumed or refundable, the reservation MUST be marked operator-action-needed and duplicate quota-consuming work for the same operation MUST be denied until resolved.
- **FR-017**: Quota denials MUST use stable reason codes suitable for product, SDK, and operator handling without parsing free-form text.
- **FR-018**: The system MUST emit audit-visible records for quota denials, plan changes, quota overrides, reservations, commits, refunds, manual adjustments, and reservation recovery decisions.
- **FR-019**: Audit-visible billing and usage records MUST include tenant, actor where available, quota category, operation identity where applicable, period, amount, reason, timestamp, and outcome.
- **FR-019a**: Billing and usage audit records MUST be retained indefinitely unless an operator later applies an explicit retention policy.
- **FR-020**: The system MUST provide tenant usage and effective quota inspection surfaces that are stable enough for product UI, SDK, operator tooling, and contract validation.
- **FR-021**: The implementation plan MUST include a first quota catalog before coding starts and MUST make the FR-007 quota metadata checklist contract-testable for every category.
- **FR-022**: The implementation plan MUST include an enforcement matrix with one row per guarded entry point and MUST cover tenant context source, quota categories touched, reservation amount calculation, commit/refund transition, operation identity source, unavailable-state behavior, and tests for allowed, denied, retry, restart, and concurrent launch scenarios.
- **FR-023**: The feature MUST include tests for quota calculation, period reset, carryover, reservation, commit, refund, idempotent updates, manual adjustments, quota denial, concurrent launch enforcement, restart recovery, inspection response shape, and enforcement matrix completeness.
- **FR-024**: External payment-provider checkout, invoices, taxes, revenue recognition, provider-specific token metering, and cross-tenant pooled quota are out of scope for this phase unless a later specification explicitly brings them in.

### Key Entities

- **Tenant Plan**: A tenant-owned entitlement package that determines default quota definitions and whether the tenant is hosted, local development, unlimited, or otherwise constrained.
- **Quota Definition**: A durable rule for a quota category, including unit, period, reset behavior, carryover behavior, denial reason, and lifecycle expectations.
- **Effective Quota Projection**: The resolved quota view for a tenant after applying plan defaults, overrides, period state, carryover, reservations, usage, and manual adjustments.
- **Quota Period**: The time boundary over which a quota is measured, reset, and optionally carried forward.
- **Usage Counter**: Tenant-scoped consumed usage for a quota category and period.
- **Usage Reservation**: A tenant-scoped hold against available quota made before guarded work starts.
- **Usage Commit**: A finalized usage record that reflects actual consumption after guarded work succeeds or partially consumes a resource.
- **Usage Refund Or Release**: A record that returns reserved allowance or corrects usage when work is denied, cancelled, retried, or fails before consumption.
- **Manual Adjustment**: An administrator-created correction to usage or entitlement state with a required reason and audit evidence.
- **Operation Identity**: A stable identity for a quota-affecting operation that prevents duplicate accounting across retries and restarts.
- **Quota Denial**: A stable, tenant-scoped decision that prevents guarded work from starting because effective quota is insufficient or quota state is unavailable.
- **Billing Or Usage Audit Event**: Audit-visible evidence of plan changes, quota overrides, reservations, commits, refunds, manual adjustments, denials, and recovery decisions.
- **Billing And Usage Audit Retention Policy**: The retention rule for billing and usage audit records. The default is indefinite retention unless an operator later applies an explicit retention policy.
- **Quota Catalog**: The planning artifact that defines every initial quota category and its lifecycle, accounting, denial, and test expectations.
- **Enforcement Matrix**: The planning artifact that maps every guarded entry point to the quota categories and lifecycle behavior required before work can start.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: This phase adds tenant plan, quota, usage, billing inspection, denial, adjustment, and audit surfaces. It changes hosted launch behavior for guarded work because runs, workflows, live validation, integration operations, tool calls, storage or artifact growth where measurable, and replay or evaluation attempts where measurable may be denied before work starts.
- **Migration / Rollback**: Existing hosted tenants must receive an explicit plan before enforcement is enabled. Local-first installations must retain an explicit development or unlimited plan. Rollout should allow operators to verify effective quotas and usage projections before enabling fail-closed hosted enforcement. Rollback must preserve recorded usage and audit evidence while allowing operators to disable hosted enforcement through a documented plan state rather than deleting accounting records.
- **Verification Strategy**: Requires unit tests for quota calculation, period reset, carryover, idempotent reservation/commit/refund, manual adjustments, and concurrency; restart tests for pending reservation recovery; integration tests proving guarded entry points deny before consumption; contract tests for plan, usage, quota, and denial shapes; and a matrix completeness test for every in-scope entry point.
- **Observability Impact**: Adds audit-visible records for plan changes, quota overrides, reservations, commits, refunds, denials, manual adjustments, and recovery decisions. Operators must be able to explain quota denials from structured tenant, category, period, amount, operation identity, reason, timestamp, and outcome fields. Billing and usage audit records are retained indefinitely unless an operator later applies an explicit retention policy.
- **Environment & Secrets**: Development and verification must use the test environment by default. Live connectors and production tenants are not required for acceptance. No payment-provider credentials are required for this phase because external checkout, invoicing, taxes, and revenue recognition are out of scope.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of hosted tenants in verification have an explicit active plan and effective quota projection before quota enforcement is enabled.
- **SC-002**: Tenant owners can inspect plan, quota, usage, remaining allowance, period boundary, reset behavior, carryover behavior, reservations, denials, and manual adjustments for a seeded tenant in under 2 minutes without operator assistance.
- **SC-003**: 100% of covered over-quota guarded entry points deny work before resource consumption or external side effects begin.
- **SC-004**: 100% of covered retry and restart cases produce exactly one correct accounting outcome per operation identity with no double-reserve, double-commit, double-refund, or bypass.
- **SC-004a**: 100% of ambiguous restart recovery tests mark unresolved pending reservations operator-action-needed and deny duplicate quota-consuming work for the same operation until resolved.
- **SC-005**: 100% of covered concurrent-launch tests prevent more work from starting than the tenant's remaining effective quota allows.
- **SC-006**: 100% of covered failure-before-consumption and cancellation scenarios release, refund, commit, or mark reservations for operator action according to the quota category's rule.
- **SC-006a**: 100% of covered storage and artifact byte tests reserve an estimate before write, commit actual bytes after write, and refund or adjust any estimate-to-actual difference.
- **SC-006b**: 100% of quota period reset tests use UTC boundaries and are unaffected by tenant locale, operator locale, or daylight-saving changes.
- **SC-007**: 100% of quota categories in the initial catalog define unit, period, reset behavior, carryover rule, reservation behavior, commit behavior, refund behavior, operation identity shape, concurrency safety requirement, stable denial reason, and required tests.
- **SC-008**: 100% of guarded entry points in the enforcement matrix identify tenant context source, quota categories touched, reservation amount, commit/refund transition, operation identity source, unavailable-state behavior, and tests for allowed, denied, retry, restart, and concurrent launch cases.
- **SC-009**: 100% of plan changes, quota overrides, quota denials, reservations, commits, refunds, manual adjustments, and recovery decisions in verification produce audit-visible records with tenant, category, amount, reason, timestamp, outcome, and actor where available.
- **SC-009a**: 100% of lowered-quota tests preserve existing usage records, apply the lower quota immediately, and deny new quota-consuming work while effective usage exceeds the new limit.
- **SC-009b**: 100% of billing and usage audit retention checks confirm audit records remain available by default unless an explicit operator retention policy is applied.
- **SC-010**: Contract validation passes for tenant plan, effective quota, usage, quota denial, reservation, commit, refund, and manual-adjustment response or event shapes.
- **SC-011**: Local-first development or unlimited plan verification shows zero hosted fail-closed quota denials unless an operator explicitly configures finite quotas.
- **SC-012**: Operator smoke verification can explain an over-quota denial, a refunded failure-before-consumption, and a manual adjustment from structured inspection and audit evidence in under 15 minutes.

## Assumptions

- Roadmap 38 builds on completed tenant identity and access, tenant-scoped data migration, and hosted secrets and integration isolation work from Roadmaps 34, 35, and 37.
- The first version uses internal plans and quota enforcement; external payment-provider checkout, invoicing, tax, and revenue-recognition workflows are not part of this phase.
- Quota enforcement applies to hosted tenants by default and uses explicit development or unlimited plan behavior for local-first installations.
- Token-level or provider-specific billing-unit metering is out of scope unless a later clarification requires it.
- Cross-tenant pooled quota is out of scope; plan, usage, quota, reservation, and adjustment state is tenant-scoped.
- Storage, artifact, replay, and evaluation usage are enforced only where the usage is measurable with enough accuracy to support a defensible reservation, commit, or adjustment.
- Product UI, SDK, and operator tooling will consume the same stable plan, quota, usage, denial, and audit concepts, so response and event shapes need contract validation.
