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
- **Profile management** (added with the config surface): `GET/PUT
  /v1/plugins/profile` read and atomically replace `plugins.json` (boot-time
  effect; responses carry `restartRequired: true`); SDK
  `getPluginProfile()`/`updatePluginProfile()`; the operator shell has a
  Plugins section (`/plugins`) — assembly list with enable/disable toggles,
  hook registrations, profile warnings, and the restart notice.

## Phase 2 — hookable agent loop (shipped 2026-08-17)

The chat pipeline (query and stream) now runs three kernel hook points:

- `chat/turn-start` — before prompt assembly; hooks may rewrite the query
  or halt (veto). Payload: `{tenantId, threadId, query, sourceKind}`.
- `chat/pre-dispatch` — after full context assembly (skills, profile,
  continuity), before the dispatch is prepared and persisted; hooks may
  rewrite provider/model/messages or halt. **This ordering is the
  "model-visible = logged" invariant**: the dispatch record is created
  after the hooks run, so what is persisted is byte-identical to what the
  provider receives (covered by tests at the service layer and end-to-end
  through the real assembly).
- `chat/turn-end` — after the dispatch settled and continuity was
  persisted; observational (a halt only stops later handlers).

A veto surfaces as `ChatError::HookVetoed` → HTTP 403 and is recorded as a
`chat.hook.vetoed` event. `GET /v1/plugins` reports every hook registration
(`hooks: [{point, pluginId}]`). Context already derives from persisted
records (continuity turns render into messages per dispatch — the
`derive_messages` shape); generalizing that log into a session-event model
is folded into the session-strategy plugin slice, which attaches at
`chat/pre-dispatch`.

### Session-strategy plugin (first slice shipped 2026-08-17)

The `session-strategy` builtin (policy engine: `crates/domains/session`)
attaches at `chat/pre-dispatch` and shapes the assembled window
deterministically — before the dispatch is persisted, so the shaped window
is exactly what is logged and what the model sees. Mechanism:
frame-preserving elision — system messages (persona, skills, safety) are
the frame and never elided; the most recent `keepRecent` non-system
messages are always kept; over-budget oldest history is replaced by one
marker line pointing at thread continuity and the memory plane. Eviction
is safe by construction: every turn is captured to L0 at settle
independently of the window.

Two strategies share the mechanism and differ by budget, keyed off the
turn's `sourceKind`: **personal** (long-session default, 48k chars) and
**thread** (`sourceKind=channel`, tight 16k chars — one context per
thread; thread scoping itself comes from the continuity plane). Operator
config via the profile entry
(`entries.session-strategy.config.{personalBudgetChars,threadBudgetChars,keepRecent}`);
a malformed config fails the boot loudly. Disabling the plugin restores
unshaped windows.

Later slices: compression-to-memory (elided spans summarized as L2 assets
instead of dropped), session frame objects (explicit goal/constraint
records), and channel-native thread segmentation policies.

## Behavioral pluginization (shipped 2026-08-17)

Assembly-level pluginization (phase 1) made subsystems disableable; this
slice moved their **behavior** onto plugin mechanisms:

- **Lifecycle seam**: plugins register `on_start`/`on_close` callbacks
  during build; `App::serve`/`App::close` run the registries instead of
  hardcoding manager names. Scheduler and reminders own their start+close,
  sandbox its close, and memory its 60s consolidation/retention tick.
- **Memory capture is a hook**: the memory plugin registers a
  `chat/turn-end` observer that L0-captures the settled turn and schedules
  due consolidation off the reply path — the hardcoded API-layer call is
  gone. Two behavior notes: stream turns are now captured too (previously
  only non-stream queries were), and channel-source turns are skipped in
  the hook because connector ingress capture already records them
  (unifying the two capture paths is a tracked follow-on).
- **Connector runtimes stay kernel-hosted for now** (recorded decision):
  the four channel runtime builders live in `App::serve` because the
  telegram/matrix transports are `!Send` and run on raw-pointer threads —
  moving that gymnastics behind a plugin-owned lifecycle callback adds
  risk without changing behavior. Channel plugins own identity,
  enablement, and gating; full runtime ownership lands with the seam-RPC
  slice, where a channel can be served out of process entirely.

## Phase 3 — out-of-process plugin providers (slice 1 shipped 2026-08-17)

Shipped: external plugins as supervised stdio processes attaching to the
hook plane.

- **Manifest** (`schemas/plugin/plugin-manifest.schema.json`): id/version/
  summary/provides/requires, `hooks: [{point, onError}]`, `entry: {kind:
  "process", command, args, timeoutMs}`.
- **Discovery**: `<data_dir>/plugins/<dir>/manifest.json` at boot.
  Third-party content never bricks the boot — malformed manifests are
  skipped with report warnings (unlike the operator-owned profile, which
  fails loudly). Externals resolve through the same profile/`requires`
  machinery as builtins and appear in `/v1/plugins` as `source:
  "external"`; duplicate ids lose to builtins.
- **Process host**: lazy spawn on first hook call, line-JSON protocol
  (request `{point, payload}` → response `{outcome, reason?, payload?}`,
  response payload replaces the hook payload), per-call timeout, one
  respawn per call for a dead child, killed on daemon close. Failures
  follow the hook's `onError` policy: `continue` (availability first,
  default) or `veto` (fail closed — policy plugins). An external process
  can therefore rewrite context or veto turns exactly like a builtin hook
  (proven end to end: manifest on disk → assembly → chat turn rewritten by
  the child process).
- **Catalog**: `kind=plugin` accepted by the install catalog.

Trust note: manifests execute commands from the data dir — installing an
external plugin is code execution with daemon privileges, same trust class
as installing a capability. Distribution/verification flows ride the
catalog trust tiers.

Remaining for later slices: seam (service) dispatch over adapter RPC —
serving a whole builtin seam (e.g. the memory consolidator or a channel)
from an external process — and catalog-driven install/update lifecycle
placing plugins into `<data_dir>/plugins/`.

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
