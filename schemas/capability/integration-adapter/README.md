# Integration Adapter Capability RPC

Schema-defined contract between the daemon and an out-of-process integration adapter
(Roadmap 59 / `docs/specs/044-external-integration-adapter-plane.md`).

- `request.json` — daemon → adapter request envelope for one `Backend` operation.
- `response.json` — adapter → daemon response envelope.
- The adapter health event lives at `schemas/events/integrations/adapter-health.json`.

Invariants (enforced in `daemon/internal/integrations/adapterrpc` and tested via
`make daemon-contract-test`):

- The adapter performs provider request/response mapping only. It MUST NOT carry ledger,
  idempotency, or side-effect-evidence state — those are daemon-owned.
- `contractVersion` MUST exact-match between daemon and adapter (refuse on mismatch).
- Credentials are injected per call and MUST NOT be persisted or logged by the adapter.
- The daemon supplies a per-operation `deadlineMs`; the adapter MUST honor it.
