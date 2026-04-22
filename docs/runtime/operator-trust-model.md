# Operator Trust Model

## Goal

Define the local-first trust boundary for the Dope daemon.

## Current Trust Boundary

- daemon operator APIs under `/v1/*` require bearer authentication when an auth manager is configured
- pairing bootstrap is the only unauthenticated control path
- pairing produces a durable local access token
- the token is persisted and restored across daemon restart
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

Health and pairing bootstrap stay outside the protected surface.

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

## Security Assumptions

- this is a local-first daemon trust model, not multi-tenant auth
- bearer tokens are sufficient for P0 because the daemon is expected to run on a user-controlled host
- token material is only returned at completion time; persisted state stores token hashes, not raw tokens
- pairing codes are one-time bootstrap secrets with expiration

## Out Of Scope

- remote SSO
- org RBAC
- delegated admin roles
- device inventory and remote revocation UI
