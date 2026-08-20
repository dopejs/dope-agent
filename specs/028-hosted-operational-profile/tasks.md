# Tasks: Hosted Operational Profile And Recovery

**Input**: Design documents from `specs/028-hosted-operational-profile/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`

**Tests**: Required by the constitution and feature specification. Contract, unit,
script, restart, redaction, retention, stable-host, and operator-evidence validation
tasks are included before implementation tasks in each user story.

**Organization**: Tasks are grouped by user story so each story can be implemented and
tested independently after the shared foundation is complete.

## Phase 1: Setup

**Purpose**: Establish Roadmap 43 implementation locations and evidence fixture anchors without changing existing Roadmap 39 behavior.

- [X] T001 Create hosted operational profile documentation stub in `docs/runtime/hosted-operational-profile.md`
- [X] T002 [P] Create hosted profile stable-host evidence guide stub in `docs/harness/hosted-operational-profile.md`
- [X] T003 [P] Create hosted profile script entrypoint stub in `scripts/production/hosted-profile.sh`
- [X] T004 [P] Create hosted evidence fixture directory notes in `specs/028-hosted-operational-profile/fixtures/README.md`
- [X] T005 [P] Create hosted evidence contract test fixture directory in `daemon/internal/opsreadiness/testdata/hosted/README.md`
- [X] T006 [P] Update production script helper inventory for hosted profile commands in `scripts/production/README.md`

---

## Phase 2: Foundational

**Purpose**: Shared hosted evidence types, validation helpers, redaction checks, and contract-test entry points that block all user stories.

**Critical**: No user story work should begin until this phase is complete.

- [X] T007 Define hosted profile, run, manifest, supervisor event, restore rehearsal, observation report, and release index types in `daemon/internal/opsreadiness/hosted_types.go`
- [X] T008 Implement hosted evidence validation helpers for required fields, identity matching, 90-day retention, and no-ship findings in `daemon/internal/opsreadiness/hosted_validation.go`
- [X] T009 [P] Implement hosted evidence fixture load helpers in `daemon/internal/opsreadiness/hosted_fixtures.go`
- [X] T010 [P] Add hosted redaction sentinel checks for reports, manifests, logs, and fixture payloads in `daemon/internal/opsreadiness/hosted_redaction.go`
- [X] T011 [P] Add contract tests mapping Roadmap 43 planning contracts to required implementation evidence in `daemon/internal/contracts/hosted_operational_profile_contract_test.go`
- [X] T012 Add foundational hosted validation tests for identity matching, retention expiry, unsupported markers, and redaction in `daemon/internal/opsreadiness/hosted_validation_test.go`
- [X] T013 [P] Add hosted evidence passing and failing fixture skeletons in `daemon/internal/opsreadiness/testdata/hosted/hosted_evidence.passing.json` and `daemon/internal/opsreadiness/testdata/hosted/hosted_evidence.failures.json`

**Checkpoint**: Shared hosted evidence validation and contract-test scaffolding are ready.

---

## Phase 3: User Story 1 - Provision And Supervise Hosted Operation (Priority: P1) MVP

**Goal**: Operators can provision the hosted profile, start the daemon through a repo-owned foreground supervisor, inspect status and health, and see failed crash/reboot recovery when health is not restored within 5 minutes.

**Independent Test**: Provision a clean test-host profile, start through the hosted supervisor, verify expected directories and manifest paths, run status and health, and validate simulated crash/reboot recovery evidence.

### Tests for User Story 1

- [X] T014 [P] [US1] Add hosted profile directory layout, default environment, and <=60 minute provisioning elapsed-time tests in `daemon/internal/opsreadiness/hosted_profile_test.go`
- [X] T015 [P] [US1] Add hosted deployment manifest required-field and redaction tests in `daemon/internal/opsreadiness/hosted_manifest_test.go`
- [X] T016 [P] [US1] Add hosted supervisor start, stop, restart, status, health, manual-stop, repeated-crash, reboot-recovery, and 5-minute recovery tests in `daemon/internal/opsreadiness/hosted_supervisor_test.go`
- [X] T017 [US1] Add script contract tests for hosted provision/start/stop/restart/status/health behavior in `daemon/internal/contracts/hosted_profile_commands_contract_test.go`

