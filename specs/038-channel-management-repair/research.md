# Research: Channel Management And Repair UX

## Decision: Build A Cross-Connector Management Projection

**Decision**: Add a shared channel management projection over existing connector setup,
diagnostic, route, delivery, conformance, and audit records instead of creating
provider-specific management UIs or a new connector runtime.

**Rationale**: Roadmap 53 is a product/repair surface for the existing connector fleet.
Discord, Telegram, Slack, and Matrix already own provider-specific setup, route,
capability, diagnostic, and delivery behavior. A shared projection keeps list/detail,
enablement, repair, support evidence, permissions, pagination, retention, and redaction
consistent while preserving provider contracts as authoritative.

**Alternatives considered**:
- Provider-specific management screens: rejected because they would duplicate permission,
  audit, retention, and repair behavior and make support evidence inconsistent.
- New connector-management runtime independent of existing connector contracts: rejected
  because it would create a second source of truth for setup, diagnostics, routing, and
  delivery outcomes.

## Decision: API, TypeScript SDK, And Web Are Required Surfaces

**Decision**: Expose Phase 53 through API resources, TypeScript SDK methods/types, and
web product flows. TUI remains out of scope unless a later roadmap recuts scope.

**Rationale**: The upstream verification requires web, SDK, and API tests for list,
detail, disable, and repair flows. API contracts provide daemon truth; the SDK keeps
client integrations typed; the web product surface closes the user-facing repair
workflow.

**Alternatives considered**:
- API/SDK only: rejected because it would not satisfy the roadmap goal that users can
  manage and repair channels without raw config edits or log access.
- Web only: rejected because schema-backed API and SDK behavior are needed for contract
  correctness and automated client tests.
- Include TUI: rejected for Phase 53 because the spec clarified API, TypeScript SDK, and
  web as required and did not include TUI in definition of done.

## Decision: Reuse Existing Tenant Permissions

**Decision**: Use existing tenant permissions: `credentials.inspect` for redacted list,
detail, route, reply, delivery, and support-evidence inspection;
`integrations.diagnostics.read` for diagnostic inspection; `connectors.manage` for
disable, re-enable, route edits, and repair starts; and both `connectors.manage` plus
`secrets.manage` for reconnect or credential rotation.

**Rationale**: The repository already exposes these permissions in SDK and tenant
identity flows. Reusing them avoids a new permission family and keeps secret-bearing
repair stricter than non-secret connector management.

**Alternatives considered**:
- Only `connectors.manage`: rejected because it would overgrant read-only inspection and
  under-specify secret-bearing reconnect or rotation.
- New channel-specific permissions: rejected because this phase can be completed with
  existing tenant permission vocabulary.
- `tenant.manage` for all actions: rejected because it is too broad for day-to-day
  connector repair and support inspection.

## Decision: Serialize Mutations Per Connector And Fail Closed On Audit Write Failure

**Decision**: Disable, re-enable, route edit, repair, reconnect, credential rotation, and
delivery-eligibility changes serialize per connector. Required audit evidence must be
recorded before a mutation commits; if audit recording fails, the mutation fails closed
and leaves connector state unchanged.

**Rationale**: Connector management changes affect inbound work, delivery eligibility,
credential use, and support evidence. Per-connector serialization prevents conflicting
states, and audit fail-closed behavior preserves operator accountability for high-risk
tenant mutations.

**Alternatives considered**:
- Last-writer-wins mutations: rejected because disablement could race with repair or
  delivery changes and produce unsafe re-enable behavior.
- Best-effort audit after mutation: rejected because a failed audit write would leave an
  untraceable tenant state transition.
- Pause all connector processing during any mutation: rejected because it is broader than
  necessary and would increase blast radius.

## Decision: Disablement Takes Precedence Until Validated Re-enable

**Decision**: Once a connector is disabled, new inbound provider events and new
background delivery eligibility remain blocked until a later re-enable succeeds after
current setup, health, diagnostic freshness, and route eligibility checks.

