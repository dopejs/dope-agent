# Contract: Enforcement Matrix

Every guarded entry point must reserve quota before expensive or side-effecting work
starts. Tests must cover allowed, denied, retry, restart, and concurrent launch behavior
where the scenario applies.

| Entry Point | Tenant Context Source | Categories Touched | Reservation Amount | Commit / Refund Transition | Operation Key Source | Quota State Unavailable | Required Tests |
|-------------|-----------------------|--------------------|--------------------|----------------------------|----------------------|-------------------------|----------------|
| `POST /v1/runs` (`handleRuns` before `runtime.CreateRun`) | protected API tenant context | `run_launches` | 1 | Commit when run is persisted; release/refund if request is denied or persistence fails before run creation | client idempotency key when present, otherwise daemon run id generated before reservation | hosted: deny with `quota_denied:quota_state_unavailable`; unlimited local: allow | allowed, denied exhausted, retry same key, persistence failure refund, restart pending recovery, concurrent last unit |
| `POST /v1/runs/{runId}/workflows` (`handleRunWorkflows`) | run tenant guard plus protected API tenant context | `workflow_launches` | 1 | Commit when workflow is persisted as planned; release/refund if planning fails before workflow persistence | `runId` + workflow client key or daemon workflow id | hosted: deny; unlimited local: allow | allowed, denied, retry, planning failure refund, restart pending, concurrent workflow creation |
| `POST /v1/runs/{runId}/workflows/{workflowId}/start` (`handleRunWorkflowStart`) | run/workflow tenant guard plus protected API tenant context | `workflow_launches` when workflow was not already reserved at create time | 1 or 0 when existing reservation is reused | Commit when workflow enters running; release/refund if start fails before execution; deny duplicate start while pending recovery | `runId` + `workflowId` | hosted: deny; unlimited local: allow | allowed, denied, retry existing reservation, start failure refund, restart pending, concurrent start |
| `POST /v1/runs/{runId}/steps/{stepId}/tool-calls` (`handleRunStepToolCalls`) | run/step tenant guard plus protected API tenant context | `runtime_tool_calls`; additional integration categories when tool call invokes integration-backed work | 1 tool call plus domain amounts | Commit when tool call is accepted/running or completed; release/refund if call creation fails before invocation | `runId` + `stepId` + tool call id or client key | hosted: deny; unlimited local: allow | allowed, denied, retry same tool call, tool creation failure refund, restart pending, concurrent tool calls |
| Roadmap 38 live-validation preflight gate contract; any concrete live-validation entry point that already exists before Roadmap 40 | protected API tenant context and live validation tenant selection when a concrete entry point exists | `live_validation_attempts`; integration category when validation uses integration operation | 1 attempt plus integration amount when applicable | Commit once live validation starts; release/refund if preflight fails before live action; not-yet-mounted gate records no external action and does not create a Roadmap 40 executor | validation request id or client key | hosted: deny before live side effects; unlimited local: allow | gate allowed, gate denied, fail-closed unavailable, retry, preflight refund, restart pending, no Roadmap 40 executor created |
| Calendar/mail/integration operation routes | protected API tenant context and integration tenant ownership | `integration_operations`; `artifact_storage_bytes` when operation writes artifacts | 1 operation plus estimated bytes | Commit operation after backend attempt is accepted; commit actual bytes after artifacts written; refund/release on preflight failure before backend or write | domain operation id or client key | hosted: deny before backend call; unlimited local: allow | allowed, denied, retry, backend preflight refund, artifact estimate reconciliation, concurrent operations |
| Computer-use and other artifact write service | artifact owner tenant from active run/session context | `artifact_storage_bytes` | defensible byte estimate | Commit actual bytes after write; refund smaller actuals; commit actual-larger over-limit usage with audit-visible evidence and deny future quota-consuming work until usage is within limit; release on write failure before consumption | artifact id, storage key, or client key | hosted: deny before write; unlimited local: allow | estimate allowed, estimate denied, actual smaller refund, actual-larger over-limit commit, future denial after over-limit commit, write failure release, restart pending |
| `POST /v1/evaluation/replay-candidates/{candidateId}/attempts` and evaluation campaign attempt creation | protected API tenant context and candidate tenant ownership | `replay_evaluation_attempts`; `run_launches`/`workflow_launches` if replay records runtime work | 1 attempt plus runtime categories when runtime work is created | Commit when attempt is accepted or runtime replay is recorded; release/refund if candidate is rejected before attempt consumption | `candidateId` + attempt id or client key | hosted: deny before replay/campaign work; unlimited local: allow | allowed, denied, retry, unreplayable preflight refund, restart pending, concurrent attempts |

Exhausted quota denials are lifecycle outcomes for the operation key. Implementations
should persist the denied reservation before returning the denial so retries do not append
duplicate evidence or perform work.

## Matrix Completeness Rule

Implementation is incomplete if a new hosted entry point can:

- start a run or workflow,
- invoke a runtime tool,
- perform live validation,
- call an integration/backend,
- write persisted artifacts or storage,
- create replay/evaluation attempts,
- or otherwise consume expensive or side-effecting resources

without either a row in this matrix or an explicit out-of-scope justification in the plan.
