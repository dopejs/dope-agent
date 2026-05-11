import { describe, expect, it, vi } from "vitest";

import { createDopeClient } from "./index.js";

function response(payload: unknown): Response {
  return new Response(JSON.stringify(payload), { status: 202, headers: { "Content-Type": "application/json" } });
}

describe("channel management repair SDK", () => {
  it("normalizes repair, reconnect, and credential rotation action kinds", async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockImplementation(() => Promise.resolve(response({ repairActionId: "repair_1" })));
    const client = createDopeClient({ baseURL: "http://127.0.0.1:19192", fetchImpl });

    await client.startChannelConnectorRepair("slack-main");
    await client.reconnectChannelConnector("slack-main");
    await client.rotateChannelConnectorCredentials("slack-main");

    const bodies = fetchImpl.mock.calls.map((call) => JSON.parse(String(call[1]?.body)));
    expect(bodies).toEqual([
      { actionKind: "repair" },
      { actionKind: "reconnect" },
      { actionKind: "credential-rotation" }
    ]);
  });
});