**Rationale**: Disable is the safest operator action during incidents, compromised
credentials, noisy routes, or tenant policy changes. It must not be undone by concurrent
repair, reconnect, route edit, provider retry, sync replay, or delayed delivery.

**Alternatives considered**:
- Allow repair completion to implicitly re-enable: rejected because it can surprise users
  who disabled a connector for containment.
- Keep delivery eligibility active while inbound is disabled: rejected because users
  expect disablement to stop new channel use.

## Decision: Paginated Deterministic Connector Lists

**Decision**: Channel connector lists default to 20 items per page and sort
action-required, unavailable, and degraded connectors first, then disabled connectors,
then ready connectors, with stable display-name and connector-ID ordering within each
group.

**Rationale**: Tenants can have multiple connectors per kind. Pagination is needed for
large tenants, and deterministic ordering prevents missing or duplicated entries when
users or tests traverse pages. Prioritizing attention-needed connectors makes the list
useful for repair workflows.

**Alternatives considered**:
- Return all connectors at once: rejected because it does not scale for large tenants and
  bypasses existing list patterns.
- Most-recently-updated ordering: rejected because it is less stable under health and
  repair churn.
- Client-defined ordering: rejected because it would make SDK/web behavior and contract
  tests inconsistent.

## Decision: Metadata-Only Support Evidence With 90-Day Default Retention

**Decision**: Support evidence bundles are metadata-only. They aggregate setup,
diagnostic, route, enablement, repair, reply, delivery, audit, redaction, and retention
metadata but never display channel message bodies or raw provider payloads. Connector
diagnostic, repair, routing, reply, delivery, and support evidence expires from normal
inspection after 90 days unless an authorized tenant retention policy requires longer
retention.

**Rationale**: Support needs enough evidence to reconstruct incidents without creating a
new message-content exposure path. The 90-day default matches existing connector and
integration diagnostic retention patterns.

**Alternatives considered**:
- Redacted message snippets: rejected because Phase 53 does not need message content for
  incident triage and snippets create privacy and redaction risk.
- Indefinite evidence retention: rejected because it increases privacy exposure and
  diverges from existing retention behavior.
- Current-state-only support view: rejected because incidents require historical route,
  repair, reply, delivery, and audit evidence.

## Decision: Repair Actions Link To Existing Setup Sessions And Diagnostics

**Decision**: Repair actions should project diagnostic next steps into existing setup
sessions, reconnect flows, supported credential rotation, route revalidation, and
diagnostic reruns rather than creating a separate remediation engine.

**Rationale**: Roadmap 46 setup sessions and Roadmap 42 diagnostics already define
terminal states, remediation owners, retry safety, redaction, and tenant-scoped evidence.
Reuse keeps repair semantics consistent across connector kinds and avoids autonomous
provider remediation.

**Alternatives considered**:
- New repair state machine independent of setup sessions: rejected because it duplicates
  setup terminal states and risks diverging from credential-bearing use gates.
- Autonomous remediation: rejected by scope; user or operator action remains required for
  setup, reconnect, and rotation.

## Decision: Additive Contracts And Persistence Only Where Projection Requires It

**Decision**: Keep existing connector runtime APIs, setup sessions, diagnostics, route
decisions, delivery outcomes, and conformance records authoritative. Add new schemas,
events, store accessors, and persistence only for management projection gaps such as
enablement audit, mutation serialization, repair action records, support evidence bundles,
pagination metadata, and retention application.

**Rationale**: Minimal additive changes preserve compatibility and reduce rollback risk.
Existing provider-specific connector records already contain most setup, diagnostic,
route, delivery, and redaction evidence needed for product projections.

**Alternatives considered**:
- Rewrite connector resources around a new management model: rejected because it would
  risk breaking existing connector tests and provider-specific contracts.
- Web-only derived state: rejected because support evidence, SDK behavior, and contract
  tests require daemon-owned state.
