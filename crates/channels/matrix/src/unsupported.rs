//! Port of daemon/internal/connectors/matrix/unsupported.go: unsupported
//! message-kind classification.

use crate::types::MessageKind;

/// Go `UnsupportedMessageKind`: unencrypted text is the only supported kind;
/// every other classified kind routes as unsupported.
#[must_use]
pub fn unsupported_message_kind(kind: MessageKind) -> bool {
    matches!(
        kind,
        MessageKind::EncryptedUnsupported
            | MessageKind::UndecryptableUnsupported
            | MessageKind::MediaUnsupported
            | MessageKind::CallUnsupported
            | MessageKind::VoiceUnsupported
            | MessageKind::ReactionUnsupported
            | MessageKind::BridgeMetadataUnsupported
            | MessageKind::Unsupported
            | MessageKind::Unknown
    )
}
