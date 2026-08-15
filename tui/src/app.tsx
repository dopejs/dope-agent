import { render, Box, Text, useApp, useInput, useStdout } from "ink";
import { useRef, useState } from "react";
import { createDopeClient, type ChatQueryInput } from "@dope/client";
import { Markdown } from "./markdown.js";
import { COMMANDS, type ChatContext, type CommandDeps, type Role } from "./commands.js";

export type AppOptions = {
  daemonURL: string;
  accessToken?: string;
  provider?: string;
  model?: string;
};

type Message = { id: number; role: Role; content: string; done: boolean };

function App({ daemonURL, accessToken, provider, model }: AppOptions) {
  const { exit } = useApp();
  const { stdout } = useStdout();
  const client = useRef(createDopeClient({ baseURL: daemonURL, accessToken }));
  const idRef = useRef(0);
  const cancelledRef = useRef(false);

  const [context, setContext] = useState<ChatContext>({ provider, model });
  const [messages, setMessages] = useState<Message[]>([
    { id: 0, role: "system", content: "DopeAgent \u00b7 " + daemonURL + "\nType /help for commands.", done: true },
  ]);
  const [input, setInput] = useState("");
  const [history, setHistory] = useState<string[]>([]);
  const [historyIndex, setHistoryIndex] = useState(-1);
  const [busy, setBusy] = useState(false);
  const [scrollOffset, setScrollOffset] = useState(0);

  const rows = stdout.rows ?? 24;

  function push(role: Role, content: string, done = true): number {
    const id = idRef.current++;
    setMessages((m) => [...m, { id, role, content, done }]);
    setScrollOffset(0);
    return id;
  }

  function patch(id: number, content: string, done = false) {
    setMessages((m) => m.map((msg) => (msg.id === id ? { ...msg, content, done } : msg)));
  }

  async function slash(text: string) {
    const [cmd, ...rest] = text.split(/\s+/);
    const args = rest.join(" ");
    const command = COMMANDS[cmd];
    if (!command) {
      push("error", "Unknown command " + cmd + ". Type /help.");
      return;
    }
    const deps: CommandDeps = {
      client: client.current,
      push,
      getContext: () => context,
      setContext: (patch) => setContext((c) => ({ ...c, ...patch })),
      exit,
    };
    await command.run(args, deps);
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
      const payload: ChatQueryInput = { query: text, provider: context.provider, model: context.model, threadId: context.threadId };
      const result = await client.current.streamChatQuery(payload, {
        onDelta(chunk) {
          if (cancelledRef.current) return;
          acc += chunk.delta;
          patch(asstId, acc);
        },
      });
      patch(asstId, result.reply || acc, true);
    } catch (e) {
      const message = e instanceof Error ? e.message : String(e);
      patch(asstId, acc ? acc + "\n\n[error] " + message : "[error] " + message, true);
    } finally {
      setBusy(false);
    }
  }

  useInput((inputChar, key) => {
    if (key.ctrl && inputChar === "c") { exit(); return; }
    if (key.escape) {
      if (busy) { cancelledRef.current = true; setBusy(false); }
      else { setInput(""); }
      return;
    }
    if (key.pageUp) { setScrollOffset((o) => o + 10); return; }
    if (key.pageDown) { setScrollOffset((o) => Math.max(0, o - 10)); return; }
    if (key.ctrl && inputChar === "l") { setMessages((m) => [m[0]]); setScrollOffset(0); return; }
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
    if (key.return) {
      if (input.endsWith("\\")) {
        setInput((s) => s.slice(0, -1) + "\n");
        return;
      }
      void submit(input);
      return;
    }
    if (key.backspace || key.delete) { setInput((s) => s.slice(0, -1)); return; }
    if (inputChar) { setInput((s) => s + inputChar); }
  });

  const headerRows = 4;
  const visibleCount = Math.max(5, rows - headerRows);
  const visible = scrollOffset === 0 ? messages.slice(-visibleCount) : messages.slice(-(visibleCount + scrollOffset), -scrollOffset);
  const status = "provider=" + (context.provider ?? "default") + " model=" + (context.model ?? "default") + " thread=" + (context.threadId ?? "new") + (scrollOffset > 0 ? "  [scrolled, PageUp/PageDown to navigate]" : "");

  return (
    <Box flexDirection="column" height={rows}>
      <Text dimColor>{status}</Text>
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