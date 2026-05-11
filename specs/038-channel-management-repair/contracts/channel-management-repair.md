# Contract: Channel Management And Repair UX

## Scope

This contract defines the Phase 53 product surfaces for managing and repairing existing
production channel connectors. Existing connector-specific setup, diagnostic, route,
delivery, and conformance contracts remain authoritative for provider behavior.

Required surfaces:

- API routes and schemas
- TypeScript SDK types and methods
- Web product flows
- Event/audit evidence
- Metadata-only support evidence

Out of scope:

- New connector kinds
- Channel marketplace
- Mobile push apps
- TUI support
- Message-body support evidence
- Raw provider payload display
- Autonomous provider remediation

## Permissions

| Action | Required Permission |
|--------|---------------------|
| List connectors | `credentials.inspect` |
| View connector detail | `credentials.inspect` |
| View route, reply, delivery, or support evidence metadata | `credentials.inspect` |
| View diagnostic details | `credentials.inspect` and `integrations.diagnostics.read` |
| Disable or re-enable connector | `connectors.manage` |
| Start repair | `connectors.manage` |
| Update route policy | `connectors.manage` |
| Reconnect provider authorization | `connectors.manage` and `secrets.manage` |
| Rotate supported connector credentials | `connectors.manage` and `secrets.manage` |

Unauthorized requests MUST return stable denials without exposing inaccessible connector
existence, provider identity, route details, diagnostics, or support evidence.

## List Contract

### API

`GET /v1/channel-management/connectors`

Query parameters:

- `limit`: Optional positive integer. Default is `20`.
- `cursor`: Optional opaque cursor returned by the previous page.
- `state`: Optional management state filter.
- `kind`: Optional connector kind filter.

Response shape:

```json
{
  "tenantId": "ten_123",
  "page": {
    "limit": 20,
    "nextCursor": "opaque_cursor",
    "order": "attention_disabled_ready_name_id"
  },
  "items": [
    {
      "connectorId": "slack-main",
      "connectorKind": "slack",
      "displayName": "Slack Main",
      "enablementState": "action-required",
      "setupState": "action-required",
      "healthStatus": "permission_blocked",
      "diagnosticFreshness": "fresh",
      "deliveryEligible": false,
      "nextAction": {
        "actionKind": "repair",
        "label": "Repair authorization",
        "reasonCode": "permission_missing",
        "remediationOwner": "tenant_admin"
      },
      "capabilities": {
        "disable": "supported",
        "re-enable": "supported",
        "repair": "supported",
        "reconnect": "supported",
        "credential-rotation": "supported",
        "route-edit": "supported",
        "foreground-reply-status": "supported",
        "background-delivery-status": "supported",
        "support-evidence": "supported"
      },
      "redactionStatus": "redacted",
      "updatedAt": "2026-05-10T10:00:00Z"
    }
  ]
}
```

Ordering:

1. `action-required`, `unavailable`, and `degraded`
2. `disabled`
3. `ready`
4. `displayName`
5. `connectorId`

Acceptance:

- Tenants with more than 20 connectors can reach all connectors across pages.
- Paginated traversal has no missing or duplicated connector entries.
- Page items are metadata-only and redacted.

## Detail Contract

### API

`GET /v1/channel-management/connectors/{connectorId}`

Response includes:

- Connector summary
- Setup state
- Enablement state
- Diagnostic summary
- Route policy summary
- Recent route decisions
- Foreground reply outcomes
- Background delivery outcomes
- Repair actions
- Support evidence availability
- Capability profile
- Retention expiry metadata

The response MUST NOT include channel message bodies or raw provider payloads.

## Enablement Contract

### Disable

`POST /v1/channel-management/connectors/{connectorId}/disable`

Request:

```json
{
  "reasonCode": "tenant_disabled",
  "note": "optional metadata-only operator note"
}
```

Response:

```json
{
  "connectorId": "slack-main",
  "enablementState": "disabled",
  "deliveryEligible": false,
  "auditEventId": "audit_123",
  "changedAt": "2026-05-10T10:00:00Z"
}
```

Rules:

- Requires `connectors.manage`.
- Required audit evidence must be recorded before state changes.
- If audit write fails, return a failure and leave state unchanged.
- New inbound work and new background deliveries are blocked while disabled.
- Historical evidence remains inspectable until retention expiry.

### Re-enable

`POST /v1/channel-management/connectors/{connectorId}/re-enable`

Rules:

- Requires `connectors.manage`.
- Requires current setup, health, diagnostic, and route eligibility checks.
- Diagnostics older than 15 minutes are stale and must be refreshed before completion.
- Disablement remains authoritative until validated re-enable succeeds.

## Repair Contract

`POST /v1/channel-management/connectors/{connectorId}/repair-actions`

Request:

```json
{
  "actionKind": "repair",
  "sourceDiagnosticStateId": "diag_123"
}
```

Allowed `actionKind` values:

- `repair`
- `reconnect`
- `credential-rotation`
- `route-revalidate`
- `diagnostic-rerun`
- `disable`

