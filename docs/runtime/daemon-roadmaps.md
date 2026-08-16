# Daemon Roadmaps — Remaining Work (Post-R72, Rust Era)

> **Supersession note (2026-08-16):** this document replaces the pre-Rust-era
> roadmap document. The full per-roadmap execution record for Roadmaps 1-72
> (goals, tasks, definitions of done) lives in this file's git history
> (revisions before 2026-08-16). That version carried stale statuses
> (`[ ] proposed`) for Roadmaps 45-72; the ledger below is the corrected
> record. Statuses here are authoritative.

## Purpose

This document defines the execution structure for the remaining daemon work.

The delivery unit is a **roadmap**, not an isolated task. Each roadmap:

- forms a closed vertical slice
- contains multiple tasks
- has a roadmap-level definition of done
- is only complete when every in-scope task meets its own completion standard

Each implementation round should finish **one whole roadmap**.

## Execution Standard

- A roadmap is the planning and delivery boundary; a task is an auditable work
  item inside it.
- A task is `[x]` only when its full definition of done is satisfied. A roadmap
  is `[x]` only when every required task is satisfied and the roadmap-level
  definition of done is met.
- Partial, provisional, or narrow implementations stay `[ ]` with a note like
  `(partial)` or `(blocked)`. Demo-grade shortcuts do not count as completion.
- A roadmap too large for one round must be re-cut before implementation
  starts, not partially completed and counted as done.
- **Implementation status and release-evidence status are tracked separately**
  (the Roadmap 44 standard). Release-closure claims must go through
  [`release-truth-checklist.md`](release-truth-checklist.md).

## Current State (2026-08)

- **Roadmaps 1-72 are all implemented and merged to `main`** (one
  `Complete phase N` / `Complete roadmap N` commit per slice; the final
  non-knowledge parity slice landed 2026-06-30, commits `9d542ea..52a2580`
  plus the MVP-gap closure through `02fd047`).
- **The Go daemon was then fully ported to the Rust workspace** (`crates/`,
  8 waves, see [`crates/MIGRATION.md`](../../crates/MIGRATION.md)). The Go
  `daemon/` tree and the TypeScript TUI are removed; the control plane is the
  Rust `dope-cli` binary, the terminal client is `dope-tui`.
- Verification commands: `cargo test --workspace` (or `make daemon-test`) and
  `make daemon-contract-test`. Go-era `go test ./...` references in archived
  documents map to these.
- **What is not closed** is captured below: release evidence (all soak/launch
  evidence predates the Rust port), deferred permissive hooks, documentation
  truth, and the gated context/knowledge/memory program.

## Completed Roadmap Ledger (1-72)

Implementation status: all `[x]`. Spec-number → roadmap-number mapping lives in
[`../specs/README.md`](../specs/README.md); the detailed execution record is in
this file's git history (pre-2026-08-16 revisions) and in `specs/<NNN>-.../`.

| Roadmaps | Program | Notes |
|---|---|---|
| 1-13 | Daemon core: runtime, supervision, LLM dispatch, trust, contracts, conversation, clients, ingress, providers, IM loop, streaming | |
| 14-24 | Test env, skills, sandbox plane, MCP execution/catalog/transports, tool-call orchestration | R20 follow-ons (stronger backends, VM-grade isolation) deferred, see below |
| 25-33 | Personal-agent surfaces: scheduling, computer use, integrations, delivery, calendar, mail, reminders, operator shell, evaluation harness | |
| 34-44 | Tenancy, hosted secrets, billing/quotas, production ops, live validation, evaluation expansion, diagnostics, hosted profile, release truth | R37/39/42/43 carry open evidence debt, see register below |
| 45-53 | Hosted activation, credential wizard, quota UX; channel conformance, Discord hardening, Telegram, Slack, Matrix, channel repair UX | |
| 54-59 | Thread/session lifecycle, continuity, group reset/handoff, persona, workspace binding, integration adapter plane | |
| 60-72 | Real calendar/mail closure (Feishu/Lark), attachments, inbox triage, routines, webhooks, catalog, exec-profile UX, operator shell productization, evidence bundle, launch-gate validator | R72 delivered the *validator*; the launch-gate *run* is still open, see Roadmap 76 |

