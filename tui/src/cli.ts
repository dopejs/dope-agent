import { createInterface } from "node:readline/promises";
import { stdin, stdout, stderr } from "node:process";

import { createDopeClient, type ChatQueryInput, type ChatQueryResponse, type DopeClient } from "@dope/client";

export type TUIOptions = {
  daemonURL: string;
  accessToken?: string;
  provider?: string;
  model?: string;
  stream: boolean;
  query?: string;
  threadId?: string;
  slackSetupConnectorId?: string;
  matrixSetupConnectorId?: string;
  listThreads?: boolean;
  listBindings?: boolean;
  listWorkspaces?: boolean;
  inspectThreadId?: string;
  traceThreadId?: string;
  inspectRunId?: string;
  continuityPreview?: { threadId: string; previewId: string };
  resetThreadId?: string;
  archiveThreadId?: string;
  reopenThreadId?: string;
  handoffWebThreadId?: string;
  listProfiles?: boolean;
  inspectProfileId?: string;
  archiveProfileId?: string;
  disableProfileId?: string;
  profileHistoryId?: string;
  rollbackProfile?: { profileId: string; versionId: string };
};

type CLIIO = {
  stdin: NodeJS.ReadableStream;
  stdout: NodeJS.WritableStream;
  stderr: NodeJS.WritableStream;
};

export type RunCLIDependencies = {
  io?: CLIIO;
  createClient?: (options: { baseURL: string; accessToken?: string }) => DopeClient;
};

