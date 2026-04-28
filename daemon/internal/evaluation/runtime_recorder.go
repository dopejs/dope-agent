package evaluation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
)

const replayRuntimeEntrypoint = "evaluation.replay"
const replayRedactedCredential = "[REDACTED]"

var replayCredentialLeakMarkers = []string{
	"R37_FAKE_SECRET",
	"R37_FAKE_TOKEN",
	"secret-token",
}

type RuntimeRecorder interface {
	RecordReplay(context.Context, ReplayRecordInput) (ReplayRecordResult, error)
}

type ReplayRecordInput struct {
	Candidate ReplayCandidate
	Attempt   ReplayAttempt
	Evidence  CapturedEvidence
	Now       time.Time
}

type ReplayRecordResult struct {
	RunID        string
	WorkflowID   string
	EvidenceRefs []SourceRef
}

type ReplayRuntime interface {
	CreateRun(runtime.CreateRunInput) (runtime.Run, error)
	CreateStep(string, runtime.CreateStepInput) (runtime.Step, error)
	UpdateStepStatusAndReconcileRun(string, string, runtime.UpdateStepStatusInput) (runtime.Step, *runtime.Run, error)
}

type ReplayRuntimeStore interface {
	UpsertRun(context.Context, runtime.Run) error
	UpsertStep(context.Context, runtime.Step) error
	UpsertWorkflow(context.Context, orchestration.Workflow) error
	ReplaceWorkflowSteps(context.Context, string, []orchestration.WorkflowStep) error
}

type RuntimeReplayRecorder struct {
	runtime ReplayRuntime
	store   ReplayRuntimeStore
}

func NewRuntimeReplayRecorder(runtimeManager ReplayRuntime, store ReplayRuntimeStore) *RuntimeReplayRecorder {
	return &RuntimeReplayRecorder{
		runtime: runtimeManager,
		store:   store,
	}
}