### Implementation for User Story 1

- [X] T018 [US1] Implement hosted profile directory layout validation and run identity generation in `daemon/internal/opsreadiness/hosted_profile.go`
- [X] T019 [US1] Implement deployment manifest generation and redaction-safe serialization in `daemon/internal/opsreadiness/hosted_manifest.go`
- [X] T020 [US1] Implement foreground supervisor event validation, reboot-recovery event handling, and 5-minute recovery classification in `daemon/internal/opsreadiness/hosted_supervisor.go`
- [X] T021 [US1] Implement hosted provision/start/stop/restart/status/health workflows in `scripts/production/hosted-profile.sh`
- [X] T022 [US1] Update hosted operational profile runbook with directory layout, default test environment, live opt-in rules, <=60 minute provisioning evidence, and supervisor commands in `docs/runtime/hosted-operational-profile.md`
- [X] T023 [P] [US1] Update stable-host evidence guide with acceptable host classes and developer-laptop limitations in `docs/harness/hosted-operational-profile.md`
- [X] T024 [US1] Update production operations index with hosted profile links in `docs/runtime/production-operations.md`
- [X] T025 [US1] Add hosted supervisor passing and failure fixtures in `daemon/internal/opsreadiness/testdata/hosted/supervisor_events.json`

**Checkpoint**: User Story 1 is complete when hosted profile provisioning, daemon supervision, status, health, and recovery evidence are independently validated.

---

## Phase 4: User Story 2 - Review Release Evidence From One Index (Priority: P1)

**Goal**: Release reviewers can open one evidence index that links all required artifacts, verifies commit/profile/run identity, flags no-ship conditions, applies 90-day retention, and excludes secret material.

**Independent Test**: Generate passing and failing release evidence fixtures and verify missing, stale, mismatched, failed, expired, and redaction-failed evidence all produce no-ship decisions.

### Tests for User Story 2

- [X] T026 [P] [US2] Add release evidence index required-link, identity-match, and 30-minute review tests in `daemon/internal/opsreadiness/hosted_release_index_test.go`
- [X] T027 [P] [US2] Add release evidence retention expiry and authorized longer-retention tests in `daemon/internal/opsreadiness/hosted_retention_test.go`
- [X] T028 [P] [US2] Add release index redaction failure tests for manifests, logs, reports, diagnostics, and resource observations in `daemon/internal/opsreadiness/hosted_redaction_test.go`
- [X] T029 [US2] Add release readiness contract tests for hosted no-ship rules and ship-with-recorded-skips inheritance in `daemon/internal/contracts/hosted_release_evidence_contract_test.go`

### Implementation for User Story 2

- [X] T030 [US2] Implement hosted release evidence index validation and decision calculation in `daemon/internal/opsreadiness/hosted_release_index.go`
- [X] T031 [US2] Implement hosted retention validation and normal-inspection expiry handling in `daemon/internal/opsreadiness/hosted_retention.go`
- [X] T032 [US2] Implement release index generation workflow in `scripts/production/hosted-profile.sh`
- [X] T033 [US2] Add passing, missing-evidence, mismatched-identity, expired, and redaction-failed release index fixtures in `daemon/internal/opsreadiness/testdata/hosted/release_index.json`
- [X] T034 [US2] Update release readiness runbook with hosted index required links, identity freshness, no-ship rules, and 90-day retention in `docs/runtime/release-readiness.md`
- [X] T035 [US2] Update hosted operational profile runbook with release evidence index generation and review workflow in `docs/runtime/hosted-operational-profile.md`
- [X] T036 [P] [US2] Update `specs/028-hosted-operational-profile/contracts/release-evidence-index.md` with final fixture names and implemented command names

