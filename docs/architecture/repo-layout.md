# Repository Layout Plan

> **Superseded (2026-08):** the daemon is now a Rust workspace under
> `crates/` (see `crates/MIGRATION.md`); this document is the original Go-era
> layout plan, retained for historical reference.

## Purpose

This document defines the planned repository structure for DopeAgent before daemon implementation begins in earnest.

The structure is designed to support:

- a Go daemon as the system spine
- TypeScript clients for Web UI and possibly TUI
- supervised capability processes
- schema-first contracts across language boundaries

This is a planning document, not a commitment to implement every directory immediately.

## Design Principles

The repository layout should follow these rules:

- system boundaries should be visible in the repo
- daemon code should not be mixed with client code
- generated contract code should not become the contract source of truth
- risky or optional capability implementations should not be embedded into the daemon tree
- the repo should support incremental delivery without forcing early microservice splits

## Recommended Top-Level Layout

```text
dope-agent/
  archive/
  docs/
  schemas/
  daemon/
  web/
  tui/
  capabilities/
  sdk/
  scripts/
  deploy/
  test/
  tools/
```

## Top-Level Directories

## `archive/`

Purpose:

- preserve exploratory or superseded prototypes without letting them define the main repository shape

Notes:

- transitional only
- not part of the long-term product architecture
- current JavaScript runtime prototype has been moved here

## `docs/`

Purpose:

- product and architecture planning
- protocol notes
- operational and deployment design
- future ADRs

Notes:

- current planning documents already live here
- this should remain the main architecture decision area

## `schemas/`

Purpose:

- source of truth for shared contracts

Should contain:

- daemon HTTP and WebSocket API schemas
- event schemas
- config schemas
- capability RPC schemas
- plugin manifest schemas

Notes:

- this is the contract source of truth
- generated Go or TypeScript types should be derived from here, not hand-maintained separately

Suggested internal structure:

```text
schemas/
  api/
  events/
  config/
  capability/
  plugin/
```

## `daemon/`

Purpose:

- the Go daemon

This should become the main system spine and T0 process.

Suggested internal structure:

```text
daemon/
  cmd/
    dope/
  internal/
    app/
    api/
    auth/
    config/
    router/
    runtime/
    policy/
    llm/
    connectors/
    capabilities/
    scheduler/
    store/
    events/
    checkpoints/
    artifacts/
    telemetry/
  pkg/
```

### `daemon/cmd/dope/`

Purpose:

- the daemon entrypoint

### `daemon/internal/app/`

Purpose:

- daemon assembly
- dependency wiring
- startup orchestration

### `daemon/internal/api/`

Purpose:

- HTTP endpoints
- WebSocket or streaming transport
- request validation and response wiring

### `daemon/internal/auth/`

Purpose:

- operator auth
- pairing
- local trust and token handling

### `daemon/internal/config/`

Purpose:

- config load
- validation
- defaults
- reload behavior

### `daemon/internal/router/`

Purpose:

- session routing
- inbound normalization
- channel and source mapping

### `daemon/internal/runtime/`

Purpose:

- run, task, and step lifecycle
- runtime state transitions

### `daemon/internal/policy/`

Purpose:

- permission decisions
- approval gates
- escalation policy

### `daemon/internal/llm/`

Purpose:

- provider abstraction
- request dispatch
- retries and streaming

### `daemon/internal/connectors/`

Purpose:

- connector registry
- connector supervision
- channel integration control

### `daemon/internal/capabilities/`

Purpose:

- capability registration
- child process supervision
- health checks

### `daemon/internal/scheduler/`

Purpose:

- background jobs
- cron-like wakeups
- retries or delayed work

### `daemon/internal/store/`

Purpose:

- SQLite access layer
- transactions
- durable state persistence

### `daemon/internal/events/`

Purpose:

- event append flow
- event fan-out to clients
- event serialization

### `daemon/internal/checkpoints/`

Purpose:

- checkpoint creation
- restore paths
- recovery flows

