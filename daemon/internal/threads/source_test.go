package threads

import "testing"

func TestNormalizeSourceContinuationKey(t *testing.T) {
	key, err := NormalizeSourceContinuationKey(SourceContinuationKey{
		TenantID:             " ten_1 ",
		ConnectorID:          " Slack-Main ",
		SourceAccountID:      " Workspace_A ",
		SourceConversationID: " Channel_A ",
	})
	if err != nil {
		t.Fatalf("NormalizeSourceContinuationKey returned error: %v", err)
	}
	if key.TenantID != "ten_1" || key.ConnectorID != "slack-main" || key.SourceAccountID != "workspace_a" || key.SourceConversationID != "channel_a" {
		t.Fatalf("unexpected normalized key: %#v", key)
	}
	if key.String() != "ten_1\x00slack-main\x00workspace_a\x00channel_a" {
		t.Fatalf("unexpected stable key string: %q", key.String())
	}
	if _, err := NormalizeSourceContinuationKey(SourceContinuationKey{TenantID: "ten_1"}); err == nil {
		t.Fatal("expected missing source fields to be rejected")
	}
}
