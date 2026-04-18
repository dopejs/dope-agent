export type ChatQueryInput = {
  provider?: string;
  model?: string;
  skills?: string[];
  query: string;
  timeoutMs?: number;
  maxRetries?: number;
};

export type ChatQueryResponse = {
  dispatchId: string;
  provider: string;
  model: string;
  skills: string[];
  query: string;
  status: "queued" | "running" | "completed" | "partial_failed" | "failed" | "cancelled";
  partial: boolean;
  reply: string;
  finishReason?: string;
  usage: {
    inputTokens: number;
    outputTokens: number;
    totalTokens: number;
  };
  errorCode?: string;
  error?: string;
};

export type ChatQueryStreamStarted = {
  dispatchId: string;
  provider: string;
  model: string;
  skills: string[];
  query: string;
};

export type ChatQueryStreamDelta = {
  dispatchId: string;
  delta: string;
  reply: string;
};

export type StreamHandlers = {
  onStarted?: (payload: ChatQueryStreamStarted) => void;
  onDelta?: (payload: ChatQueryStreamDelta) => void;
  onCompleted?: (payload: ChatQueryResponse) => void;
  onFailed?: (payload: ChatQueryResponse) => void;
  onCancelled?: (payload: ChatQueryResponse) => void;
};

export type DopeClientOptions = {
  baseURL: string;
  accessToken?: string;
  fetchImpl?: typeof fetch;
};

export class DopeClientError extends Error {
  readonly status: number;
  readonly code?: string;

  constructor(message: string, options: { status: number; code?: string }) {
    super(message);
    this.name = "DopeClientError";
    this.status = options.status;
    this.code = options.code;
  }
}

export class DopeClient {
  private readonly baseURL: string;
  private readonly accessToken?: string;
  private readonly fetchImpl: typeof fetch;

  constructor(options: DopeClientOptions) {
    this.baseURL = trimBaseURL(options.baseURL);
    this.accessToken = options.accessToken?.trim() || undefined;
    this.fetchImpl = options.fetchImpl ?? fetch;
  }

  async queryChat(input: ChatQueryInput): Promise<ChatQueryResponse> {
    const response = await this.fetchImpl(`${this.baseURL}/v1/chat/query`, {
      method: "POST",
      headers: this.buildHeaders(),
      body: JSON.stringify(normalizeInput(input)),
    });

    if (!response.ok) {
      throw await toClientError(response);
    }
    return (await response.json()) as ChatQueryResponse;
  }

  async streamChatQuery(input: ChatQueryInput, handlers: StreamHandlers = {}): Promise<ChatQueryResponse> {
    const response = await this.fetchImpl(`${this.baseURL}/v1/chat/query/stream`, {
      method: "POST",
      headers: this.buildHeaders(),
      body: JSON.stringify(normalizeInput(input)),
    });

    if (!response.ok) {
      throw await toClientError(response);
    }
    if (!response.body) {
      throw new DopeClientError("chat stream response body is missing", { status: 500, code: "stream_body_missing" });
    }

    let terminal: ChatQueryResponse | null = null;
    await readSSE(response.body, (event) => {
      switch (event.event) {
        case "chat.query.started":
          handlers.onStarted?.(JSON.parse(event.data) as ChatQueryStreamStarted);
          break;
        case "chat.query.delta":
          handlers.onDelta?.(JSON.parse(event.data) as ChatQueryStreamDelta);
          break;
        case "chat.query.completed":
          terminal = JSON.parse(event.data) as ChatQueryResponse;
          handlers.onCompleted?.(terminal);
          break;
        case "chat.query.failed":
          terminal = JSON.parse(event.data) as ChatQueryResponse;
          handlers.onFailed?.(terminal);
          break;
        case "chat.query.cancelled":
          terminal = JSON.parse(event.data) as ChatQueryResponse;
          handlers.onCancelled?.(terminal);
          break;
        default:
          break;
      }
    });

    if (!terminal) {
      throw new DopeClientError("chat stream ended without a terminal event", {
        status: 502,
        code: "stream_terminal_event_missing"
      });
    }

    return terminal;
  }

  private buildHeaders(): HeadersInit {
    const headers: Record<string, string> = {
      "Content-Type": "application/json"
    };
    if (this.accessToken) {
      headers.Authorization = `Bearer ${this.accessToken}`;
    }
    return headers;
  }
}

export function createDopeClient(options: DopeClientOptions): DopeClient {
  return new DopeClient(options);
}

function trimBaseURL(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    throw new Error("baseURL is required");
  }
  return trimmed.replace(/\/+$/, "");
}

function normalizeInput(input: ChatQueryInput): ChatQueryInput {
  return {
    provider: input.provider?.trim() || undefined,
    model: input.model?.trim() || undefined,
    skills: input.skills?.map((item) => item.trim()).filter(Boolean),
    query: input.query.trim(),
    timeoutMs: input.timeoutMs,
    maxRetries: input.maxRetries
  };
}

async function toClientError(response: Response): Promise<DopeClientError> {
  let message = `request failed with status ${response.status}`;
  let code: string | undefined;

  try {
    const payload = (await response.json()) as { error?: string; errorCode?: string };
    if (payload.error) {
      message = payload.error;
    }
    if (payload.errorCode) {
      code = payload.errorCode;
    }
  } catch {
    // ignore non-json failure bodies
  }

  return new DopeClientError(message, { status: response.status, code });
}

type SSEEvent = {
  event: string;
  data: string;
};

async function readSSE(stream: ReadableStream<Uint8Array>, onEvent: (event: SSEEvent) => void | Promise<void>): Promise<void> {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }
    buffer += decoder.decode(value, { stream: true });
    const parts = buffer.split("\n\n");
    buffer = parts.pop() ?? "";
    for (const part of parts) {
      const parsed = parseSSEEvent(part);
      if (parsed) {
        await onEvent(parsed);
      }
    }
  }

  buffer += decoder.decode();
  if (buffer.trim()) {
    const parsed = parseSSEEvent(buffer);
    if (parsed) {
      await onEvent(parsed);
    }
  }
}

function parseSSEEvent(chunk: string): SSEEvent | null {
  const lines = chunk
    .split("\n")
    .map((line) => line.trimEnd())
    .filter((line) => line !== "");

  let event = "";
  const data: string[] = [];

  for (const line of lines) {
    if (line.startsWith("event:")) {
      event = line.slice("event:".length).trim();
      continue;
    }
    if (line.startsWith("data:")) {
      data.push(line.slice("data:".length).trim());
    }
  }

  if (!event || data.length === 0) {
    return null;
  }

  return {
    event,
    data: data.join("\n")
  };
}
