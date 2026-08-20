# Contract: Matrix Channel Connector

This contract is the planning handoff for Roadmap 52. It specializes the shared phase 48
channel connector conformance contract for the new Matrix connector.

## Hosted Readiness Gates

| Gate | Required Result | Failure State | Verification |
|------|-----------------|---------------|--------------|
| Provider selection | Matrix is the chosen provider and WhatsApp is rejected for phase 52 | Planning blocked | Spec clarification and provider-risk research |
| Tenant-provided bot setup | Tenant submits Matrix bot authorization for a tenant-selected homeserver without exposing raw credentials | `action-required` or `degraded` with `auth_missing` diagnostic | Fake setup tests; redaction tests |
| Unsupported setup modes | Kura-hosted homeserver provisioning, Matrix account provisioning, local-only sessions, bridge automation, and unsupported unofficial automation are rejected | `unsupported` setup outcome or `action-required` diagnostic | Unsupported setup tests |
| Homeserver and bot binding | Exactly one tenant-selected homeserver and bot account are bound to one connector | `degraded` or `action-required` with repair evidence | Binding cardinality tests; tenant isolation tests |
| Homeserver support | Homeserver is reachable and supports required authenticated client, sync, room, and send behavior | `unavailable`, `degraded`, or `action-required` | Homeserver capability tests |
| Route policy | At least one direct allowment or selected room policy exists before ingress can create runs | `action-required` until configured | Route policy validation tests |
| Direct sender authorization | Direct messages create runs only for eligible direct senders under tenant policy | `blocked` route outcome | Direct allowment tests |
| Room invocation gate | Room messages create runs only when the room is selected and the message contains a bot mention or configured command | `ignored` or `blocked` route outcome | Room gate tests |
| Unencrypted text scope | Only unencrypted text messages can be accepted | `unsupported` for encrypted, undecryptable, media, calls, voice, reactions, or bridge-specific surfaces | Unsupported surface tests |
| Durable inbound identity | Dedupe key is tenant, connector, homeserver, room/direct conversation, and Matrix event ID; sync or transaction identity is retained as evidence | `duplicate` route outcome on replay | Dedupe/reconnect/sync replay tests |
| Conformance core invariants | Every phase 48 core invariant passes | Not ready | Matrix conformance regression |
| Optional surfaces | Every optional Matrix surface is supported, limited, or unsupported explicitly | Not ready if silent | Capability profile tests |
| Live hosted smoke | Passed with safe Matrix credentials, or structured skip exists | Release risk remains visible | Live smoke or skip tests |

## Setup State Contract

| Input Condition | Terminal State | Ready? | Required Evidence |
|-----------------|----------------|--------|-------------------|
| Bot authorization valid, homeserver reachable and supported, bot binding valid, and valid route policy exists | `ready` | Yes, if conformance gates also pass | Validation timestamp, homeserver/bot binding summary, selected route summary |
| Bot authorization missing, invalid, revoked, or incomplete | `action-required` | No | Missing or invalid authorization remediation evidence |
| Required Matrix room permission missing | `action-required` or `degraded` | No | Missing permission diagnostic and remediation owner |
| Homeserver unreachable or local network validation fails | `unavailable` | No | Homeserver/network diagnostic and retry guidance |
| Homeserver behavior unsupported for required phase 52 operations | `action-required` or `degraded` | No | Unsupported homeserver diagnostic and remediation evidence |
| Ownership mismatch or ambiguous tenant ownership | `action-required` | No | Ownership mismatch diagnostic |
| Valid bot authorization but no selected room or explicit direct allowment | `action-required` | No | Missing route policy remediation evidence |
| Selected room inaccessible, stale, encrypted, missing membership, or missing send permission | `degraded` or `action-required` | No | Per-room validation result |
| Matrix provider/homeserver rate-limited | `degraded` | No until recovered | Rate-limit diagnostic and retry-after evidence when available |
| User cancels setup | `cancelled` | No | Redacted audit evidence, no unrelated state deleted |
| Kura-hosted homeserver or account provisioning requested | `action-required` or unsupported setup outcome | No | Unsupported setup remediation evidence |

