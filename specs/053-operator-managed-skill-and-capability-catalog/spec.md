# Feature Specification: Operator-Managed Skill And Capability Catalog

**Feature Branch**: `main`
**Created**: 2026-06-30
**Status**: Draft
**Phase / Roadmap**: Phase 68 — Roadmap 68
**Upstream authority**: [docs/specs/053-operator-managed-skill-and-capability-catalog.md](../../docs/specs/053-operator-managed-skill-and-capability-catalog.md)

## Overview
Operator-managed catalog for skills, MCP servers, and supervised capabilities. Catalog items
declare source, version, trust tier, requirements, and permissions. Enablement is tenant-scoped
and auditable; policy blocks unmet requirements and denied permissions before install/execution
(fail closed); rollback restores a prior enabled version or disables safely; runtime evidence
identifies the active item version. The agent does NOT author or promote its own skills here.

## User Scenarios & Testing *(mandatory)*
### US1 - Enable a vetted capability (P1)
1. An operator registers a catalog item (versions + trust tier + requirements + permissions) and
   enables a version for a tenant; the enablement is recorded with an audit event.
2. Enablement is tenant-scoped: another tenant does not inherit it.
### US2 - Rollback (P2)
1. An operator rolls back to the prior enabled version; if none, the item disables safely.
### US3 - Inspect availability (P3)
1. A user inspects an item and sees unmet requirements / permission status (why it is unavailable).

### Edge Cases
- Unmet requirement or denied permission blocks enable (fail closed).
- A disabled item reports no active version; requirements regressing makes it inactive for runtime.

## Requirements *(mandatory)*
- **FR-001**: Catalog items MUST declare source, version, trust tier, requirements, and permissions.
- **FR-002**: Enablement MUST be tenant-scoped and auditable.
- **FR-003**: Policy MUST block unmet requirements (and denied permissions) before install/execution.
- **FR-004**: Rollback MUST restore a prior enabled version or disable safely.
- **FR-005**: Runtime evidence MUST identify the active catalog item version (and gate on it).
- **FR-006**: The agent MUST NOT author or promote skills; items are operator-curated.

### Key Entities
- Catalog Item (kind/trust tier/permissions/versions+requirements), Enablement (tenant-scoped
  active version + audit history + version stack), Inspection.

## Compatibility & Operational Impact *(mandatory)*
- **Compatibility**: Additive catalog subsystem; existing skill/MCP registries can feed item
  projections (follow-on); no registry replacement.
- **Migration / Rollback**: No migration; items/enablements in-memory for this slice with Restore.
- **Verification**: lifecycle (enable/disable/rollback/inspect), requirement + permission gating,
  active-version gating, tenant scoping.
- **Observability**: enablement audit history + active-version evidence.

## Success Criteria *(mandatory)*
- **SC-001**: Enable records a tenant-scoped, audited active version; other tenants unaffected.
- **SC-002**: Unmet requirements / denied permissions block enable.
- **SC-003**: Rollback restores prior version or disables; deterministic via version stack.
- **SC-004**: Disabled / requirement-regressed items report no runtime active version.

## Assumptions
- Items/enablements in-memory with Restore for this slice; RequirementChecker + PermissionGate
  are pluggable (permissive defaults; sandbox/secret/policy wiring is a follow-on). Community
  marketplace and agent-authored skills are out of scope.
