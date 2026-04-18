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
    query
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
      io.stdout.write(formatUsage(terminal));
      return terminal.status === "completed" ? 0 : 1;
    }

    io.stdout.write("Waiting for reply...\n");
    const response = await client.queryChat(payload);
    io.stdout.write(`Assistant: ${response.reply}\n`);
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
    daemonURL: env.DOPE_DAEMON_URL?.trim() || "http://127.0.0.1:19191",
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
      case "--stream":
        options.stream = true;
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
    "  --stream            Use streaming mode",
    "  --help              Show this help"
  ].join("\n");
}

function formatUsage(response: ChatQueryResponse): string {
  return `Usage: in=${response.usage.inputTokens} out=${response.usage.outputTokens} total=${response.usage.totalTokens}\n`;
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
