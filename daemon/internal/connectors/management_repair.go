package connectors

func TerminalStateForRepairAction(actionKind ManagementActionKind, disabled bool) ManagementTerminalState {
	if disabled {
		return ManagementTerminalDisabled
	}
	switch actionKind {
	case ManagementActionDisable:
		return ManagementTerminalDisabled
	case ManagementActionDiagnosticRerun, ManagementActionRouteRevalidate:
		return ManagementTerminalDegraded
	default:
		return ManagementTerminalActionRequired
	}
}

func RetrySafetyForRepairAction(actionKind ManagementActionKind) RetrySafety {
	switch actionKind {
	case ManagementActionReconnect, ManagementActionCredentialRotation:
		return RetrySafetyBlocked
	case ManagementActionDiagnosticRerun, ManagementActionRouteRevalidate:
		return RetrySafetyRetryable
	default:
		return RetrySafetyRetryable
	}
}