Terminal states must reuse the hosted setup wizard vocabulary: `ready`, `degraded`,
`unavailable`, `cancelled`, and `action-required`.

## Homeserver And Route Policy Contract

Matrix route policy must produce redacted tenant-scoped results for:

- exactly one active homeserver/bot binding per Matrix connector
- tenant-selected homeserver reachability and support status
- tenant-provided bot account identity and authorization status
- selected Matrix rooms
- explicitly allowed direct users or direct conversations
- bot mention and configured command gate status
- encrypted-room unsupported status
- blocked or missing route policy
- stale, inaccessible, missing-membership, or invalid room permissions
- delivery-target eligibility

Route evidence must include enough safe metadata for repair without exposing access
tokens, raw provider payloads, message bodies, room content, inaccessible event content,
or cross-tenant state.

## Routing Contract

| Scenario | Required Outcome |
|----------|------------------|
| Unencrypted direct text from explicitly allowed Matrix sender | Accepted and eligible for one agent run |
| Unencrypted direct text from unknown or unallowed sender | Blocked with no assistant reply |
| Room not selected | Blocked with no assistant reply |
| Selected unencrypted room without bot mention or configured command | Ignored with no public reply |
| Selected unencrypted room with bot mention | Accepted; mention artifact normalized before assistant handling |
| Selected unencrypted room with configured command | Accepted; command artifact normalized before assistant handling |
| Wrong homeserver or wrong bot account for connector | Blocked with no assistant reply |
| Disabled connector | Blocked or ignored with no assistant reply |
| Encrypted room or undecryptable event | Unsupported with no agent run |
| File, voice, call, reaction-only event, bridge-specific metadata, rich media, or unsupported surface | Unsupported with no agent run |
| Duplicate Matrix homeserver/room-or-direct/event ID after sync replay, transaction retry, reconnect, restart, or delayed delivery | Duplicate with at most one assistant reply |
| Missing tenant/homeserver/conversation/event identity | Failed closed unless an explicit equivalent identity rule is documented and tested |

## Durable Identity Contract

Matrix inbound dedupe identity is:

- `tenantId`
- `connectorId`
- `homeserverId`
- `conversationId` (room or direct conversation)
- `matrixEventId`

Matrix sync batch identity and transaction identity must be retained as redacted provider
delivery evidence for diagnostics, replay, delayed event delivery, reconnect, and support
inspection. They must not replace homeserver/conversation/event identity as the canonical
duplicate suppression key.

## Reply And Delivery Contract

| Scenario | Required Outcome |
|----------|------------------|
| Accepted direct message completes assistant work | Send at least one final Matrix reply in the direct conversation |
| Accepted room message completes assistant work | Send at least one final Matrix reply in the originating room |
| Assistant completes but Matrix reply fails | Assistant execution and Matrix reply outcomes remain separate |
| Matrix rate limits foreground reply | Diagnostic reason recorded; retry/degradation evidence retained |
| Matrix selected as background delivery target | Delivery success, retry, suppression, or failure is recorded separately from foreground replies |
| Background delivery fails after work succeeds | Work result remains separate from delivery failure truth |

Phase 52 reply progression declaration:

- Final-only foreground replies: required.
- Direct-message normal replies: required.
- Room replies to accepted room events: required when Matrix delivery permits it.
- Thinking visibility: unsupported unless explicitly recut.
- Incremental visible updates: unsupported unless explicitly recut.
- Rich message forms, reactions, media, voice, calls, and bridge-specific controls:
  unsupported unless explicitly recut.
- Connector-backed background delivery: supported only through explicit delivery-target
  eligibility and separate delivery outcome evidence.

## Diagnostic Mapping

