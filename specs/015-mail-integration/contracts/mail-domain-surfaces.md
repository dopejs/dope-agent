# Contract Surfaces: Mail Integration

## Goal

Add a daemon-owned mail domain that reuses shared integration readiness and delivery
truth while exposing inspectable mailbox account projection, thread and message
inspection, draft lifecycle, truthful outbound send paths, attachment metadata truth,
and background workflow linkage.

## HTTP API Surfaces

### Reused Integration Dependency

- existing phase 27 routes remain the source of truth for mail integration readiness and
  canonical-default selection:
  - `GET /v1/integrations`
  - `GET /v1/integrations/{integrationId}`
  - `POST /v1/integrations/{integrationId}/readiness`
  - `POST /v1/integrations/{integrationId}/default`
- mail routes must reference those resources rather than duplicating readiness or
  canonical-default mutation semantics

### New Mail Account Projection Routes

- `GET /v1/mail/accounts`
- `GET /v1/mail/accounts/{integrationId}`

Request and response requirements:

- list responses support filtering by:
  - `integrationId`
  - `readinessStatus`
  - `canonicalDefault`
- account projection responses return:
  - integration linkage (`integrationId`, `accountKey`, `accountLabel`)
  - `environmentScope`
  - readiness summary copied from the linked integration
  - mailbox identity (`mailboxAddress`, `mailboxLabel`)
  - capability flags for thread inspection, drafts, direct send, reply, and forward
  - timestamps for last sync or projection refresh

Schema surfaces:

- add `schemas/api/mail-account-resource.schema.json`
- add `schemas/api/mail-account-list.response.schema.json`

### New Mail Inspection Routes

- `GET /v1/mail/threads`
- `GET /v1/mail/threads/{threadId}`
- `GET /v1/mail/messages/{messageId}`
- `GET /v1/mail/drafts`
- `GET /v1/mail/drafts/{draftId}`

Request and response requirements:

- all mail read routes accept an optional request-scoped `integrationId` selector; when
  omitted, the server resolves the canonical default mailbox projection and returns the
  chosen `integrationId` in the resulting operation or resource
- thread list requests accept:
  - optional `integrationId`
  - optional `limit`
  - optional `cursor`
- thread detail requests accept:
  - optional `integrationId`
- message detail requests accept:
  - optional `integrationId`
- draft list and detail requests accept:
  - optional `integrationId`
- inspection responses return:
  - authoritative `operationId`
  - selected `integrationId`
  - mailbox account projection summary
  - thread, message, or draft resources plus linked attachment summaries when present

Schema surfaces:

- add `schemas/api/mail-thread-resource.schema.json`
- add `schemas/api/mail-thread-list.response.schema.json`
- add `schemas/api/mail-message-resource.schema.json`
- add `schemas/api/mail-draft-resource.schema.json`
- add `schemas/api/mail-draft-list.response.schema.json`

### New Mail Draft And Outbound Routes

- `POST /v1/mail/drafts`
- `POST /v1/mail/drafts/{draftId}/update`
- `POST /v1/mail/messages/send`
- `POST /v1/mail/drafts/{draftId}/send`
- `POST /v1/mail/messages/{messageId}/reply`
- `POST /v1/mail/messages/{messageId}/forward`

Request and response requirements:

- create-draft requests accept:
  - optional `integrationId`
  - `composeMode`: `new_message`, `reply`, or `forward`
  - optional `threadId`
  - optional `sourceMessageId`
  - recipient fields (`to`, `cc`, `bcc`) when applicable
  - `subject`
  - `body`
  - optional `attachmentRefs`
  - optional `source` linkage with `runId`, `workflowId`, `scheduleId`,
    `scheduleAttemptId`, `deliveryId`, and `allowSendSideEffects`
- update-draft requests accept:
  - optional `integrationId`
  - partial recipient, subject, body, and attachment-ref updates
  - optional `source` linkage
- direct-send requests accept:
  - optional `integrationId`
  - explicit recipient fields (`to`, `cc`, `bcc`)
  - `subject`
  - `body`
  - optional `attachmentRefs`
  - optional `source` linkage
- direct-send requests MUST reject:
  - missing explicit recipients for new outbound mail
  - unresolved or failed required attachment references
- send-draft requests accept:
  - optional `integrationId`
  - optional source linkage metadata if initiated from workflow or schedule context
- reply requests accept:
  - optional `integrationId`
  - `resultMode`: `draft` or `send`
  - optional body or subject adjustments
  - optional `attachmentRefs`
- forward requests accept:
  - optional `integrationId`
  - `resultMode`: `draft` or `send`
  - explicit recipient fields (`to`, `cc`, `bcc`)
  - optional body or subject adjustments
  - optional `attachmentRefs`
- background workflow requests that attempt a sent outcome MUST include
  `allowSendSideEffects: true`; otherwise `send_message`, `send_draft`, `reply`, and
  `forward` requests in background context are limited to draft-only results or blocked
  failure
- mutation responses return:
  - authoritative `operationId`
  - selected `integrationId`
  - `resultMode`
  - `sendPath` when applicable (`direct` or `draft`)
  - resulting draft or message artifact or truthful failure status

Schema surfaces:

- add `schemas/api/create-mail-draft.request.schema.json`
- add `schemas/api/update-mail-draft.request.schema.json`
- add `schemas/api/send-mail-message.request.schema.json`
- add `schemas/api/send-mail-draft.request.schema.json`
- add `schemas/api/reply-mail-message.request.schema.json`
- add `schemas/api/forward-mail-message.request.schema.json`
- add `schemas/api/mail-operation-resource.schema.json`

