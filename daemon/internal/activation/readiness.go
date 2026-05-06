package activation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

func (s *Service) activeStateForPersonalTenant(ctx context.Context, principal identity.Principal, tenant identity.Tenant, nowTimeNow time.Time) (State, error) {
	now := nowTimeNow
	if s == nil {
		return State{}, activationError(ReasonUnexpectedFailed, FailureStageUnexpected, false, RemediationOwnerOperator, "activation service is not configured")
	}
	environmentScope := s.environmentScope
	if environmentScope == "" {
		environmentScope = "test"
	}
	state := State{
		ActivationID:        stableActivationID("act", principal.PrincipalID, tenant.TenantID),
		PrincipalID:         principal.PrincipalID,
		TenantID:            tenant.TenantID,
		EnvironmentScope:    environmentScope,
		Status:              StatusActive,
		CurrentStepID:       StepTestChat,
		CompletedStepIDs:    []string{StepTenantResolved, StepQuotaBaselineReady},
		BlockingReasonCodes: []ReasonCode{},
		ReadinessItems: []ReadinessItem{
			readyReadinessItem("tenant-access", ReadinessKindTenantAccess, "Tenant access", now),
			readyReadinessItem("environment", ReadinessKindEnvironment, "Hosted environment", now),
		},
		FirstAction:     DefaultTestChatFirstAction(true, nil),
		CreatedAt:       now,
		UpdatedAt:       now,
		LastEvaluatedAt: now,
	}

	baseline, quotaItem, err := s.projectQuotaBaseline(ctx, tenant.TenantID, now)
	if err != nil {
		return State{}, err
	}
	state.QuotaBaseline = baseline
	state.ReadinessItems = append(state.ReadinessItems, quotaItem)
	if baseline.Status == QuotaBaselineStatusUnavailable {
		state.Status = StatusBlocked
		state.CurrentStepID = StepQuotaBaseline
		state.CompletedStepIDs = []string{StepTenantResolved}
		state.BlockingReasonCodes = []ReasonCode{ReasonQuotaBaselineUnavailable}
		state.FirstAction = DefaultTestChatFirstAction(false, []string{"quota-baseline"})
		state.FailureReason = &FailureReason{
			ReasonCode:       ReasonQuotaBaselineUnavailable,
			Stage:            FailureStageQuotaBaseline,
			Retryable:        true,
			RemediationOwner: RemediationOwnerOperator,
			Message:          "quota baseline is unavailable",
		}
	}
	return state, nil
}

func (s *Service) projectQuotaBaseline(ctx context.Context, tenantID string, now time.Time) (*QuotaBaseline, ReadinessItem, error) {
	if s == nil || s.billing == nil {
		return defaultQuotaBaseline(tenantID, now), readyReadinessItem("quota-baseline", ReadinessKindQuotaBaseline, "Quota baseline", now), nil
	}
	summary, err := s.billing.UsageSummary(ctx, tenantID, s.hosted)
	if err != nil {
		if errors.Is(err, billing.ErrQuotaStateUnavailable) {
			return unavailableQuotaBaseline(tenantID, now), blockedQuotaReadiness(now), nil
		}
		return nil, ReadinessItem{}, activationError(ReasonQuotaBaselineUnavailable, FailureStageQuotaBaseline, true, RemediationOwnerOperator, err.Error())
	}
	baseline := &QuotaBaseline{
		TenantID:         firstNonEmpty(summary.TenantID, tenantID),
		PlanKey:          firstNonEmpty(summary.PlanKey, "unknown"),
		EnforcementMode:  firstNonEmpty(string(summary.EnforcementMode), string(billing.EnforcementModeNotMeasurable)),
		Status:           QuotaBaselineStatusAvailable,
		Quotas:           make([]QuotaProjection, 0, len(summary.Quotas)),
		ProjectedAt:      now,
		ProjectionSource: "billing_usage_summary",
	}
	for _, quota := range summary.Quotas {
		baseline.Quotas = append(baseline.Quotas, quotaProjection(quota))
	}
	return baseline, readyReadinessItem("quota-baseline", ReadinessKindQuotaBaseline, "Quota baseline", now), nil
}

func readyReadinessItem(itemID string, kind ReadinessKind, displayName string, now time.Time) ReadinessItem {
	return ReadinessItem{
		ItemID:                itemID,
		ItemKind:              kind,
		Status:                ReadinessStatusReady,
		DisplayName:           displayName,
		RequiredForActivation: true,
		Retryable:             false,
		RemediationOwner:      RemediationOwnerNoneRequired,
		UpdatedAt:             now,
	}
}

func blockedQuotaReadiness(now time.Time) ReadinessItem {
	return ReadinessItem{
		ItemID:                "quota-baseline",
		ItemKind:              ReadinessKindQuotaBaseline,
		Status:                ReadinessStatusBlocked,
		ReasonCode:            ReasonQuotaBaselineUnavailable,
		DisplayName:           "Quota baseline",
		RequiredForActivation: true,
		Retryable:             true,
		RemediationOwner:      RemediationOwnerOperator,
		UpdatedAt:             now,
	}
}

func defaultQuotaBaseline(tenantID string, now time.Time) *QuotaBaseline {
	return &QuotaBaseline{
		TenantID:         tenantID,
		PlanKey:          "free",
		EnforcementMode:  string(billing.EnforcementModeEnforced),
		Status:           QuotaBaselineStatusAvailable,
		Quotas:           []QuotaProjection{},
		ProjectedAt:      now,
		ProjectionSource: "activation_default",
	}
}

func unavailableQuotaBaseline(tenantID string, now time.Time) *QuotaBaseline {
	return &QuotaBaseline{
		TenantID:         tenantID,
		PlanKey:          "unknown",
		EnforcementMode:  string(billing.EnforcementModeNotMeasurable),
		Status:           QuotaBaselineStatusUnavailable,
		Quotas:           []QuotaProjection{},
		ProjectedAt:      now,
		ProjectionSource: "billing_usage_summary",
		ReasonCode:       ReasonQuotaBaselineUnavailable,
	}
}

func quotaProjection(quota billing.EffectiveQuota) QuotaProjection {
	used := quota.ConsumedAmount + quota.ReservedAmount + quota.AdjustedAmount - quota.CarryoverApplied
	limit := quota.Limit
	remaining := quota.RemainingAmount
	metadata := map[string]any{}
	if !quota.PeriodStart.IsZero() {
		metadata["periodStart"] = quota.PeriodStart.UTC().Format(time.RFC3339)
	}
	if !quota.PeriodEnd.IsZero() {
		metadata["periodEnd"] = quota.PeriodEnd.UTC().Format(time.RFC3339)
	}
	if quota.PeriodAnchor != "" {
		metadata["periodAnchor"] = quota.PeriodAnchor
	}
	if quota.DenialReasonCode != "" {
		metadata["denialReasonCode"] = quota.DenialReasonCode
	}
	if quota.OverLimit {
		metadata["overLimit"] = true
	}
	if len(metadata) == 0 {
		metadata = nil
	}
	return QuotaProjection{
		Category:  string(quota.Category),
		Unit:      string(quota.Unit),
		Limit:     &limit,
		Used:      &used,
		Remaining: &remaining,
		Period:    fmt.Sprintf("%s/%s", quota.PeriodStart.UTC().Format(time.RFC3339), quota.PeriodEnd.UTC().Format(time.RFC3339)),
		Metadata:  metadata,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
