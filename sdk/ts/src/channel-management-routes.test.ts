import { describe, expect, it, vi } from "vitest";

import { createKuraClient } from "./index.js";

function response(payload: unknown): Response {
  return new Response(JSON.stringify(payload), { status: 200, headers: { "Content-Type": "application/json" } });
}

describe("channel management routes SDK", () => {
  it("routes policy and outcome reads separately", async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockImplementation(() => Promise.resolve(response({ items: [] })));
    const client = createKuraClient({ baseURL: "http://127.0.0.1:19192", fetchImpl });

    await client.getChannelRoutePolicy("matrix-main");
    await client.updateChannelRoutePolicy("matrix-main", { eligibleRooms: ["room_redacted"] });
    await client.listChannelReplyOutcomes("matrix-main");
    await client.listChannelDeliveryOutcomes("matrix-main");

    expect(fetchImpl.mock.calls.map((call) => `${call[1]?.method ?? "GET"} ${call[0]}`)).toEqual([
      "GET http://127.0.0.1:19192/v1/channel-management/connectors/matrix-main/route-policy",
      "PUT http://127.0.0.1:19192/v1/channel-management/connectors/matrix-main/route-policy",
      "GET http://127.0.0.1:19192/v1/channel-management/connectors/matrix-main/reply-outcomes",
      "GET http://127.0.0.1:19192/v1/channel-management/connectors/matrix-main/delivery-outcomes"
    ]);
  });
});
