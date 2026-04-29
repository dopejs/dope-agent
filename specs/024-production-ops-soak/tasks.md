# Tasks: Production Operations Soak

**Input**: Design documents from `specs/024-production-ops-soak/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`

**Tests**: Required by the constitution. Contract, unit, integration, restart, redaction, and operator-evidence validation tasks are included before implementation tasks in each user story.

**Organization**: Tasks are grouped by user story so each story can be implemented and tested independently after the shared foundation is complete.

## Phase 1: Setup

**Purpose**: Establish the Roadmap 39 implementation locations without changing daemon behavior.

- [X] T001 Create the ops readiness package scaffold in `daemon/internal/opsreadiness/doc.go`
- [X] T002 [P] Create production script usage, executable-permission, and invocation notes in `scripts/production/README.md`
- [X] T003 [P] Create Roadmap 39 evidence fixture notes in `specs/024-production-ops-soak/fixtures/README.md`
- [X] T004 [P] Create the production operations documentation index in `docs/runtime/production-operations.md`

---

## Phase 2: Foundational

**Purpose**: Shared evidence types, validation helpers, and contract-test entry points that block all user stories.

**Critical**: No user story work should begin until this phase is complete.

- [X] T005 [P] Define shared evidence structs and status constants in `daemon/internal/opsreadiness/types.go`
- [X] T006 Implement shared validation helpers for required fields, durations, and hard-fail results in `daemon/internal/opsreadiness/validation.go`
- [X] T007 [P] Implement JSON fixture load helpers for ops readiness tests in `daemon/internal/opsreadiness/fixtures.go`
- [X] T008 [P] Add contract coverage for Roadmap 39 planning artifacts in `daemon/internal/contracts/production_ops_contracts_test.go`

**Checkpoint**: Shared evidence validation and contract-test scaffolding are ready.

---

## Phase 3: User Story 1 - Install and upgrade with operator confidence (Priority: P1)

**Goal**: Operators can install the tenant-scoped single-node baseline and upgrade an existing installation with preflight, postflight, diagnostics, and rollback decision points.

**Independent Test**: Follow the install and upgrade runbooks against the default test environment and verify health, migration preflight/postflight evidence, tenant data integrity, and rollback guidance.

### Tests for User Story 1

- [X] T009 [P] [US1] Add install and upgrade runbook completeness and elapsed-time evidence tests in `daemon/internal/contracts/production_install_upgrade_contract_test.go`
- [X] T010 [P] [US1] Add migration preflight and postflight evidence tests in `daemon/internal/opsreadiness/migration_evidence_test.go`
- [X] T011 [P] [US1] Add upgrade rollback decision tests for unsafe in-place rollback in `daemon/internal/opsreadiness/rollback_evidence_test.go`

### Implementation for User Story 1

- [X] T012 [US1] Implement migration evidence validation in `daemon/internal/opsreadiness/migration_evidence.go`
- [X] T013 [US1] Implement rollback decision validation in `daemon/internal/opsreadiness/rollback_evidence.go`
- [X] T014 [P] [US1] Write the clean install runbook with <=60 minute elapsed-time evidence requirements in `docs/runtime/production-install.md`
- [X] T015 [P] [US1] Write the production upgrade runbook with <=90 minute elapsed-time evidence requirements in `docs/runtime/production-upgrade.md`
- [X] T016 [US1] Add upgrade preflight helper script in `scripts/production/upgrade-preflight.sh`
- [X] T017 [US1] Add upgrade postflight helper script in `scripts/production/upgrade-postflight.sh`
- [X] T018 [US1] Update migration rollback cross-references in `docs/runtime/tenant-migration-rollback.md`
- [X] T019 [US1] Update the production operations index with install and upgrade links in `docs/runtime/production-operations.md`

**Checkpoint**: User Story 1 is complete when install and upgrade runbooks pass contract tests and the test-environment walkthrough produces preflight, postflight, and rollback evidence.

---

## Phase 4: User Story 2 - Restore tenant data after failure (Priority: P1)

**Goal**: Operators can back up, restore, and verify tenant-scoped data while excluding raw credential material and leaving credential-bearing integrations blocked until reconnect or revalidation.

**Independent Test**: Create a backup from a representative three-tenant fixture, restore it into an isolated environment, and verify tenant records, quota state, work state, secret references, credential remediation, and zero cross-tenant leakage.

### Tests for User Story 2

