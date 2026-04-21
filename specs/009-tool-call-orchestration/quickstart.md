# Quickstart: Tool-Call Orchestration

## Goal

Verify in `DOPE_ENV=test` that the daemon can:

- create an inspectable workflow plan from an operator-provided goal
- execute that plan through the existing runtime step and tool-call plane
- preserve step-scoped approval and provenance truth
- run at least one mixed workflow spanning two consumer families
- preserve workflow audit truth when cancellation, partial failure, or restart
  interruption occurs

## Prerequisites

- local test daemon only; do not use `~/.dope`
- authenticated local pairing or an existing bearer token
- one MCP server available through existing daemon-owned MCP surfaces
- at least one local-tool capability or executable skill available in the local daemon
- any secret-bearing or approval-requiring step remains configured through the existing
  phase-19 or phase-21 surfaces, not through workflow-only shortcuts

## Suggested Verification Flow

1. Start the daemon in the test environment.

```bash
make daemon-run-test
```

2. Ensure one MCP server and one additional consumer family are available for mixed
workflow verification.

Examples after implementation:

- reuse the phase-23 websocket MCP helper and register an MCP server through
  `/v1/mcp/servers`
- keep one built-in local capability or one repo-owned executable skill fixture available
  for the second consumer family

3. Create a run with a workflow-oriented goal.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "entrypoint":"operator",
    "goal":"Use one MCP tool and one local tool or executable skill to complete a deterministic verification workflow."
  }' \
  http://127.0.0.1:19192/v1/runs
```

Expected outcome after implementation:

- a normal run resource is created using existing run routes
- no workflow starts yet

4. Ask the daemon to plan a workflow for that run.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}' \
  http://127.0.0.1:19192/v1/runs/$RUN_ID/workflows
```

Expected outcome after implementation:

- the response is a persisted workflow resource
- the workflow is inspectable before execution
- the plan shows selected steps, dependency order, handoff intent, and expected approval
  mode for each step

5. Inspect the planned workflow before execution.

```bash
curl -sS -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/runs/$RUN_ID/workflows/$WORKFLOW_ID
```

Expected outcome after implementation:

- workflow status is `planned`
- each step includes consumer family, selected tool, dependency truth, and approval
  expectation
- no runtime step or tool call has been created yet

6. Start workflow execution.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/runs/$RUN_ID/workflows/$WORKFLOW_ID/start
```

Expected outcome after implementation:

- the workflow transitions to `running`
- each executing workflow step creates a normal runtime step and normal tool call records
- workflow inspection exposes linkage back to runtime steps and tool calls

7. Inspect runtime and workflow history during and after execution.

```bash
curl -sS -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/runs/$RUN_ID/steps

curl -sS -H "Authorization: Bearer $DOPE_TOKEN" \
  http://127.0.0.1:19192/v1/runs/$RUN_ID/workflows/$WORKFLOW_ID
```

Expected outcome after implementation:

- workflow status and step status are consistent with concrete runtime step and tool-call
  truth
- mixed-family execution remains visible without any out-of-band execution path
- retries, blocked steps, or partial-failure outcomes are visible in workflow history

8. Verify restart interruption truth with the automated daemon-restore regression.

Expected outcome after implementation:

- if the daemon restarts while a workflow is running, completed workflow steps remain
  visible
- the workflow becomes `interrupted`
- the daemon does not silently auto-resume unfinished workflow execution in phase 24

## Automated Verification

Run targeted suites plus contract coverage:

```bash
cd daemon && go test ./internal/orchestration ./internal/api ./internal/runtime ./internal/store ./internal/app ./internal/contracts
make daemon-contract-test
cd daemon && go test ./...
```

Expected automated coverage after implementation:

- planning succeeds or fails with explicit workflow status
- start, cancel, blocked, retry, partial-failure, and interruption transitions persist
  correctly
- workflow linkage on runtime steps and tool calls is schema-backed and contract-tested
- mixed-family workflows preserve approval, sandbox, provenance, and redaction semantics
- single-step tool-call flows continue to pass without creating workflow resources

## Recorded Results

Recorded in `DOPE_ENV=test` on `2026-04-21`.

Manual acceptance:

- paired against the test daemon and installed a repo-owned stdio MCP helper as
  `filesystem-manual`
- observed inspect-before-start planning in `planned` state with no runtime linkage before
  execution
- measured workflow planning latency at about `0.04 s`, well inside the `<=5 min`
  acceptance bound
- observed a blocked mixed workflow before MCP tool allowlisting:
  - plan shape: `mcp_tool -> skill`
  - blocked truth stayed explicit on the MCP step
  - downstream skill step remained `waiting_dependency`
- allowlisted `lookup` on runtime surface `chat`, restarted the helper, and observed one
  successful mixed MCP plus executable-skill workflow end to end:
  - final workflow status `completed`
  - both workflow steps `completed`
  - handoff status advanced from `pending` to `consumed`
- executed one direct non-workflow `skill` tool call on a manually created runtime step
  and observed `completed` status with no `workflowId`, confirming the legacy path stayed
  unaffected

Automated verification:

- `make daemon-contract-test` passed
- `cd daemon && go test ./internal/orchestration ./internal/api ./internal/runtime ./internal/store ./internal/app ./internal/contracts` passed
- `cd daemon && go test ./...` passed
- restart interruption truth is covered by
  `TestRecoverPersistedStateInterruptsInFlightWorkflows`
  in `daemon/internal/app/app_test.go`

## Notes

- Keep all verification in `DOPE_ENV=test`.
- A mixed-workflow verification fixture should be deterministic and repo-owned where
  possible so roadmap closure does not depend on a third-party service.
- Manual verification should prove inspect-before-start behavior, not just successful
  execution.
