# Feature Specification: Production Operations Soak

**Feature Branch**: `024-production-ops-soak`  
**Created**: 2026-04-29  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/024-production-install-upgrade-backup-and-soak.md 完成 phase 39 的工作"

## Clarifications

### Session 2026-04-29

- Q: What credential material should backup and restore cover? → A: Backups include secret metadata and references only; restored integrations require reconnect or revalidation before credential-bearing use.
- Q: What production topology is this phase expected to validate? → A: Tenant-scoped single-node production baseline.
- Q: What tenant coverage must the representative backup/restore data set include? → A: At least three tenants with distinct credential, quota, and work states.
- Q: What pass/fail thresholds should block soak completion? → A: Hard fail on any cross-tenant leakage, unclassified failure, restart recovery over 5 minutes, retry exhaustion without operator-action-needed state, queue backlog persisting over 30 minutes, or monotonic resource growth over the full run.
- Q: Should missing safe real-account smoke credentials block release readiness? → A: Missing safe real-account credentials do not block if fake-backend coverage passes and the skip is explicitly recorded.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Install and upgrade with operator confidence (Priority: P1)

As an operator, I can install the tenant-scoped single-node production baseline and upgrade an existing installation by following documented, repeatable steps without relying on developer-only knowledge.

**Why this priority**: Installation and upgrade are the entry points for production operation. Without documented and verified paths, the product cannot be operated or recovered reliably.

**Independent Test**: Can be tested by having an operator follow the install and upgrade runbooks from a clean baseline and from a prior-version baseline, then verifying the product reaches a healthy state with tenant data intact.

**Acceptance Scenarios**:

1. **Given** a clean production-like single-node host and no existing installation, **When** an operator follows the install runbook, **Then** the product reaches a documented healthy state and the operator can confirm required services, configuration, and diagnostics.
2. **Given** an existing installation with tenant-scoped data and configuration, **When** an operator follows the upgrade runbook, **Then** migration preflight checks pass, postflight checks confirm tenant data integrity, and the operator can identify the safe rollback path.
3. **Given** an upgrade cannot be completed safely, **When** the operator reaches the rollback decision point, **Then** the runbook clearly states whether rollback can proceed in place or requires restore from backup.

---

### User Story 2 - Restore tenant data after failure (Priority: P1)

As an operator, I can create a backup, restore from it, and verify restored tenant data after corruption, failed migration, or deployment rollback.

**Why this priority**: Backup and restore define the recovery boundary for production data. The team needs proof that tenant-scoped data can be recovered without cross-tenant leakage or silent data loss.

**Independent Test**: Can be tested by creating a backup from a multi-tenant data set containing at least three tenants with distinct credential, quota, and work states, restoring it into an isolated environment, and checking that expected tenant records, secrets references, quota state, and operational metadata are present and isolated.

**Acceptance Scenarios**:

1. **Given** a multi-tenant installation with at least three tenants that have distinct credential, quota, and work states, **When** an operator follows the backup workflow, **Then** the backup artifact is created with documented contents, exclusions, retention expectations, and verification steps.
2. **Given** a valid backup artifact, **When** an operator restores it into an isolated environment, **Then** all expected tenant-scoped data is available to the correct tenants, no tenant can observe another tenant's data, and credential-bearing integrations remain disabled until reconnected or revalidated.
3. **Given** an invalid, incomplete, or incompatible backup artifact, **When** an operator attempts restore, **Then** the workflow fails clearly before partial recovery can be mistaken for success.

---

### User Story 3 - Prove long-running behavior under realistic faults (Priority: P1)

As a product engineer, I can run a long-duration soak scenario that exercises runtime, scheduling, integrations, delivery, approvals, quotas, tenant switching, and evaluation behavior under restarts and external-service faults.

**Why this priority**: The product must demonstrate stable long-running operation before later live validation and evaluation-product work can be considered release-ready.

**Independent Test**: Can be tested by running the documented soak scenario in the default isolated test environment and reviewing a generated report with duration, workload, restarts, injected faults, recovery outcomes, resource observations, and pass/fail status.

