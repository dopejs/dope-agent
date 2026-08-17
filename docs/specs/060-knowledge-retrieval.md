# 060 — Knowledge Retrieval

## Roadmap Context

Roadmap 80 (context/knowledge/memory program, slice 3). Depends on 058/059.

## Design Root Alignment

Follows the TencentDB-Agent-Memory retrieval model (see 058): hybrid BM25 +
vector scoring fused with RRF as the L1/L0 fallback behind the L3/L2
bootstrap; results capped by count/character budget with a skip-on-timeout
rule; memory exposed to agents as discoverable tools (search/recall) via
the existing tool plane, never wholesale-injected. Wiki and codegraph
collections register through the same 058 asset envelope.

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
