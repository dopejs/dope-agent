// Operator-shell product surface views (Roadmap 70). Presentational components for the Roadmap
// 65-71 surfaces, consuming @dope/client resource types. Each renders through SurfacePanel so the
// standard empty/error/denied/unsupported/loading states are consistent and explainable. Data
// fetching is done by the host shell via the SDK (the shell never bypasses daemon APIs).

import type {
  CatalogItemResource,
  EvidenceBundleResource,
  ExecutionProfileResource,
  RoutineResource,
  TriagePolicyResource,
  WebhookEndpointResource,
} from "@dope/client";

import type { ViewState } from "./navigation";

const STATE_MESSAGES: Partial<Record<ViewState, string>> = {
  loading: "Loading…",
  empty: "Nothing here yet.",
  error: "Something went wrong. Check the reason and retry.",
  denied: "Select a tenant or request access to view this surface.",
  unsupported: "This capability is not supported in the current configuration.",
};

type SurfacePanelProps = {
  title: string;
  state: ViewState;
  reason?: string;
  children?: React.ReactNode;
};

// SurfacePanel renders the standard non-ready states, or the surface body when ready.
export function SurfacePanel({ title, state, reason, children }: SurfacePanelProps) {
  return (
    <section className={`surface surface-${state}`} aria-label={title}>
      <h3 className="surface__title">{title}</h3>
      {state === "ready" ? (
        children
      ) : (
        <div className={`surface__state surface__state-${state}`} role="status">
          <p>{STATE_MESSAGES[state] ?? "Unavailable."}</p>
          {reason ? <p className="surface__reason">{reason}</p> : null}
        </div>
      )}
    </section>
  );
}

export function RoutinesView({ routines, state, reason }: { routines: RoutineResource[]; state: ViewState; reason?: string }) {
  return (
    <SurfacePanel title="Routines" state={state} reason={reason}>
      <ul className="surface__list">
        {routines.map((r) => (
          <li key={r.routineId} className="surface__row">
            <span className="surface__name">{r.name}</span>
            <span className={`status-chip status-${r.state}`}>{r.state}</span>
            <span className="surface__meta">v{r.currentVersion}</span>
          </li>
        ))}
      </ul>
    </SurfacePanel>
  );
}

export function WebhooksView({ endpoints, state, reason }: { endpoints: WebhookEndpointResource[]; state: ViewState; reason?: string }) {
  return (
    <SurfacePanel title="Webhook triggers" state={state} reason={reason}>
      <ul className="surface__list">
        {endpoints.map((e) => (
          <li key={e.webhookId} className="surface__row">
            <span className="surface__name">{e.name}</span>
            <span className={`status-chip status-${e.status}`}>{e.status}</span>
            <span className="surface__meta">{e.targetKind}:{e.targetRef}</span>
            <span className="surface__meta surface__redacted">{e.secretFingerprint}</span>
          </li>
        ))}
      </ul>
    </SurfacePanel>
  );
}

export function CatalogView({ items, state, reason }: { items: CatalogItemResource[]; state: ViewState; reason?: string }) {
  return (
    <SurfacePanel title="Skill & capability catalog" state={state} reason={reason}>
      <ul className="surface__list">
        {items.map((item) => (
          <li key={item.itemId} className="surface__row">
            <span className="surface__name">{item.name}</span>
            <span className={`trust-chip trust-${item.trustTier}`}>{item.trustTier}</span>
            <span className="surface__meta">{item.kind}</span>
            <span className="surface__meta">{item.versions.length} version(s)</span>
          </li>
        ))}
      </ul>
    </SurfacePanel>
  );
}

export function ExecutionProfilesView({ profiles, state, reason }: { profiles: ExecutionProfileResource[]; state: ViewState; reason?: string }) {
  return (
    <SurfacePanel title="Execution profiles" state={state} reason={reason}>
      <ul className="surface__list">
        {profiles.map(({ profile, status }) => (
          <li key={profile.profileId} className="surface__row">
            <span className="surface__name">{profile.name}</span>
            <span className={`status-chip status-${status.health}`}>{status.available ? "available" : status.health}</span>
            <span className={`risk-chip risk-${profile.riskTier}`}>{profile.riskTier}</span>
            {status.reason ? <span className="surface__reason">{status.reason}</span> : null}
          </li>
        ))}
      </ul>
    </SurfacePanel>
  );
}

export function TriagePoliciesView({ policies, state, reason }: { policies: TriagePolicyResource[]; state: ViewState; reason?: string }) {
  return (
    <SurfacePanel title="Inbox triage policies" state={state} reason={reason}>
      <ul className="surface__list">
        {policies.map((p) => (
          <li key={p.policyId} className="surface__row">
            <span className="surface__name">{p.name}</span>
            <span className="surface__meta">{p.rules.length} rule(s)</span>
            <span className="surface__meta">default: {p.defaultClassification}</span>
          </li>
        ))}
      </ul>
    </SurfacePanel>
  );
}

export function SupportEvidenceView({ bundles, state, reason }: { bundles: EvidenceBundleResource[]; state: ViewState; reason?: string }) {
  return (
    <SurfacePanel title="Support evidence" state={state} reason={reason}>
      <ul className="surface__list">
        {bundles.map((b) => (
          <li key={b.bundleId} className="surface__row">
            <span className="surface__name">{b.scope.kind}{b.scope.ref ? `:${b.scope.ref}` : ""}</span>
            <span className={`status-chip status-${b.redactionStatus}`}>{b.redactionStatus}</span>
            <span className="surface__meta">{(b.sections ?? []).length} section(s)</span>
          </li>
        ))}
      </ul>
    </SurfacePanel>
  );
}
