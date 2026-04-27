package evaluation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Store interface {
	UpsertReplayCandidate(context.Context, ReplayCandidate) error
	ListReplayCandidates(context.Context, CandidateFilter) ([]ReplayCandidate, error)
	GetReplayCandidate(context.Context, string, string) (ReplayCandidate, bool, error)
	UpsertReplayAttempt(context.Context, ReplayAttempt) error
	ListReplayAttempts(context.Context, AttemptFilter) ([]ReplayAttempt, error)
	GetReplayAttempt(context.Context, string, string) (ReplayAttempt, bool, error)
	UpsertComparisonResult(context.Context, ComparisonResult) error
	ListComparisonResults(context.Context, ComparisonFilter) ([]ComparisonResult, error)
	GetComparisonResult(context.Context, string, string) (ComparisonResult, bool, error)
	UpsertRegressionFixture(context.Context, RegressionFixture) error
	ListRegressionFixtures(context.Context, FixtureFilter) ([]RegressionFixture, error)
}

type Dependencies struct {
	EnvironmentScope string
	Store            Store
	FixturesDir      string
	RuntimeRecorder  RuntimeRecorder
	Clock            func() time.Time
}

type Manager struct {
	environmentScope string
	store            Store
	fixturesDir      string
	runtimeRecorder  RuntimeRecorder
	clock            func() time.Time
}

func NewManager(deps Dependencies) *Manager {
	clock := deps.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	return &Manager{
		environmentScope: firstNonEmpty(deps.EnvironmentScope, "test"),
		store:            deps.Store,
		fixturesDir:      deps.FixturesDir,
		runtimeRecorder:  deps.RuntimeRecorder,
		clock:            clock,
	}
}

