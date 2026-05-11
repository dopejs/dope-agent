package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
	"github.com/dopejs/dope-agent/daemon/internal/mcp"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/reminders"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
	"github.com/dopejs/dope-agent/daemon/internal/skills"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

type ScheduleWorkflowLauncherDependencies struct {
	Config       config.Config
	Runtime      *runtime.Manager
	Policy       *policy.Engine
	Capabilities *capabilities.Supervisor
	Skills       *skills.Registry
	MCP          *mcp.Manager
	Sandboxes    *sandbox.Manager
	Integrations *integrations.Manager
	Calendar     *calendar.Manager
	Mail         *mail.Manager
	ComputerUse  *computeruse.Manager
	Delivery     *delivery.Manager
	Billing      *billing.Manager
	EventBus     *events.Bus
	Store        *store.SQLiteStore
	Checkpoints  *checkpoints.Manager
}

type ScheduleWorkflowLauncher struct {
	cfg          config.Config
	runtime      *runtime.Manager
	policy       *policy.Engine
	capabilities *capabilities.Supervisor
	skills       *skills.Registry
	mcp          *mcp.Manager
	sandboxes    *sandbox.Manager
	integrations *integrations.Manager
	calendar     *calendar.Manager
	mail         *mail.Manager
	computerUse  *computeruse.Manager
	delivery     *delivery.Manager
	billing      *billing.Manager
	eventBus     *events.Bus
	store        *store.SQLiteStore
	checkpoints  *checkpoints.Manager
}

func NewScheduleWorkflowLauncher(deps ScheduleWorkflowLauncherDependencies) *ScheduleWorkflowLauncher {
	return &ScheduleWorkflowLauncher{
		cfg:          deps.Config,
		runtime:      deps.Runtime,
		policy:       deps.Policy,
		capabilities: deps.Capabilities,
		skills:       deps.Skills,
		mcp:          deps.MCP,
		sandboxes:    deps.Sandboxes,
		integrations: deps.Integrations,
		calendar:     deps.Calendar,
		mail:         deps.Mail,
		computerUse:  deps.ComputerUse,
		delivery:     deps.Delivery,
		billing:      deps.Billing,
		eventBus:     deps.EventBus,
		store:        deps.Store,
		checkpoints:  deps.Checkpoints,
	}
}

func (l *ScheduleWorkflowLauncher) LaunchScheduledWorkflow(ctx context.Context, target scheduler.WorkflowTarget, scheduleID, scheduleAttemptID string) (scheduler.WorkflowLaunchResult, error) {
	result, err := l.launchWorkflow(ctx, target, scheduleID, scheduleAttemptID, "", "")
	if err != nil {
		return scheduler.WorkflowLaunchResult{}, err
	}
	return scheduler.WorkflowLaunchResult{
		RunID:            result.RunID,
		WorkflowID:       result.WorkflowID,
		DownstreamStatus: mapWorkflowStatusToSchedule(result.Status),
	}, nil
}

func (l *ScheduleWorkflowLauncher) LaunchReminderWorkflow(ctx context.Context, cfg reminders.WorkflowLaunchConfig, reminderID, occurrenceID string) (reminders.WorkflowLaunchResult, error) {
	result, err := l.launchWorkflow(ctx, scheduler.WorkflowTarget{
		SessionID:      cfg.SessionID,
		Entrypoint:     cfg.Entrypoint,
		RunGoal:        cfg.RunGoal,
		WorkflowGoal:   cfg.WorkflowGoal,
		CalendarAction: cfg.CalendarAction,
		MailAction:     cfg.MailAction,
	}, "", "", reminderID, occurrenceID)
	if err != nil {
		return reminders.WorkflowLaunchResult{}, err
	}
	if result.Status == orchestration.WorkflowStatusPlanningFailed {
		return reminders.WorkflowLaunchResult{}, fmt.Errorf("workflow planning failed to start")
	}
	return reminders.WorkflowLaunchResult{RunID: result.RunID, WorkflowID: result.WorkflowID}, nil
}

type backgroundWorkflowLaunchResult struct {
	RunID      string
	WorkflowID string
	Status     orchestration.WorkflowStatus
}