## Release Evidence Debt Register

These roadmaps are implementation-complete but their release evidence is not
closed. Additionally, **every existing soak record predates the Rust port**
(Go daemon, commit `5ad95ba`, host `zentalk-1`, 2026-05-01), so under the
release-truth standard none of it certifies the shipped Rust implementation.

| Roadmap | Open evidence |
|---|---|
| 37 Hosted secrets & connector isolation | final soak evidence |
| 39 Production install/upgrade/backup/soak | final release soak evidence |
| 42 Integration health & permission diagnostics | stable-host / real-account release evidence |
| 43 Hosted operational profile & recovery | full-duration hosted daemon soak (`hosted_soak_pending`) |
| 72 Public release soak & launch gate | operator-run hosted soak + real-account evidence index; ship/no-ship decision |

Closing this register is Roadmaps 75 and 76.

---

## Roadmap 73: Documentation And Execution-Record Truth Closure

Status: `[ ] planned` (partially delivered by this document replacement)

### Goal

Make every in-repo claim about the control plane match the Rust reality, so an
engineer joining after the migration cannot be misled by Go-era text.

### Context

- Project `CLAUDE.md` still documents a "Go 1.24" daemon in `daemon/` with
  `daemon/internal/...` package paths; that tree no longer exists. The
  `Makefile` targets already run cargo.
- `crates/surface/api/src/` carries stale wave-8 TODO comments, e.g.
  `state.rs` claims "the dope-reminders crate does not exist" while the crate
  exists, compiles, and is wired in `crates/surface/app/src/lib.rs:338`.
  Stale markers also remain in `types.rs`, `middleware.rs`, `routes/auth.rs`,
  and `routes/setupwizard.rs`.
- The pre-replacement roadmap document carried `[ ] proposed` for 28 shipped
  roadmaps; the old task registry linked to a nonexistent absolute path and
  described R17/R18/R20 as in-progress/planned.

### Tasks

- [x] Replace `daemon-roadmaps.md` and `daemon-tasks.md` with current-state
      documents; retire the stale R1-72 documents and superseded roadmap-split
      planning docs to git history (this change)
- [ ] Rewrite the daemon section of `CLAUDE.md` for the Rust workspace
      (crate layout under `crates/`, cargo/make commands, entry point
      `dope-cli`, app assembly in `crates/surface/app/src/lib.rs`)
- [ ] Audit every `TODO: PLACEHOLDER` / `TODO: MISSING` marker in
      `crates/surface/api/src/`; delete markers that describe closed gaps,
      keep only ones matching a real open seam, and reference this roadmap
      from any that remain
- [ ] Sweep `docs/` for `daemon/internal/`, `cd daemon && go test`, and other
      Go-era paths; fix or annotate as historical
- [ ] Confirm `docs/specs/README.md` mapping and speckit-flow docs describe
      the current (non-speckit-tooling) authoring flow

### Definition Of Done

- No non-archive document or code comment claims the control plane is Go or
  references the removed `daemon/` tree as live code.
- A reviewer can grep `daemon/internal|go test` under `docs/`, `CLAUDE.md`,
  and `crates/` and find only archive files and deliberate historical notes.

### Explicitly Out Of Scope

- Any behavior change; this roadmap is documentation and comments only.

---

## Roadmap 74: Deferred Hook Wiring Closure

Status: `[ ] planned`

### Goal

Replace the remaining permissive in-manager defaults with real implementations
or explicit recorded risk acceptances, so defense-in-depth gates actually
defend before release evidence is gathered on top of them.

