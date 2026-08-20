import type { AgentProfileVersionResource } from "@kura/client";

type AgentProfileHistoryProps = {
  versions?: AgentProfileVersionResource[];
  onRollback?: (profileVersionId: string) => void;
};

export function AgentProfileHistory({ versions = [], onRollback }: AgentProfileHistoryProps) {
  return (
    <section aria-label="Agent profile history">
      <header className="panel-header">
        <div>
          <h2>Profile History</h2>
          <p>Retained versions and rollback evidence</p>
        </div>
        <span className="status-pill">profiles.inspect</span>
      </header>
      {versions.length === 0 ? <p className="muted">No profile versions are available.</p> : null}
      <div className="channel-list">
        {versions.map((version) => (
          <article className="channel-row" key={version.profileVersionId}>
            <div>
              <h3>Version {version.versionNumber}</h3>
              <p>{version.changeKind} · {version.changeSummary}</p>
            </div>
            <dl>
              <dt>Rollback</dt>
              <dd>{version.rollbackEligibility}</dd>
              <dt>Redaction</dt>
              <dd>{version.redactionStatus}</dd>
            </dl>
            {onRollback && version.rollbackEligibility === "eligible" ? (
              <button className="secondary" type="button" onClick={() => onRollback(version.profileVersionId)}>Rollback</button>
            ) : null}
          </article>
        ))}
      </div>
    </section>
  );
}
