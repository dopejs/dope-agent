package billing

import (
	"context"
	"fmt"
	"strings"
)

type AdminRepository interface {
	Repository
	SavePlan(ctx context.Context, plan TenantPlan) error
	SaveQuotaOverride(ctx context.Context, override QuotaOverride) error
	SaveManualAdjustment(ctx context.Context, adjustment ManualAdjustment) error
}

type ReservationAdminRepository interface {
	Repository
	ReservationByID(ctx context.Context, tenantID string, reservationID string) (UsageReservation, bool, error)
}

func (m *Manager) AssignPlan(ctx context.Context, plan TenantPlan, actorPrincipalID, reason string) error {
	repo, ok := m.repo.(AdminRepository)
	if !ok {
		return fmt.Errorf("billing repository does not support admin plan assignment")
	}
	if strings.TrimSpace(reason) == "" {
		return ErrReasonRequired
	}
	if plan.AssignmentReason == "" {
		plan.AssignmentReason = reason
	}
	if plan.AssignedByPrincipalID == "" {
		plan.AssignedByPrincipalID = actorPrincipalID
	}
	if plan.Status == "" {
		plan.Status = PlanStatusActive
	}
	if plan.EnforcementMode == "" {
		plan.EnforcementMode = EnforcementModeEnforced
	}
	if plan.EffectiveAt.IsZero() {
		plan.EffectiveAt = m.now().UTC()
	}
	if err := repo.SavePlan(ctx, plan); err != nil {
		return err
	}
	return repo.AppendUsageEvent(ctx, UsageEvent{
		UsageEventID:     "usage_event_plan_changed_" + plan.TenantID + "_" + plan.PlanID,
		TenantID:         plan.TenantID,
		EventKind:        UsageEventPlanChanged,
		ReasonCode:       "billing.plan_changed",
		Reason:           reason,
		ActorPrincipalID: actorPrincipalID,
		Outcome:          "succeeded",
		CreatedAt:        m.now().UTC(),
	})
}

func (m *Manager) ApplyQuotaOverride(ctx context.Context, override QuotaOverride) error {
	repo, ok := m.repo.(AdminRepository)
	if !ok {
		return fmt.Errorf("billing repository does not support quota overrides")
	}
	if strings.TrimSpace(override.Reason) == "" {
		return ErrReasonRequired
	}
	if override.EffectiveAt.IsZero() {
		override.EffectiveAt = m.now().UTC()
	}
	if err := repo.SaveQuotaOverride(ctx, override); err != nil {
		return err
	}
	return repo.AppendUsageEvent(ctx, UsageEvent{
		UsageEventID:     "usage_event_quota_override_" + override.TenantID + "_" + string(override.Category),
		TenantID:         override.TenantID,
		Category:         override.Category,
		EventKind:        UsageEventQuotaOverride,
		ReasonCode:       "billing.quota_override_changed",
		Reason:           override.Reason,
		ActorPrincipalID: override.CreatedByPrincipalID,
		Outcome:          "succeeded",
		CreatedAt:        m.now().UTC(),
	})
}

func (m *Manager) ApplyManualAdjustment(ctx context.Context, adjustment ManualAdjustment) error {
	repo, ok := m.repo.(AdminRepository)
	if !ok {
		return fmt.Errorf("billing repository does not support manual adjustments")
	}
	if strings.TrimSpace(adjustment.Reason) == "" {
		return ErrReasonRequired
	}
	if _, ok := DefinitionFor(adjustment.Category); !ok {
		return fmt.Errorf("unknown quota category %q", adjustment.Category)
	}
	counter, ok, err := repo.UsageCounter(ctx, adjustment.TenantID, adjustment.Category, adjustment.QuotaPeriodID)
	if err != nil {
		return err
	}
	if ok && counter.CommittedAmount+counter.ReservedAmount+counter.AdjustedAmount+adjustment.AmountDelta < 0 {
		return ErrNegativeEffectiveUsage
	}
	if err := repo.SaveManualAdjustment(ctx, adjustment); err != nil {
		return err
	}
	if ok {
		counter.AdjustedAmount += adjustment.AmountDelta
		counter.UpdatedAt = m.now().UTC()
		if err := repo.SaveUsageCounter(ctx, counter); err != nil {
			return err
		}
	}
	return repo.AppendUsageEvent(ctx, UsageEvent{
		UsageEventID:     "usage_event_adjustment_" + adjustment.TenantID + "_" + adjustment.AdjustmentID,
		TenantID:         adjustment.TenantID,
		Category:         adjustment.Category,
		QuotaPeriodID:    adjustment.QuotaPeriodID,
		EventKind:        UsageEventManualAdjustment,
		Amount:           adjustment.AmountDelta,
		ReasonCode:       "billing.manual_adjustment_created",
		Reason:           adjustment.Reason,
		ActorPrincipalID: adjustment.CreatedByPrincipalID,
		Outcome:          "succeeded",
		CreatedAt:        m.now().UTC(),
	})
}

type ResolveReservationInput struct {
	TenantID         string
	ReservationID    string
	Outcome          ReservationStatus
	Amount           int64
	Reason           string
	ActorPrincipalID string
}

func (m *Manager) ResolveReservation(ctx context.Context, input ResolveReservationInput) (UsageReservation, error) {
	if strings.TrimSpace(input.Reason) == "" {
		return UsageReservation{}, ErrReasonRequired
	}
	repo, ok := m.repo.(ReservationAdminRepository)
	if !ok {
		return UsageReservation{}, fmt.Errorf("billing repository does not support reservation resolution")
	}
	reservation, found, err := repo.ReservationByID(ctx, input.TenantID, input.ReservationID)
	if err != nil {
		return UsageReservation{}, err
	}
	if !found {
		return UsageReservation{}, fmt.Errorf("reservation not found for %s", input.ReservationID)
	}
	resolveInput := ResolveInput{
		TenantID:         input.TenantID,
		Category:         reservation.Category,
		OperationKey:     reservation.OperationKey,
		Amount:           input.Amount,
		ReasonCode:       "billing.reservation_resolved",
		Reason:           input.Reason,
		ActorPrincipalID: input.ActorPrincipalID,
	}
	switch input.Outcome {
	case ReservationStatusCommitted:
		return m.Commit(ctx, resolveInput)
	case ReservationStatusRefunded:
		return m.Refund(ctx, resolveInput)
	case ReservationStatusReleased:
		return m.Release(ctx, resolveInput)
	case ReservationStatusOperatorActionNeeded:
		return m.MarkOperatorActionNeeded(ctx, resolveInput)
	default:
		return UsageReservation{}, fmt.Errorf("unsupported reservation resolution outcome %q", input.Outcome)
	}
}
