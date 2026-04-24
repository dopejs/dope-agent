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
  listReplayCandidates: vi.fn(),
  getReplayCandidate: vi.fn(),
  createReplayAttempt: vi.fn(),
  listReplayAttempts: vi.fn(),
  getReplayAttempt: vi.fn(),
  createReplayComparison: vi.fn(),
  listReplayComparisons: vi.fn(),
  getReplayComparison: vi.fn(),
  listReplayFixtures: vi.fn(),
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
    mockClient.listReplayCandidates.mockReset().mockResolvedValue({ environmentScope: "test", items: [] });
    mockClient.getReplayCandidate.mockReset();
    mockClient.createReplayAttempt.mockReset();
    mockClient.listReplayAttempts.mockReset().mockResolvedValue({ environmentScope: "test", items: [] });
    mockClient.getReplayAttempt.mockReset();
    mockClient.createReplayComparison.mockReset();
    mockClient.listReplayComparisons.mockReset().mockResolvedValue({ environmentScope: "test", items: [] });
    mockClient.getReplayComparison.mockReset();
    mockClient.listReplayFixtures.mockReset().mockResolvedValue({ environmentScope: "test", items: [] });
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

  it("loads evaluation candidates, launches replay, compares, and exposes fixtures without editing controls", async () => {
    mockClient.getOnboarding.mockResolvedValue(onboardingFixture());
    mockClient.listApprovals.mockResolvedValue({ items: [] });
    mockClient.getActivity.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });
    mockClient.getDiagnostics.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });
    mockClient.listReplayCandidates.mockResolvedValue({
      environmentScope: "test",
      items: [
        {
          candidateId: "candidate_1",
          candidateKind: "fixture",
          displayName: "Schedule Fixture",
          sourceKind: "fixture",
          sourceId: "fixture_schedule",
          sourceRefs: [{ kind: "schedule", id: "sched_1", route: "/v1/schedules/sched_1" }],
          environmentScope: "test",
          readinessStatus: "fully_replayable",
          readinessReasons: ["fixture has evidence"],
          limitations: [],
          defaultReplayMode: "non_live",
          latestAttemptId: "attempt_1",
          createdAt: "2026-04-24T10:00:00Z",
          updatedAt: "2026-04-24T10:00:00Z"
        }
      ]
    });
    mockClient.listReplayAttempts.mockResolvedValue({ environmentScope: "test", items: [] });
    mockClient.listReplayComparisons.mockResolvedValue({
      environmentScope: "test",
      items: [
        {
          comparisonId: "comparison_existing",
          candidateId: "candidate_1",
          baselineRef: "fixture_schedule",
          attemptId: "attempt_existing",
          environmentScope: "test",
          terminalStatus: "drifted",
          runtimeSummary: "runtime changed",
          policySummary: "policy matched",
          integrationSummary: "integration matched",
          deliverySummary: "delivery matched",
          evidenceSummary: "evidence matched",
          confidence: "medium",
          limitations: ["captured evidence only"],
          driftFindings: [
            {
              findingId: "finding_1",
              comparisonId: "comparison_existing",
              plane: "runtime",
              severity: "warning",
              summary: "runtime summary changed",
              baselineValue: "old",
              replayValue: "new",
              createdAt: "2026-04-24T10:00:00Z"
            }
          ],
          generatedAt: "2026-04-24T10:00:00Z"
        }
      ]
    });
    mockClient.listReplayFixtures.mockResolvedValue({
      environmentScope: "test",
      items: [
        {
          fixtureId: "fixture_schedule",
          displayName: "Schedule Fixture",
          domainClass: "schedule",
          manifestPath: "daemon/internal/evaluation/testdata/fixtures/schedule-basic/manifest.json",
          sourceRefs: [],
          capturedEvidenceRefs: [],
          assumptions: ["captured evidence"],
          limitations: [],
          expectedReplayMode: "non_live",
          expectedComparisonSummary: { runtime: "runtime", evidence: "evidence" },
          candidateId: "candidate_1",
          environmentScope: "test",
          createdAt: "2026-04-24T10:00:00Z",
          updatedAt: "2026-04-24T10:00:00Z"
        }
      ]
    });
    mockClient.createReplayAttempt.mockResolvedValue({
      attemptId: "attempt_1",
      candidateId: "candidate_1",
      sourceRefs: [],
      environmentScope: "test",
      mode: "non_live",
      status: "completed",
      safetyScope: { mode: "non_live" },
      approvalHandling: "evidence_only",
      sideEffectHandling: "evidence_only",
      evidenceRefs: [],
      blockedReasons: [],
      createdAt: "2026-04-24T10:00:00Z",
      updatedAt: "2026-04-24T10:00:00Z"
    });
    mockClient.createReplayComparison.mockResolvedValue({
      comparisonId: "comparison_1",
      candidateId: "candidate_1",
      baselineRef: "fixture_schedule",
      attemptId: "attempt_1",
      environmentScope: "test",
      terminalStatus: "matched",
      runtimeSummary: "runtime matched",
      policySummary: "policy matched",
      integrationSummary: "integration matched",
      deliverySummary: "delivery matched",
      evidenceSummary: "evidence matched",
      confidence: "high",
      limitations: [],
      driftFindings: [],
      generatedAt: "2026-04-24T10:00:00Z"
    });

    render(<App />);
    const user = userEvent.setup();

    await user.type(screen.getByLabelText(/access token/i), "token");
    await user.click(screen.getByRole("button", { name: /load shell/i }));

    await waitFor(() => {
      expect(screen.getByText("Evaluation Replay")).not.toBeNull();
      expect(screen.getAllByText("Schedule Fixture").length).toBeGreaterThan(0);
      expect(screen.getByText(/Fixtures are engineer-managed and repo-backed/i)).not.toBeNull();
      expect(screen.getByText("Replay History")).not.toBeNull();
      expect(screen.getByText(/runtime changed/i)).not.toBeNull();
      expect(screen.getByText(/runtime summary changed/i)).not.toBeNull();
    });

    expect(screen.queryByRole("button", { name: /edit fixture/i })).toBeNull();

    await user.click(screen.getByRole("button", { name: "Launch Replay" }));

    await waitFor(() => {
      expect(mockClient.createReplayAttempt).toHaveBeenCalledWith("candidate_1", { mode: "non_live" });
      expect(screen.getByText(/Replay attempt attempt_1 completed/i)).not.toBeNull();
    });

    await user.click(screen.getByRole("button", { name: "Compare Latest" }));

    await waitFor(() => {
      expect(mockClient.createReplayComparison).toHaveBeenCalledWith("attempt_1", {});
      expect(screen.getByText(/Comparison comparison_1 matched/i)).not.toBeNull();
    });
  });
});
