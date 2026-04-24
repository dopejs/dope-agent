export type EnvironmentScope = "test" | "prod";

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

export type OperatorReadinessItem = {
  itemId: string;
  itemKind: string;
  resourceId?: string;
  displayName: string;
  status: "ready" | "blocked" | "degraded" | "missing_configuration" | "optional";
  healthState?: string;
  reason?: string;
  requiredOperatorAction?: string;
  requiredForSelectedAction: boolean;
  detailRoute?: string;
  environmentScope: EnvironmentScope;
  updatedAt: string;
};

export type OperatorFirstUsefulAction = {
  actionId: string;
  actionKind: string;
  displayName: string;
  recommended: boolean;
  available: boolean;
  blockingItemIds?: string[];
  summary?: string;
  invokeRoute: string;
  resultRoute?: string;
};

export type OperatorOnboardingResponse = {
  environmentScope: EnvironmentScope;
  status: "blocked" | "ready_for_action" | "completed";
  currentStepId?: string;
  completedStepIds?: string[];
  blockingItemIds: string[];
  optionalFollowUpItemIds: string[];
  recommendedActionId?: string;
  readinessItems: OperatorReadinessItem[];
  firstUsefulActions: OperatorFirstUsefulAction[];
  lastEvaluatedAt: string;
};

export type OperatorResourceRef = {
  kind: string;
  id: string;
  route?: string;
};

export type OperatorActivityRecord = {
  activityId: string;
  sourceKind: string;
  sourceId: string;
  title: string;
  status: string;
  summary: string;
  attentionLevel: "info" | "warning" | "critical";
  occurredAt: string;
  detailRoute?: string;
  relatedResourceRefs?: OperatorResourceRef[];
  environmentScope: EnvironmentScope;
};

export type OperatorActivityListResponse = {
  environmentScope: EnvironmentScope;
  items: OperatorActivityRecord[];
  generatedAt: string;
};

export type OperatorDiagnosticFinding = {
  findingId: string;
  sourceKind: string;
  sourceId: string;
  plane: "readiness" | "approval" | "execution" | "delivery";
  severity: "warning" | "critical";
  status: string;
  reason: string;
  recommendedAction?: string;
  detailRoute?: string;
  relatedResourceRefs?: OperatorResourceRef[];
  environmentScope: EnvironmentScope;
  capturedAt: string;
};

export type OperatorDiagnosticListResponse = {
  environmentScope: EnvironmentScope;
  items: OperatorDiagnosticFinding[];
  generatedAt: string;
};

export type OperatorActivityQuery = {
  sourceKind?: string;
  attentionOnly?: boolean;
  limit?: number;
};

export type OperatorDiagnosticsQuery = {
  sourceKind?: string;
  plane?: OperatorDiagnosticFinding["plane"];
  severity?: OperatorDiagnosticFinding["severity"];
};

export type ApprovalStatus = "pending" | "approved" | "rejected";

export type ApprovalResource = {
  approvalId: string;
  action: string;
  resourceKind?: string;
  resourceId?: string;
  reason: string;
  requestedBy?: string;
  status: ApprovalStatus;
  createdAt: string;
  updatedAt: string;
  resolvedAt?: string;
  resolution?: string;
  comment?: string;
  integrationBindings?: Array<Record<string, unknown>>;
  sandbox?: Record<string, unknown>;
};

export type ApprovalListResponse = {
  items: ApprovalResource[];
};

export type DecisionResource = {
  decisionId: string;
  action: string;
  resourceKind?: string;
  resourceId?: string;
  outcome: string;
  reason: string;
  approvalId?: string;
  createdAt: string;
  sandbox?: Record<string, unknown>;
};

export type ApprovalDecisionResponse = {
  approval: ApprovalResource;
  decision: DecisionResource;
};

export type ResolveApprovalInput = {
  resolution: "approved" | "rejected";
  comment?: string;
};

export type SessionRouteRequest = {
  kind?: string;
  channel?: string;
  accountId?: string;
  peerId?: string;
  threadId?: string;
};

export type CreateRunInput = {
  sessionId?: string;
  route?: SessionRouteRequest;
  entrypoint: string;
  goal?: string;
  input?: unknown;
};

export type RunResource = {
  runId: string;
  sessionId?: string;
  scheduleId?: string;
  scheduleAttemptId?: string;
  reminderId?: string;
  reminderOccurrenceId?: string;
  entrypoint: string;
  status: string;
  goal: string;
  activeWorkflowId?: string;
  workflowCount?: number;
  latestDeliveryId?: string;
  latestDeliveryStatus?: string;
  latestDeliveryTargetId?: string;
  createdAt: string;
  updatedAt: string;
};

export type DaemonEventScope = {
  sessionId?: string;
  runId?: string;
  workflowId?: string;
  workflowStepId?: string;
  scheduleId?: string;
  scheduleAttemptId?: string;
  stepId?: string;
  computerUseSessionId?: string;
  computerUseActionId?: string;
  connectorId?: string;
  capabilityId?: string;
};

export type DaemonEvent = {
  eventId: string;
  sequence: number;
  category: string;
  name: string;
  occurredAt: string;
  scope: DaemonEventScope;
  resource: {
    kind: string;
    id: string;
  };
  payload: Record<string, unknown>;
};

export type EventStreamQuery = {
  category?: string;
  runId?: string;
  sessionId?: string;
  scheduleId?: string;
  scheduleAttemptId?: string;
  resourceKind?: string;
  cursor?: number;
};

