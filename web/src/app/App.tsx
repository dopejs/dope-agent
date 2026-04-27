import { useEffect, useRef, useState } from "react";

import {
  createDopeClient,
  type ApprovalResource,
  type AuthMeResponse,
  type EventStreamSubscription,
  type MembershipResource,
  type OperatorActivityListResponse,
  type OperatorActivityRecord,
  type OperatorDiagnosticFinding,
  type OperatorDiagnosticListResponse,
  type OperatorFirstUsefulAction,
  type OperatorOnboardingResponse,
  type ReplayAttemptResource,
  type ReplayCandidateResource,
  type ReplayComparisonResource,
  type ReplayFixtureResource,
  type TenantPermission,
  type TenantRequestOptions,
  type TenantResource,
  type TenantRole
} from "@dope/client";

const DEFAULT_DAEMON_URL = "http://127.0.0.1:19192";
const DEFAULT_RUN_GOAL = "Run an operator shell smoke check.";
const DEFAULT_TEST_QUERY = "Return one bounded readiness confirmation.";

type ShellStatus = "idle" | "loading" | "ready" | "error";
type EventStatus = "disconnected" | "connected" | "error";
type ActiveTenantStatus = "unresolved" | "resolving" | "active" | "stale" | "denied";
type MembershipStatus = "hidden" | "loading" | "ready" | "empty" | "denied" | "error";

type ShellSnapshot = {
  onboarding: OperatorOnboardingResponse | null;
  approvals: ApprovalResource[];
  activity: OperatorActivityListResponse | null;
  diagnostics: OperatorDiagnosticListResponse | null;
  replayCandidates: ReplayCandidateResource[];
  replayAttempts: ReplayAttemptResource[];
  replayComparisons: ReplayComparisonResource[];
  replayFixtures: ReplayFixtureResource[];
};

type DetailView = {
  title: string;
  route?: string;
  tenantId: string;
  generation: number;
  payload: unknown;
};

type MembershipPanelState = {
  status: MembershipStatus;
  members: MembershipResource[];
  error: string;
  pendingMembershipId: string;
};

const EMPTY_SHELL: ShellSnapshot = {
  onboarding: null,
  approvals: [],
  activity: null,
  diagnostics: null,
  replayCandidates: [],
  replayAttempts: [],
  replayComparisons: [],
  replayFixtures: []
};

const ROLE_OPTIONS: TenantRole[] = ["owner", "admin", "operator", "viewer"];

