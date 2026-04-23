package api

import (
	"context"
	"fmt"
	"time"

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

	run, err := l.runtime.CreateRun(runtime.CreateRunInput{
		SessionID:            target.SessionID,
		ScheduleID:           scheduleID,
		ScheduleAttemptID:    scheduleAttemptID,
		ReminderID:           reminderID,
		ReminderOccurrenceID: occurrenceID,
		Entrypoint:           target.Entrypoint,
		Goal:                 scheduleTargetGoal(target),
	})
	if err != nil {
		return backgroundWorkflowLaunchResult{}, err
	}
	if err := l.store.UpsertRun(ctx, run); err != nil {
		return backgroundWorkflowLaunchResult{}, err
	}
	if err := persistCheckpoint(ctx, l.checkpoints, run.RunID); err != nil {
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
		return backgroundWorkflowLaunchResult{}, err
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