**Checkpoint**: User Story 2 is complete when the evidence index can independently accept passing fixture evidence and reject missing, stale, mismatched, failed, expired, or secret-exposing evidence.

---

## Phase 5: User Story 3 - Rehearse Backup, Restore, And Rollback (Priority: P1)

**Goal**: Operators can back up hosted profile state, restore it to an alternate target, verify at least three tenants with distinct credential, quota, and work states, and record whether rollback can happen in place or requires restore from backup.

**Independent Test**: Back up representative tenant state, restore to an alternate directory, verify tenant state, migration state, credential remediation state, quota state, daemon health, zero cross-tenant leakage, and rollback decision evidence.

### Tests for User Story 3

- [X] T037 [P] [US3] Add hosted backup evidence validation tests for run identity, checksum, included/excluded material, and redaction in `daemon/internal/opsreadiness/hosted_backup_test.go`
- [X] T038 [P] [US3] Add hosted restore rehearsal tests for alternate target, three-tenant coverage, tenant isolation, quota state, credential remediation, migration state, and daemon health in `daemon/internal/opsreadiness/hosted_restore_test.go`
- [X] T039 [P] [US3] Add hosted rollback decision tests for in-place rollback, restore-from-backup-required, no-rollback-needed, and blocked decisions in `daemon/internal/opsreadiness/hosted_rollback_test.go`
- [X] T040 [US3] Add contract tests proving release index treats failed backup, restore, tenant coverage, raw credential, and rollback evidence as no-ship in `daemon/internal/contracts/hosted_recovery_evidence_contract_test.go`

### Implementation for User Story 3

- [X] T041 [US3] Implement hosted backup evidence validation by extending backup artifact checks in `daemon/internal/opsreadiness/hosted_backup.go`
- [X] T042 [US3] Implement hosted restore rehearsal validation for alternate target and three-tenant recovery evidence in `daemon/internal/opsreadiness/hosted_restore.go`
- [X] T043 [US3] Implement hosted rollback decision validation and evidence linking in `daemon/internal/opsreadiness/hosted_rollback.go`
- [X] T044 [US3] Extend backup and restore helper scripts to emit hosted run identity and alternate-target restore evidence in `scripts/production/backup-test-state.sh` and `scripts/production/restore-test-state.sh`
- [X] T045 [US3] Add hosted backup, restore, and rollback fixture payloads in `daemon/internal/opsreadiness/testdata/hosted/recovery_evidence.json`
- [X] T046 [US3] Update backup and restore runbook with hosted alternate-target rehearsal and three-tenant verification rules in `docs/runtime/backup-restore.md`
- [X] T047 [US3] Update tenant migration rollback runbook with hosted rollback decision record requirements in `docs/runtime/tenant-migration-rollback.md`
- [X] T048 [P] [US3] Update `specs/028-hosted-operational-profile/contracts/recovery-evidence.md` with final fixture names and implemented command names

**Checkpoint**: User Story 3 is complete when hosted backup/restore rehearsal and rollback decision evidence are independently validated and linked into release readiness.

---

## Phase 6: User Story 4 - Capture Upgrade Preflight And Postflight Evidence (Priority: P2)

**Goal**: Operators can run hosted upgrade preflight and postflight checks that record deployment identity, data safety, backup state, daemon health, tenant data verification, diagnostics, and rollback guidance.

**Independent Test**: Run hosted preflight and postflight against representative fixture evidence and verify safe states pass while unsafe backup, health, migration, quota, credential, or diagnostic states block release readiness.

### Tests for User Story 4