**Acceptance Scenarios**:

1. **Given** an isolated test environment with fake backends available, **When** the soak scenario runs for the required duration, **Then** the report records workload coverage across runtime, scheduler, integration, delivery, approval, quota, tenant switching, and evaluation behavior.
2. **Given** the soak scenario is running, **When** the daemon is restarted at least three times, **Then** unfinished work reaches a documented recovered, interrupted, retried, or operator-action-needed state.
3. **Given** external-service faults are injected, **When** transient failures, rate limits, auth expiry, provider unavailability, slow responses, and malformed responses occur, **Then** each outcome is classified as recovered, retried until exhaustion, or operator-action-needed.

---

### User Story 4 - Gate future release readiness with reusable evidence (Priority: P2)

As a release owner, I can reuse the operational soak harness and release checklist after later live-validation and evaluation-product work lands, so those surfaces cannot ship without rerunning the operational baseline.

**Why this priority**: Roadmap 39 creates the baseline, but final user-deliverable readiness depends on rerunning it after Roadmaps 40 and 41 expand live side effects and evaluation product behavior.

**Independent Test**: Can be tested by reviewing the release-verification checklist and confirming it names the rerun requirement, required evidence, and blocking thresholds for future release decisions.

**Acceptance Scenarios**:

1. **Given** Roadmap 40 or 41 changes are ready for release, **When** the release owner reviews readiness criteria, **Then** the checklist requires rerunning the operational soak harness before ship approval.
2. **Given** a soak run reports cross-tenant leakage, unclassified failures, restart recovery over 5 minutes, retry exhaustion without operator-action-needed state, queue backlog persisting over 30 minutes, or monotonic resource growth over the full run, **When** the release owner evaluates the result, **Then** the release gate clearly rejects the release.
3. **Given** safe real-account smoke credentials are unavailable, **When** fake-backend coverage passes and the skip is explicitly recorded, **Then** release readiness is not blocked solely by missing real-account smoke credentials.

### Edge Cases

