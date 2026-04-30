# Implementation Plan: Hosted Operational Profile And Recovery

**Branch**: `028-hosted-operational-profile` | **Date**: 2026-04-30 | **Spec**: [`spec.md`](./spec.md)
**Input**: Feature specification from `specs/028-hosted-operational-profile/spec.md`

## Summary

Close Roadmap 43 by productizing a hosted/test-host operational profile for long-lived
daemon operation on a stable always-on test host or VPS. The implementation is additive
around the Roadmap 39 production operations baseline: define stable hosted paths, a
repo-owned foreground supervisor, deployment manifests, upgrade preflight/postflight
evidence, backup/restore rehearsal to an alternate target, rollback decision records,
observability reports, 90-day evidence retention, redaction checks, and a single release
evidence index that supports ship/no-ship review in 30 minutes or less.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; Markdown operator docs; Bash
operator scripts under `scripts/production`; JSON evidence artifacts and fixtures for
deployment, supervisor, recovery, observability, and release index contracts.  
**Primary Dependencies**: Existing Roadmap 39 production operations docs and scripts
(`docs/runtime/production-operations.md`, `production-install.md`,
`production-upgrade.md`, `backup-restore.md`, `release-readiness.md`,
`docs/harness/production-soak.md`, `scripts/production/*`), Roadmap 42 integration
diagnostics, daemon health/status behavior, `Makefile` test daemon targets,
`daemon/internal/opsreadiness`, `daemon/internal/store/migrationfixture`,
`daemon/internal/store/tenancy`, `daemon/internal/secrets`, `daemon/internal/billing`,
`daemon/internal/audit`, `daemon/internal/events`, `daemon/internal/contracts`, and
`schemas/` only if evidence is promoted to versioned schema contracts.  
**Storage**: Existing SQLite daemon state remains authoritative. Hosted operational
profile artifacts are file-based evidence under stable run-identity paths. Backup/restore
rehearsal validates at least three tenants with distinct credential, quota, and work
states. Raw credential material is excluded from backups, restored state, logs, reports,
fixtures, events, diagnostics, and release evidence.  
**Testing**: Targeted script/report contract checks for hosted directory layout,
deployment manifest, supervisor start/stop/restart/status/health behavior, crash or
reboot recovery evidence, upgrade preflight/postflight, backup/restore alternate-target
rehearsal, rollback decision records, observability report completeness, release evidence
index completeness/freshness, retention expiry, and redaction. Run `go test ./...` in
`daemon/`, `make daemon-contract-test`, `make daemon-run-test`, `make
daemon-test-status`, `go mod tidy` from `daemon/` after implementation, and a manual
stable-host smoke. Run `pnpm test:clients` and `pnpm build` only if SDK/web/TUI surfaces
change.  
**Target Platform**: Stable always-on test host or VPS using the default test environment
(`DOPE_ENV=test`, `~/.dope-test`, `127.0.0.1:19192`) and a repo-owned foreground
supervisor. Developer laptops are acceptable only for targeted validation, not release
readiness evidence. Host-native service managers, Kubernetes, cloud-specific managed
services, multi-region deployment, and payment production launch are out of scope.  
**Project Type**: Operational readiness and release-evidence change spanning repository
owned docs, scripts, fixtures, daemon test helpers, contract checks, and release
checklists.  
**Performance Goals**: Clean hosted profile provisioning reaches daemon health in <=60
minutes; crash or reboot recovery returns daemon health within <=5 minutes or records
failed recovery evidence; release evidence review completes in <=30 minutes; 100% of
required evidence indexes match the reviewed commit, profile, and run identity.  
**Constraints**: Default validation must not touch `~/.dope`, production user data, live
connectors, or privileged credentials. Live connectors require explicit operator opt-in.
Hosted operational evidence uses 90-day default retention unless an authorized policy
requires longer. Release evidence mismatched to commit, profile, or run identity is a
no-ship condition. The change must reuse Roadmap 39 scripts and runbooks where sufficient
instead of replacing them.  
**Scale/Scope**: One stable single-node hosted/test-host profile operating multiple
tenants. Required recovery evidence covers at least three tenants with distinct
credential, quota, and work states, one active profile, one alternate restore target per
rehearsal, repeated run identities, and bounded evidence retention.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** - PASS. The plan closes Roadmap 43 end-to-end: hosted layout,
  repo-owned foreground supervision, start/stop/restart/status/health workflows, 5-minute
  crash/reboot recovery evidence, deployment manifest, upgrade evidence, backup/restore
  rehearsal, rollback decision, observability, retention, redaction, stable-host smoke,
  and release evidence index.
