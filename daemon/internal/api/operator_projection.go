package api

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/activation"
	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

const operatorShellTestRunEntrypoint = "operator.shell.test"

type operatorProjectionBuilder struct {
	environment  string
	store        *store.SQLiteStore
	providers    *providers.Manager
	integrations *integrations.Manager
	connectors   *connectors.Supervisor
	capabilities *capabilities.Supervisor
	policy       *policy.Engine
	runtime      *runtime.Manager
	scheduler    *scheduler.Scheduler
	delivery     *delivery.Manager
	computerUse  *computeruse.Manager
}

func newOperatorProjectionBuilder(
	cfg config.Config,
	sqliteStore *store.SQLiteStore,
	providerManager *providers.Manager,
	integrationsManager *integrations.Manager,
	connectorSupervisor *connectors.Supervisor,
	capabilitySupervisor *capabilities.Supervisor,
	policyEngine *policy.Engine,
	runtimeManager *runtime.Manager,
	schedulerManager *scheduler.Scheduler,
	deliveryManager *delivery.Manager,
	computerUseManager *computeruse.Manager,
) operatorProjectionBuilder {
	return operatorProjectionBuilder{
		environment:  effectiveEnvironment(cfg),
		store:        sqliteStore,
		providers:    providerManager,
		integrations: integrationsManager,
		connectors:   connectorSupervisor,
		capabilities: capabilitySupervisor,
		policy:       policyEngine,
		runtime:      runtimeManager,
		scheduler:    schedulerManager,
		delivery:     deliveryManager,
		computerUse:  computerUseManager,
	}
}

func (b operatorProjectionBuilder) buildOnboarding(token auth.AccessToken, authenticated bool) OperatorOnboardingResponse {
	now := time.Now().UTC()
	readinessItems := make([]OperatorReadinessItem, 0)
	completedStepIDs := make([]string, 0)
	blockingItemIDs := make([]string, 0)
	optionalFollowUpItemIDs := make([]string, 0)

	authItem := OperatorReadinessItem{
		ItemID:                    "auth-token",
		ItemKind:                  "auth",
		ResourceID:                token.TokenID,
		DisplayName:               "Operator access token",
		Status:                    "ready",
		Reason:                    "Authenticated shell session is active.",
		RequiredOperatorAction:    "",
		RequiredForSelectedAction: true,
		DetailRoute:               "/v1/auth/me",
		EnvironmentScope:          b.environment,
		UpdatedAt:                 now,
	}
	if !authenticated {
		authItem.Status = "blocked"
		authItem.Reason = "Authentication is required before the operator shell can load."
		authItem.RequiredOperatorAction = "Pair or reuse a local access token."
		blockingItemIDs = append(blockingItemIDs, authItem.ItemID)
	} else {
		completedStepIDs = append(completedStepIDs, "auth-ready")
	}
	readinessItems = append(readinessItems, authItem)

	queryAction := OperatorFirstUsefulAction{
		ActionID:        "test_query",
		ActionKind:      "test_query",
		DisplayName:     "Run test query",
		Recommended:     false,
		Available:       false,
		BlockingItemIDs: []string{"provider-query"},
		Summary:         "Reuse /v1/chat/query to confirm the shell can produce a bounded test result.",
		InvokeRoute:     "/v1/chat/query",
		ResultRoute:     "/v1/chat/query",
	}

	if provider := b.firstReadyQueryProvider(); provider != nil {
		queryAction.Available = true
		queryAction.BlockingItemIDs = nil
		readinessItems = append(readinessItems, OperatorReadinessItem{
			ItemID:                    "provider-query",
			ItemKind:                  "provider",
			ResourceID:                provider.ProviderID,
			DisplayName:               provider.Title,
			Status:                    "ready",
			HealthState:               "healthy",
			Reason:                    "Provider is ready for bounded query execution.",
			RequiredOperatorAction:    "",
			RequiredForSelectedAction: false,
			DetailRoute:               "/v1/providers/" + provider.ProviderID,
			EnvironmentScope:          b.environment,
			UpdatedAt:                 now,
		})
	} else if b.providers != nil {
		reason := "No ready chat provider is currently configured."
		if profiles := b.providers.ListProfiles(); len(profiles) > 0 {
			reason = firstOperatorNonEmpty(strings.Join(profiles[0].Issues, "; "), reason)
		}
		readinessItems = append(readinessItems, OperatorReadinessItem{
			ItemID:                    "provider-query",
			ItemKind:                  "provider",
			DisplayName:               "Chat provider",
			Status:                    "optional",
			HealthState:               "degraded",
			Reason:                    reason,
			RequiredOperatorAction:    "Configure or authenticate a chat-capable provider to unlock test queries.",
			RequiredForSelectedAction: false,
			DetailRoute:               "/v1/providers",
			EnvironmentScope:          b.environment,
			UpdatedAt:                 now,
		})
	}

	for _, item := range b.optionalIntegrationReadiness(now) {
		readinessItems = append(readinessItems, item)
		if !item.RequiredForSelectedAction {
			optionalFollowUpItemIDs = append(optionalFollowUpItemIDs, item.ItemID)
		}
	}
	for _, item := range b.optionalConnectorReadiness(now) {
		readinessItems = append(readinessItems, item)
		if !item.RequiredForSelectedAction {
			optionalFollowUpItemIDs = append(optionalFollowUpItemIDs, item.ItemID)
		}
	}
	for _, item := range b.optionalCapabilityReadiness(now) {
		readinessItems = append(readinessItems, item)
		if !item.RequiredForSelectedAction {
			optionalFollowUpItemIDs = append(optionalFollowUpItemIDs, item.ItemID)
		}
	}

	testRunAction := OperatorFirstUsefulAction{
		ActionID:    "test_run",
		ActionKind:  "test_run",
		DisplayName: "Launch test run",
		Recommended: true,
		Available:   authenticated,
		Summary:     "Reuse /v1/runs to persist a bounded operator test action that survives refresh and restart.",
		InvokeRoute: "/v1/runs",
		ResultRoute: "/v1/runs",
	}
	if !authenticated {
		testRunAction.BlockingItemIDs = []string{"auth-token"}
	}
	activationAction := OperatorFirstUsefulAction{
		ActionID:    "test_chat",
		ActionKind:  "test_chat",
		DisplayName: "Run activation test chat",
		Recommended: false,
		Available:   authenticated,
		Summary:     "Complete the hosted personal-tenant activation first action without live connectors or production secrets.",
		InvokeRoute: "/v1/activation/test-chat",
		ResultRoute: "/v1/activation",
	}
	if !authenticated {
		activationAction.BlockingItemIDs = []string{"auth-token"}
	}

	firstUsefulActions := []OperatorFirstUsefulAction{testRunAction, activationAction}
	if b.providers != nil {
		firstUsefulActions = append(firstUsefulActions, queryAction)
	}

	status := "ready_for_action"
	currentStepID := "run-first-action"
	if len(blockingItemIDs) > 0 {
		status = "blocked"
		currentStepID = "resolve-blockers"
	}
	if b.hasRecordedShellTestRun() {
		status = "completed"
		currentStepID = "completed"
		completedStepIDs = append(completedStepIDs, "test-run-recorded")
	}

	return OperatorOnboardingResponse{
		EnvironmentScope:        b.environment,
		Status:                  status,
		CurrentStepID:           currentStepID,
		CompletedStepIDs:        completedStepIDs,
		BlockingItemIDs:         blockingItemIDs,
		OptionalFollowUpItemIDs: optionalFollowUpItemIDs,
		RecommendedActionID:     "test_run",
		ReadinessItems:          readinessItems,
		FirstUsefulActions:      firstUsefulActions,
		LastEvaluatedAt:         now,
	}
}

