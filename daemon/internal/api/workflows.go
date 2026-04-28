package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
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
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/skills"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type mcpPlanningAdapter struct {
	manager *mcp.Manager
}

type skillPlanningAdapter struct {
	registry *skills.Registry
}

func (a skillPlanningAdapter) ListSkills() []orchestration.SkillPlanningCandidate {
	if a.registry == nil {
		return nil
	}
	skillsList := a.registry.List()
	items := make([]orchestration.SkillPlanningCandidate, 0, len(skillsList))
	for _, skill := range skillsList {
		item := orchestration.SkillPlanningCandidate{
			SkillID:    skill.SkillID,
			Executable: skill.ExecutionManifest != nil,
			Available:  skill.AvailabilityStatus == skills.SkillAvailabilityStatusAvailable,
		}
		if skill.ExecutionManifest != nil {
			item.ApprovalModeExpected = string(skill.ExecutionManifest.ApprovalMode)
		}
		items = append(items, item)
	}
	return items
}

func (a mcpPlanningAdapter) ListServers() []orchestration.MCPPlanningServer {
	if a.manager == nil {
		return nil
	}
	servers := a.manager.ListServers()
	items := make([]orchestration.MCPPlanningServer, 0, len(servers))
	for _, server := range servers {
		item := orchestration.MCPPlanningServer{ServerID: server.ServerID}
		for _, tool := range server.Tools {
			item.Tools = append(item.Tools, orchestration.MCPPlanningTool{ToolName: tool.ToolName})
		}
		items = append(items, item)
	}
	return items
}

func (a mcpPlanningAdapter) ListTools(serverID string) ([]orchestration.MCPPlanningTool, error) {
	if a.manager == nil {
		return nil, nil
	}
	tools, err := a.manager.ListTools(serverID)
	if err != nil {
		return nil, err
	}
	items := make([]orchestration.MCPPlanningTool, 0, len(tools))
	for _, tool := range tools {
		items = append(items, orchestration.MCPPlanningTool{ToolName: tool.ToolName})
	}
	return items, nil
}

