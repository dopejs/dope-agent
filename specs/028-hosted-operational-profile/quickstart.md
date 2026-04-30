# Quickstart: Hosted Operational Profile And Recovery

This quickstart describes the expected validation flow for Roadmap 43 planning. Commands
may be introduced or refined during implementation, but they must preserve these
environment and evidence rules.

## Defaults

- Use the test environment by default.
- Data directory: `~/.dope-test`.
- Daemon address: `127.0.0.1:19192`.
- Live connectors: disabled unless an authorized operator explicitly opts in.
- Evidence retention: 90 days by default unless an authorized policy requires longer.

Do not point hosted-profile helpers at `~/.dope` or production user data unless the
runbook explicitly marks the current step as production recovery.

## 1. Verify Existing Baseline

```bash
test -x scripts/production/hosted-profile.sh
test -x scripts/production/upgrade-preflight.sh
test -x scripts/production/upgrade-postflight.sh
test -x scripts/production/backup-test-state.sh
test -x scripts/production/restore-test-state.sh
test -x scripts/production/run-soak.sh
test -x scripts/production/restart-test-daemon.sh
```

## 2. Start The Test Daemon

```bash
make daemon-run-test
make daemon-test-status
```

Expected evidence:

- daemon health passes
- data directory is `~/.dope-test`
- daemon address is `127.0.0.1:19192`
- live connector mode is disabled unless explicitly opted in

## 3. Generate Hosted Deployment Evidence

```bash
scripts/production/hosted-profile.sh provision
scripts/production/hosted-profile.sh start
scripts/production/hosted-profile.sh status
scripts/production/hosted-profile.sh health
```

The hosted profile must generate a deployment manifest containing:

- commit or version
- branch
- host
- operator
- start time
- configuration profile
- data directory
- artifact directory
- supervisor mode
- daemon address
- live connector mode

The manifest must not include raw environment dumps, tokens, app secrets, refresh tokens,
authorization headers, or credential-bearing values.

## 4. Validate Supervisor Behavior

Run or simulate:

- start
- status
- health check
- restart
- manual stop
- crash recovery
- reboot recovery where practical

Expected evidence:

- crash or reboot recovery returns daemon health within 5 minutes, or records failed
  recovery
- manual stop is classified separately from crash recovery
- repeated crash is visible as failed restart or operator action needed

The implemented dry-run smoke command for reboot recovery is:

```bash
scripts/production/hosted-profile.sh reboot-recovery
```

## 5. Rehearse Upgrade Evidence

```bash
scripts/production/upgrade-preflight.sh
scripts/production/upgrade-postflight.sh
```

Expected evidence:

- preflight links deployment identity, data location, artifact location, backup state,
  daemon health, configuration readiness, and blocking findings
- postflight links daemon health, tenant data verification, migration state, credential
  remediation state, quota state, diagnostics, and rollback guidance

## 6. Rehearse Backup And Restore

```bash
scripts/production/backup-test-state.sh
scripts/production/restore-test-state.sh <backup-artifact>
```

Expected evidence:

- restore uses an alternate directory or instance
- at least three tenants with distinct credential, quota, and work states are verified
- tenant data, migration state, credential remediation state, quota state, and daemon
  health pass
- cross-tenant leakage is zero
- raw credential material is absent

## 7. Generate Observability Report

The hosted observability report must include:

- daemon health
- database size
- log size
- memory
- goroutines
- file descriptors where available
- queue or backlog state
- connector health
- MCP health
- integration diagnostic state
- explicit unsupported markers for unavailable observations
- failure-owner classification

## 8. Generate Release Evidence Index

The release evidence index must link:

- deployment manifest
- configuration profile
- health checks
- logs
- soak report
- backup evidence
- restore evidence
- upgrade preflight
- upgrade postflight
- rollback decision
- integration diagnostics
- resource observations
- redaction check
- retention metadata

The index must identify the reviewed commit, hosted profile, and run identity. Missing,
stale, mismatched, failed, or secret-exposing evidence is a no-ship condition.

Implemented command:

```bash
scripts/production/hosted-profile.sh evidence-index
RELEASE_INDEX_PATH=/path/printed/by/release_evidence_index
cd daemon
go run ./cmd/hosted-evidence-validate "$RELEASE_INDEX_PATH"
```

`hosted-profile.sh evidence-index` also runs the Go validator automatically in
`--allow-no-ship` mode so generated `no_ship` indexes remain structurally
validated. Run `go run ./cmd/hosted-evidence-validate <index>` without
`--allow-no-ship` when the index is expected to be ship-ready.

## 9. Run Verification

Required before implementation can be considered complete:

