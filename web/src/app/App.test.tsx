import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { App } from "./App";

const mockClient = {
  getMe: vi.fn(),
  listTenants: vi.fn(),
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
  listMemberships: vi.fn(),
  updateMembershipRole: vi.fn(),
  streamEvents: vi.fn()
};

const createdClientOptions: unknown[] = [];

vi.mock("@dope/client", () => ({
  createDopeClient: (options: unknown) => {
    createdClientOptions.push(options);
    return mockClient;
  }
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

function tenantFixture(overrides: Partial<ReturnType<typeof tenantFixtureBase>> = {}) {
  return {
    ...tenantFixtureBase(),
    ...overrides
  };
}

function tenantFixtureBase() {
  return {
    tenantId: "ten_personal",
    tenantKind: "personal",
    displayName: "Personal Tenant",
    status: "active",
    createdAt: "2026-04-24T10:00:00Z",
    updatedAt: "2026-04-24T10:00:00Z",
    callerMembershipRole: "owner",
    callerMembershipStatus: "active",
    callerPermissions: ["tenant.manage", "runs.execute", "approvals.resolve", "evaluation.manage"],
    defaultForCurrentToken: true,
    defaultForCurrentPrincipal: true
  };
}

function authMeFixture(tenants = [tenantFixture()]) {
  return {
    token: { tokenId: "tok_1" },
    principal: {
      principalId: "prn_1",
      principalKind: "local_operator",
      displayName: "Local Operator",
      status: "active",
      defaultTenantId: tenants[0].tenantId,
      createdAt: "2026-04-24T10:00:00Z",
      updatedAt: "2026-04-24T10:00:00Z"
    },
    defaultTenant: tenants[0],
    currentTenant: tenants[0],
    allowedTenants: tenants,
    tokenGrants: [],
    permissions: tenants[0].callerPermissions,
    tenantContext: {
      principalId: "prn_1",
      tokenId: "tok_1",
      tenantId: tenants[0].tenantId,
      tenantSource: "default",
      membershipId: "mem_1",
      role: tenants[0].callerMembershipRole,
      permissions: tenants[0].callerPermissions,
      resolvedAt: "2026-04-24T10:00:00Z"
    }
  };
}

function membershipFixture(overrides: Partial<ReturnType<typeof membershipFixtureBase>> = {}) {
  return {
    ...membershipFixtureBase(),
    ...overrides
  };
}

function membershipFixtureBase() {
  return {
    membershipId: "mem_1",
    tenantId: "ten_personal",
    principalId: "prn_1",
    role: "owner",
    status: "active",
    createdAt: "2026-04-24T10:00:00Z",
    updatedAt: "2026-04-24T10:00:00Z"
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

async function loadShell(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText(/access token/i), "token");
  await user.click(screen.getByRole("button", { name: /load shell/i }));
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
    createdClientOptions.length = 0;
    window.localStorage.clear();
    const tenants = [tenantFixture()];
    mockClient.getMe.mockReset().mockResolvedValue(authMeFixture(tenants));
    mockClient.listTenants.mockReset().mockResolvedValue({ items: tenants });
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
    mockClient.listMemberships.mockReset().mockResolvedValue({ items: [membershipFixture()] });
    mockClient.updateMembershipRole.mockReset();
    emptySubscription.close.mockClear();
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
      }, { tenantId: "ten_personal" });
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
      }, { tenantId: "ten_personal" });
      expect(screen.getByText(/workflow.launch approved/i)).not.toBeNull();
    });

    await user.click(screen.getAllByRole("button", { name: "Inspect" }).find((button) => button.closest(".activity-panel")) ?? screen.getAllByRole("button", { name: "Inspect" })[0]);

    await waitFor(() => {
      expect(mockClient.fetchRoute).toHaveBeenCalledWith("/v1/runs/run_1/workflows/wf_1", { tenantId: "ten_personal" });
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
      }, { tenantId: "ten_personal" });
    });

    await user.click(screen.getAllByRole("button", { name: "Run test query" })[0]);

    await waitFor(() => {
      expect(mockClient.queryChat).toHaveBeenCalledWith({
        query: "Return one bounded readiness confirmation."
      }, { tenantId: "ten_personal" });
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
      expect(mockClient.createReplayAttempt).toHaveBeenCalledWith("candidate_1", { mode: "non_live" }, { tenantId: "ten_personal" });
      expect(screen.getByText(/Replay attempt attempt_1 completed/i)).not.toBeNull();
    });

    await user.click(screen.getByRole("button", { name: "Compare Latest" }));

    await waitFor(() => {
      expect(mockClient.createReplayComparison).toHaveBeenCalledWith("attempt_1", {}, { tenantId: "ten_personal" });
      expect(screen.getByText(/Comparison comparison_1 matched/i)).not.toBeNull();
    });
  });

  it("keeps the shell rendered when evaluation collection fields arrive as null", async () => {
    mockClient.getOnboarding.mockResolvedValue(onboardingFixture());
    mockClient.listApprovals.mockResolvedValue({ items: [] });
    mockClient.getActivity.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });
    mockClient.getDiagnostics.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });
    mockClient.listReplayCandidates.mockResolvedValue({
      environmentScope: "test",
      items: [
        {
          candidateId: "candidate_legacy",
          candidateKind: "fixture",
          displayName: "Legacy Fixture",
          sourceKind: "fixture",
          sourceId: "fixture_legacy",
          sourceRefs: [],
          environmentScope: "test",
          readinessStatus: "fully_replayable",
          readinessReasons: null,
          limitations: null,
          defaultReplayMode: "non_live",
          createdAt: "2026-04-24T10:00:00Z",
          updatedAt: "2026-04-24T10:00:00Z"
        }
      ]
    });
    mockClient.listReplayComparisons.mockResolvedValue({
      environmentScope: "test",
      items: [
        {
          comparisonId: "comparison_legacy",
          candidateId: "candidate_legacy",
          baselineRef: "attempt_base",
          attemptId: "attempt_legacy",
          environmentScope: "test",
          terminalStatus: "matched",
          runtimeSummary: "runtime matched",
          policySummary: "policy matched",
          integrationSummary: "integration matched",
          deliverySummary: "delivery matched",
          evidenceSummary: "evidence matched",
          confidence: "high",
          limitations: null,
          driftFindings: null,
          generatedAt: "2026-04-24T10:00:00Z"
        }
      ]
    });
    mockClient.listReplayFixtures.mockResolvedValue({
      environmentScope: "test",
      items: [
        {
          fixtureId: "fixture_legacy",
          displayName: "Legacy Fixture",
          domainClass: "schedule",
          manifestPath: "manifest.json",
          sourceRefs: [],
          capturedEvidenceRefs: [],
          assumptions: null,
          limitations: null,
          expectedReplayMode: "non_live",
          expectedComparisonSummary: {},
          environmentScope: "test",
          createdAt: "2026-04-24T10:00:00Z",
          updatedAt: "2026-04-24T10:00:00Z"
        }
      ]
    });

    render(<App />);
    const user = userEvent.setup();
    await loadShell(user);

    await waitFor(() => {
      expect(screen.getAllByText("Legacy Fixture").length).toBeGreaterThan(0);
    });
    expect(screen.getByText(/Replay readiness is available/i)).not.toBeNull();
    expect(screen.getByText(/No fixture assumptions recorded/i)).not.toBeNull();
    expect(screen.getByText("comparison_legacy")).not.toBeNull();
  });

  it("loads identity and allowed tenants before tenant-scoped projections", async () => {
    const me = deferred<ReturnType<typeof authMeFixture>>();
    mockClient.getMe.mockReturnValueOnce(me.promise);
    mockClient.listTenants.mockResolvedValue({ items: [tenantFixture()] });
    mockClient.getOnboarding.mockResolvedValue(onboardingFixture());
    mockClient.listApprovals.mockResolvedValue({ items: [] });
    mockClient.getActivity.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });
    mockClient.getDiagnostics.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });

    render(<App />);
    const user = userEvent.setup();
    await loadShell(user);

    expect(screen.queryByRole("button", { name: "Launch test run" })).toBeNull();
    expect(mockClient.getOnboarding).not.toHaveBeenCalled();

    me.resolve(authMeFixture());

    await waitFor(() => {
      expect(mockClient.getOnboarding).toHaveBeenCalledWith({ tenantId: "ten_personal" });
      expect((screen.getByRole("button", { name: "Launch test run" }) as HTMLButtonElement).disabled).toBe(false);
    });
  });

  it("switches among allowed tenants, clears old rows, persists selection, and refreshes projections as one tenant batch", async () => {
    const personal = tenantFixture();
    const org = tenantFixture({
      tenantId: "ten_org",
      tenantKind: "organization",
      displayName: "Org Tenant",
      defaultForCurrentToken: false,
      defaultForCurrentPrincipal: false
    });
    mockClient.getMe.mockResolvedValue(authMeFixture([personal, org]));
    mockClient.listTenants.mockResolvedValue({ items: [personal, org] });
    mockClient.getOnboarding.mockResolvedValue(onboardingFixture());
    mockClient.listApprovals.mockResolvedValue({ items: [] });
    mockClient.getActivity
      .mockResolvedValueOnce({
        environmentScope: "test",
        generatedAt: "2026-04-24T10:00:00Z",
        items: [{
          activityId: "activity_a",
          sourceKind: "run",
          sourceId: "run_a",
          title: "Tenant A row",
          status: "completed",
          summary: "personal row",
          attentionLevel: "info",
          occurredAt: "2026-04-24T10:00:00Z",
          environmentScope: "test"
        }]
      })
      .mockResolvedValueOnce({
        environmentScope: "test",
        generatedAt: "2026-04-24T10:01:00Z",
        items: [{
          activityId: "activity_b",
          sourceKind: "run",
          sourceId: "run_b",
          title: "Tenant B row",
          status: "completed",
          summary: "org row",
          attentionLevel: "info",
          occurredAt: "2026-04-24T10:01:00Z",
          environmentScope: "test"
        }]
      });
    mockClient.getDiagnostics.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });

    render(<App />);
    const user = userEvent.setup();
    await loadShell(user);

    await screen.findByText("Tenant A row");
    await user.selectOptions(screen.getByLabelText("Active Tenant"), "ten_org");

    expect(screen.queryByText("Tenant A row")).toBeNull();
    await screen.findByText("Tenant B row");
    expect(mockClient.getOnboarding).toHaveBeenLastCalledWith({ tenantId: "ten_org" });
    expect(mockClient.listApprovals).toHaveBeenLastCalledWith("pending", { tenantId: "ten_org" });
    expect(mockClient.getActivity).toHaveBeenLastCalledWith({ attentionOnly: true, limit: 20 }, { tenantId: "ten_org" });
    expect(mockClient.getDiagnostics).toHaveBeenLastCalledWith({ plane: undefined, severity: undefined }, { tenantId: "ten_org" });
    expect(mockClient.streamEvents).toHaveBeenLastCalledWith({}, expect.anything(), { tenantId: "ten_org" });
    expect(emptySubscription.close).toHaveBeenCalled();
    expect(window.localStorage.getItem("dope.activeTenant.http://127.0.0.1:19192.prn_1")).toBe("ten_org");

    cleanup();
    mockClient.getActivity.mockResolvedValue({
      environmentScope: "test",
      generatedAt: "2026-04-24T10:02:00Z",
      items: []
    });
    render(<App />);
    const restoreUser = userEvent.setup();
    await loadShell(restoreUser);
    await screen.findByText("Org Tenant (organization)");
  });

  it("shows stable denied state and does not retain previous tenant rows after active tenant revocation", async () => {
    const personal = tenantFixture();
    const org = tenantFixture({ tenantId: "ten_org", tenantKind: "organization", displayName: "Org Tenant" });
    const denial = Object.assign(new Error("tenant access denied"), {
      tenantDenied: true,
      status: 403,
      code: "tenant_access_denied"
    });
    mockClient.getMe.mockResolvedValue(authMeFixture([personal, org]));
    mockClient.listTenants.mockResolvedValue({ items: [personal, org] });
    mockClient.getOnboarding
      .mockResolvedValueOnce(onboardingFixture())
      .mockRejectedValueOnce(denial);
    mockClient.listApprovals.mockResolvedValue({ items: [] });
    mockClient.getActivity.mockResolvedValue({
      environmentScope: "test",
      generatedAt: "2026-04-24T10:00:00Z",
      items: [{
        activityId: "activity_a",
        sourceKind: "run",
        sourceId: "run_a",
        title: "Tenant A row",
        status: "completed",
        summary: "personal row",
        attentionLevel: "info",
        occurredAt: "2026-04-24T10:00:00Z",
        environmentScope: "test"
      }]
    });
    mockClient.getDiagnostics.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });

    render(<App />);
    const user = userEvent.setup();
    await loadShell(user);
    await screen.findByText("Tenant A row");

    await user.selectOptions(screen.getByLabelText("Active Tenant"), "ten_org");

    await screen.findByText(/Tenant access was denied/i);
    expect(screen.queryByText("Tenant A row")).toBeNull();
    expect(screen.getAllByText("denied").length).toBeGreaterThan(0);
  });

  it("clears tenant projections when the active event stream reports tenant denial", async () => {
    const denial = Object.assign(new Error("tenant access denied"), {
      tenantDenied: true,
      status: 403,
      code: "tenant_access_denied"
    });
    let streamHandlers: { onError?: (error: Error) => void } | undefined;
    mockClient.streamEvents.mockImplementation((_query, handlers) => {
      streamHandlers = handlers as { onError?: (error: Error) => void };
      return emptySubscription;
    });
    mockClient.getOnboarding.mockResolvedValue(onboardingFixture());
    mockClient.listApprovals.mockResolvedValue({ items: [] });
    mockClient.getActivity.mockResolvedValue({
      environmentScope: "test",
      generatedAt: "2026-04-24T10:00:00Z",
      items: [{
        activityId: "activity_a",
        sourceKind: "run",
        sourceId: "run_a",
        title: "Tenant A row",
        status: "completed",
        summary: "personal row",
        attentionLevel: "info",
        occurredAt: "2026-04-24T10:00:00Z",
        environmentScope: "test"
      }]
    });
    mockClient.getDiagnostics.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });

    render(<App />);
    const user = userEvent.setup();
    await loadShell(user);
    await screen.findByText("Tenant A row");

    await act(async () => {
      streamHandlers?.onError?.(denial);
    });

    await screen.findByText(/Tenant access was denied/i);
    expect(screen.queryByText("Tenant A row")).toBeNull();
    expect(screen.getAllByText("denied").length).toBeGreaterThan(0);
  });

  it("ignores stale in-flight action responses after switching tenants", async () => {
    const personal = tenantFixture();
    const org = tenantFixture({
      tenantId: "ten_org",
      tenantKind: "organization",
      displayName: "Org Tenant",
      defaultForCurrentToken: false,
      defaultForCurrentPrincipal: false
    });
    const run = deferred<{
      runId: string;
      entrypoint: string;
      status: string;
      goal: string;
      createdAt: string;
      updatedAt: string;
    }>();
    mockClient.getMe.mockResolvedValue(authMeFixture([personal, org]));
    mockClient.listTenants.mockResolvedValue({ items: [personal, org] });
    mockClient.getOnboarding.mockResolvedValue(onboardingFixture());
    mockClient.listApprovals.mockResolvedValue({ items: [] });
    mockClient.getActivity.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });
    mockClient.getDiagnostics.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });
    mockClient.createRun.mockReturnValueOnce(run.promise);

    render(<App />);
    const user = userEvent.setup();
    await loadShell(user);
    await screen.findByText("Personal Tenant (personal)");

    await user.click(screen.getByRole("button", { name: "Launch test run" }));
    await user.selectOptions(screen.getByLabelText("Active Tenant"), "ten_org");
    await screen.findByText("Org Tenant (organization)");

    run.resolve({
      runId: "run_old",
      entrypoint: "operator.shell.test",
      status: "queued",
      goal: "Run an operator shell smoke check.",
      createdAt: "2026-04-24T10:00:00Z",
      updatedAt: "2026-04-24T10:00:00Z"
    });

    await waitFor(() => {
      expect(mockClient.createRun).toHaveBeenCalledWith(expect.anything(), { tenantId: "ten_personal" });
      expect(screen.queryByText(/Created test run run_old/i)).toBeNull();
    });
  });

  it("ignores stale in-flight action and membership failures after switching tenants", async () => {
    const personal = tenantFixture();
    const org = tenantFixture({
      tenantId: "ten_org",
      tenantKind: "organization",
      displayName: "Org Tenant",
      defaultForCurrentToken: false,
      defaultForCurrentPrincipal: false
    });
    const run = deferred<{
      runId: string;
      entrypoint: string;
      status: string;
      goal: string;
      createdAt: string;
      updatedAt: string;
    }>();
    const member = membershipFixture({ membershipId: "mem_member", principalId: "prn_member", role: "viewer" });
    const membershipUpdate = deferred<{ membership: ReturnType<typeof membershipFixture> }>();
    mockClient.getMe.mockResolvedValue(authMeFixture([personal, org]));
    mockClient.listTenants.mockResolvedValue({ items: [personal, org] });
    mockClient.getOnboarding.mockResolvedValue(onboardingFixture());
    mockClient.listApprovals.mockResolvedValue({ items: [] });
    mockClient.getActivity.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });
    mockClient.getDiagnostics.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });
    mockClient.listMemberships.mockResolvedValue({ items: [membershipFixture(), member] });
    mockClient.createRun.mockReturnValueOnce(run.promise);
    mockClient.updateMembershipRole.mockReturnValueOnce(membershipUpdate.promise);

    render(<App />);
    const user = userEvent.setup();
    await loadShell(user);
    await screen.findByText("Personal Tenant (personal)");

    await user.click(screen.getByRole("button", { name: "Launch test run" }));
    await user.selectOptions(await screen.findByLabelText("Role for prn_member"), "admin");
    await user.selectOptions(screen.getByLabelText("Active Tenant"), "ten_org");
    await screen.findByText("Org Tenant (organization)");

    await act(async () => {
      run.reject(new Error("old tenant action failed"));
      membershipUpdate.reject(new Error("old tenant membership failed"));
      await Promise.resolve();
    });

    expect(screen.queryByText(/old tenant action failed/i)).toBeNull();
    expect(screen.queryByText(/old tenant membership failed/i)).toBeNull();
  });

  it("hides membership role controls without tenant.manage and updates authorized roles from daemon-confirmed state", async () => {
    const viewerTenant = tenantFixture({
      callerMembershipRole: "viewer",
      callerPermissions: ["read_only.inspect"]
    });
    mockClient.getMe.mockResolvedValue(authMeFixture([viewerTenant]));
    mockClient.listTenants.mockResolvedValue({ items: [viewerTenant] });
    mockClient.getOnboarding.mockResolvedValue(onboardingFixture());
    mockClient.listApprovals.mockResolvedValue({ items: [] });
    mockClient.getActivity.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });
    mockClient.getDiagnostics.mockResolvedValue({ environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });

    render(<App />);
    const user = userEvent.setup();
    await loadShell(user);

    await screen.findByText(/Membership role controls are unavailable/i);
    expect(screen.queryByLabelText(/Role for/i)).toBeNull();

    cleanup();

    const adminTenant = tenantFixture();
    const member = membershipFixture({ membershipId: "mem_member", principalId: "prn_member", role: "viewer" });
    mockClient.getMe.mockResolvedValue(authMeFixture([adminTenant]));
    mockClient.listTenants.mockResolvedValue({ items: [adminTenant] });
    mockClient.listMemberships.mockResolvedValue({ items: [membershipFixture(), member] });
    mockClient.updateMembershipRole.mockResolvedValue({ membership: { ...member, role: "admin" } });

    render(<App />);
    const adminUser = userEvent.setup();
    await loadShell(adminUser);

    const roleSelect = await screen.findByLabelText("Role for prn_member");
    await adminUser.selectOptions(roleSelect, "admin");

    await waitFor(() => {
      expect(mockClient.updateMembershipRole).toHaveBeenCalledWith("ten_personal", "mem_member", { role: "admin" }, { tenantId: "ten_personal" });
      expect(screen.getByText(/Updated prn_member to admin/i)).not.toBeNull();
    });

    mockClient.updateMembershipRole.mockRejectedValueOnce(new Error("last owner would be removed"));
    await adminUser.selectOptions(screen.getByLabelText("Role for prn_member"), "viewer");

    await waitFor(() => {
      expect(screen.getAllByText("last owner would be removed").length).toBeGreaterThan(0);
      expect((screen.getByLabelText("Role for prn_member") as HTMLSelectElement).value).toBe("admin");
    });
  });
});