func handleRunWorkflows(cfg config.Config, manager *runtime.Manager, policyEngine *policy.Engine, capabilitySupervisor *capabilities.Supervisor, skillRegistry *skills.Registry, mcpManager *mcp.Manager, sandboxManager *sandbox.Manager, integrationsManager *integrations.Manager, calendarManager *calendar.Manager, mailManager *mail.Manager, eventBus *events.Bus, deliveryManager *delivery.Manager, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, computerUseManager *computeruse.Manager, w http.ResponseWriter, r *http.Request, runID string) {
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "store is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := sqliteStore.ListWorkflows(r.Context(), string(cfg.Environment), runID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items, err = projectWorkflowDeliverySummaries(r.Context(), deliveryManager, items)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items, err = projectWorkflowsCalendarSummaries(r.Context(), sqliteStore, items)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items, err = projectWorkflowsMailSummaries(r.Context(), sqliteStore, items)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, WorkflowListResponse{Items: items})
	case http.MethodPost:
		run, ok := manager.GetRun(runID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		var input CreateWorkflowRequest
		if err := decodeJSONBody(r, &input); err != nil && !errors.Is(err, http.ErrBodyNotAllowed) && !strings.Contains(err.Error(), "EOF") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		calendarAction, err := buildCalendarAction(input.CalendarAction)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		mailAction, err := buildMailAction(input.MailAction)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		workflow := orchestration.NewManager().Plan(cfg, run, orchestration.CreateWorkflowInput{Goal: input.Goal, CalendarAction: calendarAction, MailAction: mailAction}, capabilitySupervisor, skillPlanningAdapter{registry: skillRegistry}, mcpPlanningAdapter{manager: mcpManager})
		if err := persistWorkflowDetail(r.Context(), sqliteStore, workflow); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := publishWorkflowEvent(r.Context(), eventBus, sqliteStore, "workflow.planned", workflow, nil, nil); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, workflow)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleRunWorkflowByID(deliveryManager *delivery.Manager, sqliteStore *store.SQLiteStore, environment config.Environment, w http.ResponseWriter, r *http.Request, runID, workflowID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	workflow, ok, err := sqliteStore.GetWorkflow(r.Context(), string(environment), runID, workflowID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	workflow, err = projectWorkflowDeliverySummary(r.Context(), deliveryManager, workflow)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	workflow, err = projectWorkflowCalendarSummaries(r.Context(), sqliteStore, workflow)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	workflow, err = projectWorkflowMailSummaries(r.Context(), sqliteStore, workflow)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workflow)
}

func handleRunWorkflowStart(cfg config.Config, manager *runtime.Manager, policyEngine *policy.Engine, capabilitySupervisor *capabilities.Supervisor, skillRegistry *skills.Registry, mcpManager *mcp.Manager, sandboxManager *sandbox.Manager, integrationsManager *integrations.Manager, calendarManager *calendar.Manager, mailManager *mail.Manager, eventBus *events.Bus, deliveryManager *delivery.Manager, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, computerUseManager *computeruse.Manager, w http.ResponseWriter, r *http.Request, runID, workflowID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	workflow, ok, err := sqliteStore.GetWorkflow(r.Context(), string(cfg.Environment), runID, workflowID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	if workflow.Status == orchestration.WorkflowStatusPlanningFailed {
		writeError(w, http.StatusConflict, "workflow planning failed")
		return
	}
	if workflow.Status != orchestration.WorkflowStatusPlanned && workflow.Status != orchestration.WorkflowStatusBlocked {
		writeError(w, http.StatusConflict, "workflow is not startable")
		return
	}
	workflow = orchestration.NewManager().InitializeExecution(workflow, time.Now().UTC())
	if err := persistWorkflowDetail(r.Context(), sqliteStore, workflow); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishWorkflowEvent(r.Context(), eventBus, sqliteStore, "workflow.started", workflow, nil, nil); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	workflow, err = advanceWorkflowExecution(r.Context(), cfg, manager, policyEngine, capabilitySupervisor, skillRegistry, mcpManager, sandboxManager, integrationsManager, calendarManager, mailManager, eventBus, deliveryManager, sqliteStore, checkpointManager, computerUseManager, workflow)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	workflow, err = projectWorkflowCalendarSummaries(r.Context(), sqliteStore, workflow)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	workflow, err = projectWorkflowMailSummaries(r.Context(), sqliteStore, workflow)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workflow)
}

func handleRunWorkflowCancel(cfg config.Config, manager *runtime.Manager, sandboxManager *sandbox.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, w http.ResponseWriter, r *http.Request, runID, workflowID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	workflow, ok, err := sqliteStore.GetWorkflow(r.Context(), string(cfg.Environment), runID, workflowID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	now := time.Now().UTC()
	workflow.Status = orchestration.WorkflowStatusCancelled
	workflow.UpdatedAt = now
	workflow.CompletedAt = &now
	for idx := range workflow.Steps {
		if orchestration.IsTerminalStepStatus(workflow.Steps[idx].Status) {
			continue
		}
		workflow.Steps[idx].Status = orchestration.StepStatusCancelled
		workflow.Steps[idx].UpdatedAt = now
		if workflow.Steps[idx].RuntimeStepID != "" {
			if step, runUpdate, _, cancelErr := manager.CancelStep(runID, workflow.Steps[idx].RuntimeStepID); cancelErr == nil {
				_ = persistStepCancelMutation(r.Context(), sqliteStore, checkpointManager, step, runUpdate)
			}
		}
		if workflow.Steps[idx].ActiveToolCallID != "" {
			if toolCall, exists := manager.GetToolCall(runID, workflow.Steps[idx].RuntimeStepID, workflow.Steps[idx].ActiveToolCallID); exists && toolCall.SandboxExecutionID != "" && sandboxManager != nil {
				_, _, _ = sandboxManager.CancelExecution(toolCall.SandboxExecutionID)
			}
		}
	}
	if err := persistWorkflowDetail(r.Context(), sqliteStore, workflow); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := publishWorkflowEvent(r.Context(), eventBus, sqliteStore, "workflow.status_changed", workflow, nil, map[string]any{"status": workflow.Status}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	workflow, err = projectWorkflowCalendarSummaries(r.Context(), sqliteStore, workflow)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	workflow, err = projectWorkflowMailSummaries(r.Context(), sqliteStore, workflow)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workflow)
}

func advanceWorkflowExecution(ctx context.Context, cfg config.Config, manager *runtime.Manager, policyEngine *policy.Engine, capabilitySupervisor *capabilities.Supervisor, skillRegistry *skills.Registry, mcpManager *mcp.Manager, sandboxManager *sandbox.Manager, integrationsManager *integrations.Manager, calendarManager *calendar.Manager, mailManager *mail.Manager, eventBus *events.Bus, deliveryManager *delivery.Manager, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, computerUseManager *computeruse.Manager, workflow orchestration.Workflow) (orchestration.Workflow, error) {
	workflowManager := orchestration.NewManager()
	for {
		now := time.Now().UTC()
		var changed bool
		workflow, changed = workflowManager.AdvanceReadySteps(workflow, now)
		if changed {
			if err := persistWorkflowDetail(ctx, sqliteStore, workflow); err != nil {
				return orchestration.Workflow{}, err
			}
		}
		progressed := false
		for idx := range workflow.Steps {
			if workflow.Steps[idx].Status != orchestration.StepStatusReady {
				continue
			}
			nextWorkflow, terminalSync, err := startWorkflowStepExecution(ctx, cfg, manager, policyEngine, capabilitySupervisor, skillRegistry, mcpManager, sandboxManager, integrationsManager, calendarManager, mailManager, eventBus, deliveryManager, sqliteStore, checkpointManager, computerUseManager, workflow, workflow.Steps[idx])
			if err != nil {
				return orchestration.Workflow{}, err
			}
			workflow = nextWorkflow
			progressed = true
			if !terminalSync {
				workflow = workflowManager.ReconcileStatus(workflow, time.Now().UTC())
				if err := persistWorkflowDetail(ctx, sqliteStore, workflow); err != nil {
					return orchestration.Workflow{}, err
				}
				return workflow, nil
			}
		}
		workflow = workflowManager.ReconcileStatus(workflow, time.Now().UTC())
		if err := persistWorkflowDetail(ctx, sqliteStore, workflow); err != nil {
			return orchestration.Workflow{}, err
		}
		if !progressed {
			return workflow, nil
		}
	}
}

func startWorkflowStepExecution(ctx context.Context, cfg config.Config, manager *runtime.Manager, policyEngine *policy.Engine, capabilitySupervisor *capabilities.Supervisor, skillRegistry *skills.Registry, mcpManager *mcp.Manager, sandboxManager *sandbox.Manager, integrationsManager *integrations.Manager, calendarManager *calendar.Manager, mailManager *mail.Manager, eventBus *events.Bus, deliveryManager *delivery.Manager, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, computerUseManager *computeruse.Manager, workflow orchestration.Workflow, wfStep orchestration.WorkflowStep) (orchestration.Workflow, bool, error) {
	if wfStep.ConsumerKind == "computer_use" {
		return executeWorkflowComputerUseStep(ctx, cfg, manager, policyEngine, capabilitySupervisor, skillRegistry, mcpManager, sandboxManager, integrationsManager, calendarManager, mailManager, eventBus, deliveryManager, sqliteStore, checkpointManager, computerUseManager, workflow, wfStep)
	}
	runtimeStep, err := manager.CreateStep(workflow.RunID, runtime.CreateStepInput{
		Title:          wfStep.Title,
		Kind:           "workflow",
		WorkflowID:     workflow.WorkflowID,
		WorkflowStepID: wfStep.WorkflowStepID,
		Attempt:        wfStep.AttemptCount + 1,
		Input:          wfStep.Input,
	})
	if err != nil {
		return workflow, false, err
	}
	runtimeStep, runUpdate, err := manager.UpdateStepStatusAndReconcileRun(workflow.RunID, runtimeStep.StepID, runtime.UpdateStepStatusInput{Status: runtime.StepStatusPlanning})
	if err != nil {
		return workflow, false, err
	}
	if err := persistStep(ctx, sqliteStore, runtimeStep); err != nil {
		return workflow, false, err
	}
	if runUpdate != nil {
		if err := persistRun(ctx, sqliteStore, *runUpdate); err != nil {
			return workflow, false, err
		}
	}
	runtimeStep, runUpdate, err = manager.UpdateStepStatusAndReconcileRun(workflow.RunID, runtimeStep.StepID, runtime.UpdateStepStatusInput{Status: runtime.StepStatusExecutingTool})
	if err != nil {
		return workflow, false, err
	}
	if err := persistStep(ctx, sqliteStore, runtimeStep); err != nil {
		return workflow, false, err
	}
	if runUpdate != nil {
		if err := persistRun(ctx, sqliteStore, *runUpdate); err != nil {
			return workflow, false, err
		}
	}
	if err := persistCheckpoint(ctx, checkpointManager, workflow.RunID); err != nil {
		return workflow, false, err
	}
	workflow = orchestration.NewManager().StartStepAttempt(workflow, wfStep.WorkflowStepID, runtimeStep.StepID, time.Now().UTC())

	switch wfStep.ConsumerKind {
	case "calendar":
		toolCall, stepStatus, blockedReason, err := executeWorkflowCalendarStep(ctx, manager, integrationsManager, calendarManager, eventBus, sqliteStore, checkpointManager, workflow, runtimeStep, wfStep)
		if err != nil {
			return workflow, false, err
		}
		return advanceWorkflowAfterToolCall(ctx, cfg, manager, policyEngine, capabilitySupervisor, skillRegistry, mcpManager, sandboxManager, integrationsManager, calendarManager, mailManager, eventBus, deliveryManager, sqliteStore, checkpointManager, computerUseManager, workflow, toolCall, stepStatus, blockedReason)
	case "mail":
		toolCall, stepStatus, blockedReason, err := executeWorkflowMailStep(ctx, manager, integrationsManager, mailManager, eventBus, sqliteStore, checkpointManager, workflow, runtimeStep, wfStep)
		if err != nil {
			return workflow, false, err
		}
		return advanceWorkflowAfterToolCall(ctx, cfg, manager, policyEngine, capabilitySupervisor, skillRegistry, mcpManager, sandboxManager, integrationsManager, calendarManager, mailManager, eventBus, deliveryManager, sqliteStore, checkpointManager, computerUseManager, workflow, toolCall, stepStatus, blockedReason)
	case string(runtime.ToolCallInvocationKindMCPTool):
		toolCall, stepStatus, blockedReason, err := executeWorkflowMCPTool(ctx, manager, mcpManager, eventBus, sqliteStore, checkpointManager, workflow, runtimeStep, wfStep)
		if err != nil {
			return workflow, false, err
		}
		return advanceWorkflowAfterToolCall(ctx, cfg, manager, policyEngine, capabilitySupervisor, skillRegistry, mcpManager, sandboxManager, integrationsManager, calendarManager, mailManager, eventBus, deliveryManager, sqliteStore, checkpointManager, computerUseManager, workflow, toolCall, stepStatus, blockedReason)
	case string(runtime.ToolCallInvocationKindSkill):
		toolCall, terminalSync, stepStatus, blockedReason, err := executeWorkflowSkillTool(ctx, cfg, manager, policyEngine, skillRegistry, sandboxManager, integrationsManager, calendarManager, mailManager, eventBus, deliveryManager, sqliteStore, checkpointManager, workflow, runtimeStep, wfStep)
		if err != nil {
			return workflow, false, err
		}
		if terminalSync {
			return advanceWorkflowAfterToolCall(ctx, cfg, manager, policyEngine, capabilitySupervisor, skillRegistry, mcpManager, sandboxManager, integrationsManager, calendarManager, mailManager, eventBus, deliveryManager, sqliteStore, checkpointManager, computerUseManager, workflow, toolCall, stepStatus, blockedReason)
		}
		workflow = orchestration.NewManager().BindToolCall(workflow, wfStep.WorkflowStepID, toolCall, time.Now().UTC())
		return workflow, false, persistWorkflowDetail(ctx, sqliteStore, workflow)
	default:
		toolCall, terminalSync, stepStatus, blockedReason, err := executeWorkflowCapabilityTool(ctx, cfg, manager, policyEngine, capabilitySupervisor, sandboxManager, integrationsManager, calendarManager, mailManager, eventBus, deliveryManager, sqliteStore, checkpointManager, workflow, runtimeStep, wfStep)
		if err != nil {
			return workflow, false, err
		}
		if terminalSync {
			return advanceWorkflowAfterToolCall(ctx, cfg, manager, policyEngine, capabilitySupervisor, skillRegistry, mcpManager, sandboxManager, integrationsManager, calendarManager, mailManager, eventBus, deliveryManager, sqliteStore, checkpointManager, computerUseManager, workflow, toolCall, stepStatus, blockedReason)
		}
		workflow = orchestration.NewManager().BindToolCall(workflow, wfStep.WorkflowStepID, toolCall, time.Now().UTC())
		return workflow, false, persistWorkflowDetail(ctx, sqliteStore, workflow)
	}
}

func executeWorkflowComputerUseStep(ctx context.Context, cfg config.Config, manager *runtime.Manager, policyEngine *policy.Engine, capabilitySupervisor *capabilities.Supervisor, skillRegistry *skills.Registry, mcpManager *mcp.Manager, sandboxManager *sandbox.Manager, integrationsManager *integrations.Manager, calendarManager *calendar.Manager, mailManager *mail.Manager, eventBus *events.Bus, deliveryManager *delivery.Manager, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, computerUseManager *computeruse.Manager, workflow orchestration.Workflow, wfStep orchestration.WorkflowStep) (orchestration.Workflow, bool, error) {
	if computerUseManager == nil {
		return workflow, false, errors.New("computer-use manager is not configured")
	}
	inputMap, ok := wfStep.Input.(map[string]any)
	if !ok {
		return workflow, false, errors.New("computer-use workflow input must be an object")
	}
	session, _, err := computerUseManager.AcquireSession(ctx, workflow.RunID, computeruse.CreateSessionInput{
		WorkflowID:     workflow.WorkflowID,
		WorkflowStepID: wfStep.WorkflowStepID,
		DriverKind:     mapString(inputMap["driverKind"]),
		InitialURL:     mapString(inputMap["initialUrl"]),
	})
	if err != nil {
		return workflow, false, err
	}
	workflow = orchestration.NewManager().ApplyComputerUseProjection(workflow, wfStep.WorkflowStepID, session.ComputerUseSessionID, nil, nil, time.Now().UTC())
	if err := persistWorkflowDetail(ctx, sqliteStore, workflow); err != nil {
		return workflow, false, err
	}

	actionPayloads := mapSlice(inputMap["actions"])
	if len(actionPayloads) == 0 {
		return workflow, false, errors.New("computer-use workflow step requires at least one action")
	}

	actionIDs := make([]string, 0, len(actionPayloads))
	var (
		lastAction runtime.ToolCall
		lastStepID string
		artifacts  []computeruse.Artifact
	)
	for idx, raw := range actionPayloads {
		actionInput := decodeWorkflowComputerUseAction(raw)
		result, approval, decision, err := computerUseManager.CreateAction(ctx, workflow.RunID, session.ComputerUseSessionID, "workflow:"+workflow.WorkflowID, actionInput)
		if err != nil {
			return workflow, false, err
		}
		if idx == 0 {
			workflow = orchestration.NewManager().StartStepAttempt(workflow, wfStep.WorkflowStepID, result.Action.StepID, time.Now().UTC())
		}
		actionIDs = append(actionIDs, result.Action.ComputerUseActionID)
		artifacts = append(artifacts, result.Action.Artifacts...)
		workflow = orchestration.NewManager().ApplyComputerUseProjection(workflow, wfStep.WorkflowStepID, session.ComputerUseSessionID, actionIDs, artifacts, time.Now().UTC())
		if err := persistComputerUseRuntimeTracking(ctx, sqliteStore, manager, checkpointManager, result.Action); err != nil {
			return workflow, false, err
		}
		publishComputerUseArtifacts(ctx, eventBus, sqliteStore, result.Action)
		if result.Action.FailureClass == string(computeruse.FailureClassTargetMismatch) {
			publishComputerUseTargetMismatch(ctx, eventBus, sqliteStore, result.Action)
		}
		if approval != nil {
			if err := persistApproval(ctx, sqliteStore, *approval); err != nil {
				return workflow, false, err
			}
		}
		if decision != nil {
			if err := persistDecision(ctx, sqliteStore, *decision); err != nil {
				return workflow, false, err
			}
		}
		toolCall, ok := manager.GetToolCall(result.Action.RunID, result.Action.StepID, result.Action.ToolCallID)
		if ok {
			lastAction = toolCall
			lastStepID = result.Action.StepID
			workflow = orchestration.NewManager().BindToolCall(workflow, wfStep.WorkflowStepID, toolCall, time.Now().UTC())
		}
		if result.Pending {
			if err := persistWorkflowDetail(ctx, sqliteStore, workflow); err != nil {
				return workflow, false, err
			}
			if ok {
				return advanceWorkflowAfterToolCall(ctx, cfg, manager, policyEngine, capabilitySupervisor, skillRegistry, mcpManager, sandboxManager, integrationsManager, calendarManager, mailManager, eventBus, deliveryManager, sqliteStore, checkpointManager, computerUseManager, workflow, lastAction, orchestration.StepStatusBlocked, string(orchestration.BlockedReasonApprovalDenied))
			}
			return workflow, false, nil
		}
	}
	if lastStepID == "" {
		return workflow, false, errors.New("computer-use workflow step did not create runtime linkage")
	}
	if err := persistWorkflowDetail(ctx, sqliteStore, workflow); err != nil {
		return workflow, false, err
	}
	return advanceWorkflowAfterToolCall(ctx, cfg, manager, policyEngine, capabilitySupervisor, skillRegistry, mcpManager, sandboxManager, integrationsManager, calendarManager, mailManager, eventBus, deliveryManager, sqliteStore, checkpointManager, computerUseManager, workflow, lastAction, orchestration.StepStatusCompleted, "")
}

func executeWorkflowMCPTool(ctx context.Context, manager *runtime.Manager, mcpManager *mcp.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, workflow orchestration.Workflow, runtimeStep runtime.Step, wfStep orchestration.WorkflowStep) (runtime.ToolCall, orchestration.StepStatus, string, error) {
	authorization, err := mcpManager.AuthorizeTool(ctx, wfStep.ConsumerID, wfStep.ToolName, mcp.AuthorizeToolInput{
		RuntimeSurface: "chat",
		RequestedBy:    "workflow:" + workflow.WorkflowID,
	})
	if err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusBlocked, string(orchestration.BlockedReasonConsumerUnavailable), err
	}
	if authorization.Status != mcp.ToolAuthorizationStatusAllowed {
		stepStatus := orchestration.StepStatusBlocked
		blockedReason := string(orchestration.BlockedReasonPolicyBlocked)
		if authorization.Status == mcp.ToolAuthorizationStatusRejected || authorization.Status == mcp.ToolAuthorizationStatusPending {
			blockedReason = string(orchestration.BlockedReasonApprovalDenied)
		}
		toolCall, createErr := manager.CreateToolCall(workflow.RunID, runtimeStep.StepID, runtime.CreateToolCallInput{
			WorkflowID:          workflow.WorkflowID,
			WorkflowStepID:      wfStep.WorkflowStepID,
			Attempt:             wfStep.AttemptCount + 1,
			InvocationKind:      runtime.ToolCallInvocationKindMCPTool,
			MCPServerID:         wfStep.ConsumerID,
			MCPToolName:         wfStep.ToolName,
			ToolName:            wfStep.ToolName,
			AuthorizationResult: string(authorization.Status),
			Input:               wfStep.Input,
		})
		if createErr != nil {
			return runtime.ToolCall{}, stepStatus, blockedReason, createErr
		}
		if err := persistToolCall(ctx, sqliteStore, manager, toolCall); err != nil {
			return runtime.ToolCall{}, stepStatus, blockedReason, err
		}
		return toolCall, stepStatus, blockedReason, nil
	}
	server, ok := mcpManager.GetServerResource(wfStep.ConsumerID)
	if !ok {
		return runtime.ToolCall{}, orchestration.StepStatusBlocked, string(orchestration.BlockedReasonConsumerUnavailable), errors.New("mcp server not found")
	}
	toolCall, err := manager.CreateToolCall(workflow.RunID, runtimeStep.StepID, runtime.CreateToolCallInput{
		WorkflowID:          workflow.WorkflowID,
		WorkflowStepID:      wfStep.WorkflowStepID,
		Attempt:             wfStep.AttemptCount + 1,
		InvocationKind:      runtime.ToolCallInvocationKindMCPTool,
		MCPServerID:         server.ServerID,
		MCPServerName:       server.DisplayName,
		MCPToolName:         wfStep.ToolName,
		MCPTransportKind:    string(server.TransportKind),
		MCPSessionID:        authorization.SessionID,
		AuthorizationResult: string(authorization.Status),
		ToolName:            wfStep.ToolName,
		Input:               wfStep.Input,
		Sandbox:             consumerViewMap(authorization.Sandbox),
	})
	if err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
	}
	if err := persistToolCall(ctx, sqliteStore, manager, toolCall); err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
	}
	if _, err := publishToolCallEvent(ctx, eventBus, sqliteStore, "tool_call.requested", workflow.RunID, runtimeStep.StepID, toolCall); err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
	}
	result, err := mcpManager.CallTool(ctx, wfStep.ConsumerID, wfStep.ToolName, wfStep.Input, authorization)
	if err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
	}
	output := map[string]any{
		"transportKind": server.TransportKind,
		"sessionId":     result.SessionID,
	}
	if result.Output != nil {
		output["result"] = result.Output
	}
	if strings.TrimSpace(result.FailureClass) == "" {
		toolCall, err = manager.CompleteToolCall(workflow.RunID, runtimeStep.StepID, toolCall.ToolCallID, runtime.CompleteToolCallInput{
			Output:  output,
			Sandbox: consumerViewMap(authorization.Sandbox),
		})
		if err != nil {
			return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
		}
		_, _ = publishToolCallEvent(ctx, eventBus, sqliteStore, "tool_call.completed", workflow.RunID, runtimeStep.StepID, toolCall)
		return toolCall, orchestration.StepStatusCompleted, "", persistToolCall(ctx, sqliteStore, manager, toolCall)
	}
	toolCall, err = manager.FailToolCall(workflow.RunID, runtimeStep.StepID, toolCall.ToolCallID, runtime.FailToolCallInput{
		Output:       output,
		Error:        result.Error,
		FailureClass: result.FailureClass,
		Sandbox:      consumerViewMap(authorization.Sandbox),
	})
	if err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
	}
	_, _ = publishToolCallEvent(ctx, eventBus, sqliteStore, "tool_call.failed", workflow.RunID, runtimeStep.StepID, toolCall)
	return toolCall, orchestration.StepStatusFailed, "", persistToolCall(ctx, sqliteStore, manager, toolCall)
}

