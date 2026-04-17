package capabilities

import "testing"

func TestCapabilitySupervisorLifecycle(t *testing.T) {
	supervisor := NewSupervisor()

	capability, created, err := supervisor.Register(RegisterInput{
		CapabilityID: "shell",
		Kind:         "exec",
		DisplayName:  "Shell",
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if !created {
		t.Fatal("expected first register to create capability")
	}
	if capability.Status != StatusRegistered {
		t.Fatalf("expected registered status, got %s", capability.Status)
	}

	capability, err = supervisor.ReportHealth(capability.CapabilityID, ReportHealthInput{Status: StatusHealthy})
	if err != nil {
		t.Fatalf("ReportHealth returned error: %v", err)
	}
	if capability.Status != StatusHealthy {
		t.Fatalf("expected healthy status, got %s", capability.Status)
	}

	capability, err = supervisor.ReportFailure(capability.CapabilityID, ReportFailureInput{Reason: "worker crashed"})
	if err != nil {
		t.Fatalf("ReportFailure returned error: %v", err)
	}
	if capability.Status != StatusBackingOff {
		t.Fatalf("expected backing_off status, got %s", capability.Status)
	}

	capability, err = supervisor.Restart(capability.CapabilityID)
	if err != nil {
		t.Fatalf("Restart returned error: %v", err)
	}
	if capability.Status != StatusRegistered {
		t.Fatalf("expected registered after restart, got %s", capability.Status)
	}
	if capability.RestartCount != 1 {
		t.Fatalf("expected restart count 1, got %d", capability.RestartCount)
	}
}

func TestCapabilitySupervisorFailsAfterRepeatedFailures(t *testing.T) {
	supervisor := NewSupervisor()

	capability, _, err := supervisor.Register(RegisterInput{
		CapabilityID: "browser",
		Kind:         "browser",
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	for range 5 {
		capability, err = supervisor.ReportFailure(capability.CapabilityID, ReportFailureInput{Reason: "crash loop"})
		if err != nil {
			t.Fatalf("ReportFailure returned error: %v", err)
		}
	}

	if capability.Status != StatusFailed {
		t.Fatalf("expected failed status, got %s", capability.Status)
	}
}
