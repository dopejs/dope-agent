import {
  createCheckpoint,
  createEvent,
  createRun,
  createStep,
  createToolCall,
  EVENT_KIND,
  RUN_STATUS,
  STEP_STATUS
} from "./contracts.js";
import { AllowAllPolicy } from "./policy.js";

const STEP_TRANSITIONS = Object.freeze({
  [STEP_STATUS.QUEUED]: new Set([
    STEP_STATUS.PLANNING,
    STEP_STATUS.CANCELLED
  ]),
  [STEP_STATUS.PLANNING]: new Set([
    STEP_STATUS.CALLING_MODEL,
    STEP_STATUS.EXECUTING_TOOL,
    STEP_STATUS.WAITING_INPUT,
    STEP_STATUS.BLOCKED,
    STEP_STATUS.FAILED,
    STEP_STATUS.CANCELLED
  ]),
  [STEP_STATUS.CALLING_MODEL]: new Set([
    STEP_STATUS.PLANNING,
    STEP_STATUS.EXECUTING_TOOL,
    STEP_STATUS.WAITING_INPUT,
    STEP_STATUS.BLOCKED,
    STEP_STATUS.COMPLETED,
    STEP_STATUS.FAILED,
    STEP_STATUS.CANCELLED
  ]),
  [STEP_STATUS.EXECUTING_TOOL]: new Set([
    STEP_STATUS.PLANNING,
    STEP_STATUS.WAITING_INPUT,
    STEP_STATUS.BLOCKED,
    STEP_STATUS.COMPLETED,
    STEP_STATUS.FAILED,
    STEP_STATUS.CANCELLED
  ]),
  [STEP_STATUS.WAITING_INPUT]: new Set([
    STEP_STATUS.PLANNING,
    STEP_STATUS.CANCELLED,
    STEP_STATUS.FAILED
  ]),
  [STEP_STATUS.BLOCKED]: new Set([
    STEP_STATUS.PLANNING,
    STEP_STATUS.CANCELLED,
    STEP_STATUS.FAILED
  ]),
  [STEP_STATUS.COMPLETED]: new Set(),
  [STEP_STATUS.FAILED]: new Set(),
  [STEP_STATUS.CANCELLED]: new Set()
});

export class InMemoryCheckpointStore {
  constructor() {
    this.records = new Map();
  }

  save(checkpoint) {
    this.records.set(checkpoint.checkpointId, structuredClone(checkpoint));
    return checkpoint;
  }

  get(checkpointId) {
    const checkpoint = this.records.get(checkpointId);
    return checkpoint ? structuredClone(checkpoint) : null;
  }
}

export class AgentRuntime {
  constructor({
    checkpointStore = new InMemoryCheckpointStore(),
    policy = new AllowAllPolicy()
  } = {}) {
    this.checkpointStore = checkpointStore;
    this.policy = policy;
    this.runs = new Map();
    this.stepsByRun = new Map();
    this.eventsByRun = new Map();
  }

