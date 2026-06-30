# Tasks: Operator-Managed Skill And Capability Catalog

**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Roadmap**: 68

- [X] T001 [Setup] New internal/catalog package.
- [X] T002 [Foundational] types: ItemKind/TrustTier/Requirement/Version/CatalogItem; Enablement (+version stack + history); Inspection.
- [X] T003 [Foundational] manager: RegisterItem/GetItem/ListItems; RequirementChecker + PermissionGate interfaces (fail closed defaults).
- [X] T004 [US1] Enable (permission + requirement gates; audited, tenant-scoped); Disable.
- [X] T005 [US2] Rollback (version stack: restore prior or disable).
- [X] T006 [US3] Inspect (unmet requirements + permission status); ActiveVersion runtime gating.
- [X] T007 [P] tests: enable+tenant-scope, policy blocks, rollback, active-version gating, inspect.
- [X] T008 [API] app + server wiring; /v1/catalog/items CRUD + enable/disable/rollback/inspect.
- [X] T009 [Polish] schemas + contract test; verify build/vet/test.

## Notes
Items/enablements in-memory with Restore for this slice; checker/gate permissive defaults
(sandbox/secret/policy wiring is a follow-on). Existing skill/MCP registries feed item projections later.
