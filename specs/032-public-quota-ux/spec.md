# Feature Specification: Public Quota UX

**Feature Branch**: `032-public-quota-ux`  
**Created**: 2026-05-06  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/032-public-quota-abuse-and-billing-ux.md 完成 phase 47 的工作"

## Clarifications

### Session 2026-05-06

- Q: Which quota categories must appear in the public quota visibility surface? → A: All enforced quota categories from the existing quota catalog, grouped into user-readable sections.
- Q: What format must support evidence exports use? → A: Structured redacted JSON package.

### Session 2026-05-07

- Q: How much abuse restriction detail should tenant users see? → A: Show status, affected category, duration when available, and recovery action; hide detection signals and thresholds.
- Q: What usage history window must quota summaries show? → A: Current active quota period plus immediately previous completed period.
- Q: When should quota visibility show near-limit warnings? → A: At 80% consumed or below one category-defined typical operation remaining, whichever comes first.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Understand Current Limits And Usage (Priority: P1)

As a tenant user or tenant owner, I can see the active tenant's current plan, all enforced quota categories grouped into readable sections, usage for the current active quota period and immediately previous completed period, reset timing, and whether each limit is currently available, near exhaustion, exhausted, unlimited, or temporarily restricted.

**Why this priority**: Users must understand quota status before starting work; otherwise valid enforcement appears as random product failure.

**Independent Test**: Can be tested by viewing a tenant with finite usage, exhausted usage, and unlimited development usage, then confirming the product clearly shows the correct plan, limits, current and previous-period usage, remaining allowance, and reset or recovery state without exposing another tenant's data.

**Acceptance Scenarios**:

1. **Given** a tenant with an enforced finite plan and usage in the current and immediately previous completed quota period, **When** an authorized user opens quota and usage visibility, **Then** the user sees the active plan, all enforced quota categories grouped into readable sections, current-period and previous-period consumed amounts, remaining amounts, reset timing where applicable, and any near-limit warnings triggered at 80% consumed or below one category-defined typical operation remaining, whichever comes first.
2. **Given** a tenant with an unlimited or not-measurable development plan, **When** an authorized user opens quota and usage visibility, **Then** the product identifies that limits are not currently blocking work and does not imply that payment checkout is available.
3. **Given** a user without permission to view billing or quota information, **When** the user attempts to access quota visibility, **Then** the product displays a stable authorization denial and does not reveal plan, usage, quota override, or denial details.

---

### User Story 2 - Explain Quota And Abuse Denials (Priority: P1)

As a user whose run, workflow, live validation, or integration action is blocked, I can inspect a denial explanation that identifies the source operation, stable denial reason, exhausted or restricted limit, and recovery actions available to me.

**Why this priority**: Denials must be visible before expensive or side-effecting work starts, and users need actionable next steps instead of treating enforcement as agent failure.

**Independent Test**: Can be tested by attempting work that is denied for ordinary quota exhaustion and work that is denied for a temporary abuse restriction, then confirming each denial is shown with the correct operation, reason, category, and recovery options.

**Acceptance Scenarios**:

1. **Given** an operation denied because a normal plan quota is exhausted, **When** the user opens the denial detail, **Then** the detail identifies the denied operation, limit category, denial reason, measured amount, remaining allowance, reset timing where relevant, and recommended recovery actions such as wait, reduce scope, request override, or contact support.
2. **Given** an operation denied because of a temporary abuse restriction, **When** the user opens the denial detail, **Then** the detail distinguishes the restriction from ordinary quota exhaustion and shows restriction status, affected category, duration when available, and recovery action without exposing detection signals or enforcement thresholds.
3. **Given** a denied operation for another tenant, **When** a user without access attempts to view the denial detail, **Then** the product denies access without leaking tenant, plan, operation, usage, or restriction details.

---

### User Story 3 - Make Plan Overrides And Restrictions Visible (Priority: P2)

As a tenant owner or administrator, I can see whether plan quotas are modified by tenant-specific overrides or temporary restrictions so I can explain why the effective limit differs from the base plan.

**Why this priority**: Support and tenant owners need to reason about effective limits without changing the underlying usage-accounting semantics.

**Independent Test**: Can be tested by viewing tenants with no override, increased quota override, lowered quota override, and temporary restriction, then confirming the product shows the effective limit and explains why it differs from the base plan.

**Acceptance Scenarios**:

1. **Given** a tenant with a quota override, **When** an authorized owner or administrator views plan and quota details, **Then** the product shows both the base plan limit and the effective tenant limit with the override reason visible at an appropriate level of detail.
2. **Given** a tenant with a temporary abuse restriction, **When** an authorized owner or administrator views plan and quota details, **Then** the product shows the restriction separately from normal plan quota, identifies the affected category and duration when available, and communicates whether recovery is automatic, requires waiting, or requires support/operator action without exposing detection signals or enforcement thresholds.

---

### User Story 4 - Export Support Evidence For Disputes (Priority: P3)

