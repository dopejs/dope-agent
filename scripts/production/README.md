# Production Script Helpers

These helpers are operator-facing wrappers for the Roadmap 39 tenant-scoped
single-node baseline. They default to the test daemon environment:

- data directory: `~/.dope-test`
- daemon address: `127.0.0.1:19192`
- live connectors: disabled unless an operator explicitly opts in

Before use, verify scripts are executable:

```bash
test -x scripts/production/upgrade-preflight.sh
test -x scripts/production/upgrade-postflight.sh
test -x scripts/production/backup-test-state.sh
test -x scripts/production/restore-test-state.sh
test -x scripts/production/run-soak.sh
test -x scripts/production/restart-test-daemon.sh
```

Run helpers from the repository root. Every helper prints the environment and
target path before doing work so an operator can stop before touching live state.
Do not point these helpers at `~/.dope` unless the runbook explicitly says the
current step is a production operation.
