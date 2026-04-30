package evaluation

import "fmt"

type ToolCallDiffInput struct {
	Original map[string]any
	Replay   map[string]any
	Policy   RedactionPolicy
}

func RedactedToolCallDiff(input ToolCallDiffInput) (string, RedactionStatus) {
	original := RedactEvidencePayload(input.Original, input.Policy)
	replay := RedactEvidencePayload(input.Replay, input.Policy)
	status := RedactionStatusClean
	if original.Status == RedactionStatusRedacted || replay.Status == RedactionStatusRedacted {
		status = RedactionStatusRedacted
	}
	if fmt.Sprintf("%v", original.Payload) == fmt.Sprintf("%v", replay.Payload) {
		return "tool call evidence matched after redaction", status
	}
	return "tool call evidence drifted after redaction", status
}
