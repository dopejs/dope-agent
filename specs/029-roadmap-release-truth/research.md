# Research: Roadmap Authority And Release Truth Reconciliation

## Decision: Use `docs/runtime/release-truth-checklist.md` For The Standalone Checklist

**Rationale**: `docs/runtime/release-readiness.md` already owns the release gate and
references Roadmap 42 diagnostic evidence and Roadmap 43 hosted evidence. Placing the
reusable checklist under `docs/runtime/` keeps it next to the release-readiness authority
while allowing roadmap, harness, and upstream spec materials to link to one stable
reviewer artifact.

**Alternatives considered**:

- `docs/harness/release-truth-checklist.md`: rejected because the checklist governs
  release readiness and roadmap closure, not only harness execution.
- Branch-local checklist only: rejected because Roadmap 44 requires a reusable artifact
  for future closure reviews.
- Inline checklist in `daemon-roadmaps.md`: rejected because it would be harder to reuse
  and easier to drift from release-readiness guidance.

## Decision: Treat Roadmap 42 As Implementation And Local Verification Complete

**Rationale**: The clarified spec states Roadmap 42 implementation and local verification
are complete, while stable-host or real-account release evidence remains pending. The
plan must therefore reconcile status wording around release evidence gaps rather than
generate tasks that reopen implementation scope.

**Alternatives considered**:

- Leave Roadmap 42 verification unspecified: rejected because it would preserve the
  ambiguity Roadmap 44 exists to remove.
- Treat Roadmap 42 as an implementation gap: rejected because it conflicts with the
  clarified feature scope and would create downstream rework.

## Decision: Preserve The Roadmap 43 Dry-Run Versus Full-Soak Distinction

**Rationale**: Roadmap 43 branch-local quickstart records local and stable-host dry-run
evidence, while explicitly calling out that a full Dope daemon 24-hour hosted release
soak remains pending. Reconciliation must keep that residual release gap visible and must
not classify dry-run evidence as public readiness.

**Alternatives considered**:

- Mark Roadmap 43 fully release-complete: rejected because full-duration hosted daemon
  soak evidence remains pending.
- Mark Roadmap 43 incomplete generally: rejected because it would hide the completed
  local implementation and dry-run evidence.

## Decision: Keep Verification Documentation-Only Unless A Validator Changes

**Rationale**: Roadmap 44 changes planning truth, status wording, and checklist links.
No daemon behavior, API surface, schema, script, or generated evidence format is planned.
Therefore verification should be text search, Markdown/link review, and checklist
application. Code tests become required only if implementation changes validators,
scripts, schemas, or generated artifacts.

**Alternatives considered**:

- Always run full daemon and client tests: rejected as unnecessary for documentation-only
  work and not tied to the changed surface.
- Skip verification entirely: rejected because status and link drift are the failure
  modes for this roadmap.

## Decision: Use Evidence Gap Labels Instead Of Binary Done/Pending Labels

**Rationale**: Roadmap 44's core value is distinguishing implementation completion,
local verification, stable-host dry-run evidence, full hosted soak, real-account smoke,
credential or approval blockers, and public-readiness eligibility. Binary labels would
recreate the planning ambiguity this phase is closing.

**Alternatives considered**:

- Use only `[x]` and `[ ]` roadmap markers: rejected because they cannot represent
  release-evidence residuals accurately.
- Create per-roadmap custom labels: rejected because future specs need a reusable
  vocabulary and checklist.
