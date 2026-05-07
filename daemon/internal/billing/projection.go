package billing

import (
	"context"
	"sort"
	"time"
)

type EffectiveQuota struct {
	TenantID         string          `json:"tenantId"`
	PlanKey          string          `json:"planKey"`
	Category         Category        `json:"category"`
	Unit             Unit            `json:"unit"`
	PeriodStart      time.Time       `json:"periodStart"`
	PeriodEnd        time.Time       `json:"periodEnd"`
	PeriodAnchor     string          `json:"periodAnchor"`
	Limit            int64           `json:"limit"`
	ConsumedAmount   int64           `json:"consumedAmount"`
	ReservedAmount   int64           `json:"reservedAmount"`
	AdjustedAmount   int64           `json:"adjustedAmount"`
	CarryoverApplied int64           `json:"carryoverApplied"`
	RemainingAmount  int64           `json:"remainingAmount"`
	EnforcementMode  EnforcementMode `json:"enforcementMode"`
	DenialReasonCode string          `json:"denialReasonCode,omitempty"`
	OverLimit        bool            `json:"overLimit,omitempty"`
}

type UsageSummary struct {
	TenantID          string             `json:"tenantId"`
	PlanKey           string             `json:"planKey"`
	EnforcementMode   EnforcementMode    `json:"enforcementMode"`
	Quotas            []EffectiveQuota   `json:"quotas"`
	ManualAdjustments []ManualAdjustment `json:"manualAdjustments,omitempty"`
	Denials           []QuotaDenial      `json:"denials,omitempty"`
}

type ProjectionRepository interface {
	Repository
	ListQuotaDenials(ctx context.Context, tenantID string, limit int) ([]QuotaDenial, error)
	ListManualAdjustments(ctx context.Context, tenantID string, limit int) ([]ManualAdjustment, error)
}

type PreviousPeriodRepository interface {
	PreviousQuotaPeriod(ctx context.Context, tenantID string, category Category, before time.Time) (QuotaPeriod, UsageCounter, bool, error)
}

type AbuseRestrictionRepository interface {
	ListAbuseRestrictions(ctx context.Context, tenantID string, at time.Time) ([]AbuseRestrictionRecord, error)
}

func (m *Manager) ActivePlan(ctx context.Context, tenantID string, hosted bool) (TenantPlan, error) {
	now := time.Now().UTC()
	if m != nil && m.now != nil {
		now = m.now().UTC()
	}
	if m == nil || m.repo == nil {
		if hosted {
			return TenantPlan{}, ErrQuotaStateUnavailable
		}
		return DevelopmentPlan(tenantID, now), nil
	}
	plan, found, err := m.repo.ActivePlan(ctx, tenantID)
	if err != nil {
		return TenantPlan{}, err
	}
	if found {
		return plan, nil
	}
	if hosted {
		return TenantPlan{}, ErrQuotaStateUnavailable
	}
	return DevelopmentPlan(tenantID, now), nil
}

func (m *Manager) UsageSummary(ctx context.Context, tenantID string, hosted bool) (UsageSummary, error) {
	plan, err := m.ActivePlan(ctx, tenantID, hosted)
	if err != nil {
		return UsageSummary{}, err
	}
	now := time.Now().UTC()
	if m != nil && m.now != nil {
		now = m.now().UTC()
	}
	summary := UsageSummary{
		TenantID:        tenantID,
		PlanKey:         plan.PlanKey,
		EnforcementMode: plan.EnforcementMode,
		Quotas:          make([]EffectiveQuota, 0, len(RequiredCategories())),
	}
	for _, definition := range InitialDefinitions(now) {
		start, end := PeriodFor(definition.PeriodKind, now)
		period := QuotaPeriod{
			QuotaPeriodID: "quota_period_" + tenantID + "_" + string(definition.Category) + "_" + start.Format("20060102"),
			TenantID:      tenantID,
			Category:      definition.Category,
			PeriodKind:    definition.PeriodKind,
			PeriodStart:   start,
			PeriodEnd:     end,
			Status:        "open",
		}
		var counter UsageCounter
		found := false
		if m != nil && m.repo != nil {
			var err error
			period, err = m.repo.OpenPeriod(ctx, tenantID, definition, now)
			if err != nil {
				return UsageSummary{}, err
			}
			counter, found, err = m.repo.UsageCounter(ctx, tenantID, definition.Category, period.QuotaPeriodID)
			if err != nil {
				return UsageSummary{}, err
			}
		}
		if !found {
			counter = UsageCounter{
				TenantID:      tenantID,
				Category:      definition.Category,
				QuotaPeriodID: period.QuotaPeriodID,
				UpdatedAt:     now,
			}
		}
		var override *QuotaOverride
		if m != nil && m.repo != nil {
			override, err = m.repo.QuotaOverride(ctx, tenantID, definition.Category, now)
			if err != nil {
				return UsageSummary{}, err
			}
		}
		summary.Quotas = append(summary.Quotas, ProjectQuota(plan, definition, period, counter, override))
	}
	if m != nil && m.repo != nil {
		repo, ok := m.repo.(ProjectionRepository)
		if !ok {
			return summary, nil
		}
		summary.Denials, err = repo.ListQuotaDenials(ctx, tenantID, 100)
		if err != nil {
			return UsageSummary{}, err
		}
		summary.ManualAdjustments, err = repo.ListManualAdjustments(ctx, tenantID, 100)
		if err != nil {
			return UsageSummary{}, err
		}
	}
	return summary, nil
}