func executeWorkflowCalendarStep(ctx context.Context, manager *runtime.Manager, integrationsManager *integrations.Manager, calendarManager *calendar.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, workflow orchestration.Workflow, runtimeStep runtime.Step, wfStep orchestration.WorkflowStep) (runtime.ToolCall, orchestration.StepStatus, string, error) {
	action, err := decodeCalendarAction(wfStep.Input)
	if err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
	}
	toolCall, err := manager.CreateToolCall(workflow.RunID, runtimeStep.StepID, runtime.CreateToolCallInput{
		WorkflowID:     workflow.WorkflowID,
		WorkflowStepID: wfStep.WorkflowStepID,
		Attempt:        wfStep.AttemptCount + 1,
		InvocationKind: runtime.ToolCallInvocationKindDomainTool,
		DomainKind:     "calendar",
		ToolName:       string(action.OperationClass),
		Input:          action,
	})
	if err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
	}
	if err := persistToolCall(ctx, sqliteStore, manager, toolCall); err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
	}
	if _, err := publishToolCallEvent(ctx, eventBus, sqliteStore, "tool_call.requested", workflow.RunID, runtimeStep.StepID, toolCall); err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
	}
	result, execErr := executeCalendarAction(calendarManager, integrationsManager, action, calendar.SourceLinkage{
		RunID:             workflow.RunID,
		StepID:            runtimeStep.StepID,
		ToolCallID:        toolCall.ToolCallID,
		WorkflowID:        workflow.WorkflowID,
		WorkflowStepID:    wfStep.WorkflowStepID,
		ScheduleID:        workflow.ScheduleID,
		ScheduleAttemptID: workflow.ScheduleAttemptID,
	})
	if result.Operation.OperationID != "" {
		if err := recordCalendarActivity(ctx, eventBus, sqliteStore, result.Account, result.Operation, result.Artifacts); err != nil {
			return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
		}
	}
	bindings := calendarIntegrationBindings(integrationsManager, firstNonEmpty(result.Operation.IntegrationID, action.IntegrationID))
	output := calendarToolCallOutput(result)
	if execErr == nil {
		toolCall, err = manager.CompleteToolCall(workflow.RunID, runtimeStep.StepID, toolCall.ToolCallID, runtime.CompleteToolCallInput{
			Output:              output,
			IntegrationBindings: bindings,
		})
		if err != nil {
			return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
		}
		if err := persistToolCall(ctx, sqliteStore, manager, toolCall); err != nil {
			return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
		}
		_, _ = publishToolCallEvent(ctx, eventBus, sqliteStore, "tool_call.completed", workflow.RunID, runtimeStep.StepID, toolCall)
		return toolCall, orchestration.StepStatusCompleted, "", nil
	}
	toolCall, err = manager.FailToolCall(workflow.RunID, runtimeStep.StepID, toolCall.ToolCallID, runtime.FailToolCallInput{
		Output:              output,
		Error:               execErr.Error(),
		FailureClass:        calendarToolCallFailureClass(execErr, result.Operation),
		IntegrationBindings: bindings,
	})
	if err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
	}
	if err := persistToolCall(ctx, sqliteStore, manager, toolCall); err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
	}
	_, _ = publishToolCallEvent(ctx, eventBus, sqliteStore, "tool_call.failed", workflow.RunID, runtimeStep.StepID, toolCall)
	return toolCall, orchestration.StepStatusFailed, "", nil
}

