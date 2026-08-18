# Usage

## Chat over HTTP

One-shot query:

```bash
curl -s http://127.0.0.1:19192/v1/chat/query \
  -H 'content-type: application/json' \
  -d '{
    "query": "summarize my day",
    "provider": "claude_code_cli",
    "threadId": "thr_personal"
  }'
```

Streaming (SSE):

```bash
curl -N http://127.0.0.1:19192/v1/chat/query/stream \
  -H 'content-type: application/json' \
  -d '{"query": "hello", "provider": "echo"}'
```

Every turn runs the full pipeline: skills + persona compile into the
prompt, thread continuity injects prior turns, the **context plugin**
injects memory bootstrap and query-time recall, the **session-strategy
plugin** shapes the window under budget — and whatever the model sees is
exactly what is persisted on the dispatch record
(*model-visible = logged*).

## TypeScript SDK

```ts
import { createDopeClient } from "@dope/client";

const client = createDopeClient({ baseURL: "http://127.0.0.1:19192" });

const result = await client.queryChat({ query: "hello", provider: "echo" });

// Streaming
for await (const chunk of client.streamChatQuery({ query: "hi" })) {
  process.stdout.write(chunk.delta);
}

// Memory, plugins, retrieval…
const plugins = await client.listPlugins();
const recall  = await client.queryRetrieval({ query: "package manager" });
const assets  = await client.listMemoryAssets({ layer: "l1", status: "ready" });
```

## Terminal UI

`dope tui` launches the full-screen terminal client: conversations,
thread continuity, and a live daemon event stream (`/events`).

## Web operator shell

`dope web` serves the installed web shell locally and opens your
browser. It is the operator console: memory
assets + review queue, **Plugins** (assembly report, enable/disable,
hooks), channels, routines, providers, quota, diagnostics, evaluation,
and support surfaces.

## Threads and continuity

Pass a `threadId` to keep continuity: prior turns are stored as governed
continuity records and re-rendered into each dispatch (bounded, redaction
safe). IM channels get **one context per thread** automatically.

## Events

Everything observable is an event (`llm.*`, `memory.*`, `context.*`,
`chat.*`, `improvement.*`, `system.*`), persisted in the store ledger and
published on the live bus. The TUI `/events` view and the sessions API
expose them.
