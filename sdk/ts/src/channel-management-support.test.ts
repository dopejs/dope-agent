import { describe, expect, it, vi } from "vitest";

import { createDopeClient } from "./index.js";

describe("channel management support SDK", () => {
  it("reads metadata-only support evidence through the channel management route", async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify({
      supportEvidenceId: "support_1",
      tenantId: "ten_channels",
      connectorId: "slack-main",
      generatedAt: "2026-05-10T10:00:00Z",
      currentState: "action-required",
      retentionExpiresAt: "2026-08-08T10:00:00Z",
      redactionStatus: "redacted"
    }), { status: 200, headers: { "Content-Type": "application/json" } }));
    const client = createDopeClient({ baseURL: "http://127.0.0.1:19192", fetchImpl });

    await expect(client.getChannelConnectorSupportEvidence("slack-main")).resolves.toMatchObject({ redactionStatus: "redacted" });
    expect(fetchImpl).toHaveBeenCalledWith("http://127.0.0.1:19192/v1/channel-management/connectors/slack-main/support-evidence", expect.any(Object));
  });
});
