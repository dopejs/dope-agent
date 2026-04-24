# Data Model: Operator Shell And Onboarding

## Entities

### Onboarding Progress Record

- Purpose: Daemon-owned projection of first-run setup state for one environment and one
  primary operator shell session.
- Fields:
  - `environmentScope`
  - `status`: `not_started`, `blocked`, `ready_for_action`, or `completed`
  - `currentStepId`
  - `completedStepIds`
  - `blockingItemIds`
  - `optionalFollowUpItemIds`
  - `recommendedActionId`
  - `lastEvaluatedAt`
- Validation rules:
  - onboarding progress is derived from current daemon truth rather than persisted as an
    independent writable record
  - completion is based on the minimum readiness set for the selected first useful action
  - onboarding state is environment-scoped and never merges test and live truth

### Readiness Item

- Purpose: Operator-visible explanation of one prerequisite the shell must inspect during
  onboarding or diagnostics.
- Fields:
  - `itemId`
  - `itemKind`: `auth`, `config`, `integration`, `connector`, `capability`, or
    `provider`
  - `resourceId`
  - `displayName`
  - `status`: `ready`, `missing_configuration`, `degraded`, `blocked`, or `optional`
  - `healthState`
  - `reason`
  - `requiredOperatorAction`
  - `requiredForSelectedAction`
  - `detailRoute`
  - `environmentScope`
  - `updatedAt`
- Validation rules:
  - every readiness item must link to an authoritative daemon resource or configuration
    fact
  - readiness items required for the selected first useful action must be distinguishable
    from optional follow-up setup
  - readiness reason text is derived from daemon-owned fields and must not be UI-only

### First Useful Action Option

- Purpose: A bounded shell action that demonstrates the product is usable after onboarding.
- Fields:
  - `actionId`
  - `actionKind`: `test_query` or `test_run`
  - `displayName`
  - `recommended`
  - `available`
  - `blockingItemIds`
  - `summary`
  - `invokeRoute`
  - `resultRoute`
- Validation rules:
  - first useful actions reuse existing daemon execution routes
  - exactly one action may be marked recommended at a time
  - if an action is unavailable, its blocking readiness items must be explicit

### Approval Inbox Item

- Purpose: Operator-facing approval record shown in the shell with enough linkage to act on
  and understand the blocked work.
- Fields:
  - `approvalId`
  - `action`
  - `resourceKind`
  - `resourceId`
  - `reason`
  - `requestedBy`
  - `status`: `pending`, `approved`, or `rejected`
  - `resolution`
  - `comment`
  - `createdAt`
  - `updatedAt`
  - `resolvedAt`
  - `blockedResourceSummary`
  - `detailRoute`
- Validation rules:
  - approval inbox items are sourced from the policy plane
  - pending approvals must be actionable from the shell
  - resolved approvals remain inspectable in history without being mixed with active
    pending items

### Operator Activity Record

- Purpose: Cross-domain recent activity summary for schedules, workflows, deliveries,
  approvals, and first useful action outcomes.
- Fields:
  - `activityId`
  - `sourceKind`: `approval`, `schedule`, `workflow`, `delivery`, `run`, `integration`,
    `connector`, `capability`, or `first_action`
  - `sourceId`
  - `title`
  - `status`
  - `summary`
  - `attentionLevel`: `info`, `warning`, or `critical`
  - `occurredAt`
  - `detailRoute`
  - `relatedResourceRefs`
  - `environmentScope`
- Validation rules:
  - activity records are derived from persisted events and current daemon resources
  - every activity record must link back to an authoritative resource identifier
  - activity ordering is time-based within one environment scope

### Diagnostic Finding

- Purpose: Operator-facing explanation of a blocked, degraded, interrupted, or failed
  condition that needs attention.
- Fields:
  - `findingId`
  - `sourceKind`
  - `sourceId`
  - `plane`: `readiness`, `approval`, `execution`, or `delivery`
  - `severity`: `warning` or `critical`
  - `status`
  - `reason`
  - `recommendedAction`
  - `detailRoute`
  - `relatedResourceRefs`
  - `environmentScope`
  - `capturedAt`
- Validation rules:
  - diagnostic findings are projections over explicit daemon status and reason fields
  - the shell must preserve separate findings for readiness, approval, execution, and
    delivery planes rather than collapsing them into one generic error
  - findings may disappear when the underlying daemon truth is resolved; they are not a
    writable persistent ledger in phase 32

## State Transitions

### Onboarding Progress

- `not_started` -> `blocked` when required readiness for the recommended first useful
  action is missing or degraded
- `blocked` -> `ready_for_action` when the minimum readiness set for the recommended first
  useful action becomes ready
- `ready_for_action` -> `completed` when the operator successfully completes the selected
  bounded first useful action
- `completed` -> `blocked` if a previously satisfied required readiness item later
  regresses before the shell session ends

### Readiness Item

- `missing_configuration` -> `ready` when configuration becomes valid and healthy
- `missing_configuration` -> `blocked` when another prerequisite must be completed first
- `ready` -> `degraded` when health declines but partial functionality remains
- `degraded` -> `ready` when health recovers
- any non-ready state -> `optional` only when the item is not required for the selected
  first useful action and remains follow-up setup

## Relationships

- one onboarding progress record contains many readiness items
- one onboarding progress record may expose multiple first useful action options but only
  one recommended action
- one approval inbox item may reference zero or one blocked resource summary
- one operator activity record may reference many related resources
- one diagnostic finding may point to one primary resource plus related linked resources

## Derived Views

- onboarding view filters readiness items into:
  - required for the selected first useful action
  - optional follow-up setup
- approval inbox filters by `status`
- activity feed filters by `sourceKind`, `attentionLevel`, or recency window
- diagnostics filters by `plane`, `sourceKind`, `severity`, or unresolved-only status
