package billing

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrQuotaDenied            = errors.New("quota denied")
	ErrQuotaStateUnavailable  = errors.New("quota state unavailable")
	ErrNegativeEffectiveUsage = errors.New("billing adjustment would make effective usage negative")
	ErrReasonRequired         = errors.New("billing reason is required")
	ErrOperatorActionRequired = errors.New("billing reservation requires operator action")
	ErrReservationNotFound    = errors.New("billing reservation not found")
)

type Repository interface {
	ActivePlan(ctx context.Context, tenantID string) (TenantPlan, bool, error)
	QuotaOverride(ctx context.Context, tenantID string, category Category, at time.Time) (*QuotaOverride, error)
	OpenPeriod(ctx context.Context, tenantID string, definition QuotaDefinition, at time.Time) (QuotaPeriod, error)
	UsageCounter(ctx context.Context, tenantID string, category Category, quotaPeriodID string) (UsageCounter, bool, error)
	SaveUsageCounter(ctx context.Context, counter UsageCounter) error
	ReservationByOperation(ctx context.Context, tenantID string, category Category, operationKey string) (UsageReservation, bool, error)
	SaveReservation(ctx context.Context, reservation UsageReservation) error
	AppendUsageEvent(ctx context.Context, event UsageEvent) error
	AppendQuotaDenial(ctx context.Context, denial QuotaDenial) error
	ListPendingReservations(ctx context.Context) ([]UsageReservation, error)
}

type Manager struct {
	repo Repository
	now  func() time.Time
	mu   sync.Mutex
}

func NewManager(repo Repository) *Manager {
	return &Manager{repo: repo, now: func() time.Time { return time.Now().UTC() }}
}

func NewManagerWithClock(repo Repository, now func() time.Time) *Manager {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Manager{repo: repo, now: now}
}

type ReserveInput struct {
	TenantID          string
	Category          Category
	Amount            int64
	OperationKey      string
	ReservationPoint  string
	GuardedEntryPoint string
	ActorPrincipalID  string
	Hosted            bool
}

type ReserveResult struct {
	Allowed     bool
	Reservation UsageReservation
	Denial      *DenialPayload
	Quota       EffectiveQuota
}

type ReserveAllResult struct {
	Allowed bool
	Results []ReserveResult
	Denial  *DenialPayload
}

func (m *Manager) ReserveAll(ctx context.Context, inputs []ReserveInput) (ReserveAllResult, error) {
	if m != nil && m.repo != nil {
		if txRepo, ok := m.repo.(interface {
			ReserveAllUsage(context.Context, []ReserveInput, time.Time) (ReserveAllResult, error)
		}); ok {
			now := m.now().UTC()
			return txRepo.ReserveAllUsage(ctx, inputs, now)
		}
	}
	results := make([]ReserveResult, 0, len(inputs))
	for _, input := range inputs {
		result, err := m.Reserve(ctx, input)
		results = append(results, result)
		if err == nil && result.Allowed {
			continue
		}
		for _, prior := range results[:len(results)-1] {
			if prior.Reservation.ReservationID == "" {
				continue
			}
			_, _ = m.Release(ctx, ResolveInput{
				TenantID:     prior.Reservation.TenantID,
				Category:     prior.Reservation.Category,
				OperationKey: prior.Reservation.OperationKey,
				Amount:       prior.Reservation.AmountReserved,
				ReasonCode:   "billing.multi_category_reservation_released",
				Reason:       "multi-category reservation denied",
			})
		}
		return ReserveAllResult{Allowed: false, Results: results, Denial: result.Denial}, err
	}
	return ReserveAllResult{Allowed: true, Results: results}, nil
}

