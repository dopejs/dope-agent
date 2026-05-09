package setupwizard

import (
	"sort"
	"strings"
)

func CatalogTargets(tenantID string) []SetupTarget {
	targets := []SetupTarget{
		{
			TargetID:                TargetOpenAICompatible,
			TenantID:                strings.TrimSpace(tenantID),
			TargetKind:              TargetKindProvider,
			SetupStyle:              SetupStyleSubmittedSecret,
			DisplayName:             "OpenAI-compatible provider",
			ProofTarget:             true,
			SupportStatus:           SupportStatusSupported,
			RequiredPermissions:     []string{PermissionSecretsManage, PermissionIntegrationsManage},
			LimitedSafeCapabilities: []string{"metadata_read"},
		},
		{
			TargetID:                TargetFeishuLark,
			TenantID:                strings.TrimSpace(tenantID),
			TargetKind:              TargetKindIntegration,
			SetupStyle:              SetupStyleOAuth,
			DisplayName:             "Feishu/Lark OAuth",
			ProofTarget:             true,
			SupportStatus:           SupportStatusSupported,
			RequiredPermissions:     []string{PermissionSecretsManage, PermissionIntegrationsManage},
			LimitedSafeCapabilities: []string{"metadata_read"},
		},
		{
			TargetID:                TargetDiscordConnector,
			TenantID:                strings.TrimSpace(tenantID),
			TargetKind:              TargetKindConnector,
			SetupStyle:              SetupStyleSubmittedSecret,
			DisplayName:             "Discord connector",
			ProofTarget:             true,
			SupportStatus:           SupportStatusSupported,
			RequiredPermissions:     []string{PermissionSecretsManage, PermissionIntegrationsManage},
			LimitedSafeCapabilities: []string{"metadata_read", "destination_validation"},
		},
		{
			TargetID:                TargetTelegramConnector,
			TenantID:                strings.TrimSpace(tenantID),
			TargetKind:              TargetKindConnector,
			SetupStyle:              SetupStyleSubmittedSecret,
			DisplayName:             "Telegram connector",
			ProofTarget:             true,
			SupportStatus:           SupportStatusSupported,
			RequiredPermissions:     []string{PermissionSecretsManage, PermissionIntegrationsManage},
			LimitedSafeCapabilities: []string{"metadata_read", "allowment_validation"},
		},
		{
			TargetID:                TargetSlackConnector,
			TenantID:                strings.TrimSpace(tenantID),
			TargetKind:              TargetKindConnector,
			SetupStyle:              SetupStyleOAuth,
			DisplayName:             "Slack connector",
			ProofTarget:             true,
			SupportStatus:           SupportStatusSupported,
			RequiredPermissions:     []string{PermissionSecretsManage, PermissionIntegrationsManage},
			LimitedSafeCapabilities: []string{"metadata_read", "route_policy_validation", "workspace_validation"},
		},
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].TargetID < targets[j].TargetID })
	return targets
}

func TargetByID(tenantID, targetID string) (SetupTarget, bool) {
	targetID = strings.TrimSpace(targetID)
	for _, target := range CatalogTargets(tenantID) {
		if target.TargetID == targetID {
			return target, true
		}
	}
	return SetupTarget{
		TargetID:      targetID,
		TenantID:      strings.TrimSpace(tenantID),
		TargetKind:    TargetKindProvider,
		SetupStyle:    SetupStyleUnsupported,
		DisplayName:   firstNonEmpty(targetID, "Unsupported target"),
		SupportStatus: SupportStatusUnsupported,
		CurrentState:  StateActionRequired,
		ProofTarget:   false,
		RequiredPermissions: []string{
			PermissionSecretsManage,
			PermissionIntegrationsManage,
		},
	}, false
}
