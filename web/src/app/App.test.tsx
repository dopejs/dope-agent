import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { App } from "./App";

const mockQueryChat = vi.fn();
const mockStreamChatQuery = vi.fn();

vi.mock("@dope/client", () => ({
  createDopeClient: () => ({
    queryChat: mockQueryChat,
    streamChatQuery: mockStreamChatQuery
  })
}));

describe("App", () => {
  beforeEach(() => {
    mockQueryChat.mockReset();
    mockStreamChatQuery.mockReset();
  });
  afterEach(() => {
    cleanup();
  });

  it("renders a non-stream reply", async () => {
    mockQueryChat.mockResolvedValue({
      dispatchId: "dispatch_1",
      provider: "openai_compatible",
      model: "gpt-test",
      query: "hello",
      status: "completed",
      reply: "hello world",
      usage: { inputTokens: 1, outputTokens: 1, totalTokens: 2 }
    });

    render(<App />);
    const user = userEvent.setup();

    await user.click(screen.getByRole("checkbox", { name: /use stream api/i }));
    await user.type(screen.getByRole("textbox", { name: /query/i }), "hello");
    await user.click(screen.getByRole("button", { name: /send query/i }));

    await waitFor(() => {
      expect(screen.queryByText("hello world")).not.toBeNull();
    });
    expect(mockQueryChat).toHaveBeenCalledWith({
      provider: undefined,
      model: undefined,
      query: "hello"
    });
  });

  it("renders stream deltas", async () => {
    mockStreamChatQuery.mockImplementation(async (_payload, handlers) => {
      handlers.onDelta({ dispatchId: "dispatch_1", delta: "hel", reply: "hel" });
      handlers.onDelta({ dispatchId: "dispatch_1", delta: "lo", reply: "hello" });
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

    render(<App />);
    const user = userEvent.setup();

    await user.type(screen.getByRole("textbox", { name: /query/i }), "hello");
    await user.click(screen.getByRole("button", { name: /send query/i }));

    await waitFor(() => {
      const replyBox = document.querySelector(".reply-box");
      expect(replyBox).not.toBeNull();
      expect(replyBox?.textContent).toBe("hello");
    });
    expect(mockStreamChatQuery).toHaveBeenCalled();
  });

  it("renders client errors visibly", async () => {
    mockStreamChatQuery.mockRejectedValue(new Error("bad key"));

    render(<App />);
    const user = userEvent.setup();

    await user.type(screen.getByRole("textbox", { name: /query/i }), "hello");
    await user.click(screen.getByRole("button", { name: /send query/i }));

    await waitFor(() => {
      const alert = screen.getByRole("alert");
      expect(alert.textContent).toContain("bad key");
    });
  });
});
