# Release Truth Checklist

Use this checklist when closing a roadmap, reviewing a release-readiness claim, or
reconciling roadmap status with branch-local evidence. It is reusable across Roadmap 44
and later roadmap closure reviews.

Reviewer outcomes:

- `pass`: the reviewed claim is evidence-backed and no no-ship condition applies.
- `residual_work`: implementation may be complete, but required public-readiness evidence
  remains pending, skipped, blocked, stale, or deferred.
- `no_ship`: a public-readiness or ship-readiness claim is unsupported or contradicted by
  evidence.

## Status Vocabulary

Use these labels consistently in roadmap, harness, upstream spec, quickstart, and release
readiness material:

- `proposed`: scope is planned but implementation has not started.
- `implementation complete`: required implementation work is complete, but release
  evidence may still be pending.
- `local verification complete`: targeted local checks, contract checks, or quickstart
  verification passed for the implemented scope.
- `stable-host dry-run complete`: a stable always-on host or VPS ran a dry-run or
  controlled-host smoke; this is not a full hosted release soak.
- `full hosted soak pending`: full-duration hosted daemon soak evidence is still
  required before public readiness.
- `real-account smoke pending`: safe real-account smoke evidence is still required, or
  credentials and approvals must be explicitly skipped or blocked.
- `release evidence gap`: remaining release evidence work that does not imply missing
  implementation unless `implementation_missing` is also recorded.
- `public readiness`: release claim allowed only when current linked release evidence
  satisfies this checklist.

Do not use checkbox-only, `done`, or `pending` wording without the evidence state it
represents.

## Evidence Link Review

For each reviewed roadmap, follow links to:

- upstream spec in `docs/specs/`,
- branch-local spec, plan, tasks, or quickstart in `specs/`,
- runbook, smoke, soak, release index, or acceptance evidence where available,
- explicit skipped, blocked, stale, or pending evidence notes when release evidence is
  unavailable.

Historical evidence may be linked and classified, but must not be rewritten or presented
as current public-readiness evidence unless the reviewed claim allows that scope.

## Implementation And Local Verification

Record whether implementation and local verification are:

- complete,
- incomplete,
- blocked,
- not applicable,
- unknown and requiring follow-up.

Implementation completion alone is not public readiness.

## Stable-Host Dry-Run

A stable-host dry-run may support local operational confidence, but it is not a
full-duration hosted daemon release soak. Record it separately from hosted soak evidence.

## Full Hosted Soak

Full hosted soak evidence must identify the reviewed commit or version, hosted profile,
run identity, host class, duration, and artifact links. Missing, stale, mismatched,
failed, expired, or secret-exposing hosted evidence is a no-ship condition for public
readiness.

## Real-Account Smoke

Real-account smoke must record pass, fail, blocked, skipped, limited, or unsupported
states for supported domains. Missing safe credentials do not imply missing
implementation when fake-backend coverage passes and the skip or blocked reason is
explicit.

Risky smoke probes require both tenant administrator approval and authorized operator
approval. Evidence must not expose raw secrets, OAuth tokens, refresh tokens, app secrets,
authorization headers, or credential-bearing provider payloads.

## Residual Blocker Classes

Use these blocker classes when evidence remains:

- `implementation_missing`
- `verification_missing`
- `stable_host_dry_run_pending`
- `hosted_soak_pending`
- `real_account_credentials_unavailable`
- `tenant_approval_unavailable`
- `operator_deferred`
- `evidence_stale`

Stable-host dry-run pending, hosted soak pending, and real-account credentials
unavailable are release evidence gaps unless implementation is explicitly missing.

## Public-Readiness Eligibility

A roadmap may be marked public-ready only when:

- implementation and local verification status are clear,
- all required evidence links are current and reachable,
- residual gaps are either closed or not required for the reviewed readiness claim,
- no no-ship condition applies.

## No-Ship Conditions

Classify the reviewed claim as `no_ship` when:

- public readiness is claimed without linked release evidence,
- required evidence is missing, stale, mismatched, failed, expired, or secret-exposing,
- stable-host dry-run evidence is presented as full hosted soak evidence,
- real-account smoke is absent without an explicit skip, blocked, or pending reason,
- implementation status and release evidence status contradict each other,
- reviewed evidence cannot be reached from roadmap or spec materials.

## Verification Steps

Recommended repository-local checks:

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

```sh
rg -n "50 tasks|fewer than 50 tasks|below 50 tasks|split" \
  docs/runtime/daemon-roadmaps.md \
  docs/harness/harness-architecture.md \
  docs/specs/README.md \
  docs/specs/029-roadmap-authority-and-release-truth-reconciliation.md
```

Manual review is still required: follow every changed evidence link and classify the
review outcome as `pass`, `residual_work`, or `no_ship`.

## Roadmap 42 Application

Roadmap 42 is implementation and local verification complete. It remains `residual_work`
for public readiness until stable-host or real-account release evidence is linked and
current, or until each missing real-account path has an explicit skipped or blocked
reason.

## Roadmap 43 Application

Roadmap 43 is implementation and local verification complete, and stable-host dry-run
evidence has been recorded. It remains `residual_work` for public readiness until
full-duration hosted daemon release soak evidence is linked and current.

## Public-Readiness Examples

- Missing evidence link for a public-readiness claim: `no_ship`.
- Stale evidence from a different commit, profile, or run identity: `no_ship`.
- Stable-host dry-run presented as full hosted soak evidence: `no_ship`.
- Generated evidence includes raw credential material: `no_ship`.
- Safe real-account credentials unavailable with explicit skip reason and fake-backend
  coverage passing: `residual_work` unless the reviewed claim does not require real
  account evidence.
