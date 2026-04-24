import { useEffect, useState } from "react";

import {
  createDopeClient,
  type ApprovalDecisionResponse,
  type ApprovalResource,
  type EventStreamSubscription,
  type OperatorActivityListResponse,
  type OperatorActivityRecord,
  type OperatorDiagnosticFinding,
  type OperatorDiagnosticListResponse,
  type OperatorFirstUsefulAction,
  type OperatorOnboardingResponse
} from "@dope/client";

const DEFAULT_DAEMON_URL = "http://127.0.0.1:19192";
const DEFAULT_RUN_GOAL = "Run an operator shell smoke check.";
const DEFAULT_TEST_QUERY = "Return one bounded readiness confirmation.";

type ShellStatus = "idle" | "loading" | "ready" | "error";
type EventStatus = "disconnected" | "connected" | "error";

type ShellSnapshot = {
  onboarding: OperatorOnboardingResponse | null;
  approvals: ApprovalResource[];
  activity: OperatorActivityListResponse | null;
  diagnostics: OperatorDiagnosticListResponse | null;
};

type DetailView = {
  title: string;
  route?: string;
  payload: unknown;
};

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
  const [shell, setShell] = useState<ShellSnapshot>({
    onboarding: null,
    approvals: [],
    activity: null,
    diagnostics: null
  });

  function buildClient() {
    return createDopeClient({
      baseURL: daemonURL,
      accessToken: accessToken.trim() || undefined
    });
  }

  async function refreshShell(options?: { soft?: boolean }) {
    if (!accessToken.trim()) {
      setStatus("error");
      setError("Access token is required to load the operator shell.");
      return;
    }

    if (!options?.soft) {
      setStatus("loading");
    }
    setError("");

    try {
      const client = buildClient();
      const [onboarding, approvals, activity, diagnostics] = await Promise.all([
        client.getOnboarding(),
        client.listApprovals("pending"),
        client.getActivity({ attentionOnly: true, limit: 20 }),
        client.getDiagnostics({
          plane: diagnosticPlane ? (diagnosticPlane as OperatorDiagnosticFinding["plane"]) : undefined,
          severity: diagnosticSeverity ? (diagnosticSeverity as OperatorDiagnosticFinding["severity"]) : undefined
        })
      ]);
      setShell({
        onboarding,
        approvals: approvals.items,
        activity,
        diagnostics
      });
      setStatus("ready");
    } catch (caught) {
      const message = caught instanceof Error ? caught.message : String(caught);
      setStatus("error");
      setError(message);
    }
  }

  useEffect(() => {
    if (status !== "ready" || !accessToken.trim()) {
      setEventStatus("disconnected");
      return;
    }

    const client = buildClient();
    const subscription: EventStreamSubscription = client.streamEvents({}, {
      onEvent(event) {
        setLastEvent(`${event.name} #${event.sequence}`);
        void refreshShell({ soft: true });
      },
      onError(streamError) {
        setEventStatus("error");
        setError(streamError.message);
      }
    });
    setEventStatus("connected");

    return () => {
      subscription.close();
      setEventStatus("disconnected");
    };
  }, [status, daemonURL, accessToken, diagnosticPlane, diagnosticSeverity]);

  async function inspectRoute(route: string, title: string) {
    setDetailLoading(true);
    setError("");
    try {
      const payload = await buildClient().fetchRoute<unknown>(route);
      setDetail({ title, route, payload });
    } catch (caught) {
      const message = caught instanceof Error ? caught.message : String(caught);
      setError(message);
    } finally {
      setDetailLoading(false);
    }
  }

  async function handleApprovalDecision(approval: ApprovalResource, resolution: "approved" | "rejected") {
    setActiveActionId(approval.approvalId);
    setError("");
    try {
      const response = await buildClient().resolveApproval(approval.approvalId, {
        resolution,
        comment: "Resolved in operator shell."
      });
      setActionMessage(`${approval.action} ${resolution}.`);
      setDetail({
        title: `Approval ${approval.approvalId}`,
        route: `/v1/policy/approvals/${approval.approvalId}`,
        payload: response
      });
      await refreshShell({ soft: true });
    } catch (caught) {
      const message = caught instanceof Error ? caught.message : String(caught);
      setError(message);
    } finally {
      setActiveActionId("");
    }
  }

  async function handleFirstUsefulAction(action: OperatorFirstUsefulAction) {
    setActiveActionId(action.actionId);
    setError("");
    try {
      if (action.actionKind === "test_run") {
        const run = await buildClient().createRun({
          entrypoint: "operator.shell.test",
          goal: runGoal.trim() || DEFAULT_RUN_GOAL
        });
        setActionMessage(`Created test run ${run.runId}.`);
        setDetail({
          title: "Latest Test Run",
          route: `/v1/runs/${run.runId}`,
          payload: run
        });
      } else if (action.actionKind === "test_query") {
        const response = await buildClient().queryChat({
          query: testQuery.trim() || DEFAULT_TEST_QUERY
        });
        setActionMessage(`Test query completed with ${response.usage.totalTokens} total tokens.`);
        setDetail({
          title: "Test Query Result",
          route: action.resultRoute,
          payload: response
        });
      }
      await refreshShell({ soft: true });
    } catch (caught) {
      const message = caught instanceof Error ? caught.message : String(caught);
      setError(message);
    } finally {
      setActiveActionId("");
    }
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

      {error ? (
        <div className="error-box" role="alert">
          {error}
        </div>
      ) : null}

      {actionMessage ? <div className="message-box">{actionMessage}</div> : null}

      <section className="dashboard-grid">
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
                      disabled={!action.available || activeActionId === action.actionId}
                      type="button"
                      onClick={() => {
                        void handleFirstUsefulAction(action);
                      }}
                    >
                      {activeActionId === action.actionId ? "Running..." : action.displayName}
                    </button>
                    {action.blockingItemIds?.length ? (
                      <small>Blocked by {action.blockingItemIds.join(", ")}</small>
                    ) : null}
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
                        <button
                          type="button"
                          onClick={() => {
                            void inspectRoute(item.detailRoute!, item.displayName);
                          }}
                        >
                          Inspect
                        </button>
                      ) : null}
                    </div>
                  </article>
                ))}
              </div>
            </>
          ) : (
            <div className="empty-state">Load the shell to project readiness and bounded first-use actions.</div>
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
                    <button
                      className="primary"
                      disabled={activeActionId === approval.approvalId}
                      type="button"
                      onClick={() => {
                        void handleApprovalDecision(approval, "approved");
                      }}
                    >
                      Approve
                    </button>
                    <button
                      disabled={activeActionId === approval.approvalId}
                      type="button"
                      onClick={() => {
                        void handleApprovalDecision(approval, "rejected");
                      }}
                    >
                      Reject
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        void inspectRoute(`/v1/policy/approvals/${approval.approvalId}`, `Approval ${approval.approvalId}`);
                      }}
                    >
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
              {activityItems.map((item) => (
                <ActivityCard key={item.activityId} item={item} onInspect={inspectRoute} />
              ))}
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
            <button className="secondary" type="button" onClick={() => {
              void refreshShell();
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
                      <button
                        type="button"
                        onClick={() => {
                          void inspectRoute(item.detailRoute!, `${item.sourceKind} ${item.sourceId}`);
                        }}
                      >
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

        <section className="panel detail-panel">
          <div className="panel-head">
            <div>
              <p className="section-kicker">Authoritative Detail</p>
              <h2>Same-Shell Inspection</h2>
            </div>
            {detail?.route ? <span className="route-chip">{detail.route}</span> : null}
          </div>

          {detailLoading ? <div className="empty-state">Loading detail…</div> : null}
          {!detailLoading && detail ? (
            <>
              <h3>{detail.title}</h3>
              <pre>{JSON.stringify(detail.payload, null, 2)}</pre>
            </>
          ) : null}
          {!detailLoading && !detail ? (
            <div className="empty-state">Inspect an approval, activity item, readiness item, or diagnostic to view daemon detail here.</div>
          ) : null}
        </section>
      </section>
    </main>
  );
}

function ActivityCard(props: {
  item: OperatorActivityRecord;
  onInspect: (route: string, title: string) => Promise<void>;
}) {
  const { item, onInspect } = props;

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
          <button
            type="button"
            onClick={() => {
              void onInspect(item.detailRoute!, item.title);
            }}
          >
            Inspect
          </button>
        ) : null}
        {item.relatedResourceRefs?.map((ref) => (
          <button
            key={`${item.activityId}-${ref.kind}-${ref.id}`}
            type="button"
            onClick={() => {
              if (ref.route) {
                void onInspect(ref.route, `${ref.kind} ${ref.id}`);
              }
            }}
          >
            {ref.kind}
          </button>
        ))}
      </div>
    </article>
  );
}
