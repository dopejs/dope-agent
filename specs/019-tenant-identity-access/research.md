# Research: Tenant Identity And Access Foundation

## Decision: Add A Focused Identity Domain Package

Create `daemon/internal/identity` for tenants, principals, memberships, invitations,
role-derived permissions, tenant resolution, and tenant audit behavior.

**Rationale**: The existing `daemon/internal/auth` package owns bearer pairing and token
secret handling, while `daemon/internal/api` owns HTTP routing. Tenant authorization is a
cross-cutting domain rule that should be testable without route handlers and reusable by
later tenant-scoped data roadmaps.

**Alternatives considered**:

- Put all behavior in `daemon/internal/api`: rejected because handler-local authorization
  logic would be hard to audit and easy for later routes to bypass.
- Put all behavior in `daemon/internal/auth`: rejected because authentication and
  tenant-scoped authorization have different lifecycles and data ownership.
- Delay identity package until tenant data migration: rejected because every later roadmap
  needs a shared tenant context and permission contract first.

## Decision: Extend Existing Auth Tokens Instead Of Adding A Parallel Token System

Keep pairing and bearer-secret validation in `daemon/internal/auth`, but extend token
records with principal ownership, expiry, revocation, rotation lineage, and durable tenant
grant references.

**Rationale**: Existing local clients already depend on pairing-produced bearer tokens and
`Authorization: Bearer` semantics. Extending those records is additive and preserves local
compatibility while making token lifecycle and tenant grants enforceable.

**Alternatives considered**:

- Create a new hosted-only token type: rejected because it would split auth semantics and
  complicate local-to-hosted migration.
- Replace pairing with organization login: rejected because remote SSO and enterprise
  login are out of scope for this foundation.
- Store raw token material for rotation: rejected because existing trust docs say raw
  token material is returned only at completion time and persisted state stores hashes.

## Decision: Bootstrap A Default Personal Tenant For Existing Local State

On startup, installations without tenant records receive or resolve one default personal
tenant and one active principal. Existing local tokens created before tenant grants exist
receive only that default personal tenant grant.

**Rationale**: This preserves single-user local workflows without manual organization
setup and avoids accidental authority widening when hosted organization tenants are later
introduced.

**Alternatives considered**:

- Force all existing tokens to be reissued: rejected because it breaks local-first
  installations without a security benefit proportional to the disruption.
- Grant existing tokens every tenant owned by the principal: rejected because it widens
  authority beyond the pre-tenant local boundary.
- Defer bootstrap to the first request: rejected because protected route behavior would
  depend on hidden per-request mutation and complicate audit failure handling.

## Decision: Resolve Tenant Context In Protected Route Middleware

After bearer authentication and token persistence, protected API routes resolve a tenant
context from the principal default tenant or `X-Dope-Tenant-ID`. The request proceeds only
when both principal access and token grants allow the resolved tenant.

**Rationale**: A single middleware boundary gives later routes one invariant: accepted
protected requests have both `principalId` and `tenantId`, while denied requests fail
before tenant-owned resources are accessed.

**Alternatives considered**:

- Resolve tenant in each domain handler: rejected because it invites inconsistent
  enforcement.
- Resolve tenant only in future tenant-scoped data routes: rejected because roadmap 34's
  definition of done requires all inbound protected requests to have tenant context or a
  stable authorization error.
- Trust only the token default tenant and ignore the header: rejected because API clients
  must be able to select an allowed tenant explicitly.

## Decision: Use Role-Derived Permissions With No Per-Member Overrides

Permissions derive from tenant role and lifecycle state only. Owners receive all listed
permissions; admins receive tenant, secrets, integrations, connectors, MCP, evaluation,
and billing-view permissions; operators receive run execution, approval resolution, and
live-validation execution permissions; viewers receive read-only inspection access.

**Rationale**: The clarified role baseline is testable, auditable, and sufficient for the
foundation. Per-member overrides can be added later if a hosted administration roadmap
proves the need.

**Alternatives considered**:

- Owner and admin equivalent: rejected because it widens admin authority beyond least
  privilege.
- Owner-only administration: rejected because it prevents delegated organization
  administration called for by the spec.
- Custom per-member grants: rejected because it increases data model and audit complexity
  before the hosted product needs it.

## Decision: Fail Closed When Required Audit Writes Fail

Security-relevant tenant switching, membership changes, and token lifecycle changes must
not complete when their required audit record cannot be written.

**Rationale**: These operations define the security boundary. Allowing them without an
audit record would create invisible authority changes and make production debugging
unreliable.

**Alternatives considered**:

- Complete the action and mark audit degraded: rejected because state would change without
  durable evidence.
- Allow read-only requests without audit and block mutations: partially useful, but the
  spec requires tenant switching and denied access to be auditable; protected tenant
  resolution must stay honest about audit coverage.

## Decision: Use Next-Check Enforcement For Revocations And Grant Changes

Membership, principal, token revocation, token expiry, and tenant-grant changes apply to
every authorization check after the change is durably recorded. Already-authorized
in-flight work is not cancelled by this phase.

**Rationale**: This gives a strict, testable authorization invariant without requiring the
foundation roadmap to coordinate cancellation across every domain executor before those
domains have tenant ownership.

**Alternatives considered**:

- Cancel all in-flight work immediately: rejected because it belongs with domain-specific
  tenant-scoped execution roadmaps.
- Allow asynchronous propagation windows: rejected because it weakens tests and makes
  revocation timing ambiguous.

## Decision: Additive SQLite Migration With Explicit Bootstrap

Add identity tables and token lifecycle/grant structures using the repository's forward
SQLite migration path, then bootstrap personal tenants and token grants before serving
protected routes.

**Rationale**: Existing persisted state must upgrade without destructive reset, and
rollback already relies on restoring a pre-upgrade SQLite file. Keeping migration
idempotent and bootstrap explicit matches `docs/runtime/migration-versioning.md`.

**Alternatives considered**:

- Store identity state in JSON blobs on existing auth rows: rejected because memberships,
  grants, and audits need queryable lifecycle and restart tests.
- Use per-tenant databases now: rejected by scope and by the hosted roadmap split.
- Defer persistence until hosted deployment: rejected because restart-safe grants and
  memberships are required by this roadmap.

## Decision: Contract Surfaces Are API And Event First, Not UI First

Expose tenant, membership, invitation, principal, token lifecycle, and audit behavior
through schema-backed API routes and persisted event/audit contracts. Do not build a
tenant switcher UI in this phase.

**Rationale**: Roadmap 34 is the foundation. Later roadmaps cover tenant-aware operator
shell and SDK ergonomics. The foundation must still provide enough API contract for tests,
operators, and future clients to inspect tenant authority.

**Alternatives considered**:

- Build the web tenant switcher now: rejected because roadmap 21 is explicitly dedicated
  to tenant-aware operator shell and SDK work.
- Keep all inspection internal until the shell roadmap: rejected because membership and
  token state need stable contract tests and operator-visible auditability now.