func (r *RuntimeReplayRecorder) RecordReplay(ctx context.Context, input ReplayRecordInput) (ReplayRecordResult, error) {
	if r == nil || r.runtime == nil {
		return ReplayRecordResult{}, nil
	}
	input = redactReplayRecordInput(input)
	run, err := r.runtime.CreateRun(runtime.CreateRunInput{
		Entrypoint: replayRuntimeEntrypoint,
		Goal:       replayRunGoal(input.Candidate, input.Attempt),
	})
	if err != nil {
		return ReplayRecordResult{}, err
	}
	if err := r.upsertRun(ctx, run); err != nil {
		return ReplayRecordResult{}, err
	}
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	workflowID := newID("workflow")
	workflowStepID := newID("workflow_step")

	step, err := r.runtime.CreateStep(run.RunID, runtime.CreateStepInput{
		Title:          "Replay captured evidence",
		Kind:           "evaluation_replay",
		WorkflowID:     workflowID,
		WorkflowStepID: workflowStepID,
		Input: map[string]any{
			"candidateId":       input.Candidate.CandidateID,
			"attemptId":         input.Attempt.AttemptID,
			"mode":              input.Attempt.Mode,
			"changeWindowLabel": input.Attempt.ChangeWindowLabel,
			"sourceRefs":        input.Candidate.SourceRefs,
			"evidenceRefs":      input.Attempt.EvidenceRefs,
		},
	})
	if err != nil {
		return ReplayRecordResult{}, err
	}
	if err := r.upsertStep(ctx, step); err != nil {
		return ReplayRecordResult{}, err
	}
	step, runUpdate, err := r.runtime.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusPlanning,
	})
	if err != nil {
		return ReplayRecordResult{}, err
	}
	if err := r.upsertStep(ctx, step); err != nil {
		return ReplayRecordResult{}, err
	}
	if runUpdate != nil {
		run = *runUpdate
		if err := r.upsertRun(ctx, run); err != nil {
			return ReplayRecordResult{}, err
		}
	}
	step, runUpdate, err = r.runtime.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusExecutingTool,
	})
	if err != nil {
		return ReplayRecordResult{}, err
	}
	if err := r.upsertStep(ctx, step); err != nil {
		return ReplayRecordResult{}, err
	}
	if runUpdate != nil {
		run = *runUpdate
		if err := r.upsertRun(ctx, run); err != nil {
			return ReplayRecordResult{}, err
		}
	}
	step, runUpdate, err = r.runtime.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusCompleted,
		Output: map[string]any{
			"runtimeSummary":     input.Attempt.RuntimeSummary,
			"policySummary":      input.Attempt.PolicySummary,
			"integrationSummary": input.Attempt.IntegrationSummary,
			"deliverySummary":    input.Attempt.DeliverySummary,
			"evidenceSummary":    input.Attempt.EvidenceSummary,
			"evidence":           input.Evidence,
		},
	})
	if err != nil {
		return ReplayRecordResult{}, err
	}
	if err := r.upsertStep(ctx, step); err != nil {
		return ReplayRecordResult{}, err
	}
	if runUpdate != nil {
		run = *runUpdate
		if err := r.upsertRun(ctx, run); err != nil {
			return ReplayRecordResult{}, err
		}
	}
	workflow := replayWorkflow(input, run.RunID, workflowID, workflowStepID, step.StepID, now)
	if err := r.upsertWorkflow(ctx, workflow); err != nil {
		return ReplayRecordResult{}, err
	}
	if err := r.replaceWorkflowSteps(ctx, workflow.WorkflowID, workflow.Steps); err != nil {
		return ReplayRecordResult{}, err
	}

	return ReplayRecordResult{
		RunID:      run.RunID,
		WorkflowID: workflow.WorkflowID,
		EvidenceRefs: []SourceRef{{
			Kind:  SourceKindRun,
			ID:    run.RunID,
			Route: "/v1/runs/" + run.RunID,
		}, {
			Kind:  SourceKindWorkflow,
			ID:    workflow.WorkflowID,
			Route: "/v1/runs/" + run.RunID + "/workflows/" + workflow.WorkflowID,
		}},
	}, nil
}

func (r *RuntimeReplayRecorder) upsertRun(ctx context.Context, run runtime.Run) error {
	if r.store == nil {
		return nil
	}
	if err := r.store.UpsertRun(ctx, run); err != nil {
		return fmt.Errorf("upsert run %s: %w", run.RunID, err)
	}
	return nil
}

func (r *RuntimeReplayRecorder) upsertStep(ctx context.Context, step runtime.Step) error {
	if r.store == nil {
		return nil
	}
	if err := r.store.UpsertStep(ctx, step); err != nil {
		return fmt.Errorf("upsert step %s: %w", step.StepID, err)
	}
	return nil
}

func (r *RuntimeReplayRecorder) upsertWorkflow(ctx context.Context, workflow orchestration.Workflow) error {
	if r.store == nil {
		return nil
	}
	if err := r.store.UpsertWorkflow(ctx, workflow); err != nil {
		return fmt.Errorf("upsert workflow %s: %w", workflow.WorkflowID, err)
	}
	return nil
}

func (r *RuntimeReplayRecorder) replaceWorkflowSteps(ctx context.Context, workflowID string, steps []orchestration.WorkflowStep) error {
	if r.store == nil {
		return nil
	}
	if err := r.store.ReplaceWorkflowSteps(ctx, workflowID, steps); err != nil {
		return fmt.Errorf("replace workflow steps %s: %w", workflowID, err)
	}
	return nil
}

