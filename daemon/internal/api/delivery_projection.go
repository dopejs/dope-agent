package api

import (
	"context"

	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
)

func projectRunDeliverySummaries(ctx context.Context, deliveryManager *delivery.Manager, runs []runtime.Run) ([]runtime.Run, error) {
	if deliveryManager == nil || len(runs) == 0 {
		return runs, nil
	}
	items := make([]runtime.Run, 0, len(runs))
	for _, run := range runs {
		projected, err := projectRunDeliverySummary(ctx, deliveryManager, run)
		if err != nil {
			return nil, err
		}
		items = append(items, projected)
	}
	return items, nil
}

func projectRunDeliverySummary(ctx context.Context, deliveryManager *delivery.Manager, run runtime.Run) (runtime.Run, error) {
	if deliveryManager == nil {
		return run, nil
	}
	summary, ok, err := deliveryManager.LatestSummaryForRun(ctx, run.RunID)
	if err != nil || !ok {
		return run, err
	}
	return applyLatestDeliveryToRun(run, summary), nil
}

func projectWorkflowDeliverySummaries(ctx context.Context, deliveryManager *delivery.Manager, workflows []orchestration.Workflow) ([]orchestration.Workflow, error) {
	if deliveryManager == nil || len(workflows) == 0 {
		return workflows, nil
	}
	items := make([]orchestration.Workflow, 0, len(workflows))
	for _, workflow := range workflows {
		projected, err := projectWorkflowDeliverySummary(ctx, deliveryManager, workflow)
		if err != nil {
			return nil, err
		}
		items = append(items, projected)
	}
	return items, nil
}

func projectWorkflowDeliverySummary(ctx context.Context, deliveryManager *delivery.Manager, workflow orchestration.Workflow) (orchestration.Workflow, error) {
	if deliveryManager == nil {
		return workflow, nil
	}
	summary, ok, err := deliveryManager.LatestSummaryForWorkflow(ctx, workflow.WorkflowID)
	if err != nil || !ok {
		return workflow, err
	}
	return applyLatestDeliveryToWorkflow(workflow, summary), nil
}

func projectScheduleDeliverySummaries(ctx context.Context, deliveryManager *delivery.Manager, schedules []scheduler.Schedule) ([]scheduler.Schedule, error) {
	if deliveryManager == nil || len(schedules) == 0 {
		return schedules, nil
	}
	items := make([]scheduler.Schedule, 0, len(schedules))
	for _, schedule := range schedules {
		projected, err := projectScheduleDeliverySummary(ctx, deliveryManager, schedule)
		if err != nil {
			return nil, err
		}
		items = append(items, projected)
	}
	return items, nil
}

func projectScheduleDeliverySummary(ctx context.Context, deliveryManager *delivery.Manager, schedule scheduler.Schedule) (scheduler.Schedule, error) {
	if deliveryManager == nil || len(schedule.Attempts) == 0 {
		return schedule, nil
	}
	summaries, err := deliveryManager.LatestSummariesForScheduleAttempts(ctx, schedule.ScheduleID)
	if err != nil || len(summaries) == 0 {
		return schedule, err
	}
	return applyLatestDeliveryToSchedule(schedule, summaries), nil
}
