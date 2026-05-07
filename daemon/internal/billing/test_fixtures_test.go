package billing

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fixtureRepo struct {
	mu           sync.Mutex
	plans        map[string]TenantPlan
	overrides    map[string]QuotaOverride
	periods      map[string]QuotaPeriod
	counters     map[string]UsageCounter
	reservations map[string]UsageReservation
	events       []UsageEvent
	denials      []QuotaDenial
	adjustments  []ManualAdjustment
	restrictions []AbuseRestrictionRecord
}

func newFixtureRepo(t *testing.T, now time.Time) *fixtureRepo {
	t.Helper()
	repo := &fixtureRepo{
		plans:        map[string]TenantPlan{},
		overrides:    map[string]QuotaOverride{},
		periods:      map[string]QuotaPeriod{},
		counters:     map[string]UsageCounter{},
		reservations: map[string]UsageReservation{},
	}
	repo.plans["ten_finite"] = TenantPlan{PlanID: "plan_finite", TenantID: "ten_finite", PlanKey: "hosted-finite", Status: PlanStatusActive, EnforcementMode: EnforcementModeEnforced, EffectiveAt: now}
	repo.plans["ten_unlimited"] = TenantPlan{PlanID: "plan_unlimited", TenantID: "ten_unlimited", PlanKey: "unlimited", Status: PlanStatusActive, EnforcementMode: EnforcementModeUnlimited, EffectiveAt: now}
	repo.plans["ten_dev"] = DevelopmentPlan("ten_dev", now)
	return repo
}

func (r *fixtureRepo) ActivePlan(_ context.Context, tenantID string) (TenantPlan, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.plans[tenantID]
	return item, ok, nil
}

func (r *fixtureRepo) QuotaOverride(_ context.Context, tenantID string, category Category, _ time.Time) (*QuotaOverride, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.overrides[tenantID+":"+string(category)]
	if !ok {
		return nil, nil
	}
	copy := item
	return &copy, nil
}

func (r *fixtureRepo) OpenPeriod(_ context.Context, tenantID string, definition QuotaDefinition, at time.Time) (QuotaPeriod, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	start, end := PeriodFor(definition.PeriodKind, at)
	key := tenantID + ":" + string(definition.Category) + ":" + start.Format(time.RFC3339)
	if item, ok := r.periods[key]; ok {
		return item, nil
	}
	item := QuotaPeriod{QuotaPeriodID: "period_" + key, TenantID: tenantID, Category: definition.Category, PeriodKind: definition.PeriodKind, PeriodStart: start, PeriodEnd: end, Status: "open"}
	r.periods[key] = item
	return item, nil
}

func (r *fixtureRepo) UsageCounter(_ context.Context, tenantID string, category Category, periodID string) (UsageCounter, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.counters[tenantID+":"+string(category)+":"+periodID]
	return item, ok, nil
}

func (r *fixtureRepo) SaveUsageCounter(_ context.Context, counter UsageCounter) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters[counter.TenantID+":"+string(counter.Category)+":"+counter.QuotaPeriodID] = counter
	return nil
}

func (r *fixtureRepo) ReservationByOperation(_ context.Context, tenantID string, category Category, operationKey string) (UsageReservation, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.reservations[tenantID+":"+string(category)+":"+operationKey]
	return item, ok, nil
}

func (r *fixtureRepo) ReservationByID(_ context.Context, tenantID string, reservationID string) (UsageReservation, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.reservations {
		if item.TenantID == tenantID && item.ReservationID == reservationID {
			return item, true, nil
		}
	}
	return UsageReservation{}, false, nil
}

func (r *fixtureRepo) SaveReservation(_ context.Context, reservation UsageReservation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reservations[reservation.TenantID+":"+string(reservation.Category)+":"+reservation.OperationKey] = reservation
	return nil
}

func (r *fixtureRepo) AppendUsageEvent(_ context.Context, event UsageEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}

func (r *fixtureRepo) AppendQuotaDenial(_ context.Context, denial QuotaDenial) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.denials = append(r.denials, denial)
	return nil
}

func (r *fixtureRepo) QuotaDenialByID(_ context.Context, tenantID string, denialID string) (QuotaDenial, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.denials {
		if item.TenantID == tenantID && item.DenialID == denialID {
			return item, true, nil
		}
	}
	return QuotaDenial{}, false, nil
}

func (r *fixtureRepo) ListAbuseRestrictions(_ context.Context, tenantID string, at time.Time) ([]AbuseRestrictionRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []AbuseRestrictionRecord
	for _, item := range r.restrictions {
		if item.TenantID != tenantID || item.Status != AbuseRestrictionStatusActive || item.StartedAt.After(at) {
			continue
		}
		if item.ExpiresAt != nil && !item.ExpiresAt.After(at) {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *fixtureRepo) ListUsageEvidenceRefs(_ context.Context, tenantID string, operationKey string, _ int) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, item := range r.events {
		if item.TenantID == tenantID && item.OperationKey == operationKey {
			out = append(out, "usage_event:"+item.UsageEventID)
		}
	}
	return out, nil
}

func (r *fixtureRepo) ListPendingReservations(_ context.Context) ([]UsageReservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]UsageReservation, 0)
	for _, item := range r.reservations {
		if item.Status == ReservationStatusReserved {
			out = append(out, item)
		}
	}
	return out, nil
}

func (r *fixtureRepo) SavePlan(_ context.Context, plan TenantPlan) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plans[plan.TenantID] = plan
	return nil
}

func (r *fixtureRepo) SaveQuotaOverride(_ context.Context, override QuotaOverride) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.overrides[override.TenantID+":"+string(override.Category)] = override
	return nil
}

func (r *fixtureRepo) SaveManualAdjustment(_ context.Context, adjustment ManualAdjustment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adjustments = append(r.adjustments, adjustment)
	return nil
}
