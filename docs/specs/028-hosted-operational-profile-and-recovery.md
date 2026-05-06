# Hosted Operational Profile And Recovery

Status: implementation and local verification complete in
`specs/028-hosted-operational-profile/`; stable-host dry-run evidence has been recorded
on `zentalk-1`. Full release readiness still requires an operator-run full-duration
hosted daemon soak on an always-on test host or VPS.

Authority: This document is the authoritative upstream spec for Roadmap 43, the hosted
test-host and operational recovery profile required before long-lived personal-agent
operation can be treated as stable.

Primary source documents:
- `docs/product/hosted-productization-roadmap-split.md`
- `docs/specs/024-production-install-upgrade-backup-and-soak.md`
- `docs/runtime/production-operations.md`
- `docs/runtime/production-install.md`
- `docs/runtime/production-upgrade.md`
- `docs/runtime/backup-restore.md`
- `docs/runtime/release-readiness.md`
- `docs/runtime/release-truth-checklist.md`
- `docs/harness/production-soak.md`
- `docs/runtime/hosted-operational-profile.md`
- `docs/harness/hosted-operational-profile.md`
- `specs/028-hosted-operational-profile/`

## Background

Roadmap 39 created production operations runbooks, helper scripts, backup/restore
coverage, and a reusable soak harness. The remaining hosted operational gap is that
long-running test-host operation still depends on operator convention: ad hoc directories,
hand-built SSH commands, manually tracked process trees, and evidence scattered across
logs and chat.

A stable personal agent needs a named operational profile: where code, data, logs,
artifacts, backups, and reports live; how the daemon is supervised; how upgrades and
rollback are rehearsed; and how a reviewer can finish release inspection in a bounded
amount of time.

## Goal

Define and verify a hosted/test-host operational profile for deployment, process
supervision, backup, restore, upgrade, rollback, observability, and release evidence
collection.

## Fixed Decisions

- This roadmap productizes hosted operation; it does not add new personal-agent domains.
- The first profile targets a stable always-on test host or VPS before broader
  infrastructure platforms.
- Operational evidence must be structured and artifact-backed, not only narrative
  runbook text.
- Release review must remain completable in 30 minutes or less using generated evidence.
- Backup and rollback rehearsal must run against tenant-scoped data and preserve audit
  truth.
- The profile must not require Kubernetes, cloud-specific managed services, or payment
  production launch.

## Dependencies On Completed Phases

- Roadmap 34: Tenant Identity And Access Foundation
- Roadmap 35: Tenant-Scoped Data Migration
- Roadmap 37: Hosted Secrets, Integrations, And Connector Isolation
- Roadmap 38: Billing, Quotas, And Usage Accounting
- Roadmap 39: Production Install, Upgrade, Backup, And Soak
- Roadmap 40: Live Validation And Side-Effect Replay
- Roadmap 41: Evaluation Product Expansion
- Roadmap 42: Integration Health And Permission Diagnostics

## In Scope

- hosted/test-host directory layout for code, data, logs, artifacts, backups, and reports
- environment variable and config profile for hosted `DOPE_ENV=test` and production-like
  operation
- process supervision contract for daemon start, stop, restart, health check, and failure
  recovery
- structured deployment manifest recording commit, config, data directory, artifact
  directory, supervisor, and operator
- upgrade preflight and postflight evidence collection
- backup and restore rehearsal to an alternate directory or instance
- rollback decision record, including when restore-from-backup is required
- operational observability baseline for daemon health, database size, log size, memory,
  goroutines, file descriptors, queue or backlog, connector health, MCP health, and
  integration diagnostic state
- release evidence index generation linking all required artifacts
- runbook updates for hosted/test-host operation

## Out Of Scope

- Kubernetes or container-orchestration platform support
- multi-region deployment
- external managed secret-manager integration
- payment-provider production launch
- enterprise SSO
- mobile device fleet management
- memory or context engineering

## Operator Or User Problems To Solve

- Operators need to deploy and restart a long-running daemon without reconstructing
  commands from chat history.
- Release reviewers need one artifact index that links commit, config, logs, soak report,
  backup, restore, upgrade, rollback, and diagnostic evidence.
