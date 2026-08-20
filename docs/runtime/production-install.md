# Production Install Runbook

**Scope**: tenant-scoped single-node baseline. Multi-node managed service
rollout is out of scope for Roadmap 39.

**Elapsed-time target**: a clean install must be confirmable in 60 minutes or
less using this runbook and the recorded evidence.

## Prepare

1. Confirm the target host is a single-node daemon host.
2. Confirm the default test environment uses `~/.kura-test` and
   `127.0.0.1:19192`.
3. Confirm live connectors are disabled unless explicitly opted in.
4. Record branch, build version, operator, start time, data directory, and
   daemon address.

## Start

```bash
make daemon-run-test
make daemon-test-status
```

## Verify Health

Record:

- daemon health check result
- data directory
- log location
- config source
- tenant-scoped single-node topology
- elapsed time, which must be 60 minutes or less

## Failure Handling

If health does not pass, stop the daemon, keep logs and state for diagnosis, and
do not retry against `~/.kura` unless this is an explicitly opted-in production
operation.

Cleanup for a failed test install is removal of the test state directory only
after evidence has been captured.
