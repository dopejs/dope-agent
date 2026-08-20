import type { ThreadContinuityPreviewDetail, ThreadDetailResponse, ThreadListResponse } from "@kura/client";

type ThreadLifecycleViewProps = {
  threads?: ThreadListResponse | null;
  detail?: ThreadDetailResponse | null;
  continuityPreviewDetail?: ThreadContinuityPreviewDetail | null;
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
  onHandoffToWeb?: (threadId: string) => void;
};

export function ThreadLifecycleView({
  threads,
  detail = null,
  continuityPreviewDetail = null,
  loading = false,
  error = "",
  denied = false,
  stalePermission = false,
  onRefresh,
  onNextPage,
  onSelectThread,
  onResetThread,
  onArchiveThread,
  onReopenThread,
  onHandoffToWeb
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
              {onHandoffToWeb ? <button className="secondary" type="button" onClick={() => onHandoffToWeb(thread.threadId)}>Handoff to web</button> : null}
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
            <dt>Active profile</dt>
            <dd>{detail.activeProfileProjection?.profileId ?? "unavailable"}</dd>
            <dt>Continuity previews</dt>
            <dd>{detail.continuityPreviews?.length ?? 0}</dd>
            <dt>Lifecycle actions</dt>
            <dd>{detail.lifecycleActions.length}</dd>
            <dt>Reset events</dt>
            <dd>{detail.resetEvents?.length ?? 0}</dd>
            <dt>Handoff links</dt>
            <dd>{detail.handoffLinks?.length ?? 0}</dd>
            <dt>Conversation shape</dt>
            <dd>{detail.conversationShape?.shape ?? "unavailable"}</dd>
          </dl>
          {detail.conversationShape ? (
            <section aria-label="Conversation shape">
              <h4>Conversation Shape</h4>
              <ul>
                <li>
                  <span>{detail.conversationShape.shape}</span>
                  <span>{detail.conversationShape.shapeEvidenceStatus}</span>
                  <span>{detail.conversationShape.sourceConversationSummary || detail.conversationShape.sourceConversationId || "source redacted"}</span>
                  <span>{detail.conversationShape.redactionStatus}</span>
                </li>
              </ul>
            </section>
          ) : null}
          <section aria-label="Participation decisions">
            <h4>Participation Decisions</h4>
            {(detail.participationDecisions?.length ?? 0) === 0 ? <p className="muted">No participation decisions.</p> : null}
            <ul>
              {detail.participationDecisions?.map((decision, index) => (
                <li key={decision.participationDecisionId || `${decision.conversationShape}-${index}`}>
                  <span>{decision.decision}</span>
                  <span>{decision.reasonCode}</span>
                  <span>{decision.createdAssistantWork ? "assistant work created" : "no assistant work"}</span>
                  <span>{decision.safeSummary || "metadata only"}</span>
                  <span>{decision.redactionStatus}</span>
                </li>
              ))}
            </ul>
          </section>
          <section aria-label="Reset events">
            <h4>Reset Events</h4>
            {(detail.resetEvents?.length ?? 0) === 0 ? <p className="muted">No reset events.</p> : null}
            <ul>
              {detail.resetEvents?.map((event, index) => (
                <li key={event.resetEventId || `${event.threadId}-${index}`}>
                  <span>{event.status}</span>
                  <span>{event.conversationShape}</span>
                  <span>{event.reasonCode}</span>
                  <span>{event.priorSessionSegmentId || "prior segment unavailable"}</span>
                  <span>{event.resultingSessionSegmentId || "resulting segment unavailable"}</span>
                  <span>{event.redactionStatus}</span>
                </li>
              ))}
            </ul>
          </section>
          <section aria-label="Handoff links">
            <h4>Handoff Links</h4>
            {(detail.handoffLinks?.length ?? 0) === 0 ? <p className="muted">No handoff links.</p> : null}
            <ul>
              {detail.handoffLinks?.map((link, index) => (
                <li key={link.handoffLinkId || `${link.sourceThreadId}-${link.destinationThreadId}-${index}`}>
                  <span>{link.status}</span>
                  <span>{link.sourceConversationShape} to {link.destinationConversationShape}</span>
                  <span>{link.sourceReferenceStatus}</span>
                  <span>{link.sourceThreadId}</span>
                  <span>{link.destinationThreadId}</span>
                  <span>{link.redactionStatus}</span>
                </li>
              ))}
            </ul>
          </section>
          <section aria-label="Continuity evidence">
            <h4>Continuity Evidence</h4>
            <p className="muted">Bounded recent-thread evidence, not assistant memory.</p>
            {(detail.continuityPreviews?.length ?? 0) === 0 ? <p className="muted">No continuity evidence.</p> : null}
            <ul>
              {detail.continuityPreviews?.map((preview) => (
                <li key={preview.continuityPreviewId}>
                  <span>{preview.continuityPreviewId}</span>
                  <span>{preview.status}</span>
                  <span>{preview.continuityApplied ? "applied" : "not applied"}</span>
                  <span>{preview.includedCount} included</span>
                  <span>{preview.excludedCount} excluded</span>
                  <span>{preview.sessionSegmentId || "segment unavailable"}</span>
                  <span>{preview.windowPolicyId || "default policy"}</span>
                </li>
              ))}
            </ul>
          </section>
          {continuityPreviewDetail ? (
            <section aria-label="Continuity preview detail">
              <h4>Continuity Preview Detail</h4>
              <ul>
                {continuityPreviewDetail.items.map((item, index) => (
                  <li key={item.previewItemId || `${item.itemKind}-${index}`}>
                    <span>{item.decision}</span>
                    <span>{item.reasonCode}</span>
                    <span>{item.safeSummary || item.continuityTurnId || item.artifactRef || "metadata only"}</span>
                    <span>{item.redactionStatus}</span>
                  </li>
                ))}
              </ul>
            </section>
          ) : null}
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
            {detail.activeProfileProjection ? (
              <p className="muted">
                Profile {detail.activeProfileProjection.safeDisplayName} version {detail.activeProfileProjection.profileVersionId} is explicit configuration, not assistant memory.
              </p>
            ) : null}
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
