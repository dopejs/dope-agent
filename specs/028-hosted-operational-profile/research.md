# Phase 0 Research: Hosted Operational Profile And Recovery

## Decision: Extend Roadmap 39 Operations Instead Of Creating A New Ops Stack

**Decision**: Reuse `docs/runtime/production-operations.md`,
`production-install.md`, `production-upgrade.md`, `backup-restore.md`,
`release-readiness.md`, `docs/harness/production-soak.md`, and
`scripts/production/*` as the baseline for Roadmap 43.

**Rationale**: Roadmap 43 is a hosted/test-host operational profile, not a new product
domain. Reusing the Roadmap 39 baseline preserves compatibility, reduces operator drift,
and keeps rollout reversible.

**Alternatives considered**:

- Replace Roadmap 39 scripts with a new hosted-only toolchain. Rejected because it would
  split operator behavior and make rollback harder.
- Build a separate daemon service manager first. Rejected because the spec clarifies that
  the first profile uses a repo-owned foreground supervisor.

## Decision: Repo-Owned Foreground Supervisor First

**Decision**: The first hosted profile uses a repo-owned foreground supervisor wrapping
existing daemon run and status workflows.

**Rationale**: Existing `make daemon-run-test` and `make daemon-test-status` workflows
are already the local operational baseline. A repo-owned foreground supervisor makes
start/stop/restart/status/health evidence portable across a stable test host or VPS
without requiring `systemd`, `launchd`, Kubernetes, or cloud-specific services.

**Alternatives considered**:

- Linux service-manager profile first. Rejected because it would narrow the initial test
  host target and introduce host-specific setup before the evidence model is stable.
- macOS service-manager profile first. Rejected because developer laptops are not
  acceptable release-readiness hosts.
- Contract-only supervision. Rejected because release evidence needs concrete generated
  process and recovery records.

## Decision: File-Based Evidence With Stable Run Identity

**Decision**: Hosted operational evidence remains file-based in stable artifact paths,
with every run identified by commit or version, profile, host, operator, start time, and
run id.

**Rationale**: Current production helpers already write operator-visible evidence and
fixtures. File-based evidence is easy to archive, inspect, redact-test, and link from a
release index without adding a persistence/API migration unless implementation later
proves it is necessary.

**Alternatives considered**:

- Add new daemon database tables for all hosted evidence. Rejected for initial scope
  because it increases migration and rollback risk without being required by the spec.
- Store only narrative runbook notes. Rejected because release review must be structured
  and artifact-backed.

## Decision: Release Evidence Must Match Commit, Profile, And Run Identity

**Decision**: Release evidence is valid only when it matches the reviewed commit or
version, hosted profile, and run identity.

**Rationale**: This prevents old, cross-profile, or partial artifacts from being reused
for ship/no-ship decisions. It also makes the evidence index a concrete review artifact
instead of a loose checklist.

**Alternatives considered**:

- Reuse evidence generated within a fixed time window. Rejected because code/config drift
  can happen inside the window.
- Reviewer discretion. Rejected because it weakens release gate consistency.

## Decision: 90-Day Default Evidence Retention

**Decision**: Hosted operational evidence uses a 90-day default retention period unless
an authorized policy requires longer.

**Rationale**: Roadmap 42 diagnostic evidence already uses a 90-day default. Aligning
hosted operational evidence retention with diagnostic evidence reduces policy drift and
keeps normal inspection bounded.

**Alternatives considered**:

- 30-day retention. Rejected because hosted release and recovery investigations may need
  more history.
- Latest-N retention. Rejected because high-frequency test runs could evict important
  release evidence too early.
- Indefinite retention. Rejected because it creates unbounded artifact growth and secret
  exposure risk if redaction ever fails.

## Decision: Three-Tenant Recovery Fixture Reuse

**Decision**: Hosted backup/restore rehearsal must validate at least three tenants with
distinct credential, quota, and work states.

**Rationale**: Roadmap 39 already establishes this as the representative recovery
baseline. Hosted recovery should prove the same tenant isolation, credential remediation,
quota, migration, and work-state invariants.

**Alternatives considered**:

- One-tenant recovery proof. Rejected because it cannot prove tenant isolation.
- Two-tenant proof. Rejected because it is weaker than the established Roadmap 39
  baseline and does not materially reduce implementation complexity.

## Decision: Stable-Host Smoke Is Required For Readiness

**Decision**: Release-readiness evidence must include a manual smoke on a stable
always-on test host or VPS. Developer laptops are acceptable only for targeted validation.

**Rationale**: The upstream spec explicitly identifies host sleep, power, and network
behavior as a source of ambiguous failures. A stable host is required to attribute
failures to daemon, host, network, provider, credential, quota, or operator causes.

**Alternatives considered**:

- Laptop validation only. Rejected because it cannot produce defensible long-lived
  hosted readiness evidence.
- Cloud-managed platform first. Rejected as out of scope for the first hosted profile.