func (m *Manager) LoadFixtures(ctx context.Context) error {
	if m.fixturesDir == "" {
		return nil
	}
	fixtures, err := LoadRegressionFixtures(m.fixturesDir, m.environmentScope)
	if err != nil {
		return err
	}
	for _, fixture := range fixtures {
		now := m.clock()
		fixture.CreatedAt = zeroTimeDefault(fixture.CreatedAt, now)
		fixture.UpdatedAt = zeroTimeDefault(fixture.UpdatedAt, now)
		fixture.CandidateID = candidateIDForFixture(fixture.FixtureID)
		if err := m.store.UpsertRegressionFixture(ctx, fixture); err != nil {
			return err
		}
		candidate := CandidateFromFixture(fixture, now)
		if err := m.store.UpsertReplayCandidate(ctx, candidate); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) UpsertReplayCandidate(ctx context.Context, candidate ReplayCandidate) error {
	now := m.clock()
	candidate.EnvironmentScope = firstNonEmpty(candidate.EnvironmentScope, m.environmentScope)
	candidate.DefaultReplayMode = replayModeDefault(candidate.DefaultReplayMode)
	candidate.CreatedAt = zeroTimeDefault(candidate.CreatedAt, now)
	candidate.UpdatedAt = zeroTimeDefault(candidate.UpdatedAt, now)
	if candidate.ReadinessStatus == "" {
		candidate.ReadinessStatus = ReadinessFullyReplayable
	}
	if candidate.CandidateID == "" {
		candidate.CandidateID = newID("replay_candidate")
	}
	if candidate.CandidateKind == "" {
		candidate.CandidateKind = CandidateKindCuratedWork
	}
	if err := validateReplayCandidate(candidate); err != nil {
		return err
	}
	candidate = normalizeReplayCandidate(candidate)
	return m.store.UpsertReplayCandidate(ctx, candidate)
}

func (m *Manager) ListReplayCandidates(ctx context.Context, filter CandidateFilter) ([]ReplayCandidate, error) {
	filter.EnvironmentScope = firstNonEmpty(filter.EnvironmentScope, m.environmentScope)
	items, err := m.store.ListReplayCandidates(ctx, filter)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CandidateID < items[j].CandidateID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return normalizeReplayCandidates(limitCandidates(items, filter.Limit)), nil
}

func (m *Manager) GetReplayCandidate(ctx context.Context, candidateID string) (ReplayCandidate, bool, error) {
	item, ok, err := m.store.GetReplayCandidate(ctx, m.environmentScope, candidateID)
	if err != nil || !ok {
		return item, ok, err
	}
	return normalizeReplayCandidate(item), true, nil
}

func (m *Manager) CreateReplayAttempt(ctx context.Context, candidateID string, input CreateReplayAttemptInput) (ReplayAttempt, error) {
	candidate, ok, err := m.store.GetReplayCandidate(ctx, m.environmentScope, candidateID)
	if err != nil {
		return ReplayAttempt{}, err
	}
	if !ok {
		return ReplayAttempt{}, fmt.Errorf("replay candidate %s not found", candidateID)
	}
	candidate = normalizeReplayCandidate(candidate)
	now := m.clock()
	mode := replayModeDefault(input.Mode)
	evidence, evidenceErr := m.capturedEvidenceForCandidate(ctx, candidate)
	attempt := ReplayAttempt{
		AttemptID:          newID("replay_attempt"),
		CandidateID:        candidate.CandidateID,
		SourceRefs:         append([]SourceRef(nil), candidate.SourceRefs...),
		EnvironmentScope:   candidate.EnvironmentScope,
		Mode:               mode,
		Status:             ReplayAttemptStatusCompleted,
		SafetyScope:        input.SafetyScope,
		ApprovalHandling:   ApprovalEvidenceOnly,
		SideEffectHandling: SideEffectEvidenceOnly,
		LaunchedBy:         input.LaunchedBy,
		ChangeWindowLabel:  input.ChangeWindowLabel,
		BaselineAttemptID:  input.BaselineAttemptID,
		EvidenceRefs:       append([]SourceRef(nil), candidate.CapturedEvidenceRefs...),
		RuntimeSummary:     firstNonEmpty(evidence.RuntimeSummary, candidate.ExpectedComparison.Runtime, "replay evidence completed"),
		PolicySummary:      firstNonEmpty(evidence.PolicySummary, candidate.ExpectedComparison.Policy, "policy evidence preserved"),
		IntegrationSummary: firstNonEmpty(evidence.IntegrationSummary, candidate.ExpectedComparison.Integration, "integration evidence preserved"),
		DeliverySummary:    firstNonEmpty(evidence.DeliverySummary, candidate.ExpectedComparison.Delivery, "delivery evidence preserved"),
		EvidenceSummary:    firstNonEmpty(evidence.EvidenceSummary, candidate.ExpectedComparison.Evidence, "replay evidence captured"),
		ResultRunID:        evidence.ResultRunID,
		ResultWorkflowID:   evidence.ResultWorkflowID,
		StartedAt:          now,
		CompletedAt:        now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if evidence.TerminalStatus != "" {
		attempt.Status = evidence.TerminalStatus
	}
	if evidenceErr != nil && candidate.CandidateKind == CandidateKindFixture {
		attempt.Status = ReplayAttemptStatusUnreplayable
		attempt.ApprovalHandling = ApprovalBlocked
		attempt.SideEffectHandling = SideEffectBlocked
		attempt.BlockedReasons = []string{evidenceErr.Error()}
	}
	attempt.BlockedReasons = append(attempt.BlockedReasons, evidence.BlockedReasons...)
	attempt.BlockedReasons = append(attempt.BlockedReasons, evidence.Limitations...)
	if mode == ReplayModeLiveValidation {
		attempt.Status = ReplayAttemptStatusBlocked
		attempt.ApprovalHandling = ApprovalBlocked
		attempt.SideEffectHandling = SideEffectBlocked
		attempt.BlockedReasons = append(attempt.BlockedReasons, "live validation requires an explicit live side-effect executor and approval flow before side effects can run")
	}
	if candidate.ReadinessStatus == ReadinessBlocked {
		attempt.Status = ReplayAttemptStatusBlocked
		attempt.ApprovalHandling = ApprovalBlocked
		attempt.SideEffectHandling = SideEffectBlocked
		attempt.BlockedReasons = appendReasons(candidate.ReadinessReasons, candidate.Limitations)
	}
	if candidate.ReadinessStatus == ReadinessUnreplayable {
		attempt.Status = ReplayAttemptStatusUnreplayable
		attempt.ApprovalHandling = ApprovalBlocked
		attempt.SideEffectHandling = SideEffectBlocked
		attempt.BlockedReasons = appendReasons(candidate.ReadinessReasons, candidate.Limitations)
	}
	if attempt.SafetyScope.Mode == "" {
		attempt.SafetyScope.Mode = mode
	}
	if attempt.Status == ReplayAttemptStatusCompleted && attempt.Mode == ReplayModeNonLive && m.runtimeRecorder != nil {
		record, err := m.runtimeRecorder.RecordReplay(ctx, ReplayRecordInput{
			Candidate: candidate,
			Attempt:   attempt,
			Evidence:  evidence,
			Now:       now,
		})
		if err != nil {
			attempt.Status = ReplayAttemptStatusFailed
			attempt.CompletedAt = m.clock()
			attempt.UpdatedAt = attempt.CompletedAt
			attempt.BlockedReasons = append(attempt.BlockedReasons, fmt.Sprintf("record replay runtime run: %v", err))
		} else {
			attempt.ResultRunID = firstNonEmpty(record.RunID, attempt.ResultRunID)
			attempt.ResultWorkflowID = firstNonEmpty(record.WorkflowID, attempt.ResultWorkflowID)
			attempt.EvidenceRefs = append(attempt.EvidenceRefs, record.EvidenceRefs...)
		}
	}
	attempt = normalizeReplayAttempt(attempt)
	if err := m.store.UpsertReplayAttempt(ctx, attempt); err != nil {
		return ReplayAttempt{}, err
	}
	candidate.LatestAttemptID = attempt.AttemptID
	candidate.UpdatedAt = now
	if err := m.store.UpsertReplayCandidate(ctx, candidate); err != nil {
		return ReplayAttempt{}, err
	}
	return attempt, nil
}

func (m *Manager) ListReplayAttempts(ctx context.Context, filter AttemptFilter) ([]ReplayAttempt, error) {
	filter.EnvironmentScope = firstNonEmpty(filter.EnvironmentScope, m.environmentScope)
	items, err := m.store.ListReplayAttempts(ctx, filter)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].AttemptID < items[j].AttemptID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return normalizeReplayAttempts(limitAttempts(items, filter.Limit)), nil
}

func (m *Manager) GetReplayAttempt(ctx context.Context, attemptID string) (ReplayAttempt, bool, error) {
	item, ok, err := m.store.GetReplayAttempt(ctx, m.environmentScope, attemptID)
	if err != nil || !ok {
		return item, ok, err
	}
	return normalizeReplayAttempt(item), true, nil
}

func (m *Manager) CreateComparison(ctx context.Context, attemptID string, input CreateComparisonInput) (ComparisonResult, error) {
	attempt, ok, err := m.store.GetReplayAttempt(ctx, m.environmentScope, attemptID)
	if err != nil {
		return ComparisonResult{}, err
	}
	if !ok {
		return ComparisonResult{}, fmt.Errorf("replay attempt %s not found", attemptID)
	}
	attempt = normalizeReplayAttempt(attempt)
	candidate, ok, err := m.store.GetReplayCandidate(ctx, m.environmentScope, attempt.CandidateID)
	if err != nil {
		return ComparisonResult{}, err
	}
	if !ok {
		return ComparisonResult{}, fmt.Errorf("replay candidate %s not found", attempt.CandidateID)
	}
	candidate = normalizeReplayCandidate(candidate)
	var baseline *ReplayAttempt
	if input.BaselineAttemptID != "" {
		item, ok, err := m.store.GetReplayAttempt(ctx, m.environmentScope, input.BaselineAttemptID)
		if err != nil {
			return ComparisonResult{}, err
		}
		if !ok {
			return ComparisonResult{}, fmt.Errorf("baseline replay attempt %s not found", input.BaselineAttemptID)
		}
		item = normalizeReplayAttempt(item)
		baseline = &item
	}
	comparison := normalizeComparison(CompareAttempt(candidate, baseline, attempt, input, m.clock()))
	if err := m.store.UpsertComparisonResult(ctx, comparison); err != nil {
		return ComparisonResult{}, err
	}
	candidate.LatestComparisonID = comparison.ComparisonID
	candidate.UpdatedAt = comparison.GeneratedAt
	if err := m.store.UpsertReplayCandidate(ctx, candidate); err != nil {
		return ComparisonResult{}, err
	}
	return comparison, nil
}

func (m *Manager) capturedEvidenceForCandidate(ctx context.Context, candidate ReplayCandidate) (CapturedEvidence, error) {
	if candidate.FixtureID == "" {
		return CapturedEvidence{}, nil
	}
	fixtures, err := m.store.ListRegressionFixtures(ctx, FixtureFilter{EnvironmentScope: candidate.EnvironmentScope})
	if err != nil {
		return CapturedEvidence{}, err
	}
	for _, fixture := range fixtures {
		if fixture.FixtureID == candidate.FixtureID {
			return LoadCapturedEvidence(fixture)
		}
	}
	return CapturedEvidence{}, fmt.Errorf("fixture %s not found for replay candidate %s", candidate.FixtureID, candidate.CandidateID)
}

func (m *Manager) ListComparisons(ctx context.Context, filter ComparisonFilter) ([]ComparisonResult, error) {
	filter.EnvironmentScope = firstNonEmpty(filter.EnvironmentScope, m.environmentScope)
	items, err := m.store.ListComparisonResults(ctx, filter)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].GeneratedAt.Equal(items[j].GeneratedAt) {
			return items[i].ComparisonID < items[j].ComparisonID
		}
		return items[i].GeneratedAt.Before(items[j].GeneratedAt)
	})
	return normalizeComparisons(limitComparisons(items, filter.Limit)), nil
}

