//! Port of daemon/internal/connectors/matrix/reply.go: final-only reply
//! sending with a separated assistant-vs-matrix delivery outcome.

use kura_connectors::{DiagnosticReasonCode, RedactionStatus};
use kura_imtypes::OutboundReply;

use crate::dedupe::dedupe_key;
use crate::transport::Transport;
use crate::types::{InboundEvent, ReplyOutcome};

/// Go `SendFinalReply`: fills the reply's connector/channel/reply-to
/// coordinates from the inbound event, then records a reply outcome whose
/// assistant execution truth and Matrix delivery truth are kept separate.
#[must_use]
pub fn send_final_reply(
    transport: Option<&dyn Transport>,
    event: &InboundEvent,
    mut reply: OutboundReply,
) -> ReplyOutcome {
    if reply.connector_id.is_empty() {
        reply.connector_id = event.connector_id.clone();
    }
    if reply.channel_id.is_empty() {
        reply.channel_id = event.conversation_id.clone();
    }
    if reply.reply_to_external_message_id.is_empty() {
        reply.reply_to_external_message_id = event.matrix_event_id.clone();
    }
    let mut outcome = ReplyOutcome {
        tenant_id: event.tenant_id.clone(),
        connector_id: event.connector_id.clone(),
        inbound_event_identity: dedupe_key(event),
        assistant_execution_outcome: "succeeded".to_string(),
        matrix_reply_outcome: "sent".to_string(),
        reply_progression_level: "final_only".to_string(),
        reply_context: event.conversation_type,
        failure_reason_code: String::new(),
        redaction_status: RedactionStatus::Redacted,
    };
    let Some(transport) = transport else {
        outcome.matrix_reply_outcome = "not_attempted".to_string();
        outcome.failure_reason_code = DiagnosticReasonCode::ReplyFailed.as_str().to_string();
        return outcome;
    };
    if transport.send_reply(reply).is_err() {
        outcome.matrix_reply_outcome = "failed".to_string();
        outcome.failure_reason_code = DiagnosticReasonCode::ReplyFailed.as_str().to_string();
    }
    outcome
}
