package contracts_test

import (
	"os"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/api"
	"github.com/dopejs/dope-agent/daemon/internal/billing"
)

func TestBillingEnforcementMatrixCoversRequiredCategoriesAndContractRows(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(schemaRootDir(t) + "/specs/023-billing-quotas-usage/contracts/enforcement-matrix.md")
	if err != nil {
		t.Fatalf("read enforcement matrix contract: %v", err)
	}
	matrix := api.BillingEnforcementMatrix()
	if len(matrix) == 0 {
		t.Fatal("expected registered billing enforcement matrix entries")
	}
	coveredCategories := map[billing.Category]struct{}{}
	for _, entry := range matrix {
		if entry.EntryPoint == "" || entry.OperationKeySource == "" || entry.CommitRefundTransition == "" {
			t.Fatalf("incomplete enforcement matrix entry: %#v", entry)
		}
		if len(entry.RequiredVerificationSet) < 4 {
			t.Fatalf("entry %q has insufficient verification coverage: %#v", entry.EntryPoint, entry.RequiredVerificationSet)
		}
		for _, category := range entry.Categories {
			coveredCategories[category] = struct{}{}
		}
		if !strings.Contains(string(data), entry.EntryPoint) {
			t.Fatalf("implementation matrix entry %q is not represented in contract", entry.EntryPoint)
		}
	}
	for _, category := range billing.RequiredCategories() {
		if _, ok := coveredCategories[category]; !ok {
			t.Fatalf("required category %q is not covered by enforcement matrix", category)
		}
	}
}

func TestBillingEnforcementMatrixNamesKnownExpensiveHostedEntryPoints(t *testing.T) {
	t.Parallel()

	serverData, err := os.ReadFile(schemaRootDir(t) + "/daemon/internal/api/server.go")
	if err != nil {
		t.Fatalf("read api server: %v", err)
	}
	calendarData, err := os.ReadFile(schemaRootDir(t) + "/daemon/internal/api/calendar.go")
	if err != nil {
		t.Fatalf("read calendar api: %v", err)
	}
	mailData, err := os.ReadFile(schemaRootDir(t) + "/daemon/internal/api/mail_execution.go")
	if err != nil {
		t.Fatalf("read mail execution api: %v", err)
	}
	evaluationData, err := os.ReadFile(schemaRootDir(t) + "/daemon/internal/api/evaluation.go")
	if err != nil {
		t.Fatalf("read evaluation api: %v", err)
	}
	matrix := api.BillingEnforcementMatrix()
	matrixText := ""
	for _, entry := range matrix {
		matrixText += entry.EntryPoint + "\n"
	}
	checks := []struct {
		name         string
		source       string
		sourceNeedle string
		matrixNeedle string
	}{
		{name: "run launch", source: string(serverData), sourceNeedle: "handleRuns", matrixNeedle: "POST /v1/runs"},
		{name: "workflow launch", source: string(serverData), sourceNeedle: "handleRunWorkflows", matrixNeedle: "POST /v1/runs/{runId}/workflows"},
		{name: "workflow start", source: string(serverData), sourceNeedle: "handleRunWorkflowStart", matrixNeedle: "POST /v1/runs/{runId}/workflows/{workflowId}/start"},
		{name: "tool calls", source: string(serverData), sourceNeedle: "handleRunStepToolCalls", matrixNeedle: "POST /v1/runs/{runId}/steps/{stepId}/tool-calls"},
		{name: "calendar operations", source: string(calendarData), sourceNeedle: "recordCalendarActivity", matrixNeedle: "Calendar/mail/integration operation routes"},
		{name: "mail operations", source: string(mailData), sourceNeedle: "executeMailAction", matrixNeedle: "Calendar/mail/integration operation routes"},
		{name: "evaluation replay", source: string(evaluationData), sourceNeedle: "CreateReplayAttempt", matrixNeedle: "POST /v1/evaluation/replay-candidates/{candidateId}/attempts"},
	}
	for _, check := range checks {
		if !strings.Contains(check.source, check.sourceNeedle) {
			t.Fatalf("expected source signature for %s containing %q", check.name, check.sourceNeedle)
		}
		if !strings.Contains(matrixText, check.matrixNeedle) {
			t.Fatalf("expected enforcement matrix entry for %s containing %q", check.name, check.matrixNeedle)
		}
	}
}
