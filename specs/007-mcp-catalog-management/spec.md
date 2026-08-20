# Feature Specification: MCP Catalog Management

**Feature Branch**: `[008-mcp-catalog-management]`  
**Created**: 2026-04-20  
**Status**: Draft  
**Input**: User description: "docs/specs/007-mcp-catalog-management-and-distribution.md"

## Clarifications

### Session 2026-04-20

- Q: 当 catalog-managed MCP server 仍有活跃 lifecycle 或 tool invocation 时，`uninstall` / `reinstall` / `refresh` 应该怎么处理？ → A: 直接返回 `conflict` / `busy`，要求资源先变为 idle。
- Q: `revalidation` 在 phase 22 中何时执行？ → A: 只在操作员显式触发时执行，不做 daemon startup 或后台自动检查。
- Q: `uninstall` 对 catalog-managed MCP resource 的最终效果是什么？ → A: 从 active MCP registry 中彻底移除资源，只保留 operator-visible audit/history。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Maintain Installed Catalog Entries (Priority: P1)

As an operator, I need to uninstall, reinstall, or explicitly refresh a catalog-installed
MCP server through daemon-owned workflows so I can recover from bad local state without
hand-editing MCP resources.

**Why this priority**: The current MCP catalog closes at first install. Without a managed
maintenance path, operators cannot safely recover from broken or stale catalog-managed
resources.

**Independent Test**: Install a bundled catalog entry, then uninstall or reinstall it
through daemon-owned routes and confirm the resulting MCP resource, provenance, and audit
history remain explicit and correct.

**Acceptance Scenarios**:

1. **Given** a catalog-installed MCP server exists, **When** the operator uninstalls it,
   **Then** the daemon removes the installed resource from the active MCP registry through
   one daemon-owned workflow and records explicit uninstall truth in operator-visible
   history.
2. **Given** a catalog-installed MCP server exists, **When** the operator reinstalls or
   refreshes it, **Then** the daemon recreates or updates the resource using the same
   catalog-managed resource model while preserving fresh provenance.
3. **Given** a catalog-installed MCP server has operator-owned modifications or conflicting
   state, **When** the operator attempts refresh or reinstall, **Then** the daemon fails
   closed with an explicit reason rather than silently overwriting the resource.
4. **Given** a catalog-installed MCP server still has an active lifecycle action or tool
   invocation, **When** the operator attempts uninstall, reinstall, or refresh, **Then**
   the daemon rejects the request as `conflict` or `busy` until the resource returns to
   idle.

---

### User Story 2 - Inspect Source, Version, And Drift (Priority: P2)

As an operator, I need installed MCP resources to expose source, revision, and drift truth
so I can tell whether a running server still matches the catalog definition it came from.

**Why this priority**: Catalog lifecycle is not trustworthy if operators cannot explain
where an installed MCP server came from or whether it has diverged from the current
catalog definition.

**Independent Test**: Install a catalog-managed MCP server, inspect it through daemon
routes, and verify the resource exposes source identity, install method, revision or
version metadata, and explicit drift state without reading raw stored JSON.

**Acceptance Scenarios**:

1. **Given** a catalog-managed MCP server is installed, **When** the operator inspects it,
   **Then** the daemon shows catalog source identity, install method, and current catalog
   revision or version metadata.
2. **Given** an installed MCP resource no longer matches the current catalog definition,
   **When** the operator inspects it, **Then** the daemon surfaces explicit drift or stale
   state rather than implying the resource still matches the catalog.
3. **Given** a catalog-managed MCP server has local operator changes, **When** the
   operator inspects it, **Then** the daemon distinguishes local modification from normal
   catalog freshness state.

---

### User Story 3 - Revalidate Installed Prerequisites (Priority: P3)

As an operator, I need the daemon to re-check prerequisites for installed catalog entries
so lost credentials, binaries, or transport prerequisites surface before the next manual
start attempt fails unexpectedly.

**Why this priority**: Install-time success is not enough. Operators need ongoing truth
about whether catalog-managed MCP resources are still runnable.

**Independent Test**: Install a catalog-managed MCP server, remove or invalidate one of
its prerequisites, then trigger revalidation and confirm the daemon surfaces explicit
prerequisite-loss or blocked state without requiring a start attempt.

**Acceptance Scenarios**:

1. **Given** an installed catalog-managed MCP server loses a required prerequisite, **When**
   the operator triggers revalidation, **Then** the daemon marks the resource with an
   explicit blocked, unavailable, or unsupported reason.
2. **Given** an installed MCP server is drifted but still healthy at runtime, **When** the
   operator revalidates it, **Then** the daemon distinguishes catalog drift from runtime
   health failure.
3. **Given** revalidation is run against a healthy installed resource, **When** all
   prerequisites remain satisfied, **Then** the daemon preserves ready state without
   rewriting operator-owned truth unnecessarily.
4. **Given** the daemon restarts or remains idle without operator input, **When** no
   explicit revalidation action is requested, **Then** the daemon does not run automatic
   prerequisite checks in this phase.

### Edge Cases

- What happens when an operator attempts to uninstall a catalog-managed MCP server that is
  currently healthy or restarting?
- How does the system handle refresh when the catalog definition has changed but the
  installed resource also has local operator modifications?
- What happens when prerequisite revalidation finds multiple failures at once, such as
  missing credentials and binary loss?
- How does the system handle an installed resource whose catalog source metadata is present
  but whose referenced catalog entry no longer exists?