- [X] T049 [P] [US4] Add hosted upgrade preflight validation tests for deployment identity, hosted profile identity, backup state, daemon health, configuration readiness, and blocking findings in `daemon/internal/opsreadiness/hosted_upgrade_test.go`
- [X] T050 [P] [US4] Add hosted upgrade postflight validation tests for tenant data, migration state, credential remediation, quota state, operational diagnostics, and rollback guidance in `daemon/internal/opsreadiness/hosted_upgrade_postflight_test.go`
- [X] T051 [US4] Add contract tests proving release index treats missing, failed, or mismatched upgrade evidence as no-ship in `daemon/internal/contracts/hosted_upgrade_evidence_contract_test.go`

### Implementation for User Story 4

- [X] T052 [US4] Implement hosted upgrade preflight and postflight evidence validation in `daemon/internal/opsreadiness/hosted_upgrade.go`
- [X] T053 [US4] Extend upgrade preflight helper to emit hosted deployment identity, data location, artifact location, backup state, health, readiness, and blocking findings in `scripts/production/upgrade-preflight.sh`
- [X] T054 [US4] Extend upgrade postflight helper to emit hosted daemon health, tenant verification, migration state, credential remediation, quota state, diagnostics, and rollback guidance in `scripts/production/upgrade-postflight.sh`
- [X] T055 [US4] Add hosted upgrade preflight and postflight fixture payloads in `daemon/internal/opsreadiness/testdata/hosted/upgrade_evidence.json`
- [X] T056 [US4] Update production upgrade runbook with hosted preflight/postflight evidence and rollback handoff rules in `docs/runtime/production-upgrade.md`
- [X] T057 [P] [US4] Update `specs/028-hosted-operational-profile/contracts/recovery-evidence.md` with final hosted upgrade fixture names and command fields

**Checkpoint**: User Story 4 is complete when hosted upgrade evidence can independently prove pass, fail, and rollback-required outcomes.

---

## Phase 7: User Story 5 - Diagnose Hosted Runtime And Host Drift (Priority: P2)

**Goal**: Engineers can review hosted operational observations and classify failures as daemon, host, network, provider, credential, quota, operator action, unsupported observation, or unknown.

**Independent Test**: Generate observability reports for healthy, unsupported, failed, monotonic-growth, backlog, stale-diagnostic, and redaction-failed cases and verify release evidence surfaces blocking findings.

### Tests for User Story 5

- [X] T058 [P] [US5] Add hosted observability required-field and unsupported-marker tests in `daemon/internal/opsreadiness/hosted_observability_test.go`
- [X] T059 [P] [US5] Add failure-owner classification tests for daemon, host, network, provider, credential, quota, operator action, unsupported observation, and unknown in `daemon/internal/opsreadiness/hosted_failure_owner_test.go`
- [X] T060 [P] [US5] Add hosted resource-growth and queue/backlog blocking-finding tests in `daemon/internal/opsreadiness/hosted_resource_observation_test.go`
- [X] T061 [US5] Add contract tests proving observability missing fields, unsupported markers, failed diagnostics, and redaction failures are reflected in release index decisions in `daemon/internal/contracts/hosted_observability_contract_test.go`

### Implementation for User Story 5

- [X] T062 [US5] Implement hosted observability report validation and unsupported marker handling in `daemon/internal/opsreadiness/hosted_observability.go`
- [X] T063 [US5] Implement hosted failure-owner classification helpers in `daemon/internal/opsreadiness/hosted_failure_owner.go`
- [X] T064 [US5] Extend production soak report handling to include hosted profile identity, integration diagnostic state, connector health, MCP health, unsupported markers, and failure-owner classification in `scripts/production/run-soak.sh`
- [X] T065 [US5] Add hosted observability passing and failing fixture payloads in `daemon/internal/opsreadiness/testdata/hosted/observability_report.json`
- [X] T066 [US5] Update production soak guide with hosted observability, stable-host, unsupported-marker, and failure-owner requirements in `docs/harness/production-soak.md`
- [X] T067 [P] [US5] Update `specs/028-hosted-operational-profile/contracts/observability-report.md` with final fixture names and implemented field names

