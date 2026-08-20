# Quickstart: Computer-Use Capability Plane

## Goal

Verify in `KURA_ENV=test` that the daemon can:

- create a run-scoped browser session
- execute low-risk browser actions without approval and high-risk actions with explicit
  approval
- capture screenshots or snapshots as durable artifacts
- fail target mismatch immediately with preserved evidence
- keep computer-use truth linked to normal runtime tool calls and workflow steps

## Prerequisites

- local test daemon only; do not use `~/.kura`
- authenticated local pairing or an existing bearer token
- no production connectors or production secrets are required
- a deterministic browser fixture page that is safe in `KURA_ENV=test`

Recommended fixture:

- use a deterministic local HTML fixture served from localhost or a safe `data:` URL that
  exposes one input, one button, and one stable heading for target matching

## Suggested Verification Flow

1. Start the daemon in the test environment.

```bash
make daemon-run-test
```

2. Create a normal run that will own the browser session.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "entrypoint": "operator",
    "goal": "Verify browser-first computer-use session and action truth in test."
  }' \
  http://127.0.0.1:19192/v1/runs
```

Expected outcome after implementation:

- the response returns a normal run resource
- the run is the owner for any later computer-use sessions and tool calls

3. Create a browser session under the run.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "driverKind": "browser"
  }' \
  http://127.0.0.1:19192/v1/runs/$RUN_ID/computer-use/sessions
```

Expected outcome after implementation:

- the response returns a session with `status` `active`
- the session is linked to the owning `runId`
- no cross-run or cross-schedule reuse is possible

4. Navigate to the deterministic fixture page and inspect the resulting action.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"actionKind\": \"navigate\",
    \"url\": \"$FIXTURE_URL\"
  }" \
  http://127.0.0.1:19192/v1/runs/$RUN_ID/computer-use/sessions/$SESSION_ID/actions
```

Expected outcome after implementation:

- the action completes without approval if policy classifies it as lower-risk
- the action creates normal runtime step and tool-call truth linked to the session and
  action IDs
- session detail shows the current page summary

5. Request a high-risk action that submits input or leaves the current trusted scope.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "actionKind": "input",
    "targetMatchContext": {
      "matchStrategy": "dom_selector",
      "expectedSelector": "#name"
    },
    "value": "Phase 26 test input"
  }' \
  http://127.0.0.1:19192/v1/runs/$RUN_ID/computer-use/sessions/$SESSION_ID/actions
```

Expected outcome after implementation:

- the action enters `waiting_approval`
- the response or linked policy record exposes the `approvalId`
- the operator can inspect the current page and target context before approving

6. Resolve the approval and confirm the action completes with evidence.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"resolution":"approved","comment":"phase 26 verification"}' \
  http://127.0.0.1:19192/v1/policy/approvals/$APPROVAL_ID/resolve
```

Expected outcome after implementation:

- the action transitions to `completed`
- session or action detail exposes at least one linked screenshot or snapshot artifact

7. Exercise a target-mismatch failure.

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "actionKind": "click",
    "targetMatchContext": {
      "matchStrategy": "dom_selector",
      "expectedSelector": "#missing-button"
    }
  }' \
  http://127.0.0.1:19192/v1/runs/$RUN_ID/computer-use/sessions/$SESSION_ID/actions
```

Expected outcome after implementation:

- the action fails with `failureClass` `target_mismatch`
- the action preserves the latest evidence
- no automatic retry or fallback target selection occurs

8. Verify artifact inspection and content retrieval.

```bash
curl -sS -H "Authorization: Bearer $KURA_TOKEN" \
  http://127.0.0.1:19192/v1/computer-use/artifacts/$ARTIFACT_ID
```

Expected outcome after implementation:

- artifact metadata remains inspectable after the action completes
- content download succeeds when capture status is `available`

9. Validate workflow integration with automated coverage.

Expected outcome after implementation:

- at least one `KURA_ENV=test` workflow combines computer use with another capability
- computer-use actions remain visible through workflow step and tool-call truth

## Automated Verification

Run targeted suites plus contract coverage:

```bash
cd daemon && go test ./internal/app ./internal/contracts ./internal/api ./internal/computeruse ./internal/store ./internal/orchestration
make daemon-contract-test
cd daemon && go test ./internal/api -run 'TestScheduleRoutesDispatchWorkflowTargetAndLinkWorkflowTruth|TestScheduleWorkflowComputerUseDoesNotReuseOperatorRunSession|TestWorkflowStartExecutesComputerUseStepAndProjectsEvidence|TestComputerUseSessionAndApprovalRoutes|TestComputerUseRoutesFilterArtifactsByEnvironmentAndExposeTargetMismatch' -count=1
```

