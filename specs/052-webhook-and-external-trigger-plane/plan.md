# Implementation Plan: Webhook And External Trigger Plane

**Branch**: `main` | **Spec**: [spec.md](./spec.md) | **Upstream**: [docs/specs/052-webhook-and-external-trigger-plane.md](../../docs/specs/052-webhook-and-external-trigger-plane.md)
**Phase / Roadmap**: Phase 67 — Roadmap 67

## Summary
New `internal/webhook` trigger plane: tenant-scoped endpoints with HMAC-signature auth, replay
protection, bounded+redacted payloads, pluggable quota gate, and a Firer that launches the
target workflow through the existing scheduled-workflow launcher (real runtime execution record).

## Technical Context
- Go 1.24 daemon. Firer = scheduler.WorkflowLauncher adapter (routine target -> its workflow
  goal). Endpoints/secrets/replay keys in-memory for this slice; execution evidence persists in
  the runtime/scheduler stores. API: management (protected) + inbound ingress (signature-auth).

## Constitution Check
- Roadmap closure: external systems safely wake the agent without abusing connectors/integrations.
- Production-grade: HMAC auth, replay protection, payload bounding+redaction, quota-before-fire,
  secret rotation, cross-tenant rejection.
- Contracts first: webhook endpoint + create request + trigger record schemas; contract test.
- Verification: manager security matrix + CRUD; firer linkage.
- Environment: opt-in; test env default.

## Project Structure
```
specs/052-webhook-and-external-trigger-plane/  spec.md plan.md tasks.md checklists/
daemon/internal/webhook/types.go,manager.go,manager_test.go
daemon/internal/api/webhook.go ; server.go + app.go wiring (firer adapter)
schemas/api/webhook-endpoint-resource.schema.json, create-webhook.request.schema.json, webhook-trigger-record.schema.json
daemon/internal/contracts/webhook_contracts_test.go
```

## Complexity Tracking
No violations. Additive trigger plane; fires through the existing launcher; security core is the
focus and is fully unit-tested.
