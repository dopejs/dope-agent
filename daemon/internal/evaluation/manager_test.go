package evaluation

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type memoryStore struct {
	candidates  map[string]ReplayCandidate
	attempts    map[string]ReplayAttempt
	comparisons map[string]ComparisonResult
	fixtures    map[string]RegressionFixture
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		candidates:  map[string]ReplayCandidate{},
		attempts:    map[string]ReplayAttempt{},
		comparisons: map[string]ComparisonResult{},
		fixtures:    map[string]RegressionFixture{},
	}
}

func (s *memoryStore) UpsertReplayCandidate(_ context.Context, item ReplayCandidate) error {
	s.candidates[item.CandidateID] = item
	return nil
}

func (s *memoryStore) ListReplayCandidates(_ context.Context, filter CandidateFilter) ([]ReplayCandidate, error) {
	items := make([]ReplayCandidate, 0, len(s.candidates))
	for _, item := range s.candidates {
		if filter.EnvironmentScope != "" && item.EnvironmentScope != filter.EnvironmentScope {
			continue
		}
		if filter.CandidateKind != "" && item.CandidateKind != filter.CandidateKind {
			continue
		}
		if filter.SourceKind != "" && item.SourceKind != filter.SourceKind {
			continue
		}
		if filter.ReadinessStatus != "" && item.ReadinessStatus != filter.ReadinessStatus {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *memoryStore) GetReplayCandidate(_ context.Context, _ string, candidateID string) (ReplayCandidate, bool, error) {
	item, ok := s.candidates[candidateID]
	return item, ok, nil
}

func (s *memoryStore) UpsertReplayAttempt(_ context.Context, item ReplayAttempt) error {
	s.attempts[item.AttemptID] = item
	return nil
}

func (s *memoryStore) ListReplayAttempts(_ context.Context, filter AttemptFilter) ([]ReplayAttempt, error) {
	items := make([]ReplayAttempt, 0, len(s.attempts))
	for _, item := range s.attempts {
		if filter.EnvironmentScope != "" && item.EnvironmentScope != filter.EnvironmentScope {
			continue
		}
		if filter.CandidateID != "" && item.CandidateID != filter.CandidateID {
			continue
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *memoryStore) GetReplayAttempt(_ context.Context, _ string, attemptID string) (ReplayAttempt, bool, error) {
	item, ok := s.attempts[attemptID]
	return item, ok, nil
}

func (s *memoryStore) UpsertComparisonResult(_ context.Context, item ComparisonResult) error {
	s.comparisons[item.ComparisonID] = item
	return nil
}

func (s *memoryStore) ListComparisonResults(_ context.Context, filter ComparisonFilter) ([]ComparisonResult, error) {
	items := make([]ComparisonResult, 0, len(s.comparisons))
	for _, item := range s.comparisons {
		if filter.EnvironmentScope != "" && item.EnvironmentScope != filter.EnvironmentScope {
			continue
		}
		if filter.CandidateID != "" && item.CandidateID != filter.CandidateID {
			continue
		}
		if filter.AttemptID != "" && item.AttemptID != filter.AttemptID {
			continue
		}
		if filter.TerminalStatus != "" && item.TerminalStatus != filter.TerminalStatus {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *memoryStore) GetComparisonResult(_ context.Context, _ string, comparisonID string) (ComparisonResult, bool, error) {
	item, ok := s.comparisons[comparisonID]
	return item, ok, nil
}

func (s *memoryStore) UpsertRegressionFixture(_ context.Context, item RegressionFixture) error {
	s.fixtures[item.FixtureID] = item
	return nil
}

func (s *memoryStore) ListRegressionFixtures(_ context.Context, filter FixtureFilter) ([]RegressionFixture, error) {
	items := make([]RegressionFixture, 0, len(s.fixtures))
	for _, item := range s.fixtures {
		if filter.EnvironmentScope != "" && item.EnvironmentScope != filter.EnvironmentScope {
			continue
		}
		if filter.DomainClass != "" && item.DomainClass != filter.DomainClass {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func TestManagerLoadsFixtureCandidatesAndLaunchesNonLiveReplay(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	manager := NewManager(Dependencies{
		EnvironmentScope: "test",
		Store:            store,
		FixturesDir:      filepath.Join("testdata", "fixtures"),
		Clock:            fixedClock,
	})

	if err := manager.LoadFixtures(ctx); err != nil {
		t.Fatalf("LoadFixtures returned error: %v", err)
	}

	candidates, err := manager.ListReplayCandidates(ctx, CandidateFilter{EnvironmentScope: "test"})
	if err != nil {
		t.Fatalf("ListReplayCandidates returned error: %v", err)
	}
	if len(candidates) != 3 {
		t.Fatalf("expected 3 fixture candidates, got %d", len(candidates))
	}
	if candidates[0].DefaultReplayMode != ReplayModeNonLive {
		t.Fatalf("expected non-live default mode, got %s", candidates[0].DefaultReplayMode)
	}

	attempt, err := manager.CreateReplayAttempt(ctx, candidates[0].CandidateID, CreateReplayAttemptInput{})
	if err != nil {
		t.Fatalf("CreateReplayAttempt returned error: %v", err)
	}
	if attempt.Mode != ReplayModeNonLive {
		t.Fatalf("expected non-live mode, got %s", attempt.Mode)
	}
	if attempt.Status != ReplayAttemptStatusCompleted {
		t.Fatalf("expected completed replay processing, got %s", attempt.Status)
	}
	if attempt.SideEffectHandling != SideEffectEvidenceOnly {
		t.Fatalf("expected evidence-only side effect handling, got %s", attempt.SideEffectHandling)
	}
	if len(attempt.EvidenceRefs) == 0 {
		t.Fatal("expected replay attempt evidence refs")
	}
}

func TestManagerBlocksUnreadyCandidateWithoutRunningSideEffects(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	manager := NewManager(Dependencies{EnvironmentScope: "test", Store: store, Clock: fixedClock})
	candidate := ReplayCandidate{
		CandidateID:       "candidate_blocked",
		CandidateKind:     CandidateKindCuratedWork,
		DisplayName:       "Blocked approval",
		SourceKind:        SourceKindRun,
		SourceID:          "run_blocked",
		EnvironmentScope:  "test",
		ReadinessStatus:   ReadinessBlocked,
		ReadinessReasons:  []string{"approval-gated side effect requires live validation"},
		Limitations:       []string{"default replay remains non-live"},
		DefaultReplayMode: ReplayModeNonLive,
		SourceRefs:        []SourceRef{{Kind: SourceKindRun, ID: "run_blocked", Route: "/v1/runs/run_blocked"}},
	}
	if err := manager.UpsertReplayCandidate(ctx, candidate); err != nil {
		t.Fatalf("UpsertReplayCandidate returned error: %v", err)
	}

	attempt, err := manager.CreateReplayAttempt(ctx, candidate.CandidateID, CreateReplayAttemptInput{})
	if err != nil {
		t.Fatalf("CreateReplayAttempt returned error: %v", err)
	}
	if attempt.Status != ReplayAttemptStatusBlocked {
		t.Fatalf("expected blocked attempt, got %s", attempt.Status)
	}
	if attempt.SideEffectHandling != SideEffectBlocked {
		t.Fatalf("expected blocked side effect handling, got %s", attempt.SideEffectHandling)
	}
	if len(attempt.BlockedReasons) == 0 {
		t.Fatal("expected blocked reasons")
	}
}

func TestManagerRejectsReplayCandidateWithoutSourceProvenance(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	manager := NewManager(Dependencies{EnvironmentScope: "test", Store: store, Clock: fixedClock})
	candidate := ReplayCandidate{
		CandidateID:       "candidate_missing_source",
		CandidateKind:     CandidateKindCuratedWork,
		DisplayName:       "Missing Source",
		EnvironmentScope:  "test",
		ReadinessStatus:   ReadinessFullyReplayable,
		DefaultReplayMode: ReplayModeNonLive,
	}

	err := manager.UpsertReplayCandidate(ctx, candidate)
	if err == nil {
		t.Fatal("expected missing source provenance to be rejected")
	}
	if _, ok, getErr := manager.GetReplayCandidate(ctx, candidate.CandidateID); getErr != nil || ok {
		t.Fatalf("expected rejected candidate not to be stored, ok=%v err=%v", ok, getErr)
	}
}

func TestManagerCreatesPlaneLevelComparison(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	manager := NewManager(Dependencies{
		EnvironmentScope: "test",
		Store:            store,
		FixturesDir:      filepath.Join("testdata", "fixtures"),
		Clock:            fixedClock,
	})
	if err := manager.LoadFixtures(ctx); err != nil {
		t.Fatalf("LoadFixtures returned error: %v", err)
	}
	candidates, err := manager.ListReplayCandidates(ctx, CandidateFilter{EnvironmentScope: "test", SourceKind: SourceKindFixture})
	if err != nil {
		t.Fatalf("ListReplayCandidates returned error: %v", err)
	}
	attempt, err := manager.CreateReplayAttempt(ctx, candidates[0].CandidateID, CreateReplayAttemptInput{ChangeWindowLabel: "phase-33"})
	if err != nil {
		t.Fatalf("CreateReplayAttempt returned error: %v", err)
	}

	comparison, err := manager.CreateComparison(ctx, attempt.AttemptID, CreateComparisonInput{ChangeWindowLabel: "phase-33"})
	if err != nil {
		t.Fatalf("CreateComparison returned error: %v", err)
	}
	if comparison.TerminalStatus != ComparisonMatched {
		t.Fatalf("expected matched comparison, got %s", comparison.TerminalStatus)
	}
	if comparison.RuntimeSummary == "" || comparison.PolicySummary == "" || comparison.EvidenceSummary == "" {
		t.Fatalf("expected plane summaries, got %+v", comparison)
	}
	if comparison.ChangeWindowLabel != "phase-33" {
		t.Fatalf("expected change window label, got %s", comparison.ChangeWindowLabel)
	}
}

func TestFixtureReplayUsesCapturedEvidenceInsteadOfExpectedSummary(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	fixtureDir := filepath.Join(root, "runtime-drift")
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixtureDir, "manifest.json"), []byte(`{
		"fixtureId": "fixture_runtime_drift",
		"displayName": "Runtime Drift Fixture",
		"domainClass": "schedule",
		"sourceRefs": [{"kind":"schedule","id":"sched_drift"}],
		"capturedEvidenceRefs": [{"kind":"fixture_evidence","id":"evidence.json"}],
		"assumptions": ["test fixture"],
		"limitations": [],
		"expectedReplayMode": "non_live",
		"expectedComparisonSummary": {
			"runtime": "baseline runtime",
			"policy": "baseline policy",
			"integration": "baseline integration",
			"delivery": "baseline delivery",
			"evidence": "baseline evidence"
		}
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixtureDir, "evidence.json"), []byte(`{
		"terminalStatus": "completed",
		"runtime": "replayed runtime from captured evidence",
		"policy": "baseline policy",
		"integration": "baseline integration",
		"delivery": "baseline delivery",
		"evidence": "baseline evidence"
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile(evidence) returned error: %v", err)
	}

	store := newMemoryStore()
	manager := NewManager(Dependencies{EnvironmentScope: "test", Store: store, FixturesDir: root, Clock: fixedClock})
	if err := manager.LoadFixtures(ctx); err != nil {
		t.Fatalf("LoadFixtures returned error: %v", err)
	}
	attempt, err := manager.CreateReplayAttempt(ctx, "candidate_fixture_runtime_drift", CreateReplayAttemptInput{})
	if err != nil {
		t.Fatalf("CreateReplayAttempt returned error: %v", err)
	}
	if attempt.RuntimeSummary != "replayed runtime from captured evidence" {
		t.Fatalf("expected runtime from evidence file, got %q", attempt.RuntimeSummary)
	}
	comparison, err := manager.CreateComparison(ctx, attempt.AttemptID, CreateComparisonInput{})
	if err != nil {
		t.Fatalf("CreateComparison returned error: %v", err)
	}
	if comparison.TerminalStatus != ComparisonDrifted || len(comparison.DriftFindings) != 1 || comparison.DriftFindings[0].Plane != DriftPlaneRuntime {
		t.Fatalf("expected runtime drift from evidence replay, got %+v", comparison)
	}
}

func TestComparisonCanUseBaselineAttemptEvidence(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	manager := NewManager(Dependencies{EnvironmentScope: "test", Store: store, Clock: fixedClock})
	candidate := ReplayCandidate{
		CandidateID:       "candidate_baseline",
		CandidateKind:     CandidateKindCuratedWork,
		DisplayName:       "Baseline candidate",
		SourceKind:        SourceKindRun,
		SourceID:          "run_source",
		EnvironmentScope:  "test",
		ReadinessStatus:   ReadinessFullyReplayable,
		DefaultReplayMode: ReplayModeNonLive,
		SourceRefs:        []SourceRef{{Kind: SourceKindRun, ID: "run_source"}},
	}
	if err := manager.UpsertReplayCandidate(ctx, candidate); err != nil {
		t.Fatalf("UpsertReplayCandidate returned error: %v", err)
	}
	baseline := ReplayAttempt{
		AttemptID:          "attempt_baseline",
		CandidateID:        candidate.CandidateID,
		EnvironmentScope:   "test",
		Mode:               ReplayModeNonLive,
		Status:             ReplayAttemptStatusCompleted,
		ApprovalHandling:   ApprovalEvidenceOnly,
		SideEffectHandling: SideEffectEvidenceOnly,
		RuntimeSummary:     "baseline runtime",
		PolicySummary:      "same policy",
		IntegrationSummary: "same integration",
		DeliverySummary:    "same delivery",
		EvidenceSummary:    "same evidence",
		CreatedAt:          fixedClock(),
		UpdatedAt:          fixedClock(),
	}
	replay := baseline
	replay.AttemptID = "attempt_replay"
	replay.RuntimeSummary = "changed runtime"
	if err := store.UpsertReplayAttempt(ctx, baseline); err != nil {
		t.Fatalf("UpsertReplayAttempt(baseline) returned error: %v", err)
	}
	if err := store.UpsertReplayAttempt(ctx, replay); err != nil {
		t.Fatalf("UpsertReplayAttempt(replay) returned error: %v", err)
	}

	comparison, err := manager.CreateComparison(ctx, replay.AttemptID, CreateComparisonInput{BaselineAttemptID: baseline.AttemptID})
	if err != nil {
		t.Fatalf("CreateComparison returned error: %v", err)
	}
	if comparison.BaselineRef != baseline.AttemptID || comparison.TerminalStatus != ComparisonDrifted {
		t.Fatalf("expected baseline-attempt drift comparison, got %+v", comparison)
	}
	if len(comparison.DriftFindings) != 1 || comparison.DriftFindings[0].BaselineValue != "baseline runtime" {
		t.Fatalf("expected runtime finding against baseline attempt, got %+v", comparison.DriftFindings)
	}
}

func TestLiveValidationReplayIsExplicitlyBlockedUntilExecutorExists(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	manager := NewManager(Dependencies{
		EnvironmentScope: "test",
		Store:            store,
		FixturesDir:      filepath.Join("testdata", "fixtures"),
		Clock:            fixedClock,
	})
	if err := manager.LoadFixtures(ctx); err != nil {
		t.Fatalf("LoadFixtures returned error: %v", err)
	}
	candidates, err := manager.ListReplayCandidates(ctx, CandidateFilter{EnvironmentScope: "test"})
	if err != nil {
		t.Fatalf("ListReplayCandidates returned error: %v", err)
	}
	attempt, err := manager.CreateReplayAttempt(ctx, candidates[0].CandidateID, CreateReplayAttemptInput{Mode: ReplayModeLiveValidation})
	if err != nil {
		t.Fatalf("CreateReplayAttempt returned error: %v", err)
	}
	if attempt.Status != ReplayAttemptStatusBlocked {
		t.Fatalf("expected live validation to block before an executor exists, got %+v", attempt)
	}
	if attempt.SideEffectHandling != SideEffectBlocked || len(attempt.BlockedReasons) == 0 {
		t.Fatalf("expected blocked side-effect handling and reason, got %+v", attempt)
	}
}

func TestManagerReplaysAndComparesRequiredFixtureClasses(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	manager := NewManager(Dependencies{
		EnvironmentScope: "test",
		Store:            store,
		FixturesDir:      filepath.Join("testdata", "fixtures"),
		Clock:            fixedClock,
	})
	if err := manager.LoadFixtures(ctx); err != nil {
		t.Fatalf("LoadFixtures returned error: %v", err)
	}

	fixtures, err := manager.ListFixtures(ctx, FixtureFilter{EnvironmentScope: "test"})
	if err != nil {
		t.Fatalf("ListFixtures returned error: %v", err)
	}
	seen := map[FixtureDomainClass]bool{}
	for _, fixture := range fixtures {
		seen[fixture.DomainClass] = true
		attempt, err := manager.CreateReplayAttempt(ctx, fixture.CandidateID, CreateReplayAttemptInput{})
		if err != nil {
			t.Fatalf("CreateReplayAttempt(%s) returned error: %v", fixture.CandidateID, err)
		}
		if attempt.Status != ReplayAttemptStatusCompleted {
			t.Fatalf("expected completed replay for %s, got %+v", fixture.FixtureID, attempt)
		}
		comparison, err := manager.CreateComparison(ctx, attempt.AttemptID, CreateComparisonInput{})
		if err != nil {
			t.Fatalf("CreateComparison(%s) returned error: %v", attempt.AttemptID, err)
		}
		if comparison.TerminalStatus != ComparisonMatched {
			t.Fatalf("expected matched comparison for %s, got %+v", fixture.FixtureID, comparison)
		}
	}

	for _, domain := range []FixtureDomainClass{FixtureDomainSchedule, FixtureDomainIntegration, FixtureDomainComputerUse} {
		if !seen[domain] {
			t.Fatalf("expected fixture class %s", domain)
		}
	}
}

func fixedClock() time.Time {
	return time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
}
