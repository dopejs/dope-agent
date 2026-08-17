# 059 — Context Engineering Foundation (Write/Read Timing On The 058 Base)

## Roadmap Context

Roadmap 79, slice 2 of the context/knowledge/memory program. Revised
2026-08-17 against the shipped 058 foundation: this spec now pins the
concrete integration timing onto the implemented seams (`dope-memory`'s
`capture`/`record_turn`/`idle_due`/`consolidate`, the `Consolidator`
trait, the `WriteDecision` flow, the envelope's `bindings`, and the
`memory.*` events) instead of describing them abstractly.

## Design Root Alignment

TencentDB-Agent-Memory model (see 058): L3/L2 bootstrap first under
budgets, per-agent loadout from bindings + visibility, symbolic
compression of tool logs, async consolidation off the request path.

## Part 1 — Write Timing (when memory gets written)

The base has no producers by design; this slice wires them. The rule
throughout: **nothing on the reply path blocks on memory**.

### W1. L0 capture points

- **Chat turns**: `dope-chat::Service` calls `POST`-equivalent capture
  (manager `create` L0Ref + `record_turn`) after a dispatch settles —
  once per user turn, with source links `{thread: thread_id}`,
  `{message: request_turn_id}`, `{run: dispatch/run id}`. The assistant
  reply is part of the same turn's L0 window, not a second turn.
- **Connector ingress**: the accepted branch of
  `/v1/connectors/{id}/ingress/messages` captures with
  `{message: delivery_id}` + `{thread/session}` links.
- **Workflow completion**: terminal workflows capture one L0Ref with
  `{run}`/`{workflow}` links so task outcomes are extractable evidence.
- Capture is fire-and-forget from the caller's perspective: failures log
  and never fail the originating request.

### W2. Extraction scheduling (L0 → L1)

- `record_turn` returning `true` (turn-count N=5 or warm-up 1/2/4/8…)
  enqueues a consolidation job for that tenant. Execution is a background
  task handed to the scheduler plane (`dope-scheduler` workflow target
  `memory_consolidation`), never inline with the reply.
- **Idle trigger**: a scheduler tick every 60s sweeps `idle_due(tenant,
  now)` (600s idle default) and enqueues due tenants.
- The job builds the L0 window since `last_extract_at` from conversation
  truth (thread continuity reads), then calls `Manager::consolidate`,
  persists the written assets, publishes events, and renders projections
  — exactly what the manual `/v1/memory/consolidate` route does today.
- Deduplication: at most one in-flight consolidation per tenant (the
  scheduler's per-target serialization).

### W3. Model-backed Consolidator

- Implements the 058 `Consolidator` trait over the **LLM dispatch plane**
  (an internal system dispatch; no new inference machinery):
  - `extract_l1`: prompt over the L0 window → JSON array of
    `{atomType, title, content, sourceLinks}` (source links restricted to
    ids present in the window — the extractor cannot invent citations;
    invented ids are dropped and logged).
  - `aggregate_l2`: prompt over ready atoms grouped by workspace/thread →
    scenario drafts.
  - `distill_l3`: prompt over ready scenarios → one persona draft
    (superseding the prior L3, which the base already chains).
- Config: `memory.consolidator.{provider,model,timeoutMs}` — defaults to
  the daemon's default provider; consolidation failures record the run
  with `error` and never retry more than once per trigger.
- Extractor writes carry `owner = system:consolidator`; under the default
  policy they auto-accept. Recorded decision: the stricter variant
  (consolidator writes require approval) is a policy-hook swap, not a
  redesign — operators choose via configuration in the operator shell.

### W4. Review timing (pending assets)

- Agent-authored and policy-gated writes sit in `status=pending`. They
  surface in the operator shell's memory queue (list filter
  `status=pending`) and resolve through the existing
  `/v1/memory/assets/{id}/approve|reject` — memory-native review, NOT the
  policy-engine approvals (that plane gates tool execution; mixing the
  queues would bury memory reviews under run approvals). Recorded
  decision.

### W5. Retention sweep timing

- `sweep_retention(now)` runs on the same 60s scheduler tick; expiries
  persist + publish `memory.asset_expired`.

## Part 2 — Read Timing (when memory enters context)

### R1. Loadout resolution

- Per dispatch: eligible assets = tenant match ∧ status Ready ∧
  (visibility private→owner match | team | agent→`bindings` contains the
  active persona/agent id). Personas (Roadmap 57) and workspace bindings
  (Roadmap 58) supply the ids matched against `bindings`.

### R2. Bootstrap injection (L3/L2 first)

- Chat assembly (opt-in flag `context.memory=true`, default off until 060
  retrieval ships) prepends, in order and within budget:
  1. the tenant's Ready L3 (persona) — at most one by construction;
  2. Ready L2 scenarios ranked by `updated_at`, newest first,
  until the memory budget (`context.memoryBudgetChars`, default 4000
  chars) is exhausted. L1 atoms are NOT bulk-injected — they arrive via
  retrieval (060) or drill-down on demand.
- Every injected asset id is recorded in the dispatch's
  `AssemblyRecord` (what was included, what was excluded and why) — the
  inspectability contract from the original 059 goal.
- Timeout rule (from the design root): assembly reads are in-memory
  (manager state), so no skip-path is needed here; the retrieval
  fallback in 060 carries the 5000ms skip-on-timeout rule.

### R3. Symbolic tool-log compression

- Tool-call outputs above a threshold externalize to
  `<data_dir>/memory/<tenant>/refs/<tool_call_id>.md`; the context keeps
  a one-line symbol (`ref:<tool_call_id>` + summary line). A
  `memory_ref_lookup` tool retrieves full text by id on demand.

## In Scope (implementation checklist)

- Chat/ingress/workflow capture hooks (W1) behind a config flag.
- Scheduler wiring for turn/idle triggers + retention (W2, W5).
- `LlmConsolidator` implementing the trait over the dispatch plane (W3).
- Loadout + bootstrap injection + AssemblyRecord in chat assembly (R1-R2).
- Symbolic ref externalization + lookup tool (R3).
- Operator-shell pending-review queue (W4) and the config surface.

## Out Of Scope

- Retrieval scoring and the memory-search tools (060); wiki/codegraph
  kinds; bulk import; embedding anything.

## Verification / Definition Of Done

- End-to-end test: scripted conversation → captures → turn-trigger fires
  → LlmConsolidator (fake provider in tests) writes atoms with real
  source links → L2/L3 appear after their triggers → next dispatch's
  AssemblyRecord shows the L3/L2 injection under budget.
- Idle-trigger and warm-up schedule tests over a mocked clock.
- Invented-citation drop test (extractor returns an unknown source id).
- Reply-path latency assertion: capture + trigger enqueue add no
  synchronous dispatch work beyond the in-memory bookkeeping call.
