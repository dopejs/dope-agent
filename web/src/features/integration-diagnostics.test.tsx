import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { IntegrationDiagnosticResultResource } from "@kura/client";
import { IntegrationDiagnosticsView } from "./integration-diagnostics";

function diagnostic(overrides: Partial<IntegrationDiagnosticResultResource>): IntegrationDiagnosticResultResource {
  return {
    diagnosticResultId: "diag_result_" + (overrides.status ?? "healthy") + "_" + (overrides.reasonCode ?? "healthy"),
    tenantId: "ten_diag",
    integrationId: "integration_feishu",
    domainKind: "calendar",
    providerKind: "feishu_lark",
    capability: "calendar.read",
    status: "healthy",
    reasonCode: "healthy",
    remediationOwner: "none_required",
    remediationHint: "No operator action is required.",
    retrySafety: "no_action_needed",
    checkedAt: "2026-04-30T10:00:00Z",
    staleAfter: "2026-04-30T10:15:00Z",
    freshnessState: "fresh",
    redactionStatus: "redacted",
    retentionExpiresAt: "2026-07-29T10:00:00Z",
    ...overrides
  };
}

describe("IntegrationDiagnosticsView", () => {
  it("renders healthy, blocked, limited, unsupported, stale, and redaction-failed diagnostics", () => {
    render(
      <IntegrationDiagnosticsView
        results={[
          diagnostic({ status: "healthy", reasonCode: "healthy", remediationOwner: "none_required" }),
          diagnostic({ status: "blocked", reasonCode: "scope_missing", remediationOwner: "tenant_admin", retrySafety: "blocked" }),
          diagnostic({ status: "degraded", reasonCode: "limited_diagnostic", remediationOwner: "operator", retrySafety: "no_action_needed" }),
          diagnostic({ status: "degraded", reasonCode: "blocked_route", remediationOwner: "tenant_admin", retrySafety: "blocked" }),
          diagnostic({ status: "degraded", providerKind: "matrix", capability: "matrix.reply", reasonCode: "rate_limited", remediationOwner: "provider", retrySafety: "retryable" }),
          diagnostic({ status: "unsupported", reasonCode: "unsupported_diagnostic", remediationOwner: "operator" }),
          diagnostic({ status: "blocked", reasonCode: "token_expired", remediationOwner: "product_user", freshnessState: "stale" }),
          diagnostic({ status: "unknown", reasonCode: "redaction_failed_closed", remediationOwner: "operator", redactionStatus: "failed_closed" })
        ]}
      />
    );

    expect(screen.getByText("Healthy")).toBeTruthy();
    expect(screen.getAllByText("Blocked")).toHaveLength(2);
    expect(screen.getAllByText("Limited")).toHaveLength(3);
    expect(screen.getByText("Connector route blocked")).toBeTruthy();
    expect(screen.getByText("Connector rate limited")).toBeTruthy();
    expect(screen.getByText("Unsupported")).toBeTruthy();
    expect(screen.getByText("stale")).toBeTruthy();
    expect(screen.getByText("Diagnostic detail suppressed.")).toBeTruthy();
    expect(screen.getAllByText("tenant_admin")).toHaveLength(2);
  });
});
