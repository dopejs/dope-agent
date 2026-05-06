# Quickstart: Roadmap Authority And Release Truth Reconciliation

Use this quickstart to validate Roadmap 44 documentation and release-truth reconciliation.
All checks are repository-local and documentation-focused.

## 1. Review Planning Inputs

```sh
sed -n '1,220p' specs/029-roadmap-release-truth/spec.md
sed -n '1,260p' specs/029-roadmap-release-truth/plan.md
sed -n '1,220p' specs/029-roadmap-release-truth/research.md
find specs/029-roadmap-release-truth/contracts -maxdepth 1 -type f -print
```

Expected evidence:

- Roadmap 42 is classified as implementation and local verification complete, with
  stable-host or real-account release evidence pending unless stronger evidence is
  linked.
- Roadmap 43 is classified as implementation and local verification complete, stable-host
  dry-run complete where linked, and full-duration hosted daemon release soak pending.
- The standalone release-truth checklist is planned for
  `docs/runtime/release-truth-checklist.md`.

## 2. Check Status Contradictions

Run targeted searches before and after implementation:

```sh
rg -n "Roadmap 42|Roadmap 43|Roadmap 44|implemented locally|implementation complete|local verification|stable-host|full-duration hosted|real-account smoke|public readiness|release evidence" \
  docs/runtime/daemon-roadmaps.md \
  docs/harness/harness-architecture.md \
  docs/specs/README.md \
  docs/specs/027-integration-health-and-permission-diagnostics.md \
  docs/specs/028-hosted-operational-profile-and-recovery.md \
  docs/specs/029-roadmap-authority-and-release-truth-reconciliation.md \
  specs/027-integration-diagnostics/quickstart.md \
  specs/028-hosted-operational-profile/quickstart.md
```

Expected evidence:

- No public-readiness claim lacks linked release evidence.
- No Roadmap 42 wording treats pending real-account or stable-host release evidence as
  missing implementation.
- No Roadmap 43 wording treats stable-host dry-run evidence as a full-duration hosted
  daemon release soak.
- Roadmap 44 is the closed reconciliation slice before Roadmap 45 starts.

## 3. Verify Evidence Links

Manually follow every updated link from:

- `docs/runtime/daemon-roadmaps.md`
- `docs/harness/harness-architecture.md`
- `docs/specs/README.md`
- `docs/runtime/release-readiness.md`
- `docs/runtime/release-truth-checklist.md`
- `docs/specs/029-roadmap-authority-and-release-truth-reconciliation.md`

Expected evidence:

- Roadmap 42 links to upstream spec and branch-local quickstart evidence.
- Roadmap 43 links to upstream spec, branch-local quickstart, hosted runbooks, and hosted
  profile implementation artifacts where named.
- Roadmap 44 links to its upstream spec, branch-local planning artifacts, and standalone
  release-truth checklist.
- Historical evidence is linked and classified, not rewritten.

## 4. Apply The Release-Truth Checklist

Apply `docs/runtime/release-truth-checklist.md` to Roadmaps 42 and 43.

Expected outcomes:

- Roadmap 42 produces `residual_work` unless stable-host or real-account release evidence
  is linked and current.
- Roadmap 43 produces `residual_work` until full-duration hosted daemon release soak
  evidence is linked and current.
- Any public-readiness claim without current linked evidence produces `no_ship`.

## 5. Check Planning Boundary

```sh
rg -n "50 tasks|fewer than 50 tasks|below 50 tasks|split" \
  docs/runtime/daemon-roadmaps.md \
  docs/harness/harness-architecture.md \
  docs/specs/README.md \
  docs/specs/029-roadmap-authority-and-release-truth-reconciliation.md
```

Expected evidence:

- Future standard branch-local specs in the non-knowledge parity program remain below 50
  tasks or require upstream spec splitting before implementation planning.
- The wording appears only where it is authoritative and useful for future planning.

## 6. Code Test Boundary

No Go, client, schema, or contract test is required for documentation-only edits. If
implementation changes scripts, validators, schemas, generated artifacts, daemon code, SDK
code, web, or TUI surfaces, run the relevant repository tests and record the reason in
the implementation summary.

Repository policy requires `go mod tidy` from `daemon/` after completing each spec. For
pure documentation work, record that Go/client/schema tests were not applicable and that
`go mod tidy` produced no module diff.

## 7. Validation Record

Validated on 2026-05-06 for the Roadmap 44 documentation implementation.

- Planning inputs reviewed: `spec.md`, `plan.md`, `research.md`, `data-model.md`,
  contracts, and generated `tasks.md` align on documentation-only release-truth
  reconciliation.
- Status contradiction search passed after normalization: Roadmap 42 is implementation and
  local verification complete with stable-host or real-account release evidence pending;
  Roadmap 43 is implementation and local verification complete with stable-host dry-run
  complete and full-duration hosted daemon release soak pending; Roadmap 44 is
  implementation and local verification complete as the closed reconciliation slice before
  Roadmap 45.
- Evidence-link review passed for changed Roadmap 41, 42, 43, and 44 references. The
  checked Markdown links in `docs/runtime/daemon-roadmaps.md`,
  `docs/harness/harness-architecture.md`, `docs/specs/README.md`,
  `docs/runtime/release-readiness.md`, `docs/runtime/release-truth-checklist.md`, and
  `docs/specs/029-roadmap-authority-and-release-truth-reconciliation.md` resolve.
- Roadmap 41 release-readiness stale closure text was reconciled: release readiness now
  records the accepted Roadmap 39/40/41 24-hour rerun for commit `5ad95ba` and links the
  Roadmap 41 quickstart, T153 task closure, upstream spec status, and daemon roadmap
  closure.
- Release-truth checklist application: Roadmap 42 outcome is `residual_work` until current
  stable-host or real-account release evidence, or explicit safe skip/block reasons, are
  linked. Roadmap 43 outcome is `residual_work` until current full-duration hosted daemon
  release soak evidence is linked. No current public-readiness claim passed without
  linked release evidence.
- Planning-boundary search passed for the new task-budget rule: authoritative wording now
  states that future standard branch-local specs stay below 50 tasks or split oversized
  upstream specs before implementation planning. Broad `split` matches outside that rule
  are pre-existing roadmap/history wording, not task-budget authority.
- Code test boundary: no Go, client, schema, contract, daemon, SDK, web, TUI, script, or
  generated-artifact behavior changed, so behavior tests were not applicable. `go mod tidy`
  was run from `daemon/` per repository policy and produced no `go.mod` or `go.sum` diff.
