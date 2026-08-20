import { describe, expect, it, vi } from "vitest";

import { createKuraClient } from "./index.js";

function mockJSONResponse(status: number, payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: {
      "Content-Type": "application/json"
    }
  });
}

describe("thread continuity SDK coverage", () => {
  it("sends thread-aware chat input and exposes continuity metadata", async () => {
    let body: Record<string, unknown> = {};
    const client = createKuraClient({
      baseURL: "http://127.0.0.1:19192/",
      accessToken: "token",
      fetchImpl: vi.fn<typeof fetch>(async (_input, init) => {
        body = JSON.parse(String(init?.body));
        return mockJSONResponse(200, {
          dispatchId: "dispatch_1",
          provider: "openai_compatible",
          model: "gpt-test",
          skills: [],
          query: "follow up",
          status: "completed",
          partial: false,
          reply: "answer",
          usage: { inputTokens: 2, outputTokens: 2, totalTokens: 4 },
          threadId: "thr_1",
          sessionSegmentId: "seg_1",
          requestTurnId: "turn_user_1",
          responseTurnId: "turn_assistant_1",
          continuityPreviewId: "contprev_1",
          continuityApplied: true,
          continuityStatus: "applied",
          continuityIncludedCount: 1,
          continuityExcludedCount: 0
        });
      })
    });

    const response = await client.queryChat({
      query: " follow up ",
      threadId: " thr_1 ",
      continuity: { mode: "auto" }
    });

    expect(body).toMatchObject({
      query: "follow up",
      threadId: "thr_1",
      continuity: { mode: "auto" }
    });
    expect(response.threadId).toBe("thr_1");
    expect(response.continuityApplied).toBe(true);
    expect(response.continuityIncludedCount).toBe(1);
  });

  it("keeps single-turn chat requests compatible by omitting empty continuity fields", async () => {
    let body: Record<string, unknown> = {};
    const client = createKuraClient({
      baseURL: "http://127.0.0.1:19192/",
      fetchImpl: vi.fn<typeof fetch>(async (_input, init) => {
        body = JSON.parse(String(init?.body));
        return mockJSONResponse(200, {
          dispatchId: "dispatch_1",
          provider: "openai_compatible",
          model: "gpt-test",
          skills: [],
          query: "hello",
          status: "completed",
          partial: false,
          reply: "answer",
          usage: { inputTokens: 1, outputTokens: 1, totalTokens: 2 }
        });
      })
    });

    const response = await client.queryChat({ query: " hello ", threadId: " " });

    expect(body).not.toHaveProperty("threadId");
    expect(body).not.toHaveProperty("continuity");
    expect(response.threadId).toBeUndefined();
    expect(response.continuityApplied).toBeUndefined();
  });

  it("exposes reset-boundary continuity metadata on chat responses", async () => {
    const client = createKuraClient({
      baseURL: "http://127.0.0.1:19192/",
      fetchImpl: vi.fn<typeof fetch>(async () => mockJSONResponse(200, {
        dispatchId: "dispatch_reset",
        provider: "openai_compatible",
        model: "gpt-test",
        skills: [],
        query: "after reset",
        status: "completed",
        partial: false,
        reply: "answer",
        usage: { inputTokens: 1, outputTokens: 1, totalTokens: 2 },
        threadId: "thr_reset",
        sessionSegmentId: "seg_new",
        requestTurnId: "turn_user",
        responseTurnId: "turn_assistant",
        continuityPreviewId: "contprev_reset",
        continuityApplied: false,
        continuityStatus: "empty",
        continuityIncludedCount: 0,
        continuityExcludedCount: 1
      }))
    });

    const response = await client.queryChat({ query: "after reset", threadId: "thr_reset" });

    expect(response.continuityApplied).toBe(false);
    expect(response.continuityIncludedCount).toBe(0);
    expect(response.continuityExcludedCount).toBe(1);
    expect(response.continuityPreviewId).toBe("contprev_reset");
  });

  it("fetches continuity preview detail and surfaces permission denials", async () => {
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(mockJSONResponse(200, {
        preview: {
          continuityPreviewId: "contprev_1",
          threadId: "thr_1",
          sessionSegmentId: "seg_1",
          continuityApplied: true,
          status: "applied",
          includedCount: 1,
          excludedCount: 1
        },
        items: [
          { itemKind: "turn", decision: "included", reasonCode: "included_recent", continuityTurnId: "turn_1", acceptanceSequence: 1, safeSummary: "prior", redactionStatus: "redacted" },
          { itemKind: "turn", decision: "excluded", reasonCode: "reset_boundary", continuityTurnId: "turn_0", acceptanceSequence: 0, safeSummary: "pre reset", redactionStatus: "redacted" }
        ]
      }))
      .mockResolvedValueOnce(mockJSONResponse(403, {
        error: "permission_missing",
        denial: { missingPermissions: ["credentials.inspect"] }
      }));
    const client = createKuraClient({
      baseURL: "http://127.0.0.1:19192/",
      fetchImpl
    });

    const detail = await client.getThreadContinuityPreview("thr_1", "contprev_1");

    expect(fetchImpl).toHaveBeenNthCalledWith(1, "http://127.0.0.1:19192/v1/threads/thr_1/continuity-previews/contprev_1", expect.any(Object));
    expect(detail.items).toHaveLength(2);
    expect(detail.items[1].reasonCode).toBe("reset_boundary");
    await expect(client.getThreadContinuityPreview("thr_1", "contprev_denied")).rejects.toMatchObject({ status: 403 });
  });
});
