import { describe, expect, it, vi } from "vitest";

import { createDopeClient } from "./index";

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), { status, headers: { "Content-Type": "application/json" } });
}

describe("Roadmap 65-69 product surface SDK methods (operator shell, Roadmap 70)", () => {
  it("routes triage, routine, webhook, catalog, and execution-profile calls through daemon APIs", async () => {
    const fetchImpl = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse({ policyId: "tp_1", name: "inbox", rules: [], defaultClassification: "fyi", createdAt: "t", updatedAt: "t" }))
      .mockResolvedValueOnce(jsonResponse({ runId: "tr_1", policyId: "tp_1", messageCount: 1, decisions: [{ messageId: "m1", classification: "fyi", outcome: "no_action", defaultApplied: true, replayCandidate: true, decidedAt: "t" }], createdAt: "t" }))
      .mockResolvedValueOnce(jsonResponse({ routineId: "r_1", name: "daily", state: "active", currentVersion: 1, definition: { name: "daily", trigger: { kind: "cron", cronExpr: "0 8 * * *" }, workflow: { goal: "summarize" } }, versions: [], createdAt: "t", updatedAt: "t" }))
      .mockResolvedValueOnce(jsonResponse({ routineId: "r_1", name: "daily", state: "paused", currentVersion: 1, definition: { name: "daily", trigger: { kind: "cron", cronExpr: "0 8 * * *" }, workflow: { goal: "summarize" } }, versions: [], createdAt: "t", updatedAt: "t" }))
      .mockResolvedValueOnce(jsonResponse({ endpoint: { webhookId: "wh_1", name: "hook", targetKind: "routine", targetRef: "r_1", status: "active", secretFingerprint: "sha256:abc", createdAt: "t", updatedAt: "t" }, secret: "deadbeef" }))
      .mockResolvedValueOnce(jsonResponse({ item: { itemId: "ci_1", kind: "skill", name: "pdf", trustTier: "verified", versions: [], createdAt: "t", updatedAt: "t" }, enablement: { itemId: "ci_1", state: "disabled", history: [], updatedAt: "t" }, permissionSatisfied: true }))
      .mockResolvedValueOnce(jsonResponse({ requiredCapabilities: ["docker"], eligibleProfiles: [], missingCapabilities: { subprocess: ["docker"] } }));

    const client = createDopeClient({ baseURL: "https://daemon.test", fetchImpl });

    const policy = await client.createTriagePolicy({ name: "inbox", rules: [], defaultClassification: "fyi" });
    expect(policy.policyId).toBe("tp_1");
    const run = await client.runTriagePolicy("tp_1", { messages: [{ messageId: "m1" }] });
    expect(run.decisions[0].replayCandidate).toBe(true);
    const routine = await client.createRoutine({ definition: { name: "daily", trigger: { kind: "cron", cronExpr: "0 8 * * *" }, workflow: { goal: "summarize" } } });
    expect(routine.routineId).toBe("r_1");
    const paused = await client.routineLifecycle("r_1", "pause");
    expect(paused.state).toBe("paused");
    const webhook = await client.createWebhook({ name: "hook", targetKind: "routine", targetRef: "r_1" });
    expect(webhook.secret).toBe("deadbeef");
    const inspection = await client.inspectCatalogItem("ci_1");
    expect(inspection.permissionSatisfied).toBe(true);
    const denial = await client.explainExecution({ requiredCapabilities: ["docker"] });
    expect(denial.missingCapabilities?.subprocess).toEqual(["docker"]);

    const urls = fetchImpl.mock.calls.map((c) => String(c[0]));
    expect(urls.some((u) => u.includes("/v1/triage/policies"))).toBe(true);
    expect(urls.some((u) => u.includes("/v1/routines/r_1/pause"))).toBe(true);
    expect(urls.some((u) => u.includes("/v1/webhooks"))).toBe(true);
    expect(urls.some((u) => u.includes("/v1/catalog/items/ci_1"))).toBe(true);
    expect(urls.some((u) => u.includes("/v1/execution/explain"))).toBe(true);
  });

  it("routes support evidence bundle + launch-gate calls through daemon APIs", async () => {
    const fetchImpl = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse({ bundleId: "eb_1", scope: { kind: "routine", ref: "r_1" }, redactionStatus: "redacted", createdAt: "t", retentionExpiresAt: "t" }))
      .mockResolvedValueOnce(jsonResponse({ result: "no_ship", reasons: ["missing mail provider smoke entry"], nonKnowledgeParityComplete: false, gateStatement: "gate" }));

    const client = createDopeClient({ baseURL: "https://daemon.test", fetchImpl });

    const bundle = await client.generateEvidenceBundle({ actor: "support@dope", scope: { kind: "routine", ref: "r_1" } });
    expect(bundle.redactionStatus).toBe("redacted");
    const decision = await client.validateLaunchGate({ workloads: [], soakDurationMet: false });
    expect(decision.result).toBe("no_ship");

    const urls = fetchImpl.mock.calls.map((c) => String(c[0]));
    expect(urls.some((u) => u.includes("/v1/support/evidence-bundles"))).toBe(true);
    expect(urls.some((u) => u.includes("/v1/release/launch-gate"))).toBe(true);
  });
});