As a support operator with permission, I can export a structured redacted JSON evidence package for a quota-related denial, including abuse-restriction denials, so support can resolve disputes without exposing secrets, cross-tenant data, or unrelated user content.

**Why this priority**: Public hosted operation needs operator-visible evidence for quota and abuse-limit denial disputes, but this can follow user-facing explanation once denials are understandable.

**Independent Test**: Can be tested by selecting an ordinary quota denial and an abuse-restriction denial, exporting support evidence for each denial, then confirming the structured redacted JSON package contains tenant-scoped billing evidence, denial metadata, relevant recovery state, redactions, and no unrelated content.

**Acceptance Scenarios**:

1. **Given** a support operator with `billing.evidence_export`, **When** the operator exports evidence for an ordinary quota denial or abuse-restriction denial, **Then** the structured redacted JSON package includes tenant identifier, affected operation reference, quota or restriction category, stable denial reason, relevant usage snapshot, recovery state, actor metadata where allowed, and redaction indicators.
2. **Given** a user or operator without `billing.evidence_export`, **When** they attempt to export quota evidence, **Then** the product denies the request and does not produce a partial export.

### Edge Cases

- Quota state is unavailable for a hosted tenant while a user attempts to launch work; the product must fail closed and communicate that quota status cannot be verified without starting side-effecting work.
- A tenant switches while quota or denial details are open; stale details from the previous tenant must be cleared, hidden, or clearly marked unavailable before new tenant data is shown.
- A quota reset occurs while a user is viewing usage; the next refresh must show the new current period and retain the immediately previous completed period without losing the historical denial explanation.
- An ambiguous pending reservation blocks duplicate work after restart recovery; the product must communicate that operator resolution is required rather than presenting it as ordinary quota exhaustion.
- Usage counters contain manual adjustments or carryover; summaries must make the effective result understandable without rewriting historical evidence.
- A category has low absolute quota where 80% consumed would warn too late; the product must warn when less than one typical operation remains. Typical operation amount is category-defined: count and attempt quotas use `1`, and byte quotas use the catalog's configured artifact-write reservation estimate.
- Structured redacted JSON support evidence must remain useful for disputes while excluding secrets, connector payloads, unrelated run content, and data from other tenants.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST expose tenant-scoped quota status that includes the active plan, all enforced quota categories from the existing quota catalog grouped into user-readable sections, consumed usage, remaining allowance, reset timing where applicable, and effective limit status.
- **FR-002**: The system MUST distinguish normal plan quotas, unlimited or not-measurable plans, tenant-specific quota overrides, and temporary abuse restrictions wherever the distinction affects user action.
- **FR-003**: Users MUST be able to inspect quota-relevant usage for the current active quota period and immediately previous completed period for the active tenant without seeing another tenant's usage.
- **FR-004**: The system MUST show quota denial details for denied runs, workflows, live validation attempts, integration operations, and other guarded work before suggesting that work has started.
- **FR-005**: Every denial detail MUST identify the source operation, stable denial reason, quota or restriction category, relevant measured amount, and recommended recovery actions.
- **FR-006**: Abuse-limit or temporary restriction messages MUST be visibly separate from ordinary quota exhaustion and MUST communicate status, affected category, duration when available, and whether the user should wait, reduce scope, request an override, contact support, or wait for operator resolution, without exposing detection signals or enforcement thresholds.
- **FR-007**: Tenant owners and administrators MUST be able to view effective limits and any visible tenant-level quota override or restriction state when they have billing visibility permission.
- **FR-008**: Support operators with explicit permission MUST be able to export structured redacted JSON evidence from denial records for ordinary quota denials and abuse-restriction denials; standalone abuse-restriction exports without an associated denial are out of scope for this phase.
- **FR-009**: Evidence exports MUST include enough information to support a dispute: tenant reference, affected operation reference, quota category, stable denial reason, usage snapshot, effective limit state, recovery state, and redaction status.
- **FR-010**: Quota, denial, override, restriction, and evidence views MUST be permission-gated and MUST fail without leaking cross-tenant billing, usage, or operation details.
- **FR-011**: Tenant switching MUST clear or hide quota dashboard and denial details from the previous tenant before showing data for the new tenant.
- **FR-012**: The system MUST preserve existing usage-accounting semantics and billing evidence history; this feature adds product visibility and recovery guidance rather than a new accounting ledger or payment checkout flow.
- **FR-013**: The public client contract MUST expose quota status, usage summaries, denial details, and permission-denial states consistently enough for web shell and SDK consumers to present the same user-visible meanings.
- **FR-014**: Quota-related user-visible errors MUST be stable enough for tests, support playbooks, and SDK consumers to distinguish quota exhaustion, abuse restriction, unavailable quota state, unauthorized access, and pending operator resolution.
- **FR-015**: Near-limit warnings MUST appear when a finite quota category reaches 80% consumed or has less than one typical operation remaining, whichever occurs first; typical operation amount MUST be category-defined as `1` for count and attempt quotas and the catalog's configured artifact-write reservation estimate for byte quotas.
- **FR-016**: Temporary abuse restrictions MUST be represented by explicit additive billing abuse restriction records and audit evidence separate from normal plan quotas and tenant quota overrides.
- **FR-017**: Structured redacted JSON evidence export MUST require the canonical `billing.evidence_export` permission and MUST NOT be authorized by role-only checks or by `billing.view` alone.

