package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/opsreadiness"
)

type releaseIndexFile struct {
	ReleaseIndexID            string                            `json:"releaseIndexId"`
	RunID                     string                            `json:"runId"`
	ProfileID                 string                            `json:"profileId"`
	CommitOrVersion           string                            `json:"commitOrVersion"`
	GeneratedAt               time.Time                         `json:"generatedAt"`
	ReviewTarget              string                            `json:"reviewTarget"`
	RetentionExpiresAt        time.Time                         `json:"retentionExpiresAt"`
	Decision                  string                            `json:"decision"`
	ReviewElapsedSeconds      int64                             `json:"reviewElapsedSeconds"`
	AuthorizedRetentionPolicy string                            `json:"authorizedRetentionPolicy,omitempty"`
	EvidenceLinks             []opsreadiness.HostedEvidenceLink `json:"evidenceLinks"`
}

func main() {
	allowNoShip := flag.Bool("allow-no-ship", false, "validate no_ship evidence indexes as generated artifacts instead of requiring ship readiness")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: hosted-evidence-validate [--allow-no-ship] <release-evidence-index.json>")
		os.Exit(2)
	}
	if err := validateReleaseIndexFile(flag.Arg(0), *allowNoShip); err != nil {
		fmt.Fprintf(os.Stderr, "hosted evidence validation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("hosted_evidence_validation=pass\nrelease_evidence_index=%s\n", flag.Arg(0))
}

func validateReleaseIndexFile(path string, allowNoShip bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var file releaseIndexFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}
	return validateReleaseIndex(file, allowNoShip)
}

func validateReleaseIndex(file releaseIndexFile, allowNoShip bool) error {
	var errs []error
	requireNonEmpty := func(label, value string) {
		if strings.TrimSpace(value) == "" {
			errs = append(errs, fmt.Errorf("%s is required", label))
		}
	}
	requireNonEmpty("releaseIndexId", file.ReleaseIndexID)
	requireNonEmpty("runId", file.RunID)
	requireNonEmpty("profileId", file.ProfileID)
	requireNonEmpty("commitOrVersion", file.CommitOrVersion)
	requireNonEmpty("reviewTarget", file.ReviewTarget)
	if file.GeneratedAt.IsZero() {
		errs = append(errs, errors.New("generatedAt is required"))
	}
	if file.RetentionExpiresAt.IsZero() {
		errs = append(errs, errors.New("retentionExpiresAt is required"))
	}
	if file.ReviewElapsedSeconds <= 0 {
		errs = append(errs, errors.New("reviewElapsedSeconds must be positive"))
	}
	reviewElapsed := time.Duration(file.ReviewElapsedSeconds) * time.Second
	if reviewElapsed > opsreadiness.MaxReleaseReviewElapsed {
		errs = append(errs, errors.New("reviewElapsedSeconds exceeds 30 minutes"))
	}
	switch file.Decision {
	case opsreadiness.ResultShip, opsreadiness.ResultNoShip, opsreadiness.ResultShipWithRecordedSkips:
	default:
		errs = append(errs, fmt.Errorf("decision %q is not allowed", file.Decision))
	}

	seen := map[string]bool{}
	failedLinks := 0
	for i, link := range file.EvidenceLinks {
		label := fmt.Sprintf("evidenceLinks[%d]", i)
		if strings.TrimSpace(link.EvidenceType) == "" {
			errs = append(errs, fmt.Errorf("%s.evidenceType is required", label))
		}
		if strings.TrimSpace(link.Path) == "" {
			errs = append(errs, fmt.Errorf("%s.path is required", label))
		}
		if link.RunID != file.RunID || link.ProfileID != file.ProfileID || link.CommitOrVersion != file.CommitOrVersion {
			errs = append(errs, fmt.Errorf("%s identity does not match release index", label))
		}
		if link.GeneratedAt.IsZero() {
			errs = append(errs, fmt.Errorf("%s.generatedAt is required", label))
		}
		if link.RetentionExpiresAt.IsZero() {
			errs = append(errs, fmt.Errorf("%s.retentionExpiresAt is required", label))
		}
		if link.RedactionStatus != "" && link.RedactionStatus != opsreadiness.HostedRedactionPassed {
			errs = append(errs, fmt.Errorf("%s redaction failed", label))
		}
		switch link.Status {
		case opsreadiness.StatusPass, opsreadiness.HostedResultPassed:
			if _, err := os.Stat(link.Path); err != nil {
				errs = append(errs, fmt.Errorf("%s passing path is not readable: %w", label, err))
			}
		case opsreadiness.StatusFail:
			failedLinks++
			if len(link.BlockingFindings) == 0 {
				errs = append(errs, fmt.Errorf("%s failed without blocking findings", label))
			}
		default:
			errs = append(errs, fmt.Errorf("%s status %q is not allowed", label, link.Status))
		}
		seen[link.EvidenceType] = true
	}
	for _, evidenceType := range opsreadiness.RequiredHostedEvidenceTypes {
		if !seen[evidenceType] {
			errs = append(errs, fmt.Errorf("missing required evidence link %s", evidenceType))
		}
	}
	if file.Decision == opsreadiness.ResultNoShip && failedLinks == 0 {
		errs = append(errs, errors.New("no_ship decision requires at least one failed evidence link"))
	}
	if file.Decision != opsreadiness.ResultNoShip && failedLinks > 0 {
		errs = append(errs, errors.New("ship decision cannot include failed evidence links"))
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	if file.Decision == opsreadiness.ResultNoShip && allowNoShip {
		return nil
	}
	index := opsreadiness.HostedReleaseEvidenceIndex{
		ReleaseIndexID:            file.ReleaseIndexID,
		RunID:                     file.RunID,
		ProfileID:                 file.ProfileID,
		CommitOrVersion:           file.CommitOrVersion,
		GeneratedAt:               file.GeneratedAt,
		ReviewTarget:              file.ReviewTarget,
		RetentionExpiresAt:        file.RetentionExpiresAt,
		Decision:                  file.Decision,
		ReviewElapsed:             reviewElapsed,
		AuthorizedRetentionPolicy: file.AuthorizedRetentionPolicy,
		EvidenceLinks:             file.EvidenceLinks,
	}
	return opsreadiness.ValidateHostedReleaseEvidenceIndex(index, time.Now().UTC())
}
