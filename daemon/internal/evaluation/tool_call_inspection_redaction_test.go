package evaluation

import (
	"strings"
	"testing"
)

func TestRedactedToolCallDiffRedactsOriginalReplayAndCustomSensitiveFields(t *testing.T) {
	t.Parallel()

	summary, status := RedactedToolCallDiff(ToolCallDiffInput{
		Original: map[string]any{
			"access_token":  "secret_original",
			"custom_secret": "tenant_secret",
			"result": map[string]any{
				"value": "before",
			},
		},
		Replay: map[string]any{
			"access_token":  "secret_replay",
			"custom_secret": "tenant_secret",
			"result": map[string]any{
				"value": "after",
			},
		},
		Policy: RedactionPolicy{SensitiveFieldRules: []string{"custom_secret"}},
	})
	if status != RedactionStatusRedacted {
		t.Fatalf("status=%q, want redacted", status)
	}
	if !strings.Contains(summary, "drifted") {
		t.Fatalf("summary=%q, want drifted redacted summary", summary)
	}

	matched, status := RedactedToolCallDiff(ToolCallDiffInput{
		Original: map[string]any{"access_token": "secret", "value": "same"},
		Replay:   map[string]any{"access_token": "other", "value": "same"},
	})
	if status != RedactionStatusRedacted || !strings.Contains(matched, "matched") {
		t.Fatalf("matched=%q status=%q, want matched with redaction", matched, status)
	}
}
