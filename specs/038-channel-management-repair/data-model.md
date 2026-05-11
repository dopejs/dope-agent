# Data Model: Channel Management And Repair UX

## Channel Connector

Represents one tenant-owned production connector managed by the feature.

### Fields

- `tenantId`: Tenant that owns the connector.
- `connectorId`: Stable connector identity, unique within tenant scope.
- `connectorKind`: Provider kind such as `discord`, `telegram`, `slack`, or `matrix`.
- `displayName`: User-visible connector label.
- `enablementState`: `ready`, `disabled`, `action-required`, `unavailable`, or
  `degraded` as projected for management.
- `setupState`: Current hosted setup terminal or intermediate state from the connector's
  authoritative setup source.
- `healthStatus`: Shared connector lifecycle or diagnostic status.
- `diagnosticFreshness`: `fresh` or `stale`; stale when diagnostic evidence is older
  than 15 minutes.
- `deliveryEligible`: Whether the connector may be used for new background deliveries.
- `capabilities`: Supported, limited, or unsupported management actions.
- `redactionStatus`: Redaction state for displayed metadata.
- `createdAt`, `updatedAt`: Connector timestamps.

### Relationships

- Has zero or one latest `Connector Diagnostic Summary`.
- Has one `Connector Enablement State`.
- Has zero or more `Route Policy` entries or provider-specific route policy projections.
- Has zero or more `Repair Action` records.
- Has zero or more `Foreground Reply Outcome` and `Background Delivery Outcome`
  projections.
- Has zero or more `Support Evidence Bundle` records until retention expiry.

### Validation Rules

- Reads are tenant-scoped and require `credentials.inspect`; diagnostic details require
  `integrations.diagnostics.read`.
- Mutations require `connectors.manage`; reconnect and credential rotation also require
  `secrets.manage`.
- A connector may not become delivery eligible or accept new inbound work while disabled.
- Connector detail must not include channel message bodies or raw provider payloads.

## Channel Connector Page

Represents one page of tenant connector list results.

### Fields

- `tenantId`: Tenant whose connectors are listed.
- `items`: Ordered `Channel Connector` projections.
- `page.limit`: Default `20`; may be capped by implementation policy.
- `page.nextCursor`: Opaque cursor when additional results exist.
- `page.order`: Default ordering policy identifier.

### Ordering

Default order is:

1. `action-required`, `unavailable`, and `degraded`
2. `disabled`
3. `ready`
4. Stable `displayName`
5. Stable `connectorId`

### Validation Rules

- Page traversal must not omit or duplicate connectors in normal paginated reads.
- Unauthorized reads must deny without exposing inaccessible tenant connector existence.

## Connector Enablement State

Represents whether new inbound work and background delivery use are allowed.

### Fields

- `connectorId`, `tenantId`
- `state`: `enabled`, `disabled`, or `blocked`
- `reasonCode`: Optional stable reason for disabled or blocked state.
- `changedByPrincipalId`: Actor for the latest transition.
- `changedAt`: Transition time.
- `validatedAt`: Last validation time for re-enable.
- `auditEventId`: Required audit evidence for the latest mutation.

### State Transitions

- `enabled` -> `disabled`: Authorized disable action. Blocks new inbound work and new
  background delivery eligibility.
- `disabled` -> `enabled`: Authorized re-enable only after current setup, health,
  diagnostics, and route eligibility are safe enough for declared capabilities.
- Any mutation -> unchanged: Required audit evidence cannot be recorded.

### Validation Rules

- Disablement takes precedence over concurrent repair, reconnect, credential rotation,
  route edit, inbound processing, and background delivery eligibility until validated
  re-enable succeeds.
- Mutations serialize per connector.
- Required audit write failure leaves state unchanged.

## Connector Capability

Represents a management action or behavior that a connector kind supports, limits, or
does not support.

### Fields

- `connectorKind`
- `capability`: `disable`, `re-enable`, `repair`, `reconnect`, `credential-rotation`,
  `route-edit`, `foreground-reply-status`, `background-delivery-status`,
  `support-evidence`
- `support`: `supported`, `limited`, or `unsupported`
- `reasonCode`: Required when `limited` or `unsupported`
- `updatedAt`

### Validation Rules

- Unsupported actions must be shown as unsupported instead of available or failed.
- Credential rotation requires both connector support and caller permissions.

## Repair Action

Represents a user-initiated recovery path from diagnostic next step to setup, reconnect,
credential rotation, route revalidation, or diagnostic rerun.

### Fields

- `repairActionId`
- `tenantId`, `connectorId`, `connectorKind`
- `actorPrincipalId`
- `actionKind`: `repair`, `reconnect`, `credential-rotation`, `route-revalidate`,
  `diagnostic-rerun`, or `disable`
- `sourceDiagnosticStateId`
- `setupSessionId`: Present when repair links to setup.
- `status`: `in-progress`, `ready`, `degraded`, `unavailable`, `disabled`,
  `cancelled`, or `action-required`
- `retrySafety`
- `remediationOwner`
- `startedAt`, `completedAt`
- `auditEventId`
- `redactionStatus`

