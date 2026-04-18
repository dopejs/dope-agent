import { describe, expect, it, vi } from "vitest";

import { parseArgs, runCLI } from "./cli";

describe("tui cli", () => {
  it("parses flags and env", () => {
    const options = parseArgs(["--daemon-url", "http://localhost:9999", "--stream", "--query", "hello"], {
      DOPE_ACCESS_TOKEN: "token"
    });

    expect(options.daemonURL).toBe("http://localhost:9999");
    expect(options.accessToken).toBe("token");
    expect(options.stream).toBe(true);
    expect(options.query).toBe("hello");
  });

  it("runs a non-stream query and prints reply", async () => {
    const stdout = createMemoryWriter();
    const stderr = createMemoryWriter();
    const queryChat = vi.fn().mockResolvedValue({
      dispatchId: "dispatch_1",
      provider: "openai_compatible",
      model: "gpt-test",
      query: "hello",
      status: "completed",
      reply: "world",
      usage: { inputTokens: 1, outputTokens: 1, totalTokens: 2 }
    });

    const code = await runCLI(
      {
        daemonURL: "http://127.0.0.1:19192",
        accessToken: "token",
        query: "hello",
        stream: false
      },
      {
        io: { stdin: process.stdin, stdout, stderr },
        createClient: () =>
          ({
            queryChat,
            streamChatQuery: vi.fn()
          }) as any
      }
    );

    expect(code).toBe(0);
    expect(queryChat).toHaveBeenCalled();
    expect(stdout.contents).toContain("Assistant: world");
    expect(stderr.contents).toBe("");
  });

  it("runs a stream query and prints deltas", async () => {
    const stdout = createMemoryWriter();
    const stderr = createMemoryWriter();
    const streamChatQuery = vi.fn().mockImplementation(async (_payload, handlers) => {
      handlers.onDelta?.({ dispatchId: "dispatch_1", delta: "hel", reply: "hel" });
      handlers.onDelta?.({ dispatchId: "dispatch_1", delta: "lo", reply: "hello" });
      return {
        dispatchId: "dispatch_1",
        provider: "openai_compatible",
        model: "gpt-test",
        query: "hello",
        status: "completed",
        reply: "hello",
        usage: { inputTokens: 1, outputTokens: 1, totalTokens: 2 }
      };
    });

    const code = await runCLI(
      {
        daemonURL: "http://127.0.0.1:19192",
        query: "hello",
        stream: true
      },
      {
        io: { stdin: process.stdin, stdout, stderr },
        createClient: () =>
          ({
            queryChat: vi.fn(),
            streamChatQuery
          }) as any
      }
    );

    expect(code).toBe(0);
    expect(stdout.contents).toContain("Assistant: hello");
    expect(stderr.contents).toBe("");
  });

  it("prints errors and returns failure exit code", async () => {
    const stdout = createMemoryWriter();
    const stderr = createMemoryWriter();

    const code = await runCLI(
      {
        daemonURL: "http://127.0.0.1:19192",
        query: "hello",
        stream: false
      },
      {
        io: { stdin: process.stdin, stdout, stderr },
        createClient: () =>
          ({
            queryChat: vi.fn().mockRejectedValue(new Error("bad key")),
            streamChatQuery: vi.fn()
          }) as any
      }
    );

    expect(code).toBe(1);
    expect(stderr.contents).toContain("Error: bad key");
  });
});

function createMemoryWriter() {
  let contents = "";
  return {
    get contents() {
      return contents;
    },
    write(chunk: string | Uint8Array) {
      contents += typeof chunk === "string" ? chunk : Buffer.from(chunk).toString("utf8");
      return true;
    }
  } as NodeJS.WritableStream & { contents: string };
}
