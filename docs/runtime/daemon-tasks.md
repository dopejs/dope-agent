# Daemon Task Registry — Remaining Work

> **Supersession note (2026-08-16):** this registry replaces the Go-era task
> registry, which is preserved in this file's git history (revisions before
> 2026-08-16). All tasks for Roadmaps 1-72 are closed; that history is the
> audit record.

## Purpose

Audit registry for the remaining daemon roadmaps defined in
[`daemon-roadmaps.md`](daemon-roadmaps.md). It exists to track task status,
make incomplete work visible, and prevent narrow implementations from being
counted as done.

## Status Rules

- `[x]` means the full task definition of done is satisfied
- `[ ]` means the task is not complete
- use a short suffix like `(partial)`, `(in_progress)`, or `(blocked)` when
  needed
- a task stays `[ ] (partial)` if any required API, persistence, recovery,
  eventing, contract, or testing boundary is still open
- task progress is informational only; a roadmap is still incomplete until
  every in-scope task is fully closed

Verification baseline for all tasks: `cargo test --workspace` and
`make daemon-contract-test` green at the closing commit.

## Roadmap 73: Documentation And Execution-Record Truth Closure

- [x] Replace roadmap and task documents with current-state versions; retire
      the R1-72 documents and superseded roadmap-split docs to git history
- [x] Rewrite `CLAUDE.md` daemon section for the Rust workspace
- [x] Audit and clean stale `TODO: PLACEHOLDER` / `TODO: MISSING` markers in
      `crates/surface/api/src/`
- [x] Sweep `docs/` for live references to the removed Go tree
- [x] Confirm `docs/specs/README.md` and spec-flow docs match the current
      authoring flow

## Roadmap 74: Deferred Hook Wiring Closure

- [ ] Real webhook `QuotaGate` backed by the billing/quota plane, with deny
      event
- [ ] Catalog `RequirementChecker` wired to the sandbox requirement contract
- [ ] Catalog `PermissionGate` wired to the policy plane
- [ ] Inventory of permissive `Option` hook seams across `crates/domains/*`
      with wire-or-accepted-risk outcome for each
- [ ] Decision recorded for spec 052 non-webhook trigger sources
- [ ] Behavioral tests for each newly wired gate

## Roadmap 75: Rust-Era Release Evidence Re-Run

- [ ] Soak harness and ops tooling verified against the Rust daemon
- [ ] 24-hour test-environment soak with fault drills and resource-growth
      checks (R39 evidence)
- [ ] Full-duration hosted daemon soak on a stable host (R43 evidence)
- [ ] Hosted secrets / connector isolation soak (R37 evidence)
- [ ] Diagnostics stable-host / real-account evidence (R42 evidence)
- [ ] All runs recorded via `release-truth-checklist.md` with commit, host,
      dates, artifacts

## Roadmap 76: Public Launch Gate Execution

- [ ] Real-account workloads across at least 3 channels on the Rust daemon
- [ ] Real-account Feishu/Lark calendar and mail workloads including
      attachments and triage
- [ ] Support evidence bundle generation and redaction verification on the
      release candidate
- [ ] `LaunchGateEvidence` assembled; `POST /v1/release/launch-gate` executed
- [ ] Ship / no-ship decision recorded with evidence index location

## Roadmap 77+: Context, Knowledge, And Memory Program (gated)

Design-only until the Roadmap 76 gate opens:

- [ ] Spec 058: memory plane foundation (types, write policy, retention,
      attribution, reversal)
- [ ] Specs 059+: context engineering, knowledge retrieval, agent-managed
      skills, self-improvement — authored per the `docs/specs/` standard and
      cut into roadmaps

## Working Rule

When work completes:

- update the task in this registry
- update the roadmap status in [`daemon-roadmaps.md`](daemon-roadmaps.md) if
  the task affects roadmap closure
- do not mark a task or roadmap complete until its full boundary is actually
  closed