  createRun({ entrypoint, userId, workspaceId, goal = "" }) {
    const run = createRun({
      entrypoint,
      userId,
      workspaceId,
      goal,
      status: RUN_STATUS.RUNNING
    });

    this.runs.set(run.runId, run);
    this.stepsByRun.set(run.runId, []);
    this.eventsByRun.set(run.runId, []);
    this.#appendEvent(
      createEvent({
        runId: run.runId,
        kind: EVENT_KIND.RUN_CREATED,
        payload: { entrypoint, userId, workspaceId, goal }
      })
    );
    return structuredClone(run);
  }

  getRun(runId) {
    const run = this.#requireRun(runId);
    return {
      run: structuredClone(run),
      steps: structuredClone(this.#steps(runId)),
      events: structuredClone(this.#events(runId))
    };
  }

  updateRunStatus(runId, status, payload = {}) {
    const run = this.#requireRun(runId);
    run.status = status;
    run.updatedAt = new Date().toISOString();
    this.#appendEvent(
      createEvent({
        runId,
        kind: EVENT_KIND.RUN_STATUS_UPDATED,
        payload: { status, ...payload }
      })
    );
    return structuredClone(run);
  }

  createStep(runId, { title, kind = "task", input = null }) {
    this.#requireRun(runId);
    const step = createStep({ runId, title, kind, input });
    this.#steps(runId).push(step);
    this.#appendEvent(
      createEvent({
        runId,
        stepId: step.stepId,
        kind: EVENT_KIND.STEP_CREATED,
        payload: { title, kind, input }
      })
    );
    return structuredClone(step);
  }

  updateStepStatus(runId, stepId, nextStatus, payload = {}) {
    const step = this.#requireStep(runId, stepId);
    const allowed = STEP_TRANSITIONS[step.status];
    if (!allowed.has(nextStatus)) {
      throw new Error(
        `Invalid step transition from ${step.status} to ${nextStatus}`
      );
    }
    step.status = nextStatus;
    step.updatedAt = new Date().toISOString();
    if (Object.hasOwn(payload, "output")) {
      step.output = payload.output;
    }
    this.#appendEvent(
      createEvent({
        runId,
        stepId,
        kind: EVENT_KIND.STEP_STATUS_UPDATED,
        payload: { status: nextStatus, ...payload }
      })
    );
    return structuredClone(step);
  }

  requestToolCall(runId, stepId, { toolName, input }) {
    this.#requireStep(runId, stepId);
    const toolCall = createToolCall({ runId, stepId, toolName, input });
    this.#appendEvent(
      createEvent({
        runId,
        stepId,
        kind: EVENT_KIND.TOOL_CALL_REQUESTED,
        payload: toolCall
      })
    );
    return structuredClone(toolCall);
  }

  completeToolCall(runId, stepId, toolCallId, output) {
    this.#requireStep(runId, stepId);
    this.#appendEvent(
      createEvent({
        runId,
        stepId,
        kind: EVENT_KIND.TOOL_CALL_COMPLETED,
        payload: { toolCallId, output }
      })
    );
  }

  failToolCall(runId, stepId, toolCallId, error) {
    this.#requireStep(runId, stepId);
    this.#appendEvent(
      createEvent({
        runId,
        stepId,
        kind: EVENT_KIND.TOOL_CALL_FAILED,
        payload: { toolCallId, error }
      })
    );
  }

  evaluatePolicy(runId, stepId, action, metadata = {}) {
    this.#requireRun(runId);
    const decision = this.policy.evaluate({ runId, stepId, action, metadata });
    this.#appendEvent(
      createEvent({
        runId,
        stepId,
        kind: EVENT_KIND.POLICY_DECIDED,
        payload: decision
      })
    );
    return structuredClone(decision);
  }

  saveCheckpoint(runId) {
    const snapshot = this.getRun(runId);
    const checkpoint = createCheckpoint(snapshot);
    this.checkpointStore.save(checkpoint);
    this.#appendEvent(
      createEvent({
        runId,
        kind: EVENT_KIND.CHECKPOINT_SAVED,
        payload: {
          checkpointId: checkpoint.checkpointId,
          savedAt: checkpoint.savedAt
        }
      })
    );
    return structuredClone(checkpoint);
  }

  restoreCheckpoint(checkpointId) {
    const checkpoint = this.checkpointStore.get(checkpointId);
    if (!checkpoint) {
      throw new Error(`Checkpoint not found: ${checkpointId}`);
    }
    this.runs.set(checkpoint.run.runId, structuredClone(checkpoint.run));
    this.stepsByRun.set(
      checkpoint.run.runId,
      structuredClone(checkpoint.steps)
    );
    this.eventsByRun.set(
      checkpoint.run.runId,
      structuredClone(checkpoint.events)
    );
    return this.getRun(checkpoint.run.runId);
  }

  #appendEvent(event) {
    this.#events(event.runId).push(event);
    const run = this.#requireRun(event.runId);
    run.updatedAt = event.occurredAt;
  }

  #requireRun(runId) {
    const run = this.runs.get(runId);
    if (!run) {
      throw new Error(`Run not found: ${runId}`);
    }
    return run;
  }

  #requireStep(runId, stepId) {
    this.#requireRun(runId);
    const step = this.#steps(runId).find((item) => item.stepId === stepId);
    if (!step) {
      throw new Error(`Step not found: ${stepId}`);
    }
    return step;
  }

  #steps(runId) {
    const steps = this.stepsByRun.get(runId);
    if (!steps) {
      throw new Error(`Step store missing for run: ${runId}`);
    }
    return steps;
  }

  #events(runId) {
    const events = this.eventsByRun.get(runId);
    if (!events) {
      throw new Error(`Event store missing for run: ${runId}`);
    }
    return events;
  }
}
