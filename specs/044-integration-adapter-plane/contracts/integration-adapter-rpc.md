# Contract: Integration Adapter RPC

**Status**: design artifact for Roadmap 59. During implementation these schemas are committed
under `schemas/capability/integration-adapter/` and covered by `make daemon-contract-test`.

## Transport

- Daemon spawns the adapter as a local child process (Phase 0 R1).
- Framing: newline-delimited JSON; one request and its correlated response per line-pair,
  correlated by `requestId`. The daemon enforces `deadlineMs`.
- Versioning: every envelope carries `contractVersion`; the daemon requires an **exact**
  match at readiness and per call. Mismatch ⇒ `contract_mismatch` diagnostic, no dispatch.

## Operations (mirror existing `Backend` interfaces)

Calendar (`daemon/internal/calendar/backend.go`): `ProjectAccount`, `ListEvents`, `GetEvent`,
`BusyFree`, `CreateEvent`, `UpdateEvent`, `CancelEvent`.

Mail (`daemon/internal/mail/backend.go`): `SupportsResource`, `ProjectAccount`, `ListThreads`,
`GetThread`, `GetMessage`, `ListDrafts`, `GetDraft`, `CreateDraft`, `UpdateDraft`,
`SendMessage`, `SendDraft`, `ReplyMessage`, `ForwardMessage`, `ResolveAttachments`.

> Note: `RestoreIntegrationState` and any ledger/idempotency/artifact recording remain
> daemon-owned and are NOT part of the adapter contract (FR-003).

## Failure semantics

| failureKind | Daemon classification |
|-------------|-----------------------|
| auth / scope | integration diagnostic (auth/scope), no successful side effect recorded |
| rate_limited | diagnostic; ret/replay per existing live-validation |
| unavailable | adapter/integration unavailable diagnostic |
| malformed | response rejected, no partial ledger entry |
| internal | generic adapter failure diagnostic |
| (deadline expiry) | hang/timeout; ambiguous-commit if mid side-effecting write |
| (process crash) | ambiguous-commit if mid side-effecting write; supervisor failure report |

See `integration-adapter-request.schema.json`, `integration-adapter-response.schema.json`,
and `adapter-health-event.schema.json` in this directory for the representative shapes.
