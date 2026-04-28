# Feature Specification: Hosted Secrets, Integrations, And Connector Isolation

**Feature Branch**: `022-hosted-secrets-isolation`  
**Created**: 2026-04-27  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/022-hosted-secrets-integrations-and-connector-isolation.md 完成 phase 37 的工作"

**Upstream authority**: `docs/specs/022-hosted-secrets-integrations-and-connector-isolation.md` is the authoritative design document for this work (Roadmap 37). This specification translates that design into testable scenarios, requirements, and success criteria. Where the upstream document and this spec disagree, the upstream document wins and this spec must be updated.

## Clarifications

### Session 2026-04-27

- Q: Who may inspect credential-bearing connector, MCP, integration, and sandbox state? → A: Tenant admins can manage; tenant-scoped operators with `credentials.inspect` can inspect redacted ownership and status for tenants where they have that permission; viewers cannot inspect or mutate.
- Q: What are the required secret rotation semantics? → A: Rotation creates a new active secret version; new resolutions use it, while already-started work keeps the version it resolved at start.
- Q: What should integration disconnect do to dependent connector and MCP uses? → A: Disconnect revokes provider auth, marks dependent connector and MCP uses disabled, and preserves redacted configuration for reconnect.
- Q: What audit granularity is required for successful secret reference use? → A: Emit one audit event per credential-bearing run, connector invocation, MCP invocation, or sandbox preparation that uses secret references.
- Q: What should happen when local credential bridge finds unsafe or ambiguous credential state? → A: Start with affected credential-bearing resources disabled, preserve redacted metadata, and require operator remediation before use.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Tenant Admin Manages Tenant-Owned Credentials (Priority: P1)

As a tenant administrator, I need to create, rotate, disconnect, and inspect integration account bindings and secret references that belong only to my active tenant so that one organization cannot use or alter another organization's external accounts or credential material.

**Why this priority**: Hosted operation is unsafe if credentials are global or can be reused across tenants. Tenant-local ownership of secrets, integration accounts, and provider authorization state is the core safety boundary for this phase.

**Independent Test**: Create two tenants with same-shaped integration accounts, provider authorization state, and secret references. Sign in as an administrator for tenant A and verify that tenant A can manage only its own bindings and cannot read, rotate, disconnect, or invoke tenant B credentials.

**Acceptance Scenarios**:

1. **Given** tenant A and tenant B each have an integration account for the same external provider, **When** a tenant A administrator views integration accounts, **Then** only tenant A's account binding and non-secret metadata are visible.
2. **Given** tenant A has a stored secret reference, **When** a tenant A administrator rotates the referenced secret, **Then** tenant A uses the rotated credential and tenant B credentials are unchanged.
3. **Given** the same human user belongs to tenant A and tenant B, **When** that user connects an external account in tenant A, **Then** the connection is owned by tenant A only and does not authorize tenant B.
4. **Given** a viewer lacks credential administration permission, **When** the viewer attempts to inspect, create, rotate, or disconnect a tenant secret or integration account, **Then** the action is denied without revealing secret values or provider tokens.

---

### User Story 2 - Runtime Resolves Credentials Only In The Active Tenant (Priority: P1)

As a hosted user or automation owner, I need connector, MCP, provider, and sandbox operations to resolve secret references only through the active tenant so that normal product workflows cannot accidentally or deliberately use another tenant's credential path.

**Why this priority**: Permission-gated administration is insufficient if runtime invocation can bypass tenant scope. Every credential-bearing action must resolve through the same tenant boundary that owns the resource.

**Independent Test**: Create a connector, MCP install, provider auth state, and sandbox policy in tenant A with matching names or identifiers in tenant B. Invoke each from tenant A and from tenant B. Confirm each invocation uses only the active tenant's credentials and cross-tenant attempts fail with stable errors.

**Acceptance Scenarios**:

