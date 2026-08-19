# P0 Tech Stack Recommendation

> **Superseded (2026-08):** the implementation chose Rust for the daemon
> (`crates/` workspace). This document is the original Go-era recommendation,
> retained for historical reference.

## Decision

For P0, Kura should use:

- Go for the daemon core
- TypeScript for operator-facing clients
- SQLite for local durable state
- schema-first contracts between core, clients, and capability processes

The recommended headline architecture is:

- `Kura: Go daemon + TS clients`

## Why This Is The Recommendation

The daemon is a T0 component.

It is not only a product shell. It is the long-lived system process responsible for:

- API serving
- WebSocket or streaming control-plane traffic
- Web UI asset serving
- session lifecycle
- run and task orchestration
- LLM request dispatch
- IM connector supervision
- background scheduling
- checkpoint and recovery
- plugin or capability supervision

Because of that, the daemon should be optimized first for:

- long-running stability
- concurrency control
- predictable resource behavior
- fault containment
- operational simplicity

Go is the better default fit for that daemon spine than TypeScript running on Node.

## Recommended Stack By Layer

## 1. Daemon Core

Recommended:

- Go

Responsibilities:

- process lifecycle
- config loading and reload
- API server
- control-plane transport
- session routing
- run, task, and step orchestration
- LLM request dispatch
- checkpoint and recovery
- background jobs
- connector supervision
- capability process supervision

Why:

- strong long-running service characteristics
- mature concurrency model
- better fit for a heavy always-on process
- simple deployment as a single static or near-static binary
- easier resource ownership and operational debugging than a Node-first daemon

## 2. Operator Web UI

Recommended:

- TypeScript
- React
- Vite

Responsibilities:

- operator dashboard
- session inspection
- config editing
- run and task visibility
- plugin and capability management

Why:

- UI iteration speed matters
- frontend ecosystem is strongest here
- easy contract sharing with generated client types

## 3. Operator TUI

Recommended:

- separate client, not part of the daemon core

Options:

- Go-native TUI if we want operational consistency with the daemon
- TypeScript plus Ink if we want faster product iteration

Decision guidance:

- if the TUI is primarily an operator console, Go is attractive
- if the TUI is more product-facing and interaction-rich, TS can still make sense

The key point is architectural:

- the TUI should consume the daemon API
- the TUI should not be fused with the daemon runtime

## 4. Capability Processes

Recommended:

- separate processes
- Go by default
- Node or Python only when ecosystem constraints justify it

Examples:

- browser automation
- IM adapters with fragile third-party SDKs
- voice or media workers
- indexing or embedding workers

Why:

- isolates faults
- avoids pulling unstable ecosystems into the T0 daemon
- makes restart and supervision explicit

## 5. Storage

Recommended:

- SQLite for P0 system state
- filesystem for artifacts, attachments, and large blobs

SQLite should hold:

- config metadata
- sessions
- runs
- tasks
- steps
- events
- checkpoints
- schedules
- plugin and capability metadata

Why:

- ideal for local-first and single-daemon deployment
- simple operational story
- strong enough for P0 and likely P1

## 6. Contracts And Schemas

Recommended:

- schema-first contracts
- language-neutral wire protocol

Candidate choices:

- JSON Schema as the contract source
- generated Go and TypeScript types from the schema layer

This contract layer should define:

- daemon API
- control-plane events
- connector contracts
- capability process RPC
- plugin manifests

## Architectural Consequences

This stack recommendation implies the following architecture choices.

## 1. The Daemon Owns The System Spine

The daemon is the system of record for:

- live sessions
- routing
- run execution state
- policy decisions
- connector health
- recovery state

That state should not be split across UI clients or workspace prompt files.

## 2. Clients Are Clients

Web UI and TUI are operator surfaces. They do not own runtime truth.

They should connect to the daemon through stable contracts and remain replaceable.

## 3. Risky Integrations Stay Outside The Core

Anything with weaker stability, weaker trust, or messy dependencies should prefer an external capability process over direct in-daemon linkage.

## 4. Schema Discipline Is Mandatory

Once the daemon and clients use different languages, loosely shaped JSON stops being acceptable.

We should define and version contracts early.

## 5. Runtime And Product Layers Stay Separate

Go owns the system spine.

TypeScript owns fast-moving product surfaces.

This separation is intentional and should remain visible in the repo and module map.

## What We Are Explicitly Not Recommending

## Not Recommended For P0: All-TypeScript Daemon

Why not:

- the daemon has too many T0 responsibilities
- plugin and connector instability would land inside the main runtime
- long-term pressure would likely force a partial rewrite of the core

## Not Recommended For P0: All-Rust Stack

Why not:

- too much implementation weight while product boundaries are still forming
- slows UI and product-surface iteration unnecessarily
- increases early architectural cost without clear P0 payoff

## Not Recommended For P0: Python-First Daemon

Why not:

- weaker fit for a product-grade multi-surface daemon
- worse unification story for Web UI and operator surfaces
- better used selectively for model-heavy workers if ever needed

## Repo And Module Implications

A likely repository shape is:

- `daemon/` for the Go system process
- `web/` for the React operator UI
- `tui/` for the terminal client
- `schemas/` for shared contracts
- `capabilities/` for out-of-process workers or adapters

This does not force separate repos. A monorepo is still the recommended starting point.

## Immediate Next Design Work

This document should drive the next design documents:

- module map
- daemon API and event model
- capability process contract
- deployment model

## Short Version

P0 should be built as:

- a Go daemon that owns the system spine
- TypeScript clients for Web UI and possibly TUI
- SQLite-backed local state
- schema-first contracts across all boundaries

This is the best balance between product iteration speed and long-term runtime stability.
