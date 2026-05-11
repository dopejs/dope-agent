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

  it("prints Slack setup projection output", async () => {
    const stdout = createMemoryWriter();
    const stderr = createMemoryWriter();
    const getSlackSetup = vi.fn().mockResolvedValue({
      connectorId: "slack-main",
      terminalState: "ready",
      oauthState: "grant_valid",
      routePolicyState: "valid",
      deliveryEligible: true
    });

    const options = parseArgs(["--slack-setup", "slack-main"], {});
    const code = await runCLI(options, {
      io: { stdin: process.stdin, stdout, stderr },
      createClient: () =>
        ({
          getSlackSetup,
          queryChat: vi.fn(),
          streamChatQuery: vi.fn()
        }) as any
    });

    expect(code).toBe(0);
    expect(getSlackSetup).toHaveBeenCalledWith("slack-main");
    expect(stdout.contents).toContain("Slack Setup: slack-main");
    expect(stdout.contents).toContain("Delivery Eligible: yes");
    expect(stderr.contents).toBe("");
  });

  it("prints Matrix setup projection output", async () => {
    const stdout = createMemoryWriter();
    const stderr = createMemoryWriter();
    const getMatrixSetup = vi.fn().mockResolvedValue({
      connectorId: "matrix-main",
      terminalState: "action-required",
      homeserverState: "reachable",
      routePolicyState: "valid",
      deliveryEligible: false
    });

    const options = parseArgs(["--matrix-setup", "matrix-main"], {});
    const code = await runCLI(options, {
      io: { stdin: process.stdin, stdout, stderr },
      createClient: () =>
        ({
          getMatrixSetup,
          queryChat: vi.fn(),
          streamChatQuery: vi.fn()
        }) as any
    });

    expect(code).toBe(0);
    expect(getMatrixSetup).toHaveBeenCalledWith("matrix-main");
    expect(stdout.contents).toContain("Matrix Setup: matrix-main");
    expect(stdout.contents).toContain("Delivery Eligible: no");
    expect(stderr.contents).toBe("");
  });

  it("prints thread list output", async () => {
    const stdout = createMemoryWriter();
    const stderr = createMemoryWriter();
    const listThreads = vi.fn().mockResolvedValue({
      tenantId: "ten_threads",
      page: { limit: 20, order: "active_recent_archived_id" },
      items: [{ threadId: "thr_1", lifecycleState: "active", sourceKind: "channel", sourceSummary: "Slack Main / #support", lastActivityAt: "2026-05-11T10:00:00Z", availableActions: ["reset"], redactionStatus: "redacted", updatedAt: "2026-05-11T10:00:00Z" }]
    });
    const options = parseArgs(["--threads"], {});
    const code = await runCLI(options, {
      io: { stdin: process.stdin, stdout, stderr },
      createClient: () =>
        ({
          listThreads,
          queryChat: vi.fn(),
          streamChatQuery: vi.fn()
        }) as any
    });

    expect(code).toBe(0);
    expect(stdout.contents).toContain("Threads: ten_threads");
    expect(stdout.contents).toContain("thr_1 active channel Slack Main / #support");
  });

  it("prints thread detail output and reauthorizes each command", async () => {
    const stdout = createMemoryWriter();
    const stderr = createMemoryWriter();
    const getThread = vi.fn().mockResolvedValue({
      thread: {
        threadId: "thr_1",
        tenantId: "ten_threads",
        lifecycleState: "active",
        sourceKind: "channel",
        sourceSummary: "Slack Main / #support",
        currentSessionId: "sess_1",
        lastActivityAt: "2026-05-11T10:00:00Z",
        availableActions: ["reset"],
        redactionStatus: "redacted",
        retentionExpiresAt: "2026-08-09T10:00:00Z",
        updatedAt: "2026-05-11T10:00:00Z"
      },
      sessionSegments: [{}],
      sourceLinkages: [],
      runtimeProjections: [],
      lifecycleActions: []
    });
    const options = parseArgs(["--thread", "thr_1"], {});
    const code = await runCLI(options, {
      io: { stdin: process.stdin, stdout, stderr },
      createClient: () =>
        ({
          getThread,
          queryChat: vi.fn(),
          streamChatQuery: vi.fn()
        }) as any
    });

    expect(code).toBe(0);
    expect(getThread).toHaveBeenCalledWith("thr_1");
    expect(stdout.contents).toContain("Thread: thr_1");
    expect(stdout.contents).toContain("Current Session: sess_1");
    expect(stdout.contents).toContain("Evidence: lifecycle metadata, not assistant memory");
    expect(stdout.contents).toContain("Retention: 2026-08-09T10:00:00Z");
    expect(stderr.contents).toBe("");
  });

  it("prints thread trace source and runtime evidence", async () => {
    const stdout = createMemoryWriter();
    const stderr = createMemoryWriter();
    const getThread = vi.fn().mockResolvedValue({
      thread: {
        threadId: "thr_1",
        tenantId: "ten_threads",
        lifecycleState: "active",
        sourceKind: "channel",
        sourceSummary: "Slack Main / #support",
        currentSessionId: "sess_1",
        lastActivityAt: "2026-05-11T10:00:00Z",
        availableActions: ["reset"],
        redactionStatus: "redacted",
        retentionExpiresAt: "2026-08-09T10:00:00Z",
        updatedAt: "2026-05-11T10:00:00Z"
      },
      sessionSegments: [],
      sourceLinkages: [{
        sourceLinkageId: "src_1",
        sourceKind: "channel",
        connectorKind: "slack",
        sourceConversationId: "channel_redacted",
        routingOutcome: "accepted",
        current: true,
        retentionExpiresAt: "2026-08-09T10:00:00Z",
        redactionStatus: "redacted"
      }],
      runtimeProjections: [{
        runtimeProjectionId: "rtp_1",
        resourceKind: "foreground_reply",
        resourceId: "delivery_1",
        status: "replied",
        reasonCode: "accepted",
        occurredAt: "2026-05-11T10:00:00Z",
        safeSummary: "Foreground reply replied",
        retentionExpiresAt: "2026-08-09T10:00:00Z",
        redactionStatus: "redacted"
      }],
      lifecycleActions: []
    });
    const options = parseArgs(["--thread-trace", "thr_1"], {});
    const code = await runCLI(options, {
      io: { stdin: process.stdin, stdout, stderr },
      createClient: () =>
        ({
          getThread,
          queryChat: vi.fn(),
          streamChatQuery: vi.fn()
        }) as any
    });

    expect(code).toBe(0);
    expect(getThread).toHaveBeenCalledWith("thr_1");
    expect(stdout.contents).toContain("Thread Trace: thr_1");
    expect(stdout.contents).toContain("Evidence: lifecycle metadata, not assistant memory");
    expect(stdout.contents).toContain("Source Trace:");
    expect(stdout.contents).toContain("- accepted slack channel_redacted");
    expect(stdout.contents).toContain("retention=2026-08-09T10:00:00Z");
    expect(stdout.contents).toContain("- foreground_reply replied Foreground reply replied");
    expect(stderr.contents).toBe("");
  });

  it("runs thread lifecycle mutation commands", async () => {
    const stdout = createMemoryWriter();
    const stderr = createMemoryWriter();
    const archiveThread = vi.fn().mockResolvedValue({
      threadId: "thr_1",
      lifecycleState: "archived",
      auditEventId: "audit_1",
      changedAt: "2026-05-11T10:00:00Z",
      action: "archive",
      availableActions: ["reopen"]
    });
    const options = parseArgs(["--thread-archive", "thr_1"], {});
    const code = await runCLI(options, {
      io: { stdin: process.stdin, stdout, stderr },
      createClient: () =>
        ({
          archiveThread,
          queryChat: vi.fn(),
          streamChatQuery: vi.fn()
        }) as any
    });

    expect(code).toBe(0);
    expect(archiveThread).toHaveBeenCalledWith("thr_1", { reasonCode: "tui_archive" });
    expect(stdout.contents).toContain("Thread thr_1 archive completed.");
    expect(stdout.contents).toContain("Audit: audit_1");
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
