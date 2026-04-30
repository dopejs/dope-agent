# Contract: Observability Report

## Goal

Define hosted operational observations required to diagnose whether failures belong to
daemon behavior, host behavior, network, provider, credential, quota, operator action, an
unsupported observation, or an unknown cause.

## Required Fields

| Field | Required Behavior |
|-------|-------------------|
| `observationReportId` | Unique report identity |
| `runId` | Hosted run identity |
| `sampleWindow` | Observation window |
| `daemonHealth` | Present |
| `databaseSize` | Present or unsupported marker |
| `logSize` | Present or unsupported marker |
| `memory` | Present or unsupported marker |
| `goroutines` | Present or unsupported marker |
| `fileDescriptors` | Present or unsupported marker |
| `queueOrBacklog` | Present or unsupported marker |
| `connectorHealth` | Present or unsupported marker |
| `mcpHealth` | Present or unsupported marker |
| `integrationDiagnosticState` | Present or unsupported marker |
| `unsupportedFields` | Lists unavailable required observations |
| `monotonicResourceGrowth` | Present |
| `failureOwner` | Present for failed, blocked, or ambiguous observations |
| `generatedAt` | Present |

## Failure Owner Classifications

- `daemon`
- `host`
- `network`
- `provider`
- `credential`
- `quota`
- `operator_action`
- `unsupported_observation`
- `unknown`

## Blocking Findings

The report must surface these as release-index no-ship findings:

- daemon health failure
- queue or backlog persisting beyond accepted threshold
- monotonic resource growth over the run
- missing required observation without unsupported marker
- connector health failure
- MCP health failure
- stale or failed integration diagnostic state
- failure owner cannot be classified beyond `unknown` for a blocking event

## Redaction Rules

Observation reports may include health summaries, counts, sizes, states, and redacted
diagnostic classifications. They must not include raw provider payloads, tokens,
authorization headers, app secrets, refresh tokens, or credential-bearing environment
values.

## Contract Tests

- Fixture file: `daemon/internal/opsreadiness/testdata/hosted/observability_report.json`
- Implemented command: `scripts/production/run-soak.sh`
- report fixture includes every required field or unsupported marker
- missing required observation without unsupported marker fails validation
- blocking findings are visible to release evidence index validation
- redaction fixture rejects credential-bearing values
- failure-owner classification accepts only the allowed values