export async function runCLI(options: TUIOptions, deps: RunCLIDependencies = {}): Promise<number> {
  const io = deps.io ?? { stdin, stdout, stderr };
  const createClient = deps.createClient ?? createDopeClient;
  const client = createClient({
    baseURL: options.daemonURL,
    accessToken: options.accessToken
  });

  if (options.slackSetupConnectorId) {
    try {
      const setup = await client.getSlackSetup(options.slackSetupConnectorId);
      io.stdout.write(`Slack Setup: ${setup.connectorId}\n`);
      io.stdout.write(`State: ${setup.terminalState}\n`);
      io.stdout.write(`OAuth: ${setup.oauthState}\n`);
      io.stdout.write(`Route Policy: ${setup.routePolicyState}\n`);
      io.stdout.write(`Delivery Eligible: ${setup.deliveryEligible ? "yes" : "no"}\n`);
      return 0;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      io.stderr.write(`Error: ${message}\n`);
      return 1;
    }
  }

  if (options.matrixSetupConnectorId) {
    try {
      const setup = await client.getMatrixSetup(options.matrixSetupConnectorId);
      io.stdout.write(`Matrix Setup: ${setup.connectorId}\n`);
      io.stdout.write(`State: ${setup.terminalState}\n`);
      io.stdout.write(`Homeserver: ${setup.homeserverState}\n`);
      io.stdout.write(`Route Policy: ${setup.routePolicyState}\n`);
      io.stdout.write(`Delivery Eligible: ${setup.deliveryEligible ? "yes" : "no"}\n`);
      return 0;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      io.stderr.write(`Error: ${message}\n`);
      return 1;
    }
  }

  if (options.listThreads) {
    try {
      const list = await client.listThreads();
      io.stdout.write(`Threads: ${list.tenantId}\n`);
      for (const thread of list.items) {
        io.stdout.write(`${thread.threadId} ${thread.lifecycleState} ${thread.sourceKind} ${thread.sourceSummary ?? ""}\n`);
      }
      return 0;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      io.stderr.write(`Error: ${message}\n`);
      return 1;
    }
  }

  if (options.listWorkspaces) {
    try {
      const list = await client.listWorkspaces();
      io.stdout.write(`Workspaces: ${list.tenantId ?? ""}\n`);
      for (const ws of list.workspaces) {
        io.stdout.write(`${ws.workspaceId} ${ws.status}${ws.isDefault ? " (default)" : ""} repair=${ws.repairStatus} ${ws.displayName}\n`);
      }
      return 0;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      io.stderr.write(`Error: ${message}\n`);
      return 1;
    }
  }

  if (options.listBindings) {
    try {
      const list = await client.listBindings();
      io.stdout.write(`Bindings: ${list.tenantId ?? ""}\n`);
      for (const binding of list.bindings) {
        io.stdout.write(`${binding.bindingId} ${binding.scopeKind} ${binding.scopeLabel} -> ${binding.selectedProfileId ?? "-"}/${binding.selectedWorkspaceId ?? "-"} ${binding.status} repair=${binding.repairStatus}\n`);
      }
      return 0;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      io.stderr.write(`Error: ${message}\n`);
      return 1;
    }
  }

  if (options.listProfiles) {
    try {
      const list = await client.listAgentProfiles();
      io.stdout.write(`Profiles: ${list.tenantId}\n`);
      for (const profile of list.items) {
        io.stdout.write(`${profile.profileId} ${profile.status} default=${profile.tenantDefault ? "yes" : "no"} ${profile.displayName}\n`);
      }
      return 0;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      io.stderr.write(`Error: ${message}\n`);
      return 1;
    }
  }

  if (options.inspectProfileId) {
    try {
      const detail = await client.getAgentProfile(options.inspectProfileId);
      io.stdout.write(`Profile: ${detail.profile.profileId}\n`);
      io.stdout.write(`Name: ${detail.profile.displayName}\n`);
      io.stdout.write(`Status: ${detail.profile.status}\n`);
      io.stdout.write(`Default: ${detail.profile.tenantDefault ? "yes" : "no"}\n`);
      io.stdout.write(`Summary: ${detail.profile.persona.safeSummary ?? detail.profile.displayIdentity.safeSummary ?? "metadata only"}\n`);
      io.stdout.write(`Versions: ${detail.versions.length}\n`);
      io.stdout.write(`Overlays: ${detail.overlayReferences.length}\n`);
      for (const overlay of detail.overlayReferences) {
        io.stdout.write(`- Overlay ${overlay.referenceKind} ${overlay.validationState} ${overlay.safeDisplayLabel}\n`);
      }
      io.stdout.write("Evidence: explicit profile configuration, not assistant memory\n");
      return 0;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      io.stderr.write(`Error: ${message}\n`);
      return 1;
    }
  }

  if (options.profileHistoryId) {
    try {
      const history = await client.listAgentProfileVersions(options.profileHistoryId);
      io.stdout.write(`Profile History: ${options.profileHistoryId}\n`);
      for (const version of history.items) {
        io.stdout.write(`${version.profileVersionId} v${version.versionNumber} ${version.changeKind} rollback=${version.rollbackEligibility} ${version.changeSummary}\n`);
      }
      return 0;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      io.stderr.write(`Error: ${message}\n`);
      return 1;
    }
  }

  if (options.rollbackProfile) {
    try {
      const result = await client.rollbackAgentProfile(options.rollbackProfile.profileId, {
        sourceProfileVersionId: options.rollbackProfile.versionId,
        reasonCode: "tui_profile_rollback"
      });
      io.stdout.write(`Profile ${result.profile.profileId} rollback completed.\n`);
      io.stdout.write(`Version: ${result.version.profileVersionId}\n`);
      io.stdout.write(`Status: ${result.profile.status}\n`);
      return 0;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      io.stderr.write(`Error: ${message}\n`);
      return 1;
    }
  }

  const profileRetirementCommand = options.archiveProfileId
    ? { profileId: options.archiveProfileId, action: "archive" as const, run: client.archiveAgentProfile.bind(client) }
    : options.disableProfileId
      ? { profileId: options.disableProfileId, action: "disable" as const, run: client.disableAgentProfile.bind(client) }
      : null;
  if (profileRetirementCommand) {
    try {
      const result = await profileRetirementCommand.run(profileRetirementCommand.profileId, { reasonCode: `tui_profile_${profileRetirementCommand.action}` });
      io.stdout.write(`Profile ${result.profile.profileId} ${profileRetirementCommand.action} completed.\n`);
      io.stdout.write(`Status: ${result.profile.status}\n`);
      if (result.selection?.selectionReason) {
        io.stdout.write(`Fallback: ${result.selection.selectionReason} ${result.selection.profileId}\n`);
      }
      return 0;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      io.stderr.write(`Error: ${message}\n`);
      return 1;
    }
  }

  if (options.inspectThreadId) {
    try {
      const detail = await client.getThread(options.inspectThreadId);
      const thread = detail.thread;
      io.stdout.write(`Thread: ${thread.threadId}\n`);
      io.stdout.write(`Tenant: ${thread.tenantId}\n`);
      io.stdout.write(`State: ${thread.lifecycleState}\n`);
      io.stdout.write(`Source: ${thread.sourceKind} ${thread.sourceSummary ?? ""}\n`);
      io.stdout.write(`Current Session: ${thread.currentSessionId ?? thread.currentSessionSegmentId ?? "unavailable"}\n`);
      io.stdout.write("Evidence: lifecycle metadata, not assistant memory\n");
      io.stdout.write(`Retention: ${thread.retentionExpiresAt ?? "policy default"}\n`);
      io.stdout.write(`Redaction: ${thread.redactionStatus}\n`);
      io.stdout.write(`Segments: ${detail.sessionSegments.length}\n`);
      io.stdout.write(`Source Linkages: ${detail.sourceLinkages.length}\n`);
      io.stdout.write(`Runtime Projections: ${detail.runtimeProjections.length}\n`);
      if (detail.activeProfileProjection) {
        io.stdout.write(`Active Profile: ${detail.activeProfileProjection.profileId} version=${detail.activeProfileProjection.profileVersionId} evidence=${detail.activeProfileProjection.configurationScope ?? "explicit_profile_configuration"}\n`);
      }
      io.stdout.write(`Continuity Previews: ${detail.continuityPreviews?.length ?? 0}\n`);
      io.stdout.write(`Conversation Shape: ${detail.conversationShape?.shape ?? "unavailable"}\n`);
      io.stdout.write(`Participation Decisions: ${detail.participationDecisions?.length ?? 0}\n`);
      io.stdout.write(`Reset Events: ${detail.resetEvents?.length ?? 0}\n`);
      return 0;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      io.stderr.write(`Error: ${message}\n`);
      return 1;
    }
  }

  if (options.traceThreadId) {
    try {
      const detail = await client.getThread(options.traceThreadId);
      const thread = detail.thread;
      io.stdout.write(`Thread Trace: ${thread.threadId}\n`);
      io.stdout.write(`State: ${thread.lifecycleState}\n`);
      io.stdout.write("Evidence: lifecycle metadata, not assistant memory\n");
      io.stdout.write(`Retention: ${thread.retentionExpiresAt ?? "policy default"}\n`);
      io.stdout.write(`Redaction: ${thread.redactionStatus}\n`);
      io.stdout.write(`Conversation Shape: ${detail.conversationShape?.shape ?? "unavailable"}\n`);
      io.stdout.write("Participation Decisions:\n");
      for (const decision of detail.participationDecisions ?? []) {
        io.stdout.write(`- ${decision.decision} ${decision.reasonCode} work=${decision.createdAssistantWork ? "yes" : "no"} ${decision.safeSummary ?? "metadata only"} redaction=${decision.redactionStatus}\n`);
      }
      if ((detail.participationDecisions?.length ?? 0) === 0) {
        io.stdout.write("- none\n");
      }
      io.stdout.write("Reset Events:\n");
      for (const event of detail.resetEvents ?? []) {
        io.stdout.write(`- ${event.status} ${event.conversationShape} ${event.reasonCode} prior=${event.priorSessionSegmentId ?? "unavailable"} current=${event.resultingSessionSegmentId ?? "unavailable"} redaction=${event.redactionStatus}\n`);
      }
      if ((detail.resetEvents?.length ?? 0) === 0) {
        io.stdout.write("- none\n");
      }
      io.stdout.write("Handoff Links:\n");
      for (const link of detail.handoffLinks ?? []) {
        io.stdout.write(`- ${link.status} ${link.sourceConversationShape}->${link.destinationConversationShape} refs=${link.sourceReferenceStatus} source=${link.sourceThreadId} destination=${link.destinationThreadId} redaction=${link.redactionStatus}\n`);
      }
      if ((detail.handoffLinks?.length ?? 0) === 0) {
        io.stdout.write("- none\n");
      }
      io.stdout.write("Source Trace:\n");
      for (const linkage of detail.sourceLinkages) {
        io.stdout.write(`- ${linkage.routingOutcome} ${linkage.connectorKind ?? linkage.sourceKind} ${linkage.sourceConversationId ?? "conversation redacted"} redaction=${linkage.redactionStatus} retention=${linkage.retentionExpiresAt ?? "policy default"}\n`);
      }
      if (detail.sourceLinkages.length === 0) {
        io.stdout.write("- none\n");
      }
      io.stdout.write("Runtime Trace:\n");
      if (detail.activeProfileProjection) {
        io.stdout.write(`- active-profile ${detail.activeProfileProjection.profileId} version=${detail.activeProfileProjection.profileVersionId} ${detail.activeProfileProjection.safeSummary}\n`);
      }
      for (const projection of detail.runtimeProjections) {
        io.stdout.write(`- ${projection.resourceKind} ${projection.status} ${projection.safeSummary ?? projection.reasonCode ?? "metadata only"} redaction=${projection.redactionStatus} retention=${projection.retentionExpiresAt ?? "policy default"}\n`);
      }
      if (detail.runtimeProjections.length === 0) {
        io.stdout.write("- none\n");
      }
      io.stdout.write("Continuity Evidence:\n");
      for (const preview of detail.continuityPreviews ?? []) {
        io.stdout.write(`- ${preview.status} applied=${preview.continuityApplied ? "yes" : "no"} included=${preview.includedCount} excluded=${preview.excludedCount} segment=${preview.sessionSegmentId ?? "unavailable"} policy=${preview.windowPolicyId ?? "default"}\n`);
      }
      if ((detail.continuityPreviews?.length ?? 0) === 0) {
        io.stdout.write("- none\n");
      }
      return 0;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      io.stderr.write(`Error: ${message}\n`);
      return 1;
    }
  }

  if (options.inspectRunId) {
    try {
      const run = await client.getRun(options.inspectRunId);
      io.stdout.write(`Run: ${run.runId}\n`);
      io.stdout.write(`Status: ${run.status}\n`);
      io.stdout.write(`Entrypoint: ${run.entrypoint}\n`);
      if (run.activeProfileProjection) {
        io.stdout.write(`Active Profile: ${run.activeProfileProjection.profileId} version=${run.activeProfileProjection.profileVersionId} evidence=${run.activeProfileProjection.configurationScope ?? "explicit_profile_configuration"}\n`);
      }
      return 0;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      io.stderr.write(`Error: ${message}\n`);
      return 1;
    }
  }

  if (options.continuityPreview) {
    try {
      const detail = await client.getThreadContinuityPreview(options.continuityPreview.threadId, options.continuityPreview.previewId);
      io.stdout.write(`Continuity Preview: ${detail.preview.continuityPreviewId}\n`);
      io.stdout.write(`Thread: ${detail.preview.threadId ?? options.continuityPreview.threadId}\n`);
      io.stdout.write(`Status: ${detail.preview.status}\n`);
      io.stdout.write("Evidence: bounded recent-thread continuity, not assistant memory\n");
      io.stdout.write(`Applied: ${detail.preview.continuityApplied ? "yes" : "no"}\n`);
      io.stdout.write(`Items: ${detail.items.length}\n`);
      for (const item of detail.items) {
        io.stdout.write(`- ${item.decision} ${item.reasonCode} ${item.safeSummary ?? item.continuityTurnId ?? item.artifactRef ?? "metadata only"} redaction=${item.redactionStatus}\n`);
      }
      return 0;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      io.stderr.write(`Error: ${message}\n`);
      return 1;
    }
  }

  const lifecycleCommand = options.resetThreadId
    ? { threadId: options.resetThreadId, action: "reset" as const, run: client.resetThread.bind(client) }
    : options.archiveThreadId
      ? { threadId: options.archiveThreadId, action: "archive" as const, run: client.archiveThread.bind(client) }
      : options.reopenThreadId
        ? { threadId: options.reopenThreadId, action: "reopen" as const, run: client.reopenThread.bind(client) }
        : null;
  if (lifecycleCommand) {
    try {
      const response = await lifecycleCommand.run(lifecycleCommand.threadId, { reasonCode: `tui_${lifecycleCommand.action}` });
      io.stdout.write(`Thread ${response.threadId} ${lifecycleCommand.action} completed.\n`);
      io.stdout.write(`State: ${response.lifecycleState}\n`);
      io.stdout.write(`Audit: ${response.auditEventId}\n`);
      io.stdout.write(`Current Segment: ${response.currentSessionSegmentId ?? "unchanged"}\n`);
      return 0;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      io.stderr.write(`Error: ${message}\n`);
      return 1;
    }
  }

  if (options.handoffWebThreadId) {
    try {
      const response = await client.createThreadHandoff(options.handoffWebThreadId, { destination: { surface: "web" }, reasonCode: "tui_handoff_web" });
      io.stdout.write(`Thread ${response.sourceThreadId} handoff completed.\n`);
      io.stdout.write(`Destination Thread: ${response.destinationThreadId}\n`);
      io.stdout.write(`Status: ${response.status}\n`);
      io.stdout.write(`Source References: ${response.sourceReferenceStatus}\n`);
      return 0;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      io.stderr.write(`Error: ${message}\n`);
      return 1;
    }
  }

  const query = options.query?.trim() || (await promptQuery(io));
  if (!query) {
    io.stderr.write("Query is required.\n");
    return 1;
  }

  io.stdout.write("DopeAgent TUI · single-turn mode\n");
  io.stdout.write(`Daemon: ${options.daemonURL}\n`);
  io.stdout.write(`Mode: ${options.stream ? "stream" : "non-stream"}\n\n`);

  const payload: ChatQueryInput = {
    provider: options.provider,
    model: options.model,
    query,
    threadId: options.threadId
  };

  try {
    if (options.stream) {
      io.stdout.write("Assistant: ");
      let wroteDelta = false;
      const terminal = await client.streamChatQuery(payload, {
        onDelta(chunk) {
          wroteDelta = true;
          io.stdout.write(chunk.delta);
        }
      });
      if (!wroteDelta) {
        io.stdout.write(terminal.reply);
      }
      io.stdout.write("\n");
      io.stdout.write(formatContinuity(terminal));
      io.stdout.write(formatUsage(terminal));
      return terminal.status === "completed" ? 0 : 1;
    }

    io.stdout.write("Waiting for reply...\n");
    const response = await client.queryChat(payload);
    io.stdout.write(`Assistant: ${response.reply}\n`);
    io.stdout.write(formatContinuity(response));
    io.stdout.write(formatUsage(response));
    return response.status === "completed" ? 0 : 1;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    io.stderr.write(`Error: ${message}\n`);
    return 1;
  }
}

