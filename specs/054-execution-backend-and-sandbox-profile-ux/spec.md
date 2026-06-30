# Feature Specification: Execution Backend And Sandbox Profile UX

**Feature Branch**: `main`
**Created**: 2026-06-30
**Status**: Draft
**Phase / Roadmap**: Phase 69 — Roadmap 69
**Upstream authority**: [docs/specs/054-execution-backend-and-sandbox-profile-ux.md](../../docs/specs/054-execution-backend-and-sandbox-profile-ux.md)

## Overview
Expose execution backend + sandbox profile availability, requirements, health, risk, and
selection through product surfaces. The daemon sandbox/policy layer remains authoritative for
execution permission; these projections never grant hidden access nor weaken preflight/approval
gates, and hosted defaults fail closed when a backend is unavailable.

## User Scenarios & Testing *(mandatory)*
### US1 - See profiles + why a tool cannot run (P1)
1. A user lists execution profiles with status (ready/degraded/unavailable), requirements, and
   risk tier.
2. For a tool needing a capability (e.g. docker), the system explains which profiles are eligible
   and why others are not (missing capability or unavailable backend).
### US2 - Diagnose backend unavailable / requirement denied (P2)
1. An operator sees a profile's unmet requirements and unavailability reason.
### US3 - Select a profile where policy allows (P3)
1. An operator selects a profile; selection is permission-gated, audited, and fails closed when
   the profile is unavailable.

### Edge Cases
- Unavailable backend / unmet requirement -> profile not available; selection rejected.
- Catalog item capability requirements map to compatible/incompatible profiles.

## Requirements *(mandatory)*
- **FR-001**: Expose available execution profiles with status, requirements, and risk.
- **FR-002**: Denials MUST link to missing requirements/capabilities or policy decisions.
- **FR-003**: Catalog items MUST show compatible and incompatible profiles.
- **FR-004**: Profile changes MUST be permission-gated and auditable (and fail closed when the
  profile is unavailable).
- **FR-005**: Projections MUST NOT weaken preflight/approval gates; the policy layer stays
  authoritative.

### Key Entities
- Execution Profile (backend/risk/provides/requirements), Profile Status (health + unmet
  requirements + availability), Denial Explanation, Compatibility, Selection (audited).

## Compatibility & Operational Impact *(mandatory)*
- **Compatibility**: Additive read/projection subsystem over the sandbox plane; no sandbox
  contract changes.
- **Migration / Rollback**: No migration; profiles/selections in-memory with Restore.
- **Verification**: list/status, denial explanation, compatibility, selection gating + fail-closed.
- **Observability**: profile status + selection audit history.

## Success Criteria *(mandatory)*
- **SC-001**: Profiles list with live status/requirements/risk.
- **SC-002**: Denials identify eligible profiles + missing capabilities/unavailability.
- **SC-003**: Catalog-item capability requirements resolve to compatible/incompatible profiles.
- **SC-004**: Selection is permission-gated, audited, and fails closed when unavailable.

## Assumptions
- Health/requirement/permission are pluggable (ready/met/allow defaults; sandbox health + policy
  wiring is a follow-on). A default always-available subprocess profile is registered. The
  sandbox/policy layer remains authoritative for actual execution.
