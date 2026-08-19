---
name: dope-test-env
description: Use when working on Kura locally and you need to start, inspect, debug, or validate the daemon without touching production state. Default to the test environment at ~/.dope-test and port 19192 unless the user explicitly requests production.
---

# Kura Test Environment

Use this skill for local development and debugging in this repository.

## Core Rule

Default to the **test environment**.

- Environment: `test`
- Data dir: `~/.dope-test`
- Config file: `~/.dope-test/config.json`
- Bind addr: `127.0.0.1:19192`

Only switch to production when the user explicitly asks for production validation.

## Standard Commands

- Start test daemon:
  - `make daemon-run-test`
- Start test daemon with live Discord connector behavior:
  - `make daemon-run-test-live`
- Check test daemon health:
  - `make daemon-test-status`
- Start production daemon:
  - `make daemon-run-prod`
- Check production daemon health:
  - `make daemon-prod-status`

## Debugging Workflow

1. Before starting, check whether the target port is already occupied.
2. If the listener is an existing Dope daemon for the same environment, stop it first and then restart.
3. If the listener is a different process, do not kill it automatically; surface the conflict and get explicit user direction.
4. Use `make daemon-run-test`.
5. Confirm health with `make daemon-test-status`.
6. Use `~/.dope-test/config.json` for connector and provider edits.
7. Leave `~/.dope/config.json` alone unless the user explicitly asked for production work.

`make daemon-run-test` is the safe default and disables Discord connector startup unless you explicitly opt back in.

### Port-Conflict Rule

For the test environment, the expected port is `19192`.

When `19192` is already occupied:

- If it is a Dope daemon, stop the old listener and restart cleanly.
- If it is not a Dope daemon, treat it as a real conflict and do not kill it without user approval.

Useful checks:

- `lsof -iTCP:19192 -sTCP:LISTEN -n -P`
- `ps -fp <pid>`

The same rule applies to production on `19191`.

## When This Skill Must Be Used

- daemon debugging
- provider debugging
- connector or IM channel debugging
- local manual validation
- reproducing bugs against a running daemon

## Do Not Do This By Default

- do not start from `~/.dope`
- do not bind to the production port
- do not reuse production tokens or managed-provider state unless the user explicitly approves that path