1. **Given** a connector configuration references a tenant secret, **When** the connector runs under tenant A, **Then** the secret reference resolves only if the secret belongs to tenant A.
2. **Given** an MCP tool exposure rule references credential-bearing state, **When** a tenant B user attempts to invoke it through tenant A's reference, **Then** the invocation fails with a stable cross-tenant credential error.
3. **Given** a sandbox profile references tenant secrets, **When** the sandbox is prepared for a tenant-scoped run, **Then** all referenced secrets resolve through the run's tenant and no global fallback is used.
4. **Given** provider authorization state is expired, revoked, or disconnected for the active tenant, **When** a runtime path tries to use that provider, **Then** the operation fails or requests reconnection for that tenant without using another tenant's provider state.
5. **Given** an integration account is disconnected, **When** dependent connector or MCP uses reference that account, **Then** those uses are disabled until reconnect and their redacted configuration remains inspectable to authorized users.

---

### User Story 3 - Tenant-Scoped Operator Inspects Connector And MCP Ownership Safely (Priority: P2)

As a tenant-scoped operator with `credentials.inspect` for a tenant, I need to inspect which tenant owns connector configuration, MCP installs, MCP server state, and credential-bearing sandbox policy so that production support can diagnose ownership and permission issues without accessing secret values.

**Why this priority**: Hosted operations require debuggability. Operators need ownership, state, and denial evidence, but exposing raw credentials in inspection surfaces would violate the security boundary.

**Independent Test**: Provision connector and MCP resources for two tenants. Sign in as an operator with `credentials.inspect` for tenant A and verify tenant A ownership, status, and redacted references are visible while tenant B resources and raw secret values are never returned in UI, API responses, logs, events, replay fixtures, or evaluation artifacts.

**Acceptance Scenarios**:

1. **Given** an MCP server is installed in tenant A, **When** an operator with tenant A `credentials.inspect` permission inspects MCP installs, **Then** the owning tenant, install state, and non-secret configuration are visible.
2. **Given** a connector configuration contains a secret reference, **When** an operator with tenant `credentials.inspect` inspects the connector, **Then** the reference is shown in redacted form and the raw secret value is not available.
3. **Given** a connector or MCP configuration is changed, **When** the change completes, **Then** an audit-visible record identifies the tenant, actor, resource kind, action, and time without including secret material.
4. **Given** an operator lacks permission for a tenant, **When** the operator attempts to inspect that tenant's connector or MCP state, **Then** the request is denied without falling back to global state or another tenant's state.

---

### User Story 4 - Existing Local Credential Configuration Upgrades Safely (Priority: P2)

As an existing local operator upgrading into hosted tenant support, I need current local secret configuration and external account bindings to bridge into my default personal tenant without printing, logging, or exposing secret values so that the upgrade does not break existing workflows or leak credentials.

**Why this priority**: Existing operators must have a safe continuity path. A hosted credential model that strands or exposes local credential material would be unshippable even if new hosted tenants are isolated.

**Independent Test**: Start with a pre-hosted local configuration containing fake secrets, integration accounts, connector configuration, and MCP install state. Upgrade into tenant-aware hosting and confirm the resources are owned by the default personal tenant, existing workflows continue under that tenant, and no raw secret values appear in outputs or artifacts.

**Acceptance Scenarios**:

1. **Given** local secret configuration exists before hosted credential isolation, **When** the operator upgrades, **Then** the configuration is associated with the default personal tenant without printing raw values.
2. **Given** existing connector and MCP configuration exists before the upgrade, **When** the upgrade completes, **Then** those resources are inspectable as default-personal-tenant resources with redacted credential references.
3. **Given** the operator rotates a bridged secret after upgrade, **When** tenant-scoped workflows run, **Then** the rotated value is used only by the default personal tenant.
4. **Given** upgrade or bridge logic encounters an unsafe credential state, **When** startup or verification runs, **Then** affected credential-bearing resources are disabled, redacted metadata is preserved, operator remediation is required before use, and no raw secret material is emitted.

### Edge Cases

