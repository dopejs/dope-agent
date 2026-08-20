import { useEffect, useState, type FormEvent } from "react";

import type {
  ChannelManagementActionInput,
  ChannelConnectorDetailResource,
  ChannelConnectorListResponse,
  ChannelConnectorResource,
  ChannelManagementSupportEvidence
} from "@kura/client";

type ChannelManagementProps = {
  connectors?: ChannelConnectorListResponse | null;
  selected?: ChannelConnectorDetailResource | null;
  supportEvidence?: ChannelManagementSupportEvidence | null;
  loading?: boolean;
  error?: string;
  onDisableConnector?: (connectorId: string) => void;
  onReEnableConnector?: (connectorId: string) => void;
  onStartRepair?: (connectorId: string) => void;
  onUpdateRoutePolicy?: (connectorId: string, input: ChannelManagementActionInput) => void;
  onSelectConnector?: (connectorId: string) => void;
};

export function ChannelManagementView({
  connectors,
  selected,
  supportEvidence,
  loading = false,
  error = "",
  onDisableConnector,
  onReEnableConnector,
  onStartRepair,
  onUpdateRoutePolicy,
  onSelectConnector
}: ChannelManagementProps) {
  const items = connectors?.items ?? [];
  const [eligibleRooms, setEligibleRooms] = useState("");
  const [eligibleChannels, setEligibleChannels] = useState("");
  const [eligibleConversations, setEligibleConversations] = useState("");
  const [eligibleSenders, setEligibleSenders] = useState("");
  const [backgroundDeliveryEligible, setBackgroundDeliveryEligible] = useState(true);

  useEffect(() => {
    const policy = selected?.routePolicy;
    setEligibleRooms(linesFromArray(policy?.eligibleRooms));
    setEligibleChannels(linesFromArray(policy?.eligibleChannels));
    setEligibleConversations(linesFromArray(policy?.eligibleConversations));
    setEligibleSenders(linesFromArray(policy?.eligibleSenders));
    setBackgroundDeliveryEligible(policy?.backgroundDeliveryEligible ?? selected?.deliveryEligible ?? true);
  }, [selected?.connectorId, selected?.routePolicy, selected?.deliveryEligible]);

  function submitRoutePolicy(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected || !onUpdateRoutePolicy) {
      return;
    }
    onUpdateRoutePolicy(selected.connectorId, {
      eligibleRooms: arrayFromLines(eligibleRooms),
      eligibleChannels: arrayFromLines(eligibleChannels),
      eligibleConversations: arrayFromLines(eligibleConversations),
      eligibleSenders: arrayFromLines(eligibleSenders),
      backgroundDeliveryEligible
    });
  }

  return (
    <section className="channel-management" aria-label="Channel management">
      <header className="panel-header">
        <div>
          <h2>Channels</h2>
          <p>{connectors?.tenantId ? `Tenant ${connectors.tenantId}` : "Tenant-scoped connector management"}</p>
        </div>
        <span className="status-pill">{connectors?.page.order ?? "attention_disabled_ready_name_id"}</span>
      </header>

      {loading ? <p className="muted">Loading channels.</p> : null}
      {error ? <p className="error-text">{error}</p> : null}
      {!loading && !error && items.length === 0 ? <p className="muted">No production channel connectors are configured.</p> : null}

      <div className="channel-grid">
        <div className="channel-list">
          {items.map((item) => (
            <ChannelConnectorRow key={item.connectorId} connector={item} onSelectConnector={onSelectConnector} />
          ))}
        </div>
        <div className="channel-detail">
          {selected ? (
            <>
              <h3>{selected.displayName}</h3>
              <div className="channel-actions">
                <button type="button" onClick={() => onStartRepair?.(selected.connectorId)} disabled={!onStartRepair}>
                  Repair
                </button>
                {selected.enablementState === "disabled" ? (
                  <button type="button" onClick={() => onReEnableConnector?.(selected.connectorId)} disabled={!onReEnableConnector}>
                    Re-enable
                  </button>
                ) : (
                  <button type="button" onClick={() => onDisableConnector?.(selected.connectorId)} disabled={!onDisableConnector}>
                    Disable
                  </button>
                )}
              </div>
              <dl>
                <dt>State</dt>
                <dd>{selected.enablementState}</dd>
                <dt>Health</dt>
                <dd>{selected.healthStatus}</dd>
                <dt>Diagnostics</dt>
                <dd>{selected.diagnosticFreshness}</dd>
                <dt>Delivery</dt>
                <dd>{selected.deliveryEligible ? "eligible" : "blocked"}</dd>
              </dl>
              <section aria-label="Diagnostics">
                <h4>Diagnostics</h4>
                <p>{selected.diagnosticSummary?.reasonCode ?? "No current diagnostic reason."}</p>
              </section>
              <section aria-label="Route policy">
                <h4>Route policy</h4>
                <form className="route-policy-form" onSubmit={submitRoutePolicy}>
                  <label>
                    Rooms
                    <textarea
                      aria-label="Eligible rooms"
                      value={eligibleRooms}
                      onChange={(event) => setEligibleRooms(event.target.value)}
                      rows={3}
                    />
                  </label>
                  <label>
                    Channels
                    <textarea
                      aria-label="Eligible channels"
                      value={eligibleChannels}
                      onChange={(event) => setEligibleChannels(event.target.value)}
                      rows={3}
                    />
                  </label>
                  <label>
                    Conversations
                    <textarea
                      aria-label="Eligible conversations"
                      value={eligibleConversations}
                      onChange={(event) => setEligibleConversations(event.target.value)}
                      rows={3}
                    />
                  </label>
                  <label>
                    Senders
                    <textarea
                      aria-label="Eligible senders"
                      value={eligibleSenders}
                      onChange={(event) => setEligibleSenders(event.target.value)}
                      rows={3}
                    />
                  </label>
                  <label className="checkbox-row">
                    <input
                      type="checkbox"
                      checked={backgroundDeliveryEligible}
                      onChange={(event) => setBackgroundDeliveryEligible(event.target.checked)}
                    />
                    Background delivery eligible
                  </label>
                  <button type="submit" disabled={!onUpdateRoutePolicy}>Save route policy</button>
                </form>
                <p>{selected.routePolicy?.validationState ?? "valid"}</p>
              </section>
              <section aria-label="Routing decisions">
                <h4>Routing decisions</h4>
                <OutcomeList items={selected.recentRouteDecisions} empty="No recent routing decisions." />
              </section>
              <section aria-label="Foreground replies">
                <h4>Foreground replies</h4>
                <OutcomeList items={selected.foregroundReplyOutcomes} empty="No foreground reply outcomes." />
              </section>
              <section aria-label="Background delivery">
                <h4>Background delivery</h4>
                <OutcomeList items={selected.backgroundDeliveryOutcomes} empty="No background delivery outcomes." />
              </section>
              <section aria-label="Support evidence">
                <h4>Support evidence</h4>
                <p>{supportEvidence ? supportEvidence.redactionStatus : "Metadata-only evidence available to authorized users."}</p>
              </section>
            </>
          ) : (
            <p className="muted">Select a connector to inspect setup, diagnostics, route policy, replies, delivery, and support evidence.</p>
          )}
        </div>
      </div>
    </section>
  );
}