func (m *Manager) GetComparison(ctx context.Context, comparisonID string) (ComparisonResult, bool, error) {
	item, ok, err := m.store.GetComparisonResult(ctx, m.environmentScope, comparisonID)
	if err != nil || !ok {
		return item, ok, err
	}
	return normalizeComparison(item), true, nil
}

func (m *Manager) ListFixtures(ctx context.Context, filter FixtureFilter) ([]RegressionFixture, error) {
	filter.EnvironmentScope = firstNonEmpty(filter.EnvironmentScope, m.environmentScope)
	items, err := m.store.ListRegressionFixtures(ctx, filter)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].DomainClass == items[j].DomainClass {
			return items[i].FixtureID < items[j].FixtureID
		}
		return items[i].DomainClass < items[j].DomainClass
	})
	return normalizeFixtures(limitFixtures(items, filter.Limit)), nil
}

func normalizeReplayCandidates(items []ReplayCandidate) []ReplayCandidate {
	for idx := range items {
		items[idx] = normalizeReplayCandidate(items[idx])
	}
	return items
}

func normalizeReplayCandidate(item ReplayCandidate) ReplayCandidate {
	if item.SourceRefs == nil {
		item.SourceRefs = []SourceRef{}
	}
	if item.ReadinessReasons == nil {
		item.ReadinessReasons = []string{}
	}
	if item.Limitations == nil {
		item.Limitations = []string{}
	}
	if item.CapturedEvidenceRefs == nil {
		item.CapturedEvidenceRefs = []SourceRef{}
	}
	return item
}