func executeWorkflowMailStep(ctx context.Context, manager *runtime.Manager, integrationsManager *integrations.Manager, mailManager *mail.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, workflow orchestration.Workflow, runtimeStep runtime.Step, wfStep orchestration.WorkflowStep) (runtime.ToolCall, orchestration.StepStatus, string, error) {
	action, err := decodeMailAction(wfStep.Input)
	if err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
	}
	toolCall, err := manager.CreateToolCall(workflow.RunID, runtimeStep.StepID, runtime.CreateToolCallInput{
		WorkflowID:     workflow.WorkflowID,
		WorkflowStepID: wfStep.WorkflowStepID,
		Attempt:        wfStep.AttemptCount + 1,
		InvocationKind: runtime.ToolCallInvocationKindDomainTool,
		DomainKind:     "mail",
		ToolName:       string(action.OperationClass),
		Input:          action,
	})
	if err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
	}
	if err := persistToolCall(ctx, sqliteStore, manager, toolCall); err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
	}
	if _, err := publishToolCallEvent(ctx, eventBus, sqliteStore, "tool_call.requested", workflow.RunID, runtimeStep.StepID, toolCall); err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
	}
	result, execErr := executeMailAction(mailManager, integrationsManager, action, mail.SourceLinkage{
		RunID:                workflow.RunID,
		StepID:               runtimeStep.StepID,
		ToolCallID:           toolCall.ToolCallID,
		WorkflowID:           workflow.WorkflowID,
		WorkflowStepID:       wfStep.WorkflowStepID,
		ScheduleID:           workflow.ScheduleID,
		ScheduleAttemptID:    workflow.ScheduleAttemptID,
		AllowSendSideEffects: action.AllowSendSideEffects,
	})
	if result.Operation.OperationID != "" {
		if err := recordMailActivity(ctx, eventBus, sqliteStore, result.Account, result.Operation, result.Artifacts); err != nil {
			return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
		}
	}
	bindings := calendarIntegrationBindings(integrationsManager, firstNonEmpty(result.Operation.IntegrationID, action.IntegrationID))
	output := mailToolCallOutput(result)
	if execErr == nil {
		toolCall, err = manager.CompleteToolCall(workflow.RunID, runtimeStep.StepID, toolCall.ToolCallID, runtime.CompleteToolCallInput{
			Output:              output,
			IntegrationBindings: bindings,
		})
		if err != nil {
			return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
		}
		if err := persistToolCall(ctx, sqliteStore, manager, toolCall); err != nil {
			return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
		}
		_, _ = publishToolCallEvent(ctx, eventBus, sqliteStore, "tool_call.completed", workflow.RunID, runtimeStep.StepID, toolCall)
		return toolCall, orchestration.StepStatusCompleted, "", nil
	}
	toolCall, err = manager.FailToolCall(workflow.RunID, runtimeStep.StepID, toolCall.ToolCallID, runtime.FailToolCallInput{
		Output:              output,
		Error:               execErr.Error(),
		FailureClass:        mailToolCallFailureClass(execErr, result.Operation),
		IntegrationBindings: bindings,
	})
	if err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
	}
	if err := persistToolCall(ctx, sqliteStore, manager, toolCall); err != nil {
		return runtime.ToolCall{}, orchestration.StepStatusFailed, "", err
	}
	_, _ = publishToolCallEvent(ctx, eventBus, sqliteStore, "tool_call.failed", workflow.RunID, runtimeStep.StepID, toolCall)
	return toolCall, orchestration.StepStatusFailed, "", nil
}

