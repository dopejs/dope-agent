# P0 Daemon Scope

## Purpose

This document defines the responsibility boundary of the P0 daemon.

It answers four questions:

- what the daemon is
- what the daemon owns
- what the daemon explicitly does not own
- what the minimum P0 daemon feature set should be

## Definition

The daemon is the T0 system process of Kura.

It is a long-running local-first or server-hosted process that acts as the system spine for:

- operator APIs
- session and routing truth
- runtime execution truth
- LLM dispatch
- connector and capability supervision
- recovery and persistence

The daemon is not just an API wrapper and not just a chat bridge.

It is the authoritative control and execution process of the product.

## The Daemon Owns

The daemon owns the following system truths:

- configuration truth
- operator auth and pairing truth
- session truth
- routing truth
- run, task, and step truth
- policy and approval truth
- connector liveness truth
- capability liveness truth
- checkpoint and recovery truth
- artifact metadata truth

## The Daemon Does Not Own

The daemon should not directly own:

- browser UI rendering
- terminal UI rendering
- rich browser automation internals
- fragile third-party SDK runtimes when they can be isolated
- future heavy indexing or retrieval workers

Those belong to clients or supervised capability processes.

## P0 Responsibilities

## 1. Process Lifecycle

- start
- stop
- reload config where safe
- expose health and version

## 2. Operator Control Plane

- HTTP API
- event streaming
- auth
- pairing
- local trust

## 3. Session And Routing

- normalize inbound messages
- map inbound traffic to sessions
- isolate direct and group contexts
- choose target agent or workspace

## 4. Runtime Execution

- create and drive runs
- manage tasks and steps
- append runtime events
- support cancellation and resume

## 5. Policy And Approvals

- capability allow and deny decisions
- operator approval hooks
- enforcement of runtime constraints

## 6. LLM Orchestration

- provider selection
- request shaping
- retries
- streaming
- usage accounting

## 7. Connector Supervision

- start, stop, and monitor connectors
- detect failure
- restart with policy

## 8. Capability Supervision

- spawn external capability processes
- heartbeat
- health and restart policy
- registration and discovery

## 9. Persistence And Recovery

- durable state writes
- checkpoints
- recovery after restart

## P0 Non-Goals

The daemon should not try to solve these in the first milestone:

- full long-term memory system
- complex multi-node distributed coordination
- marketplace-grade plugin runtime
- high-complexity voice and media orchestration inside the core process
- UI-specific local state management

## Minimum External Interfaces

The daemon should expose these interface categories in P0:

- operator API
- event stream
- connector contract
- capability process contract
- config contract

These should all be schema-defined.

## Architectural Rule

If a feature threatens daemon stability, it should move out of the daemon before it moves into the daemon.

That rule should guide:

- browser automation
- unofficial IM stacks
- voice and media features
- future heavy retrieval workers

## Immediate Next Use

This document should directly feed:

- daemon API and event model
- daemon package structure
- connector contract design
- capability process contract design

## Short Version

The daemon is the authoritative system spine of Kura.

It owns truth, routing, runtime, supervision, and recovery.

It should stay operationally conservative and avoid absorbing unstable feature logic that belongs in supervised external processes.
