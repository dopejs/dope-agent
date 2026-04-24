import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { App } from "./App";

const mockClient = {
  getOnboarding: vi.fn(),
  listApprovals: vi.fn(),
  getActivity: vi.fn(),
  getDiagnostics: vi.fn(),
  resolveApproval: vi.fn(),
  fetchRoute: vi.fn(),
  createRun: vi.fn(),
  queryChat: vi.fn(),
  streamEvents: vi.fn()
};

vi.mock("@dope/client", () => ({
  createDopeClient: () => mockClient
}));

const emptySubscription = {
  close: vi.fn(),
  completed: Promise.resolve()
};

function onboardingFixture(overrides: Partial<ReturnType<typeof onboardingFixtureBase>> = {}) {
  return {
    ...onboardingFixtureBase(),
    ...overrides
  };
}

function onboardingFixtureBase() {
  return {
    environmentScope: "test",
    status: "ready_for_action",
    blockingItemIds: [],
    optionalFollowUpItemIds: ["integration-calendar-a"],
    recommendedActionId: "test_run",
    readinessItems: [
      {
        itemId: "auth-token",
        itemKind: "auth",
        displayName: "Operator access token",
        status: "ready",
        requiredForSelectedAction: true,
        environmentScope: "test",
        updatedAt: "2026-04-24T10:00:00Z"
      },
      {
        itemId: "integration-calendar-a",
        itemKind: "integration",
        displayName: "Calendar A",
        status: "degraded",
        reason: "reauth required",
        requiredForSelectedAction: false,
        detailRoute: "/v1/integrations/calendar-a",
        environmentScope: "test",
        updatedAt: "2026-04-24T10:00:00Z"
      }
    ],
    firstUsefulActions: [
      {
        actionId: "test_run",
        actionKind: "test_run",
        displayName: "Launch test run",
        recommended: true,
        available: true,
        invokeRoute: "/v1/runs",
        resultRoute: "/v1/runs"
      },
      {
        actionId: "test_query",
        actionKind: "test_query",
        displayName: "Run test query",
        recommended: false,
        available: true,
        invokeRoute: "/v1/chat/query",
        resultRoute: "/v1/chat/query"
      }
    ],
    lastEvaluatedAt: "2026-04-24T10:00:00Z"
  };
}

