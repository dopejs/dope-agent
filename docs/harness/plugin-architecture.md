# Agent Pluginization

Decision date: 2026-08-17 (operator). Reference architecture:
[deepseek-ai/deepseek-harness](https://github.com/deepseek-ai/deepseek-harness)
("Everything is a Plugin"). This document is the planning record for the
program — features are now planned directly in design docs like this one and
implemented; the numbered-spec flow (`docs/specs/NNN-*`) is retired for new
work and kept as history.

## Why

Before building further agent capabilities (session/context management,
retrieval, skills), the daemon is restructured so that every non-kernel
subsystem is a **plugin**: a named unit with declared dependencies, resolved
enablement, and inspectable assembly. The payoff:

- **Session management becomes a plugin decision.** A pure personal agent
  runs a long-session plugin (when to compress, what stays resident, what is
  written to memory); IM channels that support threads run a
  context-per-thread plugin. The daemon core stops hard-coding one context
  policy.
- **Existing features become default plugins** — swappable, disableable,
  and eventually replaceable by out-of-process providers.
- **The assembly is a config artifact**, not compiled folklore: what runs,
  what is disabled, and why is dumped from `/v1/plugins`.

## What we adopt from deepseek-harness

1. **Seam model** — a capability is a Service Definition (interface) + a
   Provider (implementation) + Consumers, designed together. Swapping the
   provider swaps the behavior for every consumer at once.
2. **Profile-layered assembly** — a running daemon is a resolved tree of
   plugin entries; any entry can be disabled/configured by id; the resolved
   result is dumpable.
3. **Three event domains** — persistent facts (our event bus + store),
   waterfall interception hooks (new: `HookBus`, handlers may mutate the
   payload or halt the action), and capability events.
4. **"Model-visible = logged"** — everything that reaches a model must be
   reconstructable from the session log (lands with the hookable agent loop
   in phase 2).

## Deliberate deviation

deepseek-harness has no privileged kernel; we keep one. The trust boundary —
**store, event bus, identity, auth, policy, secrets, audit** — is built
directly in `App::with_profile` and cannot be disabled or replaced by a
plugin. Security posture outranks composability; this is a fixed decision.

## The two-tier model (Rust reality)

- **Tier 1 — builtin plugins** (shipped, phase 1): compiled-in plugins
  (`crates/surface/app/src/plugins.rs`) registered with the kernel crate
  `dope-plugin` (`crates/foundation/plugin`). No dynamic loading (dylib ABI
  hazards are not worth taking for the first release).
- **Tier 2 — out-of-process providers** (phase 3): any seam served remotely
  over the proven planes — adapter RPC, supervised capability processes,
  MCP — installable at runtime through the catalog (`kind=plugin`).
  Language-agnostic, sandboxed, supervised. Consumers cannot tell the tiers
  apart.

## Mechanisms (phase 1, shipped 2026-08-17)

- **`dope-plugin` kernel crate**: `PluginDescriptor` (id, summary,
  provides, requires), `resolve()` (explicit + transitive disable, warnings
  for unknown ids — nothing silently dropped), `PluginProfile`
  (`<data_dir>/plugins.json`; missing file = default profile, malformed file
  fails boot loudly), `SeamMap` (typed assembly-time registry), `HookBus`
  (waterfall interception: ordered handlers, mutate-or-halt).
- **Builtin plugin set**: llm, skills, sandbox, mcp, integrations, calendar,
  mail, providers, connectors, capabilities, chat, billing, activation,
  computer-use, delivery, scheduler, reminders, routines, memory, triage,
  webhooks, catalog, exec-profiles, evidence, evaluation, live-validation,
  setup-wizard, channel-{discord,telegram,slack,matrix}. Declared
  `requires` edges keep fail-closed gates honest (e.g. disabling `billing`
  transitively disables `webhooks` — the quota gate can never run
  half-wired).
- **Disable semantics**: a disabled plugin's `AppState` field stays `None`;
  the API layer already answers not-wired. Channel plugins additionally gate
  their serve-time runtime construction (profile wins over the connector
  config flag; no network or credential is touched).
- **Introspection**: `GET /v1/plugins` returns the boot-time
  `AssemblyReport` (build order, enablement, reasons, warnings). Contracts:
  `schemas/plugin/plugin-profile.schema.json`,
  `schemas/api/plugins-report.schema.json`; SDK `listPlugins()`.

## Phase 2 — hookable agent loop (next)

Restructure chat/dispatch into an explicit turn/step flow with hook points
on the `HookBus` (`agent/pre-step`, `tools/pre-execute`, `tools/post`,
`turn/end`), and establish the session-log invariant: context is **derived**
from the append-only session record (a `derive_messages` projection), not
accumulated ad hoc — anything model-visible is logged first. This is the
seam the session-strategy plugins attach to:

- `personal-session` — long-session policy: session frame (goal/
  constraints) never evicted, extraction-before-eviction into the memory
  plane (058 write path), rendered-not-accumulated window.
- `im-thread-session` — one context per thread for channels with native
  threading.

## Phase 3 — out-of-process plugin providers

Plugin manifest schema under `schemas/plugin/`, catalog `kind=plugin`,
install/lifecycle through the existing supervision planes, seam dispatch
over adapter RPC. After this, the memory consolidator, retrieval scorers,
or a whole channel can live outside the daemon process.

## Sequencing

Pluginization precedes the remaining capability program (operator decision):
session/context management ships **as plugins** after phase 2; retrieval,
agent-managed skills, and audited self-improvement follow. The operator-run
release work (Roadmap 76 soaks, Roadmap 77 launch gate) continues in
parallel and is unaffected — with the default profile the assembly is
behavior-identical to the pre-plugin daemon.

## Verification (phase 1)

- `dope-plugin` unit tests: resolution (explicit/transitive/unknown-id),
  profile load, seam map, hook waterfall (mutate + halt ordering).
- `dope-app` tests: default profile ⇒ all builtins enabled and every seam
  adapter wired (existing wiring tests unchanged); leaf disable ⇒ manager
  `None` + reason recorded; `billing` disable ⇒ dependents transitively
  disabled; `plugins.json` picked up by `App::new`; channel plugin disable
  ⇒ connector runtime skipped despite the config flag.
- Contract tests: profile + report schemas, including a round-trip of the
  actual `resolve()` output through the report schema.

Rollback: revert to the pre-pluginization assembly commit; `plugins.json`
is additive and ignored by older daemons.