func (l *ScheduleWorkflowLauncher) launchWorkflow(ctx context.Context, target scheduler.WorkflowTarget, scheduleID, scheduleAttemptID, reminderID, occurrenceID string) (backgroundWorkflowLaunchResult, error) {
	if l == nil || l.runtime == nil || l.store == nil {
		return backgroundWorkflowLaunchResult{}, fmt.Errorf("workflow launcher is not configured")
	}
	if blocked, err := l.threadContinuationArchived(ctx, target.SessionID); err != nil {
		return backgroundWorkflowLaunchResult{}, err
	} else if blocked {
		return backgroundWorkflowLaunchResult{}, fmt.Errorf("thread_archived")
	}

	runInput := runtime.CreateRunInput{
		SessionID:            target.SessionID,
		ScheduleID:           scheduleID,
		ScheduleAttemptID:    scheduleAttemptID,
		ReminderID:           reminderID,
		ReminderOccurrenceID: occurrenceID,
		Entrypoint:           target.Entrypoint,
		Goal:                 scheduleTargetGoal(target),
	}
	tenantID := ""
	if tenantContext, ok := tenantContextFromContext(ctx); ok {
		tenantID = strings.TrimSpace(tenantContext.TenantID)
	}
	var runReservation billing.UsageReservation
	var workflowReservation billing.UsageReservation
	workflowClientKey := backgroundWorkflowClientKey(scheduleID, scheduleAttemptID, reminderID, occurrenceID)
	if l.billing != nil && tenantID != "" {
		runInput.RunID = runtime.NewRunID()
		reserveAll, reserveErr := l.billing.ReserveAll(ctx, []billing.ReserveInput{
			{
				TenantID:          tenantID,
				Category:          billing.CategoryRunLaunches,
				Amount:            1,
				OperationKey:      billing.RunOperationKey(tenantID, workflowClientKey, runInput.RunID),
				ReservationPoint:  "background workflow launch before runtime.CreateRun",
				GuardedEntryPoint: "background workflow launch",
				Hosted:            l.cfg.Environment == config.EnvironmentProd,
			},
			{
				TenantID:          tenantID,
				Category:          billing.CategoryWorkflowLaunches,
				Amount:            1,
				OperationKey:      billing.WorkflowOperationKey(tenantID, runInput.RunID, "", workflowClientKey),
				ReservationPoint:  "background workflow launch before runtime.CreateRun",
				GuardedEntryPoint: "background workflow launch",
				Hosted:            l.cfg.Environment == config.EnvironmentProd,
			},
		})
		if reserveErr != nil {
			return backgroundWorkflowLaunchResult{}, reserveErr
		}
		if len(reserveAll.Results) > 0 {
			runReservation = reserveAll.Results[0].Reservation
		}
		if len(reserveAll.Results) > 1 {
			workflowReservation = reserveAll.Results[1].Reservation
		}
	}
	run, err := l.runtime.CreateRun(runInput)
	if err != nil {
		releaseBillingReservation(ctx, l.billing, runReservation, "background workflow run creation failed before persistence")
		releaseBillingReservation(ctx, l.billing, workflowReservation, "background workflow run creation failed before persistence")
		return backgroundWorkflowLaunchResult{}, err
	}
	if err := l.store.UpsertRun(ctx, run); err != nil {
		releaseBillingReservation(ctx, l.billing, runReservation, "background workflow run persistence failed before commit")
		releaseBillingReservation(ctx, l.billing, workflowReservation, "background workflow run persistence failed before commit")
		return backgroundWorkflowLaunchResult{}, err
	}
	if _, _, err := l.store.SaveThreadRuntimeProjectionForRun(ctx, run.RunID, threads.RuntimeProjectionInput{
		ProjectionID:    "rtp_run_" + run.RunID,
		ResourceKind:    threads.RuntimeResourceRun,
		ResourceID:      run.RunID,
		Status:          string(run.Status),
		ReasonCode:      "background_workflow_launch",
		OccurredAt:      run.CreatedAt,
		Route:           "/v1/runs/" + run.RunID,
		SafeSummary:     "Background workflow run " + string(run.Status),
		RedactionStatus: threads.RedactionStatusRedacted,
	}); err != nil {
		releaseBillingReservation(ctx, l.billing, workflowReservation, "background workflow run projection failed before workflow persistence")
		return backgroundWorkflowLaunchResult{}, err
	}
	if runReservation.ReservationID != "" {
		if _, err := l.billing.Commit(ctx, billing.ResolveInput{
			TenantID:     runReservation.TenantID,
			Category:     runReservation.Category,
			OperationKey: runReservation.OperationKey,
			Amount:       runReservation.AmountReserved,
			ReasonCode:   "billing.background_workflow_run_committed",
			Reason:       "background workflow run persisted",
		}); err != nil {
			return backgroundWorkflowLaunchResult{}, err
		}
	}
	if err := persistCheckpoint(ctx, l.checkpoints, run.RunID); err != nil {
		releaseBillingReservation(ctx, l.billing, workflowReservation, "background workflow checkpoint failed before workflow persistence")
		return backgroundWorkflowLaunchResult{}, err
	}

	workflow := orchestration.NewManager().Plan(
		l.cfg,
		run,
		orchestration.CreateWorkflowInput{Goal: target.WorkflowGoal, CalendarAction: target.CalendarAction, MailAction: target.MailAction},
		l.capabilities,
		skillPlanningAdapter{registry: l.skills},
		mcpPlanningAdapter{manager: l.mcp},
	)
	workflow.ScheduleID = scheduleID
	workflow.ScheduleAttemptID = scheduleAttemptID
	workflow.ReminderID = reminderID
	workflow.ReminderOccurrenceID = occurrenceID
	if err := persistWorkflowDetail(ctx, l.store, workflow); err != nil {
		releaseBillingReservation(ctx, l.billing, workflowReservation, "background workflow persistence failed before execution")
		return backgroundWorkflowLaunchResult{}, err
	}
	if workflowReservation.ReservationID != "" && workflow.Status == orchestration.WorkflowStatusPlanningFailed {
		releaseBillingReservation(ctx, l.billing, workflowReservation, "background workflow planning failed before execution")
	}
	if workflowReservation.ReservationID != "" && workflow.Status != orchestration.WorkflowStatusPlanningFailed {
		if _, err := l.billing.Commit(ctx, billing.ResolveInput{
			TenantID:     workflowReservation.TenantID,
			Category:     workflowReservation.Category,
			OperationKey: workflowReservation.OperationKey,
			Amount:       workflowReservation.AmountReserved,
			ReasonCode:   "billing.background_workflow_launch_committed",
			Reason:       "background workflow persisted before execution",
		}); err != nil {
			return backgroundWorkflowLaunchResult{}, err
		}
	}
	if _, err := publishWorkflowEvent(ctx, l.eventBus, l.store, "workflow.planned", workflow, nil, nil); err != nil {
		return backgroundWorkflowLaunchResult{}, err
	}

	if workflow.Status == orchestration.WorkflowStatusPlanningFailed {
		return backgroundWorkflowLaunchResult{
			RunID:      run.RunID,
			WorkflowID: workflow.WorkflowID,
			Status:     workflow.Status,
		}, nil
	}

	workflow = orchestration.NewManager().InitializeExecution(workflow, time.Now().UTC())
	if err := persistWorkflowDetail(ctx, l.store, workflow); err != nil {
		return backgroundWorkflowLaunchResult{}, err
	}
	if _, err := publishWorkflowEvent(ctx, l.eventBus, l.store, "workflow.started", workflow, nil, nil); err != nil {
		return backgroundWorkflowLaunchResult{}, err
	}

	workflow, err = advanceWorkflowExecution(
		ctx,
		l.cfg,
		l.runtime,
		l.policy,
		l.capabilities,
		l.skills,
		l.mcp,
		l.sandboxes,
		l.integrations,
		l.calendar,
		l.mail,
		l.eventBus,
		l.delivery,
		l.billing,
		l.store,
		l.checkpoints,
		l.computerUse,
		workflow,
	)
	if err != nil {
		return backgroundWorkflowLaunchResult{}, err
	}

	return backgroundWorkflowLaunchResult{
		RunID:      run.RunID,
		WorkflowID: workflow.WorkflowID,
		Status:     workflow.Status,
	}, nil
}

