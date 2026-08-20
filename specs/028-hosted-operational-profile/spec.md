# Feature Specification: Hosted Operational Profile And Recovery

**Feature Branch**: `028-hosted-operational-profile`  
**Created**: 2026-04-30  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/028-hosted-operational-profile-and-recovery.md 完成 phase 43 的工作"

## Clarifications

### Session 2026-04-30

- Q: What recovery target should the supervisor contract require after daemon crash or host reboot? → A: Daemon returns to healthy status within 5 minutes, or evidence marks recovery failed.
- Q: What tenant coverage should hosted backup/restore rehearsal require? → A: At least three tenants with distinct credential, quota, and work states.
- Q: How long should hosted operational evidence be retained by default? → A: 90 days by default unless an authorized policy requires longer.
- Q: Which supervisor mode should the first hosted profile require? → A: Repo-owned foreground supervisor wrapping the existing daemon run and status workflows.
- Q: What freshness rule should release evidence satisfy for ship/no-ship review? → A: Evidence must match the reviewed commit, profile, and run identity.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Provision And Supervise Hosted Operation (Priority: P1)

As an operator, I can provision a hosted or test-host operational profile with stable locations for code, data, logs, artifacts, backups, reports, and temporary work, then start and supervise the daemon without reconstructing commands from chat history.

**Why this priority**: Long-lived personal-agent operation cannot be considered stable while deployment paths, data paths, and process behavior depend on operator convention.

**Independent Test**: Can be fully tested by provisioning a clean test host from the profile, starting the daemon through the documented supervisor, and verifying that status, health, logs, data, artifacts, and reports appear in the expected locations.

**Acceptance Scenarios**:

1. **Given** a clean hosted test host with no existing profile, **When** an operator follows the provisioning profile, **Then** the expected code, data, log, artifact, backup, report, and temporary work locations exist and are identifiable.
2. **Given** the hosted profile has been provisioned, **When** the operator starts the daemon through the repo-owned foreground supervisor, **Then** the daemon reaches a healthy state and the operator can inspect its process status, health state, logs, and active profile.
3. **Given** the daemon crashes or the host reboots, **When** the supervisor recovery behavior is evaluated, **Then** the daemon returns to healthy status within 5 minutes or the evidence marks recovery failed.

---

### User Story 2 - Review Release Evidence From One Index (Priority: P1)

As a release reviewer, I can inspect one release evidence index that links deployment identity, configuration profile, logs, soak results, backup, restore, upgrade, rollback, diagnostics, and resource observations.

**Why this priority**: Release review must be artifact-backed and bounded; reviewers should not need chat history or scattered logs to decide ship or no-ship.

**Independent Test**: Can be fully tested by generating release evidence for a hosted profile run and verifying that the index links every required artifact, identifies missing or failed evidence, and supports a ship or no-ship decision within the target review window.

**Acceptance Scenarios**:

1. **Given** deployment, health, soak, backup, restore, upgrade, rollback, diagnostic, log, and resource evidence exist, **When** the release reviewer opens the evidence index, **Then** every required artifact is linked with run identity and status.
2. **Given** required evidence is missing, stale, mismatched to the reviewed commit, profile, or run identity, or a blocking threshold failed, **When** the reviewer opens the evidence index, **Then** the index identifies the issue as a no-ship condition.
3. **Given** evidence contains environment, configuration, or provider details, **When** it is reviewed, **Then** raw secrets and credential-bearing values are absent.

---

### User Story 3 - Rehearse Backup, Restore, And Rollback (Priority: P1)

As an operator, I can create a backup, restore it to an alternate directory or instance, verify tenant-scoped state, and record whether a failed upgrade requires in-place rollback or restore from backup.

**Why this priority**: Hosted operation needs a rehearsed recovery boundary before upgrades, migrations, or long-running failures can be handled safely.

**Independent Test**: Can be fully tested by backing up representative tenant data from at least three tenants with distinct credential, quota, and work states, restoring it into an isolated target, verifying tenant data, credential remediation state, quota state, migration state, and daemon health, then producing a rollback decision record.

**Acceptance Scenarios**:

1. **Given** a hosted profile contains representative tenant-scoped data for at least three tenants with distinct credential, quota, and work states, **When** an operator runs backup and restore rehearsal to an alternate target, **Then** restored tenant data, migration state, credential remediation state, quota state, and daemon health are verified.
2. **Given** an upgrade or migration verification fails, **When** the operator reaches the rollback decision point, **Then** the decision record states whether in-place rollback is safe or restore from backup is required.
3. **Given** a restore artifact is invalid, incomplete, or incompatible, **When** restore rehearsal runs, **Then** the failure is clear and cannot be mistaken for a successful recovery.

