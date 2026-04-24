package evaluation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type CapturedEvidence struct {
	TerminalStatus     ReplayAttemptStatus `json:"terminalStatus"`
	RuntimeSummary     string              `json:"runtime"`
	PolicySummary      string              `json:"policy"`
	IntegrationSummary string              `json:"integration"`
	DeliverySummary    string              `json:"delivery"`
	EvidenceSummary    string              `json:"evidence"`
	ResultRunID        string              `json:"resultRunId"`
	ResultWorkflowID   string              `json:"resultWorkflowId"`
	BlockedReasons     []string            `json:"blockedReasons"`
	Limitations        []string            `json:"limitations"`
}

func LoadRegressionFixtures(rootDir string, environmentScope string) ([]RegressionFixture, error) {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, fmt.Errorf("read fixtures dir: %w", err)
	}
	fixtures := make([]RegressionFixture, 0, len(entries))
	now := time.Now().UTC()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(rootDir, entry.Name(), "manifest.json")
		raw, err := os.ReadFile(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("read fixture manifest %s: %w", manifestPath, err)
		}
		var fixture RegressionFixture
		if err := json.Unmarshal(raw, &fixture); err != nil {
			return nil, fmt.Errorf("decode fixture manifest %s: %w", manifestPath, err)
		}
		fixture.ManifestPath = manifestPath
		fixture.EnvironmentScope = firstNonEmpty(fixture.EnvironmentScope, environmentScope)
		fixture.ExpectedReplayMode = replayModeDefault(fixture.ExpectedReplayMode)
		fixture.CreatedAt = zeroTimeDefault(fixture.CreatedAt, now)
		fixture.UpdatedAt = zeroTimeDefault(fixture.UpdatedAt, now)
		fixture.CandidateID = candidateIDForFixture(fixture.FixtureID)
		if err := validateFixture(fixture); err != nil {
			return nil, fmt.Errorf("validate fixture %s: %w", manifestPath, err)
		}
		fixtures = append(fixtures, fixture)
	}
	sort.SliceStable(fixtures, func(i, j int) bool {
		if fixtures[i].DomainClass == fixtures[j].DomainClass {
			return fixtures[i].FixtureID < fixtures[j].FixtureID
		}
		return fixtures[i].DomainClass < fixtures[j].DomainClass
	})
	return fixtures, nil
}

func LoadCapturedEvidence(fixture RegressionFixture) (CapturedEvidence, error) {
	if len(fixture.CapturedEvidenceRefs) == 0 {
		return CapturedEvidence{}, fmt.Errorf("fixture %s has no captured evidence refs", fixture.FixtureID)
	}
	ref := fixture.CapturedEvidenceRefs[0]
	evidencePath := ref.Route
	if evidencePath == "" {
		evidencePath = ref.ID
	}
	if !filepath.IsAbs(evidencePath) {
		manifestDir := filepath.Dir(fixture.ManifestPath)
		if _, err := os.Stat(filepath.Join(manifestDir, evidencePath)); err == nil {
			evidencePath = filepath.Join(manifestDir, evidencePath)
		} else if ref.ID != "" {
			idPath := filepath.Join(manifestDir, ref.ID)
			if _, err := os.Stat(idPath); err == nil {
				evidencePath = idPath
			} else if basePath := filepath.Join(manifestDir, filepath.Base(ref.ID)); filepath.Base(ref.ID) != "." {
				if _, err := os.Stat(basePath); err == nil {
					evidencePath = basePath
				}
			}
		}
	}
	raw, err := os.ReadFile(evidencePath)
	if err != nil {
		return CapturedEvidence{}, fmt.Errorf("read captured evidence %s: %w", evidencePath, err)
	}
	var evidence CapturedEvidence
	if err := json.Unmarshal(raw, &evidence); err != nil {
		return CapturedEvidence{}, fmt.Errorf("decode captured evidence %s: %w", evidencePath, err)
	}
	if evidence.TerminalStatus == "" {
		evidence.TerminalStatus = ReplayAttemptStatusCompleted
	}
	return evidence, nil
}

func CandidateFromFixture(fixture RegressionFixture, now time.Time) ReplayCandidate {
	readiness := ReadinessFullyReplayable
	reasons := []string{"fixture has captured evidence and expected comparison summaries"}
	if len(fixture.Limitations) > 0 {
		readiness = ReadinessPartiallyReplayable
		reasons = append(reasons, fixture.Limitations...)
	}
	return ReplayCandidate{
		CandidateID:          candidateIDForFixture(fixture.FixtureID),
		CandidateKind:        CandidateKindFixture,
		DisplayName:          fixture.DisplayName,
		SourceKind:           SourceKindFixture,
		SourceID:             fixture.FixtureID,
		SourceRefs:           append([]SourceRef(nil), fixture.SourceRefs...),
		EnvironmentScope:     fixture.EnvironmentScope,
		ReadinessStatus:      readiness,
		ReadinessReasons:     reasons,
		Limitations:          append([]string(nil), fixture.Limitations...),
		DefaultReplayMode:    ReplayModeNonLive,
		FixtureID:            fixture.FixtureID,
		ExpectedComparison:   fixture.ExpectedComparisonSummary,
		CapturedEvidenceRefs: append([]SourceRef(nil), fixture.CapturedEvidenceRefs...),
		CreatedAt:            zeroTimeDefault(fixture.CreatedAt, now),
		UpdatedAt:            zeroTimeDefault(fixture.UpdatedAt, now),
	}
}

func validateFixture(fixture RegressionFixture) error {
	if fixture.FixtureID == "" {
		return fmt.Errorf("fixtureId is required")
	}
	if fixture.DisplayName == "" {
		return fmt.Errorf("displayName is required")
	}
	switch fixture.DomainClass {
	case FixtureDomainSchedule, FixtureDomainIntegration, FixtureDomainComputerUse:
	default:
		return fmt.Errorf("unsupported domainClass %q", fixture.DomainClass)
	}
	if len(fixture.SourceRefs) == 0 {
		return fmt.Errorf("sourceRefs are required")
	}
	if len(fixture.CapturedEvidenceRefs) == 0 {
		return fmt.Errorf("capturedEvidenceRefs are required")
	}
	if fixture.ExpectedComparisonSummary.Runtime == "" || fixture.ExpectedComparisonSummary.Evidence == "" {
		return fmt.Errorf("expectedComparisonSummary runtime and evidence are required")
	}
	return nil
}

func candidateIDForFixture(fixtureID string) string {
	return "candidate_" + fixtureID
}
