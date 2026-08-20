import { describe, expect, it, vi } from "vitest";

import { createKuraClient } from "./index";

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), { status, headers: { "Content-Type": "application/json" } });
}

const asset = {
  assetId: "mem_1",
  kind: "chat_memory",
  layer: "l1",
  owner: { kind: "operator", id: "op_1" },
  visibility: "private",
  status: "ready",
  version: 1,
  atomType: "preference",
  title: "reply language",
  content: "The user prefers Chinese replies.",
  sourceLinks: [{ kind: "thread", id: "thr_1" }],
  createdAt: "t",
  updatedAt: "t",
};

describe("memory plane SDK methods (Roadmap 78, spec 058)", () => {
  it("routes memory asset lifecycle calls through the daemon APIs", async () => {
    const fetchImpl = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse({ asset, decision: "accept" }))
      .mockResolvedValueOnce(jsonResponse({ items: [asset] }))
      .mockResolvedValueOnce(jsonResponse({ asset, members: [] }))
      .mockResolvedValueOnce(jsonResponse({ ...asset, status: "revoked" }))
      .mockResolvedValueOnce(jsonResponse({
        runId: "memrun_1",
        trigger: "manual",
        extractedL1: 0,
        aggregatedL2: 0,
        distilledL3: 0,
        startedAt: "t",
        completedAt: "t",
      }));

    const client = createKuraClient({ baseURL: "https://daemon.test", fetchImpl });

    const created = await client.createMemoryAsset({
      layer: "l1",
      owner: { kind: "operator", id: "op_1" },
      atomType: "preference",
      content: "The user prefers Chinese replies.",
      sourceLinks: [{ kind: "thread", id: "thr_1" }],
    });
    expect(created.decision).toBe("accept");
    const listed = await client.listMemoryAssets({ layer: "l1", status: "ready" });
    expect(listed.items[0].assetId).toBe("mem_1");
    const tree = await client.getMemoryDrilldown("mem_1");
    expect(tree.asset.assetId).toBe("mem_1");
    const revoked = await client.revokeMemoryAsset("mem_1", "forget me");
    expect(revoked.status).toBe("revoked");
    const run = await client.consolidateMemory({ trigger: "manual" });
    expect(run.runId).toBe("memrun_1");

    const urls = fetchImpl.mock.calls.map((c) => String(c[0]));
    expect(urls.some((u) => u.includes("/v1/memory/assets?layer=l1&status=ready"))).toBe(true);
    expect(urls.some((u) => u.includes("/v1/memory/assets/mem_1/drilldown"))).toBe(true);
    expect(urls.some((u) => u.includes("/v1/memory/consolidate"))).toBe(true);
  });
});
