# Test Environment Workflow

## Purpose

This document defines the repository-default local development workflow for Kura.

The goal is simple:

- local development should not touch production state by default
- operators should have one obvious way to start the daemon in test mode
- future agents should inherit the same default behavior

## Environment Split

### Test

- `DOPE_ENV=test`
- data dir: `~/.dope-test`
- config file: `~/.dope-test/config.json`
- bind addr: `127.0.0.1:19192`

### Production

- `DOPE_ENV=prod`
- data dir: `~/.dope`
- config file: `~/.dope/config.json`
- bind addr: `127.0.0.1:19191`

## Default Rule

Development defaults to `test`.

Production is explicit opt-in.

This applies to:

- daemon debugging
- provider debugging
- connector and IM debugging
- manual local validation
- future agent-assisted debugging in this repository

## Standard Commands

- `make daemon-run-test`
- `make daemon-run-test-live`
- `make daemon-test-status`
- `make daemon-run-prod`
- `make daemon-prod-status`

`make daemon-run` aliases to `make daemon-run-test`.

### Default Safety Behavior

`make daemon-run-test` disables Discord connector startup by default.

This keeps the default debugging path stable even when:

- the local machine has no outbound network
- local proxy settings are broken
- test config contains live Discord credentials

If live IM validation is intentional, use:

- `make daemon-run-test-live`

or explicitly override:

- `DOPE_CONNECTORS_DISCORD_ENABLED=true make daemon-run-test`

## Agent Workflow

The repository-level agent instructions are in:

- `AGENTS.md`

The project-level skill is in:

- `.agents/skills/dope-test-env/SKILL.md`

Both documents encode the same rule:

- use the test environment by default
- do not touch production state unless the user explicitly asks for it
- if the expected daemon port is already occupied by an older Dope daemon, stop it and restart cleanly
- if the listener is not a Dope daemon, treat it as a conflict instead of killing it automatically

## Verification

The environment workflow is considered healthy when:

- the daemon resolves to `test` by default in development
- `make daemon-run-test` starts a daemon bound to `127.0.0.1:19192`
- `make daemon-test-status` succeeds against that port
- API responses expose the active environment correctly

## Why This Exists

Without an explicit test environment workflow, local debugging tends to drift into:

- reusing production config by accident
- sharing managed-provider state unintentionally
- debugging against the wrong port
- future agents repeating ad hoc startup commands

This workflow closes that gap at the repository level instead of relying on operator memory.
