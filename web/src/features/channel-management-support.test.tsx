import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { ChannelManagementView } from "./channel-management";

describe("ChannelManagementView support evidence", () => {
  afterEach(() => cleanup());

  it("renders metadata-only support evidence redaction state", () => {
    render(
      <ChannelManagementView
        selected={{
          connectorId: "slack-main",
          connectorKind: "slack",
          displayName: "Slack Main",
          enablementState: "degraded",
          setupState: "degraded",
          healthStatus: "degraded",
          diagnosticFreshness: "fresh",
          deliveryEligible: false,
          capabilities: {},
          redactionStatus: "redacted",
          updatedAt: "2026-05-10T10:00:00Z",
          supportEvidenceAvailable: true
        }}
        supportEvidence={{
          supportEvidenceId: "support_1",
          tenantId: "ten_channels",
          connectorId: "slack-main",
          generatedAt: "2026-05-10T10:00:00Z",
          currentState: "degraded",
          retentionExpiresAt: "2026-08-08T10:00:00Z",
          redactionStatus: "redacted"
        }}
      />
    );

    expect(screen.getByLabelText("Support evidence")).not.toBeNull();
    expect(screen.getByText("redacted")).not.toBeNull();
  });
});
