import { describe, expect, it, vi } from "vitest";

import { DopeClientError, createDopeClient, type MembershipResource, type TenantResource, type TenantSecretResource } from "./index.js";

function mockJSONResponse(status: number, payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: {
      "Content-Type": "application/json"
    }
  });
}

function tenantResource(overrides: Partial<TenantResource> = {}): TenantResource {
  return {
    tenantId: "ten_personal",
    tenantKind: "personal",
    displayName: "Personal Tenant",
    status: "active",
    createdAt: "2026-04-24T10:00:00Z",
    updatedAt: "2026-04-24T10:00:00Z",
    callerMembershipRole: "owner",
    callerMembershipStatus: "active",
    callerPermissions: ["tenant.manage"],
    defaultForCurrentToken: true,
    defaultForCurrentPrincipal: true,
    ...overrides
  };
}

function membershipResource(overrides: Partial<MembershipResource> = {}): MembershipResource {
  return {
    membershipId: "mem_1",
    tenantId: "ten_personal",
    principalId: "prn_1",
    role: "owner",
    status: "active",
    createdAt: "2026-04-24T10:00:00Z",
    updatedAt: "2026-04-24T10:00:00Z",
    ...overrides
  };
}

function tenantSecretResource(overrides: Partial<TenantSecretResource> = {}): TenantSecretResource {
  return {
    secretId: "sec_1",
    tenantId: "ten_personal",
    secretRef: "provider/api-key",
    displayName: "Provider API key",
    status: "active",
    activeVersionId: "secver_1",
    createdAt: "2026-04-24T10:00:00Z",
    updatedAt: "2026-04-24T10:00:00Z",
    rotatedAt: "2026-04-24T10:00:00Z",
    secretRefs: [{ secretRef: "provider/api-key", resolution: "unavailable", redactionRule: "secret_ref_only" }],
    ...overrides
  };
}

