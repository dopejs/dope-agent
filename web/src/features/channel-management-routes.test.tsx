import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ChannelManagementView } from "./channel-management";

describe("ChannelManagementView route and delivery sections", () => {
  afterEach(() => cleanup());

  it("keeps route policy, routing decisions, foreground replies, and background delivery visible as separate sections", async () => {
    const user = userEvent.setup();
    const onUpdateRoutePolicy = vi.fn();
    render(
      <ChannelManagementView
        selected={{
          connectorId: "telegram-main",
          connectorKind: "telegram",
          displayName: "Telegram Main",
          enablementState: "ready",
          setupState: "ready",
          healthStatus: "healthy",
          diagnosticFreshness: "fresh",
          deliveryEligible: true,
          capabilities: {},
          redactionStatus: "redacted",
          updatedAt: "2026-05-10T10:00:00Z",
          routePolicy: {
            tenantId: "ten_channels",
            connectorId: "telegram-main",
            eligibleRooms: ["room_existing"],
            eligibleChannels: [],
            eligibleConversations: [],
            eligibleSenders: [],
            backgroundDeliveryEligible: true,
            validationState: "valid",
            validatedAt: "2026-05-10T10:00:00Z",
            redactionStatus: "redacted"
          },
          recentRouteDecisions: [{ outcome: "blocked", reasonCode: "blocked_route" }],
          foregroundReplyOutcomes: [{ status: "failed", reasonCode: "provider_unavailable" }],
          backgroundDeliveryOutcomes: [{ status: "blocked", reasonCode: "connector_disabled" }],
          supportEvidenceAvailable: true
        }}
        onUpdateRoutePolicy={onUpdateRoutePolicy}
      />
    );

    expect(screen.getByLabelText("Route policy")).not.toBeNull();
    expect(screen.getByLabelText("Routing decisions")).not.toBeNull();
    expect(screen.getByLabelText("Foreground replies")).not.toBeNull();
    expect(screen.getByLabelText("Background delivery")).not.toBeNull();
    await user.clear(screen.getByLabelText("Eligible rooms"));
    await user.type(screen.getByLabelText("Eligible rooms"), "room_new");
    await user.click(screen.getByRole("button", { name: "Save route policy" }));
    expect(onUpdateRoutePolicy).toHaveBeenCalledWith("telegram-main", expect.objectContaining({ eligibleRooms: ["room_new"] }));
  });
});
