# Implementation Plan: Operator-Managed Skill And Capability Catalog

**Branch**: `main` | **Spec**: [spec.md](./spec.md) | **Upstream**: [docs/specs/053-operator-managed-skill-and-capability-catalog.md](../../docs/specs/053-operator-managed-skill-and-capability-catalog.md)
**Phase / Roadmap**: Phase 68 — Roadmap 68

## Summary
New `internal/catalog` subsystem: operator-curated catalog items with versions/trust tier/
requirements/permissions; tenant-scoped enablement with audit history + deterministic rollback
(version stack); pluggable RequirementChecker + PermissionGate fail-closed before enable/execution;
ActiveVersion runtime evidence gated on enabled state + met requirements.

## Constitution Check
- Roadmap closure: skill/capability catalog parity for operator-managed extensions without
  agent-managed skill generation.
- Production-grade: fail-closed policy, audited tenant-scoped enablement, deterministic rollback,
  runtime active-version gating.
- Contracts first: catalog item + enablement schemas; contract test (tenantId additive).
- Verification: lifecycle + policy + active-version + tenant-scope unit tests.

## Project Structure
```
specs/053-operator-managed-skill-and-capability-catalog/  spec.md plan.md tasks.md checklists/
daemon/internal/catalog/types.go,manager.go,manager_test.go
daemon/internal/api/catalog.go ; server.go + app.go wiring
schemas/api/catalog-item-resource.schema.json, catalog-enablement-resource.schema.json
daemon/internal/contracts/catalog_contracts_test.go
```

## Complexity Tracking
No violations. Additive subsystem; checker/gate are interfaces with permissive defaults and
test fakes; existing registries can feed item projections later.