Expected automated coverage after implementation:

- session creation, reuse within one run, and explicit close behavior
- core action matrix for navigate, back, forward, wait, screenshot, snapshot, select,
  download, and close-session behavior in the deterministic browser-first driver
- risk-based approval gating for high-risk actions
- artifact capture and restart-safe inspection
- immediate target-mismatch failure with no auto-relocation
- unavailable-consumer and interrupted-restart outcomes
- single-page session enforcement with explicit failure on extra tabs or windows
- additive computer-use linkage on runtime tool-call truth and workflow-visible execution

## Observed Manual Verification

Manual verification was completed in `KURA_ENV=test` against the local daemon on
`127.0.0.1:19192`.

Commands used:

```bash
make daemon-run-test
make daemon-test-status
curl -sS -X POST http://127.0.0.1:19192/v1/auth/pairings/start
curl -sS -X POST http://127.0.0.1:19192/v1/auth/pairings/$PAIRING_ID/complete
curl -sS -X POST -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"entrypoint":"operator","goal":"manual computer-use verification"}' \
  http://127.0.0.1:19192/v1/runs
curl -sS -X POST -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"driverKind":"browser"}' \
  http://127.0.0.1:19192/v1/runs/$RUN_ID/computer-use/sessions
curl -sS -X POST -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"actionKind":"snapshot"}' \
  http://127.0.0.1:19192/v1/runs/$RUN_ID/computer-use/sessions/$SESSION_ID/actions
curl -sS -X POST -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"actionKind":"input","value":"Phase 26 test input","targetMatchContext":{"matchStrategy":"dom_selector","expectedSelector":"#name"}}' \
  http://127.0.0.1:19192/v1/runs/$RUN_ID/computer-use/sessions/$SESSION_ID/actions
curl -sS -X POST -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"resolution":"approved","comment":"phase 26 verification"}' \
  http://127.0.0.1:19192/v1/policy/approvals/$APPROVAL_ID/resolve
curl -sS -X POST -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"actionKind":"click","targetMatchContext":{"matchStrategy":"dom_selector","expectedSelector":"#missing-button"}}' \
  http://127.0.0.1:19192/v1/runs/$RUN_ID/computer-use/sessions/$SESSION_ID/actions
curl -sS -X POST -H "Authorization: Bearer $KURA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"resolution":"approved","comment":"phase 26 mismatch verification"}' \
  http://127.0.0.1:19192/v1/policy/approvals/$MISMATCH_APPROVAL_ID/resolve
curl -sS -H "Authorization: Bearer $KURA_TOKEN" \
  http://127.0.0.1:19192/v1/computer-use/artifacts/$ARTIFACT_ID
curl -sS -H "Authorization: Bearer $KURA_TOKEN" \
  http://127.0.0.1:19192/v1/computer-use/artifacts/$ARTIFACT_ID/content
curl -sS -H "Authorization: Bearer $KURA_TOKEN" \
  "http://127.0.0.1:19192/v1/events?runId=$RUN_ID"
```

Observed results:

- pairing completed successfully through `pair_bd006b6dc5030df3`
- manual run `run_dfa00005d9412f59` was created and used as the owning run
- browser session `cusess_825539a53aa874bc` was created successfully
- low-risk snapshot action `cuact_35c37fbbfcff9066` completed and recorded artifact
  `cuart_ed4b8f703cdb73fe`
- approval-gated input action `cuact_ad217f269395fd67` resumed from approval
  `approval_ebe6411f27423da3`, completed, and recorded artifact
  `cuart_3e98aec5468885a4`
- approval-gated click action `cuact_fc086c4ec4a53aeb` resumed from approval
  `approval_1602e87b7cfe0fd7`, failed with `failureClass=target_mismatch`, and recorded
  artifact `cuart_e652dd1db93e7da2`
- artifact metadata and content retrieval both succeeded for `cuart_3e98aec5468885a4`
- event history for `run_dfa00005d9412f59` included `computer_use.session_created`,
  `computer_use.artifact_recorded`, `computer_use.action_requested`,
  `computer_use.action_status_changed`, and `computer_use.action_target_mismatch`

## Notes

- Keep all verification in `KURA_ENV=test`.
- Prefer deterministic local fixture pages over public websites.
- Manual verification should prove inspect-before-act behavior, not only successful browser
  automation.