describe("App", () => {
  beforeEach(() => {
    mockClient.getOnboarding.mockReset();
    mockClient.listApprovals.mockReset();
    mockClient.getActivity.mockReset();
    mockClient.getDiagnostics.mockReset();
    mockClient.resolveApproval.mockReset();
    mockClient.fetchRoute.mockReset();
    mockClient.createRun.mockReset();
    mockClient.queryChat.mockReset();
    mockClient.streamEvents.mockReset().mockReturnValue(emptySubscription);
  });

  afterEach(() => {
    cleanup();
  });

  it("loads onboarding state and launches the recommended test run", async () => {
    mockClient.getOnboarding
      .mockResolvedValueOnce(onboardingFixture())
      .mockResolvedValueOnce(onboardingFixture({
        status: "completed"
      }));
    mockClient.listApprovals.mockResolvedValue({ items: [] });
    mockClient.getActivity.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });
    mockClient.getDiagnostics.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });
    mockClient.createRun.mockResolvedValue({
      runId: "run_1",
      entrypoint: "operator.shell.test",
      status: "queued",
      goal: "Run an operator shell smoke check.",
      createdAt: "2026-04-24T10:00:00Z",
      updatedAt: "2026-04-24T10:00:00Z"
    });

    render(<App />);
    const user = userEvent.setup();

    await user.type(screen.getByLabelText(/access token/i), "token");
    await user.click(screen.getByRole("button", { name: /load shell/i }));

    await waitFor(() => {
      expect(screen.getByText("Single control surface for onboarding, approvals, activity, and diagnostics.")).not.toBeNull();
      expect(screen.getAllByText("Launch test run").length).toBeGreaterThan(0);
    });

    await user.click(screen.getAllByRole("button", { name: "Launch test run" })[0]);

    await waitFor(() => {
      expect(mockClient.createRun).toHaveBeenCalledWith({
        entrypoint: "operator.shell.test",
        goal: "Run an operator shell smoke check."
      });
      expect(screen.getByText(/Created test run run_1/i)).not.toBeNull();
      expect(screen.getAllByText("completed").length).toBeGreaterThan(0);
    });
  });

  it("resolves approvals and inspects linked activity detail in the shell", async () => {
    mockClient.getOnboarding.mockResolvedValue(onboardingFixture());
    mockClient.listApprovals.mockResolvedValue({
      items: [
        {
          approvalId: "approval_1",
          action: "workflow.launch",
          reason: "needs review",
          status: "pending",
          createdAt: "2026-04-24T10:00:00Z",
          updatedAt: "2026-04-24T10:00:00Z"
        }
      ]
    });
    mockClient.getActivity.mockResolvedValue({
      environmentScope: "test",
      generatedAt: "2026-04-24T10:00:00Z",
      items: [
        {
          activityId: "workflow-wf_1",
          sourceKind: "workflow",
          sourceId: "wf_1",
          title: "Workflow wf_1",
          status: "failed",
          summary: "workflow failed",
          attentionLevel: "critical",
          occurredAt: "2026-04-24T10:00:00Z",
          detailRoute: "/v1/runs/run_1/workflows/wf_1",
          relatedResourceRefs: [{ kind: "run", id: "run_1", route: "/v1/runs/run_1" }],
          environmentScope: "test"
        }
      ]
    });
    mockClient.getDiagnostics.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });
    mockClient.resolveApproval.mockResolvedValue({
      approval: {
        approvalId: "approval_1",
        action: "workflow.launch",
        reason: "needs review",
        status: "approved",
        createdAt: "2026-04-24T10:00:00Z",
        updatedAt: "2026-04-24T10:01:00Z"
      },
      decision: {
        decisionId: "decision_1",
        action: "workflow.launch",
        outcome: "approved",
        reason: "needs review",
        approvalId: "approval_1",
        createdAt: "2026-04-24T10:01:00Z"
      }
    });
    mockClient.fetchRoute.mockResolvedValue({ workflowId: "wf_1", status: "failed" });

    render(<App />);
    const user = userEvent.setup();

    await user.type(screen.getByLabelText(/access token/i), "token");
    await user.click(screen.getByRole("button", { name: /load shell/i }));

    await waitFor(() => {
      expect(screen.getByText("Approval Inbox")).not.toBeNull();
      expect(screen.getByText("Workflow wf_1")).not.toBeNull();
    });

    await user.click(screen.getByRole("button", { name: "Approve" }));

    await waitFor(() => {
      expect(mockClient.resolveApproval).toHaveBeenCalledWith("approval_1", {
        resolution: "approved",
        comment: "Resolved in operator shell."
      });
      expect(screen.getByText(/workflow.launch approved/i)).not.toBeNull();
    });

    await user.click(screen.getAllByRole("button", { name: "Inspect" }).find((button) => button.closest(".activity-panel")) ?? screen.getAllByRole("button", { name: "Inspect" })[0]);

    await waitFor(() => {
      expect(mockClient.fetchRoute).toHaveBeenCalledWith("/v1/runs/run_1/workflows/wf_1");
      expect(screen.getByText(/Authoritative Detail/i)).not.toBeNull();
      expect(screen.getByText(/"workflowId": "wf_1"/i)).not.toBeNull();
    });
  });

  it("applies diagnostics filters and runs a bounded test query", async () => {
    mockClient.getOnboarding.mockResolvedValue(onboardingFixture());
    mockClient.listApprovals.mockResolvedValue({ items: [] });
    mockClient.getActivity.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });
    mockClient.getDiagnostics
      .mockResolvedValueOnce({
        environmentScope: "test",
        generatedAt: "2026-04-24T10:00:00Z",
        items: [
          {
            findingId: "approval-1",
            sourceKind: "approval",
            sourceId: "approval_1",
            plane: "approval",
            severity: "warning",
            status: "pending",
            reason: "waiting approval",
            environmentScope: "test",
            capturedAt: "2026-04-24T10:00:00Z"
          }
        ]
      })
      .mockResolvedValueOnce({
        environmentScope: "test",
        generatedAt: "2026-04-24T10:01:00Z",
        items: [
          {
            findingId: "delivery-1",
            sourceKind: "delivery",
            sourceId: "delivery_1",
            plane: "delivery",
            severity: "critical",
            status: "failed",
            reason: "delivery failed",
            detailRoute: "/v1/deliveries/delivery_1",
            environmentScope: "test",
            capturedAt: "2026-04-24T10:01:00Z"
          }
        ]
      })
      .mockResolvedValue({
        environmentScope: "test",
        generatedAt: "2026-04-24T10:01:00Z",
        items: []
      });
    mockClient.queryChat.mockResolvedValue({
      dispatchId: "dispatch_1",
      provider: "echo",
      model: "echo-v1",
      skills: [],
      query: "Return one bounded readiness confirmation.",
      status: "completed",
      partial: false,
      reply: "ready",
      usage: { inputTokens: 1, outputTokens: 1, totalTokens: 2 }
    });

    render(<App />);
    const user = userEvent.setup();

    await user.type(screen.getByLabelText(/access token/i), "token");
    await user.click(screen.getByRole("button", { name: /load shell/i }));

    await waitFor(() => {
      expect(screen.getByText("Diagnostics")).not.toBeNull();
    });

    await user.selectOptions(screen.getByLabelText(/plane filter/i), "delivery");
    await user.selectOptions(screen.getByLabelText(/severity filter/i), "critical");
    await user.click(screen.getByRole("button", { name: /apply filters/i }));

    await waitFor(() => {
      expect(mockClient.getDiagnostics).toHaveBeenLastCalledWith({
        plane: "delivery",
        severity: "critical"
      });
    });

    await user.click(screen.getAllByRole("button", { name: "Run test query" })[0]);

    await waitFor(() => {
      expect(mockClient.queryChat).toHaveBeenCalledWith({
        query: "Return one bounded readiness confirmation."
      });
      expect(screen.getByText(/Test query completed with 2 total tokens/i)).not.toBeNull();
    });
  });
});
