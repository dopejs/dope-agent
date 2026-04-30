package opsreadiness

import "testing"

func sampleHostedBackupEvidence() HostedBackupEvidence {
	run := sampleHostedRun()
	return HostedBackupEvidence{
		BackupID:              "backup_hosted_1",
		RunID:                 run.RunID,
		SourceProfileID:       run.ProfileID,
		SourceCommitOrVersion: run.CommitOrVersion,
		ArtifactPath:          "~/.dope-test/backups/daemon.sqlite.bak",
		Checksum:              "sha256:abc123",
		TenantSummary:         sampleTenantState(),
		IncludedMaterial:      []string{"sqlite state", "secret references"},
		ExcludedMaterial:      []string{"raw secret", "access token", "refresh token", "oauth code", "provider token", "local CLI auth material", "derived credential material"},
		CompatibilityNotes:    []string{"compatible with hosted test profile"},
		RedactionStatus:       HostedRedactionPassed,
		GeneratedAt:           hostedNow,
	}
}

func TestHostedBackupEvidenceValidatesChecksumTenantCoverageAndRedaction(t *testing.T) {
	backup := sampleHostedBackupEvidence()
	assertValid(t, ValidateHostedBackupEvidence(sampleHostedRun(), backup))

	backup.Checksum = ""
	assertInvalidContains(t, ValidateHostedBackupEvidence(sampleHostedRun(), backup), "checksum")

	backup = sampleHostedBackupEvidence()
	backup.IncludedMaterial = append(backup.IncludedMaterial, "raw_secret")
	assertInvalidContains(t, ValidateHostedBackupEvidence(sampleHostedRun(), backup), "credential")
}
