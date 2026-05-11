import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";

import { ThreadLifecycleView } from "./thread-lifecycle";

describe("ThreadLifecycleView", () => {
  it("exports the thread lifecycle view skeleton", () => {
    expect(ThreadLifecycleView).toBeTypeOf("function");
  });

  it("renders thread list metadata and empty state", () => {
    render(
      <ThreadLifecycleView
        threads={{
          tenantId: "ten_threads",
          page: { limit: 20, order: "active_recent_archived_id" },
          items: [
            {
              threadId: "thr_1",
              tenantId: "ten_threads",
              lifecycleState: "active",
              sourceKind: "channel",
              sourceSummary: "Slack Main / #support",
              lastActivityAt: "2026-05-11T10:00:00Z",
              availableActions: ["reset", "archive"],
              redactionStatus: "redacted",
              updatedAt: "2026-05-11T10:00:00Z"
            }
          ]
        }}
      />
    );
    expect(screen.getByText("thr_1")).toBeTruthy();
    expect(screen.getByText("Slack Main / #support")).toBeTruthy();
  });

  it("renders loading, empty, error, denied, stale-permission, pagination, and detail states", async () => {
    const onRefresh = vi.fn();
    const onNextPage = vi.fn();
    const onSelectThread = vi.fn();
    const onResetThread = vi.fn();
    const onArchiveThread = vi.fn();
    const onReopenThread = vi.fn();
    const user = userEvent.setup();
    const { rerender } = render(<ThreadLifecycleView loading />);
    expect(screen.getByText("Loading threads.")).toBeTruthy();

    rerender(<ThreadLifecycleView threads={{ tenantId: "ten_threads", page: { limit: 20, order: "active_recent_archived_id" }, items: [] }} />);
    expect(screen.getByText("No tenant threads are available.")).toBeTruthy();

    rerender(<ThreadLifecycleView error="load failed" />);
    expect(screen.getByText("load failed")).toBeTruthy();

    rerender(<ThreadLifecycleView denied stalePermission onRefresh={onRefresh} />);
    expect(screen.getByText("credentials.inspect is required to inspect tenant threads.")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Refresh" }));
    expect(onRefresh).toHaveBeenCalled();

    rerender(
      <ThreadLifecycleView
        threads={{
          tenantId: "ten_threads",
          page: { limit: 1, nextCursor: "1", order: "active_recent_archived_id" },
          items: [{
            threadId: "thr_2",
            tenantId: "ten_threads",
            lifecycleState: "archived",
            sourceKind: "legacy",
            lastActivityAt: "2026-05-11T10:00:00Z",
            availableActions: ["reset", "archive", "reopen"],
            redactionStatus: "suppressed",
            updatedAt: "2026-05-11T10:00:00Z"
          }]
        }}
        detail={{
          thread: {
            threadId: "thr_2",
            tenantId: "ten_threads",
            lifecycleState: "archived",
            sourceKind: "legacy",
            currentSessionId: "sess_2",
            lastActivityAt: "2026-05-11T10:00:00Z",
            availableActions: ["reopen"],
            redactionStatus: "suppressed",
            retentionExpiresAt: "2026-08-09T10:00:00Z",
            updatedAt: "2026-05-11T10:00:00Z"
          },
          sessionSegments: [],
          sourceLinkages: [{
            sourceLinkageId: "src_1",
            sourceKind: "channel",
            connectorId: "slack-main",
            connectorKind: "slack",
            sourceConversationId: "channel_redacted",
            routingOutcome: "accepted",
            current: true,
            retentionExpiresAt: "2026-08-09T10:00:00Z",
            redactionStatus: "redacted"
          }],
          runtimeProjections: [{
            runtimeProjectionId: "rtp_1",
            resourceKind: "foreground_reply",
            resourceId: "delivery_1",
            status: "replied",
            reasonCode: "accepted",
            occurredAt: "2026-05-11T10:00:00Z",
            safeSummary: "Foreground reply replied",
            retentionExpiresAt: "2026-08-09T10:00:00Z",
            redactionStatus: "redacted"
          }],
          lifecycleActions: []
        }}
        onNextPage={onNextPage}
        onSelectThread={onSelectThread}
        onResetThread={onResetThread}
        onArchiveThread={onArchiveThread}
        onReopenThread={onReopenThread}
      />
    );
    expect(screen.getByText("legacy")).toBeTruthy();
    expect(screen.getByText("sess_2")).toBeTruthy();
    expect(screen.getByText("Source Trace")).toBeTruthy();
    expect(screen.getByText("Runtime Trace")).toBeTruthy();
    expect(screen.getByText("Lifecycle metadata, not assistant memory.")).toBeTruthy();
    expect(screen.getAllByText("2026-08-09T10:00:00Z").length).toBeGreaterThan(0);
    expect(screen.getByText("accepted")).toBeTruthy();
    expect(screen.getByText("Foreground reply replied")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Inspect" }));
    await user.click(screen.getByRole("button", { name: "Reset" }));
    await user.click(screen.getByRole("button", { name: "Archive" }));
    await user.click(screen.getByRole("button", { name: "Reopen" }));
    await user.click(screen.getByRole("button", { name: "Next page" }));
    expect(onSelectThread).toHaveBeenCalledWith("thr_2");
    expect(onResetThread).toHaveBeenCalledWith("thr_2");
    expect(onArchiveThread).toHaveBeenCalledWith("thr_2");
    expect(onReopenThread).toHaveBeenCalledWith("thr_2");
    expect(onNextPage).toHaveBeenCalled();
  });
});