func (m *Manager) QuotaDashboard(ctx context.Context, tenantID string, hosted bool) (TenantQuotaDashboard, error) {
	plan, err := m.ActivePlan(ctx, tenantID, hosted)
	if err != nil {
		return TenantQuotaDashboard{}, err
	}
	now := time.Now().UTC()
	if m != nil && m.now != nil {
		now = m.now().UTC()
	}
	items := make([]QuotaStatusItem, 0, len(RequiredCategories()))
	restrictionsByCategory := map[Category]*AbuseRestrictionSummary{}
	if m != nil && m.repo != nil {
		if restrictionRepo, ok := m.repo.(AbuseRestrictionRepository); ok {
			restrictions, err := restrictionRepo.ListAbuseRestrictions(ctx, tenantID, now)
			if err != nil {
				return TenantQuotaDashboard{}, err
			}
			for _, record := range restrictions {
				summary := record.Summary()
				restrictionsByCategory[record.AffectedCategory] = &summary
			}
		}
	}
	for _, definition := range InitialDefinitions(now) {
		start, end := PeriodFor(definition.PeriodKind, now)
		period := QuotaPeriod{
			QuotaPeriodID: "quota_period_" + tenantID + "_" + string(definition.Category) + "_" + start.Format("20060102"),
			TenantID:      tenantID,
			Category:      definition.Category,
			PeriodKind:    definition.PeriodKind,
			PeriodStart:   start,
			PeriodEnd:     end,
			Status:        "open",
		}
		var counter UsageCounter
		found := false
		if m != nil && m.repo != nil {
			period, err = m.repo.OpenPeriod(ctx, tenantID, definition, now)
			if err != nil {
				return TenantQuotaDashboard{}, err
			}
			counter, found, err = m.repo.UsageCounter(ctx, tenantID, definition.Category, period.QuotaPeriodID)
			if err != nil {
				return TenantQuotaDashboard{}, err
			}
		}
		if !found {
			counter = UsageCounter{
				TenantID:      tenantID,
				Category:      definition.Category,
				QuotaPeriodID: period.QuotaPeriodID,
				UpdatedAt:     now,
			}
		}
		var previous *UsagePeriodSummary
		if m != nil && m.repo != nil {
			if previousRepo, ok := m.repo.(PreviousPeriodRepository); ok {
				previousPeriod, previousCounter, previousFound, err := previousRepo.PreviousQuotaPeriod(ctx, tenantID, definition.Category, period.PeriodStart)
				if err != nil {
					return TenantQuotaDashboard{}, err
				}
				if previousFound {
					previousQuota := ProjectQuota(plan, definition, previousPeriod, previousCounter, nil)
					previous = &UsagePeriodSummary{
						PeriodStart:      previousQuota.PeriodStart,
						PeriodEnd:        previousQuota.PeriodEnd,
						PeriodAnchor:     previousQuota.PeriodAnchor,
						ConsumedAmount:   previousQuota.ConsumedAmount,
						ReservedAmount:   previousQuota.ReservedAmount,
						AdjustedAmount:   previousQuota.AdjustedAmount,
						CarryoverApplied: previousQuota.CarryoverApplied,
						RemainingAmount:  previousQuota.RemainingAmount,
						OverLimit:        previousQuota.OverLimit,
					}
				}
			}
		}
		var override *QuotaOverride
		if m != nil && m.repo != nil {
			override, err = m.repo.QuotaOverride(ctx, tenantID, definition.Category, now)
			if err != nil {
				return TenantQuotaDashboard{}, err
			}
		}
		items = append(items, BuildQuotaStatusItem(plan, definition, period, counter, previous, override, restrictionsByCategory[definition.Category]))
	}
	planLabel := plan.PlanKey
	if planLabel == "" {
		planLabel = string(plan.EnforcementMode)
	}
	return TenantQuotaDashboard{
		TenantID: tenantID,
		Plan: PlanSummary{
			PlanKey:           plan.PlanKey,
			EnforcementMode:   plan.EnforcementMode,
			Status:            plan.Status,
			EffectiveAt:       plan.EffectiveAt,
			BasePlanLabel:     planLabel,
			CheckoutAvailable: false,
		},
		Sections:    GroupQuotaStatusItems(items),
		GeneratedAt: now,
		Permission:  map[string]any{"allowed": true},
	}, nil
}

