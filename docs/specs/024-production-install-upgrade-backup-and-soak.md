# Production Install, Upgrade, Backup, And Soak

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 39, the production
readiness work required before treating the product as user-deliverable.

Primary source documents:
- `docs/product/hosted-productization-roadmap-split.md`
- `docs/runtime/p0-release-review.md`
- `docs/runtime/daemon-roadmaps.md`

## Background

Feature completeness is not enough for a personal agent product. A credible user-deliverable
system needs installation, upgrade, backup, restore, long-running behavior, real account
connection smoke, failure recovery, and external-service instability validation.

## Goal

Close the operational readiness gap between a functional daemon and a product that can be
installed, upgraded, recovered, and run for long periods with real user accounts.

## Fixed Decisions

- This roadmap validates production operation; it is not a new domain feature.
- Long-running soak must include restart, external-service fault, and recovery scenarios.
- Upgrade and rollback paths must be documented and tested against real migration artifacts.
- Real account smoke tests are required where safe credentials are available, with fake
  backend coverage remaining mandatory.
- Soak completion requires explicit pass/fail thresholds, not only a narrative runbook.
- Release readiness must include resource-growth checks and recovery-time expectations.

## Dependencies On Completed Phases

- Roadmap 33: Evaluation And Replay Harness
- Roadmap 35: Tenant-Scoped Data Migration
- Roadmap 37: Hosted Secrets, Integrations, And Connector Isolation
- Roadmap 38: Billing, Quotas, And Usage Accounting

## In Scope

- install and upgrade runbooks
- migration verification and rollback guidance
- backup and restore workflow
- long-running daemon soak scenario with explicit duration, workload, restart, and fault
  injection requirements
- real account connection smoke for supported integration domains
- external-service fault drills for transient failures, rate limits, auth expiry, and
  provider unavailability
- resource-growth checks for logs, database size, goroutine count, memory, and file
  descriptors where observable
- operator-visible recovery and diagnostic expectations

## Out Of Scope

- new integration domains
- memory or self-improvement
- payment-provider production launch
- enterprise SSO unless separately specified

## Operator Or User Problems To Solve

- Users need an installation and upgrade path that does not depend on developer knowledge.
- Operators need to recover data after corruption, failed migration, or deployment rollback.
- The team needs evidence that the daemon survives realistic long-running conditions.

## User Stories

- As an operator, I can install and upgrade the product using documented steps.
- As an operator, I can restore from backup and verify restored tenant data.
- As a product engineer, I can run a soak scenario that exercises schedules, integrations,
  approvals, delivery, and evaluation under external-service faults.

## Functional Requirements

- The repository MUST include production install and upgrade runbooks.
- Backup and restore MUST be documented and tested.
- Migration verification MUST include preflight and postflight checks.
- Soak MUST run for a documented minimum duration; the first production-readiness baseline
  MUST include at least a 24-hour `DOPE_ENV=test` soak unless the roadmap explicitly
  records a shorter temporary threshold and why it is acceptable.
- Soak MUST exercise long-running runtime, scheduler, integration, delivery, approval,
  quota, tenant switching, and evaluation behavior.
- Soak MUST include at least three daemon restarts and prove unfinished work moves to a
  defined recovered, interrupted, retried, or operator-action-needed state.
- External-service fault drills MUST classify recovery, retry, and operator-action-needed
  states.
- Fault drills MUST include transient 5xx, rate limit, auth expiry, provider unavailable,
  slow response, and malformed response cases for fake backends, plus opt-in real-account
  smoke where credentials are available.
- The soak report MUST include pass/fail thresholds for unclassified failures, recovery
  time, retry exhaustion, queue backlog, resource growth, and cross-tenant leakage.

## Compatibility And Operational Notes

- Soak runs must not touch production user data by default.
- Real account smoke must be opt-in and clearly separate from fake-backend CI.
- Rollback guidance must state when restore-from-backup is the only safe path.

## Verification Expectations

- Automated backup/restore regression where practical.
- Long-running `DOPE_ENV=test` soak script or documented test harness.
- Soak report fixture or generated artifact recording duration, workload, restarts, injected
  faults, recovery times, resource-growth observations, and pass/fail result.
- Real-account smoke checklist for enabled providers.
- Release-verification checklist covering install, upgrade, rollback, and recovery.
- Regression proving backup, restore, and migration verification can run on a tenant-scoped
  data set with multiple tenants.

## Definition Of Done

- The product has documented and verified operational paths for install, upgrade, backup,
  restore, soak, and external-service recovery.
- The soak result is measurable and rejects unbounded resource growth, unclassified
  failures, failed recovery, or cross-tenant leakage.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/024-production-install-upgrade-backup-and-soak.md 完成 phase 39 的工作`
