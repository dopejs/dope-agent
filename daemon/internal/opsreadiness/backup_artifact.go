package opsreadiness

import (
	"fmt"
	"strings"
)

func ValidateBackupArtifact(artifact BackupArtifact) error {
	errs := []error{
		RequireNonEmpty("artifact id", artifact.ArtifactID),
		RequireNonEmpty("source version", artifact.SourceVersion),
		RequireNonEmpty("source environment", artifact.SourceEnvironment),
		RequireItems("included material", artifact.IncludedMaterial),
		RequireItems("excluded material", artifact.ExcludedMaterial),
		RequireItems("integrity checks", artifact.IntegrityChecks),
		validateRepresentativeTenants(artifact.TenantCount, artifact.TenantStateSummary),
		requireNoRawCredentialSlice("included material", artifact.IncludedMaterial),
		requireNoRawCredentialSlice("integrity checks", artifact.IntegrityChecks),
		requireCredentialExclusions(artifact.ExcludedMaterial),
	}
	for i, tenant := range artifact.TenantStateSummary {
		errs = append(errs, validateTenantStateSummary(fmt.Sprintf("tenant state summary[%d]", i), tenant))
	}
	return JoinErrors(errs...)
}

func validateRepresentativeTenants(count int, tenants []TenantStateSummary) error {
	if count < MinimumTenantCount || len(tenants) < MinimumTenantCount {
		return fmt.Errorf("representative backup requires at least %d tenants", MinimumTenantCount)
	}
	seen := map[string]bool{}
	for _, tenant := range tenants {
		if seen[tenant.TenantID] {
			return fmt.Errorf("duplicate tenant %q", tenant.TenantID)
		}
		seen[tenant.TenantID] = true
	}
	return nil
}

func validateTenantStateSummary(label string, tenant TenantStateSummary) error {
	return JoinErrors(
		RequireNonEmpty(label+".tenant id", tenant.TenantID),
		RequireItems(label+".credential refs", tenant.CredentialRefs),
		RequireNonEmpty(label+".quota state", tenant.QuotaState),
		RequireNonEmpty(label+".work state", tenant.WorkState),
		requireNoRawCredentialSlice(label+".credential refs", tenant.CredentialRefs),
	)
}

func requireCredentialExclusions(values []string) error {
	required := []string{"raw secret", "access token", "refresh token", "oauth", "provider token", "derived credential"}
	joined := strings.ToLower(strings.Join(values, "\n"))
	var missing []string
	for _, item := range required {
		if !strings.Contains(joined, item) {
			missing = append(missing, item)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("excluded material missing credential exclusions: %s", strings.Join(missing, ", "))
	}
	return nil
}
