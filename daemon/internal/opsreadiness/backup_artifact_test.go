package opsreadiness

import (
	"testing"
	"time"
)

func TestBackupArtifactValidationRequiresThreeTenantsAndCredentialExclusion(t *testing.T) {
	artifact := BackupArtifact{
		ArtifactID: "backup_r39", CreatedAt: time.Now(), SourceVersion: "v39",
		SourceEnvironment: EnvironmentTest,
		IncludedMaterial:  []string{"tenant records", "secret references", "quota state", "work state"},
		ExcludedMaterial:  []string{"raw secret values", "access tokens", "refresh tokens", "oauth authorization codes", "provider tokens", "derived credential material"},
		TenantCount:       3, TenantStateSummary: sampleTenantState(),
		IntegrityChecks: []string{"sha256 verified", "sqlite integrity_check passed"},
	}
	assertValid(t, ValidateBackupArtifact(artifact))

	artifact.IncludedMaterial = append(artifact.IncludedMaterial, "R39_RAW_SECRET_DO_NOT_LEAK")
	assertInvalidContains(t, ValidateBackupArtifact(artifact), "raw credential")
}
