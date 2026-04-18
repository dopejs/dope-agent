import { describe, expect, it } from "vitest";

import { createDopeClient } from "./index.js";

function mockJSONResponse(status: number, payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: {
      "Content-Type": "application/json"
    }
  });
}

describe("DopeClient", () => {
  it("sends a non-stream chat request", async () => {
    let url = "";
    let authorization = "";

    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19191/",
      accessToken: "token",
      fetchImpl: async (input: string | URL | Request, init?: RequestInit) => {
        url = String(input);
        authorization = String((init?.headers as Record<string, string>).Authorization);
        return mockJSONResponse(200, {
          dispatchId: "dispatch_1",
          provider: "openai_compatible",
          model: "gpt-test",
          query: "hello",
          status: "completed",
          reply: "world",
          usage: { inputTokens: 1, outputTokens: 1, totalTokens: 2 }
        });
      }
    });

    const response = await client.queryChat({ query: "hello" });
    expect(url).toBe("http://127.0.0.1:19191/v1/chat/query");
    expect(authorization).toBe("Bearer token");
    expect(response.reply).toBe("world");
  });

  it("streams chat events until terminal response", async () => {
    const encoder = new TextEncoder();
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(encoder.encode("event: chat.query.started\ndata: {\"dispatchId\":\"dispatch_1\",\"provider\":\"openai_compatible\",\"model\":\"gpt-test\",\"query\":\"hello\"}\n\n"));
        controller.enqueue(encoder.encode("event: chat.query.delta\ndata: {\"dispatchId\":\"dispatch_1\",\"delta\":\"hel\",\"reply\":\"hel\"}\n\n"));
        controller.enqueue(encoder.encode("event: chat.query.delta\ndata: {\"dispatchId\":\"dispatch_1\",\"delta\":\"lo\",\"reply\":\"hello\"}\n\n"));
        controller.enqueue(encoder.encode("event: chat.query.completed\ndata: {\"dispatchId\":\"dispatch_1\",\"provider\":\"openai_compatible\",\"model\":\"gpt-test\",\"query\":\"hello\",\"status\":\"completed\",\"reply\":\"hello\",\"usage\":{\"inputTokens\":1,\"outputTokens\":1,\"totalTokens\":2}}\n\n"));
        controller.close();
      }
    });

    const deltas: string[] = [];
    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19191",
      fetchImpl: async () =>
        new Response(body, {
          status: 200,
          headers: { "Content-Type": "text/event-stream" }
        })
    });

    const response = await client.streamChatQuery({ query: "hello" }, {
      onDelta(payload: { reply: string }) {
        deltas.push(payload.reply);
      }
    });

    expect(deltas).toEqual(["hel", "hello"]);
    expect(response.reply).toBe("hello");
  });

  it("maps error responses into DopeClientError", async () => {
    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19191",
      fetchImpl: async () => mockJSONResponse(502, { error: "bad key", errorCode: "upstream_auth_failed" })
    });

    await expect(client.queryChat({ query: "hello" })).rejects.toMatchObject({
      name: "DopeClientError",
      status: 502,
      code: "upstream_auth_failed",
      message: "bad key"
    });
  });
});
