package evaluation

import "time"

func CompareAttempt(candidate ReplayCandidate, baseline *ReplayAttempt, attempt ReplayAttempt, input CreateComparisonInput, now time.Time) ComparisonResult {
	expected := candidate.ExpectedComparison
	baselineRef := firstNonEmpty(input.BaselineRef, input.BaselineAttemptID, attempt.BaselineAttemptID, candidate.SourceID)
	if baseline != nil {
		baselineRef = baseline.AttemptID
		expected = PlaneSummaries{
			Runtime:     baseline.RuntimeSummary,
			Policy:      baseline.PolicySummary,
			Integration: baseline.IntegrationSummary,
			Delivery:    baseline.DeliverySummary,
			Evidence:    baseline.EvidenceSummary,
		}
	}
	result := ComparisonResult{
		ComparisonID:       newID("replay_comparison"),
		CandidateID:        candidate.CandidateID,
		BaselineRef:        baselineRef,
		AttemptID:          attempt.AttemptID,
		EnvironmentScope:   attempt.EnvironmentScope,
		TerminalStatus:     ComparisonMatched,
		RuntimeSummary:     attempt.RuntimeSummary,
		PolicySummary:      attempt.PolicySummary,
		IntegrationSummary: attempt.IntegrationSummary,
		DeliverySummary:    attempt.DeliverySummary,
		EvidenceSummary:    attempt.EvidenceSummary,
		Confidence:         "high",
		Limitations:        append([]string(nil), candidate.Limitations...),
		ChangeWindowLabel:  input.ChangeWindowLabel,
		GeneratedAt:        now,
	}
	if attempt.Status == ReplayAttemptStatusBlocked {
		result.TerminalStatus = ComparisonBlocked
		result.Confidence = "medium"
		result.Limitations = append(result.Limitations, attempt.BlockedReasons...)
		return result
	}
	if attempt.Status == ReplayAttemptStatusUnreplayable {
		result.TerminalStatus = ComparisonUnreplayable
		result.Confidence = "low"
		result.Limitations = append(result.Limitations, attempt.BlockedReasons...)
		return result
	}

	result.DriftFindings = append(result.DriftFindings, comparePlane(result.ComparisonID, DriftPlaneRuntime, expected.Runtime, attempt.RuntimeSummary, attempt.EvidenceRefs, now)...)
	result.DriftFindings = append(result.DriftFindings, comparePlane(result.ComparisonID, DriftPlanePolicy, expected.Policy, attempt.PolicySummary, attempt.EvidenceRefs, now)...)
	result.DriftFindings = append(result.DriftFindings, comparePlane(result.ComparisonID, DriftPlaneIntegration, expected.Integration, attempt.IntegrationSummary, attempt.EvidenceRefs, now)...)
	result.DriftFindings = append(result.DriftFindings, comparePlane(result.ComparisonID, DriftPlaneDelivery, expected.Delivery, attempt.DeliverySummary, attempt.EvidenceRefs, now)...)
	result.DriftFindings = append(result.DriftFindings, comparePlane(result.ComparisonID, DriftPlaneEvidence, expected.Evidence, attempt.EvidenceSummary, attempt.EvidenceRefs, now)...)
	if len(result.DriftFindings) > 0 {
		result.TerminalStatus = ComparisonDrifted
		result.Confidence = "medium"
	}
	return result
}

func comparePlane(comparisonID string, plane DriftPlane, baseline string, replay string, refs []SourceRef, now time.Time) []DriftFinding {
	if baseline == "" || replay == "" || baseline == replay {
		return nil
	}
	return []DriftFinding{{
		FindingID:         newID("drift_finding"),
		ComparisonID:      comparisonID,
		Plane:             plane,
		Severity:          "warning",
		Summary:           string(plane) + " summary changed",
		BaselineValue:     baseline,
		ReplayValue:       replay,
		EvidenceRefs:      append([]SourceRef(nil), refs...),
		RecommendedAction: "Inspect authoritative replay evidence before treating this drift as expected.",
		CreatedAt:         now,
	}}
}
