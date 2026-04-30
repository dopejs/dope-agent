package evaluation

import (
	"errors"
	"testing"
	"time"
)

func TestRedactEvidencePayloadRemovesSensitiveFieldsBeforePersist(t *testing.T) {
	payload := map[string]any{
		"safe":          "value",
		"access_token":  "secret-token",
		"Authorization": "Bearer secret",
		"nested": map[string]any{
			"refresh_token": "secret-refresh",
			"count":         1,
		},
	}

	redacted := RedactEvidencePayload(payload, RedactionPolicy{
		SensitiveFieldRules: []string{"custom_secret"},
	})

	if redacted.Status != RedactionStatusRedacted {
		t.Fatalf("status=%s, want %s", redacted.Status, RedactionStatusRedacted)
	}
	if _, ok := redacted.Payload["access_token"]; ok {
		t.Fatal("access_token was not removed from redacted payload")
	}
	if _, ok := redacted.Payload["Authorization"]; ok {
		t.Fatal("Authorization was not removed from redacted payload")
	}
	nested, ok := redacted.Payload["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested payload type %T", redacted.Payload["nested"])
	}
	if _, ok := nested["refresh_token"]; ok {
		t.Fatal("nested refresh_token was not removed")
	}
	if nested["count"] != 1 {
		t.Fatalf("safe nested field changed: %v", nested["count"])
	}
	if len(redacted.SensitiveFieldsExcluded) != 3 {
		t.Fatalf("excluded fields=%v, want 3", redacted.SensitiveFieldsExcluded)
	}
}

func TestRedactEvidencePayloadHandlesNestedArraysAndConfiguredFields(t *testing.T) {
	payload := map[string]any{
		"events": []any{
			map[string]any{"credential": "secret", "message": "ok"},
			map[string]any{"custom_secret": "tenant-secret", "count": 2},
		},
		"profile": map[string]any{
			"api-key": "secret",
			"name":    "safe",
		},
	}

	redacted := RedactEvidencePayload(payload, RedactionPolicy{
		SensitiveFieldRules: []string{"custom_secret", "api-key"},
	})

	if redacted.Status != RedactionStatusRedacted {
		t.Fatalf("status=%s, want redacted", redacted.Status)
	}
	events := redacted.Payload["events"].([]any)
	first := events[0].(map[string]any)
	if _, ok := first["credential"]; ok {
		t.Fatal("array credential was not redacted")
	}
	second := events[1].(map[string]any)
	if _, ok := second["custom_secret"]; ok {
		t.Fatal("configured array field was not redacted")
	}
	profile := redacted.Payload["profile"].(map[string]any)
	if _, ok := profile["api-key"]; ok {
		t.Fatal("configured nested field was not redacted")
	}
	if len(redacted.SensitiveFieldsExcluded) != 3 {
		t.Fatalf("excluded fields=%v, want 3", redacted.SensitiveFieldsExcluded)
	}
}

func TestFailedClosedRedactedEvidenceCarriesReasonWithoutPayload(t *testing.T) {
	redacted := FailedClosedRedactedEvidence("evaluation.redaction_failed")

	if redacted.Status != RedactionStatusFailed {
		t.Fatalf("status=%s, want failed", redacted.Status)
	}
	if len(redacted.Payload) != 0 {
		t.Fatalf("failed-closed payload must be empty, got %+v", redacted.Payload)
	}
	if len(redacted.RedactionRulesApplied) != 1 || redacted.RedactionRulesApplied[0] != "failed_closed" {
		t.Fatalf("unexpected rules: %+v", redacted.RedactionRulesApplied)
	}
	if len(redacted.SensitiveFieldsExcluded) != 1 || redacted.SensitiveFieldsExcluded[0] != "evaluation.redaction_failed" {
		t.Fatalf("unexpected failure reason: %+v", redacted.SensitiveFieldsExcluded)
	}
}

func TestCandidateEvidenceFromPayloadRedactsBeforePersist(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	evidence, err := CandidateEvidenceFromPayload(CandidateEvidenceInput{
		TenantID:              "ten_eval",
		DiscoveredCandidateID: "candidate_1",
		SourceRefs:            []SourceRef{{Kind: SourceKindRun, ID: "run_1"}},
		Summary:               " candidate evidence ",
		Payload: map[string]any{
			"safe":         "value",
			"sessionToken": "secret",
			"nested": map[string]any{
				"custom_sensitive": "tenant secret",
				"status":           "failed",
			},
		},
		RedactionPolicy: RedactionPolicy{SensitiveFieldRules: []string{"custom_sensitive"}},
		Now:             now,
	})
	if err != nil {
		t.Fatalf("CandidateEvidenceFromPayload: %v", err)
	}
	if evidence.EvidenceID != "evidence_candidate_1" || evidence.CreatedAt != now {
		t.Fatalf("unexpected evidence identity/time: %+v", evidence)
	}
	if !evidence.MaterializationAllowed || evidence.RetentionState != RetentionStateActive {
		t.Fatalf("unexpected materialization/retention: %+v", evidence)
	}
	if _, ok := evidence.RedactedPayload["sessionToken"]; ok {
		t.Fatal("session token was persisted")
	}
	nested := evidence.RedactedPayload["nested"].(map[string]any)
	if _, ok := nested["custom_sensitive"]; ok {
		t.Fatal("configured sensitive field was persisted")
	}
	if len(evidence.SensitiveFieldsExcluded) != 2 {
		t.Fatalf("excluded fields=%v, want 2", evidence.SensitiveFieldsExcluded)
	}
}

func TestCandidateEvidenceFromPayloadRequiresCandidateSource(t *testing.T) {
	_, err := CandidateEvidenceFromPayload(CandidateEvidenceInput{TenantID: "ten_eval"})
	if !errors.Is(err, ErrEvaluationProductSourceRequired) {
		t.Fatalf("err=%v, want source required", err)
	}
}