export function parseArgs(argv: string[], env: NodeJS.ProcessEnv = process.env): TUIOptions {
  const options: TUIOptions = {
    daemonURL: env.DOPE_DAEMON_URL?.trim() || "http://127.0.0.1:19192",
    accessToken: env.DOPE_ACCESS_TOKEN?.trim() || undefined,
    provider: env.DOPE_CHAT_PROVIDER?.trim() || undefined,
    model: env.DOPE_CHAT_MODEL?.trim() || undefined,
    stream: false
  };

  for (let index = 0; index < argv.length; index += 1) {
    const current = argv[index];
    switch (current) {
      case "--daemon-url":
        options.daemonURL = argv[++index] ?? options.daemonURL;
        break;
      case "--token":
        options.accessToken = argv[++index] ?? options.accessToken;
        break;
      case "--provider":
        options.provider = argv[++index] ?? options.provider;
        break;
      case "--model":
        options.model = argv[++index] ?? options.model;
        break;
      case "--query":
        options.query = argv[++index] ?? options.query;
        break;
      case "--thread-id":
        options.threadId = argv[++index] ?? options.threadId;
        break;
      case "--stream":
        options.stream = true;
        break;
      case "--slack-setup":
        options.slackSetupConnectorId = argv[++index] ?? options.slackSetupConnectorId;
        break;
      case "--matrix-setup":
        options.matrixSetupConnectorId = argv[++index] ?? options.matrixSetupConnectorId;
        break;
      case "--threads":
        options.listThreads = true;
        break;
      case "--workspaces":
        options.listWorkspaces = true;
        break;
      case "--bindings":
        options.listBindings = true;
        break;
      case "--profiles":
        options.listProfiles = true;
        break;
      case "--profile":
        options.inspectProfileId = argv[++index] ?? options.inspectProfileId;
        break;
      case "--profile-archive":
        options.archiveProfileId = argv[++index] ?? options.archiveProfileId;
        break;
      case "--profile-disable":
        options.disableProfileId = argv[++index] ?? options.disableProfileId;
        break;
      case "--profile-history":
        options.profileHistoryId = argv[++index] ?? options.profileHistoryId;
        break;
      case "--profile-rollback": {
        const [profileId = "", versionId = ""] = (argv[++index] ?? "").split(":", 2);
        options.rollbackProfile = { profileId, versionId };
        break;
      }
      case "--thread":
        options.inspectThreadId = argv[++index] ?? options.inspectThreadId;
        break;
      case "--thread-trace":
        options.traceThreadId = argv[++index] ?? options.traceThreadId;
        break;
      case "--run":
        options.inspectRunId = argv[++index] ?? options.inspectRunId;
        break;
      case "--continuity-preview": {
        const [threadId = "", previewId = ""] = (argv[++index] ?? "").split(":", 2);
        options.continuityPreview = { threadId, previewId };
        break;
      }
      case "--thread-reset":
        options.resetThreadId = argv[++index] ?? options.resetThreadId;
        break;
      case "--thread-archive":
        options.archiveThreadId = argv[++index] ?? options.archiveThreadId;
        break;
      case "--thread-reopen":
        options.reopenThreadId = argv[++index] ?? options.reopenThreadId;
        break;
      case "--thread-handoff-web":
        options.handoffWebThreadId = argv[++index] ?? options.handoffWebThreadId;
        break;
      case "--help":
        throw new Error(helpText());
      default:
        break;
    }
  }

  return options;
}

