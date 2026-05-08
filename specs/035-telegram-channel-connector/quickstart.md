# Quickstart: Telegram Channel Connector

## Default Environment

Use the test environment by default:

```bash
make daemon-run-test
make daemon-test-status
```

Do not use production tenants, live connectors, or real Telegram credentials unless an
operator explicitly chooses a separate live validation path with safe credentials. Safe
credentials are non-production, scoped to a test tenant and test users/chats/groups,
explicitly approved by an operator, redacted in all evidence, and isolated from normal
production tenants.

## Implementation Order

1. Read the phase 50 artifacts:

   ```bash
   sed -n '1,220p' specs/035-telegram-channel-connector/spec.md
   sed -n '1,260p' specs/035-telegram-channel-connector/plan.md
   sed -n '1,260p' specs/035-telegram-channel-connector/contracts/telegram-channel-connector.md
   ```

2. Reconfirm the upstream shared contracts:

   ```bash
   sed -n '1,220p' docs/channels/channel-connector-conformance.md
   sed -n '1,260p' specs/033-channel-connector-conformance/contracts/channel-connector-conformance.md
   sed -n '1,220p' docs/specs/031-hosted-credential-and-oauth-setup-wizard.md
   sed -n '1,220p' docs/specs/013-delivery-and-notifications.md
   ```

3. Implement additively in this sequence:

   - contract/schema vocabulary for Telegram connector kind, setup, allowment,
     diagnostics, capability declaration, and smoke evidence
   - tenant-safe persistence/accessors for setup, account binding, allowment, dedupe,
     retained update evidence, reply, delivery, diagnostics, conformance, and retention
   - submitted bot-token setup through the hosted setup wizard
   - Telegram runtime under the shared connector supervisor
   - explicit direct-message sender/chat allowment and group mention/command gating
   - text-only route handling and unsupported outcomes for attachments/media/voice/
     payments/mini apps
   - chat/message identity dedupe with retained update evidence
   - final-only foreground replies and reply-failure diagnostics
   - connector-backed Telegram delivery adapter and delivery separation evidence
   - live smoke pass or structured skip evidence
   - docs and client projections only where public surfaces changed

## Targeted Verification

Run focused daemon tests while developing:

```bash
cd daemon
go test ./internal/connectors ./internal/connectors/telegram ./internal/setupwizard ./internal/im ./internal/delivery ./internal/store ./internal/api
```

Required targeted cases:

- valid Telegram setup with explicit validated allowment
- valid Telegram bot credential without explicit allowment returns `action-required`
- Telegram setup completes or returns actionable terminal-state diagnostics within 5 minutes
- malformed/invalid/revoked/missing bot token redaction
- provider unavailable setup returns `unavailable`
- no explicit allowed user/chat/group returns `action-required`
- cancelled setup preserves redacted audit evidence
- DM from allowed sender/chat accepted
- DM from unknown sender/chat blocked
- group disabled behavior ignored or blocked
- allowed group without bot mention or command ignored
- allowed group with bot mention accepted
- allowed group with command accepted
- attachment/media/voice/payment/mini-app inputs unsupported
- duplicate inbound suppression by chat/message identity after replay/reconnect
- retained Telegram update identity as redacted evidence
- reply send failure separated from assistant execution
- final-only foreground reply success
- foreground reply outcome separated from connector-backed background delivery outcome
- background delivery success, retry, suppression, and failure
- rate-limit diagnostic evidence
- provider/network diagnostic evidence
- diagnostic stale after 15 minutes
- current diagnostic truth on failed connector actions
- 90-day evidence retention expiry
- redaction suppression for unsafe evidence
- tenant isolation and permission denial
- fake safe-live smoke pass evidence
- live hosted smoke pass evidence or structured skip evidence
- authorized support can inspect latest Telegram diagnostic reason, remediation, and freshness within 2 minutes

## Contract And Full Verification

When implementation changes API, schema, event, or client-facing surfaces:

```bash
make daemon-contract-test
pnpm test:clients
pnpm build
```

Always run before completion:

```bash
cd daemon
go test ./...
go mod tidy
```

## Live Smoke Policy

Automated acceptance does not require real Telegram credentials. If safe credentials are
available and meet the safe credential rule, run the hosted/test Telegram smoke path
chosen by the implementation and record redacted evidence. If they are unavailable or
unsafe, record a structured skip with:

- owner
- reason
- date
- remaining risk
- redaction status

The absence of safe credentials must not silently pass release validation.

## Rollback Check

Before closing implementation, verify rollback notes remain true:

- existing Discord and shared connector behavior still works
- Telegram setup, runtime ingress, and delivery-target eligibility can be disabled
  without deleting retained Telegram evidence
- retained redacted evidence remains readable to authorized operators until retention
  expiry
- no raw token, authorization header, credential-bearing payload, raw provider payload,
  or cross-tenant data is present in logs, events, schemas, fixtures, support output, or
  smoke evidence
- disabled Telegram cannot accept ingress, create runs, send foreground replies, or act
  as a background delivery target

## Final Residual Risk Notes

- As of 2026-05-08, automated verification uses fake Telegram transport and structured
  smoke evidence. No real Telegram provider credential is required or assumed for local
  completion.
- Safe live Telegram credentials are optional for automated completion. If they are not
  available, release review must consume the structured skip record rather than assuming
  live Telegram behavior passed.
- A rollback should not delete Telegram setup, allowment, retained update, diagnostic,
  conformance, reply, delivery, or smoke evidence; those records are the audit trail for
  the readiness decision.
- Disabling Telegram does not weaken shared route, duplicate, redaction, tenant boundary,
  or foreground/background delivery separation checks for existing connectors.
