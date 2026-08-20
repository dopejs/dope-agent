import { describe, expect, it, vi } from "vitest";

import { createKuraClient } from "./index";

function jsonResponse(payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    status: 200,
    headers: { "Content-Type": "application/json" }
  });
}

function profile(profileId = "prof_1") {
  return {
    profileId,
    tenantId: "ten_profile",
    displayName: "Support Agent",
    displayIdentity: { name: "Support", safeSummary: "Support" },
    persona: { tone: "direct", safeSummary: "direct support" },
    defaultProviderPreference: { validationState: "valid" },
    safetyDefaults: { approvalPosture: "ask", validationState: "valid" },
    status: "active",
    activeVersionId: "profv_1",
    tenantDefault: true,
    overlayReferenceCount: 0,
    createdAt: "2026-05-12T00:00:00Z",
    updatedAt: "2026-05-12T00:00:00Z",
    redactionStatus: "redacted"
  };
}

describe("agent profile SDK", () => {
  it("routes list, detail, create, update, activation, versions, rollback, archive, and disable", async () => {
    const fetchImpl = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse({ tenantId: "ten_profile", page: { limit: 10, order: "updated_at_desc" }, items: [profile()] }))
      .mockResolvedValueOnce(jsonResponse({ profile: profile(), versions: [], overlayReferences: [{ overlayReferenceId: "ovr_1", profileId: "prof_1", tenantId: "ten_profile", referenceKind: "prompt", scope: "profile", referenceUri: "prompt://profile", safeDisplayLabel: "profile", validationState: "partial", failureReasonCode: "legacy_prompt_config_partial", createdAt: "2026-05-12T00:00:00Z", updatedAt: "2026-05-12T00:00:00Z", redactionStatus: "redacted" }], auditEvents: [] }))
      .mockResolvedValueOnce(jsonResponse({ profile: profile("prof_2"), version: { profileVersionId: "profv_2", profileId: "prof_2", tenantId: "ten_profile", versionNumber: 1, changeKind: "created", changeSummary: "Created", snapshot: profile("prof_2"), rollbackEligibility: "eligible", createdAt: "2026-05-12T00:00:00Z", redactionStatus: "redacted" }, auditEventId: "audit_1" }))
      .mockResolvedValueOnce(jsonResponse({ profile: profile(), version: { profileVersionId: "profv_3", profileId: "prof_1", tenantId: "ten_profile", versionNumber: 2, changeKind: "updated", changeSummary: "Updated", snapshot: profile(), rollbackEligibility: "eligible", createdAt: "2026-05-12T00:00:00Z", redactionStatus: "redacted" }, auditEventId: "audit_2" }))
      .mockResolvedValueOnce(jsonResponse({ selectionId: "sel_1", tenantId: "ten_profile", profileId: "prof_1", profileVersionId: "profv_3", selectionScope: "tenant_default", selectionReason: "user_activated", selectedAt: "2026-05-12T00:00:00Z", redactionStatus: "redacted" }))
      .mockResolvedValueOnce(jsonResponse({ items: [] }))
      .mockResolvedValueOnce(jsonResponse({ profile: profile(), version: { profileVersionId: "profv_4", profileId: "prof_1", tenantId: "ten_profile", versionNumber: 3, changeKind: "rolled_back", changeSummary: "Rollback", snapshot: profile(), rollbackEligibility: "eligible", createdAt: "2026-05-12T00:00:00Z", redactionStatus: "redacted" }, auditEventId: "audit_3" }))
      .mockResolvedValueOnce(jsonResponse({ profile: { ...profile(), status: "archived" }, version: { profileVersionId: "profv_5", profileId: "prof_1", tenantId: "ten_profile", versionNumber: 4, changeKind: "archived", changeSummary: "Archived", snapshot: profile(), rollbackEligibility: "profile_archived", createdAt: "2026-05-12T00:00:00Z", redactionStatus: "redacted" }, auditEventId: "audit_4" }))
      .mockResolvedValueOnce(jsonResponse({ profile: { ...profile(), status: "disabled" }, version: { profileVersionId: "profv_6", profileId: "prof_1", tenantId: "ten_profile", versionNumber: 5, changeKind: "disabled", changeSummary: "Disabled", snapshot: profile(), rollbackEligibility: "profile_disabled", createdAt: "2026-05-12T00:00:00Z", redactionStatus: "redacted" }, auditEventId: "audit_5" }));
    const client = createKuraClient({ baseURL: "http://127.0.0.1:19192", accessToken: "token", defaultTenantId: "ten_profile", fetchImpl });

    await client.listAgentProfiles({ limit: 10 });
    const detail = await client.getAgentProfile(" prof_1 ");
    await client.createAgentProfile({ displayName: "Support Agent", activate: true });
    await client.updateAgentProfile("prof_1", { displayName: "Support Agent" });
    await client.activateAgentProfile("prof_1", { profileVersionId: "profv_3" });
    await client.listAgentProfileVersions("prof_1");
    await client.rollbackAgentProfile("prof_1", { sourceProfileVersionId: "profv_1" });
    await client.archiveAgentProfile("prof_1");
    await client.disableAgentProfile("prof_1");

    expect(fetchImpl.mock.calls.map((call) => call[0])).toEqual([
      "http://127.0.0.1:19192/v1/profiles?limit=10",
      "http://127.0.0.1:19192/v1/profiles/prof_1",
      "http://127.0.0.1:19192/v1/profiles",
      "http://127.0.0.1:19192/v1/profiles/prof_1",
      "http://127.0.0.1:19192/v1/profiles/prof_1/activate",
      "http://127.0.0.1:19192/v1/profiles/prof_1/versions",
      "http://127.0.0.1:19192/v1/profiles/prof_1/rollback",
      "http://127.0.0.1:19192/v1/profiles/prof_1/archive",
      "http://127.0.0.1:19192/v1/profiles/prof_1/disable"
    ]);
    expect(fetchImpl.mock.calls[0]?.[1]?.headers).toMatchObject({ Authorization: "Bearer token", "X-Kura-Tenant-ID": "ten_profile" });
    expect(fetchImpl.mock.calls[3]?.[1]?.method).toBe("PATCH");
    expect(detail.overlayReferences[0]?.validationState).toBe("partial");
    expect(detail.overlayReferences[0]?.failureReasonCode).toBe("legacy_prompt_config_partial");
  });

  it("decodes denied profile retirement without hiding the daemon error", async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify({ error: "profiles.manage is required" }), {
      status: 403,
      headers: { "Content-Type": "application/json" }
    }));
    const client = createKuraClient({ baseURL: "http://127.0.0.1:19192", fetchImpl });
    await expect(client.archiveAgentProfile("prof_1")).rejects.toThrow("profiles.manage is required");
  });
});
