# Layered Memory System

## Why A Layered Memory Model

A single "memory store" is not sufficient for a personal agent OS. The system needs different retention, trust, and retrieval semantics for different classes of state.

The main failure modes this design is trying to avoid are:

- accidental persistence of hallucinated facts
- cross-user or cross-project contamination
- context summaries becoming the only surviving source of truth
- high retrieval volume with low decision value
- inability to explain why a memory influenced an action

## Memory Layers

### 1. Working Memory

Short-lived state for the current run or turn.

Examples:

- active objective
- current plan
- most recent tool outputs
- current constraints
- unresolved hypotheses

Properties:

- lifecycle is a run or turn
- stored inside checkpoints or runtime snapshots
- not promoted to long-term memory by default

### 2. Episodic Memory

Event-oriented memory for a session, task, or interaction thread.

Examples:

- what the user asked during a session
- which fixes were attempted
- which tool call failed and why
- what decisions were made in a specific run

Properties:

- append-first
- grounded in events
- can later be compacted into summaries, but raw events remain available

### 3. Project Memory

Stable memory tied to a project, repo, workspace, customer, or recurring problem domain.

Examples:

- repository conventions
- agreed architecture constraints
- deployment caveats
- recurring operational gotchas

Properties:

- strongly scoped
- frequently recalled
- may be pinned or versioned

### 4. Semantic Memory

Recall-oriented knowledge units for supporting retrieval.

Examples:

- document chunks
- code summaries
- FAQ items
- historical design notes

Properties:

- optimized for search
- may use vector and lexical retrieval together
- must retain source references

### 5. Procedural Memory

Memory of how to do things.

Examples:

- workflow templates
- playbooks
- tool usage strategies
- domain-specific execution recipes

Properties:

- high leverage for quality and consistency
- should be more structured than free-form notes
- often domain-specific, such as coding, browser automation, or docs editing

### 6. Canonical Sources

These are not memory in the fuzzy sense. They are the authoritative sources the agent consults.

Examples:

- code repository
- database records
- issue tracker
- document store
- calendar or messaging state

Properties:

- highest trust
- not rewritten by the agent
- used to validate recalled memory when a decision matters

### 7. Audit Journal

An immutable or append-only trail for memory operations.

Examples:

- candidate memory created
- memory promoted
- memory superseded
- memory forgotten or redacted

Properties:

- required for rollback, debugging, and compliance

## Memory Write Path

Long-term memory should not be written directly by the model. Use a staged flow:

1. `capture`
   Store candidate memory with scope, provenance, and trigger.
2. `judge`
   Apply policy to decide whether it should persist beyond the current run or session.
3. `promote`
   Write to episodic, project, semantic, or procedural memory as appropriate.
4. `audit`
   Record the promotion and any supersession or dedup decision.

## Memory Read Path

Memory recall should be a pipeline, not a single vector search.

1. scope filter
2. intent-based layer selection
3. multi-path retrieval
4. rerank by relevance, freshness, and trust
5. evidence pack generation

Suggested evidence pack fields:

- `memory_id`
- `layer`
- `scope`
- `content`
- `source_ref`
- `freshness`
- `confidence`
- `why_recalled`

## Required Invariants

- Every long-lived memory item has provenance.
- Every memory item is scoped.
- Long-lived memory writes are versioned or supersedable.
- Critical decisions can link recalled memory back to canonical source.
- Forget or redact operations also clear derived indexes and caches.

## V1 Storage Guidance

- Postgres for catalog, scope, versioning, and audit
- object storage for large artifacts and snapshots
- vector store for semantic retrieval
- Redis only for short-lived cache and queue coordination

Do not make the vector store the sole source of memory truth.
