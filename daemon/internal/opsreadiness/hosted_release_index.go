package opsreadiness

import (
	"fmt"
	"time"
)

func ValidateHostedReleaseEvidenceIndex(index HostedReleaseEvidenceIndex, now time.Time) error {
	errs := []error{
		RequireNonEmpty("release index id", index.ReleaseIndexID),
		RequireNonEmpty("run id", index.RunID),
		RequireNonEmpty("profile id", index.ProfileID),
		RequireNonEmpty("commit or version", index.CommitOrVersion),
		RequireNonEmpty("review target", index.ReviewTarget),
		RequireElapsedAtMost("release evidence review", index.ReviewElapsed, MaxReleaseReviewElapsed),
		requireAllowed("release decision", index.Decision, []string{ResultShip, ResultNoShip, ResultShipWithRecordedSkips}),
		ValidateHostedRetention("release index", index.RetentionExpiresAt, index.AuthorizedRetentionPolicy),
		ValidateHostedRedaction("release index", index),
	}
	if index.ReviewElapsed > MaxReleaseReviewElapsed {
		errs = append(errs, fmt.Errorf("release evidence review must complete in 30 minutes or less"))
	}
	if index.GeneratedAt.IsZero() {
		errs = append(errs, fmt.Errorf("release index generated at is required"))
	}
	if !now.IsZero() && index.RetentionExpiresAt.Before(now) && index.AuthorizedRetentionPolicy == "" {
		errs = append(errs, fmt.Errorf("release index evidence expired"))
	}
	seen := map[string]bool{}
	for i, link := range index.EvidenceLinks {
		errs = append(errs, validateHostedEvidenceLink(index, i, link))
		seen[link.EvidenceType] = true
	}
	for _, evidenceType := range RequiredHostedEvidenceTypes {
		if !seen[evidenceType] {
			errs = append(errs, fmt.Errorf("missing required evidence link %s", evidenceType))
		}
	}
	for _, link := range index.EvidenceLinks {
		if link.Status != StatusPass && link.Status != HostedResultPassed {
			errs = append(errs, fmt.Errorf("release evidence %s failed", link.EvidenceType))
		}
	}
	return JoinErrors(errs...)
}

func validateHostedEvidenceLink(index HostedReleaseEvidenceIndex, i int, link HostedEvidenceLink) error {
	label := fmt.Sprintf("evidence link[%d]", i)
	errs := []error{
		RequireNonEmpty(label+".evidence type", link.EvidenceType),
		RequireNonEmpty(label+".path", link.Path),
		RequireNonEmpty(label+".status", link.Status),
		ValidateHostedRetention(label, link.RetentionExpiresAt, index.AuthorizedRetentionPolicy),
	}
	if link.RunID != index.RunID || link.ProfileID != index.ProfileID || link.CommitOrVersion != index.CommitOrVersion {
		errs = append(errs, fmt.Errorf("%s identity does not match release index", label))
	}
	if link.GeneratedAt.IsZero() {
		errs = append(errs, fmt.Errorf("%s generated at is required", label))
	}
	if link.RedactionStatus != "" && link.RedactionStatus != HostedRedactionPassed {
		errs = append(errs, fmt.Errorf("%s redaction failed", label))
	}
	if len(link.BlockingFindings) > 0 {
		errs = append(errs, fmt.Errorf("%s has blocking findings", label))
	}
	return JoinErrors(errs...)
}
