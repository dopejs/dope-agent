package threads

import "testing"

func TestSafeSummarySuppressesUnsafeEvidence(t *testing.T) {
	for _, unsafe := range []string{
		"token=secret raw_payload",
		"provider payload {\"secret\":\"value\"}",
		"disallowed message body content",
		"source identity user@example.test",
	} {
		summary := SafeSummary(unsafe, false)
		if summary.Status != RedactionStatusSuppressed || summary.Text != "suppressed" {
			t.Fatalf("expected suppressed unsafe summary for %q, got %#v", unsafe, summary)
		}
	}
	redacted := SafeSummary("metadata only", true)
	if redacted.Status != RedactionStatusRedacted || redacted.Text != "metadata only" {
		t.Fatalf("expected safe summary, got %#v", redacted)
	}
}