### Key Entities

- **Tenant Quota Status**: The active tenant's current plan, all enforced quota categories grouped into user-readable sections, effective limits, usage, remaining allowance, reset timing, and status labels such as available, near limit, exhausted, unlimited, restricted, or unavailable. Near-limit status is reached at 80% consumed or below one category-defined typical operation remaining, whichever occurs first.
- **Usage Summary**: A tenant-scoped projection of measured consumption by quota category for the current active quota period and immediately previous completed period, including adjustments or carryover where they affect the effective result.
- **Quota Denial**: A product-visible explanation of blocked work, tied to an operation reference, quota category, stable denial reason, measured amount, and recovery state.
- **Abuse Restriction**: A temporary or operator-controlled additive billing record with audit evidence, separate from ordinary plan quota exhaustion and tenant quota overrides, that exposes user-actionable status, affected category, duration when available, and recovery guidance while hiding detection signals and enforcement thresholds.
- **Quota Override**: A tenant-specific effective-limit change that explains why visible limits differ from the base plan.
- **Support Evidence Export**: A structured redacted JSON tenant-scoped evidence package generated from a denial record for quota or abuse-restriction denial disputes, including relevant denial and usage metadata while excluding secrets, unrelated content, and cross-tenant data.
- **Billing Visibility Permission**: The permission boundary that determines who can view quota, usage, denial, override, and restriction information. Structured evidence export additionally requires `billing.evidence_export`.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Public billing, quota, denial, and usage visibility surfaces change additively for web shell and SDK consumers. Existing accounting records, quota definitions, denial reason meanings, and tenant resolution behavior remain authoritative.
- **Migration / Rollback**: No ledger rewrite or payment migration is in scope. Rollback should hide or disable the new product projections while preserving existing quota enforcement and billing evidence.
- **Verification Strategy**: Validate tenant-scoped dashboard behavior, denial details, abuse restriction messaging, permission gates, tenant switching, redacted support evidence, and regression coverage proving denied work is still blocked before expensive or side-effecting work starts.
- **Observability Impact**: Operator-facing support evidence and denial projections must make quota disputes debuggable without exposing secrets or unrelated tenant content. Existing audit evidence remains the source of truth.
- **Environment & Secrets**: Development and test environments must use explicit test tenants and test quota states. Live connectors, production tenants, secrets, and payment-provider checkout are out of scope for this phase.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: At least 95% of the acceptance-test denial fixture set can be explained from product-visible denial details without inspecting server logs; the fixture set MUST include every guarded Roadmap 38 category plus quota-state-unavailable, abuse-restriction, unauthorized, and operator-action-needed cases.
- **SC-002**: Authorized users can identify the active plan, exhausted limit, current and previous-period usage, reset or recovery action, and whether an abuse restriction applies within 30 seconds of opening quota visibility.
- **SC-003**: Permission tests confirm that 100% of quota, denial, override, restriction, and evidence views deny unauthorized access without exposing cross-tenant details.
- **SC-004**: Support operators can produce a structured redacted JSON quota evidence package for an eligible denial in under 2 minutes during a test walkthrough.
- **SC-005**: Tenant switching tests show zero stale previous-tenant quota or denial records visible after switching tenants.
- **SC-006**: Regression tests confirm that quota-denied guarded work starts zero expensive or side-effecting operations before the denial is surfaced.
- **SC-007**: User-facing quota and abuse messages correctly classify ordinary quota exhaustion, temporary restriction, unavailable quota state, unauthorized access, and pending operator resolution in all acceptance test cases while withholding abuse detection signals and enforcement thresholds.
- **SC-008**: Near-limit warning tests show warnings for every finite category at 80% consumed or below one category-defined typical operation remaining, whichever occurs first, including count, attempt, and byte quota categories.

## Assumptions

- Roadmap 38 billing, quota, usage accounting, enforcement, denial, and audit evidence remain the source of truth for this phase.
- Roadmap 45 hosted signup and tenant activation provide active tenants that can be resolved for quota visibility.
- Payment checkout, invoices, taxes, revenue recognition, marketplace pricing, and cross-tenant pooled quota remain out of scope.
- This phase focuses on tenant-scoped product projections, recovery guidance, and support evidence rather than changing quota accounting semantics.
- Users may belong to multiple tenants, so quota and denial views must always follow the active tenant context.
- Local-first or development tenants may have explicit unlimited or not-measurable quota states that should be shown accurately rather than treated as missing billing data.