### State Transitions

- `in-progress` -> one terminal state.
- `in-progress` -> `cancelled` when user cancels or setup session is cancelled.
- Any action -> unchanged when permission, tenant, audit, or connector-state checks fail.

### Validation Rules

- Repair starts require `connectors.manage`.
- Reconnect or credential rotation requires `connectors.manage` plus `secrets.manage`.
- Repair must carry diagnostic reason and redacted evidence link.
- Repair completion must not implicitly re-enable a disabled connector.

## Route Policy

Represents supported tenant-scoped sender, conversation, room, channel, invocation, and
delivery-target configuration for a connector.

### Fields

- `routePolicyId`
- `tenantId`, `connectorId`
- `eligibleSenders`
- `eligibleConversations`
- `eligibleRooms`
- `eligibleChannels`
- `invocationGates`
- `backgroundDeliveryEligible`
- `validationState`: `valid`, `partial`, `stale`, `blocked`, or `missing_permission`
- `reasonCode`
- `validatedAt`
- `auditEventId`
- `redactionStatus`

### Validation Rules

- Route updates require `connectors.manage`.
- Route changes affect future routing and delivery only; historical evidence is not
  rewritten.
- Removing the only allowed route must produce an explicit blocked or action-required
  management state where appropriate.

## Routing Decision

Represents metadata-only evidence for an inbound provider event routing decision.

### Fields

- `routingDecisionId`
- `tenantId`, `connectorId`, `connectorKind`
- `outcome`: `accepted`, `ignored`, `blocked`, `duplicate`, `unsupported`, `failed`, or
  `disabled`
- `reasonCode`
- `occurredAt`
- `safeEvidence`
- `redactionStatus`
- `retentionExpiresAt`

### Validation Rules

- Disabled connector inbound events must not create agent runs.
- Support output must not include message bodies or raw provider payloads.

## Foreground Reply Outcome

Represents the result of sending a reply for an accepted inbound channel message.

### Fields

- `replyOutcomeId`
- `tenantId`, `connectorId`
- `routingDecisionId`
- `status`: `sent`, `retrying`, `failed`, `suppressed`, or `unsupported`
- `reasonCode`
- `occurredAt`
- `safeEvidence`
- `redactionStatus`
- `retentionExpiresAt`

### Validation Rules

- Must be inspectable separately from agent execution and background delivery outcomes.
- Must be metadata-only in support evidence.

## Background Delivery Outcome

Represents the result of scheduled or workflow-originated work delivered through a
connector.

### Fields

- `deliveryOutcomeId`
- `tenantId`, `connectorId`
- `deliveryTargetId`
- `status`: `sent`, `retrying`, `failed`, `suppressed`, `blocked`, or `unsupported`
- `reasonCode`
- `occurredAt`
- `safeEvidence`
- `redactionStatus`
- `retentionExpiresAt`

### Validation Rules

- Disabled connectors must not be eligible for new background deliveries.
- Delivery status is independent from foreground replies and setup state.

## Connector Diagnostic Summary

Represents current diagnostic state for management and repair decisions.

### Fields

- `diagnosticStateId`
- `tenantId`, `connectorId`
- `status`
- `reasonCode`
- `remediationOwner`
- `retrySafety`
- `freshnessState`
- `evidenceTimestamp`
- `staleAfter`
- `retentionExpiresAt`
- `redactionStatus`

### Validation Rules

- Diagnostic evidence older than 15 minutes is stale.
- User-initiated repair, reconnect, rotate, disable, or re-enable must produce current
  diagnostic truth before presenting completion.
- Diagnostic details require `integrations.diagnostics.read`.

## Support Evidence Bundle

Represents metadata-only incident evidence for authorized support.

### Fields

- `supportEvidenceId`
- `tenantId`, `connectorId`
- `generatedByPrincipalId`
- `generatedAt`
- `currentState`
- `stateTransitions`
- `diagnosticRefs`
- `repairRefs`
- `routingDecisionRefs`
- `replyOutcomeRefs`
- `deliveryOutcomeRefs`
- `auditRefs`
- `redactions`
- `retentionExpiresAt`
- `redactionStatus`

### Validation Rules

- Requires authorized tenant access and inspection permissions.
- Must not display channel message bodies or raw provider payloads.
- If redaction cannot be confidently applied, unsafe detail is suppressed and
  redaction-failure audit evidence is emitted.
- Expires from normal inspection after 90 days unless an authorized tenant policy
  requires longer retention.

## Connector Audit Record

Represents tenant-scoped accountability evidence for management actions and denials.

### Fields

- `auditEventId`
- `tenantId`, `connectorId`
- `principalId`
- `action`
- `permissionGate`
- `outcome`: `succeeded`, `denied`, or `failed_closed`
- `reasonCode`
- `createdAt`
- `redactionStatus`

### Validation Rules

- Enablement, disablement, re-enablement, route changes, repair starts, repair
  completions, reconnects, credential rotations, permission denials, redaction failures,
  and audit-write failures must be audit-visible.
- Required audit write failure blocks connector mutation and leaves state unchanged.
