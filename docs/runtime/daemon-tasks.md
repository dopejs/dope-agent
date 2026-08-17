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

## Roadmap 78: Memory Plane Foundation (spec 058) — complete 2026-08-17

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
- [x] Schemas under `schemas/api/` (memory-asset-resource,
      create-memory-asset.request, memory-consolidation-run) + contract
      tests; SDK memory methods + tests; operator-shell Memory section
      (assets view + pending-review queue with approve/reject)
- [x] Phase 2 write-path activation (spec 058): capture hooks live at chat
      turn settle, connector ingress accept, and workflow terminal
      (fire-and-forget); turn-trigger consolidation runs off the reply path
      on the blocking pool; the 60s app tick sweeps idle triggers and
      retention; the LLM-dispatch-backed Consolidator extracts L1/L2/L3
      over the daemon's default provider with invented citations dropped
      (atoms without verifiable evidence are discarded)
- [x] Schema-inventory row for `memory_assets` (tenant-partitioned)

## Agent Pluginization Program — planning flow: direct design docs

Spec numbering retired 2026-08-17 (operator); plan of record is
[`../harness/plugin-architecture.md`](../harness/plugin-architecture.md).
Pluginization precedes all further capability work.

- [x] Phase 1 — plugin kernel + builtin assembly (complete 2026-08-17):
      `dope-plugin` crate (descriptors, profile-driven resolution with
      transitive disable + warnings, `<data_dir>/plugins.json`, SeamMap,
      waterfall HookBus); trust-boundary kernel carved out (store/bus/
      identity/auth/policy/secrets/audit, not disableable); every other
      subsystem re-expressed as one of 31 builtin plugins with declared
      `requires` edges; channel plugins gate serve-time runtime
      construction; `GET /v1/plugins` report + profile/report schemas +
      contract tests + SDK `listPlugins()`; behavior-identical under the
      default profile
- [x] Phase 2 — hookable agent loop (complete 2026-08-17): chat query and
      stream run `chat/turn-start` (query rewrite / veto),
      `chat/pre-dispatch` (provider/model/messages rewrite / veto, ordered
      before dispatch persist so the record is byte-identical to what the
      provider receives — "model-visible = logged", proven by service and
      end-to-end assembly tests), and `chat/turn-end` (observational);
      vetoes are 403 + `chat.hook.vetoed` events; `/v1/plugins` reports
      hook registrations; session-log generalization folded into the
      session-strategy plugin slice
- [x] Phase 3 slice 1 — external hook plugins (complete 2026-08-17):
      plugin-manifest schema; `<data_dir>/plugins/` discovery with
      warnings-not-boot-failures for bad third-party manifests; externals
      resolved through the same profile/requires machinery (`source:
      external`, duplicates lose to builtins); lazy stdio line-JSON process
      host with per-call timeout, one respawn, `onError: continue|veto`
      per hook; kill on close; catalog `kind=plugin`; end-to-end test
      (manifest on disk → chat turn rewritten by the child process)
- [ ] Phase 3 later slices — seam (service) dispatch over adapter RPC
      (serve a builtin seam from an external process) and catalog-driven
      install/update into `<data_dir>/plugins/`
- [ ] Session/context management as plugins (`personal-session`,
      `im-thread-session`) — after phase 2
- [ ] Knowledge retrieval (BM25+vector+RRF, assets-as-tools) — after the
      session-strategy slice
- [ ] Agent-managed skills; audited self-improvement — sequence tail

## Working Rule

When work completes:

- update the task in this registry
- update the roadmap status in [`daemon-roadmaps.md`](daemon-roadmaps.md) if
  the task affects roadmap closure
- do not mark a task or roadmap complete until its full boundary is actually
  closed
