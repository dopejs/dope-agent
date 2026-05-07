# Contract: Channel Connector Conformance

## Scope

This contract defines the required behavior for hosted channel connectors in Phase 48.
It applies to the shared fake connector conformance matrix and the Discord regression
baseline. Telegram, Slack, and other future providers consume this contract in later
provider-specific phases.

## Core Invariants

Every hosted-ready connector MUST pass these invariants:

| Invariant | Required Evidence |
|-----------|-------------------|
| Tenant ownership | Connector, account binding, messages, diagnostics, conformance results, and delivery outcomes are scoped to the active tenant |
| Permission gating | Unauthorized reads and mutations fail closed without revealing inaccessible tenant or connector existence |
| Redaction | No raw tokens, secret values, authorization headers, credential-bearing payloads, or cross-tenant data appear in APIs, events, logs, fixtures, support output, or conformance evidence |
| Active-tenant account binding | Runtime inbound, foreground reply, diagnostics, and connector-backed delivery resolve only through the active tenant's account binding |
| Inbound identity | Dedupe identity uses tenant, connector account, channel/conversation, and provider message ID, or an explicit equivalent durable rule |
| Durable dedupe | Provider retries and restart replays produce at most one assistant reply per original inbound message |
| Stable routing decisions | Direct, group, mention, room, and thread inputs produce accepted, ignored, blocked, duplicate, unsupported, or failed outcomes with stable reason codes |
| Minimum foreground reply | Accepted foreground messages support at least final-only reply delivery |
| Required diagnostics | Required diagnostic classifications are stable, timestamped, freshness-aware, redacted, and remediation-bearing |
| Delivery separation | Foreground reply outcomes and background delivery outcomes remain separate even when transport mechanics are shared |

## Provider-Specific Surfaces

Provider-specific surfaces MAY be `supported`, `limited`, or `unsupported` when explicit:

- direct messages
- group messages
- mention gating
- rooms
- threads
- thinking visibility
- incremental visible updates
- rich media
- placeholder/card/update mechanics
- provider-specific stop controls

Unsupported or limited surfaces MUST NOT weaken any core invariant.

## Lifecycle And Health States

Required connector lifecycle and diagnostic vocabulary:

| State | Meaning |
|-------|---------|
| `configured` | Tenant-owned connector configuration exists but runtime has not started |
| `disabled` | Connector is intentionally disabled and cannot accept ingress or delivery |
| `starting` | Connector runtime is starting or reconnecting |
| `healthy` | Connector can accept supported ingress/reply/delivery paths |
| `degraded` | Connector is partially available and may have limited surfaces |
| `failed` | Connector cannot perform required behavior without remediation |
| `permission_blocked` | Connector is blocked by tenant or provider permission state |
| `rate_limited` | Provider rate limit prevents normal operation |
| `unsupported_capability` | Requested provider-specific surface is explicit unsupported/limited behavior |

Existing internal statuses may remain backward compatible when public projections map
them into this vocabulary.

## Inbound Routing Outcomes

| Outcome | Reply Allowed | Meaning |
|---------|---------------|---------|
| `accepted` | Yes | Message satisfies tenant, account, permission, route, and identity requirements |
| `ignored` | No | Connector deliberately ignores a valid event, such as unmentioned group traffic |
| `blocked` | No | Tenant, account, channel, allowlist, or permission rule rejects the event |
| `duplicate` | No | Durable identity already exists for this inbound message |
| `unsupported` | No | Provider-specific surface is unsupported or limited for this connector |
| `failed` | No | Connector or daemon could not classify or route the event safely |

## Inbound Message Identity

Standard identity fields:

- `tenantId`
- `connectorAccountId`
- `channelOrConversationId`
- `providerMessageId`

Equivalent durable identity rules are allowed only when:

- provider mechanics cannot supply the standard fields;
- the rule is documented in the connector capability profile;
- conformance proves tenant scope, account binding, and duplicate suppression.

## Reply Progression Levels

| Level | Required Behavior |
|-------|-------------------|
| `final_only` | Send one final reply after assistant work completes |
| `thinking_plus_final` | Surface safe thinking state, then send one final reply |
| `thinking_plus_incremental` | Surface thinking, send an initial visible reply, update safely, then finalize |
| `unsupported` | Valid only for provider-specific progression surfaces; accepted foreground messages still require `final_only` |

Connectors that cannot throttle visible updates safely must degrade to a safer level.

## Diagnostic Classifications

Required diagnostic reason codes:

- `auth_missing`
- `permission_missing`
- `rate_limited`
- `provider_unavailable`
- `network_failed`
- `unsupported_capability`
- `blocked_route`
- `duplicate_inbound`
- `reply_failed`
- `unknown_connector_failure`

Every diagnostic result includes:

- tenant and connector scope;
- connector account binding summary;
- remediation owner;
- user-visible severity;
- retry safety where relevant;
- evidence timestamp;
- freshness state;
- redaction status;
- retention expiry.

Freshness and retention:

- Cached connector diagnostics may be shown but become stale after 15 minutes.
- Connector action failures must produce current diagnostic truth before remediation is
  shown.
- Connector conformance results, diagnostics, and redaction-failure outcomes use 90-day
  default retention unless an authorized longer policy applies.

## API And Schema Expectations

Add or extend JSON schema resources additively for:

- connector capability profile;
- connector diagnostic state;
- connector conformance result;
- connector account binding summary;
- routing decision or ingress outcome projection when exposed publicly;
- foreground reply outcome projection when exposed publicly;
- connector-backed background delivery linkage when not already represented by delivery
  resources.

Existing connector list/resource schemas remain backward compatible. New fields are
additive or new resources are introduced; current required fields are not repurposed.

## Event Expectations

Connector events must remain redacted and tenant-scoped. New or extended event families
must cover:

- conformance result recorded;
- diagnostic state changed;
- diagnostic redaction failed;
- inbound duplicate detected;
- route blocked or unsupported;
- foreground reply failed;
- delivery separation evidence when connector-backed delivery is used.

Events must update `schemas/events/` and contract tests together.

## Persistence Expectations

Additive persistence must support:

- standard inbound message identity or explicit equivalent durable rule;
- tenant-scoped conformance results;
- tenant-scoped diagnostic states and evidence;
- retention expiry;
- redaction-failure outcomes;
- migration compatibility for existing connector messages and Discord records.

No destructive migration is required for Phase 48.

## Verification Matrix

The shared fake connector matrix MUST include:

- core invariant pass case;
- each core invariant failure case;
- provider-specific supported, limited, and unsupported surfaces;
- direct, group, mention, room, and thread routing outcomes;
- duplicate retry and restart replay;
- missing or equivalent message identity;
- final-only, thinking-plus-final, and thinking-plus-incremental progression;
- unsafe incremental update degradation;
- foreground reply success/failure;
- connector-backed background delivery success/failure/suppression;
- diagnostics for every required reason code;
- stale diagnostics after 15 minutes;
- current diagnostic truth on connector action failure;
- redaction success and redaction fail-closed;
- 90-day retention expiry;
- tenant isolation and permission denial.

Discord regression MUST either pass required contract areas or record explicit
unsupported/limited provider-specific surfaces. No Telegram, Slack, or other
non-Discord regression is required for Phase 48.