export type EventStreamHandlers = {
  onEvent?: (event: DaemonEvent) => void;
  onError?: (error: Error) => void;
};

export type EventStreamSubscription = {
  close: () => void;
  completed: Promise<void>;
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
    this.fetchImpl = options.fetchImpl ?? globalThis.fetch.bind(globalThis);
  }

  async queryChat(input: ChatQueryInput): Promise<ChatQueryResponse> {
    return this.requestJSON<ChatQueryResponse>("/v1/chat/query", {
      method: "POST",
      body: normalizeChatInput(input)
    });
  }

  async streamChatQuery(input: ChatQueryInput, handlers: StreamHandlers = {}): Promise<ChatQueryResponse> {
    const response = await this.fetchImpl(this.buildURL("/v1/chat/query/stream"), {
      method: "POST",
      headers: this.buildHeaders(),
      body: JSON.stringify(normalizeChatInput(input))
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

  async getOnboarding(): Promise<OperatorOnboardingResponse> {
    return this.requestJSON<OperatorOnboardingResponse>("/v1/operator/onboarding");
  }

  async getActivity(query: OperatorActivityQuery = {}): Promise<OperatorActivityListResponse> {
    return this.requestJSON<OperatorActivityListResponse>("/v1/operator/activity", {
      query: {
        sourceKind: query.sourceKind,
        attentionOnly: query.attentionOnly,
        limit: query.limit
      }
    });
  }

  async getDiagnostics(query: OperatorDiagnosticsQuery = {}): Promise<OperatorDiagnosticListResponse> {
    return this.requestJSON<OperatorDiagnosticListResponse>("/v1/operator/diagnostics", {
      query: {
        sourceKind: query.sourceKind,
        plane: query.plane,
        severity: query.severity
      }
    });
  }

  async listApprovals(status?: ApprovalStatus): Promise<ApprovalListResponse> {
    return this.requestJSON<ApprovalListResponse>("/v1/policy/approvals", {
      query: { status }
    });
  }

  async getApproval(approvalId: string): Promise<ApprovalResource> {
    return this.requestJSON<ApprovalResource>(`/v1/policy/approvals/${approvalId.trim()}`);
  }

  async resolveApproval(approvalId: string, input: ResolveApprovalInput): Promise<ApprovalDecisionResponse> {
    return this.requestJSON<ApprovalDecisionResponse>(`/v1/policy/approvals/${approvalId.trim()}/resolve`, {
      method: "POST",
      body: {
        resolution: input.resolution,
        comment: input.comment?.trim() || ""
      }
    });
  }

  async createRun(input: CreateRunInput): Promise<RunResource> {
    return this.requestJSON<RunResource>("/v1/runs", {
      method: "POST",
      body: {
        sessionId: input.sessionId?.trim() || undefined,
        route: input.route,
        entrypoint: input.entrypoint.trim(),
        goal: input.goal?.trim() || undefined,
        input: input.input
      }
    });
  }

  async fetchRoute<T>(route: string): Promise<T> {
    return this.requestJSON<T>(normalizeRoute(route));
  }

  streamEvents(query: EventStreamQuery = {}, handlers: EventStreamHandlers = {}): EventStreamSubscription {
    const controller = new AbortController();
    const completed = (async () => {
      const response = await this.fetchImpl(this.buildURL("/v1/events/stream", query), {
        method: "GET",
        headers: this.buildHeaders(),
        signal: controller.signal
      });

      if (!response.ok) {
        throw await toClientError(response);
      }
      if (!response.body) {
        throw new DopeClientError("event stream response body is missing", { status: 500, code: "stream_body_missing" });
      }

      await readSSE(response.body, (event) => {
        handlers.onEvent?.(JSON.parse(event.data) as DaemonEvent);
      });
    })().catch((error: unknown) => {
      if (controller.signal.aborted) {
        return;
      }
      handlers.onError?.(error instanceof Error ? error : new Error(String(error)));
    });

    return {
      close() {
        controller.abort();
      },
      completed
    };
  }

  private async requestJSON<T>(
    route: string,
    options: {
      method?: string;
      query?: Record<string, QueryValue>;
      body?: unknown;
    } = {}
  ): Promise<T> {
    const response = await this.fetchImpl(this.buildURL(route, options.query), {
      method: options.method ?? "GET",
      headers: this.buildHeaders(),
      body: options.body === undefined ? undefined : JSON.stringify(options.body)
    });

    if (!response.ok) {
      throw await toClientError(response);
    }
    return (await response.json()) as T;
  }

  private buildURL(route: string, query?: Record<string, QueryValue>): string {
    const url = new URL(`${this.baseURL}${normalizeRoute(route)}`);
    if (!query) {
      return url.toString();
    }

    for (const [key, value] of Object.entries(query)) {
      if (value === undefined || value === null || value === "") {
        continue;
      }
      url.searchParams.set(key, String(value));
    }
    return url.toString();
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

type QueryValue = string | number | boolean | undefined | null;

function trimBaseURL(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    throw new Error("baseURL is required");
  }
  return trimmed.replace(/\/+$/, "");
}

function normalizeRoute(route: string): string {
  const trimmed = route.trim();
  if (!trimmed) {
    throw new Error("route is required");
  }
  return trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
}

function normalizeChatInput(input: ChatQueryInput): ChatQueryInput {
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
    // Ignore non-json failure bodies.
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
    if (line.startsWith(":")) {
      continue;
    }
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
