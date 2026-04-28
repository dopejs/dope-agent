package secrets

import "testing"

const (
	r37TenantAID = "tenant_r37_a"
	r37TenantBID = "tenant_r37_b"

	r37SecretRefShared = "calendar_api_token"
	r37SecretRefMCP    = "mcp_api_token"
	r37SecretRefSkill  = "skill_api_token"

	r37FakeSecretTenantA = "R37_FAKE_SECRET_TENANT_A_DO_NOT_LEAK"
	r37FakeSecretTenantB = "R37_FAKE_SECRET_TENANT_B_DO_NOT_LEAK"
	r37FakeTokenTenantA  = "R37_FAKE_TOKEN_TENANT_A_DO_NOT_LEAK"
	r37FakeTokenTenantB  = "R37_FAKE_TOKEN_TENANT_B_DO_NOT_LEAK"
)

type r37TenantSecretFixture struct {
	TenantID    string
	SecretRef   string
	DisplayName string
	RawValue    string
}

func r37TwoTenantSecretFixtures(t *testing.T) (r37TenantSecretFixture, r37TenantSecretFixture) {
	t.Helper()
	return r37TenantSecretFixture{
			TenantID:    r37TenantAID,
			SecretRef:   r37SecretRefShared,
			DisplayName: "Tenant A calendar token",
			RawValue:    r37FakeSecretTenantA,
		}, r37TenantSecretFixture{
			TenantID:    r37TenantBID,
			SecretRef:   r37SecretRefShared,
			DisplayName: "Tenant B calendar token",
			RawValue:    r37FakeSecretTenantB,
		}
}

func r37LeakSentinels() []string {
	return []string{
		r37FakeSecretTenantA,
		r37FakeSecretTenantB,
		r37FakeTokenTenantA,
		r37FakeTokenTenantB,
	}
}
