import { render, Box, Text, useApp, useInput, useStdout } from "ink";
import { useRef, useState } from "react";
import { createDopeClient, type ChatQueryInput } from "@dope/client";
import { Markdown } from "./markdown.js";

export type AppOptions = {
  daemonURL: string;
  accessToken?: string;
  provider?: string;
  model?: string;
};

type Role = "user" | "assistant" | "system" | "error";

type Message = {
  id: number;
  role: Role;
  content: string;
  done: boolean;
};

const HELP = [
  "DopeAgent TUI",
  "",
  "/help           Show this help",
  "/threads        List your threads",
  "/reset <id>     Reset a thread",
  "/exit           Quit",
  "",
  "Type a message and press Enter to chat (streaming).",
  "Up/Down browse history, Esc cancels a streaming reply, Ctrl+C quits.",
].join("\n");

function App({ daemonURL, accessToken, provider, model }: AppOptions) {
  const { exit } = useApp();
  const { stdout } = useStdout();
  const client = useRef(createDopeClient({ baseURL: daemonURL, accessToken }));
  const idRef = useRef(0);
  const cancelledRef = useRef(false);

  const [messages, setMessages] = useState<Message[]>([
    { id: 0, role: "system", content: "DopeAgent \u00b7 " + daemonURL + "\nType /help for commands.", done: true },
  ]);
  const [input, setInput] = useState("");
  const [history, setHistory] = useState<string[]>([]);
  const [historyIndex, setHistoryIndex] = useState(-1);
  const [busy, setBusy] = useState(false);

  const rows = stdout.rows ?? 24;

  function push(role: Role, content: string, done = true): number {
    const id = idRef.current++;
    setMessages((m) => [...m, { id, role, content, done }]);
    return id;
  }

  function patch(id: number, content: string, done = false) {
    setMessages((m) => m.map((msg) => (msg.id === id ? { ...msg, content, done } : msg)));
  }

  async function submit(raw: string) {
    const text = raw.trim();
    if (!text || busy) return;
    setInput("");
    setHistory((h) => [...h, text]);
    setHistoryIndex(-1);

    if (text.startsWith("/")) {
      await slash(text);
      return;
    }

    push("user", text);
    const asstId = push("assistant", "", false);
    setBusy(true);
    cancelledRef.current = false;

    let acc = "";
    try {
      const payload: ChatQueryInput = { query: text, provider, model };
      const result = await client.current.streamChatQuery(payload, {
        onDelta(chunk) {
          if (cancelledRef.current) return;
          acc += chunk.delta;
          patch(asstId, acc);
        },
      });
      patch(asstId, result.reply || acc, true);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      patch(asstId, acc ? acc + "\n\n[error] " + message : "[error] " + message, true);
    } finally {
      setBusy(false);
    }
  }

  async function slash(text: string) {
    const [cmd, ...rest] = text.split(/\s+/);
    const arg = rest.join(" ");
    switch (cmd) {
      case "/help":
        push("system", HELP);
        return;
      case "/exit":
      case "/quit":
        exit();
        return;
      case "/threads": {
        try {
          const list = await client.current.listThreads();
          const body = list.items.length === 0
            ? "No threads."
            : list.items.map((t) => t.threadId + "  " + t.lifecycleState + "  " + t.sourceKind + "  " + (t.sourceSummary ?? "")).join("\n");
          push("system", "Threads (" + list.items.length + "):\n" + body);
        } catch (err) {
          push("error", "[error] " + (err instanceof Error ? err.message : String(err)));
        }
        return;
      }
      case "/reset": {
        if (!arg) { push("error", "/reset <thread-id>"); return; }
        try {
          const res = await client.current.resetThread(arg);
          push("system", "Thread " + res.threadId + " reset \u2192 " + res.lifecycleState);
        } catch (err) {
          push("error", "[error] " + (err instanceof Error ? err.message : String(err)));
        }
        return;
      }
      default:
        push("error", "Unknown command " + cmd + ". Type /help.");
    }
  }

  useInput((inputChar, key) => {
    if (key.ctrl && inputChar === "c") { exit(); return; }
    if (key.escape) {
      if (busy) { cancelledRef.current = true; setBusy(false); }
      else { setInput(""); }
      return;
    }
    if (key.upArrow) {
      if (history.length === 0) return;
      const next = historyIndex < 0 ? history.length - 1 : Math.max(0, historyIndex - 1);
      setHistoryIndex(next);
      setInput(history[next]);
      return;
    }
    if (key.downArrow) {
      if (historyIndex < 0) return;
      const next = historyIndex + 1;
      if (next >= history.length) { setHistoryIndex(-1); setInput(""); }
      else { setHistoryIndex(next); setInput(history[next]); }
      return;
    }
    if (key.return) { void submit(input); return; }
    if (key.backspace || key.delete) { setInput((s) => s.slice(0, -1)); return; }
    if (inputChar) { setInput((s) => s + inputChar); }
  });

  const headerRows = 3;
  const visibleCount = Math.max(5, rows - headerRows);
  const visible = messages.slice(-visibleCount);

  return (
    <Box flexDirection="column" height={rows}>
      <Box flexDirection="column" flexGrow={1}>
        {visible.map((m) => (
          <Box key={m.id} flexDirection="column">
            {m.role === "user" ? (
              <Text bold color="green">{"> "}{m.content}</Text>
            ) : m.role === "assistant" ? (
              m.content ? <Markdown text={m.content} /> : <Text dimColor>{"\u2026"}</Text>
            ) : m.role === "error" ? (
              <Text color="red">{m.content}</Text>
            ) : (
              <Text dimColor>{m.content}</Text>
            )}
            {m.role === "assistant" && !m.done ? <Text dimColor>{"\u258c"}</Text> : null}
          </Box>
        ))}
      </Box>
      <Box borderStyle="single" borderColor="blue">
        <Text>
          <Text color="blue" bold>{busy ? "\u2026 " : "\u276f "}</Text>
          <Text>{input}</Text>
        </Text>
      </Box>
    </Box>
  );
}

export function runTUI(options: AppOptions) {
  render(<App {...options} />);
}