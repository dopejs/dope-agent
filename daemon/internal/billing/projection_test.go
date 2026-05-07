package billing

import (
	"context"
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

func TestBuildQuotaStatusItemClassifiesNearLimitForPercentAndTypicalOperation(t *testing.T) {
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	definition, _ := DefinitionFor(CategoryRunLaunches)
	start, end := PeriodFor(definition.PeriodKind, now)
	period := QuotaPeriod{TenantID: fixtureTenantA, Category: definition.Category, PeriodStart: start, PeriodEnd: end}
	limit := int64(10)
	override := &QuotaOverride{TenantID: fixtureTenantA, Category: definition.Category, Limit: &limit}

	item := BuildQuotaStatusItem(
		TenantPlan{TenantID: fixtureTenantA, PlanKey: "finite", EnforcementMode: EnforcementModeEnforced},
		definition,
		period,
		UsageCounter{TenantID: fixtureTenantA, Category: definition.Category, CommittedAmount: 8},
		nil,
		override,
		nil,
	)
	if item.Status != QuotaStatusNearLimit || !item.NearLimit || item.NearLimitReason != NearLimitReasonPercentThreshold {
		t.Fatalf("expected percent near-limit status, got %+v", item)
	}
	if item.TypicalOperationAmount != 1 {
		t.Fatalf("expected count quota typical operation amount 1, got %d", item.TypicalOperationAmount)
	}

	byteDefinition := QuotaDefinition{
		Category:         CategoryArtifactStorageBytes,
		Unit:             UnitBytes,
		PeriodKind:       PeriodMonthly,
		DefaultLimit:     10_000,
		DenialReasonCode: "quota_denied:artifact_storage_bytes_exhausted",
		Document:         map[string]any{"artifactWriteReservationEstimateBytes": int64(4096)},
	}
	byteStart, byteEnd := PeriodFor(byteDefinition.PeriodKind, now)
	byteItem := BuildQuotaStatusItem(
		TenantPlan{TenantID: fixtureTenantA, PlanKey: "finite", EnforcementMode: EnforcementModeEnforced},
		byteDefinition,
		QuotaPeriod{TenantID: fixtureTenantA, Category: byteDefinition.Category, PeriodStart: byteStart, PeriodEnd: byteEnd},
		UsageCounter{TenantID: fixtureTenantA, Category: byteDefinition.Category, CommittedAmount: 6_000},
		nil,
		nil,
		nil,
	)
	if byteItem.Status != QuotaStatusNearLimit || byteItem.NearLimitReason != NearLimitReasonBelowOneTypicalOperation {
		t.Fatalf("expected byte quota below-one-operation near-limit status, got %+v", byteItem)
	}
	if byteItem.TypicalOperationAmount != 4096 {
		t.Fatalf("expected byte quota typical operation amount from catalog document, got %d", byteItem.TypicalOperationAmount)
	}
}

func TestGroupQuotaStatusItemsIncludesAllRequiredCategories(t *testing.T) {
	items := make([]QuotaStatusItem, 0, len(RequiredCategories()))
	for _, category := range RequiredCategories() {
		items = append(items, QuotaStatusItem{Category: category})
	}
	sections := GroupQuotaStatusItems(items)
	seen := map[Category]bool{}
	for _, section := range sections {
		if section.SectionKey == "" || section.Label == "" {
			t.Fatalf("expected readable section metadata, got %+v", section)
		}
		for _, item := range section.Items {
			if seen[item.Category] {
				t.Fatalf("category %s appeared in multiple sections", item.Category)
			}
			seen[item.Category] = true
		}
	}
	for _, category := range RequiredCategories() {
		if !seen[category] {
			t.Fatalf("missing category %s from grouped sections %+v", category, sections)
		}
	}
}

func TestBuildQuotaStatusItemShowsOverrideAndRestrictionSeparately(t *testing.T) {
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	definition, _ := DefinitionFor(CategoryRunLaunches)
	start, end := PeriodFor(definition.PeriodKind, now)
	limit := int64(3)
	restriction := &AbuseRestrictionSummary{
		RestrictionID:         "restriction_1",
		Status:                AbuseRestrictionStatusActive,
		AffectedCategory:      CategoryRunLaunches,
		RecoveryAction:        RecoveryActionContactSupport,
		VisibleReasonCode:     "abuse_restriction:temporary",
		SourceAuditRef:        "audit_1",
		SupportContactAllowed: true,
	}
	item := BuildQuotaStatusItem(
		TenantPlan{TenantID: fixtureTenantA, PlanKey: "finite", EnforcementMode: EnforcementModeEnforced},
		definition,
		QuotaPeriod{TenantID: fixtureTenantA, Category: definition.Category, PeriodStart: start, PeriodEnd: end},
		UsageCounter{TenantID: fixtureTenantA, Category: definition.Category, CommittedAmount: 1},
		nil,
		&QuotaOverride{TenantID: fixtureTenantA, Category: definition.Category, Limit: &limit, Reason: "support override", EffectiveAt: now},
		restriction,
	)
	if item.Override == nil || item.Override.BaseLimit != definition.DefaultLimit || item.Override.EffectiveLimit != limit {
		t.Fatalf("expected visible base/effective override summary, got %+v", item)
	}
	if item.Restriction == nil || item.Status != QuotaStatusRestricted {
		t.Fatalf("expected restriction to be separate from override and drive restricted status, got %+v", item)
	}
}

func TestQuotaDashboardProjectsCurrentAndImmediatelyPreviousCompletedPeriod(t *testing.T) {
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	definition, _ := DefinitionFor(CategoryRunLaunches)
	currentStart, currentEnd := PeriodFor(definition.PeriodKind, now)
	previousStart := currentStart.AddDate(0, -1, 0)
	previousEnd := currentStart
	repo := &projectionTestRepo{
		plan: TenantPlan{
			TenantID:        fixtureTenantA,
			PlanKey:         "finite",
			Status:          PlanStatusActive,
			EnforcementMode: EnforcementModeEnforced,
			EffectiveAt:     now.Add(-time.Hour),
		},
		periods: map[Category]QuotaPeriod{
			CategoryRunLaunches: {QuotaPeriodID: "period_current", TenantID: fixtureTenantA, Category: CategoryRunLaunches, PeriodKind: definition.PeriodKind, PeriodStart: currentStart, PeriodEnd: currentEnd, Status: "open"},
		},
		counters: map[string]UsageCounter{
			"period_current": {TenantID: fixtureTenantA, Category: CategoryRunLaunches, QuotaPeriodID: "period_current", CommittedAmount: 4},
		},
		previousPeriod:  QuotaPeriod{QuotaPeriodID: "period_previous", TenantID: fixtureTenantA, Category: CategoryRunLaunches, PeriodKind: definition.PeriodKind, PeriodStart: previousStart, PeriodEnd: previousEnd, Status: "closed"},
		previousCounter: UsageCounter{TenantID: fixtureTenantA, Category: CategoryRunLaunches, QuotaPeriodID: "period_previous", CommittedAmount: 7},
	}
	manager := NewManagerWithClock(repo, func() time.Time { return now })
	dashboard, err := manager.QuotaDashboard(context.Background(), fixtureTenantA, true)
	if err != nil {
		t.Fatalf("QuotaDashboard returned error: %v", err)
	}
	for _, section := range dashboard.Sections {
		for _, item := range section.Items {
			if item.Category != CategoryRunLaunches {
				continue
			}
			if item.CurrentPeriod.ConsumedAmount != 4 {
				t.Fatalf("unexpected current period: %+v", item.CurrentPeriod)
			}
			if item.PreviousPeriod == nil || item.PreviousPeriod.ConsumedAmount != 7 || !item.PreviousPeriod.PeriodEnd.Equal(previousEnd) {
				t.Fatalf("unexpected previous period: %+v", item.PreviousPeriod)
			}
			return
		}
	}
	t.Fatal("run launch dashboard item not found")
}

func TestRecoveryActionsForExhaustedQuotaIncludeOverrideRequest(t *testing.T) {
	actions := RecoveryActionsForQuotaStatus(QuotaStatusExhausted, NearLimitReasonNone)
	for _, action := range actions {
		if action == RecoveryAction("request_override") {
			return
		}
	}
	t.Fatalf("expected exhausted quota recovery actions to include request_override, got %+v", actions)
}

type projectionTestRepo struct {
	plan            TenantPlan
	periods         map[Category]QuotaPeriod
	counters        map[string]UsageCounter
	previousPeriod  QuotaPeriod
	previousCounter UsageCounter
}

func (r *projectionTestRepo) ActivePlan(context.Context, string) (TenantPlan, bool, error) {
	return r.plan, true, nil
}

func (r *projectionTestRepo) QuotaOverride(context.Context, string, Category, time.Time) (*QuotaOverride, error) {
	return nil, nil
}

func (r *projectionTestRepo) OpenPeriod(_ context.Context, tenantID string, definition QuotaDefinition, at time.Time) (QuotaPeriod, error) {
	if period, ok := r.periods[definition.Category]; ok {
		return period, nil
	}
	start, end := PeriodFor(definition.PeriodKind, at)
	return QuotaPeriod{QuotaPeriodID: "period_" + string(definition.Category), TenantID: tenantID, Category: definition.Category, PeriodKind: definition.PeriodKind, PeriodStart: start, PeriodEnd: end, Status: "open"}, nil
}

func (r *projectionTestRepo) UsageCounter(_ context.Context, tenantID string, category Category, quotaPeriodID string) (UsageCounter, bool, error) {
	if counter, ok := r.counters[quotaPeriodID]; ok {
		return counter, true, nil
	}
	return UsageCounter{TenantID: tenantID, Category: category, QuotaPeriodID: quotaPeriodID}, false, nil
}

func (r *projectionTestRepo) PreviousQuotaPeriod(_ context.Context, _ string, category Category, _ time.Time) (QuotaPeriod, UsageCounter, bool, error) {
	if category != r.previousPeriod.Category {
		return QuotaPeriod{}, UsageCounter{}, false, nil
	}
	return r.previousPeriod, r.previousCounter, true, nil
}

func (r *projectionTestRepo) SaveUsageCounter(context.Context, UsageCounter) error { return nil }
func (r *projectionTestRepo) ReservationByOperation(context.Context, string, Category, string) (UsageReservation, bool, error) {
	return UsageReservation{}, false, nil
}
func (r *projectionTestRepo) SaveReservation(context.Context, UsageReservation) error { return nil }
func (r *projectionTestRepo) AppendUsageEvent(context.Context, UsageEvent) error      { return nil }
func (r *projectionTestRepo) AppendQuotaDenial(context.Context, QuotaDenial) error    { return nil }
func (r *projectionTestRepo) ListPendingReservations(context.Context) ([]UsageReservation, error) {
	return nil, nil
}