### `daemon/internal/artifacts/`

Purpose:

- artifact metadata
- filesystem path rules
- attachment handling

### `daemon/internal/telemetry/`

Purpose:

- logging
- metrics hooks
- health reporting

### `daemon/pkg/`

Purpose:

- narrow reusable packages only if they are genuinely externalizable

Rule:

- default to `internal/`
- do not create `pkg/` just to mirror folders

## `web/`

Purpose:

- operator-facing Web UI in TypeScript

Suggested internal structure:

```text
web/
  src/
    app/
    pages/
    features/
    components/
    hooks/
    lib/
    generated/
  public/
```

Notes:

- `generated/` is for schema-derived client types or API helpers
- runtime truth should remain in the daemon, not in browser state

## `tui/`

Purpose:

- terminal operator client

Suggested internal structure:

```text
tui/
  src/ or cmd/
  generated/
```

Notes:

- language is intentionally undecided for now
- the TUI should remain a client of daemon contracts
- it should not import daemon internals directly

## `capabilities/`

Purpose:

- supervised external capability processes

Suggested internal structure:

```text
capabilities/
  browser/
  connectors/
  media/
  memory/
```

Notes:

- only capabilities that benefit from process isolation should live here
- not every feature needs its own process in P0

### `capabilities/browser/`

Purpose:

- browser automation worker

### `capabilities/connectors/`

Purpose:

- connector processes that should not live in the daemon

Examples:

- unofficial or fragile IM SDK-based integrations

### `capabilities/media/`

Purpose:

- voice and media workers when needed

### `capabilities/memory/`

Purpose:

- future indexing or retrieval workers if memory evolves that way

## `sdk/`

Purpose:

- generated or maintained client libraries for daemon contracts

Suggested internal structure:

```text
sdk/
  ts/
  go/
```

Notes:

- do not build this before the contracts stabilize
- it exists to support automation and external clients later

## `scripts/`

Purpose:

- repo automation
- code generation
- local dev helpers
- release helpers

Examples:

- schema generation
- dev startup scripts
- fixture seeding

## `deploy/`

Purpose:

- packaging and deployment assets

Examples:

- systemd units
- launchd plists
- container files
- remote installation helpers

## `test/`

Purpose:

- cross-module integration and end-to-end tests

Suggested internal structure:

```text
test/
  integration/
  e2e/
  fixtures/
```

Rule:

- package-local unit tests should stay beside their code
- repo-level tests go here

## `tools/`

Purpose:

- developer-only helper programs
- local generators
- maintenance utilities

Examples:

- contract inspection tools
- local debug utilities
- migration helpers

## Root Files We Should Expect Later

Likely root-level files:

```text
README.md
go.work
pnpm-workspace.yaml
Makefile
.gitignore
```

Optional later:

```text
Taskfile.yml
buf.yaml
```

The exact tooling files can wait until we finalize schema tooling and client generation.

## Current Repository State

The previous exploratory JavaScript runtime prototype now lives under:

- `archive/js-runtime-prototype/`

It should be treated as historical reference only, not as the target implementation path.

## Migration Guidance

When we start real implementation, the transition should be:

1. create the final top-level directories
2. move planning-only exploratory runtime code out of the root path or archive it
3. start daemon work under `daemon/`
4. define schemas under `schemas/`
5. start Web UI under `web/`

Do not keep building new core code in archived prototype paths.

The root `src/` directory should not be recreated as the default home for new core code.

## Transitional Rule

The archived JavaScript prototype may still be useful for reference while we define the Go daemon, but:

- it should not receive new feature work
- it should not define production contracts
- it should not shape the final runtime architecture

## Immediate Next Use

This repository layout plan should inform:

- daemon feature planning
- daemon API planning
- contract generation planning
- storage model planning

## Short Version

The repository should be organized around system boundaries, not convenience folders.

The daemon gets its own Go tree.

Clients get their own trees.

Capabilities get their own supervised process area.

Contracts live in `schemas/` as the shared source of truth.