---

### User Story 4 - Capture Upgrade Preflight And Postflight Evidence (Priority: P2)

As an operator, I can run hosted-profile upgrade checks before and after a deployment so that data safety, configuration, daemon health, and rollback readiness are documented.

**Why this priority**: Upgrades are the highest-risk routine operation after deployment; release evidence must show whether the system was safe before and after the change.

**Independent Test**: Can be fully tested by running an upgrade rehearsal on a representative hosted profile and verifying that preflight and postflight evidence are generated, linked, and sufficient to identify pass, fail, or rollback-required outcomes.

**Acceptance Scenarios**:

1. **Given** a hosted profile is ready for upgrade, **When** upgrade preflight runs, **Then** it records deployment identity, data location, artifact location, configuration profile, daemon health, required backup status, and blocking findings.
2. **Given** an upgrade has completed, **When** postflight verification runs, **Then** it records daemon health, data verification, operational diagnostics, and rollback guidance.
3. **Given** preflight or postflight detects an unsafe state, **When** release evidence is generated, **Then** the unsafe state is visible as a no-ship or rollback-required condition.

---

### User Story 5 - Diagnose Hosted Runtime And Host Drift (Priority: P2)

As an engineer, I can review hosted operational observations across a soak or long-running smoke run and identify whether failure likely belongs to daemon behavior, host power or network behavior, provider instability, credential state, quota state, or operator action.

**Why this priority**: Stable long-lived operation requires enough evidence to separate product faults from host and provider conditions without guessing.

**Independent Test**: Can be fully tested by collecting observations during a hosted soak run and verifying that health, database size, log size, memory, goroutines, file descriptors where available, queue or backlog, connector health, MCP health, and integration diagnostic state are present or explicitly marked unsupported.

**Acceptance Scenarios**:

1. **Given** a hosted profile runs a soak or smoke scenario, **When** operational observations are collected, **Then** the report includes all required health, resource, backlog, connector, MCP, and diagnostic fields or explicit unsupported markers.
2. **Given** resource usage grows monotonically or a queue remains backed up, **When** the evidence index is reviewed, **Then** the condition is visible as a blocking operational finding.
3. **Given** the host sleeps, loses network, or has missing credentials during a run, **When** evidence is reviewed, **Then** the likely owner classification distinguishes host, network, credential, provider, daemon, quota, or operator action.

### Edge Cases