- What happens when uninstall or reinstall is requested while a tool invocation or server
  lifecycle action is still in progress? The daemon must fail closed with `conflict` or
  `busy` until the resource is idle.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST let operators uninstall catalog-installed MCP servers through
  daemon-owned workflows without requiring manual registry edits.
- **FR-001a**: Uninstall MUST remove the target catalog-managed MCP resource from the
  active MCP registry rather than leaving an `uninstalled`, `inactive`, or disabled
  resource model behind.
- **FR-002**: The system MUST let operators reinstall or explicitly refresh
  catalog-installed MCP servers through daemon-owned workflows.
- **FR-003**: Refresh or reinstall MUST fail closed when the target MCP resource has
  operator-owned modifications or conflicting state that would make silent overwrite
  unsafe.
- **FR-003a**: Uninstall, reinstall, and refresh MUST fail closed with an explicit
  `conflict` or `busy` result when the target MCP resource still has an active lifecycle
  action or tool invocation.
- **FR-004**: Installed catalog-managed MCP resources MUST preserve source identity,
  catalog entry identity, install method, and current catalog revision or version
  metadata.
- **FR-005**: The system MUST surface explicit drift state when an installed catalog-managed
  MCP resource no longer matches the current catalog definition.
- **FR-006**: The system MUST distinguish local operator modification from normal catalog
  drift or freshness state.
- **FR-007**: The system MUST support prerequisite revalidation for installed
  catalog-managed MCP resources without requiring a start or tool call attempt.
- **FR-007a**: In this phase, prerequisite revalidation MUST be operator-triggered only
  and MUST NOT run automatically at daemon startup or background checkpoints.
- **FR-008**: Revalidation outcomes MUST distinguish prerequisite loss, drift, and runtime
  health failure rather than collapsing them into one generic failure state.
- **FR-009**: Operator-visible API, event, and history surfaces MUST distinguish install,
  uninstall, reinstall or refresh success, drift detection, prerequisite loss, and runtime
  health failure.
- **FR-010**: Catalog-management behavior MUST remain additive to the current MCP registry
  and runtime surfaces rather than introducing a second install-state model.
- **FR-011**: Uninstall, reinstall, refresh, and revalidation workflows MUST remain
  environment-scoped so test and production MCP resources do not leak maintenance actions
  across environments.
- **FR-012**: Operator-visible source, version, and drift surfaces MUST preserve current
  secret-redaction and audit expectations.

### Key Entities *(include if feature involves data)*

- **Catalog-Managed MCP Resource**: An installed MCP server that retains catalog origin,
  install method, revision or version metadata, and local modification state.
- **Catalog Lifecycle Action**: An operator-triggered uninstall, reinstall, refresh, or
  revalidation workflow performed against a catalog-managed MCP resource.
- **Catalog Provenance Record**: The operator-visible source, install, revision, and drift
  metadata that explains where an installed MCP resource came from and whether it still
  matches the current catalog.
- **Revalidation Result**: The daemon-owned outcome describing whether installed
  prerequisites, drift status, and runtime health remain satisfied for a catalog-managed
  MCP resource.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: MCP catalog resources, events, and operator-visible history
  gain uninstall, refresh, provenance, revision, and drift surfaces. Existing MCP registry
  and runtime routes remain additive rather than replaced, and uninstall does not add a
  second inactive-resource state inside the registry.
- **Migration / Rollback**: Rollback is a single change-set revert of catalog-management
  lifecycle and provenance additions while preserving current MCP install, registry, and
  invocation surfaces. No separate data model may be introduced that would require a
  parallel rollback path.
- **Verification Strategy**: Run targeted daemon tests for uninstall, reinstall, refresh,
  drift classification, and prerequisite revalidation; run contract coverage for new
  catalog-management resources and events; record one manual `KURA_ENV=test`
  install-to-remove or install-to-refresh workflow.
- **Observability Impact**: Operator-visible API resources, events, and history must
  explain catalog source identity, revision or version truth, local modification, drift,
  uninstall and refresh outcomes, and prerequisite revalidation results.
- **Environment & Secrets**: Validation remains in `KURA_ENV=test` by default. Catalog
  lifecycle actions must stay environment-scoped and continue to respect existing secret
  redaction rules on operator-visible surfaces.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can uninstall, reinstall, or explicitly refresh a
  catalog-installed MCP server in under 5 minutes using committed docs and daemon-visible
  routes only.
- **SC-002**: 100% of validated refresh or reinstall attempts against locally modified or
  conflicting catalog-managed MCP resources fail closed with an explicit operator-visible
  reason.
- **SC-003**: 100% of validated installed catalog resources expose source identity, install
  method, and revision or version truth without requiring operators to inspect raw stored
  definitions.
- **SC-004**: 100% of validated prerequisite-loss and drift scenarios surface explicit
  operator-visible classification before a manual start attempt is required.

## Assumptions

- Roadmap 18 and Roadmap 21 MCP registry, lifecycle, install, and invocation behavior are
  already complete and remain the base for this slice.
- Catalog-managed MCP resources continue to live inside the current daemon-owned MCP
  registry rather than a separate package-management subsystem.
- Revision or version metadata may come from bundled catalog content or daemon-owned
  derived metadata as long as operator-visible source truth remains explicit.
- Catalog-management workflows remain operator-triggered in this slice; fully automated
  remote catalog distribution is out of scope.
- Automatic startup or background revalidation is out of scope for this slice.