- Upgrade preflight detects tenant-scoped data that cannot be migrated safely.
- Upgrade postflight detects mismatched counts, missing tenant state, or quota/accounting inconsistencies.
- The representative data set has fewer than three tenants or lacks distinct credential, quota, and work states.
- Rollback is requested after a migration has changed persisted state in a way that cannot be safely reversed in place.
- Restore is attempted with a backup from an incompatible version or with missing required material.
- A daemon restart occurs while scheduled work, approval waits, delivery attempts, quota enforcement, or evaluation jobs are in progress.
- External-service faults persist long enough to exhaust retry expectations.
- Real-account smoke credentials are unavailable, expired, revoked, or unsafe to use in the current environment.
- A restore succeeds for tenant data but integration credentials are not yet reconnected or revalidated.
- Resource growth stays within point-in-time limits but one tracked category grows monotonically across the full run.
- A test attempts to touch production user data or live connectors without explicit opt-in.
- An operator attempts to apply the phase 39 runbook to a multi-node managed service topology.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The product MUST provide a production install runbook for a tenant-scoped single-node baseline that lets an operator prepare the environment, configure required settings, start the product, and verify a healthy installation.
- **FR-002**: The product MUST provide an upgrade runbook that covers preflight checks, upgrade execution, postflight verification, failure handling, and rollback decision points.
- **FR-003**: Migration verification MUST include checks before and after upgrade for tenant-scoped data integrity, required configuration, and quota or usage-accounting state.
- **FR-004**: The product MUST provide backup and restore workflows that document what is captured, what is excluded, how backups are verified, and how restored data is validated.
- **FR-005**: Backup and restore verification MUST cover a representative multi-tenant data set with at least three tenants that have distinct credential, quota, and work states, and prove tenants remain isolated after restore.
- **FR-006**: Rollback guidance MUST state when in-place rollback is safe and when restore from backup is the only acceptable recovery path.
- **FR-006a**: Backup artifacts MUST include secret metadata and references only, MUST exclude raw credential material, and MUST require restored credential-bearing integrations to reconnect or revalidate before use.
- **FR-007**: The soak scenario MUST run in an isolated test environment by default and MUST NOT touch production user data unless an operator explicitly opts into a live smoke path.
- **FR-008**: The first production-readiness soak baseline MUST run for at least 24 hours unless the roadmap records a temporary shorter threshold with a reason and a follow-up requirement to rerun the full duration.
- **FR-009**: The soak scenario MUST exercise long-running runtime work, scheduling, integrations, delivery, approvals, quota enforcement, tenant switching, and evaluation behavior.
- **FR-010**: The soak scenario MUST include at least three daemon restarts and classify unfinished work after each restart as recovered, interrupted, retried, or operator-action-needed.
- **FR-011**: External-service fault drills MUST cover transient server failure, rate limiting, auth expiry, provider unavailability, slow response, and malformed response cases using fake backends.
- **FR-012**: External-service fault drills MUST classify every observed outcome as recovered, retry-exhausted, or operator-action-needed.
- **FR-013**: Real-account connection smoke checks MUST be available for supported integration domains where safe credentials are available, and MUST remain opt-in and separate from fake-backend coverage.
- **FR-013a**: Missing safe real-account smoke credentials MUST NOT block release readiness when fake-backend coverage passes and the skip is explicitly recorded with the affected integration domain and reason.
- **FR-014**: The soak report MUST record duration, workload, restarts, injected faults, recovery times, retry exhaustion, queue backlog, resource-growth observations, cross-tenant leakage checks, and overall pass/fail status.
- **FR-015**: The soak report MUST fail completion when it records any cross-tenant leakage, any unclassified failure, restart recovery over 5 minutes, retry exhaustion without an operator-action-needed state, queue backlog persisting over 30 minutes, or monotonic resource growth over the full run.
- **FR-016**: Resource-growth checks MUST cover logs, stored data size, active work count, memory, and open handles where those observations are available to operators.
- **FR-017**: Operator-facing diagnostics MUST make recovery state, fault classification, and action-needed conditions visible enough to support incident response.
- **FR-018**: The release-verification checklist MUST cover install, upgrade, backup, restore, rollback, recovery, real-account smoke eligibility, and the requirement to rerun the soak harness after Roadmaps 40 and 41.
- **FR-019**: The runbooks and release checklist MUST state that multi-node managed service rollout, clustering, and distributed failover are outside the phase 39 baseline.

### Key Entities

