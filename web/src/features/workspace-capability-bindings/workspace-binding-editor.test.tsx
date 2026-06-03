import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { WorkspaceBindingEditor } from "./WorkspaceBindingEditor";

const workspaces = {
  tenantId: "ten_58",
  workspaces: [
    {
      workspaceId: "ws_1",
      displayName: "Personal Workspace",
      status: "active" as const,
      isDefault: true,
      repairStatus: "healthy" as const,
      redactionStatus: "not_required",
      createdAt: "2026-06-03T00:00:00Z",
      updatedAt: "2026-06-03T00:00:00Z"
    }
  ]
};

const bindings = {
  tenantId: "ten_58",
  bindings: [
    {
      bindingId: "bnd_1",
      scopeKind: "channel" as const,
      scopeLabel: "discord:c1",
      selectedProfileId: "prof_1",
      selectedWorkspaceId: "ws_1",
      status: "active" as const,
      repairStatus: "needs_repair" as const,
      validationStatus: "valid" as const,
      lastMaterialChangeAt: "2026-06-03T00:00:00Z",
      redactionStatus: "redacted"
    }
  ]
};

describe("WorkspaceBindingEditor", () => {
  it("renders workspaces, bindings, repair affordance, and non-memory evidence", async () => {
    const onRepairBinding = vi.fn();
    render(
      <WorkspaceBindingEditor workspaces={workspaces} bindings={bindings} onRepairBinding={onRepairBinding} />
    );
    expect(screen.getByText(/Personal Workspace/)).toBeTruthy();
    expect(screen.getByText(/discord:c1/)).toBeTruthy();
    expect(screen.getByText(/never create memory-backed workspace knowledge/)).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Repair" }));
    expect(onRepairBinding).toHaveBeenCalledWith("bnd_1");
  });

  it("shows a denial message when access is denied", () => {
    render(<WorkspaceBindingEditor denied />);
    expect(screen.getByRole("alert").textContent).toMatch(/do not have permission/);
  });
});
