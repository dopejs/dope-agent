# Daemon Task Registry — Remaining Work

> **Supersession note (2026-08-16):** this registry replaces the Go-era task
> registry, which is preserved in this file's git history (revisions before
> 2026-08-16). All tasks for Roadmaps 1-72 are closed; that history is the
> audit record.

## Purpose

Audit registry for the remaining daemon roadmaps defined in
[`daemon-roadmaps.md`](daemon-roadmaps.md). It exists to track task status,
make incomplete work visible, and prevent narrow implementations from being
counted as done.

## Status Rules

- `[x]` means the full task definition of done is satisfied
- `[ ]` means the task is not complete
- use a short suffix like `(partial)`, `(in_progress)`, or `(blocked)` when
  needed
- a task stays `[ ] (partial)` if any required API, persistence, recovery,
  eventing, contract, or testing boundary is still open
- task progress is informational only; a roadmap is still incomplete until
  every in-scope task is fully closed

Verification baseline for all tasks: `cargo test --workspace` and
`make daemon-contract-test` green at the closing commit.

## Roadmap 73: Documentation And Execution-Record Truth Closure

- [x] Replace roadmap and task documents with current-state versions; retire
      the R1-72 documents and superseded roadmap-split docs to git history
- [x] Rewrite `CLAUDE.md` daemon section for the Rust workspace
- [x] Audit and clean stale `TODO: PLACEHOLDER` / `TODO: MISSING` markers in
      `crates/surface/api/src/`
- [x] Sweep `docs/` for live references to the removed Go tree
- [x] Confirm `docs/specs/README.md` and spec-flow docs match the current
      authoring flow

## Roadmap 74: Rust API Surface Parity Closure — complete 2026-08-16

- [x] Six missing Roadmap 65-72 route families ported with tests: triage,
      routines, catalog items, execution profiles + explain, support
      evidence bundles, release launch-gate
- [x] `/v1/config` route family
- [x] `/v1/sessions` route family (list/get/reset/events)
- [x] `/v1/llm/dispatches` + `/v1/llm/dispatches/stream` route family (SSE)
- [x] `/v1/sandboxes/{executions,profiles,explain}` route family
- [x] Parity-gate-surfaced families: `/v1/capabilities`, `/v1/connectors`
      (incl. ingress/messages pipeline), `/v1/policy/approvals` (incl.
      consumer-policy sync + computer-use resume), `/v1/providers` (incl.
      managed auth/models/checks)
- [x] `protected()` middleware attached to the production router with the Go
      unauthenticated allowlist (verified by test)
- [x] Route-table parity gate test vs the recorded Go route table
      (`crates/surface/api/tests/route_parity.rs`)

Residual (tracked under Roadmap 75): per-handler tenant-context integration
(hosted credential permission checks, tenant-scoped list/persist variants).

## Roadmap 75: Deferred Hook Wiring Closure — complete 2026-08-16

- [x] Real webhook `QuotaGate` backed by the billing plane (reserve+commit,
      deny event `webhook.trigger_quota_denied`, hosted fail-closed)
- [x] Catalog `RequirementChecker` wired to the sandbox plane (fail closed)
- [x] Catalog `PermissionGate` wired to the identity plane (Active tenant)
- [x] Execprofile requirement checker + selection gate, evidence support
      gate wired (same planes)
- [x] Hook-seam inventory recorded in `daemon-roadmaps.md` (clock and
      test-transport seams accepted with rationale)
- [x] Decision recorded for spec 052 non-webhook trigger sources (out of
      scope; webhook plane + channel connectors cover it)
- [x] Roadmap 74 tenant residual, data-isolation slice (llm tenant-scoped
      reads/persist, sessions tenant filter, connector hosted reads);
      hosted-deployment remainder moved to Roadmap 76 pre-soak tasks
- [x] Behavioral tests for each newly wired gate (app hook_wiring_tests)

## Roadmap 76: Rust-Era Release Evidence Re-Run

- [ ] Hosted tenant-context remainder closed before the hosted soak
      (providers hosted permission checks + per-tenant managed auth,
      catalog/evidence/webhook context overrides, by-id tenant guards)
- [x] Soak harness and ops tooling verified against the Rust daemon
      (2026-08-16: `run-soak.sh` targeted-validation run, daemonHealth=pass;
      first real-data boot surfaced and closed a wire-compat class — Go
      `null` for nil slices in persisted JSON, 29 serde enum values mangled
      by `rename_all` acronym snake-casing, the missing default-tenant
      auto-bind in `append_event`, and the MCP websocket
      runtime-inside-runtime boot panic — all fixed at root with the
      workspace suite green)
- [ ] 24-hour test-environment soak with fault drills and resource-growth
      checks (R39 evidence)
- [ ] Full-duration hosted daemon soak on a stable host (R43 evidence)
- [ ] Hosted secrets / connector isolation soak (R37 evidence)
- [ ] Diagnostics stable-host / real-account evidence (R42 evidence)
- [ ] All runs recorded via `release-truth-checklist.md` with commit, host,
      dates, artifacts

## Roadmap 77: Public Launch Gate Execution

- [ ] Real-account workloads across at least 3 channels on the Rust daemon
- [ ] Real-account Feishu/Lark calendar and mail workloads including
      attachments and triage
- [ ] Support evidence bundle generation and redaction verification on the
      release candidate
- [ ] `LaunchGateEvidence` assembled; `POST /v1/release/launch-gate` executed
- [ ] Ship / no-ship decision recorded with evidence index location

## Roadmap 78: Memory Plane Foundation (spec 058) — in progress

Gate opened by operator decision 2026-08-17; design root is
TencentDB-Agent-Memory (layered L0-L3 + governed asset envelope), specs
058-062 revised accordingly.

- [x] `dope-memory` crate: uniform asset envelope (kind/layer/owner/
      tenant/visibility/status/version/bindings), L1 atoms with mandatory
      source links, L2/L3 with member drill-down, policy-gated write
      lifecycle (accept/require-approval/reject, fail closed), supersede
      chains, revoke tombstones, retention sweep, consolidation seam
      (Consolidator trait + trigger bookkeeping: 5-turn/600s-idle/50-atom/
      warm-up-doubling config) and Markdown rendering
- [x] Store: v2 `memory_assets` migration on the baseline + typed DAO +
      boot restore
- [x] API family `/v1/memory/*` (assets CRUD-by-supersede, drilldown,
      approve/reject/revoke, visibility gate, capture, manual consolidate)
      with events (`memory.*`), tenant-context override, and the white-box
      Markdown projection under `<data_dir>/memory/`; behavioral tests
- [ ] Schemas under `schemas/api/` + contract tests; SDK methods;
      operator-shell read surface
- [ ] Model-backed Consolidator + scheduler trigger wiring (spec 059 scope)
- [ ] Schema-inventory row for `memory_assets` (tenant-partitioned)

## Roadmaps 79-82 (specs 059-062) — designed, next in sequence

- [x] Specs 059-062 authored and aligned to the design root
- [ ] 79 context engineering (loadout, L3/L2 bootstrap, symbolic
      compression, consolidation scheduling)
- [ ] 80 knowledge retrieval (BM25+vector+RRF, assets-as-tools)
- [ ] 81 agent-managed skills; 82 audited self-improvement

## Working Rule

When work completes:

- update the task in this registry
- update the roadmap status in [`daemon-roadmaps.md`](daemon-roadmaps.md) if
  the task affects roadmap closure
- do not mark a task or roadmap complete until its full boundary is actually
  closed