- **Install Runbook**: Operator guidance for preparing, configuring, starting, and validating a new tenant-scoped single-node production installation.
- **Upgrade Runbook**: Operator guidance for moving an existing installation forward while preserving tenant data and identifying rollback paths.
- **Backup Artifact**: Recoverable snapshot material with documented contents, exclusions, compatibility expectations, and verification status. It includes secret metadata and references only, not raw credential material.
- **Restore Verification Result**: Evidence that restored tenant data, configuration, secret references, quota state, and operational metadata are correct and isolated.
- **Migration Verification Report**: Preflight and postflight evidence for data integrity, compatibility, and safe rollback decision-making.
- **Soak Scenario**: Long-running operational workload covering runtime, scheduling, integration, delivery, approval, quota, tenant switching, evaluation, restart, and fault behavior.
- **Fault Drill**: A controlled external-service failure case with expected recovery, retry, or operator-action-needed classification.
- **Soak Report**: Evidence artifact recording workload coverage, run duration, restarts, injected faults, recovery outcomes, resource growth, pass/fail thresholds, and final result. Hard-fail thresholds include any cross-tenant leakage, any unclassified failure, restart recovery over 5 minutes, retry exhaustion without an operator-action-needed state, queue backlog persisting over 30 minutes, or monotonic resource growth over the full run.
- **Real-Account Smoke Checklist**: Opt-in checklist for validating supported integration domains with safe credentials while keeping fake-backend coverage mandatory. Missing safe credentials are recorded as an explicit skip and do not block readiness when fake-backend coverage passes.
- **Release Readiness Gate**: Operator and release-owner checklist that blocks release when required evidence is missing or thresholds fail.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: This feature changes operator documentation, verification artifacts, release gates, and operational test expectations for a tenant-scoped single-node production baseline. API, event, and schema changes are not required by the specification unless planning discovers they are necessary to expose backup, restore, diagnostics, or soak evidence.
- **Migration / Rollback**: The feature must document migration preflight and postflight checks and must define rollback paths for failed upgrades. When persisted state cannot be safely reversed in place, the rollback path must require restore from a verified backup.
- **Verification Strategy**: Required verification includes install and upgrade walkthroughs, backup/restore regression on a multi-tenant data set with at least three tenants that have distinct credential, quota, and work states, migration verification evidence, a long-running isolated soak run with restarts and fault drills, opt-in real-account smoke where credentials are available, explicit skip evidence when safe credentials are unavailable, and a release-readiness checklist review.
- **Observability Impact**: Operator-visible diagnostics, reports, logs, and release evidence must expose recovery state, retry exhaustion, queue backlog, resource growth, fault classifications, and action-needed conditions. Missing observability for any required threshold must be treated as a planning gap.
- **Environment & Secrets**: Default validation must use the isolated test environment and fake backends. Real-account smoke must be explicitly enabled, clearly separated from default coverage, and must not log, expose, back up, or restore raw credential material.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An operator can complete a clean tenant-scoped single-node install and confirm product health using only the runbook in 60 minutes or less.
- **SC-002**: An operator can complete a documented upgrade of a representative tenant-scoped installation and obtain passing preflight and postflight verification evidence in 90 minutes or less.
- **SC-003**: A backup from a representative multi-tenant data set containing at least three tenants with distinct credential, quota, and work states can be restored into an isolated environment with 100% of expected tenant-owned records available to the correct tenant, zero observed cross-tenant leakage, zero raw credential material restored, and all credential-bearing integrations blocked until reconnect or revalidation.
- **SC-004**: The first baseline soak run completes at least 24 hours in the isolated test environment and produces a report with all required workload, restart, fault, resource, and pass/fail fields populated.
- **SC-005**: The soak run includes at least three daemon restarts, and 100% of unfinished work after restart is classified as recovered, interrupted, retried, or operator-action-needed.
- **SC-006**: Fault drills cover all required fault categories, and 100% of observed fault outcomes are classified as recovered, retry-exhausted, or operator-action-needed.
- **SC-007**: The soak report records zero cross-tenant leakage, zero unclassified failures, no restart recovery over 5 minutes, no retry exhaustion without operator-action-needed state, no queue backlog persisting over 30 minutes, and no monotonic resource growth over the full run.
- **SC-008**: Release readiness review can be completed in 30 minutes or less using the produced evidence and clearly blocks release if required install, upgrade, backup, restore, rollback, recovery, or soak evidence is missing.
- **SC-009**: For every supported integration domain without safe real-account smoke credentials, readiness evidence records an explicit skip reason while fake-backend coverage remains passing.

## Assumptions

- The supported integration domains are the domains already present in the product; adding new domains is out of scope.
- Fake-backend coverage remains mandatory even when real-account smoke credentials are available.
- Real-account smoke uses operator-provided safe credentials and is skipped with an explicit notation when credentials are unavailable; missing safe credentials do not block readiness if fake-backend coverage passes.
- Restored secret references are sufficient to identify required remediation, but not sufficient to resume credential-bearing work until reconnect or revalidation completes.
- Representative backup/restore data uses at least three tenants so tenant isolation can be checked across different credential, quota, and work states.
- Production user data and live connectors are never touched by default validation.
- Roadmaps 40 and 41 will add later live-validation and evaluation-product surfaces; their final release gate must rerun this operational baseline.
- Enterprise SSO, new memory or self-improvement behavior, payment-provider production launch, and new integration domains are out of scope for this phase.
- Multi-node managed service rollout, clustering, and distributed failover are out of scope for this phase.