func (l *ScheduleWorkflowLauncher) threadContinuationArchived(ctx context.Context, sessionID string) (bool, error) {
	if l == nil || l.store == nil || strings.TrimSpace(sessionID) == "" {
		return false, nil
	}
	thread, _, found, err := l.store.GetThreadForSession(ctx, strings.TrimSpace(sessionID))
	if err != nil || !found {
		return false, err
	}
	return thread.LifecycleState == threads.LifecycleStateArchived, nil
}

func backgroundWorkflowClientKey(scheduleID, scheduleAttemptID, reminderID, occurrenceID string) string {
	switch {
	case strings.TrimSpace(scheduleID) != "":
		return "schedule:" + strings.TrimSpace(scheduleID) + ":" + strings.TrimSpace(scheduleAttemptID)
	case strings.TrimSpace(reminderID) != "":
		return "reminder:" + strings.TrimSpace(reminderID) + ":" + strings.TrimSpace(occurrenceID)
	default:
		return ""
	}
}

func scheduleTargetGoal(target scheduler.WorkflowTarget) string {
	if target.RunGoal != "" {
		return target.RunGoal
	}
	if target.WorkflowGoal != "" {
		return target.WorkflowGoal
	}
	return target.Entrypoint
}

func mapWorkflowStatusToSchedule(status orchestration.WorkflowStatus) scheduler.DownstreamStatus {
	switch status {
	case orchestration.WorkflowStatusCompleted:
		return scheduler.DownstreamStatusCompleted
	case orchestration.WorkflowStatusPlanningFailed, orchestration.WorkflowStatusFailed, orchestration.WorkflowStatusPartialFailed, orchestration.WorkflowStatusBlocked:
		return scheduler.DownstreamStatusFailed
	case orchestration.WorkflowStatusCancelled:
		return scheduler.DownstreamStatusCancelled
	case orchestration.WorkflowStatusInterrupted:
		return scheduler.DownstreamStatusInterrupted
	default:
		return scheduler.DownstreamStatusRunning
	}
}