func ProjectQuota(plan TenantPlan, definition QuotaDefinition, period QuotaPeriod, counter UsageCounter, override *QuotaOverride) EffectiveQuota {
	limit := definition.DefaultLimit
	if override != nil && override.Limit != nil {
		limit = *override.Limit
	}
	mode := plan.EnforcementMode
	if mode == "" {
		mode = EnforcementModeEnforced
	}
	effectiveUsage := counter.CommittedAmount + counter.ReservedAmount + counter.AdjustedAmount - counter.CarryoverAmount
	remaining := limit - effectiveUsage
	if mode == EnforcementModeUnlimited {
		remaining = 0
	}
	out := EffectiveQuota{
		TenantID:         plan.TenantID,
		PlanKey:          plan.PlanKey,
		Category:         definition.Category,
		Unit:             definition.Unit,
		PeriodStart:      period.PeriodStart,
		PeriodEnd:        period.PeriodEnd,
		PeriodAnchor:     PeriodAnchorUTC,
		Limit:            limit,
		ConsumedAmount:   counter.CommittedAmount,
		ReservedAmount:   counter.ReservedAmount,
		AdjustedAmount:   counter.AdjustedAmount,
		CarryoverApplied: counter.CarryoverAmount,
		RemainingAmount:  remaining,
		EnforcementMode:  mode,
	}
	if mode == EnforcementModeEnforced && remaining < 0 {
		out.OverLimit = true
		out.DenialReasonCode = definition.DenialReasonCode
	}
	return out
}

func BuildQuotaStatusItem(plan TenantPlan, definition QuotaDefinition, period QuotaPeriod, counter UsageCounter, previous *UsagePeriodSummary, override *QuotaOverride, restriction *AbuseRestrictionSummary) QuotaStatusItem {
	quota := ProjectQuota(plan, definition, period, counter, override)
	baseLimit := definition.DefaultLimit
	effectiveLimit := quota.Limit
	typicalOperationAmount := CategoryDefinedTypicalOperationAmount(definition)
	current := UsagePeriodSummary{
		PeriodStart:      quota.PeriodStart,
		PeriodEnd:        quota.PeriodEnd,
		PeriodAnchor:     quota.PeriodAnchor,
		ConsumedAmount:   quota.ConsumedAmount,
		ReservedAmount:   quota.ReservedAmount,
		AdjustedAmount:   quota.AdjustedAmount,
		CarryoverApplied: quota.CarryoverApplied,
		RemainingAmount:  quota.RemainingAmount,
		OverLimit:        quota.OverLimit,
	}
	item := QuotaStatusItem{
		Category:               definition.Category,
		Unit:                   definition.Unit,
		Status:                 QuotaStatusAvailable,
		CurrentPeriod:          current,
		PreviousPeriod:         previous,
		Limit:                  effectiveLimit,
		RemainingAmount:        quota.RemainingAmount,
		TypicalOperationAmount: typicalOperationAmount,
		BaseLimit:              baseLimit,
		EffectiveLimit:         effectiveLimit,
	}
	if override != nil {
		item.Override = &QuotaOverrideSummary{
			BaseLimit:      baseLimit,
			EffectiveLimit: effectiveLimit,
			Reason:         override.Reason,
			EffectiveAt:    override.EffectiveAt,
			ExpiresAt:      override.ExpiresAt,
		}
	}
	mode := plan.EnforcementMode
	if mode == "" {
		mode = EnforcementModeEnforced
	}
	switch {
	case restriction != nil && restriction.Status == AbuseRestrictionStatusActive:
		item.Status = QuotaStatusRestricted
		item.Restriction = restriction
	case mode == EnforcementModeUnlimited:
		item.Status = QuotaStatusUnlimited
	case mode == EnforcementModeNotMeasurable:
		item.Status = QuotaStatusNotMeasurable
	case effectiveLimit <= 0 && definition.Unit != UnitBytes:
		item.Status = QuotaStatusExhausted
	case quota.RemainingAmount <= 0:
		item.Status = QuotaStatusExhausted
	case IsQuotaNearLimit(quota, typicalOperationAmount):
		item.Status = QuotaStatusNearLimit
		item.NearLimit = true
		item.NearLimitReason = NearLimitReasonForQuota(quota, typicalOperationAmount)
	}
	item.RecoveryActions = RecoveryActionsForQuotaStatus(item.Status, item.NearLimitReason)
	return item
}