func normalizeReplayAttempts(items []ReplayAttempt) []ReplayAttempt {
	for idx := range items {
		items[idx] = normalizeReplayAttempt(items[idx])
	}
	return items
}

func normalizeReplayAttempt(item ReplayAttempt) ReplayAttempt {
	if item.SourceRefs == nil {
		item.SourceRefs = []SourceRef{}
	}
	if item.EvidenceRefs == nil {
		item.EvidenceRefs = []SourceRef{}
	}
	if item.BlockedReasons == nil {
		item.BlockedReasons = []string{}
	}
	return item
}

func normalizeComparisons(items []ComparisonResult) []ComparisonResult {
	for idx := range items {
		items[idx] = normalizeComparison(items[idx])
	}
	return items
}

func normalizeComparison(item ComparisonResult) ComparisonResult {
	if item.Limitations == nil {
		item.Limitations = []string{}
	}
	if item.DriftFindings == nil {
		item.DriftFindings = []DriftFinding{}
	}
	for idx := range item.DriftFindings {
		if item.DriftFindings[idx].EvidenceRefs == nil {
			item.DriftFindings[idx].EvidenceRefs = []SourceRef{}
		}
	}
	return item
}

func normalizeFixtures(items []RegressionFixture) []RegressionFixture {
	for idx := range items {
		if items[idx].SourceRefs == nil {
			items[idx].SourceRefs = []SourceRef{}
		}
		if items[idx].CapturedEvidenceRefs == nil {
			items[idx].CapturedEvidenceRefs = []SourceRef{}
		}
		if items[idx].Assumptions == nil {
			items[idx].Assumptions = []string{}
		}
		if items[idx].Limitations == nil {
			items[idx].Limitations = []string{}
		}
	}
	return items
}

