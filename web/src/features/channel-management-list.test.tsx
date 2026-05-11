import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ChannelManagementView } from "./channel-management";

describe("ChannelManagementView", () => {
  afterEach(() => cleanup());

  it("renders ordered fleet state, next action, diagnostics, delivery, and support evidence sections", () => {
    render(
      <ChannelManagementView
        connectors={{
          tenantId: "ten_channels",
          page: { limit: 20, order: "attention_disabled_ready_name_id" },
          items: [{
            connectorId: "slack-main",
            connectorKind: "slack",
            displayName: "Slack Main",
            enablementState: "action-required",
            setupState: "action-required",
            healthStatus: "permission_blocked",
            diagnosticFreshness: "fresh",
            deliveryEligible: false,
            nextAction: { actionKind: "reconnect", label: "Reconnect authorization", reasonCode: "permission_missing" },
            capabilities: { reconnect: "supported", "credential-rotation": "limited" },
            redactionStatus: "redacted",
            updatedAt: "2026-05-10T10:00:00Z"
          }]
        }}
        selected={{
          connectorId: "slack-main",
          connectorKind: "slack",
          displayName: "Slack Main",
          enablementState: "action-required",
          setupState: "action-required",
          healthStatus: "permission_blocked",
          diagnosticFreshness: "fresh",
          deliveryEligible: false,
          capabilities: {},
          redactionStatus: "redacted",
          updatedAt: "2026-05-10T10:00:00Z",
          diagnosticSummary: {
            diagnosticStateId: "diag_1",
            connectorId: "slack-main",
            status: "permission_blocked",
            reasonCode: "permission_missing",
            remediationOwner: "tenant_admin",
            retrySafety: "blocked",
            evidenceTimestamp: "2026-05-10T10:00:00Z",
            freshnessState: "fresh",
            redactionStatus: "redacted",
            retentionExpiresAt: "2026-08-08T10:00:00Z"
          },
          foregroundReplyOutcomes: [],
          backgroundDeliveryOutcomes: [],
          repairActions: [],
          supportEvidenceAvailable: true
        }}
        supportEvidence={{
          supportEvidenceId: "support_1",
          tenantId: "ten_channels",
          connectorId: "slack-main",
          generatedAt: "2026-05-10T10:00:00Z",
          currentState: "action-required",
          retentionExpiresAt: "2026-08-08T10:00:00Z",
          redactionStatus: "redacted"
        }}
      />
    );

    expect(screen.getAllByText("Slack Main").length).toBeGreaterThan(0);
    expect(screen.getByText("Reconnect authorization")).not.toBeNull();
    expect(screen.getByLabelText("Diagnostics")).not.toBeNull();
    expect(screen.getByLabelText("Foreground replies")).not.toBeNull();
    expect(screen.getByLabelText("Background delivery")).not.toBeNull();
    expect(screen.getByLabelText("Support evidence")).not.toBeNull();
    expect(screen.getByText("redacted")).not.toBeNull();
  });

  it("renders empty and denial states and invokes enablement actions", async () => {
    const user = userEvent.setup();
    const onDisableConnector = vi.fn();
    const onReEnableConnector = vi.fn();
    const { rerender } = render(<ChannelManagementView connectors={{ page: { limit: 20, order: "attention_disabled_ready_name_id" }, items: [] }} />);
    expect(screen.getByText("No production channel connectors are configured.")).not.toBeNull();

    rerender(<ChannelManagementView error="permission_missing" />);
    expect(screen.getAllByText("permission_missing").length).toBeGreaterThan(0);

    rerender(
      <ChannelManagementView
        connectors={{ page: { limit: 20, order: "attention_disabled_ready_name_id" }, items: [] }}
        selected={{
          connectorId: "disabled-main",
          connectorKind: "telegram",
          displayName: "Disabled Main",
          enablementState: "disabled",
          setupState: "disabled",
          healthStatus: "disabled",
          diagnosticFreshness: "stale",
          deliveryEligible: false,
          capabilities: {},
          redactionStatus: "redacted",
          updatedAt: "2026-05-10T10:00:00Z",
          supportEvidenceAvailable: true
        }}
        onDisableConnector={onDisableConnector}
        onReEnableConnector={onReEnableConnector}
      />
    );
    await user.click(screen.getByRole("button", { name: "Re-enable" }));
    expect(onReEnableConnector).toHaveBeenCalledWith("disabled-main");
    expect(onDisableConnector).not.toHaveBeenCalled();
  });
});
