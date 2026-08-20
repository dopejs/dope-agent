# Contract: Hosted Profile Commands

## Goal

Define the operator command surface for provisioning and supervising the first hosted
operational profile without requiring host-native service managers.

## Profile Defaults

| Field | Required Value |
|-------|----------------|
| Environment | `KURA_ENV=test` by default |
| Data directory | `~/.kura-test` by default |
| Daemon address | `127.0.0.1:19192` by default |
| Live connectors | Disabled unless explicit operator opt-in exists |
| Supervisor mode | Repo-owned foreground supervisor |
| Retention | 90 days unless authorized policy requires longer |

Helpers must refuse production user data or live connector usage unless the current
runbook step explicitly allows it and the operator opts in.

## Command Surface

The hosted profile must provide repository-owned workflows for:

| Command | Required Behavior |
|---------|-------------------|
| provision | Create or verify code, data, log, artifact, backup, report, and temp directories |
| start | Start daemon through the foreground supervisor and write deployment evidence |
| stop | Stop daemon and classify the stop as manual, failed, or unsupported |
| restart | Restart daemon and write supervisor event evidence |
| status | Report process status, profile identity, daemon address, and evidence paths |
| health | Check daemon health and write health evidence |
| evidence-index | Link required evidence and produce ship/no-ship readiness summary |

Command names may follow repository conventions discovered during implementation, but
the behaviors above are mandatory.

## Safety Rules

- Commands print target environment, data directory, daemon address, and artifact root
  before touching state.
- Commands default to test state.
- Commands do not dump raw environment variables.
- Commands do not include tokens, app secrets, refresh tokens, authorization headers, or
  credential-bearing values in output.
- Commands use run-identity paths so evidence from prior runs is not overwritten.

## Contract Tests

- provisioning creates or verifies all required directories
- default command execution targets `~/.kura-test`, not `~/.kura`
- production data access without explicit opt-in exits with a non-zero status
- generated output includes run identity and artifact path
- generated output excludes credential-bearing values
