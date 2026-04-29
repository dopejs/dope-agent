package contracts_test

import (
	"os"
	"regexp"
	"sort"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
)

func TestBillingQuotaCatalogMatchesPlanningContract(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(schemaRootDir(t) + "/specs/023-billing-quotas-usage/contracts/quota-catalog.md")
	if err != nil {
		t.Fatalf("read quota catalog contract: %v", err)
	}
	matches := regexp.MustCompile("`([a-z_]+)` \\|").FindAllStringSubmatch(string(data), -1)
	contractCategories := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		contractCategories[match[1]] = struct{}{}
	}
	export := billing.ExportCatalog(time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC))
	implementationCategories := make(map[string]struct{}, len(export.Categories))
	for _, entry := range export.Categories {
		implementationCategories[string(entry.Definition.Category)] = struct{}{}
		if entry.Definition.DenialReasonCode == "" {
			t.Fatalf("category %s has empty denial reason", entry.Definition.Category)
		}
		if entry.OperationKeyShape == "" || entry.ConcurrencyGuard == "" {
			t.Fatalf("category %s has incomplete enforcement metadata: %#v", entry.Definition.Category, entry)
		}
	}
	if diff := missingKeys(contractCategories, implementationCategories); len(diff) > 0 {
		t.Fatalf("contract categories missing from implementation: %v", diff)
	}
	if diff := missingKeys(implementationCategories, contractCategories); len(diff) > 0 {
		t.Fatalf("implementation categories missing from contract: %v", diff)
	}
}

func missingKeys(want, got map[string]struct{}) []string {
	var out []string
	for key := range want {
		if _, ok := got[key]; !ok {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}
