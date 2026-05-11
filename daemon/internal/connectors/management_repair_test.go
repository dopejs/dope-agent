package connectors

import "testing"

func TestRepairTerminalStateDoesNotReEnableDisabledConnector(t *testing.T) {
	t.Parallel()

	if got := TerminalStateForRepairAction(ManagementActionReconnect, true); got != ManagementTerminalDisabled {
		t.Fatalf("disabled reconnect terminal=%s, want disabled", got)
	}
	if got := RetrySafetyForRepairAction(ManagementActionCredentialRotation); got != RetrySafetyBlocked {
		t.Fatalf("credential rotation retry safety=%s, want blocked", got)
	}
}