- The profile is run from a developer laptop instead of a stable test host or VPS; evidence must show the host class and avoid treating unstable host behavior as production readiness.
- Required directories already exist with incompatible ownership, permissions, or stale artifacts from another run.
- The daemon is manually stopped; the supervisor contract must distinguish intentional stop from crash recovery.
- The daemon crashes repeatedly and cannot recover to health; status and evidence must show failed restart rather than hiding the process loop.
- The host reboots while work is active, while a backup is running, or while upgrade verification is incomplete.
- The representative backup/restore data set has fewer than three tenants or lacks distinct credential, quota, and work states.
- A backup is created before upgrade, but restore verification fails after upgrade.
- Restore succeeds for tenant data, but credential-bearing integrations require remediation before use.
- Upgrade postflight passes daemon health but fails quota, migration, diagnostic, or tenant-state verification.
- Resource observations are unavailable on the host; the report must explicitly mark unsupported fields instead of omitting them silently.
- Logs, reports, or artifacts grow without bound or overwrite prior run evidence.
- Hosted operational evidence reaches its default retention limit; it must expire from normal inspection after 90 days unless an authorized policy requires longer retention.
- Live connector credentials are missing, expired, revoked, or unsafe to use in the hosted test profile.
- Evidence collection encounters raw secrets or credential-bearing environment values; the evidence must suppress those values.
- Release review is attempted with missing evidence links, stale evidence, or artifacts from the wrong commit or profile.
- Release review is attempted with artifacts that do not match the reviewed run identity; the evidence index must treat the mismatch as a no-ship condition.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The repository MUST define a hosted or test-host operational profile for code, daemon data, logs, artifacts, backups, reports, and temporary work.
- **FR-002**: The hosted profile MUST define environment and configuration expectations for hosted test operation and production-like operation without touching production user data by default.
- **FR-003**: The first hosted profile MUST provide a repo-owned foreground supervisor that wraps existing daemon run and status workflows and exposes operator-visible start, stop, restart, status, and health-check workflows for the daemon.
- **FR-004**: The process supervision contract MUST define expected behavior after daemon crash, host reboot, manual stop, failed restart, and repeated crash, and MUST mark crash or reboot recovery failed when daemon health is not restored within 5 minutes.
- **FR-005**: The system MUST produce a structured deployment manifest or report containing commit, branch or version, host, operator, start time, configuration profile, data directory, artifact directory, and supervisor mode.
- **FR-006**: Deployment and evidence paths MUST be stable across reruns and MUST include run identity sufficient to avoid overwriting prior evidence.
- **FR-006a**: Hosted operational evidence MUST use a 90-day default retention period unless an authorized retention policy requires longer retention.
- **FR-007**: Upgrade preflight MUST record deployment identity, hosted profile identity, data location, artifact location, required backup state, daemon health, configuration readiness, and blocking findings.
- **FR-008**: Upgrade postflight MUST record daemon health, tenant data verification, migration state, credential remediation state, quota state, operational diagnostics, and rollback guidance.
- **FR-009**: Backup and restore rehearsal MUST verify restored tenant data, migration state, credential remediation state, quota state, and daemon health for at least three tenants with distinct credential, quota, and work states.
- **FR-010**: Restore rehearsal MUST run against an alternate directory or instance rather than overwriting the active hosted profile.
- **FR-011**: Rollback guidance MUST produce a decision record explaining whether in-place rollback is acceptable or restore from backup is required.
- **FR-012**: Observability collection MUST include daemon health, database size, log size, memory, goroutines, file descriptors where available, queue or backlog state, connector health, MCP health, and integration diagnostic state.
- **FR-013**: Observability reports MUST explicitly mark required fields as unsupported when the host cannot provide them.
- **FR-014**: The release evidence index MUST link deployment manifest, configuration profile, health checks, logs, soak report, backup evidence, restore evidence, upgrade preflight, upgrade postflight, rollback decision, integration diagnostics, and resource observations, and MUST identify the reviewed commit, profile, and run identity.
- **FR-015**: The release checklist MUST identify missing required evidence, stale evidence, evidence mismatched to the reviewed commit, profile, or run identity, failed health checks, failed restore verification, unsafe rollback state, failed thresholds, and suspected secret exposure as no-ship conditions.
- **FR-016**: Operational evidence MUST classify likely ownership of failures as daemon, host, network, provider, credential, quota, operator action, unsupported observation, or unknown.
- **FR-017**: Operational reports, logs, manifests, and evidence indexes MUST avoid raw secrets, credential-bearing environment dumps, tokens, authorization headers, app secrets, and refresh credentials.
- **FR-018**: Hosted or test-host defaults MUST NOT use production user data, live connectors, or privileged credentials unless an authorized operator explicitly chooses that profile.
- **FR-019**: The hosted profile MUST reuse existing production install, upgrade, backup, restore, release-readiness, and soak capabilities when their behavior satisfies these requirements.
- **FR-020**: Runbooks MUST enable a new stable host to be provisioned, started, supervised, upgraded, backed up, restored, rolled back, observed, and reviewed using repository-owned guidance and generated evidence.
- **FR-021**: The first hosted operational profile MUST support a stable always-on test host or VPS and MUST NOT require host-native service managers, Kubernetes, cloud-specific managed services, multi-region infrastructure, or payment-provider production launch.

### Key Entities

