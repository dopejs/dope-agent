package tenancy

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/mcp"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
)

const (
	r37TenantAID = "tenant_r37_a"
	r37TenantBID = "tenant_r37_b"

	r37SharedIntegrationID = "calendar-shared"
	r37SharedConnectorID   = "discord-shared"
	r37SharedMCPServerID   = "filesystem-shared"
	r37SharedProviderID    = "managed-provider-shared"
	r37SharedSecretRef     = "calendar_api_token"
)

func r37FakeIntegrationBinding(tenantID string) integrations.BindingSummary {
	return integrations.BindingSummary{
		IntegrationID:         r37SharedIntegrationID,
		DomainKind:            "calendar",
		DisplayName:           "R37 Calendar",
		AccountKey:            "acct_" + tenantID,
		CanonicalDefault:      true,
		ReadinessAtInvocation: integrations.ReadinessStatusHealthy,
		BackendKind:           integrations.BackendKindFakeLocal,
		SecretResolution:      string(sandbox.SecretResolutionResolved),
		EnvironmentScope:      "test",
		CapturedAt:            time.Now().UTC(),
	}
}

func r37FakeMCPServer(tenantID string) mcp.Server {
	now := time.Now().UTC()
	return mcp.Server{
		ServerID:         r37SharedMCPServerID,
		DisplayName:      "R37 Filesystem " + tenantID,
		Source:           mcp.SourceAPI,
		TransportKind:    mcp.TransportKindStdio,
		Command:          "r37-fake-mcp",
		SecretRefs:       []string{r37SharedSecretRef},
		EnvironmentScope: "test",
		Enabled:          true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func r37FakeSandboxProfile(tenantID string) sandbox.Profile {
	return sandbox.Profile{
		ProfileID:        "r37_profile_" + tenantID,
		Title:            "R37 Sandbox " + tenantID,
		BackendKind:      sandbox.BackendKindSubprocess,
		EnvPolicy:        sandbox.EnvironmentPolicy{Mode: sandbox.EnvironmentModeClean},
		DefaultTimeoutMs: 1000,
		MaxTimeoutMs:     1000,
		Source:           sandbox.SourceBuiltin,
		Active:           true,
	}
}
