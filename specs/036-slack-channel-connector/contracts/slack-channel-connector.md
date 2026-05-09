# Contract: Slack Channel Connector

This contract is the planning handoff for Roadmap 51. It specializes the shared phase 48
channel connector conformance contract for the new Slack connector.

## Hosted Readiness Gates

| Gate | Required Result | Failure State | Verification |
|------|-----------------|---------------|--------------|
| Hosted Slack app installation/OAuth | OAuth installation flow completes without exposing tokens, grants, authorization payloads, or raw provider payloads | `action-required` or `degraded` with `auth_missing` diagnostic | Fake OAuth setup tests; redaction tests |
| Unsupported setup modes | Submitted raw bot tokens, signing secrets, and local-only credentials are rejected | `unsupported` setup outcome or `action-required` diagnostic | Unsupported setup tests |
| Workspace binding | Exactly one Slack workspace is bound to one connector, and a tenant may own multiple Slack connectors | `degraded` or `action-required` with repair evidence | Workspace cardinality tests; tenant isolation tests |
| Required scopes and installation | Required scopes and active installation validate before readiness | `action-required` or `degraded` with `permission_missing` diagnostic | Scope and installation tests |
| Workspace approval | Workspace approval requirements map to actionable diagnostics | `action-required` with tenant-admin remediation | Approval diagnostics tests |
| Route policy | At least one selected channel or explicit DM user/user-group allowment exists before ingress can create runs | `action-required` until configured | Route policy validation tests |
| Direct-message sender authorization | Direct messages create runs only for explicitly allowed Slack users or user-group members | `blocked` route outcome | DM allowment tests |
| Channel mention gate | Channel messages create runs only when the channel is selected and the message includes an agent mention or supported invocation signal | `ignored` or `blocked` route outcome | Channel gate tests |
| Required channel thread reply | Accepted channel mentions reply in a thread rooted at the triggering channel message | `reply_failed` or explicit unsupported/failure evidence if unavailable | Thread reply tests |
| Durable inbound identity | Dedupe key is tenant, connector, workspace, conversation, and message ID; event ID is retained as evidence | `duplicate` route outcome on replay | Dedupe/reconnect tests |
| Conformance core invariants | Every phase 48 core invariant passes | Not ready | Slack conformance regression |
| Optional surfaces | Every optional Slack surface is supported, limited, or unsupported explicitly | Not ready if silent | Capability profile tests |
| Live hosted smoke | Passed with safe workspace authorization, or structured skip exists | Release risk remains visible | Live smoke or skip tests |

## Setup State Contract

| Input Condition | Terminal State | Ready? | Required Evidence |
|-----------------|----------------|--------|-------------------|
| Hosted OAuth installation valid, workspace binding valid, required scopes present, and valid route policy exists | `ready` | Yes, if conformance gates also pass | Validation timestamp, workspace binding summary, selected route summary |
| OAuth grant missing, callback incomplete, or installation revoked | `action-required` | No | Missing grant or revoked installation remediation evidence |
| Required Slack scope missing | `action-required` or `degraded` | No | Missing scope diagnostic and remediation owner |
| Workspace approval required | `action-required` | No | Tenant-admin remediation evidence |
| Workspace mismatch or ambiguous tenant ownership | `action-required` | No | Workspace mismatch diagnostic |
| Valid OAuth installation but no selected channel or explicit DM allowment | `action-required` | No | Missing route policy remediation evidence |
| Selected channel inaccessible, stale, archived, or missing app membership | `degraded` or `action-required` | No | Per-channel validation result |
| Slack provider unavailable or network validation fails | `unavailable` | No | Provider/network diagnostic and retry guidance |
| User cancels setup | `cancelled` | No | Redacted audit evidence, no unrelated state deleted |
| Submitted raw bot token, signing secret, or local-only credential setup | `action-required` or unsupported setup outcome | No | Unsupported setup remediation evidence |

Terminal states must reuse the hosted setup wizard vocabulary: `ready`, `degraded`,
`unavailable`, `cancelled`, and `action-required`.

