package delivery

import (
	"context"
	"strings"
)

type LatestSummary struct {
	LatestDeliveryID       string `json:"latestDeliveryId,omitempty"`
	LatestDeliveryStatus   string `json:"latestDeliveryStatus,omitempty"`
	LatestDeliveryTargetID string `json:"latestDeliveryTargetId,omitempty"`
}

func (m *Manager) LatestSummaryForRun(ctx context.Context, runID string) (LatestSummary, bool, error) {
	items, err := m.ListOutcomes(ctx, OutcomeFilter{
		SourceKind: "run",
		RunID:      strings.TrimSpace(runID),
	})
	if err != nil || len(items) == 0 {
		return LatestSummary{}, len(items) > 0, err
	}
	return latestSummaryFromOutcome(items[0]), true, nil
}

func (m *Manager) LatestSummaryForWorkflow(ctx context.Context, workflowID string) (LatestSummary, bool, error) {
	items, err := m.ListOutcomes(ctx, OutcomeFilter{
		SourceKind: "workflow",
		WorkflowID: strings.TrimSpace(workflowID),
	})
	if err != nil || len(items) == 0 {
		return LatestSummary{}, len(items) > 0, err
	}
	return latestSummaryFromOutcome(items[0]), true, nil
}

func (m *Manager) LatestSummariesForScheduleAttempts(ctx context.Context, scheduleID string) (map[string]LatestSummary, error) {
	items, err := m.ListOutcomes(ctx, OutcomeFilter{ScheduleID: strings.TrimSpace(scheduleID)})
	if err != nil {
		return nil, err
	}
	summaries := make(map[string]LatestSummary, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ScheduleAttemptID) == "" {
			continue
		}
		if _, exists := summaries[item.ScheduleAttemptID]; exists {
			continue
		}
		summaries[item.ScheduleAttemptID] = latestSummaryFromOutcome(item)
	}
	return summaries, nil
}

func latestSummaryFromOutcome(outcome DeliveryOutcome) LatestSummary {
	return LatestSummary{
		LatestDeliveryID:       outcome.DeliveryID,
		LatestDeliveryStatus:   string(outcome.Status),
		LatestDeliveryTargetID: outcome.ChosenTargetID,
	}
}
