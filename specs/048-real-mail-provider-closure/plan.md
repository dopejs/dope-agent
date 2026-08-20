# Implementation Plan: Real Mail Provider Closure (Feishu/Lark)

**Branch**: `main` | **Spec**: [spec.md](./spec.md) | **Upstream**: [docs/specs/048-real-mail-provider-closure.md](../../docs/specs/048-real-mail-provider-closure.md)
**Phase / Roadmap**: Phase 63 — Roadmap 63

## Summary

Close one real mail provider — Feishu/Lark — as an adapter on the integration adapter plane,
mirroring the Roadmap 60 calendar closure. Reuse the daemon-owned mail operation ledger,
diagnostics vocabulary, live-validation, and delivery truth unchanged. Net deliverables:

1. A real Feishu/Lark **mail provider** (`internal/integrations/providers/feishulark/mail.go`)
   implementing `adapterprovider.Handler` for the `mail` domain — account/thread/message/draft
   projection, draft create/update, send/send-draft/reply/forward — over an injectable Feishu
   Mail HTTP client (synthetic responses in CI). Stable redacted failure mapping; ambiguous
   sends via the contract's ambiguity channel; no message/credential leakage.
2. Mail adapter shim: carry provider kind + stable failure class (mirror calendar) so failures
   land on the `feishu_lark` diagnostics reasons.
3. Adapter binary: `KURA_ADAPTER_DOMAIN=mail` serves the real mail provider.
4. App wiring: register the mail adapter backend with provider kind feishu_lark when configured
   (credential fetcher already shared from Roadmap 60).
5. `opsreadiness` mail real-account smoke with structured skip (mirror calendar_smoke).

## Technical Context
- **Language**: Go 1.24 (daemon). Additive only.
- **Dependencies**: `internal/mail` (Backend, Manager, types, fake), `internal/integrations/
  providers/feishulark` (new mail.go reusing the existing client + token + error mapping),
  `internal/integrations/adapterprovider`, `internal/integrations` diagnostics, `internal/app`
  wiring, `internal/opsreadiness`.
- **Storage**: none new; adapter stateless.
- **Testing**: provider mail ops over httptest + in-process pipe through the mail Manager;
  ambiguous send; diagnostics mapping; smoke; existing fake mail suite green.
- **Constraints**: additive; no second mail ledger; full attachment transfer out of scope;
  no credential/message-content leakage beyond redacted evidence.

## Constitution Check
- **Roadmap closure**: one real mail provider hosted-ready for the Roadmap 30 capability set;
  fake remains required.
- **Production-grade**: per-call credentials fail-closed, deadline-bounded, ambiguous-commit
  safety, redacted diagnostics, supervised adapter.
- **Contracts first**: maps onto existing mail resources; any schema change additive.
- **Verification**: provider + shim + manager + live-validation + smoke coverage; fake unchanged.
- **Environment**: fake default; real provider only with explicit operator credentials.

## Project Structure
```
specs/048-real-mail-provider-closure/  spec.md plan.md tasks.md checklists/
daemon/internal/integrations/providers/feishulark/mail.go        # NEW provider
daemon/internal/integrations/providers/feishulark/mail_e2e_test.go # NEW tests
daemon/internal/mail/adapter_backend.go    # EDIT: provider kind + stable failure class
daemon/internal/mail/manager.go            # EDIT (if needed): providerKind into failOperation
daemon/internal/app/app.go                 # EDIT: mail adapter WithProviderKind when configured
daemon/cmd/kura-integration-adapter/main.go# EDIT: KURA_ADAPTER_DOMAIN=mail real provider
daemon/internal/opsreadiness/mail_smoke.go # NEW smoke helper
```

## Complexity Tracking
No violations. The provider is additive behind the existing mail Backend contract and the
existing `adapter_rpc` backend kind; the serve harness and credential path already exist.
</content>
