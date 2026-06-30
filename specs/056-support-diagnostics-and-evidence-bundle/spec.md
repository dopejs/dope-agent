# Feature Specification: Support Diagnostics And Evidence Bundle

**Feature Branch**: `main`
**Created**: 2026-06-30
**Status**: Draft
**Phase / Roadmap**: Phase 71 — Roadmap 71
**Upstream authority**: [docs/specs/056-support-diagnostics-and-evidence-bundle.md](../../docs/specs/056-support-diagnostics-and-evidence-bundle.md)

## Overview
A permission-gated, tenant-scoped, redacted-by-default evidence bundle for support/incident
triage. Bundles collect resource summaries + links (never raw secrets, tokens, credential-bearing
payloads, or unbounded logs) for a selectable scope (run/workflow/thread/connector/provider/
routine/quota_denial/time_window). Generation and access are audited; redaction failure fails
closed.

## User Scenarios & Testing *(mandatory)*
### US1 - Generate a bundle for a failed routine (P1)
1. An authorized actor generates a bundle for a routine scope; it contains redacted summaries +
   links and a retention expiry.
### US2 - Support requests a bundle for an authorized tenant (P2)
1. A support actor reads a bundle for the owning tenant; cross-tenant access is denied.
### US3 - Auditor sees who generated/accessed a bundle (P3)
1. Each generation/access is recorded as an audit event.

### Edge Cases
- A value carrying raw secret material under a non-sensitive key fails the bundle closed.
- Invalid scope (missing ref / window) is rejected before collection.

## Requirements *(mandatory)*
- **FR-001**: Bundles MUST carry tenant, actor, scope, created time, retention, and redaction status.
- **FR-002**: Bundles MUST include relevant resource summaries and links (not raw logs).
- **FR-003**: Bundles MUST exclude raw secrets, OAuth tokens, credential-bearing payloads, and
  inaccessible tenant data.
- **FR-004**: Bundle generation + access MUST be permission-gated (support role) and audited.
- **FR-005**: Redaction failure MUST fail closed (no bundle returned/persisted).

### Key Entities
- Scope, Section (redacted summaries + links), Bundle (+ redaction status + retention), AccessEvent.

## Compatibility & Operational Impact *(mandatory)*
- **Compatibility**: Additive; reuses existing diagnostic/eval/audit/event records via a Collector
  rather than duplicating raw data.
- **Migration / Rollback**: None; bundles in-memory with retention for this slice.
- **Verification**: redaction matrix (sensitive keys + fail-closed), permission + tenant isolation,
  audit trail, invalid scope.
- **Observability**: generation/access audit events.

## Success Criteria *(mandatory)*
- **SC-001**: Sensitive keys are redacted; no raw secret material appears in a bundle.
- **SC-002**: Raw secret material under a non-sensitive key fails the bundle closed.
- **SC-003**: Non-support generation/access denied; cross-tenant access denied.
- **SC-004**: Generation + access are audited.

## Assumptions
- Collector is injectable (empty default; real diagnostic/eval/audit collectors wired as a
  follow-on). PermissionGate enforces the support role (permissive default until wired). Bundles
  in-memory with retention; no full log archive, no cross-tenant browsing, no memory export.
