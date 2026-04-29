package billing

import "context"

type RecoveryDecision struct {
	Reservation UsageReservation
	Outcome     ReservationStatus
	Reason      string
}

func (m *Manager) RecoverPendingReservations(ctx context.Context, decide func(UsageReservation) RecoveryDecision) ([]RecoveryDecision, error) {
	if m == nil || m.repo == nil {
		return nil, nil
	}
	items, err := m.repo.ListPendingReservations(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RecoveryDecision, 0, len(items))
	for _, reservation := range items {
		decision := RecoveryDecision{
			Reservation: reservation,
			Outcome:     ReservationStatusOperatorActionNeeded,
			Reason:      "restart outcome could not be proven",
		}
		if decide != nil {
			decision = decide(reservation)
			if decision.Reservation.ReservationID == "" {
				decision.Reservation = reservation
			}
			if decision.Outcome == "" {
				decision.Outcome = ReservationStatusOperatorActionNeeded
			}
		}
		input := ResolveInput{
			TenantID:     reservation.TenantID,
			Category:     reservation.Category,
			OperationKey: reservation.OperationKey,
			Amount:       reservation.AmountReserved,
			ReasonCode:   "billing.reservation_recovery_decided",
			Reason:       decision.Reason,
		}
		switch decision.Outcome {
		case ReservationStatusCommitted:
			_, err = m.Commit(ctx, input)
		case ReservationStatusReleased:
			_, err = m.Release(ctx, input)
		case ReservationStatusRefunded:
			_, err = m.Refund(ctx, input)
		default:
			_, err = m.MarkOperatorActionNeeded(ctx, input)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, decision)
	}
	return out, nil
}