- A secret reference that exists in tenant A and has the same display name or key shape as one in tenant B must resolve only to the active tenant's secret.
- Secret rotation must create a new active version for future resolutions; work that already resolved a prior version at start must keep using that version until the work completes.
- Cross-tenant attempts to read, rotate, disconnect, invoke, or inspect credential-bearing resources must return stable denial errors and must not reveal whether the target tenant's secret value exists.
- Logs, API responses, event payloads, replay fixtures, evaluation artifacts, and test failure output must redact secret values, provider tokens, OAuth codes, refresh tokens, and derived credential material.
- Expired, revoked, or disconnected provider authorization state must not fall back to a previous token, a global token, or another tenant's token.
- Connector and MCP configuration changes must remain permission-gated even if a UI control is hidden or disabled.
- Disconnecting an integration account must revoke tenant-local provider authorization state, disable dependent connector and MCP uses, and preserve redacted configuration for reconnect.
- Deleting a credential-bearing integration account or configuration must not orphan usable provider auth state or unresolved secret references that can still be invoked.
- Tenant ownership must remain clear when the same human user connects the same external account in multiple tenants.
- Sandbox profiles that reference tenant secrets must fail closed when tenant context is missing or mismatched.
- Imported or bridged local secrets must never be echoed in upgrade summaries, diagnostics, rollback instructions, or operator documentation.
- Unsafe or ambiguous local credential state discovered during bridge must leave affected resources disabled with redacted metadata preserved until an operator remediates them.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST treat secrets as tenant-scoped, operator-owned material whose raw values are never exposed through normal product, operator, testing, replay, or evaluation surfaces.
- **FR-002**: Secret references MUST resolve only inside the active tenant and MUST fail with stable cross-tenant credential errors when tenant context is missing, unauthorized, or mismatched.
- **FR-003**: Users with credential administration permission MUST be able to create, update metadata for, rotate, and disable tenant secrets without retrieving raw secret values after storage.
- **FR-003a**: Secret rotation MUST create a new active version for future secret resolutions, and already-started work MUST continue using the secret version it resolved at start until that work completes.
- **FR-004**: Users without credential administration permission MUST be denied secret inspection, mutation, rotation, and disable operations, and denials MUST NOT reveal raw secret values.
- **FR-004a**: Tenant-scoped operators with `credentials.inspect` permission MUST be able to inspect redacted ownership, status, and non-secret metadata for credential-bearing connector, MCP, integration, and sandbox state only within tenants where they have that permission; viewers MUST NOT be able to inspect or mutate those resources.
- **FR-005**: Integration account bindings MUST be tenant-owned, including cases where the same human user belongs to multiple tenants or connects the same external account in more than one tenant.
- **FR-006**: Provider authorization state MUST be tenant-owned and MUST support tenant-local lifecycle outcomes for connect, expiry, refresh, revoke, disconnect, and rotation.
- **FR-006a**: Disconnecting an integration account MUST revoke the active tenant's provider authorization state, mark dependent connector and MCP uses disabled, preserve redacted configuration for reconnect, and prevent those dependent uses from invoking until reconnection succeeds.
- **FR-007**: Runtime paths that use provider authorization state MUST use only the active tenant's state and MUST NOT fall back to global, previous-tenant, or other-tenant credentials.
- **FR-008**: Connector configuration MUST be a tenant resource, and reading, creating, updating, deleting, enabling, disabling, or invoking credential-bearing connector configuration MUST require the appropriate tenant permission.
- **FR-009**: MCP server installs, server state, tool exposure state, and credential-bearing MCP configuration MUST be tenant resources, and MCP administration MUST require the appropriate tenant permission.
- **FR-010**: Sandbox policies and profiles that reference secrets MUST resolve those references through the active tenant and MUST fail closed when the tenant cannot be resolved or authorized.
- **FR-011**: API responses, UI-visible data, events, logs, replay fixtures, evaluation artifacts, diagnostics, and contract fixtures MUST redact secret values, OAuth codes, access tokens, refresh tokens, provider tokens, and derived credential material.
- **FR-012**: The system MUST emit audit-visible records for secret reference use, secret rotation, integration connect and disconnect, provider authorization lifecycle changes, connector configuration changes, MCP install or exposure changes, sandbox policy changes involving secrets, and denied cross-tenant credential attempts.
- **FR-012a**: Successful secret reference use MUST emit one audit-visible record per credential-bearing run, connector invocation, MCP invocation, or sandbox preparation that uses secret references, rather than one record per repeated internal resolution.
- **FR-013**: Audit-visible records for credential-bearing activity MUST include the acting tenant, actor where available, resource kind, action, timestamp, and outcome, and MUST NOT include raw secret material or another tenant's secret details.
- **FR-014**: Cross-tenant attempts to use secret references, integration accounts, provider auth state, connectors, MCP installs, or sandbox policies MUST fail with stable errors suitable for UI, SDK, and operator handling without parsing raw text.
- **FR-015**: Existing local secret configuration, integration bindings, connector configuration, MCP install state, and provider auth state MUST migrate or bridge into the default personal tenant without printing, logging, or otherwise exposing secret values.
- **FR-015a**: When local credential bridge finds unsafe or ambiguous credential state, affected credential-bearing resources MUST start disabled, preserve redacted metadata for operator remediation, and MUST NOT be usable until remediation succeeds.
- **FR-016**: Operators MUST have a documented path for rotating tenant secrets and recovering from expired, revoked, or disconnected provider authorization state.
- **FR-017**: The implementation plan MUST include a handoff table for every shared integration, connector, MCP, provider auth, sandbox policy, and secret-bearing resource touched by Roadmap 35 or Roadmap 37.
- **FR-018**: The handoff table MUST identify each resource or table name, Roadmap 35 tenant-ownership status, Roadmap 37 credential or administration behavior, permissions for read, mutate, connect, disconnect, rotate, or invoke, redaction expectations, and a cross-tenant misuse test case.
- **FR-019**: The handoff table MUST explicitly cover provider auth states, MCP servers, MCP server states, MCP tools, connectors, secret scope bindings, MCP tool exposure rules, integration accounts, tenant secrets, and sandbox policies or profiles that reference secrets.
- **FR-020**: The feature MUST include redaction contract tests, cross-tenant secret and integration isolation tests, connector and MCP isolation tests, permission-denial tests for viewer and operator roles, handoff table verification, and a manual test-environment smoke path using fake tenant-scoped integration configuration.
- **FR-021**: External enterprise secret-manager integrations, cross-tenant shared service accounts, marketplace distribution, billing enforcement, and broad cross-tenant administration are out of scope for this phase.

