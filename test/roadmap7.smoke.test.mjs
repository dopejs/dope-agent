import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { Writable } from "node:stream";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

test("roadmap 7 smoke: built sdk and built tui share the daemon chat contract", async () => {
  const { createDopeClient } = await import(pathToFileURL(path.join(repoRoot, "sdk/ts/dist/index.js")).href);
  const { runCLI } = await import(pathToFileURL(path.join(repoRoot, "tui/dist/cli.js")).href);

  const fetchImpl = createMockFetch();
  const client = createDopeClient({
    baseURL: "http://dope.local",
    accessToken: "smoke-token",
    fetchImpl
  });

  const nonStream = await client.queryChat({ query: "hello smoke" });
  assert.equal(nonStream.reply, "mock reply");

  let streamReply = "";
  const terminal = await client.streamChatQuery(
    { query: "hello smoke" },
    {
      onDelta(payload) {
        streamReply = payload.reply;
      }
    }
  );
  assert.equal(streamReply, "mock stream reply");
  assert.equal(terminal.reply, "mock stream reply");

  let tuiStdout = "";
  let tuiStderr = "";
  const exitCode = await runCLI(
    {
      daemonURL: "http://dope.local",
      accessToken: "smoke-token",
      query: "hello smoke",
      stream: false
    },
    {
      io: {
        stdin: process.stdin,
        stdout: createCaptureStream((chunk) => {
          tuiStdout += chunk;
        }),
        stderr: createCaptureStream((chunk) => {
          tuiStderr += chunk;
        })
      },
      createClient(options) {
        return createDopeClient({
          baseURL: options.baseURL,
          accessToken: options.accessToken,
          fetchImpl
        });
      }
    }
  );
  assert.equal(exitCode, 0, tuiStderr);
  assert.match(tuiStdout, /Assistant: mock reply/);

  const builtHTML = await fs.readFile(path.join(repoRoot, "web/dist/index.html"), "utf8");
  assert.match(builtHTML, /DopeAgent/i);
});

function createMockFetch() {
  return async function mockFetch(input, init = {}) {
    const url = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
    const pathname = new URL(url).pathname;
    const headers = new Headers(init.headers ?? {});

    if (headers.get("authorization") !== "Bearer smoke-token") {
      return jsonResponse(401, { error: "unauthorized" });
    }

    if (init.method === "POST" && pathname === "/v1/chat/query") {
      return jsonResponse(200, {
        dispatchId: "dispatch_1",
        provider: "openai_compatible",
        model: "gpt-test",
        query: "hello smoke",
        status: "completed",
        reply: "mock reply",
        usage: { inputTokens: 1, outputTokens: 2, totalTokens: 3 }
      });
    }

    if (init.method === "POST" && pathname === "/v1/chat/query/stream") {
      return new Response(
        createSSEStream([
          'event: chat.query.started\ndata: {"dispatchId":"dispatch_1","provider":"openai_compatible","model":"gpt-test","query":"hello smoke"}\n\n',
          'event: chat.query.delta\ndata: {"dispatchId":"dispatch_1","delta":"mock ","reply":"mock "}\n\n',
          'event: chat.query.delta\ndata: {"dispatchId":"dispatch_1","delta":"stream reply","reply":"mock stream reply"}\n\n',
          'event: chat.query.completed\ndata: {"dispatchId":"dispatch_1","provider":"openai_compatible","model":"gpt-test","query":"hello smoke","status":"completed","reply":"mock stream reply","usage":{"inputTokens":1,"outputTokens":3,"totalTokens":4}}\n\n'
        ]),
        {
          status: 200,
          headers: { "Content-Type": "text/event-stream" }
        }
      );
    }

    return jsonResponse(404, { error: "not found" });
  };
}

function jsonResponse(status, payload) {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "Content-Type": "application/json" }
  });
}

function createSSEStream(events) {
  return new ReadableStream({
    start(controller) {
      for (const event of events) {
        controller.enqueue(new TextEncoder().encode(event));
      }
      controller.close();
    }
  });
}

function createCaptureStream(onChunk) {
  return new Writable({
    write(chunk, _encoding, callback) {
      onChunk(String(chunk));
      callback();
    }
  });
}
