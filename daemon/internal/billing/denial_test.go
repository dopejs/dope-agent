package billing

import (
	"testing"
	"time"
)

func TestStableQuotaDenialPayloads(t *testing.T) {
	period := QuotaPeriod{
		QuotaPeriodID: "period_1",
		TenantID:      fixtureTenantA,
		Category:      CategoryRunLaunches,
		PeriodStart:   time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:     time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	denial := NewQuotaExhaustedDenial(fixtureTenantA, CategoryRunLaunches, "op_1", 1, 0, period).Payload
	if denial.Code != "quota_denied" || denial.ReasonCode != "quota_denied:run_launches_exhausted" {
		t.Fatalf("unexpected denial: %+v", denial)
	}
	if denial.Message == "" || denial.PeriodStart == "" || denial.PeriodEnd == "" {
		t.Fatalf("denial missing stable projection fields: %+v", denial)
	}
	unavailable := NewQuotaStateUnavailableDenial(fixtureTenantA, "op_2").Payload
	if unavailable.ReasonCode != ReasonQuotaStateUnavailable || unavailable.Category != "" {
		t.Fatalf("unexpected unavailable denial: %+v", unavailable)
	}
}
