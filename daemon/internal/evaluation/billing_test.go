package evaluation_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	. "github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

type countingReplayRecorder struct {
	calls int
}

func (r *countingReplayRecorder) RecordReplay(context.Context, ReplayRecordInput) (ReplayRecordResult, error) {
	r.calls++
	return ReplayRecordResult{RunID: "run_replay_recorded", WorkflowID: "workflow_replay_recorded"}, nil
}

func TestReplayEvaluationAttemptQuotaDeniesBeforeAttemptAndRuntimeWork(t *testing.T) {
	t.Parallel()

	ctx, sqliteStore, billingManager, tenantID := setupEvaluationBillingTest(t, map[billing.Category]int64{
		billing.CategoryReplayEvaluationAttempts: 0,
	})
	recorder := &countingReplayRecorder{}
	manager := NewManager(Dependencies{
		EnvironmentScope: "test",
		Store:            sqliteStore,
		RuntimeRecorder:  recorder,
		Billing:          billingManager,
	})
	candidate := testReplayCandidate("candidate_attempt_denied")
	if err := manager.UpsertReplayCandidate(ctx, candidate); err != nil {
		t.Fatalf("UpsertReplayCandidate returned error: %v", err)
	}

	_, err := manager.CreateReplayAttempt(ctx, candidate.CandidateID, CreateReplayAttemptInput{})
	if !errors.Is(err, billing.ErrQuotaDenied) {
		t.Fatalf("expected ErrQuotaDenied, got %v", err)
	}
	if recorder.calls != 0 {
		t.Fatalf("expected runtime recorder not to be called, got %d calls", recorder.calls)
	}
	attempts, err := manager.ListReplayAttempts(ctx, AttemptFilter{})
	if err != nil {
		t.Fatalf("ListReplayAttempts returned error: %v", err)
	}
	if len(attempts) != 0 {
		t.Fatalf("expected no replay attempt persisted before quota denial, got %+v", attempts)
	}
	assertEvaluationCounter(t, sqliteStore, tenantID, billing.CategoryReplayEvaluationAttempts, 0, 0)
}

func TestReplayRuntimeRecorderQuotaDeniesBeforeRuntimeRunCreation(t *testing.T) {
	t.Parallel()

	ctx, sqliteStore, billingManager, tenantID := setupEvaluationBillingTest(t, map[billing.Category]int64{
		billing.CategoryReplayEvaluationAttempts: 1,
		billing.CategoryRunLaunches:              0,
		billing.CategoryWorkflowLaunches:         1,
	})
	runtimeManager := runtime.NewManager()
	recorder := NewRuntimeReplayRecorder(runtimeManager, sqliteStore)
	recorder.ConfigureBilling(billingManager, false)
	manager := NewManager(Dependencies{
		EnvironmentScope: "test",
		Store:            sqliteStore,
		RuntimeRecorder:  recorder,
		Billing:          billingManager,
	})
	candidate := testReplayCandidate("candidate_runtime_denied")
	if err := manager.UpsertReplayCandidate(ctx, candidate); err != nil {
		t.Fatalf("UpsertReplayCandidate returned error: %v", err)
	}

	attempt, err := manager.CreateReplayAttempt(ctx, candidate.CandidateID, CreateReplayAttemptInput{})
	if err != nil {
		t.Fatalf("CreateReplayAttempt returned error: %v", err)
	}
	if attempt.Status != ReplayAttemptStatusFailed {
		t.Fatalf("expected failed attempt after replay runtime quota denial, got %+v", attempt)
	}
	if attempt.ResultRunID != "" || len(runtimeManager.ListRuns()) != 0 {
		t.Fatalf("expected no runtime run to be created before run quota denial, attempt=%+v runs=%+v", attempt, runtimeManager.ListRuns())
	}
	assertEvaluationCounter(t, sqliteStore, tenantID, billing.CategoryReplayEvaluationAttempts, 1, 0)
	assertEvaluationCounter(t, sqliteStore, tenantID, billing.CategoryRunLaunches, 0, 0)
}

