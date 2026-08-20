import type {
  BindingListResponse,
  BindingResource,
  WorkspaceListResponse,
  CreateBindingInput
} from "@kura/client";

type WorkspaceBindingEditorProps = {
  workspaces?: WorkspaceListResponse | null;
  bindings?: BindingListResponse | null;
  loading?: boolean;
  error?: string;
  denied?: boolean;
  onRefresh?: () => void;
  onCreateBinding?: (input: CreateBindingInput) => void;
  onDisableBinding?: (bindingId: string) => void;
  onRepairBinding?: (bindingId: string) => void;
};

const REPAIR_HINT: Record<string, string> = {
  healthy: "Healthy",
  disabled: "Disabled",
  invalid: "Invalid — needs repair",
  stale: "Stale reference — needs repair",
  unsupported: "Unsupported connector",
  needs_repair: "Needs repair"
};

export function WorkspaceBindingEditor({
  workspaces,
  bindings,
  loading = false,
  error = "",
  denied = false,
  onRefresh,
  onCreateBinding,
  onDisableBinding,
  onRepairBinding
}: WorkspaceBindingEditorProps) {
  if (denied) {
    return <div role="alert">You do not have permission to view bindings.</div>;
  }
  return (
    <section aria-label="Workspace and capability bindings">
      <header>
        <h2>Workspace &amp; Capability Bindings</h2>
        {onRefresh ? (
          <button type="button" onClick={() => onRefresh()} disabled={loading}>
            Refresh
          </button>
        ) : null}
      </header>
      {error ? <p role="alert">{error}</p> : null}

      <h3>Workspaces</h3>
      <ul>
        {(workspaces?.workspaces ?? []).map((ws) => (
          <li key={ws.workspaceId} data-default={ws.isDefault}>
            {ws.displayName}
            {ws.isDefault ? " (default)" : ""} — {ws.status}
            <span aria-label="repair status">{REPAIR_HINT[ws.repairStatus] ?? ws.repairStatus}</span>
          </li>
        ))}
      </ul>

      <h3>Bindings</h3>
      <ul>
        {(bindings?.bindings ?? []).map((b: BindingResource) => (
          <li key={b.bindingId}>
            <span>{b.scopeKind}</span>: <span>{b.scopeLabel}</span> →{" "}
            <span>{b.selectedProfileId ?? "(profile unchanged)"}</span>
            {b.selectedWorkspaceId ? <span> / {b.selectedWorkspaceId}</span> : null}
            <span aria-label="binding status">{b.status}</span>
            <span aria-label="repair status">{REPAIR_HINT[b.repairStatus] ?? b.repairStatus}</span>
            {onDisableBinding && b.status === "active" ? (
              <button type="button" onClick={() => onDisableBinding(b.bindingId)}>
                Disable
              </button>
            ) : null}
            {onRepairBinding && b.repairStatus !== "healthy" ? (
              <button type="button" onClick={() => onRepairBinding(b.bindingId)}>
                Repair
              </button>
            ) : null}
          </li>
        ))}
      </ul>
      <p>
        Workspace bindings record identity only. They never grant filesystem, connector, or
        knowledge access, and never create memory-backed workspace knowledge.
      </p>
    </section>
  );
}
