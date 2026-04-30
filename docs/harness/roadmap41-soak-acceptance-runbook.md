# Roadmap 41 Soak Acceptance Runbook

Use this runbook when asked to validate the Roadmap 39 24-hour soak rerun for
Roadmap 41 evaluation product expansion. Do not rely on prior chat context.

## Context

- VPS SSH alias: `zentalk-1`
- Deployment directory: `/root/dope-agent-r41-5ad95ba`
- Commit under test: `5ad95ba`
- Daemon data directory: `/root/.dope-r41-soak`
- Artifact directory: `/root/dope-agent-r41-artifacts`
- Full soak report: `/root/dope-agent-r41-artifacts/roadmap39-full-5ad95ba.json`
- Full soak log: `/root/dope-agent-r41-artifacts/roadmap39-full-5ad95ba.log`
- Daemon log: `/root/dope-agent-r41-artifacts/daemon-5ad95ba.log`
- Expected start window: 2026-04-30 09:49-09:50 Asia/Shanghai
- Expected earliest acceptance time: 2026-05-01 10:00 Asia/Shanghai

## What Is Running

The daemon and 24-hour soak were started on `zentalk-1` with:

```bash
DOPE_DATA_DIR=/root/.dope-r41-soak
DOPE_SOAK_BRANCH_OR_VERSION=5ad95ba
DOPE_SOAK_REPORT=/root/dope-agent-r41-artifacts/roadmap39-full-5ad95ba.json
```

Recorded PID files:

```bash
/root/dope-agent-r41-artifacts/daemon-5ad95ba.pid
/root/dope-agent-r41-artifacts/roadmap39-full-5ad95ba.pid
```

## Acceptance Query

When validating, run these checks from the local machine:

```bash
ssh -o BatchMode=yes zentalk-1 'date -u +%Y-%m-%dT%H:%M:%SZ && date +%Y-%m-%dT%H:%M:%S%z'
ssh -o BatchMode=yes zentalk-1 'ps -eo pid,ppid,comm,args | grep -E "run-soak|daemon-run-test|cmd/dope|/tmp/go-build" | grep -v grep || true'
ssh -o BatchMode=yes zentalk-1 'curl -fsS http://127.0.0.1:19192/healthz || true'
ssh -o BatchMode=yes zentalk-1 'tail -80 /root/dope-agent-r41-artifacts/roadmap39-full-5ad95ba.log'
ssh -o BatchMode=yes zentalk-1 'test -f /root/dope-agent-r41-artifacts/roadmap39-full-5ad95ba.json && cat /root/dope-agent-r41-artifacts/roadmap39-full-5ad95ba.json'
ssh -o BatchMode=yes zentalk-1 'tail -120 /root/dope-agent-r41-artifacts/daemon-5ad95ba.log'
```

## Pass Criteria

The final report must exist and meet all of these conditions:

- `branchOrVersion == "5ad95ba"`
- `environment == "test"`
- `dataDirectory == "/root/.dope-r41-soak"`
- `durationHours >= 24`
- `elapsedSeconds >= 86400`
- `temporaryShorterDuration == false`
- `followUpFullRerun == false`
- `daemonHealth == "pass"`
- `finalResult == "pass"`
- `crossTenantLeakage == false`
- `monotonicResourceGrowth == false`
- `unclassifiedFailures == []`
- `restartCount >= 3`
- `workloadCoverage` includes runtime, scheduler, integrations, delivery, approvals,
  quotas, tenant switching, and evaluation
- `faultDrills` includes transient 5xx, rate limit, auth expiry, provider unavailable,
  slow response, and malformed response
- `resourceObservations` includes logs, stored data size, queue/backlog, memory,
  open handles or file descriptors, and goroutines

## Failure Handling

If the report is missing, the soak process is still running, or the elapsed duration is
less than 24 hours, do not mark Roadmap 41 complete. Record the current status and
wait until the run finishes.

If the report exists but any pass criterion fails, do not mark T153 complete. Preserve
the report and logs, summarize the failing fields, and keep Roadmap 41 in
`implementation complete; final soak evidence pending` state.

If all criteria pass, update:

- `specs/026-evaluation-product-expansion/quickstart.md`
- `specs/026-evaluation-product-expansion/tasks.md` to mark T153 `[X]`
- `docs/runtime/daemon-roadmaps.md`
- `docs/specs/026-evaluation-product-expansion.md`

Then run focused verification and commit the evidence update.
