import { createPolicyDecision } from "./contracts.js";

export class AllowAllPolicy {
  evaluate({ runId, stepId = null, action, metadata = {} }) {
    return createPolicyDecision({
      runId,
      stepId,
      action,
      outcome: "allow",
      reason: "default allow policy",
      metadata
    });
  }
}
