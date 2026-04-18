import { useState } from "react";

import { createDopeClient } from "@dope/client";

type ChatStatus = "idle" | "loading" | "streaming" | "completed" | "error";

const DEFAULT_DAEMON_URL = "http://127.0.0.1:19191";

export function App() {
  const [daemonURL, setDaemonURL] = useState(DEFAULT_DAEMON_URL);
  const [accessToken, setAccessToken] = useState("");
  const [provider, setProvider] = useState("");
  const [model, setModel] = useState("");
  const [query, setQuery] = useState("");
  const [stream, setStream] = useState(true);
  const [status, setStatus] = useState<ChatStatus>("idle");
  const [reply, setReply] = useState("");
  const [error, setError] = useState("");
  const [usage, setUsage] = useState("");

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const trimmedQuery = query.trim();
    if (!trimmedQuery) {
      setStatus("error");
      setError("Query is required.");
      return;
    }

    setReply("");
    setUsage("");
    setError("");
    setStatus(stream ? "streaming" : "loading");

    const client = createDopeClient({
      baseURL: daemonURL,
      accessToken: accessToken.trim() || undefined
    });

    try {
      if (stream) {
        const terminal = await client.streamChatQuery(
          {
            provider: provider.trim() || undefined,
            model: model.trim() || undefined,
            query: trimmedQuery
          },
          {
            onDelta(payload) {
              setReply(payload.reply);
            }
          }
        );
        setReply(terminal.reply);
        setUsage(formatUsage(terminal.usage));
      } else {
        const response = await client.queryChat({
          provider: provider.trim() || undefined,
          model: model.trim() || undefined,
          query: trimmedQuery
        });
        setReply(response.reply);
        setUsage(formatUsage(response.usage));
      }
      setStatus("completed");
    } catch (caught) {
      const message = caught instanceof Error ? caught.message : String(caught);
      setStatus("error");
      setError(message);
    }
  }

  return (
    <main className="app-shell">
      <section className="intro-panel">
        <p className="eyebrow">DopeAgent</p>
        <h1>Single-Turn Chat Console</h1>
        <p className="lead">
          This surface talks to the daemon chat contract only. It does not carry
          daemon-side conversation history, memory, or context assembly.
        </p>
      </section>

      <section className="console-grid">
        <form className="query-panel" onSubmit={handleSubmit}>
          <div className="settings-grid">
            <label>
              <span>Daemon URL</span>
              <input value={daemonURL} onChange={(event) => setDaemonURL(event.target.value)} placeholder={DEFAULT_DAEMON_URL} />
            </label>

            <label>
              <span>Access Token</span>
              <input value={accessToken} onChange={(event) => setAccessToken(event.target.value)} placeholder="Bearer token" type="password" />
            </label>

            <label>
              <span>Provider Override</span>
              <input value={provider} onChange={(event) => setProvider(event.target.value)} placeholder="openai_compatible" />
            </label>

            <label>
              <span>Model Override</span>
              <input value={model} onChange={(event) => setModel(event.target.value)} placeholder="gpt-4.1-mini" />
            </label>
          </div>

          <label className="query-field">
            <span>Query</span>
            <textarea value={query} onChange={(event) => setQuery(event.target.value)} rows={7} placeholder="Ask the daemon for one single-turn reply." />
          </label>

          <div className="action-row">
            <label className="toggle">
              <input checked={stream} onChange={(event) => setStream(event.target.checked)} type="checkbox" />
              <span>Use stream API</span>
            </label>

            <div className="button-row">
              <button disabled={status === "loading" || status === "streaming"} type="button" onClick={() => {
                setQuery("");
                setReply("");
                setUsage("");
                setError("");
                setStatus("idle");
              }}>
                Clear
              </button>
              <button className="primary" disabled={status === "loading" || status === "streaming"} type="submit">
                {status === "loading" || status === "streaming" ? "Querying..." : "Send Query"}
              </button>
            </div>
          </div>
        </form>

        <section className="reply-panel">
          <div className="status-row">
            <span className={`status-pill status-${status}`}>{status}</span>
            {usage ? <span className="usage-chip">{usage}</span> : null}
          </div>

          {error ? (
            <div className="error-box" role="alert">
              {error}
            </div>
          ) : null}

          <div className="reply-box" data-empty={reply === ""}>
            {reply || "Reply output will appear here."}
          </div>
        </section>
      </section>
    </main>
  );
}

function formatUsage(usage: { inputTokens: number; outputTokens: number; totalTokens: number }) {
  return `usage ${usage.inputTokens}/${usage.outputTokens}/${usage.totalTokens}`;
}
