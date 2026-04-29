package contracts_test

import (
	"os"
	"strings"
	"testing"
)

func TestBillingQuickstartSmokeChecklistCoversRequiredEvidence(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(schemaRootDir(t) + "/specs/023-billing-quotas-usage/quickstart.md")
	if err != nil {
		t.Fatalf("read billing quickstart: %v", err)
	}
	quickstart := string(data)
	if strings.Contains(quickstart, "manual smoke remains open") || strings.Contains(quickstart, "16-step manual smoke remains open") {
		t.Fatal("quickstart still records the billing manual smoke as open")
	}
	for _, required := range []string{
		"Smoke Evidence Coverage",
		"Confirm denial happens",
		"same operation identity",
		"refund the difference",
		"operator-action-needed",
		"Lower tenant A quota below current usage",
		"Apply a manual adjustment with a reason",
		"plan change, denial, reservation, commit",
	} {
		if !strings.Contains(quickstart, required) {
			t.Fatalf("quickstart smoke checklist missing %q", required)
		}
	}
}