func (b operatorProjectionBuilder) optionalIntegrationReadiness(now time.Time) []OperatorReadinessItem {
	if b.integrations == nil {
		return nil
	}
	items := make([]OperatorReadinessItem, 0)
	for _, item := range b.integrations.List() {
		status, healthState := mapIntegrationReadiness(item)
		if status == "ready" {
			status = "optional"
		}
		items = append(items, OperatorReadinessItem{
			ItemID:                    "integration-" + item.IntegrationID,
			ItemKind:                  "integration",
			ResourceID:                item.IntegrationID,
			DisplayName:               item.DisplayName,
			Status:                    status,
			HealthState:               healthState,
			Reason:                    firstOperatorNonEmpty(item.ReadinessReason, "Integration readiness is projected from daemon state."),
			RequiredOperatorAction:    item.RequiredOperatorAction,
			RequiredForSelectedAction: false,
			DetailRoute:               "/v1/integrations/" + item.IntegrationID,
			EnvironmentScope:          b.environment,
			UpdatedAt:                 item.UpdatedAt,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DisplayName < items[j].DisplayName })
	return items
}

func (b operatorProjectionBuilder) optionalConnectorReadiness(now time.Time) []OperatorReadinessItem {
	if b.connectors == nil {
		return nil
	}
	items := make([]OperatorReadinessItem, 0)
	for _, item := range b.connectors.List() {
		items = append(items, OperatorReadinessItem{
			ItemID:                    "connector-" + item.ConnectorID,
			ItemKind:                  "connector",
			ResourceID:                item.ConnectorID,
			DisplayName:               item.DisplayName,
			Status:                    mapConnectorStatus(item.Status),
			HealthState:               string(item.Status),
			Reason:                    firstOperatorNonEmpty(item.LastFailureReason, "Connector health is projected from the supervisor."),
			RequiredOperatorAction:    connectorOperatorAction(item),
			RequiredForSelectedAction: false,
			DetailRoute:               "/v1/connectors/" + item.ConnectorID,
			EnvironmentScope:          b.environment,
			UpdatedAt:                 item.UpdatedAt,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DisplayName < items[j].DisplayName })
	return items
}

func (b operatorProjectionBuilder) optionalCapabilityReadiness(now time.Time) []OperatorReadinessItem {
	if b.capabilities == nil {
		return nil
	}
	items := make([]OperatorReadinessItem, 0)
	for _, item := range b.capabilities.List() {
		items = append(items, OperatorReadinessItem{
			ItemID:                    "capability-" + item.CapabilityID,
			ItemKind:                  "capability",
			ResourceID:                item.CapabilityID,
			DisplayName:               item.DisplayName,
			Status:                    mapCapabilityStatus(item.Status),
			HealthState:               string(item.Status),
			Reason:                    firstOperatorNonEmpty(item.LastFailureReason, "Capability health is projected from the supervisor."),
			RequiredOperatorAction:    capabilityOperatorAction(item),
			RequiredForSelectedAction: false,
			DetailRoute:               "/v1/capabilities/" + item.CapabilityID,
			EnvironmentScope:          b.environment,
			UpdatedAt:                 item.UpdatedAt,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DisplayName < items[j].DisplayName })
	return items
}

func (b operatorProjectionBuilder) firstReadyQueryProvider() *providers.Profile {
	if b.providers == nil {
		return nil
	}
	for _, profile := range b.providers.ListProfiles() {
		if profile.Ready && profile.Capabilities.Chat {
			profileCopy := profile
			return &profileCopy
		}
	}
	return nil
}

func (b operatorProjectionBuilder) hasRecordedShellTestRun() bool {
	if b.runtime == nil {
		return false
	}
	for _, run := range b.runtime.ListRuns() {
		if run.Entrypoint == operatorShellTestRunEntrypoint && run.Status != runtime.RunStatusCancelled {
			return true
		}
	}
	return false
}

func (b operatorProjectionBuilder) buildActivity(ctx context.Context) (OperatorActivityListResponse, error) {
	now := time.Now().UTC()
	records := make([]OperatorActivityRecord, 0)
	if b.policy != nil {
		for _, approval := range b.policy.ListApprovals("") {
			records = append(records, OperatorActivityRecord{
				ActivityID:       "approval-" + approval.ApprovalID,
				SourceKind:       "approval",
				SourceID:         approval.ApprovalID,
				Title:            fmt.Sprintf("Approval %s", approval.Action),
				Status:           string(approval.Status),
				Summary:          approval.Reason,
				AttentionLevel:   attentionLevelForApproval(approval),
				OccurredAt:       approval.UpdatedAt,
				DetailRoute:      "/v1/policy/approvals/" + approval.ApprovalID,
				EnvironmentScope: b.environment,
			})
		}
	}
	if b.scheduler != nil {
		items, err := b.scheduler.List(ctx)
		if err != nil {
			return OperatorActivityListResponse{}, err
		}
		for _, item := range items {
			records = append(records, OperatorActivityRecord{
				ActivityID:          "schedule-" + item.ScheduleID,
				SourceKind:          "schedule",
				SourceID:            item.ScheduleID,
				Title:               fmt.Sprintf("Schedule %s", item.Target.Summary),
				Status:              string(item.Status),
				Summary:             scheduleSummary(item),
				AttentionLevel:      attentionLevelForSchedule(item),
				OccurredAt:          item.UpdatedAt,
				DetailRoute:         "/v1/schedules/" + item.ScheduleID,
				RelatedResourceRefs: buildScheduleRefs(item),
				EnvironmentScope:    b.environment,
			})
		}
	}
	if b.runtime != nil {
		for _, run := range b.runtime.ListRuns() {
			records = append(records, OperatorActivityRecord{
				ActivityID:       "run-" + run.RunID,
				SourceKind:       sourceKindForRun(run),
				SourceID:         run.RunID,
				Title:            fmt.Sprintf("Run %s", firstOperatorNonEmpty(run.Goal, run.Entrypoint)),
				Status:           string(run.Status),
				Summary:          runSummary(run),
				AttentionLevel:   attentionLevelForRun(run),
				OccurredAt:       run.UpdatedAt,
				DetailRoute:      "/v1/runs/" + run.RunID,
				EnvironmentScope: b.environment,
			})
			if b.store == nil {
				continue
			}
			workflows, err := b.store.ListWorkflows(ctx, b.environment, run.RunID)
			if err != nil {
				return OperatorActivityListResponse{}, err
			}
			for _, workflow := range workflows {
				records = append(records, OperatorActivityRecord{
					ActivityID:     "workflow-" + workflow.WorkflowID,
					SourceKind:     "workflow",
					SourceID:       workflow.WorkflowID,
					Title:          fmt.Sprintf("Workflow %s", firstOperatorNonEmpty(workflow.Goal, workflow.WorkflowID)),
					Status:         string(workflow.Status),
					Summary:        workflowSummary(workflow),
					AttentionLevel: attentionLevelForWorkflow(workflow),
					OccurredAt:     workflow.UpdatedAt,
					DetailRoute:    "/v1/runs/" + workflow.RunID + "/workflows/" + workflow.WorkflowID,
					RelatedResourceRefs: []OperatorResourceRef{{
						Kind:  "run",
						ID:    workflow.RunID,
						Route: "/v1/runs/" + workflow.RunID,
					}},
					EnvironmentScope: b.environment,
				})
			}
		}
	}
	if b.delivery != nil {
		items, err := b.delivery.ListOutcomes(ctx, delivery.OutcomeFilter{})
		if err != nil {
			return OperatorActivityListResponse{}, err
		}
		for _, item := range items {
			records = append(records, OperatorActivityRecord{
				ActivityID:          "delivery-" + item.DeliveryID,
				SourceKind:          "delivery",
				SourceID:            item.DeliveryID,
				Title:               fmt.Sprintf("Delivery %s", item.ResultClass),
				Status:              string(item.Status),
				Summary:             deliverySummary(item),
				AttentionLevel:      attentionLevelForDelivery(item),
				OccurredAt:          item.UpdatedAt,
				DetailRoute:         "/v1/deliveries/" + item.DeliveryID,
				RelatedResourceRefs: buildDeliveryRefs(item),
				EnvironmentScope:    b.environment,
			})
		}
	}
	eventRecords, err := b.buildEventBackedActivity(ctx, records)
	if err != nil {
		return OperatorActivityListResponse{}, err
	}
	records = append(records, eventRecords...)
	sort.Slice(records, func(i, j int) bool { return records[i].OccurredAt.After(records[j].OccurredAt) })
	return OperatorActivityListResponse{
		EnvironmentScope: b.environment,
		Items:            records,
		GeneratedAt:      now,
	}, nil
}

func (b operatorProjectionBuilder) buildEventBackedActivity(ctx context.Context, current []OperatorActivityRecord) ([]OperatorActivityRecord, error) {
	if b.store == nil {
		return nil, nil
	}

	items, err := b.store.ListEvents(ctx, events.Filter{EnvironmentScope: b.environment})
	if err != nil {
		return nil, err
	}

	seenResources := make(map[string]struct{}, len(current))
	for _, item := range current {
		seenResources[item.SourceKind+"::"+item.SourceID] = struct{}{}
	}

	records := make([]OperatorActivityRecord, 0)
	for _, item := range items {
		record, ok := operatorActivityRecordFromEvent(item, b.environment)
		if !ok {
			continue
		}
		if _, exists := seenResources[record.SourceKind+"::"+record.SourceID]; exists {
			continue
		}
		records = append(records, record)
		seenResources[record.SourceKind+"::"+record.SourceID] = struct{}{}
	}

	return records, nil
}

func (b operatorProjectionBuilder) buildDiagnostics(ctx context.Context) (OperatorDiagnosticListResponse, error) {
	now := time.Now().UTC()
	findings := make([]OperatorDiagnosticFinding, 0)
	onboarding := b.buildOnboarding(auth.AccessToken{}, true)
	for _, item := range onboarding.ReadinessItems {
		if item.Status == "ready" || item.Status == "optional" {
			continue
		}
		findings = append(findings, OperatorDiagnosticFinding{
			FindingID:         "readiness-" + item.ItemID,
			SourceKind:        item.ItemKind,
			SourceID:          item.ResourceID,
			Plane:             "readiness",
			Severity:          severityForReadiness(item.Status),
			Status:            item.Status,
			Reason:            firstOperatorNonEmpty(item.Reason, "Readiness is not satisfied."),
			RecommendedAction: item.RequiredOperatorAction,
			DetailRoute:       item.DetailRoute,
			EnvironmentScope:  b.environment,
			CapturedAt:        item.UpdatedAt,
		})
	}
	if b.policy != nil {
		for _, approval := range b.policy.ListApprovals(policy.ApprovalStatusPending) {
			findings = append(findings, OperatorDiagnosticFinding{
				FindingID:         "approval-" + approval.ApprovalID,
				SourceKind:        "approval",
				SourceID:          approval.ApprovalID,
				Plane:             "approval",
				Severity:          "warning",
				Status:            string(approval.Status),
				Reason:            approval.Reason,
				RecommendedAction: "Approve or reject the pending request.",
				DetailRoute:       "/v1/policy/approvals/" + approval.ApprovalID,
				EnvironmentScope:  b.environment,
				CapturedAt:        approval.UpdatedAt,
			})
		}
	}
	if b.scheduler != nil {
		items, err := b.scheduler.List(ctx)
		if err != nil {
			return OperatorDiagnosticListResponse{}, err
		}
		for _, item := range items {
			if item.Status == scheduler.ScheduleStatusActive || item.Status == scheduler.ScheduleStatusScheduled || item.Status == scheduler.ScheduleStatusCompleted {
				continue
			}
			findings = append(findings, OperatorDiagnosticFinding{
				FindingID:           "schedule-" + item.ScheduleID,
				SourceKind:          "schedule",
				SourceID:            item.ScheduleID,
				Plane:               "execution",
				Severity:            severityForSchedule(item),
				Status:              string(item.Status),
				Reason:              scheduleSummary(item),
				RecommendedAction:   "Inspect the schedule target and its latest attempt.",
				DetailRoute:         "/v1/schedules/" + item.ScheduleID,
				RelatedResourceRefs: buildScheduleRefs(item),
				EnvironmentScope:    b.environment,
				CapturedAt:          item.UpdatedAt,
			})
		}
	}
	if b.runtime != nil {
		for _, run := range b.runtime.ListRuns() {
			if run.Status != runtime.RunStatusBlocked && run.Status != runtime.RunStatusFailed && run.Status != runtime.RunStatusCancelled {
				continue
			}
			findings = append(findings, OperatorDiagnosticFinding{
				FindingID:         "run-" + run.RunID,
				SourceKind:        "run",
				SourceID:          run.RunID,
				Plane:             "execution",
				Severity:          severityForRun(run),
				Status:            string(run.Status),
				Reason:            runSummary(run),
				RecommendedAction: "Inspect the run and any linked workflow or approval blockers.",
				DetailRoute:       "/v1/runs/" + run.RunID,
				EnvironmentScope:  b.environment,
				CapturedAt:        run.UpdatedAt,
			})
			if b.store == nil {
				continue
			}
			workflows, err := b.store.ListWorkflows(ctx, b.environment, run.RunID)
			if err != nil {
				return OperatorDiagnosticListResponse{}, err
			}
			for _, workflow := range workflows {
				if workflow.Status == orchestration.WorkflowStatusCompleted || workflow.Status == orchestration.WorkflowStatusPlanned || workflow.Status == orchestration.WorkflowStatusRunning {
					continue
				}
				findings = append(findings, OperatorDiagnosticFinding{
					FindingID:         "workflow-" + workflow.WorkflowID,
					SourceKind:        "workflow",
					SourceID:          workflow.WorkflowID,
					Plane:             "execution",
					Severity:          severityForWorkflow(workflow),
					Status:            string(workflow.Status),
					Reason:            workflowSummary(workflow),
					RecommendedAction: "Inspect the workflow plan, steps, and linked delivery state.",
					DetailRoute:       "/v1/runs/" + workflow.RunID + "/workflows/" + workflow.WorkflowID,
					EnvironmentScope:  b.environment,
					CapturedAt:        workflow.UpdatedAt,
				})
				findings = append(findings, b.computerUseFindings(ctx, run.RunID, workflow)...)
			}
		}
	}
	if b.delivery != nil {
		items, err := b.delivery.ListOutcomes(ctx, delivery.OutcomeFilter{})
		if err != nil {
			return OperatorDiagnosticListResponse{}, err
		}
		for _, item := range items {
			if item.Status != delivery.OutcomeStatusFailed && item.Status != delivery.OutcomeStatusSuppressed {
				continue
			}
			findings = append(findings, OperatorDiagnosticFinding{
				FindingID:           "delivery-" + item.DeliveryID,
				SourceKind:          "delivery",
				SourceID:            item.DeliveryID,
				Plane:               "delivery",
				Severity:            severityForDelivery(item),
				Status:              string(item.Status),
				Reason:              deliverySummary(item),
				RecommendedAction:   "Inspect delivery attempts, target state, and source execution.",
				DetailRoute:         "/v1/deliveries/" + item.DeliveryID,
				RelatedResourceRefs: buildDeliveryRefs(item),
				EnvironmentScope:    b.environment,
				CapturedAt:          item.UpdatedAt,
			})
		}
	}
	activationFindings, err := b.activationFindings(ctx)
	if err != nil {
		return OperatorDiagnosticListResponse{}, err
	}
	findings = append(findings, activationFindings...)
	sort.Slice(findings, func(i, j int) bool { return findings[i].CapturedAt.After(findings[j].CapturedAt) })
	return OperatorDiagnosticListResponse{
		EnvironmentScope: b.environment,
		Items:            findings,
		GeneratedAt:      now,
	}, nil
}

func (b operatorProjectionBuilder) activationFindings(ctx context.Context) ([]OperatorDiagnosticFinding, error) {
	if b.store == nil {
		return nil, nil
	}
	states, err := b.store.ListActivationStates(ctx, b.environment)
	if err != nil {
		return nil, err
	}
	items := make([]OperatorDiagnosticFinding, 0)
	for _, state := range states {
		reason, ok := activationFailureForOperator(state)
		if !ok {
			continue
		}
		items = append(items, OperatorDiagnosticFinding{
			FindingID:         "activation-" + state.ActivationID,
			SourceKind:        "activation",
			SourceID:          state.ActivationID,
			Plane:             "readiness",
			Severity:          activationSeverity(state, reason),
			Status:            string(state.Status),
			Reason:            string(reason.ReasonCode),
			RecommendedAction: activationRecommendedAction(reason),
			DetailRoute:       "/v1/activation/diagnostics",
			RelatedResourceRefs: []OperatorResourceRef{{
				Kind: "tenant",
				ID:   state.TenantID,
			}},
			EnvironmentScope: b.environment,
			CapturedAt:       state.UpdatedAt,
		})
	}
	return items, nil
}

func activationFailureForOperator(state activation.State) (activation.FailureReason, bool) {
	if state.FailureReason != nil {
		return *state.FailureReason, true
	}
	if len(state.BlockingReasonCodes) == 0 {
		return activation.FailureReason{}, false
	}
	reasonCode := state.BlockingReasonCodes[0]
	return activation.FailureReason{
		ReasonCode:       reasonCode,
		Stage:            activationStageForOperator(reasonCode),
		Retryable:        reasonCode == activation.ReasonQuotaBaselineUnavailable || reasonCode == activation.ReasonTestChatUnavailable,
		RemediationOwner: activation.RemediationOwnerOperator,
	}, true
}

func activationStageForOperator(reason activation.ReasonCode) activation.FailureStage {
	switch reason {
	case activation.ReasonQuotaBaselineUnavailable:
		return activation.FailureStageQuotaBaseline
	case activation.ReasonTenantAccessRevoked:
		return activation.FailureStageAuthorization
	case activation.ReasonPrincipalDenied, activation.ReasonPrincipalDisabled:
		return activation.FailureStageEligibility
	case activation.ReasonTestChatFailed, activation.ReasonTestChatUnavailable:
		return activation.FailureStageTestChat
	case activation.ReasonAuditWriteFailed:
		return activation.FailureStageAudit
	case activation.ReasonPersistenceFailed:
		return activation.FailureStagePersistence
	case activation.ReasonTenantResolutionFailed:
		return activation.FailureStageTenantResolution
	default:
		return activation.FailureStageUnexpected
	}
}

func activationSeverity(state activation.State, reason activation.FailureReason) string {
	if state.Status == activation.StatusBlocked || reason.Retryable {
		return "warning"
	}
	return "critical"
}

func activationRecommendedAction(reason activation.FailureReason) string {
	switch reason.RemediationOwner {
	case activation.RemediationOwnerProductUser:
		return "Ask the user to retry activation from an allowed active tenant context."
	case activation.RemediationOwnerTenantAdmin:
		return "Review tenant membership and invitation state before retrying activation."
	case activation.RemediationOwnerSystem:
		return "Inspect system health and retry activation after recovery."
	default:
		return "Inspect activation diagnostics and retry after the blocked stage is resolved."
	}
}

func (b operatorProjectionBuilder) computerUseFindings(ctx context.Context, runID string, workflow orchestration.Workflow) []OperatorDiagnosticFinding {
	if b.computerUse == nil {
		return nil
	}
	items := make([]OperatorDiagnosticFinding, 0)
	for _, step := range workflow.Steps {
		if step.ComputerUseSessionID == "" {
			continue
		}
		session, ok, err := b.computerUse.GetSession(ctx, runID, step.ComputerUseSessionID)
		if err != nil || !ok {
			continue
		}
		switch session.Status {
		case computeruse.SessionStatusBlocked, computeruse.SessionStatusFailed, computeruse.SessionStatusInterrupted:
			items = append(items, OperatorDiagnosticFinding{
				FindingID:         "computer-use-session-" + session.ComputerUseSessionID,
				SourceKind:        "computer_use_session",
				SourceID:          session.ComputerUseSessionID,
				Plane:             "execution",
				Severity:          "critical",
				Status:            string(session.Status),
				Reason:            "Computer-use session needs operator attention.",
				RecommendedAction: "Inspect the browser session and its latest action.",
				DetailRoute:       "/v1/runs/" + runID + "/computer-use/sessions/" + session.ComputerUseSessionID,
				EnvironmentScope:  b.environment,
				CapturedAt:        session.UpdatedAt,
			})
		}
	}
	return items
}

func mapIntegrationReadiness(item integrations.Resource) (status string, health string) {
	switch item.ReadinessStatus {
	case integrations.ReadinessStatusHealthy:
		return "ready", firstOperatorNonEmpty(string(item.HealthState), "healthy")
	case integrations.ReadinessStatusDegraded:
		return "degraded", firstOperatorNonEmpty(string(item.HealthState), "degraded")
	case integrations.ReadinessStatusUnavailable:
		return "blocked", firstOperatorNonEmpty(string(item.HealthState), "unavailable")
	case integrations.ReadinessStatusAuthPending, integrations.ReadinessStatusNotConfigured:
		return "missing_configuration", firstOperatorNonEmpty(string(item.HealthState), "unknown")
	default:
		return "blocked", firstOperatorNonEmpty(string(item.HealthState), "unknown")
	}
}

func mapConnectorStatus(status connectors.Status) string {
	switch status {
	case connectors.StatusHealthy:
		return "optional"
	case connectors.StatusDegraded, connectors.StatusBackingOff:
		return "degraded"
	case connectors.StatusFailed:
		return "blocked"
	default:
		return "optional"
	}
}

func mapCapabilityStatus(status capabilities.Status) string {
	switch status {
	case capabilities.StatusHealthy:
		return "optional"
	case capabilities.StatusDegraded, capabilities.StatusBackingOff:
		return "degraded"
	case capabilities.StatusFailed:
		return "blocked"
	default:
		return "optional"
	}
}

func connectorOperatorAction(item connectors.Connector) string {
	switch item.Status {
	case connectors.StatusBackingOff:
		return "Wait for the scheduled restart or inspect the connector logs."
	case connectors.StatusFailed:
		return "Restart or reconfigure the connector."
	case connectors.StatusDegraded:
		return "Inspect connector health and recover the downstream transport."
	default:
		return ""
	}
}

func capabilityOperatorAction(item capabilities.Capability) string {
	switch item.Status {
	case capabilities.StatusBackingOff:
		return "Wait for the capability restart window or inspect its worker."
	case capabilities.StatusFailed:
		return "Restart or repair the capability implementation."
	case capabilities.StatusDegraded:
		return "Inspect capability health and recover the degraded dependency."
	default:
		return ""
	}
}

func attentionLevelForApproval(item policy.Approval) string {
	if item.Status == policy.ApprovalStatusPending {
		return "warning"
	}
	return "info"
}

func attentionLevelForSchedule(item scheduler.Schedule) string {
	switch item.Status {
	case scheduler.ScheduleStatusDispatchFailed, scheduler.ScheduleStatusCancelled:
		return "critical"
	case scheduler.ScheduleStatusPaused:
		return "warning"
	default:
		return "info"
	}
}

func attentionLevelForRun(item runtime.Run) string {
	switch item.Status {
	case runtime.RunStatusFailed, runtime.RunStatusCancelled:
		return "critical"
	case runtime.RunStatusBlocked, runtime.RunStatusWaitingInput:
		return "warning"
	default:
		return "info"
	}
}

func attentionLevelForWorkflow(item orchestration.Workflow) string {
	switch item.Status {
	case orchestration.WorkflowStatusFailed, orchestration.WorkflowStatusInterrupted, orchestration.WorkflowStatusPlanningFailed:
		return "critical"
	case orchestration.WorkflowStatusBlocked, orchestration.WorkflowStatusPartialFailed, orchestration.WorkflowStatusCancelled:
		return "warning"
	default:
		return "info"
	}
}

func attentionLevelForDelivery(item delivery.DeliveryOutcome) string {
	switch item.Status {
	case delivery.OutcomeStatusFailed:
		return "critical"
	case delivery.OutcomeStatusSuppressed:
		return "warning"
	default:
		return "info"
	}
}

func severityForReadiness(status string) string {
	switch status {
	case "missing_configuration", "blocked":
		return "critical"
	default:
		return "warning"
	}
}

func severityForSchedule(item scheduler.Schedule) string {
	if item.Status == scheduler.ScheduleStatusDispatchFailed || item.Status == scheduler.ScheduleStatusCancelled {
		return "critical"
	}
	return "warning"
}

func severityForRun(item runtime.Run) string {
	if item.Status == runtime.RunStatusFailed {
		return "critical"
	}
	return "warning"
}

func severityForWorkflow(item orchestration.Workflow) string {
	switch item.Status {
	case orchestration.WorkflowStatusFailed, orchestration.WorkflowStatusPlanningFailed, orchestration.WorkflowStatusInterrupted:
		return "critical"
	default:
		return "warning"
	}
}

func severityForDelivery(item delivery.DeliveryOutcome) string {
	if item.Status == delivery.OutcomeStatusFailed {
		return "critical"
	}
	return "warning"
}

func sourceKindForRun(item runtime.Run) string {
	if item.Entrypoint == operatorShellTestRunEntrypoint {
		return "first_action"
	}
	return "run"
}

func runSummary(item runtime.Run) string {
	parts := []string{fmt.Sprintf("Entrypoint %s", item.Entrypoint)}
	if item.Goal != "" {
		parts = append(parts, fmt.Sprintf("goal: %s", item.Goal))
	}
	if item.ActiveWorkflowID != "" {
		parts = append(parts, fmt.Sprintf("active workflow %s", item.ActiveWorkflowID))
	}
	if item.LatestDeliveryStatus != "" {
		parts = append(parts, fmt.Sprintf("latest delivery %s", item.LatestDeliveryStatus))
	}
	return strings.Join(parts, " | ")
}

func workflowSummary(item orchestration.Workflow) string {
	parts := []string{firstOperatorNonEmpty(item.PlanSummary, "Workflow state projected from daemon truth.")}
	if item.FailureSummary != "" {
		parts = append(parts, item.FailureSummary)
	}
	if item.LatestDeliveryStatus != "" {
		parts = append(parts, fmt.Sprintf("latest delivery %s", item.LatestDeliveryStatus))
	}
	return strings.Join(parts, " | ")
}

func scheduleSummary(item scheduler.Schedule) string {
	parts := []string{fmt.Sprintf("Trigger %s", item.Trigger.Kind)}
	if item.LastOutcome != "" {
		parts = append(parts, fmt.Sprintf("last outcome %s", item.LastOutcome))
	}
	if len(item.Attempts) > 0 {
		last := item.Attempts[len(item.Attempts)-1]
		if last.FailureReason != "" {
			parts = append(parts, last.FailureReason)
		} else if last.DispatchStatus != "" {
			parts = append(parts, fmt.Sprintf("dispatch %s", last.DispatchStatus))
		}
	}
	return strings.Join(parts, " | ")
}

func deliverySummary(item delivery.DeliveryOutcome) string {
	parts := []string{fmt.Sprintf("Source %s", item.SourceKind)}
	if item.PayloadPreview != "" {
		parts = append(parts, item.PayloadPreview)
	}
	if item.SuppressionReason != "" {
		parts = append(parts, item.SuppressionReason)
	}
	return strings.Join(parts, " | ")
}

func operatorActivityRecordFromEvent(item events.Event, environment string) (OperatorActivityRecord, bool) {
	sourceKind := operatorSourceKindForEvent(item)
	if sourceKind == "" || item.Resource.ID == "" {
		return OperatorActivityRecord{}, false
	}

	return OperatorActivityRecord{
		ActivityID:          "event-" + item.EventID,
		SourceKind:          sourceKind,
		SourceID:            item.Resource.ID,
		Title:               operatorEventTitle(item),
		Status:              operatorEventStatus(item),
		Summary:             operatorEventSummary(item),
		AttentionLevel:      attentionLevelForEvent(item),
		OccurredAt:          item.OccurredAt,
		DetailRoute:         operatorDetailRouteForEvent(item),
		RelatedResourceRefs: buildEventRefs(item),
		EnvironmentScope:    environment,
	}, true
}

func operatorSourceKindForEvent(item events.Event) string {
	switch item.Resource.Kind {
	case "approval", "schedule", "run", "workflow", "delivery", "computer_use_session", "computer_use_action":
		return item.Resource.Kind
	case "decision":
		return "approval"
	default:
		switch item.Category {
		case "approval", "schedule", "run", "workflow", "delivery", "computer_use":
			return item.Category
		default:
			return ""
		}
	}
}

func operatorEventTitle(item events.Event) string {
	sourceKind := operatorSourceKindForEvent(item)
	resourceLabel := firstOperatorNonEmpty(item.Resource.ID, item.Name)
	if sourceKind == "" {
		return firstOperatorNonEmpty(item.Name, "Operator event")
	}
	return fmt.Sprintf("%s %s", operatorSourceLabel(sourceKind), resourceLabel)
}

func operatorEventStatus(item events.Event) string {
	if status, ok := item.Payload["status"].(string); ok && strings.TrimSpace(status) != "" {
		return strings.TrimSpace(status)
	}
	if parts := strings.Split(strings.TrimSpace(item.Name), "."); len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return firstOperatorNonEmpty(item.Category, "observed")
}

func operatorEventSummary(item events.Event) string {
	status := operatorEventStatus(item)
	return fmt.Sprintf("%s via persisted event %s", strings.ReplaceAll(status, "_", " "), item.Name)
}

func attentionLevelForEvent(item events.Event) string {
	status := strings.ToLower(operatorEventStatus(item))
	name := strings.ToLower(item.Name)
	switch {
	case strings.Contains(status, "fail"), strings.Contains(status, "cancel"), strings.Contains(status, "reject"), strings.Contains(name, "failed"), strings.Contains(name, "cancelled"), strings.Contains(name, "rejected"):
		return "critical"
	case strings.Contains(status, "block"), strings.Contains(status, "wait"), strings.Contains(status, "pending"), strings.Contains(status, "pause"), strings.Contains(name, "blocked"), strings.Contains(name, "pending"), strings.Contains(name, "paused"):
		return "warning"
	default:
		return "info"
	}
}

func operatorDetailRouteForEvent(item events.Event) string {
	switch operatorSourceKindForEvent(item) {
	case "approval":
		if approvalID, ok := item.Payload["approvalId"].(string); ok && strings.TrimSpace(approvalID) != "" {
			return "/v1/policy/approvals/" + strings.TrimSpace(approvalID)
		}
		return "/v1/policy/approvals/" + item.Resource.ID
	case "schedule":
		return "/v1/schedules/" + item.Resource.ID
	case "run":
		return "/v1/runs/" + item.Resource.ID
	case "workflow":
		if item.Scope.RunID != "" {
			return "/v1/runs/" + item.Scope.RunID + "/workflows/" + item.Resource.ID
		}
	case "delivery":
		return "/v1/deliveries/" + item.Resource.ID
	case "computer_use_session":
		if item.Scope.RunID != "" {
			return "/v1/runs/" + item.Scope.RunID + "/computer-use/sessions/" + item.Resource.ID
		}
	case "computer_use_action":
		if item.Scope.RunID != "" && item.Scope.ComputerUseSessionID != "" {
			return "/v1/runs/" + item.Scope.RunID + "/computer-use/sessions/" + item.Scope.ComputerUseSessionID + "/actions/" + item.Resource.ID
		}
	}
	return ""
}

func buildEventRefs(item events.Event) []OperatorResourceRef {
	refs := make([]OperatorResourceRef, 0, 3)
	if item.Scope.RunID != "" {
		refs = append(refs, OperatorResourceRef{Kind: "run", ID: item.Scope.RunID, Route: "/v1/runs/" + item.Scope.RunID})
	}
	if item.Scope.WorkflowID != "" && item.Scope.RunID != "" {
		refs = append(refs, OperatorResourceRef{Kind: "workflow", ID: item.Scope.WorkflowID, Route: "/v1/runs/" + item.Scope.RunID + "/workflows/" + item.Scope.WorkflowID})
	}
	if item.Scope.ScheduleID != "" {
		refs = append(refs, OperatorResourceRef{Kind: "schedule", ID: item.Scope.ScheduleID, Route: "/v1/schedules/" + item.Scope.ScheduleID})
	}
	return refs
}

func operatorSourceLabel(sourceKind string) string {
	switch sourceKind {
	case "approval":
		return "Approval"
	case "schedule":
		return "Schedule"
	case "run":
		return "Run"
	case "workflow":
		return "Workflow"
	case "delivery":
		return "Delivery"
	case "computer_use_session":
		return "Computer Use Session"
	case "computer_use_action":
		return "Computer Use Action"
	default:
		return "Operator Event"
	}
}

func buildScheduleRefs(item scheduler.Schedule) []OperatorResourceRef {
	refs := make([]OperatorResourceRef, 0)
	for _, attempt := range item.Attempts {
		if attempt.RunID != "" {
			refs = append(refs, OperatorResourceRef{Kind: "run", ID: attempt.RunID, Route: "/v1/runs/" + attempt.RunID})
		}
		if attempt.WorkflowID != "" {
			refs = append(refs, OperatorResourceRef{Kind: "workflow", ID: attempt.WorkflowID, Route: "/v1/runs/" + attempt.RunID + "/workflows/" + attempt.WorkflowID})
		}
		if attempt.LatestDeliveryID != "" {
			refs = append(refs, OperatorResourceRef{Kind: "delivery", ID: attempt.LatestDeliveryID, Route: "/v1/deliveries/" + attempt.LatestDeliveryID})
		}
	}
	return refs
}

func buildDeliveryRefs(item delivery.DeliveryOutcome) []OperatorResourceRef {
	refs := make([]OperatorResourceRef, 0)
	if item.RunID != "" {
		refs = append(refs, OperatorResourceRef{Kind: "run", ID: item.RunID, Route: "/v1/runs/" + item.RunID})
	}
	if item.WorkflowID != "" {
		refs = append(refs, OperatorResourceRef{Kind: "workflow", ID: item.WorkflowID, Route: "/v1/runs/" + item.RunID + "/workflows/" + item.WorkflowID})
	}
	if item.ScheduleID != "" {
		refs = append(refs, OperatorResourceRef{Kind: "schedule", ID: item.ScheduleID, Route: "/v1/schedules/" + item.ScheduleID})
	}
	return refs
}

func firstOperatorNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