**Checkpoint**: User Story 5 is complete when hosted runtime observations can independently identify missing telemetry, blocking growth/backlog, stale diagnostics, and likely failure owner.

---

## Phase 8: Polish & Cross-Cutting Verification

**Purpose**: Final consistency, contract coverage, stable-host validation, and repository-level verification.

- [X] T068 [P] Reconcile hosted implementation notes and command examples in `specs/028-hosted-operational-profile/quickstart.md`
- [X] T069 [P] Reconcile task completion evidence and residual risks in `specs/028-hosted-operational-profile/plan.md`
- [X] T070 [P] Update hosted upstream spec status and artifact links in `docs/specs/028-hosted-operational-profile-and-recovery.md`
- [X] T071 [P] Update daemon roadmap Roadmap 43 status and artifact links in `docs/runtime/daemon-roadmaps.md`
- [X] T072 [P] Run targeted Go tests for hosted readiness packages with `go test ./internal/opsreadiness ./internal/contracts ./internal/store/migrationfixture ./internal/store/tenancy ./internal/secrets ./internal/billing` from `daemon/` and record results in `specs/028-hosted-operational-profile/quickstart.md`
- [X] T073 Run `go test ./...` from `daemon/` and record results in `specs/028-hosted-operational-profile/quickstart.md`
- [X] T074 Run `go mod tidy` from `daemon/` and record module diff status in `specs/028-hosted-operational-profile/quickstart.md`
- [X] T075 Run `make daemon-contract-test` from the repository root and record results in `specs/028-hosted-operational-profile/quickstart.md`
- [X] T076 Run `make daemon-run-test` and `make daemon-test-status` from the repository root and record hosted test daemon health evidence in `specs/028-hosted-operational-profile/quickstart.md`
- [X] T077 Run hosted-profile targeted validation through `scripts/production/hosted-profile.sh` and record <=60 minute provision, start, status, health, evidence-index, and stop evidence in `specs/028-hosted-operational-profile/quickstart.md`
- [X] T078 Run a manual smoke on a stable always-on test host or VPS and record host class, run identity, reboot-recovery evidence, and any residual gaps in `specs/028-hosted-operational-profile/quickstart.md`
- [X] T079 Run `pnpm test:clients` and `pnpm build` only if hosted evidence is exposed through SDK, web, or TUI surfaces, then record results or no-op rationale in `specs/028-hosted-operational-profile/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies.
- **Foundational (Phase 2)**: Depends on Phase 1 and blocks all user stories.
- **User Stories (Phases 3-7)**: Depend on Phase 2. User Stories 1, 2, and 3 are all P1 and may proceed in parallel after foundation if file ownership is coordinated.
- **Polish (Phase 8)**: Depends on all user stories needed for Roadmap 43 closure.

### User Story Dependencies

- **US1 Provision and supervise**: Can start after foundation. It is the MVP and provides hosted profile identity used by other stories.
- **US2 Release evidence index**: Can start after foundation using fixtures, but final acceptance depends on evidence produced by US1, US3, US4, and US5.
- **US3 Backup, restore, and rollback**: Can start after foundation and uses existing Roadmap 39 backup/restore fixtures.
- **US4 Upgrade evidence**: Can start after foundation and integrates with US3 rollback evidence for final readiness.
- **US5 Runtime and host drift diagnostics**: Can start after foundation and integrates with US2 release index validation.

### Within Each User Story

- Write story tests before implementation.
- Implement validators before scripts and runbooks depend on the evidence format.
- Add fixtures before finalizing contract tests.
- Update docs and planning contracts after implementation names and file paths are stable.
- Story checkpoint must pass before relying on that story for Roadmap 43 closure.

## Parallel Opportunities

- Setup tasks T002-T006 can run in parallel.
- Foundational tasks T009-T011 and T013 can run in parallel after T007 defines shared types.
- US1 tests T014-T016 can run in parallel; T017 depends on command contract wording.
- US2 tests T026-T028 can run in parallel; T029 depends on final index no-ship rules.
- US3 tests T037-T039 can run in parallel; T040 depends on release-index contract integration.
- US4 tests T049-T050 can run in parallel; T051 depends on release-index contract integration.
- US5 tests T058-T060 can run in parallel; T061 depends on release-index contract integration.
- Documentation tasks in different files can run in parallel after validators define final field names.
- Avoid parallel edits to `scripts/production/hosted-profile.sh`, `scripts/production/run-soak.sh`, `docs/runtime/hosted-operational-profile.md`, and `specs/028-hosted-operational-profile/quickstart.md` unless file ownership is explicitly split.

## Parallel Example: User Story 1

```text
Task: "T014 [P] [US1] Add hosted profile directory layout and default environment tests in daemon/internal/opsreadiness/hosted_profile_test.go"
Task: "T015 [P] [US1] Add hosted deployment manifest required-field and redaction tests in daemon/internal/opsreadiness/hosted_manifest_test.go"
Task: "T016 [P] [US1] Add hosted supervisor lifecycle and recovery tests in daemon/internal/opsreadiness/hosted_supervisor_test.go"
```

## Parallel Example: User Story 3

```text
Task: "T037 [P] [US3] Add hosted backup evidence validation tests in daemon/internal/opsreadiness/hosted_backup_test.go"
Task: "T038 [P] [US3] Add hosted restore rehearsal tests in daemon/internal/opsreadiness/hosted_restore_test.go"
Task: "T039 [P] [US3] Add hosted rollback decision tests in daemon/internal/opsreadiness/hosted_rollback_test.go"
Task: "T046 [US3] Update backup and restore runbook in docs/runtime/backup-restore.md"
```

## Parallel Example: User Story 5

```text
Task: "T058 [P] [US5] Add hosted observability required-field and unsupported-marker tests in daemon/internal/opsreadiness/hosted_observability_test.go"
Task: "T059 [P] [US5] Add failure-owner classification tests in daemon/internal/opsreadiness/hosted_failure_owner_test.go"
Task: "T060 [P] [US5] Add hosted resource-growth and queue/backlog tests in daemon/internal/opsreadiness/hosted_resource_observation_test.go"
Task: "T066 [US5] Update production soak guide in docs/harness/production-soak.md"
```

## Implementation Strategy

### MVP First

1. Complete Phase 1 and Phase 2.
2. Complete Phase 3 for User Story 1.
3. Validate hosted profile provisioning and supervision independently.
4. Stop at this checkpoint if needed; this is a useful operator increment but not full Roadmap 43 closure.

### Roadmap Closure

1. Complete US1 for hosted provision and supervision.
2. Complete US2 for release evidence index and no-ship rules.
3. Complete US3 for backup, restore, and rollback rehearsal.
4. Complete US4 for upgrade preflight and postflight evidence.
5. Complete US5 for observability and failure-owner classification.
6. Complete Phase 8 verification, including stable-host smoke and explicit residual risk notes.

### Parallel Team Strategy

After Phase 2, separate owners can work on US1 hosted supervisor, US2 release index,
US3 recovery evidence, US4 upgrade evidence, and US5 observability. Coordinate shared
edits in `daemon/internal/opsreadiness/hosted_types.go`,
`daemon/internal/opsreadiness/hosted_validation.go`,
`scripts/production/hosted-profile.sh`, and
`specs/028-hosted-operational-profile/quickstart.md`.

## Notes

- `[P]` tasks touch different files and can run in parallel after their phase prerequisites.
- Story labels map directly to `spec.md` user stories.
- Contract, redaction, retention, restart, stable-host, and evidence validation are required production checks.
- Default validation must use `~/.kura-test`; live connectors and production user data require explicit opt-in.
- Roadmap 43 is incomplete until release evidence links hosted profile identity, recovery evidence, upgrade evidence, observability, diagnostics, retention, and redaction checks for the reviewed commit/profile/run identity.
