# Research: MCP Catalog Management

## Decisions

### Decision: Catalog maintenance actions should live on installed MCP server routes, not on catalog entry routes

- Rationale: `uninstall`, `refresh`, `reinstall`, and `revalidate` operate on an existing
  installed resource with lifecycle state, operator modifications, exposure rules, and
  history. Attaching these verbs to `/v1/mcp/servers/{serverId}/...` keeps one installed
  resource model and avoids inventing a second action target keyed only by catalog entry.
- Alternatives considered:
  - Add maintenance routes under `/v1/mcp/catalog/{entryId}/...`.
    - Rejected because it loses the concrete installed resource identity and makes
      conflict detection against live state less direct.
  - Reuse generic `PATCH /v1/mcp/servers/{serverId}` with ad hoc fields such as
    `{"action":"refresh"}`.
    - Rejected because explicit action routes are easier to validate, audit, and bind to
      discrete schemas and events.

### Decision: Persist catalog-management truth inside the existing MCP server document

- Rationale: Roadmap 22 explicitly forbids a second install-state model. The existing
  `mcp_servers` table already stores opaque server documents and is the natural place to
  add install snapshot, catalog revision, drift, and revalidation metadata without a new
  catalog package table.
- Alternatives considered:
  - Add a new `mcp_catalog_installs` table for package lifecycle state.
    - Rejected because it would split the source of truth between server resources and
      package records.
  - Keep all revision and drift data derived in memory only.
    - Rejected because restart-safe operator inspection and audit history require durable
      provenance and revalidation output.

### Decision: Use a canonical catalog revision fingerprint, not a hand-maintained version string

- Rationale: Bundled catalog entries currently do not expose explicit versions. A
  canonical content fingerprint of the normalized catalog entry and effective install spec
  is additive, deterministic, and enough to classify `installed revision` versus `current
  catalog revision` in this slice.
- Alternatives considered:
  - Add a human-authored semantic version field to every bundled catalog entry now.
    - Rejected because it creates new release bookkeeping without improving current
      operator safety.
  - Use only `updatedAt` timestamps from the installed server resource.
    - Rejected because timestamps do not describe whether the current catalog definition
      materially changed.

### Decision: `refresh` and `reinstall` must preserve install-time overrides through a persisted install snapshot

- Rationale: The current installed server document stores the effective MCP server config,
  but phase 22 needs to distinguish install-time overrides from later operator edits.
  Persisting an override-safe install snapshot lets refresh apply the latest catalog
  defaults plus original install overrides, and lets reinstall recreate the same logical
  resource with fresh provenance.
- Alternatives considered:
  - Rebuild refresh or reinstall directly from the current server document.
    - Rejected because that cannot distinguish operator edits after install from original
      install overrides.
  - Force operators to supply a new install body for every reinstall or refresh.
    - Rejected because it weakens daemon-owned maintenance and makes verification harder.

### Decision: `refresh` updates in place, while `reinstall` recreates the same `serverId` from the persisted install snapshot

- Rationale: Operators need both a lightweight definition resync and a “fresh start”
  maintenance path. `refresh` keeps the current resource identity and state lineage while
  reapplying the catalog definition; `reinstall` performs delete-and-recreate semantics
  with the same `serverId`, resetting state and tool discovery while preserving audit
  linkage and catalog provenance.
- Alternatives considered:
  - Treat `refresh` and `reinstall` as identical aliases.
    - Rejected because it collapses two materially different operator intents.
  - Make `reinstall` require a prior standalone uninstall.
    - Rejected because uninstall removes the active resource, which would make reinstall
      impossible without a new tombstone or catalog-only action target.

### Decision: Busy resources fail closed instead of auto-stopping or queueing maintenance

- Rationale: Clarification fixed this for phase 22. Returning explicit `busy` or
  `conflict` preserves truthful operator control and avoids a new pending-action state
  machine that would have to coordinate lifecycle, tool calls, and restore behavior.
- Alternatives considered:
  - Force stop and continue with the action.
    - Rejected because that creates hidden interruption semantics for running MCP work.
  - Queue the action until the server becomes idle.
    - Rejected because it introduces background mutation and a second scheduler-like
      workflow.

### Decision: Revalidation is explicit and operator-triggered only

- Rationale: Clarification fixed this for phase 22. Revalidation should surface truth on
  demand without becoming a startup sweep or background automation loop. This keeps scope
  bounded and avoids surprise state churn in server resources.
- Alternatives considered:
  - Run revalidation automatically on daemon startup.
    - Rejected because it changes startup semantics and could create noisy state flips.
  - Add daemon-defined checkpoints or background periodic revalidation.
    - Rejected because it expands this slice into a scheduler and alerting problem.

### Decision: Revalidation should produce one primary classification plus a list of issues

- Rationale: The spec calls out multiple simultaneous failures, such as lost secrets and
  missing binaries. A primary classification keeps operator summaries stable, while an
  `issues[]` list preserves the full set of detected failures without flattening
  everything into one generic reason.
- Alternatives considered:
  - Return only one reason string.
    - Rejected because it hides additional failures and makes repeated revalidation less
      informative.
  - Return a fully unbounded nested diagnostic tree.
    - Rejected because it is excessive for the current operator surface and contract load.

## Implementation Notes

- Catalog management stayed on the existing MCP server plane. The implementation added
  explicit maintenance routes and additive server-resource projection instead of creating
  a separate package-state API.
- Persistence stayed inside `mcp_servers.document_json`. Install snapshot, installed or
  current revision, drift summary, last action truth, and last revalidation snapshot are
  durable and restart-safe.
- Reinstall uses same-`serverId` delete-and-recreate semantics with a best-effort
  rollback to the previous input if recreation fails after delete.
- Busy detection is enforced against both live MCP lifecycle state and active MCP tool
  calls, so maintenance actions fail closed before mutation.
- Revalidation remains operator-triggered only. The implementation updates availability
  truth from the stored revalidation snapshot without introducing daemon-startup or
  background checks.
