# Contract: Roadmap Evidence Linkage

## Goal

Make evidence traceable from roadmap summaries and planning materials without requiring
chat history or status archaeology.

## Required Link Targets

For implemented roadmaps after Roadmap 39 that are referenced by Roadmap 44, link the
available evidence classes:

- upstream spec in `docs/specs/`,
- branch-local spec and plan in `specs/`,
- quickstart or runbook verification notes,
- release-readiness or hosted evidence index when available,
- explicit skip, blocked, or pending evidence note when release evidence is unavailable.

## Roadmap 42 Link Requirements

Roadmap 42 materials must link or reference:

- `docs/specs/027-integration-health-and-permission-diagnostics.md`,
- `specs/027-integration-diagnostics/`,
- `specs/027-integration-diagnostics/quickstart.md`,
- release-readiness evidence or explicit pending/skipped real-account smoke status.

The wording must distinguish implementation/local verification completion from
stable-host or real-account release evidence still pending.

## Roadmap 43 Link Requirements

Roadmap 43 materials must link or reference:

- `docs/specs/028-hosted-operational-profile-and-recovery.md`,
- `specs/028-hosted-operational-profile/`,
- `specs/028-hosted-operational-profile/quickstart.md`,
- `docs/runtime/hosted-operational-profile.md`,
- `docs/harness/hosted-operational-profile.md`,
- `scripts/production/hosted-profile.sh` where implementation artifacts are named.

The wording must distinguish local implementation, stable-host dry-run evidence, and
pending full-duration hosted daemon release soak.

## Roadmap 44 Link Requirements

Roadmap 44 materials must link or reference:

- `docs/specs/029-roadmap-authority-and-release-truth-reconciliation.md`,
- `specs/029-roadmap-release-truth/`,
- the standalone `docs/runtime/release-truth-checklist.md` once implemented.

Roadmap 44 must remain a documentation and release-truth reconciliation slice, not a
runtime behavior change.

## Residual Gap Labels

Allowed residual blocker classes:

- `implementation_missing`
- `verification_missing`
- `stable_host_dry_run_pending`
- `hosted_soak_pending`
- `real_account_credentials_unavailable`
- `tenant_approval_unavailable`
- `operator_deferred`
- `evidence_stale`

## Verification

Required checks:

- Follow every newly added or updated Markdown link.
- Confirm referenced evidence exists or the residual gap is explicit.
- Confirm no historical evidence is rewritten or silently superseded.
- Confirm link text describes dry-run, full soak, real-account smoke, or skip status
  accurately.