function ChannelConnectorRow({ connector, onSelectConnector }: { connector: ChannelConnectorResource; onSelectConnector?: (connectorId: string) => void }) {
  return (
    <article className="channel-row">
      <div>
        <h3>{connector.displayName}</h3>
        <p>{connector.connectorKind}</p>
      </div>
      <div>
        <span className="status-pill">{connector.enablementState}</span>
        <span className="status-pill">{connector.diagnosticFreshness}</span>
      </div>
      <p>{connector.nextAction?.label ?? "No action required"}</p>
      <button type="button" className="secondary" disabled={!onSelectConnector} onClick={() => onSelectConnector?.(connector.connectorId)}>
        Inspect
      </button>
    </article>
  );
}

function OutcomeList({ items, empty }: { items?: unknown[]; empty: string }) {
  const safeItems = Array.isArray(items) ? items : [];
  if (safeItems.length === 0) {
    return <p>{empty}</p>;
  }
  return (
    <ul className="outcome-list">
      {safeItems.slice(0, 5).map((item, index) => {
        const outcome = item as { status?: string; outcome?: string; reasonCode?: string };
        return (
          <li key={`${outcome.status ?? outcome.outcome ?? "outcome"}-${index}`}>
            <span>{outcome.status ?? outcome.outcome ?? "recorded"}</span>
            {outcome.reasonCode ? <small>{outcome.reasonCode}</small> : null}
          </li>
        );
      })}
    </ul>
  );
}

function linesFromArray(values?: string[]) {
  return Array.isArray(values) ? values.join("\n") : "";
}

function arrayFromLines(value: string) {
  return value
    .split(/\r?\n|,/)
    .map((item) => item.trim())
    .filter(Boolean);
}
