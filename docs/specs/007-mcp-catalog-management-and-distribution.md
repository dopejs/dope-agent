# Spec 007: MCP Catalog Management And Distribution

## Status

Planned

## Authority

This document is the authoritative upstream spec for Roadmap 22 before a branch-local
speckit feature spec is created.

Primary source documents:

- `docs/runtime/daemon-roadmaps.md` Roadmap 22
- `docs/harness/harness-architecture.md` follow-on MCP sequencing

## Background

Roadmap 21 closed the first complete MCP product surface for the daemon:

- bundled starter catalog
- daemon API and repo-script install paths
- MCP server inspection, lifecycle, invocation, and audit continuity

What is still missing is catalog lifecycle after first install. Operators can install a
starter entry, but they do not yet have a daemon-owned model for:

- uninstalling an installed entry
- reinstalling a broken or drifted entry
- explicitly refreshing a catalog-managed server definition
- seeing source, version, and drift truth without hand-comparing definitions

This phase turns MCP catalog handling from a one-shot install path into a maintained
operator surface.

## Goal

Make MCP catalog management a first-class daemon-owned product surface with explicit
lifecycle, source provenance, and drift visibility.

## Fixed Decisions

- This phase extends the existing daemon-owned MCP catalog and registry rather than
  creating a second package-management subsystem.
- Source, version, and drift truth must remain operator-visible through daemon surfaces.
- Refresh and reinstall remain fail-closed when operator-owned modifications or conflicting
  state exist.
- Marketplace discovery and signed remote catalog distribution stay out of scope.

## Dependencies

- Roadmap 18: MCP Execution Plane
- Roadmap 21: Complete MCP Runtime And Catalog

## Scope

### In Scope

- uninstall flow for catalog-installed MCP servers
- reinstall and explicit refresh flows
- source provenance on installed resources
- catalog revision or version metadata
- drift detection between installed resource and current catalog definition
- prerequisite revalidation after install
- operator-visible lifecycle truth in API, events, and docs
- verification in `DOPE_ENV=test`

### Out Of Scope

- third-party marketplace discovery
- signed remote catalog distribution
- non-MCP package ecosystems
- adding new MCP transports
- multi-tool orchestration

## Operator Problems To Solve

1. An installed MCP server becomes stale, drifted, or partially broken, and the operator
   needs a safe daemon-owned refresh path.
2. An operator needs to remove a bundled MCP integration without hand-editing registry
   state.
3. An operator needs to understand whether a running MCP server came from a bundled
   catalog definition, which version it came from, and whether local edits have diverged.
4. An installed MCP entry may have passed install-time checks but later lost prerequisites,
   credentials, or binary availability, and the daemon should surface that truth before the
   next manual start attempt.

## User Stories

### Story 1: Remove Or Reinstall A Catalog-Managed MCP Server

As an operator, I want to uninstall or reinstall a catalog-managed MCP server through the
daemon so I can recover from bad local state without hand-editing resources.

Acceptance expectations:

- uninstall removes or deactivates the installed catalog-managed resource through one
  daemon-owned workflow
- reinstall creates the same resource shape as first install, with fresh provenance
- uninstall and reinstall remain fail-closed when a resource has conflicting state or
  operator-owned modifications

### Story 2: Inspect Source, Version, And Drift

As an operator, I want installed MCP resources to preserve catalog source and revision
information so I can tell whether a server still matches the definition it came from.

Acceptance expectations:

- installed resources expose source kind, catalog entry id, install method, and current
  catalog revision or version metadata
- the daemon can mark an installed resource as drifted, stale, or locally modified
- operator inspection does not require reading raw stored JSON to understand provenance

### Story 3: Revalidate Installed Entries

As an operator, I want the daemon to re-check prerequisites for installed catalog entries
so missing credentials, binaries, or transport prerequisites surface explicitly before I
discover them through a failed manual start.

Acceptance expectations:

- revalidation can be triggered on demand or at daemon-defined checkpoints
- results distinguish catalog drift from runtime health failure
- unavailable, blocked, unsupported, and stale states remain explicit

## Functional Requirements

- Installed catalog-managed MCP servers must support explicit uninstall, reinstall, and
  refresh actions through daemon-owned workflows.
- Lifecycle operations must preserve source provenance instead of flattening installed
  entries into indistinguishable manual resources.
- The daemon must fail closed when a refresh or reinstall would overwrite operator-owned or
  conflicting state without explicit permission.
- Installed catalog-managed MCP servers must expose source metadata sufficient to explain
  catalog origin, install method, and current catalog revision or version.
- The daemon must detect and surface catalog drift between the installed resource and the
  current catalog definition.
- The daemon must support prerequisite revalidation for installed catalog-managed MCP
  resources without requiring a start or tool call attempt.
- Operator-visible API, event, and history surfaces must distinguish:
  - install success
  - uninstall success
  - refresh success
  - drift detected
  - prerequisite loss
  - runtime health failure
- All catalog-management behavior must remain additive to the current MCP registry and
  runtime surfaces.

## Compatibility And Operational Notes

- This phase extends MCP catalog resources and events but must not create a second registry
  or install state model.
- Rollback should be a single change-set revert of catalog-lifecycle and provenance
  additions while preserving already-installed MCP runtime surfaces.
- Operator-visible truth remains more important than convenience automation. Refresh and
  reinstall must never silently mutate operator-owned state.

## Verification

- targeted daemon tests for uninstall, reinstall, refresh, drift classification, and
  prerequisite revalidation
- contract coverage for new catalog-management resources and events
- manual `DOPE_ENV=test` workflow covering at least one install-to-remove or
  install-to-refresh cycle

## Definition Of Done

- MCP catalog management is a first-class daemon product surface rather than a one-shot
  install helper
- installed MCP resources preserve enough provenance to explain source, version, and drift
- operators can maintain bundled MCP integrations without hand-editing resource definitions

## Recommended Speckit Input

```text
$speckit-specify 结合 docs/specs/007-mcp-catalog-management-and-distribution.md 完成 phase 22 的工作
```
