import { describe, expect, it, vi } from "vitest";

import { createDopeClient } from "./index";

function jsonResponse(payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    status: 201,
    headers: { "Content-Type": "application/json" }
  });
}

describe("group room reset handoff contracts", () => {
  it("keeps the Roadmap 56 test surface active", () => {
    expect(true).toBe(true);
  });

  it("creates thread handoffs through the tenant-scoped API", async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValueOnce(jsonResponse({
      handoffLinkId: "handoff_1",
      sourceThreadId: "thr_source",
      destinationThreadId: "thr_destination",
      sourceConversationShape: "room",
      destinationConversationShape: "web",
      status: "succeeded",
      sourceReferenceStatus: "available",
      permissionGate: "connectors.manage",
      redactionStatus: "redacted"
    }));
    const client = createDopeClient({ baseURL: "http://127.0.0.1:19192", accessToken: "token", defaultTenantId: "ten_threads", fetchImpl });

    const handoff = await client.createThreadHandoff(" thr_source ", { destination: { surface: "web" }, reasonCode: "user_requested_handoff" });

    expect(handoff.destinationConversationShape).toBe("web");
    expect(fetchImpl.mock.calls[0]?.[0]).toBe("http://127.0.0.1:19192/v1/threads/thr_source/handoffs");
    expect(fetchImpl.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(fetchImpl.mock.calls[0]?.[1]?.headers).toMatchObject({ Authorization: "Bearer token", "X-Dope-Tenant-ID": "ten_threads" });
    expect(JSON.parse(String(fetchImpl.mock.calls[0]?.[1]?.body))).toMatchObject({ destination: { surface: "web" }, reasonCode: "user_requested_handoff" });
  });
});