## Workspace And Route Policy Contract

Slack route policy must produce redacted tenant-scoped results for:

- exactly one workspace binding per Slack connector
- multiple Slack connectors per tenant
- selected Slack channels
- explicitly allowed Slack users for DMs
- explicitly allowed Slack user groups for DMs
- selected-channel mention gate status
- required channel thread reply mode
- blocked or missing route policy
- stale, inaccessible, archived, or invalid provider scopes
- delivery-target eligibility

Route evidence must include enough safe metadata for repair without exposing OAuth
tokens, installation grants, authorization headers, raw provider payloads, inaccessible
message content, or cross-tenant state.

## Routing Contract

| Scenario | Required Outcome |
|----------|------------------|
| Direct text from explicitly allowed Slack user | Accepted and eligible for one agent run |
| Direct text from member of explicitly allowed Slack user group | Accepted and eligible for one agent run |
| Direct text from unknown or unallowed sender | Blocked with no assistant reply |
| Channel not selected | Blocked with no assistant reply |
| Selected channel without agent mention or supported invocation signal | Ignored with no public reply |
| Selected channel with agent mention | Accepted; mention artifact normalized before assistant handling |
| Wrong workspace for connector | Blocked with no assistant reply |
| Disabled connector | Blocked or ignored with no assistant reply |
| File, voice clip, huddle, canvas, workflow button, interactive block, rich media, or unsupported surface | Unsupported with no agent run |
| Duplicate Slack workspace/conversation/message identity after retry/reconnect/restart | Duplicate with at most one assistant reply |
| Missing tenant/workspace/conversation/message identity | Failed closed unless an explicit equivalent identity rule is documented and tested |

## Durable Identity Contract

Slack inbound dedupe identity is:

- `tenantId`
- `connectorId`
- `workspaceId`
- `conversationId`
- `messageId`

Slack event identity must be retained as redacted provider delivery evidence for
diagnostics, replay, delayed event delivery, reconnect, and support inspection. It must
not replace workspace/conversation/message identity as the canonical duplicate
suppression key.

## Reply And Delivery Contract

| Scenario | Required Outcome |
|----------|------------------|
| Accepted direct message completes assistant work | Send at least one final Slack reply in the DM conversation |
| Accepted channel mention completes assistant work | Send at least one final Slack reply in a thread rooted at the triggering channel message |
| Assistant completes but Slack reply fails | Assistant execution and Slack reply outcomes remain separate |
| Required channel thread reply fails | Diagnostic reason recorded; reply failure evidence retained |
| Slack rate limits foreground reply | Diagnostic reason recorded; retry/degradation evidence retained |
| Slack selected as background delivery target | Delivery success, retry, suppression, or failure is recorded separately from foreground replies |
| Background delivery fails after work succeeds | Work result remains separate from delivery failure truth |

Phase 51 reply progression declaration:

- Final-only foreground replies: required.
- Channel mention thread-rooted replies: required.
- Direct-message normal replies: required.
- Thinking visibility: unsupported unless explicitly recut.
- Incremental visible updates: unsupported unless explicitly recut.
- Interactive controls and rich message forms: unsupported unless explicitly recut.
- Connector-backed background delivery: supported only through explicit delivery-target
  eligibility and separate delivery outcome evidence.

## Diagnostic Mapping

