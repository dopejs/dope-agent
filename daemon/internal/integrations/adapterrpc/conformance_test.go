package adapterrpc_test

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterref"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterrpc"
)

// US4: the conformance harness passes the reference adapter, fails a contract-violating
// adapter, and refuses a version mismatch before any operation (FR-008, FR-009).

func TestConformancePassesReferenceAdapter(t *testing.T) {
	client, stop := adapterref.NewPipeClient()
	defer stop()
	report := adapterrpc.RunConformance(context.Background(), client)
	if !report.Passed() {
		t.Fatalf("reference adapter failed conformance: ready=%v results=%+v", report.ReadyErr, report.Results)
	}
}

func TestConformanceRefusesVersionMismatch(t *testing.T) {
	client, stop := adapterref.NewPipeClientWithOptions(adapterref.Options{ContractVer: "999"})
	defer stop()
	report := adapterrpc.RunConformance(context.Background(), client)
	if report.ReadyErr != adapterrpc.ErrContractMismatch {
		t.Fatalf("want ErrContractMismatch at readiness, got %v", report.ReadyErr)
	}
	if report.Passed() {
		t.Fatal("conformance must fail on version mismatch")
	}
}

func TestConformanceFailsContractViolatingAdapter(t *testing.T) {
	// An adapter that emits non-contract frames violates the contract.
	client, stop := adapterref.NewPipeClientWithOptions(adapterref.Options{FailMode: adapterref.FailMalformed})
	defer stop()
	report := adapterrpc.RunConformance(context.Background(), client)
	if report.Passed() {
		t.Fatal("conformance must fail for a contract-violating adapter")
	}
}
