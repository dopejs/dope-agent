package identity

import "testing"

func TestPermissionsForRoleUsesTieredLeastPrivilege(t *testing.T) {
	tests := []struct {
		name string
		role Role
		want []Permission
	}{
		{name: "owner", role: RoleOwner, want: AllSensitivePermissions},
		{name: "admin", role: RoleAdmin, want: []Permission{PermissionTenantManage, PermissionSecretsManage, PermissionCredentialsInspect, PermissionIntegrationsManage, PermissionConnectorsManage, PermissionMCPManage, PermissionEvaluationManage, PermissionBillingView, PermissionBillingManage}},
		{name: "operator", role: RoleOperator, want: []Permission{PermissionRunsExecute, PermissionApprovalsResolve, PermissionLiveValidationExecute}},
		{name: "viewer", role: RoleViewer, want: []Permission{PermissionReadOnlyInspect}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PermissionsForRole(tt.role, StatusActive)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d permissions, got %d: %v", len(tt.want), len(got), got)
			}
			for _, permission := range tt.want {
				if !HasPermission(got, permission) {
					t.Fatalf("expected %s in %v", permission, got)
				}
			}
		})
	}
}

func TestPermissionsDeniedForInactiveLifecycle(t *testing.T) {
	statuses := []LifecycleStatus{StatusInvited, StatusDisabled, StatusRemoved, StatusRevoked, StatusExpired, StatusRotated}
	for _, status := range statuses {
		if got := PermissionsForRole(RoleOwner, status); len(got) != 0 {
			t.Fatalf("expected no permissions for status %s, got %v", status, got)
		}
	}
}

func TestPermissionEvaluatorCoversSensitiveCapabilities(t *testing.T) {
	rolePermissions := map[Role][]Permission{
		RoleOwner:    AllSensitivePermissions,
		RoleAdmin:    {PermissionTenantManage, PermissionSecretsManage, PermissionCredentialsInspect, PermissionIntegrationsManage, PermissionConnectorsManage, PermissionMCPManage, PermissionEvaluationManage, PermissionBillingView, PermissionBillingManage},
		RoleOperator: {PermissionRunsExecute, PermissionApprovalsResolve, PermissionLiveValidationExecute},
		RoleViewer:   {PermissionReadOnlyInspect},
	}
	for role, allowed := range rolePermissions {
		for _, permission := range append(AllSensitivePermissions, PermissionReadOnlyInspect) {
			evaluation := EvaluatePermission(TenantContext{
				PrincipalID: "prn_1",
				TenantID:    "ten_1",
				Role:        role,
				Permissions: PermissionsForRole(role, StatusActive),
			}, permission)
			wantAllowed := HasPermission(allowed, permission)
			if evaluation.Allowed != wantAllowed {
				t.Fatalf("role %s permission %s allowed=%v, want %v", role, permission, evaluation.Allowed, wantAllowed)
			}
		}
	}
}
