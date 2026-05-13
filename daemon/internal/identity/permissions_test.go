package identity

import "testing"

func TestPermissionsForRoleUsesTieredLeastPrivilege(t *testing.T) {
	tests := []struct {
		name string
		role Role
		want []Permission
	}{
		{name: "owner", role: RoleOwner, want: AllSensitivePermissions},
		{name: "admin", role: RoleAdmin, want: []Permission{PermissionTenantManage, PermissionSecretsManage, PermissionCredentialsInspect, PermissionIntegrationsManage, PermissionIntegrationDiagnosticsRead, PermissionIntegrationDiagnosticsRun, PermissionIntegrationDiagnosticsSmoke, PermissionIntegrationDiagnosticsSmokeRisky, PermissionConnectorsManage, PermissionMCPManage, PermissionLiveValidationReconcile, PermissionEvaluationManage, PermissionEvaluationDiscoveryRead, PermissionEvaluationDiscoveryRun, PermissionEvaluationDiscoverySuppress, PermissionEvaluationFixtureRead, PermissionEvaluationFixtureManage, PermissionEvaluationFixtureReview, PermissionEvaluationFixtureSuppress, PermissionEvaluationCampaignRead, PermissionEvaluationCampaignManage, PermissionEvaluationDashboardRead, PermissionEvaluationInspectionRead, PermissionEvaluationRetentionManage, PermissionBillingView, PermissionBillingManage, PermissionProfilesInspect, PermissionProfilesManage}},
		{name: "operator", role: RoleOperator, want: []Permission{PermissionRunsExecute, PermissionApprovalsResolve, PermissionLiveValidationExecute, PermissionIntegrationDiagnosticsRead, PermissionIntegrationDiagnosticsRun, PermissionIntegrationDiagnosticsSmoke, PermissionEvaluationDiscoveryRead, PermissionEvaluationDiscoveryRun, PermissionEvaluationDiscoverySuppress, PermissionEvaluationFixtureRead, PermissionEvaluationFixtureSuppress, PermissionEvaluationCampaignRead, PermissionEvaluationCampaignManage, PermissionEvaluationDashboardRead, PermissionEvaluationInspectionRead}},
		{name: "viewer", role: RoleViewer, want: []Permission{PermissionReadOnlyInspect, PermissionEvaluationCampaignRead, PermissionEvaluationDashboardRead, PermissionEvaluationInspectionRead}},
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
		RoleAdmin:    {PermissionTenantManage, PermissionSecretsManage, PermissionCredentialsInspect, PermissionIntegrationsManage, PermissionIntegrationDiagnosticsRead, PermissionIntegrationDiagnosticsRun, PermissionIntegrationDiagnosticsSmoke, PermissionIntegrationDiagnosticsSmokeRisky, PermissionConnectorsManage, PermissionMCPManage, PermissionLiveValidationReconcile, PermissionEvaluationManage, PermissionEvaluationDiscoveryRead, PermissionEvaluationDiscoveryRun, PermissionEvaluationDiscoverySuppress, PermissionEvaluationFixtureRead, PermissionEvaluationFixtureManage, PermissionEvaluationFixtureReview, PermissionEvaluationFixtureSuppress, PermissionEvaluationCampaignRead, PermissionEvaluationCampaignManage, PermissionEvaluationDashboardRead, PermissionEvaluationInspectionRead, PermissionEvaluationRetentionManage, PermissionBillingView, PermissionBillingManage, PermissionProfilesInspect, PermissionProfilesManage},
		RoleOperator: {PermissionRunsExecute, PermissionApprovalsResolve, PermissionLiveValidationExecute, PermissionIntegrationDiagnosticsRead, PermissionIntegrationDiagnosticsRun, PermissionIntegrationDiagnosticsSmoke, PermissionEvaluationDiscoveryRead, PermissionEvaluationDiscoveryRun, PermissionEvaluationDiscoverySuppress, PermissionEvaluationFixtureRead, PermissionEvaluationFixtureSuppress, PermissionEvaluationCampaignRead, PermissionEvaluationCampaignManage, PermissionEvaluationDashboardRead, PermissionEvaluationInspectionRead},
		RoleViewer:   {PermissionReadOnlyInspect, PermissionEvaluationCampaignRead, PermissionEvaluationDashboardRead, PermissionEvaluationInspectionRead},
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

func TestCanResolveLiveValidationReconciliationRequiresOwnerAdminOrPermission(t *testing.T) {
	tests := []struct {
		name    string
		context TenantContext
		want    bool
	}{
		{
			name: "owner role",
			context: TenantContext{
				PrincipalID: "prn_owner",
				TenantID:    "ten_1",
				Role:        RoleOwner,
				Permissions: PermissionsForRole(RoleOwner, StatusActive),
			},
			want: true,
		},
		{
			name: "admin role",
			context: TenantContext{
				PrincipalID: "prn_admin",
				TenantID:    "ten_1",
				Role:        RoleAdmin,
				Permissions: PermissionsForRole(RoleAdmin, StatusActive),
			},
			want: true,
		},
		{
			name: "explicit permission",
			context: TenantContext{
				PrincipalID: "prn_reconciler",
				TenantID:    "ten_1",
				Role:        RoleOperator,
				Permissions: []Permission{PermissionLiveValidationReconcile},
			},
			want: true,
		},
		{
			name: "operator execute only",
			context: TenantContext{
				PrincipalID: "prn_operator",
				TenantID:    "ten_1",
				Role:        RoleOperator,
				Permissions: PermissionsForRole(RoleOperator, StatusActive),
			},
			want: false,
		},
		{
			name: "missing tenant context",
			context: TenantContext{
				PrincipalID: "prn_owner",
				Role:        RoleOwner,
				Permissions: PermissionsForRole(RoleOwner, StatusActive),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanResolveLiveValidationReconciliation(tt.context); got != tt.want {
				t.Fatalf("CanResolveLiveValidationReconciliation()=%v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluationProductPermissionsForRoles(t *testing.T) {
	readOnlyProductPermissions := []Permission{
		PermissionEvaluationCampaignRead,
		PermissionEvaluationDashboardRead,
		PermissionEvaluationInspectionRead,
	}
	adminProductPermissions := []Permission{
		PermissionEvaluationDiscoveryRead,
		PermissionEvaluationDiscoveryRun,
		PermissionEvaluationDiscoverySuppress,
		PermissionEvaluationFixtureRead,
		PermissionEvaluationFixtureManage,
		PermissionEvaluationFixtureReview,
		PermissionEvaluationFixtureSuppress,
		PermissionEvaluationCampaignRead,
		PermissionEvaluationCampaignManage,
		PermissionEvaluationDashboardRead,
		PermissionEvaluationInspectionRead,
		PermissionEvaluationRetentionManage,
	}
	operatorProductPermissions := []Permission{
		PermissionEvaluationDiscoveryRead,
		PermissionEvaluationDiscoveryRun,
		PermissionEvaluationDiscoverySuppress,
		PermissionEvaluationFixtureRead,
		PermissionEvaluationFixtureSuppress,
		PermissionEvaluationCampaignRead,
		PermissionEvaluationCampaignManage,
		PermissionEvaluationDashboardRead,
		PermissionEvaluationInspectionRead,
	}

	for _, permission := range adminProductPermissions {
		if !Can(RoleOwner, StatusActive, permission) {
			t.Fatalf("owner missing product permission %s", permission)
		}
		if !Can(RoleAdmin, StatusActive, permission) {
			t.Fatalf("admin missing product permission %s", permission)
		}
	}
	for _, permission := range operatorProductPermissions {
		if !Can(RoleOperator, StatusActive, permission) {
			t.Fatalf("operator missing product permission %s", permission)
		}
	}
	for _, permission := range readOnlyProductPermissions {
		if !Can(RoleViewer, StatusActive, permission) {
			t.Fatalf("viewer missing read-only product permission %s", permission)
		}
	}

	deniedForOperator := []Permission{
		PermissionEvaluationFixtureManage,
		PermissionEvaluationFixtureReview,
		PermissionEvaluationRetentionManage,
	}
	for _, permission := range deniedForOperator {
		if Can(RoleOperator, StatusActive, permission) {
			t.Fatalf("operator unexpectedly has product permission %s", permission)
		}
	}
	if Can(RoleViewer, StatusActive, PermissionEvaluationCampaignManage) {
		t.Fatal("viewer unexpectedly has campaign manage permission")
	}
	if Can(RoleViewer, StatusActive, PermissionEvaluationDiscoveryRun) {
		t.Fatal("viewer unexpectedly has discovery run permission")
	}
}

func TestBillingEvidenceExportPermissionIsCanonicalAndSeparateFromBillingView(t *testing.T) {
	if !Can(RoleOwner, StatusActive, PermissionBillingEvidenceExport) {
		t.Fatal("owner should have billing evidence export permission")
	}
	if Can(RoleAdmin, StatusActive, PermissionBillingEvidenceExport) {
		t.Fatal("admin should not receive billing evidence export by role-only billing.view/billing.manage permissions")
	}
	viewOnly := TenantContext{
		PrincipalID: "prn_view",
		TenantID:    "ten_1",
		Role:        RoleAdmin,
		Permissions: []Permission{PermissionBillingView},
	}
	if EvaluatePermission(viewOnly, PermissionBillingEvidenceExport).Allowed {
		t.Fatal("billing.view alone must not authorize evidence export")
	}
	explicit := viewOnly
	explicit.Permissions = append(explicit.Permissions, PermissionBillingEvidenceExport)
	if !EvaluatePermission(explicit, PermissionBillingEvidenceExport).Allowed {
		t.Fatal("explicit billing.evidence_export permission should authorize evidence export")
	}
}