func executeWorkflowSkillTool(ctx context.Context, cfg config.Config, manager *runtime.Manager, policyEngine *policy.Engine, skillRegistry *skills.Registry, sandboxManager *sandbox.Manager, integrationsManager *integrations.Manager, calendarManager *calendar.Manager, mailManager *mail.Manager, eventBus *events.Bus, deliveryManager *delivery.Manager, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, workflow orchestration.Workflow, runtimeStep runtime.Step, wfStep orchestration.WorkflowStep) (runtime.ToolCall, bool, orchestration.StepStatus, string, error) {
	request := createToolCallRequest{
		SkillID:  wfStep.ConsumerID,
		ToolName: wfStep.ToolName,
		Input:    wfStep.Input,
	}
	createInput, consumer, executionReq, approvalOutcome, err := prepareExecutableSkillToolCall(ctx, cfg, policyEngine, sqliteStore, eventBus, skillRegistry, nil, request, "workflow:"+workflow.WorkflowID)
	if err != nil {
		return runtime.ToolCall{}, false, orchestration.StepStatusFailed, "", err
	}
	createInput.WorkflowID = workflow.WorkflowID
	createInput.WorkflowStepID = wfStep.WorkflowStepID
	createInput.Attempt = wfStep.AttemptCount + 1
	if approvalOutcome != nil {
		toolCall, createErr := manager.CreateToolCall(workflow.RunID, runtimeStep.StepID, createInput)
		if createErr != nil {
			return runtime.ToolCall{}, false, orchestration.StepStatusBlocked, string(orchestration.BlockedReasonApprovalDenied), createErr
		}
		if err := persistToolCall(ctx, sqliteStore, manager, toolCall); err != nil {
			return runtime.ToolCall{}, false, orchestration.StepStatusBlocked, string(orchestration.BlockedReasonApprovalDenied), err
		}
		return toolCall, true, orchestration.StepStatusBlocked, string(orchestration.BlockedReasonApprovalDenied), nil
	}
	toolCall, err := manager.CreateToolCall(workflow.RunID, runtimeStep.StepID, createInput)
	if err != nil {
		return runtime.ToolCall{}, false, orchestration.StepStatusFailed, "", err
	}
	if consumer != nil && consumer.PolicyRecord != nil {
		consumer.PolicyRecord.ToolCallID = toolCall.ToolCallID
	}
	executionReq.Consumer = consumer
	toolCall.Sandbox = consumerViewMap(consumer)
	execution, err := sandboxManager.StartExecution(ctx, executionReq)
	if err != nil {
		return runtime.ToolCall{}, false, orchestration.StepStatusFailed, "", err
	}
	toolCall.SandboxExecutionID = execution.ExecutionID
	toolCall.Sandbox = consumerViewMap(execution.Consumer)
	if err := persistToolCall(ctx, sqliteStore, manager, toolCall); err != nil {
		return runtime.ToolCall{}, false, orchestration.StepStatusFailed, "", err
	}
	if _, err := publishToolCallEvent(ctx, eventBus, sqliteStore, "tool_call.requested", workflow.RunID, runtimeStep.StepID, toolCall); err != nil {
		return runtime.ToolCall{}, false, orchestration.StepStatusFailed, "", err
	}
	go watchWorkflowSandboxExecution(cfg, manager, policyEngine, nil, skillRegistry, nil, sandboxManager, integrationsManager, calendarManager, mailManager, eventBus, deliveryManager, sqliteStore, checkpointManager, workflow.WorkflowID, wfStep.WorkflowStepID, workflow.RunID, runtimeStep.StepID, toolCall.ToolCallID, execution.ExecutionID)
	return toolCall, false, orchestration.StepStatusRunning, "", nil
}