- **Production-grade, minimal, reversible change** - PASS. The change is additive around
  existing Roadmap 39 operations. Rollback returns to existing production runbooks and
  disables hosted-profile evidence generation while preserving already-written evidence
  for authorized review until retention expiry.
- **Contracts and auditability** - PASS. Planning contracts define hosted profile
  commands, deployment/supervisor evidence, recovery evidence, observability reports, and
  release evidence index rules. Any promoted schema/API/event changes must update
  schemas, fixtures, docs, and contract tests in the same implementation slice.
- **Verification and observability** - PASS. The plan names report contract tests,
  daemon smoke, stable-host smoke, backup/restore rehearsal, upgrade rehearsal,
  observability completeness, failure-owner classification, retention, and redaction.
- **Environment and secrets** - PASS. Default execution uses `DOPE_ENV=test`,
  `~/.dope-test`, and disabled live connectors. Privileged/live access is explicit, and
  raw credential material is forbidden in all generated evidence.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/028-hosted-operational-profile/
|-- plan.md
|-- research.md
|-- data-model.md
|-- quickstart.md
|-- contracts/
|   |-- hosted-profile-commands.md
|   |-- deployment-supervisor-evidence.md
|   |-- recovery-evidence.md
|   |-- observability-report.md
|   `-- release-evidence-index.md
|-- checklists/
|   `-- requirements.md
`-- tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
scripts/
`-- production/                         # hosted profile wrappers, manifest,
                                        # supervisor, recovery, and evidence index helpers

docs/
|-- runtime/                            # hosted profile, production operations,
|                                       # upgrade, backup/restore, release readiness
`-- harness/                            # hosted soak/stable-host evidence guidance

daemon/
|-- internal/
|   |-- opsreadiness/                   # evidence validation and release gate helpers
|   |-- store/
|   |   |-- migrationfixture/           # three-tenant recovery fixture reuse
|   |   `-- tenancy/                    # tenant isolation checks used by restore
|   |-- secrets/                        # redaction checks and credential remediation state
|   |-- billing/                        # quota state checks
|   |-- audit/ and events/              # evidence/audit linkage if surfaced
|   `-- contracts/                      # report/fixture contract tests
`-- go.mod

schemas/                                # update only if hosted evidence becomes a
                                        # versioned API/event/schema surface

Makefile                                # update only if new hosted profile targets are added

sdk/ts/, web/, tui/                     # update only if hosted evidence is exposed
                                        # through client/operator UI surfaces