- [X] T020 [P] [US2] Add representative three-tenant fixture tests in `daemon/internal/store/migrationfixture/r39_production_ops_test.go`
- [X] T021 [P] [US2] Add backup artifact validation and raw credential exclusion tests in `daemon/internal/opsreadiness/backup_artifact_test.go`
- [X] T022 [P] [US2] Add restore validation and cross-tenant leakage tests in `daemon/internal/opsreadiness/restore_validation_test.go`
- [X] T023 [P] [US2] Add credential reconnect and revalidation state tests after restore in `daemon/internal/opsreadiness/restore_credentials_test.go`

### Implementation for User Story 2

- [X] T024 [US2] Implement the Roadmap 39 three-tenant fixture builder in `daemon/internal/store/migrationfixture/r39_production_ops.go`
- [X] T025 [US2] Implement backup artifact validation in `daemon/internal/opsreadiness/backup_artifact.go`
- [X] T026 [US2] Implement restore validation checks in `daemon/internal/opsreadiness/restore_validation.go`
- [X] T027 [US2] Implement restored credential remediation checks in `daemon/internal/opsreadiness/restore_credentials.go`
- [X] T028 [US2] Add test-state backup helper script in `scripts/production/backup-test-state.sh`
- [X] T029 [US2] Add test-state restore helper script in `scripts/production/restore-test-state.sh`
- [X] T030 [P] [US2] Write the backup and restore runbook in `docs/runtime/backup-restore.md`
- [X] T031 [P] [US2] Update credential isolation docs with backup and restore redaction rules in `docs/runtime/hosted-credential-isolation.md`
- [X] T032 [US2] Update the backup and restore evidence contract with final fixture names in `specs/024-production-ops-soak/contracts/backup-restore-evidence.md`

**Checkpoint**: User Story 2 is complete when backup/restore validation passes on the three-tenant fixture, raw credential material is absent, and restored credential-bearing integrations require reconnect or revalidation.

---

## Phase 5: User Story 3 - Prove long-running behavior under realistic faults (Priority: P1)

**Goal**: Product engineers can run a 24-hour `DOPE_ENV=test` soak that exercises runtime, scheduler, integrations, delivery, approvals, quotas, tenant switching, and evaluation under restarts and external-service faults.

**Independent Test**: Run or validate the soak harness against a generated report containing duration, workload coverage, three restarts, fault drills, recovery classifications, resource observations, and hard-fail threshold results.

### Tests for User Story 3

- [X] T033 [P] [US3] Add soak report required-field and threshold tests in `daemon/internal/opsreadiness/soak_report_test.go`
- [X] T034 [P] [US3] Add soak workload coverage tests in `daemon/internal/opsreadiness/soak_workload_test.go`
- [X] T035 [P] [US3] Add restart recovery classification tests in `daemon/internal/opsreadiness/restart_recovery_test.go`
- [X] T036 [P] [US3] Add fake backend fault drill tests in `daemon/internal/integrations/fault_drill_test.go`
- [X] T037 [P] [US3] Add resource observation tests for logs, stored data size, active work or queue backlog, memory, open handles or file descriptors where available, goroutine count where available, and monotonic growth in `daemon/internal/opsreadiness/resource_observation_test.go`

### Implementation for User Story 3

- [X] T038 [US3] Implement soak report validation in `daemon/internal/opsreadiness/soak_report.go`
- [X] T039 [US3] Implement soak workload coverage validation in `daemon/internal/opsreadiness/soak_workload.go`
- [X] T040 [US3] Implement restart recovery classification validation in `daemon/internal/opsreadiness/restart_recovery.go`
- [X] T041 [US3] Extend fake integration backend fault injection controls in `daemon/internal/integrations/fake_backend.go`
- [X] T042 [US3] Implement resource observation helpers for logs, stored data size, active work or queue backlog, memory, open handles or file descriptors where available, goroutine count where available, and monotonic growth in `daemon/internal/opsreadiness/resource_observation.go`
- [X] T043 [US3] Add the 24-hour soak runner script in `scripts/production/run-soak.sh`
- [X] T044 [US3] Add test daemon restart helper script in `scripts/production/restart-test-daemon.sh`
- [X] T045 [P] [US3] Add a passing soak report fixture in `specs/024-production-ops-soak/fixtures/soak-report.passing.json`
- [X] T046 [P] [US3] Add failing soak report fixtures in `specs/024-production-ops-soak/fixtures/soak-report.failures.json`
- [X] T047 [P] [US3] Write the soak harness operator guide in `docs/harness/production-soak.md`
- [X] T048 [US3] Update the soak harness contract with final report fixture names in `specs/024-production-ops-soak/contracts/soak-harness-report.md`

