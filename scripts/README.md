# Scripts

Reserved for repository automation and local development helpers.

## Current Helpers

- `run-daemon.sh test` — start the daemon in the test environment (`~/.dope-test`, `127.0.0.1:19192`) with Discord disabled unless explicitly re-enabled
- `run-daemon.sh prod` — start the daemon in the production environment (`~/.dope`, `127.0.0.1:19191`)
- `check-daemon-health.sh <url>` — poll a daemon health endpoint with short retries
