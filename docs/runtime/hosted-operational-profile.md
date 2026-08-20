# Hosted Operational Profile

Roadmap 43 defines a hosted/test-host operational profile for long-lived daemon
operation on a stable always-on test host or VPS. The profile is additive to the
Roadmap 39 production operations baseline and defaults to the test environment.

## Defaults

- Environment: `KURA_ENV=test`
- Data directory: `~/.kura-test`
- Daemon address: `127.0.0.1:19192`
- Supervisor mode: repo-owned foreground supervisor
- Live connectors: disabled unless an operator explicitly opts in
- Evidence retention: 90 days unless an authorized policy requires longer

The hosted profile must not use `~/.kura` or production user data unless the
current runbook step explicitly permits production recovery and the operator
sets `KURA_LIVE_OPT_IN=yes`.

## Directory Layout

`scripts/production/hosted-profile.sh provision` creates or verifies:

- data: `KURA_DATA_DIR` or `~/.kura-test`
- logs: `KURA_HOSTED_LOG_DIR` or `$KURA_DATA_DIR/logs`
- artifacts: `KURA_HOSTED_ARTIFACT_DIR/$KURA_HOSTED_RUN_ID`
- backups: `KURA_HOSTED_BACKUP_DIR` or `$KURA_DATA_DIR/backups`
- reports: `KURA_HOSTED_REPORT_DIR/$KURA_HOSTED_RUN_ID`
- temporary work: `KURA_HOSTED_TMP_DIR` or `$KURA_DATA_DIR/tmp`

Run identities prevent evidence from prior runs being overwritten. Use
`KURA_HOSTED_RUN_ID` to pin a reviewed run and `KURA_HOSTED_COMMIT` to pin the
reviewed commit or version.

## Commands

Run from the repository root:

```bash
scripts/production/hosted-profile.sh provision
scripts/production/hosted-profile.sh start
scripts/production/hosted-profile.sh status
scripts/production/hosted-profile.sh health
scripts/production/hosted-profile.sh restart
scripts/production/hosted-profile.sh reboot-recovery
scripts/production/hosted-profile.sh stop
scripts/production/hosted-profile.sh evidence-index
```

`start`, `restart`, `stop`, and `health` write supervisor or health evidence.
For this phase the supervisor mode is `repo_foreground`; the hosted wrapper
launches the repo-owned daemon command, records the supervisor PID and log path,
checks daemon health, and classifies failed recovery evidence. Clean hosted
provisioning must reach daemon health in 60 minutes or less. Crash or reboot
recovery must return daemon health in 5 minutes or less, or the supervisor
evidence records a failed recovery.

## Release Evidence Index

`scripts/production/hosted-profile.sh evidence-index` writes:

```text
$KURA_HOSTED_REPORT_DIR/$KURA_HOSTED_RUN_ID/release-evidence-index.json
```

The index links deployment manifest, configuration profile, health checks,
logs, soak report, backup evidence, restore evidence, upgrade preflight,
upgrade postflight, rollback decision, integration diagnostics, resource
observations, redaction check, and retention metadata. The generator inspects
linked paths and health/redaction states before assigning pass/fail status, then
runs `hosted-evidence-validate` (not yet ported to Rust) and records
`release-evidence-validation.txt` next to the index. All linked evidence must
match commit, profile, and run identity. Missing, stale, mismatched, failed,
expired, or secret-exposing evidence is a no-ship condition.

Use the validator in default mode for a ship-ready review:

```bash
RELEASE_INDEX_PATH=/path/printed/by/release_evidence_index
# hosted-evidence-validate is not yet ported to Rust
```
