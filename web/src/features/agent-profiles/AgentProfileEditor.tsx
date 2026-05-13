import { useEffect, useState } from "react";

import type { AgentProfileDetailResponse, AgentProfileListResponse, AgentProfileMutationInput } from "@dope/client";

type OverlayInput = NonNullable<AgentProfileMutationInput["overlayReferences"]>[number];

type AgentProfileEditorProps = {
  profiles?: AgentProfileListResponse | null;
  detail?: AgentProfileDetailResponse | null;
  loading?: boolean;
  error?: string;
  denied?: boolean;
  draft?: AgentProfileMutationInput;
  onRefresh?: () => void;
  onSelectProfile?: (profileId: string) => void;
  onCreate?: (input: AgentProfileMutationInput) => void;
  onSave?: (input: AgentProfileMutationInput) => void;
  onActivate?: (profileId: string) => void;
  onArchive?: (profileId: string) => void;
  onDisable?: (profileId: string) => void;
};

export function AgentProfileEditor({
  profiles,
  detail = null,
  loading = false,
  error = "",
  denied = false,
  draft,
  onRefresh,
  onSelectProfile,
  onCreate,
  onSave,
  onActivate,
  onArchive,
  onDisable
}: AgentProfileEditorProps) {
  const items = profiles?.items ?? [];
  const current = detail?.profile ?? items[0] ?? null;
  const initialInput = draft ?? {
    displayName: current?.displayName ?? "",
    displayIdentity: current?.displayIdentity ?? {},
    persona: current?.persona ?? {},
    defaultProviderPreference: current?.defaultProviderPreference ?? {},
    safetyDefaults: current?.safetyDefaults ?? {},
    overlayReferences: detail?.overlayReferences?.map((overlay) => ({
      referenceKind: overlay.referenceKind,
      referenceUri: overlay.referenceUri,
      scope: overlay.scope
    })) ?? []
  };
  const [input, setInput] = useState<AgentProfileMutationInput>(initialInput);

  useEffect(() => {
    setInput(initialInput);
  }, [current?.profileId, draft, detail?.overlayReferences?.map((overlay) => overlay.overlayReferenceId).join(",")]);

  function updateInput(patch: Partial<AgentProfileMutationInput>) {
    setInput((previous) => ({ ...previous, ...patch }));
  }

  function updateOverlay(index: number, field: "referenceKind" | "referenceUri", value: string) {
    setInput((previous) => ({
      ...previous,
      overlayReferences: normalizeOverlayInputs((previous.overlayReferences ?? []).map((overlay, overlayIndex) => (
        overlayIndex === index ? { ...overlay, scope: overlay.scope || "profile", [field]: value } : overlay
      )))
    }));
  }

  function addOverlay() {
    setInput((previous) => ({
      ...previous,
      overlayReferences: [...(previous.overlayReferences ?? []), { referenceKind: "prompt", referenceUri: "", scope: "profile" }]
    }));
  }

  function removeOverlay(index: number) {
    setInput((previous) => ({
      ...previous,
      overlayReferences: (previous.overlayReferences ?? []).filter((_, overlayIndex) => overlayIndex !== index)
    }));
  }

  return (
    <section aria-label="Agent profiles">
      <header className="panel-header">
        <div>
          <h2>Agent Profiles</h2>
          <p>{profiles?.tenantId ? `Tenant ${profiles.tenantId}` : "Explicit persona and provider configuration"}</p>
        </div>
        <span className="status-pill">profiles.inspect</span>
      </header>
      {denied ? <p className="error-text">profiles.inspect is required to inspect agent profiles.</p> : null}
      {loading ? <p className="muted">Loading profiles.</p> : null}
      {error ? <p className="error-text">{error}</p> : null}
      {!loading && !error && !denied && items.length === 0 ? <p className="muted">No agent profiles are available.</p> : null}
      {onRefresh ? <button className="secondary" type="button" onClick={onRefresh}>Refresh</button> : null}
      <div className="channel-list">
        {items.map((profile) => (
          <article className="channel-row" key={profile.profileId}>
            <div>
              <h3>{profile.displayName}</h3>
              <p>{profile.persona.safeSummary ?? profile.displayIdentity.safeSummary ?? profile.profileId}</p>
            </div>
            <dl>
              <dt>Status</dt>
              <dd>{profile.status}</dd>
              <dt>Default</dt>
              <dd>{profile.tenantDefault ? "yes" : "no"}</dd>
            </dl>
            {onSelectProfile ? <button className="secondary" type="button" onClick={() => onSelectProfile(profile.profileId)}>Inspect</button> : null}
            <div className="row-actions">
              {onActivate ? <button className="secondary" type="button" onClick={() => onActivate(profile.profileId)}>Activate</button> : null}
              {onArchive ? <button className="secondary" type="button" onClick={() => onArchive(profile.profileId)}>Archive</button> : null}
              {onDisable ? <button className="secondary" type="button" onClick={() => onDisable(profile.profileId)}>Disable</button> : null}
            </div>
          </article>
        ))}
      </div>
      <form
        className="form-grid"
        onSubmit={(event) => {
          event.preventDefault();
          onSave?.(input);
        }}
      >
        <label>
          Display name
          <input value={input.displayName} onChange={(event) => updateInput({ displayName: event.target.value })} />
        </label>
        <label>
          Display identity
          <input value={input.displayIdentity?.name ?? ""} onChange={(event) => updateInput({ displayIdentity: { ...input.displayIdentity, name: event.target.value } })} />
        </label>
        <label>
          Tone
          <input value={input.persona?.tone ?? ""} onChange={(event) => updateInput({ persona: { ...input.persona, tone: event.target.value } })} />
        </label>
        <label>
          Provider
          <input value={input.defaultProviderPreference?.providerId ?? ""} onChange={(event) => updateInput({ defaultProviderPreference: { ...input.defaultProviderPreference, providerId: event.target.value } })} />
        </label>
        <label>
          Model
          <input value={input.defaultProviderPreference?.model ?? ""} onChange={(event) => updateInput({ defaultProviderPreference: { ...input.defaultProviderPreference, model: event.target.value } })} />
        </label>
        <label>
          Safety
          <input value={input.safetyDefaults?.approvalPosture ?? ""} onChange={(event) => updateInput({ safetyDefaults: { ...input.safetyDefaults, approvalPosture: event.target.value } })} />
        </label>
        <div className="form-grid wide-field">
          {(input.overlayReferences ?? []).map((overlay, index) => (
            <div className="form-grid wide-field" key={`${index}-${overlay.scope ?? "profile"}`}>
              <label>
                Overlay kind
                <input aria-label={`Overlay ${index + 1} kind`} value={overlay.referenceKind ?? ""} onChange={(event) => updateOverlay(index, "referenceKind", event.target.value)} />
              </label>
              <label>
                Overlay URI
                <input aria-label={`Overlay ${index + 1} URI`} value={overlay.referenceUri ?? ""} onChange={(event) => updateOverlay(index, "referenceUri", event.target.value)} />
              </label>
              <button className="secondary" type="button" onClick={() => removeOverlay(index)}>Remove overlay</button>
            </div>
          ))}
          <button className="secondary" type="button" onClick={addOverlay}>Add overlay</button>
        </div>
        {onSave ? <button type="submit" disabled={!current}>Save profile</button> : null}
        {onCreate ? <button className="secondary" type="button" onClick={() => onCreate(input)}>Create profile</button> : null}
      </form>
      {detail ? (
        <aside className="message-box">
          <span>Versions {detail.versions.length}</span>
          <span>Overlays {detail.overlayReferences.length}</span>
          {detail.overlayReferences.map((overlay) => (
            <span key={overlay.overlayReferenceId}>{overlay.referenceKind} {overlay.validationState} {overlay.safeDisplayLabel}</span>
          ))}
          <span>Evidence is explicit profile configuration, not assistant memory.</span>
        </aside>
      ) : null}
    </section>
  );
}

function normalizeOverlayInputs(overlays: OverlayInput[]): OverlayInput[] {
  return overlays.filter((overlay) => overlay.referenceUri.trim() || (overlay.referenceKind ?? "").trim());
}
