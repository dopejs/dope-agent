package opsreadiness

import (
	"testing"
	"time"
)

func sampleHostedEvidenceLinks() []HostedEvidenceLink {
	run := sampleHostedRun()
	links := make([]HostedEvidenceLink, 0, len(RequiredHostedEvidenceTypes))
	for _, evidenceType := range RequiredHostedEvidenceTypes {
		links = append(links, HostedEvidenceLink{
			EvidenceType:       evidenceType,
			Path:               "~/.dope-test/artifacts/hosted_run_20260430/" + evidenceType + ".json",
			RunID:              run.RunID,
			ProfileID:          run.ProfileID,
			CommitOrVersion:    run.CommitOrVersion,
			Status:             StatusPass,
			GeneratedAt:        hostedNow,
			RetentionExpiresAt: run.RetentionExpiresAt,
			RedactionStatus:    HostedRedactionPassed,
		})
	}
	return links
}

func sampleReleaseIndex() HostedReleaseEvidenceIndex {
	run := sampleHostedRun()
	return HostedReleaseEvidenceIndex{
		ReleaseIndexID:     "release_index_1",
		RunID:              run.RunID,
		ProfileID:          run.ProfileID,
		CommitOrVersion:    run.CommitOrVersion,
		GeneratedAt:        hostedNow,
		ReviewTarget:       "Roadmap 43 release",
		RetentionExpiresAt: run.RetentionExpiresAt,
		Decision:           ResultShip,
		ReviewElapsed:      25 * time.Minute,
		EvidenceLinks:      sampleHostedEvidenceLinks(),
	}
}

func TestHostedReleaseIndexValidatesLinksIdentityAndReviewTime(t *testing.T) {
	index := sampleReleaseIndex()
	assertValid(t, ValidateHostedReleaseEvidenceIndex(index, hostedNow))

	index.EvidenceLinks = index.EvidenceLinks[:len(index.EvidenceLinks)-1]
	index.Decision = ResultNoShip
	assertInvalidContains(t, ValidateHostedReleaseEvidenceIndex(index, hostedNow), "missing")

	index = sampleReleaseIndex()
	index.EvidenceLinks[0].RunID = "stale_run"
	index.Decision = ResultNoShip
	assertInvalidContains(t, ValidateHostedReleaseEvidenceIndex(index, hostedNow), "identity")

	index = sampleReleaseIndex()
	index.ReviewElapsed = 31 * time.Minute
	index.Decision = ResultNoShip
	assertInvalidContains(t, ValidateHostedReleaseEvidenceIndex(index, hostedNow), "30 minutes")
}