| Slack Condition | Shared Reason Code | Lifecycle/Terminal State | Remediation Owner |
|-----------------|--------------------|--------------------------|-------------------|
| Missing OAuth grant, incomplete callback, or revoked installation | `auth_missing` | `action-required` | `product_user` |
| Missing required Slack scope | `permission_missing` | `action-required` or `degraded` | `tenant_admin` |
| Workspace approval required | `permission_missing` | `action-required` | `tenant_admin` |
| Channel inaccessible, archived, or app membership missing | `permission_missing` | `degraded` or `action-required` | `tenant_admin` |
| Unknown sender, unallowed DM sender, unselected channel, wrong workspace | `blocked_route` | `degraded` | `tenant_admin` |
| Selected channel lacks mention or supported invocation signal | `blocked_route` | `degraded` | `none_required` |
| Slack rate limit on event, reply, or delivery handling | `rate_limited` | `degraded` | `provider` |
| Slack provider unavailable | `provider_unavailable` | `unavailable` | `provider` |
| Network unavailable, event-delivery validation failed, reconnect failed, or retry exhausted | `network_failed` | `unavailable` or `degraded` | `operator` |
| Duplicate inbound workspace/conversation/message after replay | `duplicate_inbound` | `degraded` | `none_required` |
| Reply send failed after assistant work | `reply_failed` | `degraded` | `operator` |
| Raw-token setup, marketplace publication, enterprise grid administration, memory-based team context, files, voice clips, huddles, canvases, workflow buttons, interactive blocks, rich media, thinking, or incremental update | `unsupported_capability` | `unsupported_capability` | `none_required` |
| Unclassified Slack connector failure | `unknown_connector_failure` | `degraded` | `operator` |

## Freshness, Retention, And Redaction

- Cached Slack diagnostics may be shown, but diagnostics older than 15 minutes must be
  marked `stale`.
- Slack actions that fail must produce current diagnostic truth before remediation is
  shown.
- Slack setup evidence, route policy evidence, diagnostic evidence, conformance evidence,
  smoke evidence, retained event evidence, and redaction-failure outcomes expire from
  normal inspection after 90 days unless an authorized longer retention policy applies.
- If evidence cannot be confidently redacted, detailed evidence is suppressed and a safe
  generic classification plus redaction-failure marker is recorded for authorized
  operators.

## Safe Live Authorization Rule

Safe live Slack authorization must satisfy all of these conditions before live smoke can
run:

- authorization belongs to a non-production or explicitly approved test Slack workspace
- authorization is scoped to a test tenant and selected test users/channels only
- an operator explicitly approves use for the validation path
- evidence is redacted and retained under the same 90-day default retention rule
- production tenants, production channels, and normal live connector state are not
  touched

## Capability Declaration

Slack must declare:

- Core invariants: all pass before ready.
- Hosted Slack app installation/OAuth setup: required.
- Submitted raw-token setup: unsupported for phase 51.
- Multiple connectors per tenant: supported.
- Workspace binding: exactly one workspace per connector.
- Direct messages: supported only for explicitly allowed Slack users or user groups.
- Channel messages: supported only for selected channels with agent mention or supported
  invocation signal.
- Channel thread replies: required for accepted channel mentions.
- Final-only foreground replies: required minimum for accepted messages.
- Connector-backed background delivery: supported only when destination eligibility is
  explicit and delivery truth remains separate.
- Slack marketplace publication, enterprise grid administration, files, voice clips,
  huddles, canvases, workflow buttons, interactive blocks, rich media, memory-based team
  context, thinking visibility, and incremental visible updates: unsupported for phase 51.

## API, Schema, Event, And Documentation Impact

Any exposed public shape must update schemas and fixtures with the implementation:

- `GET /v1/config` or connector list projections include Slack connector health,
  capability, workspace binding, selected-route, and diagnostic summaries additively
  when exposed.
- `GET /v1/connectors/{connectorId}/slack-setup` returns a
  `SlackHostedSetupResource` with setup terminal state, workspace binding summary, route
  policy summaries, diagnostic linkage, retention expiry, and redaction status for
  authorized operators.
- Slack setup persistence stores terminal state, workspace binding, route policy,
  diagnostic linkage, and redacted evidence by tenant and connector.
- Slack inbound message persistence stores canonical workspace/conversation/message
  dedupe identity and retained event identity as redacted evidence.
- Connector capability profile and conformance results include Slack provider surfaces.
- Connector diagnostic state maps Slack failures to shared reason codes.
- Delivery target resources represent Slack eligibility only after setup and destination
  policy validate.
- `connector.slack_setup_validated`, `connector.diagnostic_state_changed`,
  `connector.route_outcome_recorded`, `connector.inbound_duplicate_detected`,
  `connector.foreground_reply_failed`, and connector-backed delivery events expose only
  redacted evidence if implemented as provider-specific events.