Rules:

- `repair`, `route-revalidate`, and `diagnostic-rerun` require `connectors.manage`.
- `reconnect` and `credential-rotation` require `connectors.manage` and
  `secrets.manage`.
- Repair actions link to setup sessions when setup is required.
- Terminal states are `ready`, `degraded`, `unavailable`, `disabled`, `cancelled`, or
  `action-required`.
- Repair completion does not implicitly re-enable a disabled connector.

## Route Policy Contract

`GET /v1/channel-management/connectors/{connectorId}/route-policy`

`PUT /v1/channel-management/connectors/{connectorId}/route-policy`

Rules:

- Reads require `credentials.inspect`.
- Updates require `connectors.manage`.
- Route updates affect only future routing and delivery decisions.
- Historical route, reply, delivery, diagnostic, and audit evidence is not rewritten.
- Unsupported route editing is reported as `unsupported`, not as a failed mutation.

## Diagnostics Contract

`GET /v1/channel-management/connectors/{connectorId}/diagnostics`

Rules:

- Requires `credentials.inspect` and `integrations.diagnostics.read`.
- Diagnostic state older than 15 minutes is `stale`.
- User-initiated repair, reconnect, rotate, disable, and re-enable actions must produce
  current diagnostic truth before presenting completion.
- Diagnostic output is redacted and metadata-only.

## Reply And Delivery Status Contract

`GET /v1/channel-management/connectors/{connectorId}/reply-outcomes`

`GET /v1/channel-management/connectors/{connectorId}/delivery-outcomes`

Rules:

- Reads require `credentials.inspect`.
- Foreground reply outcomes remain separate from agent execution and background delivery.
- Background delivery outcomes remain separate from foreground replies and setup state.
- Disabled connectors are not eligible for new background deliveries.

## Support Evidence Contract

`GET /v1/channel-management/connectors/{connectorId}/support-evidence`

Rules:

- Requires authorized tenant access and inspection permissions.
- Evidence is metadata-only.
- Message bodies and raw provider payloads are never displayed in Phase 53.
- Redaction failure suppresses unsafe detail, emits redaction-failure audit evidence, and
  shows only a generic safe classification.
- Evidence expires from normal inspection after 90 days unless an authorized tenant
  retention policy requires longer retention.

Response includes metadata references for:

- Setup state
- Diagnostics
- Enablement transitions
- Repair actions
- Route decisions
- Reply outcomes
- Delivery outcomes
- Audit records
- Redaction status
- Retention expiry

## Mutation Serialization Contract

The daemon MUST serialize these mutation classes per connector:

- Disable
- Re-enable
- Route edit
- Repair start/completion
- Reconnect
- Credential rotation
- Delivery eligibility changes

Disablement takes precedence over concurrent or in-progress repair, reconnect,
credential rotation, route edit, inbound processing, and background delivery eligibility
until a later validated re-enable succeeds.

## Event And Audit Contract

Required audit-visible actions:

- Connector list/detail denial
- Disable
- Re-enable
- Route change
- Repair start
- Repair completion
- Reconnect
- Credential rotation
- Permission denial
- Redaction failure
- Audit-write failure
- Retention application

Required audit write failure blocks connector mutation and leaves state unchanged.

Event/schema additions should remain additive. Candidate event names:

- `connector.management_disabled`
- `connector.management_reenabled`
- `connector.management_repair_started`
- `connector.management_repair_completed`
- `connector.management_route_policy_updated`
- `connector.management_support_evidence_generated`
- `connector.management_retention_applied`
- `connector.management_audit_failed_closed`

## SDK Contract

The TypeScript SDK MUST expose typed resources and methods for:

- List channel connectors with `{ limit, cursor, state, kind }`
- Get connector management detail
- Disable connector
- Re-enable connector
- Start repair action
- Get/update route policy
- List diagnostics
- List reply outcomes
- List delivery outcomes
- Get support evidence

SDK tests must prove URL construction, tenant headers, request body shape, and response
typing for representative flows.

## Web Contract

The web product surface MUST expose:

- Paginated connector fleet list with deterministic ordering.
- Empty state for tenants with no configured production connectors.
- Connector detail view with setup, health, diagnostics, routes, reply status, delivery
  status, repair actions, and metadata-only support evidence.
- Disable and re-enable controls with permission-denied and audit-failure states.
- Repair and reconnect paths from diagnostic next steps.
- Unsupported action states for connector kinds that do not support a capability.
- Foreground reply and background delivery status as separate sections.
- Metadata-only support evidence with redaction/retention state.

Web tests must cover at least one representative connector for list, detail, disable,
re-enable, repair, diagnostics, and support evidence flows.

## Compatibility Requirements

- Existing `/v1/connectors` behavior and provider-specific setup routes must remain
  compatible unless intentionally extended additively.
- Existing Discord, Telegram, Slack, and Matrix setup, diagnostic, ingress, delivery,
  conformance, smoke, and retention behavior remains authoritative.
- Rollback can hide new management routes and web controls without disabling existing
  connector runtime behavior.
