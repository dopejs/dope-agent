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
