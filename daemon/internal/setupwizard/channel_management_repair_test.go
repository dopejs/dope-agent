package setupwizard

import "testing"

func TestChannelRepairSetupSessionHintLinksRepairToSetupSession(t *testing.T) {
	t.Parallel()

	ref := ChannelRepairSetupSessionHint(ChannelRepairLinkInput{
		ConnectorID:    "slack-main",
		RepairActionID: "repair_1",
		SetupSessionID: "setup_1",
	})
	if ref.Kind != "channel_management_repair" || ref.ID != "setup_1" {
		t.Fatalf("unexpected repair setup ref: %+v", ref)
	}
}
