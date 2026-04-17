# Foundation-First Plan

## Decision

Defer the memory system and build the agent OS foundation first.

## Why

Designing memory before the runtime boundary is stable creates avoidable churn. Memory shape depends on:

- what a run is
- what a step is
- what gets checkpointed
- what an event means
- how tool execution is represented
- where policy gates sit
- how a host platform hands work to the core

Without those primitives, memory design tends to become prompt-centric and hard to evolve.

## What Counts As The Foundation

The initial foundation should include:

- runtime object model
- event schema
- step state machine
- checkpoint and restore
- tool execution envelope
- policy decision envelope
- replayable event log
- host adapter contract

## What Is Explicitly Out Of Scope For This Stage

- semantic memory
- project memory
- episodic memory promotion
- recall and rerank pipelines
- memory compaction policies

## Success Criteria

- A run can be created, resumed, and cancelled.
- A run can produce ordered step events.
- Tool calls and policy decisions are represented as first-class records.
- Runtime state can be checkpointed and restored.
- The OpenClaw host boundary can be described without relying on markdown prompt files as state carriers.

## Suggested First Build Order

1. contracts
2. runtime state machine
3. checkpoint store abstraction
4. tool gateway abstraction
5. policy abstraction
6. host adapter boundary
7. replay and inspection helpers
