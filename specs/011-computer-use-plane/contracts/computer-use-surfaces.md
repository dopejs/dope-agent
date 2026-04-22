# Contract Surfaces: Computer-Use Capability Plane

## Goal

Add a first-class browser-first computer-use plane that remains subordinate to run,
workflow, step, and tool-call truth while exposing session, action, approval, and
artifact inspection surfaces.

## HTTP API Surfaces

### New Run-Scoped Computer-Use Routes

- `POST /v1/runs/{runId}/computer-use/sessions`
- `GET /v1/runs/{runId}/computer-use/sessions`
- `GET /v1/runs/{runId}/computer-use/sessions/{computerUseSessionId}`
- `POST /v1/runs/{runId}/computer-use/sessions/{computerUseSessionId}/actions`
- `GET /v1/runs/{runId}/computer-use/sessions/{computerUseSessionId}/actions`
- `GET /v1/runs/{runId}/computer-use/sessions/{computerUseSessionId}/actions/{computerUseActionId}`
- `POST /v1/runs/{runId}/computer-use/sessions/{computerUseSessionId}/close`

Request and response requirements:

- session creation accepts:
  - optional workflow linkage when the session is opened for a workflow step
  - optional initial page request
  - requested driver kind or browser profile summary
- session resources return:
  - `computerUseSessionId`
  - run/workflow linkage
  - current status
  - current page summary
  - current trusted page scope summary
  - most recent action summary
- action creation accepts:
  - one of the phase 26 action kinds: navigate, back, forward, wait, screenshot,
    snapshot, click, input, select, download, or close_session
  - target-match context or trusted-scope reference when required
  - operator-visible rationale for high-risk actions when approval is expected
- action detail returns:
  - action status
  - approval linkage when applicable
  - target-match result
  - page-before and page-after summaries
  - linked artifacts
  - failure class and reason when applicable

Schema surfaces:

- add `schemas/api/create-computer-use-session.request.schema.json`
- add `schemas/api/computer-use-session-resource.schema.json`
- add `schemas/api/computer-use-session-list.response.schema.json`
- add `schemas/api/create-computer-use-action.request.schema.json`
- add `schemas/api/computer-use-action-resource.schema.json`
- add `schemas/api/computer-use-action-list.response.schema.json`
- add `schemas/api/computer-use-artifact-resource.schema.json`

### New Computer-Use Artifact Routes

- `GET /v1/computer-use/artifacts/{artifactId}`
- `GET /v1/computer-use/artifacts/{artifactId}/content`

Requirements:

- artifact metadata routes return stable evidence summaries, capture status, linkage to
  run/session/action truth, and file metadata
- content download returns the stored binary or serialized page snapshot when capture was
  successful
- failed captures remain visible as artifact metadata even when no content is available

Schema surfaces:

- add `schemas/api/computer-use-artifact-content.response.schema.json` if content delivery
  needs structured metadata in addition to raw bytes

### Existing Runtime And Workflow Surfaces Extended

- `GET /v1/runs`
- `GET /v1/runs/{runId}`
- `GET /v1/runs/{runId}/steps/{stepId}` if step detail exists
- `GET /v1/runs/{runId}/tool-calls`
- `GET /v1/runs/{runId}/workflows/{workflowId}`

Additive requirements:

- tool-call resources gain additive computer-use linkage:
  - `computerUseSessionId`
  - `computerUseActionId`
  - target page summary when applicable
- workflow step resources may expose active computer-use linkage via existing runtime step
  and tool-call references without requiring a second execution path
- existing non-computer-use runtime routes remain backward compatible

Schema surfaces:

- update `schemas/api/tool-call-resource.schema.json`
- update `schemas/api/tool-call-list.response.schema.json`
- update `schemas/api/workflow-step-resource.schema.json` if active computer-use linkage is
  projected there

## Event And History Surfaces

New computer-use event families:

- `computer_use.session_created`
- `computer_use.session_status_changed`
- `computer_use.action_requested`
- `computer_use.action_status_changed`
- `computer_use.action_target_mismatch`
- `computer_use.artifact_recorded`

Event payload requirements:

- session identifiers:
  - `computerUseSessionId`
  - `runId`
  - `workflowId`
  - `workflowStepId`
- action identifiers:
  - `computerUseActionId`
  - `stepId`
  - `toolCallId`
  - `actionKind`
  - `status`
- trust and approval context:
  - `riskLevel`
  - `approvalId`
  - `trustedScopeRevision`
  - `matchResult`
- page context:
  - current or observed URL
  - title when available
  - target selector or text anchor summary when relevant
- artifact linkage:
  - `artifactId`
  - `artifactKind`
  - `captureStatus`
- failure metadata:
  - `failureClass`
  - `failureReason`

Schema surfaces:

- update `schemas/events/runtime-event.schema.json` to include
  `computerUseSessionId` and `computerUseActionId` in event scope while keeping the
  existing event categories in phase 26
- add `schemas/events/computer-use-session-created.event.schema.json`
- add `schemas/events/computer-use-session-status-changed.event.schema.json`
- add `schemas/events/computer-use-action-requested.event.schema.json`
- add `schemas/events/computer-use-action-status-changed.event.schema.json`
- add `schemas/events/computer-use-action-target-mismatch.event.schema.json`
- add `schemas/events/computer-use-artifact-recorded.event.schema.json`

Truthfulness rules:

- action denial, target mismatch, navigation failure, and unavailable-consumer failure
  must remain distinguishable
- target mismatch must be recorded as an immediate terminal action failure until a new
  inspected action is requested
- restart must record interrupted session/action truth rather than silently resuming work

## Persistence Surfaces

Persistence remains additive to the daemon-owned SQLite store:

- add a `computer_use_sessions` table for run-scoped browser session resources
- add a `computer_use_actions` table for action requests, statuses, target context, and
  runtime linkage
- add a `computer_use_artifacts` table for evidence metadata and stored content linkage
- add nullable computer-use linkage columns to `tool_calls`
- optionally add lightweight page-scope metadata storage if separate normalization improves
  queryability

Persistence rules:

- sessions, actions, and artifacts are environment-scoped and durable across daemon
  restart
- session rows never migrate across run boundaries
- artifact metadata remains queryable without replaying raw events
- browser evidence storage must not create a hidden execution plane or bypass existing run
  and tool-call truth

## Documentation Surfaces

Docs updated by implementation:

- `docs/harness/harness-architecture.md`
- `docs/runtime/daemon-roadmaps.md`
- `docs/runtime/operator-trust-model.md`
- runtime or harness docs describing browser-first scope, risk-based approvals, target
  mismatch behavior, artifact inspection, and restart interruption semantics

## Truthfulness Constraints

- phase 26 is browser-first and single-page per session
- sessions may be reused inside one run or workflow but never across separate runs or
  schedule dispatches
- high-risk actions require approval tied to the concrete action and page context
- action execution remains on the normal runtime plane with additive computer-use linkage
- screenshots, snapshots, and downloads are durable evidence, not transient log lines
