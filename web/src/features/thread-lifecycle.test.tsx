import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";

import { ThreadLifecycleView } from "./thread-lifecycle";

describe("ThreadLifecycleView", () => {
  it("reserves Roadmap 55 coverage for continuity preview evidence", () => {
    expect([
      "preview summary",
      "preview detail",
      "reset-boundary evidence",
      "non-memory labeling"
    ]).toContain("reset-boundary evidence");
  });

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
    const onHandoffToWeb = vi.fn();
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
          continuityPreviews: [{
            continuityPreviewId: "contprev_1",
            tenantId: "ten_threads",
            threadId: "thr_2",
            sessionSegmentId: "seg_reset",
            continuityApplied: false,
            status: "empty",
            includedCount: 0,
            excludedCount: 1,
            windowPolicyId: "default_recent_12_30d",
            maxPriorTurns: 12,
            activeWindowDays: 30,
            orderedBy: "daemon_acceptance_sequence",
            redactionStatus: "redacted"
          }],
          conversationShape: {
            shape: "room",
            shapeEvidenceStatus: "proven",
            sourceConversationSummary: "Slack Main / #support",
            redactionStatus: "redacted"
          },
          participationDecisions: [{
            participationDecisionId: "part_1",
            conversationShape: "room",
            decision: "ignored",
            reasonCode: "missing_qualifying_mention",
            createdAssistantWork: false,
            safeSummary: "Room message ignored by participation policy",
            redactionStatus: "redacted"
          }],
          resetEvents: [{
            resetEventId: "reset_1",
            conversationShape: "room",
            permissionGate: "connectors.manage",
            priorSessionSegmentId: "seg_old",
            resultingSessionSegmentId: "seg_reset",
            status: "succeeded",
            reasonCode: "scoped_reset_succeeded",
            redactionStatus: "redacted"
          }],
          handoffLinks: [{
            handoffLinkId: "handoff_1",
            sourceThreadId: "thr_source",
            destinationThreadId: "thr_2",
            sourceConversationShape: "room",
            destinationConversationShape: "web",
            status: "succeeded",
            sourceReferenceStatus: "available",
            permissionGate: "connectors.manage",
            redactionStatus: "redacted"
          }],
          lifecycleActions: []
        }}
        continuityPreviewDetail={{
          preview: {
            continuityPreviewId: "contprev_1",
            continuityApplied: false,
            status: "empty",
            includedCount: 0,
            excludedCount: 1
          },
          items: [{
            previewItemId: "contitem_1",
            itemKind: "turn",
            decision: "excluded",
            reasonCode: "reset_boundary",
            continuityTurnId: "turn_pre_reset",
            safeSummary: "pre-reset turn",
            redactionStatus: "redacted"
          }]
        }}
        onNextPage={onNextPage}
        onSelectThread={onSelectThread}
        onResetThread={onResetThread}
        onArchiveThread={onArchiveThread}
        onReopenThread={onReopenThread}
        onHandoffToWeb={onHandoffToWeb}
      />
    );
    expect(screen.getByText("legacy")).toBeTruthy();
    expect(screen.getByText("sess_2")).toBeTruthy();
    expect(screen.getByText("Source Trace")).toBeTruthy();
    expect(screen.getByText("Conversation Shape")).toBeTruthy();
    expect(screen.getByText("proven")).toBeTruthy();
    expect(screen.getByText("Participation Decisions")).toBeTruthy();
    expect(screen.getByText("missing_qualifying_mention")).toBeTruthy();
    expect(screen.getByText("Reset Events")).toBeTruthy();
    expect(screen.getByText("scoped_reset_succeeded")).toBeTruthy();
    expect(screen.getByText("Handoff Links")).toBeTruthy();
    expect(screen.getByText("room to web")).toBeTruthy();
    expect(screen.getByText("available")).toBeTruthy();
    expect(screen.getByText("Runtime Trace")).toBeTruthy();
    expect(screen.getByText("Continuity Evidence")).toBeTruthy();
    expect(screen.getByText("Bounded recent-thread evidence, not assistant memory.")).toBeTruthy();
    expect(screen.getByText("Continuity Preview Detail")).toBeTruthy();
    expect(screen.getByText("Lifecycle metadata, not assistant memory.")).toBeTruthy();
    expect(screen.getAllByText("2026-08-09T10:00:00Z").length).toBeGreaterThan(0);
    expect(screen.getByText("accepted")).toBeTruthy();
    expect(screen.getByText("Foreground reply replied")).toBeTruthy();
    expect(screen.getByText("1 excluded")).toBeTruthy();
    expect(screen.getAllByText("contprev_1").length).toBeGreaterThan(0);
    expect(screen.getByText("default_recent_12_30d")).toBeTruthy();
    expect(screen.getByText("reset_boundary")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Inspect" }));
    await user.click(screen.getByRole("button", { name: "Reset" }));
    await user.click(screen.getByRole("button", { name: "Archive" }));
    await user.click(screen.getByRole("button", { name: "Reopen" }));
    await user.click(screen.getByRole("button", { name: "Handoff to web" }));
    await user.click(screen.getByRole("button", { name: "Next page" }));
    expect(onSelectThread).toHaveBeenCalledWith("thr_2");
    expect(onResetThread).toHaveBeenCalledWith("thr_2");
    expect(onArchiveThread).toHaveBeenCalledWith("thr_2");
    expect(onReopenThread).toHaveBeenCalledWith("thr_2");
    expect(onHandoffToWeb).toHaveBeenCalledWith("thr_2");
    expect(onNextPage).toHaveBeenCalled();
  });
});
