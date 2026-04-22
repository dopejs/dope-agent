package delivery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
)

func (m *Manager) queueDigestOutcome(ctx context.Context, outcome DeliveryOutcome, pref DeliveryPreference, target DeliveryTarget) (DeliveryOutcome, error) {
	window, err := m.findOrCreateOpenWindow(ctx, pref, target)
	if err != nil {
		return DeliveryOutcome{}, err
	}
	now := time.Now().UTC()
	outcome.Status = OutcomeStatusQueued
	outcome.SummaryWindowID = window.SummaryWindowID
	outcome.UpdatedAt = now
	if err := m.storeOutcome(ctx, outcome); err != nil {
		return DeliveryOutcome{}, err
	}
	m.scheduleWindow(window.SummaryWindowID, window.WindowEndsAt)
	return m.attachAttempts(ctx, outcome)
}

func (m *Manager) findOrCreateOpenWindow(ctx context.Context, pref DeliveryPreference, target DeliveryTarget) (SummaryWindow, error) {
	items, err := m.ListSummaryWindows(ctx)
	if err != nil {
		return SummaryWindow{}, err
	}
	now := time.Now().UTC()
	for _, item := range items {
		if item.PreferenceID == pref.PreferenceID && item.TargetID == target.TargetID && item.Status == SummaryWindowStatusOpen && item.WindowEndsAt.After(now) {
			item.ResultCount++
			item.UpdatedAt = now
			if err := m.storeWindow(ctx, item); err != nil {
				return SummaryWindow{}, err
			}
			return item, nil
		}
	}
	windowMinutes := pref.SummaryPolicy.WindowMinutes
	if windowMinutes <= 0 {
		windowMinutes = 15
	}
	window := SummaryWindow{
		SummaryWindowID:  newSummaryWindowID(),
		EnvironmentScope: m.environmentScope,
		TargetID:         target.TargetID,
		PreferenceID:     pref.PreferenceID,
		Status:           SummaryWindowStatusOpen,
		WindowStartedAt:  now,
		WindowEndsAt:     now.Add(time.Duration(windowMinutes) * time.Minute),
		ResultCount:      1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := m.storeWindow(ctx, window); err != nil {
		return SummaryWindow{}, err
	}
	return window, nil
}

func (m *Manager) scheduleWindow(summaryWindowID string, when time.Time) {
	if m == nil || strings.TrimSpace(summaryWindowID) == "" {
		return
	}
	m.mu.Lock()
	if _, exists := m.windowScheduled[summaryWindowID]; exists {
		m.mu.Unlock()
		return
	}
	m.windowScheduled[summaryWindowID] = struct{}{}
	m.mu.Unlock()

	go func() {
		delay := time.Until(when)
		if delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			<-timer.C
		}
		_ = m.emitWindow(context.Background(), summaryWindowID)
	}()
}

func (m *Manager) clearWindowSchedule(summaryWindowID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.windowScheduled, summaryWindowID)
}

func (m *Manager) emitWindow(ctx context.Context, summaryWindowID string) error {
	defer m.clearWindowSchedule(summaryWindowID)
	window, ok, err := m.GetSummaryWindow(ctx, summaryWindowID)
	if err != nil || !ok {
		return err
	}
	now := time.Now().UTC()
	if window.Status == SummaryWindowStatusOpen && window.WindowEndsAt.After(now) {
		m.scheduleWindow(window.SummaryWindowID, window.WindowEndsAt)
		return nil
	}
	if window.ResultCount <= 0 {
		window.Status = SummaryWindowStatusCancelled
		window.UpdatedAt = now
		return m.storeWindow(ctx, window)
	}
	if window.Status != SummaryWindowStatusOpen && window.Status != SummaryWindowStatusReady && window.Status != SummaryWindowStatusDispatching {
		return nil
	}

	window.Status = SummaryWindowStatusDispatching
	window.UpdatedAt = now
	if err := m.storeWindow(ctx, window); err != nil {
		return err
	}

	target, ok, err := m.GetTarget(ctx, window.TargetID)
	if err != nil {
		return err
	}
	if !ok {
		window.Status = SummaryWindowStatusFailed
		window.UpdatedAt = time.Now().UTC()
		return m.storeWindow(ctx, window)
	}
	outcome := DeliveryOutcome{
		DeliveryID:       newDeliveryID(),
		EnvironmentScope: m.environmentScope,
		SourceKind:       "summary_window",
		SourceID:         window.SummaryWindowID,
		ResultClass:      ResultClassRoutineSuccess,
		Mode:             DeliveryModeImmediate,
		Status:           OutcomeStatusPending,
		ChosenTargetID:   target.TargetID,
		PreferenceID:     window.PreferenceID,
		SummaryWindowID:  window.SummaryWindowID,
		PayloadPreview:   fmt.Sprintf("digest summary with %d routed results", window.ResultCount),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := m.storeOutcome(ctx, outcome); err != nil {
		return err
	}
	if err := m.publishOutcomeCreated(ctx, outcome); err != nil {
		return err
	}
	outcome, err = m.dispatchAttempt(ctx, outcome, target, 1)
	if err != nil {
		return err
	}
	window.EmittedDeliveryID = outcome.DeliveryID
	window.UpdatedAt = time.Now().UTC()
	if outcome.Status == OutcomeStatusDelivered {
		window.Status = SummaryWindowStatusDelivered
	} else {
		window.Status = SummaryWindowStatusFailed
	}
	if err := m.storeWindow(ctx, window); err != nil {
		return err
	}
	return m.publishEvent(ctx, "delivery.summary_emitted", events.Resource{Kind: "delivery_summary_window", ID: window.SummaryWindowID}, map[string]any{
		"summaryWindowId":   window.SummaryWindowID,
		"resultCount":       window.ResultCount,
		"emittedDeliveryId": window.EmittedDeliveryID,
	})
}

func (m *Manager) Restore(ctx context.Context) error {
	if m == nil {
		return nil
	}
	outcomes, err := m.ListOutcomes(ctx, OutcomeFilter{})
	if err != nil {
		return err
	}
	for _, outcome := range outcomes {
		if outcome.Status != OutcomeStatusQueued && outcome.Status != OutcomeStatusDispatching {
			continue
		}
		nextRunAt := time.Now().UTC()
		if len(outcome.Attempts) > 0 {
			last := outcome.Attempts[len(outcome.Attempts)-1]
			if last.NextRetryAt != nil {
				nextRunAt = last.NextRetryAt.UTC()
			}
		}
		if outcome.Mode == DeliveryModeDigest && strings.TrimSpace(outcome.SummaryWindowID) != "" {
			continue
		}
		m.scheduleRetry(outcome.DeliveryID, nextRunAt)
	}

	windows, err := m.ListSummaryWindows(ctx)
	if err != nil {
		return err
	}
	for _, window := range windows {
		switch window.Status {
		case SummaryWindowStatusOpen:
			m.scheduleWindow(window.SummaryWindowID, window.WindowEndsAt)
		case SummaryWindowStatusReady, SummaryWindowStatusDispatching:
			m.scheduleWindow(window.SummaryWindowID, time.Now().UTC())
		}
	}
	return nil
}
