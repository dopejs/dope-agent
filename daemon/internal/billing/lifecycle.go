package billing

import (
	"context"
	"fmt"
	"time"
)

type ResolveInput struct {
	TenantID         string
	Category         Category
	OperationKey     string
	Amount           int64
	ReasonCode       string
	Reason           string
	ActorPrincipalID string
}

func (m *Manager) Commit(ctx context.Context, input ResolveInput) (UsageReservation, error) {
	return m.resolve(ctx, input, ReservationStatusCommitted, UsageEventCommit)
}

func (m *Manager) Refund(ctx context.Context, input ResolveInput) (UsageReservation, error) {
	return m.resolve(ctx, input, ReservationStatusRefunded, UsageEventRefund)
}

func (m *Manager) Release(ctx context.Context, input ResolveInput) (UsageReservation, error) {
	return m.resolve(ctx, input, ReservationStatusReleased, UsageEventRelease)
}

func (m *Manager) MarkOperatorActionNeeded(ctx context.Context, input ResolveInput) (UsageReservation, error) {
	if input.Reason == "" {
		return UsageReservation{}, ErrReasonRequired
	}
	return m.resolve(ctx, input, ReservationStatusOperatorActionNeeded, UsageEventRecoveryDecision)
}

func (m *Manager) resolve(ctx context.Context, input ResolveInput, status ReservationStatus, eventKind UsageEventKind) (UsageReservation, error) {
	if m == nil || m.repo == nil {
		return UsageReservation{}, nil
	}
	if txRepo, ok := m.repo.(interface {
		ResolveUsage(context.Context, ResolveInput, ReservationStatus, UsageEventKind, time.Time) (UsageReservation, error)
	}); ok {
		now := m.now().UTC()
		return txRepo.ResolveUsage(ctx, input, status, eventKind, now)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now().UTC()
	reservation, ok, err := m.repo.ReservationByOperation(ctx, input.TenantID, input.Category, input.OperationKey)
	if err != nil {
		return UsageReservation{}, err
	}
	if !ok {
		return UsageReservation{}, fmt.Errorf("%w for %s", ErrReservationNotFound, input.OperationKey)
	}
	if reservation.Status == status {
		return reservation, nil
	}
	counter, ok, err := m.repo.UsageCounter(ctx, input.TenantID, input.Category, reservation.QuotaPeriodID)
	if err != nil {
		return UsageReservation{}, err
	}
	if !ok {
		return UsageReservation{}, fmt.Errorf("counter not found for reservation %s", reservation.ReservationID)
	}
	amount := input.Amount
	if amount <= 0 {
		amount = reservation.AmountReserved
	}
	switch status {
	case ReservationStatusCommitted:
		delta := amount - reservation.AmountCommitted
		reservedRelease := min64(counter.ReservedAmount, reservation.AmountReserved-reservation.AmountRefunded)
		counter.ReservedAmount -= reservedRelease
		counter.CommittedAmount += delta
		reservation.AmountCommitted = amount
		if amount < reservation.AmountReserved {
			reservation.AmountRefunded += reservation.AmountReserved - amount
		}
	case ReservationStatusRefunded, ReservationStatusReleased:
		refund := min64(counter.ReservedAmount, reservation.AmountReserved-reservation.AmountRefunded)
		counter.ReservedAmount -= refund
		reservation.AmountRefunded += refund
	case ReservationStatusOperatorActionNeeded:
		reservation.RecoveryReason = input.Reason
	}
	counter.UpdatedAt = now
	reservation.Status = status
	reservation.UpdatedAt = now
	if err := m.repo.SaveUsageCounter(ctx, counter); err != nil {
		return UsageReservation{}, err
	}
	if err := m.repo.SaveReservation(ctx, reservation); err != nil {
		return UsageReservation{}, err
	}
	reasonCode := input.ReasonCode
	if reasonCode == "" {
		reasonCode = string(eventKind)
	}
	if err := m.repo.AppendUsageEvent(ctx, UsageEvent{
		UsageEventID:     fmt.Sprintf("usage_event_%s_%s", eventKind, input.OperationKey),
		TenantID:         input.TenantID,
		Category:         input.Category,
		QuotaPeriodID:    reservation.QuotaPeriodID,
		OperationKey:     input.OperationKey,
		EventKind:        eventKind,
		Amount:           amount,
		ReasonCode:       reasonCode,
		Reason:           input.Reason,
		ActorPrincipalID: input.ActorPrincipalID,
		Outcome:          string(status),
		CreatedAt:        now,
	}); err != nil {
		return UsageReservation{}, err
	}
	return reservation, nil
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
