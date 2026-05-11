package threads

import (
	"testing"
	"time"
)

func TestContinuityPreviewSuppressesUnsafeTurnAndArtifactEvidence(t *testing.T) {
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	turn := ContinuityTurn{
		ContinuityTurnID:       "turn_unsafe",
		TenantID:               "ten_1",
		ThreadID:               "thr_1",
		SessionSegmentID:       "seg_1",
		AcceptanceSequence:     1,
		Role:                   ContinuityRoleUser,
		SourceKind:             SourceKindChat,
		SafeContent:            "raw unsafe content",
		ContentRedactionStatus: RedactionStatusRedactionFailed,
		RecordedAt:             now,
		RetentionExpiresAt:     now.Add(90 * 24 * time.Hour),
	}
	item := PreviewItemForTurn(turn, ContinuityDecisionExcluded, ContinuityReasonRedactionFailed, 0)
	if item.SafeSummary != "suppressed" || item.RedactionStatus != RedactionStatusSuppressed {
		t.Fatalf("expected unsafe turn summary suppressed, got %+v", item)
	}

	artifact := RuntimeArtifactExcerpt{
		ArtifactExcerptID: "artex_1",
		ResourceKind:      "run",
		ResourceID:        "run_1",
		ExcerptText:       SafeSummary("unsafe artifact", false).Text,
		RedactionStatus:   SafeSummary("unsafe artifact", false).Status,
	}
	if artifact.ExcerptText != "suppressed" || artifact.RedactionStatus != RedactionStatusSuppressed {
		t.Fatalf("expected unsafe artifact excerpt suppressed, got %+v", artifact)
	}
}

func TestSafeContinuityContentSuppressesSecretsAndProviderPayloads(t *testing.T) {
	for _, input := range []string{
		"Authorization: Bearer token_redacted",
		"api_key=sk-secretsecretsecret",
		`{"choices":[{"message":{"content":"raw"}}],"usage":{"total_tokens":1}}`,
	} {
		summary := SafeContinuityContent(input)
		if summary.Text != "suppressed" || summary.Status != RedactionStatusSuppressed {
			t.Fatalf("expected %q suppressed, got %+v", input, summary)
		}
	}
	summary := SafeContinuityContent("ordinary follow-up text")
	if summary.Text != "ordinary follow-up text" || summary.Status != RedactionStatusRedacted {
		t.Fatalf("expected ordinary text redacted-safe, got %+v", summary)
	}
}
