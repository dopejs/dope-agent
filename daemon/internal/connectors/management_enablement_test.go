package connectors

import (
	"testing"
	"time"
)

func TestManagementProjectionDisablesDeliveryForDisabledConnector(t *testing.T) {
	t.Parallel()

	projection := BuildConnectorProjection(Connector{
		ConnectorID:    "discord-main",
		Kind:           "discord",
		DisplayName:    "Discord Main",
		Status:         StatusDisabled,
		DisabledReason: "maintenance",
		UpdatedAt:      time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC),
	}, nil, time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC))

	if projection.EnablementState != ManagementStateDisabled || projection.DeliveryEligible {
		t.Fatalf("expected disabled projection to block delivery, got %+v", projection)
	}
	if projection.NextAction == nil || projection.NextAction.ActionKind != ManagementActionReEnable {
		t.Fatalf("expected re-enable next action, got %+v", projection.NextAction)
	}
}

func TestSupervisorRequireInboundReadyFailsClosedForDisabledConnector(t *testing.T) {
	t.Parallel()

	supervisor := NewSupervisor()
	connector, _, err := supervisor.Register(RegisterInput{
		TenantID:    "ten_channels",
		ConnectorID: "discord-main",
		Kind:        "discord",
		DisplayName: "Discord Main",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, err := supervisor.Disable(connector.ConnectorID, "maintenance"); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	if _, err := supervisor.RequireInboundReady(connector.ConnectorID, "ten_channels"); err != ErrConnectorDisabled {
		t.Fatalf("expected ErrConnectorDisabled before ingress creates work, got %v", err)
	}
}
