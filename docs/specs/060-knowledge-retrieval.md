# 060 — Knowledge Retrieval

## Roadmap Context

Roadmap 80 (context/knowledge/memory program, slice 3). Depends on 058/059.

## Design Root Alignment (revised against the 058 base)

Follows the TencentDB-Agent-Memory retrieval model: hybrid BM25 + vector
scoring fused with RRF as the L1/L0 fallback behind 059's L3/L2 bootstrap;
results capped by count (default 5) and character budget with the 5000ms
skip-on-timeout rule (a slow recall skips injection, never blocks the
reply); memory exposed to agents as discoverable tools, never
wholesale-injected. Wiki and codegraph collections register through the
same 058 asset envelope (`kind=wiki|code_graph`).

Timing pinned to the base:

- **Index update timing**: the BM25 index updates on the `memory.*` event
  stream (asset_written/superseded/revoked/expired) — no separate crawl of
  the assets table; boot rebuilds from `list_all_memory_assets`.
- **Query timing**: (a) dispatch assembly calls recall AFTER the 059
  bootstrap when the remaining memory budget is non-zero; (b) agents call
  the `memory_search` / `memory_recall` tools mid-run through the existing
  tool plane; (c) the drill-down of every hit stays attached (`sourceLinks`
  / `memberAssetIds` are never stripped).
- **Visibility timing**: recall applies the same loadout filter as 059
  (tenant ∧ Ready ∧ visibility/bindings) at query time, not index time, so
  visibility changes take effect immediately without reindexing.
- **Vector scoring**: optional, feature-flagged, embeddings via the
  adapter plane; absent embeddings, BM25-only is the supported first
  release mode.

## Goal

Source-linked retrieval over the memory plane and operator-registered
document collections: queries return scored candidates whose every item
cites its source record, for consumption by the context pipeline (059).

## In Scope

- A `retrieval` domain crate: collection registry (memory plane is the
  first collection; operator-registered document roots the second),
  indexing lifecycle with recorded index state, lexical scoring first
  (deterministic BM25-class), and a pluggable embedding hook (off by
  default; when enabled, embeddings ride the existing sandbox/adapter
  planes — no in-daemon model inference).
- `POST /v1/retrieval/queries` + index management APIs; retrieval results
  carry `sourceLink` + score + collection provenance; events + schemas +
  SDK + operator-shell surfaces.
- 059 integration: retrieval becomes an additional context source under
  the same budget/assembly-record rules.

## Out Of Scope

- A general-purpose knowledge graph as primary storage (non-goal).
- Autonomous re-indexing policies; index refresh is operator-triggered or
  schedule-driven via the existing scheduler.

## Fixed Decisions

- Recalled results are evidence, not truth: consumers must surface the
  citation; the retrieval API never strips source links.
- Deterministic lexical scoring ships first; semantic scoring is an
  additive, feature-flagged layer.

## Verification / Definition Of Done

- Golden-query behavioral tests over fixture collections; index
  restart-recovery tests; contract tests; a quickstart tracing one answer
  from query to cited source.