export function App() {
  const [daemonURL, setDaemonURL] = useState(DEFAULT_DAEMON_URL);
  const [accessToken, setAccessToken] = useState("");
  const [status, setStatus] = useState<ShellStatus>("idle");
  const [eventStatus, setEventStatus] = useState<EventStatus>("disconnected");
  const [error, setError] = useState("");
  const [actionMessage, setActionMessage] = useState("");
  const [lastEvent, setLastEvent] = useState("");
  const [detail, setDetail] = useState<DetailView | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [activeActionId, setActiveActionId] = useState("");
  const [runGoal, setRunGoal] = useState(DEFAULT_RUN_GOAL);
  const [testQuery, setTestQuery] = useState(DEFAULT_TEST_QUERY);
  const [diagnosticPlane, setDiagnosticPlane] = useState("");
  const [diagnosticSeverity, setDiagnosticSeverity] = useState("");
  const [authMe, setAuthMe] = useState<AuthMeResponse | null>(null);
  const [allowedTenants, setAllowedTenants] = useState<TenantResource[]>([]);
  const [activeTenantId, setActiveTenantId] = useState("");
  const [activeTenantStatus, setActiveTenantStatus] = useState<ActiveTenantStatus>("unresolved");
  const [tenantMessage, setTenantMessage] = useState("Tenant context has not been resolved.");
  const [shell, setShell] = useState<ShellSnapshot>(EMPTY_SHELL);
  const [memberships, setMemberships] = useState<MembershipPanelState>({
    status: "hidden",
    members: [],
    error: "",
    pendingMembershipId: ""
  });

  const generationRef = useRef(0);
  const activeTenantRef = useRef("");

  const activeTenant = allowedTenants.find((tenant) => tenant.tenantId === activeTenantId) ?? null;
  const tenantOptions = activeTenantId ? { tenantId: activeTenantId } : undefined;
  const canUseTenantActions = status === "ready" && activeTenantStatus === "active" && Boolean(activeTenantId);
  const canManageMemberships = hasPermission(activeTenant, "tenant.manage");

  function buildClient(defaultTenantId = activeTenantId) {
    return createDopeClient({
      baseURL: daemonURL,
      accessToken: accessToken.trim() || undefined,
      defaultTenantId: defaultTenantId || undefined
    });
  }

  function markActiveTenantDenied(message: string) {
    activeTenantRef.current = "";
    setActiveTenantStatus("denied");
    setTenantMessage(message);
    setShell(EMPTY_SHELL);
    setDetail(null);
    setDetailLoading(false);
    setMemberships({ status: "denied", members: [], error: message, pendingMembershipId: "" });
    setStatus("ready");
  }

  function isCurrentTenantWork(generation: number, tenantId: string): boolean {
    return generation === generationRef.current && activeTenantRef.current === tenantId;
  }

  async function refreshShell(options: { soft?: boolean; tenantId?: string; explicitSelection?: boolean } = {}) {
    if (!accessToken.trim()) {
      setStatus("error");
      setActiveTenantStatus("denied");
      setError("Access token is required to load the operator shell.");
      return;
    }

    const generation = generationRef.current + 1;
    generationRef.current = generation;
    activeTenantRef.current = "";
    setError("");
    if (!options.soft) {
      setActionMessage("");
      setDetail(null);
      setDetailLoading(false);
      setActiveActionId("");
    }
    setMemberships({ status: "loading", members: [], error: "", pendingMembershipId: "" });
    setActiveTenantStatus(options.soft ? "stale" : "resolving");
    setTenantMessage(options.soft ? "Refreshing tenant-scoped views." : "Resolving tenant context.");
    if (!options.soft) {
      setShell(EMPTY_SHELL);
      setStatus("loading");
    }

    try {
      const bootstrapClient = buildClient("");
      const me = await bootstrapClient.getMe();
      const tenantList = await bootstrapClient.listTenants();
      if (generation !== generationRef.current) {
        return;
      }

      const allowed = tenantList.items.length ? tenantList.items : me.allowedTenants;
      setAuthMe(me);
      setAllowedTenants(allowed);

      const selected = resolveActiveTenant({
        allowed,
        me,
        requestedTenantId: options.tenantId,
        explicitSelection: options.explicitSelection,
        savedTenantId: readTenantPreference(daemonURL, me.principal.principalId)
      });

      if (!selected.tenant) {
        setActiveTenantId("");
        markActiveTenantDenied(selected.message);
        return;
      }

      const tenant = selected.tenant;
      activeTenantRef.current = tenant.tenantId;
      setActiveTenantId(tenant.tenantId);
      setActiveTenantStatus("active");
      setTenantMessage(selected.message);
      if (options.explicitSelection) {
        writeTenantPreference(daemonURL, me.principal.principalId, tenant.tenantId);
      }

      const scopedClient = buildClient(tenant.tenantId);
      const scopedOptions: TenantRequestOptions = { tenantId: tenant.tenantId };
      const membershipPromise = hasPermission(tenant, "tenant.manage")
        ? scopedClient.listMemberships(tenant.tenantId, {}, scopedOptions).then((response) => response.items)
        : Promise.resolve<MembershipResource[]>([]);

      const [
        onboarding,
        approvals,
        activity,
        diagnostics,
        replayCandidates,
        replayAttempts,
        replayComparisons,
        replayFixtures,
        membershipItems
      ] = await Promise.all([
        scopedClient.getOnboarding(scopedOptions),
        scopedClient.listApprovals("pending", scopedOptions),
        scopedClient.getActivity({ attentionOnly: true, limit: 20 }, scopedOptions),
        scopedClient.getDiagnostics({
          plane: diagnosticPlane ? (diagnosticPlane as OperatorDiagnosticFinding["plane"]) : undefined,
          severity: diagnosticSeverity ? (diagnosticSeverity as OperatorDiagnosticFinding["severity"]) : undefined
        }, scopedOptions),
        scopedClient.listReplayCandidates({ limit: 20 }, scopedOptions),
        scopedClient.listReplayAttempts({ limit: 20 }, scopedOptions),
        scopedClient.listReplayComparisons({ limit: 20 }, scopedOptions),
        scopedClient.listReplayFixtures({}, scopedOptions),
        membershipPromise
      ]);

      if (generation !== generationRef.current || activeTenantRef.current !== tenant.tenantId) {
        return;
      }

      setShell({
        onboarding,
        approvals: approvals.items,
        activity,
        diagnostics,
        replayCandidates: replayCandidates.items,
        replayAttempts: replayAttempts.items,
        replayComparisons: replayComparisons.items,
        replayFixtures: replayFixtures.items
      });
      setMemberships({
        status: hasPermission(tenant, "tenant.manage") ? membershipStatusFor(membershipItems) : "hidden",
        members: membershipItems,
        error: "",
        pendingMembershipId: ""
      });
      setStatus("ready");
    } catch (caught) {
      const message = errorMessage(caught);
      if (generation !== generationRef.current) {
        return;
      }
      if (isTenantDenied(caught)) {
        markActiveTenantDenied("Tenant access was denied. Choose another allowed tenant before continuing.");
        setMemberships({ status: "denied", members: [], error: message, pendingMembershipId: "" });
        return;
      }
      setStatus("error");
      setActiveTenantStatus("denied");
      setError(message);
      setMemberships({ status: "error", members: [], error: message, pendingMembershipId: "" });
    }
  }

  useEffect(() => {
    if (!canUseTenantActions || !accessToken.trim() || !tenantOptions) {
      setEventStatus("disconnected");
      return;
    }

    const streamTenantId = activeTenantId;
    const client = buildClient(streamTenantId);
    const subscription: EventStreamSubscription = client.streamEvents({}, {
      onEvent(event) {
        if (activeTenantRef.current !== streamTenantId) {
          return;
        }
        setLastEvent(`${event.name} #${event.sequence}`);
        void refreshShell({ soft: true, tenantId: streamTenantId });
      },
      onError(streamError) {
        if (activeTenantRef.current !== streamTenantId) {
          return;
        }
        if (isTenantDenied(streamError)) {
          markActiveTenantDenied("Tenant access was denied. Choose another allowed tenant before continuing.");
          return;
        }
        setEventStatus("error");
        setError(streamError.message);
      }
    }, tenantOptions);
    setEventStatus("connected");

    return () => {
      subscription.close();
      setEventStatus("disconnected");
    };
  }, [canUseTenantActions, activeTenantId, daemonURL, accessToken, diagnosticPlane, diagnosticSeverity]);

  async function inspectRoute(route: string, title: string) {
    const scoped = currentTenantOptions();
    if (!scoped) {
      setError("Resolve an active tenant before inspecting tenant-scoped details.");
      return;
    }
    const generation = generationRef.current;
    const tenantId = scoped.tenantId!;
    setDetailLoading(true);
    setError("");
    try {
      const payload = await buildClient(tenantId).fetchRoute<unknown>(route, scoped);
      if (generation !== generationRef.current || activeTenantRef.current !== tenantId) {
        return;
      }
      setDetail({ title, route, tenantId, generation, payload });
    } catch (caught) {
      if (!isCurrentTenantWork(generation, tenantId)) {
        return;
      }
      const message = errorMessage(caught);
      if (isTenantDenied(caught)) {
        markActiveTenantDenied("Tenant access was denied. Choose another allowed tenant before continuing.");
        return;
      }
      setError(message);
    } finally {
      if (activeTenantRef.current === tenantId) {
        setDetailLoading(false);
      }
    }
  }

  async function handleApprovalDecision(approval: ApprovalResource, resolution: "approved" | "rejected") {
    const scoped = currentTenantOptions();
    if (!scoped) {
      return;
    }
    const tenantId = scoped.tenantId!;
    const generation = generationRef.current;
    setActiveActionId(approval.approvalId);
    setError("");
    try {
      const response = await buildClient(tenantId).resolveApproval(approval.approvalId, {
        resolution,
        comment: "Resolved in operator shell."
      }, scoped);
      if (generation !== generationRef.current || activeTenantRef.current !== tenantId) {
        return;
      }
      setActionMessage(`${approval.action} ${resolution}.`);
      setDetail({
        title: `Approval ${approval.approvalId}`,
        route: `/v1/policy/approvals/${approval.approvalId}`,
        tenantId,
        generation,
        payload: response
      });
      setActiveActionId("");
      await refreshShell({ soft: true, tenantId });
    } catch (caught) {
      if (!isCurrentTenantWork(generation, tenantId)) {
        return;
      }
      if (isTenantDenied(caught)) {
        markActiveTenantDenied("Tenant access was denied. Choose another allowed tenant before continuing.");
        return;
      }
      setError(errorMessage(caught));
    } finally {
      if (activeTenantRef.current === tenantId) {
        setActiveActionId("");
      }
    }
  }

  async function handleFirstUsefulAction(action: OperatorFirstUsefulAction) {
    const scoped = currentTenantOptions();
    if (!scoped) {
      return;
    }
    const tenantId = scoped.tenantId!;
    const generation = generationRef.current;
    setActiveActionId(action.actionId);
    setError("");
    try {
      if (action.actionKind === "test_run") {
        const run = await buildClient(tenantId).createRun({
          entrypoint: "operator.shell.test",
          goal: runGoal.trim() || DEFAULT_RUN_GOAL
        }, scoped);
        if (generation !== generationRef.current || activeTenantRef.current !== tenantId) {
          return;
        }
        setActionMessage(`Created test run ${run.runId}.`);
        setDetail({ title: "Latest Test Run", route: `/v1/runs/${run.runId}`, tenantId, generation, payload: run });
      } else if (action.actionKind === "test_query") {
        const response = await buildClient(tenantId).queryChat({
          query: testQuery.trim() || DEFAULT_TEST_QUERY
        }, scoped);
        if (generation !== generationRef.current || activeTenantRef.current !== tenantId) {
          return;
        }
        setActionMessage(`Test query completed with ${response.usage.totalTokens} total tokens.`);
        setDetail({ title: "Test Query Result", route: action.resultRoute, tenantId, generation, payload: response });
      }
      setActiveActionId("");
      await refreshShell({ soft: true, tenantId });
    } catch (caught) {
      if (!isCurrentTenantWork(generation, tenantId)) {
        return;
      }
      if (isTenantDenied(caught)) {
        markActiveTenantDenied("Tenant access was denied. Choose another allowed tenant before continuing.");
        return;
      }
      setError(errorMessage(caught));
    } finally {
      if (activeTenantRef.current === tenantId) {
        setActiveActionId("");
      }
    }
  }

  async function handleLaunchReplay(candidate: ReplayCandidateResource) {
    const scoped = currentTenantOptions();
    if (!scoped) {
      return;
    }
    const tenantId = scoped.tenantId!;
    const generation = generationRef.current;
    setActiveActionId(candidate.candidateId);
    setError("");
    try {
      const attempt = await buildClient(tenantId).createReplayAttempt(candidate.candidateId, { mode: "non_live" }, scoped);
      if (generation !== generationRef.current || activeTenantRef.current !== tenantId) {
        return;
      }
      setActionMessage(`Replay attempt ${attempt.attemptId} ${attempt.status}.`);
      setDetail({ title: `Replay Attempt ${attempt.attemptId}`, route: `/v1/evaluation/replay-attempts/${attempt.attemptId}`, tenantId, generation, payload: attempt });
      setActiveActionId("");
      await refreshShell({ soft: true, tenantId });
    } catch (caught) {
      if (!isCurrentTenantWork(generation, tenantId)) {
        return;
      }
      if (isTenantDenied(caught)) {
        markActiveTenantDenied("Tenant access was denied. Choose another allowed tenant before continuing.");
        return;
      }
      setError(errorMessage(caught));
    } finally {
      if (activeTenantRef.current === tenantId) {
        setActiveActionId("");
      }
    }
  }

  async function handleCompareLatest(candidate: ReplayCandidateResource) {
    const attemptId = candidate.latestAttemptId || shell.replayAttempts.find((attempt) => attempt.candidateId === candidate.candidateId)?.attemptId;
    if (!attemptId) {
      setError("No replay attempt is available to compare for this candidate.");
      return;
    }
    const scoped = currentTenantOptions();
    if (!scoped) {
      return;
    }
    const tenantId = scoped.tenantId!;
    const generation = generationRef.current;
    setActiveActionId(`compare-${candidate.candidateId}`);
    setError("");
    try {
      const comparison = await buildClient(tenantId).createReplayComparison(attemptId, {}, scoped);
      if (generation !== generationRef.current || activeTenantRef.current !== tenantId) {
        return;
      }
      setActionMessage(`Comparison ${comparison.comparisonId} ${comparison.terminalStatus}.`);
      setDetail({ title: `Comparison ${comparison.comparisonId}`, route: `/v1/evaluation/comparisons/${comparison.comparisonId}`, tenantId, generation, payload: comparison });
      setActiveActionId("");
      await refreshShell({ soft: true, tenantId });
    } catch (caught) {
      if (!isCurrentTenantWork(generation, tenantId)) {
        return;
      }
      if (isTenantDenied(caught)) {
        markActiveTenantDenied("Tenant access was denied. Choose another allowed tenant before continuing.");
        return;
      }
      setError(errorMessage(caught));
    } finally {
      if (activeTenantRef.current === tenantId) {
        setActiveActionId("");
      }
    }
  }

  async function handleMembershipRoleChange(membership: MembershipResource, role: TenantRole) {
    const scoped = currentTenantOptions();
    if (!scoped) {
      return;
    }
    const tenantId = scoped.tenantId!;
    const generation = generationRef.current;
    setMemberships((current) => ({ ...current, pendingMembershipId: membership.membershipId, error: "" }));
    setError("");
    try {
      const response = await buildClient(tenantId).updateMembershipRole(tenantId, membership.membershipId, { role }, scoped);
      if (!isCurrentTenantWork(generation, tenantId)) {
        return;
      }
      setMemberships((current) => {
        const nextMembers = current.members.map((item) => item.membershipId === response.membership.membershipId ? response.membership : item);
        return {
          status: membershipStatusFor(nextMembers),
          members: nextMembers,
          error: "",
          pendingMembershipId: ""
        };
      });
      setActionMessage(`Updated ${membership.principalId} to ${response.membership.role}.`);
    } catch (caught) {
      if (!isCurrentTenantWork(generation, tenantId)) {
        return;
      }
      if (isTenantDenied(caught)) {
        markActiveTenantDenied("Tenant access was denied. Choose another allowed tenant before continuing.");
        return;
      }
      const message = errorMessage(caught);
      setMemberships((current) => ({ ...current, status: "error", error: message, pendingMembershipId: "" }));
      setError(message);
    }
  }

  function currentTenantOptions(): TenantRequestOptions | null {
    if (!canUseTenantActions || !activeTenantId) {
      setError("Resolve an active tenant before performing tenant-scoped work.");
      return null;
    }
    return { tenantId: activeTenantId };
  }

  const onboarding = shell.onboarding;
  const activityItems = shell.activity?.items ?? [];
  const diagnosticItems = shell.diagnostics?.items ?? [];

  return (
    <main className="operator-shell">
      <section className="hero-panel">
        <div>
          <p className="eyebrow">Operator Shell</p>
          <h1>Single control surface for onboarding, approvals, activity, and diagnostics.</h1>
        </div>
        <p className="hero-copy">
          This web shell projects daemon-owned readiness, approvals, workflows, deliveries, and failures without dropping the
          operator into raw route tabs.
        </p>
      </section>

      <section className="config-panel">
        <label>
          <span>Daemon URL</span>
          <input value={daemonURL} onChange={(event) => setDaemonURL(event.target.value)} placeholder={DEFAULT_DAEMON_URL} />
        </label>
        <label>
          <span>Access Token</span>
          <input value={accessToken} onChange={(event) => setAccessToken(event.target.value)} placeholder="Bearer token" type="password" />
        </label>
        <label>
          <span>Active Tenant</span>
          <select
            aria-label="Active Tenant"
            disabled={!allowedTenants.length || activeTenantStatus === "resolving" || activeTenantStatus === "stale"}
            value={activeTenantId}
            onChange={(event) => {
              void refreshShell({ tenantId: event.target.value, explicitSelection: true });
            }}
          >
            <option value="">Resolve tenant</option>
            {allowedTenants.map((tenant) => (
              <option key={tenant.tenantId} value={tenant.tenantId}>{tenant.displayName}</option>
            ))}
          </select>
        </label>
        <div className="config-actions">
          <button className="secondary" disabled={status === "loading"} type="button" onClick={() => {
            setError("");
            setActionMessage("");
            setDetail(null);
            void refreshShell();
          }}>
            {status === "loading" ? "Loading..." : status === "ready" ? "Refresh Shell" : "Load Shell"}
          </button>
        </div>
      </section>

      <section className="tenant-strip" aria-label="tenant status">
        <div>
          <span className="banner-label">Active Tenant</span>
          <strong>{activeTenant ? `${activeTenant.displayName} (${activeTenant.tenantKind})` : "unresolved"}</strong>
        </div>
        <span className={`status-chip status-${activeTenantStatus}`}>{activeTenantStatus}</span>
        <p>{tenantMessage}</p>
      </section>

      <section className="banner-strip" aria-label="shell status">
        <div className="banner-card">
          <span className="banner-label">Environment</span>
          <strong>{onboarding?.environmentScope ?? "not loaded"}</strong>
        </div>
        <div className="banner-card">
          <span className="banner-label">Onboarding</span>
          <strong>{onboarding?.status ?? status}</strong>
        </div>
        <div className="banner-card">
          <span className="banner-label">Event Stream</span>
          <strong>{eventStatus}</strong>
        </div>
        <div className="banner-card">
          <span className="banner-label">Last Event</span>
          <strong>{lastEvent || "waiting"}</strong>
        </div>
      </section>

      {error ? <div className="error-box" role="alert">{error}</div> : null}
      {actionMessage ? <div className="message-box">{actionMessage}</div> : null}

      <section className={`dashboard-grid tenant-${activeTenantStatus}`}>
        <section className="panel onboarding-panel">
          <div className="panel-head">
            <div>
              <p className="section-kicker">First Run</p>
              <h2>Onboarding</h2>
            </div>
            {onboarding ? <span className={`status-chip status-${onboarding.status}`}>{onboarding.status}</span> : null}
          </div>

          {onboarding ? (
            <>
              <div className="action-inputs">
                <label>
                  <span>Test Run Goal</span>
                  <input value={runGoal} onChange={(event) => setRunGoal(event.target.value)} />
                </label>
                <label>
                  <span>Test Query</span>
                  <textarea value={testQuery} onChange={(event) => setTestQuery(event.target.value)} rows={4} />
                </label>
              </div>

              <div className="action-grid">
                {onboarding.firstUsefulActions.map((action) => (
                  <article className="action-card" key={action.actionId}>
                    <div className="action-title-row">
                      <strong>{action.displayName}</strong>
                      {action.recommended ? <span className="tag">Recommended</span> : null}
                    </div>
                    <p>{action.summary || "No action summary."}</p>
                    <button
                      className="primary"
                      disabled={!canUseTenantActions || !action.available || activeActionId === action.actionId}
                      type="button"
                      onClick={() => {
                        void handleFirstUsefulAction(action);
                      }}
                    >
                      {activeActionId === action.actionId ? "Running..." : action.displayName}
                    </button>
                    {action.blockingItemIds?.length ? <small>Blocked by {action.blockingItemIds.join(", ")}</small> : null}
                  </article>
                ))}
              </div>

              <div className="readiness-list">
                {onboarding.readinessItems.map((item) => (
                  <article className="readiness-card" key={item.itemId}>
                    <div className="readiness-head">
                      <strong>{item.displayName}</strong>
                      <span className={`status-chip status-${item.status}`}>{item.status}</span>
                    </div>
                    <p>{item.reason || "No readiness note."}</p>
                    <small>{item.requiredForSelectedAction ? "Required for selected action." : "Optional follow-up."}</small>
                    <div className="inline-actions">
                      {item.detailRoute ? (
                        <button disabled={!canUseTenantActions} type="button" onClick={() => {
                          void inspectRoute(item.detailRoute!, item.displayName);
                        }}>
                          Inspect
                        </button>
                      ) : null}
                    </div>
                  </article>
                ))}
              </div>
            </>
          ) : (
            <div className="empty-state">{activeTenantStatus === "denied" ? "Tenant access is denied. Choose another allowed tenant." : "Load the shell to project readiness and bounded first-use actions."}</div>
          )}
        </section>

        <section className="panel approvals-panel">
          <div className="panel-head">
            <div>
              <p className="section-kicker">Human Gate</p>
              <h2>Approval Inbox</h2>
            </div>
            <span className="count-chip">{shell.approvals.length}</span>
          </div>

          {shell.approvals.length ? (
            <div className="stack-list">
              {shell.approvals.map((approval) => (
                <article className="stack-card" key={approval.approvalId}>
                  <div className="stack-head">
                    <strong>{approval.action}</strong>
                    <span className={`status-chip status-${approval.status}`}>{approval.status}</span>
                  </div>
                  <p>{approval.reason}</p>
                  <div className="inline-actions">
                    <button className="primary" disabled={!canUseTenantActions || activeActionId === approval.approvalId} type="button" onClick={() => {
                      void handleApprovalDecision(approval, "approved");
                    }}>
                      Approve
                    </button>
                    <button disabled={!canUseTenantActions || activeActionId === approval.approvalId} type="button" onClick={() => {
                      void handleApprovalDecision(approval, "rejected");
                    }}>
                      Reject
                    </button>
                    <button disabled={!canUseTenantActions} type="button" onClick={() => {
                      void inspectRoute(`/v1/policy/approvals/${approval.approvalId}`, `Approval ${approval.approvalId}`);
                    }}>
                      Inspect
                    </button>
                  </div>
                </article>
              ))}
            </div>
          ) : (
            <div className="empty-state">No pending approvals.</div>
          )}
        </section>

        <section className="panel activity-panel">
          <div className="panel-head">
            <div>
              <p className="section-kicker">Recent Work</p>
              <h2>Activity</h2>
            </div>
            <span className="count-chip">{activityItems.length}</span>
          </div>

          {activityItems.length ? (
            <div className="stack-list">
              {activityItems.map((item) => <ActivityCard key={item.activityId} disabled={!canUseTenantActions} item={item} onInspect={inspectRoute} />)}
            </div>
          ) : (
            <div className="empty-state">No recent operator activity projected yet.</div>
          )}
        </section>

        <section className="panel diagnostics-panel">
          <div className="panel-head">
            <div>
              <p className="section-kicker">Failure Planes</p>
              <h2>Diagnostics</h2>
            </div>
            <span className="count-chip">{diagnosticItems.length}</span>
          </div>

          <div className="filter-row">
            <label>
              <span>Plane Filter</span>
              <select value={diagnosticPlane} onChange={(event) => setDiagnosticPlane(event.target.value)}>
                <option value="">All planes</option>
                <option value="readiness">Readiness</option>
                <option value="approval">Approval</option>
                <option value="execution">Execution</option>
                <option value="delivery">Delivery</option>
              </select>
            </label>
            <label>
              <span>Severity Filter</span>
              <select value={diagnosticSeverity} onChange={(event) => setDiagnosticSeverity(event.target.value)}>
                <option value="">All severities</option>
                <option value="warning">Warning</option>
                <option value="critical">Critical</option>
              </select>
            </label>
            <button className="secondary" disabled={!canUseTenantActions} type="button" onClick={() => {
              void refreshShell({ tenantId: activeTenantId });
            }}>
              Apply Filters
            </button>
          </div>

          {diagnosticItems.length ? (
            <div className="stack-list">
              {diagnosticItems.map((item) => (
                <article className="stack-card" key={item.findingId}>
                  <div className="stack-head">
                    <strong>{item.sourceKind}</strong>
                    <span className={`status-chip status-${item.severity}`}>{item.severity}</span>
                  </div>
                  <p>{item.reason}</p>
                  <small>{item.plane} plane · {item.status}</small>
                  <div className="inline-actions">
                    {item.detailRoute ? (
                      <button disabled={!canUseTenantActions} type="button" onClick={() => {
                        void inspectRoute(item.detailRoute!, `${item.sourceKind} ${item.sourceId}`);
                      }}>
                        Inspect
                      </button>
                    ) : null}
                  </div>
                </article>
              ))}
            </div>
          ) : (
            <div className="empty-state">No matching diagnostics in the current filter window.</div>
          )}
        </section>

        <section className="panel membership-panel">
          <div className="panel-head">
            <div>
              <p className="section-kicker">Tenant Access</p>
              <h2>Memberships</h2>
            </div>
            <span className={`status-chip status-${memberships.status}`}>{memberships.status}</span>
          </div>
          <MembershipPanel
            canManage={canManageMemberships}
            memberships={memberships}
            onRoleChange={handleMembershipRoleChange}
          />
        </section>

        <section className="panel evaluation-panel">
          <div className="panel-head">
            <div>
              <p className="section-kicker">Deterministic Harness</p>
              <h2>Evaluation Replay</h2>
            </div>
            <span className="count-chip">{shell.replayCandidates.length}</span>
          </div>

          {shell.replayCandidates.length ? (
            <div className="stack-list">
              {shell.replayCandidates.map((candidate) => (
                <article className="stack-card" key={candidate.candidateId}>
                  <div className="stack-head">
                    <strong>{candidate.displayName}</strong>
                    <span className={`status-chip status-${candidate.readinessStatus}`}>{candidate.readinessStatus}</span>
                  </div>
                  <p>{(candidate.readinessReasons ?? []).join(" ") || "Replay readiness is available."}</p>
                  <small>{candidate.candidateKind} · {candidate.sourceKind} · {candidate.defaultReplayMode}</small>
                  {(candidate.limitations ?? []).length ? <p className="muted-line">{(candidate.limitations ?? []).join(" ")}</p> : null}
                  <div className="inline-actions">
                    <button className="primary" disabled={!canUseTenantActions || activeActionId === candidate.candidateId} type="button" onClick={() => {
                      void handleLaunchReplay(candidate);
                    }}>
                      Launch Replay
                    </button>
                    <button disabled={!canUseTenantActions || activeActionId === `compare-${candidate.candidateId}`} type="button" onClick={() => {
                      void handleCompareLatest(candidate);
                    }}>
                      Compare Latest
                    </button>
                    <button disabled={!canUseTenantActions} type="button" onClick={() => {
                      void inspectRoute(`/v1/evaluation/replay-candidates/${candidate.candidateId}`, candidate.displayName);
                    }}>
                      Inspect
                    </button>
                  </div>
                </article>
              ))}
            </div>
          ) : (
            <div className="empty-state">No curated replay candidates or fixtures are available in this environment.</div>
          )}

          <div className="fixture-strip">
            <strong>Fixtures</strong>
            <span>Fixtures are engineer-managed and repo-backed; this shell intentionally does not expose fixture editing controls.</span>
          </div>
          {shell.replayFixtures.length ? (
            <div className="mini-card-grid">
              {shell.replayFixtures.map((fixture) => (
                <article className="mini-card" key={fixture.fixtureId}>
                  <strong>{fixture.displayName}</strong>
                  <small>{fixture.domainClass}</small>
                  <p>{(fixture.assumptions ?? []).join(" ") || "No fixture assumptions recorded."}</p>
                </article>
              ))}
            </div>
          ) : null}

          <div className="fixture-strip">
            <strong>Replay History</strong>
            <span>Attempts and comparisons are daemon-owned records; inspect detail for the authoritative payload.</span>
          </div>
          {shell.replayAttempts.length || shell.replayComparisons.length ? (
            <div className="stack-list">
              {shell.replayAttempts.map((attempt) => (
                <article className="stack-card" key={attempt.attemptId}>
                  <div className="stack-head">
                    <strong>{attempt.attemptId}</strong>
                    <span className={`status-chip status-${attempt.status}`}>{attempt.status}</span>
                  </div>
                  <p>{attempt.runtimeSummary || attempt.evidenceSummary || "Replay attempt has no summary yet."}</p>
                  <small>{attempt.mode} · {attempt.sideEffectHandling} · {attempt.candidateId}</small>
                  <div className="inline-actions">
                    <button disabled={!canUseTenantActions} type="button" onClick={() => {
                      void inspectRoute(`/v1/evaluation/replay-attempts/${attempt.attemptId}`, attempt.attemptId);
                    }}>
                      Inspect Attempt
                    </button>
                  </div>
                </article>
              ))}
              {shell.replayComparisons.map((comparison) => (
                <article className="stack-card" key={comparison.comparisonId}>
                  <div className="stack-head">
                    <strong>{comparison.comparisonId}</strong>
                    <span className={`status-chip status-${comparison.terminalStatus}`}>{comparison.terminalStatus}</span>
                  </div>
                  <p>{comparison.runtimeSummary}</p>
                  <small>policy: {comparison.policySummary || "n/a"} · integration: {comparison.integrationSummary || "n/a"} · delivery: {comparison.deliverySummary || "n/a"}</small>
                  {(comparison.driftFindings ?? []).length ? (
                    <div className="drift-list">
                      {(comparison.driftFindings ?? []).map((finding) => <p key={finding.findingId}><strong>{finding.plane}</strong>: {finding.summary}</p>)}
                    </div>
                  ) : null}
                  <div className="inline-actions">
                    <button disabled={!canUseTenantActions} type="button" onClick={() => {
                      void inspectRoute(`/v1/evaluation/comparisons/${comparison.comparisonId}`, comparison.comparisonId);
                    }}>
                      Inspect Comparison
                    </button>
                  </div>
                </article>
              ))}
            </div>
          ) : (
            <div className="empty-state">No replay attempts or comparisons have been recorded yet.</div>
          )}
        </section>

        <section className="panel detail-panel">
          <div className="panel-head">
            <div>
              <p className="section-kicker">Authoritative Detail</p>
              <h2>Same-Shell Inspection</h2>
            </div>
            {detail?.route ? <span className="route-chip">{detail.route}</span> : null}
          </div>

          {detailLoading ? <div className="empty-state">Loading detail...</div> : null}
          {!detailLoading && detail ? (
            <>
              <h3>{detail.title}</h3>
              <small>Loaded under tenant {detail.tenantId}</small>
              <pre>{JSON.stringify(detail.payload, null, 2)}</pre>
            </>
          ) : null}
          {!detailLoading && !detail ? <div className="empty-state">Inspect an approval, activity item, readiness item, or diagnostic to view daemon detail here.</div> : null}
        </section>
      </section>
    </main>
  );
}

