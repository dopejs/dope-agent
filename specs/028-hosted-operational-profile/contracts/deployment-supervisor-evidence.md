# Contract: Deployment And Supervisor Evidence

## Goal

Define the deployment manifest and supervisor event evidence required for hosted daemon
operation.

## Deployment Manifest Fields

| Field | Required |
|-------|----------|
| `manifestId` | yes |
| `runId` | yes |
| `commitOrVersion` | yes |
| `branch` | yes |
| `host` | yes |
| `operator` | yes |
| `startedAt` | yes |
| `configurationProfile` | yes |
| `dataDirectory` | yes |
| `artifactDirectory` | yes |
| `supervisorMode` | yes, `repo_foreground` for this phase |
| `daemonAddress` | yes |
| `liveConnectorMode` | yes |
| `redactionStatus` | yes |
| `retentionExpiresAt` | yes |

## Supervisor Event Fields

| Field | Required |
|-------|----------|
| `eventId` | yes |
| `runId` | yes |
| `eventType` | yes |
| `requestedBy` | yes when operator initiated |
| `startedAt` | yes |
| `completedAt` | yes when complete |
| `daemonHealth` | yes |
| `recoverySeconds` | yes for crash or reboot recovery |
| `result` | yes |
| `failureOwner` | yes for failed, blocked, or unsupported outcomes |
| `evidencePath` | yes |

## Event Types

- `start`
- `stop`
- `restart`
- `status`
- `health_check`
- `crash_detected`
- `reboot_recovery`
- `manual_stop`
- `failed_restart`
- `repeated_crash`

## Recovery Rules

- Crash or reboot recovery must return daemon health within 5 minutes.
- Recovery over 5 minutes is `failed`.
- Manual stop must not be treated as crash recovery.
- Repeated crash must be visible as `failed_restart` or `operator_action_needed`.

## Redaction Rules

Deployment and supervisor evidence must not include raw environment dumps, tokens, app
secrets, refresh tokens, authorization headers, provider credentials, or local CLI auth
material.

## Contract Tests

- manifest generation includes every required field
- crash recovery over 5 minutes fails the supervisor event
- manual stop is classified separately from crash recovery
- repeated crash does not hide process-loop failure
- manifest and event fixtures pass redaction checks
