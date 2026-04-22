package api

import (
	"context"
	"fmt"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/mcp"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
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
	ComputerUse  *computeruse.Manager
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
	computerUse  *computeruse.Manager
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
		computerUse:  deps.ComputerUse,
		eventBus:     deps.EventBus,
		store:        deps.Store,
		checkpoints:  deps.Checkpoints,
	}
}

func (l *ScheduleWorkflowLauncher) LaunchScheduledWorkflow(ctx context.Context, target scheduler.WorkflowTarget, scheduleID, scheduleAttemptID string) (scheduler.WorkflowLaunchResult, error) {
	if l == nil || l.runtime == nil || l.store == nil {
		return scheduler.WorkflowLaunchResult{}, fmt.Errorf("workflow launcher is not configured")
	}

	run, err := l.runtime.CreateRun(runtime.CreateRunInput{
		SessionID:         target.SessionID,
		ScheduleID:        scheduleID,
		ScheduleAttemptID: scheduleAttemptID,
		Entrypoint:        target.Entrypoint,
		Goal:              scheduleTargetGoal(target),
	})
	if err != nil {
		return scheduler.WorkflowLaunchResult{}, err
	}
	if err := l.store.UpsertRun(ctx, run); err != nil {
		return scheduler.WorkflowLaunchResult{}, err
	}
	if err := persistCheckpoint(ctx, l.checkpoints, run.RunID); err != nil {
		return scheduler.WorkflowLaunchResult{}, err
	}

	workflow := orchestration.NewManager().Plan(
		l.cfg,
		run,
		orchestration.CreateWorkflowInput{Goal: target.WorkflowGoal},
		l.capabilities,
		skillPlanningAdapter{registry: l.skills},
		mcpPlanningAdapter{manager: l.mcp},
	)
	workflow.ScheduleID = scheduleID
	workflow.ScheduleAttemptID = scheduleAttemptID
	if err := persistWorkflowDetail(ctx, l.store, workflow); err != nil {
		return scheduler.WorkflowLaunchResult{}, err
	}
	if _, err := publishWorkflowEvent(ctx, l.eventBus, l.store, "workflow.planned", workflow, nil, nil); err != nil {
		return scheduler.WorkflowLaunchResult{}, err
	}

	if workflow.Status == orchestration.WorkflowStatusPlanningFailed {
		return scheduler.WorkflowLaunchResult{
			RunID:            run.RunID,
			WorkflowID:       workflow.WorkflowID,
			DownstreamStatus: scheduler.DownstreamStatusFailed,
		}, nil
	}

	workflow = orchestration.NewManager().InitializeExecution(workflow, time.Now().UTC())
	if err := persistWorkflowDetail(ctx, l.store, workflow); err != nil {
		return scheduler.WorkflowLaunchResult{}, err
	}
	if _, err := publishWorkflowEvent(ctx, l.eventBus, l.store, "workflow.started", workflow, nil, nil); err != nil {
		return scheduler.WorkflowLaunchResult{}, err
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
		l.eventBus,
		l.store,
		l.checkpoints,
		l.computerUse,
		workflow,
	)
	if err != nil {
		return scheduler.WorkflowLaunchResult{}, err
	}

	return scheduler.WorkflowLaunchResult{
		RunID:            run.RunID,
		WorkflowID:       workflow.WorkflowID,
		DownstreamStatus: mapWorkflowStatusToSchedule(workflow.Status),
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
