package api

import (
	"context"
	"sort"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func projectToolCallsCalendarSummaries(ctx context.Context, sqliteStore *store.SQLiteStore, toolCalls []runtime.ToolCall) ([]runtime.ToolCall, error) {
	if sqliteStore == nil || len(toolCalls) == 0 {
		return toolCalls, nil
	}
	items := make([]runtime.ToolCall, 0, len(toolCalls))
	for _, item := range toolCalls {
		projected, err := projectToolCallCalendarSummaries(ctx, sqliteStore, item)
		if err != nil {
			return nil, err
		}
		items = append(items, projected)
	}
	return items, nil
}

func projectToolCallCalendarSummaries(ctx context.Context, sqliteStore *store.SQLiteStore, toolCall runtime.ToolCall) (runtime.ToolCall, error) {
	if sqliteStore == nil {
		return toolCall, nil
	}
	ops, err := sqliteStore.ListCalendarOperations(ctx, events.EnvironmentScopeFromContext(ctx), store.CalendarOperationFilter{RunID: toolCall.RunID})
	if err != nil {
		return runtime.ToolCall{}, err
	}
	filtered := make([]calendar.Operation, 0)
	for _, item := range ops {
		if strings.TrimSpace(item.StepID) != toolCall.StepID {
			continue
		}
		if trimmed := strings.TrimSpace(item.ToolCallID); trimmed != "" && trimmed != toolCall.ToolCallID {
			continue
		}
		filtered = append(filtered, item)
	}
	toolCall.CalendarOperationSummaries = summarizeCalendarOperations(filtered)
	return toolCall, nil
}

func projectWorkflowsCalendarSummaries(ctx context.Context, sqliteStore *store.SQLiteStore, workflows []orchestration.Workflow) ([]orchestration.Workflow, error) {
	if sqliteStore == nil || len(workflows) == 0 {
		return workflows, nil
	}
	items := make([]orchestration.Workflow, 0, len(workflows))
	for _, item := range workflows {
		projected, err := projectWorkflowCalendarSummaries(ctx, sqliteStore, item)
		if err != nil {
			return nil, err
		}
		items = append(items, projected)
	}
	return items, nil
}

func projectWorkflowCalendarSummaries(ctx context.Context, sqliteStore *store.SQLiteStore, workflow orchestration.Workflow) (orchestration.Workflow, error) {
	if sqliteStore == nil || strings.TrimSpace(workflow.EnvironmentScope) == "" || strings.TrimSpace(workflow.WorkflowID) == "" {
		return workflow, nil
	}
	ops, err := sqliteStore.ListCalendarOperations(ctx, workflow.EnvironmentScope, store.CalendarOperationFilter{WorkflowID: workflow.WorkflowID})
	if err != nil {
		return orchestration.Workflow{}, err
	}
	for idx := range workflow.Steps {
		filtered := make([]calendar.Operation, 0)
		for _, item := range ops {
			if strings.TrimSpace(item.WorkflowStepID) == workflow.Steps[idx].WorkflowStepID {
				filtered = append(filtered, item)
				continue
			}
			if workflow.Steps[idx].RuntimeStepID != "" && strings.TrimSpace(item.StepID) == workflow.Steps[idx].RuntimeStepID {
				filtered = append(filtered, item)
			}
		}
		workflow.Steps[idx].CalendarOperationSummaries = summarizeCalendarOperations(filtered)
	}
	return workflow, nil
}

func projectSchedulesCalendarSummaries(ctx context.Context, sqliteStore *store.SQLiteStore, schedules []scheduler.Schedule) ([]scheduler.Schedule, error) {
	if sqliteStore == nil || len(schedules) == 0 {
		return schedules, nil
	}
	items := make([]scheduler.Schedule, 0, len(schedules))
	for _, item := range schedules {
		projected, err := projectScheduleCalendarSummaries(ctx, sqliteStore, item)
		if err != nil {
			return nil, err
		}
		items = append(items, projected)
	}
	return items, nil
}

func projectScheduleCalendarSummaries(ctx context.Context, sqliteStore *store.SQLiteStore, schedule scheduler.Schedule) (scheduler.Schedule, error) {
	if sqliteStore == nil || strings.TrimSpace(schedule.EnvironmentScope) == "" || strings.TrimSpace(schedule.ScheduleID) == "" {
		return schedule, nil
	}
	ops, err := sqliteStore.ListCalendarOperations(ctx, schedule.EnvironmentScope, store.CalendarOperationFilter{ScheduleID: schedule.ScheduleID})
	if err != nil {
		return scheduler.Schedule{}, err
	}
	for idx := range schedule.Attempts {
		filtered := make([]calendar.Operation, 0)
		for _, item := range ops {
			if strings.TrimSpace(item.ScheduleAttemptID) == schedule.Attempts[idx].AttemptID {
				filtered = append(filtered, item)
			}
		}
		schedule.Attempts[idx].CalendarOperationSummaries = summarizeCalendarOperations(filtered)
	}
	return schedule, nil
}

func projectDeliveryOutcomesCalendarLinkage(ctx context.Context, sqliteStore *store.SQLiteStore, items []delivery.DeliveryOutcome) ([]delivery.DeliveryOutcome, error) {
	if sqliteStore == nil || len(items) == 0 {
		return items, nil
	}
	projected := make([]delivery.DeliveryOutcome, 0, len(items))
	for _, item := range items {
		next, err := projectDeliveryOutcomeCalendarLinkage(ctx, sqliteStore, item)
		if err != nil {
			return nil, err
		}
		projected = append(projected, next)
	}
	return projected, nil
}

func projectDeliveryOutcomeCalendarLinkage(ctx context.Context, sqliteStore *store.SQLiteStore, outcome delivery.DeliveryOutcome) (delivery.DeliveryOutcome, error) {
	if sqliteStore == nil || strings.TrimSpace(outcome.EnvironmentScope) == "" {
		return outcome, nil
	}
	filter := store.CalendarOperationFilter{}
	switch {
	case strings.TrimSpace(outcome.DeliveryID) != "":
		filter.DeliveryID = outcome.DeliveryID
	case strings.TrimSpace(outcome.WorkflowID) != "":
		filter.WorkflowID = outcome.WorkflowID
	case strings.TrimSpace(outcome.ScheduleID) != "":
		filter.ScheduleID = outcome.ScheduleID
	case strings.TrimSpace(outcome.RunID) != "":
		filter.RunID = outcome.RunID
	}
	ops, err := sqliteStore.ListCalendarOperations(ctx, outcome.EnvironmentScope, filter)
	if err != nil {
		return delivery.DeliveryOutcome{}, err
	}
	outcome.CalendarOperationSummaries = summarizeCalendarOperations(ops)
	outcome.CalendarOperationIDs = make([]string, 0, len(outcome.CalendarOperationSummaries))
	for _, item := range outcome.CalendarOperationSummaries {
		outcome.CalendarOperationIDs = append(outcome.CalendarOperationIDs, item.OperationID)
	}
	return outcome, nil
}

func summarizeCalendarOperations(items []calendar.Operation) []calendar.OperationSummary {
	if len(items) == 0 {
		return nil
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].OperationID < items[j].OperationID
		}
		return items[i].UpdatedAt.Before(items[j].UpdatedAt)
	})
	summaries := make([]calendar.OperationSummary, 0, len(items))
	for _, item := range items {
		capturedAt := item.UpdatedAt
		if item.CompletedAt != nil {
			capturedAt = item.CompletedAt.UTC()
		}
		summaries = append(summaries, calendar.OperationSummary{
			OperationID:     item.OperationID,
			OperationClass:  item.OperationClass,
			IntegrationID:   item.IntegrationID,
			ExternalEventID: item.ExternalEventID,
			Status:          item.Status,
			TimezoneUsed:    item.TimezoneUsed,
			CapturedAt:      capturedAt,
		})
	}
	return summaries
}
