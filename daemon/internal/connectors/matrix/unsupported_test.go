package matrix

import "testing"

func TestUnsupportedMessageKindClassifiesMatrixUnsupportedSurfaces(t *testing.T) {
	t.Parallel()

	for _, kind := range []MessageKind{
		MessageEncryptedUnsupported,
		MessageUndecryptableUnsupported,
		MessageUnsupported,
		MessageMediaUnsupported,
		MessageCallUnsupported,
		MessageVoiceUnsupported,
		MessageReactionUnsupported,
		MessageBridgeMetadataUnsupported,
		MessageUnknown,
	} {
		if !UnsupportedMessageKind(kind) {
			t.Fatalf("kind %s should be unsupported", kind)
		}
	}
	if UnsupportedMessageKind(MessageUnencryptedText) {
		t.Fatal("unencrypted text should remain supported")
	}
}