**Checkpoint**: User Story 3 is complete when the soak harness report validator rejects all hard-fail cases and accepts a report with 24-hour duration, three restarts, all required fault drills, resource observations, and no leakage or unclassified failures.

---

## Phase 6: User Story 4 - Gate future release readiness with reusable evidence (Priority: P2)

**Goal**: Release owners can make a ship/no-ship decision from reusable evidence, including the Roadmaps 40 and 41 rerun gate and real-account smoke skip policy.

**Independent Test**: Validate a release-readiness evidence fixture that passes with recorded real-account skips when fake-backend coverage passes, and fails when required evidence, soak thresholds, credential redaction, or Roadmaps 40/41 rerun requirements are missing.

### Tests for User Story 4

- [X] T049 [P] [US4] Add release readiness gate validator tests including <=30 minute review elapsed-time evidence in `daemon/internal/opsreadiness/release_readiness_test.go`
- [X] T050 [P] [US4] Add real-account smoke skip policy tests in `daemon/internal/opsreadiness/real_account_smoke_test.go`
- [X] T051 [P] [US4] Add Roadmaps 40 and 41 rerun gate contract tests in `daemon/internal/contracts/production_release_gate_contract_test.go`

### Implementation for User Story 4

- [X] T052 [US4] Implement release readiness gate validation including <=30 minute review elapsed-time evidence in `daemon/internal/opsreadiness/release_readiness.go`
- [X] T053 [US4] Implement real-account smoke checklist validation in `daemon/internal/opsreadiness/real_account_smoke.go`
- [X] T054 [P] [US4] Add passing release readiness fixture in `specs/024-production-ops-soak/fixtures/release-readiness.passing.json`
- [X] T055 [P] [US4] Add failing release readiness fixtures in `specs/024-production-ops-soak/fixtures/release-readiness.failures.json`
- [X] T056 [P] [US4] Write the release readiness gate runbook with <=30 minute review elapsed-time evidence requirements in `docs/runtime/release-readiness.md`
- [X] T057 [P] [US4] Write the real-account smoke policy guide in `docs/providers/real-account-smoke.md`
- [X] T058 [US4] Update hosted product roadmap references in `docs/runtime/daemon-roadmaps.md`
- [X] T059 [US4] Update the release readiness gate contract with final fixture names in `specs/024-production-ops-soak/contracts/release-readiness-gate.md`
- [X] T060 [US4] Update upstream Roadmap 39 spec status and artifact links in `docs/specs/024-production-install-upgrade-backup-and-soak.md`

**Checkpoint**: User Story 4 is complete when readiness validation blocks missing or failed evidence, permits recorded real-account skips only with passing fake-backend coverage, and requires the Roadmaps 40/41 soak rerun gate.

---

## Phase 7: Polish & Cross-Cutting Verification

**Purpose**: Final consistency, contract coverage, and repository-level verification.