### Key Entities

- **Tenant Secret**: Credential material owned by one tenant. Its value is usable through controlled resolution paths but is not returned after storage.
- **Secret Reference**: A tenant-scoped handle to a tenant secret. It can appear in configuration and runtime policy but resolves only inside the active tenant.
- **Integration Account Binding**: A tenant-owned relationship between a tenant and an external account or provider connection.
- **Provider Authorization State**: Tenant-owned authorization lifecycle state for an external provider, including connection, expiry, refresh, revocation, disconnect, and rotation outcomes.
- **Secret Version**: A tenant-owned version of secret material. Rotation creates a new active version for future resolutions while preserving deterministic use by already-started work.
- **Disabled Dependent Use**: A connector or MCP use that remains configured in redacted form after integration disconnect but cannot invoke credential-bearing behavior until reconnection succeeds.
- **Disabled Bridged Credential Resource**: A credential-bearing resource discovered during local bridge that is preserved in redacted form but cannot be used until an operator remediates unsafe or ambiguous state.
- **Connector Configuration**: Tenant-owned configuration for an external connector, including redacted secret references and permission-gated administration state.
- **MCP Install And Exposure State**: Tenant-owned MCP server installation, server state, tool state, and exposure rules, including any credential-bearing configuration.
- **Sandbox Policy Or Profile**: Runtime policy that may reference tenant secrets and must resolve those references through the active tenant.
- **Credential Audit Event**: Audit-visible evidence of secret reference use, credential lifecycle changes, connector or MCP administration, sandbox policy changes involving secrets, and denied cross-tenant attempts. Successful runtime secret use is recorded once per credential-bearing run, connector invocation, MCP invocation, or sandbox preparation.
- **Roadmap 37 Handoff Table**: Planning artifact that maps shared resources to their Roadmap 35 ownership status and Roadmap 37 credential, permission, redaction, and misuse-test requirements.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: This phase changes credential, integration, connector, MCP, sandbox policy, event, audit, and operator inspection surfaces so they are tenant-owned and redacted. Raw secret values remain non-returnable. Existing local configuration must continue through default-personal-tenant bridge behavior.
- **Migration / Rollback**: Existing local credential-bearing configuration must migrate or bridge into the default personal tenant without exposing values. Unsafe or ambiguous bridged credential state must start disabled with redacted metadata preserved until operator remediation. Rollback must preserve a documented path to return to the prior local configuration or restore from backup without requiring operators to copy raw secrets from logs, events, or generated artifacts.
- **Verification Strategy**: Requires redaction contract tests, cross-tenant isolation tests for secrets, integrations, provider auth, connectors, MCP installs, and sandbox policies, viewer and operator permission-denial tests, handoff table verification, and a manual test-environment smoke using fake tenant-scoped integration configuration.
- **Observability Impact**: Adds audit-visible records for secret reference use, integration connect and disconnect, provider auth lifecycle changes, connector and MCP configuration changes, sandbox policy changes involving secrets, rotation, and denied cross-tenant credential attempts. All observability output must preserve redaction.
- **Environment & Secrets**: Development and verification must use the test environment and fake credentials by default. Live connectors and production secrets are not required for acceptance and must not be touched unless an operator explicitly chooses a live validation path outside this spec.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of covered secret reference resolution paths resolve only within the active tenant and fail closed when tenant context is missing, unauthorized, or mismatched.
- **SC-002**: 100% of redaction contract cases confirm that API responses, UI-visible data, events, logs, replay fixtures, evaluation artifacts, diagnostics, and contract fixtures contain no raw secret values or provider tokens.
- **SC-003**: Cross-tenant isolation tests for secrets, integration accounts, provider authorization state, connectors, MCP installs, and sandbox policies pass for two same-shaped tenants with zero observed cross-tenant reads, mutations, invocations, or fallback behavior.
- **SC-004**: Viewer and unauthorized operator permission tests deny 100% of covered credential, connector, MCP, integration, and sandbox policy administration attempts without exposing secret material.
- **SC-005**: 100% of covered integration connect, disconnect, expiry, revoke, refresh, and rotation scenarios affect only the active tenant's provider authorization state.
- **SC-005a**: 100% of covered secret rotation tests show that new work uses the newly active version and already-started work continues with the version resolved at start.
- **SC-005b**: 100% of covered integration disconnect tests show dependent connector and MCP uses become disabled, remain redacted and inspectable to authorized users, and cannot invoke until reconnect succeeds.
- **SC-006**: 100% of connector and MCP administration changes in covered tests produce audit-visible records that identify tenant, actor where available, resource kind, action, timestamp, and outcome while preserving redaction.
- **SC-006a**: 100% of covered successful secret-use tests emit exactly one audit-visible record per credential-bearing run, connector invocation, MCP invocation, or sandbox preparation, even when that work performs repeated internal secret resolutions.
- **SC-007**: The Roadmap 37 handoff verification covers every required shared resource and includes ownership status, credential or administration behavior, permissions, redaction expectations, and a cross-tenant misuse test case for each entry.
- **SC-008**: Upgrade or bridge verification for existing local fake credential configuration completes without emitting raw secret values and leaves the resources usable from the default personal tenant.
- **SC-008a**: Unsafe local credential bridge tests show 100% of affected credential-bearing resources start disabled, preserve redacted metadata, and cannot be invoked before operator remediation.
- **SC-009**: The manual test-environment smoke demonstrates tenant-scoped fake integration configuration, redacted inspection, successful active-tenant invocation, and stable denial for cross-tenant misuse in under 15 minutes.

## Assumptions

- Roadmap 37 builds on the tenant identity and access foundation from Roadmap 34 and tenant-scoped ownership work from Roadmap 35.
- Roadmap 35 owns tenant ownership and filtering for persisted rows where already implemented; Roadmap 37 owns credential semantics, redaction, provider auth lifecycle, runtime secret resolution, and administration permissions.
- The default personal tenant is the compatibility target for existing local credential-bearing configuration.
- Secret values are operator-owned material and are not recoverable through normal read APIs after storage.
- Test coverage uses fake providers, fake credentials, and the test environment by default.
- Enterprise secret manager integrations, cross-tenant shared service accounts, marketplace distribution, billing enforcement, and broad cross-tenant administration remain out of scope.
