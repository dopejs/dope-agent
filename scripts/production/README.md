# Production Script Helpers

These helpers are operator-facing wrappers for the Roadmap 39 tenant-scoped
single-node baseline. They default to the test daemon environment:

- data directory: `~/.kura-test`
- daemon address: `127.0.0.1:19192`
- live connectors: disabled unless an operator explicitly opts in

Before use, verify scripts are executable:

```bash
test -x scripts/production/hosted-profile.sh
test -x scripts/production/upgrade-preflight.sh
test -x scripts/production/upgrade-postflight.sh
test -x scripts/production/backup-test-state.sh
test -x scripts/production/restore-test-state.sh
test -x scripts/production/run-soak.sh
test -x scripts/production/restart-test-daemon.sh
```

Run helpers from the repository root. Every helper prints the environment and
target path before doing work so an operator can stop before touching live state.
Do not point these helpers at `~/.kura` unless the runbook explicitly says the
current step is a production operation.

`hosted-profile.sh` owns the Roadmap 43 hosted profile command surface:
`provision`, `start`, `stop`, `restart`, `status`, `health`, and
`evidence-index`. These commands default to `~/.kura-test` and write
run-identity evidence under the hosted artifact root. `evidence-index` runs
`daemon/cmd/hosted-evidence-validate` after writing the release index and records
the validator output next to the index.