- [X] T061 [P] Reconcile implementation notes, script executable-permission checks, script invocation checks, and verification expectations in `specs/024-production-ops-soak/quickstart.md`
- [X] T062 [P] Reconcile task completion evidence and residual risks in `specs/024-production-ops-soak/plan.md`
- [X] T063 [P] Update the production operations documentation index with all final artifacts in `docs/runtime/production-operations.md`
- [X] T064 [P] Run targeted Go tests for ops readiness packages and record results in `specs/024-production-ops-soak/quickstart.md`
- [X] T065 Run `go test ./...` from `daemon/` and record results in `specs/024-production-ops-soak/quickstart.md`
- [X] T066 Run `go mod tidy` from `daemon/` and record module diff status in `specs/024-production-ops-soak/quickstart.md`
- [X] T067 Run `make daemon-contract-test` from the repository root and record results in `specs/024-production-ops-soak/quickstart.md`
- [X] T068 Run `pnpm test:clients` and `pnpm build` if client-visible surfaces changed, then record results or no-op rationale in `specs/024-production-ops-soak/quickstart.md`
- [X] T069 Run the test daemon smoke with `make daemon-run-test` and `make daemon-test-status`, then record health and shutdown evidence in `specs/024-production-ops-soak/quickstart.md`
- [X] T070 Run the 24-hour `DOPE_ENV=test` soak and record pass/fail results in `specs/024-production-ops-soak/quickstart.md`; if a temporary shorter threshold is used, record the rationale and mandatory follow-up full-duration rerun in `specs/024-production-ops-soak/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies.
- **Foundational (Phase 2)**: Depends on Phase 1 and blocks all user stories.
- **User Stories (Phases 3-6)**: Depend on Phase 2. User Stories 1, 2, and 3 are all P1 and may proceed in parallel after foundation if file ownership is coordinated.
- **Polish (Phase 7)**: Depends on selected user stories being complete; full roadmap closure requires all user stories.

### User Story Dependencies

- **US1 Install and upgrade**: Can start after foundation. It does not depend on US2, US3, or US4.
- **US2 Backup and restore**: Can start after foundation. It benefits from US1 rollback wording but is independently testable through fixture and restore validation.
- **US3 Long-running soak**: Can start after foundation. It can use US2 fixtures when available, but report validation can be developed independently with fixtures.
- **US4 Release readiness**: Can start after foundation, but final acceptance depends on evidence from US1, US2, and US3.

### Within Each User Story

- Write story tests before implementation.
- Implement validators before scripts or runbooks depend on their evidence format.
- Add fixtures before contract tests are finalized.
- Update docs and contracts after implementation names and file paths are stable.

## Parallel Opportunities

- Setup tasks T002, T003, and T004 can run in parallel.
- Foundational tasks T005, T007, and T008 can run in parallel once T001 exists.
- US1 tests T009, T010, and T011 can run in parallel.
- US2 tests T020, T021, T022, and T023 can run in parallel.
- US3 tests T033, T034, T035, T036, and T037 can run in parallel.
- US4 tests T049, T050, and T051 can run in parallel.
- Documentation tasks in different files can run in parallel with implementation tasks after validators define final evidence names.

## Parallel Example: User Story 2

```text
Task: "T020 Add representative three-tenant fixture tests in daemon/internal/store/migrationfixture/r39_production_ops_test.go"
Task: "T021 Add backup artifact validation and raw credential exclusion tests in daemon/internal/opsreadiness/backup_artifact_test.go"
Task: "T022 Add restore validation and cross-tenant leakage tests in daemon/internal/opsreadiness/restore_validation_test.go"
Task: "T023 Add credential reconnect and revalidation state tests after restore in daemon/internal/opsreadiness/restore_credentials_test.go"
Task: "T030 Write the backup and restore runbook in docs/runtime/backup-restore.md"
Task: "T031 Update credential isolation docs with backup and restore redaction rules in docs/runtime/hosted-credential-isolation.md"
```

## Parallel Example: User Story 3

```text
Task: "T033 Add soak report required-field and threshold tests in daemon/internal/opsreadiness/soak_report_test.go"
Task: "T034 Add soak workload coverage tests in daemon/internal/opsreadiness/soak_workload_test.go"
Task: "T035 Add restart recovery classification tests in daemon/internal/opsreadiness/restart_recovery_test.go"
Task: "T036 Add fake backend fault drill tests in daemon/internal/integrations/fault_drill_test.go"
Task: "T037 Add resource observation and monotonic-growth tests in daemon/internal/opsreadiness/resource_observation_test.go"
Task: "T047 Write the soak harness operator guide in docs/harness/production-soak.md"
```

## Implementation Strategy

### First Increment

1. Complete Phase 1 and Phase 2.
2. Complete User Story 1 to establish install and upgrade confidence.
3. Validate US1 independently with runbook contract tests and a test-environment walkthrough; this is an implementation checkpoint, not shippable roadmap completion.

### Roadmap Closure

1. Complete US1 for install and upgrade.
2. Complete US2 for backup, restore, tenant isolation, and credential remediation.
3. Complete US3 for 24-hour soak, restart/fault classification, and resource-growth checks.
4. Complete US4 for release readiness and Roadmaps 40/41 rerun gate.
5. Complete Phase 7 verification and record any residual risks.

### Parallel Team Strategy

After Phase 2, separate owners can work on US1 docs/scripts, US2 fixture/restore validation, US3 soak harness/report validation, and US4 release gate validation. Coordinate shared edits in `daemon/internal/opsreadiness/types.go`, `daemon/internal/opsreadiness/validation.go`, and `specs/024-production-ops-soak/quickstart.md`.

## Notes

- `[P]` tasks touch different files and can run in parallel after their phase prerequisites.
- Story labels map directly to `spec.md` user stories.
- Contract, redaction, restart, and evidence validation are required production checks.
- Default validation must use `~/.dope-test`; live connectors and real-account credentials require explicit opt-in.
- Completion requires the 24-hour soak evidence or an explicitly documented temporary shorter threshold with a follow-up full-duration rerun.
