import { describe, expect, it, vi } from "vitest";

import { createDopeClient } from "./index";

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), { status, headers: { "Content-Type": "application/json" } });
}

describe("plugin assembly SDK methods", () => {
  it("fetches the boot-time plugin assembly report", async () => {
    const report = {
      plugins: [
        {
          id: "llm",
          summary: "LLM dispatcher",
          source: "builtin",
          enabled: true,
          provides: ["llm.dispatcher"],
          requires: [],
        },
        {
          id: "webhooks",
          summary: "Webhook ingress",
          source: "builtin",
          enabled: false,
          reason: "requires disabled plugin `billing`",
          provides: ["webhooks.manager"],
          requires: ["billing"],
        },
      ],
      warnings: ["profile disables unknown plugin `ghost`"],
      hooks: [{ point: "chat/pre-dispatch", pluginId: "session-strategy" }],
    };
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValueOnce(jsonResponse(report));
    const client = createDopeClient({ baseURL: "https://daemon.test", fetchImpl });

    const result = await client.listPlugins();
    expect(fetchImpl).toHaveBeenCalledWith(
      "https://daemon.test/v1/plugins",
      expect.objectContaining({ method: "GET" }),
    );
    expect(result.plugins).toHaveLength(2);
    expect(result.plugins[1].reason).toContain("billing");
    expect(result.warnings).toHaveLength(1);
    expect(result.hooks?.[0].point).toBe("chat/pre-dispatch");
  });
});
