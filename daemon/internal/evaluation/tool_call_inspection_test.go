package evaluation

import (
	"testing"
	"time"
)

func TestToolCallInspectionClassificationStates(t *testing.T) {
	t.Parallel()

	base := ToolCallInspectionInput{
		TenantID:                 "ten_eval",
		CampaignID:               "campaign_eval",
		CampaignItemID:           "campaign_item_eval",
		ToolCallRef:              "tool_call_eval",
		OriginalEvidenceRef:      "original_evidence",
		NonLiveReplayEvidenceRef: "replay_evidence",
	}
	cases := []struct {
		name string
		edit func(*ToolCallInspectionInput)
		want string
	}{
		{name: "matched", want: InspectionMatched},
		{name: "drifted", edit: func(input *ToolCallInspectionInput) { input.Drifted = true }, want: InspectionDrifted},
		{name: "failed", edit: func(input *ToolCallInspectionInput) { input.Failed = true }, want: InspectionFailed},
		{name: "unsupported", edit: func(input *ToolCallInspectionInput) { input.Unsupported = true }, want: InspectionUnsupported},
		{name: "missing original", edit: func(input *ToolCallInspectionInput) { input.OriginalEvidenceRef = "" }, want: InspectionMissingOriginalEvidence},
		{name: "missing replay", edit: func(input *ToolCallInspectionInput) { input.NonLiveReplayEvidenceRef = "" }, want: InspectionMissingReplayEvidence},
		{name: "live denied", edit: func(input *ToolCallInspectionInput) { input.LiveValidationOutcome = "denied" }, want: InspectionLiveValidationDenied},
		{name: "live aborted", edit: func(input *ToolCallInspectionInput) { input.LiveValidationOutcome = "aborted" }, want: InspectionLiveValidationAborted},
		{name: "live failed", edit: func(input *ToolCallInspectionInput) { input.LiveValidationOutcome = "failed" }, want: InspectionLiveValidationFailed},
		{name: "live operator action", edit: func(input *ToolCallInspectionInput) { input.LiveValidationOutcome = "operator_action_needed" }, want: InspectionLiveValidationOperatorAction},
		{name: "live completed", edit: func(input *ToolCallInspectionInput) { input.LiveValidationOutcome = "completed" }, want: InspectionLiveValidationCompleted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			if tc.edit != nil {
				tc.edit(&input)
			}
			got, err := BuildToolCallInspection(input, time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatalf("BuildToolCallInspection: %v", err)
			}
			if got.Classification != tc.want {
				t.Fatalf("classification=%q, want %q", got.Classification, tc.want)
			}
		})
	}
}

func TestBuildToolCallInspectionRequiresEvidenceCoordinates(t *testing.T) {
	t.Parallel()

	_, err := BuildToolCallInspection(ToolCallInspectionInput{TenantID: "ten_eval"}, time.Now())
	if err != ErrEvaluationToolCallInspectionEvidenceRequired {
		t.Fatalf("err=%v, want evidence required", err)
	}
}