func (m *Manager) Reserve(ctx context.Context, input ReserveInput) (ReserveResult, error) {
	if m == nil || m.repo == nil {
		if input.Hosted {
			denial := NewQuotaStateUnavailableDenial(input.TenantID, input.OperationKey).Payload
			return ReserveResult{Allowed: false, Denial: &denial}, ErrQuotaStateUnavailable
		}
		return ReserveResult{Allowed: true}, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now().UTC()
	if txRepo, ok := m.repo.(interface {
		ReserveUsage(context.Context, ReserveInput, time.Time) (ReserveResult, error)
	}); ok {
		return txRepo.ReserveUsage(ctx, input, now)
	}
	amount := input.Amount
	if amount <= 0 {
		amount = 1
	}
	definition, ok := DefinitionFor(input.Category)
	if !ok {
		return ReserveResult{}, fmt.Errorf("unknown quota category %q", input.Category)
	}
	plan, ok, err := m.repo.ActivePlan(ctx, input.TenantID)
	if err != nil {
		return ReserveResult{}, err
	}
	if !ok {
		if input.Hosted {
			denial := NewQuotaStateUnavailableDenial(input.TenantID, input.OperationKey).Payload
			return ReserveResult{Allowed: false, Denial: &denial}, ErrQuotaStateUnavailable
		}
		plan = DevelopmentPlan(input.TenantID, now)
	}
	if plan.EnforcementMode == EnforcementModeUnlimited {
		return ReserveResult{Allowed: true}, nil
	}
	period, err := m.repo.OpenPeriod(ctx, input.TenantID, definition, now)
	if err != nil {
		if input.Hosted {
			denial := NewQuotaStateUnavailableDenial(input.TenantID, input.OperationKey).Payload
			return ReserveResult{Allowed: false, Denial: &denial}, ErrQuotaStateUnavailable
		}
		return ReserveResult{}, err
	}
	if existing, ok, err := m.repo.ReservationByOperation(ctx, input.TenantID, input.Category, input.OperationKey); err != nil {
		return ReserveResult{}, err
	} else if ok {
		if existing.Status == ReservationStatusOperatorActionNeeded {
			denial := NewQuotaStateUnavailableDenial(input.TenantID, input.OperationKey).Payload
			return ReserveResult{Allowed: false, Reservation: existing, Denial: &denial}, ErrOperatorActionRequired
		}
		if existing.Status == ReservationStatusDenied {
			denialErr := NewQuotaExhaustedDenial(input.TenantID, input.Category, input.OperationKey, amount, 0, period)
			payload := denialErr.Payload
			return ReserveResult{Allowed: false, Reservation: existing, Denial: &payload}, ErrQuotaDenied
		}
		return ReserveResult{Allowed: existing.Status != ReservationStatusDenied, Reservation: existing}, nil
	}
	counter, ok, err := m.repo.UsageCounter(ctx, input.TenantID, input.Category, period.QuotaPeriodID)
	if err != nil {
		return ReserveResult{}, err
	}
	if !ok {
		counter = UsageCounter{
			UsageCounterID: "usage_counter_" + input.TenantID + "_" + string(input.Category) + "_" + period.QuotaPeriodID,
			TenantID:       input.TenantID,
			Category:       input.Category,
			QuotaPeriodID:  period.QuotaPeriodID,
			UpdatedAt:      now,
		}
	}
	override, err := m.repo.QuotaOverride(ctx, input.TenantID, input.Category, now)
	if err != nil {
		return ReserveResult{}, err
	}
	quota := ProjectQuota(plan, definition, period, counter, override)
	if quota.RemainingAmount < amount {
		denialErr := NewQuotaExhaustedDenial(input.TenantID, input.Category, input.OperationKey, amount, quota.RemainingAmount, period)
		reservation := UsageReservation{
			ReservationID:    "reservation_" + input.OperationKey,
			TenantID:         input.TenantID,
			Category:         input.Category,
			QuotaPeriodID:    period.QuotaPeriodID,
			OperationKey:     input.OperationKey,
			AmountReserved:   amount,
			Status:           ReservationStatusDenied,
			ReservationPoint: input.ReservationPoint,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := m.repo.SaveReservation(ctx, reservation); err != nil {
			return ReserveResult{}, err
		}
		denial := QuotaDenial{
			DenialID:          "denial_" + input.OperationKey,
			TenantID:          input.TenantID,
			Category:          input.Category,
			QuotaPeriodID:     period.QuotaPeriodID,
			OperationKey:      input.OperationKey,
			ReasonCode:        denialErr.Payload.ReasonCode,
			RequestedAmount:   amount,
			RemainingAmount:   quota.RemainingAmount,
			GuardedEntryPoint: input.GuardedEntryPoint,
			CreatedAt:         now,
		}
		if err := m.repo.AppendQuotaDenial(ctx, denial); err != nil {
			return ReserveResult{}, err
		}
		_ = m.repo.AppendUsageEvent(ctx, UsageEvent{
			UsageEventID:  "usage_event_denial_" + input.OperationKey,
			TenantID:      input.TenantID,
			Category:      input.Category,
			QuotaPeriodID: period.QuotaPeriodID,
			OperationKey:  input.OperationKey,
			EventKind:     UsageEventDenial,
			Amount:        amount,
			ReasonCode:    denial.ReasonCode,
			Outcome:       "denied",
			CreatedAt:     now,
		})
		payload := denialErr.Payload
		return ReserveResult{Allowed: false, Reservation: reservation, Denial: &payload, Quota: quota}, ErrQuotaDenied
	}
	reservation := UsageReservation{
		ReservationID:    "reservation_" + input.OperationKey,
		TenantID:         input.TenantID,
		Category:         input.Category,
		QuotaPeriodID:    period.QuotaPeriodID,
		OperationKey:     input.OperationKey,
		AmountReserved:   amount,
		Status:           ReservationStatusReserved,
		ReservationPoint: input.ReservationPoint,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	counter.ReservedAmount += amount
	counter.UpdatedAt = now
	if err := m.repo.SaveUsageCounter(ctx, counter); err != nil {
		return ReserveResult{}, err
	}
	if err := m.repo.SaveReservation(ctx, reservation); err != nil {
		return ReserveResult{}, err
	}
	if err := m.repo.AppendUsageEvent(ctx, UsageEvent{
		UsageEventID:     "usage_event_reserved_" + input.OperationKey,
		TenantID:         input.TenantID,
		Category:         input.Category,
		QuotaPeriodID:    period.QuotaPeriodID,
		OperationKey:     input.OperationKey,
		EventKind:        UsageEventReservation,
		Amount:           amount,
		ReasonCode:       "usage_reserved",
		ActorPrincipalID: input.ActorPrincipalID,
		Outcome:          "reserved",
		CreatedAt:        now,
	}); err != nil {
		return ReserveResult{}, err
	}
	return ReserveResult{Allowed: true, Reservation: reservation, Quota: quota}, nil
}

func DevelopmentPlan(tenantID string, now time.Time) TenantPlan {
	return TenantPlan{
		PlanID:          "plan_" + tenantID + "_development",
		TenantID:        tenantID,
		PlanKey:         "development",
		Status:          PlanStatusActive,
		EnforcementMode: EnforcementModeUnlimited,
		EffectiveAt:     now.UTC(),
	}
}