- **Hosted Operational Profile**: The named operational contract for hosted or test-host operation, including paths, environment expectations, supervision expectations, and default safety boundaries.
- **Directory Layout**: Stable locations for code, daemon data, logs, artifacts, backups, reports, and temporary work.
- **Supervisor Contract**: Operator-facing behavior for daemon start, stop, restart, health, crash recovery, reboot recovery, manual stop, and failed restart through the repo-owned foreground supervisor.
- **Deployment Manifest**: Structured deployment identity for one hosted run, including commit or version, host, operator, start time, configuration profile, data directory, artifact directory, and supervisor mode.
- **Upgrade Evidence**: Preflight and postflight records showing readiness, executed change identity, daemon health, data verification, operational diagnostics, and rollback guidance.
- **Backup Artifact**: Recoverable snapshot material with documented identity, contents, exclusions, compatibility expectations, verification status, and no raw credential material.
- **Restore Rehearsal Result**: Evidence that a backup restored into an alternate target and preserved tenant data, migration state, credential remediation state, quota state, and daemon health.
- **Rollback Decision Record**: Operator-facing record that states whether rollback can proceed in place or must restore from backup, with supporting evidence links.
- **Operational Observation Report**: Resource, health, backlog, connector, MCP, and diagnostic evidence gathered during hosted operation or soak.
- **Release Evidence Index**: Single artifact index linking all required hosted operational evidence and surfacing no-ship conditions.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: This feature changes operator-facing deployment profile expectations, runbooks, generated operational evidence, release checklist requirements, and hosted test-host verification. It should be additive to existing production operations behavior; API, event, schema, or storage surface changes are not required by this specification unless planning discovers they are necessary to store or expose required evidence.
- **Migration / Rollback**: The feature must preserve existing local test and production-operation workflows while adding a hosted profile. Rollout can be reversed by returning to the prior operator runbooks and disabling hosted-profile evidence generation, but already-produced deployment, backup, restore, rollback, and audit evidence must remain readable for review.
- **Verification Strategy**: Required validation includes directory-layout checks, deployment manifest checks, supervisor start/stop/restart/status/health smoke, crash and reboot recovery evidence where practical, upgrade preflight and postflight rehearsal, backup and restore rehearsal to an alternate target, observability report fixture validation, release evidence index validation, redaction checks, and a manual smoke on a stable test host or VPS.
- **Observability Impact**: The feature must add or update operator-visible evidence for daemon health, database size, log size, memory, goroutines, file descriptors where available, queue or backlog state, connector health, MCP health, integration diagnostic state, failure-owner classification, artifact freshness, and no-ship conditions.
- **Environment & Secrets**: Development and automated validation must default to the test environment and must not touch production user data. Live connectors or privileged credentials require explicit operator opt-in, and operational evidence must never expose raw secrets, tokens, credential-bearing environment values, authorization headers, app secrets, or refresh credentials.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A new stable test host or VPS can be provisioned from the hosted profile and reach daemon health using repository-owned guidance in 60 minutes or less.
- **SC-002**: 100% of hosted profile runs produce a deployment manifest containing commit or version, host, operator, start time, configuration profile, data directory, artifact directory, and supervisor mode.
- **SC-003**: Start, stop, restart, status, and health-check workflows succeed in controlled hosted-profile smoke testing, and 100% of crash or reboot recovery checks either restore daemon health within 5 minutes or record failed recovery evidence.
- **SC-004**: Upgrade rehearsal produces both preflight and postflight evidence, and 100% of seeded blocking findings are visible as no-ship or rollback-required conditions.
- **SC-005**: Backup and restore rehearsal to an alternate target verifies 100% of expected tenant data, migration state, credential remediation state, quota state, and daemon health for at least three tenants with distinct credential, quota, and work states, with zero observed cross-tenant leakage.
- **SC-006**: 100% of rollback decision records state whether in-place rollback is safe or restore from backup is required and link the supporting evidence.
- **SC-007**: 100% of operational observation reports include daemon health, database size, log size, memory, goroutines, queue or backlog state, connector health, MCP health, integration diagnostic state, and either file-descriptor observations or an explicit unsupported marker.
- **SC-008**: 100% of release evidence indexes link the required deployment, configuration, health, log, soak, backup, restore, upgrade, rollback, diagnostic, and resource evidence for the reviewed commit, profile, and run identity, or explicitly identify the missing or mismatched item as a no-ship condition.
- **SC-008a**: 100% of hosted operational evidence in retention tests expires from normal inspection after 90 days unless covered by an authorized longer retention policy.
- **SC-009**: Release reviewers can reach a defensible ship or no-ship decision in 30 minutes or less using the generated evidence index without reading chat history.
- **SC-010**: Redaction validation finds zero raw secrets, tokens, credential-bearing environment values, authorization headers, app secrets, or refresh credentials in generated operational evidence.

## Assumptions

- Roadmaps 34 through 42 are available prerequisites, including tenant identity, tenant-scoped data migration, hosted secrets isolation, billing and quota accounting, production operations soak, live validation, evaluation product evidence, and integration diagnostics.
- The first hosted operational profile targets a stable always-on test host or VPS, not a developer laptop as the readiness baseline.
- Existing production install, upgrade, backup, restore, release-readiness, and soak work should be reused where it already satisfies the hosted profile requirements.
- The first implementation uses a repo-owned foreground supervisor around existing daemon run and status workflows; host-native service managers may be considered later but are out of scope for this phase.
- Hosted test defaults use `KURA_ENV=test` behavior and production-like operation without touching production user data by default.
- Hosted operational evidence uses a 90-day default retention period unless an authorized longer retention policy applies.
- This phase does not add new personal-agent domains, Kubernetes support, cloud-specific managed services, multi-region deployment, external managed secret-manager integration, enterprise SSO, payment-provider production launch, mobile fleet management, or memory/context engineering.
