# Contract: Replay Support Matrix

Implementation planning and runtime readiness MUST maintain an explicit replay support
matrix. Missing rows are treated as `unsupported`. Unsupported rows never perform live
side effects.

## Required Columns

| Column | Requirement |
|--------|-------------|
| `toolClass` | Stable class name or resource kind reachable from replay candidates. |
| `safetyClass` | `read_only`, `idempotent_mutation`, `non_idempotent_mutation`, or `unsupported`. |
| `permission` | Tenant permission required before this class can be live-validated. |
| `approval` | Whether approval is not required, scope-level, or per-action and the recorded approval action. |
| `idempotency` | Correlation key, downstream idempotency support, or explicit lack of support. |
| `retryPolicy` | `automatic_retry`, `manual_retry`, or `no_retry`. |
| `ambiguousCommitBehavior` | State used when submit status is unknown. |
| `compensation` | Automatic compensation, manual confirmation, or unsupported. |
| `ledgerEvents` | Required ledger outcomes for this class. |
| `testCase` | Fake-backend or completeness test proving the row. |

## Initial Matrix

| Tool Class | Safety Class | Permission | Approval | Idempotency | Retry Policy | Ambiguous Commit Behavior | Compensation | Ledger Events | Test Case |
|------------|--------------|------------|----------|-------------|--------------|---------------------------|--------------|---------------|-----------|
| `daemon.inspection.read` | `read_only` | `live_validation.execute` | scope-level approval allowed when included in explicit scope | validation correlation key | automatic retry for transient local read failure | not applicable; no external commit | not applicable | attempted, completed, failed, aborted, denied | matrix completeness and read-only replay test |
| `runtime.local_tool_call` | `idempotent_mutation` when replay writes daemon-local evidence; otherwise `read_only` | `live_validation.execute` | scope-level approval allowed | validation id + source tool call id | automatic retry only before accepted local state transition | operator_action_needed if local persistence outcome cannot be proven after restart | manual confirmation | attempted, completed, failed, aborted, denied, operator-action-needed | fake runtime local tool replay/restart test |
| `mcp.tool_call` | `unsupported` by default until each MCP server/tool declares replay safety | `live_validation.execute` plus `mcp.manage` when tenant-managed MCP credentials are touched | unsupported unless explicit row overrides | server/tool-specific idempotency required for mutations | no retry by default | operator_action_needed for any submit-unknown override | unsupported by default | skipped, denied | MCP unsupported completeness test |
| `integration.probe.read` | `read_only` | `live_validation.execute` plus integration inspect/manage permission required by existing credential policy | scope-level approval allowed | validation correlation key | automatic retry for fake/read transient failures | not applicable unless downstream read has side effects, then unsupported | not applicable | attempted, completed, failed, aborted, denied | fake integration read probe test |
| `integration.probe.mutation` | `idempotent_mutation` only when fake/live backend supports idempotency; otherwise `unsupported` | `live_validation.execute` plus integration manage permission | scope-level approval for idempotent mutation | provider operation id or client idempotency key | manual retry unless idempotency proves duplicate safety | operator_action_needed | manual confirmation | attempted, completed, failed, aborted, denied, operator-action-needed | fake integration mutation timeout/retry test |
| `calendar.event.create` | `non_idempotent_mutation` unless provider idempotency key is proven | `live_validation.execute` plus calendar/integration manage permission | per-action approval required | calendar operation id or provider idempotency key where supported | no automatic retry after submit-unknown | operator_action_needed | manual confirmation or explicit cancel compensation when available | attempted, completed, failed, aborted, denied, operator-action-needed | fake calendar create ambiguous commit test |
| `calendar.event.update` | `idempotent_mutation` when target event and revision/correlation are stable | `live_validation.execute` plus calendar/integration manage permission | scope-level approval allowed for idempotent rows; per-action if classified non-idempotent | event id + revision/correlation key | manual retry unless idempotency proves duplicate safety | operator_action_needed | manual confirmation or inverse update where safe | attempted, completed, failed, aborted, denied, operator-action-needed | fake calendar update retry/reconciliation test |
| `calendar.event.cancel` | `non_idempotent_mutation` unless provider cancellation is idempotent for target event | `live_validation.execute` plus calendar/integration manage permission | per-action approval required when non-idempotent | event id + cancellation correlation | no automatic retry after submit-unknown | operator_action_needed | manual confirmation | attempted, completed, failed, aborted, denied, operator-action-needed | fake calendar cancel submit-unknown test |
| `mail.draft.create` | `idempotent_mutation` when draft id/client key is stable | `live_validation.execute` plus mail/integration manage permission | scope-level approval allowed | draft client key or provider draft id | manual retry with idempotency evidence | operator_action_needed | manual confirmation or draft delete when supported | attempted, completed, failed, aborted, denied, operator-action-needed | fake mail draft create test |
| `mail.draft.update` | `idempotent_mutation` when draft id is stable | `live_validation.execute` plus mail/integration manage permission | scope-level approval allowed | draft id + revision/correlation | manual retry with idempotency evidence | operator_action_needed | manual confirmation or inverse update when supported | attempted, completed, failed, aborted, denied, operator-action-needed | fake mail draft update test |
| `mail.send` | `non_idempotent_mutation` | `live_validation.execute` plus mail/integration manage permission | per-action approval required | message idempotency key where provider supports it | no automatic retry after submit-unknown | operator_action_needed | manual confirmation; no automatic compensation | attempted, completed, failed, aborted, denied, operator-action-needed | fake mail send ambiguous commit test |
| `mail.reply` | `non_idempotent_mutation` | `live_validation.execute` plus mail/integration manage permission | per-action approval required | thread/message correlation where supported | no automatic retry after submit-unknown | operator_action_needed | manual confirmation; no automatic compensation | attempted, completed, failed, aborted, denied, operator-action-needed | fake mail reply ambiguous commit test |
| `mail.forward` | `non_idempotent_mutation` | `live_validation.execute` plus mail/integration manage permission | per-action approval required | message correlation where supported | no automatic retry after submit-unknown | operator_action_needed | manual confirmation; no automatic compensation | attempted, completed, failed, aborted, denied, operator-action-needed | fake mail forward ambiguous commit test |
| `reminder.lifecycle.mutation` | `idempotent_mutation` when reminder id is stable | `live_validation.execute` | scope-level approval allowed for idempotent rows | reminder id + desired state | automatic retry before accepted state transition; manual retry after unknown persistence | operator_action_needed | manual confirmation | attempted, completed, failed, aborted, denied, operator-action-needed | fake reminder lifecycle test |
| `delivery.dispatch` | `non_idempotent_mutation` unless dispatch sink exposes idempotency key | `live_validation.execute` plus connector/delivery permission where required | per-action approval required when non-idempotent | dispatch id or sink idempotency key | no automatic retry after submit-unknown unless idempotent sink proves safety | operator_action_needed | manual confirmation | attempted, completed, failed, aborted, denied, operator-action-needed | fake delivery dispatch duplicate retry test |
| `connector.message.send` | `non_idempotent_mutation` | `live_validation.execute` plus connector manage permission | per-action approval required | connector message idempotency key where supported | no automatic retry after submit-unknown | operator_action_needed | manual confirmation; no automatic compensation | attempted, completed, failed, aborted, denied, operator-action-needed | fake connector message send test |
| `provider.sandbox.unsupported` | `unsupported` | not applicable | not applicable | not available | no retry | unsupported validation state | unsupported | skipped, denied | unsupported provider/sandbox classification test |

## Completeness Rule

Implementation is incomplete if a tool-call class reachable from replay candidates can
start live validation without either:

- a row in this matrix and a proving test, or
- an explicit unsupported row and operator-visible unsupported state.

## Mixed Candidate Rule

When a candidate contains both supported and unsupported classes, unsupported classes
block only their own steps. Supported steps may run only after the operator explicitly
excludes unsupported work from the live-validation scope.
