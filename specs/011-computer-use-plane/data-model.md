# Data Model: Computer-Use Capability Plane

## Entities

### Computer-Use Session

- Purpose: Operator-visible browser interaction context owned by exactly one run and
  optionally one workflow path.
- Fields:
  - `computerUseSessionId`
  - `environmentScope`
  - `runId`
  - `workflowId`
  - `workflowStepId`
  - `status`: `starting`, `active`, `blocked`, `closing`, `closed`, `failed`, or
    `interrupted`
  - `driverKind`
  - `trustedPageScope`
  - `currentPage`
  - `lastActionId`
  - `startedAt`
  - `updatedAt`
  - `closedAt`
  - `interruptedAt`
  - `actions[]`
- Validation rules:
  - every session belongs to exactly one daemon environment scope and one run
  - a session may be reused across multiple actions only within the same run or workflow
  - a session supports exactly one active page in phase 26
  - a session cannot be resumed into a different run or schedule dispatch after restart

### Computer-Use Action

- Purpose: One concrete navigation or browser interaction request with approval, target,
  and runtime linkage.
- Fields:
  - `computerUseActionId`
  - `computerUseSessionId`
  - `runId`
  - `stepId`
  - `toolCallId`
  - `workflowId`
  - `workflowStepId`
  - `actionKind`: `navigate`, `back`, `forward`, `wait`, `screenshot`, `snapshot`,
    `click`, `input`, `select`, `download`, or `close_session`
  - `status`: `requested`, `waiting_approval`, `running`, `completed`, `denied`,
    `failed`, `interrupted`
  - `riskLevel`: `low` or `high`
  - `approvalId`
  - `targetMatchContext`
  - `pageBefore`
  - `pageAfter`
  - `failureClass`
  - `failureReason`
  - `requestedAt`
  - `updatedAt`
  - `completedAt`
  - `artifacts[]`
- Validation rules:
  - each action belongs to one session and one runtime tool call
  - high-risk actions can move to `running` only after approval resolves to approved
  - target mismatch is terminal for the action and requires a later action with renewed
    inspection rather than automatic continuation
  - action kinds requiring additional tabs or windows are invalid in phase 26

### Target Match Context

- Purpose: Inspectable description of what target the operator approved and what the
  browser observed when execution started.
- Fields:
  - `matchStrategy`: `page_url`, `dom_selector`, `text_anchor`, or `download_target`
  - `expectedPageUrl`
  - `expectedSelector`
  - `expectedText`
  - `trustedScopeRevision`
  - `observedPageUrl`
  - `observedSelectorState`
  - `matchResult`: `matched` or `mismatched`
  - `evaluatedAt`
- Validation rules:
  - actions that can alter page state or external state must persist target-match context
  - mismatch results must retain the latest observed browser evidence
  - trusted scope revisions are immutable once an action is requested

### Computer-Use Artifact

- Purpose: First-class evidence object explaining what a computer-use action saw or
  changed.
- Fields:
  - `artifactId`
  - `computerUseSessionId`
  - `computerUseActionId`
  - `runId`
  - `kind`: `screenshot`, `page_snapshot`, or `download`
  - `status`: `capturing`, `available`, or `capture_failed`
  - `mimeType`
  - `fileName`
  - `byteSize`
  - `storageKey`
  - `sha256`
  - `createdAt`
  - `availableAt`
- Validation rules:
  - artifacts are environment-scoped and immutable after capture
  - an artifact belongs to one action and optionally one session-wide summary view
  - failed captures remain visible as metadata even when content is unavailable

### Trusted Page Scope

- Purpose: Session-local summary of the currently trusted browser context used to classify
  whether a later action leaves the approved page boundary.
- Fields:
  - `scopeId`
  - `computerUseSessionId`
  - `origin`
  - `pageUrl`
  - `title`
  - `scopeRevision`
  - `derivedFromActionId`
  - `createdAt`
- Validation rules:
  - scope revisions advance only after a completed action updates the active page
  - actions marked high-risk when they leave the current trusted page scope must persist
    the scope revision they were approved against

## State Transitions

### Session Lifecycle

- `starting` -> `active` when the browser driver is ready and the initial page is known
- `active` -> `blocked` when a pending high-risk action is waiting for approval
- `blocked` -> `active` when the pending action is approved, denied, or fails terminally
- `active` -> `closing` when the owning run or operator requests session shutdown
- `closing` -> `closed` when the driver confirms teardown
- `starting`, `active`, or `blocked` -> `failed` when the browser consumer becomes
  unavailable
- `starting`, `active`, or `blocked` -> `interrupted` on daemon restart or process loss

### Action Lifecycle

- `requested` -> `waiting_approval` for high-risk actions
- `requested` -> `running` for lower-risk actions that do not need approval
- `waiting_approval` -> `running` after approval
- `waiting_approval` -> `denied` when approval is denied
- `running` -> `completed` when the action finishes and any expected evidence is recorded
- `running` -> `failed` on target mismatch, navigation failure, consumer unavailability,
  or explicit policy block
- `running` -> `interrupted` when daemon restart or browser loss happens before completion

## Relationships

- one run can own many computer-use sessions, but a session belongs to exactly one run
- one session owns many actions over time
- one action belongs to exactly one runtime step and one tool call
- one action can create many artifacts
- one trusted page scope revision belongs to one session and may be referenced by many
  later actions

## Derived Views

- run detail and tool-call detail can project current or recent computer-use linkage
  without duplicating session state
- session detail expands current page, trusted scope, recent actions, and artifact
  summaries
- action detail distinguishes approval denial, target mismatch, navigation failure, and
  unavailable-consumer outcomes without raw-log reconstruction
- artifact detail provides stable evidence metadata and download access after restart