func executeWorkflowCapabilityTool(ctx context.Context, cfg config.Config, manager *runtime.Manager, policyEngine *policy.Engine, capabilitySupervisor *capabilities.Supervisor, sandboxManager *sandbox.Manager, integrationsManager *integrations.Manager, calendarManager *calendar.Manager, mailManager *mail.Manager, eventBus *events.Bus, deliveryManager *delivery.Manager, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, workflow orchestration.Workflow, runtimeStep runtime.Step, wfStep orchestration.WorkflowStep) (runtime.ToolCall, bool, orchestration.StepStatus, string, error) {
	request := createToolCallRequest{
		CapabilityID: wfStep.ConsumerID,
		ToolName:     wfStep.ToolName,
		Input:        wfStep.Input,
	}
	createInput, consumer, executionReq, approvalOutcome, err := prepareCapabilityToolCall(ctx, cfg, policyEngine, sqliteStore, eventBus, capabilitySupervisor, request, "workflow:"+workflow.WorkflowID)
	if err != nil {
		return runtime.ToolCall{}, false, orchestration.StepStatusFailed, "", err
	}
	createInput.WorkflowID = workflow.WorkflowID
	createInput.WorkflowStepID = wfStep.WorkflowStepID
	createInput.Attempt = wfStep.AttemptCount + 1
	if approvalOutcome != nil {
		toolCall, createErr := manager.CreateToolCall(workflow.RunID, runtimeStep.StepID, createInput)
		if createErr != nil {
			return runtime.ToolCall{}, false, orchestration.StepStatusBlocked, string(orchestration.BlockedReasonApprovalDenied), createErr
		}
		if err := persistToolCall(ctx, sqliteStore, manager, toolCall); err != nil {
			return runtime.ToolCall{}, false, orchestration.StepStatusBlocked, string(orchestration.BlockedReasonApprovalDenied), err
		}
		return toolCall, true, orchestration.StepStatusBlocked, string(orchestration.BlockedReasonApprovalDenied), nil
	}
	toolCall, err := manager.CreateToolCall(workflow.RunID, runtimeStep.StepID, createInput)
	if err != nil {
		return runtime.ToolCall{}, false, orchestration.StepStatusFailed, "", err
	}
	if consumer != nil && consumer.PolicyRecord != nil {
		consumer.PolicyRecord.ToolCallID = toolCall.ToolCallID
	}
	executionReq.Consumer = consumer
	toolCall.Sandbox = consumerViewMap(consumer)
	execution, err := sandboxManager.StartExecution(ctx, executionReq)
	if err != nil {
		return runtime.ToolCall{}, false, orchestration.StepStatusFailed, "", err
	}
	toolCall.SandboxExecutionID = execution.ExecutionID
	toolCall.Sandbox = consumerViewMap(execution.Consumer)
	if err := persistToolCall(ctx, sqliteStore, manager, toolCall); err != nil {
		return runtime.ToolCall{}, false, orchestration.StepStatusFailed, "", err
	}
	if _, err := publishToolCallEvent(ctx, eventBus, sqliteStore, "tool_call.requested", workflow.RunID, runtimeStep.StepID, toolCall); err != nil {
		return runtime.ToolCall{}, false, orchestration.StepStatusFailed, "", err
	}
	go watchWorkflowSandboxExecution(cfg, manager, policyEngine, capabilitySupervisor, nil, nil, sandboxManager, integrationsManager, calendarManager, mailManager, eventBus, deliveryManager, sqliteStore, checkpointManager, workflow.WorkflowID, wfStep.WorkflowStepID, workflow.RunID, runtimeStep.StepID, toolCall.ToolCallID, execution.ExecutionID)
	return toolCall, false, orchestration.StepStatusRunning, "", nil
}