func validateReplayCandidate(candidate ReplayCandidate) error {
	if strings.TrimSpace(candidate.CandidateID) == "" {
		return errors.New("candidateId is required")
	}
	if candidate.CandidateKind != CandidateKindCuratedWork && candidate.CandidateKind != CandidateKindFixture {
		return fmt.Errorf("unsupported candidateKind %q", candidate.CandidateKind)
	}
	if strings.TrimSpace(candidate.DisplayName) == "" {
		return errors.New("displayName is required")
	}
	if candidate.SourceKind == "" {
		return errors.New("sourceKind is required")
	}
	if strings.TrimSpace(candidate.SourceID) == "" {
		return errors.New("sourceId is required")
	}
	if len(candidate.SourceRefs) == 0 {
		return errors.New("sourceRefs is required")
	}
	for idx, ref := range candidate.SourceRefs {
		if ref.Kind == "" || strings.TrimSpace(ref.ID) == "" {
			return fmt.Errorf("sourceRefs[%d] requires kind and id", idx)
		}
	}
	if candidate.ReadinessStatus == ReadinessBlocked || candidate.ReadinessStatus == ReadinessUnreplayable {
		if len(candidate.ReadinessReasons) == 0 && len(candidate.Limitations) == 0 {
			return errors.New("blocked or unreplayable candidates require readinessReasons or limitations")
		}
	}
	if candidate.DefaultReplayMode != ReplayModeNonLive {
		return fmt.Errorf("defaultReplayMode must be %q", ReplayModeNonLive)
	}
	return nil
}

func replayModeDefault(mode ReplayMode) ReplayMode {
	if mode == "" {
		return ReplayModeNonLive
	}
	return mode
}

func zeroTimeDefault(value time.Time, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value
}

func appendReasons(primary, secondary []string) []string {
	items := append([]string(nil), primary...)
	items = append(items, secondary...)
	if len(items) == 0 {
		return []string{"replay cannot proceed safely with available evidence"}
	}
	return items
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_fallback"
	}
	return prefix + "_" + hex.EncodeToString(buf)
}

func ensureStore(store Store) error {
	if store == nil {
		return errors.New("evaluation store is not configured")
	}
	return nil
}

func limitCandidates(items []ReplayCandidate, limit int) []ReplayCandidate {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func limitAttempts(items []ReplayAttempt, limit int) []ReplayAttempt {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func limitComparisons(items []ComparisonResult, limit int) []ComparisonResult {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func limitFixtures(items []RegressionFixture, limit int) []RegressionFixture {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}