- Foreground reply events carry separate `assistantExecutionOutcome`,
  `slackReplyOutcome`, and `replyContext` fields when public reply outcome detail is
  exposed.
- `docs/channels/slack-channel-loop.md`,
  `docs/channels/channel-connector-conformance.md`, and related operator docs describe
  setup, routing, unsupported surfaces, diagnostics, live smoke, and rollback behavior.

Recommended Phase 51 surface names:

- API: `GET /v1/connectors/{connectorId}/slack-setup`,
  `GET /v1/connectors/{connectorId}/slack-smoke`,
  `GET /v1/live-validations/slack-smoke`, and
  `GET /v1/live-validations/slack-conformance`.
- Config: `connectors.slack` in `config.json`, `DOPE_CONNECTORS_SLACK_*` environment
  overrides, and `/v1/config.connectors.slack`.
- Schemas: `slack-hosted-setup-resource`, `slack-route-policy-resource`,
  `slack-smoke-evidence-resource`, and `connector-slack-setup-validated.event`.
- Storage: `slack_hosted_setups`, `slack_workspace_bindings`,
  `slack_route_policies`, `slack_smoke_evidence`, and `slack_event_evidence`.
- SDK: `getSlackSetup`, `getSlackSmokeEvidence`,
  `getSlackLiveValidationSmokeEvidence`, and `getSlackConformanceEvidence` for
  client-facing setup, smoke, live-validation smoke, and conformance projections.

Implemented Phase 51 surface names use:

- Runtime package: `daemon/internal/connectors/slack`.
- Setup API projection: `slackHostedSetupResource`.
- Route API projection: `slackRoutePolicyResource`.
- Smoke API projection: `slackSmokeEvidenceResource`.
- Live validation smoke route: `/v1/live-validations/slack-smoke`.
- Conformance route: `/v1/live-validations/slack-conformance`.
- Connector diagnostics route: `/v1/connectors/{connectorId}/diagnostics`.
- TUI setup command: `--slack-setup <connectorId>`.

## Verification Gates

Implementation is incomplete until these cases are covered:

- hosted Slack app installation/OAuth setup becomes ready only after OAuth grant,
  workspace binding, required scopes, route policy, and conformance validate
- missing OAuth grant, revoked installation, and incomplete callback fail without leaking
  token or authorization material
- missing Slack scope is classified separately from missing installation
- workspace approval required is classified with tenant-admin remediation
- workspace mismatch or cross-tenant workspace binding fails closed
- exactly one workspace is bound per connector
- one tenant can own multiple Slack connectors for multiple workspaces
- submitted raw bot token, signing secret, and local-only credential setup are rejected
- no selected channel or explicit DM allowment returns `action-required`
- cancelled setup preserves audit evidence without deleting unrelated state
- DM from explicitly allowed user accepted
- DM from explicitly allowed user-group member accepted
- DM from unknown or unallowed sender blocked
- unselected channel blocked
- selected channel without mention ignored
- selected channel with agent mention accepted
- wrong workspace blocked
- unsupported marketplace publication, enterprise grid administration, memory-based team
  context, files, voice clips, huddles, canvases, workflow buttons, interactive blocks,
  rich media, thinking, and incremental updates produce unsupported outcomes
- duplicate inbound suppression by workspace/conversation/message identity after retry,
  delayed event delivery, reconnect, or restart
- Slack event identity is retained as redacted delivery evidence
- direct-message final reply success
- channel mention final reply in a thread rooted at the triggering message
- required channel thread reply failure separated from assistant execution
- connector-backed background delivery success, retry, suppression, and failure separated
  from foreground reply truth
- rate-limit diagnostic evidence
- provider/network/event-delivery diagnostic evidence
- diagnostic stale after 15 minutes and current truth on failed actions
- 90-day retention expiry
- redaction suppression on unreliable evidence
- tenant isolation and permission denial
- live hosted smoke pass or structured skip
- schema/contract fixtures and docs updated with public shape changes