### New Mail Operation Inspection Routes

- `GET /v1/mail/operations`
- `GET /v1/mail/operations/{operationId}`

Request and response requirements:

- operation list responses support filtering by:
  - `integrationId`
  - `runId`
  - `workflowId`
  - `scheduleId`
  - `deliveryId`
  - `operationClass`
  - `resultMode`
  - `threadId`
  - `messageId`
  - `draftId`
- operation detail returns:
  - mailbox selection and account identity
  - result mode and send path when present
  - source linkage to run, workflow, schedule, and delivery truth when present
  - linked thread, message, draft, and attachment artifacts
  - failure class and reason when the operation did not complete

Schema surfaces:

- add `schemas/api/mail-operation-list.response.schema.json`
- add `schemas/api/mail-operation-summary.schema.json`

### Existing Runtime, Workflow, Schedule, And Delivery Surfaces Extended

- `POST /v1/workflows`
- `POST /v1/schedules`
- `GET /v1/runs/{runId}/tool-calls`
- `GET /v1/runs/{runId}/workflows/{workflowId}`
- `GET /v1/schedules/{scheduleId}`
- `GET /v1/deliveries`
- `GET /v1/deliveries/{deliveryId}`

Additive requirements:

- workflow targets gain optional `mailAction`
- `mailAction` accepts:
  - `operationClass`
  - optional `integrationId`
  - `threadId`, `messageId`, or `draftId` when required
  - recipient fields, subject, body, and attachment refs when required
  - optional `resultMode`
  - `allowSendSideEffects`
- tool-call resources gain `mailOperationSummaries`
- workflow-step resources gain `mailOperationSummaries`
- schedule-attempt resources may expose latest mail-operation summary for background mail
  runs
- delivery outcome detail may include linked `mailOperationIds` when the background
  result originated from mail-domain work

Schema surfaces:

- add `schemas/api/mail-workflow-action.schema.json`
- update `schemas/api/create-workflow.request.schema.json`
- update `schemas/api/create-schedule.request.schema.json`
- update `schemas/api/tool-call-resource.schema.json`
- update `schemas/api/workflow-step-resource.schema.json`
- update `schemas/api/schedule-attempt-resource.schema.json`
- update `schemas/api/delivery-outcome-resource.schema.json`

## Event And History Surfaces

New mail event families:

- `mail.account_projected`
- `mail.operation_requested`
- `mail.operation_completed`
- `mail.operation_failed`
- `mail.artifact_recorded`

Event payload requirements:

- account projection truth:
  - `integrationId`
  - `accountKey`
  - `mailboxAddress`
  - `readinessStatus`
  - `canonicalDefault`
- operation truth:
  - `operationId`
  - `operationClass`
  - `integrationId`
  - `runId`
  - `workflowId`
  - `scheduleId`
  - `resultMode`
  - `sendPath`
  - `threadId`
  - `messageId`
  - `draftId`
  - `failureClass`
- artifact truth:
  - `artifactId`
  - `operationId`
  - `threadId`
  - `messageId`
  - `draftId`
  - `attachmentRefIds`

Schema surfaces:

- add `schemas/events/mail-account-projected.event.schema.json`
- add `schemas/events/mail-operation-requested.event.schema.json`
- add `schemas/events/mail-operation-completed.event.schema.json`
- add `schemas/events/mail-operation-failed.event.schema.json`
- add `schemas/events/mail-artifact-recorded.event.schema.json`

## Persistence Surfaces

Persistence remains additive to the daemon-owned SQLite store:

- add a `mail_accounts` table for mailbox projection and capability metadata
- add a `mail_operations` table for domain action truth and source linkage
- add a `mail_artifacts` table for structured thread, message, draft, and attachment
  snapshots
- extend runtime tool-call and workflow-step documents with `mailOperationSummaries`
- extend schedule-attempt documents with additive `mailOperationSummaries`
- extend delivery-outcome documents or linkage indexes to reference originating mail
  operations when present

Persistence rules:

- mailbox projection, operation truth, and artifact snapshots are environment-scoped and
  durable across daemon restart
- thread, message, and draft inspection store structured artifacts only when backend
  state was actually observed
- attachment artifacts remain metadata-only in phase 30 and must not require persisted
  binary content
- stored artifacts must not depend on live backend re-fetch to remain inspectable
- secret-bearing backend details remain outside mail-domain documents and stay owned by
  the integrations plane

## Documentation Surfaces

Docs updated by implementation:

- `docs/runtime/daemon-roadmaps.md`
- `docs/runtime/daemon-api-and-event-model.md`
- `docs/runtime/operator-trust-model.md`
- `docs/harness/harness-architecture.md`
- `docs/specs/015-mail-integration.md`

## Truthfulness Constraints

- integration readiness, mail execution, and delivery outcomes remain separate planes
- read, draft, direct send, send-existing-draft, reply, and forward remain distinct
  operation classes
- direct-send and send-existing-draft truth remain distinguishable in operator-visible
  history
- new outbound mail that is not reply or forward requires explicit recipients from the
  current request
- attachment scope remains metadata and failure truth only; unresolved required
  attachment references block final send
- background final send requires explicit workflow send permission
- delivery outcomes project `mailOperationIds` and summaries additively; they do not
  redefine whether the underlying mail action succeeded