```bash
cd daemon
go test ./...
go mod tidy
cd ..
make daemon-contract-test
make daemon-run-test
make daemon-test-status
```

Also run a manual smoke on a stable always-on test host or VPS. A developer laptop is
acceptable for targeted validation only and is not sufficient release evidence.

Run client tests only if client surfaces change:

```bash
pnpm test:clients
pnpm build
```

## Implementation Verification Results

Recorded on 2026-04-30 Asia/Shanghai for branch
`028-hosted-operational-profile`.

- Targeted Go packages: `go test ./internal/opsreadiness ./internal/contracts ./internal/store/migrationfixture ./internal/store/tenancy ./internal/secrets ./internal/billing` from `daemon/` passed.
- Full daemon tests: first `go test ./...` observed a transient `internal/app` SQLite `database is locked` failure in `TestAppRestartPreservesOperatorVisibleSandboxLinkageForCancelledToolCall`; the isolated test passed, and the full rerun passed.
- Module tidy: `go mod tidy` from `daemon/` produced no `go.mod` or `go.sum` diff.
- Contract tests: `make daemon-contract-test` passed.
- Test daemon smoke: `make daemon-run-test` started the test daemon on `127.0.0.1:19192`; `make daemon-test-status` returned `{"ok":true,"service":"dope"}`; the test daemon was stopped after verification.
- Hosted-profile targeted validation: with `DOPE_DATA_DIR=/tmp/dope-hosted-028`, `DOPE_HOSTED_RUN_ID=hosted_validation_028`, and `DOPE_HOSTED_DRY_RUN=1`, `provision`, `start`, `status`, `health`, `evidence-index`, and `stop` passed and produced `/tmp/dope-hosted-028/reports/hosted_validation_028/release-evidence-index.json`.
- Stable-host smoke: `zentalk-1` (`VM-0-7-centos`, uptime 56 days) ran a dry-run hosted profile smoke using `/tmp/hosted-profile-028.sh`; run identity `stable_host_028` produced deployment, health, release-index, stop, and `reboot_recovery` supervisor evidence with `recoverySeconds: 60`.
- Client tests/build: not run; hosted evidence is not exposed through SDK, web, or TUI surfaces in this implementation.

## Post-Review Verification Results

Recorded after fixing implementation gaps found during review:

- Added contract coverage proving hosted `start/status/stop` maintains a real
  supervisor PID, missing release artifacts become `no_ship`, hosted backup and
  restore reject missing three-tenant evidence, hosted preflight records missing
  backup evidence as blocking, hosted postflight records missing representative
  tenants as blocking, hosted soak attributes health failure to `daemon`, and
  unobserved connector/MCP/diagnostic states are marked `unsupported` instead
  of passing by default.
- `go test -count=1 ./internal/contracts ./internal/opsreadiness` passed.
- `go test ./...` from `daemon/` passed.
- `make daemon-contract-test` passed.
- `go mod tidy` from `daemon/` produced no module diff.
- Release-index targeted validation with `DOPE_HOSTED_HEALTH_COMMAND=false`
  produced `decision: no_ship`, failed `health_checks`, and failed missing
  `restore_evidence`.
- Hosted release evidence validation now runs through
  `daemon/cmd/hosted-evidence-validate`; `hosted-profile.sh evidence-index`
  writes `release-evidence-validation.txt`, and the ship-ready contract fixture
  passes the validator in default readiness mode.
- Stable-host supervisor smoke on `zentalk-1` used
  `DOPE_HOSTED_DAEMON_COMMAND="sh -c 'while :; do sleep 1; done'"` and
  `DOPE_HOSTED_HEALTH_COMMAND=true`; run identity `stable_host_028_fixed`
  recorded `process_status=running`, `health=pass`, reboot-recovery evidence,
  and a clean stop.
- Final stable-host supervisor smoke on `zentalk-1` used
  `DOPE_HOSTED_DAEMON_COMMAND="sleep 300"` and `DOPE_HOSTED_HEALTH_COMMAND=true`;
  run identity `stable_host_028_final` recorded `supervisor_pid=2621861`,
  `process_status=running`, `health=pass`, release-index generation,
  reboot-recovery evidence, and a verified stopped process. Because the script
  was copied to `/tmp` outside the repo layout for this smoke,
  `DOPE_HOSTED_SKIP_GO_VALIDATOR=1` was used there; Go validator execution is
  covered by local contract and full daemon verification above.

Residual risk: the stable-host smoke validates the repo-owned supervisor with a
controlled long-running process. It still does not run the full Dope daemon on
the remote host for a 24-hour release soak; that remains a release-readiness
operation, not a local implementation test.
