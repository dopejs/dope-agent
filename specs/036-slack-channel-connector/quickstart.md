# Quickstart: Slack Channel Connector

## Default Environment

Use the test environment by default:

```bash
make daemon-run-test
make daemon-test-status
```

Do not use production tenants, live connectors, or real Slack workspace authorization
unless an operator explicitly chooses a separate live validation path with safe
authorization. Safe authorization belongs to a non-production or approved test Slack
workspace, is scoped to a test tenant and selected test users/channels, is explicitly
approved by an operator, redacted in all evidence, and isolated from normal production
tenants.

## Implementation Order

1. Read the phase 51 artifacts:

   ```bash
   sed -n '1,260p' specs/036-slack-channel-connector/spec.md
   sed -n '1,320p' specs/036-slack-channel-connector/plan.md
   sed -n '1,320p' specs/036-slack-channel-connector/contracts/slack-channel-connector.md
   ```

2. Reconfirm the upstream shared contracts:

   ```bash
   sed -n '1,240p' docs/channels/channel-connector-conformance.md
   sed -n '1,260p' specs/033-channel-connector-conformance/contracts/channel-connector-conformance.md
   sed -n '1,240p' docs/specs/031-hosted-credential-and-oauth-setup-wizard.md
   sed -n '1,220p' docs/specs/027-integration-health-and-permission-diagnostics.md
   sed -n '1,220p' docs/specs/013-delivery-and-notifications.md
   ```

3. Implement additively in this sequence:

   - contract/schema vocabulary for Slack connector kind, hosted OAuth setup, workspace
     binding, route policy, diagnostics, capability declaration, and smoke evidence
   - tenant-safe persistence/accessors for setup, OAuth installation evidence, workspace
     binding, selected channels, explicit DM user/user-group allowment, dedupe, retained
     event evidence, reply, delivery, diagnostics, conformance, and retention
   - hosted Slack app installation/OAuth setup through the hosted setup wizard
   - Slack runtime under the shared connector supervisor
   - explicit direct-message user/user-group allowment and selected-channel mention gating
   - unsupported setup outcomes for raw tokens, signing secrets, and local-only credentials
   - unsupported outcomes for Slack marketplace publication, enterprise grid
     administration, files, voice clips, huddles, canvases, workflow buttons,
     interactive blocks, broad rich media, memory-based team context, thinking, and
     incremental updates
   - workspace/conversation/message identity dedupe with retained event evidence
   - final-only foreground replies with channel mentions rooted in threads and DMs
     replying normally
   - connector-backed Slack delivery adapter and delivery separation evidence
   - live smoke pass or structured skip evidence
   - docs and client projections only where public surfaces changed

## Targeted Verification

Run focused daemon tests while developing:

```bash
cd daemon
go test ./internal/connectors ./internal/connectors/slack ./internal/setupwizard ./internal/im ./internal/delivery ./internal/store ./internal/api ./internal/livevalidation
```

Required targeted cases:

- hosted Slack app installation/OAuth setup with required scope and selected route policy
- setup completes or returns actionable terminal-state diagnostics within 5 minutes
- missing OAuth grant, revoked installation, incomplete callback, and missing scope
- missing installation distinguished from missing scope
- workspace approval required diagnostic
- workspace mismatch and cross-tenant workspace binding blocked
- exactly one workspace bound per Slack connector
- multiple Slack connectors allowed per tenant
- raw bot token, signing secret, and local-only credential setup unsupported
- no selected channel or explicit DM allowment returns `action-required`
- cancelled setup preserves redacted audit evidence
- DM from explicitly allowed Slack user accepted
- DM from explicitly allowed Slack user-group member accepted
- DM from unknown or unallowed sender blocked
- unselected channel blocked
- selected channel without mention ignored
- selected channel with agent mention accepted
- wrong workspace blocked
- Slack marketplace publication, enterprise grid administration, file, voice clip,
  huddle, canvas, workflow button, interactive block, rich media, memory-based team
  context, thinking, and incremental update inputs unsupported
- duplicate inbound suppression by workspace/conversation/message identity after replay,
  reconnect, restart, or delayed event delivery
- retained Slack event identity as redacted evidence
- direct-message final reply success
- channel mention final reply in a thread rooted at the triggering message
- foreground reply failure separated from assistant execution
- foreground reply outcome separated from connector-backed background delivery outcome
- background delivery success, retry, suppression, and failure
- rate-limit diagnostic evidence
- provider/network/event-delivery diagnostic evidence
- diagnostic stale after 15 minutes
- current diagnostic truth on failed connector actions
- 90-day evidence retention expiry
- redaction suppression for unsafe evidence
- tenant isolation and permission denial
- fake safe-live smoke pass evidence
- live hosted smoke pass evidence or structured skip evidence
- authorized support can inspect latest Slack diagnostic reason, remediation, freshness,
  workspace binding, selected routes, and event-delivery status within 2 minutes

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

Automated acceptance does not require real Slack authorization. If safe authorization is
available and meets the safe authorization rule, run the hosted/test Slack smoke path
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

- existing Discord, Telegram, and shared connector behavior still works
- Slack setup, runtime ingress, and delivery-target eligibility can be disabled without
  deleting retained Slack evidence
- retained redacted evidence remains readable to authorized operators until retention
  expiry
- no OAuth token, installation grant, authorization header, signing secret,
  credential-bearing payload, raw provider payload, or cross-tenant data is present in
  logs, events, schemas, fixtures, support output, or smoke evidence
- disabled Slack cannot accept ingress, create runs, send foreground replies, or act as a
  background delivery target

## Final Residual Risk Notes

- Automated verification covers the Slack Web API transport boundary with local HTTP
  fixtures, including OAuth code exchange, token storage in the tenant secret store,
  `auth.test`, and `chat.postMessage`. No real Slack workspace authorization is required
  or assumed for local completion.
- Safe live Slack workspace authorization remains optional for automated completion. If
  it is not available, release review must consume the structured skip record rather
  than assuming live Slack behavior passed against an external workspace.
- A rollback should not delete Slack setup, workspace binding, route policy, retained
  event, diagnostic, conformance, reply, delivery, or smoke evidence; those records are
  the audit trail for the readiness decision.
- Disabling Slack does not weaken shared route, duplicate, redaction, tenant boundary, or
  foreground/background delivery separation checks for existing connectors.