func CategoryDefinedTypicalOperationAmount(definition QuotaDefinition) int64 {
	if definition.Unit == UnitBytes {
		if amount := int64FromDocument(definition.Document, "artifactWriteReservationEstimateBytes"); amount > 0 {
			return amount
		}
		if amount := int64FromDocument(definition.Document, "typicalOperationAmount"); amount > 0 {
			return amount
		}
		return 1
	}
	return 1
}

func int64FromDocument(document map[string]any, key string) int64 {
	if document == nil {
		return 0
	}
	switch value := document[key].(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case int32:
		return int64(value)
	case float64:
		return int64(value)
	case float32:
		return int64(value)
	case uint64:
		if value <= uint64(^uint64(0)>>1) {
			return int64(value)
		}
	case jsonNumber:
		n, _ := value.Int64()
		return n
	}
	return 0
}

type jsonNumber interface {
	Int64() (int64, error)
}

func IsQuotaNearLimit(quota EffectiveQuota, typicalOperationAmount int64) bool {
	return NearLimitReasonForQuota(quota, typicalOperationAmount) != NearLimitReasonNone
}

func NearLimitReasonForQuota(quota EffectiveQuota, typicalOperationAmount int64) NearLimitReason {
	if quota.EnforcementMode != EnforcementModeEnforced || quota.Limit <= 0 || quota.RemainingAmount <= 0 {
		return NearLimitReasonNone
	}
	used := quota.ConsumedAmount + quota.ReservedAmount + quota.AdjustedAmount - quota.CarryoverApplied
	if used*100 >= quota.Limit*80 {
		return NearLimitReasonPercentThreshold
	}
	if typicalOperationAmount > 0 && quota.RemainingAmount < typicalOperationAmount {
		return NearLimitReasonBelowOneTypicalOperation
	}
	return NearLimitReasonNone
}

func RecoveryActionsForQuotaStatus(status QuotaStatus, reason NearLimitReason) []RecoveryAction {
	switch status {
	case QuotaStatusRestricted:
		return []RecoveryAction{RecoveryActionContactSupport}
	case QuotaStatusUnavailable:
		return []RecoveryAction{RecoveryActionOperatorResolutionRequired, RecoveryActionRetryLater}
	case QuotaStatusExhausted:
		return []RecoveryAction{RecoveryActionWait, RecoveryActionReduceScope, RecoveryActionRequestOverride}
	case QuotaStatusNearLimit:
		if reason == NearLimitReasonBelowOneTypicalOperation {
			return []RecoveryAction{RecoveryActionReduceScope, RecoveryActionWait}
		}
		return []RecoveryAction{RecoveryActionWait, RecoveryActionReduceScope}
	default:
		return nil
	}
}

func GroupQuotaStatusItems(items []QuotaStatusItem) []QuotaSection {
	grouped := map[string]*QuotaSection{}
	order := []string{"launches", "runtime", "integrations", "storage", "evaluations"}
	for _, item := range items {
		key, label := quotaSectionForCategory(item.Category)
		section := grouped[key]
		if section == nil {
			section = &QuotaSection{SectionKey: key, Label: label}
			grouped[key] = section
		}
		section.Items = append(section.Items, item)
	}
	seenOrder := map[string]bool{}
	sections := make([]QuotaSection, 0, len(grouped))
	for _, key := range order {
		if section := grouped[key]; section != nil {
			sections = append(sections, *section)
			seenOrder[key] = true
		}
	}
	var extra []string
	for key := range grouped {
		if !seenOrder[key] {
			extra = append(extra, key)
		}
	}
	sort.Strings(extra)
	for _, key := range extra {
		sections = append(sections, *grouped[key])
	}
	return sections
}

func quotaSectionForCategory(category Category) (string, string) {
	switch category {
	case CategoryRunLaunches, CategoryWorkflowLaunches:
		return "launches", "Launches"
	case CategoryRuntimeToolCalls, CategoryLiveValidationAttempts:
		return "runtime", "Runtime"
	case CategoryIntegrationOperations:
		return "integrations", "Integrations"
	case CategoryArtifactStorageBytes:
		return "storage", "Artifact Storage"
	case CategoryReplayEvaluationAttempts:
		return "evaluations", "Evaluations"
	default:
		return "other", "Other"
	}
}

func PeriodFor(kind PeriodKind, now time.Time) (time.Time, time.Time) {
	now = now.UTC()
	switch kind {
	case PeriodDaily:
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 0, 1)
	case PeriodMonthly:
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 1, 0)
	default:
		start := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
		return start, time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
	}
}
