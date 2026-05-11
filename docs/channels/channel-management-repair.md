# Channel Management And Repair

Roadmap 53 adds a tenant-scoped management surface for existing production channel
connectors. It does not add a new connector kind or replace Discord, Telegram, Slack, or
Matrix setup, route, diagnostic, delivery, or conformance contracts.

## Surfaces

- API: `/v1/channel-management/connectors`
- TypeScript SDK: channel management connector methods on `DopeClient`
- Web: channel management feature panel

## Permissions

- Redacted list, detail, route, reply, delivery, and support evidence inspection require
  `credentials.inspect`.
- Diagnostic inspection also requires `integrations.diagnostics.read`.
- Disable, re-enable, route edits, and repair starts require `connectors.manage`.
- Reconnect and credential rotation also require `secrets.manage`.

Unauthorized responses must not expose connector existence, provider identity, route
details, diagnostics, raw provider payloads, message bodies, or support evidence.

## Fleet Inspection

The connector list defaults to 20 items per page and uses deterministic ordering:
action-required, unavailable, and degraded connectors first, then disabled connectors,
then ready connectors. Items are stable by display name and connector id inside each
state group.

The detail view aggregates connector state, setup state, diagnostics, route policy,
foreground reply outcomes, background delivery outcomes, repair actions, capability
support, and metadata-only support evidence.

## Disable And Re-enable

Disable is the containment action. It blocks new inbound work and new background delivery
eligibility while preserving prior setup, route, diagnostic, reply, delivery, support,
and audit evidence. Re-enable requires current setup, health, diagnostics, and route
eligibility checks. Diagnostic evidence older than 15 minutes is stale.

Connector management mutations fail closed if required audit evidence cannot be written.
Disablement remains authoritative over concurrent repair, reconnect, credential
rotation, route edits, inbound processing, and delivery eligibility until a later
validated re-enable succeeds.

## Repair

Repair actions start from diagnostic next steps and may link to setup, reconnect,
supported credential rotation, route revalidation, or diagnostic rerun behavior.
Reconnect and credential rotation require both `connectors.manage` and `secrets.manage`.
Repair does not implicitly re-enable a disabled connector.

Terminal repair states are ready, degraded, unavailable, disabled, cancelled, or
action-required.

## Route And Delivery Visibility

Route policy changes affect only future routing and delivery decisions. Historical
message, route, reply, delivery, diagnostic, and audit evidence is not rewritten.
Foreground reply outcomes remain separate from agent execution and background delivery
outcomes.

## Support Evidence

Support evidence is metadata-only. Message bodies and raw provider payloads are never
shown in this phase. Evidence that cannot be confidently redacted is suppressed and
recorded as redaction-failure audit evidence.

Connector diagnostic, repair, routing, reply, delivery, and support evidence expires
from normal inspection after 90 days unless an authorized tenant retention policy
requires longer retention.

## Observability

Every management mutation records redacted audit evidence before state changes are
applied. Permission denials are recorded best-effort with tenant, connector, action,
permission gate, denial outcome, and redaction status. Support reconstruction relies on
metadata IDs for diagnostics, repair actions, route decisions, foreground replies,
background deliveries, and audit records rather than channel content.

Live connector validation is intentionally out of scope for the default test workflow.
Skipped live checks must record connector kind, owner, reason, remaining risk, timestamp,
retention expiry, and redaction status in the roadmap quickstart.

## Rollback

Rollback may hide channel management API routes, SDK/web entry points, repair actions,
route-edit controls, and support evidence exports. Existing connector runtime, setup,
diagnostics, ingress, delivery, conformance behavior, and already-written evidence remain
intact until retention expiry.

If rollback is needed, disable the route registration and web entry point first, then
leave additive tables in place for retention-managed audit/support evidence. The schema
migration is additive; data rollback is backup/restore only if operators need to remove
already-recorded channel management evidence.
