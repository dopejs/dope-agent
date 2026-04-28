# Phase 0 Research: Hosted Secrets, Integrations, And Connector Isolation

## R1. Tenant Secret Storage Boundary

**Decision**: Add a shared `daemon/internal/secrets` package that owns tenant secret
metadata, versioned resolution, redaction helpers, and bridge behavior. Persist metadata
and version state in SQLite; store local/test secret values in a daemon-owned value backend
under the active data directory with restrictive file permissions. Keep external secret
manager integration out of scope.

**Rationale**: Existing secret readers are scattered across MCP, executable skills,
sandbox preparation, and provider bridge logic. Centralizing resolution is the smallest
way to prove active-tenant-only behavior and redaction across domains. A local value
backend preserves existing local operator workflows and avoids adding a new external
dependency in this roadmap.

**Alternatives considered**:

- Store all secret values directly in operator-visible domain records: rejected because it
  makes redaction harder and increases blast radius.
- Integrate with an enterprise secret manager now: rejected because the upstream spec
  explicitly excludes it.
- Keep `mcp-secrets.json` / `skill-secrets.json` as the only source of truth: rejected
  because they are environment-scoped but not tenant-scoped.

## R2. R37 Boundary Resource Ownership

**Decision**: Convert the R35 Group B resources (`provider_auth_states`, `mcp_servers`,
`mcp_server_states`, `mcp_tools`, `connectors`) from global runtime truth into tenant-owned
resources in this roadmap. Extend existing tenant-owned boundary rows
(`secret_scope_bindings`, `mcp_tool_exposure_rules`) with credential/admin behavior rather
than changing their ownership classification.

**Rationale**: The upstream Roadmap Split explicitly leaves these resources for R37. Hosted
isolation cannot be proven while provider auth state, connector state, or MCP install state
remain global daemon truth.

**Alternatives considered**:

- Leave MCP/connectors global and add runtime filters only: rejected because inspection and
  lifecycle operations would still leak global state.
- Store tenant ownership only in `document_json`: rejected because query helpers, indexes,
  and misuse tests need explicit ownership.

## R3. Secret Rotation Semantics

**Decision**: Rotation creates a new active secret version. New credential resolutions use
the active version; already-started work continues using the version resolved at work
start.

**Rationale**: This preserves deterministic execution and gives operators a clear audit
trail for which credential version a run, connector invocation, MCP invocation, or sandbox
preparation used.

**Alternatives considered**:

- Replace values in place: rejected because in-flight behavior becomes nondeterministic.
- Stage-and-validate before activation: useful later, but this roadmap can close safely
  with versioned activation and explicit failure/disable states.

## R4. Integration Disconnect Semantics

**Decision**: Disconnect revokes tenant-local provider auth, marks dependent connector and
MCP uses disabled, and preserves redacted configuration for reconnect.

**Rationale**: Preserving redacted configuration improves operator recovery while disabling
dependent use prevents orphaned credentials from being invoked.

**Alternatives considered**:

- Delete dependent configuration immediately: rejected because it destroys operator
  context and complicates reconnect.
- Let dependent uses fail only on invocation: rejected because disabled state should be
  visible before a user starts work.

## R5. Audit Granularity

**Decision**: Successful runtime secret use emits one audit-visible record per
credential-bearing run, connector invocation, MCP invocation, or sandbox preparation. Admin
changes and denied cross-tenant attempts emit their own audit records.

**Rationale**: This gives operators durable evidence without flooding audit storage with
repeated internal secret-resolution events during a single work item.

**Alternatives considered**:

- Emit on every individual resolution: rejected due to excessive event volume and noisy
  investigation paths.
- Emit only admin and denial events: rejected because successful credential use must be
  auditable in hosted operation.

## R6. Unsafe Local Credential Bridge

**Decision**: Bridge existing local credential-bearing configuration into the default
personal tenant. If bridge logic finds unsafe or ambiguous state, affected resources start
disabled, preserve redacted metadata, and require operator remediation before use.

**Rationale**: This avoids silent data loss and avoids using credentials whose tenant or
validity cannot be proven. It also lets the daemon start in a contained remediation state.

**Alternatives considered**:

- Refuse daemon startup: rejected because a disabled remediation state is safer and more
  operable for existing local users.
- Bridge everything with warnings: rejected because ambiguous credentials could become
  usable under the wrong tenant.

## R7. Redaction Contract Scope

**Decision**: Redaction is a shared contract for API responses, UI-visible data, events,
logs, replay fixtures, evaluation artifacts, diagnostics, contract fixtures, and test
failure output. Raw secret values, OAuth codes, access tokens, refresh tokens, provider
tokens, and derived credential material are excluded from every listed surface.

**Rationale**: Roadmap 37's definition of done depends as much on artifact hygiene as on
runtime authorization. Shared redaction tests reduce the chance that a new domain leaks a
credential through a secondary path.

**Alternatives considered**:

- Redact only API responses and events: rejected because logs, replay, and evaluation
  fixtures are operator-visible in this codebase.
