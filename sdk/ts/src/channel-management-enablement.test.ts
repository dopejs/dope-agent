import { describe, expect, it, vi } from "vitest";

import { createKuraClient } from "./index.js";

function response(payload: unknown): Response {
  return new Response(JSON.stringify(payload), { status: 200, headers: { "Content-Type": "application/json" } });
}

describe("channel management enablement SDK", () => {
  it("sends disable and re-enable mutations to connector-specific routes", async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockImplementation(() => Promise.resolve(response({ connectorId: "slack-main", enablementState: "disabled", deliveryEligible: false, auditEventId: "audit_1", changedAt: "2026-05-10T10:00:00Z" })));
    const client = createKuraClient({ baseURL: "http://127.0.0.1:19192", fetchImpl });

    await client.disableChannelConnector("slack-main", { reasonCode: "maintenance" });
    await client.reEnableChannelConnector("slack-main");

    expect(fetchImpl.mock.calls.map((call) => `${call[1]?.method} ${call[0]}`)).toEqual([
      "POST http://127.0.0.1:19192/v1/channel-management/connectors/slack-main/disable",
      "POST http://127.0.0.1:19192/v1/channel-management/connectors/slack-main/re-enable"
    ]);
  });
});
