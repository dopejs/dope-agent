# Tasks: Support Diagnostics And Evidence Bundle

**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Roadmap**: 71

- [X] T001 [Setup] New internal/evidence package.
- [X] T002 [Foundational] types: Scope/Section/Bundle/AccessEvent + scope/redaction enums.
- [X] T003 [Foundational] redaction: scrub sensitive keys + fail-closed on residual secret markers.
- [X] T004 [US1] manager.Generate (permission + scope validation + collect + redact fail-closed + retention + audit).
- [X] T005 [US2] Get/ListForTenant (permission-gated, cross-tenant denied, access audited).
- [X] T006 [US3] AuditTrail (generated/accessed events).
- [X] T007 [P] tests: redaction matrix + fail-closed, permission + tenant isolation, audit, invalid scope.
- [X] T008 [API] app + server wiring; /v1/support/evidence-bundles generate/list/get.
- [X] T009 [Polish] schemas + contract test; verify build/vet/test.

## Notes
Collector empty default (real diagnostic/eval/audit collectors are a follow-on); PermissionGate
permissive default until support-role wiring. Bundles in-memory with retention.
