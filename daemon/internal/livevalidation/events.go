package livevalidation

import "context"

type LedgerEventSink func(ctx context.Context, eventName string, entry SideEffectLedgerEntry)

const (
	LedgerEventSideEffectRecorded   = "live_validation.side_effect_recorded"
	LedgerEventOperatorActionNeeded = "live_validation.operator_action_needed"
)

func (m *Manager) emitLedgerEvent(ctx context.Context, eventName string, entry SideEffectLedgerEntry) {
	if m == nil || m.ledgerEventSink == nil {
		return
	}
	m.ledgerEventSink(ctx, eventName, entry)
}
