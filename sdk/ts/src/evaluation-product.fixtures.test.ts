import { describe, expect, it } from "vitest";
import type {
  EvaluationProductDenial,
  EvaluationProductListResponse,
  EvaluationProductRetentionOutcome,
  EvaluationSuppressionRecord
} from "./index";

export const evaluationProductCursor: EvaluationProductListResponse<EvaluationSuppressionRecord>["page"] = {
  cursor: "cur_eval_product",
  limit: 25
};

export const evaluationProductDenial: EvaluationProductDenial = {
  code: "evaluation_permission_denied",
  message: "evaluation product action denied",
  reasonCode: "missing_permission"
};

export const evaluationProductRetentionOutcome: EvaluationProductRetentionOutcome = {
  applicationId: "retention_1",
  tenantId: "ten_eval",
  resourceKind: "discovered_candidate",
  resourceId: "candidate_1",
  dryRun: false,
  outcome: "expired",
  affectedCount: 1,
  appliedAt: "2026-04-29T10:00:00Z"
};

describe("evaluation product SDK fixtures", () => {
  it("defines stable cursor and denial fixture shapes", () => {
    expect(evaluationProductCursor.limit).toBe(25);
    expect(evaluationProductDenial.reasonCode).toBe("missing_permission");
    expect(evaluationProductRetentionOutcome.resourceKind).toBe("discovered_candidate");
  });
});