| Matrix Condition | Shared Reason Code | Lifecycle/Terminal State | Remediation Owner |
|------------------|--------------------|--------------------------|-------------------|
| Bot authorization missing, invalid, incomplete, or revoked | `auth_missing` | `action-required` | `product_user` |
| Required room permission, membership, or send permission missing | `permission_missing` | `action-required` or `degraded` | `tenant_admin` |
| Homeserver account or room ownership mismatch | `permission_missing` | `action-required` | `tenant_admin` |
| Unknown sender, unallowed direct sender, unselected room, wrong homeserver, or wrong bot account | `blocked_route` | `degraded` | `tenant_admin` |
| Selected room lacks bot mention or configured command | `blocked_route` | `degraded` | `none_required` |
| Matrix rate limit on setup, sync, reply, or delivery handling | `rate_limited` | `degraded` | `provider` |
| Homeserver or Matrix provider unavailable | `provider_unavailable` | `unavailable` | `provider` |
| Local network unavailable, sync failed, reconnect failed, federation failed, or retry exhausted | `network_failed` | `unavailable` or `degraded` | `operator` |
| Duplicate inbound homeserver/room-or-direct/event after replay | `duplicate_inbound` | `degraded` | `none_required` |
| Reply send failed after assistant work | `reply_failed` | `degraded` | `operator` |
| Kura-hosted homeserver provisioning, account provisioning, encrypted rooms, undecryptable events, E2EE key/session management, files, voice, calls, reactions, bridge automation, rich media, thinking, or incremental update | `unsupported_capability` | `unsupported_capability` | `none_required` |
| Unclassified Matrix connector failure | `unknown_connector_failure` | `degraded` | `operator` |

## Freshness, Retention, And Redaction

- Cached Matrix diagnostics may be shown, but diagnostics older than 15 minutes must be
  marked `stale`.
- Matrix actions that fail must produce current diagnostic truth before remediation is
  shown.
- Matrix setup evidence, route policy evidence, diagnostic evidence, conformance
  evidence, smoke evidence, retained sync/transaction evidence, and redaction-failure
  outcomes expire from normal inspection after 90 days unless an authorized longer
  retention policy applies.
- If evidence cannot be confidently redacted, detailed evidence is suppressed and a safe
  generic classification plus redaction-failure marker is recorded for authorized
  operators.

## Safe Live Authorization Rule

Safe live Matrix authorization must satisfy all of these conditions before live smoke can
run:

- authorization belongs to a non-production or explicitly approved test Matrix bot
  account
- homeserver and rooms are tenant-selected and approved for validation
- target rooms are unencrypted test rooms only
- authorization is scoped to a test tenant and selected test users/rooms only
- an operator explicitly approves use for the validation path
- evidence is redacted and retained under the same 90-day default retention rule
- production tenants, production rooms, and normal live connector state are not touched

## Capability Declaration

Matrix must declare:

- Core invariants: all pass before ready.
- Tenant-provided bot account setup: required.
- Kura-hosted homeserver provisioning: unsupported for phase 52.
- Kura Matrix account provisioning: unsupported for phase 52.
- Direct messages: supported only for explicitly allowed senders or direct routes.
- Room messages: supported only for selected unencrypted rooms with bot mention or
  configured command.
- Unencrypted text: supported.
- Encrypted rooms and undecryptable events: unsupported.
- E2EE key/session management: unsupported.
- Final-only foreground replies: required minimum for accepted messages.
- Connector-backed background delivery: supported only when destination eligibility is
  explicit and delivery truth remains separate.
- WhatsApp, bridge automation, broad media, voice, calls, reactions, memory-based
  context, thinking visibility, and incremental visible updates: unsupported for phase
  52.

## API, Schema, Event, And Documentation Impact

Any exposed public shape must update schemas and fixtures with the implementation:

- `GET /v1/config` or connector list projections include Matrix connector health,
  capability, homeserver/bot binding, selected-route, and diagnostic summaries
  additively when exposed.
