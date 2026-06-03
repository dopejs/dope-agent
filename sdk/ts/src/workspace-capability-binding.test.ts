import { describe, expect, it, vi } from "vitest";

import { createDopeClient } from "./index";

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "Content-Type": "application/json" }
  });
}

function workspace(workspaceId = "ws_1", isDefault = true) {
  return {
    workspaceId,
    tenantId: "ten_58",
    displayName: "Personal Workspace",
    status: "active",
    isDefault,
    repairStatus: "healthy",
    redactionStatus: "not_required",
    createdAt: "2026-06-03T00:00:00Z",
    updatedAt: "2026-06-03T00:00:00Z"
  };
}

function binding(bindingId = "bnd_1") {
  return {
    bindingId,
    tenantId: "ten_58",
    scopeKind: "channel",
    scopeLabel: "discord:c1",
    selectedProfileId: "prof_1",
    selectedWorkspaceId: "ws_1",
    status: "active",
    repairStatus: "healthy",
    validationStatus: "valid",
    resultingSelectionSummary: "profile+workspace",
    lastMaterialChangeAt: "2026-06-03T00:00:00Z",
    redactionStatus: "redacted"
  };
}

describe("workspace and capability binding SDK", () => {
  it("routes workspace, binding, and capability visibility operations", async () => {
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse({ tenantId: "ten_58", workspaces: [workspace()] }))
      .mockResolvedValueOnce(jsonResponse(binding()))
      .mockResolvedValueOnce(jsonResponse({ tenantId: "ten_58", bindings: [binding()] }))
      .mockResolvedValueOnce(jsonResponse({ ...binding(), status: "disabled", repairStatus: "disabled" }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(jsonResponse({ policyId: "cvp_1", tenantId: "ten_58", scopeKind: "workspace", scopeRef: "ws_1", capabilityId: "tool.shell", visibility: "hidden", validationStatus: "valid" }));

    const client = createDopeClient({ baseURL: "http://localhost:19192", fetchImpl, accessToken: "t" });

    const workspaces = await client.listWorkspaces();
    expect(workspaces.workspaces[0].isDefault).toBe(true);

    const created = await client.createBinding({ scopeKind: "channel", scopeRef: "discord:c1", selectedProfileId: "prof_1", selectedWorkspaceId: "ws_1" });
    expect(created.bindingId).toBe("bnd_1");

    const list = await client.listBindings();
    expect(list.bindings).toHaveLength(1);

    const disabled = await client.updateBinding("bnd_1", { disable: true });
    expect(disabled.status).toBe("disabled");

    await client.removeBinding("bnd_1");

    const policy = await client.setCapabilityVisibility({ scopeKind: "workspace", scopeRef: "ws_1", capabilityId: "tool.shell", visibility: "hidden" });
    expect(policy.visibility).toBe("hidden");

    expect(fetchImpl).toHaveBeenCalledTimes(6);
  });

  it("remains backward compatible when binding fields and tenantId are absent (FR-024)", async () => {
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse({ workspaces: [{ ...workspace(), tenantId: undefined }] }));
    const client = createDopeClient({ baseURL: "http://localhost:19192", fetchImpl, accessToken: "t" });
    const workspaces = await client.listWorkspaces();
    // An older daemon/client omitting tenantId must still parse without error.
    expect(workspaces.workspaces[0].workspaceId).toBe("ws_1");
  });
});
