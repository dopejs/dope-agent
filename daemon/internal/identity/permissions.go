package identity

func PermissionsForRole(role Role, lifecycle LifecycleStatus) []Permission {
	if lifecycle != StatusActive {
		return nil
	}
	switch role {
	case RoleOwner:
		return append([]Permission(nil), AllSensitivePermissions...)
	case RoleAdmin:
		return []Permission{
			PermissionTenantManage,
			PermissionSecretsManage,
			PermissionCredentialsInspect,
			PermissionIntegrationsManage,
			PermissionIntegrationDiagnosticsRead,
			PermissionIntegrationDiagnosticsRun,
			PermissionIntegrationDiagnosticsSmoke,
			PermissionIntegrationDiagnosticsSmokeRisky,
			PermissionConnectorsManage,
			PermissionMCPManage,
			PermissionLiveValidationReconcile,
			PermissionEvaluationManage,
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
			PermissionBillingView,
			PermissionBillingManage,
			PermissionProfilesInspect,
			PermissionProfilesManage,
			PermissionBindingsInspect,
			PermissionBindingsManage,
		}
	case RoleOperator:
		return []Permission{
			PermissionRunsExecute,
			PermissionApprovalsResolve,
			PermissionLiveValidationExecute,
			PermissionIntegrationDiagnosticsRead,
			PermissionIntegrationDiagnosticsRun,
			PermissionIntegrationDiagnosticsSmoke,
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
	case RoleViewer:
		return []Permission{
			PermissionReadOnlyInspect,
			PermissionEvaluationCampaignRead,
			PermissionEvaluationDashboardRead,
			PermissionEvaluationInspectionRead,
		}
	default:
		return nil
	}
}

func CanInspectCredentials(tenantContext TenantContext, managePermissions ...Permission) bool {
	if tenantContext.PrincipalID == "" || tenantContext.TenantID == "" {
		return false
	}
	if HasPermission(tenantContext.Permissions, PermissionCredentialsInspect) {
		return true
	}
	for _, permission := range managePermissions {
		if HasPermission(tenantContext.Permissions, permission) {
			return true
		}
	}
	return false
}

func CanResolveLiveValidationReconciliation(tenantContext TenantContext) bool {
	if tenantContext.PrincipalID == "" || tenantContext.TenantID == "" {
		return false
	}
	if tenantContext.Role == RoleOwner || tenantContext.Role == RoleAdmin {
		return true
	}
	return HasPermission(tenantContext.Permissions, PermissionLiveValidationReconcile)
}

func HasPermission(perms []Permission, permission Permission) bool {
	for _, item := range perms {
		if item == permission {
			return true
		}
	}
	return false
}

func Can(role Role, lifecycle LifecycleStatus, permission Permission) bool {
	return HasPermission(PermissionsForRole(role, lifecycle), permission)
}

func EvaluatePermission(tenantContext TenantContext, permission Permission) PermissionEvaluation {
	evaluation := PermissionEvaluation{
		Permission: permission,
		ReasonCode: "permission_missing",
	}
	if tenantContext.PrincipalID == "" || tenantContext.TenantID == "" {
		evaluation.ReasonCode = "tenant_context_missing"
		return evaluation
	}
	if !HasPermission(tenantContext.Permissions, permission) {
		return evaluation
	}
	evaluation.Allowed = true
	evaluation.ReasonCode = ""
	return evaluation
}

func RequirePermission(tenantContext TenantContext, permission Permission) error {
	if !EvaluatePermission(tenantContext, permission).Allowed {
		return ErrPermissionDenied
	}
	return nil
}