### Context

Route-level `protected()` middleware remains the authoritative authorization
boundary; the in-manager hooks below are defense-in-depth and currently
permissive in the production assembly (`crates/surface/app/src/lib.rs`):

- `dope_webhook::Manager` is constructed with `quota: None`, falling back to
  `AllowAllQuota` (`crates/domains/webhook/src/lib.rs:195`). The webhook
  ingress `/v1/triggers/webhook/` is signature-authed rather than
  `protected()`, which makes an unbounded quota gate a real exposure, not
  just defense-in-depth.
- `dope_catalog::Manager` is constructed with `checker: None` (falls back to
  `AllMet`) and `permissions: None` — the sandbox/secret/policy wiring that
  spec 053 explicitly deferred as follow-on.
- Spec 052's deferred non-webhook inbound trigger sources remain unbuilt
  (decide: implement or formally re-scope).

### Tasks

- [ ] Implement a real webhook `QuotaGate` backed by the billing/quota plane
      (Roadmap 38 surfaces), with per-tenant bounds and a deny event
- [ ] Implement the catalog `RequirementChecker` against the sandbox
      requirement contract (Roadmap 17) and `PermissionGate` against the
      policy plane
- [ ] Inventory every remaining `Option<Box<dyn ...>>` hook seam across
      `crates/domains/*` whose `None` fallback is permissive; for each, wire
      a real implementation or record an accepted-risk decision in this
      document
- [ ] Decide and record the fate of spec 052's non-webhook trigger sources
- [ ] Behavioral tests for each newly wired gate (allow, deny, restart
      persistence where applicable)

### Definition Of Done

- No production assembly path constructs a manager with a permissive default
  hook unless an accepted-risk note in this document says so and why.
- The webhook ingress denies over-quota deliveries and emits an auditable
  event for the denial.

### Explicitly Out Of Scope

- New product surfaces; this roadmap only closes seams the shipped specs
  already declared as follow-ons.

---

## Roadmap 75: Rust-Era Release Evidence Re-Run

Status: `[ ] planned` — depends on Roadmap 74 (soak must cover final wiring)

### Goal

Re-establish the entire release evidence base on the Rust daemon and close
the R37/39/42/43 evidence debt register.

### Context

The production operations baseline (install, upgrade, backup, restore,
rollback, the reusable 24-hour soak harness, fault drills, real-account smoke
policy, resource-growth checks) is implemented; operator entry point
[`production-operations.md`](production-operations.md). All recorded evidence
is Go-era and therefore void for the Rust binary.

### Tasks

- [ ] Verify the soak harness, fault drills, and ops tooling run unmodified
      against the Rust daemon; port any Go-specific probes
- [ ] 24-hour test-environment soak of the Rust daemon with fake-backend
      fault drills and resource-growth checks (closes R39 evidence)
- [ ] Full-duration hosted daemon soak on a stable host (closes R43
      `hosted_soak_pending`)
- [ ] Hosted secrets / connector isolation soak evidence (closes R37)
- [ ] Integration health and permission diagnostics evidence on a stable host
      with real accounts (closes R42)
- [ ] Record every run through
      [`release-truth-checklist.md`](release-truth-checklist.md) with commit
      hash, host, dates, and artifacts linked from this document

### Definition Of Done

- The Release Evidence Debt Register above shrinks to only the Roadmap 76
  launch-gate row, each closed row linking Rust-era evidence.
- Every recorded run identifies the exact Rust commit it certifies.

### Explicitly Out Of Scope

- The public launch-gate run itself (Roadmap 76).

---

## Roadmap 76: Public Launch Gate Execution

Status: `[ ] planned` — depends on Roadmaps 74 and 75

### Goal

Execute the launch gate that Roadmap 72 codified: assemble the real evidence
index, run the validator, and record a ship / no-ship decision for public
release.

### Context

