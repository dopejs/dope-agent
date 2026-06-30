# Tasks: Webhook And External Trigger Plane

**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Roadmap**: 67

- [X] T001 [Setup] Confirm scheduler WorkflowLauncher + routine manager for firer linkage.
- [X] T002 [Foundational] webhook types: Endpoint/TriggerRecord/CreateSecret + status/target enums; MaxPayloadBytes.
- [X] T003 [Foundational] manager: Create/Rotate/Disable/Get/List (tenant-scoped); HMAC sign/verify; replay dedup; payload bound; quota gate; Firer.
- [X] T004 [US1] Trigger: tenant resolve -> status -> bound -> signature -> replay -> quota -> fire; redacted record.
- [X] T005 [US2] Rotate invalidates prior secret; Disable rejects; secret never projected.
- [X] T006 [US3] replay suppression via idempotency key.
- [X] T007 [P] security-matrix tests: missing auth, bad signature, cross-tenant, oversized, disabled, quota, replay, rotate.
- [X] T008 [API] app firer adapter (workflow launcher) + manager wiring; /v1/webhooks management + /v1/triggers/webhook ingress.
- [X] T009 [Polish] schemas + contract test; verify build/vet/test.

## Notes
Endpoints/secrets in-memory for this slice; quota gate permissive default (billing wiring is a
follow-on). Execution evidence persists in the runtime/scheduler stores via the launcher.
