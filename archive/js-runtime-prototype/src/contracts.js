import { randomUUID } from "node:crypto";

export const RUN_STATUS = Object.freeze({
  QUEUED: "queued",
  RUNNING: "running",
  WAITING_INPUT: "waiting_input",
  BLOCKED: "blocked",
  COMPLETED: "completed",
  FAILED: "failed",
  CANCELLED: "cancelled"
});

export const STEP_STATUS = Object.freeze({
  QUEUED: "queued",
  PLANNING: "planning",
  CALLING_MODEL: "calling_model",
  EXECUTING_TOOL: "executing_tool",
  WAITING_INPUT: "waiting_input",
  BLOCKED: "blocked",
  COMPLETED: "completed",
  FAILED: "failed",
  CANCELLED: "cancelled"
});

export const EVENT_KIND = Object.freeze({
  RUN_CREATED: "run.created",
  RUN_STATUS_UPDATED: "run.status_updated",
  STEP_CREATED: "step.created",
  STEP_STATUS_UPDATED: "step.status_updated",
  TOOL_CALL_REQUESTED: "tool_call.requested",
  TOOL_CALL_COMPLETED: "tool_call.completed",
  TOOL_CALL_FAILED: "tool_call.failed",
  POLICY_DECIDED: "policy.decided",
  CHECKPOINT_SAVED: "checkpoint.saved"
});

export function createRun({
  runId = randomUUID(),
  entrypoint,
  userId,
  workspaceId,
  goal = "",
  status = RUN_STATUS.QUEUED,
  createdAt = new Date().toISOString(),
  updatedAt = createdAt
}) {
  return {
    runId,
    entrypoint,
    userId,
    workspaceId,
    goal,
    status,
    createdAt,
    updatedAt
  };
}

export function createStep({
  stepId = randomUUID(),
  runId,
  title,
  kind = "task",
  status = STEP_STATUS.QUEUED,
  input = null,
  output = null,
  createdAt = new Date().toISOString(),
  updatedAt = createdAt
}) {
  return {
    stepId,
    runId,
    title,
    kind,
    status,
    input,
    output,
    createdAt,
    updatedAt
  };
}

export function createEvent({
  eventId = randomUUID(),
  runId,
  stepId = null,
  kind,
  payload = {},
  occurredAt = new Date().toISOString()
}) {
  return {
    eventId,
    runId,
    stepId,
    kind,
    payload,
    occurredAt
  };
}

export function createToolCall({
  toolCallId = randomUUID(),
  runId,
  stepId,
  toolName,
  input,
  idempotencyKey = `${runId}:${stepId}:${toolName}`
}) {
  return {
    toolCallId,
    runId,
    stepId,
    toolName,
    input,
    idempotencyKey
  };
}

export function createPolicyDecision({
  decisionId = randomUUID(),
  runId,
  stepId = null,
  action,
  outcome,
  reason = "",
  metadata = {}
}) {
  return {
    decisionId,
    runId,
    stepId,
    action,
    outcome,
    reason,
    metadata
  };
}

export function createCheckpoint({
  checkpointId = randomUUID(),
  run,
  steps,
  events,
  savedAt = new Date().toISOString()
}) {
  return {
    checkpointId,
    run,
    steps,
    events,
    savedAt
  };
}
