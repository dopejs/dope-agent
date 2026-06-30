# Feature Specification: Webhook And External Trigger Plane

**Feature Branch**: `main`
**Created**: 2026-06-30
**Status**: Draft
**Phase / Roadmap**: Phase 67 — Roadmap 67
**Upstream authority**: [docs/specs/052-webhook-and-external-trigger-plane.md](../../docs/specs/052-webhook-and-external-trigger-plane.md)

## Overview
Tenant-scoped webhook endpoints that safely trigger runs/workflows/routines. Every inbound
request authenticates via an HMAC signature, resolves tenant from the signed endpoint, enforces
replay protection (idempotency key), bounds + redacts the payload, runs quota/permission checks
before execution, and records an audited trigger outcome. Webhooks are trigger resources — not
channel connectors — and never ingest payloads into memory.

## User Scenarios & Testing *(mandatory)*
### US1 - Create a webhook that triggers a routine (P1)
1. A user creates a webhook (target routine/workflow/run); the signing secret is returned once.
2. A correctly-signed request fires the target exactly once and records a fired outcome.
### US2 - Rotate/inspect (P2)
1. An operator rotates the signing secret; the previous secret no longer authenticates.
2. An operator disables a webhook; further triggers are rejected.
### US3 - Confirm replay suppression (P3)
1. Support confirms duplicate payloads (same idempotency key) are suppressed, not re-fired.

### Edge Cases / Security
- Missing signature, bad signature, cross-tenant, oversized payload, disabled endpoint, quota
  denial — all rejected without firing; outcomes recorded redacted (size only, no payload body).

## Requirements *(mandatory)*
- **FR-001**: Webhook requests MUST authenticate (HMAC signature) and resolve tenant context.
- **FR-002**: The system MUST enforce idempotency/replay protection.
- **FR-003**: Payloads MUST be size-bounded and redacted in logs/events (records carry size only).
- **FR-004**: Webhook triggers MUST create normal runtime/routine execution records (via the
  existing scheduled-workflow launcher).
- **FR-005**: Quota and permission checks MUST run before execution starts.
- **FR-006**: Webhooks are trigger resources, not channel connectors; no memory ingestion.

### Key Entities
- Webhook Endpoint (target + status + redacted secret fingerprint), Trigger Record (redacted
  outcome), Signing Secret (returned once, never projected).

## Compatibility & Operational Impact *(mandatory)*
- **Compatibility**: Additive trigger plane; fires through the existing workflow launcher.
- **Migration / Rollback**: No migration; disable/rotate to revoke. Endpoints in-memory for this
  slice; execution evidence lives in the runtime/scheduler stores.
- **Verification**: API CRUD tests; security matrix (missing auth, bad signature, replay,
  oversized, cross-tenant, disabled, quota) at the manager level; firer linkage.
- **Observability**: trigger records (redacted); no payload content in logs/events.

## Success Criteria *(mandatory)*
- **SC-001**: Signed trigger fires the target once; unsigned/bad-signature/cross-tenant rejected.
- **SC-002**: Duplicate idempotency key suppressed (fires once).
- **SC-003**: Oversized payload + disabled + quota-denied rejected before firing.
- **SC-004**: Rotating the secret invalidates the prior one; secret never projected.

## Assumptions
- HMAC-SHA256 signature over the raw payload using the per-endpoint secret. Endpoints in-memory
  with future durable persistence; quota gate is pluggable (permissive default, billing wiring is
  a follow-on). Inbound chat connectors are out of scope.