- `GET /v1/connectors/{connectorId}/matrix-setup` returns a
  `MatrixHostedSetupResource` with setup terminal state, homeserver/bot binding summary,
  route policy summaries, diagnostic linkage, retention expiry, and redaction status for
  authorized operators.
- Matrix setup persistence stores terminal state, homeserver/bot binding, route policy,
  diagnostic linkage, and redacted evidence by tenant and connector.
- Matrix inbound event persistence stores canonical homeserver/room-or-direct/event
  dedupe identity and retained sync/transaction identity as redacted evidence.
- Connector capability profile and conformance results include Matrix provider surfaces.
- Connector diagnostic state maps Matrix failures to shared reason codes.
- Delivery target resources represent Matrix eligibility only after setup and
  destination policy validate.
- `connector.matrix_setup_validated`, `connector.diagnostic_state_changed`,
  `connector.route_outcome_recorded`, `connector.inbound_duplicate_detected`,
  `connector.foreground_reply_failed`, and connector-backed delivery events expose only
  redacted evidence if implemented as provider-specific events.
- Foreground reply events carry separate `assistantExecutionOutcome`,
  `matrixReplyOutcome`, and `replyContext` fields when public reply outcome detail is
  exposed.
- `docs/channels/matrix-channel-loop.md`,
  `docs/channels/channel-connector-conformance.md`, and related operator docs describe
  setup, routing, unsupported surfaces, diagnostics, live smoke, and rollback behavior.

Recommended Phase 52 surface names:

- API: `GET /v1/connectors/{connectorId}/matrix-setup`,
  `GET /v1/connectors/{connectorId}/matrix-smoke`,
  `GET /v1/live-validations/matrix-smoke`, and
  `GET /v1/live-validations/matrix-conformance`.
- Config: `connectors.matrix` in `config.json`, `KURA_CONNECTORS_MATRIX_*`
  environment overrides, and `/v1/config.connectors.matrix`.
- Schemas: `matrix-hosted-setup-resource`, `matrix-route-policy-resource`,
  `matrix-smoke-evidence-resource`, and
  `connector-matrix-setup-validated.event`.
- Storage: `matrix_hosted_setups`, `matrix_homeserver_bindings`,
  `matrix_route_policies`, `matrix_smoke_evidence`, and `matrix_event_evidence`.
- SDK: `getMatrixSetup`, `getMatrixSmokeEvidence`,
  `getMatrixLiveValidationSmokeEvidence`, and `getMatrixConformanceEvidence` for
  client-facing setup, smoke, live-validation smoke, and conformance projections.

## Verification Gates

Implementation is incomplete until these cases are covered:

- Matrix setup becomes ready only after bot authorization, homeserver binding, room or
  direct route policy, and conformance validate
- invalid, revoked, incomplete, or missing bot authorization fails without leaking token
  material
- unsupported homeserver behavior is classified separately from network failure
- homeserver unreachable and federation/sync failures map to actionable diagnostics
- ownership mismatch or cross-tenant homeserver/bot/room binding fails closed
- exactly one homeserver/bot binding is active per Matrix connector
- Kura-hosted homeserver provisioning and Matrix account provisioning are rejected
- no selected room or direct allowment returns `action-required`
- cancelled setup preserves audit evidence without deleting unrelated state
- direct unencrypted text from explicitly allowed sender accepted
- direct unencrypted text from unknown or unallowed sender blocked
- selected room with bot mention accepted
- selected room with configured command accepted
- selected room without mention or command ignored
- unselected room blocked
- wrong homeserver or wrong bot account blocked
- encrypted room and undecryptable event unsupported
- media, file, voice, call, reaction-only, bridge metadata, thinking, and incremental
  update inputs unsupported
- duplicate inbound suppression by homeserver/room-or-direct/event ID after sync replay,
  transaction retry, reconnect, restart, or delayed event delivery
- retained sync or transaction identity as redacted delivery evidence
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
