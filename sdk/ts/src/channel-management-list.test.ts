import { describe, expect, it, vi } from "vitest";

import { createDopeClient } from "./index.js";

function jsonResponse(payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    status: 200,
    headers: { "Content-Type": "application/json" }
  });
}

describe("channel management client", () => {
  it("routes list, detail, and diagnostics through tenant-scoped channel management endpoints", async () => {
    const fetchImpl = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse({ tenantId: "ten_channels", page: { limit: 2, order: "attention_disabled_ready_name_id" }, items: [] }))
      .mockResolvedValueOnce(jsonResponse({ connectorId: "slack-main", connectorKind: "slack", displayName: "Slack Main", enablementState: "ready" }))
      .mockResolvedValueOnce(jsonResponse({ items: [{ diagnosticStateId: "diag_1", connectorId: "slack-main" }] }));
    const client = createDopeClient({ baseURL: "http://127.0.0.1:19192", accessToken: "token", defaultTenantId: "ten_channels", fetchImpl });

    await client.listChannelConnectors({ limit: 2, state: "ready" });
    await client.getChannelConnector(" slack-main ");
    await client.getChannelConnectorDiagnostics("slack-main");

    expect(fetchImpl.mock.calls[0]?.[0]).toBe("http://127.0.0.1:19192/v1/channel-management/connectors?limit=2&state=ready");
    expect(fetchImpl.mock.calls[1]?.[0]).toBe("http://127.0.0.1:19192/v1/channel-management/connectors/slack-main");
    expect(fetchImpl.mock.calls[2]?.[0]).toBe("http://127.0.0.1:19192/v1/channel-management/connectors/slack-main/diagnostics");
    expect(fetchImpl.mock.calls[0]?.[1]?.headers).toMatchObject({ Authorization: "Bearer token", "X-Dope-Tenant-ID": "ten_channels" });
  });

  it("routes enablement, repair, route, outcome, and support evidence methods", async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockImplementation(() => Promise.resolve(jsonResponse({ items: [] })));
    const client = createDopeClient({ baseURL: "http://127.0.0.1:19192", fetchImpl });

    await client.disableChannelConnector("slack-main", { reasonCode: "maintenance" }, { tenantId: "ten_channels" });
    await client.reEnableChannelConnector("slack-main", {}, { tenantId: "ten_channels" });
    await client.startChannelConnectorRepair("slack-main", { actionKind: "repair" }, { tenantId: "ten_channels" });
    await client.reconnectChannelConnector("slack-main", { sourceDiagnosticStateId: "diag_1" }, { tenantId: "ten_channels" });
    await client.rotateChannelConnectorCredentials("slack-main", {}, { tenantId: "ten_channels" });
    await client.getChannelRoutePolicy("slack-main", { tenantId: "ten_channels" });
    await client.updateChannelRoutePolicy("slack-main", { eligibleChannels: ["chan_redacted"] }, { tenantId: "ten_channels" });
    await client.listChannelReplyOutcomes("slack-main", { tenantId: "ten_channels" });
    await client.listChannelDeliveryOutcomes("slack-main", { tenantId: "ten_channels" });
    await client.getChannelConnectorSupportEvidence("slack-main", { tenantId: "ten_channels" });

    const routes = fetchImpl.mock.calls.map((call) => `${(call[1] as RequestInit).method ?? "GET"} ${call[0]}`);
    expect(routes).toEqual([
      "POST http://127.0.0.1:19192/v1/channel-management/connectors/slack-main/disable",
      "POST http://127.0.0.1:19192/v1/channel-management/connectors/slack-main/re-enable",
      "POST http://127.0.0.1:19192/v1/channel-management/connectors/slack-main/repair-actions",
      "POST http://127.0.0.1:19192/v1/channel-management/connectors/slack-main/repair-actions",
      "POST http://127.0.0.1:19192/v1/channel-management/connectors/slack-main/repair-actions",
      "GET http://127.0.0.1:19192/v1/channel-management/connectors/slack-main/route-policy",
      "PUT http://127.0.0.1:19192/v1/channel-management/connectors/slack-main/route-policy",
      "GET http://127.0.0.1:19192/v1/channel-management/connectors/slack-main/reply-outcomes",
      "GET http://127.0.0.1:19192/v1/channel-management/connectors/slack-main/delivery-outcomes",
      "GET http://127.0.0.1:19192/v1/channel-management/connectors/slack-main/support-evidence"
    ]);
    expect(JSON.parse(String((fetchImpl.mock.calls[3]?.[1] as RequestInit).body))).toMatchObject({ actionKind: "reconnect", sourceDiagnosticStateId: "diag_1" });
    expect(JSON.parse(String((fetchImpl.mock.calls[4]?.[1] as RequestInit).body))).toMatchObject({ actionKind: "credential-rotation" });
  });
});