function ActivityCard(props: {
  item: OperatorActivityRecord;
  disabled: boolean;
  onInspect: (route: string, title: string) => Promise<void>;
}) {
  const { item, disabled, onInspect } = props;

  return (
    <article className="stack-card">
      <div className="stack-head">
        <strong>{item.title}</strong>
        <span className={`status-chip status-${item.attentionLevel}`}>{item.attentionLevel}</span>
      </div>
      <p>{item.summary}</p>
      <small>{item.sourceKind} · {item.status}</small>
      <div className="inline-actions">
        {item.detailRoute ? (
          <button disabled={disabled} type="button" onClick={() => {
            void onInspect(item.detailRoute!, item.title);
          }}>
            Inspect
          </button>
        ) : null}
        {item.relatedResourceRefs?.map((ref) => (
          <button disabled={disabled || !ref.route} key={`${item.activityId}-${ref.kind}-${ref.id}`} type="button" onClick={() => {
            if (ref.route) {
              void onInspect(ref.route, `${ref.kind} ${ref.id}`);
            }
          }}>
            {ref.kind}
          </button>
        ))}
      </div>
    </article>
  );
}

function MembershipPanel(props: {
  canManage: boolean;
  memberships: MembershipPanelState;
  onRoleChange: (membership: MembershipResource, role: TenantRole) => Promise<void>;
}) {
  const { canManage, memberships, onRoleChange } = props;

  if (!canManage) {
    return <div className="empty-state">Membership role controls are unavailable for the active tenant.</div>;
  }
  if (memberships.status === "loading") {
    return <div className="empty-state">Loading active tenant memberships.</div>;
  }
  if (memberships.status === "denied") {
    return <div className="empty-state">Membership access was denied for the active tenant.</div>;
  }
  if (memberships.status === "empty") {
    return <div className="empty-state">Only the owner is active for this organization tenant.</div>;
  }

  return (
    <>
      {memberships.error ? <div className="error-box" role="alert">{memberships.error}</div> : null}
      <div className="membership-table">
        {memberships.members.map((membership) => (
          <article className="membership-row" key={membership.membershipId}>
            <div>
              <strong>{membership.principalId}</strong>
              <small>{membership.status}</small>
            </div>
            <select
              aria-label={`Role for ${membership.principalId}`}
              disabled={memberships.pendingMembershipId === membership.membershipId}
              value={membership.role}
              onChange={(event) => {
                void onRoleChange(membership, event.target.value as TenantRole);
              }}
            >
              {ROLE_OPTIONS.map((role) => <option key={role} value={role}>{role}</option>)}
            </select>
          </article>
        ))}
      </div>
    </>
  );
}

