# Contract: Initial Quota Catalog

This catalog is required before implementation. Every category must have a stable
identifier, explicit unit, UTC period semantics, carryover behavior, lifecycle points,
operation identity, concurrency guard, stable denial shape, and required tests.

## Common Rules

- Period boundaries are UTC.
- Hosted tenants fail closed when quota state is unavailable.
- Local development/unlimited plans project `enforcementMode = unlimited` and allow work.
- Multi-category operations reserve all required categories atomically or deny the whole
  operation without consuming resources.
- Stable denial reason codes use the prefix `quota_denied:`.
- Exhausted quota denials also persist a denied reservation for the operation key so
  client retries replay the same lifecycle outcome without duplicating denial evidence.

## Categories

| Category | Unit | Period | Carryover | Reservation Point | Commit Point | Refund Point | Operation Key | Concurrency Guard | Denial Shape | Required Tests |
|----------|------|--------|-----------|-------------------|--------------|--------------|---------------|-------------------|--------------|----------------|
| `run_launches` | count | monthly UTC | none | `POST /v1/runs` before `runtime.CreateRun` | run persisted | route denial or failure before run persisted | `tenant:{tenantId}:run:{clientKey|runId}` | one transaction over tenant/category/period counter and reservation row | `quota_denied:run_launches_exhausted` | allowed, denied, retry same key, restart pending, concurrent last unit |
| `workflow_launches` | count | monthly UTC | none | `POST /v1/runs/{runId}/workflows` and `POST /v1/runs/{runId}/workflows/{workflowId}/start` before workflow execution starts | workflow reaches planned/running state | planning/start denial, cancellation before execution, failed start before side effects | `tenant:{tenantId}:workflow:{runId}:{workflowId|clientKey}` | one transaction over tenant/category/period counter and reservation row | `quota_denied:workflow_launches_exhausted` | allowed, denied, retry, restart pending, concurrent start |
| `runtime_tool_calls` | count | daily UTC | none | `POST /v1/runs/{runId}/steps/{stepId}/tool-calls` before tool call creation or external tool invocation | tool call transitions to running or completed after accepted invocation | denial, failed creation, cancellation before invocation | `tenant:{tenantId}:tool_call:{runId}:{stepId}:{toolCallId|clientKey}` | one transaction over tenant/category/period counter and reservation row | `quota_denied:runtime_tool_calls_exhausted` | allowed, denied, retry same tool call, restart pending, concurrent tool calls |
| `live_validation_attempts` | attempts | daily UTC | none | Roadmap 38 reusable live-validation preflight gate, and any concrete live-validation entry point that exists before Roadmap 40 | validation starts or records attempted live action; not-yet-mounted gate records no external action | denial, unsafe preflight failure before live action | `tenant:{tenantId}:live_validation:{validationId|clientKey}` | one transaction over tenant/category/period counter and reservation row | `quota_denied:live_validation_attempts_exhausted` | gate allowed, gate denied, fail-closed unavailable, retry, restart pending, no Roadmap 40 executor created |
| `integration_operations` | count | monthly UTC | optional per plan, capped | calendar/mail/integration operation handlers before external or fake backend operation | operation record persisted after backend attempt | denial, failed preflight before backend attempt, cancellation before backend attempt | `tenant:{tenantId}:integration:{domain}:{operationId|clientKey}` | one transaction over tenant/category/period counter and reservation row | `quota_denied:integration_operations_exhausted` | allowed, denied, retry, restart pending, concurrent operations |
| `artifact_storage_bytes` | bytes | monthly UTC | optional per plan, capped | artifact write service before writing bytes, using defensible estimate | actual bytes known after write; actual larger than estimate is committed even if it places tenant over quota | write failure before consumption, estimate reconciliation, actual smaller refund | `tenant:{tenantId}:artifact:{artifactId|storageKey|clientKey}` | one transaction over tenant/category/period counter and reservation row; reconcile actual bytes in follow-up transaction tied to same operation key; over-limit actual commits deny future quota-consuming work until usage is within limit | `quota_denied:artifact_storage_bytes_exhausted` | estimate allowed, estimate denied, actual smaller refund, actual larger over-limit commit, future denial after over-limit commit, retry, restart pending |
| `replay_evaluation_attempts` | attempts | monthly UTC | none | replay/evaluation attempt creation before runtime recorder or campaign attempt starts | attempt persisted as started/completed/unreplayable after accepted attempt | denial, preflight unreplayable before attempt consumption, cancellation before side effects | `tenant:{tenantId}:evaluation:{candidateId}:{attemptId|clientKey}` | one transaction over tenant/category/period counter and reservation row | `quota_denied:replay_evaluation_attempts_exhausted` | allowed, denied, retry, restart pending, concurrent attempt |

## Stable Denial Payload

Every denial response or event must expose:

```json
{
  "code": "quota_denied",
  "reasonCode": "quota_denied:<category>_exhausted",
  "tenantId": "ten_example",
  "category": "run_launches",
  "operationKey": "tenant:ten_example:run:req_123",
  "periodStart": "2026-04-01T00:00:00Z",
  "periodEnd": "2026-05-01T00:00:00Z",
  "requestedAmount": 1,
  "remainingAmount": 0,
  "message": "Quota exhausted for run launches."
}
```

Hosted quota-state-unavailable uses:

```json
{
  "code": "quota_denied",
  "reasonCode": "quota_denied:quota_state_unavailable",
  "tenantId": "ten_example",
  "operationKey": "tenant:ten_example:run:req_123",
  "message": "Quota state is unavailable; hosted work cannot start."
}
```
