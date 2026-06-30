# Implementation Plan: Execution Backend And Sandbox Profile UX

**Branch**: `main` | **Spec**: [spec.md](./spec.md) | **Upstream**: [docs/specs/054-execution-backend-and-sandbox-profile-ux.md](../../docs/specs/054-execution-backend-and-sandbox-profile-ux.md)
**Phase / Roadmap**: Phase 69 — Roadmap 69

## Summary
New `internal/execprofile` projection subsystem: execution profiles (backend/risk/provides/
requirements) with live status (HealthChecker + RequirementChecker), denial explanations
(eligible vs missing-capability vs unavailable), catalog compatibility, and permission-gated +
audited selection that fails closed when unavailable. The sandbox/policy layer stays authoritative.

## Constitution Check
- Roadmap closure: users/operators understand backend availability + sandbox denials from product
  surfaces.
- Production-grade: fail-closed selection, denial linkage, projections never weaken gates.
- Contracts first: profile resource + denial explanation schemas; contract test.
- Verification: list/status, denial, compatibility, selection gating unit tests.

## Project Structure
```
specs/054-execution-backend-and-sandbox-profile-ux/  spec.md plan.md tasks.md checklists/
daemon/internal/execprofile/types.go,manager.go,manager_test.go
daemon/internal/api/execprofile.go ; server.go + app.go wiring (default subprocess profile)
schemas/api/execution-profile-resource.schema.json, execution-denial-explanation.schema.json
daemon/internal/contracts/execprofile_contracts_test.go
```

## Complexity Tracking
No violations. Read/projection subsystem; checker/gate interfaces with defaults + test fakes.
