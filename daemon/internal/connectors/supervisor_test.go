package connectors

import "testing"

func TestConnectorSupervisorLifecycle(t *testing.T) {
	supervisor := NewSupervisor()

	connector, created, err := supervisor.Register(RegisterInput{
		ConnectorID: "slack-main",
		Kind:        "slack",
		DisplayName: "Slack Main",
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if !created {
		t.Fatal("expected first register to create connector")
	}
	if connector.Status != StatusRegistered {
		t.Fatalf("expected registered status, got %s", connector.Status)
	}

	connector, err = supervisor.ReportHealth(connector.ConnectorID, ReportHealthInput{Status: StatusHealthy})
	if err != nil {
		t.Fatalf("ReportHealth returned error: %v", err)
	}
	if connector.Status != StatusHealthy {
		t.Fatalf("expected healthy status, got %s", connector.Status)
	}

	connector, err = supervisor.ReportFailure(connector.ConnectorID, ReportFailureInput{Reason: "socket disconnected"})
	if err != nil {
		t.Fatalf("ReportFailure returned error: %v", err)
	}
	if connector.Status != StatusBackingOff {
		t.Fatalf("expected backing_off status, got %s", connector.Status)
	}
	if connector.BackoffSeconds == 0 || connector.NextRestartAt == nil {
		t.Fatalf("expected backoff state to be set, got %+v", connector)
	}

	connector, err = supervisor.Restart(connector.ConnectorID)
	if err != nil {
		t.Fatalf("Restart returned error: %v", err)
	}
	if connector.Status != StatusRegistered {
		t.Fatalf("expected registered after restart, got %s", connector.Status)
	}
	if connector.RestartCount != 1 {
		t.Fatalf("expected restart count 1, got %d", connector.RestartCount)
	}
}

func TestConnectorSupervisorFailsAfterRepeatedFailures(t *testing.T) {
	supervisor := NewSupervisor()

	connector, _, err := supervisor.Register(RegisterInput{
		ConnectorID: "discord-main",
		Kind:        "discord",
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	for range 5 {
		connector, err = supervisor.ReportFailure(connector.ConnectorID, ReportFailureInput{Reason: "connection failed"})
		if err != nil {
			t.Fatalf("ReportFailure returned error: %v", err)
		}
	}

	if connector.Status != StatusFailed {
		t.Fatalf("expected failed status, got %s", connector.Status)
	}
}

func TestConnectorSupervisorTenantOwnershipAndDisable(t *testing.T) {
	supervisor := NewSupervisor()
	if _, _, err := supervisor.Register(RegisterInput{
		TenantID:    "ten_a",
		ConnectorID: "discord-shared",
		Kind:        "discord",
		SecretRefs:  []string{"discord/token"},
	}); err != nil {
		t.Fatalf("Register tenant A returned error: %v", err)
	}
	if _, _, err := supervisor.Register(RegisterInput{
		TenantID:    "ten_b",
		ConnectorID: "slack-b",
		Kind:        "slack",
	}); err != nil {
		t.Fatalf("Register tenant B returned error: %v", err)
	}
	if got := supervisor.ListForTenant("ten_a"); len(got) != 1 || got[0].ConnectorID != "discord-shared" || got[0].SecretRefs[0] != "discord/token" {
		t.Fatalf("tenant A list did not preserve ownership and refs: %+v", got)
	}
	if _, ok := supervisor.GetForTenant("discord-shared", "ten_b"); ok {
		t.Fatal("tenant B unexpectedly resolved tenant A connector")
	}
	disabled, err := supervisor.Disable("discord-shared", "integration disconnected")
	if err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}
	if disabled.Status != StatusDisabled || disabled.DisabledReason == "" {
		t.Fatalf("expected disabled connector with reason, got %+v", disabled)
	}
	if _, err := supervisor.Restart("discord-shared"); err != ErrConnectorDisabled {
		t.Fatalf("expected disabled restart denial, got %v", err)
	}
}
