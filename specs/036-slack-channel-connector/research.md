# Phase 0 Research: Slack Channel Connector

## Decision: Add Slack as a provider-specific connector that consumes shared channel contracts

**Rationale**: Roadmap 51 is a provider-specific channel slice after the shared phase 48
connector contract, Discord hardening, and Telegram connector work. Reusing connector
supervision, hosted setup, IM loop routing/dedupe, diagnostics, delivery, persistence,
API/schema/event, and contract-test boundaries keeps the change minimal while proving a
work-channel connector against the same invariants.

**Alternatives considered**:

- Build a separate Slack execution service: rejected because it duplicates connector
  runtime, routing, diagnostics, delivery, and rollback boundaries.
- Generalize all future work-channel providers first: rejected because the shared
  conformance contract already provides the cross-channel abstraction needed for this
  phase.

## Decision: Use hosted Slack app installation/OAuth setup only

**Rationale**: Slack workspace identity, installation status, scopes, approval state, and
OAuth grants must be proven during hosted setup. Using the hosted OAuth setup path keeps
tenant authorization, repair, diagnostics, redaction, and retry behavior aligned with
phase 46 and avoids making submitted raw bot tokens or signing secrets part of the
hosted product surface.

**Alternatives considered**:

- Submitted bot token and signing secret setup: rejected because it weakens hosted
  workspace identity proof and increases raw-secret handling.
- Support both OAuth and submitted-secret setup in phase 51: rejected because it doubles
  setup states and repair paths before the hosted path is proven.
- Operator-managed local Slack credentials only: rejected because the roadmap requires a
  hosted-ready connector.

## Decision: Bind exactly one Slack workspace per connector and allow multiple connectors per tenant

**Rationale**: One workspace per connector keeps readiness, route policy, diagnostics,
dedupe, delivery eligibility, smoke evidence, and rollback boundaries unambiguous. A
tenant can still connect multiple workspaces by creating multiple Slack connectors, which
avoids a later model break for organizations with more than one workspace.

**Alternatives considered**:

- One Slack workspace per tenant: rejected because it blocks common multi-workspace
  tenants without improving isolation.
- One connector spanning multiple workspaces: rejected because workspace-specific scopes,
  routes, diagnostics, and rollback would become coupled.
- A shared Slack workspace across tenants: rejected because it violates tenant ownership
  and connector conformance boundaries.

## Decision: Require explicit Slack user or user-group allowment before DM ingress creates runs

**Rationale**: Installing a Slack app in a workspace must not allow every workspace
member to reach a tenant-owned agent. Explicit DM allowment preserves tenant boundaries,
gives operators auditable blocked-route evidence, and aligns with Telegram's explicit
allowment posture while using Slack-specific users and user groups.

**Alternatives considered**:

- Accept any workspace member DM after setup: rejected because workspace membership is
  too broad to serve as agent access consent.
- Allow only the installing user: rejected because legitimate team usage often includes
  multiple explicitly selected users.
- Disable Slack DMs: rejected because DMs are in the upstream phase 51 scope.

## Decision: Require selected channels plus agent mention for channel routing

**Rationale**: Slack channel traffic is high-volume and public to workspace members.
Requiring both selected channel allowment and an agent mention or explicit invocation
signal prevents accidental activation while preserving purposeful work-channel usage.

**Alternatives considered**:

- Selected channel alone accepts every message: rejected because ordinary channel chatter
  could create agent runs and replies.
- Mention-only without selected channel allowment: rejected because it weakens route
  control and makes private or archived channel access harder to reason about.

## Decision: Send channel mention replies in a thread rooted at the triggering message

**Rationale**: Thread-rooted replies reduce channel noise, make response context obvious,
and produce a concrete verification target. Direct messages remain normal foreground
replies in the DM conversation.

**Alternatives considered**:

- Reply directly in the channel: rejected because it increases channel noise and makes
  follow-up context ambiguous.
- Thread only when the incoming message is already threaded: rejected because new channel
  mentions would still produce noisy top-level replies.
- Make thread behavior configurable in phase 51: rejected because it expands the route
  policy before the safest default is proven.

## Decision: Dedupe by Slack workspace, conversation, and message identity and retain event identity as evidence

**Rationale**: Workspace/conversation/message identity represents the user-visible Slack
message that must produce at most one agent run and one foreground reply. Slack event
identity is still useful as redacted provider delivery evidence for retries, delayed
event delivery, and diagnostics, but it should not be the canonical duplicate
suppression key.

**Alternatives considered**:

- Dedupe by Slack event identity only: rejected because transport redelivery mechanics
  can diverge from user-visible message identity.
- Dedupe by user and timestamp window: rejected because it risks suppressing legitimate
  distinct messages or accepting duplicates.
- Drop event identity after dedupe: rejected because support loses useful event-delivery
  evidence.

## Decision: Keep phase 51 final-only foreground reply progression with explicit unsupported rich surfaces

**Rationale**: Final-only replies satisfy the shared connector contract and keep Slack
rate-limit and thread behavior focused for the first slice. Files, voice clips, huddles,
canvases, workflow buttons, interactive blocks, broad rich media, thinking visibility,
and incremental edits require separate callback, storage, update, and redaction
decisions and should be explicit unsupported or limited outcomes unless the roadmap is
recut.

**Alternatives considered**:

- Add incremental Slack message edits in phase 51: rejected because thread correctness,
  rate-limit behavior, and hosted setup are the roadmap-critical paths.
- Support basic file metadata only: rejected because users may infer attachment handling
  exists while actual download/redaction behavior remains incomplete.

## Decision: Reuse shared connector diagnostic freshness, retention, and redaction rules

**Rationale**: Slack should not create provider-specific policy for stale diagnostics or
evidence retention. Cached connector diagnostics become stale after 15 minutes, failed
actions must produce current diagnostic truth before remediation is shown, and
diagnostic/conformance/smoke/redaction-failure evidence expires from normal inspection
after 90 days by default.

**Alternatives considered**:

- Define Slack-specific freshness and retention rules: rejected because it would diverge
  from phase 48 and make support behavior inconsistent across channels.
- Display raw Slack provider errors for support: rejected because raw payloads can
  contain token, workspace, channel, user, message, or tenant data and are not a stable
  public contract.

## Decision: Live Slack smoke is optional but must produce explicit evidence

**Rationale**: Automated acceptance must not require real Slack workspace authorization.
When safe authorization exists, a live hosted/test smoke can prove setup, routing, reply,
delivery, and diagnostic behavior. When it does not, the result must be a structured
skip with owner, reason, date, remaining risk, and redaction status so release review
does not mistake missing live validation for success.

**Alternatives considered**:

- Require live Slack smoke for all acceptance: rejected because it would force real
  external authorization into normal local verification.
- Omit live smoke entirely: rejected because public hosted readiness benefits from
  explicit live validation or a reviewable skip.
