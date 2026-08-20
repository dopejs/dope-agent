# Production Upgrade Runbook

**Scope**: tenant-scoped single-node upgrade. Multi-node managed service
rollout, clustering, and distributed failover are outside Roadmap 39.

**Elapsed-time target**: representative upgrade verification must complete in
90 minutes or less, including preflight and postflight evidence.

## Preflight

1. Stop write traffic or the test daemon before copying state.
2. Take a verified backup using `scripts/production/backup-test-state.sh`.
3. Run:

   ```bash
   scripts/production/upgrade-preflight.sh
   ```

4. Record tenant integrity, required config, quota/accounting state, schema
   version, backup integrity, hosted deployment identity, data location,
   artifact location, daemon health, configuration readiness, and blocking
   findings.

## Upgrade

Install the target build, start the daemon in the test environment, and keep
migration lifecycle logs.

## Postflight

Run:

```bash
scripts/production/upgrade-postflight.sh
```

Record tenant data counts, quota/accounting consistency, credential remediation
state, health checks, operational diagnostics, rollback guidance, and elapsed
time. When `KURA_HOSTED_RUN_ID` is set, the preflight and postflight helpers
write `upgrade-preflight.json` and `upgrade-postflight.json` under the hosted
run artifact directory.

## Rollback Decision

In-place rollback is safe only when persisted state remains compatible with the
previous binary. When migrations changed persisted state in a way that cannot be
reversed safely, restore from backup is the only acceptable recovery path. Old
binaries must not be pointed at incompatible newer schema versions.
