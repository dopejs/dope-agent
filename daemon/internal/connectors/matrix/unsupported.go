package matrix

func UnsupportedMessageKind(kind MessageKind) bool {
	return kind == MessageEncryptedUnsupported ||
		kind == MessageUndecryptableUnsupported ||
		kind == MessageMediaUnsupported ||
		kind == MessageCallUnsupported ||
		kind == MessageVoiceUnsupported ||
		kind == MessageReactionUnsupported ||
		kind == MessageBridgeMetadataUnsupported ||
		kind == MessageUnsupported ||
		kind == MessageUnknown
}
