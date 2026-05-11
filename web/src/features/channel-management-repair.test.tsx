import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ChannelManagementView } from "./channel-management";

describe("ChannelManagementView repair controls", () => {
  afterEach(() => cleanup());

  it("invokes repair from the selected connector detail", async () => {
    const user = userEvent.setup();
    const onStartRepair = vi.fn();
    render(
      <ChannelManagementView
        selected={{
          connectorId: "matrix-main",
          connectorKind: "matrix",
          displayName: "Matrix Main",
          enablementState: "action-required",
          setupState: "action-required",
          healthStatus: "failed",
          diagnosticFreshness: "fresh",
          deliveryEligible: false,
          capabilities: {},
          redactionStatus: "redacted",
          updatedAt: "2026-05-10T10:00:00Z",
          supportEvidenceAvailable: true
        }}
        onStartRepair={onStartRepair}
      />
    );

    await user.click(screen.getByRole("button", { name: "Repair" }));
    expect(onStartRepair).toHaveBeenCalledWith("matrix-main");
  });
});
