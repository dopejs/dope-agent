# 058 — Memory Plane Foundation (Layered Asset Model)

## Roadmap Context

Roadmap 78, first slice of the context/knowledge/memory program. The
program's implementation gate was opened by operator decision on 2026-08-17
(recorded in `docs/runtime/daemon-roadmaps.md`); launch-gate evidence work
(Roadmaps 76/77) continues in parallel as operator-run activity.

## Design Root

The memory management philosophy follows
[TencentCloud/TencentDB-Agent-Memory](https://github.com/TencentCloud/TencentDB-Agent-Memory),
adapted onto Kura's planes. The adopted core ideas:

1. **Layered memory pyramid (L0→L3)** — raw conversation (L0), extracted
   atoms (L1: facts/preferences/constraints/events), scenario blocks (L2),
   distilled persona/core profile (L3) — with **deterministic drill-down
   paths**: every layer's record cites the lower-layer records it derives
   from, so any recalled memory can be verified back to raw evidence.
2. **Uniform asset registration** — memories, skills, wiki documents, and
   code graphs are all governed *memory assets* sharing one envelope:
   owner, tenant, visibility, version chain, processing status, and agent
   bindings. Knowledge is decoupled from any agent framework.
3. **Asynchronous consolidation pipelines** — L0 captures synchronously;
   L1/L2/L3 refinement runs as background jobs on turn-count, idle-timeout,
   and volume triggers (with a warm-up doubling schedule for new sessions).
4. **Layered retrieval with fusion fallback** — context bootstrap loads
   L3/L2 first; specific-fact queries fall back to hybrid BM25 + vector
   scoring fused with RRF over L1/L0, capped by item count, character
   budget, and a timeout that skips injection rather than blocking.
5. **Assets as tools, no wholesale injection** — memory enters context
   on-demand through discoverable tools, not as a static prepended prompt.
6. **Dual-layer white-box storage** — ground truth in SQLite; L2/L3
   projections additionally rendered as human-readable Markdown artifacts
   for operator inspection.
7. **Private-first sharing** — new assets default to the owner's private
   visibility; wider visibility is an explicit, auditable transition.

These compose with Kura's binding constraints rather than replacing
them: attribution/scoping/reversibility stay mandatory, recalled memory
stays evidence-not-truth (the drill-down path IS the citation), and every
write stays policy-gated.

## Goal

Ship the memory plane's foundation: the asset envelope, the L1 atom model
with L0 source links, L2/L3 record types with Markdown projections, the
policy-gated write lifecycle, the consolidation seam, storage, API, events,
and operator visibility. Retrieval scoring and automatic pipeline
scheduling are the next slices (059/060).

## In Scope

- A `memory` domain crate owning:
  - **Asset envelope**: `asset_id`, `kind`, `layer` (l0_ref/l1/l2/l3),
    `owner` (actor kind + id), `tenant_id`, `visibility`
    (private/team/restricted/agent), `status`
    (pending/ready/superseded/revoked/expired), `version`,
    `supersedes_asset_id`, agent/persona `bindings`.
  - **L1 atoms**: typed `fact | preference | constraint | event | decision
    | reference`, each with `content`, `source_links` (thread/run/event/
    message ids — required, rejected when absent) and retention class.
  - **L2 scenarios**: titled blocks aggregating member L1 atom ids around
    a workspace/project/use case, with a rendered Markdown body.
  - **L3 persona/core**: per-tenant(+persona) distilled profile with
    member L2 ids and a rendered Markdown body.
  - **Write policy hook**: every create/consolidate/visibility-change is a
    proposal evaluated by a pluggable policy (default: auto-accept
    `preference`/`reference` atoms from operator actors, approval-gated
    agent-actor writes and all visibility widenings, always-reject
    unattributed writes). Fail closed on hook errors.
  - **Lifecycle**: immutable-after-ready; changes supersede; revocation
    tombstones; retention expiry is a recorded transition.
  - **Consolidation seam**: a `Consolidator` trait (extract L1 from an L0
    window; aggregate L2; distill L3) with trigger bookkeeping
    (turn-count N=5, idle 600s, L3 every 50 atoms, warm-up doubling —
    configuration, not code constants). This slice ships the seam, the
    bookkeeping, and a manual trigger API; the model-backed extractor and
    scheduler wiring land with 059.
- **L0**: no duplication — L0 is the existing conversation truth (threads,
  connector messages, events). A `memory.capture` API records out-of-band
  episodes as L0 references.
- **Storage**: `memory_assets` table (tenant-partitioned, schema inventory
  updated) via a v2 migration on the baseline; Markdown projections of
  L2/L3 written under `<data_dir>/memory/` for white-box inspection.
- **API family** (protected, tenant-scoped):
  `GET/POST /v1/memory/assets`, `GET /v1/memory/assets/{id}`,
  `POST /v1/memory/assets/{id}/revoke`,
  `POST /v1/memory/assets/{id}/visibility`,
  `POST /v1/memory/consolidate` (manual trigger),
  `GET /v1/memory/assets/{id}/drilldown` (the deterministic path to L0).
- **Events**: `memory.asset_written/superseded/revoked/expired`,
  `memory.consolidation_run`.
- Schemas + contract tests, SDK methods, operator-shell read surface.

## Phase 2 — Write Path Activation (producers and scheduling)

Phase 1 shipped the plane with zero producers; phase 2 makes memory write
itself. The rule throughout: nothing on the reply path blocks on memory.

- **L0 capture points** (fire-and-forget; failures log, never fail the
  originating request):
  1. chat turns — `kura-chat` captures after a dispatch settles, source
     links `{thread, message/turn, run}`; the assistant reply belongs to
     the same turn's L0 window;
  2. connector ingress — the accepted branch captures with
     `{message: delivery_id}` + thread/session links;
  3. workflow completion — terminal workflows capture one L0Ref with
     `{run}/{workflow}` links.
- **Extraction scheduling**: `record_turn` due (5-turn / warm-up
  1-2-4-8…) enqueues a `memory_consolidation` job on the scheduler plane;
  a 60s scheduler tick sweeps `idle_due` tenants (600s idle) and runs
  `sweep_retention`; at most one in-flight pass per tenant.
- **Model-backed Consolidator**: implements the phase-1 trait over the
  LLM dispatch plane (internal system dispatch, own
  `memory.consolidator.{provider,model,timeoutMs}` config). Guard: the
  extractor cannot invent citations — source links must reference ids
  present in the supplied L0 window; invented ids are dropped and logged.
  Consolidation failures record the run's `error` and retry at most once
  per trigger.
- **Review timing**: pending assets resolve through the memory-native
  `approve/reject` queue in the operator shell — deliberately NOT the
  policy-engine approvals (that plane gates tool execution; mixing queues
  would bury memory reviews under run approvals).

## Out Of Scope (deferred to 059/060)

- Context assembly/loadout/budgets and symbolic compression (059 —
  context engineering consumes this plane, it does not operate it).
- Retrieval scoring (BM25/vector/RRF), assets-as-tools exposure, bulk
  cold-start import, wiki and codegraph asset kinds (the envelope
  reserves the `kind` values) — 060.

## Fixed Decisions

- Attribution is mandatory at the API boundary; the drill-down path is the
  citation and is never stripped.
- Ground truth is SQLite; Markdown projections are derived artifacts and
  never read back as truth.
- Consolidation triggers are configuration with the TencentDB-Agent-Memory
  defaults (5 turns / 600s idle / 50-atom L3 / warm-up doubling).
- Private-first visibility; widening requires the policy gate.
- No embedding/model inference inside the daemon; future vector scoring
  rides the sandbox/adapter planes (060).

## Verification / Definition Of Done

- Behavioral tests: policy accept/approve/reject, supersede chains,
  revoke, retention expiry, drill-down integrity (L3→L2→L1→L0 ids
  resolve), restart restore, tenant isolation.
- Contract tests for all schemas; route-parity gate updated.
- Quickstart demonstrating capture → manual consolidation → drill-down →
  revoke with the Markdown projection inspected on disk.
