package events

const (
	ConnectorEventMatrixSetupValidated         = "connector.matrix_setup_validated"
	ConnectorEventMatrixSmokeEvidenceRecorded  = "connector.matrix_smoke_evidence_recorded"
	ConnectorEventRouteOutcomeRecorded         = "connector.route_outcome_recorded"
	ConnectorEventInboundDuplicateDetected     = "connector.inbound_duplicate_detected"
	ConnectorEventReplySent                    = "connector.reply_sent"
	ConnectorEventReplyFailed                  = "connector.reply_failed"
	ConnectorEventForegroundReplyFailed        = "connector.foreground_reply_failed"
	ConnectorEventDeliverySeparationRecorded   = "connector.delivery_separation_recorded"
	ConnectorEventDiagnosticStateChanged       = "connector.diagnostic_state_changed"
	ConnectorEventDiagnosticRedactionFailed    = "connector.diagnostic_redaction_failed"
	ConnectorEventConnectorDiagnosticRecorded  = "connector.diagnostic_recorded"
	ConnectorEventConnectorSetupRepairRequired = "connector.setup_repair_required"
)

var MatrixConnectorEventNames = []string{
	ConnectorEventMatrixSetupValidated,
	ConnectorEventRouteOutcomeRecorded,
	ConnectorEventInboundDuplicateDetected,
	ConnectorEventReplySent,
	ConnectorEventReplyFailed,
	ConnectorEventForegroundReplyFailed,
	ConnectorEventDeliverySeparationRecorded,
	ConnectorEventDiagnosticStateChanged,
	ConnectorEventDiagnosticRedactionFailed,
	ConnectorEventMatrixSmokeEvidenceRecorded,
}
