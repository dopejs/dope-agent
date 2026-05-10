import type { IntegrationDiagnosticResultResource, SmokeMatrixReportResource } from "@dope/client";

type IntegrationDiagnosticsViewProps = {
  results: IntegrationDiagnosticResultResource[];
  smokeReports?: SmokeMatrixReportResource[];
};

const STATUS_LABELS: Record<IntegrationDiagnosticResultResource["status"], string> = {
  unknown: "Unknown",
  healthy: "Healthy",
  degraded: "Limited",
  blocked: "Blocked",
  unsupported: "Unsupported"
};

const CONNECTOR_REASON_LABELS: Partial<Record<IntegrationDiagnosticResultResource["reasonCode"], string>> = {
  auth_missing: "Connector auth missing",
  permission_missing: "Connector permission missing",
  blocked_route: "Connector route blocked",
  duplicate_inbound: "Connector duplicate inbound",
  reply_failed: "Connector reply failed",
  unsupported_capability: "Connector capability unsupported",
  rate_limited: "Connector rate limited",
  provider_unavailable: "Connector provider unavailable",
  network_failed: "Connector network failed",
  unknown_connector_failure: "Connector failure"
};

export function IntegrationDiagnosticsView({ results, smokeReports = [] }: IntegrationDiagnosticsViewProps) {
  if (!results.length && !smokeReports.length) {
    return <div className="empty-state">No integration diagnostics available.</div>;
  }

  return (
    <section className="integration-diagnostics" aria-label="Integration diagnostics">
      {smokeReports.map((report) => (
        <article className={`diagnostic-card diagnostic-${report.status}`} key={report.smokeReportId}>
          <div className="diagnostic-card__header">
            <div>
              <h3>Smoke matrix</h3>
              <p>{report.smokeReportId}</p>
            </div>
            <span className={`status-chip status-${report.status}`}>{report.status}</span>
          </div>
          <dl className="diagnostic-card__facts">
            {Object.entries(report.domainSummary).map(([domain, result]) => (
              <div key={domain}>
                <dt>{domain}</dt>
                <dd>{result}</dd>
              </div>
            ))}
          </dl>
        </article>
      ))}
      {results.map((result) => (
        <article className={`diagnostic-card diagnostic-${result.status}`} key={result.diagnosticResultId}>
          <div className="diagnostic-card__header">
            <div>
              <h3>{result.capability}</h3>
              <p>{result.domainKind} · {result.providerKind}</p>
            </div>
            <span className={`status-chip status-${result.status}`}>{STATUS_LABELS[result.status]}</span>
          </div>
          <dl className="diagnostic-card__facts">
            <div>
              <dt>Reason</dt>
              <dd>{CONNECTOR_REASON_LABELS[result.reasonCode] ?? result.reasonCode}</dd>
            </div>
            <div>
              <dt>Owner</dt>
              <dd>{result.remediationOwner}</dd>
            </div>
            <div>
              <dt>Retry</dt>
              <dd>{result.retrySafety}</dd>
            </div>
            <div>
              <dt>Freshness</dt>
              <dd>{result.freshnessState}</dd>
            </div>
          </dl>
          {result.redactionStatus === "failed_closed" ? (
            <p className="diagnostic-warning">Diagnostic detail suppressed.</p>
          ) : (
            <p>{result.remediationHint || result.evidenceSummary || "No diagnostic detail."}</p>
          )}
        </article>
      ))}
    </section>
  );
}
