# Implementation Plan: Support Diagnostics And Evidence Bundle

**Branch**: `main` | **Spec**: [spec.md](./spec.md) | **Upstream**: [docs/specs/056-support-diagnostics-and-evidence-bundle.md](../../docs/specs/056-support-diagnostics-and-evidence-bundle.md)
**Phase / Roadmap**: Phase 71 — Roadmap 71

## Summary
New `internal/evidence` subsystem: permission-gated, tenant-scoped, redacted-by-default support
evidence bundles. A Collector gathers redaction-candidate sections (summaries + links) for a
scope; a redactor scrubs sensitive keys and fails closed on residual secret markers; generation
and access are audited with retention.

## Constitution Check
- Roadmap closure: support can triage hosted failures from safe bundles without DB/secret access.
- Production-grade: redaction fail-closed, permission + tenant isolation, audit, retention.
- Contracts first: bundle + request schemas; contract test.
- Verification: redaction matrix, permission/isolation, audit, invalid-scope.

## Project Structure
```
specs/056-support-diagnostics-and-evidence-bundle/  spec.md plan.md tasks.md checklists/
daemon/internal/evidence/types.go,redaction.go,manager.go,manager_test.go
daemon/internal/api/evidence.go ; server.go + app.go wiring
schemas/api/evidence-bundle-resource.schema.json, generate-evidence-bundle.request.schema.json
daemon/internal/contracts/evidence_contracts_test.go
```

## Complexity Tracking
No violations. Additive; Collector/PermissionGate are interfaces with defaults + fakes; redaction
fails closed.
