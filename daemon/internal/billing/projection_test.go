package billing

import (
	"testing"
	"time"
)

func TestPeriodForUsesUTCBoundaries(t *testing.T) {
	local := time.FixedZone("tenant", 8*60*60)
	now := time.Date(2026, 4, 28, 23, 30, 0, 0, local)
	start, end := PeriodFor(PeriodDaily, now)
	if start.Location() != time.UTC || end.Location() != time.UTC {
		t.Fatalf("period must use UTC: %s %s", start.Location(), end.Location())
	}
	if start.Hour() != 0 || start.Minute() != 0 || end.Sub(start) != 24*time.Hour {
		t.Fatalf("unexpected daily boundary %s - %s", start, end)
	}
}

func TestProjectQuotaShowsUnlimitedAndOverLimitStates(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	definition, _ := DefinitionFor(CategoryRunLaunches)
	start, end := PeriodFor(definition.PeriodKind, now)
	period := QuotaPeriod{TenantID: fixtureTenantA, Category: definition.Category, PeriodStart: start, PeriodEnd: end}
	counter := UsageCounter{TenantID: fixtureTenantA, Category: definition.Category, CommittedAmount: 11}
	limit := int64(10)
	override := &QuotaOverride{TenantID: fixtureTenantA, Category: definition.Category, Limit: &limit}
	quota := ProjectQuota(TenantPlan{TenantID: fixtureTenantA, PlanKey: "finite", EnforcementMode: EnforcementModeEnforced}, definition, period, counter, override)
	if !quota.OverLimit || quota.RemainingAmount != -1 || quota.DenialReasonCode == "" {
		t.Fatalf("expected over-limit finite projection, got %+v", quota)
	}
	unlimited := ProjectQuota(DevelopmentPlan("ten_dev", now), definition, period, counter, nil)
	if unlimited.EnforcementMode != EnforcementModeUnlimited || unlimited.OverLimit {
		t.Fatalf("expected explicit unlimited projection, got %+v", unlimited)
	}
}

func TestProjectQuotaAccountsForCarryover(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	definition, _ := DefinitionFor(CategoryRunLaunches)
	start, end := PeriodFor(definition.PeriodKind, now)
	period := QuotaPeriod{TenantID: fixtureTenantA, Category: definition.Category, PeriodStart: start, PeriodEnd: end}
	counter := UsageCounter{
		TenantID:        fixtureTenantA,
		Category:        definition.Category,
		CommittedAmount: 5,
		ReservedAmount:  2,
		AdjustedAmount:  -1,
		CarryoverAmount: 3,
	}
	limit := int64(10)
	override := &QuotaOverride{TenantID: fixtureTenantA, Category: definition.Category, Limit: &limit}
	quota := ProjectQuota(TenantPlan{TenantID: fixtureTenantA, PlanKey: "finite", EnforcementMode: EnforcementModeEnforced}, definition, period, counter, override)
	if quota.CarryoverApplied != 3 || quota.RemainingAmount != 7 {
		t.Fatalf("expected carryover to increase remaining amount, got %+v", quota)
	}
}
