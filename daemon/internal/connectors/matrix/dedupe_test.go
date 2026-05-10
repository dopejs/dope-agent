package matrix

import "testing"

func TestDedupeUsesHomeserverConversationAndEventID(t *testing.T) {
	t.Parallel()

	cache := NewDedupeCache()
	first := InboundEvent{TenantID: "ten", ConnectorID: "matrix-main", HomeserverID: "matrix.example.org", ConversationID: "!room:example.org", MatrixEventID: "$event"}
	replayed := first
	replayed.SyncBatchID = "sync-2"

	if cache.MarkDuplicate(first) {
		t.Fatal("first event should not be duplicate")
	}
	if !cache.MarkDuplicate(replayed) {
		t.Fatal("same homeserver/conversation/event should be duplicate despite different sync batch")
	}

	otherRoom := first
	otherRoom.ConversationID = "!other:example.org"
	if cache.MarkDuplicate(otherRoom) {
		t.Fatal("same event id in different conversation should not be duplicate")
	}
}
