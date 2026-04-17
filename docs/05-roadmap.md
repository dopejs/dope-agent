# Initial Roadmap

## Phase 0: Framing

Deliverables:

- architecture decision memo
- runtime object model
- host platform boundary decision
- foundation-first implementation plan

Exit criteria:

- clear V1 scope
- accepted invariants
- agreed control-plane/core boundary
- memory explicitly deferred from the first implementation milestone

## Phase 1: Core Skeleton

Deliverables:

- repository scaffold
- event schema
- run, step, and checkpoint model
- minimal orchestrator
- minimal tool gateway contract
- policy gate skeleton
- replay-friendly event model

Exit criteria:

- can create a run
- can append events
- can save and load checkpoints
- can execute valid step transitions
- can reject invalid transitions cleanly

## Phase 2: Host Integration MVP

Deliverables:

- adapter from OpenClaw shell into new runtime
- dual-run mode for migration
- mapping from existing sessions to new run model
- explicit host-to-core contract

Exit criteria:

- OpenClaw can host the new runtime without relying on legacy memory semantics
- host and core state ownership are explicit

## Phase 3: Context Assembly

Deliverables:

- structured context envelope
- compaction engine
- handoff protocol between parent and specialized agents

Exit criteria:

- model context is assembled from runtime state rather than mostly free-form prompt files
- context shrinkage is loss-aware and testable

## Phase 4: Memory MVP

Deliverables:

- candidate memory capture path
- project and episodic memory stores
- semantic retrieval path
- recall API with evidence packs
- audit journal

Exit criteria:

- runtime can request memory recall by scope and intent
- candidate memories can be promoted with audit records
- memory influence is inspectable in replay

## Phase 5: Capability Domains

Deliverables:

- coding capability pack
- architecture review capability pack
- personal assistant capability pack

Exit criteria:

- capability agents share runtime and memory primitives
- domain-specific procedures improve quality without redefining the core

## Cross-Cutting Work

- observability
- replay tooling
- permission and escalation policy
- redaction and data deletion flows
- evals and reliability tests

## Suggested Immediate Next Steps

1. Define the runtime event schema.
2. Build a minimal run plus checkpoint prototype.
3. Define the tool-call and policy contracts.
4. Sketch the OpenClaw adapter boundary.
5. Defer memory API design until the runtime boundary is stable.
