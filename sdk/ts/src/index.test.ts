import { describe, expect, it, vi } from "vitest";

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
      baseURL: "http://127.0.0.1:19192/",
      accessToken: "token",
      fetchImpl: async (input: string | URL | Request, init?: RequestInit) => {
        url = String(input);
        authorization = String((init?.headers as Record<string, string>).Authorization);
        return mockJSONResponse(200, {
          dispatchId: "dispatch_1",
          provider: "openai_compatible",
          model: "gpt-test",
          skills: ["shared"],
          query: "hello",
          status: "completed",
          partial: false,
          reply: "world",
          usage: { inputTokens: 1, outputTokens: 1, totalTokens: 2 }
        });
      }
    });

    const response = await client.queryChat({ query: "hello", skills: [" shared ", ""] });
    expect(url).toBe("http://127.0.0.1:19192/v1/chat/query");
    expect(authorization).toBe("Bearer token");
    expect(response.reply).toBe("world");
    expect(response.skills).toEqual(["shared"]);
  });

  it("loads operator surfaces, approvals, details, and run creation with normalized URLs", async () => {
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(mockJSONResponse(200, {
        environmentScope: "test",
        status: "ready_for_action",
        blockingItemIds: [],
        optionalFollowUpItemIds: [],
        readinessItems: [],
        firstUsefulActions: [],
        lastEvaluatedAt: "2026-04-24T10:00:00Z"
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        environmentScope: "test",
        items: [],
        generatedAt: "2026-04-24T10:00:00Z"
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        environmentScope: "test",
        items: [],
        generatedAt: "2026-04-24T10:00:00Z"
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        items: [{ approvalId: "approval_1", action: "workflow.launch", reason: "review", status: "pending", createdAt: "2026-04-24T10:00:00Z", updatedAt: "2026-04-24T10:00:00Z" }]
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        runId: "run_1",
        entrypoint: "operator.shell.test",
        status: "queued",
        goal: "shell smoke",
        createdAt: "2026-04-24T10:00:00Z",
        updatedAt: "2026-04-24T10:00:00Z"
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        approval: {
          approvalId: "approval_1",
          action: "workflow.launch",
          reason: "review",
          status: "approved",
          createdAt: "2026-04-24T10:00:00Z",
          updatedAt: "2026-04-24T10:01:00Z",
          resolvedAt: "2026-04-24T10:01:00Z",
          resolution: "approved"
        },
        decision: {
          decisionId: "decision_1",
          action: "workflow.launch",
          outcome: "approved",
          reason: "review",
          approvalId: "approval_1",
          createdAt: "2026-04-24T10:01:00Z"
        }
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        runId: "run_1",
        entrypoint: "operator.shell.test",
        status: "completed",
        goal: "shell smoke",
        createdAt: "2026-04-24T10:00:00Z",
        updatedAt: "2026-04-24T10:02:00Z"
      }));

    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19192/",
      accessToken: "token",
      fetchImpl
    });

    await client.getOnboarding();
    await client.getActivity({ attentionOnly: true, limit: 5 });
    await client.getDiagnostics({ plane: "delivery", severity: "critical" });
    await client.listApprovals("pending");
    await client.createRun({ entrypoint: "operator.shell.test", goal: "shell smoke" });
    await client.resolveApproval("approval_1", { resolution: "approved", comment: "ok" });
    const detail = await client.fetchRoute<{ runId: string }>("v1/runs/run_1");

    expect(fetchImpl).toHaveBeenNthCalledWith(1, "http://127.0.0.1:19192/v1/operator/onboarding", expect.anything());
    expect(fetchImpl).toHaveBeenNthCalledWith(2, "http://127.0.0.1:19192/v1/operator/activity?attentionOnly=true&limit=5", expect.anything());
    expect(fetchImpl).toHaveBeenNthCalledWith(3, "http://127.0.0.1:19192/v1/operator/diagnostics?plane=delivery&severity=critical", expect.anything());
    expect(fetchImpl).toHaveBeenNthCalledWith(4, "http://127.0.0.1:19192/v1/policy/approvals?status=pending", expect.anything());
    expect(fetchImpl).toHaveBeenNthCalledWith(5, "http://127.0.0.1:19192/v1/runs", expect.objectContaining({ method: "POST" }));
    expect(fetchImpl).toHaveBeenNthCalledWith(6, "http://127.0.0.1:19192/v1/policy/approvals/approval_1/resolve", expect.objectContaining({ method: "POST" }));
    expect(fetchImpl).toHaveBeenNthCalledWith(7, "http://127.0.0.1:19192/v1/runs/run_1", expect.anything());
    expect(detail.runId).toBe("run_1");
  });

  it("streams chat events until terminal response", async () => {
    const encoder = new TextEncoder();
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(encoder.encode("event: chat.query.started\ndata: {\"dispatchId\":\"dispatch_1\",\"provider\":\"openai_compatible\",\"model\":\"gpt-test\",\"skills\":[\"shared\"],\"query\":\"hello\"}\n\n"));
        controller.enqueue(encoder.encode("event: chat.query.delta\ndata: {\"dispatchId\":\"dispatch_1\",\"delta\":\"hel\",\"reply\":\"hel\"}\n\n"));
        controller.enqueue(encoder.encode("event: chat.query.delta\ndata: {\"dispatchId\":\"dispatch_1\",\"delta\":\"lo\",\"reply\":\"hello\"}\n\n"));
        controller.enqueue(encoder.encode("event: chat.query.completed\ndata: {\"dispatchId\":\"dispatch_1\",\"provider\":\"openai_compatible\",\"model\":\"gpt-test\",\"skills\":[\"shared\"],\"query\":\"hello\",\"status\":\"completed\",\"partial\":false,\"reply\":\"hello\",\"usage\":{\"inputTokens\":1,\"outputTokens\":1,\"totalTokens\":2}}\n\n"));
        controller.close();
      }
    });

    const deltas: string[] = [];
    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19192",
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

  it("streams daemon events and surfaces them to handlers", async () => {
    const encoder = new TextEncoder();
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(encoder.encode(": stream-open\n\n"));
        controller.enqueue(encoder.encode("id: 12\nevent: policy.approval_requested\ndata: {\"eventId\":\"evt_1\",\"sequence\":12,\"category\":\"policy\",\"name\":\"policy.approval_requested\",\"occurredAt\":\"2026-04-24T10:00:00Z\",\"scope\":{\"runId\":\"run_1\"},\"resource\":{\"kind\":\"approval\",\"id\":\"approval_1\"},\"payload\":{\"status\":\"pending\"}}\n\n"));
        controller.close();
      }
    });

    const seen: string[] = [];
    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19192",
      accessToken: "token",
      fetchImpl: async () =>
        new Response(body, {
          status: 200,
          headers: { "Content-Type": "text/event-stream" }
        })
    });

    const subscription = client.streamEvents({ category: "policy", cursor: 10 }, {
      onEvent(event) {
        seen.push(`${event.name}:${event.sequence}`);
      }
    });

    await subscription.completed;
    expect(seen).toEqual(["policy.approval_requested:12"]);
  });

  it("maps error responses into DopeClientError", async () => {
    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19192",
      fetchImpl: async () => mockJSONResponse(502, { error: "bad key", errorCode: "upstream_auth_failed" })
    });

    await expect(client.queryChat({ query: "hello" })).rejects.toMatchObject({
      name: "DopeClientError",
      status: 502,
      code: "upstream_auth_failed",
      message: "bad key"
    });
  });

  it("binds the default fetch implementation to the browser global", async () => {
    const originalFetch = globalThis.fetch;
    let observedThis: unknown;

    globalThis.fetch = function (this: unknown, input: string | URL | Request, init?: RequestInit): Promise<Response> {
      observedThis = this;
      return Promise.resolve(mockJSONResponse(200, {
        dispatchId: "dispatch_1",
        provider: "openai_compatible",
        model: "gpt-test",
        skills: [],
        query: "hello",
        status: "completed",
        partial: false,
        reply: "world",
        usage: { inputTokens: 1, outputTokens: 1, totalTokens: 2 }
      }));
    } as typeof fetch;

    try {
      const client = createDopeClient({
        baseURL: "http://127.0.0.1:19192"
      });

      await client.queryChat({ query: "hello" });
      expect(observedThis).toBe(globalThis);
    } finally {
      globalThis.fetch = originalFetch;
    }
  });
});
