package contracts_test

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/opsreadiness"
)

func TestHostedObservabilityEvidenceBlocksReleaseOnMissingUnsupportedOrRedactionFailure(t *testing.T) {
	run := hostedContractRun()
	report := opsreadiness.HostedObservationReport{
		ObservationReportID:        "obs_contract_1",
		RunID:                      run.RunID,
		SampleWindow:               "2026-04-30T11:00:00Z/2026-04-30T12:00:00Z",
		DaemonHealth:               opsreadiness.StatusPass,
		DatabaseSize:               opsreadiness.HostedObservation{Value: "10MiB"},
		LogSize:                    opsreadiness.HostedObservation{Value: "1MiB"},
		Memory:                     opsreadiness.HostedObservation{Value: "100MiB"},
		Goroutines:                 opsreadiness.HostedObservation{Value: "12"},
		FileDescriptors:            opsreadiness.HostedObservation{Unsupported: true},
		QueueOrBacklog:             opsreadiness.HostedObservation{Value: "0"},
		ConnectorHealth:            opsreadiness.HostedObservation{Value: opsreadiness.StatusPass},
		MCPHealth:                  opsreadiness.HostedObservation{Value: opsreadiness.StatusPass},
		IntegrationDiagnosticState: opsreadiness.HostedObservation{Value: opsreadiness.StatusPass},
		UnsupportedFields:          []string{"fileDescriptors"},
		GeneratedAt:                hostedContractNow,
	}
	if err := opsreadiness.ValidateHostedObservationReport(run, report); err != nil {
		t.Fatalf("passing observability report invalid: %v", err)
	}
	report.UnsupportedFields = nil
	if err := opsreadiness.ValidateHostedObservationReport(run, report); err == nil {
		t.Fatalf("expected unsupported observation without marker to fail")
	}
	report = opsreadiness.HostedObservationReport{ObservationReportID: "obs_contract_secret", RunID: run.RunID, SampleWindow: "w", DaemonHealth: opsreadiness.StatusPass, DatabaseSize: opsreadiness.HostedObservation{Value: "access_token"}, GeneratedAt: hostedContractNow}
	if err := opsreadiness.ValidateHostedObservationReport(run, report); err == nil {
		t.Fatalf("expected raw credential material to fail observability evidence")
	}
}
