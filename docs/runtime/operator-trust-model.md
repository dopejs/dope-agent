# Operator Trust Model

## Goal

Define the local-first trust boundary for the Dope daemon.

## Current Trust Boundary

- daemon operator APIs under `/v1/*` require bearer authentication when an auth manager is configured
- pairing bootstrap is the only unauthenticated control path
- pairing produces a durable local access token
- the token is persisted and restored across daemon restart
- protected requests resolve a principal and tenant context before tenant-owned control
  surfaces run
- `X-Dope-Tenant-ID` can select a non-default tenant only when both principal membership
  and token tenant grant allow it
- high-risk capability execution is gated by approval state

## Pairing Flow

1. operator client calls `POST /v1/auth/pairings/start`
2. daemon returns a pending pairing plus a short pairing code
3. operator client calls `POST /v1/auth/pairings/{pairingId}/complete` with the code
4. daemon returns an access token
5. operator client uses `Authorization: Bearer <token>` on protected routes

## Protected Surface

When auth is enabled, protected routes include:

- config inspection
- event streaming
- runs and steps
- sessions
- policy approvals
- llm dispatch
- connectors
- capabilities
- `GET /v1/auth/me`
- tenant, principal, membership, invitation, tenant-audit, and auth-token lifecycle APIs

Health and pairing bootstrap stay outside the protected surface.

## Tenant Identity Boundary

Roadmap 34 adds a local-first tenant identity layer without migrating every domain record
to tenant ownership yet:

- existing local operators bootstrap into one default personal tenant
- organization tenants have owner/admin/operator/viewer memberships
- sensitive tenant administration uses `tenant.manage`
- token tenant grants limit which tenants a bearer token may resolve
- disabled principals, removed memberships, revoked tokens, expired tokens, and rotated
  tokens fail before tenant-scoped handlers run
- tenant selection denials use a stable response and do not reveal whether an inaccessible
  tenant exists
- membership changes, invitation decisions, permission denials, token lifecycle changes,
  and tenant grant changes write durable tenant audit events
- security-relevant membership and token grant changes fail closed when required audit
  persistence fails

## Approval Boundary

The first real guarded action is executable-skill and high-risk local-tool execution:

- executable skills default to approval mode `ask` unless the manifest declares `allow` or `deny`
- executable skills can now explicitly require the stronger `docker` backend by selecting
  `docker_default`
- capability kinds `exec`, `shell`, and `browser` require approval
- if no approval is supplied, daemon creates a pending approval and denies execution
- approved approval IDs allow execution
- rejected approval IDs block execution
- the approval gate records declaration-backed consumer provenance and redacted
  secret-resolution metadata for executable skills and the current high-risk tool-call path
- launched in-scope work now crosses the sandbox execution boundary instead of an unmanaged
  subprocess path
- if a request explicitly requires `docker` and the host cannot satisfy prerequisites or
  declared access guarantees, the canonical result is `unsupported`
- operator inspection surfaces now expose backend capability, host readiness, and mismatch
  truth before execution
- daemon restart recovers interrupted in-flight executable-skill and high-risk local-tool
  executions as `cancelled`
- this remains a policy and provenance boundary for the existing tool-call surface, not a
  blanket migration of every local capability

This keeps approvals attached to a real side-effecting action instead of existing only as standalone resources.

For workflow orchestration:

- workflow planning routes are protected operator APIs like the rest of the `/v1/runs/*`
  surface
- planning only previews `approvalModeExpected`; it does not grant a workflow-level
  approval bypass
- each executing workflow step still creates normal runtime steps and tool calls, so
  approval, redaction, provenance, and sandbox policy stay attached to the concrete
  action
- daemon restart interrupts unfinished workflows and leaves them visible as
  `interrupted`; phase 24 does not silently auto-resume pending side effects

For browser-first computer use:

- only the browser driver is in scope for phase 26; generalized desktop automation,
  additional tabs, and new windows are rejected explicitly
- low-risk read-only actions can execute immediately, but high-risk actions such as input,
  click, select, download, or trusted-scope exit require action-scoped approval
- the operator inspects concrete page and target context before approving; approval does
  not grant a broad session-level bypass
- target mismatch is terminal for that action and records evidence-backed failure truth
  instead of auto-retrying or selecting a nearby target
- successful and failed actions both remain inspectable through linked session, action,
  tool-call, artifact, and event surfaces
- daemon restart marks in-flight computer-use sessions and actions `interrupted` rather
  than silently resuming browser side effects

For MCP:

