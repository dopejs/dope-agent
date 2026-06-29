# Phase 1 Data Model: External Integration Adapter Plane

This phase introduces no new SQLite tables. It reuses the existing `capabilities.Capability`
supervision records (already persisted/recovered via `recoverPersistedStateWithSecrets`) and
the existing integration `BackendBinding`. New shapes below are RPC envelope / in-memory
runtime structures and additive event fields.

## Entities

### IntegrationAdapterRuntime (in-memory; supervised)

The daemon-side owner of one adapter subprocess for one integration domain. Bridges process
lifecycle into `capabilities.Supervisor`.

| Field | Type | Notes |
|-------|------|-------|
| capabilityId | string | id registered with `capabilities.Supervisor` |
| domain | string | `calendar` \| `mail` |
| contractVersion | string | adapter-advertised version; must exact-match daemon |
| readiness | enum | `pending` \| `ready` \| `unavailable` |
| process | os/exec handle | not serialized |

Lifecycle status comes from the existing supervisor: `registered → healthy → degraded →
backing_off → failed`. `failed` is surfaced as integration **unavailable**.

### BackendBinding (existing — extended usage)

Reuses `integrations.BackendBinding`. New selector value:

| Field | Value |
|-------|-------|
| BackendKind | `adapter_rpc` (NEW enum value) |
| BackendRefID | adapter runtime / capability id |

Selection path unchanged: `resource.BackendBinding.BackendKind` keys the manager `backends`
map. Rollback = rebind to `fake_local` / `native`.

### Adapter RPC Request envelope

Sent daemon → adapter, one per `Backend` operation.

| Field | Type | Notes |
|-------|------|-------|
| requestId | string | correlation id |
| contractVersion | string | exact-match required |
| domain | string | `calendar` \| `mail` |
| operation | string | e.g. `ProjectAccount`, `ListEvents`, `CreateEvent`, `SendMessage` |
| deadlineMs | integer | daemon-supplied per-operation deadline (Q1); default applied if absent |
| resource | object | normalized integration `Resource` projection (no secrets) |
| credential | object | per-call scoped, short-lived material (NOT persisted by adapter) |
| payload | object | operation-specific input (mirrors existing `*Input` types) |

### Adapter RPC Response envelope

Sent adapter → daemon.

| Field | Type | Notes |
|-------|------|-------|
| requestId | string | echoes request |
| contractVersion | string | exact-match required |
| status | enum | `ok` \| `failure` |
| failureKind | enum? | `auth` \| `scope` \| `rate_limited` \| `unavailable` \| `malformed` \| `internal` (when status=failure) |
| payload | object? | normalized result (mirrors existing snapshot/projection types) when status=ok |
| diagnostic | object? | structured detail mapped to existing integration diagnostics |

The adapter MUST NOT return ledger, idempotency, or side-effect-evidence state — those are
daemon-owned (FR-003).

### Adapter Health Event (additive)

Emitted on adapter readiness/health/restart/circuit-break transitions, consistent with the
connector/capability health surface.

| Field | Type | Notes |
|-------|------|-------|
| capabilityId | string | |
| domain | string | |
| status | enum | mirrors `capabilities.Status` |
| readiness | enum | `pending` \| `ready` \| `unavailable` |
| restartCount | integer | |
| failureCount | integer | |
| contractVersion | string | |
| reason | string? | redacted failure/diagnostic reason |

## Validation rules

- `contractVersion` MUST exact-match between daemon and adapter at readiness and per call;
  mismatch ⇒ refuse with `contract_mismatch`, no operation attempted (FR-008, Q3).
- `credential` MUST be present for operations requiring provider auth, MUST be scoped and
  short-lived, and MUST NOT appear in adapter persistence or logs (FR-006).
- `deadlineMs` MUST be honored by the adapter; expiry ⇒ hang/timeout classification (FR-007b).
- A `failure` during a side-effecting write whose outcome is unconfirmed ⇒ ambiguous-commit
  classification in the single daemon ledger (FR-007a).
- Malformed/contract-violating responses ⇒ rejected with no partial ledger entry (FR-013).

## State transitions (reused supervisor semantics)

```
registered ──ready──▶ healthy ──failure──▶ backing_off ──restart──▶ registered
   │                     │                      │
   │                     └──degraded (heartbeat/health report)
   └────────────────── failureCount ≥ 5 ──────────────▶ failed (= integration unavailable)
```
