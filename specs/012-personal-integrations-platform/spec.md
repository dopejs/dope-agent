# Feature Specification: Personal Integrations Platform

**Feature Branch**: `[012-personal-integrations-platform]`  
**Created**: 2026-04-22  
**Status**: Draft  
**Input**: User description: "结合 docs/specs/012-personal-integrations-platform.md 完成 phase 27 的工作"

## Clarifications

### Session 2026-04-22

- Q: How should integration uniqueness work when the same account can be connected more than once in the same environment? → A: Allow multiple integrations for the same domain/account/environment, but require one canonical default record.
- Q: What should `degraded` mean for integration-backed execution? → A: Block all work only for `unavailable`; keep `degraded` inspectable but require operation-specific gating.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Inspect Integration Readiness (Priority: P1)

As an operator, I need to see which personal systems are connected, which account each
connection represents, and whether each connection is ready to use so I can trust the
agent before any calendar, mail, or reminder workflow depends on it.

**Why this priority**: Roadmap 27 primarily closes an operator-trust gap. If readiness,
account identity, or auth state remain hidden in raw config or provider-specific details,
later personal-domain slices will inherit unreliable foundations.

**Independent Test**: Create or inspect integration records for representative personal
systems, move them through not configured, auth pending, healthy, degraded, and
unavailable states, and confirm an operator can determine readiness and account identity
from operator-visible surfaces alone.

**Acceptance Scenarios**:

1. **Given** a personal system has not yet been connected, **When** the operator inspects
   available integrations, **Then** the system shows that the integration is not
   configured rather than appearing silently absent or healthy.
2. **Given** a personal system is connected but still waiting on user authorization,
   **When** the operator inspects that integration, **Then** the system shows auth
   pending status, the intended account identity if known, and the missing readiness step.
3. **Given** an integration becomes healthy, degraded, or unavailable, **When** the
   operator inspects it, **Then** the system shows the current readiness state, account
   identity, environment scope, and recent health truth without requiring raw config
   access.

---

### User Story 2 - Run Personal-System Work On Shared Truth (Priority: P2)

As an operator, I need personal-system work to use one shared integration model so
calendar, mail, and reminder actions inherit the same auth, approval, provenance, and
environment rules instead of each domain inventing separate connection behavior.

**Why this priority**: The roadmap is only closed if later domains can depend on one
stable substrate. Rebuilding auth and readiness rules per domain would recreate the same
operational risk in multiple places.

**Independent Test**: Execute representative integration-backed work in `DOPE_ENV=test`
using at least one repo-owned fake or local integration path, then confirm the run or
workflow truth retains integration identity, readiness, approval, and redacted
secret-scope provenance in a shared way that is not domain-specific.

**Acceptance Scenarios**:

1. **Given** a run or workflow invokes work backed by a connected personal system,
   **When** the work executes, **Then** the resulting operator-visible history preserves
   which integration and account were used and whether readiness or approval constrained
   the work.
2. **Given** the same underlying auth or health problem affects multiple personal
   domains, **When** the operator inspects the impacted work, **Then** the system uses
   one shared readiness vocabulary rather than domain-specific status meanings.
3. **Given** secret-backed integration material is required for a personal-system action,
   **When** the operator reviews the resulting history, **Then** the system exposes
   provenance and environment scope without revealing raw secret values.

---

### User Story 3 - Reuse One Integration Contract Across Domains (Priority: P3)

As a product or platform engineer, I need calendar, mail, and future personal domains to
build on one integration contract so new domains can add domain logic without reopening
connection lifecycle, health semantics, or operator-visible provenance.

**Why this priority**: This phase is a platform slice whose value compounds in later
roadmaps. Without a reusable contract, every domain implementation becomes a partial
platform rewrite.

**Independent Test**: Review the shared integration model against at least two downstream
domain specs and confirm they can reuse the same identity, readiness, and provenance
contract without redefining connection lifecycle concepts.

**Acceptance Scenarios**:

