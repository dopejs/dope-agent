import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ChannelManagementView } from "./channel-management";

describe("ChannelManagementView enablement controls", () => {
  afterEach(() => cleanup());

  it("invokes disable control for enabled connectors", async () => {
    const user = userEvent.setup();
    const onDisableConnector = vi.fn();
    render(
      <ChannelManagementView
        selected={{
          connectorId: "slack-main",
          connectorKind: "slack",
          displayName: "Slack Main",
          enablementState: "ready",
          setupState: "ready",
          healthStatus: "healthy",
          diagnosticFreshness: "fresh",
          deliveryEligible: true,
          capabilities: {},
          redactionStatus: "redacted",
          updatedAt: "2026-05-10T10:00:00Z",
          supportEvidenceAvailable: true
        }}
        onDisableConnector={onDisableConnector}
      />
    );

    await user.click(screen.getByRole("button", { name: "Disable" }));
    expect(onDisableConnector).toHaveBeenCalledWith("slack-main");
  });
});
