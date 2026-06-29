# Integration Adapters

Reserved home for out-of-process integration adapter processes (Roadmap 59).

An integration adapter implements the capability RPC contract under
`schemas/capability/integration-adapter/` and is reached by the daemon's in-process
`Backend` shim selected via `BackendBinding.BackendKind = adapter_rpc`. The daemon owns the
operation ledger, idempotency, evidence, and persistence; an adapter performs provider
request/response mapping only and holds no durable tenant state, credentials, or content.

The reference adapter skeleton (no real provider) is the Go binary
`daemon/cmd/dope-integration-adapter`. Real providers (Google Calendar, Gmail) land as
adapters in Roadmap 60 / 63.
