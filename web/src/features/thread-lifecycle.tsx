import type { ThreadDetailResponse, ThreadListResponse } from "@dope/client";

type ThreadLifecycleViewProps = {
  threads?: ThreadListResponse | null;
  detail?: ThreadDetailResponse | null;
  loading?: boolean;
  error?: string;
  denied?: boolean;
  stalePermission?: boolean;
  onRefresh?: () => void;
  onNextPage?: () => void;
  onSelectThread?: (threadId: string) => void;
  onResetThread?: (threadId: string) => void;
  onArchiveThread?: (threadId: string) => void;
  onReopenThread?: (threadId: string) => void;
};

export function ThreadLifecycleView({
  threads,
  detail = null,
  loading = false,
  error = "",
  denied = false,
  stalePermission = false,
  onRefresh,
  onNextPage,
  onSelectThread,
  onResetThread,
  onArchiveThread,
  onReopenThread
}: ThreadLifecycleViewProps) {
  const items = threads?.items ?? [];
  const detailThread = detail?.thread;
  return (
    <section aria-label="Thread lifecycle">
      <header className="panel-header">
        <div>
          <h2>Threads</h2>
          <p>{threads?.tenantId ? `Tenant ${threads.tenantId}` : "Tenant-scoped conversation lifecycle"}</p>
        </div>
        <span className="status-pill">{threads?.page.order ?? "active_recent_archived_id"}</span>
      </header>
      {denied ? <p className="error-text">credentials.inspect is required to inspect tenant threads.</p> : null}
      {stalePermission ? (
        <div className="message-box">
          <span>Thread permissions changed.</span>
          {onRefresh ? <button className="secondary" type="button" onClick={onRefresh}>Refresh</button> : null}
        </div>
      ) : null}
      {loading ? <p className="muted">Loading threads.</p> : null}
      {error ? <p className="error-text">{error}</p> : null}
      {!loading && !error && !denied && items.length === 0 ? <p className="muted">No tenant threads are available.</p> : null}
      <div className="channel-list">
        {items.map((thread) => (
          <article className="channel-row" key={thread.threadId}>
            <div>
              <h3>{thread.threadId}</h3>
              <p>{thread.sourceSummary || thread.sourceKind}</p>
            </div>
            <dl>
              <dt>State</dt>
              <dd>{thread.lifecycleState}</dd>
              <dt>Last activity</dt>
              <dd>{thread.lastActivityAt}</dd>
            </dl>
            {onSelectThread ? <button className="secondary" type="button" onClick={() => onSelectThread(thread.threadId)}>Inspect</button> : null}
            <div className="row-actions">
              {thread.availableActions.includes("reset") && onResetThread ? <button className="secondary" type="button" onClick={() => onResetThread(thread.threadId)}>Reset</button> : null}
              {thread.availableActions.includes("archive") && onArchiveThread ? <button className="secondary" type="button" onClick={() => onArchiveThread(thread.threadId)}>Archive</button> : null}
              {thread.availableActions.includes("reopen") && onReopenThread ? <button className="secondary" type="button" onClick={() => onReopenThread(thread.threadId)}>Reopen</button> : null}
            </div>
          </article>
        ))}
      </div>
      {threads?.page.nextCursor && onNextPage ? <button className="secondary" type="button" onClick={onNextPage}>Next page</button> : null}
      {detailThread ? (
        <section aria-label="Thread detail" className="detail-panel">
          <h3>{detailThread.threadId}</h3>
          <p className="muted">Lifecycle metadata, not assistant memory.</p>
          <dl>
            <dt>Current session</dt>
            <dd>{detailThread.currentSessionId || detailThread.currentSessionSegmentId || "unavailable"}</dd>
            <dt>Redaction</dt>
            <dd>{detailThread.redactionStatus}</dd>
            <dt>Retention</dt>
            <dd>{detailThread.retentionExpiresAt || "policy default"}</dd>
            <dt>Source linkages</dt>
            <dd>{detail.sourceLinkages.length}</dd>
            <dt>Runtime projections</dt>
            <dd>{detail.runtimeProjections.length}</dd>
            <dt>Lifecycle actions</dt>
            <dd>{detail.lifecycleActions.length}</dd>
          </dl>
          <section aria-label="Source trace">
            <h4>Source Trace</h4>
            {detail.sourceLinkages.length === 0 ? <p className="muted">No source evidence.</p> : null}
            <ul>
              {detail.sourceLinkages.map((linkage) => (
                <li key={linkage.sourceLinkageId}>
                  <span>{linkage.routingOutcome}</span>
                  <span>{linkage.connectorKind || linkage.sourceKind}</span>
                  <span>{linkage.sourceConversationId || "conversation redacted"}</span>
                  <span>{linkage.redactionStatus}</span>
                  <span>{linkage.retentionExpiresAt || "policy default"}</span>
                </li>
              ))}
            </ul>
          </section>
          <section aria-label="Runtime trace">
            <h4>Runtime Trace</h4>
            {detail.runtimeProjections.length === 0 ? <p className="muted">No runtime evidence.</p> : null}
            <ul>
              {detail.runtimeProjections.map((projection) => (
                <li key={projection.runtimeProjectionId}>
                  <span>{projection.resourceKind}</span>
                  <span>{projection.status}</span>
                  <span>{projection.safeSummary || projection.reasonCode || "metadata only"}</span>
                  <span>{projection.redactionStatus}</span>
                  <span>{projection.retentionExpiresAt || "policy default"}</span>
                </li>
              ))}
            </ul>
          </section>
        </section>
      ) : null}
    </section>
  );
}
