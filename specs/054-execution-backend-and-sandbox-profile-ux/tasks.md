# Tasks: Execution Backend And Sandbox Profile UX

**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Roadmap**: 69

- [X] T001 [Setup] New internal/execprofile package.
- [X] T002 [Foundational] types: ExecutionProfile/ProfileStatus/Projection/DenialExplanation/Compatibility/Selection; backend/risk/health enums.
- [X] T003 [Foundational] manager: HealthChecker/RequirementChecker/PermissionGate (defaults); RegisterProfile; live status.
- [X] T004 [US1] ListProfiles + GetProfile (status/requirements/risk); ExplainDenial (eligible/missing/unavailable).
- [X] T005 [US2] requirement + health surfaced in status + denial reasons.
- [X] T006 [US3] CompatibilityFor (catalog item caps); SelectProfile (permission-gated, audited, fail-closed).
- [X] T007 [P] tests: list/status, denial (+unavailable), compatibility, selection gating.
- [X] T008 [API] app + server wiring (default subprocess profile); /v1/execution/profiles + /explain.
- [X] T009 [Polish] schemas + contract test; verify build/vet/test.

## Notes
Profiles/selections in-memory with Restore; checker/gate defaults (sandbox health + policy wiring
is a follow-on). The sandbox/policy layer remains authoritative for execution permission.