```

**Structure Decision**: Keep Roadmap 43 as an operational profile and evidence layer.
Reuse `scripts/production`, `docs/runtime`, `docs/harness`, and existing daemon
readiness/fixture packages. Add new scripts or Make targets only where they reduce
operator error, produce structured evidence, or make the hosted profile repeatable. Avoid
new daemon product domains and avoid host-native service managers in this phase.

## Roadmap 43 Planning Contracts

The implementation plan MUST keep these artifacts complete before `/speckit.tasks`:

- [`contracts/hosted-profile-commands.md`](./contracts/hosted-profile-commands.md) -
  hosted directory layout, default environment, live opt-in rules, and repo-owned
  foreground supervisor command surface.
- [`contracts/deployment-supervisor-evidence.md`](./contracts/deployment-supervisor-evidence.md) -
  deployment manifest fields, supervisor events, 5-minute recovery threshold, and
  start/stop/restart/status/health report rules.
- [`contracts/recovery-evidence.md`](./contracts/recovery-evidence.md) -
  upgrade preflight/postflight, backup/restore alternate-target rehearsal, three-tenant
  verification, and rollback decision record.
- [`contracts/observability-report.md`](./contracts/observability-report.md) -
  daemon health, database size, log size, memory, goroutines, file descriptors, backlog,
  connector health, MCP health, integration diagnostics, unsupported markers, and
  failure-owner classification.
- [`contracts/release-evidence-index.md`](./contracts/release-evidence-index.md) -
  required evidence links, commit/profile/run freshness, no-ship rules, 90-day
  retention, redaction, and 30-minute review requirements.

These artifacts are planning gates. Implementation is incomplete if a hosted profile run
can be treated as release evidence without matching commit/profile/run identity, required
artifact links, retention metadata, redaction checks, restore rehearsal evidence,
supervisor recovery evidence, and explicit no-ship conditions.

## Migration And Rollback Plan

1. Add hosted profile docs and evidence contracts without changing existing local test or
   production operation commands.
2. Add repo-owned foreground supervisor and manifest/report helpers that default to
   `DOPE_ENV=test`, `~/.dope-test`, and disabled live connectors.
3. Add generated evidence paths with run identity and 90-day retention metadata; do not
   remove or rewrite existing Roadmap 39 evidence paths.
4. Reuse existing backup/restore, upgrade, and soak helpers; extend them only where
   required to produce hosted-profile identity, alternate-target restore evidence,
   failure-owner classifications, and release index links.
5. Add release evidence index generation and validation after individual evidence
   contracts are stable.

Rollback disables or removes hosted-profile helper entrypoints and reverts runbooks to
the Roadmap 39 baseline. Already-generated evidence remains readable until retention
expiry. Existing daemon data, production runbooks, backup artifacts, and local test
environment behavior remain compatible.

## Post-Design Constitution Check

- **Roadmap closure** - PASS. `research.md`, `data-model.md`, `quickstart.md`, and the
  five contracts cover the full Roadmap 43 operational profile and explicitly defer
  host-native service managers, Kubernetes, multi-region deployment, managed cloud
  services, payment production launch, enterprise SSO, new personal-agent domains, and
  memory/context engineering.
- **Production-grade, minimal, reversible change** - PASS. Design is staged and additive:
  docs/contracts first, hosted profile command wrappers, manifest/supervisor evidence,
  recovery evidence, observability, release index validation, and stable-host smoke.
  Rollback leaves existing Roadmap 39 operations intact.
- **Contracts and auditability** - PASS. Contracts define command behavior, report
  fields, failure thresholds, no-ship rules, retention, redaction, and evidence identity.
  Any schema/API/event promotion must update schemas, fixtures, docs, and contract tests
  together.
- **Verification and observability** - PASS. Quickstart requires targeted checks, daemon
  tests, contract tests, test daemon smoke, stable-host smoke, backup/restore rehearsal,
  upgrade rehearsal, redaction, retention, and release evidence index review.
- **Environment and secrets** - PASS. Design defaults to test state and fake/safe
  operation, requires explicit live opt-in, and forbids raw credential material in
  backups, restores, logs, manifests, reports, fixtures, events, diagnostics, and release
  evidence.

No post-design violations require justification.

## Complexity Tracking

> Filled only when Constitution Check has unjustified violations. None for this plan.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    |            |                                     |

## Implementation Completion Notes

Roadmap 43 local implementation added the hosted evidence validation package,
contract tests, hosted-profile command wrapper, hosted extensions to production
backup/restore/upgrade/soak scripts, fixtures, and runbook updates.

Verification recorded in `quickstart.md`:

- targeted Go readiness packages passed
- `go test ./...` passed on rerun after one transient SQLite lock in `internal/app`
- `go mod tidy` produced no module diff
- `make daemon-contract-test` passed
- `make daemon-run-test` plus `make daemon-test-status` produced healthy local test daemon evidence
- hosted-profile dry-run validation produced local release index evidence
- stable-host dry-run smoke on `zentalk-1` produced release index and reboot-recovery evidence
- post-review contract tests now cover actual supervisor PID lifecycle, missing
  release evidence as no-ship, hosted backup/restore tenant coverage failure,
  upgrade blocking evidence, hosted soak failure-owner attribution, unsupported
  connector/MCP/diagnostic markers, and Go-level release evidence validation
- `daemon/cmd/hosted-evidence-validate` validates generated release indexes;
  `hosted-profile.sh evidence-index` records validator output next to the index
- stable-host supervisor smoke on `zentalk-1` now starts, reports, and stops a
  controlled long-running process through the hosted supervisor wrapper

Rollback path remains additive: remove `scripts/production/hosted-profile.sh`,
ignore hosted evidence files under run-specific artifact roots, and continue
using the Roadmap 39 production operations baseline.