func redactReplayRecordInput(input ReplayRecordInput) ReplayRecordInput {
	input.Attempt.ChangeWindowLabel = redactReplayCredentialString(input.Attempt.ChangeWindowLabel)
	input.Attempt.RuntimeSummary = redactReplayCredentialString(input.Attempt.RuntimeSummary)
	input.Attempt.PolicySummary = redactReplayCredentialString(input.Attempt.PolicySummary)
	input.Attempt.IntegrationSummary = redactReplayCredentialString(input.Attempt.IntegrationSummary)
	input.Attempt.DeliverySummary = redactReplayCredentialString(input.Attempt.DeliverySummary)
	input.Attempt.EvidenceSummary = redactReplayCredentialString(input.Attempt.EvidenceSummary)
	input.Attempt.BlockedReasons = redactReplayCredentialStrings(input.Attempt.BlockedReasons)
	input.Evidence.RuntimeSummary = redactReplayCredentialString(input.Evidence.RuntimeSummary)
	input.Evidence.PolicySummary = redactReplayCredentialString(input.Evidence.PolicySummary)
	input.Evidence.IntegrationSummary = redactReplayCredentialString(input.Evidence.IntegrationSummary)
	input.Evidence.DeliverySummary = redactReplayCredentialString(input.Evidence.DeliverySummary)
	input.Evidence.EvidenceSummary = redactReplayCredentialString(input.Evidence.EvidenceSummary)
	input.Evidence.BlockedReasons = redactReplayCredentialStrings(input.Evidence.BlockedReasons)
	input.Evidence.Limitations = redactReplayCredentialStrings(input.Evidence.Limitations)
	return input
}

func redactReplayCredentialStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	out := append([]string(nil), values...)
	for i := range out {
		out[i] = redactReplayCredentialString(out[i])
	}
	return out
}

func redactReplayCredentialString(value string) string {
	for _, marker := range replayCredentialLeakMarkers {
		if marker != "" && strings.Contains(value, marker) {
			return replayRedactedCredential
		}
	}
	return value
}

func replayRunGoal(candidate ReplayCandidate, attempt ReplayAttempt) string {
	name := firstNonEmpty(candidate.DisplayName, candidate.CandidateID)
	if attempt.ChangeWindowLabel == "" {
		return "Replay evaluation candidate " + name
	}
	return "Replay evaluation candidate " + name + " for " + attempt.ChangeWindowLabel
}

func replayWorkflow(input ReplayRecordInput, runID, workflowID, workflowStepID, runtimeStepID string, now time.Time) orchestration.Workflow {
	goal := replayRunGoal(input.Candidate, input.Attempt)
	step := orchestration.WorkflowStep{
		WorkflowStepID: workflowStepID,
		WorkflowID:     workflowID,
		Title:          "Replay captured evidence",
		Position:       1,
		ConsumerKind:   "evaluation",
		ConsumerID:     input.Candidate.CandidateID,
		ToolName:       replayRuntimeEntrypoint,
		Input: map[string]any{
			"candidateId": input.Candidate.CandidateID,
			"attemptId":   input.Attempt.AttemptID,
			"sourceRefs":  input.Candidate.SourceRefs,
			"evidenceRefs": append(
				append([]SourceRef(nil), input.Attempt.EvidenceRefs...),
				input.Candidate.CapturedEvidenceRefs...,
			),
		},
		Status:        orchestration.StepStatusCompleted,
		RuntimeStepID: runtimeStepID,
		AttemptCount:  1,
		MaxAttempts:   1,
		OutputSummary: input.Attempt.EvidenceSummary,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	startedAt := now
	completedAt := now
	return orchestration.Workflow{
		WorkflowID:       workflowID,
		RunID:            runID,
		EnvironmentScope: input.Candidate.EnvironmentScope,
		Goal:             goal,
		Status:           orchestration.WorkflowStatusCompleted,
		PlanSummary:      "Replay captured evidence through the evaluation runtime envelope.",
		CreatedAt:        now,
		UpdatedAt:        now,
		StartedAt:        &startedAt,
		CompletedAt:      &completedAt,
		Steps:            []orchestration.WorkflowStep{step},
	}
}