function resolveActiveTenant(input: {
  allowed: TenantResource[];
  me: AuthMeResponse;
  requestedTenantId?: string;
  explicitSelection?: boolean;
  savedTenantId?: string;
}): { tenant: TenantResource | null; message: string } {
  const requested = input.requestedTenantId?.trim();
  if (requested) {
    const tenant = input.allowed.find((item) => item.tenantId === requested) ?? null;
    if (tenant) {
      return { tenant, message: `Tenant ${tenant.displayName} is active.` };
    }
    if (input.explicitSelection) {
      return { tenant: null, message: "Selected tenant is no longer allowed for this principal." };
    }
  }

  const saved = input.savedTenantId?.trim();
  if (saved) {
    const tenant = input.allowed.find((item) => item.tenantId === saved);
    if (tenant) {
      return { tenant, message: `Restored tenant ${tenant.displayName} after revalidation.` };
    }
  }

  const preferred =
    input.allowed.find((item) => item.defaultForCurrentToken) ??
    input.allowed.find((item) => item.defaultForCurrentPrincipal) ??
    input.allowed.find((item) => item.tenantId === input.me.currentTenant.tenantId) ??
    input.allowed.find((item) => item.tenantId === input.me.defaultTenant.tenantId) ??
    input.allowed[0] ??
    null;

  if (!preferred) {
    return { tenant: null, message: "No allowed tenant is available for this principal." };
  }
  return { tenant: preferred, message: `Tenant ${preferred.displayName} is active.` };
}