1. **Given** a new personal domain is added after roadmap 27, **When** it binds to an
   external account-backed system, **Then** it can reuse the shared integration identity
   and readiness model instead of defining a separate connection contract.
2. **Given** multiple backend styles are used for integrations, **When** operators or
   downstream domains inspect them, **Then** the operator-visible surface converges on
   the same integration concepts even if backend implementations differ.
3. **Given** a future domain adds its own domain objects and actions, **When** it reports
   results or failures, **Then** the shared integration substrate continues to own account
   binding, auth readiness, and provenance truth while domain-specific behavior stays
   separate.

### Edge Cases

- If an integration is configured in one environment but not another, the system shows
  environment-scoped readiness and never implies cross-environment reuse.
- If authorization expires after an integration was previously healthy, the system
  records a transition to auth pending or degraded truth instead of continuing to present
  the integration as ready.
- If backend health becomes unavailable while existing workflows still reference the
  integration, the system exposes the loss of readiness explicitly rather than failing as
  an unexplained downstream domain error.
- If an integration is degraded rather than unavailable, the system keeps that degraded
  state operator-visible and leaves downstream execution to operation-specific gating
  instead of silently treating the integration as either fully healthy or fully blocked.
- If the same personal account can be reached through different backend styles, the
  operator-visible surface still distinguishes the chosen integration record and its
  provenance without forcing provider-specific inspection, and one canonical default
  record remains identifiable for downstream use.
- If an operator removes or revokes secret-backed material, the system preserves redacted
  historical provenance and readiness history without keeping raw secret values visible.
- If a restart happens while readiness truth or health checks are mid-update, the system
  restores the last durable operator-visible state and does not silently invent a healthy
  outcome.
- Phase 27 does not add marketplace discovery, multi-user tenancy, or domain-specific
  calendar or mail behaviors; requests for those capabilities remain out of scope.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST expose first-class integration resources for account-backed
  personal systems rather than hiding connection truth inside provider-specific config.
- **FR-002**: Each integration resource MUST show the personal system it represents, the
  bound account identity when known, the environment scope, and the current readiness
  state.
- **FR-003**: The system MUST distinguish at least these readiness states for
  integrations, with operator-facing prose mapping directly to API enum values:
  not configured (`not_configured`), auth pending (`auth_pending`), healthy
  (`healthy`), degraded (`degraded`), and unavailable (`unavailable`).
- **FR-004**: The system MUST preserve operator-visible provenance for each integration,
  including how it is backed, which environment it belongs to, and whether secret-backed
  material is present without exposing raw secret values.
- **FR-004a**: The system MUST allow multiple integration records for the same domain,
  account identity, and environment when operators intentionally connect more than one
  backend path, but it MUST identify exactly one canonical default record for downstream
  use at a time.
- **FR-005**: Integration-backed work MUST preserve shared approval, provenance, and
  redacted secret-scope truth when runs or workflows use a personal-system connection.
- **FR-006**: Domain-specific implementations MUST reuse the shared integration identity
  and readiness semantics instead of redefining separate connection lifecycle models.
- **FR-007**: The system MUST surface readiness changes and health degradation in an
  operator-visible way so operators can tell why a personal integration is not currently
  usable.
- **FR-007a**: `Unavailable` integrations MUST block integration-backed work until
  readiness is restored, while `degraded` integrations MUST remain inspectable and rely
  on operation-specific gating rather than one blanket execution rule.
- **FR-008**: Integration resources MUST remain inspectable across daemon restart within
  the same environment, including the last durable readiness and provenance truth.
- **FR-009**: The system MUST support shared integration semantics across different
  backend styles while converging on one operator-visible resource model.
- **FR-010**: The system MUST keep integration readiness separate from delivery or
  notification behavior so later phases can depend on the same readiness truth without
  coupling delivery outcomes to connection health.
- **FR-011**: The system MUST provide at least one repo-owned fake or local integration
  verification path so integration readiness, auth-state transitions, and provenance can
  be validated in `DOPE_ENV=test`.