- MCP server register, start, stop, restart, and cancel stay daemon-managed and do not require routine approval
- MCP tool exposure can be marked approval-required per tool and runtime surface
- MCP tool invocation now runs through the existing runtime tool-call plane rather than a
  parallel MCP-only invoke API
- bundled MCP catalog install can happen through daemon API or the repo helper script, but
  both paths converge on the same daemon-managed MCP server resource and audit history
- `streamable-http` transport health and failure state stay explicit beside existing stdio
  subprocess lifecycle semantics
- `websocket` transport capability, auth readiness, reconnect state, and terminal recovery
  truth stay explicit beside existing stdio and `streamable-http` semantics
- operator-visible MCP resources still project declaration-backed provenance and redacted secret-scope summaries
- operator-visible MCP tool-call output, history, and event payloads remain redacted when
  installed or invoked MCP servers touch secret-backed configuration

For personal integrations:

- daemon-owned `/v1/integrations` resources are protected operator APIs like other `/v1/*`
  control-plane routes when auth is enabled
- readiness is explicit operator truth with one canonical default per
  `domainKind/accountKey/environmentScope`, but multiple records may still exist for
  intentional multi-backend cases
- `degraded` integrations remain inspectable and can still surface linkage, while
  `unavailable` blocks integration-backed execution explicitly
- mutation probes on integrations require normal approval resources instead of a special
  integration-only bypass
- approvals, runtime tool calls, and workflow steps expose redacted `integrationBindings`
  so operators can inspect which integration identity and readiness truth was used at
  invocation time
- integration readiness does not redefine delivery or notification outcomes; it only
  governs integration-backed execution readiness

For the calendar domain:

- `/v1/calendar/accounts`, `/v1/calendar/events`, `/v1/calendar/availability/queries`,
  and `/v1/calendar/operations` are protected operator APIs like the rest of `/v1/*`
- calendar account projection, calendar execution truth, and delivery outcome truth stay
  separate so operators can distinguish readiness failure, mutation failure, and delivery
  failure without inference
- background workflow and schedule surfaces project additive
  `calendarOperationSummaries`; they do not replace the authoritative calendar operation
  record
- delivery outcomes project additive `calendarOperationIds` and summaries so a delivered
  or failed notification never becomes the only evidence for what happened in calendar

For the mail domain:

- `/v1/mail/accounts`, `/v1/mail/threads`, `/v1/mail/messages`, `/v1/mail/drafts`, and
  `/v1/mail/operations` are protected operator APIs like the rest of `/v1/*`
- mail account projection, mail execution truth, and delivery outcome truth stay
  separate so operators can distinguish readiness failure, blocked send, send failure,
  and delivery failure without inference
- new outbound mail that is not reply or forward requires explicit recipients from the
  current request; the daemon does not infer those recipients from mailbox history
- background final send requires explicit `allowSendSideEffects`; absent that flag, the
  daemon records blocked mail truth rather than silently sending
- workflow, schedule, and delivery resources project additive `mailOperationSummaries`
  and `mailOperationIds`; they do not replace the authoritative mail operation record

For the delivery plane:

- delivery targets, preferences, outcomes, attempts, and summary windows are protected
  operator APIs like other `/v1/*` control-plane routes when auth is enabled
- foreground connector replies remain channel mechanics; they do not become the only
  source of truth for background delivery
- delivery failure, suppression, and retry state remain explicit even when source
  execution succeeded
- connector-backed delivery attempts may link to `connector_messages`, but transport
  evidence remains subordinate to the daemon-owned delivery ledger

For the reminders domain:

- `/v1/reminders`, `/v1/reminders/occurrences`, and reminder lifecycle command routes are
  protected operator APIs like the rest of `/v1/*`
- reminder lifecycle truth, workflow execution truth, and delivery truth stay separate so
  operators can distinguish acknowledgement, downstream execution, and notification state
  without inference
- reminder-triggered workflow launch auto-acknowledges only after launch succeeds; launch
  failure is recorded explicitly and leaves the occurrence `due` or later `overdue`
- recurring rollover preserves one active unresolved occurrence at a time by moving the
  prior unresolved occurrence to `missed` while keeping acknowledged history intact
- follow-up reminders project additive source references and stale-source state; they do
  not replace authoritative calendar, mail, run, or workflow records

## Security Assumptions

- this remains a local-first daemon trust model; hosted tenant data isolation is covered
  by later roadmaps
- bearer tokens are sufficient for this phase because the daemon is expected to run on a
  user-controlled host
- token material is only returned at completion time; persisted state stores token hashes, not raw tokens
- pairing codes are one-time bootstrap secrets with expiration

## Out Of Scope

- remote SSO
- per-domain tenant data migration
- device inventory and remote revocation UI
