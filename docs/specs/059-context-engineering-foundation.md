# 059 — Context Engineering Foundation

## Roadmap Context

Roadmap 79, slice 2 of the context/knowledge/memory program. Revised
2026-08-17: the memory *write path* (capture points, consolidation
scheduling, the model-backed Consolidator, review timing) belongs to the
memory plane itself and moved to 058 phase 2. This spec is context
engineering proper: **how a dispatch's context gets assembled** —
deterministically, inspectably, under budgets — from the sources the
daemon owns.

## Design Root Alignment

TencentDB-Agent-Memory (see 058): L3/L2 bootstrap first under explicit
budgets; per-agent **loadout** instead of global injection; symbolic
compression of bulky tool logs; nothing enters context without an
on-demand path back to its evidence.

## Goal

A daemon-owned context assembly pipeline that builds each dispatch's
context from typed sources under an explicit budget and records exactly
what was included, what was excluded, and why — reconstructable after the
fact. The model proposes nothing here: assembly is deterministic given the
same inputs (control-plane determinism principle).

## The Assembly Pipeline

### Sources (typed, ordered)

1. **Persona** (Roadmap 57): the active agent profile projection.
2. **Workspace bindings** (Roadmap 58): the dispatch's workspace/capability
   context.
3. **Thread continuity** (Roadmap 55): the bounded continuity preview.
4. **Memory bootstrap** (058): the tenant's Ready L3, then Ready L2
   scenarios newest-first — L1 atoms are never bulk-injected (they arrive
   via retrieval (060) or drill-down on demand).
5. **Retrieval results** (060, when shipped): recall over the remaining
   memory budget.

### Loadout resolution

Eligible memory assets per dispatch = tenant match ∧ status Ready ∧
visibility admits the caller (`private` → owner match, `team`,
`agent` → the asset's `bindings` contains the active persona/agent id).
Persona and workspace bindings supply the ids matched against `bindings`.
Visibility is evaluated at assembly time, so changes take effect on the
next dispatch without any rebuild.

### Budgets

- Per-source character allowances from configuration
  (`context.budget.{persona,continuity,memory,retrieval}` — memory
  default 4000 chars). Total context stays within
  `context.budget.total`.
- Overflow policy: sources truncate in reverse priority order (retrieval
  first, persona last); every truncation/exclusion is recorded, never
  silent.

### AssemblyRecord

Every assembled context produces an `AssemblyRecord`: the ordered list of
included items (source kind, asset/preview id, chars), the excluded items
with reasons (over-budget, visibility, timeout), and the budget totals.
Records are inspectable via `GET /v1/context/assemblies/{id}` and linked
from the dispatch (`context.assembled` event carries the id). This is the
inspectability contract: an engineer can reconstruct exactly what the
model saw.

### Symbolic tool-log compression

Tool-call outputs above a size threshold externalize to
`<data_dir>/memory/<tenant>/refs/<tool_call_id>.md`; context keeps a
one-line symbol (`ref:<tool_call_id>` + summary). A `memory_ref_lookup`
tool retrieves the full text by id on demand — token cost drops, full
traceability stays.

### Dispatch integration

Chat/dispatch opt in per request (`context.memory=true`; default off
until 060's retrieval ships alongside). With the flag off, behavior is
byte-identical to today.

## In Scope

- A `context` domain crate: source registry, loadout resolver, budget
  model, assembler, AssemblyRecord (+ store table via v3 migration).
- Chat-service integration behind the opt-in flag.
- `GET /v1/context/assemblies/{id}` + `context.assembled` event, schemas,
  SDK.
- Symbolic ref externalization + the `memory_ref_lookup` tool.

## Out Of Scope

- Everything that writes memory (058 phase 2); retrieval scoring and
  memory-search tools (060); learned/adaptive budget weights (082 —
  proposals through the audited improvement loop only).

## Fixed Decisions

- Assembly is deterministic given the same inputs; no model-driven choice
  inside the pipeline.
- Every assembled context is reconstructable from its AssemblyRecord.
- Recalled memory enters context with its citation intact, never as bare
  text.

## Verification / Definition Of Done

- Golden tests: fixed sources → identical AssemblyRecord across runs.
- Budget-overflow tests: exclusion ordering and recorded reasons.
- Loadout tests: visibility/bindings changes reflected on the next
  assembly.
- End-to-end: a dispatch with the flag on shows the L3/L2 injection in
  its AssemblyRecord; with the flag off, unchanged behavior.
- Ref-compression round trip: externalized tool log retrievable by id.
