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

The first real guarded action is high-risk tool execution:

- capability kinds `exec`, `shell`, and `browser` require approval
- if no approval is supplied, daemon creates a pending approval and denies execution
- approved approval IDs allow execution
- rejected approval IDs block execution
- the approval gate now records declaration-backed consumer provenance and redacted secret-resolution metadata for the current high-risk tool-call path
- this remains a policy and provenance boundary for the existing tool-call surface, not generic sandbox subprocess routing for all local tools

This keeps approvals attached to a real side-effecting action instead of existing only as standalone resources.

For MCP:

- MCP server register, start, stop, restart, and cancel stay daemon-managed and do not require routine approval
- MCP tool exposure can be marked approval-required per tool and runtime surface
- operator-visible MCP resources still project declaration-backed provenance and redacted secret-scope summaries

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
