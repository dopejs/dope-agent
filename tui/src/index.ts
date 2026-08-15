#!/usr/bin/env node

import { parseArgs, runCLI, type TUIOptions } from "./cli.js";
import { runTUI } from "./app.js";

/** True when the invocation is a one-shot scriptable command (old CLI), false for interactive TUI. */
function hasCommandFlag(options: TUIOptions): boolean {
  return Boolean(
    options.query ||
    options.slackSetupConnectorId ||
    options.matrixSetupConnectorId ||
    options.listThreads ||
    options.listBindings ||
    options.listWorkspaces ||
    options.inspectThreadId ||
    options.traceThreadId ||
    options.inspectRunId ||
    options.continuityPreview ||
    options.resetThreadId ||
    options.archiveThreadId ||
    options.reopenThreadId ||
    options.handoffWebThreadId ||
    options.listProfiles ||
    options.inspectProfileId ||
    options.archiveProfileId ||
    options.disableProfileId ||
    options.profileHistoryId ||
    options.rollbackProfile
  );
}

async function main(): Promise<void> {
  try {
    const options = parseArgs(process.argv.slice(2));
    if (hasCommandFlag(options)) {
      process.exitCode = await runCLI(options);
    } else {
      runTUI({
        daemonURL: options.daemonURL,
        accessToken: options.accessToken,
        provider: options.provider,
        model: options.model
      });
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    process.stderr.write(`${message}\n`);
    process.exitCode = 1;
  }
}

void main();