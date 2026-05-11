package setupwizard

import "strings"

type ChannelRepairLinkInput struct {
	TenantID        string
	ConnectorID     string
	ConnectorKind   string
	RepairActionID  string
	SetupSessionID  string
	RepairAction    string
	DiagnosticRefID string
}

func ChannelRepairSetupSessionHint(input ChannelRepairLinkInput) ResourceRef {
	id := strings.TrimSpace(input.SetupSessionID)
	if id == "" {
		id = strings.TrimSpace(input.ConnectorID)
	}
	return ResourceRef{
		Kind: "channel_management_repair",
		ID:   id,
	}
}
