# Memory

DopeAgent's memory plane follows the layered
[TencentDB-Agent-Memory](https://github.com/TencentCloud/TencentDB-Agent-Memory)
model: a pyramid of governed, attributable, reversible assets.

## The layers

| Layer | What it holds | Rule |
|-------|---------------|------|
| **L0** | episode references — bounded excerpts pointing at the real conversation/run records | truth stays behind the source links |
| **L1** | atoms: facts, preferences, constraints, decisions | **mandatory source links** — no atom without evidence |
| **L2** | scenario summaries | drill down via member assets |
| **L3** | persona distillations | the top of the pyramid |

Everything shares one envelope: kind (`chat_memory`/`skill`/`wiki`/
`code_graph`), owner, tenant, visibility (`private`/`team`/`agent`/
`restricted`), status, version, supersede chains, revoke tombstones,
retention.

## How memory gets written

Five capture paths feed L0 automatically:

1. **Chat turns** (query + stream) — via the memory plugin's
   `chat/turn-end` hook
2. **IM gateway traffic** — same hook (`sourceKind: channel`, with
   message/thread/dispatch links)
3. **HTTP ingress** — accepted inbound messages
4. **Workflow terminals** — task outcomes
5. **Session eviction** — spans elided from the context window are
   captured before they leave (never plain-dropped)

An LLM-backed **consolidator** distills L0 → L1 atoms → L2 scenarios →
L3 personas, off the reply path (turn triggers + a 60s idle tick).
Extracted atoms must cite verifiable evidence — **invented citations are
discarded**.

## Governance

- Write policy is fail-closed: agent-authored writes require operator
  approval (the review queue in the web shell); visibility widening
  requires approval.
- Every asset is white-box: ready L2/L3 assets project to Markdown under
  `<data_dir>/memory/` for direct inspection.
- Revocation tombstones and retention sweeps are first-class.

## API

```bash
GET  /v1/memory/assets?layer=l1&status=ready
POST /v1/memory/assets                       # governed create
GET  /v1/memory/assets/{id}/drilldown        # deterministic path to evidence
POST /v1/memory/assets/{id}/approve|reject|revoke|visibility
POST /v1/memory/consolidate                  # manual trigger
```

SDK: `listMemoryAssets`, `createMemoryAsset`, `getMemoryDrilldown`,
`approveMemoryAsset`, `consolidateMemory`, …
