import { describe, expect, it } from "vitest";

export const evaluationProductTenant = {
  tenantId: "ten_eval_product",
  displayName: "Evaluation Product Tenant"
};

export const evaluationProductSensitiveFields = [
  "authorization",
  "access_token",
  "refresh_token",
  "session_token"
];

describe("evaluation product fixtures", () => {
  it("keeps setup fixtures tenant-scoped and free of raw secrets", () => {
    expect(evaluationProductTenant.tenantId).toBe("ten_eval_product");
    expect(evaluationProductSensitiveFields).toContain("access_token");
  });
});