export function helpText(): string {
  return [
    "DopeAgent TUI",
    "",
    "Options:",
    "  --daemon-url <url>  Daemon base URL",
    "  --token <token>     Access token for daemon auth",
    "  --provider <name>   Optional provider override",
    "  --model <name>      Optional model override",
    "  --query <text>      Single-turn query",
    "  --thread-id <id>    Attach chat query to a daemon thread",
    "  --stream            Use streaming mode",
    "  --slack-setup <id>  Print Slack hosted setup state",
    "  --matrix-setup <id> Print Matrix hosted setup state",
    "  --threads           List tenant thread lifecycle metadata",
    "  --profiles          List tenant agent profiles",
    "  --profile <id>      Inspect one tenant agent profile",
    "  --profile-history <id> Print retained profile versions",
    "  --profile-rollback <profileId:versionId> Roll back one profile",
    "  --profile-archive <id> Archive one tenant agent profile",
    "  --profile-disable <id> Disable one tenant agent profile",
    "  --thread <id>       Inspect one tenant thread",
    "  --thread-trace <id> Print source-to-runtime trace evidence",
    "  --run <id>          Inspect one tenant run",
    "  --continuity-preview <threadId:previewId> Inspect one continuity preview",
    "  --thread-reset <id> Reset one tenant thread",
    "  --thread-archive <id> Archive one tenant thread",
    "  --thread-reopen <id> Reopen one tenant thread",
    "  --thread-handoff-web <id> Hand off one tenant thread to web",
    "  --help              Show this help"
  ].join("\n");
}

function formatUsage(response: ChatQueryResponse): string {
  return `Usage: in=${response.usage.inputTokens} out=${response.usage.outputTokens} total=${response.usage.totalTokens}\n`;
}

function formatContinuity(response: ChatQueryResponse): string {
  if (!response.threadId) {
    return "";
  }
  return `Continuity: thread=${response.threadId} preview=${response.continuityPreviewId ?? "unavailable"} applied=${response.continuityApplied ? "yes" : "no"} included=${response.continuityIncludedCount ?? 0} excluded=${response.continuityExcludedCount ?? 0} evidence=bounded-not-memory\n`;
}

async function promptQuery(io: CLIIO): Promise<string> {
  const rl = createInterface({
    input: io.stdin,
    output: io.stdout
  });
  try {
    return (await rl.question("Query: ")).trim();
  } finally {
    rl.close();
  }
}
