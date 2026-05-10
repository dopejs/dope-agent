# Quickstart: Matrix Channel Connector

## Default Environment

Use the test environment by default:

```bash
make daemon-run-test
make daemon-test-status
```

Do not use production tenants, live connectors, or real Matrix bot credentials unless an
operator explicitly chooses a separate live validation path with safe authorization. Safe
authorization belongs to a non-production or approved test Matrix bot account, is scoped
to a test tenant and selected unencrypted test rooms/direct users, is explicitly approved
by an operator, redacted in all evidence, and isolated from normal production tenants.

## Implementation Order

1. Read the phase 52 artifacts:

   ```bash
   sed -n '1,260p' specs/037-whatsapp-matrix-channel/spec.md
   sed -n '1,360p' specs/037-whatsapp-matrix-channel/plan.md
   sed -n '1,360p' specs/037-whatsapp-matrix-channel/contracts/matrix-channel-connector.md
   ```

2. Reconfirm the upstream shared contracts:

   ```bash
   sed -n '1,260p' docs/channels/channel-connector-conformance.md
   sed -n '1,260p' specs/033-channel-connector-conformance/contracts/channel-connector-conformance.md
   sed -n '1,240p' docs/specs/031-hosted-credential-and-oauth-setup-wizard.md
   sed -n '1,220p' docs/specs/027-integration-health-and-permission-diagnostics.md
   sed -n '1,220p' docs/specs/013-delivery-and-notifications.md
   ```

3. Implement additively in this sequence:

   - contract/schema vocabulary for Matrix connector kind, tenant-provided bot setup,
     homeserver/bot binding, route policy, diagnostics, capability declaration, and smoke
     evidence
   - tenant-safe persistence/accessors for setup, homeserver binding, bot account
     binding, selected rooms, explicit direct allowment, dedupe, retained sync or
     transaction evidence, reply, delivery, diagnostics, conformance, and retention
   - Matrix tenant-provided bot setup through the hosted setup wizard
   - Matrix runtime under the shared connector supervisor
   - explicit direct allowment and allowed-room mention/command gating
   - unsupported setup outcomes for DopeAgent-hosted homeserver provisioning, Matrix
     account provisioning, local-only sessions, and unsupported unofficial automation
   - unsupported outcomes for encrypted rooms, undecryptable events, E2EE key/session
     management, files, voice, calls, reactions, bridge metadata, broad rich media,
     memory-based context, thinking, and incremental updates
   - homeserver/room-or-direct/event identity dedupe with retained sync or transaction
     evidence
   - final-only foreground replies in direct conversations and originating rooms
   - connector-backed Matrix delivery adapter and delivery separation evidence
   - live smoke pass or structured skip evidence
   - docs and client projections only where public surfaces changed

## Targeted Verification

Run focused daemon tests while developing:

```bash
cd daemon
go test ./internal/connectors ./internal/connectors/matrix ./internal/setupwizard ./internal/im ./internal/delivery ./internal/store ./internal/api ./internal/livevalidation
```

Required targeted cases:

- Matrix setup with tenant-provided bot account and tenant-selected homeserver
- setup completes or returns actionable terminal-state diagnostics within 5 minutes
- missing, invalid, revoked, or incomplete bot authorization
- unsupported homeserver distinguished from homeserver unreachable
- federation/sync/network failures classified
- homeserver/bot ownership mismatch and cross-tenant binding blocked
- exactly one homeserver/bot binding active per Matrix connector
- DopeAgent-hosted homeserver provisioning and Matrix account provisioning unsupported
- no selected room or direct allowment returns `action-required`
- cancelled setup preserves redacted audit evidence
- direct unencrypted text from explicitly allowed sender accepted
- direct unencrypted text from unknown or unallowed sender blocked
- selected unencrypted room with bot mention accepted
- selected unencrypted room with configured command accepted
- selected room without mention or command ignored
- unselected room blocked
- wrong homeserver or wrong bot account blocked
- encrypted room and undecryptable event unsupported
- Matrix file, voice, call, reaction-only, bridge metadata, rich media, memory-based
  context, thinking, and incremental update inputs unsupported
- duplicate inbound suppression by homeserver/room-or-direct/event ID after sync replay,
  transaction retry, reconnect, restart, or delayed event delivery
- retained Matrix sync or transaction identity as redacted evidence
- direct-message final reply success
- room final reply success
- foreground reply failure separated from assistant execution
- foreground reply outcome separated from connector-backed background delivery outcome
- background delivery success, retry, suppression, and failure
- rate-limit diagnostic evidence
- provider/homeserver/federation/network diagnostic evidence
- diagnostic stale after 15 minutes
- current diagnostic truth on failed connector actions
- 90-day evidence retention expiry
- redaction suppression for unsafe evidence
- tenant isolation and permission denial
- fake safe-live smoke pass evidence
- live Matrix smoke pass evidence or structured skip evidence
- authorized support can inspect latest Matrix diagnostic reason, remediation, freshness,
  homeserver/bot binding, selected routes, and delivery eligibility within 2 minutes

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

Automated acceptance does not require real Matrix credentials. If safe authorization is
available and meets the safe authorization rule, run the hosted/test Matrix smoke path
chosen by the implementation and record redacted evidence. If authorization is
unavailable or unsafe, record a structured skip with:

- owner
- reason
- date
- remaining risk
- redaction status

The absence of safe authorization must not silently pass release validation.

## Rollback Check

Before closing implementation, verify rollback notes remain true:

- existing Discord, Telegram, Slack, and shared connector behavior still works
- Matrix setup, runtime ingress, and delivery-target eligibility can be disabled without
  deleting retained Matrix evidence
- retained redacted evidence remains readable to authorized operators until retention
  expiry
- no Matrix access token, credential-bearing payload, raw provider payload, event body,
  room content, or cross-tenant data is present in logs, events, schemas, fixtures,
  support output, or smoke evidence
- disabled Matrix cannot accept ingress, create runs, send foreground replies, or act as
  a background delivery target

## Final Residual Risk Notes

- Automated verification covers the Matrix transport boundary with fake transport and
  local fixtures. No real Matrix homeserver, room, or bot credential is required or
  assumed for local completion.
- Safe live Matrix credentials remain optional for automated completion. If they are not
  available, release review must consume the structured skip record rather than assuming
  live Matrix behavior passed against an external homeserver.
- A rollback should not delete Matrix setup, homeserver/bot binding, route policy,
  retained sync/transaction, diagnostic, conformance, reply, delivery, or smoke evidence;
  those records are the audit trail for the readiness decision.
- Disabling Matrix does not weaken shared route, duplicate, redaction, tenant boundary,
  or foreground/background delivery separation checks for existing connectors.
