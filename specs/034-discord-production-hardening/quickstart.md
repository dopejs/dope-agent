# Quickstart: Discord Production Hardening

## Default Environment

Use the test environment by default:

```bash
make daemon-run-test
make daemon-test-status
```

Do not use production tenants, live connectors, or real Discord credentials unless an
operator explicitly chooses a separate live validation path with safe credentials. Safe
credentials are non-production, scoped to a test tenant and test destinations, explicitly
approved by an operator, redacted in all evidence, and isolated from normal production
tenants.

## Implementation Order

1. Read the phase 49 artifacts:

   ```bash
   sed -n '1,220p' specs/034-discord-production-hardening/spec.md
   sed -n '1,260p' specs/034-discord-production-hardening/plan.md
   sed -n '1,260p' specs/034-discord-production-hardening/contracts/discord-production-hardening.md
   ```

2. Reconfirm the phase 48 shared contract:

   ```bash
   sed -n '1,220p' docs/channels/channel-connector-conformance.md
   sed -n '1,260p' specs/033-channel-connector-conformance/contracts/channel-connector-conformance.md
   ```

3. Implement additively in this sequence:

   - contract/schema vocabulary for Discord hosted setup and diagnostics
   - tenant-safe persistence/accessors for setup validation and repair evidence
   - Discord readiness gating for degraded/needs-repair setup state
   - Discord diagnostic mapping, freshness, retention, and redaction behavior
   - route, dedupe, reply progression, reply failure, reconnect, and rate-limit evidence
   - live smoke pass or structured skip evidence
   - docs and client projections only where public surfaces changed

## Targeted Verification

Run focused daemon tests while developing:

```bash
cd daemon
go test ./internal/connectors ./internal/connectors/discord ./internal/im ./internal/store ./internal/api
```

Required targeted cases:

- valid Discord setup with explicit validated destinations
- invalid/revoked/missing credential redaction
- no explicit hosted guild/channel destinations saves degraded/needs repair
- partially invalid destinations save degraded/needs repair
- DM enabled/disabled behavior
- mention-required behavior and mention normalization
- allowed and blocked guild/channel routing
- duplicate inbound suppression after replay/reconnect
- gateway disconnect and reconnect diagnostic evidence
- rate-limit diagnostic evidence
- reply progression degradation to a safer mode
- reply send/edit failure separated from assistant execution
- foreground reply outcome separated from connector-backed background delivery outcome
- diagnostic stale after 15 minutes
- current diagnostic truth on failed connector actions
- 90-day evidence retention expiry
- redaction suppression for unsafe evidence
- existing local Discord config compatibility
- live hosted smoke pass or structured skip

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

Automated acceptance does not require real Discord credentials. If safe credentials are
available and meet the safe credential rule, run the hosted/test Discord smoke path chosen
by the implementation and record redacted evidence. If they are unavailable or unsafe,
record a structured skip with:

- owner
- reason
- date
- remaining risk
- redaction status

The absence of safe credentials must not silently pass release validation.

## Rollback Check

Before closing implementation, verify rollback notes remain true:

- existing local Discord gateway usage still works
- hosted-ready gating can be disabled by treating `connectors.discord.hostedReadiness`
  and `/v1/connectors/{connectorId}/discord-setup` as advisory while leaving retained
  evidence in place
- retained redacted evidence remains readable to authorized operators until retention
  expiry
- no raw token, authorization header, credential-bearing payload, or cross-tenant data is
  present in logs, events, schemas, fixtures, support output, or smoke evidence

## Final Residual Risk Notes

- Safe live Discord credentials are optional for automated completion. If they are not
  available, release review must consume the structured skip record rather than assuming
  live Discord behavior passed.
- A rollback should not delete `discord_hosted_setups`, `discord_destination_validations`,
  `discord_smoke_evidence`, connector diagnostic, conformance, or reply-failure evidence;
  those records are the audit trail for the readiness decision.
- Disabling hosted-ready gating does not weaken route, duplicate, redaction, tenant
  boundary, or foreground/background delivery separation checks.
