import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AgentProfileEditor } from "./AgentProfileEditor";
import { AgentProfileHistory } from "./AgentProfileHistory";

const profile = {
  profileId: "prof_1",
  tenantId: "ten_profile",
  displayName: "Support Agent",
  displayIdentity: { name: "Support", safeSummary: "Support" },
  persona: { tone: "direct", safeSummary: "direct support" },
  defaultProviderPreference: { validationState: "valid" as const },
  safetyDefaults: { approvalPosture: "ask", validationState: "valid" as const },
  status: "active" as const,
  tenantDefault: true,
  overlayReferenceCount: 0,
  createdAt: "2026-05-12T00:00:00Z",
  updatedAt: "2026-05-12T00:00:00Z",
  redactionStatus: "redacted" as const
};

describe("AgentProfileEditor", () => {
  it("renders list, action controls, and non-memory evidence labels", async () => {
    const onSelectProfile = vi.fn();
    const onActivate = vi.fn();
    const onArchive = vi.fn();
    const onDisable = vi.fn();
    const onSave = vi.fn();
    const onCreate = vi.fn();
    const user = userEvent.setup();
    render(
      <AgentProfileEditor
        profiles={{ tenantId: "ten_profile", page: { limit: 20, order: "updated_at_desc" }, items: [profile] }}
        detail={{ profile, versions: [], overlayReferences: [
          { overlayReferenceId: "ovr_1", profileId: "prof_1", tenantId: "ten_profile", referenceKind: "prompt", scope: "profile", referenceUri: "prompt://profile", safeDisplayLabel: "profile", validationState: "valid", createdAt: "2026-05-12T00:00:00Z", updatedAt: "2026-05-12T00:00:00Z", redactionStatus: "redacted" },
          { overlayReferenceId: "ovr_2", profileId: "prof_1", tenantId: "ten_profile", referenceKind: "config", scope: "profile", referenceUri: "config://profile", safeDisplayLabel: "profile", validationState: "partial", createdAt: "2026-05-12T00:00:00Z", updatedAt: "2026-05-12T00:00:00Z", redactionStatus: "redacted" }
        ], auditEvents: [] }}
        onSelectProfile={onSelectProfile}
        onSave={onSave}
        onCreate={onCreate}
        onActivate={onActivate}
        onArchive={onArchive}
        onDisable={onDisable}
      />
    );

    expect(screen.getByText("Support Agent")).toBeTruthy();
    expect(screen.getByText("Evidence is explicit profile configuration, not assistant memory.")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Inspect" }));
    await user.click(screen.getByRole("button", { name: "Activate" }));
    await user.click(screen.getByRole("button", { name: "Archive" }));
    await user.click(screen.getByRole("button", { name: "Disable" }));
    await user.clear(screen.getByLabelText("Display name"));
    await user.type(screen.getByLabelText("Display name"), "Escalation Agent");
    await user.clear(screen.getByLabelText("Overlay 2 URI"));
    await user.type(screen.getByLabelText("Overlay 2 URI"), "config://profile-updated");
    await user.click(screen.getByRole("button", { name: "Add overlay" }));
    await user.type(screen.getByLabelText("Overlay 3 URI"), "prompt://profile-extra");
    await user.click(screen.getByRole("button", { name: "Save profile" }));
    await user.click(screen.getByRole("button", { name: "Create profile" }));
    expect(onSelectProfile).toHaveBeenCalledWith("prof_1");
    expect(onActivate).toHaveBeenCalledWith("prof_1");
    expect(onArchive).toHaveBeenCalledWith("prof_1");
    expect(onDisable).toHaveBeenCalledWith("prof_1");
    expect(onSave).toHaveBeenCalledWith(expect.objectContaining({
      displayName: "Escalation Agent",
      overlayReferences: [
        expect.objectContaining({ referenceUri: "prompt://profile" }),
        expect.objectContaining({ referenceUri: "config://profile-updated" }),
        expect.objectContaining({ referenceUri: "prompt://profile-extra" })
      ]
    }));
    expect(onCreate).toHaveBeenCalledWith(expect.objectContaining({ displayName: "Escalation Agent" }));
    expect(screen.getByText("prompt valid profile")).toBeTruthy();
    expect(screen.getByText("config partial profile")).toBeTruthy();
  });

  it("renders denied and history rollback states", async () => {
    const onRollback = vi.fn();
    const user = userEvent.setup();
    const version = {
      profileVersionId: "profv_1",
      profileId: "prof_1",
      tenantId: "ten_profile",
      versionNumber: 1,
      changeKind: "created" as const,
      changeSummary: "Created",
      snapshot: profile,
      rollbackEligibility: "eligible" as const,
      createdAt: "2026-05-12T00:00:00Z",
      redactionStatus: "redacted" as const
    };
    const { rerender } = render(<AgentProfileEditor denied />);
    expect(screen.getByText("profiles.inspect is required to inspect agent profiles.")).toBeTruthy();

    rerender(<AgentProfileHistory versions={[version]} onRollback={onRollback} />);
    await user.click(screen.getByRole("button", { name: "Rollback" }));
    expect(onRollback).toHaveBeenCalledWith("profv_1");
  });
});
