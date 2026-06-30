import { describe, expect, it } from "vitest";

import {
  OPERATOR_SHELL_SECTIONS,
  allSurfaceRoutes,
  criticalSurfaces,
  resolveViewState,
} from "./navigation";

describe("operator shell information architecture (Roadmap 70)", () => {
  it("organizes all public non-knowledge surfaces into coherent sections (FR-001)", () => {
    const sectionIds = OPERATOR_SHELL_SECTIONS.map((s) => s.id);
    for (const required of ["setup", "channels", "sessions", "profiles", "routines", "providers", "quota", "diagnostics"]) {
      expect(sectionIds).toContain(required);
    }
    // The Roadmap 65-69 surfaces are reachable from the shell.
    const routes = allSurfaceRoutes();
    expect(routes).toContain("/routines/new");
    expect(routes).toContain("/routines/webhooks");
    expect(routes).toContain("/providers/catalog");
    expect(routes).toContain("/providers/execution");
    expect(routes).toContain("/diagnostics/support");
  });

  it("declares permission/approval/quota/side-effect expectations on critical actions (FR: critical actions)", () => {
    for (const surface of criticalSurfaces()) {
      const c = surface.critical!;
      const declaresSomething = Boolean(c.permission) || Boolean(c.approval) || c.quota === true || c.sideEffect === true;
      expect(declaresSomething).toBe(true);
    }
    const routineCreate = criticalSurfaces().find((s) => s.id === "routine-create");
    expect(routineCreate?.critical?.quota).toBe(true);
    expect(routineCreate?.critical?.approval).toBe("scope");
  });

  it("requires tenant selection (denied without tenant) so tenant context is preserved (FR-002)", () => {
    expect(resolveViewState({ tenantSelected: false, requiresTenant: true })).toBe("denied");
    expect(resolveViewState({ tenantSelected: true, requiresTenant: true, loading: true })).toBe("loading");
  });

  it("maps every load result to a stable, explainable state (FR-001/FR-004)", () => {
    const base = { tenantSelected: true, requiresTenant: true };
    expect(resolveViewState({ ...base, itemCount: 0 })).toBe("empty");
    expect(resolveViewState({ ...base, itemCount: 3 })).toBe("ready");
    expect(resolveViewState({ ...base, errorCode: "permission_denied" })).toBe("denied");
    expect(resolveViewState({ ...base, errorCode: "unsupported" })).toBe("unsupported");
    expect(resolveViewState({ ...base, errorCode: "provider_unavailable" })).toBe("error");
  });
});
