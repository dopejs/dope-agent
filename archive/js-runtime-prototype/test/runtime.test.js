import test from "node:test";
import assert from "node:assert/strict";

import {
  AgentRuntime,
  EVENT_KIND,
  RUN_STATUS,
  STEP_STATUS
} from "../src/index.js";

test("runtime creates a run and records the creation event", () => {
  const runtime = new AgentRuntime();
  const run = runtime.createRun({
    entrypoint: "chat",
    userId: "user-1",
    workspaceId: "workspace-1",
    goal: "help the user ship a task"
  });

  const snapshot = runtime.getRun(run.runId);
  assert.equal(snapshot.run.status, RUN_STATUS.RUNNING);
  assert.equal(snapshot.events.length, 1);
  assert.equal(snapshot.events[0].kind, EVENT_KIND.RUN_CREATED);
});

test("runtime enforces valid step transitions", () => {
  const runtime = new AgentRuntime();
  const run = runtime.createRun({
    entrypoint: "chat",
    userId: "user-1",
    workspaceId: "workspace-1"
  });
  const step = runtime.createStep(run.runId, {
    title: "plan the task"
  });

  runtime.updateStepStatus(run.runId, step.stepId, STEP_STATUS.PLANNING);
  runtime.updateStepStatus(run.runId, step.stepId, STEP_STATUS.CALLING_MODEL);
  runtime.updateStepStatus(run.runId, step.stepId, STEP_STATUS.EXECUTING_TOOL);
  const completed = runtime.updateStepStatus(
    run.runId,
    step.stepId,
    STEP_STATUS.COMPLETED,
    { output: { summary: "done" } }
  );

  assert.equal(completed.status, STEP_STATUS.COMPLETED);
  assert.deepEqual(completed.output, { summary: "done" });
});

test("runtime rejects invalid step transitions", () => {
  const runtime = new AgentRuntime();
  const run = runtime.createRun({
    entrypoint: "chat",
    userId: "user-1",
    workspaceId: "workspace-1"
  });
  const step = runtime.createStep(run.runId, {
    title: "bad transition test"
  });

  assert.throws(() => {
    runtime.updateStepStatus(run.runId, step.stepId, STEP_STATUS.COMPLETED);
  }, /Invalid step transition/);
});

test("runtime can checkpoint and restore", () => {
  const runtime = new AgentRuntime();
  const run = runtime.createRun({
    entrypoint: "chat",
    userId: "user-1",
    workspaceId: "workspace-1"
  });
  const step = runtime.createStep(run.runId, {
    title: "checkpoint test"
  });
  runtime.updateStepStatus(run.runId, step.stepId, STEP_STATUS.PLANNING);
  const checkpoint = runtime.saveCheckpoint(run.runId);

  const recoveredRuntime = new AgentRuntime({
    checkpointStore: runtime.checkpointStore
  });
  const restored = recoveredRuntime.restoreCheckpoint(checkpoint.checkpointId);

  assert.equal(restored.run.runId, run.runId);
  assert.equal(restored.steps.length, 1);
  assert.equal(restored.steps[0].status, STEP_STATUS.PLANNING);
  assert.equal(
    restored.events.some((event) => event.kind === EVENT_KIND.STEP_STATUS_UPDATED),
    true
  );
});

test("runtime records policy and tool call envelopes", () => {
  const runtime = new AgentRuntime();
  const run = runtime.createRun({
    entrypoint: "chat",
    userId: "user-1",
    workspaceId: "workspace-1"
  });
  const step = runtime.createStep(run.runId, {
    title: "tool test"
  });
  runtime.updateStepStatus(run.runId, step.stepId, STEP_STATUS.PLANNING);

  const decision = runtime.evaluatePolicy(
    run.runId,
    step.stepId,
    "tool.execute",
    { toolName: "shell" }
  );
  const toolCall = runtime.requestToolCall(run.runId, step.stepId, {
    toolName: "shell",
    input: { cmd: "pwd" }
  });
  runtime.completeToolCall(run.runId, step.stepId, toolCall.toolCallId, {
    exitCode: 0
  });

  const snapshot = runtime.getRun(run.runId);
  assert.equal(decision.outcome, "allow");
  assert.equal(toolCall.toolName, "shell");
  assert.equal(
    snapshot.events.some((event) => event.kind === EVENT_KIND.POLICY_DECIDED),
    true
  );
  assert.equal(
    snapshot.events.some((event) => event.kind === EVENT_KIND.TOOL_CALL_COMPLETED),
    true
  );
});
