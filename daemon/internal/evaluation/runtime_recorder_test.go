package evaluation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
)

type memoryRuntimeStore struct {
	runs      map[string]runtime.Run
	steps     map[string]runtime.Step
	workflows map[string]orchestration.Workflow
}

func TestRuntimeReplayRecorderPreservesRedactedCredentialEvidence(t *testing.T) {
	ctx := context.Background()
	runtimeManager := runtime.NewManager()
	runtimeStore := newMemoryRuntimeStore()
	recorder := NewRuntimeReplayRecorder(runtimeManager, runtimeStore)
	_, err := recorder.RecordReplay(ctx, ReplayRecordInput{
		Candidate: ReplayCandidate{
			CandidateID: "candidate_redacted_runtime",
			SourceKind:  SourceKindRun,
			SourceID:    "run_redacted_source",
			SourceRefs:  []SourceRef{{Kind: SourceKindRun, ID: "run_redacted_source"}},
		},
		Attempt: ReplayAttempt{
			AttemptID:       "attempt_redacted_runtime",
			Mode:            ReplayModeNonLive,
			RuntimeSummary:  "credential-backed runtime completed with token=R37_FAKE_SECRET_TENANT_A_DO_NOT_LEAK",
			PolicySummary:   "policy blocked secret-token",
			EvidenceSummary: "captured output was already redacted",
			BlockedReasons:  []string{"never persist R37_FAKE_TOKEN_TENANT_B_DO_NOT_LEAK"},
			EvidenceRefs:    []SourceRef{{Kind: SourceKindRun, ID: "run_redacted_source"}},
		},
		Evidence: CapturedEvidence{
			RuntimeSummary:     "credential-backed runtime completed with token=R37_FAKE_SECRET_TENANT_A_DO_NOT_LEAK",
			PolicySummary:      "policy blocked secret-token",
			IntegrationSummary: "integration used R37_FAKE_TOKEN_TENANT_B_DO_NOT_LEAK",
			EvidenceSummary:    "captured output was already redacted",
			Limitations:        []string{"do not persist R37_FAKE_SECRET_TENANT_B_DO_NOT_LEAK"},
		},
	})
	if err != nil {
		t.Fatalf("RecordReplay returned error: %v", err)
	}
	data, err := json.Marshal(map[string]any{
		"runs":      runtimeStore.runs,
		"steps":     runtimeStore.steps,
		"workflows": runtimeStore.workflows,
	})
	if err != nil {
		t.Fatalf("marshal runtime store: %v", err)
	}
	if strings.Contains(string(data), "secret-token") || strings.Contains(string(data), "R37_FAKE_SECRET") {
		t.Fatalf("runtime replay artifacts leaked credential material: %s", string(data))
	}
	if strings.Contains(string(data), "R37_FAKE_TOKEN") {
		t.Fatalf("runtime replay artifacts leaked token material: %s", string(data))
	}
	if !strings.Contains(string(data), "[REDACTED]") {
		t.Fatalf("expected redacted credential marker to be preserved: %s", string(data))
	}
}

func newMemoryRuntimeStore() *memoryRuntimeStore {
	return &memoryRuntimeStore{
		runs:      map[string]runtime.Run{},
		steps:     map[string]runtime.Step{},
		workflows: map[string]orchestration.Workflow{},
	}
}

func (s *memoryRuntimeStore) UpsertRun(_ context.Context, run runtime.Run) error {
	s.runs[run.RunID] = run
	return nil
}

func (s *memoryRuntimeStore) UpsertStep(_ context.Context, step runtime.Step) error {
	s.steps[step.StepID] = step
	return nil
}

func (s *memoryRuntimeStore) UpsertWorkflow(_ context.Context, workflow orchestration.Workflow) error {
	s.workflows[workflow.WorkflowID] = workflow
	return nil
}

func (s *memoryRuntimeStore) ReplaceWorkflowSteps(_ context.Context, workflowID string, steps []orchestration.WorkflowStep) error {
	workflow := s.workflows[workflowID]
	workflow.Steps = append([]orchestration.WorkflowStep(nil), steps...)
	s.workflows[workflowID] = workflow
	return nil
}

func TestManagerRecordsCompletedReplayInRuntimePlane(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	runtimeManager := runtime.NewManager()
	runtimeStore := newMemoryRuntimeStore()
	manager := NewManager(Dependencies{
		EnvironmentScope: "test",
		Store:            store,
		RuntimeRecorder:  NewRuntimeReplayRecorder(runtimeManager, runtimeStore),
		Clock:            fixedClock,
	})
	candidate := ReplayCandidate{
		CandidateID:       "candidate_runtime_record",
		CandidateKind:     CandidateKindCuratedWork,
		DisplayName:       "Runtime record candidate",
		SourceKind:        SourceKindRun,
		SourceID:          "run_source",
		EnvironmentScope:  "test",
		ReadinessStatus:   ReadinessFullyReplayable,
		DefaultReplayMode: ReplayModeNonLive,
		SourceRefs:        []SourceRef{{Kind: SourceKindRun, ID: "run_source", Route: "/v1/runs/run_source"}},
		ExpectedComparison: PlaneSummaries{
			Runtime:  "expected runtime",
			Policy:   "expected policy",
			Evidence: "expected evidence",
		},
	}
	if err := manager.UpsertReplayCandidate(ctx, candidate); err != nil {
		t.Fatalf("UpsertReplayCandidate returned error: %v", err)
	}

	attempt, err := manager.CreateReplayAttempt(ctx, candidate.CandidateID, CreateReplayAttemptInput{})
	if err != nil {
		t.Fatalf("CreateReplayAttempt returned error: %v", err)
	}
	if attempt.ResultRunID == "" {
		t.Fatalf("expected replay attempt to link a runtime run, got %+v", attempt)
	}
	if attempt.ResultWorkflowID == "" {
		t.Fatalf("expected replay attempt to link a workflow, got %+v", attempt)
	}
	if _, ok := runtimeManager.GetRun(attempt.ResultRunID); !ok {
		t.Fatalf("expected runtime manager to contain replay run %s", attempt.ResultRunID)
	}
	if _, ok := runtimeStore.runs[attempt.ResultRunID]; !ok {
		t.Fatalf("expected runtime store to persist replay run %s", attempt.ResultRunID)
	}
	workflow, ok := runtimeStore.workflows[attempt.ResultWorkflowID]
	if !ok {
		t.Fatalf("expected runtime store to persist replay workflow %s", attempt.ResultWorkflowID)
	}
	if workflow.RunID != attempt.ResultRunID || workflow.Status != orchestration.WorkflowStatusCompleted {
		t.Fatalf("expected completed replay workflow linked to run, got %+v", workflow)
	}
	if len(runtimeStore.steps) != 1 {
		t.Fatalf("expected one persisted replay step, got %d", len(runtimeStore.steps))
	}
	if len(attempt.EvidenceRefs) < 2 {
		t.Fatalf("expected attempt evidence refs to include replay run and workflow, got %+v", attempt.EvidenceRefs)
	}
	if attempt.EvidenceRefs[len(attempt.EvidenceRefs)-2].ID != attempt.ResultRunID || attempt.EvidenceRefs[len(attempt.EvidenceRefs)-1].ID != attempt.ResultWorkflowID {
		t.Fatalf("expected attempt evidence refs to include replay run and workflow, got %+v", attempt.EvidenceRefs)
	}
}
