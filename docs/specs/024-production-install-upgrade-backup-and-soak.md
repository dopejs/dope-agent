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

## Dependencies On Completed Phases

- Roadmap 33: Evaluation And Replay Harness
- Roadmap 35: Tenant-Scoped Data Migration
- Roadmap 37: Hosted Secrets, Integrations, And Connector Isolation
- Roadmap 38: Billing, Quotas, And Usage Accounting

## In Scope

- install and upgrade runbooks
- migration verification and rollback guidance
- backup and restore workflow
- long-running daemon soak scenario
- real account connection smoke for supported integration domains
- external-service fault drills for transient failures, rate limits, auth expiry, and
  provider unavailability
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
- Soak MUST exercise long-running runtime, scheduler, integration, delivery, and evaluation
  behavior.
- External-service fault drills MUST classify recovery, retry, and operator-action-needed
  states.

## Compatibility And Operational Notes

- Soak runs must not touch production user data by default.
- Real account smoke must be opt-in and clearly separate from fake-backend CI.
- Rollback guidance must state when restore-from-backup is the only safe path.

## Verification Expectations

- Automated backup/restore regression where practical.
- Long-running `DOPE_ENV=test` soak script or documented test harness.
- Real-account smoke checklist for enabled providers.
- Release-verification checklist covering install, upgrade, rollback, and recovery.

## Definition Of Done

- The product has documented and verified operational paths for install, upgrade, backup,
  restore, soak, and external-service recovery.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/024-production-install-upgrade-backup-and-soak.md 完成 phase 39 的工作`