- **FR-012**: Existing non-personal-system runtime behavior MUST continue to work without
  adopting the new integration substrate unless a workflow or feature explicitly uses an
  integration-backed domain.
- **FR-013**: Phase 27 MUST stay domain-agnostic; it MUST NOT require domain-specific
  calendar, mail, file, or reminder object behavior to claim completion.
- **FR-014**: Phase 27 MUST NOT introduce marketplace discovery or multi-user tenancy as
  a prerequisite for shared integration readiness and provenance truth.

### Key Entities *(include if feature involves data)*

- **Integration Resource**: The operator-visible record for one account-backed personal
  system connection, including its domain family, account identity, backing style,
  environment scope, readiness, and provenance.
- **Account Binding**: The durable association between a personal-system integration and
  the user account or mailbox, calendar, or equivalent external identity it represents,
  including whether that record is the canonical default binding for downstream use in
  its environment.
- **Integration Readiness State**: The shared truth describing whether the integration is
  not configured, waiting on authorization, healthy, degraded, or unavailable, where
  operator-facing labels map directly to the API enum values above, `unavailable` blocks
  work, and `degraded` requires operation-specific gating.
- **Integration Provenance Record**: The inspectable history describing how an
  integration is backed, whether required secret-backed material exists, and what
  redacted environment or auth context explains its readiness.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Additive API, schema, event, config, and storage surface
  changes are expected for integration resources, readiness projections, provenance
  truth, and downstream runtime linkage. Existing non-integration runtime behavior remains
  backward compatible.
- **Migration / Rollback**: Additive integration-resource persistence and operator
  surfaces are required. Rollback is a revert of integration-specific routes,
  projections, and persistence while preserving already-recorded history as read-only
  audit truth where needed.
- **Verification Strategy**: Required validation includes targeted store, API, runtime,
  workflow, and approval coverage for integration lifecycle, readiness transitions, and
  binding projection; contract coverage for shared integration resources and events;
  restart coverage for durable readiness truth; and one repo-owned fake or local
  integration path in `DOPE_ENV=test`.
- **Observability Impact**: Operators must be able to inspect integration identity,
  readiness transitions, degraded or unavailable causes, redacted provenance, and
  environment scope without reading raw config files or provider logs.
- **Environment & Secrets**: Work defaults to `DOPE_ENV=test`. Live personal-system
  connectors are not required to validate the phase. Secret-backed material remains
  operator-owned, environment-scoped, and redacted in operator-visible history.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In manual validation, an operator can determine whether a representative
  personal integration is not configured, auth pending, healthy, degraded, or unavailable
  in under 2 minutes using operator-visible surfaces only.
- **SC-002**: In automated verification, 100% of exercised readiness transitions preserve
  account identity when known, environment scope, and redacted provenance without
  requiring raw secret inspection.
- **SC-003**: At least one repo-owned fake or local integration path in `DOPE_ENV=test`
  can demonstrate initial setup, auth-pending transition, healthy readiness, and a
  degraded or unavailable outcome end to end.
- **SC-004**: During planning review for later personal domains, at least two downstream
  domain specs can reference the shared integration model without redefining readiness or
  account-binding semantics.
- **SC-005**: After restart validation, previously recorded integration readiness and
  provenance remain inspectable, and no tested restart path falsely reports an unhealthy
  or unknown integration as healthy.

## Assumptions

- Phase 27 establishes a shared platform contract for account-backed personal systems and
  does not by itself close calendar, mail, reminders, or file-domain behavior.
- The daemon remains the owner of integration truth, and existing run or workflow
  surfaces stay the execution hosts for later integration-backed actions.
- Single-operator environment behavior remains the default; multi-user tenancy is out of
  scope for this phase.
- Different backend styles may exist behind integrations, but operator-visible readiness,
  identity, and provenance must converge on one shared model.
- Secret-backed material can be referenced and validated for presence or readiness
  without exposing raw secret values in operator-visible surfaces.
- Delivery and notification outcomes may later depend on integration readiness, but this
  phase does not define delivery behavior itself.
