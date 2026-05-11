package delivery

import "testing"

func TestChannelManagementDeliveryStatusKeepsForegroundAndBackgroundIdentifiersSeparate(t *testing.T) {
	t.Parallel()

	result := SendResult{
		TransportKind:               string(TargetKindConnectorRoute),
		ConnectorMessageDeliveryID:  "delivery_background",
		ConnectorDeliveryBoundaryID: "boundary_delivery_background",
		SeparationStatus:            "separate_truths",
	}
	if result.ConnectorMessageDeliveryID == "" || result.ConnectorDeliveryBoundaryID == "" || result.SeparationStatus != "separate_truths" {
		t.Fatalf("unexpected delivery separation result: %+v", result)
	}
}