- Engineers need to know whether a failed soak or smoke came from daemon logic, host
  sleep/power/network behavior, missing credentials, or provider instability.
- Operators need a rehearsed rollback path when upgrade or migration verification fails.

## User Stories

- As an operator, I can provision a hosted/test-host profile with fixed paths and start the
  daemon through a documented supervisor.
- As a release reviewer, I can inspect a release evidence bundle and decide ship/no-ship
  in 30 minutes or less.
- As an operator, I can restore a backup to an alternate directory and verify tenant data,
  credential remediation state, quota state, and daemon health.
- As an engineer, I can compare resource observations across a soak run and identify
  monotonic growth or missing telemetry.

## Functional Requirements

- The repository MUST define a hosted/test-host directory layout for app code, daemon data,
  logs, artifacts, backups, reports, and temporary work.
- The system MUST provide a standard deployment manifest or report containing commit,
  branch or version, host, operator, start time, config profile, data directory, artifact
  directory, and supervisor mode.
- The operational profile MUST define start, stop, restart, status, and health-check
  commands.
- The process supervisor contract MUST define expected behavior after daemon crash,
  host reboot, manual stop, and failed restart.
- Upgrade preflight and postflight MUST write structured evidence.
- Backup and restore rehearsal MUST verify restored tenant data, migration state,
  credential remediation state, quota state, and daemon health.
- Rollback guidance MUST produce a decision record explaining whether in-place rollback or
  restore-from-backup is required.
- Observability collection MUST include daemon health, database size, log size, memory,
  goroutines, file descriptors when available, queue/backlog, connector health, MCP
  health, and integration diagnostic state.
- Release evidence MUST be linkable from a single index.
- The release checklist MUST identify missing required evidence and failed thresholds as
  no-ship conditions.

## Compatibility And Operational Notes

- Existing Roadmap 39 scripts and runbooks should be reused rather than replaced when
  their behavior is sufficient.
- The first implementation may support one supervisor mode if the mode is explicit,
  restart-safe, and documented.
- Hosted/test-host defaults must not touch production user data unless the operator
  explicitly chooses a production profile.
- Evidence paths should be stable across reruns and include commit or run identifiers to
  avoid overwriting prior evidence.
- Operational reports must avoid raw secrets and credential-bearing environment dumps.
- Release-truth classification treats stable-host dry-run evidence separately from the
  pending full-duration hosted daemon soak.

## Verification Expectations

- Script or command tests for deployment manifest generation and evidence path layout.
- Supervisor contract tests or controlled smoke proving start, stop, restart, crash
  recovery, and health check behavior.
- Backup/restore rehearsal against tenant-scoped data with verification output.
- Upgrade preflight/postflight rehearsal with structured evidence.
- Observability report fixture proving all required fields are present or explicitly
  unsupported for the host.
- Release evidence index fixture linking install, upgrade, backup, restore, rollback,
  soak, integration diagnostics, logs, and resource observations.
- Manual smoke on a stable test host or VPS demonstrating the profile can run without
  relying on a movable developer laptop.
- Release-truth checklist review must classify the remaining full-duration hosted daemon
  soak as `hosted_soak_pending` until current evidence is linked.

## Implemented Artifacts

- `scripts/production/hosted-profile.sh`
- `daemon/internal/opsreadiness/hosted_*.go`
- `daemon/internal/contracts/hosted_*_test.go`
- `daemon/internal/opsreadiness/testdata/hosted/`
- `docs/runtime/hosted-operational-profile.md`
- `docs/harness/hosted-operational-profile.md`

## Definition Of Done

- A new stable host can be provisioned and operated from documented repo-owned commands
  and runbooks.
- A failed upgrade or failed soak leaves enough structured evidence to identify the
  likely owner: daemon, host, network, provider, credential, quota, or operator action.
- Backup, restore, upgrade, rollback, health, and observability evidence are generated in
  stable locations and linked from a release evidence index.
- Release review can reach a defensible ship/no-ship decision without reading chat
  history.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/028-hosted-operational-profile-and-recovery.md 完成 phase 43 的工作`