`ValidateLaunchGate` and `POST /v1/release/launch-gate` exist and enforce:
required workload coverage, at least 3 channels, real calendar and mail
providers, soak, support-bundle, and redaction evidence
(see [`release-readiness.md`](release-readiness.md) and
`specs/057-public-release-soak-and-launch-gate/`). The gate is a no-ship
gate; missing evidence must fail it, not be waived.

### Tasks

- [ ] Operator-run real-account workloads across at least 3 channels
      (Discord, Telegram, Slack; Matrix as available) on the Rust daemon
- [ ] Real-account Feishu/Lark calendar and mail workloads, including
      attachment transfer and inbox triage paths
- [ ] Support evidence bundle generation and redaction verification on the
      release candidate
- [ ] Assemble `LaunchGateEvidence` from Roadmap 75 artifacts plus the runs
      above; execute `POST /v1/release/launch-gate`
- [ ] Record the ship / no-ship decision, the evidence index location, and —
      on no-ship — the enumerated blockers as new roadmap tasks

### Definition Of Done

- A dated launch-gate decision for a named Rust commit is recorded in this
  document and in the release evidence index.
- On ship: the Roadmap 77 entry gate is formally open. On no-ship: every
  blocker is a tracked task with an owner roadmap.

---

## Roadmap 77+: Context, Knowledge, And Memory Program

Status: `[ ] gated` — entry gate is the Roadmap 76 ship decision. Until the
gate opens, only design work (spec authoring) is authorized; no
implementation.

### Goal

Deliver the product-thesis pillars that the entire non-knowledge program
deliberately excluded: layered memory, knowledge retrieval, context
engineering, agent-managed skills, and audited self-improvement
(see [`../product/product-outline.md`](../product/product-outline.md)).

### Binding Constraints (from the product outline)

- Memory writes must be attributable, scoped, and reversible.
- Recalled memory is evidence, not truth; important decisions must link back
  to source.
- No self-modifying agent behavior without review and audit.
- No general-purpose knowledge graph as the primary storage model.

### Program Shape (to be cut into roadmaps at design time)

Author upstream specs continuing the `docs/specs/` sequence at `058`:

1. **Memory plane foundation** — memory types, data model, write policy,
   retention, attribution, reversal (recommended first slice, spec 058)
2. **Context engineering** — context assembly over threads/continuity/
   workspace bindings (R54-58 surfaces are the substrate)
3. **Knowledge retrieval** — retrieval over the memory plane with
   source-linking
4. **Agent-managed skills** — extending the operator catalog (R68) with
   agent-proposed, operator-approved skill lifecycle
5. **Self-improvement design** — review- and audit-gated behavior change

Each spec follows the existing `docs/specs/` authoring standard and the
roadmap-per-slice delivery unit. Numbering continues from Roadmap 77.

---

## Deferred, Non-Gating Work

Carried follow-ons that do not block the launch gate; schedule after
Roadmap 76 unless a launch blocker pulls them forward:

- **Sandbox backend expansion** (from Roadmap 20): additional consumer
  families onto sandbox execution, stronger backends beyond `docker`,
  VM-grade isolation, remote execution control planes.
- **Client-surface polish** beyond the operator-shell scope shipped in R70
  (the Rust TUI and web shell evolve by product need, not by this program).

## Relationship To Other Planning Artifacts

- Git history of this file (pre-2026-08-16) — frozen execution record for
  Roadmaps 1-72.
- [`daemon-tasks.md`](daemon-tasks.md) — task registry for the roadmaps in
  this document (the R1-72 registry lives in that file's git history).
- [`../specs/README.md`](../specs/README.md) — upstream spec index and
  spec→roadmap mapping; continues at 058 for the knowledge program.
- [`release-truth-checklist.md`](release-truth-checklist.md) — mandatory for
  every evidence claim in Roadmaps 75 and 76.
- `specs/<NNN>-.../` — per-slice working areas once implementation begins.