func TestReplayRuntimeWorkflowReservationUsesActualRunID(t *testing.T) {
	t.Parallel()

	ctx, sqliteStore, billingManager, tenantID := setupEvaluationBillingTest(t, map[billing.Category]int64{
		billing.CategoryReplayEvaluationAttempts: 1,
		billing.CategoryRunLaunches:              1,
		billing.CategoryWorkflowLaunches:         1,
	})
	runtimeManager := runtime.NewManager()
	recorder := NewRuntimeReplayRecorder(runtimeManager, sqliteStore)
	recorder.ConfigureBilling(billingManager, false)
	manager := NewManager(Dependencies{
		EnvironmentScope: "test",
		Store:            sqliteStore,
		RuntimeRecorder:  recorder,
		Billing:          billingManager,
	})
	candidate := testReplayCandidate("candidate_runtime_operation_key")
	if err := manager.UpsertReplayCandidate(ctx, candidate); err != nil {
		t.Fatalf("UpsertReplayCandidate returned error: %v", err)
	}

	attempt, err := manager.CreateReplayAttempt(ctx, candidate.CandidateID, CreateReplayAttemptInput{})
	if err != nil {
		t.Fatalf("CreateReplayAttempt returned error: %v", err)
	}
	if attempt.ResultRunID == "" || attempt.ResultWorkflowID == "" {
		t.Fatalf("expected runtime replay IDs, got %+v", attempt)
	}
	operationKey := billing.WorkflowOperationKey(tenantID, attempt.ResultRunID, attempt.ResultWorkflowID, "evaluation:"+attempt.AttemptID)
	reservation, found, err := sqliteStore.ReservationByOperation(ctx, tenantID, billing.CategoryWorkflowLaunches, operationKey)
	if err != nil {
		t.Fatalf("ReservationByOperation returned error: %v", err)
	}
	if !found {
		t.Fatalf("expected workflow reservation with actual run id operation key %q", operationKey)
	}
	if strings.Contains(reservation.OperationKey, "workflow:unknown:") {
		t.Fatalf("expected actual run id in workflow reservation operation key, got %q", reservation.OperationKey)
	}
}

func setupEvaluationBillingTest(t *testing.T, limits map[billing.Category]int64) (context.Context, *store.SQLiteStore, *billing.Manager, string) {
	t.Helper()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	ctx := context.Background()
	if err := sqliteStore.EnsureBillingCatalog(ctx); err != nil {
		t.Fatalf("EnsureBillingCatalog returned error: %v", err)
	}
	tenantID := "ten_eval_" + strings.NewReplacer("/", "_").Replace(t.Name())
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{
		PlanID:          "plan_" + tenantID,
		TenantID:        tenantID,
		PlanKey:         "finite",
		Status:          billing.PlanStatusActive,
		EnforcementMode: billing.EnforcementModeEnforced,
		EffectiveAt:     now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	for category, limit := range limits {
		if err := sqliteStore.SaveQuotaOverride(ctx, billing.QuotaOverride{
			QuotaOverrideID: "override_" + tenantID + "_" + string(category),
			TenantID:        tenantID,
			Category:        category,
			Limit:           &limit,
			EffectiveAt:     now.Add(-time.Minute),
			Reason:          "test evaluation billing limit",
		}); err != nil {
			t.Fatalf("SaveQuotaOverride(%s) returned error: %v", category, err)
		}
	}
	ctx = tenantctx.WithContext(ctx, identity.TenantContext{TenantID: tenantID, PrincipalID: "prn_" + tenantID})
	return ctx, sqliteStore, billing.NewManager(sqliteStore), tenantID
}

func testReplayCandidate(candidateID string) ReplayCandidate {
	return ReplayCandidate{
		CandidateID:        candidateID,
		CandidateKind:      CandidateKindCuratedWork,
		DisplayName:        "Replay Candidate " + candidateID,
		SourceKind:         SourceKindRun,
		SourceID:           "run_" + candidateID,
		SourceRefs:         []SourceRef{{Kind: SourceKindRun, ID: "run_" + candidateID, Route: "/v1/runs/run_" + candidateID}},
		EnvironmentScope:   "test",
		ReadinessStatus:    ReadinessFullyReplayable,
		DefaultReplayMode:  ReplayModeNonLive,
		ExpectedComparison: PlaneSummaries{Runtime: "runtime captured", Policy: "policy captured", Evidence: "evidence captured"},
	}
}

func assertEvaluationCounter(t *testing.T, sqliteStore *store.SQLiteStore, tenantID string, category billing.Category, committed, reserved int64) {
	t.Helper()

	definition, ok := billing.DefinitionFor(category)
	if !ok {
		t.Fatalf("%s quota definition missing", category)
	}
	period, err := sqliteStore.OpenPeriod(context.Background(), tenantID, definition, time.Now().UTC())
	if err != nil {
		t.Fatalf("OpenPeriod(%s) returned error: %v", category, err)
	}
	counter, ok, err := sqliteStore.UsageCounter(context.Background(), tenantID, category, period.QuotaPeriodID)
	if err != nil {
		t.Fatalf("UsageCounter(%s) returned error: %v", category, err)
	}
	if !ok {
		counter = billing.UsageCounter{}
	}
	if counter.CommittedAmount != committed || counter.ReservedAmount != reserved {
		t.Fatalf("expected %s counter committed=%d reserved=%d, got %+v", category, committed, reserved, counter)
	}
}