func watchWorkflowSandboxExecution(cfg config.Config, manager *runtime.Manager, policyEngine *policy.Engine, capabilitySupervisor *capabilities.Supervisor, skillRegistry *skills.Registry, mcpManager *mcp.Manager, sandboxManager *sandbox.Manager, integrationsManager *integrations.Manager, calendarManager *calendar.Manager, mailManager *mail.Manager, eventBus *events.Bus, deliveryManager *delivery.Manager, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, workflowID, workflowStepID, runID, stepID, toolCallID, executionID string) {
	bgCtx := events.WithEnvironmentScope(context.Background(), string(cfg.Environment))
	for {
		execution, ok := sandboxManager.GetExecution(executionID)
		if !ok {
			return
		}
		var (
			toolCall runtime.ToolCall
			err      error
			terminal bool
		)
		switch execution.Status {
		case sandbox.ExecutionStatusPending:
		case sandbox.ExecutionStatusRunning:
			toolCall, err = manager.MarkToolCallRunning(runID, stepID, toolCallID, execution.ExecutionID, consumerViewMap(execution.Consumer))
			if err == nil {
				_ = persistToolCall(context.Background(), sqliteStore, manager, toolCall)
				_ = persistCheckpoint(context.Background(), checkpointManager, runID)
			}
		case sandbox.ExecutionStatusCompleted:
			toolCall, err = manager.CompleteToolCall(runID, stepID, toolCallID, runtime.CompleteToolCallInput{Output: buildSandboxToolCallOutput(execution), SandboxExecutionID: execution.ExecutionID, Sandbox: consumerViewMap(execution.Consumer)})
			terminal = true
		case sandbox.ExecutionStatusFailed:
			toolCall, err = manager.FailToolCall(runID, stepID, toolCallID, runtime.FailToolCallInput{Output: buildSandboxToolCallOutput(execution), Error: execution.Result.Error, FailureClass: string(execution.Result.ErrorClass), SandboxExecutionID: execution.ExecutionID, Sandbox: consumerViewMap(execution.Consumer)})
			terminal = true
		case sandbox.ExecutionStatusCancelled:
			toolCall, err = manager.CancelToolCall(runID, stepID, toolCallID, runtime.CancelToolCallInput{Output: buildSandboxToolCallOutput(execution), Error: execution.Result.Error, FailureClass: string(execution.Result.ErrorClass), SandboxExecutionID: execution.ExecutionID, Sandbox: consumerViewMap(execution.Consumer)})
			terminal = true
		case sandbox.ExecutionStatusDenied:
			toolCall, err = manager.DenyToolCall(runID, stepID, toolCallID, runtime.DenyToolCallInput{Output: buildSandboxToolCallOutput(execution), Error: execution.Result.Error, FailureClass: string(execution.Result.ErrorClass), SandboxExecutionID: execution.ExecutionID, Sandbox: consumerViewMap(execution.Consumer)})
			terminal = true
		case sandbox.ExecutionStatusUnsupported:
			toolCall, err = manager.FailToolCall(runID, stepID, toolCallID, runtime.FailToolCallInput{Output: buildSandboxToolCallOutput(execution), Error: execution.Result.Error, FailureClass: string(execution.Result.ErrorClass), SandboxExecutionID: execution.ExecutionID, Sandbox: consumerViewMap(execution.Consumer)})
			terminal = true
		}
		if err == nil && terminal {
			_ = persistToolCall(bgCtx, sqliteStore, manager, toolCall)
			_ = persistCheckpoint(bgCtx, checkpointManager, runID)
			if toolCall.Status == runtime.ToolCallStatusCompleted {
				_, _ = publishToolCallEvent(bgCtx, eventBus, sqliteStore, "tool_call.completed", runID, stepID, toolCall)
			} else {
				_, _ = publishToolCallEvent(bgCtx, eventBus, sqliteStore, "tool_call.failed", runID, stepID, toolCall)
			}
			workflow, ok, getErr := sqliteStore.GetWorkflow(bgCtx, string(cfg.Environment), runID, workflowID)
			if getErr == nil && ok {
				_, _, _ = advanceWorkflowAfterToolCall(bgCtx, cfg, manager, policyEngine, capabilitySupervisor, skillRegistry, mcpManager, sandboxManager, integrationsManager, calendarManager, mailManager, eventBus, deliveryManager, sqliteStore, checkpointManager, nil, workflow, toolCall, orchestration.StepStatusRunning, "")
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func advanceWorkflowAfterToolCall(ctx context.Context, cfg config.Config, manager *runtime.Manager, policyEngine *policy.Engine, capabilitySupervisor *capabilities.Supervisor, skillRegistry *skills.Registry, mcpManager *mcp.Manager, sandboxManager *sandbox.Manager, integrationsManager *integrations.Manager, calendarManager *calendar.Manager, mailManager *mail.Manager, eventBus *events.Bus, deliveryManager *delivery.Manager, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, computerUseManager *computeruse.Manager, workflow orchestration.Workflow, toolCall runtime.ToolCall, hintedStatus orchestration.StepStatus, blockedReason string) (orchestration.Workflow, bool, error) {
	previousStatus := workflow.Status
	workflow = orchestration.NewManager().ApplyToolCallResult(workflow, toolCall, hintedStatus, blockedReason, time.Now().UTC())
	step := orchestration.WorkflowStepByID(workflow, toolCall.WorkflowStepID)
	if step != nil {
		switch step.Status {
		case orchestration.StepStatusCompleted:
			if updatedStep, runUpdate, err := manager.UpdateStepStatusAndReconcileRun(workflow.RunID, toolCall.StepID, runtime.UpdateStepStatusInput{Status: runtime.StepStatusCompleted, Output: toolCall.Output}); err == nil {
				_ = persistStep(ctx, sqliteStore, updatedStep)
				if runUpdate != nil {
					_ = persistRun(ctx, sqliteStore, *runUpdate)
				}
			}
		case orchestration.StepStatusCancelled:
			if updatedStep, runUpdate, _, err := manager.CancelStep(workflow.RunID, toolCall.StepID); err == nil {
				_ = persistStepCancelMutation(ctx, sqliteStore, checkpointManager, updatedStep, runUpdate)
			}
		case orchestration.StepStatusFailed:
			if updatedStep, runUpdate, err := manager.UpdateStepStatusAndReconcileRun(workflow.RunID, toolCall.StepID, runtime.UpdateStepStatusInput{Status: runtime.StepStatusFailed, Output: toolCall.Output}); err == nil {
				_ = persistStep(ctx, sqliteStore, updatedStep)
				if runUpdate != nil {
					_ = persistRun(ctx, sqliteStore, *runUpdate)
				}
			}
		}
	}
	if err := persistWorkflowDetail(ctx, sqliteStore, workflow); err != nil {
		return orchestration.Workflow{}, false, err
	}
	if toolCall.WorkflowStepID != "" {
		_, _ = publishWorkflowEvent(ctx, eventBus, sqliteStore, "workflow.step_status_changed", workflow, &toolCall, map[string]any{"workflowStepId": toolCall.WorkflowStepID})
	}
	if workflow.Status != previousStatus {
		_, _ = publishWorkflowEvent(ctx, eventBus, sqliteStore, "workflow.status_changed", workflow, &toolCall, nil)
		if err := maybeEmitWorkflowDelivery(ctx, deliveryManager, manager, calendarManager, sqliteStore, workflow, mailManager); err != nil {
			return orchestration.Workflow{}, false, err
		}
	}
	nextWorkflow, err := advanceWorkflowExecution(ctx, cfg, manager, policyEngine, capabilitySupervisor, skillRegistry, mcpManager, sandboxManager, integrationsManager, calendarManager, mailManager, eventBus, deliveryManager, sqliteStore, checkpointManager, computerUseManager, workflow)
	return nextWorkflow, true, err
}

func persistWorkflowDetail(ctx context.Context, sqliteStore *store.SQLiteStore, workflow orchestration.Workflow) error {
	if err := sqliteStore.UpsertWorkflow(ctx, workflow); err != nil {
		return err
	}
	if err := sqliteStore.ReplaceWorkflowSteps(ctx, workflow.WorkflowID, workflow.Steps); err != nil {
		return err
	}
	if err := sqliteStore.ReplaceWorkflowDependencies(ctx, workflow.WorkflowID, workflow.Dependencies); err != nil {
		return err
	}
	return sqliteStore.ReplaceWorkflowHandoffs(ctx, workflow.WorkflowID, workflow.Handoffs)
}

func publishWorkflowEvent(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, name string, workflow orchestration.Workflow, toolCall *runtime.ToolCall, extra map[string]any) (events.Event, error) {
	step := workflowEventStep(workflow, toolCall, extra)
	payload := map[string]any{
		"workflowId": workflow.WorkflowID,
		"runId":      workflow.RunID,
		"status":     string(workflow.Status),
	}
	if toolCall != nil {
		payload["workflowStepId"] = toolCall.WorkflowStepID
		payload["runtimeStepId"] = toolCall.StepID
		payload["toolCallId"] = toolCall.ToolCallID
		payload["attempt"] = toolCall.Attempt
		payload["toolName"] = toolCall.ToolName
		payload["invocationKind"] = toolCall.InvocationKind
		if toolCall.CapabilityID != "" {
			payload["consumerId"] = toolCall.CapabilityID
			payload["consumerKind"] = toolCall.InvocationKind
		}
		if toolCall.SkillID != "" {
			payload["consumerId"] = toolCall.SkillID
			payload["consumerKind"] = toolCall.InvocationKind
		}
		if toolCall.MCPServerID != "" {
			payload["consumerId"] = toolCall.MCPServerID
			payload["consumerKind"] = toolCall.InvocationKind
		}
		if toolCall.FailureClass != "" {
			payload["failureClass"] = toolCall.FailureClass
		}
	}
	if step != nil {
		if name == "workflow.step_status_changed" {
			payload["status"] = string(step.Status)
		}
		if step.WorkflowStepID != "" {
			payload["workflowStepId"] = step.WorkflowStepID
		}
		if step.ConsumerKind != "" {
			payload["consumerKind"] = step.ConsumerKind
		}
		if step.ConsumerID != "" {
			payload["consumerId"] = step.ConsumerID
		}
		if step.ToolName != "" {
			payload["toolName"] = step.ToolName
		}
		if step.ApprovalModeExpected != "" {
			payload["approvalModeExpected"] = step.ApprovalModeExpected
		}
		if step.BlockedReason != "" {
			payload["blockedReason"] = step.BlockedReason
		}
		if step.LastFailureClass != "" && payload["failureClass"] == nil {
			payload["failureClass"] = step.LastFailureClass
		}
	}
	for key, value := range extra {
		payload[key] = value
	}
	return publishEvent(ctx, eventBus, sqliteStore, events.Event{
		Category: "workflow",
		Name:     name,
		Scope: events.Scope{
			RunID:          workflow.RunID,
			WorkflowID:     workflow.WorkflowID,
			WorkflowStepID: workflowEventStepID(toolCall, step, extra),
		},
		Resource: events.Resource{Kind: "workflow", ID: workflow.WorkflowID},
		Payload:  payload,
	})
}

func publishComputerUseArtifacts(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, action computeruse.Action) {
	for _, artifact := range action.Artifacts {
		_, _ = publishEvent(ctx, eventBus, sqliteStore, events.Event{
			Category: "capability",
			Name:     "computer_use.artifact_recorded",
			Scope: events.Scope{
				RunID:                action.RunID,
				ComputerUseSessionID: action.ComputerUseSessionID,
				ComputerUseActionID:  action.ComputerUseActionID,
			},
			Resource: events.Resource{Kind: "computer_use_artifact", ID: artifact.ArtifactID},
			Payload: map[string]any{
				"artifactId":    artifact.ArtifactID,
				"artifactKind":  artifact.Kind,
				"captureStatus": artifact.Status,
			},
		})
	}
}

func publishComputerUseTargetMismatch(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, action computeruse.Action) {
	_, _ = publishEvent(ctx, eventBus, sqliteStore, events.Event{
		Category: "capability",
		Name:     "computer_use.action_target_mismatch",
		Scope: events.Scope{
			RunID:                action.RunID,
			StepID:               action.StepID,
			ComputerUseSessionID: action.ComputerUseSessionID,
			ComputerUseActionID:  action.ComputerUseActionID,
		},
		Resource: events.Resource{Kind: "computer_use_action", ID: action.ComputerUseActionID},
		Payload: map[string]any{
			"status":               action.Status,
			"failureClass":         action.FailureClass,
			"computerUseSessionId": action.ComputerUseSessionID,
			"computerUseActionId":  action.ComputerUseActionID,
		},
	})
}

func mapString(value any) string {
	item, _ := value.(string)
	return item
}

func mapSlice(value any) []map[string]any {
	switch raw := value.(type) {
	case []map[string]any:
		return raw
	case []any:
		items := make([]map[string]any, 0, len(raw))
		for _, entry := range raw {
			item, ok := entry.(map[string]any)
			if ok {
				items = append(items, item)
			}
		}
		return items
	default:
		return nil
	}
}

func maybeEmitWorkflowDelivery(ctx context.Context, deliveryManager *delivery.Manager, runtimeManager *runtime.Manager, calendarManager *calendar.Manager, sqliteStore *store.SQLiteStore, workflow orchestration.Workflow, mailManagers ...*mail.Manager) error {
	var mailManager *mail.Manager
	if len(mailManagers) > 0 {
		mailManager = mailManagers[0]
	}
	if deliveryManager == nil || !orchestration.IsTerminalWorkflowStatus(workflow.Status) {
		return nil
	}
	run, ok := runtimeManager.GetRun(workflow.RunID)
	if !ok || run.SessionID != "" {
		return nil
	}
	resultClass := delivery.ResultClassFailure
	switch workflow.Status {
	case orchestration.WorkflowStatusCompleted:
		resultClass = delivery.ResultClassRoutineSuccess
	case orchestration.WorkflowStatusCancelled:
		resultClass = delivery.ResultClassUrgent
	}
	preview := strings.TrimSpace(workflow.Goal)
	if preview == "" {
		preview = "background workflow reached terminal state"
	}
	integrationID := resolveWorkflowIntegrationID(workflow)
	outcome, err := deliveryManager.EmitOutcome(ctx, delivery.OutcomeInput{
		SourceKind:        "workflow",
		SourceID:          workflow.WorkflowID,
		RunID:             workflow.RunID,
		WorkflowID:        workflow.WorkflowID,
		ScheduleID:        workflow.ScheduleID,
		ScheduleAttemptID: workflow.ScheduleAttemptID,
		IntegrationID:     integrationID,
		ResultClass:       resultClass,
		PayloadPreview:    preview,
	})
	if err != nil {
		return err
	}
	if err := linkWorkflowCalendarOperationsToDelivery(ctx, calendarManager, sqliteStore, workflow, outcome.DeliveryID); err != nil {
		return err
	}
	return linkWorkflowMailOperationsToDelivery(ctx, mailManager, sqliteStore, workflow, outcome.DeliveryID)
}

func resolveWorkflowIntegrationID(workflow orchestration.Workflow) string {
	for _, step := range workflow.Steps {
		for _, binding := range step.IntegrationBindings {
			if strings.TrimSpace(binding.IntegrationID) != "" {
				return strings.TrimSpace(binding.IntegrationID)
			}
		}
	}
	return ""
}

func calendarIntegrationBindings(integrationsManager *integrations.Manager, integrationID string) []integrations.BindingSummary {
	if integrationsManager == nil || strings.TrimSpace(integrationID) == "" {
		return nil
	}
	binding, err := integrationsManager.BindingSummary(integrationID, time.Now().UTC())
	if err != nil {
		return nil
	}
	return []integrations.BindingSummary{binding}
}

func calendarToolCallOutput(result calendarExecutionResult) map[string]any {
	output := map[string]any{
		"operation": result.Operation,
	}
	if len(result.Artifacts) > 0 {
		output["artifacts"] = result.Artifacts
	}
	if result.Output != nil {
		output["result"] = result.Output
	}
	return output
}

func calendarToolCallFailureClass(err error, operation calendar.Operation) string {
	if strings.TrimSpace(operation.FailureClass) != "" {
		return operation.FailureClass
	}
	switch {
	case errors.Is(err, calendar.ErrCalendarIntegrationNotFound):
		return "integration_not_found"
	case errors.Is(err, calendar.ErrCalendarUnavailable):
		return "calendar_unavailable"
	case errors.Is(err, calendar.ErrCalendarSelectionInvalid):
		return "selection_invalid"
	case errors.Is(err, calendar.ErrCalendarEventNotFound):
		return "event_not_found"
	case errors.Is(err, calendar.ErrCalendarInvalidTimeRange):
		return "invalid_time_range"
	case errors.Is(err, calendar.ErrCalendarRecurringUnsupported),
		errors.Is(err, calendar.ErrCalendarAllDayUnsupported),
		errors.Is(err, calendar.ErrCalendarAttendeesUnsupported),
		errors.Is(err, calendar.ErrCalendarAlternateCalendarDeny):
		return "scope_violation"
	default:
		return "calendar_error"
	}
}

func mailToolCallFailureClass(err error, operation mail.Operation) string {
	if strings.TrimSpace(operation.FailureClass) != "" {
		return operation.FailureClass
	}
	switch {
	case errors.Is(err, mail.ErrMailIntegrationNotFound):
		return "integration_not_found"
	case errors.Is(err, mail.ErrMailUnavailable):
		return "mail_unavailable"
	case errors.Is(err, mail.ErrMailSelectionInvalid):
		return "selection_invalid"
	case errors.Is(err, mail.ErrMailThreadNotFound):
		return "thread_not_found"
	case errors.Is(err, mail.ErrMailMessageNotFound):
		return "message_not_found"
	case errors.Is(err, mail.ErrMailDraftNotFound):
		return "draft_not_found"
	case errors.Is(err, mail.ErrMailRecipientRequired):
		return "recipient_required"
	case errors.Is(err, mail.ErrMailAttachmentUnresolved):
		return "attachment_unresolved"
	case errors.Is(err, mail.ErrMailBackgroundSendBlocked):
		return "send_permission_required"
	default:
		return "mail_execution_failed"
	}
}

func linkWorkflowCalendarOperationsToDelivery(ctx context.Context, calendarManager *calendar.Manager, sqliteStore *store.SQLiteStore, workflow orchestration.Workflow, deliveryID string) error {
	if sqliteStore == nil || strings.TrimSpace(deliveryID) == "" {
		return nil
	}
	ops, err := sqliteStore.ListCalendarOperations(ctx, workflow.EnvironmentScope, store.CalendarOperationFilter{WorkflowID: workflow.WorkflowID})
	if err != nil {
		return err
	}
	for _, item := range ops {
		if strings.TrimSpace(item.DeliveryID) == strings.TrimSpace(deliveryID) {
			continue
		}
		item.DeliveryID = strings.TrimSpace(deliveryID)
		item.UpdatedAt = time.Now().UTC()
		if err := sqliteStore.UpsertCalendarOperation(ctx, item); err != nil {
			return err
		}
		if calendarManager != nil {
			calendarManager.StoreOperation(item)
		}
	}
	return nil
}

func linkWorkflowMailOperationsToDelivery(ctx context.Context, mailManager *mail.Manager, sqliteStore *store.SQLiteStore, workflow orchestration.Workflow, deliveryID string) error {
	if sqliteStore == nil || strings.TrimSpace(deliveryID) == "" {
		return nil
	}
	ops, err := sqliteStore.ListMailOperations(ctx, workflow.EnvironmentScope, store.MailOperationFilter{WorkflowID: workflow.WorkflowID})
	if err != nil {
		return err
	}
	for _, item := range ops {
		if strings.TrimSpace(item.DeliveryID) == strings.TrimSpace(deliveryID) {
			continue
		}
		item.DeliveryID = strings.TrimSpace(deliveryID)
		item.UpdatedAt = time.Now().UTC()
		if err := sqliteStore.UpsertMailOperation(ctx, item); err != nil {
			return err
		}
		if mailManager != nil {
			mailManager.StoreOperation(item)
		}
	}
	return nil
}

func decodeWorkflowComputerUseAction(payload map[string]any) computeruse.CreateActionInput {
	return computeruse.CreateActionInput{
		ActionKind:    computeruse.ActionKind(mapString(payload["actionKind"])),
		URL:           mapString(payload["url"]),
		Value:         mapString(payload["value"]),
		SelectedValue: mapString(payload["selectedValue"]),
		PageTarget:    computeruse.PageTarget(mapString(payload["pageTarget"])),
		Rationale:     mapString(payload["rationale"]),
	}
}

func workflowEventStep(workflow orchestration.Workflow, toolCall *runtime.ToolCall, extra map[string]any) *orchestration.WorkflowStep {
	if toolCall != nil && toolCall.WorkflowStepID != "" {
		return orchestration.WorkflowStepByID(workflow, toolCall.WorkflowStepID)
	}
	if extra == nil {
		return nil
	}
	workflowStepID, ok := extra["workflowStepId"].(string)
	if !ok || strings.TrimSpace(workflowStepID) == "" {
		return nil
	}
	return orchestration.WorkflowStepByID(workflow, workflowStepID)
}

func workflowEventStepID(toolCall *runtime.ToolCall, step *orchestration.WorkflowStep, extra map[string]any) string {
	if toolCall != nil && strings.TrimSpace(toolCall.WorkflowStepID) != "" {
		return toolCall.WorkflowStepID
	}
	if step != nil && strings.TrimSpace(step.WorkflowStepID) != "" {
		return step.WorkflowStepID
	}
	if extra == nil {
		return ""
	}
	workflowStepID, _ := extra["workflowStepId"].(string)
	return workflowStepID
}
