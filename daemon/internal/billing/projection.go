package billing

import (
	"context"
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