function hasPermission(tenant: TenantResource | null, permission: TenantPermission): boolean {
  return Boolean(tenant?.callerPermissions?.includes(permission));
}

function membershipStatusFor(items: MembershipResource[]): MembershipStatus {
  const activeItems = items.filter((item) => item.status === "active");
  if (activeItems.length <= 1 && activeItems.every((item) => item.role === "owner")) {
    return "empty";
  }
  return "ready";
}

function preferenceKey(daemonURL: string, principalId: string): string {
  return `dope.activeTenant.${daemonURL.trim()}.${principalId}`;
}

function readTenantPreference(daemonURL: string, principalId: string): string | undefined {
  try {
    return window.localStorage.getItem(preferenceKey(daemonURL, principalId)) ?? undefined;
  } catch {
    return undefined;
  }
}

function writeTenantPreference(daemonURL: string, principalId: string, tenantId: string) {
  try {
    window.localStorage.setItem(preferenceKey(daemonURL, principalId), tenantId);
  } catch {
    // Browser storage is continuity only; failing closed to no preference is acceptable.
  }
}

function errorMessage(caught: unknown): string {
  return caught instanceof Error ? caught.message : String(caught);
}

function isTenantDenied(caught: unknown): boolean {
  return typeof caught === "object" && caught !== null && "tenantDenied" in caught && Boolean((caught as { tenantDenied?: boolean }).tenantDenied);
}
