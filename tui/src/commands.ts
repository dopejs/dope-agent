import type { DopeClient } from "@dope/client";

export type Role = "user" | "assistant" | "system" | "error";

export type ChatContext = {
  provider?: string;
  model?: string;
  threadId?: string;
};

export type CommandDeps = {
  client: DopeClient;
  push: (role: Role, content: string) => void;
  getContext: () => ChatContext;
  setContext: (patch: Partial<ChatContext>) => void;
  exit: () => void;
};

export type Command = {
  help: string;
  run: (args: string, deps: CommandDeps) => Promise<void>;
};

function err(deps: CommandDeps, e: unknown) {
  deps.push("error", "[error] " + (e instanceof Error ? e.message : String(e)));
}

function oneLine(items: unknown[]): string {
  return items.length === 0 ? "  (none)" : items.map((x) => "  " + JSON.stringify(x)).join("\n");
}

export const COMMANDS: Record<string, Command> = {
  "/help": {
    help: "Show this help",
    run: async (_args, deps) => {
      deps.push("system", Object.entries(COMMANDS).map(([name, c]) => name.padEnd(28) + c.help).join("\n") + "\n\nType a message to chat. End a line with \\ to continue multi-line.");
    },
  },
  "/exit": { help: "Quit", run: async (_a, deps) => deps.exit() },
  "/quit": { help: "Quit", run: async (_a, deps) => deps.exit() },
  "/model": {
    help: "Set the model (empty to clear)",
    run: async (args, deps) => {
      deps.setContext({ model: args.trim() || undefined });
      deps.push("system", "model = " + (args.trim() || "(default)"));
    },
  },
  "/provider": {
    help: "Set the provider (empty to clear)",
    run: async (args, deps) => {
      deps.setContext({ provider: args.trim() || undefined });
      deps.push("system", "provider = " + (args.trim() || "(default)"));
    },
  },
  "/thread": {
    help: "Set the active thread (empty to clear)",
    run: async (args, deps) => {
      deps.setContext({ threadId: args.trim() || undefined });
      deps.push("system", "thread = " + (args.trim() || "(new thread)"));
    },
  },
  "/threads": {
    help: "List threads",
    run: async (_args, deps) => {
      try {
        const list = await deps.client.listThreads();
        deps.push("system", "Threads (" + list.items.length + "):\n" + list.items.map((t) => "  " + t.threadId + "  " + t.lifecycleState + "  " + t.sourceKind + "  " + (t.sourceSummary ?? "")).join("\n") + "\nUse /thread <id> to continue one.");
      } catch (e) { err(deps, e); }
    },
  },
  "/reset": {
    help: "Reset a thread",
    run: async (args, deps) => {
      if (!args.trim()) { deps.push("error", "/reset <thread-id>"); return; }
      try { const r = await deps.client.resetThread(args.trim()); deps.push("system", "Thread " + r.threadId + " reset -> " + r.lifecycleState); } catch (e) { err(deps, e); }
    },
  },
  "/workspaces": {
    help: "List workspaces",
    run: async (_args, deps) => {
      try { const list = await deps.client.listWorkspaces(); deps.push("system", "Workspaces:\n" + list.workspaces.map((w) => "  " + w.workspaceId + "  " + w.status + (w.isDefault ? " (default)" : "") + "  " + w.displayName).join("\n")); } catch (e) { err(deps, e); }
    },
  },
  "/bindings": {
    help: "List workspace bindings",
    run: async (_args, deps) => {
      try { const list = await deps.client.listBindings(); deps.push("system", "Bindings:\n" + list.bindings.map((b) => "  " + b.bindingId + "  " + b.scopeKind + " " + b.scopeLabel + " -> " + (b.selectedProfileId ?? "-") + "/" + (b.selectedWorkspaceId ?? "-") + "  " + b.status).join("\n")); } catch (e) { err(deps, e); }
    },
  },
  "/profiles": {
    help: "List agent profiles",
    run: async (_args, deps) => {
      try { const list = await deps.client.listAgentProfiles(); deps.push("system", "Profiles:\n" + list.items.map((p) => "  " + p.profileId + "  " + p.status + (p.tenantDefault ? " (default)" : "") + "  " + p.displayName).join("\n")); } catch (e) { err(deps, e); }
    },
  },
  "/connectors": {
    help: "List channel connectors",
    run: async (_args, deps) => {
      try { const list = await deps.client.listChannelConnectors(); deps.push("system", "Connectors:\n" + list.items.map((c) => "  " + c.connectorId + "  " + c.connectorKind + "  " + c.healthStatus + "  " + c.displayName).join("\n")); } catch (e) { err(deps, e); }
    },
  },
  "/tenants": {
    help: "List tenants",
    run: async (_args, deps) => {
      try { const list = await deps.client.listTenants(); deps.push("system", "Tenants:\n" + list.items.map((t) => "  " + t.tenantId + "  " + t.tenantKind + "  " + t.displayName).join("\n")); } catch (e) { err(deps, e); }
    },
  },
  "/me": {
    help: "Show the authenticated principal",
    run: async (_args, deps) => {
      try { const me = await deps.client.getMe(); deps.push("system", "Principal: " + (me.principal?.principalId ?? me.token?.principalId ?? "unavailable")); } catch (e) { err(deps, e); }
    },
  },
  "/config": {
    help: "Show daemon config",
    run: async (_args, deps) => {
      try { const cfg = await deps.client.getConfig(); deps.push("system", "Config:\n" + JSON.stringify(cfg, null, 2)); } catch (e) { err(deps, e); }
    },
  },
};