describe("DopeClient", () => {
  it("sends a non-stream chat request", async () => {
    let url = "";
    let authorization = "";

    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19192/",
      accessToken: "token",
      fetchImpl: async (input: string | URL | Request, init?: RequestInit) => {
        url = String(input);
        authorization = String((init?.headers as Record<string, string>).Authorization);
        return mockJSONResponse(200, {
          dispatchId: "dispatch_1",
          provider: "openai_compatible",
          model: "gpt-test",
          skills: ["shared"],
          query: "hello",
          status: "completed",
          partial: false,
          reply: "world",
          usage: { inputTokens: 1, outputTokens: 1, totalTokens: 2 }
        });
      }
    });

    const response = await client.queryChat({ query: "hello", skills: [" shared ", ""] });
    expect(url).toBe("http://127.0.0.1:19192/v1/chat/query");
    expect(authorization).toBe("Bearer token");
    expect(response.reply).toBe("world");
    expect(response.skills).toEqual(["shared"]);
  });

  it("loads operator surfaces, approvals, details, and run creation with normalized URLs", async () => {
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(mockJSONResponse(200, {
        environmentScope: "test",
        status: "ready_for_action",
        blockingItemIds: [],
        optionalFollowUpItemIds: [],
        readinessItems: [],
        firstUsefulActions: [],
        lastEvaluatedAt: "2026-04-24T10:00:00Z"
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        environmentScope: "test",
        items: [],
        generatedAt: "2026-04-24T10:00:00Z"
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        environmentScope: "test",
        items: [],
        generatedAt: "2026-04-24T10:00:00Z"
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        items: [{ approvalId: "approval_1", action: "workflow.launch", reason: "review", status: "pending", createdAt: "2026-04-24T10:00:00Z", updatedAt: "2026-04-24T10:00:00Z" }]
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        runId: "run_1",
        entrypoint: "operator.shell.test",
        status: "queued",
        goal: "shell smoke",
        createdAt: "2026-04-24T10:00:00Z",
        updatedAt: "2026-04-24T10:00:00Z"
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        approval: {
          approvalId: "approval_1",
          action: "workflow.launch",
          reason: "review",
          status: "approved",
          createdAt: "2026-04-24T10:00:00Z",
          updatedAt: "2026-04-24T10:01:00Z",
          resolvedAt: "2026-04-24T10:01:00Z",
          resolution: "approved"
        },
        decision: {
          decisionId: "decision_1",
          action: "workflow.launch",
          outcome: "approved",
          reason: "review",
          approvalId: "approval_1",
          createdAt: "2026-04-24T10:01:00Z"
        }
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        runId: "run_1",
        entrypoint: "operator.shell.test",
        status: "completed",
        goal: "shell smoke",
        createdAt: "2026-04-24T10:00:00Z",
        updatedAt: "2026-04-24T10:02:00Z"
      }));

    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19192/",
      accessToken: "token",
      fetchImpl
    });

    await client.getOnboarding();
    await client.getActivity({ attentionOnly: true, limit: 5 });
    await client.getDiagnostics({ plane: "delivery", severity: "critical" });
    await client.listApprovals("pending");
    await client.createRun({ entrypoint: "operator.shell.test", goal: "shell smoke" });
    await client.resolveApproval("approval_1", { resolution: "approved", comment: "ok" });
    const detail = await client.fetchRoute<{ runId: string }>("v1/runs/run_1");

    expect(fetchImpl).toHaveBeenNthCalledWith(1, "http://127.0.0.1:19192/v1/operator/onboarding", expect.anything());
    expect(fetchImpl).toHaveBeenNthCalledWith(2, "http://127.0.0.1:19192/v1/operator/activity?attentionOnly=true&limit=5", expect.anything());
    expect(fetchImpl).toHaveBeenNthCalledWith(3, "http://127.0.0.1:19192/v1/operator/diagnostics?plane=delivery&severity=critical", expect.anything());
    expect(fetchImpl).toHaveBeenNthCalledWith(4, "http://127.0.0.1:19192/v1/policy/approvals?status=pending", expect.anything());
    expect(fetchImpl).toHaveBeenNthCalledWith(5, "http://127.0.0.1:19192/v1/runs", expect.objectContaining({ method: "POST" }));
    expect(fetchImpl).toHaveBeenNthCalledWith(6, "http://127.0.0.1:19192/v1/policy/approvals/approval_1/resolve", expect.objectContaining({ method: "POST" }));
    expect(fetchImpl).toHaveBeenNthCalledWith(7, "http://127.0.0.1:19192/v1/runs/run_1", expect.anything());
    expect(detail.runId).toBe("run_1");
  });

  it("calls evaluation replay and comparison surfaces", async () => {
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(mockJSONResponse(200, {
        environmentScope: "test",
        items: [{ candidateId: "candidate_1", candidateKind: "fixture", displayName: "Fixture", sourceKind: "fixture", sourceId: "fixture_1", sourceRefs: [], environmentScope: "test", readinessStatus: "fully_replayable", readinessReasons: [], limitations: [], defaultReplayMode: "non_live", createdAt: "2026-04-24T10:00:00Z", updatedAt: "2026-04-24T10:00:00Z" }]
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        candidateId: "candidate_1", candidateKind: "fixture", displayName: "Fixture", sourceKind: "fixture", sourceId: "fixture_1", sourceRefs: [], environmentScope: "test", readinessStatus: "fully_replayable", readinessReasons: [], limitations: [], defaultReplayMode: "non_live", createdAt: "2026-04-24T10:00:00Z", updatedAt: "2026-04-24T10:00:00Z"
      }))
      .mockResolvedValueOnce(mockJSONResponse(201, {
        candidateId: "candidate_curated", candidateKind: "curated_work", displayName: "Curated", sourceKind: "run", sourceId: "run_1", sourceRefs: [], environmentScope: "test", readinessStatus: "partially_replayable", readinessReasons: ["curated"], limitations: ["evidence-only"], defaultReplayMode: "non_live", createdAt: "2026-04-24T10:00:00Z", updatedAt: "2026-04-24T10:00:00Z"
      }))
      .mockResolvedValueOnce(mockJSONResponse(202, {
        attemptId: "attempt_1", candidateId: "candidate_1", sourceRefs: [], environmentScope: "test", mode: "non_live", status: "completed", safetyScope: { mode: "non_live" }, approvalHandling: "evidence_only", sideEffectHandling: "evidence_only", evidenceRefs: [], blockedReasons: [], createdAt: "2026-04-24T10:00:00Z", updatedAt: "2026-04-24T10:00:00Z"
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        environmentScope: "test",
        items: []
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        attemptId: "attempt_1", candidateId: "candidate_1", sourceRefs: [], environmentScope: "test", mode: "non_live", status: "completed", safetyScope: { mode: "non_live" }, approvalHandling: "evidence_only", sideEffectHandling: "evidence_only", evidenceRefs: [], blockedReasons: [], createdAt: "2026-04-24T10:00:00Z", updatedAt: "2026-04-24T10:00:00Z"
      }))
      .mockResolvedValueOnce(mockJSONResponse(201, {
        comparisonId: "comparison_1", candidateId: "candidate_1", baselineRef: "fixture_1", attemptId: "attempt_1", environmentScope: "test", terminalStatus: "matched", runtimeSummary: "runtime", policySummary: "policy", integrationSummary: "integration", deliverySummary: "delivery", evidenceSummary: "evidence", confidence: "high", limitations: [], driftFindings: [], generatedAt: "2026-04-24T10:00:00Z"
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        environmentScope: "test",
        items: []
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        comparisonId: "comparison_1", candidateId: "candidate_1", baselineRef: "fixture_1", attemptId: "attempt_1", environmentScope: "test", terminalStatus: "matched", runtimeSummary: "runtime", policySummary: "policy", integrationSummary: "integration", deliverySummary: "delivery", evidenceSummary: "evidence", confidence: "high", limitations: [], driftFindings: [], generatedAt: "2026-04-24T10:00:00Z"
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        environmentScope: "test",
        items: []
      }));

    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19192/",
      accessToken: "token",
      fetchImpl
    });

    await client.listReplayCandidates({ readinessStatus: "fully_replayable", limit: 5 });
    await client.getReplayCandidate("candidate_1");
    await client.createReplayCandidate({
      candidateId: "candidate_curated",
      candidateKind: "curated_work",
      displayName: "Curated",
      sourceKind: "run",
      sourceId: "run_1",
      sourceRefs: [],
      environmentScope: "test",
      readinessStatus: "partially_replayable",
      readinessReasons: ["curated"],
      limitations: ["evidence-only"],
      defaultReplayMode: "non_live"
    });
    await client.createReplayAttempt("candidate_1", { changeWindowLabel: "phase-33" });
    await client.listReplayAttempts({ candidateId: "candidate_1" });
    await client.getReplayAttempt("attempt_1");
    await client.createReplayComparison("attempt_1", { changeWindowLabel: "phase-33" });
    await client.listReplayComparisons({ terminalStatus: "matched" });
    await client.getReplayComparison("comparison_1");
    await client.listReplayFixtures({ domainClass: "schedule" });

    expect(fetchImpl).toHaveBeenNthCalledWith(1, "http://127.0.0.1:19192/v1/evaluation/replay-candidates?readinessStatus=fully_replayable&limit=5", expect.anything());
    expect(fetchImpl).toHaveBeenNthCalledWith(2, "http://127.0.0.1:19192/v1/evaluation/replay-candidates/candidate_1", expect.anything());
    expect(fetchImpl).toHaveBeenNthCalledWith(3, "http://127.0.0.1:19192/v1/evaluation/replay-candidates", expect.objectContaining({ method: "POST" }));
    expect(fetchImpl).toHaveBeenNthCalledWith(4, "http://127.0.0.1:19192/v1/evaluation/replay-candidates/candidate_1/attempts", expect.objectContaining({ method: "POST" }));
    expect(fetchImpl).toHaveBeenNthCalledWith(7, "http://127.0.0.1:19192/v1/evaluation/replay-attempts/attempt_1/compare", expect.objectContaining({ method: "POST" }));
    expect(fetchImpl).toHaveBeenNthCalledWith(10, "http://127.0.0.1:19192/v1/evaluation/fixtures?domainClass=schedule", expect.anything());
  });

  it("streams chat events until terminal response", async () => {
    const encoder = new TextEncoder();
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(encoder.encode("event: chat.query.started\ndata: {\"dispatchId\":\"dispatch_1\",\"provider\":\"openai_compatible\",\"model\":\"gpt-test\",\"skills\":[\"shared\"],\"query\":\"hello\"}\n\n"));
        controller.enqueue(encoder.encode("event: chat.query.delta\ndata: {\"dispatchId\":\"dispatch_1\",\"delta\":\"hel\",\"reply\":\"hel\"}\n\n"));
        controller.enqueue(encoder.encode("event: chat.query.delta\ndata: {\"dispatchId\":\"dispatch_1\",\"delta\":\"lo\",\"reply\":\"hello\"}\n\n"));
        controller.enqueue(encoder.encode("event: chat.query.completed\ndata: {\"dispatchId\":\"dispatch_1\",\"provider\":\"openai_compatible\",\"model\":\"gpt-test\",\"skills\":[\"shared\"],\"query\":\"hello\",\"status\":\"completed\",\"partial\":false,\"reply\":\"hello\",\"usage\":{\"inputTokens\":1,\"outputTokens\":1,\"totalTokens\":2}}\n\n"));
        controller.close();
      }
    });

    const deltas: string[] = [];
    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19192",
      fetchImpl: async () =>
        new Response(body, {
          status: 200,
          headers: { "Content-Type": "text/event-stream" }
        })
    });

    const response = await client.streamChatQuery({ query: "hello" }, {
      onDelta(payload: { reply: string }) {
        deltas.push(payload.reply);
      }
    });

    expect(deltas).toEqual(["hel", "hello"]);
    expect(response.reply).toBe("hello");
  });

  it("streams daemon events and surfaces them to handlers", async () => {
    const encoder = new TextEncoder();
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(encoder.encode(": stream-open\n\n"));
        controller.enqueue(encoder.encode("id: 12\nevent: policy.approval_requested\ndata: {\"eventId\":\"evt_1\",\"sequence\":12,\"category\":\"policy\",\"name\":\"policy.approval_requested\",\"occurredAt\":\"2026-04-24T10:00:00Z\",\"scope\":{\"runId\":\"run_1\"},\"resource\":{\"kind\":\"approval\",\"id\":\"approval_1\"},\"payload\":{\"status\":\"pending\"}}\n\n"));
        controller.close();
      }
    });

    const seen: string[] = [];
    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19192",
      accessToken: "token",
      fetchImpl: async () =>
        new Response(body, {
          status: 200,
          headers: { "Content-Type": "text/event-stream" }
        })
    });

    const subscription = client.streamEvents({ category: "policy", cursor: 10 }, {
      onEvent(event) {
        seen.push(`${event.name}:${event.sequence}`);
      }
    });

    await subscription.completed;
    expect(seen).toEqual(["policy.approval_requested:12"]);
  });

  it("maps error responses into DopeClientError", async () => {
    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19192",
      fetchImpl: async () => mockJSONResponse(502, { error: "bad key", errorCode: "upstream_auth_failed" })
    });

    await expect(client.queryChat({ query: "hello" })).rejects.toMatchObject({
      name: "DopeClientError",
      status: 502,
      code: "upstream_auth_failed",
      message: "bad key"
    });
  });

  it("binds the default fetch implementation to the browser global", async () => {
    const originalFetch = globalThis.fetch;
    let observedThis: unknown;

    globalThis.fetch = function (this: unknown, input: string | URL | Request, init?: RequestInit): Promise<Response> {
      observedThis = this;
      return Promise.resolve(mockJSONResponse(200, {
        dispatchId: "dispatch_1",
        provider: "openai_compatible",
        model: "gpt-test",
        skills: [],
        query: "hello",
        status: "completed",
        partial: false,
        reply: "world",
        usage: { inputTokens: 1, outputTokens: 1, totalTokens: 2 }
      }));
    } as typeof fetch;

    try {
      const client = createDopeClient({
        baseURL: "http://127.0.0.1:19192"
      });

      await client.queryChat({ query: "hello" });
      expect(observedThis).toBe(globalThis);
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  it("propagates default tenant, per-request override, omitted tenant, and stream tenant headers", async () => {
    const observedHeaders: Array<Record<string, string>> = [];
    const encoder = new TextEncoder();
    const streamBody = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(encoder.encode("event: chat.query.completed\ndata: {\"dispatchId\":\"dispatch_1\",\"provider\":\"echo\",\"model\":\"echo\",\"skills\":[],\"query\":\"hello\",\"status\":\"completed\",\"partial\":false,\"reply\":\"ok\",\"usage\":{\"inputTokens\":1,\"outputTokens\":1,\"totalTokens\":2}}\n\n"));
        controller.close();
      }
    });

    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19192",
      defaultTenantId: "ten_default",
      fetchImpl: async (_input, init) => {
        observedHeaders.push(init?.headers as Record<string, string>);
        if (observedHeaders.length === 3) {
          return new Response(streamBody, { status: 200, headers: { "Content-Type": "text/event-stream" } });
        }
        return mockJSONResponse(200, {
          environmentScope: "test",
          items: [],
          generatedAt: "2026-04-24T10:00:00Z"
        });
      }
    });

    await client.getActivity();
    await client.getActivity({}, { tenantId: "ten_override" });
    await client.streamChatQuery({ query: "hello" });

    const tenantlessClient = createDopeClient({
      baseURL: "http://127.0.0.1:19192",
      fetchImpl: async (_input, init) => {
        observedHeaders.push(init?.headers as Record<string, string>);
        return mockJSONResponse(200, { environmentScope: "test", items: [], generatedAt: "2026-04-24T10:00:00Z" });
      }
    });
    await tenantlessClient.getActivity();

    expect(observedHeaders[0]["X-Dope-Tenant-ID"]).toBe("ten_default");
    expect(observedHeaders[1]["X-Dope-Tenant-ID"]).toBe("ten_override");
    expect(observedHeaders[2]["X-Dope-Tenant-ID"]).toBe("ten_default");
    expect(observedHeaders[3]["X-Dope-Tenant-ID"]).toBeUndefined();
  });

  it("exports tenant helpers and maps stable tenant denial metadata", async () => {
    const tenant = tenantResource();
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(mockJSONResponse(200, {
        token: { tokenId: "tok_1" },
        principal: {
          principalId: "prn_1",
          principalKind: "local_operator",
          displayName: "Local",
          status: "active",
          defaultTenantId: tenant.tenantId,
          createdAt: "2026-04-24T10:00:00Z",
          updatedAt: "2026-04-24T10:00:00Z"
        },
        defaultTenant: tenant,
        currentTenant: tenant,
        allowedTenants: [tenant],
        tokenGrants: [],
        permissions: ["tenant.manage"],
        tenantContext: {
          principalId: "prn_1",
          tokenId: "tok_1",
          tenantId: tenant.tenantId,
          tenantSource: "default",
          permissions: ["tenant.manage"],
          resolvedAt: "2026-04-24T10:00:00Z"
        }
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, { items: [tenant] }))
      .mockResolvedValueOnce(mockJSONResponse(200, { tenant, tenantContext: { principalId: "prn_1", tokenId: "tok_1", tenantId: tenant.tenantId, tenantSource: "explicit_header", permissions: ["tenant.manage"], resolvedAt: "2026-04-24T10:00:00Z" } }))
      .mockResolvedValueOnce(mockJSONResponse(403, { error: "tenant access denied", errorCode: "tenant_access_denied" }));

    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19192",
      fetchImpl
    });

    await expect(client.getMe()).resolves.toMatchObject({ currentTenant: { tenantId: "ten_personal" } });
    await expect(client.listTenants()).resolves.toMatchObject({ items: [{ tenantId: "ten_personal" }] });
    await expect(client.getTenant("ten_personal", { tenantId: "ten_personal" })).resolves.toMatchObject({ tenant: { tenantId: "ten_personal" } });
    await expect(client.getTenant("ten_denied", { tenantId: "ten_denied" })).rejects.toMatchObject({
      name: "DopeClientError",
      status: 403,
      code: "tenant_access_denied",
      tenantDenied: true,
      denial: { errorCode: "tenant_access_denied" }
    });
  });

  it("calls membership helper routes with active tenant intent", async () => {
    const membership = membershipResource();
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(mockJSONResponse(200, { items: [membership] }))
      .mockResolvedValueOnce(mockJSONResponse(200, { membership: { ...membership, role: "admin" } }))
      .mockResolvedValueOnce(mockJSONResponse(200, { membership: { ...membership, status: "removed" } }))
      .mockResolvedValueOnce(mockJSONResponse(200, { items: [membership] }));

    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19192",
      fetchImpl
    });

    await client.listMemberships("ten_personal", {}, { tenantId: "ten_personal" });
    await client.updateMembershipRole("ten_personal", "mem_1", { role: "admin" }, { tenantId: "ten_personal" });
    await client.removeMembership("ten_personal", "mem_1", { tenantId: "ten_personal" });
    await client.listMemberships("ten_personal");

    expect(fetchImpl).toHaveBeenNthCalledWith(1, "http://127.0.0.1:19192/v1/tenants/ten_personal/memberships", expect.objectContaining({
      headers: expect.objectContaining({ "X-Dope-Tenant-ID": "ten_personal" })
    }));
    expect(fetchImpl).toHaveBeenNthCalledWith(2, "http://127.0.0.1:19192/v1/tenants/ten_personal/memberships/mem_1", expect.objectContaining({
      method: "PATCH",
      headers: expect.objectContaining({ "X-Dope-Tenant-ID": "ten_personal" })
    }));
    expect(fetchImpl).toHaveBeenNthCalledWith(3, "http://127.0.0.1:19192/v1/tenants/ten_personal/memberships/mem_1", expect.objectContaining({
      method: "DELETE",
      headers: expect.objectContaining({ "X-Dope-Tenant-ID": "ten_personal" })
    }));
    const fourthCall = fetchImpl.mock.calls[3][1];
    expect((fourthCall?.headers as Record<string, string>)["X-Dope-Tenant-ID"]).toBeUndefined();
  });

  it("calls tenant secret helper routes with redacted resource types", async () => {
    const secret = tenantSecretResource();
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(mockJSONResponse(200, { items: [secret] }))
      .mockResolvedValueOnce(mockJSONResponse(200, { secret }))
      .mockResolvedValueOnce(mockJSONResponse(201, { secret }))
      .mockResolvedValueOnce(mockJSONResponse(200, { secret: { ...secret, displayName: "Updated Key" } }))
      .mockResolvedValueOnce(mockJSONResponse(200, { secret: { ...secret, activeVersionId: "secver_2" } }))
      .mockResolvedValueOnce(mockJSONResponse(200, { secret: { ...secret, status: "disabled", disabledReason: "operator_request" } }));

    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19192",
      fetchImpl,
      defaultTenantId: "ten_personal"
    });

    await expect(client.listTenantSecrets()).resolves.toMatchObject({ items: [{ secretRef: "provider/api-key" }] });
    await expect(client.getTenantSecret("provider/api-key")).resolves.toMatchObject({ secret: { secretRef: "provider/api-key" } });
    await client.createTenantSecret({ secretRef: "provider/api-key", displayName: " Provider API key ", value: "raw-secret" });
    await client.updateTenantSecret("provider/api-key", { displayName: " Updated Key " });
    await client.rotateTenantSecret("provider/api-key", { value: "new-raw-secret" });
    await client.disableTenantSecret("provider/api-key", { disabledReason: " operator_request " });

    expect(fetchImpl).toHaveBeenNthCalledWith(2, "http://127.0.0.1:19192/v1/tenant-secrets/provider%2Fapi-key", expect.objectContaining({
      headers: expect.objectContaining({ "X-Dope-Tenant-ID": "ten_personal" })
    }));
    expect(fetchImpl).toHaveBeenNthCalledWith(3, "http://127.0.0.1:19192/v1/tenant-secrets", expect.objectContaining({
      method: "POST",
      body: JSON.stringify({ secretRef: "provider/api-key", displayName: "Provider API key", value: "raw-secret" })
    }));
    expect(fetchImpl).toHaveBeenNthCalledWith(5, "http://127.0.0.1:19192/v1/tenant-secrets/provider%2Fapi-key/rotate", expect.objectContaining({
      method: "POST",
      body: JSON.stringify({ value: "new-raw-secret" })
    }));
    expect(fetchImpl).toHaveBeenNthCalledWith(6, "http://127.0.0.1:19192/v1/tenant-secrets/provider%2Fapi-key/disable", expect.objectContaining({
      method: "POST",
      body: JSON.stringify({ disabledReason: "operator_request" })
    }));
  });

  it("maps hosted credential stable denials", async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValueOnce(mockJSONResponse(403, {
      error: "credential_access_denied",
      reasonCode: "credential_denied:missing_permission"
    }));
    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19192",
      fetchImpl
    });

    await expect(client.listTenantSecrets({ tenantId: "ten_personal" })).rejects.toMatchObject({
      name: "DopeClientError",
      status: 403,
      code: "credential_denied:missing_permission",
      tenantDenied: true
    });
  });

  it("calls billing inspection and admin routes with tenant intent", async () => {
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(mockJSONResponse(200, {
        planId: "plan_1",
        tenantId: "ten_personal",
        planKey: "finite",
        status: "active",
        enforcementMode: "enforced",
        effectiveAt: "2026-04-28T10:00:00Z"
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        tenantId: "ten_personal",
        planKey: "finite",
        enforcementMode: "enforced",
        quotas: []
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, { items: [] }))
      .mockResolvedValueOnce(mockJSONResponse(200, { items: [] }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        planId: "plan_2",
        tenantId: "ten_personal",
        planKey: "unlimited",
        status: "active",
        enforcementMode: "unlimited",
        effectiveAt: "2026-04-28T10:01:00Z"
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        quotaOverrideId: "override_1",
        tenantId: "ten_personal",
        category: "run_launches",
        limit: 10,
        reason: "temporary increase"
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        adjustmentId: "adjustment_1",
        tenantId: "ten_personal",
        category: "run_launches",
        quotaPeriodId: "period_1",
        amountDelta: -1,
        reason: "operator correction",
        createdAt: "2026-04-28T10:02:00Z"
      }))
      .mockResolvedValueOnce(mockJSONResponse(200, {
        reservationId: "reservation_1",
        tenantId: "ten_personal",
        category: "run_launches",
        quotaPeriodId: "period_1",
        operationKey: "tenant:ten_personal:run:client_1",
        amountReserved: 1,
        amountCommitted: 0,
        amountRefunded: 1,
        status: "released",
        createdAt: "2026-04-28T10:00:00Z",
        updatedAt: "2026-04-28T10:03:00Z"
      }));

    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19192/",
      defaultTenantId: "ten_personal",
      fetchImpl
    });

    await client.getBillingPlan();
    await client.getBillingUsage();
    await client.listBillingQuotas();
    await client.listBillingDenials();
    await client.assignBillingPlan("ten_personal", { planKey: " unlimited ", enforcementMode: "unlimited", reason: " operator request " });
    await client.createBillingQuotaOverride("ten_personal", { category: "run_launches", limit: 10, reason: " temporary increase " });
    await client.createBillingManualAdjustment("ten_personal", { category: "run_launches", quotaPeriodId: " period_1 ", amountDelta: -1, reason: " operator correction " });
    await client.resolveBillingReservation("ten_personal", " reservation_1 ", { outcome: "released", reason: " operator verified no work started ", amount: 1 });

    expect(fetchImpl).toHaveBeenNthCalledWith(1, "http://127.0.0.1:19192/v1/billing/plan", expect.objectContaining({ headers: expect.objectContaining({ "X-Dope-Tenant-ID": "ten_personal" }) }));
    expect(fetchImpl).toHaveBeenNthCalledWith(2, "http://127.0.0.1:19192/v1/billing/usage", expect.anything());
    expect(fetchImpl).toHaveBeenNthCalledWith(3, "http://127.0.0.1:19192/v1/billing/quotas", expect.anything());
    expect(fetchImpl).toHaveBeenNthCalledWith(4, "http://127.0.0.1:19192/v1/billing/denials", expect.anything());
    expect(fetchImpl).toHaveBeenNthCalledWith(5, "http://127.0.0.1:19192/v1/admin/billing/tenants/ten_personal/plan", expect.objectContaining({ method: "POST" }));
    expect(fetchImpl).toHaveBeenNthCalledWith(6, "http://127.0.0.1:19192/v1/admin/billing/tenants/ten_personal/quota-overrides", expect.objectContaining({ method: "POST" }));
    expect(fetchImpl).toHaveBeenNthCalledWith(7, "http://127.0.0.1:19192/v1/admin/billing/tenants/ten_personal/manual-adjustments", expect.objectContaining({ method: "POST" }));
    expect(fetchImpl).toHaveBeenNthCalledWith(8, "http://127.0.0.1:19192/v1/admin/billing/tenants/ten_personal/reservations/reservation_1/resolve", expect.objectContaining({ method: "POST" }));
  });

  it("maps quota denial payloads into DopeClientError", async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValueOnce(mockJSONResponse(429, {
      error: "quota exhausted",
      code: "quota_denied",
      reasonCode: "quota_denied:run_launches_exhausted",
      tenantId: "ten_personal",
      category: "run_launches",
      operationKey: "tenant:ten_personal:run:client_1",
      requestedAmount: 1,
      remainingAmount: 0,
      periodStart: "2026-04-01T00:00:00Z",
      periodEnd: "2026-05-01T00:00:00Z"
    }));
    const client = createDopeClient({
      baseURL: "http://127.0.0.1:19192",
      fetchImpl
    });

    await expect(client.createRun({ entrypoint: "operator.shell.test" })).rejects.toMatchObject({
      name: "DopeClientError",
      status: 429,
      code: "quota_denied:run_launches_exhausted",
      quotaDenial: {
        code: "quota_denied",
        reasonCode: "quota_denied:run_launches_exhausted",
        category: "run_launches",
        remainingAmount: 0
      }
    });
  });
});
